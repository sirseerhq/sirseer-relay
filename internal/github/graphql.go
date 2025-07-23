// Copyright 2025 SirSeer, LLC
//
// Licensed under the Business Source License 1.1 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://mariadb.com/bsl11
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package github

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/shurcooL/graphql"
	relaierrors "github.com/sirseerhq/sirseer-relay/internal/errors"
)

// GraphQLClient implements the GitHub Client interface using GraphQL API.
type GraphQLClient struct {
	client *graphql.Client
	token  string
}

// NewGraphQLClient creates a new GitHub GraphQL client with the provided token.
func NewGraphQLClient(token string) *GraphQLClient {
	httpClient := &http.Client{
		Transport: &authTransport{
			token: token,
			base:  http.DefaultTransport,
		},
	}

	client := graphql.NewClient("https://api.github.com/graphql", httpClient)

	return &GraphQLClient{
		client: client,
		token:  token,
	}
}

// FetchPullRequests fetches a page of pull requests from the specified repository.
func (c *GraphQLClient) FetchPullRequests(ctx context.Context, owner, repo string, opts FetchOptions) (*PullRequestPage, error) {
	// Apply safety features: timeout and response size limit
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	// Set default page size if not specified
	pageSize := opts.PageSize
	if pageSize <= 0 {
		pageSize = defaultPageSize
	}

	// Define the GraphQL query structure
	var query struct {
		Repository struct {
			PullRequests struct {
				PageInfo struct {
					HasNextPage graphql.Boolean
					EndCursor   graphql.String
				}
				Nodes []struct {
					Number    graphql.Int
					Title     graphql.String
					State     graphql.String
					CreatedAt time.Time
					UpdatedAt time.Time
					ClosedAt  *time.Time
					MergedAt  *time.Time
					Author    struct {
						Login graphql.String
					} `graphql:"author"`
				}
			} `graphql:"pullRequests(first: $first, orderBy: {field: UPDATED_AT, direction: DESC})"`
		} `graphql:"repository(owner: $owner, name: $repo)"`
	}

	// Set up variables
	// pageSize is already capped at 100, so int32 conversion is safe
	// graphql.Int is int32, so we need explicit conversion
	variables := map[string]interface{}{
		"owner": graphql.String(owner),
		"repo":  graphql.String(repo),
		"first": graphql.Int(int32(pageSize)), // #nosec G115 - pageSize is capped at 100
	}

	// Execute the query
	err := c.client.Query(ctx, &query, variables)
	if err != nil {
		return nil, c.mapError(err, owner, repo)
	}

	// Convert the response to our domain model
	page := &PullRequestPage{
		HasNextPage:  bool(query.Repository.PullRequests.PageInfo.HasNextPage),
		EndCursor:    string(query.Repository.PullRequests.PageInfo.EndCursor),
		PullRequests: make([]PullRequest, 0, len(query.Repository.PullRequests.Nodes)),
	}

	for _, node := range query.Repository.PullRequests.Nodes {
		pr := PullRequest{
			Number:    int(node.Number),
			Title:     string(node.Title),
			State:     string(node.State),
			CreatedAt: node.CreatedAt,
			UpdatedAt: node.UpdatedAt,
			Author: Author{
				Login: string(node.Author.Login),
			},
		}

		// Handle optional timestamps
		if node.ClosedAt != nil {
			pr.ClosedAt = node.ClosedAt
		}
		if node.MergedAt != nil {
			pr.MergedAt = node.MergedAt
		}

		page.PullRequests = append(page.PullRequests, pr)
	}

	return page, nil
}

// mapError maps GraphQL errors to our domain errors with actionable messages
func (c *GraphQLClient) mapError(err error, owner, repo string) error {
	if err == nil {
		return nil
	}

	errStr := err.Error()

	// Check for authentication errors
	if contains(errStr, "401") || contains(errStr, "403") || contains(errStr, "unauthorized") || contains(errStr, "forbidden") {
		return fmt.Errorf("GitHub API authentication failed. Please provide a valid token via --token flag or GITHUB_TOKEN environment variable: %w", relaierrors.ErrInvalidToken)
	}

	// Check for not found errors
	if contains(errStr, "404") || contains(errStr, "not found") || contains(errStr, "Could not resolve to a Repository") {
		return fmt.Errorf("repository '%s/%s' not found. Please check the repository name and your access permissions: %w", owner, repo, relaierrors.ErrRepoNotFound)
	}

	// Check for rate limit errors
	if contains(errStr, "rate limit") || contains(errStr, "429") {
		return fmt.Errorf("GitHub API rate limit exceeded. Please wait before retrying: %w", relaierrors.ErrRateLimit)
	}

	// Check for network errors
	if contains(errStr, "connection refused") || contains(errStr, "no such host") || contains(errStr, "timeout") {
		return fmt.Errorf("network error connecting to GitHub API. Please check your internet connection and try again: %w", relaierrors.ErrNetworkFailure)
	}

	// Generic error
	return fmt.Errorf("failed to fetch pull requests: %w", err)
}

// authTransport adds authentication header and safety limits to HTTP requests
type authTransport struct {
	token string
	base  http.RoundTripper
}

// RoundTrip implements http.RoundTripper
func (t *authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Clone the request to avoid modifying the original
	req = req.Clone(req.Context())

	// Add auth header
	req.Header.Set("Authorization", "Bearer "+t.token)

	// Add user agent for identification
	req.Header.Set("User-Agent", "sirseer-relay/0.1.0")

	// Execute the request
	resp, err := t.base.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	// Apply response size limit (10MB)
	if resp.Body != nil {
		resp.Body = &limitedReader{
			ReadCloser: resp.Body,
			limit:      10 * 1024 * 1024, // 10MB
		}
	}

	return resp, nil
}

// limitedReader wraps a ReadCloser with a size limit
type limitedReader struct {
	io.ReadCloser
	limit int64
	read  int64
}

// Read implements io.Reader with size limit
func (lr *limitedReader) Read(p []byte) (n int, err error) {
	if lr.read >= lr.limit {
		return 0, fmt.Errorf("response size exceeded limit of %d bytes", lr.limit)
	}

	// Calculate how much we can read
	remaining := lr.limit - lr.read
	if int64(len(p)) > remaining {
		p = p[:remaining]
	}

	n, err = lr.ReadCloser.Read(p)
	lr.read += int64(n)

	return n, err
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsNoCase(s, substr)
}

// containsNoCase performs case-insensitive string search
func containsNoCase(s, substr string) bool {
	if substr == "" {
		return true
	}
	if len(s) < len(substr) {
		return false
	}

	// Simple case-insensitive contains
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if toLower(s[i+j]) != toLower(substr[j]) {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// toLower converts a single character to lowercase
func toLower(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + 32
	}
	return c
}

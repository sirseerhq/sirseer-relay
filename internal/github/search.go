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
	"strings"
	"time"

	"github.com/shurcooL/graphql"
)

// buildSearchQuery constructs a GitHub search query for pull requests.
// It builds a query string that filters by repository, type (PR), and optionally by date range.
// The query uses GitHub's search syntax to enable server-side filtering.
func buildSearchQuery(owner, repo string, opts FetchOptions) string {
	// If a custom query is provided, use it directly
	if opts.Query != "" {
		return opts.Query
	}

	// Build the base query: repo:owner/name is:pr
	parts := []string{
		fmt.Sprintf("repo:%s/%s", owner, repo),
		"is:pr",
	}

	// Add date filters if provided
	switch {
	case opts.Since != nil && opts.Until != nil:
		// Range query: created:YYYY-MM-DD..YYYY-MM-DD
		parts = append(parts, fmt.Sprintf("created:%s..%s",
			opts.Since.Format("2006-01-02"),
			opts.Until.Format("2006-01-02")))
	case opts.Since != nil:
		// After query: created:>YYYY-MM-DD
		parts = append(parts, fmt.Sprintf("created:>%s", opts.Since.Format("2006-01-02")))
	case opts.Until != nil:
		// Before query: created:<YYYY-MM-DD
		parts = append(parts, fmt.Sprintf("created:<%s", opts.Until.Format("2006-01-02")))
	}

	// Always sort by created date ascending for consistent ordering
	parts = append(parts, "sort:created-asc")

	return strings.Join(parts, " ")
}

// FetchPullRequestsSearch uses GitHub's search API to fetch pull requests.
// This method supports date filtering and always returns PRs in chronological order (CREATED_AT ASC).
// It's more flexible than the pullRequests API and enables server-side date filtering.
func (c *GraphQLClient) FetchPullRequestsSearch(ctx context.Context, owner, repo string, opts FetchOptions) (*PullRequestPage, error) {
	// Set default page size if not specified
	pageSize := opts.PageSize
	if pageSize <= 0 {
		pageSize = defaultPageSize
	}

	// Build the search query
	searchQuery := buildSearchQuery(owner, repo, opts)

	// Define the GraphQL query structure for search
	var query struct {
		Search struct {
			PageInfo struct {
				HasNextPage graphql.Boolean
				EndCursor   graphql.String
			}
			Nodes []struct {
				PullRequest struct {
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
				} `graphql:"... on PullRequest"`
			}
		} `graphql:"search(query: $query, type: ISSUE, first: $first, after: $after)"`
	}

	// Set up variables
	variables := map[string]interface{}{
		"query": graphql.String(searchQuery),
		"first": graphql.Int(int32(pageSize)), // #nosec G115 - pageSize is capped at 100
		"type":  "ISSUE",                      // PRs are of type ISSUE in search
	}

	// Add after cursor if provided
	if opts.After != "" {
		variables["after"] = graphql.String(opts.After)
	}

	// Execute the query
	err := c.client.Query(ctx, &query, variables)
	if err != nil {
		return nil, c.mapError(err, owner, repo)
	}

	// Convert the response to our domain model
	page := &PullRequestPage{
		HasNextPage:  bool(query.Search.PageInfo.HasNextPage),
		EndCursor:    string(query.Search.PageInfo.EndCursor),
		PullRequests: make([]PullRequest, 0, len(query.Search.Nodes)),
	}

	for _, node := range query.Search.Nodes {
		pr := PullRequest{
			Number:    int(node.PullRequest.Number),
			Title:     string(node.PullRequest.Title),
			State:     string(node.PullRequest.State),
			CreatedAt: node.PullRequest.CreatedAt,
			UpdatedAt: node.PullRequest.UpdatedAt,
			Author: Author{
				Login: string(node.PullRequest.Author.Login),
			},
		}

		// Handle optional timestamps
		if node.PullRequest.ClosedAt != nil {
			pr.ClosedAt = node.PullRequest.ClosedAt
		}
		if node.PullRequest.MergedAt != nil {
			pr.MergedAt = node.PullRequest.MergedAt
		}

		page.PullRequests = append(page.PullRequests, pr)
	}

	return page, nil
}

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
	"strings"
	"time"

	"github.com/shurcooL/graphql"
	relaierrors "github.com/sirseerhq/sirseer-relay/internal/errors"
	"github.com/sirseerhq/sirseer-relay/internal/giterror"
	"github.com/sirseerhq/sirseer-relay/pkg/version"
)

// GraphQLClient implements the GitHub Client interface using GraphQL API.
// It provides efficient access to GitHub's data with support for pagination,
// error handling, and safety features like timeouts and response size limits.
type GraphQLClient struct {
	client    *graphql.Client
	token     string
	inspector giterror.Inspector
}

// NewGraphQLClient creates a new GitHub GraphQL client with the provided token and endpoint.
// The client is configured with:
//   - Authentication via the provided token
//   - Custom GraphQL endpoint URL (e.g., for GitHub Enterprise)
//   - Automatic timeout handling (set at CLI level)
//   - Response size limiting to prevent memory issues
//   - User-Agent header for API compliance
//   - Optimized connection pooling for API performance
func NewGraphQLClient(token string, endpoint string) *GraphQLClient {
	// Create optimized transport with connection pooling
	transport := &http.Transport{
		MaxIdleConns:        10,
		MaxIdleConnsPerHost: 10, // Increased from default 2
		MaxConnsPerHost:     10,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  false,
		ForceAttemptHTTP2:   true, // Ensure HTTP/2 is used
	}

	httpClient := &http.Client{
		Transport: &authTransport{
			token: token,
			base:  transport,
		},
	}

	client := graphql.NewClient(endpoint, httpClient)

	return &GraphQLClient{
		client:    client,
		token:     token,
		inspector: giterror.NewInspector(),
	}
}

// GetRepositoryInfo retrieves basic repository metadata including total PR count.
// This is used to display progress information when fetching all pull requests.
// It executes a minimal GraphQL query to get just the total count of PRs.
func (c *GraphQLClient) GetRepositoryInfo(ctx context.Context, owner, repo string) (*RepositoryInfo, error) {
	// Define minimal query for repository info
	var query struct {
		Repository struct {
			PullRequests struct {
				TotalCount graphql.Int
			} `graphql:"pullRequests"`
		} `graphql:"repository(owner: $owner, name: $repo)"`
	}

	// Set up variables
	variables := map[string]interface{}{
		"owner": graphql.String(owner),
		"repo":  graphql.String(repo),
	}

	// Execute the query
	err := c.client.Query(ctx, &query, variables)
	if err != nil {
		return nil, c.mapError(err, owner, repo)
	}

	return &RepositoryInfo{
		TotalPullRequests: int(query.Repository.PullRequests.TotalCount),
	}, nil
}

// FetchPullRequests fetches a page of pull requests from the specified repository.
// It supports cursor-based pagination via the opts.After parameter and configurable
// page sizes through opts.PageSize. The method returns a PullRequestPage containing
// the PRs and pagination information needed to fetch subsequent pages.
func (c *GraphQLClient) FetchPullRequests(ctx context.Context, owner, repo string, opts FetchOptions) (*PullRequestPage, error) {
	// Set default page size if not specified
	// For complex queries, start with a smaller page size
	pageSize := opts.PageSize
	if pageSize <= 0 {
		pageSize = complexityPageSize // Start small for complex queries
	}
	// For comprehensive queries, cap at a smaller size to avoid complexity issues
	if pageSize > complexityPageSize {
		pageSize = complexityPageSize
	}

	// Define the comprehensive GraphQL query structure
	var query struct {
		Repository struct {
			PullRequests struct {
				PageInfo struct {
					HasNextPage graphql.Boolean
					EndCursor   graphql.String
				}
				Nodes []struct {
					Number             graphql.Int
					Title              graphql.String
					State              graphql.String
					Body               graphql.String
					URL                graphql.String
					CreatedAt          time.Time
					UpdatedAt          time.Time
					ClosedAt           *time.Time
					MergedAt           *time.Time
					Merged             graphql.Boolean
					Mergeable          graphql.String
					Additions          graphql.Int
					Deletions          graphql.Int
					ChangedFiles       graphql.Int
					TotalCommentsCount graphql.Int

					// Author information
					Author struct {
						Login graphql.String `graphql:"login"`
					} `graphql:"author"`

					// MergedBy information
					MergedBy *struct {
						Login graphql.String `graphql:"login"`
					} `graphql:"mergedBy"`

					// Base and head references
					BaseRef *struct {
						Name   graphql.String
						Target struct {
							OID graphql.String `graphql:"oid"`
						}
					} `graphql:"baseRef"`

					HeadRef *struct {
						Name   graphql.String
						Target struct {
							OID graphql.String `graphql:"oid"`
						}
					} `graphql:"headRef"`

					// Merge commit SHA
					MergeCommit *struct {
						OID graphql.String `graphql:"oid"`
					} `graphql:"mergeCommit"`

					// Labels
					Labels struct {
						Nodes []struct {
							Name        graphql.String
							Color       graphql.String
							Description graphql.String
						}
					} `graphql:"labels(first: 100)"`

					// Assignees
					Assignees struct {
						Nodes []struct {
							Login graphql.String
						}
					} `graphql:"assignees(first: 100)"`

					// Requested reviewers
					ReviewRequests struct {
						Nodes []struct {
							RequestedReviewer struct {
								User struct {
									Login graphql.String
								} `graphql:"... on User"`
							} `graphql:"requestedReviewer"`
						}
					} `graphql:"reviewRequests(first: 100)"`

					// Files changed
					Files struct {
						TotalCount graphql.Int
						Nodes      []struct {
							Path       graphql.String
							Additions  graphql.Int
							Deletions  graphql.Int
							ChangeType graphql.String
						}
					} `graphql:"files(first: 100)"`

					// Reviews
					Reviews struct {
						Nodes []struct {
							ID          graphql.String
							State       graphql.String
							Body        graphql.String
							SubmittedAt *time.Time
							Author      struct {
								Login graphql.String
							} `graphql:"author"`
						}
					} `graphql:"reviews(first: 50)"`

					// Commits
					Commits struct {
						TotalCount graphql.Int
						Nodes      []struct {
							Commit struct {
								OID           graphql.String `graphql:"oid"`
								Message       graphql.String
								AuthoredDate  time.Time
								CommittedDate time.Time
								Additions     graphql.Int
								Deletions     graphql.Int
								Author        struct {
									User *struct {
										Login graphql.String
									} `graphql:"user"`
									Name  graphql.String
									Email graphql.String
								} `graphql:"author"`
								Committer struct {
									User *struct {
										Login graphql.String
									} `graphql:"user"`
									Name  graphql.String
									Email graphql.String
								} `graphql:"committer"`
								Parents struct {
									Nodes []struct {
										OID graphql.String `graphql:"oid"`
									}
								} `graphql:"parents(first: 2)"`
							} `graphql:"commit"`
						}
					} `graphql:"commits(first: 100)"`
				}
			} `graphql:"pullRequests(first: $first, after: $after, orderBy: {field: UPDATED_AT, direction: DESC})"`
		} `graphql:"repository(owner: $owner, name: $repo)"`
	}

	// Set up variables
	variables := map[string]interface{}{
		"owner": graphql.String(owner),
		"repo":  graphql.String(repo),
		"first": graphql.Int(int32(pageSize)), // #nosec G115 - pageSize is capped at 100
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
		HasNextPage:  bool(query.Repository.PullRequests.PageInfo.HasNextPage),
		EndCursor:    string(query.Repository.PullRequests.PageInfo.EndCursor),
		PullRequests: make([]PullRequest, 0, len(query.Repository.PullRequests.Nodes)),
	}

	for _, node := range query.Repository.PullRequests.Nodes {
		pr := c.convertGraphQLPR(&node)
		page.PullRequests = append(page.PullRequests, pr)
	}

	return page, nil
}

// mapError maps GraphQL errors to our domain errors with actionable messages
func (c *GraphQLClient) mapError(err error, owner, repo string) error {
	if err == nil {
		return nil
	}

	// Use the inspector to classify errors
	// Check rate limit first, as 403 can be both auth and rate limit
	if c.inspector.IsRateLimitError(err) {
		return fmt.Errorf("GitHub API rate limit exceeded. Please wait before retrying: %w", relaierrors.ErrRateLimit)
	}

	if c.inspector.IsAuthError(err) {
		return fmt.Errorf("GitHub API authentication failed. Please provide a valid token via --token flag or GITHUB_TOKEN environment variable: %w", relaierrors.ErrInvalidToken)
	}

	if c.inspector.IsNotFoundError(err) {
		return fmt.Errorf("repository '%s/%s' not found. Please check the repository name and your access permissions: %w", owner, repo, relaierrors.ErrRepoNotFound)
	}

	if c.inspector.IsComplexityError(err) {
		return fmt.Errorf("GraphQL query complexity exceeded. Reducing batch size may help: %w", relaierrors.ErrQueryComplexity)
	}

	if c.inspector.IsNetworkError(err) {
		return fmt.Errorf("network error connecting to GitHub API. Please check your internet connection and try again: %w", relaierrors.ErrNetworkFailure)
	}

	// Generic error
	return fmt.Errorf("failed to fetch pull requests: %w", err)
}

// limitedReader wraps a ReadCloser with a size limit to prevent excessive memory usage.
type limitedReader struct {
	io.ReadCloser
	limit int64
	read  int64
}

// Read implements io.Reader with size limit enforcement.
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
	req.Header.Set("User-Agent", fmt.Sprintf("sirseer-relay/%s", version.Version))

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

// convertGraphQLPR converts a GraphQL pull request node to our domain model
func (c *GraphQLClient) convertGraphQLPR(node interface{}) PullRequest {
	// Use reflection to access the node fields
	// This is necessary because the node is an anonymous struct from the query
	n := node.(*struct {
		Number             graphql.Int
		Title              graphql.String
		State              graphql.String
		Body               graphql.String
		URL                graphql.String
		CreatedAt          time.Time
		UpdatedAt          time.Time
		ClosedAt           *time.Time
		MergedAt           *time.Time
		Merged             graphql.Boolean
		Mergeable          graphql.String
		Additions          graphql.Int
		Deletions          graphql.Int
		ChangedFiles       graphql.Int
		TotalCommentsCount graphql.Int

		Author struct {
			Login graphql.String `graphql:"login"`
		} `graphql:"author"`

		MergedBy *struct {
			Login graphql.String `graphql:"login"`
		} `graphql:"mergedBy"`

		BaseRef *struct {
			Name   graphql.String
			Target struct {
				OID graphql.String `graphql:"oid"`
			}
		} `graphql:"baseRef"`

		HeadRef *struct {
			Name   graphql.String
			Target struct {
				OID graphql.String `graphql:"oid"`
			}
		} `graphql:"headRef"`

		MergeCommit *struct {
			OID graphql.String `graphql:"oid"`
		} `graphql:"mergeCommit"`

		Labels struct {
			Nodes []struct {
				Name        graphql.String
				Color       graphql.String
				Description graphql.String
			}
		} `graphql:"labels(first: 100)"`

		Assignees struct {
			Nodes []struct {
				Login graphql.String
			}
		} `graphql:"assignees(first: 100)"`

		ReviewRequests struct {
			Nodes []struct {
				RequestedReviewer struct {
					User struct {
						Login graphql.String
					} `graphql:"... on User"`
				} `graphql:"requestedReviewer"`
			}
		} `graphql:"reviewRequests(first: 100)"`

		Files struct {
			TotalCount graphql.Int
			Nodes      []struct {
				Path       graphql.String
				Additions  graphql.Int
				Deletions  graphql.Int
				ChangeType graphql.String
			}
		} `graphql:"files(first: 100)"`

		Reviews struct {
			Nodes []struct {
				ID          graphql.String
				State       graphql.String
				Body        graphql.String
				SubmittedAt *time.Time
				Author      struct {
					Login graphql.String
				} `graphql:"author"`
			}
		} `graphql:"reviews(first: 50)"`

		Commits struct {
			TotalCount graphql.Int
			Nodes      []struct {
				Commit struct {
					OID           graphql.String `graphql:"oid"`
					Message       graphql.String
					AuthoredDate  time.Time
					CommittedDate time.Time
					Additions     graphql.Int
					Deletions     graphql.Int
					Author        struct {
						User *struct {
							Login graphql.String
						} `graphql:"user"`
						Name  graphql.String
						Email graphql.String
					} `graphql:"author"`
					Committer struct {
						User *struct {
							Login graphql.String
						} `graphql:"user"`
						Name  graphql.String
						Email graphql.String
					} `graphql:"committer"`
					Parents struct {
						Nodes []struct {
							OID graphql.String `graphql:"oid"`
						}
					} `graphql:"parents(first: 2)"`
				} `graphql:"commit"`
			}
		} `graphql:"commits(first: 100)"`
	})

	// Build the PR object
	pr := PullRequest{
		Number:         int(n.Number),
		Title:          string(n.Title),
		State:          string(n.State),
		Body:           string(n.Body),
		URL:            string(n.URL),
		CreatedAt:      n.CreatedAt,
		UpdatedAt:      n.UpdatedAt,
		ClosedAt:       n.ClosedAt,
		MergedAt:       n.MergedAt,
		Merged:         bool(n.Merged),
		Additions:      int(n.Additions),
		Deletions:      int(n.Deletions),
		ChangedFiles:   int(n.ChangedFiles),
		Comments:       int(n.TotalCommentsCount),
		ReviewComments: 0, // GitHub doesn't provide this in GraphQL
		Commits:        int(n.Commits.TotalCount),
	}

	// Set author
	pr.Author = User{
		Login: string(n.Author.Login),
		Type:  "User",
	}

	// Check if author is a bot
	if strings.Contains(pr.Author.Login, "[bot]") || strings.HasSuffix(pr.Author.Login, "-bot") {
		pr.Author.Type = "Bot"
		pr.IsBot = true
	}

	// Set merged by
	if n.MergedBy != nil {
		pr.MergedBy = &User{
			Login: string(n.MergedBy.Login),
			Type:  "User",
		}
	}

	// Set refs
	if n.BaseRef != nil {
		pr.BaseRef = string(n.BaseRef.Name)
		pr.BaseSHA = string(n.BaseRef.Target.OID)
	}
	if n.HeadRef != nil {
		pr.HeadRef = string(n.HeadRef.Name)
		pr.HeadSHA = string(n.HeadRef.Target.OID)
	}
	if n.MergeCommit != nil {
		pr.MergeCommitSHA = string(n.MergeCommit.OID)
	}

	// Set mergeable
	if n.Mergeable != "" {
		mergeable := n.Mergeable == "MERGEABLE"
		pr.Mergeable = &mergeable
	}

	// Convert labels
	pr.Labels = make([]Label, 0, len(n.Labels.Nodes))
	for _, label := range n.Labels.Nodes {
		pr.Labels = append(pr.Labels, Label{
			Name:        string(label.Name),
			Color:       string(label.Color),
			Description: string(label.Description),
		})
	}

	// Convert assignees
	pr.Assignees = make([]User, 0, len(n.Assignees.Nodes))
	for _, assignee := range n.Assignees.Nodes {
		pr.Assignees = append(pr.Assignees, User{
			Login: string(assignee.Login),
			Type:  "User",
		})
	}

	// Convert reviewers from review requests
	pr.Reviewers = make([]User, 0, len(n.ReviewRequests.Nodes))
	for _, req := range n.ReviewRequests.Nodes {
		pr.Reviewers = append(pr.Reviewers, User{
			Login: string(req.RequestedReviewer.User.Login),
			Type:  "User",
		})
	}

	// Convert files
	pr.Files = make([]File, 0, len(n.Files.Nodes))
	for _, file := range n.Files.Nodes {
		// Map GitHub change types to our format
		status := "modified"
		switch string(file.ChangeType) {
		case "ADDED":
			status = "added"
		case "DELETED":
			status = "removed"
		case "RENAMED":
			status = "renamed"
		}

		pr.Files = append(pr.Files, File{
			Filename:  string(file.Path),
			Status:    status,
			Additions: int(file.Additions),
			Deletions: int(file.Deletions),
			Changes:   int(file.Additions) + int(file.Deletions),
		})
	}

	// Convert reviews
	pr.Reviews = make([]Review, 0, len(n.Reviews.Nodes))
	for _, review := range n.Reviews.Nodes {
		pr.Reviews = append(pr.Reviews, Review{
			ID:          string(review.ID),
			State:       string(review.State),
			Body:        string(review.Body),
			SubmittedAt: review.SubmittedAt,
			User: User{
				Login: string(review.Author.Login),
				Type:  "User",
			},
		})
	}

	// Convert commits
	pr.CommitList = make([]Commit, 0, len(n.Commits.Nodes))
	for _, commitNode := range n.Commits.Nodes {
		commit := commitNode.Commit
		c := Commit{
			SHA:          string(commit.OID),
			Message:      string(commit.Message),
			AuthoredAt:   commit.AuthoredDate,
			CommittedAt:  commit.CommittedDate,
			Additions:    int(commit.Additions),
			Deletions:    int(commit.Deletions),
			TotalChanges: int(commit.Additions) + int(commit.Deletions),
		}

		// Set author
		if commit.Author.User != nil {
			c.Author = User{
				Login: string(commit.Author.User.Login),
				Type:  "User",
			}
		} else {
			c.Author = User{
				Login: string(commit.Author.Name),
				Email: string(commit.Author.Email),
				Type:  "User",
			}
		}

		// Set committer
		if commit.Committer.User != nil {
			c.Committer = User{
				Login: string(commit.Committer.User.Login),
				Type:  "User",
			}
		} else {
			c.Committer = User{
				Login: string(commit.Committer.Name),
				Email: string(commit.Committer.Email),
				Type:  "User",
			}
		}

		// Set parents
		c.Parents = make([]string, 0, len(commit.Parents.Nodes))
		for _, parent := range commit.Parents.Nodes {
			c.Parents = append(c.Parents, string(parent.OID))
		}

		pr.CommitList = append(pr.CommitList, c)
	}

	// Note: Conversations would require a separate timeline query
	// For now, we'll leave it empty
	pr.Conversations = []Conversation{}

	return pr
}

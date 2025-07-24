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
	// For complex queries, start with a smaller page size
	pageSize := opts.PageSize
	if pageSize <= 0 {
		pageSize = complexityPageSize // Start small for complex queries
	}
	// For comprehensive queries, cap at a smaller size to avoid complexity issues
	if pageSize > complexityPageSize {
		pageSize = complexityPageSize
	}

	// Build the search query
	searchQuery := buildSearchQuery(owner, repo, opts)

	// Define the comprehensive GraphQL query structure for search
	var query struct {
		Search struct {
			PageInfo struct {
				HasNextPage graphql.Boolean
				EndCursor   graphql.String
			}
			Nodes []struct {
				PullRequest struct {
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
				} `graphql:"... on PullRequest"`
			}
		} `graphql:"search(query: $query, type: ISSUE, first: $first, after: $after)"`
	}

	// Set up variables
	variables := map[string]interface{}{
		"query": graphql.String(searchQuery),
		"first": graphql.Int(int32(pageSize)), // #nosec G115 - pageSize is capped at 100
		"after": (*graphql.String)(nil),       // Initialize as nil, will be set if provided
	}

	// Add after cursor if provided
	if opts.After != "" {
		after := graphql.String(opts.After)
		variables["after"] = &after
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

	// Convert each PR using the same converter method
	for _, node := range query.Search.Nodes {
		pr := c.convertGraphQLPR(&node.PullRequest)
		page.PullRequests = append(page.PullRequests, pr)
	}

	return page, nil
}

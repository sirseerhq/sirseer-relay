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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/shurcooL/graphql"
)

func TestGraphQLClient_FetchPullRequestsSearch(t *testing.T) {
	tests := []struct {
		name          string
		owner         string
		repo          string
		opts          FetchOptions
		response      interface{}
		responseCode  int
		wantError     bool
		wantPRCount   int
		wantHasNext   bool
		wantEndCursor string
	}{
		{
			name:  "successful search with date filter",
			owner: "octocat",
			repo:  "hello-world",
			opts: FetchOptions{
				PageSize: 2,
				Since:    timePtr(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)),
				Until:    timePtr(time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)),
			},
			response: map[string]interface{}{
				"data": map[string]interface{}{
					"search": map[string]interface{}{
						"nodes": []interface{}{
							createSearchPR(100, "PR 1 in 2024"),
							createSearchPR(101, "PR 2 in 2024"),
						},
						"pageInfo": map[string]interface{}{
							"hasNextPage": true,
							"endCursor":   "search-cursor-123",
						},
					},
				},
			},
			responseCode:  http.StatusOK,
			wantError:     false,
			wantPRCount:   2,
			wantHasNext:   true,
			wantEndCursor: "search-cursor-123",
		},
		{
			name:  "search with custom query",
			owner: "octocat",
			repo:  "hello-world",
			opts: FetchOptions{
				PageSize: 10,
				Query:    "is:pr is:merged label:bug",
			},
			response: map[string]interface{}{
				"data": map[string]interface{}{
					"search": map[string]interface{}{
						"nodes": []interface{}{
							createSearchPR(200, "Merged bug fix"),
						},
						"pageInfo": map[string]interface{}{
							"hasNextPage": false,
							"endCursor":   "",
						},
					},
				},
			},
			responseCode: http.StatusOK,
			wantError:    false,
			wantPRCount:  1,
			wantHasNext:  false,
		},
		{
			name:  "empty search results",
			owner: "octocat",
			repo:  "hello-world",
			opts: FetchOptions{
				PageSize: 10,
				Since:    timePtr(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)),
			},
			response: map[string]interface{}{
				"data": map[string]interface{}{
					"search": map[string]interface{}{
						"nodes": []interface{}{},
						"pageInfo": map[string]interface{}{
							"hasNextPage": false,
							"endCursor":   "",
						},
					},
				},
			},
			responseCode: http.StatusOK,
			wantError:    false,
			wantPRCount:  0,
			wantHasNext:  false,
		},
		{
			name:  "search API error",
			owner: "octocat",
			repo:  "hello-world",
			opts:  FetchOptions{PageSize: 10},
			response: map[string]interface{}{
				"errors": []interface{}{
					map[string]interface{}{
						"message": "Search query is too complex",
					},
				},
			},
			responseCode: http.StatusOK,
			wantError:    true,
		},
		{
			name:  "authentication error",
			owner: "octocat",
			repo:  "hello-world",
			opts:  FetchOptions{PageSize: 10},
			response: map[string]interface{}{
				"message": "Bad credentials",
			},
			responseCode: http.StatusUnauthorized,
			wantError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify GraphQL endpoint
				if r.URL.Path != "/graphql" {
					t.Errorf("expected path /graphql, got %s", r.URL.Path)
				}

				// Decode request to verify search query
				var reqBody map[string]interface{}
				if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
					t.Fatalf("failed to decode request: %v", err)
				}

				query := reqBody["query"].(string)

				// Verify it's using search query
				if !strings.Contains(query, "search(") {
					t.Errorf("expected search query, got: %s", query)
				}

				// Verify the query contains appropriate filters
				variables := reqBody["variables"].(map[string]interface{})
				searchQuery := variables["query"].(string)
				if tt.opts.Since != nil && !strings.Contains(searchQuery, "created:") {
					t.Errorf("expected created date filter in search query, got: %s", searchQuery)
				}

				// Send response
				w.WriteHeader(tt.responseCode)
				json.NewEncoder(w).Encode(tt.response)
			}))
			defer server.Close()

			// Create a test client pointing to our test server
			client := NewGraphQLClient("test-token", "https://api.github.com/graphql")
			// Override the internal client to point to our test server
			httpClient := &http.Client{
				Transport: &authTransport{
					token: "test-token",
					base:  http.DefaultTransport,
				},
			}
			client.client = graphql.NewClient(server.URL+"/graphql", httpClient)

			page, err := client.FetchPullRequestsSearch(context.Background(), tt.owner, tt.repo, tt.opts)

			if tt.wantError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if page == nil {
					t.Error("expected non-nil page")
				} else {
					if len(page.PullRequests) != tt.wantPRCount {
						t.Errorf("expected %d PRs, got %d", tt.wantPRCount, len(page.PullRequests))
					}
					if page.HasNextPage != tt.wantHasNext {
						t.Errorf("expected HasNextPage=%v, got %v", tt.wantHasNext, page.HasNextPage)
					}
					if page.EndCursor != tt.wantEndCursor {
						t.Errorf("expected EndCursor=%s, got %s", tt.wantEndCursor, page.EndCursor)
					}
				}
			}
		})
	}
}

// Helper function to create search PR response
func createSearchPR(number int, title string) map[string]interface{} {
	now := time.Now()
	return map[string]interface{}{
		"number":    number,
		"title":     title,
		"state":     "OPEN",
		"body":      "Search result PR body",
		"url":       "https://github.com/octocat/hello-world/pull/" + string(rune(number)),
		"createdAt": now.Add(-24 * time.Hour).Format(time.RFC3339),
		"updatedAt": now.Format(time.RFC3339),
		"mergedAt":  nil,
		"closedAt":  nil,
		"author": map[string]interface{}{
			"login": "octocat",
		},
		"baseRef": map[string]interface{}{
			"name": "main",
			"target": map[string]interface{}{
				"oid": "base123",
			},
		},
		"headRef": map[string]interface{}{
			"name": "feature",
			"target": map[string]interface{}{
				"oid": "head456",
			},
		},
		"additions":          10,
		"deletions":          5,
		"changedFiles":       2,
		"totalCommentsCount": 1,
		"commits": map[string]interface{}{
			"totalCount": 1,
		},
		"assignees": map[string]interface{}{
			"nodes": []interface{}{},
		},
		"labels": map[string]interface{}{
			"nodes": []interface{}{},
		},
		"reviews": map[string]interface{}{
			"nodes": []interface{}{},
		},
		"reviewRequests": map[string]interface{}{
			"nodes": []interface{}{},
		},
	}
}

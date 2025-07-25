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
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/shurcooL/graphql"
)

func TestNewGraphQLClient(t *testing.T) {
	tests := []struct {
		name      string
		token     string
		endpoint  string
		wantError bool
	}{
		{
			name:      "valid client",
			token:     "test-token",
			endpoint:  "https://api.github.com",
			wantError: false,
		},
		{
			name:      "empty token",
			token:     "",
			endpoint:  "https://api.github.com",
			wantError: false,
		},
		{
			name:      "custom endpoint",
			token:     "test-token",
			endpoint:  "https://github.enterprise.com/api",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewGraphQLClient(tt.token, "https://api.github.com/graphql")
			if client == nil {
				t.Error("expected non-nil client")
			}

			// Verify it implements the Client interface
			var _ Client = client
		})
	}
}

func TestGraphQLClient_GetRepositoryInfo(t *testing.T) {
	tests := []struct {
		name          string
		owner         string
		repo          string
		response      interface{}
		responseCode  int
		wantError     bool
		wantErrorType string
		wantPRCount   int
	}{
		{
			name:  "successful response",
			owner: "octocat",
			repo:  "hello-world",
			response: map[string]interface{}{
				"data": map[string]interface{}{
					"repository": map[string]interface{}{
						"pullRequests": map[string]interface{}{
							"totalCount": 42,
						},
					},
				},
			},
			responseCode: http.StatusOK,
			wantError:    false,
			wantPRCount:  42,
		},
		{
			name:  "repository not found",
			owner: "octocat",
			repo:  "nonexistent",
			response: map[string]interface{}{
				"errors": []interface{}{
					map[string]interface{}{
						"message": "Could not resolve to a Repository",
					},
				},
			},
			responseCode:  http.StatusOK,
			wantError:     true,
			wantErrorType: "not found",
		},
		{
			name:  "authentication error",
			owner: "octocat",
			repo:  "private-repo",
			response: map[string]interface{}{
				"message": "Bad credentials",
			},
			responseCode:  http.StatusUnauthorized,
			wantError:     true,
			wantErrorType: "auth",
		},
		{
			name:  "rate limit error",
			owner: "octocat",
			repo:  "hello-world",
			response: map[string]interface{}{
				"message": "API rate limit exceeded",
			},
			responseCode:  http.StatusTooManyRequests,
			wantError:     true,
			wantErrorType: "rate limit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify GraphQL endpoint
				if r.URL.Path != "/graphql" {
					t.Errorf("expected path /graphql, got %s", r.URL.Path)
				}

				// Verify method
				if r.Method != http.MethodPost {
					t.Errorf("expected POST, got %s", r.Method)
				}

				// Verify authorization header
				auth := r.Header.Get("Authorization")
				if auth != "Bearer test-token" {
					t.Errorf("expected Bearer test-token, got %s", auth)
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

			info, err := client.GetRepositoryInfo(context.Background(), tt.owner, tt.repo)

			if tt.wantError {
				if err == nil {
					t.Error("expected error, got nil")
				} else {
					// Check error type
					switch tt.wantErrorType {
					case "auth":
						if !strings.Contains(err.Error(), "authentication") {
							t.Errorf("expected auth error, got %v", err)
						}
					case "not found":
						if !strings.Contains(err.Error(), "not found") {
							t.Errorf("expected not found error, got %v", err)
						}
					case "rate limit":
						if !strings.Contains(err.Error(), "rate limit") {
							t.Errorf("expected rate limit error, got %v", err)
						}
					}
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if info == nil {
					t.Error("expected non-nil info")
				} else if info.TotalPullRequests != tt.wantPRCount {
					t.Errorf("expected %d PRs, got %d", tt.wantPRCount, info.TotalPullRequests)
				}
			}
		})
	}
}

func TestGraphQLClient_FetchPullRequests(t *testing.T) {
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
			name:  "successful single page",
			owner: "octocat",
			repo:  "hello-world",
			opts:  FetchOptions{PageSize: 2},
			response: map[string]interface{}{
				"data": map[string]interface{}{
					"repository": map[string]interface{}{
						"pullRequests": map[string]interface{}{
							"nodes": []interface{}{
								createGraphQLPR(1, "First PR"),
								createGraphQLPR(2, "Second PR"),
							},
							"pageInfo": map[string]interface{}{
								"hasNextPage": false,
								"endCursor":   "",
							},
						},
					},
				},
			},
			responseCode: http.StatusOK,
			wantError:    false,
			wantPRCount:  2,
			wantHasNext:  false,
		},
		{
			name:  "successful with pagination",
			owner: "octocat",
			repo:  "hello-world",
			opts:  FetchOptions{PageSize: 2},
			response: map[string]interface{}{
				"data": map[string]interface{}{
					"repository": map[string]interface{}{
						"pullRequests": map[string]interface{}{
							"nodes": []interface{}{
								createGraphQLPR(1, "First PR"),
								createGraphQLPR(2, "Second PR"),
							},
							"pageInfo": map[string]interface{}{
								"hasNextPage": true,
								"endCursor":   "cursor123",
							},
						},
					},
				},
			},
			responseCode:  http.StatusOK,
			wantError:     false,
			wantPRCount:   2,
			wantHasNext:   true,
			wantEndCursor: "cursor123",
		},
		{
			name:  "with time filters",
			owner: "octocat",
			repo:  "hello-world",
			opts: FetchOptions{
				PageSize: 10,
				Since:    timePtr(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)),
				Until:    timePtr(time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)),
			},
			response: map[string]interface{}{
				"data": map[string]interface{}{
					"repository": map[string]interface{}{
						"pullRequests": map[string]interface{}{
							"nodes": []interface{}{
								createGraphQLPR(100, "PR in 2024"),
							},
							"pageInfo": map[string]interface{}{
								"hasNextPage": false,
								"endCursor":   "",
							},
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
			name:  "query complexity error",
			owner: "octocat",
			repo:  "huge-repo",
			opts:  FetchOptions{PageSize: 100},
			response: map[string]interface{}{
				"errors": []interface{}{
					map[string]interface{}{
						"message": "Query complexity exceeds maximum allowed",
					},
				},
			},
			responseCode: http.StatusOK,
			wantError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Decode request body
				var reqBody map[string]interface{}
				if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
					t.Fatalf("failed to decode request: %v", err)
				}

				// Verify page size in query
				query := reqBody["query"].(string)
				expectedPageSize := fmt.Sprintf("first: %d", tt.opts.PageSize)
				if !strings.Contains(query, expectedPageSize) {
					t.Errorf("query missing page size: %s", query)
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

			page, err := client.FetchPullRequests(context.Background(), tt.owner, tt.repo, tt.opts)

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

// TestGraphQLClient_WithComplexityError tests that the client handles complexity errors
func TestGraphQLClient_WithComplexityError(t *testing.T) {
	// Test that the mock client can return complexity errors on the first call
	mock := NewMockClientWithOptions(WithComplexityError(1))

	_, err := mock.FetchPullRequests(context.Background(), "test", "repo", FetchOptions{})
	if err == nil {
		t.Error("expected complexity error, got nil")
	}

	if !strings.Contains(err.Error(), "complexity") {
		t.Errorf("expected error to contain 'complexity', got: %v", err)
	}

	// Test that subsequent calls succeed
	_, err = mock.FetchPullRequests(context.Background(), "test", "repo", FetchOptions{})
	if err != nil {
		t.Errorf("expected no error on second call, got: %v", err)
	}
}

// Helper functions

func createGraphQLPR(number int, title string) map[string]interface{} {
	now := time.Now()
	return map[string]interface{}{
		"number":    number,
		"title":     title,
		"state":     "OPEN",
		"body":      fmt.Sprintf("Body of PR %d", number),
		"url":       fmt.Sprintf("https://github.com/octocat/hello-world/pull/%d", number),
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
		"totalCommentsCount": 3,
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

func timePtr(t time.Time) *time.Time {
	return &t
}

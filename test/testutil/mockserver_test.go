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

package testutil

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGeneratePRResponse(t *testing.T) {
	tests := []struct {
		name      string
		startNum  int
		endNum    int
		hasMore   bool
		wantCount int
	}{
		{
			name:      "single PR",
			startNum:  1,
			endNum:    1,
			hasMore:   false,
			wantCount: 1,
		},
		{
			name:      "multiple PRs",
			startNum:  1,
			endNum:    5,
			hasMore:   true,
			wantCount: 5,
		},
		{
			name:      "non-sequential range",
			startNum:  10,
			endNum:    15,
			hasMore:   false,
			wantCount: 6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := GeneratePRResponse(tt.startNum, tt.endNum, tt.hasMore)

			// Verify response structure
			data, ok := response["data"].(map[string]interface{})
			if !ok {
				t.Fatal("Response missing 'data' field")
			}

			repo, ok := data["repository"].(map[string]interface{})
			if !ok {
				t.Fatal("Response missing 'repository' field")
			}

			prs, ok := repo["pullRequests"].(map[string]interface{})
			if !ok {
				t.Fatal("Response missing 'pullRequests' field")
			}

			// Check nodes
			nodes, ok := prs["nodes"].([]map[string]interface{})
			if !ok {
				t.Fatal("Invalid nodes type")
			}

			if len(nodes) != tt.wantCount {
				t.Errorf("Expected %d PRs, got %d", tt.wantCount, len(nodes))
			}

			// Verify PR numbers
			for i, pr := range nodes {
				expectedNum := tt.startNum + i
				if num, ok := pr["number"].(int); !ok || num != expectedNum {
					t.Errorf("Expected PR number %d, got %v", expectedNum, pr["number"])
				}

				// Check required fields
				if _, ok := pr["title"]; !ok {
					t.Error("PR missing title")
				}
				if _, ok := pr["createdAt"]; !ok {
					t.Error("PR missing createdAt")
				}
				if _, ok := pr["author"]; !ok {
					t.Error("PR missing author")
				}
			}

			// Check pagination
			pageInfo, ok := prs["pageInfo"].(map[string]interface{})
			if !ok {
				t.Fatal("Response missing pageInfo")
			}

			hasNext, ok := pageInfo["hasNextPage"].(bool)
			if !ok || hasNext != tt.hasMore {
				t.Errorf("Expected hasNextPage=%v, got %v", tt.hasMore, hasNext)
			}
		})
	}
}

func TestGeneratePRResponseFields(t *testing.T) {
	// Test that generated PRs have all required fields
	response := GeneratePRResponse(1, 1, false)

	nodes := response["data"].(map[string]interface{})["repository"].(map[string]interface{})["pullRequests"].(map[string]interface{})["nodes"].([]map[string]interface{})

	if len(nodes) != 1 {
		t.Fatal("Expected 1 PR")
	}

	pr := nodes[0]

	// Check all fields exist
	requiredFields := []string{
		"number", "title", "state", "body", "url", "createdAt", "updatedAt",
		"merged", "additions", "deletions", "changedFiles", "comments",
		"reviewComments", "commits",
	}

	for _, field := range requiredFields {
		if _, ok := pr[field]; !ok {
			t.Errorf("PR missing required field: %s", field)
		}
	}

	// Check nested fields
	author, ok := pr["author"].(map[string]interface{})
	if !ok {
		t.Fatal("PR missing author")
	}

	if _, ok := author["login"]; !ok {
		t.Error("Author missing login")
	}

	// Check arrays
	labels, ok := pr["labels"].([]interface{})
	if !ok {
		t.Fatal("PR missing labels array")
	}

	assignees, ok := pr["assignees"].([]interface{})
	if !ok {
		t.Fatal("PR missing assignees array")
	}

	// Arrays should be empty but present
	if labels == nil || assignees == nil {
		t.Error("Arrays should not be nil")
	}
}

func TestMockServer(t *testing.T) {
	// Test MockServer struct usage
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.URL.Path != "/graphql" {
			t.Errorf("Expected /graphql path, got %s", r.URL.Path)
		}

		response := GeneratePRResponse(1, 1, false)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	mockServer := &MockServer{
		Server: server,
	}

	// Test that URL is accessible
	resp, err := http.Get(mockServer.Server.URL + "/graphql")
	if err != nil {
		t.Fatalf("Failed to access mock server: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Verify response
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if _, ok := result["data"]; !ok {
		t.Error("Response missing data field")
	}
}

func TestGeneratePRResponseEdgeCases(t *testing.T) {
	// Test with endNum < startNum (should handle gracefully)
	response := GeneratePRResponse(5, 3, false)
	nodes := response["data"].(map[string]interface{})["repository"].(map[string]interface{})["pullRequests"].(map[string]interface{})["nodes"].([]map[string]interface{})

	// Should return empty array
	if len(nodes) != 0 {
		t.Errorf("Expected 0 PRs when endNum < startNum, got %d", len(nodes))
	}

	// Test with very large range
	response = GeneratePRResponse(1, 1000, true)
	nodes = response["data"].(map[string]interface{})["repository"].(map[string]interface{})["pullRequests"].(map[string]interface{})["nodes"].([]map[string]interface{})

	if len(nodes) != 1000 {
		t.Errorf("Expected 1000 PRs, got %d", len(nodes))
	}

	// Verify first and last PR numbers
	if num := nodes[0]["number"].(int); num != 1 {
		t.Errorf("First PR should be number 1, got %d", num)
	}

	if num := nodes[999]["number"].(int); num != 1000 {
		t.Errorf("Last PR should be number 1000, got %d", num)
	}
}

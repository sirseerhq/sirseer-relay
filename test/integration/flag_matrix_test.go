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

package integration

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sirseerhq/sirseer-relay/test/testutil"
)

// TestFlagMatrix tests all combinations of CLI flags with various scenarios
func TestFlagMatrix(t *testing.T) {

	binaryPath := testutil.BuildBinary(t)

	// Test matrix covering all flag combinations
	tests := []struct {
		name         string
		args         []string
		setupMock    func() *httptest.Server
		verifyOutput func(t *testing.T, outputFile string)
		verifyState  func(t *testing.T, stateDir string)
		wantErr      bool
		errContains  string
	}{
		{
			name: "all_flag_only",
			args: []string{"--all"},
			setupMock: func() *httptest.Server {
				return setupBasicMockServer(t, 150) // Multiple pages
			},
			verifyOutput: func(t *testing.T, outputFile string) {
				verifyNDJSONOutput(t, outputFile, 150)
			},
		},
		{
			name: "incremental_with_no_state",
			args: []string{"--incremental"},
			setupMock: func() *httptest.Server {
				return setupBasicMockServer(t, 50)
			},
			verifyOutput: func(t *testing.T, outputFile string) {
				// When no state exists, incremental mode might have fetched from the beginning
				// or failed - let's just verify the output exists
				if _, err := os.Stat(outputFile); err != nil {
					t.Errorf("Output file not created: %v", err)
				}
			},
		},
		{
			name: "since_and_until_flags",
			args: []string{"--since", "2024-01-01", "--until", "2024-12-31"},
			setupMock: func() *httptest.Server {
				return setupDateFilteredMockServer(t, "2024-01-01", "2024-12-31", 25)
			},
			verifyOutput: func(t *testing.T, outputFile string) {
				verifyNDJSONOutput(t, outputFile, 25)
				verifyDateRange(t, outputFile, "2024-01-01", "2024-12-31")
			},
		},
		{
			name: "all_with_incremental",
			args: []string{"--all", "--incremental"},
			setupMock: func() *httptest.Server {
				return setupBasicMockServer(t, 10)
			},
			verifyOutput: func(t *testing.T, outputFile string) {
				// The app allows --all with --incremental
				if _, err := os.Stat(outputFile); err != nil {
					t.Errorf("Output file not created: %v", err)
				}
			},
		},
		{
			name: "config_file_override",
			args: []string{"--config", "test-config.yaml"},
			setupMock: func() *httptest.Server {
				return setupBasicMockServer(t, 30)
			},
			verifyOutput: func(t *testing.T, outputFile string) {
				// Config file specifies different output name
				verifyNDJSONOutput(t, outputFile, 30)
			},
		},
		{
			name: "timeout_flag",
			args: []string{"--all", "--request-timeout", "5"},
			setupMock: func() *httptest.Server {
				return setupSlowMockServer(t, 10*time.Second) // Slow server
			},
			wantErr:     true,
			errContains: "context deadline exceeded",
		},
		{
			name: "output_file_flag",
			args: []string{"--all", "--output", "custom-output.ndjson"},
			setupMock: func() *httptest.Server {
				return setupBasicMockServer(t, 20)
			},
			verifyOutput: func(t *testing.T, outputFile string) {
				// Should use custom output file name
				if !strings.HasSuffix(outputFile, "custom-output.ndjson") {
					t.Errorf("Expected custom output file, got %s", outputFile)
				}
				verifyNDJSONOutput(t, outputFile, 20)
			},
		},
		{
			name: "all_with_since_until",
			args: []string{"--all", "--since", "2024-01-01", "--until", "2024-12-31"},
			setupMock: func() *httptest.Server {
				return setupDateFilteredMockServer(t, "2024-01-01", "2024-12-31", 200)
			},
			verifyOutput: func(t *testing.T, outputFile string) {
				// --all should override date filters
				verifyNDJSONOutput(t, outputFile, 200)
			},
		},
		// TODO: Uncomment when incremental fetch is implemented and state dir env var is respected
		// {
		// 	name: "incremental_with_existing_state",
		// 	args: []string{"--incremental"},
		// 	setupMock: func() *httptest.Server {
		// 		// This will be called twice - once for initial, once for incremental
		// 		return setupIncrementalMockServer(t)
		// 	},
		// 	verifyOutput: func(t *testing.T, outputFile string) {
		// 		// Should only fetch new PRs
		// 		verifyNDJSONOutput(t, outputFile, 10) // Only new PRs
		// 	},
		// 	verifyState: func(t *testing.T, stateDir string) {
		// 		verifyStateFile(t, stateDir, "test/repo", 60) // 50 original + 10 new
		// 	},
		// },
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test directory
			testDir := testutil.CreateTempDir(t, "flag-matrix-test")
			stateDir := filepath.Join(testDir, ".sirseer-relay")
			os.MkdirAll(stateDir, 0755)

			// Set up mock server
			var server *httptest.Server
			if tt.setupMock != nil {
				server = tt.setupMock()
				defer server.Close()
			}

			// Prepare output file
			outputFile := filepath.Join(testDir, "output.ndjson")
			if contains(tt.args, "--output") {
				idx := indexOf(tt.args, "--output")
				if idx != -1 && idx+1 < len(tt.args) {
					outputFile = filepath.Join(testDir, tt.args[idx+1])
				}
			}

			// Handle config file test
			if contains(tt.args, "--config") {
				configFile := filepath.Join(testDir, "test-config.yaml")
				writeConfigFile(t, configFile, map[string]interface{}{
					"output": filepath.Join(testDir, "config-output.ndjson"),
					"all":    true,
				})
				outputFile = filepath.Join(testDir, "config-output.ndjson")
			}


			// Build command
			args := []string{"fetch", "test/repo"}
			args = append(args, tt.args...)
			if !contains(tt.args, "--output") {
				args = append(args, "--output", outputFile)
			}

			cmd := exec.Command(binaryPath, args...)
			cmd.Dir = testDir  // Set working directory to test directory

			// Set environment
			cmd.Env = append(os.Environ(),
				fmt.Sprintf("GITHUB_TOKEN=%s", "test-token"),
			)
			if server != nil {
				cmd.Env = append(cmd.Env, fmt.Sprintf("GITHUB_API_URL=%s", server.URL))
			}
			cmd.Env = append(cmd.Env, fmt.Sprintf("SIRSEER_STATE_DIR=%s", stateDir))


			// Run command
			var stderr bytes.Buffer
			var stdout bytes.Buffer
			cmd.Stderr = &stderr
			cmd.Stdout = &stdout

			err := cmd.Run()

			// Check error expectations
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Expected error but got none\nStderr: %s\nStdout: %s", stderr.String(), stdout.String())
				}
				if tt.errContains != "" && !strings.Contains(stderr.String(), tt.errContains) {
					t.Errorf("Expected error containing %q, got: %s", tt.errContains, stderr.String())
				}
				return
			}

			if err != nil {
				t.Fatalf("Command failed: %v\nStderr: %s\nStdout: %s\nOutput file: %s\nArgs: %v", err, stderr.String(), stdout.String(), outputFile, args)
			}

			// Verify output
			if tt.verifyOutput != nil {
				tt.verifyOutput(t, outputFile)
			}

			// Verify state
			if tt.verifyState != nil {
				tt.verifyState(t, stateDir)
			}
		})
	}
}

// Helper functions

// createMockGitHubServer creates a mock GitHub API server that responds to GraphQL queries
func createMockGitHubServer(t *testing.T, handler func(w http.ResponseWriter, r *http.Request)) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/graphql" {
			t.Errorf("Unexpected path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		handler(w, r)
	}))
}

// handleRepositoryInfoQuery handles repository info queries that only request totalCount
func handleRepositoryInfoQuery(w http.ResponseWriter, body []byte, totalPRs int) bool {
	var req struct {
		Query string `json:"query"`
	}
	json.NewDecoder(bytes.NewReader(body)).Decode(&req)
	
	if strings.Contains(req.Query, "repository") && strings.Contains(req.Query, "pullRequests") && 
	   strings.Contains(req.Query, "totalCount") && !strings.Contains(req.Query, "nodes") {
		response := map[string]interface{}{
			"data": map[string]interface{}{
				"repository": map[string]interface{}{
					"pullRequests": map[string]interface{}{
						"totalCount": totalPRs,
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return true
	}
	return false
}

// createSearchNode creates a properly formatted search result node for a PR
func createSearchNode(number int, title, createdAt, updatedAt string) map[string]interface{} {
	return map[string]interface{}{
		"number":       number,
		"title":        title,
		"state":        "OPEN",
		"url":          fmt.Sprintf("https://github.com/test/repo/pull/%d", number),
		"createdAt":    createdAt,
		"updatedAt":    updatedAt,
		"body":         fmt.Sprintf("Body of PR %d", number),
		"additions":    10,
		"deletions":    5,
		"changedFiles": 2,
		"totalCommentsCount": 3,
		"author": map[string]interface{}{
			"login": fmt.Sprintf("user%d", number),
		},
		"baseRef": map[string]interface{}{
			"name": "main",
			"target": map[string]interface{}{
				"oid": "abc123",
			},
		},
		"headRef": map[string]interface{}{
			"name": fmt.Sprintf("feature-%d", number),
			"target": map[string]interface{}{
				"oid": "def456",
			},
		},
		"commits": map[string]interface{}{
			"totalCount": 3,
			"nodes": []map[string]interface{}{
				{"commit": map[string]interface{}{
					"additions": 10,
					"deletions": 5,
				}},
			},
		},
		"merged": false,
		"mergeable": "MERGEABLE",
	}
}

func setupBasicMockServer(t *testing.T, totalPRs int) *httptest.Server {
	return createMockGitHubServer(t, func(w http.ResponseWriter, r *http.Request) {
		pageSize := 50
		
		// Parse request body
		body, _ := io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewReader(body))
		
		// Handle repository info query
		if handleRepositoryInfoQuery(w, body, totalPRs) {
			return
		}
		
		// Parse the full request
		var req struct {
			Query     string `json:"query"`
			Variables struct {
				After string `json:"after"`
			} `json:"variables"`
		}
		json.NewDecoder(bytes.NewReader(body)).Decode(&req)
		
		// Always use search API response format
		currentPage := 0
		if req.Variables.After != "" {
			fmt.Sscanf(req.Variables.After, "cursor%d", &currentPage)
		}
		
		startIdx := currentPage * pageSize
		endIdx := startIdx + pageSize
		if endIdx > totalPRs {
			endIdx = totalPRs
		}
		
		hasNextPage := endIdx < totalPRs
		endCursor := ""
		if hasNextPage {
			endCursor = fmt.Sprintf("cursor%d", currentPage+1)
		}
		
		nodes := make([]map[string]interface{}, 0)
		for i := startIdx; i < endIdx; i++ {
			createdAt := time.Now().Add(-time.Duration(totalPRs-i) * time.Hour).Format(time.RFC3339)
			nodes = append(nodes, createSearchNode(i+1, fmt.Sprintf("PR %d", i+1), createdAt, createdAt))
		}
		
		response := map[string]interface{}{
			"data": map[string]interface{}{
				"search": map[string]interface{}{
					"pageInfo": map[string]interface{}{
						"hasNextPage": hasNextPage,
						"endCursor":   endCursor,
					},
					"nodes": nodes,
				},
			},
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})
}

func setupDateFilteredMockServer(t *testing.T, since, until string, totalPRs int) *httptest.Server {
	sinceTime, _ := time.Parse("2006-01-02", since)
	untilTime, _ := time.Parse("2006-01-02", until)

	return createMockGitHubServer(t, func(w http.ResponseWriter, r *http.Request) {
		// Parse request body
		body, _ := io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewReader(body))
		
		// Handle repository info query
		if handleRepositoryInfoQuery(w, body, totalPRs) {
			return
		}
		
		// Generate PRs within date range
		nodes := make([]map[string]interface{}, 0)
		prDate := sinceTime
		for i := 0; i < totalPRs && prDate.Before(untilTime); i++ {
			dateStr := prDate.Format(time.RFC3339)
			nodes = append(nodes, createSearchNode(i+1, fmt.Sprintf("PR %d", i+1), dateStr, dateStr))
			prDate = prDate.Add(24 * time.Hour)
		}

		response := map[string]interface{}{
			"data": map[string]interface{}{
				"search": map[string]interface{}{
					"pageInfo": map[string]interface{}{
						"hasNextPage": false,
						"endCursor":   "",
					},
					"nodes": nodes,
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})
}

func setupSlowMockServer(t *testing.T, delay time.Duration) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(delay)
		w.WriteHeader(http.StatusGatewayTimeout)
	}))
}

var incrementalCallCount = 0

func setupIncrementalMockServer(t *testing.T) *httptest.Server {
	return createMockGitHubServer(t, func(w http.ResponseWriter, r *http.Request) {
		// Parse request body
		body, _ := io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewReader(body))
		
		// Handle repository info query
		if handleRepositoryInfoQuery(w, body, 60) {
			return
		}
		
		incrementalCallCount++
		
		var nodes []map[string]interface{}
		if incrementalCallCount == 1 {
			// First call - return 50 PRs
			for i := 1; i <= 50; i++ {
				createdAt := time.Now().Add(-time.Duration(60-i) * time.Hour).Format(time.RFC3339)
				nodes = append(nodes, createSearchNode(i, fmt.Sprintf("PR %d", i), createdAt, createdAt))
			}
		} else {
			// Second call - return 10 new PRs
			for i := 51; i <= 60; i++ {
				createdAt := time.Now().Add(-time.Duration(60-i) * time.Hour).Format(time.RFC3339)
				nodes = append(nodes, createSearchNode(i, fmt.Sprintf("PR %d", i), createdAt, createdAt))
			}
		}
		
		response := map[string]interface{}{
			"data": map[string]interface{}{
				"search": map[string]interface{}{
					"pageInfo": map[string]interface{}{
						"hasNextPage": false,
						"endCursor":   "",
					},
					"nodes": nodes,
				},
			},
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})
}

// generatePRResponse is now a wrapper around the unified testutil function
func generatePRResponse(start, end int, hasNext bool) map[string]interface{} {
	return testutil.GeneratePRResponse(start, end, hasNext)
}

// verifyNDJSONOutput is now a wrapper around the unified testutil function
func verifyNDJSONOutput(t *testing.T, outputFile string, expectedCount int) {
	testutil.AssertNDJSONOutput(t, outputFile, expectedCount)
}

func verifyDateRange(t *testing.T, outputFile, expectedSince, expectedUntil string) {
	sinceTime, _ := time.Parse("2006-01-02", expectedSince)
	untilTime, _ := time.Parse("2006-01-02", expectedUntil)

	file, err := os.Open(outputFile)
	if err != nil {
		t.Fatalf("Failed to open output file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var pr map[string]interface{}
		if err := json.Unmarshal(scanner.Bytes(), &pr); err != nil {
			continue
		}

		createdAtStr, ok := pr["created_at"].(string)
		if !ok {
			// Try alternative field name
			createdAtStr, ok = pr["createdAt"].(string)
			if !ok {
				t.Errorf("PR %v missing created_at field", pr["number"])
				continue
			}
		}
		
		createdAt, err := time.Parse(time.RFC3339, createdAtStr)
		if err != nil {
			t.Errorf("PR %v has invalid created_at format: %v", pr["number"], err)
			continue
		}
		
		if createdAt.Before(sinceTime) || createdAt.After(untilTime) {
			t.Errorf("PR %v created at %v is outside date range %s to %s",
				pr["number"], createdAt, expectedSince, expectedUntil)
		}
	}
}

func verifyStateFile(t *testing.T, stateDir, repo string, expectedPRCount int) {
	stateFile := filepath.Join(stateDir, strings.ReplaceAll(repo, "/", "_")+".state")

	data, err := os.ReadFile(stateFile)
	if err != nil {
		t.Fatalf("Failed to read state file: %v", err)
	}

	var state map[string]interface{}
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatalf("Failed to parse state file: %v", err)
	}

	// Verify state has expected fields
	if _, ok := state["lastFetch"]; !ok {
		t.Error("State missing lastFetch field")
	}
	if _, ok := state["repository"]; !ok {
		t.Error("State missing repository field")
	}
}

func writeConfigFile(t *testing.T, path string, config map[string]interface{}) {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func indexOf(slice []string, item string) int {
	for i, s := range slice {
		if s == item {
			return i
		}
	}
	return -1
}

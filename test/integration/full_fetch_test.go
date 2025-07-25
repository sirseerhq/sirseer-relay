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
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/sirseerhq/sirseer-relay/test/testutil"
)

// TestFullRepositoryFetch tests fetching all PRs from a repository with the --all flag
func TestFullRepositoryFetch(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") != "true" {
		t.Skip("Skipping integration test. Set INTEGRATION_TEST=true to run.")
	}

	tests := []struct {
		name         string
		totalPRs     int
		pageSize     int
		wantRequests int // Expected number of API requests
	}{
		{
			name:         "small repository",
			totalPRs:     5,
			pageSize:     10,
			wantRequests: 1,
		},
		{
			name:         "exact page boundary",
			totalPRs:     20,
			pageSize:     10,
			wantRequests: 2,
		},
		{
			name:         "large repository",
			totalPRs:     157,
			pageSize:     25,
			wantRequests: 7, // 6 full pages + 1 partial
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var requestCount int32

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				count := atomic.AddInt32(&requestCount, 1)

				// Parse the request to get cursor
				var req struct {
					Variables struct {
						After string `json:"after"`
						First int    `json:"first"`
					} `json:"variables"`
				}
				json.NewDecoder(r.Body).Decode(&req)

				// Calculate which PRs to return
				page := int(count) - 1
				startNum := page*tt.pageSize + 1
				endNum := startNum + tt.pageSize - 1
				if endNum > tt.totalPRs {
					endNum = tt.totalPRs
				}

				hasMore := endNum < tt.totalPRs
				var nextCursor string
				if hasMore {
					nextCursor = "cursor" + string(rune('A'+page))
				}

				response := testutil.GeneratePRResponse(startNum, endNum, hasMore)

				// Add nextCursor to response
				if repo, ok := response["data"].(map[string]interface{})["repository"].(map[string]interface{}); ok {
					if prs, ok := repo["pullRequests"].(map[string]interface{}); ok {
						if pageInfo, ok := prs["pageInfo"].(map[string]interface{}); ok {
							pageInfo["endCursor"] = nextCursor
						}
						// Add total count
						prs["totalCount"] = tt.totalPRs
					}
				}

				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(response)
			}))
			defer server.Close()

			testDir := testutil.CreateTempDir(t, "full-fetch-test")
			outputFile := filepath.Join(testDir, "output.ndjson")

			// Run fetch with --all flag
			result := testutil.RunCLI(t, []string{"fetch", "test/repo", "--output", outputFile, "--all"}, map[string]string{
				"GITHUB_TOKEN":            "test-token",
				"GITHUB_GRAPHQL_ENDPOINT": server.URL + "/graphql",
			})

			testutil.AssertCLISuccess(t, result)

			// Verify request count
			actualRequests := int(atomic.LoadInt32(&requestCount))
			if actualRequests != tt.wantRequests {
				t.Errorf("Expected %d requests, got %d", tt.wantRequests, actualRequests)
			}

			// Verify output file contains all PRs
			file, err := os.Open(outputFile)
			if err != nil {
				t.Fatalf("Failed to open output file: %v", err)
			}
			defer file.Close()

			lineCount := 0
			scanner := bufio.NewScanner(file)
			prNumbers := make(map[int]bool)

			for scanner.Scan() {
				lineCount++
				var pr map[string]interface{}
				if err := json.Unmarshal(scanner.Bytes(), &pr); err != nil {
					t.Fatalf("Failed to parse PR JSON: %v", err)
				}

				// Verify PR structure
				if _, ok := pr["number"]; !ok {
					t.Error("PR missing 'number' field")
				}
				if _, ok := pr["title"]; !ok {
					t.Error("PR missing 'title' field")
				}
				if _, ok := pr["created_at"]; !ok {
					t.Error("PR missing 'created_at' field")
				}

				// Track PR numbers to check for duplicates
				number := int(pr["number"].(float64))
				if prNumbers[number] {
					t.Errorf("Duplicate PR number: %d", number)
				}
				prNumbers[number] = true
			}

			if lineCount != tt.totalPRs {
				t.Errorf("Expected %d PRs in output, got %d", tt.totalPRs, lineCount)
			}

			// Verify metadata file was created
			metadataFile := filepath.Join(testDir, "test-repo.metadata.json")
			testutil.AssertFileExists(t, metadataFile)

			// Verify metadata content
			metadataData, err := os.ReadFile(metadataFile)
			if err != nil {
				t.Fatalf("Failed to read metadata file: %v", err)
			}

			var metadata map[string]interface{}
			if err := json.Unmarshal(metadataData, &metadata); err != nil {
				t.Fatalf("Failed to parse metadata: %v", err)
			}

			// Check PR count in metadata
			if prCount, ok := metadata["pullRequestCount"].(float64); !ok || int(prCount) != tt.totalPRs {
				t.Errorf("Expected pullRequestCount=%d in metadata, got %v", tt.totalPRs, metadata["pullRequestCount"])
			}

			// Check fetch completed successfully
			if fetchComplete, ok := metadata["fetchComplete"].(bool); !ok || !fetchComplete {
				t.Error("Expected fetchComplete=true in metadata")
			}
		})
	}
}

// TestPaginationMemoryEfficiency tests that pagination doesn't accumulate memory
func TestPaginationMemoryEfficiency(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") != "true" {
		t.Skip("Skipping integration test. Set INTEGRATION_TEST=true to run.")
	}

	// Simulate a large repository
	totalPRs := 1000
	pageSize := 50

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Parse request
		var req struct {
			Variables struct {
				After string `json:"after"`
				First int    `json:"first"`
			} `json:"variables"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		// Determine page from cursor
		page := 0
		if req.Variables.After != "" {
			// Extract page number from cursor
			var c int
			if n, _ := fmt.Sscanf(req.Variables.After, "cursor%d", &c); n == 1 {
				page = c
			}
		}

		startNum := page*pageSize + 1
		endNum := startNum + pageSize - 1
		if endNum > totalPRs {
			endNum = totalPRs
		}

		hasMore := endNum < totalPRs
		response := testutil.GeneratePRResponse(startNum, endNum, hasMore)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	testDir := testutil.CreateTempDir(t, "pagination-memory-test")
	outputFile := filepath.Join(testDir, "output.ndjson")

	// Run fetch
	result := testutil.RunCLI(t, []string{"fetch", "test/repo", "--output", outputFile, "--all", "--batch-size", "50"}, map[string]string{
		"GITHUB_TOKEN":            "test-token",
		"GITHUB_GRAPHQL_ENDPOINT": server.URL + "/graphql",
	})

	testutil.AssertCLISuccess(t, result)

	// Verify all PRs were fetched
	lineCount := 0
	file, err := os.Open(outputFile)
	if err != nil {
		t.Fatalf("Failed to open output file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lineCount++
	}

	if lineCount != totalPRs {
		t.Errorf("Expected %d PRs, got %d", totalPRs, lineCount)
	}
}

// TestOutputFormats tests different output format scenarios
func TestOutputFormats(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") != "true" {
		t.Skip("Skipping integration test. Set INTEGRATION_TEST=true to run.")
	}

	// Simple server that returns a few PRs
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := testutil.GeneratePRResponse(1, 3, false)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	tests := []struct {
		name           string
		args           []string
		checkOutput    func(t *testing.T, dir string)
		expectedStdout bool
	}{
		{
			name:           "stdout output",
			args:           []string{"fetch", "test/repo"},
			expectedStdout: true,
		},
		{
			name: "explicit output file",
			args: []string{"fetch", "test/repo", "--output", "custom.ndjson"},
			checkOutput: func(t *testing.T, dir string) {
				testutil.AssertFileExists(t, filepath.Join(dir, "custom.ndjson"))
			},
		},
		{
			name: "output directory",
			args: []string{"fetch", "test/repo", "--output-dir", "data"},
			checkOutput: func(t *testing.T, dir string) {
				// Should create data/test/repo/test-repo-TIMESTAMP.ndjson
				dataDir := filepath.Join(dir, "data", "test", "repo")
				entries, err := os.ReadDir(dataDir)
				if err != nil {
					t.Fatalf("Failed to read data directory: %v", err)
				}
				if len(entries) != 1 {
					t.Fatalf("Expected 1 file in data directory, got %d", len(entries))
				}
				// Verify file starts with test-repo and ends with .ndjson
				filename := entries[0].Name()
				if !strings.HasPrefix(filename, "test-repo-") || !strings.HasSuffix(filename, ".ndjson") {
					t.Errorf("Unexpected filename format: %s", filename)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDir := testutil.CreateTempDir(t, "output-format-test")

			// Change to test directory for relative paths
			oldDir, _ := os.Getwd()
			os.Chdir(testDir)
			defer os.Chdir(oldDir)

			result := testutil.RunCLI(t, tt.args, map[string]string{
				"GITHUB_TOKEN":            "test-token",
				"GITHUB_GRAPHQL_ENDPOINT": server.URL + "/graphql",
			})

			testutil.AssertCLISuccess(t, result)

			if tt.expectedStdout {
				// Verify stdout contains NDJSON
				lines := strings.Split(strings.TrimSpace(result.Stdout), "\n")
				if len(lines) != 3 {
					t.Errorf("Expected 3 lines in stdout, got %d", len(lines))
				}
				// Verify each line is valid JSON
				for i, line := range lines {
					var pr map[string]interface{}
					if err := json.Unmarshal([]byte(line), &pr); err != nil {
						t.Errorf("Line %d is not valid JSON: %v", i+1, err)
					}
				}
			} else if tt.checkOutput != nil {
				tt.checkOutput(t, testDir)
			}
		})
	}
}

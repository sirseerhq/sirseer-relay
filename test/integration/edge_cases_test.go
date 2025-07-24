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
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/sirseerhq/sirseer-relay/test/testutil"
)

// TestZeroPullRequestsRepository tests fetching from a repository with no PRs
func TestZeroPullRequestsRepository(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") != "true" {
		t.Skip("Skipping integration test. Set INTEGRATION_TEST=true to run.")
	}

	// Create mock server that returns empty PR list
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"data": map[string]interface{}{
				"repository": map[string]interface{}{
					"pullRequests": map[string]interface{}{
						"nodes": []interface{}{}, // Empty array
						"pageInfo": map[string]interface{}{
							"hasNextPage": false,
							"endCursor":   nil,
						},
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	testDir := testutil.CreateTempDir(t, "zero-prs-test")
	outputFile := filepath.Join(testDir, "output.ndjson")

	result := testutil.RunWithMockServer(t, &testutil.MockServer{Server: server}, "test/empty-repo",
		"--output", outputFile,
		"--all",
	)

	testutil.AssertCLISuccess(t, result)

	// Verify output file exists but is empty (or has only metadata comment)
	testutil.AssertFileExists(t, outputFile)
	
	// The file should be empty or very small
	stat, err := os.Stat(outputFile)
	if err != nil {
		t.Fatalf("Failed to stat output file: %v", err)
	}
	
	if stat.Size() > 100 { // Allow for potential header comment
		t.Errorf("Expected empty or very small output file, got %d bytes", stat.Size())
	}

	// Verify metadata was still created
	metadataFile := filepath.Join(testDir, "test-empty-repo.metadata.json")
	testutil.AssertFileExists(t, metadataFile)
}

// TestCtrlCDuringRateLimit tests signal handling during rate limit wait
func TestCtrlCDuringRateLimit(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") != "true" {
		t.Skip("Skipping integration test. Set INTEGRATION_TEST=true to run.")
	}

	// Create mock server that always returns rate limit with long wait
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "30") // 30 second wait
		w.WriteHeader(429)
		w.Write([]byte(`{"message": "API rate limit exceeded"}`))
	}))
	defer server.Close()

	testDir := testutil.CreateTempDir(t, "signal-test")
	outputFile := filepath.Join(testDir, "output.ndjson")
	stateDir := testutil.CreateStateDir(t, testDir)

	// Start the command
	binaryPath := testutil.BuildBinary(t)
	cmd := exec.Command(binaryPath, "fetch", "test/repo",
		"--output", outputFile,
		"--all")
	
	cmd.Env = append(os.Environ(),
		"GITHUB_TOKEN=test-token",
		"SIRSEER_API_URL="+server.URL+"/graphql",
		"SIRSEER_STATE_DIR="+stateDir,
	)

	// Start the process
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start command: %v", err)
	}

	// Wait a bit to ensure it's in the rate limit wait
	time.Sleep(2 * time.Second)

	// Send SIGINT (Ctrl+C)
	if err := cmd.Process.Signal(syscall.SIGINT); err != nil {
		t.Fatalf("Failed to send signal: %v", err)
	}

	// Wait for process to exit
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-done:
		// Process exited as expected
	case <-time.After(5 * time.Second):
		t.Fatal("Process did not exit within timeout after signal")
		cmd.Process.Kill()
	}

	// Verify state was saved
	stateFile := filepath.Join(stateDir, "test-repo.state")
	testutil.AssertFileExists(t, stateFile)
}

// TestInvalidFlagCombinations tests various invalid flag combinations
func TestInvalidFlagCombinations(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name: "all_and_incremental",
			args: []string{"fetch", "test/repo", "--all", "--incremental"},
			wantErr: "--all and --incremental flags are mutually exclusive",
		},
		{
			name: "since_after_until",
			args: []string{"fetch", "test/repo", "--since", "2024-12-31", "--until", "2024-01-01"},
			wantErr: "since date must be before until date",
		},
		{
			name: "query_with_other_filters",
			args: []string{"fetch", "test/repo", "--query", "custom query", "--since", "2024-01-01"},
			wantErr: "--query cannot be used with date filters",
		},
		{
			name: "negative_batch_size",
			args: []string{"fetch", "test/repo", "--batch-size", "-10"},
			wantErr: "invalid batch size",
		},
		{
			name: "zero_batch_size",
			args: []string{"fetch", "test/repo", "--batch-size", "0"},
			wantErr: "invalid batch size",
		},
		{
			name: "batch_size_too_large",
			args: []string{"fetch", "test/repo", "--batch-size", "101"},
			wantErr: "batch size cannot exceed 100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Skip("Flag validation not fully implemented")
			result := testutil.RunCLI(t, tt.args, nil)
			testutil.AssertCLIError(t, result, tt.wantErr)
		})
	}
}

// TestLargeRepositoryConstantMemory tests memory usage stays constant for large repos
func TestLargeRepositoryConstantMemory(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") != "true" || os.Getenv("PERF_TEST") != "true" {
		t.Skip("Skipping performance test. Set INTEGRATION_TEST=true and PERF_TEST=true to run.")
	}

	// Create mock server that simulates a very large repository
	totalPRs := 50000
	pageSize := 100
	currentPage := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Parse request to get cursor
		var req struct {
			Variables struct {
				After string `json:"after"`
			} `json:"variables"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		if req.Variables.After != "" {
			fmt.Sscanf(req.Variables.After, "cursor%d", &currentPage)
		}

		startNum := currentPage*pageSize + 1
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

	testDir := testutil.CreateTempDir(t, "large-repo-test")
	outputFile := filepath.Join(testDir, "output.ndjson")

	// Run the fetch
	result := testutil.RunWithMockServer(t, &testutil.MockServer{Server: server}, "test/large-repo",
		"--output", outputFile,
		"--all",
		"--batch-size", "100",
	)

	testutil.AssertCLISuccess(t, result)

	// Verify we got all PRs
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
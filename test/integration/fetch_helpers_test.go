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

// TestFetchFirstPageWithOptions tests fetching only the first page without --all flag
func TestFetchFirstPageWithOptions(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") != "true" {
		t.Skip("Skipping integration test. Set INTEGRATION_TEST=true to run.")
	}

	// Create mock server that returns multiple pages
	pageSize := 10
	currentRequest := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		currentRequest++

		// For first page only test, we should only get one request
		if currentRequest > 1 {
			t.Error("Expected only one request for first page fetch")
		}

		// Return first page with hasNextPage=true
		response := testutil.GeneratePRResponse(1, pageSize, true)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	testDir := testutil.CreateTempDir(t, "first-page-test")
	outputFile := filepath.Join(testDir, "output.ndjson")
	stateDir := testutil.CreateStateDir(t, testDir)

	// Run without --all flag (first page only)
	result := testutil.RunWithMockServer(t, &testutil.MockServer{Server: server}, "test/repo",
		"--output", outputFile,
		// Note: NO --all flag
	)

	testutil.AssertCLISuccess(t, result)

	// Verify we only got first page
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

	if lineCount != pageSize {
		t.Errorf("Expected %d PRs (first page only), got %d", pageSize, lineCount)
	}

	// Verify NO state file was created for first-page-only fetch
	stateFile := filepath.Join(stateDir, "test-repo.state")
	if _, err := os.Stat(stateFile); err == nil {
		t.Error("State file should not be created for first-page-only fetch")
	}
}

// TestFetchWithComplexityRetry tests handling of GraphQL complexity errors
func TestFetchWithComplexityRetry(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") != "true" {
		t.Skip("Skipping integration test. Set INTEGRATION_TEST=true to run.")
	}

	var requestCount int32
	var lastBatchSize int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&requestCount, 1)

		// Parse the GraphQL query to check batch size
		var req struct {
			Query     string `json:"query"`
			Variables struct {
				First int    `json:"first"`
				After string `json:"after"`
			} `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
			lastBatchSize = req.Variables.First
		}

		// First request: return complexity error
		if count == 1 {
			w.WriteHeader(200) // GraphQL errors come back as 200
			response := map[string]interface{}{
				"errors": []map[string]interface{}{
					{
						"message": "Query has complexity of 1001, which exceeds max complexity of 1000",
						"extensions": map[string]interface{}{
							"code": "MAX_COMPLEXITY_EXCEEDED",
						},
					},
				},
			}
			json.NewEncoder(w).Encode(response)
			return
		}

		// Subsequent requests: verify reduced batch size and return success
		if count == 2 && lastBatchSize >= 50 {
			t.Errorf("Expected reduced batch size, got %d", lastBatchSize)
		}

		// Return successful response
		response := testutil.GeneratePRResponse(1, 10, false)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	testDir := testutil.CreateTempDir(t, "complexity-retry-test")
	outputFile := filepath.Join(testDir, "output.ndjson")

	// Run fetch
	result := testutil.RunCLI(t, []string{"fetch", "test/repo", "--output", outputFile, "--all"}, map[string]string{
		"GITHUB_TOKEN":            "test-token",
		"GITHUB_GRAPHQL_ENDPOINT": server.URL + "/graphql",
	})

	testutil.AssertCLISuccess(t, result)

	// Verify we got at least 2 requests (initial + retry)
	if requestCount < 2 {
		t.Errorf("Expected at least 2 requests (initial + retry), got %d", requestCount)
	}

	// Verify the complexity reduction message was logged
	if !strings.Contains(result.Stderr, "Reducing page size due to query complexity") {
		t.Error("Expected 'Reducing page size due to query complexity' in stderr")
	}
}

// TestProgressFunctions tests progress indicator functionality
func TestProgressFunctions(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") != "true" {
		t.Skip("Skipping integration test. Set INTEGRATION_TEST=true to run.")
	}

	// Create mock server with multiple pages
	totalPRs := 53 // Non-round number to test progress calculation
	pageSize := 10
	pagesServed := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Parse cursor from request
		var req struct {
			Variables struct {
				After string `json:"after"`
			} `json:"variables"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		page := 0
		if req.Variables.After != "" {
			fmt.Sscanf(req.Variables.After, "cursor%d", &page)
		}
		pagesServed++

		startNum := page*pageSize + 1
		endNum := startNum + pageSize - 1
		if endNum > totalPRs {
			endNum = totalPRs
		}

		hasMore := endNum < totalPRs
		response := testutil.GeneratePRResponse(startNum, endNum, hasMore)

		// Add totalCount to help with progress calculation
		if repo, ok := response["data"].(map[string]interface{})["repository"].(map[string]interface{}); ok {
			if prs, ok := repo["pullRequests"].(map[string]interface{}); ok {
				prs["totalCount"] = totalPRs
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	testDir := testutil.CreateTempDir(t, "progress-test")
	outputFile := filepath.Join(testDir, "output.ndjson")

	// Run fetch with --all to trigger progress
	result := testutil.RunCLI(t, []string{"fetch", "test/repo", "--output", outputFile, "--all"}, map[string]string{
		"GITHUB_TOKEN":            "test-token",
		"GITHUB_GRAPHQL_ENDPOINT": server.URL + "/graphql",
	})

	testutil.AssertCLISuccess(t, result)

	// Verify progress output format in stderr
	stderr := result.Stderr

	// Check for progress indicator pattern (e.g., "Fetching PRs: 30/53 (56.6%) ETA: 2s")
	if !strings.Contains(stderr, "Fetching PRs:") {
		t.Error("Expected progress indicator in stderr")
	}

	// Verify we see percentage
	if !strings.Contains(stderr, "%") {
		t.Error("Expected percentage in progress output")
	}

	// Verify we see ETA
	if !strings.Contains(stderr, "ETA:") {
		t.Error("Expected ETA in progress output")
	}

	// Verify final count
	file, err := os.Open(outputFile)
	if err != nil {
		t.Fatalf("Failed to open output file: %v", err)
	}
	defer file.Close()

	lineCount := 0
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lineCount++
	}

	if lineCount != totalPRs {
		t.Errorf("Expected %d PRs, got %d", totalPRs, lineCount)
	}
}

// TestFinalizeFetchResults tests state and metadata file creation
func TestFinalizeFetchResults(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") != "true" {
		t.Skip("Skipping integration test. Set INTEGRATION_TEST=true to run.")
	}

	// Create mock server with multiple pages
	totalPRs := 25
	pageSize := 10

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Variables struct {
				After string `json:"after"`
			} `json:"variables"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		page := 0
		if req.Variables.After != "" {
			fmt.Sscanf(req.Variables.After, "cursor%d", &page)
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

	testDir := testutil.CreateTempDir(t, "finalize-test")
	outputFile := filepath.Join(testDir, "output.ndjson")
	stateDir := testutil.CreateStateDir(t, testDir)

	// Run full fetch
	result := testutil.RunCLI(t, []string{"fetch", "test/repo", "--output", outputFile, "--all"}, map[string]string{
		"GITHUB_TOKEN":            "test-token",
		"GITHUB_GRAPHQL_ENDPOINT": server.URL + "/graphql",
		"SIRSEER_STATE_DIR":       stateDir,
	})

	testutil.AssertCLISuccess(t, result)

	// Verify state file was created
	stateFile := filepath.Join(stateDir, "test-repo.state")
	testutil.AssertFileExists(t, stateFile)

	// Verify state file has correct structure
	stateData, err := os.ReadFile(stateFile)
	if err != nil {
		t.Fatalf("Failed to read state file: %v", err)
	}

	var state map[string]interface{}
	if err := json.Unmarshal(stateData, &state); err != nil {
		t.Fatalf("Failed to parse state file: %v", err)
	}

	// Check state contains expected fields
	if _, ok := state["lastFetchTime"]; !ok {
		t.Error("State file missing lastFetchTime")
	}
	if _, ok := state["checkpoint"]; !ok {
		t.Error("State file missing checkpoint")
	}
	if _, ok := state["checksum"]; !ok {
		t.Error("State file missing checksum")
	}

	// Verify metadata file was created
	metadataFile := filepath.Join(testDir, "test-repo.metadata.json")
	testutil.AssertFileExists(t, metadataFile)

	// Verify metadata contains accurate statistics
	metadataData, err := os.ReadFile(metadataFile)
	if err != nil {
		t.Fatalf("Failed to read metadata file: %v", err)
	}

	var metadata map[string]interface{}
	if err := json.Unmarshal(metadataData, &metadata); err != nil {
		t.Fatalf("Failed to parse metadata file: %v", err)
	}

	// Check metadata statistics
	if prCount, ok := metadata["pullRequestCount"].(float64); !ok || int(prCount) != totalPRs {
		t.Errorf("Expected pullRequestCount=%d in metadata, got %v", totalPRs, metadata["pullRequestCount"])
	}

	// Verify file permissions (should be readable)
	stateInfo, _ := os.Stat(stateFile)
	if stateInfo.Mode()&0400 == 0 {
		t.Error("State file should be readable")
	}

	metadataInfo, _ := os.Stat(metadataFile)
	if metadataInfo.Mode()&0400 == 0 {
		t.Error("Metadata file should be readable")
	}
}

// TestIncrementalFetchFunctions tests incremental fetch with state
func TestIncrementalFetchFunctions(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") != "true" {
		t.Skip("Skipping integration test. Set INTEGRATION_TEST=true to run.")
	}

	// Track which PRs we've served
	servedPRs := make(map[int]bool)
	var requestCount int
	var incrementalRequestCount int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		var req struct {
			Query     string `json:"query"`
			Variables struct {
				Query string `json:"query"` // For search API
				First int    `json:"first"`
				After string `json:"after"`
			} `json:"variables"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		// Check if this is an incremental request (using search query)
		isIncremental := strings.Contains(req.Query, "is:pr") && strings.Contains(req.Variables.Query, "created:>")

		if isIncremental {
			incrementalRequestCount++
			// Return only new PRs (26-30)
			prs := make([]map[string]interface{}, 0)
			for i := 26; i <= 30; i++ {
				prs = append(prs, map[string]interface{}{
					"number":    i,
					"title":     fmt.Sprintf("PR %d", i),
					"state":     "OPEN",
					"createdAt": fmt.Sprintf("2024-12-%02dT00:00:00Z", i-20), // Newer dates
					"updatedAt": fmt.Sprintf("2024-12-%02dT12:00:00Z", i-20),
					"author": map[string]interface{}{
						"login": fmt.Sprintf("user%d", i),
					},
				})
			}

			// Use search API response format
			response := map[string]interface{}{
				"data": map[string]interface{}{
					"search": map[string]interface{}{
						"nodes": prs,
						"pageInfo": map[string]interface{}{
							"hasNextPage": false,
							"endCursor":   nil,
						},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		} else {
			// Initial full fetch (PRs 1-25)
			page := 0
			if req.Variables.After != "" {
				fmt.Sscanf(req.Variables.After, "cursor%d", &page)
			}

			startNum := page*10 + 1
			endNum := startNum + 9
			if endNum > 25 {
				endNum = 25
			}

			hasMore := endNum < 25
			response := testutil.GeneratePRResponse(startNum, endNum, hasMore)

			// Track what we served
			for i := startNum; i <= endNum; i++ {
				servedPRs[i] = true
			}

			// Add cursor to response
			if hasMore {
				if repo, ok := response["data"].(map[string]interface{})["repository"].(map[string]interface{}); ok {
					if prs, ok := repo["pullRequests"].(map[string]interface{}); ok {
						if pageInfo, ok := prs["pageInfo"].(map[string]interface{}); ok {
							pageInfo["endCursor"] = fmt.Sprintf("cursor%d", page+1)
						}
					}
				}
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		}
	}))
	defer server.Close()

	testDir := testutil.CreateTempDir(t, "incremental-test")
	outputFile1 := filepath.Join(testDir, "output1.ndjson")
	outputFile2 := filepath.Join(testDir, "output2.ndjson")
	stateDir := testutil.CreateStateDir(t, testDir)

	// First run: Fetch all PRs (1-25)
	result1 := testutil.RunCLI(t, []string{"fetch", "test/repo", "--output", outputFile1, "--all"}, map[string]string{
		"GITHUB_TOKEN":            "test-token",
		"GITHUB_GRAPHQL_ENDPOINT": server.URL + "/graphql",
		"SIRSEER_STATE_DIR":       stateDir,
	})

	testutil.AssertCLISuccess(t, result1)

	// Verify state file exists
	stateFile := filepath.Join(stateDir, "test-repo.state")
	testutil.AssertFileExists(t, stateFile)

	// Count PRs in first output
	count1 := countLines(t, outputFile1)
	if count1 != 25 {
		t.Errorf("Expected 25 PRs in first fetch, got %d", count1)
	}

	// Verify state contains correct last PR info
	stateData, err := os.ReadFile(stateFile)
	if err != nil {
		t.Fatalf("Failed to read state file: %v", err)
	}

	var state map[string]interface{}
	if err := json.Unmarshal(stateData, &state); err != nil {
		t.Fatalf("Failed to parse state file: %v", err)
	}

	// Check last PR number
	if lastPRNumber, ok := state["last_pr_number"].(float64); !ok || int(lastPRNumber) != 25 {
		t.Errorf("Expected last_pr_number=25 in state, got %v", state["last_pr_number"])
	}

	// Second run: Incremental fetch (should get PRs 26-30)
	result2 := testutil.RunCLI(t, []string{"fetch", "test/repo", "--output", outputFile2, "--incremental"}, map[string]string{
		"GITHUB_TOKEN":            "test-token",
		"GITHUB_GRAPHQL_ENDPOINT": server.URL + "/graphql",
		"SIRSEER_STATE_DIR":       stateDir,
	})

	testutil.AssertCLISuccess(t, result2)

	// Count PRs in second output
	count2 := countLines(t, outputFile2)
	if count2 != 5 {
		t.Errorf("Expected 5 new PRs in incremental fetch, got %d", count2)
	}

	// Verify deduplication - no PR numbers from first fetch should appear in second
	file2, err := os.Open(outputFile2)
	if err != nil {
		t.Fatalf("Failed to open second output file: %v", err)
	}
	defer file2.Close()

	scanner := bufio.NewScanner(file2)
	for scanner.Scan() {
		var pr map[string]interface{}
		if err := json.Unmarshal(scanner.Bytes(), &pr); err != nil {
			continue
		}
		number := int(pr["number"].(float64))
		if number <= 25 {
			t.Errorf("Found duplicate PR #%d in incremental fetch", number)
		}
	}

	// Verify state was updated with new last PR
	newStateData, err := os.ReadFile(stateFile)
	if err != nil {
		t.Fatalf("Failed to read updated state file: %v", err)
	}

	var newState map[string]interface{}
	if err := json.Unmarshal(newStateData, &newState); err != nil {
		t.Fatalf("Failed to parse updated state file: %v", err)
	}

	// Check updated last PR number
	if lastPRNumber, ok := newState["last_pr_number"].(float64); !ok || int(lastPRNumber) != 30 {
		t.Errorf("Expected last_pr_number=30 in updated state, got %v", newState["last_pr_number"])
	}

	// Verify incremental request was made
	if incrementalRequestCount == 0 {
		t.Error("Expected at least one incremental request using search API")
	}
}

// Helper function to count lines in a file
func countLines(t *testing.T, filename string) int {
	t.Helper()

	file, err := os.Open(filename)
	if err != nil {
		t.Fatalf("Failed to open file %s: %v", filename, err)
		return 0
	}
	defer file.Close()

	count := 0
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		count++
	}

	if err := scanner.Err(); err != nil {
		t.Fatalf("Error reading file %s: %v", filename, err)
	}

	return count
}

// TestIncrementalWithoutPriorState tests incremental flag without existing state
func TestIncrementalWithoutPriorState(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") != "true" {
		t.Skip("Skipping integration test. Set INTEGRATION_TEST=true to run.")
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Should never be called - expect error before request
		t.Error("Server should not be called when incremental fetch has no prior state")
		w.WriteHeader(500)
	}))
	defer server.Close()

	testDir := testutil.CreateTempDir(t, "incremental-no-state-test")
	outputFile := filepath.Join(testDir, "output.ndjson")
	stateDir := testutil.CreateStateDir(t, testDir)

	// Run incremental without prior state
	result := testutil.RunCLI(t, []string{"fetch", "test/repo", "--output", outputFile, "--incremental"}, map[string]string{
		"GITHUB_TOKEN":            "test-token",
		"GITHUB_GRAPHQL_ENDPOINT": server.URL + "/graphql",
		"SIRSEER_STATE_DIR":       stateDir,
	})

	// Should fail with appropriate error
	testutil.AssertCLIError(t, result, "no previous state found")
}

// TestBatchSizeReduction tests that batch size is properly reduced on complexity errors
func TestBatchSizeReduction(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") != "true" {
		t.Skip("Skipping integration test. Set INTEGRATION_TEST=true to run.")
	}

	requestBatchSizes := []int{}
	complexityErrorCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Parse the request to get batch size
		var req struct {
			Variables struct {
				First int `json:"first"`
			} `json:"variables"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		requestBatchSizes = append(requestBatchSizes, req.Variables.First)

		// Return complexity error for first few requests with large batch sizes
		if req.Variables.First > 25 && complexityErrorCount < 2 {
			complexityErrorCount++
			w.WriteHeader(200)
			response := map[string]interface{}{
				"errors": []map[string]interface{}{
					{
						"message": fmt.Sprintf("Query has complexity of %d, which exceeds max complexity of 1000", 1000+req.Variables.First),
						"extensions": map[string]interface{}{
							"code": "MAX_COMPLEXITY_EXCEEDED",
						},
					},
				},
			}
			json.NewEncoder(w).Encode(response)
			return
		}

		// Return success
		response := testutil.GeneratePRResponse(1, 5, false)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	testDir := testutil.CreateTempDir(t, "batch-size-test")
	outputFile := filepath.Join(testDir, "output.ndjson")

	// Run with explicit batch size that will trigger complexity
	result := testutil.RunCLI(t, []string{"fetch", "test/repo", "--output", outputFile, "--all", "--batch-size", "50"}, map[string]string{
		"GITHUB_TOKEN":            "test-token",
		"GITHUB_GRAPHQL_ENDPOINT": server.URL + "/graphql",
	})

	testutil.AssertCLISuccess(t, result)

	// Verify batch sizes were reduced
	if len(requestBatchSizes) < 3 {
		t.Fatalf("Expected at least 3 requests, got %d", len(requestBatchSizes))
	}

	// First request should be original size
	if requestBatchSizes[0] != 50 {
		t.Errorf("First request should have batch size 50, got %d", requestBatchSizes[0])
	}

	// Second request should be reduced
	if requestBatchSizes[1] >= requestBatchSizes[0] {
		t.Errorf("Second request should have reduced batch size, got %d", requestBatchSizes[1])
	}

	// Third request should be further reduced or same
	if requestBatchSizes[2] > requestBatchSizes[1] {
		t.Errorf("Third request should not increase batch size, got %d", requestBatchSizes[2])
	}
}

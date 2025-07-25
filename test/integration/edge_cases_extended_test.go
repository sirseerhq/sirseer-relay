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
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/sirseerhq/sirseer-relay/test/testutil"
)

// TestUnicodePRData tests handling of Unicode and special characters
func TestUnicodePRData(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") != "true" {
		t.Skip("Skipping integration test. Set INTEGRATION_TEST=true to run.")
	}

	// Create PRs with various Unicode content
	prs := []map[string]interface{}{
		testutil.NewPullRequestBuilder(1).
			WithTitle("🚀 Rocket ship emoji PR").
			WithBody("This PR contains emojis 😀 and special chars: ñ, ü, ß").
			WithAuthor("user-名前").
			Build(),
		testutil.NewPullRequestBuilder(2).
			WithTitle("中文标题 - Chinese title").
			WithBody("Содержание на русском языке\nمحتوى عربي\n日本語の内容").
			Build(),
		testutil.NewPullRequestBuilder(3).
			WithTitle("Mixed: English/한국어/Español").
			WithBody("Special chars: \n\t\r quotes: \"'` backslash: \\ null: \x00").
			Build(),
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := testutil.NewGraphQLResponseBuilder().
			WithPullRequests(prs...).
			WithPagination(false, "").
			Build()

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	testDir := testutil.CreateTempDir(t, "unicode-test")
	outputFile := filepath.Join(testDir, "output.ndjson")

	result := testutil.RunWithMockServer(t, &testutil.MockServer{Server: server}, "test/unicode-repo",
		"--output", outputFile,
		"--all",
	)

	testutil.AssertCLISuccess(t, result)
	testutil.AssertNDJSONOutput(t, outputFile, 3)

	// Verify Unicode content is preserved
	data, err := os.ReadFile(outputFile)
	testutil.AssertNoError(t, err)

	content := string(data)
	testutil.AssertContainsString(t, content, "🚀")
	testutil.AssertContainsString(t, content, "中文标题")
	testutil.AssertContainsString(t, content, "한국어")
}

// TestVeryLargePR tests handling of PRs with thousands of files
func TestVeryLargePR(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") != "true" {
		t.Skip("Skipping integration test. Set INTEGRATION_TEST=true to run.")
	}

	// Create a PR with 5000 changed files
	largePR := testutil.NewPullRequestBuilder(1).
		WithTitle("Massive refactoring PR").
		WithChanges(50000, 30000, 5000). // 50k additions, 30k deletions, 5k files
		WithComments(250).
		WithCommits(150).
		Build()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow response for large PR
		time.Sleep(100 * time.Millisecond)

		response := testutil.NewGraphQLResponseBuilder().
			WithPullRequests(largePR).
			WithPagination(false, "").
			Build()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	testDir := testutil.CreateTempDir(t, "large-pr-test")
	outputFile := filepath.Join(testDir, "output.ndjson")

	result := testutil.RunWithMockServer(t, &testutil.MockServer{Server: server}, "test/large-repo",
		"--output", outputFile,
		"--all",
	)

	testutil.AssertCLISuccess(t, result)
	testutil.AssertNDJSONOutput(t, outputFile, 1)

	// Verify large numbers are preserved
	data, err := os.ReadFile(outputFile)
	testutil.AssertNoError(t, err)

	var pr map[string]interface{}
	testutil.AssertNoError(t, json.Unmarshal(data, &pr))
	testutil.AssertEqual(t, pr["changedFiles"], float64(5000))
	testutil.AssertEqual(t, pr["additions"], float64(50000))
}

// TestConcurrentRequests tests handling of concurrent API requests
func TestConcurrentRequests(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") != "true" {
		t.Skip("Skipping integration test. Set INTEGRATION_TEST=true to run.")
	}

	var (
		mu            sync.Mutex
		requestCount  int
		maxConcurrent int
		currentActive int
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		currentActive++
		if currentActive > maxConcurrent {
			maxConcurrent = currentActive
		}
		mu.Unlock()

		// Simulate processing time
		time.Sleep(50 * time.Millisecond)

		mu.Lock()
		currentActive--
		mu.Unlock()

		// Return different pages based on cursor
		var cursor string
		var reqBody map[string]interface{}
		json.NewDecoder(r.Body).Decode(&reqBody)
		if vars, ok := reqBody["variables"].(map[string]interface{}); ok {
			if c, ok := vars["after"].(string); ok {
				cursor = c
			}
		}

		page := 0
		fmt.Sscanf(cursor, "cursor_%d", &page)

		startNum := page*10 + 1
		endNum := startNum + 9
		hasMore := endNum < 30

		response := testutil.GeneratePRResponse(startNum, endNum, hasMore)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	testDir := testutil.CreateTempDir(t, "concurrent-test")
	outputFile := filepath.Join(testDir, "output.ndjson")

	// This should trigger multiple concurrent requests due to pagination
	result := testutil.RunWithMockServer(t, &testutil.MockServer{Server: server}, "test/concurrent-repo",
		"--output", outputFile,
		"--all",
	)

	testutil.AssertCLISuccess(t, result)
	testutil.AssertNDJSONOutput(t, outputFile, 30)

	// Verify we made multiple requests
	if requestCount < 3 {
		t.Errorf("Expected at least 3 requests for pagination, got %d", requestCount)
	}

	// Note: sirseer-relay processes pages sequentially, so maxConcurrent should be 1
	if maxConcurrent != 1 {
		t.Logf("Max concurrent requests: %d (sequential processing expected)", maxConcurrent)
	}
}

// TestMalformedGraphQLResponse tests handling of invalid API responses
func TestMalformedGraphQLResponse(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") != "true" {
		t.Skip("Skipping integration test. Set INTEGRATION_TEST=true to run.")
	}

	tests := []struct {
		name          string
		response      string
		responseCode  int
		expectedError string
	}{
		{
			name:          "invalidJSON",
			response:      `{"data": {"repository": {"pullRequests": {"nodes": [`,
			responseCode:  http.StatusOK,
			expectedError: "parsing",
		},
		{
			name:          "missingDataField",
			response:      `{"errors": []}`,
			responseCode:  http.StatusOK,
			expectedError: "error",
		},
		{
			name:          "nullRepository",
			response:      `{"data": {"repository": null}}`,
			responseCode:  http.StatusOK,
			expectedError: "repository",
		},
		{
			name:          "htmlErrorPage",
			response:      `<html><body>502 Bad Gateway</body></html>`,
			responseCode:  http.StatusBadGateway,
			expectedError: "502",
		},
		{
			name:          "emptyResponse",
			response:      ``,
			responseCode:  http.StatusOK,
			expectedError: "empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.responseCode)
				w.Write([]byte(tt.response))
			}))
			defer server.Close()

			testDir := testutil.CreateTempDir(t, "malformed-"+tt.name)
			outputFile := filepath.Join(testDir, "output.ndjson")

			result := testutil.RunWithMockServer(t, &testutil.MockServer{Server: server}, "test/malformed-repo",
				"--output", outputFile,
				"--all",
			)

			testutil.AssertCLIError(t, result, tt.expectedError)
		})
	}
}

// TestFileSystemErrors tests handling of file system issues
func TestFileSystemErrors(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") != "true" {
		t.Skip("Skipping integration test. Set INTEGRATION_TEST=true to run.")
	}

	server := testutil.NewMockServerBuilder(t).
		WithPullRequests(10, 10).
		Build()
	defer server.Server.Close()

	tests := []struct {
		name          string
		setupFS       func(dir string) string
		expectedError string
	}{
		{
			name: "readOnlyOutputDirectory",
			setupFS: func(dir string) string {
				outputDir := filepath.Join(dir, "readonly")
				os.MkdirAll(outputDir, 0755)
				os.Chmod(outputDir, 0555) // Read-only
				return filepath.Join(outputDir, "output.ndjson")
			},
			expectedError: "permission denied",
		},
		{
			name: "outputPathIsDirectory",
			setupFS: func(dir string) string {
				outputDir := filepath.Join(dir, "output.ndjson")
				os.MkdirAll(outputDir, 0755) // Create as directory
				return outputDir
			},
			expectedError: "is a directory",
		},
		{
			name: "nonExistentParentDirectory",
			setupFS: func(dir string) string {
				return filepath.Join(dir, "does", "not", "exist", "output.ndjson")
			},
			expectedError: "no such file or directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDir := testutil.CreateTempDir(t, "fs-error-"+tt.name)
			outputFile := tt.setupFS(testDir)

			result := testutil.RunWithMockServer(t, server, "test/repo",
				"--output", outputFile,
				"--all",
			)

			testutil.AssertCLIError(t, result, tt.expectedError)
		})
	}
}

// TestPRsWithNullFields tests handling of PRs with null/missing fields
func TestPRsWithNullFields(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") != "true" {
		t.Skip("Skipping integration test. Set INTEGRATION_TEST=true to run.")
	}

	// Create PRs with various null fields
	prs := []map[string]interface{}{
		{
			"number":    1,
			"title":     "PR with null author",
			"state":     "OPEN",
			"createdAt": time.Now().Format(time.RFC3339),
			"updatedAt": time.Now().Format(time.RFC3339),
			"url":       "https://github.com/test/repo/pull/1",
			"author":    nil, // Null author (deleted user)
			"body":      nil, // No description
			"baseRef":   nil, // Branch deleted
			"headRef":   nil, // Branch deleted
		},
		{
			"number":       2,
			"title":        "PR with minimal fields",
			"state":        "OPEN",
			"createdAt":    time.Now().Format(time.RFC3339),
			"updatedAt":    time.Now().Format(time.RFC3339),
			"url":          "https://github.com/test/repo/pull/2",
			"author":       map[string]interface{}{"login": "ghost"},
			"additions":    nil,
			"deletions":    nil,
			"changedFiles": nil,
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := testutil.NewGraphQLResponseBuilder().
			WithPullRequests(prs...).
			WithPagination(false, "").
			Build()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	testDir := testutil.CreateTempDir(t, "null-fields-test")
	outputFile := filepath.Join(testDir, "output.ndjson")

	result := testutil.RunWithMockServer(t, &testutil.MockServer{Server: server}, "test/null-repo",
		"--output", outputFile,
		"--all",
	)

	testutil.AssertCLISuccess(t, result)

	// Verify we can read the output despite null fields
	file, err := os.Open(outputFile)
	testutil.AssertNoError(t, err)
	defer file.Close()

	decoder := json.NewDecoder(file)
	prCount := 0
	for decoder.More() {
		var pr map[string]interface{}
		err := decoder.Decode(&pr)
		testutil.AssertNoError(t, err)
		prCount++

		// Verify required fields exist
		if _, ok := pr["number"]; !ok {
			t.Error("Missing required field: number")
		}
		if _, ok := pr["title"]; !ok {
			t.Error("Missing required field: title")
		}
	}

	testutil.AssertEqual(t, prCount, 2)
}

// TestRapidPagination tests handling of many small pages
func TestRapidPagination(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") != "true" {
		t.Skip("Skipping integration test. Set INTEGRATION_TEST=true to run.")
	}

	pageCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pageCount++

		// Return 2 PRs per page, 20 pages total
		startNum := (pageCount-1)*2 + 1
		endNum := startNum + 1
		hasMore := pageCount < 20

		response := testutil.GeneratePRResponse(startNum, endNum, hasMore)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	testDir := testutil.CreateTempDir(t, "rapid-pagination-test")
	outputFile := filepath.Join(testDir, "output.ndjson")

	start := time.Now()
	result := testutil.RunWithMockServer(t, &testutil.MockServer{Server: server}, "test/paginated-repo",
		"--output", outputFile,
		"--all",
	)
	elapsed := time.Since(start)

	testutil.AssertCLISuccess(t, result)
	testutil.AssertNDJSONOutput(t, outputFile, 40)

	if pageCount != 20 {
		t.Errorf("Expected 20 page requests, got %d", pageCount)
	}

	// Should complete reasonably quickly despite many pages
	if elapsed > 30*time.Second {
		t.Errorf("Pagination took too long: %v", elapsed)
	}
}

// TestInterruptedFetch tests recovery from interrupted fetches
func TestInterruptedFetch(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") != "true" {
		t.Skip("Skipping integration test. Set INTEGRATION_TEST=true to run.")
	}

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		// Fail on the 3rd request to simulate interruption
		if requestCount == 3 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Internal Server Error"))
			return
		}

		// Otherwise return normal pages
		startNum := (requestCount-1)*10 + 1
		endNum := startNum + 9
		hasMore = endNum < 50

		response := testutil.GeneratePRResponse(startNum, endNum, hasMore)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	testDir := testutil.CreateTempDir(t, "interrupted-test")
	outputFile := filepath.Join(testDir, "output.ndjson")

	// First attempt - will fail
	result := testutil.RunWithMockServer(t, &testutil.MockServer{Server: server}, "test/interrupted-repo",
		"--output", outputFile,
		"--all",
	)

	// Should fail on the 3rd page
	testutil.AssertCLIError(t, result, "500")

	// Verify partial output was saved
	testutil.AssertFileExists(t, outputFile)

	// Check how many PRs were saved before failure
	file, err := os.Open(outputFile)
	testutil.AssertNoError(t, err)

	decoder := json.NewDecoder(file)
	savedPRs := 0
	for decoder.More() {
		var pr map[string]interface{}
		decoder.Decode(&pr)
		savedPRs++
	}
	file.Close()

	// Should have saved 2 pages (20 PRs) before failure
	if savedPRs != 20 {
		t.Errorf("Expected 20 PRs saved before failure, got %d", savedPRs)
	}
}

var hasMore bool // Add this variable declaration

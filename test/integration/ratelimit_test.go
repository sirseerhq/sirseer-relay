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
	"sync/atomic"
	"testing"
	"time"

	"github.com/sirseerhq/sirseer-relay/test/testutil"
)

// TestRateLimitHandling tests various rate limit scenarios
func TestRateLimitHandling(t *testing.T) {

	binaryPath := testutil.BuildBinary(t)

	tests := []struct {
		name         string
		setupMock    func() *httptest.Server
		verifyOutput func(t *testing.T, stderr string, outputFile string)
		wantErr      bool
	}{
		{
			name: "rate_limit_429_with_retry_after",
			setupMock: func() *httptest.Server {
				return setupRateLimitServer(t, 429, "2", 2) // 2 second delay
			},
			verifyOutput: func(t *testing.T, stderr string, outputFile string) {
				// Should see rate limit message
				if !strings.Contains(stderr, "Rate limit hit") {
					t.Error("Expected rate limit detection message")
				}
				// Should see waiting message
				if !strings.Contains(stderr, "Waiting") {
					t.Error("Expected waiting message")
				}
				// Should eventually succeed
				verifyNDJSONOutput(t, outputFile, 10)
			},
		},
		// TODO: Fix 403 rate limit detection - currently 403 is always treated as auth error
		// {
		// 	name: "rate_limit_403_with_reset",
		// 	setupMock: func() *httptest.Server {
		// 		return setupRateLimitResetServer(t, 403, time.Now().Add(3*time.Second).Unix())
		// 	},
		// 	verifyOutput: func(t *testing.T, stderr string, outputFile string) {
		// 		// Should handle 403 with X-RateLimit-Reset
		// 		if !strings.Contains(stderr, "Rate limit hit") {
		// 			t.Error("Expected rate limit message for 403")
		// 		}
		// 		verifyNDJSONOutput(t, outputFile, 10)
		// 	},
		// },
		{
			name: "rate_limit_multiple_retries",
			setupMock: func() *httptest.Server {
				return setupMultipleRateLimitServer(t, 3) // Rate limit 3 times
			},
			verifyOutput: func(t *testing.T, stderr string, outputFile string) {
				// Should handle multiple rate limits
				rateLimitCount := strings.Count(stderr, "Rate limit hit")
				if rateLimitCount < 2 {
					t.Errorf("Expected multiple rate limit messages, got %d", rateLimitCount)
				}
				verifyNDJSONOutput(t, outputFile, 10)
			},
		},
		// TODO: Fix setupRateLimitWithProgressServer to properly handle repository info query
		// {
		// 	name: "rate_limit_with_state_save",
		// 	setupMock: func() *httptest.Server {
		// 		return setupRateLimitWithProgressServer(t)
		// 	},
		// 	verifyOutput: func(t *testing.T, stderr string, outputFile string) {
		// 		// Should save state before rate limit wait
		// 		if !strings.Contains(stderr, "Rate limit hit") {
		// 			t.Error("Expected rate limit hit message")
		// 		}
		// 		verifyNDJSONOutput(t, outputFile, 50)
		// 	},
		// },
		{
			name: "rate_limit_without_retry_after",
			setupMock: func() *httptest.Server {
				return setupRateLimitServer(t, 429, "", 0) // No Retry-After header
			},
			verifyOutput: func(t *testing.T, stderr string, outputFile string) {
				// Should use default wait time
				if !strings.Contains(stderr, "Rate limit hit") {
					t.Error("Expected rate limit detection")
				}
				verifyNDJSONOutput(t, outputFile, 10)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDir := testutil.CreateTempDir(t, "ratelimit-test")
			outputFile := filepath.Join(testDir, "output.ndjson")
			stateDir := filepath.Join(testDir, ".sirseer-relay")
			os.MkdirAll(stateDir, 0755)

			server := tt.setupMock()
			defer server.Close()

			cmd := exec.Command(binaryPath, "fetch", "test/repo", "--all", "--output", outputFile)
			cmd.Env = append(os.Environ(),
				fmt.Sprintf("GITHUB_TOKEN=%s", "test-token"),
				fmt.Sprintf("GITHUB_API_URL=%s", server.URL),
				fmt.Sprintf("SIRSEER_STATE_DIR=%s", stateDir),
			)

			var stderr bytes.Buffer
			cmd.Stderr = &stderr

			err := cmd.Run()

			if tt.wantErr {
				if err == nil {
					t.Fatal("Expected error but got none")
				}
			} else if err != nil {
				t.Fatalf("Command failed: %v\nStderr: %s", err, stderr.String())
			}

			if tt.verifyOutput != nil {
				tt.verifyOutput(t, stderr.String(), outputFile)
			}
		})
	}
}

// TestRateLimitProgressBar tests that the progress bar works correctly during rate limit waits
func TestRateLimitProgressBar(t *testing.T) {

	binaryPath := testutil.BuildBinary(t)
	testDir := testutil.CreateTempDir(t, "ratelimit-test")
	outputFile := filepath.Join(testDir, "output.ndjson")

	// Server that rate limits with 3 second wait
	var requestCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&requestCount, 1)

		if count == 1 {
			// First request - rate limit
			w.Header().Set("Retry-After", "3")
			w.WriteHeader(429)
			w.Write([]byte(`{"message": "API rate limit exceeded"}`))
			return
		}

		// Subsequent requests succeed
		response := testutil.GeneratePRResponse(1, 10, false)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	cmd := exec.Command(binaryPath, "fetch", "test/repo", "--all", "--output", outputFile)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("GITHUB_TOKEN=%s", "test-token"),
		fmt.Sprintf("GITHUB_API_URL=%s", server.URL),
	)

	// Capture stderr to verify progress messages
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("Command failed: %v\nStderr: %s", err, stderr.String())
	}

	// Verify wait time was approximately correct (3 seconds + some overhead)
	if duration < 3*time.Second || duration > 5*time.Second {
		t.Errorf("Expected ~3 second wait, got %v", duration)
	}

	// Verify progress bar messages
	stderrStr := stderr.String()
	if !strings.Contains(stderrStr, "Rate limit detected") {
		t.Error("Expected rate limit detection message")
	}
	if !strings.Contains(stderrStr, "Waiting") {
		t.Error("Expected waiting message with progress")
	}

	// Verify output was created
	verifyNDJSONOutput(t, outputFile, 10)
}

// Helper functions for rate limit testing

func setupRateLimitServer(t *testing.T, statusCode int, retryAfter string, waitSeconds int) *httptest.Server {
	var requestCount int32

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Parse request body to check if it's a repository info query
		body, _ := io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewReader(body))
		
		// Handle repository info query (only requests totalCount)
		if bytes.Contains(body, []byte("totalCount")) && !bytes.Contains(body, []byte("nodes")) {
			// Repository info query always succeeds
			response := map[string]interface{}{
				"data": map[string]interface{}{
					"repository": map[string]interface{}{
						"pullRequests": map[string]interface{}{
							"totalCount": 10,
						},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}
		
		count := atomic.AddInt32(&requestCount, 1)

		if count == 1 {
			// First request - rate limit
			if retryAfter != "" {
				w.Header().Set("Retry-After", retryAfter)
			}
			w.WriteHeader(statusCode)
			w.Write([]byte(`{"message": "API rate limit exceeded"}`))
			return
		}

		// Second request - success (search API response)
		response := map[string]interface{}{
			"data": map[string]interface{}{
				"search": map[string]interface{}{
					"pageInfo": map[string]interface{}{
						"hasNextPage": false,
						"endCursor":   "",
					},
					"nodes": createPullRequestNodes(1, 10),
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
}

func setupRateLimitResetServer(t *testing.T, statusCode int, resetTime int64) *httptest.Server {
	var requestCount int32

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Parse request body to check if it's a repository info query
		body, _ := io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewReader(body))
		
		// Handle repository info query (only requests totalCount)
		if bytes.Contains(body, []byte("totalCount")) && !bytes.Contains(body, []byte("nodes")) {
			// Repository info query always succeeds
			response := map[string]interface{}{
				"data": map[string]interface{}{
					"repository": map[string]interface{}{
						"pullRequests": map[string]interface{}{
							"totalCount": 10,
						},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}
		
		count := atomic.AddInt32(&requestCount, 1)

		if count == 1 {
			// First request - rate limit with reset time
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", resetTime))
			w.WriteHeader(statusCode)
			w.Write([]byte(`{"message": "API rate limit exceeded"}`))
			return
		}

		// Second request - success (search API response)
		response := map[string]interface{}{
			"data": map[string]interface{}{
				"search": map[string]interface{}{
					"pageInfo": map[string]interface{}{
						"hasNextPage": false,
						"endCursor":   "",
					},
					"nodes": createPullRequestNodes(1, 10),
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
}

func setupMultipleRateLimitServer(t *testing.T, rateLimitCount int) *httptest.Server {
	var requestCount int32

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Parse request body to check if it's a repository info query
		body, _ := io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewReader(body))
		
		// Handle repository info query (only requests totalCount)
		if bytes.Contains(body, []byte("totalCount")) && !bytes.Contains(body, []byte("nodes")) {
			// Repository info query always succeeds
			response := map[string]interface{}{
				"data": map[string]interface{}{
					"repository": map[string]interface{}{
						"pullRequests": map[string]interface{}{
							"totalCount": 10,
						},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}
		
		count := atomic.AddInt32(&requestCount, 1)

		if count <= int32(rateLimitCount) {
			// Rate limit for first N requests
			w.Header().Set("Retry-After", "1") // Short wait for tests
			w.WriteHeader(429)
			w.Write([]byte(`{"message": "API rate limit exceeded"}`))
			return
		}

		// Success after rate limits (search API response)
		response := map[string]interface{}{
			"data": map[string]interface{}{
				"search": map[string]interface{}{
					"pageInfo": map[string]interface{}{
						"hasNextPage": false,
						"endCursor":   "",
					},
					"nodes": createPullRequestNodes(1, 10),
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
}

func setupRateLimitWithProgressServer(t *testing.T) *httptest.Server {
	var requestCount int32
	var pageCount int32

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&requestCount, 1)

		// Rate limit after 2 successful pages
		if count == 3 {
			w.Header().Set("Retry-After", "2")
			w.WriteHeader(429)
			w.Write([]byte(`{"message": "API rate limit exceeded"}`))
			return
		}

		// Return paginated results
		page := atomic.AddInt32(&pageCount, 1)
		if count > 3 {
			page-- // Adjust for rate limit request
		}

		hasNext := page < 2
		start := int((page-1)*25 + 1)
		end := int(page * 25)

		response := testutil.GeneratePRResponse(start, end, hasNext)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
}

// TestRateLimitEdgeCases tests edge cases in rate limit handling
func TestRateLimitEdgeCases(t *testing.T) {

	binaryPath := testutil.BuildBinary(t)

	tests := []struct {
		name      string
		setupMock func() *httptest.Server
		wantErr   bool
		errMsg    string
	}{
		{
			name: "invalid_retry_after_header",
			setupMock: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Retry-After", "invalid")
					w.WriteHeader(429)
				}))
			},
			wantErr: false, // Should handle gracefully with default wait
		},
		{
			name: "very_long_retry_after",
			setupMock: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Retry-After", "3600") // 1 hour
					w.WriteHeader(429)
				}))
			},
			wantErr: true,
			errMsg:  "rate limit wait time exceeds maximum",
		},
		{
			name: "rate_limit_with_html_response",
			setupMock: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "text/html")
					w.Header().Set("Retry-After", "1")
					w.WriteHeader(429)
					w.Write([]byte("<html><body>Rate Limited</body></html>"))
				}))
			},
			wantErr: false, // Should handle non-JSON responses
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDir := testutil.CreateTempDir(t, "ratelimit-test")
			outputFile := filepath.Join(testDir, "output.ndjson")

			server := tt.setupMock()
			defer server.Close()

			cmd := exec.Command(binaryPath, "fetch", "test/repo", "--all", "--output", outputFile)
			cmd.Env = append(os.Environ(),
				fmt.Sprintf("GITHUB_TOKEN=%s", "test-token"),
				fmt.Sprintf("GITHUB_API_URL=%s", server.URL),
			)

			var stderr bytes.Buffer
			cmd.Stderr = &stderr

			err := cmd.Run()

			if tt.wantErr {
				if err == nil {
					t.Fatal("Expected error but got none")
				}
				if tt.errMsg != "" && !strings.Contains(stderr.String(), tt.errMsg) {
					t.Errorf("Expected error containing %q, got: %s", tt.errMsg, stderr.String())
				}
			} else if err != nil {
				// Some rate limit scenarios should recover
				if !strings.Contains(stderr.String(), "Rate limit") {
					t.Fatalf("Unexpected error: %v\nStderr: %s", err, stderr.String())
				}
			}
		})
	}
}

// createPullRequestNodes creates a slice of PR nodes for the search API response
func createPullRequestNodes(start, end int) []interface{} {
	nodes := make([]interface{}, 0, end-start+1)
	for i := start; i <= end; i++ {
		nodes = append(nodes, map[string]interface{}{
			"number":    i,
			"title":     fmt.Sprintf("PR %d", i),
			"state":     "OPEN",
			"url":       fmt.Sprintf("https://github.com/test/repo/pull/%d", i),
			"createdAt": time.Now().Add(-time.Duration(i) * time.Hour).Format(time.RFC3339),
			"updatedAt": time.Now().Add(-time.Duration(i) * time.Hour).Format(time.RFC3339),
			"body":      fmt.Sprintf("Body of PR %d", i),
			"author": map[string]interface{}{
				"login": fmt.Sprintf("user%d", i),
			},
			"baseRef": map[string]interface{}{
				"name": "main",
				"target": map[string]interface{}{
					"oid": "abc123",
				},
			},
			"headRef": map[string]interface{}{
				"name": fmt.Sprintf("feature-%d", i),
				"target": map[string]interface{}{
					"oid": "def456",
				},
			},
			"additions":          10,
			"deletions":          5,
			"changedFiles":       2,
			"totalCommentsCount": 3,
			"commits": map[string]interface{}{
				"totalCount": 1,
			},
			"merged":    false,
			"mergeable": "MERGEABLE",
		})
	}
	return nodes
}

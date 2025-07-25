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
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sirseerhq/sirseer-relay/test/testutil"
)

// TestNetworkFailureRecovery tests various network failure scenarios and recovery
func TestNetworkFailureRecovery(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") != "true" {
		t.Skip("Skipping integration test. Set INTEGRATION_TEST=true to run.")
	}

	binaryPath := testutil.BuildBinary(t)

	tests := []struct {
		name         string
		setupMock    func() *httptest.Server
		verifyOutput func(t *testing.T, stderr string, outputFile string)
		wantErr      bool
		errContains  string
	}{
		{
			name: "transient_502_recovery",
			setupMock: func() *httptest.Server {
				return setupTransientErrorServer(t, http.StatusBadGateway, 2) // Fail twice, then succeed
			},
			verifyOutput: func(t *testing.T, stderr string, outputFile string) {
				// Should see retry messages
				if !strings.Contains(stderr, "Retrying") {
					t.Error("Expected retry messages")
				}
				// Should eventually succeed
				verifyNDJSONOutput(t, outputFile, 10)
			},
		},
		{
			name: "transient_503_recovery",
			setupMock: func() *httptest.Server {
				return setupTransientErrorServer(t, http.StatusServiceUnavailable, 3)
			},
			verifyOutput: func(t *testing.T, stderr string, outputFile string) {
				retryCount := strings.Count(stderr, "Retrying")
				if retryCount < 2 {
					t.Errorf("Expected at least 2 retries, got %d", retryCount)
				}
				verifyNDJSONOutput(t, outputFile, 10)
			},
		},
		{
			name: "connection_timeout_recovery",
			setupMock: func() *httptest.Server {
				return setupTimeoutServer(t, 2) // Timeout twice, then succeed
			},
			verifyOutput: func(t *testing.T, stderr string, outputFile string) {
				if !strings.Contains(stderr, "timeout") || !strings.Contains(stderr, "Retrying") {
					t.Error("Expected timeout and retry messages")
				}
				verifyNDJSONOutput(t, outputFile, 10)
			},
		},
		{
			name: "max_retries_exceeded",
			setupMock: func() *httptest.Server {
				return setupPermanentErrorServer(t, http.StatusBadGateway) // Always fail
			},
			wantErr:     true,
			errContains: "after 5 attempts",
		},
		{
			name: "connection_refused_recovery",
			setupMock: func() *httptest.Server {
				return setupConnectionRefusedServer(t, 2) // Refuse 2 times, then work
			},
			verifyOutput: func(t *testing.T, stderr string, outputFile string) {
				if !strings.Contains(stderr, "connection refused") || !strings.Contains(stderr, "Retrying") {
					t.Error("Expected connection refused and retry messages")
				}
				verifyNDJSONOutput(t, outputFile, 10)
			},
		},
		{
			name: "dns_resolution_failure",
			setupMock: func() *httptest.Server {
				// Return a server that will be immediately closed
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
				server.Close()
				return server
			},
			wantErr:     true,
			errContains: "connection refused",
		},
		{
			name: "partial_response_recovery",
			setupMock: func() *httptest.Server {
				return setupPartialResponseServer(t)
			},
			verifyOutput: func(t *testing.T, stderr string, outputFile string) {
				// Should handle partial responses and retry
				if !strings.Contains(stderr, "Retrying") {
					t.Error("Expected retry on partial response")
				}
				verifyNDJSONOutput(t, outputFile, 50)
			},
		},
		{
			name: "exponential_backoff_timing",
			setupMock: func() *httptest.Server {
				return setupTransientErrorServer(t, http.StatusBadGateway, 4) // Fail 4 times
			},
			verifyOutput: func(t *testing.T, stderr string, outputFile string) {
				// Verify exponential backoff is working
				retryCount := strings.Count(stderr, "Retrying")
				if retryCount < 3 {
					t.Errorf("Expected at least 3 retries, got %d", retryCount)
				}
				verifyNDJSONOutput(t, outputFile, 10)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDir := testutil.CreateTempDir(t, "network-test")
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

			start := time.Now()
			err := cmd.Run()
			duration := time.Since(start)

			if tt.wantErr {
				if err == nil {
					t.Fatal("Expected error but got none")
				}
				if tt.errContains != "" && !strings.Contains(stderr.String(), tt.errContains) {
					t.Errorf("Expected error containing %q, got: %s", tt.errContains, stderr.String())
				}
			} else {
				if err != nil {
					t.Fatalf("Command failed: %v\nStderr: %s", err, stderr.String())
				}
				// Verify retries added delay (exponential backoff)
				if tt.name == "exponential_backoff_timing" && duration < 5*time.Second {
					t.Errorf("Expected exponential backoff to add delay, but took only %v", duration)
				}
			}

			if tt.verifyOutput != nil {
				tt.verifyOutput(t, stderr.String(), outputFile)
			}
		})
	}
}

// TestNetworkFailureWithState tests network failures during stateful operations
func TestNetworkFailureWithState(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") != "true" {
		t.Skip("Skipping integration test. Set INTEGRATION_TEST=true to run.")
	}

	binaryPath := testutil.BuildBinary(t)
	testDir := testutil.CreateTempDir(t, "network-test")
	outputFile := filepath.Join(testDir, "output.ndjson")
	stateDir := filepath.Join(testDir, ".sirseer-relay")
	os.MkdirAll(stateDir, 0755)

	// Server that fails after returning some data
	var requestCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&requestCount, 1)

		// Parse request to check for cursor (pagination)
		var req struct {
			Variables struct {
				After string `json:"after"`
			} `json:"variables"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		if count <= 2 {
			// First two requests succeed (initial pages)
			page := count
			hasNext := true
			start := int((page-1)*25 + 1)
			end := int(page * 25)

			response := testutil.GeneratePRResponse(start, end, hasNext)
			response["data"].(map[string]interface{})["repository"].(map[string]interface{})["pullRequests"].(map[string]interface{})["pageInfo"].(map[string]interface{})["endCursor"] = fmt.Sprintf("cursor%d", page)

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		} else if count <= 4 {
			// Next two requests fail
			w.WriteHeader(http.StatusBadGateway)
			w.Write([]byte("Bad Gateway"))
		} else {
			// Resume from saved state
			if req.Variables.After == "cursor2" {
				// Correct cursor, continue
				response := testutil.GeneratePRResponse(51, 75, false)
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(response)
			} else {
				t.Errorf("Expected cursor2, got %s", req.Variables.After)
				w.WriteHeader(http.StatusBadRequest)
			}
		}
	}))
	defer server.Close()

	// Run fetch
	cmd := exec.Command(binaryPath, "fetch", "test/repo", "--all", "--output", outputFile)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("GITHUB_TOKEN=%s", "test-token"),
		fmt.Sprintf("GITHUB_API_URL=%s", server.URL),
		fmt.Sprintf("SIRSEER_STATE_DIR=%s", stateDir),
	)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		t.Fatalf("Command failed: %v\nStderr: %s", err, stderr.String())
	}

	// Verify state was saved and resumed correctly
	stderrStr := stderr.String()
	if !strings.Contains(stderrStr, "Saving state") {
		t.Error("Expected state save messages")
	}
	if !strings.Contains(stderrStr, "Retrying") {
		t.Error("Expected retry messages")
	}

	// Verify we got all data despite failures
	verifyNDJSONOutput(t, outputFile, 75)
}

// Helper functions for network failure testing

func setupTransientErrorServer(t *testing.T, errorCode int, failCount int) *httptest.Server {
	// Use the new mock server builder for consistency
	return testutil.NewMockServerBuilder(t).
		WithFailures(failCount, errorCode).
		WithPullRequests(10, 10).
		Build().Server
}

func setupTimeoutServer(t *testing.T, timeoutCount int) *httptest.Server {
	// Use the timeout server from testutil
	return testutil.NewTimeoutServer(t, timeoutCount).Server
}

func setupPermanentErrorServer(t *testing.T, errorCode int) *httptest.Server {
	// Use the error server from testutil
	return testutil.NewErrorServer(t, errorCode).Server
}

func setupConnectionRefusedServer(t *testing.T, refuseCount int) *httptest.Server {
	var requestCount int32
	var actualURL string

	// Create a listener we can control
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}

	server := &httptest.Server{
		Listener: listener,
		Config: &http.Server{
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				count := atomic.AddInt32(&requestCount, 1)

				if count > int32(refuseCount) {
					// Success after connection refusals
					response := testutil.GeneratePRResponse(1, 10, false)
					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode(response)
				}
			}),
		},
	}

	actualURL = "http://" + listener.Addr().String()

	// Start server in background
	go func() {
		// Simulate connection refused by delaying server start
		time.Sleep(2 * time.Second)
		server.Start()
	}()

	// Override URL to return the correct address
	server.URL = actualURL

	return server
}

func setupPartialResponseServer(t *testing.T) *httptest.Server {
	var requestCount int32
	var pageRequestsMu sync.Mutex
	pageRequests := make(map[string]int32)

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)

		// Parse cursor
		var req struct {
			Variables struct {
				After string `json:"after"`
			} `json:"variables"`
		}
		json.NewDecoder(r.Body).Decode(&req)

		cursor := req.Variables.After
		if cursor == "" {
			cursor = "start"
		}

		// Get current count for this cursor (thread-safe)
		pageRequestsMu.Lock()
		pageRequests[cursor]++
		cursorCount := pageRequests[cursor]
		pageRequestsMu.Unlock()

		// First attempt at page 2 fails with partial response
		if cursor == "cursor1" && cursorCount == 1 {
			// Start sending response but cut it off
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"data":{"repository":{"pullRequests":{`))
			// Forcefully close connection to simulate network failure
			if hijacker, ok := w.(http.Hijacker); ok {
				conn, _, _ := hijacker.Hijack()
				conn.Close()
			}
			return
		}

		// Normal response
		page := 1
		if cursor == "cursor1" {
			page = 2
		}

		hasNext := page < 2
		start := (page-1)*25 + 1
		end := page * 25

		response := testutil.GeneratePRResponse(start, end, hasNext)
		if hasNext {
			response["data"].(map[string]interface{})["repository"].(map[string]interface{})["pullRequests"].(map[string]interface{})["pageInfo"].(map[string]interface{})["endCursor"] = fmt.Sprintf("cursor%d", page)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
}

// TestNetworkResilienceStressTest performs a stress test of network resilience
func TestNetworkResilienceStressTest(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") != "true" || os.Getenv("STRESS_TEST") != "true" {
		t.Skip("Skipping stress test. Set INTEGRATION_TEST=true and STRESS_TEST=true to run.")
	}

	binaryPath := testutil.BuildBinary(t)
	testDir := testutil.CreateTempDir(t, "network-test")
	outputFile := filepath.Join(testDir, "output.ndjson")

	// Server with random failures
	var requestCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&requestCount, 1)

		// Random failures on ~30% of requests
		if count%3 == 0 {
			errors := []int{
				http.StatusBadGateway,
				http.StatusServiceUnavailable,
				http.StatusGatewayTimeout,
			}
			errorCode := errors[count%int32(len(errors))]
			w.WriteHeader(errorCode)
			return
		}

		// Success - return data
		page := int((count + 2) / 3) // Adjust for failures
		hasNext := page < 10
		start := (page-1)*10 + 1
		end := page * 10

		response := testutil.GeneratePRResponse(start, end, hasNext)
		if hasNext {
			response["data"].(map[string]interface{})["repository"].(map[string]interface{})["pullRequests"].(map[string]interface{})["pageInfo"].(map[string]interface{})["endCursor"] = fmt.Sprintf("cursor%d", page)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	cmd := exec.Command(binaryPath, "fetch", "test/repo", "--all", "--output", outputFile)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("GITHUB_TOKEN=%s", "test-token"),
		fmt.Sprintf("GITHUB_API_URL=%s", server.URL),
	)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		t.Fatalf("Command failed despite retries: %v\nStderr: %s", err, stderr.String())
	}

	// Verify all data was fetched despite random failures
	verifyNDJSONOutput(t, outputFile, 100)

	// Verify retries happened
	retryCount := strings.Count(stderr.String(), "Retrying")
	t.Logf("Stress test completed with %d retries", retryCount)
	if retryCount == 0 {
		t.Error("Expected some retries during stress test")
	}
}

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

// Package testutil provides common test helpers for sirseer-relay
package testutil

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

// MockServer provides common mock server configurations for testing
type MockServer struct {
	*httptest.Server
	RequestCount int32
}

// NewMockServer creates a basic mock server that responds to GraphQL requests
func NewMockServer(t *testing.T, handler http.HandlerFunc) *MockServer {
	t.Helper()
	server := httptest.NewServer(handler)
	return &MockServer{Server: server}
}

// NewRateLimitServer creates a mock server that simulates rate limiting
func NewRateLimitServer(t *testing.T, retryAfter, successAfterCount int) *MockServer {
	t.Helper()
	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&requestCount, 1)

		if count <= int32(successAfterCount) {
			w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
			w.WriteHeader(429)
			_, _ = w.Write([]byte(`{"message": "API rate limit exceeded"}`))
			return
		}

		// Success response after rate limit
		response := GeneratePRResponse(1, 10, false)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))

	return &MockServer{Server: server, RequestCount: requestCount}
}

// NewErrorServer creates a mock server that always returns the specified error
func NewErrorServer(t *testing.T, statusCode int) *MockServer {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(statusCode)
		_, _ = w.Write([]byte(http.StatusText(statusCode)))
	}))
	return &MockServer{Server: server}
}

// NewTransientErrorServer creates a mock server that fails N times then succeeds
func NewTransientErrorServer(t *testing.T, failCount, errorCode int) *MockServer {
	t.Helper()
	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&requestCount, 1)

		if count <= int32(failCount) {
			w.WriteHeader(errorCode)
			_, _ = w.Write([]byte(http.StatusText(errorCode)))
			return
		}

		// Success after failures
		response := GeneratePRResponse(1, 10, false)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))

	return &MockServer{Server: server, RequestCount: requestCount}
}

// NewTimeoutServer creates a mock server that times out N times then succeeds
func NewTimeoutServer(t *testing.T, timeoutCount int) *MockServer {
	t.Helper()
	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&requestCount, 1)

		if count <= int32(timeoutCount) {
			// Simulate timeout by sleeping longer than client timeout
			time.Sleep(10 * time.Second)
			return
		}

		// Success after timeouts
		response := GeneratePRResponse(1, 10, false)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))

	return &MockServer{Server: server, RequestCount: requestCount}
}

// GeneratePRResponse generates a mock GraphQL response with PRs
func GeneratePRResponse(startNum, endNum int, hasMore bool) map[string]interface{} {
	prs := make([]map[string]interface{}, 0)

	for i := startNum; i <= endNum; i++ {
		prs = append(prs, map[string]interface{}{
			"number":    i,
			"title":     fmt.Sprintf("PR %d", i),
			"state":     "OPEN",
			"createdAt": time.Now().AddDate(0, 0, -i).Format(time.RFC3339),
			"author": map[string]interface{}{
				"login": fmt.Sprintf("user%d", i),
			},
		})
	}

	var cursor *string
	if hasMore {
		c := fmt.Sprintf("cursor%d", endNum)
		cursor = &c
	}

	return map[string]interface{}{
		"data": map[string]interface{}{
			"repository": map[string]interface{}{
				"pullRequests": map[string]interface{}{
					"nodes": prs,
					"pageInfo": map[string]interface{}{
						"hasNextPage": hasMore,
						"endCursor":   cursor,
					},
				},
			},
		},
	}
}

// AssertGraphQLRequest validates a GraphQL request structure
func AssertGraphQLRequest(t *testing.T, r *http.Request) {
	t.Helper()
	if r.URL.Path != "/graphql" {
		t.Errorf("Unexpected path: %s", r.URL.Path)
	}
	if r.Method != "POST" {
		t.Errorf("Expected POST method, got: %s", r.Method)
	}
	if ct := r.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Expected Content-Type: application/json, got: %s", ct)
	}
}

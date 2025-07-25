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

// MockServerBuilder provides a fluent API for creating configured mock servers
type MockServerBuilder struct {
	t                *testing.T
	failureCount     int
	errorCode        int
	retryAfter       int
	responseDelay    time.Duration
	totalPRs         int
	pageSize         int
	rateLimitResetAt int64
	complexityError  bool
	customHandler    http.HandlerFunc
}

// NewMockServerBuilder creates a new mock server builder
func NewMockServerBuilder(t *testing.T) *MockServerBuilder {
	t.Helper()
	return &MockServerBuilder{
		t:        t,
		pageSize: 10,
	}
}

// WithFailures configures the server to fail N times before succeeding
func (b *MockServerBuilder) WithFailures(count int, errorCode int) *MockServerBuilder {
	b.failureCount = count
	b.errorCode = errorCode
	return b
}

// WithRateLimit configures rate limiting behavior
func (b *MockServerBuilder) WithRateLimit(retryAfter int) *MockServerBuilder {
	b.retryAfter = retryAfter
	return b
}

// WithRateLimitReset configures rate limit reset time
func (b *MockServerBuilder) WithRateLimitReset(resetAt int64) *MockServerBuilder {
	b.rateLimitResetAt = resetAt
	return b
}

// WithResponseDelay adds a delay before responding
func (b *MockServerBuilder) WithResponseDelay(delay time.Duration) *MockServerBuilder {
	b.responseDelay = delay
	return b
}

// WithPullRequests configures the number of PRs to return
func (b *MockServerBuilder) WithPullRequests(total int, pageSize int) *MockServerBuilder {
	b.totalPRs = total
	b.pageSize = pageSize
	return b
}

// WithComplexityError simulates query complexity errors
func (b *MockServerBuilder) WithComplexityError() *MockServerBuilder {
	b.complexityError = true
	return b
}

// WithCustomHandler uses a custom handler function
func (b *MockServerBuilder) WithCustomHandler(handler http.HandlerFunc) *MockServerBuilder {
	b.customHandler = handler
	return b
}

// Build creates the configured mock server
func (b *MockServerBuilder) Build() *MockServer {
	if b.customHandler != nil {
		server := httptest.NewServer(b.customHandler)
		return &MockServer{Server: server}
	}

	var requestCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&requestCount, 1)

		// Add response delay if configured
		if b.responseDelay > 0 {
			time.Sleep(b.responseDelay)
		}

		// Handle transient failures
		if b.failureCount > 0 && count <= int32(b.failureCount) {
			if b.retryAfter > 0 {
				w.Header().Set("Retry-After", strconv.Itoa(b.retryAfter))
			}
			if b.rateLimitResetAt > 0 {
				w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(b.rateLimitResetAt, 10))
			}
			w.WriteHeader(b.errorCode)
			if b.errorCode == 429 {
				w.Write([]byte(`{"message": "API rate limit exceeded"}`))
			} else {
				w.Write([]byte(http.StatusText(b.errorCode)))
			}
			return
		}

		// Handle complexity error
		if b.complexityError {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"errors": []interface{}{
					map[string]interface{}{
						"message": "Query complexity exceeds maximum allowed",
					},
				},
			})
			return
		}

		// Default: return PRs
		startNum := 1
		endNum := b.pageSize
		if b.totalPRs > 0 && endNum > b.totalPRs {
			endNum = b.totalPRs
		}
		hasMore := b.totalPRs > 0 && endNum < b.totalPRs

		response := GeneratePRResponse(startNum, endNum, hasMore)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))

	return &MockServer{Server: server, RequestCount: requestCount}
}

// GeneratePRResponse generates a mock GraphQL response with PRs
func GeneratePRResponse(startNum, endNum int, hasMore bool) map[string]interface{} {
	prs := make([]map[string]interface{}, 0)

	for i := startNum; i <= endNum; i++ {
		prs = append(prs, CreatePullRequest(i))
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

// CreatePullRequest creates a single PR with realistic data
func CreatePullRequest(number int) map[string]interface{} {
	now := time.Now()
	createdAt := now.AddDate(0, 0, -number)
	updatedAt := createdAt.Add(time.Hour * 24)

	return map[string]interface{}{
		"number":    number,
		"title":     fmt.Sprintf("PR %d", number),
		"state":     "OPEN",
		"body":      fmt.Sprintf("This is the body of PR %d", number),
		"url":       fmt.Sprintf("https://github.com/test/repo/pull/%d", number),
		"createdAt": createdAt.Format(time.RFC3339),
		"updatedAt": updatedAt.Format(time.RFC3339),
		"mergedAt":  nil,
		"closedAt":  nil,
		"merged":    false,
		"author": map[string]interface{}{
			"login": fmt.Sprintf("user%d", number),
		},
		"baseRef": map[string]interface{}{
			"name": "main",
			"target": map[string]interface{}{
				"oid": "base" + strconv.Itoa(number),
			},
		},
		"headRef": map[string]interface{}{
			"name": fmt.Sprintf("feature-%d", number),
			"target": map[string]interface{}{
				"oid": "head" + strconv.Itoa(number),
			},
		},
		"additions":          10 + number,
		"deletions":          5 + number,
		"changedFiles":       2 + (number % 3),
		"comments":           number % 5,
		"reviewComments":     number % 3,
		"totalCommentsCount": number % 5,
		"commits": map[string]interface{}{
			"totalCount": 1 + (number % 4),
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

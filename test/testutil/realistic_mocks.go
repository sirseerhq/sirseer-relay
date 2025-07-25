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
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// GitHubLikeMockServer creates a mock server that behaves like the real GitHub API
type GitHubLikeMockServer struct {
	*httptest.Server
	mu                 sync.RWMutex
	rateLimitRemaining int32
	rateLimitReset     int64
	complexity         int32
	requestHistory     []GraphQLRequest
}

// GraphQLRequest represents a parsed GraphQL request
type GraphQLRequest struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables"`
	Timestamp time.Time
}

// NewGitHubLikeMockServer creates a realistic GitHub API mock
func NewGitHubLikeMockServer(t *testing.T) *GitHubLikeMockServer {
	t.Helper()

	mock := &GitHubLikeMockServer{
		rateLimitRemaining: 5000,
		rateLimitReset:     time.Now().Add(time.Hour).Unix(),
		requestHistory:     []GraphQLRequest{},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Validate request method and path
		if r.Method != "POST" || r.URL.Path != "/graphql" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// Check authorization
		auth := r.Header.Get("Authorization")
		if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{
				"message":           "Bad credentials",
				"documentation_url": "https://docs.github.com/en/rest",
			})
			return
		}

		// Parse GraphQL request
		var req GraphQLRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"message": "Problems parsing JSON",
			})
			return
		}
		req.Timestamp = time.Now()

		// Store request history
		mock.mu.Lock()
		mock.requestHistory = append(mock.requestHistory, req)
		mock.mu.Unlock()

		// Calculate query complexity
		complexity := mock.calculateQueryComplexity(req.Query)

		// Check rate limit
		remaining := atomic.AddInt32(&mock.rateLimitRemaining, -1)
		if remaining < 0 {
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(mock.rateLimitReset, 10))
			w.Header().Set("Retry-After", "3600")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]string{
				"message":           "API rate limit exceeded",
				"documentation_url": "https://docs.github.com/en/rest/rate-limit",
			})
			return
		}

		// Set rate limit headers
		w.Header().Set("X-RateLimit-Limit", "5000")
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(int(remaining)))
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(mock.rateLimitReset, 10))

		// Check query complexity
		if complexity > 1000 {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"errors": []map[string]interface{}{
					{
						"message": fmt.Sprintf("Query has complexity %d, which exceeds max complexity of 1000", complexity),
						"type":    "QUERY_COMPLEXITY",
					},
				},
			})
			return
		}

		// Parse variables
		pageSize := 10
		if req.Variables != nil {
			if ps, ok := req.Variables["pageSize"].(float64); ok {
				pageSize = int(ps)
			}
		}

		// Simulate response based on query
		response := mock.generateResponse(req, pageSize)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))

	mock.Server = server
	return mock
}

// calculateQueryComplexity estimates query complexity based on fields and page size
func (m *GitHubLikeMockServer) calculateQueryComplexity(query string) int32 {
	// Basic complexity calculation
	complexity := int32(1)

	// Count fields
	fields := strings.Count(query, "\n") + 1
	complexity += int32(fields * 2)

	// Check for nested fields
	if strings.Contains(query, "author {") {
		complexity += 10
	}
	if strings.Contains(query, "commits {") {
		complexity += 50
	}
	if strings.Contains(query, "reviews {") {
		complexity += 30
	}
	if strings.Contains(query, "reviewRequests {") {
		complexity += 20
	}

	// Check page size
	if strings.Contains(query, "first:") {
		// Extract page size from query
		start := strings.Index(query, "first:") + 6
		end := start
		for end < len(query) && query[end] >= '0' && query[end] <= '9' {
			end++
		}
		if pageSize, err := strconv.Atoi(query[start:end]); err == nil {
			complexity += int32(pageSize * 10)
		}
	}

	return complexity
}

// generateResponse creates a realistic GraphQL response
func (m *GitHubLikeMockServer) generateResponse(req GraphQLRequest, pageSize int) map[string]interface{} {
	// Check if it's a repository info query
	if strings.Contains(req.Query, "pullRequests { totalCount }") && !strings.Contains(req.Query, "nodes {") {
		return map[string]interface{}{
			"data": map[string]interface{}{
				"repository": map[string]interface{}{
					"pullRequests": map[string]interface{}{
						"totalCount": 1234,
					},
				},
			},
		}
	}

	// Generate PR list response
	cursor := ""
	if req.Variables != nil {
		if c, ok := req.Variables["after"].(string); ok {
			cursor = c
		}
	}

	// Parse cursor to determine page
	page := 0
	if cursor != "" {
		fmt.Sscanf(cursor, "cursor_%d", &page)
		page++
	}

	// Calculate PR range
	startNum := page*pageSize + 1
	endNum := startNum + pageSize - 1
	totalPRs := 100 // Simulate 100 total PRs

	if startNum > totalPRs {
		return map[string]interface{}{
			"data": map[string]interface{}{
				"repository": map[string]interface{}{
					"pullRequests": map[string]interface{}{
						"nodes": []interface{}{},
						"pageInfo": map[string]interface{}{
							"hasNextPage": false,
							"endCursor":   nil,
						},
					},
				},
			},
		}
	}

	if endNum > totalPRs {
		endNum = totalPRs
	}

	hasMore := endNum < totalPRs
	nextCursor := fmt.Sprintf("cursor_%d", page)

	// Build PR nodes
	nodes := []map[string]interface{}{}
	for i := startNum; i <= endNum; i++ {
		pr := NewPullRequestBuilder(i).Build()

		// Randomly make some PRs closed or merged
		if rand.Float32() < 0.3 {
			pr["state"] = "CLOSED"
			closedAt := time.Now().AddDate(0, 0, -i+10)
			pr["closedAt"] = closedAt.Format(time.RFC3339)

			if rand.Float32() < 0.7 {
				pr["state"] = "MERGED"
				pr["mergedAt"] = closedAt.Format(time.RFC3339)
			}
		}

		nodes = append(nodes, pr)
	}

	return map[string]interface{}{
		"data": map[string]interface{}{
			"repository": map[string]interface{}{
				"pullRequests": map[string]interface{}{
					"nodes": nodes,
					"pageInfo": map[string]interface{}{
						"hasNextPage": hasMore,
						"endCursor":   nextCursor,
					},
				},
			},
		},
	}
}

// GetRequestHistory returns the history of GraphQL requests
func (m *GitHubLikeMockServer) GetRequestHistory() []GraphQLRequest {
	m.mu.RLock()
	defer m.mu.RUnlock()

	history := make([]GraphQLRequest, len(m.requestHistory))
	copy(history, m.requestHistory)
	return history
}

// ResetRateLimit resets the rate limit counter
func (m *GitHubLikeMockServer) ResetRateLimit() {
	atomic.StoreInt32(&m.rateLimitRemaining, 5000)
	m.rateLimitReset = time.Now().Add(time.Hour).Unix()
}

// SetRateLimit sets a specific rate limit
func (m *GitHubLikeMockServer) SetRateLimit(remaining int32) {
	atomic.StoreInt32(&m.rateLimitRemaining, remaining)
}

// FlakeyNetworkServer creates a server that randomly fails
type FlakeyNetworkServer struct {
	*httptest.Server
	failureRate float32
	mu          sync.Mutex
	requests    int32
}

// NewFlakeyNetworkServer creates a server with intermittent failures
func NewFlakeyNetworkServer(t *testing.T, failureRate float32) *FlakeyNetworkServer {
	t.Helper()

	mock := &FlakeyNetworkServer{
		failureRate: failureRate,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&mock.requests, 1)

		// Randomly fail based on failure rate
		if rand.Float32() < mock.failureRate {
			// Pick a random failure mode
			switch rand.Intn(4) {
			case 0:
				// Connection timeout (no response)
				time.Sleep(10 * time.Second)
			case 1:
				// 502 Bad Gateway
				w.WriteHeader(http.StatusBadGateway)
				w.Write([]byte("Bad Gateway"))
			case 2:
				// 503 Service Unavailable
				w.WriteHeader(http.StatusServiceUnavailable)
				w.Write([]byte("Service Unavailable"))
			case 3:
				// Partial response (corrupted JSON)
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"data": {"repository": {"pullRequests": {"nodes": [`))
				// Abruptly end response
			}
			return
		}

		// Success response
		response := GeneratePRResponse(1, 10, false)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))

	mock.Server = server
	return mock
}

// GetRequestCount returns the number of requests received
func (f *FlakeyNetworkServer) GetRequestCount() int32 {
	return atomic.LoadInt32(&f.requests)
}

// SetFailureRate updates the failure rate
func (f *FlakeyNetworkServer) SetFailureRate(rate float32) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failureRate = rate
}

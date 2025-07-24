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

package github

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	relaierrors "github.com/sirseerhq/sirseer-relay/internal/errors"
)

// Compile-time check that GraphQLClient implements Client
var _ Client = (*GraphQLClient)(nil)

func TestGraphQLClient_MapError(t *testing.T) {
	client := &GraphQLClient{}

	tests := []struct {
		name        string
		err         error
		owner       string
		repo        string
		wantErr     error
		wantMessage string
	}{
		{
			name:        "authentication error 401",
			err:         fmt.Errorf("401 Unauthorized"),
			owner:       "test",
			repo:        "repo",
			wantErr:     relaierrors.ErrInvalidToken,
			wantMessage: "GitHub API authentication failed",
		},
		{
			name:        "authentication error 403",
			err:         fmt.Errorf("403 Forbidden"),
			owner:       "test",
			repo:        "repo",
			wantErr:     relaierrors.ErrInvalidToken,
			wantMessage: "GitHub API authentication failed",
		},
		{
			name:        "authentication error unauthorized",
			err:         fmt.Errorf("request is unauthorized"),
			owner:       "test",
			repo:        "repo",
			wantErr:     relaierrors.ErrInvalidToken,
			wantMessage: "GitHub API authentication failed",
		},
		{
			name:        "not found error 404",
			err:         fmt.Errorf("404 Not Found"),
			owner:       "test",
			repo:        "repo",
			wantErr:     relaierrors.ErrRepoNotFound,
			wantMessage: "repository 'test/repo' not found",
		},
		{
			name:        "GraphQL repo not found",
			err:         fmt.Errorf("Could not resolve to a Repository"),
			owner:       "test",
			repo:        "repo",
			wantErr:     relaierrors.ErrRepoNotFound,
			wantMessage: "repository 'test/repo' not found",
		},
		{
			name:        "rate limit error",
			err:         fmt.Errorf("API rate limit exceeded"),
			owner:       "test",
			repo:        "repo",
			wantErr:     relaierrors.ErrRateLimit,
			wantMessage: "rate limit exceeded",
		},
		{
			name:        "rate limit 429",
			err:         fmt.Errorf("429 Too Many Requests"),
			owner:       "test",
			repo:        "repo",
			wantErr:     relaierrors.ErrRateLimit,
			wantMessage: "rate limit exceeded",
		},
		{
			name:        "network error - connection refused",
			err:         fmt.Errorf("connection refused"),
			owner:       "test",
			repo:        "repo",
			wantErr:     relaierrors.ErrNetworkFailure,
			wantMessage: "network error",
		},
		{
			name:        "network error - no such host",
			err:         fmt.Errorf("no such host"),
			owner:       "test",
			repo:        "repo",
			wantErr:     relaierrors.ErrNetworkFailure,
			wantMessage: "network error",
		},
		{
			name:        "network error - timeout",
			err:         fmt.Errorf("request timeout"),
			owner:       "test",
			repo:        "repo",
			wantErr:     relaierrors.ErrNetworkFailure,
			wantMessage: "network error",
		},
		{
			name:        "generic error",
			err:         fmt.Errorf("something went wrong"),
			owner:       "test",
			repo:        "repo",
			wantErr:     nil,
			wantMessage: "failed to fetch pull requests",
		},
		{
			name:        "nil error",
			err:         nil,
			owner:       "test",
			repo:        "repo",
			wantErr:     nil,
			wantMessage: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := client.mapError(tt.err, tt.owner, tt.repo)

			if tt.err == nil {
				if err != nil {
					t.Fatalf("expected nil error, got %v", err)
				}
				return
			}

			if err == nil {
				t.Fatal("expected error, got nil")
			}

			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Errorf("expected error %v, got %v", tt.wantErr, err)
			}

			if tt.wantMessage != "" && !strings.Contains(err.Error(), tt.wantMessage) {
				t.Errorf("expected error message to contain %q, got %v", tt.wantMessage, err)
			}
		})
	}
}

func TestAuthTransport(t *testing.T) {
	token := "test-token"
	transport := &authTransport{
		token: token,
		base:  http.DefaultTransport,
	}

	// Create a test server to verify headers
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check auth header
		auth := r.Header.Get("Authorization")
		if auth != "Bearer "+token {
			t.Errorf("expected Authorization header 'Bearer %s', got %q", token, auth)
		}

		// Check user agent
		ua := r.Header.Get("User-Agent")
		if !strings.Contains(ua, "sirseer-relay") {
			t.Errorf("expected User-Agent to contain 'sirseer-relay', got %q", ua)
		}

		// Return some data to test size limiting
		fmt.Fprint(w, "test response")
	}))
	defer server.Close()

	// Make a request
	req, err := http.NewRequest("GET", server.URL, http.NoBody)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	client := &http.Client{Transport: transport}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}

	if string(body) != "test response" {
		t.Errorf("expected body 'test response', got %q", string(body))
	}
}

func TestLimitedReader(t *testing.T) {
	t.Run("within limit", func(t *testing.T) {
		data := "hello world"
		reader := &limitedReader{
			ReadCloser: io.NopCloser(strings.NewReader(data)),
			limit:      100,
		}

		body, err := io.ReadAll(reader)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if string(body) != data {
			t.Errorf("expected %q, got %q", data, string(body))
		}
	})

	t.Run("exceeds limit", func(t *testing.T) {
		// Test that limited reader enforces size limit
		data := strings.Repeat("a", 100)
		reader := &limitedReader{
			ReadCloser: io.NopCloser(strings.NewReader(data)),
			limit:      50,
		}

		buf := make([]byte, 200)
		totalRead := 0

		for {
			n, err := reader.Read(buf[totalRead:])
			totalRead += n

			if err != nil {
				if err == io.EOF && totalRead == 50 {
					// This is expected - we read up to the limit
					break
				}
				if strings.Contains(err.Error(), "exceeded limit") {
					// This is also acceptable
					break
				}
				t.Fatalf("unexpected error: %v", err)
			}

			if totalRead > 50 {
				t.Errorf("read more than limit: %d > 50", totalRead)
				break
			}
		}

		if totalRead != 50 {
			t.Errorf("expected to read exactly 50 bytes, got %d", totalRead)
		}
	})
}


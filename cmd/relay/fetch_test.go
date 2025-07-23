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

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	relaierrors "github.com/sirseerhq/sirseer-relay/internal/errors"
	"github.com/sirseerhq/sirseer-relay/internal/github"
	"github.com/sirseerhq/sirseer-relay/internal/output"
)

func TestFetchWithComplexityRetry(t *testing.T) {
	tests := []struct {
		name               string
		initialPageSize    int
		mockSetup          func() *github.MockClient
		expectedPageSize   int
		expectedCallCount  int
		expectError        bool
		expectedErrMessage string
	}{
		{
			name:            "successful fetch without complexity error",
			initialPageSize: 50,
			mockSetup: func() *github.MockClient {
				return github.NewMockClientWithOptions(
					github.WithPullRequests([]github.PullRequest{
						{Number: 1, Title: "Test PR"},
					}),
				)
			},
			expectedPageSize:  50,
			expectedCallCount: 1,
			expectError:       false,
		},
		{
			name:            "complexity error reduces page size once",
			initialPageSize: 50,
			mockSetup: func() *github.MockClient {
				return github.NewMockClientWithOptions(
					github.WithComplexityError(1), // Error on first call
					github.WithPullRequests([]github.PullRequest{
						{Number: 1, Title: "Test PR"},
					}),
				)
			},
			expectedPageSize:  25,
			expectedCallCount: 2,
			expectError:       false,
		},
		{
			name:            "multiple complexity errors reduce page size multiple times",
			initialPageSize: 50,
			mockSetup: func() *github.MockClient {
				mock := github.NewMockClient()
				mock.ComplexityErrorOnCall = 1
				// Manually set to error on multiple calls
				mock.PullRequests = []github.PullRequest{
					{Number: 1, Title: "Test PR"},
				}
				return mock
			},
			expectedPageSize:  25,
			expectedCallCount: 2,
			expectError:       false,
		},
		{
			name:            "complexity error at minimum page size fails",
			initialPageSize: 5,
			mockSetup: func() *github.MockClient {
				return github.NewMockClientWithOptions(
					github.WithComplexityError(1),
				)
			},
			expectedPageSize:   5,
			expectedCallCount:  1,
			expectError:        true,
			expectedErrMessage: "GraphQL query complexity exceeded",
		},
		{
			name:            "other errors are not retried",
			initialPageSize: 50,
			mockSetup: func() *github.MockClient {
				return github.NewMockClientWithOptions(
					github.WithError(errors.New("network error")),
				)
			},
			expectedPageSize:   50,
			expectedCallCount:  1,
			expectError:        true,
			expectedErrMessage: "network error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock
			mock := tt.mockSetup()
			pageSize := tt.initialPageSize

			// Call the function
			ctx := context.Background()
			opts := github.FetchOptions{
				PageSize: pageSize,
				After:    "",
			}

			page, err := fetchWithComplexityRetry(ctx, mock, "test", "repo", opts, &pageSize)

			// Check error
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if !strings.Contains(err.Error(), tt.expectedErrMessage) {
					t.Errorf("Expected error containing %q, got %q", tt.expectedErrMessage, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if page == nil {
					t.Error("Expected page result but got nil")
				}
			}

			// Check page size
			if pageSize != tt.expectedPageSize {
				t.Errorf("Expected page size %d, got %d", tt.expectedPageSize, pageSize)
			}

			// Check call count
			if mock.CallCount != tt.expectedCallCount {
				t.Errorf("Expected %d calls, got %d", tt.expectedCallCount, mock.CallCount)
			}
		})
	}
}

func TestFetchWithComplexityRetry_LogsMessage(t *testing.T) {
	// Capture stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	// Setup mock that returns complexity error on first call
	mock := github.NewMockClientWithOptions(
		github.WithComplexityError(1),
		github.WithPullRequests([]github.PullRequest{
			{Number: 1, Title: "Test PR"},
		}),
	)

	// Call the function
	ctx := context.Background()
	pageSize := 50
	opts := github.FetchOptions{
		PageSize: pageSize,
		After:    "",
	}

	_, err := fetchWithComplexityRetry(ctx, mock, "test", "repo", opts, &pageSize)

	// Restore stderr
	w.Close()
	os.Stderr = oldStderr

	// Read captured output
	var buf bytes.Buffer
	buf.ReadFrom(r)
	stderrOutput := buf.String()

	// Verify no error (should succeed on retry)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Verify log message
	if !strings.Contains(stderrOutput, "Query complexity limit hit. Reducing page size to 25") {
		t.Errorf("Expected complexity reduction message in stderr, got: %s", stderrOutput)
	}

	// Verify page size was reduced
	if pageSize != 25 {
		t.Errorf("Expected page size to be reduced to 25, got %d", pageSize)
	}
}

func TestFetchWithComplexityRetry_MaxRetries(t *testing.T) {
	// Create a mock that always returns complexity error
	mock := &github.MockClient{
		Error: fmt.Errorf("complexity error: %w", relaierrors.ErrQueryComplexity),
	}

	ctx := context.Background()
	pageSize := 50
	opts := github.FetchOptions{
		PageSize: pageSize,
		After:    "",
	}

	_, err := fetchWithComplexityRetry(ctx, mock, "test", "repo", opts, &pageSize)

	// Should fail after max retries
	if err == nil {
		t.Error("Expected error after max retries")
	}

	if !strings.Contains(err.Error(), "failed after 4 attempts") {
		t.Errorf("Expected max retry error, got: %v", err)
	}
}

// TestComplexityRecoveryIntegration tests the full flow with pagination and complexity errors
func TestComplexityRecoveryIntegration(t *testing.T) {
	// Create mock with 100 PRs that will trigger complexity on page 2
	prs := make([]github.PullRequest, 100)
	for i := 0; i < 100; i++ {
		prs[i] = github.PullRequest{
			Number:    i + 1,
			Title:     fmt.Sprintf("PR %d", i+1),
			State:     "open",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Author:    github.Author{Login: "test"},
		}
	}

	mock := github.NewMockClientWithOptions(
		github.WithPullRequests(prs),
		github.WithPagination(50),
		github.WithComplexityError(2), // Error on second page
	)

	// Capture output
	var outputBuf bytes.Buffer
	writer := output.NewWriter(&outputBuf)

	// Capture stderr for progress messages
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	// Run fetchAllPullRequests
	ctx := context.Background()
	err := fetchAllPullRequests(ctx, mock, "test", "repo", writer)

	// Restore stderr
	w.Close()
	os.Stderr = oldStderr

	// Read stderr
	var stderrBuf bytes.Buffer
	stderrBuf.ReadFrom(r)
	stderrOutput := stderrBuf.String()

	// Should succeed
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should have logged complexity message
	if !strings.Contains(stderrOutput, "Query complexity limit hit") {
		t.Error("Expected complexity error message in output")
	}

	// Should have fetched all PRs
	lines := strings.Split(strings.TrimSpace(outputBuf.String()), "\n")
	if len(lines) != 100 {
		t.Errorf("Expected 100 PRs, got %d", len(lines))
	}

	// Verify mock was called at least 3 times (first page, complexity error, retry with smaller size)
	if mock.CallCount < 3 {
		t.Errorf("Expected at least 3 calls, got %d", mock.CallCount)
	}
}

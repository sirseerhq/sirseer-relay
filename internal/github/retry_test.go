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
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

// mockClientWithErrors is a mock client that returns specific errors
type mockClientWithErrors struct {
	attempts      int
	maxFailures   int
	failureError  error
	successResult *PullRequestPage
}

func (m *mockClientWithErrors) FetchPullRequests(ctx context.Context, owner, repo string, opts FetchOptions) (*PullRequestPage, error) {
	m.attempts++
	if m.attempts <= m.maxFailures {
		return nil, m.failureError
	}
	return m.successResult, nil
}

func (m *mockClientWithErrors) FetchPullRequestsSearch(ctx context.Context, owner, repo string, opts FetchOptions) (*PullRequestPage, error) {
	m.attempts++
	if m.attempts <= m.maxFailures {
		return nil, m.failureError
	}
	return m.successResult, nil
}

func (m *mockClientWithErrors) GetRepositoryInfo(ctx context.Context, owner, repo string) (*RepositoryInfo, error) {
	m.attempts++
	if m.attempts <= m.maxFailures {
		return nil, m.failureError
	}
	return &RepositoryInfo{TotalPullRequests: 100}, nil
}

func TestRetryClient_RateLimitRetry(t *testing.T) {
	tests := []struct {
		name         string
		maxFailures  int
		maxRetries   int
		expectError  bool
		expectedAttempts int
	}{
		{
			name:         "succeeds after one retry",
			maxFailures:  1,
			maxRetries:   3,
			expectError:  false,
			expectedAttempts: 2,
		},
		{
			name:         "succeeds after max retries",
			maxFailures:  3,
			maxRetries:   3,
			expectError:  false,
			expectedAttempts: 4,
		},
		{
			name:         "fails after max retries exceeded",
			maxFailures:  5,
			maxRetries:   3,
			expectError:  true,
			expectedAttempts: 4,
		},
		{
			name:         "succeeds immediately",
			maxFailures:  0,
			maxRetries:   3,
			expectError:  false,
			expectedAttempts: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock client that fails with rate limit error
			mockClient := &mockClientWithErrors{
				maxFailures:   tt.maxFailures,
				failureError:  errors.New("API rate limit exceeded"),
				successResult: &PullRequestPage{},
			}

			// Create retry client with fast backoff for testing
			config := &RetryConfig{
				MaxRetries:        tt.maxRetries,
				InitialBackoff:    time.Millisecond,
				MaxBackoff:        10 * time.Millisecond,
				BackoffMultiplier: 2.0,
			}
			retryClient := NewRetryClient(mockClient, config)

			// Execute request
			ctx := context.Background()
			_, err := retryClient.FetchPullRequests(ctx, "owner", "repo", FetchOptions{})

			// Verify error expectation
			if tt.expectError && err == nil {
				t.Error("expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			// Verify number of attempts
			if mockClient.attempts != tt.expectedAttempts {
				t.Errorf("expected %d attempts, got %d", tt.expectedAttempts, mockClient.attempts)
			}
		})
	}
}

func TestRetryClient_NetworkErrorRetry(t *testing.T) {
	// Create mock client that fails with network error
	mockClient := &mockClientWithErrors{
		maxFailures:   2,
		failureError:  errors.New("dial tcp: connection refused"),
		successResult: &PullRequestPage{},
	}

	// Create retry client with fast backoff for testing
	config := &RetryConfig{
		MaxRetries:        3,
		InitialBackoff:    time.Millisecond,
		MaxBackoff:        10 * time.Millisecond,
		BackoffMultiplier: 2.0,
	}
	retryClient := NewRetryClient(mockClient, config)

	// Execute request
	ctx := context.Background()
	_, err := retryClient.FetchPullRequestsSearch(ctx, "owner", "repo", FetchOptions{})

	// Should succeed after retries
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Should have made 3 attempts (2 failures + 1 success)
	if mockClient.attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", mockClient.attempts)
	}
}

func TestRetryClient_NonRetryableError(t *testing.T) {
	nonRetryableErrors := []struct {
		name  string
		error string
	}{
		{"auth error", "401 unauthorized"},
		{"not found", "404 not found"},
		{"forbidden", "403 forbidden"},
	}

	for _, tt := range nonRetryableErrors {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock client that fails with non-retryable error
			mockClient := &mockClientWithErrors{
				maxFailures:  10,
				failureError: errors.New(tt.error),
			}

			// Create retry client
			config := &RetryConfig{
				MaxRetries:        3,
				InitialBackoff:    time.Millisecond,
				MaxBackoff:        10 * time.Millisecond,
				BackoffMultiplier: 2.0,
			}
			retryClient := NewRetryClient(mockClient, config)

			// Execute request
			ctx := context.Background()
			_, err := retryClient.GetRepositoryInfo(ctx, "owner", "repo")

			// Should fail immediately without retries
			if err == nil {
				t.Error("expected error but got nil")
			}

			// Should only make 1 attempt
			if mockClient.attempts != 1 {
				t.Errorf("expected 1 attempt, got %d", mockClient.attempts)
			}
		})
	}
}

func TestRetryClient_ContextCancellation(t *testing.T) {
	// Create mock client that always fails
	mockClient := &mockClientWithErrors{
		maxFailures:  10,
		failureError: errors.New("API rate limit exceeded"),
	}

	// Create retry client with longer backoff
	config := &RetryConfig{
		MaxRetries:        3,
		InitialBackoff:    100 * time.Millisecond,
		MaxBackoff:        1 * time.Second,
		BackoffMultiplier: 2.0,
	}
	retryClient := NewRetryClient(mockClient, config)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// Execute request
	start := time.Now()
	_, err := retryClient.FetchPullRequests(ctx, "owner", "repo", FetchOptions{})
	duration := time.Since(start)

	// Should fail with context error
	if err == nil || err != context.DeadlineExceeded {
		t.Errorf("expected context deadline exceeded error, got: %v", err)
	}

	// Should complete quickly due to context cancellation
	if duration > 100*time.Millisecond {
		t.Errorf("operation took too long: %v", duration)
	}

	// Should only make 1 or 2 attempts before context cancellation
	if mockClient.attempts > 2 {
		t.Errorf("too many attempts: %d", mockClient.attempts)
	}
}

func TestRetryClient_BackoffCalculation(t *testing.T) {
	config := &RetryConfig{
		MaxRetries:        5,
		InitialBackoff:    1 * time.Second,
		MaxBackoff:        30 * time.Second,
		BackoffMultiplier: 2.0,
	}
	client := &RetryClient{config: config}

	tests := []struct {
		attempt       int
		minExpected   time.Duration
		maxExpected   time.Duration
	}{
		{0, 900 * time.Millisecond, 1100 * time.Millisecond},     // 1s ± 10%
		{1, 1800 * time.Millisecond, 2200 * time.Millisecond},    // 2s ± 10%
		{2, 3600 * time.Millisecond, 4400 * time.Millisecond},    // 4s ± 10%
		{3, 7200 * time.Millisecond, 8800 * time.Millisecond},    // 8s ± 10%
		{4, 14400 * time.Millisecond, 17600 * time.Millisecond},  // 16s ± 10%
		{5, 27000 * time.Millisecond, 33000 * time.Millisecond},  // 30s (max) ± 10%
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("attempt_%d", tt.attempt), func(t *testing.T) {
			backoff := client.calculateBackoff(tt.attempt)
			if backoff < tt.minExpected || backoff > tt.maxExpected {
				t.Errorf("backoff for attempt %d = %v, want between %v and %v",
					tt.attempt, backoff, tt.minExpected, tt.maxExpected)
			}
		})
	}
}
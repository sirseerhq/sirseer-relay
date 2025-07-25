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
	"fmt"
	"math"
	"os"
	"time"

	"github.com/sirseerhq/sirseer-relay/internal/giterror"
)

// RetryConfig configures the retry behavior for API calls
type RetryConfig struct {
	// MaxRetries is the maximum number of retry attempts
	MaxRetries int
	// InitialBackoff is the initial backoff duration
	InitialBackoff time.Duration
	// MaxBackoff is the maximum backoff duration
	MaxBackoff time.Duration
	// BackoffMultiplier is the multiplier for exponential backoff
	BackoffMultiplier float64
}

// DefaultRetryConfig returns the default retry configuration
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxRetries:        3,
		InitialBackoff:    1 * time.Second,
		MaxBackoff:        30 * time.Second,
		BackoffMultiplier: 2.0,
	}
}

// RetryClient wraps a GitHub client with automatic retry logic for
// rate limits and transient network errors using exponential backoff.
type RetryClient struct {
	client    Client
	config    *RetryConfig
	inspector giterror.Inspector
}

// NewRetryClient creates a new RetryClient with the given configuration
func NewRetryClient(client Client, config *RetryConfig) Client {
	if config == nil {
		config = DefaultRetryConfig()
	}
	return &RetryClient{
		client:    client,
		config:    config,
		inspector: giterror.NewInspector(),
	}
}

// FetchPullRequests implements the Client interface with retry logic
func (r *RetryClient) FetchPullRequests(ctx context.Context, owner, repo string, opts FetchOptions) (*PullRequestPage, error) {
	var lastErr error
	
	for attempt := 0; attempt <= r.config.MaxRetries; attempt++ {
		page, err := r.client.FetchPullRequests(ctx, owner, repo, opts)
		if err == nil {
			return page, nil
		}
		
		lastErr = err
		
		// Don't retry on non-retryable errors
		if !r.shouldRetry(err) {
			return nil, err
		}
		
		// Don't retry if context is cancelled
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		
		// Calculate backoff duration
		backoff := r.calculateBackoff(attempt)
		
		// For rate limit errors, check if we have a specific reset time
		if r.inspector.IsRateLimitError(err) {
			fmt.Fprintf(os.Stderr, "\n⚠️  Rate limit hit. Waiting %v before retry (attempt %d/%d)...\n", 
				backoff, attempt+1, r.config.MaxRetries)
		} else {
			fmt.Fprintf(os.Stderr, "\n⚠️  Network error. Retrying in %v (attempt %d/%d)...\n", 
				backoff, attempt+1, r.config.MaxRetries)
		}
		
		// Wait with context cancellation support
		select {
		case <-time.After(backoff):
			// Continue to next retry
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	
	return nil, fmt.Errorf("failed after %d retries: %w", r.config.MaxRetries, lastErr)
}

// FetchPullRequestsSearch implements the Client interface with retry logic
func (r *RetryClient) FetchPullRequestsSearch(ctx context.Context, owner, repo string, opts FetchOptions) (*PullRequestPage, error) {
	var lastErr error
	
	for attempt := 0; attempt <= r.config.MaxRetries; attempt++ {
		page, err := r.client.FetchPullRequestsSearch(ctx, owner, repo, opts)
		if err == nil {
			return page, nil
		}
		
		lastErr = err
		
		// Don't retry on non-retryable errors
		if !r.shouldRetry(err) {
			return nil, err
		}
		
		// Don't retry if context is cancelled
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		
		// Calculate backoff duration
		backoff := r.calculateBackoff(attempt)
		
		// For rate limit errors, check if we have a specific reset time
		if r.inspector.IsRateLimitError(err) {
			fmt.Fprintf(os.Stderr, "\n⚠️  Rate limit hit. Waiting %v before retry (attempt %d/%d)...\n", 
				backoff, attempt+1, r.config.MaxRetries)
		} else {
			fmt.Fprintf(os.Stderr, "\n⚠️  Network error. Retrying in %v (attempt %d/%d)...\n", 
				backoff, attempt+1, r.config.MaxRetries)
		}
		
		// Wait with context cancellation support
		select {
		case <-time.After(backoff):
			// Continue to next retry
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	
	return nil, fmt.Errorf("failed after %d retries: %w", r.config.MaxRetries, lastErr)
}

// GetRepositoryInfo implements the Client interface with retry logic
func (r *RetryClient) GetRepositoryInfo(ctx context.Context, owner, repo string) (*RepositoryInfo, error) {
	var lastErr error
	
	for attempt := 0; attempt <= r.config.MaxRetries; attempt++ {
		info, err := r.client.GetRepositoryInfo(ctx, owner, repo)
		if err == nil {
			return info, nil
		}
		
		lastErr = err
		
		// Don't retry on non-retryable errors
		if !r.shouldRetry(err) {
			return nil, err
		}
		
		// Don't retry if context is cancelled
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		
		// Calculate backoff duration
		backoff := r.calculateBackoff(attempt)
		
		// For rate limit errors, check if we have a specific reset time
		if r.inspector.IsRateLimitError(err) {
			fmt.Fprintf(os.Stderr, "\n⚠️  Rate limit hit. Waiting %v before retry (attempt %d/%d)...\n", 
				backoff, attempt+1, r.config.MaxRetries)
		} else {
			fmt.Fprintf(os.Stderr, "\n⚠️  Network error. Retrying in %v (attempt %d/%d)...\n", 
				backoff, attempt+1, r.config.MaxRetries)
		}
		
		// Wait with context cancellation support
		select {
		case <-time.After(backoff):
			// Continue to next retry
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	
	return nil, fmt.Errorf("failed after %d retries: %w", r.config.MaxRetries, lastErr)
}

// shouldRetry determines if an error is retryable
func (r *RetryClient) shouldRetry(err error) bool {
	// Retry on rate limit errors
	if r.inspector.IsRateLimitError(err) {
		return true
	}
	
	// Retry on network errors
	if r.inspector.IsNetworkError(err) {
		return true
	}
	
	// Don't retry on other errors (auth, not found, etc.)
	return false
}

// calculateBackoff calculates the backoff duration for the given attempt
func (r *RetryClient) calculateBackoff(attempt int) time.Duration {
	// Calculate exponential backoff
	backoff := float64(r.config.InitialBackoff) * math.Pow(r.config.BackoffMultiplier, float64(attempt))
	
	// Apply max backoff limit
	if backoff > float64(r.config.MaxBackoff) {
		backoff = float64(r.config.MaxBackoff)
	}
	
	// Add jitter (±10%) to prevent thundering herd
	jitter := backoff * 0.1 * (2*float64(time.Now().UnixNano()%100)/100 - 1)
	backoff += jitter
	
	return time.Duration(backoff)
}
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
	"time"

	relaierrors "github.com/sirseerhq/sirseer-relay/internal/errors"
)

// MockClient is a mock implementation of the GitHub Client interface for testing.
type MockClient struct {
	// PullRequests to return
	PullRequests []PullRequest
	
	// Error to return
	Error error
	
	// Behavior flags
	ShouldFailAuth    bool
	ShouldFailNetwork bool
	ShouldFailNotFound bool
	
	// Track calls for verification
	CallCount int
	LastOwner string
	LastRepo  string
	LastOpts  FetchOptions
}

// NewMockClient creates a new mock client with default test data
func NewMockClient() *MockClient {
	return &MockClient{
		PullRequests: generateTestPRs(),
	}
}

// FetchPullRequests implements the Client interface
func (m *MockClient) FetchPullRequests(ctx context.Context, owner, repo string, opts FetchOptions) (*PullRequestPage, error) {
	// Track the call
	m.CallCount++
	m.LastOwner = owner
	m.LastRepo = repo
	m.LastOpts = opts
	
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	
	// Simulate various error conditions
	if m.ShouldFailAuth {
		return nil, fmt.Errorf("authentication failed: %w", relaierrors.ErrInvalidToken)
	}
	
	if m.ShouldFailNetwork {
		return nil, fmt.Errorf("network timeout: %w", relaierrors.ErrNetworkFailure)
	}
	
	if m.ShouldFailNotFound || (owner == "nonexistent" && repo == "repo") {
		return nil, fmt.Errorf("repository not found: %w", relaierrors.ErrRepoNotFound)
	}
	
	// Return configured error if set
	if m.Error != nil {
		return nil, m.Error
	}
	
	// Return the mock data
	page := &PullRequestPage{
		PullRequests: m.PullRequests,
		HasNextPage:  false, // Phase 1 only fetches one page
		EndCursor:    "",
	}
	
	return page, nil
}

// generateTestPRs creates sample pull request data for testing
func generateTestPRs() []PullRequest {
	now := time.Now().UTC()
	yesterday := now.Add(-24 * time.Hour)
	lastWeek := now.Add(-7 * 24 * time.Hour)
	
	merged := yesterday
	
	return []PullRequest{
		{
			Number:    1234,
			Title:     "Add new feature for data processing",
			State:     "open",
			CreatedAt: lastWeek,
			UpdatedAt: now,
			Author:    Author{Login: "alice"},
		},
		{
			Number:    1233,
			Title:     "Fix memory leak in parser",
			State:     "closed",
			CreatedAt: lastWeek,
			UpdatedAt: yesterday,
			ClosedAt:  &yesterday,
			MergedAt:  &merged,
			Author:    Author{Login: "bob"},
		},
		{
			Number:    1232,
			Title:     "Update documentation",
			State:     "open",
			CreatedAt: yesterday,
			UpdatedAt: yesterday,
			Author:    Author{Login: "charlie"},
		},
	}
}

// MockClientOption allows configuring the mock client
type MockClientOption func(*MockClient)

// WithPullRequests sets specific pull requests to return
func WithPullRequests(prs []PullRequest) MockClientOption {
	return func(m *MockClient) {
		m.PullRequests = prs
	}
}

// WithError makes the client return a specific error
func WithError(err error) MockClientOption {
	return func(m *MockClient) {
		m.Error = err
	}
}

// WithAuthFailure makes the client simulate authentication failure
func WithAuthFailure() MockClientOption {
	return func(m *MockClient) {
		m.ShouldFailAuth = true
	}
}

// NewMockClientWithOptions creates a mock client with options
func NewMockClientWithOptions(opts ...MockClientOption) *MockClient {
	mock := NewMockClient()
	for _, opt := range opts {
		opt(mock)
	}
	return mock
}
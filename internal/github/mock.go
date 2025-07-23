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
	ShouldFailAuth     bool
	ShouldFailNetwork  bool
	ShouldFailNotFound bool

	// Track calls for verification
	CallCount int
	LastOwner string
	LastRepo  string
	LastOpts  FetchOptions

	// Pagination support
	TotalPullRequests int
	PageSize          int
	SimulatePages     bool

	// Query complexity simulation
	ComplexityErrorOnCall int // Return complexity error on this call number (0 = never)
	CallsSinceComplexity  int // Track calls since last complexity error
}

// NewMockClient creates a new mock client with default test data
func NewMockClient() *MockClient {
	prs := generateTestPRs()
	return &MockClient{
		PullRequests:      prs,
		TotalPullRequests: len(prs),
		PageSize:          50,
	}
}

// GetRepositoryInfo implements the Client interface
func (m *MockClient) GetRepositoryInfo(ctx context.Context, owner, repo string) (*RepositoryInfo, error) {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Simulate error conditions
	if m.ShouldFailAuth {
		return nil, fmt.Errorf("authentication failed: %w", relaierrors.ErrInvalidToken)
	}

	if m.ShouldFailNetwork {
		return nil, fmt.Errorf("network timeout: %w", relaierrors.ErrNetworkFailure)
	}

	const nonexistentRepoName = "repo"
	if m.ShouldFailNotFound || (owner == "nonexistent" && repo == nonexistentRepoName) {
		return nil, fmt.Errorf("repository not found: %w", relaierrors.ErrRepoNotFound)
	}

	// Return configured error if set
	if m.Error != nil {
		return nil, m.Error
	}

	return &RepositoryInfo{
		TotalPullRequests: m.TotalPullRequests,
	}, nil
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

	// Check for query complexity error first (before other errors)
	if m.ComplexityErrorOnCall > 0 && m.CallCount == m.ComplexityErrorOnCall {
		m.CallsSinceComplexity = 0
		return nil, fmt.Errorf("GraphQL query complexity exceeded: %w", relaierrors.ErrQueryComplexity)
	}
	m.CallsSinceComplexity++

	// Check for other errors
	if err := m.checkErrors(owner, repo); err != nil {
		return nil, err
	}

	// Handle pagination if enabled
	if m.SimulatePages && m.PageSize > 0 {
		return m.getPaginatedPage(opts)
	}

	// Default behavior: return all PRs in one page
	return &PullRequestPage{
		PullRequests: m.PullRequests,
		HasNextPage:  false,
		EndCursor:    "",
	}, nil
}

func (m *MockClient) checkErrors(owner, repo string) error {
	// Simulate various error conditions
	if m.ShouldFailAuth {
		return fmt.Errorf("authentication failed: %w", relaierrors.ErrInvalidToken)
	}

	if m.ShouldFailNetwork {
		return fmt.Errorf("network timeout: %w", relaierrors.ErrNetworkFailure)
	}

	const nonexistentRepoName = "repo"
	if m.ShouldFailNotFound || (owner == "nonexistent" && repo == nonexistentRepoName) {
		return fmt.Errorf("repository not found: %w", relaierrors.ErrRepoNotFound)
	}

	// Return configured error if set
	return m.Error
}

// FetchPullRequestsSearch implements the Client interface
func (m *MockClient) FetchPullRequestsSearch(ctx context.Context, owner, repo string, opts FetchOptions) (*PullRequestPage, error) {
	// For the mock, just delegate to FetchPullRequests
	// In real implementation, this would use the search API with date filtering
	return m.FetchPullRequests(ctx, owner, repo, opts)
}

func (m *MockClient) getPaginatedPage(opts FetchOptions) (*PullRequestPage, error) {
	// Calculate pagination based on cursor
	startIdx := 0
	if opts.After != "" {
		// Simple cursor implementation: cursor is the index
		// Ignore error as we have a default startIdx of 0
		if n, err := fmt.Sscanf(opts.After, "cursor_%d", &startIdx); err != nil || n != 1 {
			startIdx = 0 // Use default on parse error
		}
	}

	pageSize := opts.PageSize
	if pageSize <= 0 {
		pageSize = m.PageSize
	}

	endIdx := startIdx + pageSize
	if endIdx > len(m.PullRequests) {
		endIdx = len(m.PullRequests)
	}

	prs := []PullRequest{}
	if startIdx < len(m.PullRequests) {
		prs = m.PullRequests[startIdx:endIdx]
	}

	hasNext := endIdx < len(m.PullRequests)
	cursor := ""
	if hasNext {
		cursor = fmt.Sprintf("cursor_%d", endIdx)
	}

	return &PullRequestPage{
		PullRequests: prs,
		HasNextPage:  hasNext,
		EndCursor:    cursor,
	}, nil
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
		m.TotalPullRequests = len(prs)
	}
}

// WithPagination enables pagination simulation with the given page size
func WithPagination(pageSize int) MockClientOption {
	return func(m *MockClient) {
		m.SimulatePages = true
		m.PageSize = pageSize
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

// WithComplexityError makes the client return a query complexity error on specific call
func WithComplexityError(callNumber int) MockClientOption {
	return func(m *MockClient) {
		m.ComplexityErrorOnCall = callNumber
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

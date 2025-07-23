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

	relaierrors "github.com/sirseerhq/sirseer-relay/internal/errors"
)

// Compile-time check that MockClient implements Client
var _ Client = (*MockClient)(nil)

func TestMockClient_FetchPullRequests(t *testing.T) {
	ctx := context.Background()

	t.Run("returns default test data", func(t *testing.T) {
		mock := NewMockClient()

		page, err := mock.FetchPullRequests(ctx, "test", "repo", FetchOptions{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(page.PullRequests) != 3 {
			t.Errorf("expected 3 PRs, got %d", len(page.PullRequests))
		}

		if page.HasNextPage {
			t.Error("expected HasNextPage to be false for Phase 1")
		}

		// Verify call tracking
		if mock.CallCount != 1 {
			t.Errorf("expected 1 call, got %d", mock.CallCount)
		}
		if mock.LastOwner != "test" {
			t.Errorf("expected owner 'test', got %q", mock.LastOwner)
		}
		if mock.LastRepo != "repo" {
			t.Errorf("expected repo 'repo', got %q", mock.LastRepo)
		}
	})

	t.Run("simulates auth failure", func(t *testing.T) {
		mock := NewMockClientWithOptions(WithAuthFailure())

		_, err := mock.FetchPullRequests(ctx, "test", "repo", FetchOptions{})
		if err == nil {
			t.Fatal("expected error, got nil")
		}

		if !errors.Is(err, relaierrors.ErrInvalidToken) {
			t.Errorf("expected ErrInvalidToken, got %v", err)
		}
	})

	t.Run("simulates network failure", func(t *testing.T) {
		mock := NewMockClient()
		mock.ShouldFailNetwork = true

		_, err := mock.FetchPullRequests(ctx, "test", "repo", FetchOptions{})
		if err == nil {
			t.Fatal("expected error, got nil")
		}

		if !errors.Is(err, relaierrors.ErrNetworkFailure) {
			t.Errorf("expected ErrNetworkFailure, got %v", err)
		}
	})

	t.Run("simulates repo not found", func(t *testing.T) {
		mock := NewMockClient()

		_, err := mock.FetchPullRequests(ctx, "nonexistent", "repo", FetchOptions{})
		if err == nil {
			t.Fatal("expected error, got nil")
		}

		if !errors.Is(err, relaierrors.ErrRepoNotFound) {
			t.Errorf("expected ErrRepoNotFound, got %v", err)
		}
	})

	t.Run("respects context cancellation", func(t *testing.T) {
		mock := NewMockClient()

		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		_, err := mock.FetchPullRequests(cancelCtx, "test", "repo", FetchOptions{})
		if err == nil {
			t.Fatal("expected error, got nil")
		}

		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	})

	t.Run("custom pull requests", func(t *testing.T) {
		customPRs := []PullRequest{
			{Number: 1, Title: "Custom PR", State: "open"},
		}

		mock := NewMockClientWithOptions(WithPullRequests(customPRs))

		page, err := mock.FetchPullRequests(ctx, "test", "repo", FetchOptions{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(page.PullRequests) != 1 {
			t.Errorf("expected 1 PR, got %d", len(page.PullRequests))
		}

		if page.PullRequests[0].Title != "Custom PR" {
			t.Errorf("expected title 'Custom PR', got %q", page.PullRequests[0].Title)
		}
	})
}

func TestMockClientOptions(t *testing.T) {
	t.Run("with custom error", func(t *testing.T) {
		customErr := errors.New("custom error")
		mock := NewMockClientWithOptions(WithError(customErr))

		_, err := mock.FetchPullRequests(context.Background(), "test", "repo", FetchOptions{})
		if err == nil {
			t.Fatal("expected error, got nil")
		}

		if !errors.Is(err, customErr) {
			t.Errorf("expected custom error, got %v", err)
		}
	})
}

func TestGenerateTestPRs(t *testing.T) {
	prs := generateTestPRs()

	if len(prs) != 3 {
		t.Fatalf("expected 3 test PRs, got %d", len(prs))
	}

	// Check first PR (open)
	if prs[0].State != "open" {
		t.Errorf("PR 0: expected state 'open', got %q", prs[0].State)
	}
	if prs[0].ClosedAt != nil {
		t.Error("PR 0: expected nil ClosedAt for open PR")
	}

	// Check second PR (closed/merged)
	if prs[1].State != "closed" {
		t.Errorf("PR 1: expected state 'closed', got %q", prs[1].State)
	}
	if prs[1].ClosedAt == nil {
		t.Error("PR 1: expected non-nil ClosedAt for closed PR")
	}
	if prs[1].MergedAt == nil {
		t.Error("PR 1: expected non-nil MergedAt for merged PR")
	}

	// Verify timestamps are reasonable
	now := time.Now()
	for i, pr := range prs {
		if pr.CreatedAt.After(now) {
			t.Errorf("PR %d: CreatedAt is in the future", i)
		}
		if pr.UpdatedAt.Before(pr.CreatedAt) {
			t.Errorf("PR %d: UpdatedAt is before CreatedAt", i)
		}
	}
}

func TestMockClient_Pagination(t *testing.T) {
	// Create test PRs for pagination
	testPRs := make([]PullRequest, 0, 150)
	for i := 1; i <= 150; i++ {
		testPRs = append(testPRs, PullRequest{
			Number:    i,
			Title:     fmt.Sprintf("PR %d", i),
			State:     "open",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Author:    Author{Login: "test"},
		})
	}

	tests := []struct {
		name          string
		pageSize      int
		totalPRs      []PullRequest
		expectedPages int
	}{
		{
			name:          "multiple full pages",
			pageSize:      50,
			totalPRs:      testPRs[:100], // Exactly 2 pages
			expectedPages: 2,
		},
		{
			name:          "last page partial",
			pageSize:      50,
			totalPRs:      testPRs[:75], // 1.5 pages
			expectedPages: 2,
		},
		{
			name:          "single page",
			pageSize:      50,
			totalPRs:      testPRs[:30], // Less than 1 page
			expectedPages: 1,
		},
		{
			name:          "small page size",
			pageSize:      10,
			totalPRs:      testPRs[:25], // 2.5 pages
			expectedPages: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockClientWithOptions(
				WithPullRequests(tt.totalPRs),
				WithPagination(tt.pageSize),
			)

			var allPRs []PullRequest
			cursor := ""
			pages := 0

			for {
				page, err := mock.FetchPullRequests(context.Background(), "test", "repo", FetchOptions{
					PageSize: tt.pageSize,
					After:    cursor,
				})
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				allPRs = append(allPRs, page.PullRequests...)
				pages++

				if !page.HasNextPage {
					break
				}
				cursor = page.EndCursor
			}

			// Verify we got all PRs
			if len(allPRs) != len(tt.totalPRs) {
				t.Errorf("got %d PRs, want %d", len(allPRs), len(tt.totalPRs))
			}

			// Verify page count
			if pages != tt.expectedPages {
				t.Errorf("got %d pages, want %d", pages, tt.expectedPages)
			}

			// Verify PRs are in order
			for i, pr := range allPRs {
				if pr.Number != tt.totalPRs[i].Number {
					t.Errorf("PR at index %d has number %d, want %d", i, pr.Number, tt.totalPRs[i].Number)
				}
			}
		})
	}
}

func TestMockClient_GetRepositoryInfo(t *testing.T) {
	tests := []struct {
		name      string
		mock      *MockClient
		wantTotal int
		wantErr   bool
	}{
		{
			name:      "default mock",
			mock:      NewMockClient(),
			wantTotal: 3, // Default has 3 PRs
			wantErr:   false,
		},
		{
			name: "custom total",
			mock: &MockClient{
				TotalPullRequests: 12345,
			},
			wantTotal: 12345,
			wantErr:   false,
		},
		{
			name: "auth failure",
			mock: &MockClient{
				ShouldFailAuth: true,
			},
			wantErr: true,
		},
		{
			name: "network failure",
			mock: &MockClient{
				ShouldFailNetwork: true,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := tt.mock.GetRepositoryInfo(context.Background(), "test", "repo")

			if (err != nil) != tt.wantErr {
				t.Errorf("GetRepositoryInfo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && info.TotalPullRequests != tt.wantTotal {
				t.Errorf("got total %d, want %d", info.TotalPullRequests, tt.wantTotal)
			}
		})
	}
}

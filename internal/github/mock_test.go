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
		
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately
		
		_, err := mock.FetchPullRequests(ctx, "test", "repo", FetchOptions{})
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
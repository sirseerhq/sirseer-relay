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
	"testing"
	"time"
)

// BenchmarkProcessPullRequests benchmarks the PR processing logic
func BenchmarkProcessPullRequests(b *testing.B) {
	// Create sample PRs of varying sizes
	benchmarks := []struct {
		name    string
		prCount int
	}{
		{"Small_10PRs", 10},
		{"Medium_100PRs", 100},
		{"Large_1000PRs", 1000},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			prs := generateMockPRs(bm.prCount)
			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				// Simulate processing PRs - since we're working with PullRequest structs
				// directly, we just access fields to simulate processing
				for _, pr := range prs {
					// Simulate accessing fields as would happen during NDJSON serialization
					_ = pr.Number
					_ = pr.Title
					_ = pr.State
					_ = pr.Author.Login
				}
			}
		})
	}
}

// BenchmarkPRStructCreation benchmarks creation of PR structs
func BenchmarkPRStructCreation(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = generateMockPR(i)
	}
}

// BenchmarkPullRequestPage benchmarks processing PullRequestPage structures
func BenchmarkPullRequestPage(b *testing.B) {
	// Simulate different page sizes
	benchmarks := []struct {
		name     string
		pageSize int
	}{
		{"Small_50PRs", 50},
		{"Medium_100PRs", 100},
		{"Large_500PRs", 500},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			page := generateMockPullRequestPage(bm.pageSize)
			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				// Simulate processing a page of PRs
				for _, pr := range page.PullRequests {
					// Simulate accessing fields as would happen during processing
					_ = pr.Number
					_ = pr.Title
					_ = pr.State
					_ = pr.Author.Login
				}
				_ = page.HasNextPage
				_ = page.EndCursor
			}
		})
	}
}

// Helper functions for generating mock data

func generateMockPRs(count int) []PullRequest {
	prs := make([]PullRequest, count)
	for i := 0; i < count; i++ {
		prs[i] = generateMockPR(i + 1)
	}
	return prs
}

func generateMockPR(num int) PullRequest {
	now := time.Now()
	closedAt := now.Add(-24 * time.Hour)
	mergedAt := now.Add(-23 * time.Hour)

	return PullRequest{
		Number:    num,
		Title:     "feat: implement new feature for enhanced performance",
		State:     "MERGED",
		CreatedAt: now.Add(-72 * time.Hour),
		UpdatedAt: now.Add(-24 * time.Hour),
		ClosedAt:  &closedAt,
		MergedAt:  &mergedAt,
		Author: User{
			Login: "developer123",
			Type:  "User",
		},
	}
}

func generateMockPullRequestPage(nodeCount int) *PullRequestPage {
	prs := make([]PullRequest, nodeCount)
	for i := 0; i < nodeCount; i++ {
		prs[i] = generateMockPR(i + 1)
	}

	return &PullRequestPage{
		PullRequests: prs,
		HasNextPage:  false,
		EndCursor:    "cursor_final",
	}
}

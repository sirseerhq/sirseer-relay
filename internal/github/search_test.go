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

func TestBuildSearchQuery(t *testing.T) {
	tests := []struct {
		name     string
		owner    string
		repo     string
		opts     FetchOptions
		expected string
	}{
		{
			name:     "basic query without dates",
			owner:    "kubernetes",
			repo:     "kubernetes",
			opts:     FetchOptions{},
			expected: "repo:kubernetes/kubernetes is:pr sort:created-asc",
		},
		{
			name:  "query with since date",
			owner: "kubernetes",
			repo:  "kubernetes",
			opts: FetchOptions{
				Since: timePtr(time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)),
			},
			expected: "repo:kubernetes/kubernetes is:pr created:>2024-01-15 sort:created-asc",
		},
		{
			name:  "query with until date",
			owner: "kubernetes",
			repo:  "kubernetes",
			opts: FetchOptions{
				Until: timePtr(time.Date(2024, 6, 30, 0, 0, 0, 0, time.UTC)),
			},
			expected: "repo:kubernetes/kubernetes is:pr created:<2024-06-30 sort:created-asc",
		},
		{
			name:  "query with date range",
			owner: "kubernetes",
			repo:  "kubernetes",
			opts: FetchOptions{
				Since: timePtr(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)),
				Until: timePtr(time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)),
			},
			expected: "repo:kubernetes/kubernetes is:pr created:2024-01-01..2024-12-31 sort:created-asc",
		},
		{
			name:  "custom query overrides everything",
			owner: "kubernetes",
			repo:  "kubernetes",
			opts: FetchOptions{
				Query: "custom search query",
				Since: timePtr(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)),
			},
			expected: "custom search query",
		},
		{
			name:     "org with special characters",
			owner:    "org-with-dash",
			repo:     "repo.with.dots",
			opts:     FetchOptions{},
			expected: "repo:org-with-dash/repo.with.dots is:pr sort:created-asc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildSearchQuery(tt.owner, tt.repo, tt.opts)
			if result != tt.expected {
				t.Errorf("buildSearchQuery() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// timePtr is a helper function to create a pointer to a time.Time
func timePtr(t time.Time) *time.Time {
	return &t
}

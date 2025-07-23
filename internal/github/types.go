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

// Package github provides types and interfaces for interacting with the GitHub API.
package github

import "time"

// PullRequest represents a GitHub pull request with essential metadata.
// This struct contains only the fields needed for Phase 1 implementation.
type PullRequest struct {
	Number    int        `json:"number"`
	Title     string     `json:"title"`
	State     string     `json:"state"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	ClosedAt  *time.Time `json:"closed_at,omitempty"`
	MergedAt  *time.Time `json:"merged_at,omitempty"`
	Author    Author     `json:"author"`
}

// Author represents the author of a pull request.
type Author struct {
	Login string `json:"login"`
}

// PullRequestPage represents a page of pull requests with pagination info.
type PullRequestPage struct {
	PullRequests []PullRequest
	HasNextPage  bool
	EndCursor    string
}

// FetchOptions configures how pull requests are fetched.
type FetchOptions struct {
	// PageSize controls how many PRs to fetch per page.
	// For Phase 1, this is not user-configurable.
	PageSize int

	// After is the cursor for pagination.
	// Empty string fetches from the beginning.
	After string
}

// Default values for fetch operations
const (
	defaultPageSize = 50
)

// RepositoryInfo contains basic repository metadata.
// Used to get total PR count for progress tracking.
type RepositoryInfo struct {
	TotalPullRequests int
}

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
// This is the core data structure that gets serialized to NDJSON output.
// It includes basic PR information while keeping memory usage minimal
// to support streaming large repositories.
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
// Currently only includes the login name to minimize GraphQL query complexity.
// Additional fields can be added in future releases if needed.
type Author struct {
	Login string `json:"login"`
}

// PullRequestPage represents a page of pull requests from a GraphQL query.
// It includes the pull requests for the current page and pagination information
// to support fetching subsequent pages. This enables efficient streaming
// without loading all PRs into memory at once.
type PullRequestPage struct {
	PullRequests []PullRequest
	HasNextPage  bool
	EndCursor    string
}

// FetchOptions configures how pull requests are fetched.
// It supports pagination through the After cursor field and
// allows customization of the page size for each request.
type FetchOptions struct {
	// PageSize controls how many PRs to fetch per page.
	// Defaults to 50 if not specified. Maximum is 100 per GitHub's API limits.
	PageSize int

	// After is the cursor for pagination.
	// Empty string fetches from the beginning.
	// Use PullRequestPage.EndCursor from previous response for next page.
	After string
}

// Default values for fetch operations
const (
	defaultPageSize = 50
)

// RepositoryInfo contains basic repository metadata.
// Used primarily to get the total PR count for accurate progress tracking
// and ETA calculation when fetching all pull requests with the --all flag.
type RepositoryInfo struct {
	TotalPullRequests int
}

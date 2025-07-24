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

// PullRequest represents a GitHub pull request with comprehensive metadata.
// This is the core data structure that gets serialized to NDJSON output.
// It includes all PR information needed for detailed analysis including
// files, reviews, commits, and timeline events.
type PullRequest struct {
	// Core identification and metadata
	Number    int        `json:"number"`
	Title     string     `json:"title"`
	State     string     `json:"state"`
	Body      string     `json:"body,omitempty"`
	URL       string     `json:"url"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	ClosedAt  *time.Time `json:"closed_at,omitempty"`
	MergedAt  *time.Time `json:"merged_at,omitempty"`

	// User relationships
	Author    User   `json:"author"`
	MergedBy  *User  `json:"merged_by,omitempty"`
	Assignees []User `json:"assignees,omitempty"`
	Reviewers []User `json:"reviewers,omitempty"`

	// Git references and commit information
	BaseRef        string `json:"base_ref"`
	HeadRef        string `json:"head_ref"`
	BaseSHA        string `json:"base_sha"`
	HeadSHA        string `json:"head_sha"`
	MergeCommitSHA string `json:"merge_commit_sha,omitempty"`

	// PR statistics
	Additions      int `json:"additions"`
	Deletions      int `json:"deletions"`
	ChangedFiles   int `json:"changed_files"`
	Comments       int `json:"comments"`
	ReviewComments int `json:"review_comments"`
	Commits        int `json:"commits"`

	// Status flags
	Merged    bool  `json:"merged"`
	Mergeable *bool `json:"mergeable,omitempty"`
	IsBot     bool  `json:"is_bot"`

	// Complex nested data
	Labels        []Label        `json:"labels,omitempty"`
	Files         []File         `json:"files,omitempty"`
	Reviews       []Review       `json:"reviews,omitempty"`
	CommitList    []Commit       `json:"commit_list,omitempty"`
	Conversations []Conversation `json:"conversations,omitempty"`
}

// User represents a GitHub user account.
// This can be a regular user, bot, or organization.
type User struct {
	Login string `json:"login"`
	Type  string `json:"type,omitempty"` // User, Bot, or Organization
	Email string `json:"email,omitempty"`
}

// Author represents the author of a pull request.
// Deprecated: Use User instead. Kept for backward compatibility.
type Author struct {
	Login string `json:"login"`
}

// Label represents a GitHub label applied to a pull request.
type Label struct {
	Name        string `json:"name"`
	Color       string `json:"color"`
	Description string `json:"description,omitempty"`
}

// File represents a file changed in a pull request.
type File struct {
	Filename  string `json:"filename"`
	Status    string `json:"status"` // added, removed, modified, renamed
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	Changes   int    `json:"changes"`
}

// Review represents a formal code review on a pull request.
type Review struct {
	ID          string     `json:"id"`
	User        User       `json:"user"`
	State       string     `json:"state"` // APPROVED, CHANGES_REQUESTED, COMMENTED, DISMISSED
	Body        string     `json:"body,omitempty"`
	SubmittedAt *time.Time `json:"submitted_at,omitempty"`
}

// Commit represents a git commit in a pull request.
type Commit struct {
	SHA          string    `json:"sha"`
	Message      string    `json:"message"`
	Author       User      `json:"author"`
	Committer    User      `json:"committer"`
	AuthoredAt   time.Time `json:"authored_at"`
	CommittedAt  time.Time `json:"committed_at"`
	Additions    int       `json:"additions"`
	Deletions    int       `json:"deletions"`
	TotalChanges int       `json:"total_changes"`
	Parents      []string  `json:"parents,omitempty"`
}

// Conversation represents a timeline event on a pull request.
// This includes comments, reviews, status updates, and other events.
type Conversation struct {
	Type      string    `json:"type"` // comment, review, event
	Username  string    `json:"username"`
	Timestamp time.Time `json:"timestamp"`
	Body      string    `json:"body,omitempty"`
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
// Time window filtering is supported through Since and Until fields.
type FetchOptions struct {
	// PageSize controls how many PRs to fetch per page.
	// Defaults to 50 if not specified. Maximum is 100 per GitHub's API limits.
	PageSize int

	// After is the cursor for pagination.
	// Empty string fetches from the beginning.
	// Use PullRequestPage.EndCursor from previous response for next page.
	After string

	// Since filters PRs created after this date (inclusive).
	// When nil, no lower bound is applied.
	Since *time.Time

	// Until filters PRs created before this date (inclusive).
	// When nil, no upper bound is applied.
	Until *time.Time

	// Query is the raw GitHub search query to use.
	// If provided, it overrides the default query construction.
	// This allows for advanced filtering beyond date ranges.
	Query string
}

// Default values for fetch operations
const (
	defaultPageSize    = 50
	minPageSize        = 5
	maxPageSize        = 100
	complexityPageSize = 10 // Start with smaller size for complex queries
)

// RepositoryInfo contains basic repository metadata.
// Used primarily to get the total PR count for accurate progress tracking
// and ETA calculation when fetching all pull requests with the --all flag.
type RepositoryInfo struct {
	TotalPullRequests int
}

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

import "context"

// Client defines the interface for interacting with GitHub's API.
// This interface allows for easy mocking in tests.
type Client interface {
	// FetchPullRequests retrieves a page of pull requests from the specified repository.
	// It supports cursor-based pagination through the opts.After parameter to fetch
	// subsequent pages. The page size can be configured via opts.PageSize.
	FetchPullRequests(ctx context.Context, owner, repo string, opts FetchOptions) (*PullRequestPage, error)

	// GetRepositoryInfo retrieves basic repository metadata including total PR count.
	// Used for progress tracking and ETA calculation.
	GetRepositoryInfo(ctx context.Context, owner, repo string) (*RepositoryInfo, error)
}

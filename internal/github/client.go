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
	// For Phase 1, this will fetch only the first page (up to 50 PRs).
	FetchPullRequests(ctx context.Context, owner, repo string, opts FetchOptions) (*PullRequestPage, error)
}

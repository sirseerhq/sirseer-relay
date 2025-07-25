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

// Package github provides a client for interacting with GitHub's GraphQL API
// to fetch pull request data. It abstracts the complexity of GraphQL queries
// and provides a simple interface for retrieving pull requests with support
// for pagination, error handling, and rate limiting.
//
// The package includes:
//   - A Client interface for fetching pull requests and repository information
//   - A GraphQL implementation using the shurcooL/graphql library
//   - Mock client for testing
//   - Type definitions for pull request data
//
// Basic usage:
//
//	client := github.NewGraphQLClient("your-github-token", "https://api.github.com/graphql")
//	page, err := client.FetchPullRequests(ctx, "golang", "go", github.FetchOptions{
//	    PageSize: 50,
//	})
//	if err != nil {
//	    // Handle error
//	}
//	for _, pr := range page.PullRequests {
//	    // Process pull request
//	}
package github

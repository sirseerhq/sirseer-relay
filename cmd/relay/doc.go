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

// Package main implements the sirseer-relay command-line interface.
// This tool fetches pull request data from GitHub repositories and
// outputs it in NDJSON format for efficient streaming and processing.
//
// The CLI supports:
//   - Fetching a single page of pull requests (default behavior)
//   - Fetching all pull requests with the --all flag
//   - Customizable output destinations (stdout or file)
//   - GitHub token authentication via flag or environment variable
//   - Graceful error handling with appropriate exit codes
//
// Usage:
//
//	sirseer-relay fetch <org>/<repo> [flags]
//
// Example:
//
//	export GITHUB_TOKEN=your_token
//	sirseer-relay fetch golang/go --output prs.ndjson
//
// Exit codes:
//   - 0: Success
//   - 1: General error
//   - 2: Authentication/authorization error
//   - 3: Network error
package main

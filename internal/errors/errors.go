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

// Package errors defines sentinel errors for consistent error handling across the application.
// These errors map to specific exit codes in the CLI for proper scripting support.
package errors

import "errors"

// Sentinel errors for consistent error handling and exit code mapping
var (
	// ErrInvalidToken indicates GitHub authentication failed.
	// Maps to exit code 2.
	ErrInvalidToken = errors.New("invalid github token")

	// ErrRepoNotFound indicates the specified repository does not exist or is not accessible.
	// Maps to exit code 2.
	ErrRepoNotFound = errors.New("repository not found")

	// ErrNetworkFailure indicates a network connection problem.
	// Maps to exit code 3.
	ErrNetworkFailure = errors.New("network connection failed")

	// ErrRateLimit indicates GitHub API rate limit has been exceeded.
	// Maps to exit code 2.
	ErrRateLimit = errors.New("github rate limit exceeded")
)

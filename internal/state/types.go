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

package state

import (
	"time"
)

// CurrentVersion is the current state schema version.
// Increment this when making breaking changes to the FetchState structure.
const CurrentVersion = 1

// FetchState represents the persistent state of a repository fetch operation.
// It tracks the progress of fetching pull requests to enable incremental updates.
// The state is designed to be forward-compatible through versioning and includes
// integrity validation through checksums.
type FetchState struct {
	// Version indicates the schema version of this state file.
	// Used to handle migrations and compatibility checks.
	Version int `json:"version"`

	// Checksum is the SHA256 hash of the state content (excluding this field).
	// Used to detect corruption or tampering.
	Checksum string `json:"checksum"`

	// Repository is the full repository name in "org/repo" format.
	// Example: "kubernetes/kubernetes"
	Repository string `json:"repository"`

	// LastFetchID is a unique identifier for the fetch operation.
	// Can be used for debugging and correlation.
	LastFetchID string `json:"last_fetch_id"`

	// LastPRNumber is the highest pull request number seen in the last fetch.
	// Used for deduplication in incremental fetches.
	LastPRNumber int `json:"last_pr_number"`

	// LastPRDate is the creation date of the newest PR fetched.
	// Used as the starting point for incremental fetches.
	LastPRDate time.Time `json:"last_pr_date"`

	// LastFetchTime records when the fetch operation completed successfully.
	// Useful for debugging and monitoring.
	LastFetchTime time.Time `json:"last_fetch_time"`

	// TotalFetched is the total number of PRs fetched in the last operation.
	// Provides insight into fetch size and performance.
	TotalFetched int `json:"total_fetched"`
}

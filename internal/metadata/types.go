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

// Package metadata types define the structures used for tracking and
// persisting information about fetch operations. These types capture
// comprehensive statistics and audit information for enterprise compliance.
package metadata

import (
	"time"
)

// FetchMetadata represents the complete metadata record for a single fetch
// operation. It captures all relevant information about what was fetched,
// how it was fetched, and the results. This structure is designed to provide
// a complete audit trail for enterprise compliance and troubleshooting.
type FetchMetadata struct {
	RelayVersion  string       `json:"relay_version"`
	MethodVersion string       `json:"method_version"`
	FetchID       string       `json:"fetch_id"`
	Parameters    FetchParams  `json:"parameters"`
	Results       FetchResults `json:"results"`
	Incremental   bool         `json:"incremental"`
	PreviousFetch *FetchRef    `json:"previous_fetch,omitempty"`
}

// FetchParams captures the input parameters used for a fetch operation.
// This includes the target repository, time windows for incremental fetches,
// and operational settings like batch size. These parameters are preserved
// to enable reproducible fetches and debugging.
type FetchParams struct {
	Organization string     `json:"organization"`
	Repository   string     `json:"repository"`
	Since        *time.Time `json:"since,omitempty"`
	Until        *time.Time `json:"until,omitempty"`
	FetchAll     bool       `json:"fetch_all"`
	BatchSize    int        `json:"batch_size"`
}

// FetchResults contains comprehensive statistics about a completed fetch
// operation. It tracks both quantitative metrics (PR counts, API calls)
// and temporal information (date ranges, duration). This data is essential
// for performance monitoring and troubleshooting.
type FetchResults struct {
	TotalPRs     int       `json:"total_prs"`
	FirstPR      int       `json:"first_pr_number"`
	LastPR       int       `json:"last_pr_number"`
	OldestPR     time.Time `json:"oldest_pr_date"`
	NewestPR     time.Time `json:"newest_pr_date"`
	Duration     string    `json:"fetch_duration"`
	APICallCount int       `json:"api_calls_made"`
	StartedAt    time.Time `json:"started_at"`
	CompletedAt  time.Time `json:"completed_at"`
}

// FetchRef provides a lightweight reference to a previous fetch operation,
// used to link incremental fetches to their predecessors. This creates an
// audit trail showing the chain of fetches for a repository.
type FetchRef struct {
	FetchID     string    `json:"fetch_id"`
	CompletedAt time.Time `json:"completed_at"`
}

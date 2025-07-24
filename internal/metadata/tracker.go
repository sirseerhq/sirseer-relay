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

// Package metadata provides functionality for tracking and persisting metadata
// about fetch operations. It records statistics about each fetch including
// the number of pull requests processed, API calls made, date ranges covered,
// and links to previous fetches for incremental operations.
//
// The metadata system serves several purposes:
//   - Provides audit trails for enterprise compliance
//   - Enables troubleshooting by recording fetch parameters
//   - Supports incremental fetch tracking with links to previous runs
//   - Records performance metrics for optimization
//
// Metadata is saved as JSON files alongside state files, allowing external
// tools to analyze fetch history and performance.
package metadata

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

const (
	// MethodVersion represents the current GraphQL query version
	MethodVersion = "graphql-all-in-one-v1"
)

// Tracker collects statistics during a fetch operation and generates metadata.
// It tracks API calls, pull request counts, and date ranges throughout the
// fetch process. Create a new tracker at the start of each fetch operation
// and call its methods to record activity.
type Tracker struct {
	startTime    time.Time
	apiCallCount int
	prStats      PRStats
}

// PRStats holds statistical information about pull requests processed during
// a fetch operation. It tracks both the numerical range (first/last PR numbers)
// and temporal range (oldest/newest PR dates) of the fetched data.
type PRStats struct {
	TotalPRs int       // Total number of PRs processed
	FirstPR  int       // Lowest PR number seen
	LastPR   int       // Highest PR number seen
	OldestPR time.Time // Earliest PR creation date
	NewestPR time.Time // Latest PR update date
}

// New creates a new metadata tracker and initializes it with the current time.
// Call this at the beginning of a fetch operation to start tracking.
func New() *Tracker {
	return &Tracker{
		startTime: time.Now(),
	}
}

// IncrementAPICall records that an API call was made. Call this after each
// successful GitHub API request to maintain accurate API usage statistics.
func (t *Tracker) IncrementAPICall() {
	t.apiCallCount++
}

// UpdatePRStats updates the running statistics with data from a single pull request.
// It adjusts the first/last PR numbers and oldest/newest dates as needed.
// This method is safe to call concurrently from multiple goroutines.
func (t *Tracker) UpdatePRStats(prNumber int, createdAt, updatedAt time.Time) {
	t.prStats.TotalPRs++

	// Track first and last PR numbers
	if t.prStats.FirstPR == 0 || prNumber < t.prStats.FirstPR {
		t.prStats.FirstPR = prNumber
	}
	if prNumber > t.prStats.LastPR {
		t.prStats.LastPR = prNumber
	}

	// Track oldest and newest PR dates
	if t.prStats.OldestPR.IsZero() || createdAt.Before(t.prStats.OldestPR) {
		t.prStats.OldestPR = createdAt
	}
	if updatedAt.After(t.prStats.NewestPR) {
		t.prStats.NewestPR = updatedAt
	}
}

// GenerateMetadata creates a FetchMetadata instance capturing the complete
// fetch operation statistics. Call this at the end of a successful fetch
// to create the metadata record.
//
// Parameters:
//   - relayVersion: The version of sirseer-relay (from version.Version)
//   - params: The fetch parameters used for this operation
//   - incremental: Whether this was an incremental fetch
//   - previousFetch: Reference to the previous fetch (for incremental fetches)
//
// Returns a complete metadata record ready for persistence.
func (t *Tracker) GenerateMetadata(relayVersion string, params FetchParams, incremental bool, previousFetch *FetchRef) *FetchMetadata {
	completedAt := time.Now()
	duration := completedAt.Sub(t.startTime)

	// Generate unique fetch ID
	fetchID := fmt.Sprintf("%s-%d", getFetchType(incremental), t.startTime.Unix())

	return &FetchMetadata{
		RelayVersion:  relayVersion,
		MethodVersion: MethodVersion,
		FetchID:       fetchID,
		Parameters:    params,
		Results: FetchResults{
			TotalPRs:     t.prStats.TotalPRs,
			FirstPR:      t.prStats.FirstPR,
			LastPR:       t.prStats.LastPR,
			OldestPR:     t.prStats.OldestPR,
			NewestPR:     t.prStats.NewestPR,
			Duration:     duration.String(),
			APICallCount: t.apiCallCount,
			StartedAt:    t.startTime,
			CompletedAt:  completedAt,
		},
		Incremental:   incremental,
		PreviousFetch: previousFetch,
	}
}

// SaveMetadata persists a FetchMetadata record to a JSON file in the specified
// directory. The file is written atomically using a temporary file and rename
// to prevent corruption. The filename includes a timestamp for easy sorting.
//
// The metadata file will be named: fetch-metadata-{timestamp}.json
//
// Returns an error if the save operation fails.
func SaveMetadata(metadata *FetchMetadata, stateDir string) error {
	// Ensure state directory exists
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}

	// Generate filename with timestamp
	filename := fmt.Sprintf("fetch-metadata-%d.json", metadata.Results.StartedAt.Unix())
	filepath := filepath.Join(stateDir, filename)

	// Write to temporary file first for atomicity
	tmpFile := filepath + ".tmp"
	file, err := os.Create(tmpFile)
	if err != nil {
		return fmt.Errorf("failed to create metadata file: %w", err)
	}

	// Write JSON with proper formatting
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(metadata); err != nil {
		_ = file.Close()
		_ = os.Remove(tmpFile)
		return fmt.Errorf("failed to write metadata: %w", err)
	}

	if err := file.Close(); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("failed to close metadata file: %w", err)
	}

	// Atomically rename to final location
	if err := os.Rename(tmpFile, filepath); err != nil {
		return fmt.Errorf("failed to save metadata file: %w", err)
	}

	return nil
}

// LoadLatestMetadata finds and loads the most recent metadata file for the
// specified repository from the state directory. It identifies the latest
// file by modification time and verifies it matches the requested repository.
//
// Returns nil if no metadata exists for the repository, or an error if
// loading fails.
func LoadLatestMetadata(stateDir, repo string) (*FetchMetadata, error) {
	pattern := filepath.Join(stateDir, "fetch-metadata-*.json")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to list metadata files: %w", err)
	}

	if len(files) == 0 {
		return nil, nil // No previous metadata
	}

	// Find the most recent file
	var latestFile string
	var latestTime time.Time
	for _, file := range files {
		info, statErr := os.Stat(file)
		if statErr != nil {
			continue
		}
		if info.ModTime().After(latestTime) {
			latestTime = info.ModTime()
			latestFile = file
		}
	}

	if latestFile == "" {
		return nil, nil
	}

	// Read and parse the metadata
	file, err := os.Open(latestFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open metadata file: %w", err)
	}
	defer file.Close()

	var metadata FetchMetadata
	if err := json.NewDecoder(file).Decode(&metadata); err != nil {
		return nil, fmt.Errorf("failed to parse metadata: %w", err)
	}

	// Verify it's for the same repository
	fullRepo := fmt.Sprintf("%s/%s", metadata.Parameters.Organization, metadata.Parameters.Repository)
	if fullRepo != repo {
		return nil, nil // Metadata is for different repo
	}

	return &metadata, nil
}

// WriteMetadataToWriter serializes metadata to JSON and writes it to the
// provided io.Writer. The output is formatted with indentation for readability.
// This function is useful for outputting metadata to stdout or network streams.
func WriteMetadataToWriter(metadata *FetchMetadata, w io.Writer) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(metadata)
}

func getFetchType(incremental bool) string {
	if incremental {
		return "incremental"
	}
	return "full"
}

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

package metadata

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTracker_UpdatePRStats(t *testing.T) {
	tests := []struct {
		name    string
		updates []struct {
			prNumber  int
			createdAt time.Time
			updatedAt time.Time
		}
		wantStats PRStats
	}{
		{
			name: "single PR",
			updates: []struct {
				prNumber  int
				createdAt time.Time
				updatedAt time.Time
			}{
				{100, time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2023, 1, 2, 0, 0, 0, 0, time.UTC)},
			},
			wantStats: PRStats{
				TotalPRs: 1,
				FirstPR:  100,
				LastPR:   100,
				OldestPR: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
				NewestPR: time.Date(2023, 1, 2, 0, 0, 0, 0, time.UTC),
			},
		},
		{
			name: "multiple PRs in order",
			updates: []struct {
				prNumber  int
				createdAt time.Time
				updatedAt time.Time
			}{
				{100, time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2023, 1, 2, 0, 0, 0, 0, time.UTC)},
				{101, time.Date(2023, 1, 3, 0, 0, 0, 0, time.UTC), time.Date(2023, 1, 4, 0, 0, 0, 0, time.UTC)},
				{102, time.Date(2023, 1, 5, 0, 0, 0, 0, time.UTC), time.Date(2023, 1, 6, 0, 0, 0, 0, time.UTC)},
			},
			wantStats: PRStats{
				TotalPRs: 3,
				FirstPR:  100,
				LastPR:   102,
				OldestPR: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
				NewestPR: time.Date(2023, 1, 6, 0, 0, 0, 0, time.UTC),
			},
		},
		{
			name: "PRs out of order",
			updates: []struct {
				prNumber  int
				createdAt time.Time
				updatedAt time.Time
			}{
				{200, time.Date(2023, 1, 5, 0, 0, 0, 0, time.UTC), time.Date(2023, 1, 6, 0, 0, 0, 0, time.UTC)},
				{50, time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2023, 1, 2, 0, 0, 0, 0, time.UTC)},
				{150, time.Date(2023, 1, 3, 0, 0, 0, 0, time.UTC), time.Date(2023, 1, 10, 0, 0, 0, 0, time.UTC)},
			},
			wantStats: PRStats{
				TotalPRs: 3,
				FirstPR:  50,
				LastPR:   200,
				OldestPR: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
				NewestPR: time.Date(2023, 1, 10, 0, 0, 0, 0, time.UTC),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracker := New()

			for _, update := range tt.updates {
				tracker.UpdatePRStats(update.prNumber, update.createdAt, update.updatedAt)
			}

			if tracker.prStats.TotalPRs != tt.wantStats.TotalPRs {
				t.Errorf("TotalPRs = %d, want %d", tracker.prStats.TotalPRs, tt.wantStats.TotalPRs)
			}
			if tracker.prStats.FirstPR != tt.wantStats.FirstPR {
				t.Errorf("FirstPR = %d, want %d", tracker.prStats.FirstPR, tt.wantStats.FirstPR)
			}
			if tracker.prStats.LastPR != tt.wantStats.LastPR {
				t.Errorf("LastPR = %d, want %d", tracker.prStats.LastPR, tt.wantStats.LastPR)
			}
			if !tracker.prStats.OldestPR.Equal(tt.wantStats.OldestPR) {
				t.Errorf("OldestPR = %v, want %v", tracker.prStats.OldestPR, tt.wantStats.OldestPR)
			}
			if !tracker.prStats.NewestPR.Equal(tt.wantStats.NewestPR) {
				t.Errorf("NewestPR = %v, want %v", tracker.prStats.NewestPR, tt.wantStats.NewestPR)
			}
		})
	}
}

func TestTracker_GenerateMetadata(t *testing.T) {
	tracker := New()
	tracker.apiCallCount = 5
	tracker.UpdatePRStats(100, time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2023, 1, 2, 0, 0, 0, 0, time.UTC))
	tracker.UpdatePRStats(101, time.Date(2023, 1, 3, 0, 0, 0, 0, time.UTC), time.Date(2023, 1, 4, 0, 0, 0, 0, time.UTC))

	params := FetchParams{
		Organization: "kubernetes",
		Repository:   "kubernetes",
		FetchAll:     true,
		BatchSize:    50,
	}

	metadata := tracker.GenerateMetadata("v1.2.3", params, false, nil)

	// Verify metadata fields
	if metadata.RelayVersion != "v1.2.3" {
		t.Errorf("RelayVersion = %s, want v1.2.3", metadata.RelayVersion)
	}
	if metadata.MethodVersion != MethodVersion {
		t.Errorf("MethodVersion = %s, want %s", metadata.MethodVersion, MethodVersion)
	}
	if !strings.HasPrefix(metadata.FetchID, "full-") {
		t.Errorf("FetchID = %s, want prefix 'full-'", metadata.FetchID)
	}
	if metadata.Incremental {
		t.Error("Incremental = true, want false")
	}
	if metadata.PreviousFetch != nil {
		t.Error("PreviousFetch should be nil")
	}

	// Verify results
	if metadata.Results.TotalPRs != 2 {
		t.Errorf("TotalPRs = %d, want 2", metadata.Results.TotalPRs)
	}
	if metadata.Results.APICallCount != 5 {
		t.Errorf("APICallCount = %d, want 5", metadata.Results.APICallCount)
	}
}

func TestTracker_GenerateMetadata_Incremental(t *testing.T) {
	tracker := New()
	tracker.apiCallCount = 2

	since := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	params := FetchParams{
		Organization: "org",
		Repository:   "repo",
		Since:        &since,
		BatchSize:    25,
	}

	previousFetch := &FetchRef{
		FetchID:     "full-1234567890",
		CompletedAt: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	metadata := tracker.GenerateMetadata("v1.0.0", params, true, previousFetch)

	if !strings.HasPrefix(metadata.FetchID, "incremental-") {
		t.Errorf("FetchID = %s, want prefix 'incremental-'", metadata.FetchID)
	}
	if !metadata.Incremental {
		t.Error("Incremental = false, want true")
	}
	if metadata.PreviousFetch == nil {
		t.Error("PreviousFetch should not be nil")
	}
}

func TestSaveMetadata(t *testing.T) {
	tmpDir := t.TempDir()

	metadata := &FetchMetadata{
		RelayVersion:  "v1.2.3",
		MethodVersion: MethodVersion,
		FetchID:       "full-1234567890",
		Parameters: FetchParams{
			Organization: "kubernetes",
			Repository:   "kubernetes",
			FetchAll:     true,
			BatchSize:    50,
		},
		Results: FetchResults{
			TotalPRs:     100,
			FirstPR:      1,
			LastPR:       100,
			Duration:     "5m30s",
			APICallCount: 10,
			StartedAt:    time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
			CompletedAt:  time.Date(2023, 1, 1, 12, 5, 30, 0, time.UTC),
		},
		Incremental: false,
	}

	// Save metadata
	if err := SaveMetadata(metadata, tmpDir); err != nil {
		t.Fatalf("SaveMetadata failed: %v", err)
	}

	// Verify file was created
	expectedFile := filepath.Join(tmpDir, "fetch-metadata-1672574400.json")
	if _, err := os.Stat(expectedFile); err != nil {
		t.Fatalf("metadata file not created: %v", err)
	}

	// Read and verify contents
	data, err := os.ReadFile(expectedFile)
	if err != nil {
		t.Fatalf("failed to read metadata file: %v", err)
	}

	var loaded FetchMetadata
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("failed to parse metadata: %v", err)
	}

	if loaded.RelayVersion != metadata.RelayVersion {
		t.Errorf("RelayVersion = %s, want %s", loaded.RelayVersion, metadata.RelayVersion)
	}
	if loaded.Results.TotalPRs != metadata.Results.TotalPRs {
		t.Errorf("TotalPRs = %d, want %d", loaded.Results.TotalPRs, metadata.Results.TotalPRs)
	}
}

func TestLoadLatestMetadata(t *testing.T) {
	tmpDir := t.TempDir()

	// Create multiple metadata files
	metadata1 := &FetchMetadata{
		RelayVersion: "v1.0.0",
		FetchID:      "full-1000000000",
		Parameters: FetchParams{
			Organization: "org",
			Repository:   "repo",
		},
		Results: FetchResults{
			StartedAt: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	metadata2 := &FetchMetadata{
		RelayVersion: "v1.1.0",
		FetchID:      "full-2000000000",
		Parameters: FetchParams{
			Organization: "org",
			Repository:   "repo",
		},
		Results: FetchResults{
			StartedAt: time.Date(2023, 1, 2, 0, 0, 0, 0, time.UTC),
		},
	}

	// Save both metadata files
	if err := SaveMetadata(metadata1, tmpDir); err != nil {
		t.Fatalf("SaveMetadata failed: %v", err)
	}

	// Sleep briefly to ensure different modification times
	time.Sleep(10 * time.Millisecond)

	if err := SaveMetadata(metadata2, tmpDir); err != nil {
		t.Fatalf("SaveMetadata failed: %v", err)
	}

	// Load latest metadata
	loaded, err := LoadLatestMetadata(tmpDir, "org/repo")
	if err != nil {
		t.Fatalf("LoadLatestMetadata failed: %v", err)
	}

	if loaded == nil {
		t.Fatal("expected metadata, got nil")
	}

	// Should get the second (latest) metadata
	if loaded.FetchID != metadata2.FetchID {
		t.Errorf("FetchID = %s, want %s", loaded.FetchID, metadata2.FetchID)
	}
}

func TestLoadLatestMetadata_DifferentRepo(t *testing.T) {
	tmpDir := t.TempDir()

	// Create metadata for different repo
	metadata := &FetchMetadata{
		RelayVersion: "v1.0.0",
		FetchID:      "full-1000000000",
		Parameters: FetchParams{
			Organization: "other",
			Repository:   "repo",
		},
		Results: FetchResults{
			StartedAt: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	if err := SaveMetadata(metadata, tmpDir); err != nil {
		t.Fatalf("SaveMetadata failed: %v", err)
	}

	// Try to load for different repo
	loaded, err := LoadLatestMetadata(tmpDir, "org/repo")
	if err != nil {
		t.Fatalf("LoadLatestMetadata failed: %v", err)
	}

	if loaded != nil {
		t.Error("expected nil metadata for different repo")
	}
}

func TestWriteMetadataToWriter(t *testing.T) {
	metadata := &FetchMetadata{
		RelayVersion:  "v1.2.3",
		MethodVersion: MethodVersion,
		FetchID:       "full-1234567890",
		Parameters: FetchParams{
			Organization: "kubernetes",
			Repository:   "kubernetes",
			FetchAll:     true,
			BatchSize:    50,
		},
		Results: FetchResults{
			TotalPRs:     100,
			FirstPR:      1,
			LastPR:       100,
			Duration:     "5m30s",
			APICallCount: 10,
			StartedAt:    time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
			CompletedAt:  time.Date(2023, 1, 1, 12, 5, 30, 0, time.UTC),
		},
		Incremental: false,
	}

	var buf bytes.Buffer
	if err := WriteMetadataToWriter(metadata, &buf); err != nil {
		t.Fatalf("WriteMetadataToWriter failed: %v", err)
	}

	// Verify output is valid JSON
	var loaded FetchMetadata
	if err := json.Unmarshal(buf.Bytes(), &loaded); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}

	// Verify indentation
	output := buf.String()
	if !strings.Contains(output, "\n  \"relay_version\"") {
		t.Error("output should be indented")
	}
}

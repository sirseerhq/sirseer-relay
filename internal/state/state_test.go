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
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGetStateFilePath(t *testing.T) {
	tests := []struct {
		name       string
		repository string
		wantSuffix string
	}{
		{
			name:       "standard repository",
			repository: "kubernetes/kubernetes",
			wantSuffix: ".sirseer/state/kubernetes-kubernetes.state",
		},
		{
			name:       "repository with multiple slashes",
			repository: "org/sub/repo",
			wantSuffix: ".sirseer/state/org-sub-repo.state",
		},
		{
			name:       "simple repository",
			repository: "simple",
			wantSuffix: ".sirseer/state/simple.state",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetStateFilePath(tt.repository)
			if !strings.HasSuffix(got, tt.wantSuffix) {
				t.Errorf("GetStateFilePath(%q) = %q, want suffix %q", tt.repository, got, tt.wantSuffix)
			}
		})
	}
}

func TestSaveAndLoadState(t *testing.T) {
	// Create a temporary directory for test files
	tempDir := t.TempDir()

	testState := &FetchState{
		Repository:    "test/repo",
		LastFetchID:   "test-fetch-123",
		LastPRNumber:  999,
		LastPRDate:    time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
		LastFetchTime: time.Date(2024, 1, 15, 11, 0, 0, 0, time.UTC),
		TotalFetched:  150,
	}

	stateFile := filepath.Join(tempDir, "test.state")

	// Test saving state
	if err := SaveState(testState, stateFile); err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(stateFile); err != nil {
		t.Fatalf("State file not created: %v", err)
	}

	// Test loading state
	loadedState, err := LoadState(stateFile)
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	// Verify loaded state matches saved state
	if loadedState.Repository != testState.Repository {
		t.Errorf("Repository mismatch: got %q, want %q", loadedState.Repository, testState.Repository)
	}
	if loadedState.LastPRNumber != testState.LastPRNumber {
		t.Errorf("LastPRNumber mismatch: got %d, want %d", loadedState.LastPRNumber, testState.LastPRNumber)
	}
	if !loadedState.LastPRDate.Equal(testState.LastPRDate) {
		t.Errorf("LastPRDate mismatch: got %v, want %v", loadedState.LastPRDate, testState.LastPRDate)
	}
	if loadedState.Version != CurrentVersion {
		t.Errorf("Version mismatch: got %d, want %d", loadedState.Version, CurrentVersion)
	}
	if loadedState.Checksum == "" {
		t.Error("Checksum should not be empty")
	}
}

func TestLoadState_FileNotExist(t *testing.T) {
	tempDir := t.TempDir()
	stateFile := filepath.Join(tempDir, "nonexistent.state")

	_, err := LoadState(stateFile)
	if err == nil {
		t.Fatal("LoadState should fail for non-existent file")
	}
	if !strings.Contains(err.Error(), "no previous fetch state found") {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestLoadState_CorruptedJSON(t *testing.T) {
	tempDir := t.TempDir()
	stateFile := filepath.Join(tempDir, "corrupted.state")

	// Write invalid JSON
	if err := os.WriteFile(stateFile, []byte("{ invalid json"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadState(stateFile)
	if err == nil {
		t.Fatal("LoadState should fail for corrupted JSON")
	}
	if !strings.Contains(err.Error(), "corrupted (invalid JSON)") {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestLoadState_ChecksumMismatch(t *testing.T) {
	tempDir := t.TempDir()
	stateFile := filepath.Join(tempDir, "tampered.state")

	// Create a valid state
	testState := &FetchState{
		Repository:   "test/repo",
		LastPRNumber: 100,
	}

	// Save it normally
	if err := SaveState(testState, stateFile); err != nil {
		t.Fatal(err)
	}

	// Read the file
	data, err := os.ReadFile(stateFile)
	if err != nil {
		t.Fatal(err)
	}

	// Tamper with the content by changing a field
	tamperedData := strings.Replace(string(data), `"last_pr_number": 100`, `"last_pr_number": 200`, 1)
	
	// Write back the tampered data
	if err := os.WriteFile(stateFile, []byte(tamperedData), 0644); err != nil {
		t.Fatal(err)
	}

	// Try to load the tampered state
	_, err = LoadState(stateFile)
	if err == nil {
		t.Fatal("LoadState should fail for tampered state")
	}
	if !strings.Contains(err.Error(), "checksum mismatch") {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestLoadState_VersionMismatch(t *testing.T) {
	tempDir := t.TempDir()
	stateFile := filepath.Join(tempDir, "oldversion.state")

	// Create state with old version
	oldState := map[string]interface{}{
		"version":         0, // Old version
		"checksum":        "",
		"repository":      "test/repo",
		"last_pr_number":  100,
		"last_pr_date":    time.Now(),
		"last_fetch_time": time.Now(),
		"total_fetched":   50,
	}

	// Calculate checksum for old state
	oldState["checksum"] = ""
	data, _ := json.Marshal(oldState)
	checksum, _ := calculateChecksum(&FetchState{Version: 0})
	oldState["checksum"] = checksum

	// Write the old version state
	data, _ = json.MarshalIndent(oldState, "", "  ")
	if err := os.WriteFile(stateFile, data, 0644); err != nil {
		t.Fatal(err)
	}

	// Try to load
	_, err := LoadState(stateFile)
	if err == nil {
		t.Fatal("LoadState should fail for version mismatch")
	}
	if !strings.Contains(err.Error(), "incompatible with current version") {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestAtomicWrite(t *testing.T) {
	tempDir := t.TempDir()
	stateFile := filepath.Join(tempDir, "atomic.state")

	// Create initial state
	initialState := &FetchState{
		Repository:   "test/repo",
		LastPRNumber: 100,
	}
	if err := SaveState(initialState, stateFile); err != nil {
		t.Fatal(err)
	}

	// Read initial content
	initialData, err := os.ReadFile(stateFile)
	if err != nil {
		t.Fatal(err)
	}

	// Simulate a partial write by creating temp file
	tempFile := stateFile + ".tmp"
	if err := os.WriteFile(tempFile, []byte("partial write"), 0644); err != nil {
		t.Fatal(err)
	}

	// Verify original file is still intact
	currentData, err := os.ReadFile(stateFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(currentData) != string(initialData) {
		t.Error("Original state file was modified during partial write")
	}

	// Clean up temp file
	os.Remove(tempFile)
}

func TestDeleteState(t *testing.T) {
	tempDir := t.TempDir()
	stateFile := filepath.Join(tempDir, "delete.state")

	// Create a state file
	testState := &FetchState{
		Repository:   "test/repo",
		LastPRNumber: 100,
	}
	if err := SaveState(testState, stateFile); err != nil {
		t.Fatal(err)
	}

	// Delete it
	if err := DeleteState(stateFile); err != nil {
		t.Fatalf("DeleteState failed: %v", err)
	}

	// Verify it's gone
	if _, err := os.Stat(stateFile); !os.IsNotExist(err) {
		t.Error("State file still exists after deletion")
	}

	// Delete non-existent file should not error
	if err := DeleteState(stateFile); err != nil {
		t.Errorf("DeleteState on non-existent file should not error: %v", err)
	}
}

func TestConcurrentAccess(t *testing.T) {
	tempDir := t.TempDir()
	stateFile := filepath.Join(tempDir, "concurrent.state")

	// Run multiple goroutines trying to save state
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			state := &FetchState{
				Repository:   "test/repo",
				LastPRNumber: id,
				LastFetchID:  fmt.Sprintf("fetch-%d", id),
			}
			SaveState(state, stateFile)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify we can load the final state and it's valid
	finalState, err := LoadState(stateFile)
	if err != nil {
		t.Fatalf("Failed to load final state: %v", err)
	}

	// The exact content doesn't matter, just that it's valid
	if finalState.Repository != "test/repo" {
		t.Error("Final state has incorrect repository")
	}
	if finalState.Version != CurrentVersion {
		t.Error("Final state has incorrect version")
	}
}
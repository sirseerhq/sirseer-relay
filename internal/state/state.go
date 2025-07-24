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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// GetStateFilePath returns the standard path for a repository's state file.
// Repository should be in "org/repo" format.
// Returns: ~/.sirseer/state/org-repo.state
func GetStateFilePath(repository string) string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// Fallback to current directory if home directory is not accessible
		homeDir = "."
	}

	// Replace slashes with dashes for filesystem compatibility
	safeRepoName := strings.ReplaceAll(repository, "/", "-")

	return filepath.Join(homeDir, ".sirseer", "state", safeRepoName+".state")
}

// SaveState atomically saves the fetch state to disk with integrity validation.
// It uses a write-to-temp-and-rename pattern to ensure atomicity.
// The checksum is calculated and stored to detect corruption.
func SaveState(state *FetchState, stateFile string) error {
	// Set version to current
	state.Version = CurrentVersion

	// Calculate checksum before adding it to the struct
	checksum, err := calculateChecksum(state)
	if err != nil {
		return fmt.Errorf("failed to calculate checksum: %w", err)
	}
	state.Checksum = checksum

	// Ensure the directory exists
	stateDir := filepath.Dir(stateFile)
	if mkdirErr := os.MkdirAll(stateDir, 0o755); mkdirErr != nil {
		return fmt.Errorf("failed to create state directory: %w", mkdirErr)
	}

	// Create a temporary file in the same directory
	tempFile := stateFile + ".tmp"

	// Marshal state to compact JSON for efficiency
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	// Write to temporary file with restricted permissions
	if writeErr := os.WriteFile(tempFile, data, 0o600); writeErr != nil {
		return fmt.Errorf("failed to write temporary state file: %w", writeErr)
	}

	// Sync to ensure data is flushed to disk
	file, err := os.Open(tempFile)
	if err != nil {
		// Clean up temp file
		_ = os.Remove(tempFile)
		return fmt.Errorf("failed to open temp file for sync: %w", err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		_ = os.Remove(tempFile)
		return fmt.Errorf("failed to sync temp file: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(tempFile)
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tempFile, stateFile); err != nil {
		// Clean up temp file
		_ = os.Remove(tempFile)
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// LoadState reads and validates the fetch state from disk.
// It verifies the checksum and version compatibility.
func LoadState(stateFile string) (*FetchState, error) {
	// Read the state file
	data, err := os.ReadFile(stateFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no previous fetch state found at %s. Use --all flag for initial fetch", stateFile)
		}
		return nil, fmt.Errorf("failed to read state file %s: %w", stateFile, err)
	}

	// Unmarshal the state
	var state FetchState
	if unmarshalErr := json.Unmarshal(data, &state); unmarshalErr != nil {
		return nil, fmt.Errorf("state file is corrupted (invalid JSON): %w", unmarshalErr)
	}

	// Check version compatibility
	if state.Version != CurrentVersion {
		return nil, fmt.Errorf("state file version (%d) is incompatible with current version (%d)",
			state.Version, CurrentVersion)
	}

	// Verify checksum
	savedChecksum := state.Checksum
	state.Checksum = "" // Clear for recalculation

	calculatedChecksum, err := calculateChecksum(&state)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate checksum for validation: %w", err)
	}

	if savedChecksum != calculatedChecksum {
		return nil, fmt.Errorf("state file is corrupted (checksum mismatch)")
	}

	// Restore the checksum field
	state.Checksum = savedChecksum

	return &state, nil
}

// DeleteState removes the state file for a repository.
// This is useful for resetting to a clean state.
func DeleteState(stateFile string) error {
	err := os.Remove(stateFile)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete state file: %w", err)
	}
	return nil
}

// calculateChecksum computes the SHA256 hash of the state content.
// The checksum field itself is excluded from the calculation.
func calculateChecksum(state *FetchState) (string, error) {
	// Create a copy without the checksum field
	stateCopy := *state
	stateCopy.Checksum = ""

	// Marshal to JSON for consistent hashing
	data, err := json.Marshal(stateCopy)
	if err != nil {
		return "", err
	}

	// Calculate SHA256
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}

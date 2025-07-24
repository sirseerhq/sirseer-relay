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

package integration

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sirseerhq/sirseer-relay/internal/metadata"
)

func TestMetadataGeneration_BasicFetch(t *testing.T) {
	if os.Getenv("GITHUB_TOKEN") == "" {
		t.Skip("Skipping test: GITHUB_TOKEN not set")
	}

	binaryPath := buildBinary(t)
	tmpDir := t.TempDir()

	// Run a small fetch that should generate metadata
	outputFile := filepath.Join(tmpDir, "test.ndjson")
	metadataFile := filepath.Join(tmpDir, "fetch-metadata.json")

	cmd := exec.Command(binaryPath, "fetch", "golang/mock",
		"--output", outputFile,
		"--metadata-file", metadataFile)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		t.Fatalf("Fetch failed: %v\nStderr: %s", err, stderr.String())
	}

	// Verify metadata file was created
	if _, statErr := os.Stat(metadataFile); os.IsNotExist(statErr) {
		t.Fatal("Metadata file was not created")
	}

	// Load and validate metadata
	data, err := os.ReadFile(metadataFile)
	if err != nil {
		t.Fatalf("Failed to read metadata file: %v", err)
	}

	var fetchMetadata metadata.FetchMetadata
	if err := json.Unmarshal(data, &fetchMetadata); err != nil {
		t.Fatalf("Failed to parse metadata: %v", err)
	}

	// Validate metadata contents
	if fetchMetadata.RelayVersion == "" {
		t.Error("Missing relay version in metadata")
	}

	if fetchMetadata.MethodVersion != "graphql-all-in-one-v1" {
		t.Errorf("Unexpected method version: %s", fetchMetadata.MethodVersion)
	}

	if fetchMetadata.FetchID == "" {
		t.Error("Missing fetch ID")
	}

	if fetchMetadata.Parameters.Organization != "golang" {
		t.Errorf("Wrong organization: %s", fetchMetadata.Parameters.Organization)
	}

	if fetchMetadata.Parameters.Repository != "mock" {
		t.Errorf("Wrong repository: %s", fetchMetadata.Parameters.Repository)
	}

	if fetchMetadata.Results.TotalPRs == 0 {
		t.Error("No PRs recorded in metadata")
	}

	if fetchMetadata.Results.APICallCount == 0 {
		t.Error("No API calls recorded in metadata")
	}

	if fetchMetadata.Results.Duration == "" {
		t.Error("Missing duration in metadata")
	}

	if fetchMetadata.Incremental {
		t.Error("Expected non-incremental fetch")
	}

	if fetchMetadata.PreviousFetch != nil {
		t.Error("Expected no previous fetch reference")
	}
}

func TestMetadataGeneration_DefaultLocation(t *testing.T) {
	if os.Getenv("GITHUB_TOKEN") == "" {
		t.Skip("Skipping test: GITHUB_TOKEN not set")
	}

	binaryPath := buildBinary(t)
	tmpDir := t.TempDir()

	// Change to temp directory to test default metadata location
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer os.Chdir(originalWd)

	if chdirErr := os.Chdir(tmpDir); chdirErr != nil {
		t.Fatalf("Failed to change directory: %v", chdirErr)
	}

	// Run fetch without specifying metadata file
	cmd := exec.Command(binaryPath, "fetch", "golang/mock",
		"--output", "test.ndjson")

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil {
		t.Fatalf("Fetch failed: %v\nStderr: %s", err, stderr.String())
	}

	// Check for metadata file in current directory
	metadataFile := "fetch-metadata.json"
	if _, statErr := os.Stat(metadataFile); os.IsNotExist(statErr) {
		t.Fatal("Metadata file was not created in current directory")
	}

	// Verify it's valid JSON
	data, err := os.ReadFile(metadataFile)
	if err != nil {
		t.Fatalf("Failed to read metadata file: %v", err)
	}

	var fetchMetadata metadata.FetchMetadata
	if err := json.Unmarshal(data, &fetchMetadata); err != nil {
		t.Fatalf("Failed to parse metadata: %v", err)
	}
}

func TestMetadataGeneration_TimeWindows(t *testing.T) {
	if os.Getenv("GITHUB_TOKEN") == "" {
		t.Skip("Skipping test: GITHUB_TOKEN not set")
	}

	binaryPath := buildBinary(t)
	tmpDir := t.TempDir()

	outputFile := filepath.Join(tmpDir, "test.ndjson")
	metadataFile := filepath.Join(tmpDir, "fetch-metadata.json")

	// Run fetch with time windows
	cmd := exec.Command(binaryPath, "fetch", "golang/mock",
		"--output", outputFile,
		"--metadata-file", metadataFile,
		"--since", "2023-01-01",
		"--until", "2023-12-31")

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		t.Fatalf("Fetch failed: %v\nStderr: %s", err, stderr.String())
	}

	// Load metadata
	data, err := os.ReadFile(metadataFile)
	if err != nil {
		t.Fatalf("Failed to read metadata file: %v", err)
	}

	var fetchMetadata metadata.FetchMetadata
	if err := json.Unmarshal(data, &fetchMetadata); err != nil {
		t.Fatalf("Failed to parse metadata: %v", err)
	}

	// Verify time parameters were recorded
	if fetchMetadata.Parameters.Since == nil {
		t.Error("Since parameter not recorded in metadata")
	} else if !strings.Contains(fetchMetadata.Parameters.Since.String(), "2023-01-01") {
		t.Errorf("Unexpected since date: %v", fetchMetadata.Parameters.Since)
	}

	if fetchMetadata.Parameters.Until == nil {
		t.Error("Until parameter not recorded in metadata")
	} else if !strings.Contains(fetchMetadata.Parameters.Until.String(), "2023-12-31") {
		t.Errorf("Unexpected until date: %v", fetchMetadata.Parameters.Until)
	}
}

func TestMetadataGeneration_IncrementalFetch(t *testing.T) {
	if os.Getenv("GITHUB_TOKEN") == "" {
		t.Skip("Skipping test: GITHUB_TOKEN not set")
	}

	binaryPath := buildBinary(t)
	tmpDir := t.TempDir()

	// First, do a full fetch
	outputFile1 := filepath.Join(tmpDir, "test1.ndjson")
	metadataFile1 := filepath.Join(tmpDir, "metadata1.json")

	cmd1 := exec.Command(binaryPath, "fetch", "golang/mock",
		"--output", outputFile1,
		"--metadata-file", metadataFile1,
		"--since", "2023-01-01",
		"--until", "2023-06-30")

	if err := cmd1.Run(); err != nil {
		t.Fatalf("First fetch failed: %v", err)
	}

	// Load first metadata to get fetch ID
	data1, err := os.ReadFile(metadataFile1)
	if err != nil {
		t.Fatalf("Failed to read first metadata: %v", err)
	}

	var metadata1 metadata.FetchMetadata
	if unmarshalErr := json.Unmarshal(data1, &metadata1); unmarshalErr != nil {
		t.Fatalf("Failed to parse first metadata: %v", unmarshalErr)
	}

	// Now do an incremental fetch
	outputFile2 := filepath.Join(tmpDir, "test2.ndjson")
	metadataFile2 := filepath.Join(tmpDir, "metadata2.json")

	cmd2 := exec.Command(binaryPath, "fetch", "golang/mock",
		"--output", outputFile2,
		"--metadata-file", metadataFile2,
		"--incremental")

	if runErr := cmd2.Run(); runErr != nil {
		t.Fatalf("Incremental fetch failed: %v", runErr)
	}

	// Load second metadata
	data2, err := os.ReadFile(metadataFile2)
	if err != nil {
		t.Fatalf("Failed to read second metadata: %v", err)
	}

	var metadata2 metadata.FetchMetadata
	if err := json.Unmarshal(data2, &metadata2); err != nil {
		t.Fatalf("Failed to parse second metadata: %v", err)
	}

	// Verify incremental metadata
	if !metadata2.Incremental {
		t.Error("Expected incremental flag to be true")
	}

	// For now, we don't have previous fetch tracking implemented,
	// but when we do, verify it here:
	// if metadata2.PreviousFetch == nil {
	//     t.Error("Expected previous fetch reference")
	// } else if metadata2.PreviousFetch.FetchID != metadata1.FetchID {
	//     t.Errorf("Wrong previous fetch ID: %s", metadata2.PreviousFetch.FetchID)
	// }
}

func TestMetadataGeneration_BatchSize(t *testing.T) {
	if os.Getenv("GITHUB_TOKEN") == "" {
		t.Skip("Skipping test: GITHUB_TOKEN not set")
	}

	binaryPath := buildBinary(t)
	tmpDir := t.TempDir()

	outputFile := filepath.Join(tmpDir, "test.ndjson")
	metadataFile := filepath.Join(tmpDir, "fetch-metadata.json")

	// Run fetch with custom batch size
	cmd := exec.Command(binaryPath, "fetch", "golang/mock",
		"--output", outputFile,
		"--metadata-file", metadataFile)

	// Set custom batch size via environment variable
	cmd.Env = append(os.Environ(), "SIRSEER_BATCH_SIZE=25")

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		t.Fatalf("Fetch failed: %v\nStderr: %s", err, stderr.String())
	}

	// Load metadata
	data, err := os.ReadFile(metadataFile)
	if err != nil {
		t.Fatalf("Failed to read metadata file: %v", err)
	}

	var fetchMetadata metadata.FetchMetadata
	if err := json.Unmarshal(data, &fetchMetadata); err != nil {
		t.Fatalf("Failed to parse metadata: %v", err)
	}

	// Verify batch size was recorded
	if fetchMetadata.Parameters.BatchSize != 25 {
		t.Errorf("Expected batch size 25, got %d", fetchMetadata.Parameters.BatchSize)
	}
}

func TestMetadataGeneration_FetchAll(t *testing.T) {
	if os.Getenv("GITHUB_TOKEN") == "" {
		t.Skip("Skipping test: GITHUB_TOKEN not set")
	}

	binaryPath := buildBinary(t)
	tmpDir := t.TempDir()

	outputFile := filepath.Join(tmpDir, "test.ndjson")
	metadataFile := filepath.Join(tmpDir, "fetch-metadata.json")

	// Run fetch with --all flag
	cmd := exec.Command(binaryPath, "fetch", "golang/mock",
		"--output", outputFile,
		"--metadata-file", metadataFile,
		"--all")

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		t.Fatalf("Fetch failed: %v\nStderr: %s", err, stderr.String())
	}

	// Load metadata
	data, err := os.ReadFile(metadataFile)
	if err != nil {
		t.Fatalf("Failed to read metadata file: %v", err)
	}

	var fetchMetadata metadata.FetchMetadata
	if err := json.Unmarshal(data, &fetchMetadata); err != nil {
		t.Fatalf("Failed to parse metadata: %v", err)
	}

	// Verify fetch_all flag was recorded
	if !fetchMetadata.Parameters.FetchAll {
		t.Error("Expected fetch_all to be true")
	}
}

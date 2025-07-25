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

package testutil

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// AssertNDJSONOutput validates that a file contains valid NDJSON with expected PR count
func AssertNDJSONOutput(t *testing.T, filePath string, expectedPRCount int) {
	t.Helper()

	file, err := os.Open(filePath)
	if err != nil {
		t.Fatalf("Failed to open output file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	count := 0

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var pr map[string]interface{}
		if err := json.Unmarshal([]byte(line), &pr); err != nil {
			t.Errorf("Line %d: invalid JSON: %v", count+1, err)
			continue
		}

		// Validate PR has required fields
		requiredFields := []string{"number", "title", "state", "url", "created_at", "updated_at", "author"}
		for _, field := range requiredFields {
			if _, ok := pr[field]; !ok {
				t.Errorf("Line %d: missing required field '%s'", count+1, field)
			}
		}

		count++
	}

	if err := scanner.Err(); err != nil {
		t.Fatalf("Error reading file: %v", err)
	}

	if count != expectedPRCount {
		t.Errorf("Expected %d PRs, got %d", expectedPRCount, count)
	}
}

// AssertMetadataFile validates metadata file contents
func AssertMetadataFile(t *testing.T, dir string, repo string) {
	t.Helper()

	// Look for metadata file
	pattern := filepath.Join(dir, repo+".metadata.json")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatalf("Failed to glob metadata files: %v", err)
	}

	if len(matches) == 0 {
		pattern = filepath.Join(dir, "*-metadata.json")
		matches, err = filepath.Glob(pattern)
		if err != nil {
			t.Fatalf("Failed to glob metadata files: %v", err)
		}
	}

	if len(matches) == 0 {
		t.Fatal("No metadata file found")
	}

	// Read and validate metadata
	data, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("Failed to read metadata file: %v", err)
	}

	var metadata map[string]interface{}
	if err := json.Unmarshal(data, &metadata); err != nil {
		t.Fatalf("Invalid metadata JSON: %v", err)
	}

	// Check required fields
	requiredFields := []string{"version", "repository", "fetch_started", "fetch_completed", "total_prs_fetched"}
	for _, field := range requiredFields {
		if _, ok := metadata[field]; !ok {
			t.Errorf("Missing required metadata field: %s", field)
		}
	}
}

// AssertContainsString checks if a string contains a substring
func AssertContainsString(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Errorf("Expected string to contain %q, got: %s", needle, haystack)
	}
}

// AssertNotContainsString checks if a string does not contain a substring
func AssertNotContainsString(t *testing.T, haystack, needle string) {
	t.Helper()
	if strings.Contains(haystack, needle) {
		t.Errorf("Expected string to NOT contain %q, got: %s", needle, haystack)
	}
}

// AssertErrorContains checks if an error contains expected text
func AssertErrorContains(t *testing.T, err error, expected string) {
	t.Helper()
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !strings.Contains(err.Error(), expected) {
		t.Errorf("Expected error to contain %q, got: %v", expected, err)
	}
}

// AssertNoError fails the test if err is not nil
func AssertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}

// AssertEqual compares two values and fails if they're not equal
func AssertEqual(t *testing.T, got, want interface{}) {
	t.Helper()
	if got != want {
		t.Errorf("Got %v, want %v", got, want)
	}
}

// AssertNotEqual compares two values and fails if they're equal
func AssertNotEqual(t *testing.T, got, notWant interface{}) {
	t.Helper()
	if got == notWant {
		t.Errorf("Got %v, but didn't want it", got)
	}
}

// AssertFilePermissions checks file has expected permissions
func AssertFilePermissions(t *testing.T, path string, expectedMode os.FileMode) {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}

	mode := info.Mode()
	if mode != expectedMode {
		t.Errorf("Expected file mode %v, got %v", expectedMode, mode)
	}
}

// AssertDirExists checks that a directory exists
func AssertDirExists(t *testing.T, path string) {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			t.Fatalf("Expected directory to exist: %s", path)
		}
		t.Fatalf("Failed to stat directory: %v", err)
	}

	if !info.IsDir() {
		t.Fatalf("Expected %s to be a directory", path)
	}
}

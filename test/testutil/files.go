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
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// CreateTempFile creates a temporary file with the given content
func CreateTempFile(t *testing.T, dir, pattern, content string) string {
	t.Helper()

	file, err := os.CreateTemp(dir, pattern)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	if _, err := file.WriteString(content); err != nil {
		file.Close()
		t.Fatalf("Failed to write to temp file: %v", err)
	}

	if err := file.Close(); err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}

	t.Cleanup(func() {
		os.Remove(file.Name())
	})

	return file.Name()
}

// CreateTempDir creates a temporary directory that's automatically cleaned up
func CreateTempDir(t *testing.T, pattern string) string {
	t.Helper()

	dir, err := os.MkdirTemp("", pattern)
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	t.Cleanup(func() {
		os.RemoveAll(dir)
	})

	return dir
}

// WriteJSON writes a struct as JSON to a file
func WriteJSON(t *testing.T, path string, data interface{}) {
	t.Helper()

	content, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal JSON: %v", err)
	}

	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatalf("Failed to write JSON file: %v", err)
	}
}

// ReadJSON reads JSON from a file into a struct
func ReadJSON(t *testing.T, path string, v interface{}) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if err := json.Unmarshal(data, v); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}
}

// AssertFileExists checks that a file exists
func AssertFileExists(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatalf("Expected file to exist: %s", path)
	}
}

// AssertFileNotExists checks that a file does not exist
func AssertFileNotExists(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("Expected file to not exist: %s", path)
	}
}

// AssertFileContains checks that a file contains the expected string
func AssertFileContains(t *testing.T, path, expected string) {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if string(content) != expected {
		t.Errorf("File content mismatch\nGot:\n%s\nWant:\n%s", content, expected)
	}
}

// CreateStateDir creates a standard state directory structure for tests
func CreateStateDir(t *testing.T, baseDir string) string {
	t.Helper()

	stateDir := filepath.Join(baseDir, ".sirseer-relay")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("Failed to create state dir: %v", err)
	}

	return stateDir
}


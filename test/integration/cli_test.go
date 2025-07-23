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
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func buildBinary(t *testing.T) string {
	// Build binary in temp directory
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "sirseer-relay")

	cmd := exec.Command("go", "build", "-o", binaryPath, "../../cmd/relay")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build binary: %v\nOutput: %s", err, output)
	}

	return binaryPath
}

func TestCLI_InvalidRepoFormat(t *testing.T) {
	binaryPath := buildBinary(t)

	tests := []struct {
		name    string
		repo    string
		wantErr string
	}{
		{
			name:    "missing slash",
			repo:    "invalid-repo-format",
			wantErr: "invalid repository format",
		},
		{
			name:    "too many slashes",
			repo:    "org/repo/extra",
			wantErr: "invalid repository format",
		},
		{
			name:    "empty owner",
			repo:    "/repo",
			wantErr: "invalid repository format",
		},
		{
			name:    "empty repo",
			repo:    "org/",
			wantErr: "invalid repository format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(binaryPath, "fetch", tt.repo)

			var stderr bytes.Buffer
			cmd.Stderr = &stderr

			err := cmd.Run()
			if err == nil {
				t.Fatal("Expected command to fail")
			}

			// Verify error message
			stderrStr := stderr.String()
			if !strings.Contains(stderrStr, tt.wantErr) {
				t.Errorf("Expected error containing %q, got: %s", tt.wantErr, stderrStr)
			}
		})
	}
}

func TestCLI_MissingToken(t *testing.T) {
	binaryPath := buildBinary(t)

	// Clear any existing GITHUB_TOKEN
	cmd := exec.Command(binaryPath, "fetch", "test/repo")
	cmd.Env = []string{"PATH=" + os.Getenv("PATH")}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		t.Fatal("Expected command to fail")
	}

	// Verify error message
	stderrStr := stderr.String()
	if !strings.Contains(stderrStr, "GitHub token not found") {
		t.Errorf("Expected missing token error, got: %s", stderrStr)
	}
}

func TestCLI_HelpCommand(t *testing.T) {
	binaryPath := buildBinary(t)

	tests := []struct {
		name string
		args []string
	}{
		{
			name: "main help",
			args: []string{"--help"},
		},
		{
			name: "fetch help",
			args: []string{"fetch", "--help"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(binaryPath, tt.args...)

			var stdout bytes.Buffer
			cmd.Stdout = &stdout

			err := cmd.Run()
			if err != nil {
				t.Fatalf("Help command failed: %v", err)
			}

			output := stdout.String()

			// Verify help content
			if !strings.Contains(output, "sirseer-relay") {
				t.Error("Expected binary name in help output")
			}

			if len(tt.args) > 1 && tt.args[0] == "fetch" {
				// Fetch-specific help
				if !strings.Contains(output, "--all") {
					t.Error("Expected --all flag in fetch help")
				}
				if !strings.Contains(output, "--request-timeout") {
					t.Error("Expected --request-timeout flag in fetch help")
				}
				if !strings.Contains(output, "Fetch all pull requests from the repository") {
					t.Error("Expected --all flag description")
				}
			}
		})
	}
}

func TestCLI_VersionFlag(t *testing.T) {
	binaryPath := buildBinary(t)

	cmd := exec.Command(binaryPath, "--version")

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	err := cmd.Run()
	if err != nil {
		t.Fatalf("Version flag failed: %v", err)
	}

	output := stdout.String()

	// Version should contain "sirseer-relay" and a version
	if !strings.Contains(output, "sirseer-relay") {
		t.Error("Expected binary name in version output")
	}
}

func TestCLI_Flags(t *testing.T) {
	binaryPath := buildBinary(t)

	// Test with all flags (will fail due to no token, but we can verify parsing)
	cmd := exec.Command(binaryPath, "fetch", "test/repo",
		"--output", "test.ndjson",
		"--all",
		"--request-timeout", "300",
		"--since", "2024-01-01",
		"--until", "2024-12-31",
		"--incremental")

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		t.Fatal("Expected command to fail (no token)")
	}

	// Should fail with missing token, not flag parsing error
	stderrStr := stderr.String()
	if !strings.Contains(stderrStr, "GitHub token not found") {
		t.Errorf("Expected missing token error, got: %s", stderrStr)
	}
}

func TestCLI_InvalidFlags(t *testing.T) {
	binaryPath := buildBinary(t)

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "unknown flag",
			args:    []string{"fetch", "test/repo", "--unknown-flag"},
			wantErr: "unknown flag",
		},
		{
			name:    "invalid timeout",
			args:    []string{"fetch", "test/repo", "--request-timeout", "not-a-number"},
			wantErr: "invalid",
		},
		{
			name:    "missing repo argument",
			args:    []string{"fetch"},
			wantErr: "accepts 1 arg",
		},
		{
			name:    "too many arguments",
			args:    []string{"fetch", "repo1", "repo2"},
			wantErr: "accepts 1 arg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(binaryPath, tt.args...)

			var stderr bytes.Buffer
			cmd.Stderr = &stderr

			err := cmd.Run()
			if err == nil {
				t.Fatal("Expected command to fail")
			}

			stderrStr := stderr.String()
			if !strings.Contains(strings.ToLower(stderrStr), tt.wantErr) {
				t.Errorf("Expected error containing %q, got: %s", tt.wantErr, stderrStr)
			}
		})
	}
}

// TestCLI_ExitCodes verifies that the CLI returns appropriate exit codes
func TestCLI_ExitCodes(t *testing.T) {
	binaryPath := buildBinary(t)

	tests := []struct {
		name         string
		args         []string
		env          []string
		wantExitCode int
	}{
		{
			name:         "missing token",
			args:         []string{"fetch", "test/repo"},
			env:          []string{"PATH=" + os.Getenv("PATH")},
			wantExitCode: 1,
		},
		{
			name:         "invalid repo format",
			args:         []string{"fetch", "invalid"},
			wantExitCode: 1,
		},
		{
			name:         "help command",
			args:         []string{"--help"},
			wantExitCode: 0,
		},
		{
			name:         "version flag",
			args:         []string{"--version"},
			wantExitCode: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(binaryPath, tt.args...)
			if tt.env != nil {
				cmd.Env = tt.env
			}

			err := cmd.Run()

			exitCode := 0
			if err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					exitCode = exitErr.ExitCode()
				} else {
					t.Fatalf("Unexpected error type: %v", err)
				}
			}

			if exitCode != tt.wantExitCode {
				t.Errorf("Expected exit code %d, got %d", tt.wantExitCode, exitCode)
			}
		})
	}
}

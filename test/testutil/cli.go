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
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
)

var (
	binaryOnce sync.Once
	binaryPath string
	buildErr   error
)

// BuildBinary builds the sirseer-relay binary once per test run
func BuildBinary(t *testing.T) string {
	t.Helper()

	binaryOnce.Do(func() {
		// Create a persistent temp directory, not tied to test cleanup
		tmpDir, err := os.MkdirTemp("", "sirseer-relay-test")
		if err != nil {
			buildErr = err
			return
		}
		binaryPath = filepath.Join(tmpDir, "sirseer-relay")

		// Find project root by looking for go.mod
		projectRoot, err := findProjectRoot()
		if err != nil {
			buildErr = err
			return
		}

		cmd := exec.Command("go", "build", "-o", binaryPath, filepath.Join(projectRoot, "cmd", "relay"))
		if output, err := cmd.CombinedOutput(); err != nil {
			buildErr = err
			t.Logf("Build output: %s", output)
		}
	})

	if buildErr != nil {
		t.Fatalf("Failed to build binary: %v", buildErr)
	}

	return binaryPath
}

// CLIResult contains the result of running a CLI command
type CLIResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
	Err      error
}

// RunCLI executes the sirseer-relay binary with the given arguments
func RunCLI(t *testing.T, args []string, env map[string]string) CLIResult {
	t.Helper()

	binary := BuildBinary(t)

	cmd := exec.Command(binary, args...)

	// Set up environment
	cmd.Env = os.Environ()
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	exitCode := 0
	if exitErr, ok := err.(*exec.ExitError); ok {
		exitCode = exitErr.ExitCode()
	} else if err != nil {
		exitCode = -1
	}

	return CLIResult{
		ExitCode: exitCode,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Err:      err,
	}
}

// AssertCLISuccess checks that the CLI command succeeded
func AssertCLISuccess(t *testing.T, result CLIResult) {
	t.Helper()

	if result.Err != nil {
		t.Fatalf("Command failed: %v\nStderr: %s", result.Err, result.Stderr)
	}
}

// AssertCLIError checks that the CLI command failed with expected error
func AssertCLIError(t *testing.T, result CLIResult, expectedError string) {
	t.Helper()

	if result.Err == nil {
		t.Fatal("Expected command to fail, but it succeeded")
	}

	if expectedError != "" && !bytes.Contains([]byte(result.Stderr), []byte(expectedError)) {
		t.Errorf("Expected error containing %q, got: %s", expectedError, result.Stderr)
	}
}

// AssertExitCode checks the command exit code
func AssertExitCode(t *testing.T, result CLIResult, expected int) {
	t.Helper()

	if result.ExitCode != expected {
		t.Errorf("Expected exit code %d, got %d\nStderr: %s", expected, result.ExitCode, result.Stderr)
	}
}

// RunWithMockServer runs CLI with a mock server and returns the result
func RunWithMockServer(t *testing.T, server *MockServer, repo string, args ...string) CLIResult {
	t.Helper()

	// Build full args
	fullArgs := []string{"fetch", repo}
	fullArgs = append(fullArgs, args...)

	// Set up environment with test token
	env := map[string]string{
		"GITHUB_TOKEN":    "test-token",
		"SIRSEER_API_URL": server.URL + "/graphql",
	}

	return RunCLI(t, fullArgs, env)
}

// findProjectRoot finds the project root by looking for go.mod
func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}


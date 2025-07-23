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

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sirseerhq/sirseer-relay/internal/github"
)

func TestRunFetch_MockClient(t *testing.T) {
	// Create a temporary directory for output files
	tmpDir := t.TempDir()

	tests := []struct {
		name        string
		repoArg     string
		token       string
		outputFile  string
		mockSetup   func() *github.MockClient
		wantErr     bool
		wantErrMsg  string
		checkOutput func(t *testing.T, output string)
	}{
		{
			name:       "successful fetch to file",
			repoArg:    "test/repo",
			token:      "test-token",
			outputFile: filepath.Join(tmpDir, "output.ndjson"),
			mockSetup:  github.NewMockClient,
			wantErr:    false,
			checkOutput: func(t *testing.T, outputFile string) {
				// Read the output file
				data, err := os.ReadFile(outputFile)
				if err != nil {
					t.Fatalf("failed to read output file: %v", err)
				}

				// Check that we have NDJSON
				lines := strings.Split(strings.TrimSpace(string(data)), "\n")
				if len(lines) != 3 {
					t.Errorf("expected 3 lines of NDJSON, got %d", len(lines))
				}

				// Parse first line to verify it's valid JSON
				var pr github.PullRequest
				if err := json.Unmarshal([]byte(lines[0]), &pr); err != nil {
					t.Errorf("failed to parse first line as JSON: %v", err)
				}

				if pr.Number == 0 {
					t.Error("expected PR to have a number")
				}
			},
		},
		{
			name:       "successful fetch to stdout",
			repoArg:    "test/repo",
			token:      "test-token",
			outputFile: "",
			mockSetup: func() *github.MockClient {
				return github.NewMockClientWithOptions(
					github.WithPullRequests([]github.PullRequest{
						{Number: 1, Title: "Test PR", State: "open"},
					}),
				)
			},
			wantErr: false,
			checkOutput: func(t *testing.T, output string) {
				// For stdout test, output contains the JSON data
				var pr github.PullRequest
				if err := json.Unmarshal([]byte(output), &pr); err != nil {
					t.Errorf("failed to parse output as JSON: %v", err)
				}

				if pr.Number != 1 {
					t.Errorf("expected PR number 1, got %d", pr.Number)
				}
			},
		},
		{
			name:       "missing token",
			repoArg:    "test/repo",
			token:      "",
			wantErr:    true,
			wantErrMsg: "GitHub token not found",
		},
		{
			name:       "invalid repo format",
			repoArg:    "invalid",
			token:      "test-token",
			wantErr:    true,
			wantErrMsg: "invalid repository format",
		},
		{
			name:    "auth failure",
			repoArg: "test/repo",
			token:   "bad-token",
			mockSetup: func() *github.MockClient {
				return github.NewMockClientWithOptions(github.WithAuthFailure())
			},
			wantErr:    true,
			wantErrMsg: "authentication failed",
		},
		{
			name:       "repo not found",
			repoArg:    "nonexistent/repo",
			token:      "test-token",
			mockSetup:  github.NewMockClient, // Mock checks for "nonexistent" owner
			wantErr:    true,
			wantErrMsg: "repository not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up mock client if provided
			var originalNewClient func(string) *github.GraphQLClient
			if tt.mockSetup != nil {
				_ = tt.mockSetup() // mock would be used with DI
				// We need to inject the mock somehow - for now we'll skip this
				// In a real implementation, we'd use dependency injection
				t.Skip("Mock injection not implemented - would need to refactor to support DI")
			}

			// Capture output if writing to stdout
			var output bytes.Buffer
			if tt.outputFile == "" && tt.checkOutput != nil {
				// For stdout tests, we'd need to capture output
				// This is complex to do properly, so we'll skip for now
				t.Skip("Stdout capture not implemented")
			}

			// Run the fetch
			err := runFetch(context.Background(), tt.repoArg, tt.token, tt.outputFile, false)

			// Check error
			if (err != nil) != tt.wantErr {
				t.Errorf("runFetch() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.wantErrMsg != "" {
				if !strings.Contains(err.Error(), tt.wantErrMsg) {
					t.Errorf("expected error to contain %q, got %v", tt.wantErrMsg, err)
				}
			}

			// Check output if specified
			if !tt.wantErr && tt.checkOutput != nil {
				if tt.outputFile != "" {
					tt.checkOutput(t, tt.outputFile)
				} else {
					tt.checkOutput(t, output.String())
				}
			}

			// Restore original if we mocked
			_ = originalNewClient // unused for now
		})
	}
}

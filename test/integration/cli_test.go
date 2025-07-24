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
	"strings"
	"testing"

	"github.com/sirseerhq/sirseer-relay/test/testutil"
)

func TestCLI_InvalidRepoFormat(t *testing.T) {
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
			name:    "empty owner",
			repo:    "/repo",
			wantErr: "invalid repository format",
		},
		{
			name:    "empty repo",
			repo:    "owner/",
			wantErr: "invalid repository format",
		},
		{
			name:    "too many slashes",
			repo:    "owner/repo/extra",
			wantErr: "invalid repository format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := testutil.RunCLI(t, []string{"fetch", tt.repo}, nil)
			testutil.AssertCLIError(t, result, tt.wantErr)
		})
	}
}

func TestCLI_MissingToken(t *testing.T) {
	// Clear any existing GITHUB_TOKEN by providing minimal env
	env := map[string]string{
		"PATH": "/usr/bin:/bin", // Minimal PATH
	}
	
	result := testutil.RunCLI(t, []string{"fetch", "test/repo"}, env)
	testutil.AssertCLIError(t, result, "GitHub token not found")
}

func TestCLI_HelpCommand(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "root help",
			args: []string{"--help"},
		},
		{
			name: "help command",
			args: []string{"help"},
		},
		{
			name: "fetch help",
			args: []string{"fetch", "--help"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := testutil.RunCLI(t, tt.args, nil)
			testutil.AssertCLISuccess(t, result)

			// Verify help content
			if !strings.Contains(result.Stdout, "sirseer-relay") {
				t.Error("Expected binary name in help output")
			}

			if len(tt.args) > 1 && tt.args[0] == "fetch" {
				// Fetch-specific help
				if !strings.Contains(result.Stdout, "--all") {
					t.Error("Expected --all flag in fetch help")
				}
				if !strings.Contains(result.Stdout, "--request-timeout") {
					t.Error("Expected --request-timeout flag in fetch help")
				}
				if !strings.Contains(result.Stdout, "Fetch all pull requests from the repository") {
					t.Error("Expected --all flag description")
				}
			}
		})
	}
}

func TestCLI_VersionFlag(t *testing.T) {
	result := testutil.RunCLI(t, []string{"--version"}, nil)
	testutil.AssertCLISuccess(t, result)

	// Version should contain "sirseer-relay" and a version
	if !strings.Contains(result.Stdout, "sirseer-relay") {
		t.Error("Expected binary name in version output")
	}
}

func TestCLI_Flags(t *testing.T) {
	// Test with all flags (will fail due to no token, but we can verify parsing)
	args := []string{
		"fetch", "test/repo",
		"--output", "test.ndjson",
		"--all",
		"--request-timeout", "300",
		"--since", "2024-01-01",
		"--until", "2024-12-31",
		"--incremental",
	}
	
	result := testutil.RunCLI(t, args, nil)
	
	// Should fail with missing token, not flag parsing error
	testutil.AssertCLIError(t, result, "GitHub token not found")
}

func TestCLI_InvalidFlags(t *testing.T) {
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
			result := testutil.RunCLI(t, tt.args, nil)
			testutil.AssertCLIError(t, result, tt.wantErr)
		})
	}
}

func TestCLI_DateFormats(t *testing.T) {
	tests := []struct {
		name       string
		sinceDate  string
		untilDate  string
		shouldFail bool
		errMsg     string
	}{
		{
			name:       "valid date formats",
			sinceDate:  "2024-01-01",
			untilDate:  "2024-12-31",
			shouldFail: true, // Still fails due to test token
			errMsg:     "GitHub API authentication failed",
		},
		{
			name:       "invalid since date",
			sinceDate:  "01-01-2024", // Wrong format
			untilDate:  "2024-12-31",
			shouldFail: true,
			errMsg:     "invalid --since date format",
		},
		{
			name:       "invalid until date",
			sinceDate:  "2024-01-01",
			untilDate:  "2024/12/31", // Wrong format
			shouldFail: true,
			errMsg:     "invalid --until date format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := []string{"fetch", "test/repo"}
			if tt.sinceDate != "" {
				args = append(args, "--since", tt.sinceDate)
			}
			if tt.untilDate != "" {
				args = append(args, "--until", tt.untilDate)
			}

			// Provide a token to get past token validation
			env := map[string]string{"GITHUB_TOKEN": "test-token"}
			result := testutil.RunCLI(t, args, env)

			if tt.shouldFail {
				testutil.AssertCLIError(t, result, tt.errMsg)
			} else {
				testutil.AssertCLISuccess(t, result)
			}
		})
	}
}

func TestCLI_ConflictingFlags(t *testing.T) {
	t.Skip("Flag conflict validation not yet implemented")
	// Test that --all and --incremental are mutually exclusive
	args := []string{
		"fetch", "test/repo",
		"--all",
		"--incremental",
	}
	
	result := testutil.RunCLI(t, args, nil)
	testutil.AssertCLIError(t, result, "--all and --incremental flags are mutually exclusive")
}
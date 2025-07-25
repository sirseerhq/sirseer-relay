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
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	relaierrors "github.com/sirseerhq/sirseer-relay/internal/errors"
	"github.com/sirseerhq/sirseer-relay/internal/github"
	"github.com/sirseerhq/sirseer-relay/internal/metadata"
	"github.com/sirseerhq/sirseer-relay/internal/output"
	"github.com/sirseerhq/sirseer-relay/internal/state"
)

func TestNewFetchCommand(t *testing.T) {
	cmd := newFetchCommand("config.yaml")

	if cmd.Use != "fetch <org>/<repo>" {
		t.Errorf("expected Use to be 'fetch <org>/<repo>', got %s", cmd.Use)
	}

	if cmd.Short != "Fetch pull request data from a GitHub repository" {
		t.Errorf("unexpected Short description: %s", cmd.Short)
	}

	// Check that required flags exist
	flags := []string{"output", "token", "all", "request-timeout", "batch-size"}
	for _, flag := range flags {
		if cmd.Flag(flag) == nil {
			t.Errorf("expected flag %s to be defined", flag)
		}
	}
}

func TestParseRepository(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantOrg  string
		wantRepo string
		wantErr  bool
	}{
		{
			name:     "valid repository",
			input:    "github/hub",
			wantOrg:  "github",
			wantRepo: "hub",
			wantErr:  false,
		},
		{
			name:     "repository with hyphens",
			input:    "my-org/my-repo",
			wantOrg:  "my-org",
			wantRepo: "my-repo",
			wantErr:  false,
		},
		{
			name:    "missing slash",
			input:   "invalidrepo",
			wantErr: true,
		},
		{
			name:    "empty org",
			input:   "/repo",
			wantErr: true,
		},
		{
			name:    "empty repo",
			input:   "org/",
			wantErr: true,
		},
		{
			name:    "multiple slashes",
			input:   "org/repo/extra",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			org, repo, err := parseRepository(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseRepository() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if org != tt.wantOrg {
					t.Errorf("parseRepository() org = %v, want %v", org, tt.wantOrg)
				}
				if repo != tt.wantRepo {
					t.Errorf("parseRepository() repo = %v, want %v", repo, tt.wantRepo)
				}
			}
		})
	}
}

func TestParseDateFlags(t *testing.T) {
	tests := []struct {
		name      string
		sinceStr  string
		untilStr  string
		wantSince *time.Time
		wantUntil *time.Time
		wantErr   bool
	}{
		{
			name:      "both empty",
			sinceStr:  "",
			untilStr:  "",
			wantSince: nil,
			wantUntil: nil,
			wantErr:   false,
		},
		{
			name:      "valid date format",
			sinceStr:  "2024-01-15",
			untilStr:  "2024-02-15",
			wantSince: timePtr(time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)),
			wantUntil: timePtr(time.Date(2024, 2, 15, 0, 0, 0, 0, time.UTC)),
			wantErr:   false,
		},
		{
			name:     "invalid since format",
			sinceStr: "invalid-date",
			untilStr: "",
			wantErr:  true,
		},
		{
			name:     "invalid until format",
			sinceStr: "",
			untilStr: "invalid-date",
			wantErr:  true,
		},
		{
			name:      "datetime format",
			sinceStr:  "2024-01-15T10:30:00Z",
			untilStr:  "",
			wantSince: timePtr(time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)),
			wantUntil: nil,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSince, gotUntil, err := parseDateFlags(tt.sinceStr, tt.untilStr)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseDateFlags() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if !timeEqual(gotSince, tt.wantSince) {
					t.Errorf("parseDateFlags() since = %v, want %v", gotSince, tt.wantSince)
				}
				if !timeEqual(gotUntil, tt.wantUntil) {
					t.Errorf("parseDateFlags() until = %v, want %v", gotUntil, tt.wantUntil)
				}
			}
		})
	}
}

func TestCreateOutputWriter(t *testing.T) {
	tests := []struct {
		name       string
		outputPath string
		wantStdout bool
		wantErr    bool
		setup      func(t *testing.T, dir string)
		cleanup    func(t *testing.T, dir string)
	}{
		{
			name:       "stdout when empty",
			outputPath: "",
			wantStdout: true,
			wantErr:    false,
		},
		{
			name:       "stdout when dash",
			outputPath: "-",
			wantStdout: true,
			wantErr:    false,
		},
		{
			name:       "valid file path",
			outputPath: "test-output.ndjson",
			wantStdout: false,
			wantErr:    false,
		},
		{
			name:       "invalid directory",
			outputPath: "/nonexistent/path/output.ndjson",
			wantStdout: false,
			wantErr:    true,
		},
		{
			name:       "create directory if not exists",
			outputPath: "newdir/output.ndjson",
			wantStdout: false,
			wantErr:    true, // createOutputWriter doesn't create parent dirs for explicit output files
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			oldWd, _ := os.Getwd()
			os.Chdir(tmpDir)
			defer os.Chdir(oldWd)

			if tt.setup != nil {
				tt.setup(t, tmpDir)
			}

			writer, closerFunc, err := createOutputWriter(tt.outputPath, "", "test-org", "test-repo")
			if (err != nil) != tt.wantErr {
				t.Errorf("createOutputWriter() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if writer == nil {
					t.Error("createOutputWriter() returned nil writer")
				}
				if closerFunc != "" {
					// For file outputs, verify the file will be created
					defer func() {
						if f, err := os.Open(closerFunc); err == nil {
							f.Close()
							os.Remove(closerFunc)
						}
					}()
				}

				// Test that we can write to the writer
				testPR := &github.PullRequest{Number: 1}
				err = writer.Write(testPR)
				if err != nil {
					t.Errorf("failed to write to output writer: %v", err)
				}
			}

			if tt.cleanup != nil {
				tt.cleanup(t, tmpDir)
			}
		})
	}
}

func TestSaveMetadata(t *testing.T) {
	tests := []struct {
		name         string
		metadataPath string
		metadata     *metadata.FetchMetadata
		wantErr      bool
		setup        func(t *testing.T, dir string)
	}{
		{
			name:         "save to specific path",
			metadataPath: "test-metadata.json",
			metadata: &metadata.FetchMetadata{
				RelayVersion:  "test-version",
				MethodVersion: metadata.MethodVersion,
				FetchID:       "test-123",
				Parameters: metadata.FetchParams{
					Organization: "org",
					Repository:   "repo",
					FetchAll:     true,
					BatchSize:    50,
				},
				Results: metadata.FetchResults{
					TotalPRs:     10,
					FirstPR:      1,
					LastPR:       10,
					StartedAt:    time.Now(),
					CompletedAt:  time.Now(),
					APICallCount: 1,
				},
			},
			wantErr: false,
		},
		{
			name:         "save to output directory",
			metadataPath: "",
			metadata: &metadata.FetchMetadata{
				RelayVersion:  "test-version",
				MethodVersion: metadata.MethodVersion,
				FetchID:       "test-456",
				Parameters: metadata.FetchParams{
					Organization: "org",
					Repository:   "repo",
				},
				Results: metadata.FetchResults{
					TotalPRs: 5,
				},
			},
			wantErr: false,
			setup: func(t *testing.T, dir string) {
				os.MkdirAll(filepath.Join(dir, "output", "org", "repo"), 0755)
			},
		},
		{
			name:         "invalid path",
			metadataPath: "/nonexistent/path/metadata.json",
			metadata: &metadata.FetchMetadata{
				FetchID: "test-789",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			oldWd, _ := os.Getwd()
			os.Chdir(tmpDir)
			defer os.Chdir(oldWd)

			if tt.setup != nil {
				tt.setup(t, tmpDir)
			}

			err := saveMetadata(tt.metadata, tt.metadataPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("saveMetadata() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr && tt.metadataPath != "" {
				// Verify file was created
				if _, err := os.Stat(tt.metadataPath); os.IsNotExist(err) {
					t.Errorf("expected metadata file to be created at %s", tt.metadataPath)
				}
			}
		})
	}
}

func TestFetchOperationsWithMockClient(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		setupMock func() github.Client
		fetchOpts github.FetchOptions
		wantErr   bool
		wantPRs   int
		errCheck  func(error) bool
	}{
		{
			name: "successful fetch",
			setupMock: func() github.Client {
				mock := github.NewMockClient()
				// Mock client has default test PRs
				return mock
			},
			fetchOpts: github.FetchOptions{
				PageSize: 2,
			},
			wantErr: false,
			wantPRs: 3, // Mock client has 3 default PRs
		},
		{
			name: "auth failure",
			setupMock: func() github.Client {
				mock := github.NewMockClient()
				mock.ShouldFailAuth = true
				return mock
			},
			fetchOpts: github.FetchOptions{},
			wantErr:   true,
			errCheck: func(err error) bool {
				return strings.Contains(err.Error(), "authentication failed")
			},
		},
		{
			name: "network failure",
			setupMock: func() github.Client {
				mock := github.NewMockClient()
				mock.ShouldFailNetwork = true
				return mock
			},
			fetchOpts: github.FetchOptions{},
			wantErr:   true,
			errCheck: func(err error) bool {
				return strings.Contains(err.Error(), "network")
			},
		},
		{
			name: "empty repository",
			setupMock: func() github.Client {
				return github.NewMockClientWithOptions(
					github.WithPullRequests([]github.PullRequest{}),
				)
			},
			fetchOpts: github.FetchOptions{},
			wantErr:   false,
			wantPRs:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := tt.setupMock()
			var buf bytes.Buffer
			writer := output.NewWriter(&buf)

			prCount := 0
			err := fetchPRsWithClient(ctx, client, writer, "test-org", "test-repo", tt.fetchOpts, func(pr *github.PullRequest) error {
				prCount++
				return nil
			})

			if (err != nil) != tt.wantErr {
				t.Errorf("fetchPRsWithClient() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.errCheck != nil && err != nil && !tt.errCheck(err) {
				t.Errorf("error check failed for error: %v", err)
			}

			if !tt.wantErr && prCount != tt.wantPRs {
				t.Errorf("fetchPRsWithClient() fetched %d PRs, want %d", prCount, tt.wantPRs)
			}
		})
	}
}

func TestIncrementalFetchLogic(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name         string
		setupState   func(t *testing.T, stateFile string)
		setupMock    func() github.Client
		wantErr      bool
		wantNewPRs   int
		wantModified int
	}{
		{
			name: "incremental with existing state",
			setupState: func(t *testing.T, stateFile string) {
				// Create a state with some existing PRs
				s := &state.FetchState{
					Version:       state.CurrentVersion,
					LastFetchTime: time.Now().Add(-24 * time.Hour),
					Repository:    "test-org/test-repo",
					TotalFetched:  3,
					LastPRNumber:  3,
					LastPRDate:    time.Now().Add(-24 * time.Hour),
				}
				state.SaveState(s, stateFile)
			},
			setupMock: func() github.Client {
				// Return some new and modified PRs
				prs := []github.PullRequest{
					{Number: 2, UpdatedAt: time.Now().Add(-12 * time.Hour)}, // Modified
					{Number: 4, UpdatedAt: time.Now().Add(-6 * time.Hour)},  // New
					{Number: 5, UpdatedAt: time.Now()},                      // New
				}
				return github.NewMockClientWithOptions(
					github.WithPullRequests(prs),
				)
			},
			wantErr:      false,
			wantNewPRs:   2, // PRs 4 and 5 are new
			wantModified: 1, // PR 2 is modified
		},
		{
			name: "incremental with no state",
			setupState: func(t *testing.T, stateFile string) {
				// No state file
			},
			setupMock: func() github.Client {
				// Use default test PRs
				return github.NewMockClient()
			},
			wantErr:    false,
			wantNewPRs: 3, // Mock client has 3 default PRs
		},
		{
			name: "incremental with corrupted state",
			setupState: func(t *testing.T, stateFile string) {
				// Write invalid JSON
				os.WriteFile(stateFile, []byte("{invalid json"), 0644)
			},
			setupMock: func() github.Client {
				return github.NewMockClient()
			},
			wantErr:    false, // LoadState returns nil on error, so fetch proceeds normally
			wantNewPRs: 3,     // Will fetch all PRs as if no state exists
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			stateFile := filepath.Join(tmpDir, "test-state.json")

			if tt.setupState != nil {
				tt.setupState(t, stateFile)
			}

			client := tt.setupMock()
			var buf bytes.Buffer
			writer := output.NewWriter(&buf)

			opts := github.FetchOptions{}

			// Simulate incremental fetch logic
			existingState, _ := state.LoadState(stateFile)
			if existingState != nil && existingState.Repository != "test-org/test-repo" {
				t.Errorf("state repository mismatch")
				return
			}

			newPRs := 0
			modifiedPRs := 0

			err := fetchPRsWithClient(ctx, client, writer, "test-org", "test-repo", opts, func(pr *github.PullRequest) error {
				if existingState != nil {
					// In a real incremental fetch, we'd check if this PR is newer than LastPRDate
					// For this test, just count based on PR number
					if pr.Number > existingState.LastPRNumber {
						newPRs++
					} else {
						modifiedPRs++
					}
				} else {
					newPRs++
				}
				return nil
			})

			if (err != nil) != tt.wantErr {
				t.Errorf("incremental fetch error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				if newPRs != tt.wantNewPRs {
					t.Errorf("got %d new PRs, want %d", newPRs, tt.wantNewPRs)
				}
				if modifiedPRs != tt.wantModified {
					t.Errorf("got %d modified PRs, want %d", modifiedPRs, tt.wantModified)
				}
			}
		})
	}
}

// Helper functions

func timePtr(t time.Time) *time.Time {
	return &t
}

func timeEqual(t1, t2 *time.Time) bool {
	if t1 == nil && t2 == nil {
		return true
	}
	if t1 == nil || t2 == nil {
		return false
	}
	return t1.Equal(*t2)
}

// fetchPRsWithClient is a helper that simulates the core fetch logic
func fetchPRsWithClient(ctx context.Context, client github.Client, writer output.OutputWriter, owner, repo string, opts github.FetchOptions, onPR func(*github.PullRequest) error) error {
	page, err := client.FetchPullRequests(ctx, owner, repo, opts)
	if err != nil {
		return err
	}

	for _, pr := range page.PullRequests {
		if err := writer.Write(pr); err != nil {
			return fmt.Errorf("failed to write PR: %w", err)
		}
		if onPR != nil {
			if err := onPR(&pr); err != nil {
				return err
			}
		}
	}

	// In a real implementation, we would handle pagination here
	// For tests, the mock client returns all PRs in one page

	return nil
}

// TestFetchFirstPageWithOptions tests the first page fetch function
func TestFetchFirstPageWithOptions(t *testing.T) {
	tests := []struct {
		name    string
		mock    func() github.Client
		wantErr bool
		wantPRs int
	}{
		{
			name: "successful first page",
			mock: func() github.Client {
				return &github.MockClient{
					PullRequests: []github.PullRequest{
						{Number: 1, Title: "PR 1"},
						{Number: 2, Title: "PR 2"},
					},
					TotalPullRequests: 10, // More than one page
					PageSize:          2,
				}
			},
			wantErr: false,
			wantPRs: 2,
		},
		{
			name: "empty repository",
			mock: func() github.Client {
				return &github.MockClient{
					PullRequests:      []github.PullRequest{},
					TotalPullRequests: 0,
				}
			},
			wantErr: false,
			wantPRs: 0,
		},
		{
			name: "network error",
			mock: func() github.Client {
				return &github.MockClient{
					ShouldFailNetwork: true,
				}
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			writer := output.NewWriter(&buf)
			tmpDir := t.TempDir()
			metadataFile := filepath.Join(tmpDir, "metadata.json")

			err := fetchFirstPageWithOptions(context.Background(), tt.mock(), "test", "repo", writer, metadataFile, github.FetchOptions{})

			if (err != nil) != tt.wantErr {
				t.Errorf("fetchFirstPageWithOptions() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
				if strings.TrimSpace(buf.String()) == "" {
					lines = []string{}
				}
				if len(lines) != tt.wantPRs {
					t.Errorf("got %d PRs, want %d", len(lines), tt.wantPRs)
				}
			}
		})
	}
}

// TestFetchAllPullRequestsWithOptions tests fetching all pages
func TestFetchAllPullRequestsWithOptions(t *testing.T) {
	tests := []struct {
		name    string
		mock    func() github.Client
		wantErr bool
		wantPRs int
	}{
		{
			name: "multiple pages",
			mock: func() github.Client {
				// MockClient with SimulatePages will handle pagination
				return &github.MockClient{
					PullRequests: []github.PullRequest{
						{Number: 1}, {Number: 2}, {Number: 3}, {Number: 4},
					},
					TotalPullRequests: 4,
					PageSize:          2,
					SimulatePages:     true,
				}
			},
			wantErr: false,
			wantPRs: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			writer := output.NewWriter(&buf)
			tmpDir := t.TempDir()
			metadataFile := filepath.Join(tmpDir, "metadata.json")

			err := fetchAllPullRequestsWithOptions(context.Background(), tt.mock(), "test", "repo", writer, metadataFile, github.FetchOptions{PageSize: 2})

			if (err != nil) != tt.wantErr {
				t.Errorf("fetchAllPullRequestsWithOptions() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestFetchWithComplexityRetry tests the complexity retry logic
func TestFetchWithComplexityRetry(t *testing.T) {
	tests := []struct {
		name         string
		mock         func() github.Client
		pageSize     int
		minPageSize  int
		wantErr      bool
		wantPageSize int
	}{
		{
			name: "successful fetch without complexity error",
			mock: func() github.Client {
				return &github.MockClient{
					PullRequests: []github.PullRequest{
						{Number: 1}, {Number: 2},
					},
					TotalPullRequests: 2,
					PageSize:          50,
				}
			},
			pageSize:     50,
			minPageSize:  10,
			wantErr:      false,
			wantPageSize: 50,
		},
		{
			name: "complexity error triggers retry with reduced page size",
			mock: func() github.Client {
				return &github.MockClient{
					ComplexityErrorOnCall: 1, // First call returns complexity error
					PullRequests: []github.PullRequest{
						{Number: 1}, {Number: 2},
					},
					TotalPullRequests: 2,
					PageSize:          25, // Will be reduced from 50
				}
			},
			pageSize:     50,
			minPageSize:  10,
			wantErr:      false,
			wantPageSize: 25, // Should be reduced by half
		},
		{
			name: "multiple complexity errors until minimum",
			mock: func() github.Client {
				// This will keep failing with complexity error until it reaches minimum page size
				return &github.MockClient{
					PullRequests:          []github.PullRequest{{Number: 1}},
					TotalPullRequests:     1,
					PageSize:              5,
					ComplexityErrorOnCall: 1, // Error on first call only
				}
			},
			pageSize:     20,
			minPageSize:  5,
			wantErr:      false,
			wantPageSize: 10, // 20 -> 10 (then success)
		},
		{
			name: "complexity error at minimum page size",
			mock: func() github.Client {
				return &github.MockClient{
					ComplexityErrorOnCall: 1,
					Error:                 fmt.Errorf("query complexity: %w", relaierrors.ErrQueryComplexity),
				}
			},
			pageSize:    10,
			minPageSize: 10,
			wantErr:     true, // Should fail when can't reduce further
		},
		{
			name: "non-complexity error is not retried",
			mock: func() github.Client {
				return &github.MockClient{
					ShouldFailNetwork: true,
				}
			},
			pageSize:    50,
			minPageSize: 10,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := tt.mock()

			pageSizePtr := &tt.pageSize
			result, err := fetchWithComplexityRetry(
				context.Background(),
				client,
				"test",
				"repo",
				github.FetchOptions{PageSize: tt.pageSize},
				pageSizePtr,
			)

			if (err != nil) != tt.wantErr {
				t.Errorf("fetchWithComplexityRetry() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				if *pageSizePtr != tt.wantPageSize {
					t.Errorf("expected page size %d, got %d", tt.wantPageSize, *pageSizePtr)
				}
				if result == nil {
					t.Error("expected non-nil result")
				}
			}
		})
	}
}

// TestProcessFetchBatch tests the batch processing function
func TestProcessFetchBatch(t *testing.T) {
	tests := []struct {
		name          string
		batch         []github.PullRequest
		wantErr       bool
		wantWrites    int
		wantProcessed int
		wantLastPRNum int
	}{
		{
			name: "successful batch processing",
			batch: []github.PullRequest{
				{Number: 1, Title: "PR 1", CreatedAt: time.Now()},
				{Number: 2, Title: "PR 2", CreatedAt: time.Now()},
				{Number: 3, Title: "PR 3", CreatedAt: time.Now()},
			},
			wantErr:       false,
			wantWrites:    3,
			wantProcessed: 3,
			wantLastPRNum: 3,
		},
		{
			name:          "empty batch",
			batch:         []github.PullRequest{},
			wantErr:       false,
			wantWrites:    0,
			wantProcessed: 0,
			wantLastPRNum: 0,
		},
		{
			name: "single PR batch",
			batch: []github.PullRequest{
				{Number: 42, Title: "Single PR", CreatedAt: time.Now()},
			},
			wantErr:       false,
			wantWrites:    1,
			wantProcessed: 1,
			wantLastPRNum: 42,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			writer := output.NewWriter(&buf)

			// Create metadata tracker
			tracker := metadata.New()

			// Initialize progress tracker
			progress := &progressTracker{
				allPRsProcessed: 0,
				totalPRs:        100,
				startTime:       time.Now(),
			}

			// Save initial progress state
			initialProcessed := progress.allPRsProcessed

			err := processFetchBatch(tt.batch, writer, tracker, progress)

			if (err != nil) != tt.wantErr {
				t.Errorf("processFetchBatch() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				// Check processed count
				processed := progress.allPRsProcessed - initialProcessed
				if processed != tt.wantProcessed {
					t.Errorf("expected %d PRs processed, got %d", tt.wantProcessed, processed)
				}

				// Check last PR number
				if progress.lastPRNumber != tt.wantLastPRNum {
					t.Errorf("expected lastPRNumber %d, got %d", tt.wantLastPRNum, progress.lastPRNumber)
				}

				// Verify writes
				lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
				if strings.TrimSpace(buf.String()) == "" {
					lines = []string{}
				}
				if len(lines) != tt.wantWrites {
					t.Errorf("expected %d writes, got %d", tt.wantWrites, len(lines))
				}
			}
		})
	}
}

// TestRunFetch tests the main fetch command function
func TestRunFetch(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		envVars   map[string]string
		setupMock func() github.Client
		wantErr   bool
		errMsg    string
		checkFunc func(t *testing.T, outputPath string)
	}{
		{
			name: "successful fetch with basic args",
			args: []string{"test-org/test-repo"},
			envVars: map[string]string{
				"GITHUB_TOKEN": "test-token",
			},
			setupMock: func() github.Client {
				return &github.MockClient{
					PullRequests: []github.PullRequest{
						{Number: 1, Title: "Test PR"},
					},
					TotalPullRequests: 1,
				}
			},
			wantErr: false,
		},
		{
			name:    "missing repository argument",
			args:    []string{},
			wantErr: true,
			errMsg:  "expected exactly 1 argument",
		},
		{
			name:    "invalid repository format",
			args:    []string{"invalid-repo"},
			wantErr: true,
			errMsg:  "must be in format",
		},
		{
			name: "missing token",
			args: []string{"test-org/test-repo"},
			envVars: map[string]string{
				"GITHUB_TOKEN": "",
			},
			wantErr: true,
			errMsg:  "GitHub token is required",
		},
		{
			name: "fetch with output file",
			args: []string{"test-org/test-repo", "--output", "output.ndjson"},
			envVars: map[string]string{
				"GITHUB_TOKEN": "test-token",
			},
			setupMock: func() github.Client {
				return &github.MockClient{
					PullRequests: []github.PullRequest{
						{Number: 1, Title: "Test PR"},
					},
					TotalPullRequests: 1,
				}
			},
			wantErr: false,
			checkFunc: func(t *testing.T, outputPath string) {
				// Verify output file was created
				if _, err := os.Stat("output.ndjson"); err != nil {
					t.Error("Expected output file to be created")
				}
			},
		},
		{
			name: "fetch with date filters",
			args: []string{"test-org/test-repo", "--since", "2024-01-01", "--until", "2024-12-31"},
			envVars: map[string]string{
				"GITHUB_TOKEN": "test-token",
			},
			setupMock: func() github.Client {
				return &github.MockClient{
					PullRequests:      []github.PullRequest{},
					TotalPullRequests: 0,
				}
			},
			wantErr: false,
		},
		{
			name: "network error",
			args: []string{"test-org/test-repo"},
			envVars: map[string]string{
				"GITHUB_TOKEN": "test-token",
			},
			setupMock: func() github.Client {
				return &github.MockClient{
					ShouldFailNetwork: true,
				}
			},
			wantErr: true,
			errMsg:  "network",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp directory for test
			tmpDir := t.TempDir()
			oldWd, _ := os.Getwd()
			os.Chdir(tmpDir)
			defer os.Chdir(oldWd)

			// Set up environment
			oldEnv := os.Environ()
			defer func() {
				// Restore environment
				os.Clearenv()
				for _, e := range oldEnv {
					parts := strings.SplitN(e, "=", 2)
					if len(parts) == 2 {
						os.Setenv(parts[0], parts[1])
					}
				}
			}()

			// Set test environment variables
			for k, v := range tt.envVars {
				os.Setenv(k, v)
			}

			// Create command
			cmd := newFetchCommand("")
			cmd.SetArgs(tt.args)

			// Capture output
			var outBuf, errBuf bytes.Buffer
			cmd.SetOut(&outBuf)
			cmd.SetErr(&errBuf)

			// Set up mock client if provided
			if tt.setupMock != nil {
				// This would require injecting the mock client into the command
				// For now, we'll test the command validation logic
			}

			// Execute command
			err := cmd.Execute()

			// Check error
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
			}

			if err != nil && tt.errMsg != "" {
				errStr := err.Error() + errBuf.String()
				if !strings.Contains(errStr, tt.errMsg) {
					t.Errorf("Expected error containing %q, got %q", tt.errMsg, errStr)
				}
			}

			// Run additional checks
			if tt.checkFunc != nil {
				tt.checkFunc(t, tmpDir)
			}
		})
	}
}

// TestGetToken tests the token retrieval function
func TestGetToken(t *testing.T) {
	tests := []struct {
		name       string
		flagToken  string
		envVarName string
		envValue   string
		want       string
	}{
		{
			name:       "token from flag",
			flagToken:  "flag-token",
			envVarName: "GITHUB_TOKEN",
			envValue:   "env-token",
			want:       "flag-token", // Flag takes precedence
		},
		{
			name:       "token from environment",
			flagToken:  "",
			envVarName: "GITHUB_TOKEN",
			envValue:   "env-token",
			want:       "env-token",
		},
		{
			name:       "missing token",
			flagToken:  "",
			envVarName: "GITHUB_TOKEN",
			envValue:   "",
			want:       "",
		},
		{
			name:       "custom env var",
			flagToken:  "",
			envVarName: "CUSTOM_TOKEN",
			envValue:   "custom-value",
			want:       "custom-value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and restore env
			oldToken := os.Getenv(tt.envVarName)
			defer os.Setenv(tt.envVarName, oldToken)

			os.Setenv(tt.envVarName, tt.envValue)

			result := getToken(tt.flagToken, tt.envVarName)

			if result != tt.want {
				t.Errorf("getToken() = %v, want %v", result, tt.want)
			}
		})
	}
}

// TestParseDate tests the date parsing function
func TestParseDate(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid date",
			input:   "2024-01-15",
			wantErr: false,
		},
		{
			name:    "valid datetime",
			input:   "2024-01-15T10:30:00Z",
			wantErr: false,
		},
		{
			name:    "invalid format",
			input:   "01/15/2024",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseDate(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseDate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestLoadAndValidateIncrementalState tests the loadAndValidateIncrementalState function
func TestLoadAndValidateIncrementalState(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func(t *testing.T) string
		repoPath  string
		wantErr   bool
		wantState bool
	}{
		{
			name: "valid state file",
			setupFunc: func(t *testing.T) string {
				tmpDir := t.TempDir()
				stateFile := filepath.Join(tmpDir, "test-repo.state")

				// Create a valid state
				s := &state.FetchState{
					Version:       state.CurrentVersion,
					Repository:    "test/repo",
					LastFetchTime: time.Now().UTC(),
					LastPRDate:    time.Now().Add(-1 * time.Hour).UTC(),
					LastPRNumber:  3,
					TotalFetched:  3,
				}

				// Save state with checksum
				if err := state.SaveState(s, stateFile); err != nil {
					t.Fatalf("Failed to save state: %v", err)
				}

				return stateFile
			},
			repoPath:  "test/repo",
			wantErr:   false,
			wantState: true,
		},
		{
			name: "no state file",
			setupFunc: func(t *testing.T) string {
				tmpDir := t.TempDir()
				return filepath.Join(tmpDir, "test-repo.state")
			},
			repoPath:  "test/repo",
			wantErr:   true,
			wantState: false,
		},
		{
			name: "corrupted state file",
			setupFunc: func(t *testing.T) string {
				tmpDir := t.TempDir()
				stateFile := filepath.Join(tmpDir, "test-repo.state")

				// Write invalid JSON
				os.WriteFile(stateFile, []byte("{invalid json"), 0644)

				return stateFile
			},
			repoPath:  "test/repo",
			wantErr:   true,
			wantState: false,
		},
		{
			name: "repository mismatch",
			setupFunc: func(t *testing.T) string {
				tmpDir := t.TempDir()
				stateFile := filepath.Join(tmpDir, "test-repo.state")

				// Create state for different repo
				s := &state.FetchState{
					Version:       state.CurrentVersion,
					Repository:    "other/repo",
					LastFetchTime: time.Now().UTC(),
				}

				if err := state.SaveState(s, stateFile); err != nil {
					t.Fatalf("Failed to save state: %v", err)
				}

				return stateFile
			},
			repoPath:  "test/repo",
			wantErr:   true,
			wantState: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stateFile := tt.setupFunc(t)

			got, err := loadAndValidateIncrementalState(stateFile, tt.repoPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("loadAndValidateIncrementalState() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if (got != nil) != tt.wantState {
				t.Errorf("loadAndValidateIncrementalState() returned state = %v, wantState %v", got != nil, tt.wantState)
			}
		})
	}
}

// TestProcessIncrementalPR tests the processIncrementalPR function
func TestProcessIncrementalPR(t *testing.T) {
	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "output.ndjson")

	// Create output writer
	file, err := os.Create(outputFile)
	if err != nil {
		t.Fatalf("Failed to create output file: %v", err)
	}
	defer file.Close()

	writer := output.NewWriter(file)

	// Create tracker
	tracker := metadata.New()

	// Create previous state with LastPRNumber
	prevState := &state.FetchState{
		LastPRNumber: 3,
		Repository:   "test/repo",
	}

	// Create current state that will be updated
	currentState := &state.FetchState{
		Repository: "test/repo",
	}

	tests := []struct {
		name      string
		pr        github.PullRequest
		wantWrite bool
	}{
		{
			name: "new PR",
			pr: github.PullRequest{
				Number:    4,
				Title:     "New PR",
				CreatedAt: time.Now(),
			},
			wantWrite: true,
		},
		{
			name: "existing PR",
			pr: github.PullRequest{
				Number:    2,
				Title:     "Existing PR",
				CreatedAt: time.Now().Add(-24 * time.Hour),
			},
			wantWrite: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset current state for each test
			currentState.LastPRNumber = prevState.LastPRNumber
			currentState.LastPRDate = prevState.LastPRDate
			currentState.TotalFetched = 0

			wrote, err := processIncrementalPR(&tt.pr, prevState, currentState, writer, tracker)
			if err != nil {
				t.Fatalf("processIncrementalPR() error = %v", err)
			}

			if wrote != tt.wantWrite {
				t.Errorf("Expected wrote=%v, got %v", tt.wantWrite, wrote)
			}

			if tt.wantWrite {
				// Check that current state was updated
				if currentState.LastPRNumber != tt.pr.Number {
					t.Errorf("Expected LastPRNumber to be updated to %d, got %d", tt.pr.Number, currentState.LastPRNumber)
				}
			}
		})
	}
}

// TestPrepareIncrementalFetch tests the prepareIncrementalFetch function
func TestPrepareIncrementalFetch(t *testing.T) {
	tests := []struct {
		name      string
		prevState *state.FetchState
		repoPath  string
		sinceTime *time.Time
		untilTime *time.Time
		wantErr   bool
		checkFunc func(t *testing.T, ctx *incrementalFetchContext)
	}{
		{
			name: "use last PR date when sinceTime is nil",
			prevState: &state.FetchState{
				LastPRDate:   time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
				LastPRNumber: 5,
			},
			repoPath:  "test/repo",
			sinceTime: nil,
			untilTime: nil,
			wantErr:   false,
			checkFunc: func(t *testing.T, ctx *incrementalFetchContext) {
				if ctx.opts.Since == nil || !ctx.opts.Since.Equal(time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)) {
					t.Error("Expected opts.Since to be set from LastPRDate")
				}
			},
		},
		{
			name: "use provided sinceTime",
			prevState: &state.FetchState{
				LastPRDate:   time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
				LastPRNumber: 5,
			},
			repoPath:  "test/repo",
			sinceTime: timePtr(time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)),
			untilTime: nil,
			wantErr:   false,
			checkFunc: func(t *testing.T, ctx *incrementalFetchContext) {
				if ctx.opts.Since == nil || !ctx.opts.Since.Equal(time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)) {
					t.Error("Expected opts.Since to use provided value")
				}
			},
		},
		{
			name: "set until time",
			prevState: &state.FetchState{
				LastPRDate:   time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
				LastPRNumber: 5,
			},
			repoPath:  "test/repo",
			sinceTime: nil,
			untilTime: timePtr(time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)),
			wantErr:   false,
			checkFunc: func(t *testing.T, ctx *incrementalFetchContext) {
				if ctx.opts.Until == nil || !ctx.opts.Until.Equal(time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)) {
					t.Error("Expected opts.Until to be set")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, err := prepareIncrementalFetch(tt.prevState, tt.repoPath, tt.sinceTime, tt.untilTime)

			if (err != nil) != tt.wantErr {
				t.Errorf("prepareIncrementalFetch() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.checkFunc != nil {
				tt.checkFunc(t, ctx)
			}
		})
	}
}

// TestSaveIncrementalResults tests the saveIncrementalResults function
func TestSaveIncrementalResults(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "test-repo.state")
	metadataFile := filepath.Join(tmpDir, "test-repo.metadata.json")

	// Create current state
	currentState := &state.FetchState{
		Version:      state.CurrentVersion,
		Repository:   "test/repo",
		LastPRNumber: 10,
		LastPRDate:   time.Now(),
	}

	// Create tracker
	tracker := metadata.New()

	// Create previous fetch reference
	previousFetch := &metadata.FetchRef{
		FetchID:     "prev-123",
		CompletedAt: time.Now().Add(-1 * time.Hour),
	}

	// Call saveIncrementalResults
	err := saveIncrementalResults(
		currentState,
		stateFile,
		5, // newPRCount
		tracker,
		metadataFile,
		"test",
		"repo",
		github.FetchOptions{
			PageSize: 50,
			Since:    timePtr(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)),
		},
		false, // fetchAll
		50,    // pageSize
		previousFetch,
	)

	if err != nil {
		t.Fatalf("saveIncrementalResults() error = %v", err)
	}

	// Verify state file was created and can be loaded
	if _, err := os.Stat(stateFile); os.IsNotExist(err) {
		t.Error("State file was not created")
	}

	// Load and verify state
	savedState, err := state.LoadState(stateFile)
	if err != nil {
		t.Fatalf("Failed to load saved state: %v", err)
	}

	// Verify state was updated correctly
	if savedState.TotalFetched != 5 {
		t.Errorf("Expected TotalFetched=5, got %d", savedState.TotalFetched)
	}

	if savedState.LastPRNumber != 10 {
		t.Errorf("Expected LastPRNumber=10, got %d", savedState.LastPRNumber)
	}

	// Verify metadata file was created
	if _, err := os.Stat(metadataFile); os.IsNotExist(err) {
		t.Error("Metadata file was not created")
	}
}

// TestPerformIncrementalFetch tests the performIncrementalFetch function
func TestPerformIncrementalFetch(t *testing.T) {
	tests := []struct {
		name         string
		setupMock    func() github.Client
		prevState    *state.FetchState
		fetchContext *incrementalFetchContext
		fetchAll     bool
		wantErr      bool
		wantNewPRs   int
	}{
		{
			name: "successful incremental fetch with new PRs",
			setupMock: func() github.Client {
				// Return 3 new PRs
				return &github.MockClient{
					PullRequests: []github.PullRequest{
						{Number: 6, Title: "New PR 6", CreatedAt: time.Now().Add(-12 * time.Hour)},
						{Number: 7, Title: "New PR 7", CreatedAt: time.Now().Add(-6 * time.Hour)},
						{Number: 8, Title: "New PR 8", CreatedAt: time.Now()},
					},
					TotalPullRequests: 3,
				}
			},
			prevState: &state.FetchState{
				Version:       state.CurrentVersion,
				Repository:    "test/repo",
				LastFetchTime: time.Now().Add(-24 * time.Hour),
				LastPRNumber:  5,
				LastPRDate:    time.Now().Add(-48 * time.Hour),
				TotalFetched:  5,
			},
			fetchContext: &incrementalFetchContext{
				currentState: &state.FetchState{
					Repository:   "test/repo",
					LastPRNumber: 5,
					LastPRDate:   time.Now().Add(-48 * time.Hour),
				},
				tracker:       metadata.New(),
				previousFetch: nil,
				opts: github.FetchOptions{
					Since: timePtr(time.Now().Add(-48 * time.Hour)),
				},
				pageSize: 50,
			},
			fetchAll:   false,
			wantErr:    false,
			wantNewPRs: 3,
		},
		{
			name: "no new PRs",
			setupMock: func() github.Client {
				// Return no new PRs (all have numbers <= 10)
				return &github.MockClient{
					PullRequests:      []github.PullRequest{},
					TotalPullRequests: 0,
				}
			},
			prevState: &state.FetchState{
				Version:       state.CurrentVersion,
				Repository:    "test/repo",
				LastFetchTime: time.Now().Add(-1 * time.Hour),
				LastPRNumber:  10,
				LastPRDate:    time.Now().Add(-2 * time.Hour),
				TotalFetched:  10,
			},
			fetchContext: &incrementalFetchContext{
				currentState: &state.FetchState{
					Repository:   "test/repo",
					LastPRNumber: 10,
					LastPRDate:   time.Now().Add(-2 * time.Hour),
				},
				tracker:       metadata.New(),
				previousFetch: nil,
				opts: github.FetchOptions{
					Since: timePtr(time.Now().Add(-2 * time.Hour)),
				},
				pageSize: 50,
			},
			fetchAll:   false,
			wantErr:    false,
			wantNewPRs: 0,
		},
		{
			name: "network error",
			setupMock: func() github.Client {
				return &github.MockClient{
					ShouldFailNetwork: true,
				}
			},
			prevState: &state.FetchState{
				LastPRNumber: 5,
			},
			fetchContext: &incrementalFetchContext{
				currentState: &state.FetchState{
					Repository: "test/repo",
				},
				tracker:  metadata.New(),
				opts:     github.FetchOptions{},
				pageSize: 50,
			},
			fetchAll: false,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			outputFile := filepath.Join(tmpDir, "output.ndjson")

			// Create output writer
			file, err := os.Create(outputFile)
			if err != nil {
				t.Fatalf("Failed to create output file: %v", err)
			}
			defer file.Close()

			writer := output.NewWriter(file)
			client := tt.setupMock()

			newPRCount, err := performIncrementalFetch(
				context.Background(),
				client,
				"test",
				"repo",
				writer,
				tt.prevState,
				tt.fetchContext,
				tt.fetchAll,
			)

			if (err != nil) != tt.wantErr {
				t.Errorf("performIncrementalFetch() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				if newPRCount != tt.wantNewPRs {
					t.Errorf("Expected %d new PRs, got %d", tt.wantNewPRs, newPRCount)
				}

				// Verify output file contains the expected PRs
				data, err := os.ReadFile(outputFile)
				if err != nil {
					t.Fatalf("Failed to read output file: %v", err)
				}

				lines := strings.Split(strings.TrimSpace(string(data)), "\n")
				if strings.TrimSpace(string(data)) == "" {
					lines = []string{}
				}

				if len(lines) != tt.wantNewPRs {
					t.Errorf("Expected %d PRs in output file, got %d", tt.wantNewPRs, len(lines))
				}
			}
		})
	}
}

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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sirseerhq/sirseer-relay/internal/config"
	relaierrors "github.com/sirseerhq/sirseer-relay/internal/errors"
	"github.com/sirseerhq/sirseer-relay/internal/github"
	"github.com/sirseerhq/sirseer-relay/internal/metadata"
	"github.com/sirseerhq/sirseer-relay/internal/output"
	"github.com/sirseerhq/sirseer-relay/internal/state"
	"github.com/sirseerhq/sirseer-relay/pkg/version"
	"github.com/spf13/cobra"
)

// newFetchCommand creates the 'fetch' subcommand for the CLI.
// This command fetches pull request data from a specified GitHub repository
// and outputs it in NDJSON format. By default, it fetches only the first page
// of pull requests (up to 50). Use the --all flag to fetch all pull requests.
func newFetchCommand(configFile string) *cobra.Command {
	var (
		token          string
		outputFile     string
		outputDir      string
		metadataFile   string
		fetchAll       bool
		requestTimeout int
		batchSize      int
	)

	cmd := &cobra.Command{
		Use:   "fetch <org>/<repo>",
		Short: "Fetch pull request data from a GitHub repository",
		Long: `Fetch pull request data from a GitHub repository and output in NDJSON format.

The repository must be specified in the format: <org>/<repo>
For example: golang/go, kubernetes/kubernetes

Authentication is required via GitHub token:
  - Use --token flag to provide token directly
  - Or set GITHUB_TOKEN environment variable

Examples:
  # Fetch first page of PRs (most recent 50)
  sirseer-relay fetch golang/go

  # Fetch all PRs from a repository
  sirseer-relay fetch kubernetes/kubernetes --all

  # Fetch PRs created in 2024
  sirseer-relay fetch golang/go --since 2024-01-01 --until 2024-12-31

  # Resume from previous fetch (incremental update)
  sirseer-relay fetch golang/go --incremental

  # Save output to a file
  sirseer-relay fetch golang/go --all --output prs.ndjson`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load configuration
			cfg, err := config.LoadConfigForRepo(configFile, args[0])
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Validate configuration
			if validateErr := cfg.Validate(); validateErr != nil {
				return fmt.Errorf("invalid configuration: %w", validateErr)
			}

			// Apply config defaults and CLI overrides
			// CLI flags take precedence over config file
			if batchSize == 0 {
				batchSize = cfg.GetBatchSize(args[0])
			}

			// Create context with timeout
			ctx, cancel := context.WithTimeout(cmd.Context(), time.Duration(requestTimeout)*time.Second)
			defer cancel()

			// Get date flags
			since, err := cmd.Flags().GetString("since")
			if err != nil {
				return fmt.Errorf("failed to get since flag: %w", err)
			}
			until, err := cmd.Flags().GetString("until")
			if err != nil {
				return fmt.Errorf("failed to get until flag: %w", err)
			}
			incremental, err := cmd.Flags().GetBool("incremental")
			if err != nil {
				return fmt.Errorf("failed to get incremental flag: %w", err)
			}

			return runFetch(ctx, args[0], token, outputFile, outputDir, metadataFile, fetchAll, batchSize, since, until, incremental, cfg)
		},
	}

	// Define flags
	cmd.Flags().StringVar(&token, "token", "", "GitHub personal access token (overrides GITHUB_TOKEN env var)")
	cmd.Flags().StringVar(&outputFile, "output", "", "Output file path (default: stdout)")
	cmd.Flags().StringVar(&outputDir, "output-dir", "", "Output directory for generated files (default: ./output)")
	cmd.Flags().IntVar(&requestTimeout, "request-timeout", 180, "Request timeout in seconds (default: 3 minutes)")

	// Pagination flag
	cmd.Flags().BoolVar(&fetchAll, "all", false, "Fetch all pull requests from the repository")

	// Time window filtering
	cmd.Flags().String("since", "", "Fetch PRs created on or after this date (format: YYYY-MM-DD, RFC3339, or relative like 7d)")
	cmd.Flags().String("until", "", "Fetch PRs created on or before this date (format: YYYY-MM-DD, RFC3339, or relative like 7d)")

	// Incremental fetch
	cmd.Flags().Bool("incremental", false, "Continue from the last successful fetch (requires previous state file)")

	// Configuration
	cmd.Flags().IntVar(&batchSize, "batch-size", 0, "Number of PRs to fetch per API call (default from config or 50)")

	// Metadata
	cmd.Flags().StringVar(&metadataFile, "metadata-file", "", "Path to save fetch metadata (default: fetch-metadata.json)")

	return cmd
}

// runFetch executes the main fetch logic. It parses the repository argument,
// validates the GitHub token, creates the output writer, and delegates to either
// fetchFirstPageWithOptions (default) or fetchAllPullRequestsWithOptions (with --all flag).
// Returns an error if any step fails, which will be mapped to an appropriate exit code.
func runFetch(ctx context.Context, repoArg, tokenFlag, outputFile, outputDir, metadataFile string, fetchAll bool, batchSize int, since, until string, incremental bool, cfg *config.Config) error {
	// Create default client factory
	clientFactory := func(token string) github.Client {
		return github.NewGraphQLClient(token, cfg.GitHub.GraphQLEndpoint)
	}
	return RunFetchWithClient(ctx, repoArg, tokenFlag, outputFile, outputDir, metadataFile, fetchAll, batchSize, since, until, incremental, cfg, clientFactory)
}

// RunFetchWithClient executes the main fetch logic with an injectable client factory.
// This allows for easier testing with mock clients.
func RunFetchWithClient(ctx context.Context, repoArg, tokenFlag, outputFile, outputDir, metadataFile string, fetchAll bool, batchSize int, since, until string, incremental bool, cfg *config.Config, clientFactory func(token string) github.Client) error {
	// Parse repository argument
	owner, repo, err := parseRepository(repoArg)
	if err != nil {
		return err
	}

	// Get GitHub token
	token := getToken(tokenFlag, cfg.GitHub.TokenEnv)
	if token == "" {
		return fmt.Errorf("GitHub token not found. Set %s or use --token flag", cfg.GitHub.TokenEnv)
	}

	// Create output writer
	// If no output flags specified, use default output directory
	if outputFile == "" && outputDir == "" {
		outputDir = "output"
	}
	writer, generatedOutputFile, err := createOutputWriter(outputFile, outputDir, owner, repo)
	if err != nil {
		return err
	}
	defer writer.Close()

	// Create GitHub client using the factory
	client := clientFactory(token)

	// Parse and validate date flags
	sinceTime, untilTime, err := parseDateFlags(since, until)
	if err != nil {
		return err
	}

	// Handle incremental fetch
	if incremental {
		return fetchIncremental(ctx, client, owner, repo, writer, metadataFile, sinceTime, untilTime, fetchAll)
	}

	// Build fetch options with batch size
	opts := github.FetchOptions{
		Since:    sinceTime,
		Until:    untilTime,
		PageSize: batchSize,
	}

	// Handle metadata file path
	if metadataFile == "" && generatedOutputFile != "" {
		// Auto-generate metadata filename based on output file
		metadataFile = strings.TrimSuffix(generatedOutputFile, ".ndjson") + "-metadata.json"
	}

	// Fetch all PRs if --all flag is set
	if fetchAll {
		return fetchAllPullRequestsWithOptions(ctx, client, owner, repo, writer, metadataFile, opts)
	}

	// Default behavior: fetch first page only
	return fetchFirstPageWithOptions(ctx, client, owner, repo, writer, metadataFile, opts)
}

// createOutputWriter creates an output writer based on the output file parameter.
// If outputFile is empty, it generates a timestamped filename in the output directory.
// Returns the writer and the actual output file path used.
func createOutputWriter(outputFile, outputDir, owner, repo string) (output.OutputWriter, string, error) {
	// If explicit output file is specified
	if outputFile != "" {
		// Special case: "-" means stdout
		if outputFile == "-" {
			return output.NewWriter(os.Stdout), "", nil
		}
		// Otherwise use the specified file
		fileWriter, err := output.NewFileWriter(outputFile)
		if err != nil {
			return nil, "", fmt.Errorf("failed to create output file: %w", err)
		}
		return fileWriter, outputFile, nil
	}

	// If writing to stdout (no output file or dir), return stdout writer
	if outputDir == "" && outputFile == "" {
		return output.NewWriter(os.Stdout), "", nil
	}

	// Generate timestamped filename in output directory
	if outputDir == "" {
		outputDir = "output"
	}

	// Create directory structure: output/<org>/<repo>/
	dirPath := filepath.Join(outputDir, owner, repo)
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return nil, "", fmt.Errorf("failed to create output directory: %w", err)
	}

	// Generate timestamped filename
	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("%s-%s.ndjson", repo, timestamp)
	fullPath := filepath.Join(dirPath, filename)

	// Create file writer
	fileWriter, err := output.NewFileWriter(fullPath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create output file: %w", err)
	}

	// Print the output path so user knows where to find it
	fmt.Fprintf(os.Stderr, "Output file: %s\n", fullPath)

	return fileWriter, fullPath, nil
}

// parseDateFlags parses and validates the since and until date flags.
// It returns parsed time pointers and ensures that since is before until if both are provided.
func parseDateFlags(since, until string) (sinceTime *time.Time, untilTime *time.Time, err error) {
	if since != "" {
		parsed, parseErr := parseDate(since)
		if parseErr != nil {
			return nil, nil, fmt.Errorf("invalid --since date format: %w", parseErr)
		}
		sinceTime = &parsed
	}

	if until != "" {
		parsed, parseErr := parseDate(until)
		if parseErr != nil {
			return nil, nil, fmt.Errorf("invalid --until date format: %w", parseErr)
		}
		untilTime = &parsed
	}

	// Validate date range
	if sinceTime != nil && untilTime != nil && sinceTime.After(*untilTime) {
		return nil, nil, fmt.Errorf("--since date must be before --until date")
	}

	return sinceTime, untilTime, nil
}

// parseDate parses a date string in various formats.
// Supports:
//   - RFC3339: 2024-01-15T10:00:00Z
//   - Date only: 2024-01-15 (interpreted as start of day UTC)
//   - Relative: 7d, 1w, 1m (days, weeks, months ago)
func parseDate(dateStr string) (time.Time, error) {
	// Try RFC3339 first
	if t, err := time.Parse(time.RFC3339, dateStr); err == nil {
		return t, nil
	}

	// Try date only format (YYYY-MM-DD)
	if t, err := time.Parse("2006-01-02", dateStr); err == nil {
		// Return start of day UTC
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC), nil
	}

	// Try relative date formats
	if strings.HasSuffix(dateStr, "d") {
		days := strings.TrimSuffix(dateStr, "d")
		if n, err := time.ParseDuration(days + "h"); err == nil {
			return time.Now().UTC().Add(-n * 24), nil
		}
	}

	return time.Time{}, fmt.Errorf("unsupported date format. Use YYYY-MM-DD, RFC3339, or relative (7d, 1w)")
}

// parseRepository parses a repository argument in the format "owner/repo"
// into separate owner and repository name components. It validates that
// both components are present and non-empty.
// Example: "golang/go" returns ("golang", "go", nil)
func parseRepository(repoArg string) (owner, repo string, err error) {
	parts := strings.Split(repoArg, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid repository format. Expected: <org>/<repo>, got: %s", repoArg)
	}

	owner = strings.TrimSpace(parts[0])
	repo = strings.TrimSpace(parts[1])

	if owner == "" || repo == "" {
		return "", "", fmt.Errorf("invalid repository format. Expected: <org>/<repo>, got: %s", repoArg)
	}

	return owner, repo, nil
}

// getToken retrieves the GitHub authentication token. It first checks
// the --token flag value, and if empty, falls back to the configured
// environment variable. This allows users flexibility in how they provide
// authentication credentials.
func getToken(flagToken, envVar string) string {
	if flagToken != "" {
		return flagToken
	}
	return os.Getenv(envVar)
}

// saveMetadata saves fetch metadata to the specified file or default location
func saveMetadata(fetchMetadata *metadata.FetchMetadata, metadataFile string) error {
	// Determine the output path
	outputPath := metadataFile
	if outputPath == "" {
		outputPath = "fetch-metadata.json"
	}

	// Convert to absolute path if relative
	if !filepath.IsAbs(outputPath) {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}
		outputPath = filepath.Join(cwd, outputPath)
	}

	// Ensure parent directory exists
	parentDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		return fmt.Errorf("failed to create metadata directory: %w", err)
	}

	// Write to temporary file first for atomicity
	tmpFile := outputPath + ".tmp"
	file, err := os.Create(tmpFile)
	if err != nil {
		return fmt.Errorf("failed to create metadata file: %w", err)
	}

	// Write JSON with proper formatting
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(fetchMetadata); err != nil {
		file.Close()
		os.Remove(tmpFile)
		return fmt.Errorf("failed to write metadata: %w", err)
	}

	if err := file.Close(); err != nil {
		os.Remove(tmpFile)
		return fmt.Errorf("failed to close metadata file: %w", err)
	}

	// Atomically rename to final location
	if err := os.Rename(tmpFile, outputPath); err != nil {
		return fmt.Errorf("failed to save metadata file: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Fetch metadata saved to: %s\n", outputPath)
	return nil
}

// fetchFirstPageWithOptions fetches the first page of pull requests with custom options.
func fetchFirstPageWithOptions(ctx context.Context, client github.Client, owner, repo string, writer output.OutputWriter, metadataFile string, opts github.FetchOptions) error {
	if opts.PageSize <= 0 {
		opts.PageSize = 50
	}

	// Initialize metadata tracker
	tracker := metadata.New()

	// Show progress
	fmt.Fprintf(os.Stderr, "Fetching pull requests from %s/%s...", owner, repo)

	// Fetch PRs using search API
	page, err := client.FetchPullRequestsSearch(ctx, owner, repo, opts)
	if err != nil {
		// Clear progress line
		fmt.Fprintf(os.Stderr, "\r\033[K")
		return err
	}

	// Track API call
	tracker.IncrementAPICall()

	// Write PRs to output
	prCount := 0
	for _, pr := range page.PullRequests {
		if err := writer.Write(pr); err != nil {
			return fmt.Errorf("failed to write PR: %w", err)
		}
		prCount++

		// Update metadata tracker
		tracker.UpdatePRStats(pr.Number, pr.CreatedAt, pr.UpdatedAt)

		// Update progress
		fmt.Fprintf(os.Stderr, "\rFetching pull requests from %s/%s... %d PRs fetched", owner, repo, prCount)
	}

	// Final message
	fmt.Fprintf(os.Stderr, "\r\033[K") // Clear progress line

	if prCount > 0 {
		fmt.Fprintf(os.Stderr, "Successfully fetched %d pull requests\n", prCount)

		// Generate and save metadata for single page fetch
		params := metadata.FetchParams{
			Organization: owner,
			Repository:   repo,
			Since:        opts.Since,
			Until:        opts.Until,
			FetchAll:     false,
			BatchSize:    opts.PageSize,
		}

		fetchMetadata := tracker.GenerateMetadata(version.Version, params, false, nil)

		// Save metadata
		if err := saveMetadata(fetchMetadata, metadataFile); err != nil {
			// Don't fail the fetch, just warn
			fmt.Fprintf(os.Stderr, "Warning: failed to save fetch metadata: %v\n", err)
		}
	} else {
		fmt.Fprintf(os.Stderr, "No pull requests found in %s/%s\n", owner, repo)
	}

	return nil
}

// fetchAllPullRequestsWithOptions fetches all pull requests with custom options.
func fetchAllPullRequestsWithOptions(ctx context.Context, client github.Client, owner, repo string, writer output.OutputWriter, metadataFile string, opts github.FetchOptions) error {
	// First, get repository info for total PR count
	repoInfo, err := client.GetRepositoryInfo(ctx, owner, repo)
	if err != nil {
		return fmt.Errorf("failed to get repository info: %w", err)
	}

	totalPRs := repoInfo.TotalPullRequests
	if totalPRs == 0 {
		fmt.Fprintf(os.Stderr, "No pull requests found in %s/%s\n", owner, repo)
		return nil
	}

	// Initialize metadata tracker
	tracker := metadata.New()

	// Initialize progress tracking
	progress := initializeProgress(totalPRs, owner, repo)

	for progress.hasMore {
		progress.pageNum++
		pageOpts := github.FetchOptions{
			PageSize: progress.pageSize,
			After:    progress.cursor,
			Since:    opts.Since,
			Until:    opts.Until,
		}

		// Fetch page with retry on complexity errors
		page, err := fetchWithComplexityRetry(ctx, client, owner, repo, pageOpts, &progress.pageSize)
		if err != nil {
			fmt.Fprintf(os.Stderr, "\r\033[K") // Clear progress line
			return err
		}

		// Track API call
		tracker.IncrementAPICall()

		// Process batch of PRs
		if err := processFetchBatch(page.PullRequests, writer, tracker, progress); err != nil {
			return err
		}

		progress.cursor = page.EndCursor
		progress.hasMore = page.HasNextPage
	}

	// Finalize results and save state/metadata
	return finalizeFetchResults(owner, repo, progress, tracker, metadataFile, opts)
}

// progressTracker holds the state for tracking fetch progress.
type progressTracker struct {
	allPRsProcessed int
	cursor          string
	hasMore         bool
	startTime       time.Time
	pageSize        int
	pageNum         int
	lastPRNumber    int
	lastPRDate      time.Time
	totalPRs        int
}

// initializeProgress sets up progress tracking for fetching all PRs.
func initializeProgress(totalPRs int, owner, repo string) *progressTracker {
	fmt.Fprintf(os.Stderr, "Fetching all %d pull requests from %s/%s...\n", totalPRs, owner, repo)
	return &progressTracker{
		allPRsProcessed: 0,
		cursor:          "",
		hasMore:         true,
		startTime:       time.Now(),
		pageSize:        50,
		pageNum:         0,
		lastPRNumber:    0,
		totalPRs:        totalPRs,
	}
}

// processFetchBatch writes PRs to output and updates tracking information.
func processFetchBatch(prs []github.PullRequest, writer output.OutputWriter, tracker *metadata.Tracker, progress *progressTracker) error {
	for _, pr := range prs {
		if err := writer.Write(pr); err != nil {
			return fmt.Errorf("failed to write PR: %w", err)
		}
		progress.allPRsProcessed++

		// Update metadata tracker
		tracker.UpdatePRStats(pr.Number, pr.CreatedAt, pr.UpdatedAt)

		// Track last PR for state
		if pr.Number > progress.lastPRNumber {
			progress.lastPRNumber = pr.Number
		}
		if pr.CreatedAt.After(progress.lastPRDate) {
			progress.lastPRDate = pr.CreatedAt
		}

		// Update progress with ETA
		updateProgress(progress.allPRsProcessed, progress.totalPRs, progress.pageNum, progress.startTime)
	}
	return nil
}

// finalizeFetchResults saves state and metadata after completing the fetch.
func finalizeFetchResults(owner, repo string, progress *progressTracker, tracker *metadata.Tracker, metadataFile string, opts github.FetchOptions) error {
	// Final message
	fmt.Fprintf(os.Stderr, "\r\033[K") // Clear progress line
	elapsed := time.Since(progress.startTime)
	fmt.Fprintf(os.Stderr, "Successfully fetched all %d pull requests in %s\n", progress.allPRsProcessed, elapsed.Round(time.Second))

	// Save state if we fetched any PRs
	if progress.allPRsProcessed > 0 && progress.lastPRNumber > 0 {
		repoPath := fmt.Sprintf("%s/%s", owner, repo)
		stateFile := state.GetStateFilePath(repoPath)

		fetchState := &state.FetchState{
			Repository:    repoPath,
			LastFetchID:   fmt.Sprintf("full-%d", time.Now().Unix()),
			LastPRNumber:  progress.lastPRNumber,
			LastPRDate:    progress.lastPRDate,
			LastFetchTime: time.Now().UTC(),
			TotalFetched:  progress.allPRsProcessed,
		}

		if err := state.SaveState(fetchState, stateFile); err != nil {
			// Don't fail the fetch, just warn
			fmt.Fprintf(os.Stderr, "Warning: failed to save state for incremental fetch: %v\n", err)
		}

		// Generate and save metadata
		params := metadata.FetchParams{
			Organization: owner,
			Repository:   repo,
			Since:        opts.Since,
			Until:        opts.Until,
			FetchAll:     true,
			BatchSize:    progress.pageSize,
		}

		fetchMetadata := tracker.GenerateMetadata(version.Version, params, false, nil)

		// Save metadata
		if err := saveMetadata(fetchMetadata, metadataFile); err != nil {
			// Don't fail the fetch, just warn
			fmt.Fprintf(os.Stderr, "Warning: failed to save fetch metadata: %v\n", err)
		}
	}

	return nil
}

// fetchWithComplexityRetry implements intelligent retry logic for GraphQL query complexity errors.
// GitHub's GraphQL API has complexity limits to prevent expensive queries. When a query exceeds
// these limits, this function automatically retries with a smaller batch size.
//
// The retry strategy:
//   - Detects GraphQL complexity errors using the ErrQueryComplexity sentinel error
//   - Reduces the page size by half on each retry (down to a minimum of 5)
//   - Retries up to 4 times before giving up
//   - Preserves the reduced page size for subsequent requests to avoid repeated errors
//
// This approach ensures that the tool can handle repositories with complex PR data (many
// reviews, comments, or files) by automatically adjusting to stay within API limits.
func fetchWithComplexityRetry(ctx context.Context, client github.Client, owner, repo string, opts github.FetchOptions, pageSize *int) (*github.PullRequestPage, error) {
	maxRetries := 4
	minPageSize := 5

	for attempt := 0; attempt < maxRetries; attempt++ {
		opts.PageSize = *pageSize
		page, err := client.FetchPullRequestsSearch(ctx, owner, repo, opts)

		if err == nil {
			return page, nil
		}

		// Check if it's a complexity error
		if errors.Is(err, relaierrors.ErrQueryComplexity) && *pageSize > minPageSize {
			// Reduce page size by half
			*pageSize /= 2
			if *pageSize < minPageSize {
				*pageSize = minPageSize
			}

			fmt.Fprintf(os.Stderr, "\r\033[K") // Clear line
			fmt.Fprintf(os.Stderr, "Query complexity limit hit. Reducing page size to %d...\n", *pageSize)
			continue
		}

		// Not a complexity error or can't reduce further
		return nil, err
	}

	return nil, fmt.Errorf("failed after %d attempts to reduce query complexity", maxRetries)
}

// loadAndValidateIncrementalState loads the previous fetch state and validates it matches the current repository.
// Returns the previous state or an error with appropriate user-friendly message.
func loadAndValidateIncrementalState(stateFile, repoPath string) (*state.FetchState, error) {
	prevState, err := state.LoadState(stateFile)
	if err != nil {
		if strings.Contains(err.Error(), "no previous fetch state found") {
			return nil, fmt.Errorf("no previous fetch state found for %s. To start an incremental fetch, first run a full fetch without --incremental", repoPath)
		}
		if strings.Contains(err.Error(), "corrupted") {
			return nil, fmt.Errorf("state file is corrupted. To recover: Delete '%s' and run again. Your previous data in the output file is safe", stateFile)
		}
		if strings.Contains(err.Error(), "incompatible") {
			return nil, fmt.Errorf("state file version is incompatible. This usually means the tool has been updated. To recover: Delete '%s' and run a full fetch", stateFile)
		}
		return nil, fmt.Errorf("failed to load state: %w", err)
	}

	// Verify repository matches
	if prevState.Repository != repoPath {
		return nil, fmt.Errorf("state file is for repository %s but current command is for %s", prevState.Repository, repoPath)
	}

	return prevState, nil
}

// prepareIncrementalMetadata loads previous metadata and prepares tracking for the new fetch.
// Returns the metadata tracker and previous fetch reference.
func prepareIncrementalMetadata(stateDir, repoPath string) (*metadata.Tracker, *metadata.FetchRef) {
	tracker := metadata.New()

	previousMetadata, err := metadata.LoadLatestMetadata(stateDir, repoPath)
	if err != nil {
		// Log warning but continue - metadata is optional
		fmt.Fprintf(os.Stderr, "Warning: failed to load previous metadata: %v\n", err)
	}

	var previousFetch *metadata.FetchRef
	if previousMetadata != nil {
		previousFetch = &metadata.FetchRef{
			FetchID:     previousMetadata.FetchID,
			CompletedAt: previousMetadata.Results.CompletedAt,
		}
	}

	return tracker, previousFetch
}

// processIncrementalPR processes a single PR during incremental fetch.
// Returns true if the PR was new and written, false if it was skipped.
func processIncrementalPR(pr *github.PullRequest, prevState, currentState *state.FetchState, writer output.OutputWriter, tracker *metadata.Tracker) (bool, error) {
	// Skip PRs we've already seen (based on PR number)
	if pr.Number <= prevState.LastPRNumber {
		return false, nil
	}

	// Write new PR
	if err := writer.Write(pr); err != nil {
		return false, fmt.Errorf("failed to write PR: %w", err)
	}

	// Update metadata tracker
	tracker.UpdatePRStats(pr.Number, pr.CreatedAt, pr.UpdatedAt)

	// Update state tracking
	if pr.Number > currentState.LastPRNumber {
		currentState.LastPRNumber = pr.Number
	}
	if pr.CreatedAt.After(currentState.LastPRDate) {
		currentState.LastPRDate = pr.CreatedAt
	}

	return true, nil
}

// saveIncrementalResults saves the state and metadata after an incremental fetch.
func saveIncrementalResults(currentState *state.FetchState, stateFile string, newPRCount int, tracker *metadata.Tracker, metadataFile, owner, repo string, opts github.FetchOptions, fetchAll bool, pageSize int, previousFetch *metadata.FetchRef) error {
	// Update final state
	currentState.TotalFetched = newPRCount

	// Save state
	if err := state.SaveState(currentState, stateFile); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	// Generate and save metadata if we fetched any PRs
	if newPRCount > 0 {
		params := metadata.FetchParams{
			Organization: owner,
			Repository:   repo,
			Since:        opts.Since,
			Until:        opts.Until,
			FetchAll:     fetchAll,
			BatchSize:    pageSize,
		}

		fetchMetadata := tracker.GenerateMetadata(version.Version, params, true, previousFetch)

		if err := saveMetadata(fetchMetadata, metadataFile); err != nil {
			// Don't fail the fetch, just warn
			fmt.Fprintf(os.Stderr, "Warning: failed to save fetch metadata: %v\n", err)
		}
	}

	return nil
}

// fetchIncremental handles incremental fetching by loading previous state and resuming.
func fetchIncremental(ctx context.Context, client github.Client, owner, repo string, writer output.OutputWriter, metadataFile string, sinceTime, untilTime *time.Time, fetchAll bool) error {
	repoPath := fmt.Sprintf("%s/%s", owner, repo)
	stateFile := state.GetStateFilePath(repoPath)

	// Load and validate previous state
	prevState, err := loadAndValidateIncrementalState(stateFile, repoPath)
	if err != nil {
		return err
	}

	// Prepare incremental fetch context
	fetchCtx, err := prepareIncrementalFetch(prevState, repoPath, sinceTime, untilTime)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Resuming from PR #%d (created %s)\n", prevState.LastPRNumber, prevState.LastPRDate.Format("2006-01-02"))

	// Perform the incremental fetch
	newPRCount, err := performIncrementalFetch(ctx, client, owner, repo, writer, prevState, fetchCtx, fetchAll)
	if err != nil {
		return err
	}

	// Save state and metadata
	return saveIncrementalResults(fetchCtx.currentState, stateFile, newPRCount, fetchCtx.tracker, metadataFile, owner, repo, fetchCtx.opts, fetchAll, fetchCtx.pageSize, fetchCtx.previousFetch)
}

// incrementalFetchContext holds the context needed for an incremental fetch.
type incrementalFetchContext struct {
	currentState  *state.FetchState
	tracker       *metadata.Tracker
	previousFetch *metadata.FetchRef
	opts          github.FetchOptions
	pageSize      int
}

// prepareIncrementalFetch sets up the context for an incremental fetch operation.
func prepareIncrementalFetch(prevState *state.FetchState, repoPath string, sinceTime, untilTime *time.Time) (*incrementalFetchContext, error) {
	// Use the last PR date as the starting point if not specified
	if sinceTime == nil {
		sinceTime = &prevState.LastPRDate
	}

	// Build fetch options
	opts := github.FetchOptions{
		Since: sinceTime,
		Until: untilTime,
	}

	// Prepare metadata tracking
	stateDir := filepath.Dir(state.GetStateFilePath(repoPath))
	tracker, previousFetch := prepareIncrementalMetadata(stateDir, repoPath)

	// Track state for this fetch
	currentState := &state.FetchState{
		Repository:    repoPath,
		LastFetchID:   fmt.Sprintf("inc-%d", time.Now().Unix()),
		LastPRNumber:  prevState.LastPRNumber,
		LastPRDate:    prevState.LastPRDate,
		LastFetchTime: time.Now().UTC(),
		TotalFetched:  0,
	}

	return &incrementalFetchContext{
		currentState:  currentState,
		tracker:       tracker,
		previousFetch: previousFetch,
		opts:          opts,
		pageSize:      50,
	}, nil
}

// performIncrementalFetch executes the main fetch loop for incremental updates.
func performIncrementalFetch(ctx context.Context, client github.Client, owner, repo string, writer output.OutputWriter, prevState *state.FetchState, fetchCtx *incrementalFetchContext, fetchAll bool) (int, error) {
	var (
		hasMore    = true
		cursor     = ""
		pageNum    = 0
		newPRCount = 0
	)

	for hasMore {
		pageNum++
		pageOpts := github.FetchOptions{
			PageSize: fetchCtx.pageSize,
			After:    cursor,
			Since:    fetchCtx.opts.Since,
			Until:    fetchCtx.opts.Until,
		}

		// Fetch page
		page, err := fetchWithComplexityRetry(ctx, client, owner, repo, pageOpts, &fetchCtx.pageSize)
		if err != nil {
			return 0, err
		}

		// Track API call
		fetchCtx.tracker.IncrementAPICall()

		// Process PRs with deduplication
		for _, pr := range page.PullRequests {
			isNew, err := processIncrementalPR(&pr, prevState, fetchCtx.currentState, writer, fetchCtx.tracker)
			if err != nil {
				return 0, err
			}
			if isNew {
				newPRCount++
			}
		}

		cursor = page.EndCursor
		hasMore = page.HasNextPage && (fetchAll || newPRCount == 0)
	}

	fmt.Fprintf(os.Stderr, "Successfully fetched %d new pull requests\n", newPRCount)
	return newPRCount, nil
}

// updateProgress displays a real-time progress indicator with percentage completion and ETA.
// It uses ANSI escape sequences to update the progress line in place, providing a smooth
// user experience without scrolling the terminal.
//
// The progress indicator shows:
//   - Current number of PRs processed vs total
//   - Percentage completion with one decimal place
//   - Current page number being processed
//   - Estimated time to completion based on current processing rate
//
// The ETA calculation uses the elapsed time and current progress to extrapolate the
// remaining time, providing users with a realistic expectation of completion time.
func updateProgress(current, total, pageNum int, startTime time.Time) {
	if total == 0 {
		return
	}

	percent := float64(current) * 100 / float64(total)
	elapsed := time.Since(startTime)

	// Calculate ETA
	var eta string
	if current > 0 {
		totalTime := elapsed.Seconds() * float64(total) / float64(current)
		remaining := time.Duration(totalTime-elapsed.Seconds()) * time.Second

		if remaining > 0 {
			eta = fmt.Sprintf(" | ETA: %s", remaining.Round(time.Second))
		}
	}

	fmt.Fprintf(os.Stderr, "\rProgress: %d / %d PRs [%.1f%%] | Page %d%s",
		current, total, percent, pageNum, eta)
}

// mapErrorToExitCode converts internal error types to appropriate shell exit codes.
// This provides meaningful exit codes for scripting and automation:
//   - 0: Success (no error)
//   - 1: General error
//   - 2: Authentication/authorization errors (invalid token, repo not found, rate limit)
//   - 3: Network errors
func mapErrorToExitCode(err error) int {
	if err == nil {
		return 0
	}

	// Check for specific error types
	if errors.Is(err, relaierrors.ErrInvalidToken) ||
		errors.Is(err, relaierrors.ErrRepoNotFound) ||
		errors.Is(err, relaierrors.ErrRateLimit) {
		return 2 // Authentication/authorization errors
	}

	if errors.Is(err, relaierrors.ErrNetworkFailure) {
		return 3 // Network errors
	}

	return 1 // General error
}

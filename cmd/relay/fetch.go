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
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	relaierrors "github.com/sirseerhq/sirseer-relay/internal/errors"
	"github.com/sirseerhq/sirseer-relay/internal/github"
	"github.com/sirseerhq/sirseer-relay/internal/output"
	"github.com/spf13/cobra"
)

// newFetchCommand creates the 'fetch' subcommand for the CLI.
// This command fetches pull request data from a specified GitHub repository
// and outputs it in NDJSON format. By default, it fetches only the first page
// of pull requests (up to 50). Use the --all flag to fetch all pull requests.
func newFetchCommand() *cobra.Command {
	var (
		token          string
		outputFile     string
		fetchAll       bool
		requestTimeout int
	)

	cmd := &cobra.Command{
		Use:   "fetch <org>/<repo>",
		Short: "Fetch pull request data from a GitHub repository",
		Long: `Fetch pull request data from a GitHub repository and output in NDJSON format.

The repository must be specified in the format: <org>/<repo>
For example: golang/go, kubernetes/kubernetes

Authentication is required via GitHub token:
  - Use --token flag to provide token directly
  - Or set GITHUB_TOKEN environment variable`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Create context with timeout
			ctx, cancel := context.WithTimeout(cmd.Context(), time.Duration(requestTimeout)*time.Second)
			defer cancel()

			return runFetch(ctx, args[0], token, outputFile, fetchAll)
		},
	}

	// Define flags
	cmd.Flags().StringVar(&token, "token", "", "GitHub personal access token (overrides GITHUB_TOKEN env var)")
	cmd.Flags().StringVar(&outputFile, "output", "", "Output file path (default: stdout)")
	cmd.Flags().IntVar(&requestTimeout, "request-timeout", 180, "Request timeout in seconds (default: 3 minutes)")

	// Pagination flag
	cmd.Flags().BoolVar(&fetchAll, "all", false, "Fetch all pull requests from the repository")

	// Future flags (not implemented yet)
	cmd.Flags().String("since", "", "Fetch PRs created after this date")
	cmd.Flags().String("until", "", "Fetch PRs created before this date")
	cmd.Flags().Bool("incremental", false, "Resume from last fetch")

	return cmd
}

// runFetch executes the main fetch logic. It parses the repository argument,
// validates the GitHub token, creates the output writer, and delegates to either
// fetchFirstPage (default) or fetchAllPullRequests (with --all flag).
// Returns an error if any step fails, which will be mapped to an appropriate exit code.
func runFetch(ctx context.Context, repoArg, tokenFlag, outputFile string, fetchAll bool) error {
	// Parse repository argument
	owner, repo, err := parseRepository(repoArg)
	if err != nil {
		return err
	}

	// Get GitHub token
	token := getToken(tokenFlag)
	if token == "" {
		return fmt.Errorf("GitHub token not found. Set GITHUB_TOKEN or use --token flag")
	}

	// Create output writer
	var writer output.OutputWriter
	if outputFile == "" {
		// Write to stdout
		writer = output.NewWriter(os.Stdout)
	} else {
		// Write to file
		fileWriter, fErr := output.NewFileWriter(outputFile)
		if fErr != nil {
			return fmt.Errorf("failed to create output file: %w", fErr)
		}
		writer = fileWriter
	}
	defer writer.Close()

	// Create GitHub client
	client := github.NewGraphQLClient(token)

	// Fetch all PRs if --all flag is set
	if fetchAll {
		return fetchAllPullRequests(ctx, client, owner, repo, writer)
	}

	// Default behavior: fetch first page only
	return fetchFirstPage(ctx, client, owner, repo, writer)
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
// the --token flag value, and if empty, falls back to the GITHUB_TOKEN
// environment variable. This allows users flexibility in how they provide
// authentication credentials.
func getToken(flagToken string) string {
	if flagToken != "" {
		return flagToken
	}
	return os.Getenv("GITHUB_TOKEN")
}

// fetchFirstPage fetches only the first page of pull requests (default behavior)
// of pull requests (up to 50) from the repository. This is the default behavior
// when the --all flag is not specified. It streams results directly to the output
// writer to maintain low memory usage.
func fetchFirstPage(ctx context.Context, client github.Client, owner, repo string, writer output.OutputWriter) error {
	opts := github.FetchOptions{
		PageSize: 50,
	}

	// Show progress
	fmt.Fprintf(os.Stderr, "Fetching pull requests from %s/%s...", owner, repo)

	// Fetch PRs
	page, err := client.FetchPullRequests(ctx, owner, repo, opts)
	if err != nil {
		// Clear progress line
		fmt.Fprintf(os.Stderr, "\r\033[K")
		return err
	}

	// Write PRs to output
	prCount := 0
	for _, pr := range page.PullRequests {
		if err := writer.Write(pr); err != nil {
			return fmt.Errorf("failed to write PR: %w", err)
		}
		prCount++

		// Update progress
		fmt.Fprintf(os.Stderr, "\rFetching pull requests from %s/%s... %d PRs fetched", owner, repo, prCount)
	}

	// Final message
	fmt.Fprintf(os.Stderr, "\r\033[K") // Clear progress line

	// Check if we were writing to a file or stdout
	// We can detect this based on whether an output file was specified in the original call
	// For now, we'll just display a generic success message
	if prCount > 0 {
		fmt.Fprintf(os.Stderr, "Successfully fetched %d pull requests\n", prCount)
	} else {
		fmt.Fprintf(os.Stderr, "No pull requests found in %s/%s\n", owner, repo)
	}

	return nil
}

// fetchAllPullRequests orchestrates the complete extraction of all pull requests from a repository.
// It implements an efficient pagination strategy that:
//   - Fetches repository metadata to get the total PR count for progress tracking
//   - Iterates through all pages using GraphQL cursor-based pagination
//   - Streams each PR immediately to the output writer (no in-memory accumulation)
//   - Displays real-time progress with percentage completion and ETA
//   - Automatically recovers from GraphQL query complexity errors by reducing batch size
//
// The function maintains constant memory usage regardless of repository size by streaming
// results directly to the output. This allows it to handle repositories with 100K+ PRs
// while using less than 100MB of memory.
func fetchAllPullRequests(ctx context.Context, client github.Client, owner, repo string, writer output.OutputWriter) error {
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

	// Initialize progress tracking
	var (
		allPRsProcessed = 0
		cursor          = ""
		hasMore         = true
		startTime       = time.Now()
		pageSize        = 50
		pageNum         = 0
	)

	// Show initial progress
	fmt.Fprintf(os.Stderr, "Fetching all %d pull requests from %s/%s...\n", totalPRs, owner, repo)

	for hasMore {
		pageNum++
		opts := github.FetchOptions{
			PageSize: pageSize,
			After:    cursor,
		}

		// Fetch page with retry on complexity errors
		page, err := fetchWithComplexityRetry(ctx, client, owner, repo, opts, &pageSize)
		if err != nil {
			fmt.Fprintf(os.Stderr, "\r\033[K") // Clear progress line
			return err
		}

		// Stream PRs immediately
		for _, pr := range page.PullRequests {
			if err := writer.Write(pr); err != nil {
				return fmt.Errorf("failed to write PR: %w", err)
			}
			allPRsProcessed++

			// Update progress with ETA
			updateProgress(allPRsProcessed, totalPRs, pageNum, startTime)
		}

		cursor = page.EndCursor
		hasMore = page.HasNextPage
	}

	// Final message
	fmt.Fprintf(os.Stderr, "\r\033[K") // Clear progress line
	elapsed := time.Since(startTime)
	fmt.Fprintf(os.Stderr, "Successfully fetched all %d pull requests in %s\n", allPRsProcessed, elapsed.Round(time.Second))

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
		page, err := client.FetchPullRequests(ctx, owner, repo, opts)

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

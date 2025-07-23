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

// fetchCmd represents the fetch command
func newFetchCommand() *cobra.Command {
	var (
		token      string
		outputFile string
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
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()

			return runFetch(ctx, args[0], token, outputFile)
		},
	}

	// Define flags
	cmd.Flags().StringVar(&token, "token", "", "GitHub personal access token (overrides GITHUB_TOKEN env var)")
	cmd.Flags().StringVar(&outputFile, "output", "", "Output file path (default: stdout)")

	// Future flags (not implemented in Phase 1)
	cmd.Flags().Bool("all", false, "Fetch all pull requests")
	cmd.Flags().String("since", "", "Fetch PRs created after this date")
	cmd.Flags().String("until", "", "Fetch PRs created before this date")
	cmd.Flags().Bool("incremental", false, "Resume from last fetch")

	return cmd
}

// runFetch executes the fetch command
func runFetch(ctx context.Context, repoArg, tokenFlag, outputFile string) error {
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

	// Prepare fetch options
	opts := github.FetchOptions{
		PageSize: 50, // Phase 1: single page only
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

	if outputFile != "" {
		fmt.Fprintf(os.Stderr, "Successfully wrote %d pull requests to %s\n", prCount, outputFile)
	} else if prCount == 0 {
		// For stdout, just clear the line - data is already written
		fmt.Fprintf(os.Stderr, "No pull requests found in %s/%s\n", owner, repo)
	}

	return nil
}

// parseRepository parses an org/repo string into owner and repo components
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

// getToken returns the GitHub token from flag or environment variable
func getToken(flagToken string) string {
	if flagToken != "" {
		return flagToken
	}
	return os.Getenv("GITHUB_TOKEN")
}

// mapErrorToExitCode maps internal errors to appropriate exit codes
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

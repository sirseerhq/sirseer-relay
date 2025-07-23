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

	"github.com/spf13/cobra"
	// "github.com/sirseerhq/sirseer-relay/internal/github" // TODO: uncomment when implementing client
	"github.com/sirseerhq/sirseer-relay/internal/output"
	relaierrors "github.com/sirseerhq/sirseer-relay/internal/errors"
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
			return runFetch(cmd.Context(), args[0], token, outputFile)
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
		w, err := output.NewFileWriter(outputFile)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		writer = w
	}
	defer writer.Close()

	// TODO: Create GitHub client and fetch PRs
	// For now, return a placeholder message
	fmt.Fprintf(os.Stderr, "Fetching pull requests from %s/%s...\n", owner, repo)
	
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
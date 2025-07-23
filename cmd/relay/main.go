package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var version = "dev"

func main() {
	rootCmd := &cobra.Command{
		Use:   "sirseer-relay",
		Short: "Extract pull request metadata from GitHub repositories",
		Long: `SirSeer Relay is a high-performance tool for extracting comprehensive
pull request data from GitHub repositories. It efficiently handles repositories
of any size while maintaining low memory usage through streaming architecture.`,
		Version: version,
	}

	rootCmd.AddCommand(newFetchCommand())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func newFetchCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fetch <org>/<repo>",
		Short: "Fetch pull request data from a GitHub repository",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Fetch command not yet implemented")
		},
	}

	cmd.Flags().Bool("all", false, "Fetch all pull requests")
	cmd.Flags().String("since", "", "Fetch PRs created after this date")
	cmd.Flags().String("until", "", "Fetch PRs created before this date")
	cmd.Flags().Bool("incremental", false, "Resume from last fetch")

	return cmd
}
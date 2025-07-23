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
		Version:       version,
		SilenceUsage:  true, // Don't show usage on error
		SilenceErrors: true, // We'll handle error printing ourselves
	}

	rootCmd.AddCommand(newFetchCommand())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(mapErrorToExitCode(err))
	}
}


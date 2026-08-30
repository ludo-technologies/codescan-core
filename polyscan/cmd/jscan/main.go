// Command jscan is a code quality analyzer for JavaScript and TypeScript.
//
// It provides five subcommands. analyze runs the full analysis and writes a
// report, check enforces thresholds and sets an exit code for continuous
// integration, deps inspects and exports the module dependency graph, init
// writes a configuration file, and version prints build information.
//
// Only check varies its exit code by result: 0 when every check passes, 1 when
// a threshold is violated, and 2 when the analysis could not run. It signals
// those through CheckExitError, which main unwraps so that the process exits
// with the right code without cobra printing usage over the report.
//
// Full documentation is at https://jscan.codescan.dev/.
package main

import (
	"fmt"
	"os"

	"github.com/ludo-technologies/polyscan/polyscan/internal/js/version"
	"github.com/spf13/cobra"
)

var (
	// Version information (set via ldflags during build)
	Version = version.Version
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "jscan",
		Short: "jscan - JavaScript/TypeScript static analyzer",
		Long: `jscan is a high-performance static analyzer for JavaScript and TypeScript code.
It provides complexity analysis, dead code detection, and more.`,
		Version: Version,
	}

	// Add subcommands
	rootCmd.AddCommand(analyzeCmd())
	rootCmd.AddCommand(depsCmd())
	rootCmd.AddCommand(checkCmd())
	rootCmd.AddCommand(initCmd())
	rootCmd.AddCommand(versionCmd())

	if err := rootCmd.Execute(); err != nil {
		// Handle custom exit codes from check command
		if exitErr, ok := err.(*CheckExitError); ok {
			if exitErr.Message != "" {
				fmt.Fprintf(os.Stderr, "Error: %s\n", exitErr.Message)
			}
			// Silently exit with the specified code (output already printed)
			os.Exit(exitErr.Code)
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func versionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			verbose, _ := cmd.Flags().GetBool("verbose")
			if verbose {
				fmt.Println(version.GetFullVersion())
			} else {
				fmt.Printf("jscan version %s\n", version.GetVersion())
			}
		},
	}

	cmd.Flags().BoolP("verbose", "v", false, "Show detailed version information")
	return cmd
}

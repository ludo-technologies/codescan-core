// Command polyscan is a code quality analyzer that detects the language of
// each file by its extension.
//
// It has two subcommands: analyze runs the analysis and writes a report, and
// version prints build information.
package main

import (
	"fmt"
	"os"

	"github.com/ludo-technologies/polyscan/polyscan/internal/version"
	"github.com/spf13/cobra"
)

func main() {
	if err := rootCmd().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func rootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "polyscan",
		Short:   "polyscan - multi-language static analyzer",
		Long:    "polyscan is a static analyzer that measures code quality across languages.\nIt currently analyzes cyclomatic complexity and code clones for Go and Rust.",
		Version: version.Version,
	}
	cmd.AddCommand(analyzeCmd())
	cmd.AddCommand(versionCmd())
	return cmd
}

func versionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			verbose, _ := cmd.Flags().GetBool("verbose")
			if verbose {
				fmt.Fprintln(cmd.OutOrStdout(), version.Full())
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "polyscan version %s\n", version.Version)
			}
		},
	}
	cmd.Flags().BoolP("verbose", "v", false, "Show detailed version information")
	return cmd
}

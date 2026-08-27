package main

import (
	"fmt"
	"time"

	"github.com/ludo-technologies/polyscan/core/domain"
	"github.com/ludo-technologies/polyscan/polyscan/internal/analysis"
	"github.com/ludo-technologies/polyscan/polyscan/internal/report"
	"github.com/ludo-technologies/polyscan/polyscan/internal/version"
	"github.com/spf13/cobra"
)

func analyzeCmd() *cobra.Command {
	var (
		format        string
		minComplexity int
	)

	cmd := &cobra.Command{
		Use:   "analyze [path...]",
		Short: "Analyze source files",
		Long: `Analyze source files for cyclomatic complexity.

The language of each file is detected from its extension. Supported: Go.

Examples:
  polyscan analyze .                        # Text report to stdout
  polyscan analyze --format json src/       # JSON report to stdout
  polyscan analyze --min-complexity 10 .    # List only functions at or above 10`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			outputFormat := domain.OutputFormat(format)
			if outputFormat != domain.OutputFormatText && outputFormat != domain.OutputFormatJSON {
				return fmt.Errorf("invalid format %q, must be one of: text, json", format)
			}
			if minComplexity < 1 {
				return fmt.Errorf("--min-complexity must be at least 1")
			}

			start := time.Now()
			result, err := analysis.Analyze(args)
			if err != nil {
				return err
			}

			return report.Write(cmd.OutOrStdout(), &report.Document{
				Version:       version.Version,
				GeneratedAt:   start,
				DurationMs:    time.Since(start).Milliseconds(),
				Complexity:    result,
				MinComplexity: minComplexity,
			}, outputFormat)
		},
	}

	cmd.Flags().StringVarP(&format, "format", "f", "text", "Output format: text, json")
	cmd.Flags().IntVar(&minComplexity, "min-complexity", 1, "List only functions with at least this complexity")
	return cmd
}

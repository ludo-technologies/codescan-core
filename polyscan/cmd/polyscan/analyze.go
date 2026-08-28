package main

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ludo-technologies/polyscan/core/domain"
	"github.com/ludo-technologies/polyscan/core/util"
	"github.com/ludo-technologies/polyscan/polyscan/internal/analysis"
	"github.com/ludo-technologies/polyscan/polyscan/internal/report"
	"github.com/ludo-technologies/polyscan/polyscan/internal/version"
	"github.com/spf13/cobra"
)

const (
	selectComplexity = "complexity"
	selectClone      = "clone"

	defaultReportPath = "polyscan-report.html"
)

func analyzeCmd() *cobra.Command {
	var (
		selected      []string
		format        string
		outputPath    string
		noOpen        bool
		minComplexity int
	)

	cmd := &cobra.Command{
		Use:   "analyze [path...]",
		Short: "Analyze source files",
		Long: `Analyze source files for cyclomatic complexity and code clones.

The language of each file is detected from its extension. Supported: Go, Rust, C++.

By default, generates an HTML report and opens it in your browser.

Examples:
  polyscan analyze .                        # HTML report, opened in the browser
  polyscan analyze --no-open -o out.html .  # HTML report written to out.html
  polyscan analyze --format json src/       # JSON report to stdout
  polyscan analyze --format text src/       # Text report to stdout
  polyscan analyze --select clone .         # Clone detection only
  polyscan analyze --min-complexity 10 .    # List only functions at or above 10`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			outputFormat := domain.OutputFormat(format)
			switch outputFormat {
			case domain.OutputFormatHTML, domain.OutputFormatJSON, domain.OutputFormatText:
			default:
				return fmt.Errorf("invalid format %q, must be one of: html, json, text", format)
			}
			if minComplexity < 1 {
				return fmt.Errorf("--min-complexity must be at least 1")
			}
			options, err := parseSelection(selected)
			if err != nil {
				return err
			}

			start := time.Now()
			result, err := analysis.Analyze(args, options)
			if err != nil {
				return err
			}
			doc := &report.Document{
				Version:       version.Version,
				GeneratedAt:   start,
				DurationMs:    time.Since(start).Milliseconds(),
				Report:        result,
				MinComplexity: minComplexity,
			}

			if outputFormat != domain.OutputFormatHTML {
				return report.Write(cmd.OutOrStdout(), doc, outputFormat)
			}
			if outputPath == "" {
				outputPath = defaultReportPath
			}
			if err := writeReportFile(outputPath, doc); err != nil {
				return err
			}
			absPath, err := filepath.Abs(outputPath)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "HTML report written to %s\n", absPath)
			if noOpen || util.IsSSH() {
				return nil
			}
			if err := util.OpenBrowser(fileURL(absPath)); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Could not open the browser: %v\n", err)
			}
			return nil
		},
	}

	cmd.Flags().StringSliceVarP(&selected, "select", "s", []string{selectComplexity, selectClone},
		"Analyses to run (comma-separated): complexity,clone")
	cmd.Flags().StringVarP(&format, "format", "f", "html", "Output format: html, json, text")
	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "HTML report path (default: "+defaultReportPath+")")
	cmd.Flags().BoolVar(&noOpen, "no-open", false, "Don't open the HTML report in the browser")
	cmd.Flags().IntVar(&minComplexity, "min-complexity", 1, "List only functions with at least this complexity")
	return cmd
}

// fileURL turns an absolute path into a file URL, escaping characters such
// as # and ? that would otherwise be read as a fragment or query, and
// giving Windows drive paths the leading slash the scheme requires.
func fileURL(absPath string) string {
	path := filepath.ToSlash(absPath)
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return (&url.URL{Scheme: "file", Path: path}).String()
}

func writeReportFile(path string, doc *report.Document) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create HTML report: %w", err)
	}
	if err := report.Write(file, doc, domain.OutputFormatHTML); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}

func parseSelection(selected []string) (analysis.Options, error) {
	var options analysis.Options
	for _, name := range selected {
		switch name {
		case selectComplexity:
			options.Complexity = true
		case selectClone:
			options.Clones = true
		default:
			return options, fmt.Errorf("invalid analysis %q, must be one of: complexity, clone", name)
		}
	}
	if !options.Complexity && !options.Clones {
		return options, fmt.Errorf("no analysis selected")
	}
	return options, nil
}

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ludo-technologies/polyscan/core/util"
	"github.com/ludo-technologies/polyscan/polyscan/internal/analysis"
	"github.com/ludo-technologies/polyscan/polyscan/internal/js"
	jsdomain "github.com/ludo-technologies/polyscan/polyscan/internal/js/domain"
	"github.com/ludo-technologies/polyscan/polyscan/internal/js/service"
	"github.com/ludo-technologies/polyscan/polyscan/internal/report"
	"github.com/spf13/cobra"
)

const (
	selectComplexity = "complexity"
	selectDeadCode   = "deadcode"
	selectClone      = "clone"
	selectCBO        = "cbo"
	selectDeps       = "deps"

	defaultReportPath = "polyscan-report.html"
)

// allAnalyses is the default selection: every analysis a language supports.
var allAnalyses = []string{selectComplexity, selectDeadCode, selectClone, selectCBO, selectDeps}

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
		Long: `Analyze source files and report one health score across languages.

The language of each file is detected from its extension. Supported: Go, Rust,
C++ and JavaScript/TypeScript.

Complexity and clone analysis cover every language; dead code, coupling (CBO)
and dependency analysis exist for JavaScript/TypeScript only. The health score
is computed over the dimensions that ran: a dimension a language does not have
is left out, not scored as clean.

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
			outputFormat := jsdomain.OutputFormat(format)
			switch outputFormat {
			case jsdomain.OutputFormatHTML, jsdomain.OutputFormatJSON, jsdomain.OutputFormatText:
			default:
				return fmt.Errorf("invalid format %q, must be one of: html, json, text", format)
			}
			if minComplexity < 1 {
				return fmt.Errorf("--min-complexity must be at least 1")
			}
			options, selection, err := parseSelection(selected)
			if err != nil {
				return err
			}

			start := time.Now()
			var generic *analysis.Report
			if options.Complexity || options.Clones {
				generic, err = analysis.Analyze(args, options)
				if err != nil && !errors.Is(err, analysis.ErrNoFiles) {
					return err
				}
			}
			javascript, err := analyzeJavaScript(args, selection, cmd.ErrOrStderr())
			if err != nil {
				return err
			}
			if generic == nil && javascript == nil {
				return analysis.ErrNoFiles
			}
			responses, err := report.Combine(generic, javascript)
			if err != nil {
				return err
			}
			filterFunctions(responses.Complexity, minComplexity)
			duration := time.Since(start)

			formatter := service.NewOutputFormatter()
			write := func(w io.Writer, format jsdomain.OutputFormat) error {
				return formatter.WriteAnalyze(responses.Complexity, responses.DeadCode,
					responses.Clone, responses.CBO, responses.Deps, format, w, duration)
			}
			summarize := func(w io.Writer) {
				summary := service.BuildAnalyzeSummary(responses.Complexity, responses.DeadCode,
					responses.Clone, responses.CBO, responses.Deps)
				fmt.Fprint(w, service.FormatCLISummary(summary, duration, unanalyzedFiles(responses.Complexity)))
			}

			switch outputFormat {
			case jsdomain.OutputFormatJSON:
				if err := write(cmd.OutOrStdout(), outputFormat); err != nil {
					return err
				}
				// The summary goes to stderr so it cannot pollute the
				// machine-readable output.
				summarize(cmd.ErrOrStderr())
				return nil
			case jsdomain.OutputFormatText:
				// The text report carries its own health score section.
				return write(cmd.OutOrStdout(), outputFormat)
			}

			if outputPath == "" {
				outputPath = defaultReportPath
			}
			if err := writeReportFile(outputPath, write); err != nil {
				return err
			}
			absPath, err := filepath.Abs(outputPath)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "HTML report written to %s\n", absPath)
			summarize(cmd.OutOrStdout())
			if noOpen || util.IsSSH() {
				return nil
			}
			if err := util.OpenBrowser(fileURL(absPath)); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Could not open the browser: %v\n", err)
			}
			return nil
		},
	}

	cmd.Flags().StringSliceVarP(&selected, "select", "s", allAnalyses,
		"Analyses to run (comma-separated): complexity,deadcode,clone,cbo,deps\n"+
			"deadcode, cbo and deps apply to JavaScript/TypeScript only")
	cmd.Flags().StringVarP(&format, "format", "f", "html", "Output format: html, json, text")
	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "HTML report path (default: "+defaultReportPath+")")
	cmd.Flags().BoolVar(&noOpen, "no-open", false, "Don't open the HTML report in the browser")
	cmd.Flags().IntVar(&minComplexity, "min-complexity", 1, "List only functions with at least this complexity")
	return cmd
}

// analyzeJavaScript runs the selected jscan analyses over the JavaScript/
// TypeScript files under paths, or returns nil when there are none. The
// files are collected with jscan's own configuration discovery and
// exclusion rules, so a JavaScript project keeps exactly the analysis
// jscan gave it. An analysis that fails is reported on warn and left out
// of the report, as in jscan.
//
// A tree without JavaScript skips the pipeline before configuration
// discovery, so a jscan configuration that would not load cannot fail the
// other languages' analysis.
func analyzeJavaScript(paths []string, selection js.Selection, warn io.Writer) (*js.Result, error) {
	hasJS, err := js.ContainsFiles(paths)
	if err != nil {
		return nil, err
	}
	if !hasJS {
		return nil, nil
	}
	cfg, err := js.LoadConfig("", paths[0], warn)
	if err != nil {
		return nil, fmt.Errorf("failed to load the JavaScript configuration: %w", err)
	}
	files, err := js.CollectFiles(paths, cfg)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, nil
	}

	result := js.Run(context.Background(), files, cfg, selection)
	for _, failure := range []struct {
		name string
		err  error
	}{
		{"complexity", result.ComplexityErr},
		{"dead code", result.DeadCodeErr},
		{"clone", result.ClonesErr},
		{"CBO", result.CBOErr},
		{"dependency", result.DepsErr},
	} {
		if failure.err != nil {
			fmt.Fprintf(warn, "JavaScript %s analysis error: %v\n", failure.name, failure.err)
		}
	}
	return result, nil
}

// filterFunctions drops the listed functions below minComplexity. The summary
// and every score still cover the complete analyzed population; the filter
// only limits which functions the report lists.
func filterFunctions(complexity *jsdomain.ComplexityResponse, minComplexity int) {
	if complexity == nil || minComplexity <= 1 {
		return
	}
	listed := complexity.Functions[:0]
	for _, fn := range complexity.Functions {
		if fn.Metrics.Complexity >= minComplexity {
			listed = append(listed, fn)
		}
	}
	complexity.Functions = listed
}

// unanalyzedFiles returns the per-file failures the complexity analysis
// collected: it is the only analysis that reports the files it dropped, and
// it runs over the same file set as the others, so its list identifies what
// every score is missing.
func unanalyzedFiles(complexity *jsdomain.ComplexityResponse) []string {
	if complexity == nil {
		return nil
	}
	return complexity.Errors
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

func writeReportFile(path string, write func(io.Writer, jsdomain.OutputFormat) error) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create HTML report: %w", err)
	}
	if err := write(file, jsdomain.OutputFormatHTML); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}

// parseSelection maps the selected analysis names onto the generic engine's
// options and the JavaScript/TypeScript selection. deadcode, cbo and deps
// exist only for JavaScript/TypeScript.
func parseSelection(selected []string) (analysis.Options, js.Selection, error) {
	var options analysis.Options
	var selection js.Selection
	for _, name := range selected {
		switch name {
		case selectComplexity:
			options.Complexity = true
			selection.Complexity = true
		case selectClone:
			options.Clones = true
			selection.Clones = true
		case selectDeadCode:
			selection.DeadCode = true
		case selectCBO:
			selection.CBO = true
		case selectDeps:
			selection.Deps = true
		default:
			return options, selection, fmt.Errorf(
				"invalid analysis %q, must be one of: %s", name, strings.Join(allAnalyses, ", "))
		}
	}
	if selection == (js.Selection{}) {
		return options, selection, fmt.Errorf("no analysis selected")
	}
	return options, selection, nil
}

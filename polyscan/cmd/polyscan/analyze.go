package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ludo-technologies/polyscan/core/domain"
	"github.com/ludo-technologies/polyscan/core/util"
	"github.com/ludo-technologies/polyscan/polyscan/internal/analysis"
	"github.com/ludo-technologies/polyscan/polyscan/internal/js"
	jsdomain "github.com/ludo-technologies/polyscan/polyscan/internal/js/domain"
	"github.com/ludo-technologies/polyscan/polyscan/internal/js/service"
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

The language of each file is detected from its extension. Supported: Go, Rust,
C++ and JavaScript/TypeScript.

JavaScript/TypeScript files get jscan's full analysis (complexity, dead code,
clones, coupling, dependencies) with jscan's own file collection, configuration
discovery and output; --select and --min-complexity apply to the other
languages. The reports stay separate until they are unified.

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
			if err != nil && !errors.Is(err, analysis.ErrNoFiles) {
				return err
			}
			javascript, jsErr := analyzeJavaScript(args, cmd.ErrOrStderr())
			if jsErr != nil {
				return jsErr
			}
			if result == nil && javascript == nil {
				return analysis.ErrNoFiles
			}
			doc := &report.Document{
				Version:       version.Version,
				GeneratedAt:   start,
				DurationMs:    time.Since(start).Milliseconds(),
				Report:        result,
				MinComplexity: minComplexity,
			}

			switch outputFormat {
			case domain.OutputFormatJSON:
				if javascript != nil {
					var buf bytes.Buffer
					if err := javascript.write(jsdomain.OutputFormatJSON, &buf); err != nil {
						return err
					}
					doc.JavaScript = buf.Bytes()
				}
				return report.Write(cmd.OutOrStdout(), doc, outputFormat)
			case domain.OutputFormatText:
				if result != nil {
					if err := report.Write(cmd.OutOrStdout(), doc, outputFormat); err != nil {
						return err
					}
				}
				if javascript != nil {
					return javascript.write(jsdomain.OutputFormatText, cmd.OutOrStdout())
				}
				return nil
			}

			if outputPath == "" {
				outputPath = defaultReportPath
			}
			var openPath string
			if result != nil {
				if err := writeReportFile(outputPath, doc); err != nil {
					return err
				}
				absPath, err := filepath.Abs(outputPath)
				if err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "HTML report written to %s\n", absPath)
				openPath = absPath
			}
			if javascript != nil {
				// The JavaScript report is a separate page until the
				// reports are unified; it takes the main path when it is
				// the only one.
				jsPath := outputPath
				if result != nil {
					jsPath = jsReportPath(outputPath)
				}
				if err := writeJSReportFile(jsPath, javascript); err != nil {
					return err
				}
				absPath, err := filepath.Abs(jsPath)
				if err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "JavaScript HTML report written to %s\n", absPath)
				if openPath == "" {
					openPath = absPath
				}
			}
			if noOpen || util.IsSSH() {
				return nil
			}
			if err := util.OpenBrowser(fileURL(openPath)); err != nil {
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

// jsAnalysis is the jscan analysis of the JavaScript/TypeScript files,
// with the duration jscan stamps into its report.
type jsAnalysis struct {
	result   *js.Result
	duration time.Duration
}

// analyzeJavaScript runs the full jscan analysis over the JavaScript/
// TypeScript files under paths, or returns nil when there are none. The
// files are collected with jscan's own configuration discovery and
// exclusion rules, so a JavaScript project keeps exactly the analysis
// jscan gave it. An analysis that fails is reported on warn and left out
// of the report, as in jscan.
func analyzeJavaScript(paths []string, warn io.Writer) (*jsAnalysis, error) {
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

	start := time.Now()
	result := js.Run(context.Background(), files, cfg, js.AllAnalyses())
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
	return &jsAnalysis{result: result, duration: time.Since(start)}, nil
}

// write renders the analysis in jscan's own format.
func (a *jsAnalysis) write(format jsdomain.OutputFormat, w io.Writer) error {
	formatter := service.NewOutputFormatter()
	return formatter.WriteAnalyze(a.result.Complexity, a.result.DeadCode, a.result.Clones, a.result.CBO, a.result.Deps, format, w, a.duration)
}

// jsReportPath places the JavaScript report next to the main one:
// polyscan-report.html becomes polyscan-report.js.html.
func jsReportPath(path string) string {
	ext := filepath.Ext(path)
	return strings.TrimSuffix(path, ext) + ".js" + ext
}

func writeJSReportFile(path string, javascript *jsAnalysis) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create JavaScript HTML report: %w", err)
	}
	if err := javascript.write(jsdomain.OutputFormatHTML, file); err != nil {
		file.Close()
		return err
	}
	return file.Close()
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

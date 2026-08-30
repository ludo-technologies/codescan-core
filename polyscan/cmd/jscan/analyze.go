package main

import (
	"context"
	"fmt"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ludo-technologies/polyscan/polyscan/internal/js"
	"github.com/ludo-technologies/polyscan/polyscan/internal/js/config"
	"github.com/ludo-technologies/polyscan/polyscan/internal/js/domain"
	"github.com/ludo-technologies/polyscan/polyscan/internal/js/service"
	"github.com/spf13/cobra"
)

var (
	selectAnalyses []string
	outputFormat   string
	configPath     string
	jsonOutput     bool
	htmlOutput     bool
	textOutput     bool
	noOpenBrowser  bool
	outputPath     string
)

func analyzeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "analyze [path...]",
		Short: "Analyze JavaScript/TypeScript files",
		Long: `Analyze JavaScript/TypeScript files for complexity, dead code, code clones, and coupling.

By default, generates an HTML report and opens it in your browser.

Examples:
  jscan analyze src/                              # All analyses (default)
  jscan analyze --select complexity,deadcode src/ # Complexity + dead code only
  jscan analyze --select clone src/               # Clone detection only
  jscan analyze --select cbo src/                 # CBO coupling analysis only
  jscan analyze --json src/                       # Output JSON to stdout
  jscan analyze --text src/                       # Output text to stdout
  jscan analyze --format yaml src/                # Output YAML to stdout
  jscan analyze --format csv src/                 # Output CSV to stdout
  jscan analyze --no-open src/                    # Generate HTML without opening browser
  jscan analyze -o report.html src/               # Custom output path`,
		RunE: runAnalyze,
	}

	cmd.Flags().StringSliceVarP(&selectAnalyses, "select", "s", []string{"complexity", "deadcode", "clone", "cbo", "deps"},
		"Analyses to run (comma-separated): complexity,deadcode,clone,cbo,deps")
	cmd.Flags().StringVarP(&outputFormat, "format", "f", "html",
		"Output format: html, json, text, yaml, csv (default: html)")
	cmd.Flags().BoolVar(&jsonOutput, "json", false,
		"Output results as JSON to stdout")
	cmd.Flags().BoolVar(&textOutput, "text", false,
		"Output results as text to stdout")
	cmd.Flags().BoolVar(&htmlOutput, "html", false,
		"Output results as HTML report (default)")
	cmd.Flags().BoolVar(&noOpenBrowser, "no-open", false,
		"Don't auto-open HTML report in browser")
	cmd.Flags().StringVarP(&outputPath, "output", "o", "",
		"Output file path (default: jscan-report.html)")
	cmd.Flags().StringVarP(&configPath, "config", "c", "",
		"Path to config file")

	return cmd
}

func runAnalyze(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no paths specified")
	}

	// Determine output format (default: HTML)
	var format domain.OutputFormat
	if cmd.Flags().Changed("format") {
		switch outputFormat {
		case "html":
			format = domain.OutputFormatHTML
		case "json":
			format = domain.OutputFormatJSON
		case "text":
			format = domain.OutputFormatText
		case "yaml":
			format = domain.OutputFormatYAML
		case "csv":
			format = domain.OutputFormatCSV
		default:
			return fmt.Errorf("invalid format %q, must be one of: html, json, text, yaml, csv", outputFormat)
		}
	} else {
		switch {
		case jsonOutput:
			format = domain.OutputFormatJSON
		case textOutput:
			format = domain.OutputFormatText
		default:
			format = domain.OutputFormatHTML
		}
	}

	isStructured := format == domain.OutputFormatJSON || format == domain.OutputFormatYAML || format == domain.OutputFormatCSV

	// Load configuration
	cfg, err := js.LoadConfig(configPath, args[0], os.Stderr)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}
	if configPath != "" && !isStructured {
		fmt.Printf("Using config: %s\n", configPath)
	}

	// Collect JavaScript/TypeScript files (using the patterns from config)
	files, err := js.CollectFiles(args, cfg)
	if err != nil {
		return err
	}

	if len(files) == 0 {
		return fmt.Errorf("no JavaScript/TypeScript files found")
	}

	if !isStructured {
		fmt.Printf("Analyzing %d files...\n", len(files))
	}

	// Create progress manager (auto-disabled for structured output or non-TTY)
	pm := service.NewProgressManager(!isStructured)
	defer pm.Close()

	// Start timing
	startTime := time.Now()

	// Determine which analyses to run
	selection := js.Selection{
		Complexity: contains(selectAnalyses, "complexity"),
		DeadCode:   contains(selectAnalyses, "deadcode"),
		Clones:     contains(selectAnalyses, "clone"),
		CBO:        contains(selectAnalyses, "cbo"),
		Deps:       contains(selectAnalyses, "deps"),
	}

	// Single progress bar for all analyses (only when interactive)
	var task domain.TaskProgress
	var progressDone chan struct{}
	if pm.IsInteractive() {
		task = pm.StartTask("Analyzing", 100)
		estimatedDuration := estimateAnalysisDuration(len(files), selection)
		progressDone = startTimeBasedProgressUpdater(task, estimatedDuration)
	}

	// Run analyses in parallel
	result := js.Run(context.Background(), files, cfg, selection)
	complexityResponse := result.Complexity
	deadCodeResponse := result.DeadCode
	cloneResponse := result.Clones
	cboResponse := result.CBO
	depsResponse := result.Deps
	complexityErr, deadCodeErr, cloneErr, cboErr, depsErr := result.ComplexityErr, result.DeadCodeErr, result.ClonesErr, result.CBOErr, result.DepsErr

	if progressDone != nil {
		close(progressDone)
	}
	if task != nil {
		task.Describe("Analyzing...")
		task.Complete()
	}

	// Handle errors
	if complexityErr != nil && !isStructured {
		fmt.Fprintf(os.Stderr, "Complexity analysis error: %v\n", complexityErr)
	}
	if deadCodeErr != nil && !isStructured {
		fmt.Fprintf(os.Stderr, "Dead code analysis error: %v\n", deadCodeErr)
	}
	if cloneErr != nil && !isStructured {
		fmt.Fprintf(os.Stderr, "Clone analysis error: %v\n", cloneErr)
	}
	if cboErr != nil && !isStructured {
		fmt.Fprintf(os.Stderr, "CBO analysis error: %v\n", cboErr)
	}
	if depsErr != nil && !isStructured {
		fmt.Fprintf(os.Stderr, "Dependency analysis error: %v\n", depsErr)
	}

	// Calculate duration
	duration := time.Since(startTime)

	// Output results
	formatter := service.NewOutputFormatter()

	// Handle HTML output with file writing and browser opening
	if format == domain.OutputFormatHTML {
		// Determine output path
		htmlPath := outputPath
		if htmlPath == "" {
			htmlPath = "jscan-report.html"
		}

		// Create HTML file
		file, err := os.Create(htmlPath)
		if err != nil {
			return fmt.Errorf("failed to create HTML file: %w", err)
		}
		defer file.Close()

		// Write HTML
		if err := formatter.WriteAnalyze(complexityResponse, deadCodeResponse, cloneResponse, cboResponse, depsResponse, format, file, duration); err != nil {
			return err
		}

		// Get absolute path for display
		absPath, _ := filepath.Abs(htmlPath)
		fmt.Printf("\U0001F4CA Unified HTML report generated and opened: %s\n", absPath)

		// Open in browser unless disabled
		if !noOpenBrowser && !service.IsSSH() {
			if err := service.OpenBrowser(fileURL(absPath)); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: Could not open browser: %v\n", err)
			}
		}

		// Print CLI summary
		summary := service.BuildAnalyzeSummary(complexityResponse, deadCodeResponse, cloneResponse, cboResponse, depsResponse)
		fmt.Print(service.FormatCLISummary(summary, duration, unanalyzedFiles(complexityResponse)))

		return nil
	}

	// JSON, Text, or other format output to stdout
	if err := formatter.WriteAnalyze(complexityResponse, deadCodeResponse, cloneResponse, cboResponse, depsResponse, format, os.Stdout, duration); err != nil {
		return err
	}

	// Print CLI summary to stderr for structured formats (JSON/YAML/CSV)
	// so it doesn't pollute the machine-readable output on stdout.
	// Text format already includes a Health Score section, so skip it.
	if format != domain.OutputFormatText {
		summary := service.BuildAnalyzeSummary(complexityResponse, deadCodeResponse, cloneResponse, cboResponse, depsResponse)
		fmt.Fprint(os.Stderr, service.FormatCLISummary(summary, duration, unanalyzedFiles(complexityResponse)))
	}

	return nil
}

// unanalyzedFiles returns the per-file failures the complexity analysis
// collected. It is the only analysis that reports the files it dropped, and it
// runs over the same file set as the others, so its list identifies what every
// score is missing.
func unanalyzedFiles(complexityResponse *domain.ComplexityResponse) []string {
	if complexityResponse == nil {
		return nil
	}
	return complexityResponse.Errors
}

// runDeadCodeAnalysis runs dead code analysis on the given files with progress tracking
// This is used by check.go which has its own progress management
func runDeadCodeAnalysis(files []string, cfg *config.Config, pm domain.ProgressManager) (*domain.DeadCodeResponse, error) {
	task := pm.StartTask("Detecting dead code", len(files))
	defer task.Complete()

	return service.AnalyzeDeadCodeWithTask(context.Background(), js.DeadCodeRequest(files, cfg), task)
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// estimateAnalysisDuration estimates total analysis time based on file count.
// Since analyses run in parallel, the time is based on the slower analysis (not the sum).
func estimateAnalysisDuration(fileCount int, selection js.Selection) time.Duration {
	perFileMs := 20.0

	if selection.Complexity {
		perFileMs = math.Max(perFileMs, 20.0)
	}
	if selection.DeadCode {
		perFileMs = math.Max(perFileMs, 35.0)
	}
	if selection.Clones {
		perFileMs = math.Max(perFileMs, 45.0)
	}
	if selection.CBO {
		perFileMs = math.Max(perFileMs, 25.0)
	}
	if selection.Deps {
		perFileMs = math.Max(perFileMs, 20.0)
	}

	estimatedMs := float64(fileCount) * perFileMs
	if estimatedMs < 3000 {
		estimatedMs = 3000
	}
	estimatedMs *= 1.25 // buffer

	return time.Duration(estimatedMs) * time.Millisecond
}

func calculateProgressPercent(elapsed, estimatedDuration time.Duration) int {
	if estimatedDuration <= 0 || elapsed <= 0 {
		return 0
	}

	// Phase 1: quickly reach up to 90% around the estimated completion time.
	if elapsed <= estimatedDuration {
		return int((float64(elapsed) / float64(estimatedDuration)) * 90)
	}

	// Phase 2: slowly approach 99% so long-running analyses do not appear stuck.
	tailDuration := estimatedDuration * 4
	if tailDuration <= 0 {
		tailDuration = time.Second
	}

	tailRatio := float64(elapsed-estimatedDuration) / float64(tailDuration)
	if tailRatio > 1 {
		tailRatio = 1
	}

	progress := 90 + int(tailRatio*9)
	if progress > 99 {
		return 99
	}
	return progress
}

// startTimeBasedProgressUpdater starts background progress updates
func startTimeBasedProgressUpdater(task domain.TaskProgress, estimatedDuration time.Duration) chan struct{} {
	done := make(chan struct{})
	startTime := time.Now()
	lastProgress := 0

	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				elapsed := time.Since(startTime)
				progress := calculateProgressPercent(elapsed, estimatedDuration)
				if delta := progress - lastProgress; delta > 0 {
					task.Increment(delta)
					lastProgress = progress
				}
				task.Describe("Analyzing...")
			case <-done:
				return
			}
		}
	}()

	return done
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

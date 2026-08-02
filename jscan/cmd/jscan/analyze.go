package main

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ludo-technologies/polyscan/jscan/app"
	"github.com/ludo-technologies/polyscan/jscan/domain"
	"github.com/ludo-technologies/polyscan/jscan/internal/config"
	"github.com/ludo-technologies/polyscan/jscan/service"
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
  jscan analyze --no-open src/                    # Generate HTML without opening browser
  jscan analyze -o report.html src/               # Custom output path`,
		RunE: runAnalyze,
	}

	cmd.Flags().StringSliceVarP(&selectAnalyses, "select", "s", []string{"complexity", "deadcode", "clone", "cbo", "deps"},
		"Analyses to run (comma-separated): complexity,deadcode,clone,cbo,deps")
	cmd.Flags().StringVarP(&outputFormat, "format", "f", "html",
		"Output format: html, json, text (default: html)")
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
	format := domain.OutputFormatHTML
	if jsonOutput || outputFormat == "json" {
		format = domain.OutputFormatJSON
	} else if textOutput || outputFormat == "text" {
		format = domain.OutputFormatText
	}

	// Load configuration
	cfg, err := config.LoadConfigWithTarget(configPath, args[0])
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}
	if configPath != "" && format != domain.OutputFormatJSON {
		fmt.Printf("Using config: %s\n", configPath)
	}

	// Collect JavaScript/TypeScript files (using exclude patterns from config)
	var files []string
	for _, path := range args {
		pathFiles, err := collectJSFiles(path, cfg.Analysis.ExcludePatterns)
		if err != nil {
			return fmt.Errorf("failed to collect files from %s: %w", path, err)
		}
		files = append(files, pathFiles...)
	}

	if len(files) == 0 {
		return fmt.Errorf("no JavaScript/TypeScript files found")
	}

	if format != domain.OutputFormatJSON {
		fmt.Printf("Analyzing %d files...\n", len(files))
	}

	// Create progress manager (auto-disabled for JSON output or non-TTY)
	pm := service.NewProgressManager(format != domain.OutputFormatJSON)
	defer pm.Close()

	// Start timing
	startTime := time.Now()

	// Initialize responses
	var complexityResponse *domain.ComplexityResponse
	var deadCodeResponse *domain.DeadCodeResponse
	var cloneResponse *domain.CloneResponse
	var cboResponse *domain.CBOResponse
	var depsResponse *domain.DependencyGraphResponse

	// Determine which analyses to run
	runComplexity := contains(selectAnalyses, "complexity")
	runDeadCode := contains(selectAnalyses, "deadcode")
	runClone := contains(selectAnalyses, "clone")
	runCBO := contains(selectAnalyses, "cbo")
	runDeps := contains(selectAnalyses, "deps")

	// Single progress bar for all analyses (only when interactive)
	var task domain.TaskProgress
	var progressDone chan struct{}
	if pm.IsInteractive() {
		task = pm.StartTask("Analyzing", 100)
		estimatedDuration := estimateAnalysisDuration(len(files), runComplexity, runDeadCode, runClone, runCBO, runDeps)
		progressDone = startTimeBasedProgressUpdater(task, estimatedDuration)
	}

	// Run analyses in parallel
	var wg sync.WaitGroup
	var complexityErr, deadCodeErr, cloneErr, cboErr, depsErr error
	var mu sync.Mutex
	ctx := context.Background()

	if runComplexity {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := runComplexityAnalysisInternal(files, cfg)
			mu.Lock()
			complexityResponse = resp
			complexityErr = err
			mu.Unlock()
		}()
	}

	if runDeadCode {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := runDeadCodeAnalysisInternal(files)
			mu.Lock()
			deadCodeResponse = resp
			deadCodeErr = err
			mu.Unlock()
		}()
	}

	if runClone {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := runCloneAnalysisInternal(ctx, files)
			mu.Lock()
			cloneResponse = resp
			cloneErr = err
			mu.Unlock()
		}()
	}

	if runCBO {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := runCBOAnalysisInternal(ctx, files)
			mu.Lock()
			cboResponse = resp
			cboErr = err
			mu.Unlock()
		}()
	}

	if runDeps {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := runDepsAnalysisInternal(ctx, files)
			mu.Lock()
			depsResponse = resp
			depsErr = err
			mu.Unlock()
		}()
	}

	wg.Wait()
	if progressDone != nil {
		close(progressDone)
	}
	if task != nil {
		task.Describe("Analyzing...")
		task.Complete()
	}

	// Handle errors
	if complexityErr != nil && format != domain.OutputFormatJSON {
		fmt.Fprintf(os.Stderr, "Complexity analysis error: %v\n", complexityErr)
	}
	if deadCodeErr != nil && format != domain.OutputFormatJSON {
		fmt.Fprintf(os.Stderr, "Dead code analysis error: %v\n", deadCodeErr)
	}
	if cloneErr != nil && format != domain.OutputFormatJSON {
		fmt.Fprintf(os.Stderr, "Clone analysis error: %v\n", cloneErr)
	}
	if cboErr != nil && format != domain.OutputFormatJSON {
		fmt.Fprintf(os.Stderr, "CBO analysis error: %v\n", cboErr)
	}
	if depsErr != nil && format != domain.OutputFormatJSON {
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
			if err := service.OpenBrowser("file://" + absPath); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: Could not open browser: %v\n", err)
			}
		}

		// Print CLI summary
		summary := service.BuildAnalyzeSummary(complexityResponse, deadCodeResponse, cloneResponse, cboResponse, depsResponse)
		fmt.Print(service.FormatCLISummary(summary, duration))

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
		fmt.Fprint(os.Stderr, service.FormatCLISummary(summary, duration))
	}

	return nil
}

// runComplexityAnalysisInternal runs complexity analysis on the given files without progress tracking
func runComplexityAnalysisInternal(files []string, cfg *config.Config) (*domain.ComplexityResponse, error) {
	svc := service.NewComplexityService(&cfg.Complexity)

	req := domain.ComplexityRequest{
		Paths:           files,
		LowThreshold:    cfg.Complexity.LowThreshold,
		MediumThreshold: cfg.Complexity.MediumThreshold,
		MinComplexity:   cfg.Output.MinComplexity,
		SortBy:          domain.SortByComplexity,
	}

	ctx := context.Background()
	return svc.Analyze(ctx, req)
}

// runDeadCodeAnalysis runs dead code analysis on the given files with progress tracking
// This is used by check.go which has its own progress management
func runDeadCodeAnalysis(files []string, _ *config.Config, pm domain.ProgressManager) (*domain.DeadCodeResponse, error) {
	task := pm.StartTask("Detecting dead code", len(files))
	defer task.Complete()

	req := domain.DeadCodeRequest{
		Paths:       files,
		MinSeverity: domain.DeadCodeSeverityInfo,
		SortBy:      domain.DeadCodeSortBySeverity,
	}

	return service.AnalyzeDeadCodeWithTask(context.Background(), req, task)
}

// runDeadCodeAnalysisInternal runs dead code analysis on the given files without progress tracking
func runDeadCodeAnalysisInternal(files []string) (*domain.DeadCodeResponse, error) {
	req := domain.DeadCodeRequest{
		Paths:       files,
		MinSeverity: domain.DeadCodeSeverityInfo,
		SortBy:      domain.DeadCodeSortBySeverity,
	}

	return service.AnalyzeDeadCode(context.Background(), req)
}

// collectJSFiles collects JavaScript/TypeScript files from a path using FileHelper
func collectJSFiles(path string, excludePatterns []string) ([]string, error) {
	helper := app.NewFileHelper()
	return helper.CollectJSFiles([]string{path}, true, nil, excludePatterns)
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
func estimateAnalysisDuration(fileCount int, runComplexity, runDeadCode, runClone, runCBO, runDeps bool) time.Duration {
	perFileMs := 20.0

	if runComplexity {
		perFileMs = math.Max(perFileMs, 20.0)
	}
	if runDeadCode {
		perFileMs = math.Max(perFileMs, 35.0)
	}
	if runClone {
		perFileMs = math.Max(perFileMs, 45.0)
	}
	if runCBO {
		perFileMs = math.Max(perFileMs, 25.0)
	}
	if runDeps {
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

// runCloneAnalysisInternal runs clone detection without progress tracking
func runCloneAnalysisInternal(ctx context.Context, files []string) (*domain.CloneResponse, error) {
	svc := service.NewCloneServiceWithDefaults()

	req := domain.DefaultCloneRequest()
	req.Paths = files

	return svc.DetectClones(ctx, req)
}

// runCBOAnalysisInternal runs CBO analysis without progress tracking
func runCBOAnalysisInternal(ctx context.Context, files []string) (*domain.CBOResponse, error) {
	svc := service.NewCBOServiceWithDefaults()

	req := domain.CBORequest{
		Paths: files,
	}

	return svc.Analyze(ctx, req)
}

// runDepsAnalysisInternal runs dependency analysis without progress tracking
func runDepsAnalysisInternal(ctx context.Context, files []string) (*domain.DependencyGraphResponse, error) {
	svc := service.NewDependencyGraphServiceWithDefaults()

	req := domain.DependencyGraphRequest{
		Paths:        files,
		DetectCycles: domain.BoolPtr(true),
	}

	return svc.Analyze(ctx, req)
}

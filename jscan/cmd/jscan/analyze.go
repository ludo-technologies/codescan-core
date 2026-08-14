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

	if cmd.Flags().Changed("format") {
		switch outputFormat {
		case "html", "json", "text", "yaml", "csv":
		default:
			return fmt.Errorf("invalid format %q, must be one of: html, json, text, yaml, csv", outputFormat)
		}
	}

	// Determine output format (default: HTML)
	var format domain.OutputFormat
	switch {
	case jsonOutput, outputFormat == "json":
		format = domain.OutputFormatJSON
	case textOutput, outputFormat == "text":
		format = domain.OutputFormatText
	case htmlOutput, outputFormat == "html":
		format = domain.OutputFormatHTML
	case outputFormat == "yaml":
		format = domain.OutputFormatYAML
	case outputFormat == "csv":
		format = domain.OutputFormatCSV
	default:
		return fmt.Errorf("invalid format %q, must be one of: html, json, text, yaml, csv", outputFormat)
	}

	isStructured := format == domain.OutputFormatJSON || format == domain.OutputFormatYAML || format == domain.OutputFormatCSV

	// Load configuration
	cfg, err := loadCommandConfig(configPath, args[0], os.Stderr)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}
	if configPath != "" && !isStructured {
		fmt.Printf("Using config: %s\n", configPath)
	}

	// Collect JavaScript/TypeScript files (using the patterns from config)
	var files []string
	for _, path := range args {
		pathFiles, err := collectJSFiles(path, cfg)
		if err != nil {
			return fmt.Errorf("failed to collect files from %s: %w", path, err)
		}
		files = append(files, pathFiles...)
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

	// With several analyses selected, parse every file once and share the parse
	// trees across them. Nothing references the snapshot after the goroutines
	// below finish, so the shared trees become collectable as soon as the
	// analyses are done with them — clone detection drops its per-fragment AST
	// references itself once fragments are converted for APTED. A single
	// selected analysis has nobody to share with and skips the snapshot: the
	// services' own entry points then release each file as they go, which
	// holds far fewer parse trees at once.
	var snapshot *service.ProjectSnapshot
	if countSelected(runComplexity, runDeadCode, runClone, runCBO, runDeps) > 1 {
		snapshot = service.BuildProjectSnapshot(ctx, files, nil)
	}

	if runComplexity {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := runComplexityAnalysisInternal(ctx, snapshot, files, cfg)
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
			resp, err := runDeadCodeAnalysisInternal(ctx, snapshot, files, cfg)
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
			resp, err := runCloneAnalysisInternal(ctx, snapshot, files)
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
			resp, err := runCBOAnalysisInternal(ctx, snapshot, files)
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
			resp, err := runDepsAnalysisInternal(ctx, snapshot, files)
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

// countSelected reports how many analyses the run will execute.
func countSelected(selected ...bool) int {
	count := 0
	for _, s := range selected {
		if s {
			count++
		}
	}
	return count
}

// runComplexityAnalysisInternal runs complexity analysis without progress
// tracking, over the shared snapshot when one exists.
func runComplexityAnalysisInternal(ctx context.Context, snapshot *service.ProjectSnapshot, files []string, cfg *config.Config) (*domain.ComplexityResponse, error) {
	svc := service.NewComplexityService(&cfg.Complexity)

	req := domain.ComplexityRequest{
		Paths:           files,
		LowThreshold:    cfg.Complexity.LowThreshold,
		MediumThreshold: cfg.Complexity.MediumThreshold,
		MinComplexity:   cfg.Output.MinComplexity,
		SortBy:          domain.SortCriteria(cfg.Output.SortBy),
	}

	if snapshot != nil {
		return svc.AnalyzeSnapshot(ctx, snapshot, req)
	}
	return svc.Analyze(ctx, req)
}

// runDeadCodeAnalysis runs dead code analysis on the given files with progress tracking
// This is used by check.go which has its own progress management
func runDeadCodeAnalysis(files []string, cfg *config.Config, pm domain.ProgressManager) (*domain.DeadCodeResponse, error) {
	task := pm.StartTask("Detecting dead code", len(files))
	defer task.Complete()

	return service.AnalyzeDeadCodeWithTask(context.Background(), deadCodeRequest(files, cfg), task)
}

// runDeadCodeAnalysisInternal runs dead code analysis without progress
// tracking, over the shared snapshot when one exists.
func runDeadCodeAnalysisInternal(ctx context.Context, snapshot *service.ProjectSnapshot, files []string, cfg *config.Config) (*domain.DeadCodeResponse, error) {
	if snapshot != nil {
		return service.AnalyzeDeadCodeSnapshot(ctx, snapshot, deadCodeRequest(files, cfg))
	}
	return service.AnalyzeDeadCode(ctx, deadCodeRequest(files, cfg))
}

// deadCodeRequest builds the dead code request both entry points share.
func deadCodeRequest(files []string, cfg *config.Config) domain.DeadCodeRequest {
	return domain.DeadCodeRequest{
		Paths:       files,
		MinSeverity: domain.DeadCodeSeverity(cfg.DeadCode.MinSeverity),
		SortBy:      domain.DeadCodeSortCriteria(cfg.DeadCode.SortBy),
	}
}

// collectJSFiles collects JavaScript/TypeScript files from a path using FileHelper
func collectJSFiles(path string, cfg *config.Config) ([]string, error) {
	helper := app.NewFileHelper()
	return helper.CollectJSFiles([]string{path}, cfg.Analysis.Recursive, cfg.Analysis.IncludePatterns, cfg.Analysis.ExcludePatterns)
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

// runCloneAnalysisInternal runs clone detection without progress tracking,
// over the shared snapshot when one exists.
func runCloneAnalysisInternal(ctx context.Context, snapshot *service.ProjectSnapshot, files []string) (*domain.CloneResponse, error) {
	svc := service.NewCloneServiceWithDefaults()

	req := domain.DefaultCloneRequest()
	req.Paths = files

	if snapshot != nil {
		return svc.DetectClonesInSnapshot(ctx, snapshot, req)
	}
	return svc.DetectClones(ctx, req)
}

// runCBOAnalysisInternal runs CBO analysis without progress tracking, over the
// shared snapshot when one exists.
func runCBOAnalysisInternal(ctx context.Context, snapshot *service.ProjectSnapshot, files []string) (*domain.CBOResponse, error) {
	svc := service.NewCBOServiceWithDefaults()

	req := domain.CBORequest{
		Paths: files,
	}

	if snapshot != nil {
		return svc.AnalyzeSnapshot(ctx, snapshot, req)
	}
	return svc.Analyze(ctx, req)
}

// runDepsAnalysisInternal runs dependency analysis without progress tracking,
// over the shared snapshot when one exists.
func runDepsAnalysisInternal(ctx context.Context, snapshot *service.ProjectSnapshot, files []string) (*domain.DependencyGraphResponse, error) {
	svc := service.NewDependencyGraphServiceWithDefaults()

	req := domain.DependencyGraphRequest{
		Paths:        files,
		DetectCycles: domain.BoolPtr(true),
	}

	if snapshot != nil {
		return svc.AnalyzeSnapshot(ctx, snapshot, req)
	}
	return svc.Analyze(ctx, req)
}

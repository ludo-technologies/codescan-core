package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/ludo-technologies/polyscan/jscan/domain"
	"github.com/ludo-technologies/polyscan/jscan/internal/config"
	"github.com/ludo-technologies/polyscan/jscan/internal/version"
	"github.com/ludo-technologies/polyscan/jscan/service"
	"github.com/spf13/cobra"
)

// CheckExitError is a custom error type for check command exit codes
type CheckExitError struct {
	Code    int
	Message string
}

func (e *CheckExitError) Error() string {
	return e.Message
}

var (
	checkMaxComplexity    int
	checkAllowDeadCode    bool
	checkAllowCircDeps    bool
	checkAllowParseErrors bool
	checkMaxCycles        int
	checkSelectAnalyses   []string
	checkVerbose          bool
	checkJSON             bool
	checkConfigPath       string
)

// checkAnalyses are the analyses --select accepts.
var checkAnalyses = []string{"complexity", "deadcode", "deps"}

func checkCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check [path...]",
		Short: "Fast quality check for CI/CD pipelines",
		Long: `Run quality checks against configurable thresholds for CI/CD integration.

Exit codes:
  0 - All checks pass
  1 - Quality threshold(s) violated
  2 - Analysis error (file not found, parse error, etc.)

Examples:
  # Basic check with defaults
  jscan check src/

  # Strict complexity check
  jscan check --max-complexity 10 src/

  # Allow dead code, fail on circular deps
  jscan check --allow-dead-code src/

  # JSON output for machine parsing
  jscan check --json src/

  # Select specific analyses
  jscan check --select complexity,deps src/`,
		RunE:          runCheck,
		SilenceUsage:  true, // Don't print usage on errors (we handle our own output)
		SilenceErrors: true, // Don't print error messages (we handle our own output)
	}

	// An unusable invocation is invalid input, which the exit-code contract
	// above assigns to 2. Without this an unknown flag exits 1 and reads as a
	// quality verdict.
	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return &CheckExitError{Code: 2, Message: err.Error()}
	})

	cmd.Flags().IntVar(&checkMaxComplexity, "max-complexity", 10,
		"Maximum allowed cyclomatic complexity per function")
	cmd.Flags().BoolVar(&checkAllowDeadCode, "allow-dead-code", false,
		"Allow dead code findings without failing")
	cmd.Flags().BoolVar(&checkAllowCircDeps, "allow-circular-deps", false,
		"Allow circular dependencies without failing")
	cmd.Flags().BoolVar(&checkAllowParseErrors, "allow-parse-errors", false,
		"Report files that could not be read or parsed without failing")
	cmd.Flags().IntVar(&checkMaxCycles, "max-cycles", 0,
		"Maximum allowed dependency cycles (0 = none allowed)")
	cmd.Flags().StringSliceVarP(&checkSelectAnalyses, "select", "s",
		checkAnalyses,
		"Analyses to run: complexity,deadcode,deps")
	cmd.Flags().BoolVarP(&checkVerbose, "verbose", "v", false,
		"Show detailed output")
	cmd.Flags().BoolVar(&checkJSON, "json", false,
		"Output results as JSON")
	cmd.Flags().StringVarP(&checkConfigPath, "config", "c", "",
		"Path to config file")

	return cmd
}

func runCheck(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return &CheckExitError{Code: 2, Message: "no paths specified"}
	}

	// A misspelled analysis name would otherwise select nothing and pass.
	for _, selected := range checkSelectAnalyses {
		if !contains(checkAnalyses, selected) {
			return &CheckExitError{Code: 2, Message: fmt.Sprintf(
				"invalid --select value %q: expected one of %v", selected, checkAnalyses)}
		}
	}

	startTime := time.Now()

	// Load configuration
	cfg, err := loadCommandConfig(checkConfigPath, args[0], os.Stderr)
	if err != nil {
		return &CheckExitError{Code: 2, Message: fmt.Sprintf("failed to load configuration: %v", err)}
	}

	// Apply config values for flags not explicitly set on CLI
	if !cmd.Flags().Changed("max-complexity") && cfg.Complexity.MaxComplexity > 0 {
		checkMaxComplexity = cfg.Complexity.MaxComplexity
	}

	// Collect JavaScript/TypeScript files (using the patterns from config)
	var files []string
	for _, path := range args {
		pathFiles, err := collectJSFiles(path, cfg)
		if err != nil {
			return &CheckExitError{Code: 2, Message: fmt.Sprintf("failed to collect files from %s: %v", path, err)}
		}
		files = append(files, pathFiles...)
	}

	if len(files) == 0 {
		return &CheckExitError{Code: 2, Message: "no JavaScript/TypeScript files found"}
	}

	// Create progress manager (auto-disabled for JSON output or non-TTY/CI)
	pm := service.NewProgressManager(!checkJSON)
	defer pm.Close()

	// Initialize result
	result := &domain.CheckResult{
		Passed:     true,
		ExitCode:   0,
		Violations: []domain.CheckViolation{},
		Summary: domain.CheckSummary{
			FilesAnalyzed: len(files),
		},
	}

	ctx := context.Background()

	// Run selected analyses
	if contains(checkSelectAnalyses, "complexity") {
		if err := checkComplexity(ctx, files, cfg, result, pm); err != nil {
			return &CheckExitError{Code: 2, Message: err.Error()}
		}
	}

	if contains(checkSelectAnalyses, "deadcode") {
		if err := checkDeadCode(ctx, files, cfg, result, pm); err != nil {
			return &CheckExitError{Code: 2, Message: err.Error()}
		}
	}

	if contains(checkSelectAnalyses, "deps") {
		if err := checkDependencies(ctx, files, cfg, result); err != nil {
			return &CheckExitError{Code: 2, Message: err.Error()}
		}
	}

	return outputCheckResult(result, startTime)
}

// reportUnanalyzedFiles lists what an analysis could not use and reports whether
// that should fail the gate. A file that cannot be read or parsed is absent from
// every metric, so it clears every threshold by contributing nothing: without
// this the gate passes on code it never looked at.
func reportUnanalyzedFiles(analysis string, errs []string) error {
	if len(errs) == 0 {
		return nil
	}

	for _, err := range errs {
		fmt.Fprintf(os.Stderr, "  [%s] %s\n", analysis, err)
	}

	if checkAllowParseErrors {
		return nil
	}

	// Counted as errors, not files: an analysis is free to report more than one
	// problem per file, and the listing above is what identifies them.
	return fmt.Errorf("%d %s error(s), listed above (use --allow-parse-errors to ignore)", len(errs), analysis)
}

func checkComplexity(ctx context.Context, files []string, cfg *config.Config, result *domain.CheckResult, pm domain.ProgressManager) error {
	result.Summary.ComplexityChecked = true

	svc := service.NewComplexityServiceWithProgress(&cfg.Complexity, pm)
	req := domain.ComplexityRequest{
		Paths:           files,
		LowThreshold:    cfg.Complexity.LowThreshold,
		MediumThreshold: cfg.Complexity.MediumThreshold,
		SortBy:          domain.SortCriteria(cfg.Output.SortBy),
	}

	resp, err := svc.Analyze(ctx, req)
	if err != nil {
		return fmt.Errorf("complexity analysis failed: %w", err)
	}

	if err := reportUnanalyzedFiles("complexity", resp.Errors); err != nil {
		return err
	}

	// Check each function against threshold
	for _, fn := range resp.Functions {
		if fn.Metrics.Complexity > checkMaxComplexity {
			result.Passed = false
			result.Summary.HighComplexityFunctions++
			result.Violations = append(result.Violations, domain.CheckViolation{
				Category:  "complexity",
				Rule:      "max-complexity",
				Severity:  "error",
				Message:   fmt.Sprintf("Function '%s' has complexity %d", fn.Name, fn.Metrics.Complexity),
				Location:  fmt.Sprintf("%s:%d", fn.FilePath, fn.StartLine),
				Actual:    strconv.Itoa(fn.Metrics.Complexity),
				Threshold: strconv.Itoa(checkMaxComplexity),
			})
		}
	}

	return nil
}

func checkDeadCode(_ context.Context, files []string, cfg *config.Config, result *domain.CheckResult, pm domain.ProgressManager) error {
	result.Summary.DeadCodeChecked = true

	resp, err := runDeadCodeAnalysis(files, cfg, pm)
	if err != nil {
		return fmt.Errorf("dead code analysis failed: %w", err)
	}

	if err := reportUnanalyzedFiles("dead code", resp.Errors); err != nil {
		return err
	}

	result.Summary.DeadCodeFindings = resp.Summary.TotalFindings

	if !checkAllowDeadCode && resp.Summary.TotalFindings > 0 {
		result.Passed = false

		// Add violation for critical findings
		if resp.Summary.CriticalFindings > 0 {
			result.Violations = append(result.Violations, domain.CheckViolation{
				Category:  "deadcode",
				Rule:      "no-dead-code",
				Severity:  "error",
				Message:   fmt.Sprintf("Found %d critical dead code issues", resp.Summary.CriticalFindings),
				Actual:    strconv.Itoa(resp.Summary.CriticalFindings),
				Threshold: "0",
			})
		}

		// Add violation for warnings
		if resp.Summary.WarningFindings > 0 {
			result.Violations = append(result.Violations, domain.CheckViolation{
				Category:  "deadcode",
				Rule:      "no-dead-code",
				Severity:  "warning",
				Message:   fmt.Sprintf("Found %d warning-level dead code issues", resp.Summary.WarningFindings),
				Actual:    strconv.Itoa(resp.Summary.WarningFindings),
				Threshold: "0",
			})
		}
	}

	return nil
}

func checkDependencies(ctx context.Context, files []string, _ *config.Config, result *domain.CheckResult) error {
	result.Summary.DepsChecked = true

	// Create dependency graph service
	svc := service.NewDependencyGraphService(false, true)

	req := domain.DependencyGraphRequest{
		Paths:        files,
		DetectCycles: domain.BoolPtr(true),
	}

	resp, err := svc.Analyze(ctx, req)
	if err != nil {
		return fmt.Errorf("dependency analysis failed: %w", err)
	}

	if err := reportUnanalyzedFiles("dependencies", resp.Errors); err != nil {
		return err
	}

	if resp.Analysis != nil && resp.Analysis.CircularDependencies != nil {
		cd := resp.Analysis.CircularDependencies
		result.Summary.CircularDependencies = cd.TotalCycles

		if cd.HasCircularDependencies {
			// Check against allowed cycles
			if !checkAllowCircDeps && cd.TotalCycles > checkMaxCycles {
				result.Passed = false
				result.Violations = append(result.Violations, domain.CheckViolation{
					Category:  "deps",
					Rule:      "max-cycles",
					Severity:  "error",
					Message:   fmt.Sprintf("Found %d circular dependency cycles (max: %d)", cd.TotalCycles, checkMaxCycles),
					Actual:    strconv.Itoa(cd.TotalCycles),
					Threshold: strconv.Itoa(checkMaxCycles),
				})

				// Add details for each cycle in verbose mode
				if checkVerbose {
					for _, cycle := range cd.CircularDependencies {
						result.Violations = append(result.Violations, domain.CheckViolation{
							Category: "deps",
							Rule:     "circular-dependency",
							Severity: string(cycle.Severity),
							Message:  cycle.Description,
						})
					}
				}
			}
		}
	}

	return nil
}

func outputCheckResult(result *domain.CheckResult, startTime time.Time) error {
	result.Duration = time.Since(startTime).Milliseconds()
	result.GeneratedAt = time.Now().Format(time.RFC3339)
	result.Version = version.Version
	result.ExitCode = 0
	if !result.Passed {
		result.ExitCode = 1
	}
	result.Summary.TotalViolations = len(result.Violations)

	if checkJSON {
		return outputCheckJSON(result)
	}

	return outputCheckText(result)
}

func outputCheckText(result *domain.CheckResult) error {
	if result.Passed {
		fmt.Println("PASS: All quality checks passed")
		if checkVerbose {
			fmt.Printf("  Files analyzed: %d\n", result.Summary.FilesAnalyzed)
			fmt.Printf("  Duration: %dms\n", result.Duration)
			if result.Summary.ComplexityChecked {
				fmt.Printf("  Complexity: checked (max: %d)\n", checkMaxComplexity)
			}
			if result.Summary.DeadCodeChecked {
				fmt.Printf("  Dead code: checked\n")
			}
			if result.Summary.DepsChecked {
				fmt.Printf("  Dependencies: checked\n")
			}
		}
		return nil
	}

	fmt.Println("FAIL: Quality check failed")
	fmt.Printf("  Violations: %d\n", result.Summary.TotalViolations)

	// Print violations
	for _, v := range result.Violations {
		severity := "ERROR"
		if v.Severity == "warning" {
			severity = "WARN"
		}
		fmt.Printf("  [%s] %s: %s\n", severity, v.Category, v.Message)
		if checkVerbose && v.Location != "" {
			fmt.Printf("         at %s\n", v.Location)
		}
	}

	if checkVerbose {
		fmt.Printf("\nSummary:\n")
		fmt.Printf("  Files: %d\n", result.Summary.FilesAnalyzed)
		if result.Summary.ComplexityChecked {
			fmt.Printf("  High complexity functions: %d\n", result.Summary.HighComplexityFunctions)
		}
		if result.Summary.DeadCodeChecked {
			fmt.Printf("  Dead code findings: %d\n", result.Summary.DeadCodeFindings)
		}
		if result.Summary.DepsChecked {
			fmt.Printf("  Circular dependencies: %d\n", result.Summary.CircularDependencies)
		}
		fmt.Printf("  Duration: %dms\n", result.Duration)
	}

	return &CheckExitError{Code: 1, Message: ""}
}

func outputCheckJSON(result *domain.CheckResult) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(result); err != nil {
		return &CheckExitError{Code: 2, Message: fmt.Sprintf("failed to encode JSON: %v", err)}
	}

	if !result.Passed {
		return &CheckExitError{Code: 1, Message: ""}
	}
	return nil
}

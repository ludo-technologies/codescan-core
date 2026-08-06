package service

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ludo-technologies/polyscan/jscan/domain"
	"github.com/ludo-technologies/polyscan/jscan/internal/version"
	"gopkg.in/yaml.v3"
)

// OutputFormatterImpl implements the OutputFormatter interface
type OutputFormatterImpl struct{}

// NewOutputFormatter creates a new output formatter
func NewOutputFormatter() *OutputFormatterImpl {
	return &OutputFormatterImpl{}
}

// FormatUtils provides formatting helper functions
type FormatUtils struct{}

// NewFormatUtils creates a new FormatUtils instance
func NewFormatUtils() *FormatUtils {
	return &FormatUtils{}
}

// WriteJSON writes data as JSON to the writer
func WriteJSON(writer io.Writer, data interface{}) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

// ComplexityResponseJSON wraps ComplexityResponse with JSON metadata
type ComplexityResponseJSON struct {
	Version     string                      `json:"version"`
	GeneratedAt string                      `json:"generated_at"`
	DurationMs  int64                       `json:"duration_ms,omitempty"`
	Functions   []domain.FunctionComplexity `json:"functions"`
	Summary     domain.ComplexitySummary    `json:"summary"`
	Warnings    []string                    `json:"warnings,omitempty"`
	Errors      []string                    `json:"errors,omitempty"`
	Config      interface{}                 `json:"config,omitempty"`
}

// DeadCodeResponseJSON wraps DeadCodeResponse with JSON metadata
type DeadCodeResponseJSON struct {
	Version     string                 `json:"version"`
	GeneratedAt string                 `json:"generated_at"`
	DurationMs  int64                  `json:"duration_ms,omitempty"`
	Files       []domain.FileDeadCode  `json:"files"`
	Summary     domain.DeadCodeSummary `json:"summary"`
	Warnings    []string               `json:"warnings,omitempty"`
	Errors      []string               `json:"errors,omitempty"`
	Config      interface{}            `json:"config,omitempty"`
}

// CloneResponseJSON wraps CloneResponse with JSON metadata
type CloneResponseJSON struct {
	Version     string                  `json:"version"`
	GeneratedAt string                  `json:"generated_at"`
	DurationMs  int64                   `json:"duration_ms,omitempty"`
	ClonePairs  []*domain.ClonePair     `json:"clone_pairs"`
	CloneGroups []*domain.CloneGroup    `json:"clone_groups"`
	Statistics  *domain.CloneStatistics `json:"statistics"`
	Success     bool                    `json:"success"`
	Error       string                  `json:"error,omitempty"`
	Config      interface{}             `json:"config,omitempty"`
}

// CBOResponseJSON wraps CBOResponse with JSON metadata
type CBOResponseJSON struct {
	Version     string                 `json:"version"`
	GeneratedAt string                 `json:"generated_at"`
	DurationMs  int64                  `json:"duration_ms,omitempty"`
	Classes     []domain.ClassCoupling `json:"classes"`
	Summary     domain.CBOSummary      `json:"summary"`
	Warnings    []string               `json:"warnings,omitempty"`
	Errors      []string               `json:"errors,omitempty"`
	Config      interface{}            `json:"config,omitempty"`
}

// DepsResponseJSON wraps DependencyGraphResponse with JSON metadata
type DepsResponseJSON struct {
	Version     string                           `json:"version"`
	GeneratedAt string                           `json:"generated_at"`
	Graph       *domain.DependencyGraph          `json:"graph,omitempty"`
	Analysis    *domain.DependencyAnalysisResult `json:"analysis,omitempty"`
	Warnings    []string                         `json:"warnings,omitempty"`
	Errors      []string                         `json:"errors,omitempty"`
}

// AnalyzeResponseJSON represents the unified analysis response for JSON output
type AnalyzeResponseJSON struct {
	Version     string                  `json:"version"`
	GeneratedAt string                  `json:"generated_at"`
	DurationMs  int64                   `json:"duration_ms"`
	Complexity  *ComplexityResponseJSON `json:"complexity,omitempty"`
	DeadCode    *DeadCodeResponseJSON   `json:"dead_code,omitempty"`
	Clone       *CloneResponseJSON      `json:"clone,omitempty"`
	CBO         *CBOResponseJSON        `json:"cbo,omitempty"`
	Deps        *DepsResponseJSON       `json:"deps,omitempty"`
	Summary     *domain.AnalyzeSummary  `json:"summary,omitempty"`
}

// Write writes the complexity response in the specified format
func (f *OutputFormatterImpl) Write(response *domain.ComplexityResponse, format domain.OutputFormat, writer io.Writer) error {
	switch format {
	case domain.OutputFormatJSON:
		return f.writeComplexityJSON(response, writer)
	case domain.OutputFormatText:
		return f.writeComplexityText(response, writer)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

// WriteDeadCode writes the dead code response in the specified format
func (f *OutputFormatterImpl) WriteDeadCode(response *domain.DeadCodeResponse, format domain.OutputFormat, writer io.Writer) error {
	switch format {
	case domain.OutputFormatJSON:
		return f.writeDeadCodeJSON(response, writer)
	case domain.OutputFormatText:
		return f.writeDeadCodeText(response, writer)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

// WriteAnalyze writes the unified analysis response in the specified format
func (f *OutputFormatterImpl) WriteAnalyze(
	complexityResponse *domain.ComplexityResponse,
	deadCodeResponse *domain.DeadCodeResponse,
	cloneResponse *domain.CloneResponse,
	cboResponse *domain.CBOResponse,
	depsResponse *domain.DependencyGraphResponse,
	format domain.OutputFormat,
	writer io.Writer,
	duration time.Duration,
) error {
	switch format {
	case domain.OutputFormatJSON:
		return f.writeAnalyzeJSON(complexityResponse, deadCodeResponse, cloneResponse, cboResponse, depsResponse, writer, duration)
	case domain.OutputFormatText:
		return f.writeAnalyzeText(complexityResponse, deadCodeResponse, cloneResponse, cboResponse, depsResponse, writer, duration)
	case domain.OutputFormatHTML:
		return f.WriteHTML(complexityResponse, deadCodeResponse, cloneResponse, cboResponse, depsResponse, writer, duration)
	case domain.OutputFormatYAML:
		return f.writeAnalyzeYAML(complexityResponse, deadCodeResponse, cloneResponse, cboResponse, depsResponse, writer, duration)
	case domain.OutputFormatCSV:
		return f.writeAnalyzeCSV(complexityResponse, deadCodeResponse, cloneResponse, cboResponse, depsResponse, writer, duration)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

// writeComplexityJSON writes complexity response as JSON
func (f *OutputFormatterImpl) writeComplexityJSON(response *domain.ComplexityResponse, writer io.Writer) error {
	jsonResponse := ComplexityResponseJSON{
		Version:     version.Version,
		GeneratedAt: response.GeneratedAt,
		Functions:   response.Functions,
		Summary:     response.Summary,
		Warnings:    response.Warnings,
		Errors:      response.Errors,
		Config:      response.Config,
	}
	return WriteJSON(writer, jsonResponse)
}

// writeDeadCodeJSON writes dead code response as JSON
func (f *OutputFormatterImpl) writeDeadCodeJSON(response *domain.DeadCodeResponse, writer io.Writer) error {
	jsonResponse := DeadCodeResponseJSON{
		Version:     version.Version,
		GeneratedAt: response.GeneratedAt,
		Files:       response.Files,
		Summary:     response.Summary,
		Warnings:    response.Warnings,
		Errors:      response.Errors,
		Config:      response.Config,
	}
	return WriteJSON(writer, jsonResponse)
}

// BuildAnalyzeSummary builds an AnalyzeSummary from analysis responses
func BuildAnalyzeSummary(
	complexityResponse *domain.ComplexityResponse,
	deadCodeResponse *domain.DeadCodeResponse,
	cloneResponse *domain.CloneResponse,
	cboResponse *domain.CBOResponse,
	depsResponse *domain.DependencyGraphResponse,
) *domain.AnalyzeSummary {
	summary := &domain.AnalyzeSummary{}

	if complexityResponse != nil {
		summary.ComplexityEnabled = true
		summary.TotalFunctions = complexityResponse.Summary.TotalFunctions
		summary.FunctionsParsed = complexityResponse.Summary.FunctionsParsed
		summary.AverageComplexity = complexityResponse.Summary.AverageComplexity
		summary.HighComplexityCount = complexityResponse.Summary.HighRiskFunctions
		summary.MediumComplexityCount = complexityResponse.Summary.MediumRiskFunctions
		summary.AnalyzedFiles = complexityResponse.Summary.FilesAnalyzed
	}

	if deadCodeResponse != nil {
		summary.DeadCodeEnabled = true
		summary.DeadCodeCount = deadCodeResponse.Summary.TotalFindings
		summary.CriticalDeadCode = deadCodeResponse.Summary.CriticalFindings
		summary.WarningDeadCode = deadCodeResponse.Summary.WarningFindings
		summary.InfoDeadCode = deadCodeResponse.Summary.InfoFindings
		if deadCodeResponse.Summary.TotalFiles > summary.TotalFiles {
			summary.TotalFiles = deadCodeResponse.Summary.TotalFiles
		}
	}

	if cloneResponse != nil {
		summary.CloneEnabled = true
		if cloneResponse.Statistics != nil {
			summary.TotalClones = cloneResponse.Statistics.TotalClones
			summary.ClonePairs = cloneResponse.Statistics.TotalClonePairs
			summary.CloneGroups = cloneResponse.Statistics.TotalCloneGroups
			summary.CodeDuplication = calculateDuplicationPercentage(cloneResponse)
			summary.TotalLOC = cloneResponse.Statistics.LinesAnalyzed
		}
	}

	if cboResponse != nil {
		summary.CBOEnabled = true
		summary.CBOClasses = cboResponse.Summary.TotalClasses
		summary.HighCouplingClasses = cboResponse.Summary.HighRiskClasses
		summary.MediumCouplingClasses = cboResponse.Summary.MediumRiskClasses
		summary.AverageCoupling = cboResponse.Summary.AverageCBO
	}

	if depsResponse != nil {
		summary.DepsEnabled = true
		if depsResponse.Graph != nil {
			summary.DepsTotalModules = depsResponse.Graph.NodeCount()
		}
		if depsResponse.Analysis != nil {
			if depsResponse.Analysis.CircularDependencies != nil {
				summary.DepsModulesInCycles = depsResponse.Analysis.CircularDependencies.TotalModulesInCycles
			}
			summary.DepsMaxDepth = depsResponse.Analysis.MaxDepth
			if depsResponse.Analysis.CouplingAnalysis != nil {
				summary.DepsMainSequenceDeviation = depsResponse.Analysis.CouplingAnalysis.MainSequenceDeviation
			}
		}
	}

	_ = summary.CalculateHealthScore()
	return summary
}

// FormatProjectScale renders the project scale label together with the counts it
// was derived from, e.g. "Medium (123 files, 456 functions, 7890 LOC)". The LOC
// part is dropped when clone analysis did not run and no line count is available.
func FormatProjectScale(summary *domain.AnalyzeSummary) string {
	if summary.TotalLOC > 0 {
		return fmt.Sprintf("%s (%d files, %d functions, %d LOC)",
			summary.ProjectScale, summary.AnalyzedFiles, summary.TotalFunctions, summary.TotalLOC)
	}
	return fmt.Sprintf("%s (%d files, %d functions)",
		summary.ProjectScale, summary.AnalyzedFiles, summary.TotalFunctions)
}

// FormatCLISummary formats an AnalyzeSummary as a compact CLI string (pyscn-style)
func FormatCLISummary(summary *domain.AnalyzeSummary, duration time.Duration) string {
	w := &strings.Builder{}

	fmt.Fprintf(w, "\n\U0001F4CA Analysis Summary:\n")
	fmt.Fprintf(w, "Health Score: %d/100 (Grade: %s)\n", summary.HealthScore, summary.Grade)
	fmt.Fprintf(w, "Project Scale: %s\n", FormatProjectScale(summary))
	fmt.Fprintf(w, "Total time: %dms\n", duration.Milliseconds())

	fmt.Fprintf(w, "\n\U0001F4C8 Detailed Scores:\n")

	if summary.ComplexityEnabled {
		fmt.Fprintf(w, "  Complexity:      %3d/100 %s  (avg: %.1f, high-risk: %d functions)\n",
			summary.ComplexityScore, scoreIndicator(summary.ComplexityScore),
			summary.AverageComplexity, summary.HighComplexityCount)
	}
	if summary.DeadCodeEnabled {
		fmt.Fprintf(w, "  Dead Code:       %3d/100 %s  (%d issues, %d critical)\n",
			summary.DeadCodeScore, scoreIndicator(summary.DeadCodeScore),
			summary.DeadCodeCount, summary.CriticalDeadCode)
	}
	if summary.CloneEnabled {
		fmt.Fprintf(w, "  Duplication:     %3d/100 %s  (%.1f%% fragments cloned, %d groups)\n",
			summary.DuplicationScore, scoreIndicator(summary.DuplicationScore),
			summary.CodeDuplication, summary.CloneGroups)
	}
	if summary.CBOEnabled {
		fmt.Fprintf(w, "  Coupling (CBO):  %3d/100 %s  (avg: %.1f, %d/%d high-coupling)\n",
			summary.CouplingScore, scoreIndicator(summary.CouplingScore),
			summary.AverageCoupling, summary.HighCouplingClasses, summary.CBOClasses)
	}
	if summary.DepsEnabled {
		cycles := 0
		if summary.DepsModulesInCycles > 0 {
			cycles = summary.DepsModulesInCycles
		}
		fmt.Fprintf(w, "  Dependencies:    %3d/100 %s  (%d cycles, depth: %d)\n",
			summary.DependencyScore, scoreIndicator(summary.DependencyScore),
			cycles, summary.DepsMaxDepth)
	}

	return w.String()
}

// scoreIndicator returns a status emoji based on the score.
// Thresholds align with grade boundaries: ✅ A/B (>=75), ⚠️ C (>=60), ❌ D/F (<60)
func scoreIndicator(score int) string {
	switch {
	case score >= domain.ScoreThresholdGood:
		return "\u2705" // ✅
	case score >= domain.ScoreThresholdFair:
		return "\u26A0\uFE0F" // ⚠️
	default:
		return "\u274C" // ❌
	}
}

// writeAnalyzeJSON writes unified analysis response as JSON
func (f *OutputFormatterImpl) writeAnalyzeJSON(
	complexityResponse *domain.ComplexityResponse,
	deadCodeResponse *domain.DeadCodeResponse,
	cloneResponse *domain.CloneResponse,
	cboResponse *domain.CBOResponse,
	depsResponse *domain.DependencyGraphResponse,
	writer io.Writer,
	duration time.Duration,
) error {
	now := time.Now()

	response := AnalyzeResponseJSON{
		Version:     version.Version,
		GeneratedAt: now.Format(time.RFC3339),
		DurationMs:  duration.Milliseconds(),
	}

	// Add individual response data
	if complexityResponse != nil {
		response.Complexity = &ComplexityResponseJSON{
			Version:     version.Version,
			GeneratedAt: complexityResponse.GeneratedAt,
			Functions:   complexityResponse.Functions,
			Summary:     complexityResponse.Summary,
			Warnings:    complexityResponse.Warnings,
			Errors:      complexityResponse.Errors,
			Config:      complexityResponse.Config,
		}
	}
	if deadCodeResponse != nil {
		response.DeadCode = &DeadCodeResponseJSON{
			Version:     version.Version,
			GeneratedAt: deadCodeResponse.GeneratedAt,
			Files:       deadCodeResponse.Files,
			Summary:     deadCodeResponse.Summary,
			Warnings:    deadCodeResponse.Warnings,
			Errors:      deadCodeResponse.Errors,
			Config:      deadCodeResponse.Config,
		}
	}
	if cloneResponse != nil {
		response.Clone = &CloneResponseJSON{
			Version:     version.Version,
			GeneratedAt: now.Format(time.RFC3339),
			DurationMs:  cloneResponse.Duration,
			ClonePairs:  cloneResponse.ClonePairs,
			CloneGroups: cloneResponse.CloneGroups,
			Statistics:  cloneResponse.Statistics,
			Success:     cloneResponse.Success,
			Error:       cloneResponse.Error,
		}
	}
	if cboResponse != nil {
		response.CBO = &CBOResponseJSON{
			Version:     version.Version,
			GeneratedAt: cboResponse.GeneratedAt,
			Classes:     cboResponse.Classes,
			Summary:     cboResponse.Summary,
			Warnings:    cboResponse.Warnings,
			Errors:      cboResponse.Errors,
			Config:      cboResponse.Config,
		}
	}
	if depsResponse != nil {
		response.Deps = &DepsResponseJSON{
			Version:     version.Version,
			GeneratedAt: depsResponse.GeneratedAt,
			Graph:       depsResponse.Graph,
			Analysis:    depsResponse.Analysis,
			Warnings:    depsResponse.Warnings,
			Errors:      depsResponse.Errors,
		}
	}

	summary := BuildAnalyzeSummary(complexityResponse, deadCodeResponse, cloneResponse, cboResponse, depsResponse)
	response.Summary = summary

	return WriteJSON(writer, response)
}

// calculateDuplicationPercentage calculates the code duplication metric based on
// fragment ratio: the proportion of all code fragments that are involved in
// duplication (part of at least one clone pair or group).
func calculateDuplicationPercentage(response *domain.CloneResponse) float64 {
	if response == nil || response.Statistics == nil {
		return 0.0
	}

	totalFragments := response.Statistics.TotalFragments
	totalClones := response.Statistics.TotalClones
	if totalFragments == 0 || totalClones == 0 {
		return 0.0
	}

	return float64(totalClones) / float64(totalFragments) * 100
}

// writeComplexityText writes complexity response as plain text
// formatFunctionCoverage renders the post-filter function count, disclosing the
// pre-filter parsed count when min_complexity filtering dropped functions.
func formatFunctionCoverage(reported, parsed int) string {
	if parsed > 0 && parsed != reported {
		return fmt.Sprintf("%d reported / %d parsed", reported, parsed)
	}
	return fmt.Sprintf("%d", reported)
}

// complexityFunctionsHeading names the criterion the functions are listed by,
// taken from the configuration the analysis reported back so that the heading
// cannot drift away from the actual order.
//
// When the criterion cannot be read back, the heading claims nothing rather
// than guessing: a heading that names the wrong order is worse than one that
// names none. The value arrives as a SortCriteria in process and as a plain
// string once the response has been through JSON, so both are accepted.
func complexityFunctionsHeading(responseConfig interface{}) string {
	cfg, ok := responseConfig.(map[string]interface{})
	if !ok {
		return "Functions:"
	}

	var sortBy string
	switch value := cfg["sort_by"].(type) {
	case domain.SortCriteria:
		sortBy = string(value)
	case string:
		sortBy = value
	}
	if sortBy == "" {
		return "Functions:"
	}
	return fmt.Sprintf("Functions (sorted by %s):", sortBy)
}

func (f *OutputFormatterImpl) writeComplexityText(response *domain.ComplexityResponse, writer io.Writer) error {
	fmt.Fprintf(writer, "\n=== Complexity Analysis ===\n\n")
	fmt.Fprintf(writer, "Generated: %s\n", response.GeneratedAt)
	fmt.Fprintf(writer, "Version: %s\n\n", response.Version)

	// Summary
	fmt.Fprintf(writer, "Summary:\n")
	fmt.Fprintf(writer, "  Files analyzed: %d\n", response.Summary.FilesAnalyzed)
	fmt.Fprintf(writer, "  Total functions: %s\n", formatFunctionCoverage(response.Summary.TotalFunctions, response.Summary.FunctionsParsed))
	fmt.Fprintf(writer, "  Average complexity: %.2f\n", response.Summary.AverageComplexity)
	fmt.Fprintf(writer, "  Max complexity: %d\n", response.Summary.MaxComplexity)
	fmt.Fprintf(writer, "  Min complexity: %d\n", response.Summary.MinComplexity)
	fmt.Fprintf(writer, "\n")

	// Risk distribution
	fmt.Fprintf(writer, "Risk Distribution:\n")
	fmt.Fprintf(writer, "  High risk: %d\n", response.Summary.HighRiskFunctions)
	fmt.Fprintf(writer, "  Medium risk: %d\n", response.Summary.MediumRiskFunctions)
	fmt.Fprintf(writer, "  Low risk: %d\n", response.Summary.LowRiskFunctions)
	fmt.Fprintf(writer, "\n")

	// Function details
	if len(response.Functions) > 0 {
		fmt.Fprintf(writer, "%s\n", complexityFunctionsHeading(response.Config))
		for _, fn := range response.Functions {
			riskIndicator := ""
			switch fn.RiskLevel {
			case domain.RiskLevelHigh:
				riskIndicator = " [HIGH]"
			case domain.RiskLevelMedium:
				riskIndicator = " [MEDIUM]"
			}
			fmt.Fprintf(writer, "  %s: %d%s\n", fn.Name, fn.Metrics.Complexity, riskIndicator)
			fmt.Fprintf(writer, "    File: %s:%d-%d\n", fn.FilePath, fn.StartLine, fn.EndLine)
		}
	}

	// Warnings
	if len(response.Warnings) > 0 {
		fmt.Fprintf(writer, "\nWarnings:\n")
		for _, w := range response.Warnings {
			fmt.Fprintf(writer, "  - %s\n", w)
		}
	}

	// Errors
	if len(response.Errors) > 0 {
		fmt.Fprintf(writer, "\nErrors:\n")
		for _, e := range response.Errors {
			fmt.Fprintf(writer, "  - %s\n", e)
		}
	}

	return nil
}

// writeDeadCodeText writes dead code response as plain text
func (f *OutputFormatterImpl) writeDeadCodeText(response *domain.DeadCodeResponse, writer io.Writer) error {
	fmt.Fprintf(writer, "\n=== Dead Code Analysis ===\n\n")
	fmt.Fprintf(writer, "Generated: %s\n", response.GeneratedAt)
	fmt.Fprintf(writer, "Version: %s\n\n", response.Version)

	// Summary
	fmt.Fprintf(writer, "Summary:\n")
	fmt.Fprintf(writer, "  Total files: %d\n", response.Summary.TotalFiles)
	fmt.Fprintf(writer, "  Total functions: %d\n", response.Summary.TotalFunctions)
	fmt.Fprintf(writer, "  Total findings: %d\n", response.Summary.TotalFindings)
	fmt.Fprintf(writer, "\n")

	// Severity distribution
	fmt.Fprintf(writer, "Severity Distribution:\n")
	fmt.Fprintf(writer, "  Critical: %d\n", response.Summary.CriticalFindings)
	fmt.Fprintf(writer, "  Warning: %d\n", response.Summary.WarningFindings)
	fmt.Fprintf(writer, "  Info: %d\n", response.Summary.InfoFindings)
	fmt.Fprintf(writer, "\n")

	// File details
	for _, file := range response.Files {
		if file.TotalFindings > 0 {
			fmt.Fprintf(writer, "%s:\n", file.FilePath)

			// File-level findings (unused imports/exports)
			for _, finding := range file.FileLevelFindings {
				severityIndicator := ""
				switch finding.Severity {
				case domain.DeadCodeSeverityCritical:
					severityIndicator = " [CRITICAL]"
				case domain.DeadCodeSeverityWarning:
					severityIndicator = " [WARNING]"
				case domain.DeadCodeSeverityInfo:
					severityIndicator = " [INFO]"
				}
				fmt.Fprintf(writer, "  <file-level>:\n")
				fmt.Fprintf(writer, "    Line %d-%d: %s%s\n",
					finding.Location.StartLine, finding.Location.EndLine,
					finding.Reason, severityIndicator)
				if finding.Description != "" {
					fmt.Fprintf(writer, "      %s\n", finding.Description)
				}
			}

			// Function-level findings
			for _, fn := range file.Functions {
				if len(fn.Findings) > 0 {
					fmt.Fprintf(writer, "  %s:\n", fn.Name)
					for _, finding := range fn.Findings {
						severityIndicator := ""
						switch finding.Severity {
						case domain.DeadCodeSeverityCritical:
							severityIndicator = " [CRITICAL]"
						case domain.DeadCodeSeverityWarning:
							severityIndicator = " [WARNING]"
						case domain.DeadCodeSeverityInfo:
							severityIndicator = " [INFO]"
						}
						fmt.Fprintf(writer, "    Line %d-%d: %s%s\n",
							finding.Location.StartLine, finding.Location.EndLine,
							finding.Reason, severityIndicator)
						if finding.Description != "" {
							fmt.Fprintf(writer, "      %s\n", finding.Description)
						}
					}
				}
			}
		}
	}

	if response.Summary.TotalFindings == 0 {
		fmt.Fprintf(writer, "No dead code found.\n")
	}

	return nil
}

// writeAnalyzeText writes unified analysis response as plain text
func (f *OutputFormatterImpl) writeAnalyzeText(
	complexityResponse *domain.ComplexityResponse,
	deadCodeResponse *domain.DeadCodeResponse,
	cloneResponse *domain.CloneResponse,
	cboResponse *domain.CBOResponse,
	depsResponse *domain.DependencyGraphResponse,
	writer io.Writer,
	duration time.Duration,
) error {
	fmt.Fprintf(writer, "\n=== jscan Analysis Report ===\n")
	fmt.Fprintf(writer, "Generated: %s\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(writer, "Duration: %dms\n", duration.Milliseconds())
	fmt.Fprintf(writer, "Version: %s\n\n", version.Version)

	// Complexity results
	if complexityResponse != nil {
		if err := f.writeComplexityText(complexityResponse, writer); err != nil {
			return err
		}
	}

	// Dead code results
	if deadCodeResponse != nil {
		if err := f.writeDeadCodeText(deadCodeResponse, writer); err != nil {
			return err
		}
	}

	// Clone detection results
	if cloneResponse != nil {
		if err := f.writeCloneText(cloneResponse, writer); err != nil {
			return err
		}
	}

	// CBO analysis results
	if cboResponse != nil {
		if err := f.writeCBOText(cboResponse, writer); err != nil {
			return err
		}
	}

	// Dependency analysis results
	if depsResponse != nil {
		if err := f.writeDepsText(depsResponse, writer); err != nil {
			return err
		}
	}

	summary := BuildAnalyzeSummary(complexityResponse, deadCodeResponse, cloneResponse, cboResponse, depsResponse)

	// Write Health Score section
	fmt.Fprintf(writer, "\n=== Health Score ===\n\n")
	fmt.Fprintf(writer, "Overall: %d/100 (Grade: %s)\n", summary.HealthScore, summary.Grade)
	fmt.Fprintf(writer, "Project Scale: %s\n\n", FormatProjectScale(summary))
	fmt.Fprintf(writer, "Category Scores:\n")
	fmt.Fprintf(writer, "  Complexity:       %3d/100\n", summary.ComplexityScore)
	fmt.Fprintf(writer, "  Dead Code:        %3d/100\n", summary.DeadCodeScore)
	fmt.Fprintf(writer, "  Code Duplication: %3d/100\n", summary.DuplicationScore)
	fmt.Fprintf(writer, "  Coupling:         %3d/100\n", summary.CouplingScore)
	fmt.Fprintf(writer, "  Dependencies:     %3d/100\n", summary.DependencyScore)

	return nil
}

// writeDepsText writes dependency analysis results as plain text
func (f *OutputFormatterImpl) writeDepsText(response *domain.DependencyGraphResponse, writer io.Writer) error {
	fmt.Fprintf(writer, "\n=== Dependency Analysis ===\n\n")

	if response.Graph != nil {
		fmt.Fprintf(writer, "Summary:\n")
		fmt.Fprintf(writer, "  Total modules: %d\n", response.Graph.NodeCount())
		fmt.Fprintf(writer, "  Total dependencies: %d\n", response.Graph.EdgeCount())
	}

	if response.Analysis != nil {
		fmt.Fprintf(writer, "  Entry points: %d\n", len(response.Analysis.RootModules))
		fmt.Fprintf(writer, "  Leaf modules: %d\n", len(response.Analysis.LeafModules))
		fmt.Fprintf(writer, "  Max depth: %d\n", response.Analysis.MaxDepth)

		if response.Analysis.CircularDependencies != nil && response.Analysis.CircularDependencies.HasCircularDependencies {
			cd := response.Analysis.CircularDependencies
			fmt.Fprintf(writer, "\nCircular Dependencies:\n")
			fmt.Fprintf(writer, "  Cycles found: %d\n", cd.TotalCycles)
			fmt.Fprintf(writer, "  Modules in cycles: %d\n", cd.TotalModulesInCycles)

			for i, cycle := range cd.CircularDependencies {
				if i >= 5 {
					fmt.Fprintf(writer, "  ... and %d more cycles\n", len(cd.CircularDependencies)-5)
					break
				}
				fmt.Fprintf(writer, "  Cycle %d [%s]: %v\n", i+1, cycle.Severity, cycle.Modules)
			}
		} else {
			fmt.Fprintf(writer, "\nNo circular dependencies detected.\n")
		}
	}

	return nil
}

// writeCloneText writes clone detection results as plain text
func (f *OutputFormatterImpl) writeCloneText(response *domain.CloneResponse, writer io.Writer) error {
	fmt.Fprintf(writer, "\n=== Clone Detection ===\n\n")

	if response.Statistics != nil {
		fmt.Fprintf(writer, "Statistics:\n")
		fmt.Fprintf(writer, "  Total clone pairs: %d\n", response.Statistics.TotalClonePairs)
		fmt.Fprintf(writer, "  Total clone groups: %d\n", response.Statistics.TotalCloneGroups)
		fmt.Fprintf(writer, "  Files analyzed: %d\n", response.Statistics.FilesAnalyzed)
		fmt.Fprintf(writer, "  Average similarity: %.2f\n", response.Statistics.AverageSimilarity)
		fmt.Fprintf(writer, "\n")

		// Clone type distribution
		if len(response.Statistics.ClonesByType) > 0 {
			fmt.Fprintf(writer, "Clone Types:\n")
			for cloneType, count := range response.Statistics.ClonesByType {
				fmt.Fprintf(writer, "  %s: %d\n", cloneType, count)
			}
			fmt.Fprintf(writer, "\n")
		}
	}

	// Top clone pairs
	if len(response.ClonePairs) > 0 {
		fmt.Fprintf(writer, "Top Clone Pairs:\n")
		limit := 10
		if len(response.ClonePairs) < limit {
			limit = len(response.ClonePairs)
		}
		for i := 0; i < limit; i++ {
			pair := response.ClonePairs[i]
			loc1 := "unknown"
			loc2 := "unknown"
			if pair.Clone1 != nil && pair.Clone1.Location != nil {
				loc1 = pair.Clone1.Location.String()
			}
			if pair.Clone2 != nil && pair.Clone2.Location != nil {
				loc2 = pair.Clone2.Location.String()
			}
			fmt.Fprintf(writer, "  %s: %s <-> %s (%.1f%% similar)\n",
				pair.Type.String(), loc1, loc2, pair.Similarity*100)
		}
	} else {
		fmt.Fprintf(writer, "No code clones detected.\n")
	}

	return nil
}

// writeCBOText writes CBO analysis results as plain text
func (f *OutputFormatterImpl) writeCBOText(response *domain.CBOResponse, writer io.Writer) error {
	fmt.Fprintf(writer, "\n=== CBO Analysis ===\n\n")

	fmt.Fprintf(writer, "Summary:\n")
	fmt.Fprintf(writer, "  Total classes: %d\n", response.Summary.TotalClasses)
	fmt.Fprintf(writer, "  Average CBO: %.2f\n", response.Summary.AverageCBO)
	fmt.Fprintf(writer, "  Max CBO: %d\n", response.Summary.MaxCBO)
	fmt.Fprintf(writer, "\n")

	fmt.Fprintf(writer, "Risk Distribution:\n")
	fmt.Fprintf(writer, "  High risk: %d\n", response.Summary.HighRiskClasses)
	fmt.Fprintf(writer, "  Medium risk: %d\n", response.Summary.MediumRiskClasses)
	fmt.Fprintf(writer, "  Low risk: %d\n", response.Summary.LowRiskClasses)
	fmt.Fprintf(writer, "\n")

	// Top coupled classes
	if len(response.Summary.MostCoupledClasses) > 0 {
		fmt.Fprintf(writer, "Most Coupled Classes:\n")
		for _, class := range response.Summary.MostCoupledClasses {
			fmt.Fprintf(writer, "  %s: CBO=%d [%s]\n",
				class.Name, class.Metrics.CouplingCount, class.RiskLevel)
		}
	} else if len(response.Classes) == 0 {
		fmt.Fprintf(writer, "No classes found for CBO analysis.\n")
	}

	return nil
}

// writeAnalyzeYAML writes unified analysis response as YAML
func (f *OutputFormatterImpl) writeAnalyzeYAML(
	complexityResponse *domain.ComplexityResponse,
	deadCodeResponse *domain.DeadCodeResponse,
	cloneResponse *domain.CloneResponse,
	cboResponse *domain.CBOResponse,
	depsResponse *domain.DependencyGraphResponse,
	writer io.Writer,
	duration time.Duration,
) error {
	now := time.Now()

	response := AnalyzeResponseJSON{
		Version:     version.Version,
		GeneratedAt: now.Format(time.RFC3339),
		DurationMs:  duration.Milliseconds(),
	}

	if complexityResponse != nil {
		response.Complexity = &ComplexityResponseJSON{
			Version:     version.Version,
			GeneratedAt: complexityResponse.GeneratedAt,
			Functions:   complexityResponse.Functions,
			Summary:     complexityResponse.Summary,
			Warnings:    complexityResponse.Warnings,
			Errors:      complexityResponse.Errors,
			Config:      complexityResponse.Config,
		}
	}
	if deadCodeResponse != nil {
		response.DeadCode = &DeadCodeResponseJSON{
			Version:     version.Version,
			GeneratedAt: deadCodeResponse.GeneratedAt,
			Files:       deadCodeResponse.Files,
			Summary:     deadCodeResponse.Summary,
			Warnings:    deadCodeResponse.Warnings,
			Errors:      deadCodeResponse.Errors,
			Config:      deadCodeResponse.Config,
		}
	}
	if cloneResponse != nil {
		response.Clone = &CloneResponseJSON{
			Version:     version.Version,
			GeneratedAt: now.Format(time.RFC3339),
			DurationMs:  cloneResponse.Duration,
			ClonePairs:  cloneResponse.ClonePairs,
			CloneGroups: cloneResponse.CloneGroups,
			Statistics:  cloneResponse.Statistics,
			Success:     cloneResponse.Success,
			Error:       cloneResponse.Error,
		}
	}
	if cboResponse != nil {
		response.CBO = &CBOResponseJSON{
			Version:     version.Version,
			GeneratedAt: cboResponse.GeneratedAt,
			Classes:     cboResponse.Classes,
			Summary:     cboResponse.Summary,
			Warnings:    cboResponse.Warnings,
			Errors:      cboResponse.Errors,
			Config:      cboResponse.Config,
		}
	}
	if depsResponse != nil {
		response.Deps = &DepsResponseJSON{
			Version:     version.Version,
			GeneratedAt: depsResponse.GeneratedAt,
			Graph:       depsResponse.Graph,
			Analysis:    depsResponse.Analysis,
			Warnings:    depsResponse.Warnings,
			Errors:      depsResponse.Errors,
		}
	}

	summary := BuildAnalyzeSummary(complexityResponse, deadCodeResponse, cloneResponse, cboResponse, depsResponse)
	response.Summary = summary

	// Write YAML
	encoder := yaml.NewEncoder(writer)
	encoder.SetIndent(2)
	return encoder.Encode(response)
}

// writeAnalyzeCSV writes unified analysis response as CSV
func (f *OutputFormatterImpl) writeAnalyzeCSV(
	complexityResponse *domain.ComplexityResponse,
	deadCodeResponse *domain.DeadCodeResponse,
	cloneResponse *domain.CloneResponse,
	cboResponse *domain.CBOResponse,
	depsResponse *domain.DependencyGraphResponse,
	writer io.Writer,
	duration time.Duration,
) error {
	csvWriter := csv.NewWriter(writer)
	defer csvWriter.Flush()

	needsSeparator := false

	// Write complexity results
	if complexityResponse != nil {
		// Write header
		if err := csvWriter.Write([]string{
			"type", "file", "function", "start_line", "end_line",
			"complexity", "risk_level", "nodes", "edges",
		}); err != nil {
			return err
		}

		// Write function data
		for _, fn := range complexityResponse.Functions {
			record := []string{
				"complexity",
				fn.FilePath,
				fn.Name,
				strconv.Itoa(fn.StartLine),
				strconv.Itoa(fn.EndLine),
				strconv.Itoa(fn.Metrics.Complexity),
				string(fn.RiskLevel),
				strconv.Itoa(fn.Metrics.Nodes),
				strconv.Itoa(fn.Metrics.Edges),
			}
			if err := csvWriter.Write(record); err != nil {
				return err
			}
		}
		needsSeparator = true
	}

	// Write dead code results
	if deadCodeResponse != nil {
		if needsSeparator {
			if err := csvWriter.Write([]string{}); err != nil {
				return err
			}
		}
		if err := csvWriter.Write([]string{
			"type", "file", "function", "start_line", "end_line",
			"severity", "reason", "description",
		}); err != nil {
			return err
		}

		// Write dead code findings
		for _, file := range deadCodeResponse.Files {
			// File-level findings (unused imports/exports)
			for _, finding := range file.FileLevelFindings {
				record := []string{
					"dead_code",
					finding.Location.FilePath,
					"<file-level>",
					strconv.Itoa(finding.Location.StartLine),
					strconv.Itoa(finding.Location.EndLine),
					string(finding.Severity),
					finding.Reason,
					finding.Description,
				}
				if err := csvWriter.Write(record); err != nil {
					return err
				}
			}
			// Function-level findings
			for _, fn := range file.Functions {
				for _, finding := range fn.Findings {
					record := []string{
						"dead_code",
						finding.Location.FilePath,
						finding.FunctionName,
						strconv.Itoa(finding.Location.StartLine),
						strconv.Itoa(finding.Location.EndLine),
						string(finding.Severity),
						finding.Reason,
						finding.Description,
					}
					if err := csvWriter.Write(record); err != nil {
						return err
					}
				}
			}
		}
		needsSeparator = true
	}

	// Write clone results
	if cloneResponse != nil && len(cloneResponse.ClonePairs) > 0 {
		if needsSeparator {
			if err := csvWriter.Write([]string{}); err != nil {
				return err
			}
		}
		if err := csvWriter.Write([]string{
			"type", "file1", "start_line1", "end_line1",
			"file2", "start_line2", "end_line2",
			"clone_type", "similarity",
		}); err != nil {
			return err
		}

		for _, pair := range cloneResponse.ClonePairs {
			file1, start1, end1 := "", "0", "0"
			file2, start2, end2 := "", "0", "0"
			if pair.Clone1 != nil && pair.Clone1.Location != nil {
				file1 = pair.Clone1.Location.FilePath
				start1 = strconv.Itoa(pair.Clone1.Location.StartLine)
				end1 = strconv.Itoa(pair.Clone1.Location.EndLine)
			}
			if pair.Clone2 != nil && pair.Clone2.Location != nil {
				file2 = pair.Clone2.Location.FilePath
				start2 = strconv.Itoa(pair.Clone2.Location.StartLine)
				end2 = strconv.Itoa(pair.Clone2.Location.EndLine)
			}
			record := []string{
				"clone",
				file1, start1, end1,
				file2, start2, end2,
				pair.Type.String(),
				fmt.Sprintf("%.3f", pair.Similarity),
			}
			if err := csvWriter.Write(record); err != nil {
				return err
			}
		}
		needsSeparator = true
	}

	// Write CBO results
	if cboResponse != nil && len(cboResponse.Classes) > 0 {
		if needsSeparator {
			if err := csvWriter.Write([]string{}); err != nil {
				return err
			}
		}
		if err := csvWriter.Write([]string{
			"type", "class", "file", "cbo", "risk_level",
		}); err != nil {
			return err
		}

		for _, class := range cboResponse.Classes {
			record := []string{
				"cbo",
				class.Name,
				class.FilePath,
				strconv.Itoa(class.Metrics.CouplingCount),
				string(class.RiskLevel),
			}
			if err := csvWriter.Write(record); err != nil {
				return err
			}
		}
		needsSeparator = true
	}

	// Write dependency graph results
	if depsResponse != nil && depsResponse.Graph != nil {
		if needsSeparator {
			if err := csvWriter.Write([]string{}); err != nil {
				return err
			}
		}
		if err := csvWriter.Write([]string{
			"type", "from", "to", "edge_type", "weight",
		}); err != nil {
			return err
		}

		fromIDs := make([]string, 0, len(depsResponse.Graph.Edges))
		for from := range depsResponse.Graph.Edges {
			fromIDs = append(fromIDs, from)
		}
		sort.Strings(fromIDs)

		for _, from := range fromIDs {
			edges := append([]*domain.DependencyEdge(nil), depsResponse.Graph.Edges[from]...)
			sort.Slice(edges, func(i, j int) bool {
				if edges[i].To == edges[j].To {
					return edges[i].EdgeType < edges[j].EdgeType
				}
				return edges[i].To < edges[j].To
			})

			for _, edge := range edges {
				record := []string{
					"deps",
					edge.From,
					edge.To,
					string(edge.EdgeType),
					strconv.Itoa(edge.Weight),
				}
				if err := csvWriter.Write(record); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// WriteDependencyGraph writes the dependency graph response in the specified format
func (f *OutputFormatterImpl) WriteDependencyGraph(response *domain.DependencyGraphResponse, format domain.OutputFormat, writer io.Writer) error {
	switch format {
	case domain.OutputFormatJSON:
		return f.writeDependencyGraphJSON(response, writer)
	case domain.OutputFormatText:
		return f.writeDependencyGraphText(response, writer)
	case domain.OutputFormatDOT:
		dotFormatter := NewDOTFormatter(nil)
		return dotFormatter.WriteDependencyGraph(response, writer)
	default:
		return fmt.Errorf("unsupported output format for dependency graph: %s", format)
	}
}

// writeDependencyGraphJSON writes dependency graph as JSON
func (f *OutputFormatterImpl) writeDependencyGraphJSON(response *domain.DependencyGraphResponse, writer io.Writer) error {
	return WriteJSON(writer, response)
}

// writeDependencyGraphText writes dependency graph as plain text
func (f *OutputFormatterImpl) writeDependencyGraphText(response *domain.DependencyGraphResponse, writer io.Writer) error {
	fmt.Fprintf(writer, "\n=== Dependency Graph Analysis ===\n\n")
	fmt.Fprintf(writer, "Generated: %s\n", response.GeneratedAt)
	fmt.Fprintf(writer, "Version: %s\n\n", response.Version)

	if response.Graph == nil {
		fmt.Fprintln(writer, "No graph data available.")
		return nil
	}

	graph := response.Graph
	analysis := response.Analysis

	// Summary
	fmt.Fprintln(writer, "Summary:")
	fmt.Fprintf(writer, "  Total modules: %d\n", graph.NodeCount())
	fmt.Fprintf(writer, "  Total dependencies: %d\n", graph.EdgeCount())

	if analysis != nil {
		fmt.Fprintf(writer, "  Root modules (entry points): %d\n", len(analysis.RootModules))
		fmt.Fprintf(writer, "  Leaf modules (no dependencies): %d\n", len(analysis.LeafModules))
		fmt.Fprintf(writer, "  Max depth: %d\n", analysis.MaxDepth)
	}
	fmt.Fprintln(writer)

	// Circular dependencies
	if analysis != nil && analysis.CircularDependencies != nil && analysis.CircularDependencies.HasCircularDependencies {
		cd := analysis.CircularDependencies
		fmt.Fprintln(writer, "Circular Dependencies:")
		fmt.Fprintf(writer, "  Total cycles: %d\n", cd.TotalCycles)
		fmt.Fprintf(writer, "  Modules in cycles: %d\n", cd.TotalModulesInCycles)
		fmt.Fprintln(writer)

		for i, cycle := range cd.CircularDependencies {
			fmt.Fprintf(writer, "  Cycle %d [%s]:\n", i+1, cycle.Severity)
			for _, mod := range cycle.Modules {
				fmt.Fprintf(writer, "    - %s\n", mod)
			}
		}
		fmt.Fprintln(writer)
	}

	// Coupling analysis
	if analysis != nil && analysis.CouplingAnalysis != nil {
		ca := analysis.CouplingAnalysis
		fmt.Fprintln(writer, "Coupling Analysis:")
		fmt.Fprintf(writer, "  Average coupling: %.2f\n", ca.AverageCoupling)
		fmt.Fprintf(writer, "  Average instability: %.2f\n", ca.AverageInstability)
		fmt.Fprintf(writer, "  Highly coupled modules: %d\n", len(ca.HighlyCoupledModules))
		fmt.Fprintf(writer, "  Stable modules: %d\n", len(ca.StableModules))
		fmt.Fprintln(writer)
	}

	// Entry points
	if analysis != nil && len(analysis.RootModules) > 0 {
		fmt.Fprintln(writer, "Entry Points:")
		for _, mod := range analysis.RootModules {
			fmt.Fprintf(writer, "  - %s\n", mod)
		}
		fmt.Fprintln(writer)
	}

	// Warnings
	if len(response.Warnings) > 0 {
		fmt.Fprintln(writer, "Warnings:")
		for _, w := range response.Warnings {
			fmt.Fprintf(writer, "  - %s\n", w)
		}
		fmt.Fprintln(writer)
	}

	// Errors
	if len(response.Errors) > 0 {
		fmt.Fprintln(writer, "Errors:")
		for _, e := range response.Errors {
			fmt.Fprintf(writer, "  - %s\n", e)
		}
		fmt.Fprintln(writer)
	}

	return nil
}

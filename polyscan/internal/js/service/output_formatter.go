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

	"github.com/ludo-technologies/polyscan/polyscan/internal/js/domain"
	"github.com/ludo-technologies/polyscan/polyscan/internal/js/version"
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
	Version     string                                `json:"version"`
	GeneratedAt string                                `json:"generated_at"`
	DurationMs  int64                                 `json:"duration_ms,omitempty"`
	Functions   []domain.FunctionComplexity           `json:"functions"`
	ByDirectory domain.DirectoryComplexityMetricsList `json:"by_directory"`
	Summary     domain.ComplexitySummary              `json:"summary"`
	Warnings    []string                              `json:"warnings,omitempty"`
	Errors      []string                              `json:"errors,omitempty"`
	Config      interface{}                           `json:"config,omitempty"`
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
	Version       string                        `json:"version"`
	GeneratedAt   string                        `json:"generated_at"`
	DurationMs    int64                         `json:"duration_ms"`
	Complexity    *ComplexityResponseJSON       `json:"complexity,omitempty"`
	DeadCode      *DeadCodeResponseJSON         `json:"dead_code,omitempty"`
	Clone         *CloneResponseJSON            `json:"clone,omitempty"`
	CBO           *CBOResponseJSON              `json:"cbo,omitempty"`
	Deps          *DepsResponseJSON             `json:"deps,omitempty"`
	ModuleQuality []domain.ModuleQualityMetrics `json:"module_quality,omitempty"`
	Summary       *domain.AnalyzeSummary        `json:"summary,omitempty"`
}

// newComplexityResponseJSON is the single place the complexity payload is
// shaped, so the standalone and unified outputs cannot drift apart.
func newComplexityResponseJSON(response *domain.ComplexityResponse) *ComplexityResponseJSON {
	return &ComplexityResponseJSON{
		Version:     version.Version,
		GeneratedAt: response.GeneratedAt,
		Functions:   response.Functions,
		ByDirectory: response.ByDirectory,
		Summary:     response.Summary,
		Warnings:    response.Warnings,
		Errors:      response.Errors,
		Config:      response.Config,
	}
}

// newAnalyzeResponseJSON assembles the unified payload shared by the JSON and
// YAML outputs, including the module quality join.
func newAnalyzeResponseJSON(
	results domain.AnalysisResults,
	duration time.Duration,
	now time.Time,
) AnalyzeResponseJSON {
	response := AnalyzeResponseJSON{
		Version:       version.Version,
		GeneratedAt:   now.Format(time.RFC3339),
		DurationMs:    duration.Milliseconds(),
		ModuleQuality: BuildModuleQuality(results.Complexity, results.DeadCode, results.Deps),
		Summary:       BuildAnalyzeSummary(results),
	}

	if results.Complexity != nil {
		response.Complexity = newComplexityResponseJSON(results.Complexity)
	}
	if results.DeadCode != nil {
		response.DeadCode = &DeadCodeResponseJSON{
			Version:     version.Version,
			GeneratedAt: results.DeadCode.GeneratedAt,
			Files:       results.DeadCode.Files,
			Summary:     results.DeadCode.Summary,
			Warnings:    results.DeadCode.Warnings,
			Errors:      results.DeadCode.Errors,
			Config:      results.DeadCode.Config,
		}
	}
	if results.Clone != nil {
		response.Clone = &CloneResponseJSON{
			Version:     version.Version,
			GeneratedAt: now.Format(time.RFC3339),
			DurationMs:  results.Clone.Duration,
			ClonePairs:  results.Clone.ClonePairs,
			CloneGroups: results.Clone.CloneGroups,
			Statistics:  results.Clone.Statistics,
			Success:     results.Clone.Success,
			Error:       results.Clone.Error,
		}
	}
	if results.CBO != nil {
		response.CBO = &CBOResponseJSON{
			Version:     version.Version,
			GeneratedAt: results.CBO.GeneratedAt,
			Classes:     results.CBO.Classes,
			Summary:     results.CBO.Summary,
			Warnings:    results.CBO.Warnings,
			Errors:      results.CBO.Errors,
			Config:      results.CBO.Config,
		}
	}
	if results.Deps != nil {
		response.Deps = &DepsResponseJSON{
			Version:     version.Version,
			GeneratedAt: results.Deps.GeneratedAt,
			Graph:       results.Deps.Graph,
			Analysis:    results.Deps.Analysis,
			Warnings:    results.Deps.Warnings,
			Errors:      results.Deps.Errors,
		}
	}

	return response
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
	results domain.AnalysisResults,
	format domain.OutputFormat,
	writer io.Writer,
	duration time.Duration,
) error {
	switch format {
	case domain.OutputFormatJSON:
		return f.writeAnalyzeJSON(results, writer, duration)
	case domain.OutputFormatText:
		return f.writeAnalyzeText(results, writer, duration)
	case domain.OutputFormatHTML:
		return f.WriteHTML(results, writer, duration)
	case domain.OutputFormatYAML:
		return f.writeAnalyzeYAML(results, writer, duration)
	case domain.OutputFormatCSV:
		return f.writeAnalyzeCSV(results, writer, duration)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

// writeComplexityJSON writes complexity response as JSON
func (f *OutputFormatterImpl) writeComplexityJSON(response *domain.ComplexityResponse, writer io.Writer) error {
	return WriteJSON(writer, newComplexityResponseJSON(response))
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

// BuildAnalyzeSummary builds an AnalyzeSummary from a run's results. The file
// counts come from the run's own accounting rather than from any one analysis,
// so the parse-error penalty charges unparsable files whichever analyses ran.
func BuildAnalyzeSummary(results domain.AnalysisResults) *domain.AnalyzeSummary {
	summary := &domain.AnalyzeSummary{
		TotalFiles:    results.Files.Total,
		AnalyzedFiles: results.Files.Total - results.Files.Skipped,
		SkippedFiles:  results.Files.Skipped,
	}

	if results.Complexity != nil {
		summary.ComplexityEnabled = true
		summary.TotalFunctions = results.Complexity.Summary.TotalFunctions
		summary.FunctionsParsed = results.Complexity.Summary.FunctionsParsed
		summary.AverageComplexity = results.Complexity.Summary.AverageComplexity
		summary.HighComplexityCount = results.Complexity.Summary.HighRiskFunctions
		summary.MediumComplexityCount = results.Complexity.Summary.MediumRiskFunctions
	}

	if results.DeadCode != nil {
		summary.DeadCodeEnabled = true
		summary.DeadCodeCount = results.DeadCode.Summary.TotalFindings
		summary.CriticalDeadCode = results.DeadCode.Summary.CriticalFindings
		summary.WarningDeadCode = results.DeadCode.Summary.WarningFindings
		summary.InfoDeadCode = results.DeadCode.Summary.InfoFindings
	}

	if results.Clone != nil {
		summary.CloneEnabled = true
		if results.Clone.Statistics != nil {
			summary.TotalClones = results.Clone.Statistics.TotalClones
			summary.ClonePairs = results.Clone.Statistics.TotalClonePairs
			summary.CloneGroups = results.Clone.Statistics.TotalCloneGroups
			summary.CodeDuplication = calculateDuplicationPercentage(results.Clone)
			summary.TotalLOC = results.Clone.Statistics.LinesAnalyzed
		}
	}

	if results.CBO != nil {
		summary.CBOEnabled = true
		summary.CBOClasses = results.CBO.Summary.TotalClasses
		summary.HighCouplingClasses = results.CBO.Summary.HighRiskClasses
		summary.MediumCouplingClasses = results.CBO.Summary.MediumRiskClasses
		summary.AverageCoupling = results.CBO.Summary.AverageCBO
	}

	if results.Deps != nil {
		summary.DepsEnabled = true
		if results.Deps.Graph != nil {
			summary.DepsTotalModules = results.Deps.Graph.NodeCount()
		}
		if results.Deps.Analysis != nil {
			if results.Deps.Analysis.CircularDependencies != nil {
				summary.DepsModulesInCycles = results.Deps.Analysis.CircularDependencies.TotalModulesInCycles
			}
			summary.DepsMaxDepth = results.Deps.Analysis.MaxDepth
			if results.Deps.Analysis.CouplingAnalysis != nil {
				summary.DepsMainSequenceDeviation = results.Deps.Analysis.CouplingAnalysis.MainSequenceDeviation
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

// FormatCLISummary formats an AnalyzeSummary as a compact CLI string (pyscn-style).
// skipped names the files no analysis could use; the summary carries their count,
// but a count alone says the report is incomplete without saying what to fix.
func FormatCLISummary(summary *domain.AnalyzeSummary, duration time.Duration, skipped []string) string {
	w := &strings.Builder{}

	fmt.Fprintf(w, "\n\U0001F4CA Analysis Summary:\n")
	fmt.Fprintf(w, "Health Score: %d/100 (Grade: %s)\n", summary.HealthScore, summary.Grade)
	fmt.Fprintf(w, "Project Scale: %s\n", FormatProjectScale(summary))
	if summary.SkippedFiles > 0 {
		fmt.Fprintf(w, "⚠️  %d of %d files skipped (parse errors) - excluded from every score below\n",
			summary.SkippedFiles, summary.TotalFiles)
		for _, entry := range skipped {
			fmt.Fprintf(w, "    %s\n", entry)
		}
	}
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
	results domain.AnalysisResults,
	writer io.Writer,
	duration time.Duration,
) error {
	response := newAnalyzeResponseJSON(results, duration, time.Now())
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

// writeComplexityText writes complexity response as plain text
func (f *OutputFormatterImpl) writeComplexityText(response *domain.ComplexityResponse, writer io.Writer) error {
	fmt.Fprintf(writer, "\n=== Complexity Analysis ===\n\n")
	fmt.Fprintf(writer, "Generated: %s\n", response.GeneratedAt)
	fmt.Fprintf(writer, "Version: %s\n\n", response.Version)

	// Summary
	fmt.Fprintf(writer, "Summary:\n")
	fmt.Fprintf(writer, "  Files analyzed: %d\n", response.Summary.FilesAnalyzed)
	fmt.Fprintf(writer, "  Total functions: %d\n", response.Summary.TotalFunctions)
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

	writeDirectoryComplexityText(writer, response.ByDirectory)

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
	results domain.AnalysisResults,
	writer io.Writer,
	duration time.Duration,
) error {
	fmt.Fprintf(writer, "\n=== polyscan Analysis Report ===\n")
	fmt.Fprintf(writer, "Generated: %s\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(writer, "Duration: %dms\n", duration.Milliseconds())
	fmt.Fprintf(writer, "Version: %s\n\n", version.Version)

	// Complexity results
	if results.Complexity != nil {
		if err := f.writeComplexityText(results.Complexity, writer); err != nil {
			return err
		}
	}

	// Dead code results
	if results.DeadCode != nil {
		if err := f.writeDeadCodeText(results.DeadCode, writer); err != nil {
			return err
		}
	}

	// Clone detection results
	if results.Clone != nil {
		if err := f.writeCloneText(results.Clone, writer); err != nil {
			return err
		}
	}

	// CBO analysis results
	if results.CBO != nil {
		if err := f.writeCBOText(results.CBO, writer); err != nil {
			return err
		}
	}

	// Dependency analysis results
	if results.Deps != nil {
		if err := f.writeDepsText(results.Deps, writer); err != nil {
			return err
		}
	}

	writeModuleQualityText(writer, BuildModuleQuality(results.Complexity, results.DeadCode, results.Deps))

	summary := BuildAnalyzeSummary(results)

	// Write Health Score section
	fmt.Fprintf(writer, "\n=== Health Score ===\n\n")
	fmt.Fprintf(writer, "Overall: %d/100 (Grade: %s)\n", summary.HealthScore, summary.Grade)
	fmt.Fprintf(writer, "Project Scale: %s\n", FormatProjectScale(summary))
	if summary.SkippedFiles > 0 {
		fmt.Fprintf(writer, "%d of %d files skipped (parse errors) - excluded from every score below\n",
			summary.SkippedFiles, summary.TotalFiles)
	}
	fmt.Fprintf(writer, "\n")
	// Only the dimensions that ran are listed: the others are left out of
	// the health score, so a line for them would misread as a clean result.
	fmt.Fprintf(writer, "Category Scores:\n")
	if summary.ComplexityEnabled {
		fmt.Fprintf(writer, "  Complexity:       %3d/100\n", summary.ComplexityScore)
	}
	if summary.DeadCodeEnabled {
		fmt.Fprintf(writer, "  Dead Code:        %3d/100\n", summary.DeadCodeScore)
	}
	if summary.CloneEnabled {
		fmt.Fprintf(writer, "  Code Duplication: %3d/100\n", summary.DuplicationScore)
	}
	if summary.CBOEnabled {
		fmt.Fprintf(writer, "  Coupling:         %3d/100\n", summary.CouplingScore)
	}
	if summary.DepsEnabled {
		fmt.Fprintf(writer, "  Dependencies:     %3d/100\n", summary.DependencyScore)
	}

	return nil
}

// moduleQualityTextLimit caps how many modules the text hotspot list shows.
// The list is ranked worst-first, so the cut only ever hides healthier modules.
const moduleQualityTextLimit = 10

// writeModuleQualityText writes the per-module hotspot list as plain text.
func writeModuleQualityText(writer io.Writer, modules []domain.ModuleQualityMetrics) {
	if len(modules) == 0 {
		return
	}

	fmt.Fprintf(writer, "\n=== Module Quality Hotspots ===\n\n")
	for index, module := range modules {
		if index >= moduleQualityTextLimit {
			break
		}

		label := module.FilePath
		if module.ModuleName != "" {
			label = fmt.Sprintf("%s (%s)", module.ModuleName, module.FilePath)
		}
		fmt.Fprintf(writer, "  %s\n", label)
		fmt.Fprintf(writer, "    Lines: %d, functions analyzed: %d\n", module.LinesOfCode, module.AnalyzedFunctionCount)
		fmt.Fprintf(writer, "    Complexity: avg %.2f, max %d, high-risk %d, handlers %d\n",
			module.AverageComplexity, module.MaxComplexity, module.HighRiskFunctionCount, module.ExceptionHandlerCount)
		fmt.Fprintf(writer, "    Dead code: %d findings, %d blocks\n",
			module.DeadCodeFindingCount, module.DeadCodeBlockCount)
	}
	if len(modules) > moduleQualityTextLimit {
		fmt.Fprintf(writer, "  Showing top %d of %d modules\n", moduleQualityTextLimit, len(modules))
	}
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
	results domain.AnalysisResults,
	writer io.Writer,
	duration time.Duration,
) error {
	response := newAnalyzeResponseJSON(results, duration, time.Now())

	// Write YAML
	encoder := yaml.NewEncoder(writer)
	encoder.SetIndent(2)
	return encoder.Encode(response)
}

// writeAnalyzeCSV writes unified analysis response as CSV
func (f *OutputFormatterImpl) writeAnalyzeCSV(
	results domain.AnalysisResults,
	writer io.Writer,
	duration time.Duration,
) error {
	csvWriter := csv.NewWriter(writer)
	defer csvWriter.Flush()

	needsSeparator := false

	// Write complexity results
	if results.Complexity != nil {
		// Write header
		if err := csvWriter.Write([]string{
			"type", "file", "function", "start_line", "end_line",
			"complexity", "risk_level", "nodes", "edges",
		}); err != nil {
			return err
		}

		// Write function data
		for _, fn := range results.Complexity.Functions {
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

		if len(results.Complexity.ByDirectory) > 0 {
			if err := csvWriter.Write([]string{}); err != nil {
				return err
			}
			if err := csvWriter.Write([]string{
				"type", "directory", "function_count", "average_complexity", "max_complexity",
				"high_risk_functions", "average_nesting_depth", "max_nesting_depth",
			}); err != nil {
				return err
			}

			for _, directory := range results.Complexity.ByDirectory {
				record := []string{
					"directory_complexity",
					directory.DirectoryPath,
					strconv.Itoa(directory.FunctionCount),
					fmt.Sprintf("%.2f", directory.AverageComplexity),
					strconv.Itoa(directory.MaxComplexity),
					strconv.Itoa(directory.HighRiskFunctionCount),
					fmt.Sprintf("%.2f", directory.AverageNestingDepth),
					strconv.Itoa(directory.MaxNestingDepth),
				}
				if err := csvWriter.Write(record); err != nil {
					return err
				}
			}
		}
	}

	// Write dead code results
	if results.DeadCode != nil {
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
		for _, file := range results.DeadCode.Files {
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
	if results.Clone != nil && len(results.Clone.ClonePairs) > 0 {
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

		for _, pair := range results.Clone.ClonePairs {
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
	if results.CBO != nil && len(results.CBO.Classes) > 0 {
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

		for _, class := range results.CBO.Classes {
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
	if results.Deps != nil && results.Deps.Graph != nil {
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

		fromIDs := make([]string, 0, len(results.Deps.Graph.Edges))
		for from := range results.Deps.Graph.Edges {
			fromIDs = append(fromIDs, from)
		}
		sort.Strings(fromIDs)

		for _, from := range fromIDs {
			edges := append([]*domain.DependencyEdge(nil), results.Deps.Graph.Edges[from]...)
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
		needsSeparator = true
	}

	// Write the per-module join last: it restates the analyses above, so a
	// reader who only wants raw findings can stop before it.
	modules := BuildModuleQuality(results.Complexity, results.DeadCode, results.Deps)
	if len(modules) > 0 {
		if needsSeparator {
			if err := csvWriter.Write([]string{}); err != nil {
				return err
			}
		}
		if err := csvWriter.Write([]string{
			"type", "module", "file", "lines_of_code", "analyzed_functions",
			"average_complexity", "max_complexity", "high_risk_functions",
			"exception_handlers", "dead_code_findings", "dead_code_blocks",
		}); err != nil {
			return err
		}

		for _, module := range modules {
			record := []string{
				"module_quality",
				module.ModuleName,
				module.FilePath,
				strconv.Itoa(module.LinesOfCode),
				strconv.Itoa(module.AnalyzedFunctionCount),
				fmt.Sprintf("%.2f", module.AverageComplexity),
				strconv.Itoa(module.MaxComplexity),
				strconv.Itoa(module.HighRiskFunctionCount),
				strconv.Itoa(module.ExceptionHandlerCount),
				strconv.Itoa(module.DeadCodeFindingCount),
				strconv.Itoa(module.DeadCodeBlockCount),
			}
			if err := csvWriter.Write(record); err != nil {
				return err
			}
		}
	}

	return nil
}

package service

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"time"

	"github.com/ludo-technologies/polyscan/jscan/domain"
	"github.com/ludo-technologies/polyscan/jscan/internal/analyzer"
	"github.com/ludo-technologies/polyscan/jscan/internal/config"
	"github.com/ludo-technologies/polyscan/jscan/internal/version"
)

// ComplexityServiceImpl implements the ComplexityService interface
type ComplexityServiceImpl struct {
	config   *config.ComplexityConfig
	progress domain.ProgressManager
}

// NewComplexityService creates a new complexity service implementation
func NewComplexityService(cfg *config.ComplexityConfig) *ComplexityServiceImpl {
	return &ComplexityServiceImpl{
		config: cfg,
	}
}

// NewComplexityServiceWithProgress creates a new complexity service with progress reporting
func NewComplexityServiceWithProgress(cfg *config.ComplexityConfig, pm domain.ProgressManager) *ComplexityServiceImpl {
	return &ComplexityServiceImpl{
		config:   cfg,
		progress: pm,
	}
}

// Analyze performs complexity analysis on multiple files. Each file is read,
// parsed, and analyzed inside the fan-out and released as soon as its results
// are extracted, so peak memory holds only as many parse trees as there are
// workers — use AnalyzeSnapshot when several analyses should share the trees.
func (s *ComplexityServiceImpl) Analyze(ctx context.Context, req domain.ComplexityRequest) (*domain.ComplexityResponse, error) {
	// Set up progress tracking (use no-op if progress manager not set)
	var task domain.TaskProgress = &NoOpTaskProgress{}
	if s.progress != nil {
		task = s.progress.StartTask("Analyzing complexity", len(req.Paths))
	}
	defer task.Complete()

	results := analyzeProjectFilesFromPaths(ctx, req.Paths, task, s.analyzeProjectFile)
	return s.buildResponse(ctx, results, req.Paths, req)
}

// AnalyzeSnapshot performs complexity analysis on already parsed project files.
// The snapshot defines the analyzed file set; req.Paths, when set, must name
// the same files.
func (s *ComplexityServiceImpl) AnalyzeSnapshot(ctx context.Context, snapshot *ProjectSnapshot, req domain.ComplexityRequest) (*domain.ComplexityResponse, error) {
	if err := snapshot.validateRequestPaths(req.Paths); err != nil {
		return nil, err
	}

	results := analyzeFilesConcurrently(ctx, snapshot.Files, nil,
		func(_ context.Context, file *ProjectFile) fileAnalysis[fileComplexity] {
			return s.analyzeProjectFile(file)
		})
	return s.buildResponse(ctx, results, snapshot.Paths(), req)
}

// buildResponse aggregates per-file results, in input order, into the response
// both entry points share. paths must parallel results.
func (s *ComplexityServiceImpl) buildResponse(ctx context.Context, results []fileAnalysis[fileComplexity], paths []string, req domain.ComplexityRequest) (*domain.ComplexityResponse, error) {
	if ctx.Err() != nil {
		return nil, fmt.Errorf("complexity analysis cancelled: %w", ctx.Err())
	}

	var allFunctions []domain.FunctionComplexity
	var warnings []string
	var errors []string
	var analyzedPaths []string
	linesOfCode := make(map[string]int, len(paths))

	for index, result := range results {
		if len(result.errors) > 0 {
			errors = append(errors, result.errors...)
			continue // Skip this file but continue with others
		}

		allFunctions = append(allFunctions, result.value.functions...)
		warnings = append(warnings, result.warnings...)
		filePath := paths[index]
		analyzedPaths = append(analyzedPaths, filePath)
		linesOfCode[filepath.Clean(filePath)] = result.value.linesOfCode
	}

	if len(allFunctions) == 0 {
		return nil, domain.NewAnalysisError("no functions found to analyze", nil)
	}

	// Roll up per module before filtering: the rollups describe the analyzed
	// population, not the subset min/max complexity leaves visible.
	moduleRollups := moduleComplexityRollups(allFunctions, linesOfCode)

	// Filter and sort results
	filteredFunctions, functionsParsed := s.filterFunctions(allFunctions, req)
	sortedFunctions := s.sortFunctions(filteredFunctions, req.SortBy)

	byDirectory, err := aggregateDirectoryComplexity(sortedFunctions, analyzedPaths)
	if err != nil {
		return nil, domain.NewAnalysisError("failed to aggregate directory complexity", err)
	}

	// Generate summary
	summary := s.generateSummary(sortedFunctions, len(analyzedPaths), req, functionsParsed)

	return &domain.ComplexityResponse{
		Functions:     sortedFunctions,
		ByDirectory:   byDirectory,
		Summary:       summary,
		ModuleRollups: moduleRollups,
		Warnings:      warnings,
		Errors:        errors,
		GeneratedAt:   time.Now().Format(time.RFC3339),
		Version:       version.Version,
		Config:        s.buildConfigForResponse(req),
	}, nil
}

// moduleComplexityRollups joins the per-module complexity aggregation with the
// line counts only this service holds. Files that parsed without yielding a
// single function still get an entry, so the module report can show a large
// file that has no functions to blame.
func moduleComplexityRollups(functions []domain.FunctionComplexity, linesOfCode map[string]int) map[string]domain.ModuleComplexityMetrics {
	rollups := domain.AggregateComplexityByModule(functions)
	for path, lines := range linesOfCode {
		metrics := rollups[path]
		metrics.LinesOfCode = lines
		rollups[path] = metrics
	}
	return rollups
}

// aggregateDirectoryComplexity reports the directory rollups relative to the
// deepest directory that contains every analyzed file.
func aggregateDirectoryComplexity(functions []domain.FunctionComplexity, analyzedPaths []string) (domain.DirectoryComplexityMetricsList, error) {
	projectRoot, err := domain.ComplexityDirectoryRoot(analyzedPaths)
	if err != nil {
		return nil, err
	}
	return domain.AggregateComplexityByDirectory(functions, projectRoot)
}

// AnalyzeFile analyzes a single JavaScript/TypeScript file
func (s *ComplexityServiceImpl) AnalyzeFile(ctx context.Context, filePath string, req domain.ComplexityRequest) (*domain.ComplexityResponse, error) {
	// Update the request to analyze only this file
	singleFileReq := req
	singleFileReq.Paths = []string{filePath}

	return s.Analyze(ctx, singleFileReq)
}

// fileComplexity is everything complexity analysis derives from one file: the
// per-function results plus the file-level size the module rollups report.
type fileComplexity struct {
	functions   []domain.FunctionComplexity
	linesOfCode int
}

// countSourceLines counts the physical lines of a source file. The last line
// counts whether or not it ends in a newline, matching pyscn so module line
// counts mean the same thing in both tools' reports.
func countSourceLines(content []byte) int {
	lines := 1
	for _, b := range content {
		if b == '\n' {
			lines++
		}
	}
	return lines
}

// analyzeProjectFile performs complexity analysis on a single parsed file.
func (s *ComplexityServiceImpl) analyzeProjectFile(projectFile *ProjectFile) fileAnalysis[fileComplexity] {
	var file fileComplexity

	filePath := projectFile.Path
	if projectFile.ReadErr != nil {
		return fileAnalysis[fileComplexity]{
			errors: []string{fmt.Sprintf("[%s] Failed to read file: %v", filePath, projectFile.ReadErr)},
		}
	}
	file.linesOfCode = countSourceLines(projectFile.Content)

	if projectFile.ParseErr != nil {
		return fileAnalysis[fileComplexity]{
			value:  file,
			errors: []string{fmt.Sprintf("[%s] Failed to parse: %v", filePath, projectFile.ParseErr)},
		}
	}

	// Build (or reuse) the CFGs for all functions
	cfgs, err := projectFile.CFGs()
	if err != nil {
		return fileAnalysis[fileComplexity]{
			value:  file,
			errors: []string{fmt.Sprintf("[%s] Failed to build CFG: %v", filePath, err)},
		}
	}

	// Analyze complexity for each function
	for funcName, cfg := range cfgs {
		if funcName == domain.ModuleFunctionName {
			continue // Skip main module
		}

		result := analyzer.CalculateComplexityWithConfig(cfg, s.config)

		// Convert to domain model
		funcComplexity := domain.FunctionComplexity{
			Name:        funcName,
			FilePath:    filePath,
			StartLine:   result.StartLine,
			StartColumn: result.StartCol,
			EndLine:     result.EndLine,
			Metrics: domain.ComplexityMetrics{
				Complexity:        result.Complexity,
				Nodes:             result.Nodes,
				Edges:             result.Edges,
				NestingDepth:      result.NestingDepth,
				IfStatements:      result.IfStatements,
				LoopStatements:    result.LoopStatements,
				ExceptionHandlers: result.ExceptionHandlers,
				SwitchCases:       result.SwitchCases,
			},
			RiskLevel: domain.RiskLevel(result.RiskLevel),
		}

		file.functions = append(file.functions, funcComplexity)
	}

	return fileAnalysis[fileComplexity]{value: file}
}

// filterFunctions returns the visible functions plus the count of functions that
// reached the complexity filters. report_unchanged is part of the reporting
// contract rather than a complexity filter, so functions it drops are excluded
// from the parsed count as well.
func (s *ComplexityServiceImpl) filterFunctions(functions []domain.FunctionComplexity, req domain.ComplexityRequest) ([]domain.FunctionComplexity, int) {
	var filtered []domain.FunctionComplexity
	functionsParsed := 0

	for _, fn := range functions {
		// Skip unchanged (complexity = 1) if requested
		if !s.config.ReportUnchanged && fn.Metrics.Complexity == 1 {
			continue
		}

		functionsParsed++

		// Filter by minimum complexity
		if req.MinComplexity > 0 && fn.Metrics.Complexity < req.MinComplexity {
			continue
		}

		// Filter by maximum complexity
		if req.MaxComplexity > 0 && fn.Metrics.Complexity > req.MaxComplexity {
			continue
		}

		filtered = append(filtered, fn)
	}

	return filtered, functionsParsed
}

// sortFunctions sorts functions based on the specified criteria
func (s *ComplexityServiceImpl) sortFunctions(functions []domain.FunctionComplexity, sortBy domain.SortCriteria) []domain.FunctionComplexity {
	sorted := make([]domain.FunctionComplexity, len(functions))
	copy(sorted, functions)

	// Every comparator falls back to source location so that functions the
	// primary criterion cannot separate still come out in the same order on
	// every run.
	switch sortBy {
	case domain.SortByName:
		sort.Slice(sorted, func(i, j int) bool {
			if sorted[i].Name != sorted[j].Name {
				return sorted[i].Name < sorted[j].Name
			}
			return functionPrecedes(sorted[i], sorted[j])
		})
	case domain.SortByRisk:
		riskOrder := map[domain.RiskLevel]int{domain.RiskLevelHigh: 0, domain.RiskLevelMedium: 1, domain.RiskLevelLow: 2}
		sort.Slice(sorted, func(i, j int) bool {
			if riskOrder[sorted[i].RiskLevel] != riskOrder[sorted[j].RiskLevel] {
				return riskOrder[sorted[i].RiskLevel] < riskOrder[sorted[j].RiskLevel]
			}
			return functionPrecedes(sorted[i], sorted[j])
		})
	default:
		// Default and domain.SortByComplexity: complexity descending.
		sort.Slice(sorted, func(i, j int) bool {
			if sorted[i].Metrics.Complexity != sorted[j].Metrics.Complexity {
				return sorted[i].Metrics.Complexity > sorted[j].Metrics.Complexity
			}
			return functionPrecedes(sorted[i], sorted[j])
		})
	}

	return sorted
}

// functionPrecedes orders two functions by where they appear in the project,
// which is unique per function and independent of analysis scheduling.
func functionPrecedes(a, b domain.FunctionComplexity) bool {
	if a.FilePath != b.FilePath {
		return a.FilePath < b.FilePath
	}
	if a.StartLine != b.StartLine {
		return a.StartLine < b.StartLine
	}
	return a.Name < b.Name
}

// generateSummary generates a summary of the complexity analysis.
// functionsParsed is the reportable function count from filterFunctions: functions that
// survived report_unchanged, counted before the min/max complexity filters.
func (s *ComplexityServiceImpl) generateSummary(functions []domain.FunctionComplexity, filesProcessed int, req domain.ComplexityRequest, functionsParsed int) domain.ComplexitySummary {
	summary := domain.ComplexitySummary{
		FilesAnalyzed:   filesProcessed,
		TotalFunctions:  len(functions),
		FunctionsParsed: functionsParsed,
	}

	if len(functions) == 0 {
		return summary
	}

	// Calculate statistics
	totalComplexity := 0
	maxComplexity := 0
	minComplexity := functions[0].Metrics.Complexity

	for _, fn := range functions {
		totalComplexity += fn.Metrics.Complexity

		if fn.Metrics.Complexity > maxComplexity {
			maxComplexity = fn.Metrics.Complexity
		}
		if fn.Metrics.Complexity < minComplexity {
			minComplexity = fn.Metrics.Complexity
		}

		// Count by risk level
		switch fn.RiskLevel {
		case domain.RiskLevelHigh:
			summary.HighRiskFunctions++
		case domain.RiskLevelMedium:
			summary.MediumRiskFunctions++
		case domain.RiskLevelLow:
			summary.LowRiskFunctions++
		}
	}

	summary.AverageComplexity = float64(totalComplexity) / float64(len(functions))
	summary.MaxComplexity = maxComplexity
	summary.MinComplexity = minComplexity

	return summary
}

// buildConfigForResponse builds the configuration section for the response
func (s *ComplexityServiceImpl) buildConfigForResponse(req domain.ComplexityRequest) map[string]interface{} {
	return map[string]interface{}{
		"low_threshold":    s.config.LowThreshold,
		"medium_threshold": s.config.MediumThreshold,
		"max_complexity":   s.config.MaxComplexity,
		"sort_by":          req.SortBy,
		"min_complexity":   req.MinComplexity,
	}
}

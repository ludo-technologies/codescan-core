package service

import (
	"context"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/ludo-technologies/polyscan/jscan/domain"
	"github.com/ludo-technologies/polyscan/jscan/internal/analyzer"
	"github.com/ludo-technologies/polyscan/jscan/internal/config"
	"github.com/ludo-technologies/polyscan/jscan/internal/parser"
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

// Analyze performs complexity analysis on multiple files
func (s *ComplexityServiceImpl) Analyze(ctx context.Context, req domain.ComplexityRequest) (*domain.ComplexityResponse, error) {
	var allFunctions []domain.FunctionComplexity
	var warnings []string
	var errors []string
	filesProcessed := 0

	// Set up progress tracking (use no-op if progress manager not set)
	var task domain.TaskProgress = &NoOpTaskProgress{}
	if s.progress != nil {
		task = s.progress.StartTask("Analyzing complexity", len(req.Paths))
	}
	defer task.Complete()

	for _, filePath := range req.Paths {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("complexity analysis cancelled: %w", ctx.Err())
		default:
		}

		// Analyze single file
		functions, fileWarnings, fileErrors := s.analyzeFile(ctx, filePath, req)

		if len(fileErrors) > 0 {
			errors = append(errors, fileErrors...)
			task.Increment(1)
			continue // Skip this file but continue with others
		}

		allFunctions = append(allFunctions, functions...)
		warnings = append(warnings, fileWarnings...)
		filesProcessed++
		task.Increment(1)
	}

	if len(allFunctions) == 0 {
		return nil, domain.NewAnalysisError("no functions found to analyze", nil)
	}

	// Filter and sort results
	functionsParsed := len(allFunctions)
	filteredFunctions := s.filterFunctions(allFunctions, req)
	sortedFunctions := s.sortFunctions(filteredFunctions, req.SortBy)

	// Generate summary
	summary := s.generateSummary(sortedFunctions, filesProcessed, req, functionsParsed)

	return &domain.ComplexityResponse{
		Functions:   sortedFunctions,
		Summary:     summary,
		Warnings:    warnings,
		Errors:      errors,
		GeneratedAt: time.Now().Format(time.RFC3339),
		Version:     version.Version,
		Config:      s.buildConfigForResponse(req),
	}, nil
}

// AnalyzeFile analyzes a single JavaScript/TypeScript file
func (s *ComplexityServiceImpl) AnalyzeFile(ctx context.Context, filePath string, req domain.ComplexityRequest) (*domain.ComplexityResponse, error) {
	// Update the request to analyze only this file
	singleFileReq := req
	singleFileReq.Paths = []string{filePath}

	return s.Analyze(ctx, singleFileReq)
}

// analyzeFile performs complexity analysis on a single file
func (s *ComplexityServiceImpl) analyzeFile(ctx context.Context, filePath string, req domain.ComplexityRequest) ([]domain.FunctionComplexity, []string, []string) {
	var functions []domain.FunctionComplexity
	var warnings []string
	var errors []string

	// Parse the file
	content, err := s.readFile(filePath)
	if err != nil {
		errors = append(errors, fmt.Sprintf("[%s] Failed to read file: %v", filePath, err))
		return functions, warnings, errors
	}

	// Parse JavaScript/TypeScript
	ast, err := parser.ParseForLanguage(filePath, content)
	if err != nil {
		errors = append(errors, fmt.Sprintf("[%s] Failed to parse: %v", filePath, err))
		return functions, warnings, errors
	}

	// Build CFGs for all functions
	builder := analyzer.NewCFGBuilder()
	cfgs, err := builder.BuildAll(ast)
	if err != nil {
		errors = append(errors, fmt.Sprintf("[%s] Failed to build CFG: %v", filePath, err))
		return functions, warnings, errors
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
			},
			RiskLevel: domain.RiskLevel(result.RiskLevel),
		}

		functions = append(functions, funcComplexity)
	}

	return functions, warnings, errors
}

// filterFunctions filters functions based on request criteria
func (s *ComplexityServiceImpl) filterFunctions(functions []domain.FunctionComplexity, req domain.ComplexityRequest) []domain.FunctionComplexity {
	var filtered []domain.FunctionComplexity

	for _, fn := range functions {
		// Filter by minimum complexity
		if req.MinComplexity > 0 && fn.Metrics.Complexity < req.MinComplexity {
			continue
		}

		// Filter by maximum complexity
		if req.MaxComplexity > 0 && fn.Metrics.Complexity > req.MaxComplexity {
			continue
		}

		// Skip unchanged (complexity = 1) if requested
		if !s.config.ReportUnchanged && fn.Metrics.Complexity == 1 {
			continue
		}

		filtered = append(filtered, fn)
	}

	return filtered
}

// sortFunctions sorts functions based on the specified criteria
func (s *ComplexityServiceImpl) sortFunctions(functions []domain.FunctionComplexity, sortBy domain.SortCriteria) []domain.FunctionComplexity {
	sorted := make([]domain.FunctionComplexity, len(functions))
	copy(sorted, functions)

	switch sortBy {
	case domain.SortByComplexity:
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].Metrics.Complexity > sorted[j].Metrics.Complexity
		})
	case domain.SortByName:
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].Name < sorted[j].Name
		})
	case domain.SortByRisk:
		riskOrder := map[domain.RiskLevel]int{domain.RiskLevelHigh: 0, domain.RiskLevelMedium: 1, domain.RiskLevelLow: 2}
		sort.Slice(sorted, func(i, j int) bool {
			return riskOrder[sorted[i].RiskLevel] < riskOrder[sorted[j].RiskLevel]
		})
	default:
		// Default: sort by complexity descending
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].Metrics.Complexity > sorted[j].Metrics.Complexity
		})
	}

	return sorted
}

// generateSummary generates a summary of the complexity analysis.
// functionsParsed is the pre-filter function count (all functions parsed before min_complexity filtering).
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

// readFile reads the contents of a file
func (s *ComplexityServiceImpl) readFile(filePath string) ([]byte, error) {
	return os.ReadFile(filePath)
}

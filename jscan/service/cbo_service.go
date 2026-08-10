package service

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/ludo-technologies/polyscan/jscan/domain"
	"github.com/ludo-technologies/polyscan/jscan/internal/analyzer"
	"github.com/ludo-technologies/polyscan/jscan/internal/version"
)

// CBOServiceImpl implements the CBOService interface
type CBOServiceImpl struct {
	config *analyzer.CBOAnalyzerConfig
}

// NewCBOService creates a new CBO service implementation
func NewCBOService(lowThreshold, mediumThreshold int, includeBuiltins, includeTypeImports bool) *CBOServiceImpl {
	return &CBOServiceImpl{
		config: &analyzer.CBOAnalyzerConfig{
			IncludeBuiltins:    includeBuiltins,
			IncludeTypeImports: includeTypeImports,
			LowThreshold:       lowThreshold,
			MediumThreshold:    mediumThreshold,
		},
	}
}

// NewCBOServiceWithDefaults creates a new CBO service with default configuration
func NewCBOServiceWithDefaults() *CBOServiceImpl {
	return &CBOServiceImpl{
		config: analyzer.DefaultCBOAnalyzerConfig(),
	}
}

// Analyze performs CBO analysis on multiple files. Each file is read, parsed,
// and analyzed inside the fan-out and released as soon as its coupling is
// extracted — use AnalyzeSnapshot when several analyses should share the
// parse trees.
func (s *CBOServiceImpl) Analyze(ctx context.Context, req domain.CBORequest) (*domain.CBOResponse, error) {
	config := s.effectiveConfig(req)
	cboAnalyzer := analyzer.NewCBOAnalyzer(&config)

	results := analyzeProjectFilesFromPaths(ctx, req.Paths, nil,
		func(file *ProjectFile) fileAnalysis[*domain.ClassCoupling] {
			return s.analyzeProjectFile(cboAnalyzer, file)
		})
	return s.buildResponse(ctx, results, &config, req)
}

// AnalyzeSnapshot performs CBO analysis on already parsed project files. The
// snapshot defines the analyzed file set; req.Paths, when set, must name the
// same files.
func (s *CBOServiceImpl) AnalyzeSnapshot(ctx context.Context, snapshot *ProjectSnapshot, req domain.CBORequest) (*domain.CBOResponse, error) {
	if err := snapshot.validateRequestPaths(req.Paths); err != nil {
		return nil, err
	}

	config := s.effectiveConfig(req)
	cboAnalyzer := analyzer.NewCBOAnalyzer(&config)

	results := analyzeFilesConcurrently(ctx, snapshot.Files, nil,
		func(_ context.Context, file *ProjectFile) fileAnalysis[*domain.ClassCoupling] {
			return s.analyzeProjectFile(cboAnalyzer, file)
		})
	return s.buildResponse(ctx, results, &config, req)
}

// effectiveConfig applies request thresholds on top of the service config.
func (s *CBOServiceImpl) effectiveConfig(req domain.CBORequest) analyzer.CBOAnalyzerConfig {
	config := *s.config
	if req.LowThreshold > 0 {
		config.LowThreshold = req.LowThreshold
	}
	if req.MediumThreshold > 0 {
		config.MediumThreshold = req.MediumThreshold
	}
	if req.IncludeBuiltins != nil {
		config.IncludeBuiltins = *req.IncludeBuiltins
	}
	return config
}

// buildResponse aggregates per-file couplings, in input order, into the
// response both entry points share.
func (s *CBOServiceImpl) buildResponse(ctx context.Context, results []fileAnalysis[*domain.ClassCoupling], config *analyzer.CBOAnalyzerConfig, req domain.CBORequest) (*domain.CBOResponse, error) {
	if ctx.Err() != nil {
		return nil, fmt.Errorf("CBO analysis cancelled: %w", ctx.Err())
	}

	var allClasses []domain.ClassCoupling
	var warnings []string
	var errors []string
	filesProcessed := 0

	for _, result := range results {
		if len(result.errors) > 0 {
			errors = append(errors, result.errors...)
			continue
		}

		if result.value != nil {
			allClasses = append(allClasses, *result.value)
		}
		warnings = append(warnings, result.warnings...)
		filesProcessed++
	}

	if len(allClasses) == 0 && len(errors) > 0 {
		return nil, domain.NewAnalysisError("failed to analyze any files", nil)
	}

	// Filter and sort results
	filteredClasses := s.filterClasses(allClasses, req)
	sortedClasses := s.sortClasses(filteredClasses, req.SortBy)

	// Generate summary
	summary := s.generateSummary(sortedClasses, filesProcessed, req)

	return &domain.CBOResponse{
		Classes:     sortedClasses,
		Summary:     summary,
		Warnings:    warnings,
		Errors:      errors,
		GeneratedAt: time.Now().Format(time.RFC3339),
		Version:     version.Version,
		Config:      s.buildConfigForResponse(config, req),
	}, nil
}

// AnalyzeFile analyzes a single JavaScript/TypeScript file
func (s *CBOServiceImpl) AnalyzeFile(ctx context.Context, filePath string, req domain.CBORequest) (*domain.CBOResponse, error) {
	singleFileReq := req
	singleFileReq.Paths = []string{filePath}
	return s.Analyze(ctx, singleFileReq)
}

// analyzeProjectFile performs CBO analysis on a single parsed file
func (s *CBOServiceImpl) analyzeProjectFile(cboAnalyzer *analyzer.CBOAnalyzer, file *ProjectFile) fileAnalysis[*domain.ClassCoupling] {
	filePath := file.Path
	if file.ReadErr != nil {
		return fileAnalysis[*domain.ClassCoupling]{
			errors: []string{fmt.Sprintf("[%s] Failed to read file: %v", filePath, file.ReadErr)},
		}
	}

	if file.ParseErr != nil {
		return fileAnalysis[*domain.ClassCoupling]{
			errors: []string{fmt.Sprintf("[%s] Failed to parse: %v", filePath, file.ParseErr)},
		}
	}

	// Analyze CBO
	classCoupling, err := cboAnalyzer.AnalyzeFile(file.AST, filePath)
	if err != nil {
		return fileAnalysis[*domain.ClassCoupling]{
			errors: []string{fmt.Sprintf("[%s] Failed to analyze CBO: %v", filePath, err)},
		}
	}

	return fileAnalysis[*domain.ClassCoupling]{value: classCoupling}
}

// filterClasses filters classes based on request criteria
func (s *CBOServiceImpl) filterClasses(classes []domain.ClassCoupling, req domain.CBORequest) []domain.ClassCoupling {
	var filtered []domain.ClassCoupling

	for _, class := range classes {
		// Filter by minimum CBO
		if req.MinCBO > 0 && class.Metrics.CouplingCount < req.MinCBO {
			continue
		}

		// Filter by maximum CBO
		if req.MaxCBO > 0 && class.Metrics.CouplingCount > req.MaxCBO {
			continue
		}

		// Skip zeros if requested
		if req.ShowZeros != nil && !*req.ShowZeros && class.Metrics.CouplingCount == 0 {
			continue
		}

		filtered = append(filtered, class)
	}

	return filtered
}

// sortClasses sorts classes based on the specified criteria
func (s *CBOServiceImpl) sortClasses(classes []domain.ClassCoupling, sortBy domain.SortCriteria) []domain.ClassCoupling {
	sorted := make([]domain.ClassCoupling, len(classes))
	copy(sorted, classes)

	switch sortBy {
	case domain.SortByCoupling:
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].Metrics.CouplingCount > sorted[j].Metrics.CouplingCount
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
		// Default: sort by coupling descending
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].Metrics.CouplingCount > sorted[j].Metrics.CouplingCount
		})
	}

	return sorted
}

// generateSummary generates a summary of the CBO analysis
func (s *CBOServiceImpl) generateSummary(classes []domain.ClassCoupling, filesProcessed int, req domain.CBORequest) domain.CBOSummary {
	summary := domain.CBOSummary{
		TotalClasses:    len(classes),
		ClassesAnalyzed: len(classes),
		FilesAnalyzed:   filesProcessed,
		CBODistribution: make(map[string]int),
	}

	if len(classes) == 0 {
		return summary
	}

	// Calculate statistics
	totalCBO := 0
	maxCBO := 0
	minCBO := classes[0].Metrics.CouplingCount

	for _, class := range classes {
		cbo := class.Metrics.CouplingCount
		totalCBO += cbo

		if cbo > maxCBO {
			maxCBO = cbo
		}
		if cbo < minCBO {
			minCBO = cbo
		}

		// Count by risk level
		switch class.RiskLevel {
		case domain.RiskLevelHigh:
			summary.HighRiskClasses++
		case domain.RiskLevelMedium:
			summary.MediumRiskClasses++
		case domain.RiskLevelLow:
			summary.LowRiskClasses++
		}

		// Build distribution
		rangeKey := s.getCBORange(cbo)
		summary.CBODistribution[rangeKey]++
	}

	summary.AverageCBO = float64(totalCBO) / float64(len(classes))
	summary.MaxCBO = maxCBO
	summary.MinCBO = minCBO

	// Get most coupled classes (top 10)
	sortedByCoupling := make([]domain.ClassCoupling, len(classes))
	copy(sortedByCoupling, classes)
	sort.Slice(sortedByCoupling, func(i, j int) bool {
		return sortedByCoupling[i].Metrics.CouplingCount > sortedByCoupling[j].Metrics.CouplingCount
	})

	maxMostCoupled := min(10, len(sortedByCoupling))
	summary.MostCoupledClasses = sortedByCoupling[:maxMostCoupled]

	return summary
}

// getCBORange returns a string representing the CBO range for distribution
func (s *CBOServiceImpl) getCBORange(cbo int) string {
	switch {
	case cbo == 0:
		return "0"
	case cbo <= 3:
		return "1-3"
	case cbo <= 7:
		return "4-7"
	case cbo <= 10:
		return "8-10"
	default:
		return "10+"
	}
}

// buildConfigForResponse builds the configuration section for the response
func (s *CBOServiceImpl) buildConfigForResponse(config *analyzer.CBOAnalyzerConfig, req domain.CBORequest) map[string]any {
	return map[string]any{
		"low_threshold":        config.LowThreshold,
		"medium_threshold":     config.MediumThreshold,
		"include_builtins":     config.IncludeBuiltins,
		"include_type_imports": config.IncludeTypeImports,
		"sort_by":              req.SortBy,
		"min_cbo":              req.MinCBO,
		"max_cbo":              req.MaxCBO,
	}
}

package service

import (
	"context"
	"fmt"
	"os"
	"sort"

	"github.com/ludo-technologies/polyscan/polyscan/internal/js/analyzer"
	"github.com/ludo-technologies/polyscan/polyscan/internal/js/domain"
	"github.com/ludo-technologies/polyscan/polyscan/internal/js/parser"
)

// DeadCodeServiceImpl implements the DeadCodeService interface
type DeadCodeServiceImpl struct{}

// NewDeadCodeService creates a new dead code service implementation
func NewDeadCodeService() *DeadCodeServiceImpl {
	return &DeadCodeServiceImpl{}
}

// Analyze performs dead code analysis on multiple files
func (s *DeadCodeServiceImpl) Analyze(ctx context.Context, req domain.DeadCodeRequest) (*domain.DeadCodeResponse, error) {
	return AnalyzeDeadCode(ctx, req)
}

// AnalyzeFile analyzes a single JavaScript/TypeScript file for dead code
func (s *DeadCodeServiceImpl) AnalyzeFile(ctx context.Context, filePath string, req domain.DeadCodeRequest) (*domain.FileDeadCode, error) {
	fileResult, _, fileErrors := s.analyzeFile(ctx, filePath, req)

	if len(fileErrors) > 0 {
		return nil, domain.NewAnalysisError(fmt.Sprintf("failed to analyze file %s", filePath), fmt.Errorf("%v", fileErrors))
	}

	return fileResult, nil
}

// AnalyzeFunction analyzes a single function for dead code
func (s *DeadCodeServiceImpl) AnalyzeFunction(ctx context.Context, functionCFG interface{}, req domain.DeadCodeRequest) (*domain.FunctionDeadCode, error) {
	cfg, ok := functionCFG.(*analyzer.CFG)
	if !ok {
		return nil, domain.NewInvalidInputError("invalid CFG type", nil)
	}

	// Create detector and detect dead code
	detector := analyzer.NewDeadCodeDetector(cfg)
	result := detector.Detect()
	if result == nil {
		return nil, domain.NewAnalysisError("failed to analyze function", nil)
	}

	funcResult := s.convertToFunctionDeadCode(result, "unknown", req)
	return &funcResult, nil
}

// analyzeFile performs dead code analysis on a single file
func (s *DeadCodeServiceImpl) analyzeFile(ctx context.Context, filePath string, req domain.DeadCodeRequest) (*domain.FileDeadCode, []string, []string) {
	var warnings []string
	var errors []string

	// Parse the file
	content, err := s.readFile(filePath)
	if err != nil {
		errors = append(errors, fmt.Sprintf("[%s] Failed to read file: %v", filePath, err))
		return nil, warnings, errors
	}

	ast, err := parser.ParseForLanguage(filePath, content)
	if err != nil {
		errors = append(errors, fmt.Sprintf("[%s] Parse error: %v", filePath, err))
		return nil, warnings, errors
	}

	// Build CFGs for all functions
	builder := analyzer.NewCFGBuilder()
	cfgs, err := builder.BuildAll(ast)
	if err != nil {
		errors = append(errors, fmt.Sprintf("[%s] CFG construction failed: %v", filePath, err))
		return nil, warnings, errors
	}

	if len(cfgs) == 0 {
		warnings = append(warnings, fmt.Sprintf("[%s] No functions found in file", filePath))
		return &domain.FileDeadCode{
			FilePath:          filePath,
			Functions:         []domain.FunctionDeadCode{},
			TotalFindings:     0,
			TotalFunctions:    0,
			AffectedFunctions: 0,
			DeadCodeRatio:     0.0,
		}, warnings, errors
	}

	// Analyze dead code for each function
	var functions []domain.FunctionDeadCode
	totalFindings := 0
	affectedFunctions := 0

	for functionName, cfg := range cfgs {
		// Skip module-level code
		if functionName == domain.ModuleFunctionName {
			continue
		}

		// Create detector and detect dead code
		detector := analyzer.NewDeadCodeDetectorWithFilePath(cfg, filePath)
		result := detector.Detect()
		if result == nil {
			continue
		}

		funcResult := s.convertToFunctionDeadCode(result, functionName, req)

		// Only include functions with findings if filtering by severity
		if len(funcResult.Findings) > 0 {
			functions = append(functions, funcResult)
			affectedFunctions++
			totalFindings += len(funcResult.Findings)
		}
	}

	return &domain.FileDeadCode{
		FilePath:          filePath,
		Functions:         functions,
		TotalFindings:     totalFindings,
		TotalFunctions:    len(cfgs),
		AffectedFunctions: affectedFunctions,
		DeadCodeRatio:     float64(affectedFunctions) / float64(len(cfgs)),
	}, warnings, errors
}

// convertToFunctionDeadCode converts internal dead code result to domain model
func (s *DeadCodeServiceImpl) convertToFunctionDeadCode(result *analyzer.DeadCodeResult, functionName string, req domain.DeadCodeRequest) domain.FunctionDeadCode {
	var findings []domain.DeadCodeFinding

	for _, finding := range result.Findings {
		severity := domain.DeadCodeSeverity(finding.Severity)

		// Apply severity filter
		if !severity.IsAtLeast(req.MinSeverity) {
			continue
		}

		f := domain.DeadCodeFinding{
			Location: domain.DeadCodeLocation{
				FilePath:    result.FilePath,
				StartLine:   finding.StartLine,
				EndLine:     finding.EndLine,
				StartColumn: 0, // Not available in analyzer
				EndColumn:   0, // Not available in analyzer
			},
			FunctionName: functionName,
			Code:         finding.Code,
			Reason:       string(finding.Reason),
			Severity:     severity,
			Description:  finding.Description,
			BlockID:      finding.BlockID,
		}

		// TODO: Add context if req.ShowContext && req.ContextLines > 0

		findings = append(findings, f)
	}

	funcDeadCode := domain.FunctionDeadCode{
		Name:           functionName,
		FilePath:       result.FilePath,
		Findings:       findings,
		TotalBlocks:    result.TotalBlocks,
		DeadBlocks:     result.DeadBlocks,
		ReachableRatio: result.ReachableRatio,
	}

	funcDeadCode.CalculateSeverityCounts()
	return funcDeadCode
}

// filterFiles filters files based on request criteria
func (s *DeadCodeServiceImpl) filterFiles(files []domain.FileDeadCode, req domain.DeadCodeRequest) []domain.FileDeadCode {
	var filtered []domain.FileDeadCode

	for _, file := range files {
		// Filter functions within file
		var filteredFunctions []domain.FunctionDeadCode
		for _, fn := range file.Functions {
			// Check if function has findings at required severity
			if fn.HasFindingsAtSeverity(req.MinSeverity) {
				filteredFunctions = append(filteredFunctions, fn)
			}
		}

		if len(filteredFunctions) > 0 {
			file.Functions = filteredFunctions
			file.TotalFindings = 0
			for _, fn := range filteredFunctions {
				file.TotalFindings += len(fn.Findings)
			}
			file.AffectedFunctions = len(filteredFunctions)
			filtered = append(filtered, file)
		}
	}

	return filtered
}

// sortFiles sorts files based on the specified criteria
func (s *DeadCodeServiceImpl) sortFiles(files []domain.FileDeadCode, sortBy domain.DeadCodeSortCriteria) []domain.FileDeadCode {
	sorted := make([]domain.FileDeadCode, len(files))
	copy(sorted, files)

	switch sortBy {
	case domain.DeadCodeSortBySeverity:
		sort.Slice(sorted, func(i, j int) bool {
			iMax := s.getMaxSeverity(sorted[i])
			jMax := s.getMaxSeverity(sorted[j])
			return iMax > jMax
		})
	case domain.DeadCodeSortByLine:
		sort.Slice(sorted, func(i, j int) bool {
			iLine := s.getFirstLine(sorted[i])
			jLine := s.getFirstLine(sorted[j])
			return iLine < jLine
		})
	case domain.DeadCodeSortByFile:
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].FilePath < sorted[j].FilePath
		})
	case domain.DeadCodeSortByFunction:
		sort.Slice(sorted, func(i, j int) bool {
			iFunc := s.getFirstFunction(sorted[i])
			jFunc := s.getFirstFunction(sorted[j])
			return iFunc < jFunc
		})
	default:
		// Default: sort by severity
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].TotalFindings > sorted[j].TotalFindings
		})
	}

	return sorted
}

// getMaxSeverity returns the maximum severity level in a file
func (s *DeadCodeServiceImpl) getMaxSeverity(file domain.FileDeadCode) int {
	maxSeverity := 0
	for _, fn := range file.Functions {
		for _, finding := range fn.Findings {
			level := finding.Severity.Level()
			if level > maxSeverity {
				maxSeverity = level
			}
		}
	}
	return maxSeverity
}

// getFirstLine returns the first line number in a file's findings
func (s *DeadCodeServiceImpl) getFirstLine(file domain.FileDeadCode) int {
	if len(file.Functions) == 0 {
		return 0
	}
	if len(file.Functions[0].Findings) == 0 {
		return 0
	}
	return file.Functions[0].Findings[0].Location.StartLine
}

// getFirstFunction returns the name of the first function in a file
func (s *DeadCodeServiceImpl) getFirstFunction(file domain.FileDeadCode) string {
	if len(file.Functions) == 0 {
		return ""
	}
	return file.Functions[0].Name
}

// generateSummary generates a summary of the dead code analysis
func (s *DeadCodeServiceImpl) generateSummary(files []domain.FileDeadCode, filesProcessed int, req domain.DeadCodeRequest) domain.DeadCodeSummary {
	summary := domain.DeadCodeSummary{
		TotalFiles:        filesProcessed,
		FilesWithDeadCode: len(files),
		FindingsByReason:  make(map[string]int),
	}

	for _, file := range files {
		summary.TotalFunctions += file.TotalFunctions
		summary.FunctionsWithDeadCode += file.AffectedFunctions
		summary.TotalFindings += file.TotalFindings
		summary.TotalBlocks += s.getTotalBlocks(file)
		summary.DeadBlocks += s.getDeadBlocks(file)

		for _, fn := range file.Functions {
			summary.CriticalFindings += fn.CriticalCount
			summary.WarningFindings += fn.WarningCount
			summary.InfoFindings += fn.InfoCount

			for _, finding := range fn.Findings {
				summary.FindingsByReason[finding.Reason]++
			}
		}
	}

	if summary.TotalBlocks > 0 {
		summary.OverallDeadRatio = float64(summary.DeadBlocks) / float64(summary.TotalBlocks)
	}

	return summary
}

// getTotalBlocks returns the total number of blocks in a file
func (s *DeadCodeServiceImpl) getTotalBlocks(file domain.FileDeadCode) int {
	total := 0
	for _, fn := range file.Functions {
		total += fn.TotalBlocks
	}
	return total
}

// getDeadBlocks returns the total number of dead blocks in a file
func (s *DeadCodeServiceImpl) getDeadBlocks(file domain.FileDeadCode) int {
	total := 0
	for _, fn := range file.Functions {
		total += fn.DeadBlocks
	}
	return total
}

// buildConfigForResponse builds the configuration section for the response
func (s *DeadCodeServiceImpl) buildConfigForResponse(req domain.DeadCodeRequest) map[string]interface{} {
	return map[string]interface{}{
		"min_severity":    req.MinSeverity,
		"sort_by":         req.SortBy,
		"show_context":    domain.BoolValue(req.ShowContext, false),
		"context_lines":   req.ContextLines,
		"detect_return":   domain.BoolValue(req.DetectAfterReturn, true),
		"detect_break":    domain.BoolValue(req.DetectAfterBreak, true),
		"detect_continue": domain.BoolValue(req.DetectAfterContinue, true),
		"detect_throw":    domain.BoolValue(req.DetectAfterThrow, true),
	}
}

// readFile reads the contents of a file
func (s *DeadCodeServiceImpl) readFile(filePath string) ([]byte, error) {
	return os.ReadFile(filePath)
}

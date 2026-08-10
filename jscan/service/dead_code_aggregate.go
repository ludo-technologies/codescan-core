package service

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"time"

	"github.com/ludo-technologies/polyscan/jscan/domain"
	"github.com/ludo-technologies/polyscan/jscan/internal/analyzer"
	"github.com/ludo-technologies/polyscan/jscan/internal/version"
)

// scannedFile is everything dead code analysis derives from one file on its
// own, before findings are aggregated across the project.
type scannedFile struct {
	deadCode      map[string]*analyzer.DeadCodeResult
	moduleInfo    *domain.ModuleInfo
	unusedImports []*analyzer.DeadCodeFinding
}

// scanFileForDeadCode analyzes one already parsed file. The module analyzer
// only reads its configuration, so several files can be scanned at once.
func scanFileForDeadCode(moduleAnalyzer *analyzer.ModuleAnalyzer, file *ProjectFile) fileAnalysis[*scannedFile] {
	filePath := file.Path
	if file.ReadErr != nil {
		return fileAnalysis[*scannedFile]{
			errors: []string{fmt.Sprintf("[%s] failed to read file: %v", filePath, file.ReadErr)},
		}
	}

	if file.ParseErr != nil {
		return fileAnalysis[*scannedFile]{
			errors: []string{fmt.Sprintf("[%s] failed to parse file: %v", filePath, file.ParseErr)},
		}
	}
	ast := file.AST

	cfgs, err := file.CFGs()
	if err != nil {
		return fileAnalysis[*scannedFile]{
			errors: []string{fmt.Sprintf("[%s] failed to build CFG: %v", filePath, err)},
		}
	}

	scan := fileAnalysis[*scannedFile]{
		value: &scannedFile{deadCode: analyzer.DetectAll(cfgs, filePath)},
	}
	if len(cfgs) == 0 {
		scan.warnings = append(scan.warnings, fmt.Sprintf("[%s] no functions found in file", filePath))
	}

	moduleInfo, moduleErr := moduleAnalyzer.AnalyzeFile(ast, filePath)
	if moduleErr != nil {
		scan.warnings = append(scan.warnings, fmt.Sprintf("[%s] module analysis warning: %v", filePath, moduleErr))
	} else {
		// Only a cleanly analyzed module joins the project-wide import graph.
		scan.value.moduleInfo = moduleInfo
	}
	if moduleInfo != nil {
		scan.value.unusedImports = analyzer.DetectUnusedImports(ast, moduleInfo, filePath)
	}

	return scan
}

// moduleDeadCodeRollup accumulates the per-module dead-code counts the unified
// analyze report joins with complexity. It is fed from the detector output
// rather than from the response, so raising min_severity hides findings from
// the report without shrinking a module's measured dead-code weight.
type moduleDeadCodeRollup map[string]domain.ModuleDeadCodeMetrics

func newModuleDeadCodeRollup() moduleDeadCodeRollup {
	return make(moduleDeadCodeRollup)
}

func (r moduleDeadCodeRollup) add(filePath string, findings, blocks int) {
	key := filepath.Clean(filePath)
	metrics := r[key]
	metrics.DeadCodeFindingCount += findings
	metrics.DeadCodeBlockCount += blocks
	r[key] = metrics
}

// AnalyzeDeadCode runs dead code analysis using the shared aggregation path.
func AnalyzeDeadCode(ctx context.Context, req domain.DeadCodeRequest) (*domain.DeadCodeResponse, error) {
	return AnalyzeDeadCodeWithTask(ctx, req, nil)
}

// AnalyzeDeadCodeWithTask runs dead code analysis with optional progress reporting.
func AnalyzeDeadCodeWithTask(ctx context.Context, req domain.DeadCodeRequest, task domain.TaskProgress) (*domain.DeadCodeResponse, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	snapshot := BuildProjectSnapshot(ctx, req.Paths, task)
	return AnalyzeDeadCodeSnapshot(ctx, snapshot, req)
}

// AnalyzeDeadCodeSnapshot runs dead code analysis on already parsed project files.
func AnalyzeDeadCodeSnapshot(ctx context.Context, snapshot *ProjectSnapshot, req domain.DeadCodeRequest) (*domain.DeadCodeResponse, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if snapshot == nil {
		return nil, domain.NewInvalidInputError("project snapshot cannot be nil", nil)
	}

	minSeverity := req.MinSeverity
	if minSeverity == "" {
		minSeverity = domain.DeadCodeSeverityInfo
	}
	sortBy := req.SortBy
	if sortBy == "" {
		sortBy = domain.DeadCodeSortBySeverity
	}

	var files []domain.FileDeadCode
	fileIndexMap := make(map[string]int)
	type fileMetrics struct {
		totalFunctions    int
		affectedFunctions int
		deadCodeRatio     float64
	}
	fileMetricsByPath := make(map[string]fileMetrics)
	var warnings []string
	var errors []string

	var totalFindings, criticalFindings, warningFindings, infoFindings int
	var totalFunctions, functionsWithDeadCode int
	var totalBlocks, deadBlocks int

	moduleAnalyzer := analyzer.NewModuleAnalyzer(nil)
	allModuleInfos := make(map[string]*domain.ModuleInfo)
	analyzedFiles := make(map[string]bool)
	unusedFuncDedup := make(map[string]map[int]bool) // filePath -> startLine -> true
	// The module rollups count what the detectors produced, so they need their
	// own dedup of the same locations: unusedFuncDedup only remembers findings
	// that passed the severity filter, which is the right rule for the report
	// (a hidden finding must not suppress the one remaining report of that
	// location) but would let a dropped finding be counted twice here.
	rollup := newModuleDeadCodeRollup()
	rollupFuncDedup := make(map[string]map[int]bool)

	addFileLevelFinding := func(f domain.DeadCodeFinding) {
		if !f.Severity.IsAtLeast(minSeverity) {
			return
		}

		filePath := f.Location.FilePath
		if idx, ok := fileIndexMap[filePath]; ok {
			files[idx].FileLevelFindings = append(files[idx].FileLevelFindings, f)
			files[idx].TotalFindings++
		} else {
			metrics := fileMetricsByPath[filePath]
			entry := domain.FileDeadCode{
				FilePath:          filePath,
				FileLevelFindings: []domain.DeadCodeFinding{f},
				TotalFindings:     1,
				TotalFunctions:    metrics.totalFunctions,
				AffectedFunctions: metrics.affectedFunctions,
				DeadCodeRatio:     metrics.deadCodeRatio,
			}
			fileIndexMap[filePath] = len(files)
			files = append(files, entry)
		}

		switch f.Severity {
		case domain.DeadCodeSeverityCritical:
			criticalFindings++
		case domain.DeadCodeSeverityWarning:
			warningFindings++
		case domain.DeadCodeSeverityInfo:
			infoFindings++
		}
		totalFindings++
	}

	scanned := analyzeFilesConcurrently(ctx, snapshot.Files, nil,
		func(_ context.Context, file *ProjectFile) fileAnalysis[*scannedFile] {
			return scanFileForDeadCode(moduleAnalyzer, file)
		})
	if ctx.Err() != nil {
		return nil, fmt.Errorf("dead code analysis cancelled: %w", ctx.Err())
	}

	for index, scan := range scanned {
		filePath := snapshot.Files[index].Path
		warnings = append(warnings, scan.warnings...)
		errors = append(errors, scan.errors...)

		analyzedFiles[filePath] = true
		if scan.value == nil {
			continue
		}

		results := scan.value.deadCode
		moduleInfo := scan.value.moduleInfo
		if moduleInfo != nil {
			allModuleInfos[filePath] = moduleInfo
		}

		var fileFunctions []domain.FunctionDeadCode
		var fileLevelFindings []domain.DeadCodeFinding
		fileTotalFunctions := 0
		fileDeadBlocks := 0
		fileTotalBlocks := 0

		rollup.add(filePath, len(scan.value.unusedImports), 0)

		for _, finding := range scan.value.unusedImports {
			f := domain.DeadCodeFinding{
				Location: domain.DeadCodeLocation{
					FilePath:  filePath,
					StartLine: finding.StartLine,
					EndLine:   finding.EndLine,
				},
				Reason:      string(finding.Reason),
				Severity:    domain.DeadCodeSeverity(finding.Severity),
				Description: finding.Description,
			}
			if !f.Severity.IsAtLeast(minSeverity) {
				continue
			}
			fileLevelFindings = append(fileLevelFindings, f)

			switch f.Severity {
			case domain.DeadCodeSeverityCritical:
				criticalFindings++
			case domain.DeadCodeSeverityWarning:
				warningFindings++
			case domain.DeadCodeSeverityInfo:
				infoFindings++
			}
			totalFindings++
		}

		// Detection results are keyed by function name; walk them in sorted
		// order so the report does not depend on Go's map iteration order.
		funcNames := make([]string, 0, len(results))
		for funcName := range results {
			funcNames = append(funcNames, funcName)
		}
		sort.Strings(funcNames)

		for _, funcName := range funcNames {
			if funcName == domain.ModuleFunctionName {
				continue
			}
			result := results[funcName]

			fileTotalFunctions++
			totalFunctions++
			fileTotalBlocks += result.TotalBlocks
			fileDeadBlocks += result.DeadBlocks
			rollup.add(filePath, len(result.Findings), result.DeadBlocks)

			var findings []domain.DeadCodeFinding
			for _, finding := range result.Findings {
				severity := domain.DeadCodeSeverity(finding.Severity)
				if !severity.IsAtLeast(minSeverity) {
					continue
				}

				f := domain.DeadCodeFinding{
					Location: domain.DeadCodeLocation{
						FilePath:  filePath,
						StartLine: finding.StartLine,
						EndLine:   finding.EndLine,
					},
					FunctionName: funcName,
					Reason:       string(finding.Reason),
					Severity:     severity,
					Description:  finding.Description,
				}
				findings = append(findings, f)

				switch severity {
				case domain.DeadCodeSeverityCritical:
					criticalFindings++
				case domain.DeadCodeSeverityWarning:
					warningFindings++
				case domain.DeadCodeSeverityInfo:
					infoFindings++
				}
				totalFindings++
			}

			if len(findings) > 0 {
				functionsWithDeadCode++
				fn := domain.FunctionDeadCode{
					Name:           funcName,
					FilePath:       filePath,
					Findings:       findings,
					TotalBlocks:    result.TotalBlocks,
					DeadBlocks:     result.DeadBlocks,
					ReachableRatio: result.ReachableRatio,
				}
				fn.CalculateSeverityCounts()
				fileFunctions = append(fileFunctions, fn)
			}
		}

		fileFindingsCount := len(fileLevelFindings)
		for _, fn := range fileFunctions {
			fileFindingsCount += len(fn.Findings)
		}
		fileAffectedFunctions := len(fileFunctions)
		fileDeadCodeRatio := 0.0
		if fileTotalFunctions > 0 {
			fileDeadCodeRatio = float64(fileAffectedFunctions) / float64(fileTotalFunctions)
		}
		fileMetricsByPath[filePath] = fileMetrics{
			totalFunctions:    fileTotalFunctions,
			affectedFunctions: fileAffectedFunctions,
			deadCodeRatio:     fileDeadCodeRatio,
		}

		if fileFindingsCount > 0 {
			entry := domain.FileDeadCode{
				FilePath:          filePath,
				Functions:         fileFunctions,
				FileLevelFindings: fileLevelFindings,
				TotalFindings:     fileFindingsCount,
				TotalFunctions:    fileTotalFunctions,
				AffectedFunctions: fileAffectedFunctions,
				DeadCodeRatio:     fileDeadCodeRatio,
			}
			fileIndexMap[filePath] = len(files)
			files = append(files, entry)
		}

		totalBlocks += fileTotalBlocks
		deadBlocks += fileDeadBlocks
	}

	graph := analyzer.BuildImportGraph(allModuleInfos, analyzedFiles)
	unusedFuncFindings := analyzer.DetectUnusedExportedFunctions(allModuleInfos, graph)
	for _, finding := range unusedFuncFindings {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("dead code analysis cancelled: %w", ctx.Err())
		default:
		}

		f := domain.DeadCodeFinding{
			Location: domain.DeadCodeLocation{
				FilePath:  finding.FilePath,
				StartLine: finding.StartLine,
				EndLine:   finding.EndLine,
			},
			Reason:      string(finding.Reason),
			Severity:    domain.DeadCodeSeverity(finding.Severity),
			Description: finding.Description,
		}

		rollup.add(finding.FilePath, 1, 0)
		if rollupFuncDedup[finding.FilePath] == nil {
			rollupFuncDedup[finding.FilePath] = make(map[int]bool)
		}
		rollupFuncDedup[finding.FilePath][finding.StartLine] = true

		addFileLevelFinding(f)
		if f.Severity.IsAtLeast(minSeverity) {
			if unusedFuncDedup[finding.FilePath] == nil {
				unusedFuncDedup[finding.FilePath] = make(map[int]bool)
			}
			unusedFuncDedup[finding.FilePath][finding.StartLine] = true
		}
	}

	unusedExports := analyzer.DetectUnusedExports(allModuleInfos, graph)
	for _, finding := range unusedExports {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("dead code analysis cancelled: %w", ctx.Err())
		default:
		}

		if lines, ok := rollupFuncDedup[finding.FilePath]; !ok || !lines[finding.StartLine] {
			rollup.add(finding.FilePath, 1, 0)
		}

		if lines, ok := unusedFuncDedup[finding.FilePath]; ok && lines[finding.StartLine] {
			continue
		}

		f := domain.DeadCodeFinding{
			Location: domain.DeadCodeLocation{
				FilePath:  finding.FilePath,
				StartLine: finding.StartLine,
				EndLine:   finding.EndLine,
			},
			Reason:      string(finding.Reason),
			Severity:    domain.DeadCodeSeverity(finding.Severity),
			Description: finding.Description,
		}
		addFileLevelFinding(f)
	}

	orphanFindings := analyzer.DetectOrphanFiles(allModuleInfos, graph)
	for _, finding := range orphanFindings {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("dead code analysis cancelled: %w", ctx.Err())
		default:
		}

		f := domain.DeadCodeFinding{
			Location: domain.DeadCodeLocation{
				FilePath: finding.FilePath,
			},
			Reason:      string(finding.Reason),
			Severity:    domain.DeadCodeSeverity(finding.Severity),
			Description: finding.Description,
		}
		rollup.add(finding.FilePath, 1, 0)
		addFileLevelFinding(f)
	}

	// Every comparator falls back to file path, which is unique per entry, so
	// files the primary criterion cannot separate keep the same order on every
	// run regardless of the order findings were collected in.
	sort.Slice(files, func(i, j int) bool {
		switch sortBy {
		case domain.DeadCodeSortByFile:
			return files[i].FilePath < files[j].FilePath
		case domain.DeadCodeSortByLine:
			if firstDeadCodeLine(files[i]) != firstDeadCodeLine(files[j]) {
				return firstDeadCodeLine(files[i]) < firstDeadCodeLine(files[j])
			}
		case domain.DeadCodeSortByFunction:
			if firstDeadCodeFunction(files[i]) != firstDeadCodeFunction(files[j]) {
				return firstDeadCodeFunction(files[i]) < firstDeadCodeFunction(files[j])
			}
		case domain.DeadCodeSortBySeverity:
			fallthrough
		default:
			if fileMaxSeverity(files[i]) != fileMaxSeverity(files[j]) {
				return fileMaxSeverity(files[i]) > fileMaxSeverity(files[j])
			}
		}
		return files[i].FilePath < files[j].FilePath
	})

	findingsByReason := make(map[string]int)
	for _, file := range files {
		for _, fn := range file.Functions {
			for _, finding := range fn.Findings {
				findingsByReason[finding.Reason]++
			}
		}
		for _, finding := range file.FileLevelFindings {
			findingsByReason[finding.Reason]++
		}
	}

	summary := domain.DeadCodeSummary{
		TotalFiles:            len(snapshot.Files),
		TotalFunctions:        totalFunctions,
		TotalFindings:         totalFindings,
		FilesWithDeadCode:     len(files),
		FunctionsWithDeadCode: functionsWithDeadCode,
		CriticalFindings:      criticalFindings,
		WarningFindings:       warningFindings,
		InfoFindings:          infoFindings,
		FindingsByReason:      findingsByReason,
		TotalBlocks:           totalBlocks,
		DeadBlocks:            deadBlocks,
	}
	if totalBlocks > 0 {
		summary.OverallDeadRatio = float64(deadBlocks) / float64(totalBlocks)
	}

	return &domain.DeadCodeResponse{
		Files:         files,
		Summary:       summary,
		ModuleRollups: rollup,
		Warnings:      warnings,
		Errors:        errors,
		GeneratedAt:   time.Now().Format(time.RFC3339),
		Version:       version.Version,
		Config: map[string]interface{}{
			"min_severity":   minSeverity,
			"sort_by":        sortBy,
			"cross_file":     true,
			"files_analyzed": len(snapshot.Files),
		},
	}, nil
}

func fileMaxSeverity(file domain.FileDeadCode) int {
	maxSeverity := 0
	for _, fn := range file.Functions {
		for _, finding := range fn.Findings {
			if level := finding.Severity.Level(); level > maxSeverity {
				maxSeverity = level
			}
		}
	}
	for _, finding := range file.FileLevelFindings {
		if level := finding.Severity.Level(); level > maxSeverity {
			maxSeverity = level
		}
	}
	return maxSeverity
}

func firstDeadCodeLine(file domain.FileDeadCode) int {
	first := -1
	for _, fn := range file.Functions {
		for _, finding := range fn.Findings {
			if first == -1 || finding.Location.StartLine < first {
				first = finding.Location.StartLine
			}
		}
	}
	for _, finding := range file.FileLevelFindings {
		line := finding.Location.StartLine
		if line == 0 {
			continue
		}
		if first == -1 || line < first {
			first = line
		}
	}
	if first == -1 {
		return 0
	}
	return first
}

func firstDeadCodeFunction(file domain.FileDeadCode) string {
	if len(file.Functions) == 0 {
		return ""
	}
	return file.Functions[0].Name
}

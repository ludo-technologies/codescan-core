package service

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ludo-technologies/polyscan/jscan/domain"
)

func TestDeadCodeServiceAnalyze(t *testing.T) {
	// Create a temp file with dead code
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.js")
	content := `
function hasDeadCode() {
    return 42;
    console.log("never executed");
}

function noDeadCode() {
    const x = 1;
    return x;
}
`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	svc := NewDeadCodeService()
	ctx := context.Background()

	req := domain.DeadCodeRequest{
		Paths:       []string{testFile},
		MinSeverity: domain.DeadCodeSeverityInfo,
		SortBy:      domain.DeadCodeSortBySeverity,
	}

	response, err := svc.Analyze(ctx, req)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if response == nil {
		t.Fatal("Expected non-nil response")
	}

	// Verify summary was generated
	if response.Summary.TotalFiles != 1 {
		t.Errorf("Expected 1 file processed, got %d", response.Summary.TotalFiles)
	}
	if response.Summary.FilesWithDeadCode != 1 {
		t.Errorf("Expected FilesWithDeadCode to be 1, got %d", response.Summary.FilesWithDeadCode)
	}
}

func TestDeadCodeServiceAnalyze_ModuleRollupsPrecedeTheSeverityFilter(t *testing.T) {
	tempDir := t.TempDir()
	helperFile := filepath.Join(tempDir, "helper.js")
	if err := os.WriteFile(helperFile, []byte("export function helper() { return 1; }\n"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	testFile := filepath.Join(tempDir, "test.js")
	content := `
import { helper } from "./helper";

export function hasDeadCode() {
    return 42;
    console.log("never executed");
}
`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	svc := NewDeadCodeService()
	request := domain.DeadCodeRequest{
		Paths:  []string{testFile, helperFile},
		SortBy: domain.DeadCodeSortBySeverity,
	}

	request.MinSeverity = domain.DeadCodeSeverityInfo
	reported, err := svc.Analyze(context.Background(), request)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// The same project, reported at the strictest severity: the report shrinks,
	// the rollups must not.
	request.MinSeverity = domain.DeadCodeSeverityCritical
	filtered, err := svc.Analyze(context.Background(), request)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if filtered.Summary.TotalFindings >= reported.Summary.TotalFindings {
		t.Fatalf("expected the critical-only report to drop findings, got %d of %d",
			filtered.Summary.TotalFindings, reported.Summary.TotalFindings)
	}

	rollup, ok := filtered.ModuleRollups[testFile]
	if !ok {
		t.Fatalf("expected a rollup for %s, got %v", testFile, filtered.ModuleRollups)
	}
	if rollup != reported.ModuleRollups[testFile] {
		t.Errorf("rollups should not depend on min_severity: %+v vs %+v", rollup, reported.ModuleRollups[testFile])
	}
	if rollup.DeadCodeFindingCount == 0 || rollup.DeadCodeBlockCount == 0 {
		t.Errorf("expected the rollup to count the detected dead code, got %+v", rollup)
	}
}

func TestDeadCodeServiceAnalyzeFile(t *testing.T) {
	// Create a temp file with dead code
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.js")
	content := `
function example() {
    if (true) {
        return 1;
    }
    return 2;
}
`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	svc := NewDeadCodeService()
	ctx := context.Background()

	req := domain.DeadCodeRequest{
		MinSeverity: domain.DeadCodeSeverityInfo,
	}

	result, err := svc.AnalyzeFile(ctx, testFile, req)
	if err != nil {
		t.Fatalf("AnalyzeFile failed: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	if result.FilePath != testFile {
		t.Errorf("Expected file path %s, got %s", testFile, result.FilePath)
	}
}

func TestDeadCodeServiceAnalyze_ContextCancelled(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.js")
	content := `
function hasDeadCode() {
    return 1;
    console.log("never");
}
`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	svc := NewDeadCodeService()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	req := domain.DeadCodeRequest{
		Paths:       []string{testFile},
		MinSeverity: domain.DeadCodeSeverityInfo,
		SortBy:      domain.DeadCodeSortBySeverity,
	}

	_, err := svc.Analyze(ctx, req)
	if err == nil {
		t.Fatal("Expected cancellation error")
	}
	if !strings.Contains(err.Error(), "cancelled") {
		t.Fatalf("Expected cancelled error, got: %v", err)
	}
}

func TestDeadCodeServiceAnalyze_NoFunctionModuleIncludesFileLevelFindings(t *testing.T) {
	tempDir := t.TempDir()
	moduleFile := filepath.Join(tempDir, "module.js")
	depFile := filepath.Join(tempDir, "dep.js")

	moduleContent := `
import dep from "./dep.js";
export const answer = 42;
`
	depContent := `export default 1;`

	if err := os.WriteFile(moduleFile, []byte(moduleContent), 0644); err != nil {
		t.Fatalf("Failed to create module file: %v", err)
	}
	if err := os.WriteFile(depFile, []byte(depContent), 0644); err != nil {
		t.Fatalf("Failed to create dep file: %v", err)
	}

	svc := NewDeadCodeService()
	req := domain.DeadCodeRequest{
		Paths:       []string{moduleFile, depFile},
		MinSeverity: domain.DeadCodeSeverityInfo,
		SortBy:      domain.DeadCodeSortBySeverity,
	}

	resp, err := svc.Analyze(context.Background(), req)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	var moduleResult *domain.FileDeadCode
	for i := range resp.Files {
		if resp.Files[i].FilePath == moduleFile {
			moduleResult = &resp.Files[i]
			break
		}
	}

	if moduleResult == nil {
		t.Fatalf("Expected result entry for %s", moduleFile)
	}
	if len(moduleResult.FileLevelFindings) == 0 {
		t.Fatalf("Expected file-level findings for module with no functions")
	}
	if resp.Summary.TotalFindings == 0 {
		t.Fatalf("Expected non-zero summary findings")
	}
}

func TestDeadCodeServiceAnalyze_SummaryMatchesFileTotalsWithUnusedImports(t *testing.T) {
	tempDir := t.TempDir()
	moduleFile := filepath.Join(tempDir, "module.js")
	depFile := filepath.Join(tempDir, "dep.js")

	moduleContent := `
import dep from "./dep.js";
export const answer = 42;
`
	depContent := `export default 1;`

	if err := os.WriteFile(moduleFile, []byte(moduleContent), 0644); err != nil {
		t.Fatalf("Failed to create module file: %v", err)
	}
	if err := os.WriteFile(depFile, []byte(depContent), 0644); err != nil {
		t.Fatalf("Failed to create dep file: %v", err)
	}

	svc := NewDeadCodeService()
	req := domain.DeadCodeRequest{
		Paths:       []string{moduleFile, depFile},
		MinSeverity: domain.DeadCodeSeverityInfo,
		SortBy:      domain.DeadCodeSortBySeverity,
	}

	resp, err := svc.Analyze(context.Background(), req)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	totalFindingsFromFiles := 0
	critical := 0
	warning := 0
	info := 0
	for _, file := range resp.Files {
		totalFindingsFromFiles += file.TotalFindings
		for _, fn := range file.Functions {
			for _, finding := range fn.Findings {
				switch finding.Severity {
				case domain.DeadCodeSeverityCritical:
					critical++
				case domain.DeadCodeSeverityWarning:
					warning++
				case domain.DeadCodeSeverityInfo:
					info++
				}
			}
		}
		for _, finding := range file.FileLevelFindings {
			switch finding.Severity {
			case domain.DeadCodeSeverityCritical:
				critical++
			case domain.DeadCodeSeverityWarning:
				warning++
			case domain.DeadCodeSeverityInfo:
				info++
			}
		}
	}

	if resp.Summary.TotalFindings != totalFindingsFromFiles {
		t.Fatalf("Summary.TotalFindings=%d, file total=%d", resp.Summary.TotalFindings, totalFindingsFromFiles)
	}
	if resp.Summary.CriticalFindings != critical {
		t.Fatalf("Summary.CriticalFindings=%d, computed=%d", resp.Summary.CriticalFindings, critical)
	}
	if resp.Summary.WarningFindings != warning {
		t.Fatalf("Summary.WarningFindings=%d, computed=%d", resp.Summary.WarningFindings, warning)
	}
	if resp.Summary.InfoFindings != info {
		t.Fatalf("Summary.InfoFindings=%d, computed=%d", resp.Summary.InfoFindings, info)
	}
}

func TestDeadCodeServiceAnalyze_CrossFileOnlyFindingPreservesFunctionMetrics(t *testing.T) {
	tempDir := t.TempDir()
	moduleFile := filepath.Join(tempDir, "module.js")
	moduleContent := `
export function alive() {
  return 42;
}
`
	if err := os.WriteFile(moduleFile, []byte(moduleContent), 0644); err != nil {
		t.Fatalf("Failed to create module file: %v", err)
	}

	svc := NewDeadCodeService()
	req := domain.DeadCodeRequest{
		Paths:       []string{moduleFile},
		MinSeverity: domain.DeadCodeSeverityInfo,
		SortBy:      domain.DeadCodeSortBySeverity,
	}

	resp, err := svc.Analyze(context.Background(), req)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if len(resp.Files) != 1 {
		t.Fatalf("Expected exactly one file result, got %d", len(resp.Files))
	}

	file := resp.Files[0]
	if file.TotalFunctions != 1 {
		t.Fatalf("Expected TotalFunctions=1, got %d", file.TotalFunctions)
	}
	if file.AffectedFunctions != 0 {
		t.Fatalf("Expected AffectedFunctions=0, got %d", file.AffectedFunctions)
	}
	if file.DeadCodeRatio != 0 {
		t.Fatalf("Expected DeadCodeRatio=0, got %f", file.DeadCodeRatio)
	}
	if len(file.FileLevelFindings) == 0 {
		t.Fatal("Expected cross-file file-level findings")
	}
}

func TestDeadCodeServiceSorting(t *testing.T) {
	svc := NewDeadCodeService()

	files := []domain.FileDeadCode{
		{
			FilePath:      "b.js",
			TotalFindings: 5,
			Functions: []domain.FunctionDeadCode{
				{
					Name: "func1",
					Findings: []domain.DeadCodeFinding{
						{Severity: domain.DeadCodeSeverityWarning},
					},
				},
			},
		},
		{
			FilePath:      "a.js",
			TotalFindings: 10,
			Functions: []domain.FunctionDeadCode{
				{
					Name: "func2",
					Findings: []domain.DeadCodeFinding{
						{Severity: domain.DeadCodeSeverityCritical},
					},
				},
			},
		},
	}

	// Test sort by file
	sortedByFile := svc.sortFiles(files, domain.DeadCodeSortByFile)
	if sortedByFile[0].FilePath != "a.js" {
		t.Error("Expected files to be sorted by file path")
	}

	// Test sort by severity
	sortedBySeverity := svc.sortFiles(files, domain.DeadCodeSortBySeverity)
	if sortedBySeverity[0].FilePath != "a.js" {
		t.Error("Expected files to be sorted by severity (critical first)")
	}
}

func TestDeadCodeServiceFiltering(t *testing.T) {
	svc := NewDeadCodeService()

	files := []domain.FileDeadCode{
		{
			FilePath: "test.js",
			Functions: []domain.FunctionDeadCode{
				{
					Name: "func1",
					Findings: []domain.DeadCodeFinding{
						{Severity: domain.DeadCodeSeverityInfo},
						{Severity: domain.DeadCodeSeverityWarning},
					},
				},
				{
					Name: "func2",
					Findings: []domain.DeadCodeFinding{
						{Severity: domain.DeadCodeSeverityInfo},
					},
				},
			},
		},
	}

	// Filter with warning minimum severity
	req := domain.DeadCodeRequest{
		MinSeverity: domain.DeadCodeSeverityWarning,
	}

	filtered := svc.filterFiles(files, req)
	if len(filtered) != 1 {
		t.Errorf("Expected 1 file after filtering, got %d", len(filtered))
	}

	// func2 should be filtered out (only has info severity)
	if len(filtered[0].Functions) != 1 {
		t.Errorf("Expected 1 function after filtering, got %d", len(filtered[0].Functions))
	}
	if filtered[0].Functions[0].Name != "func1" {
		t.Errorf("Expected func1 to remain, got %s", filtered[0].Functions[0].Name)
	}
}

func TestDeadCodeServiceSummaryGeneration(t *testing.T) {
	svc := NewDeadCodeService()

	files := []domain.FileDeadCode{
		{
			FilePath:          "test1.js",
			TotalFunctions:    3,
			AffectedFunctions: 2,
			TotalFindings:     5,
			Functions: []domain.FunctionDeadCode{
				{
					CriticalCount: 2,
					WarningCount:  2,
					InfoCount:     1,
					Findings: []domain.DeadCodeFinding{
						{Reason: "unreachable_after_return", Severity: domain.DeadCodeSeverityCritical},
						{Reason: "unreachable_after_return", Severity: domain.DeadCodeSeverityCritical},
						{Reason: "unreachable_branch", Severity: domain.DeadCodeSeverityWarning},
						{Reason: "unreachable_branch", Severity: domain.DeadCodeSeverityWarning},
						{Reason: "dead_assignment", Severity: domain.DeadCodeSeverityInfo},
					},
				},
			},
		},
	}

	req := domain.DeadCodeRequest{
		MinSeverity: domain.DeadCodeSeverityInfo,
	}

	summary := svc.generateSummary(files, 2, req)

	if summary.TotalFiles != 2 {
		t.Errorf("Expected TotalFiles to be 2, got %d", summary.TotalFiles)
	}
	if summary.FilesWithDeadCode != 1 {
		t.Errorf("Expected FilesWithDeadCode to be 1, got %d", summary.FilesWithDeadCode)
	}
	if summary.TotalFindings != 5 {
		t.Errorf("Expected TotalFindings to be 5, got %d", summary.TotalFindings)
	}
	if summary.CriticalFindings != 2 {
		t.Errorf("Expected CriticalFindings to be 2, got %d", summary.CriticalFindings)
	}
	if summary.WarningFindings != 2 {
		t.Errorf("Expected WarningFindings to be 2, got %d", summary.WarningFindings)
	}
	if summary.FindingsByReason["unreachable_after_return"] != 2 {
		t.Errorf("Expected 2 unreachable_after_return findings, got %d", summary.FindingsByReason["unreachable_after_return"])
	}
}

func TestDeadCodeServiceBuildConfig(t *testing.T) {
	svc := NewDeadCodeService()

	req := domain.DeadCodeRequest{
		MinSeverity:  domain.DeadCodeSeverityWarning,
		SortBy:       domain.DeadCodeSortByFile,
		ShowContext:  domain.BoolPtr(true),
		ContextLines: 5,
	}

	config := svc.buildConfigForResponse(req)

	if config["min_severity"] != domain.DeadCodeSeverityWarning {
		t.Errorf("Expected min_severity to be warning, got %v", config["min_severity"])
	}
	if config["sort_by"] != domain.DeadCodeSortByFile {
		t.Errorf("Expected sort_by to be file, got %v", config["sort_by"])
	}
	if config["show_context"] != true {
		t.Errorf("Expected show_context to be true, got %v", config["show_context"])
	}
	if config["context_lines"] != 5 {
		t.Errorf("Expected context_lines to be 5, got %v", config["context_lines"])
	}
}

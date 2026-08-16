package service

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/ludo-technologies/polyscan/jscan/domain"
)

func moduleQualityFixtures() (*domain.ComplexityResponse, *domain.DeadCodeResponse, *domain.DependencyGraphResponse) {
	complexityResponse := &domain.ComplexityResponse{
		ModuleRollups: map[string]domain.ModuleComplexityMetrics{
			"src/a.ts": {
				LinesOfCode:           120,
				AnalyzedFunctionCount: 3,
				AverageComplexity:     6,
				MaxComplexity:         14,
				HighRiskFunctionCount: 1,
				ExceptionHandlerCount: 2,
			},
			"src/b.ts": {LinesOfCode: 20, AnalyzedFunctionCount: 1, AverageComplexity: 1, MaxComplexity: 1},
		},
	}

	deadCodeResponse := &domain.DeadCodeResponse{
		ModuleRollups: map[string]domain.ModuleDeadCodeMetrics{
			"src/b.ts": {DeadCodeFindingCount: 4, DeadCodeBlockCount: 2},
		},
	}

	depsResponse := &domain.DependencyGraphResponse{
		Analysis: &domain.DependencyAnalysisResult{
			ModuleMetrics: map[string]*domain.ModuleDependencyMetrics{
				"a":       {ModuleName: "a", FilePath: "./src/a.ts"},
				"vendor":  {ModuleName: "vendor", FilePath: "vendor/other.ts"},
				"missing": nil,
			},
		},
	}

	return complexityResponse, deadCodeResponse, depsResponse
}

func TestBuildModuleQualityJoinsEveryAnalysis(t *testing.T) {
	modules := BuildModuleQuality(moduleQualityFixtures())

	if len(modules) != 2 {
		t.Fatalf("expected 2 modules, got %d", len(modules))
	}

	// src/a.ts owns the only high-risk function, so it ranks first.
	worst := modules[0]
	if worst.FilePath != "src/a.ts" {
		t.Fatalf("expected src/a.ts first, got %q", worst.FilePath)
	}
	if worst.ModuleName != "a" {
		t.Errorf("expected the module name from dependency analysis, got %q", worst.ModuleName)
	}
	if worst.LinesOfCode != 120 || worst.AnalyzedFunctionCount != 3 || worst.ExceptionHandlerCount != 2 {
		t.Errorf("complexity rollup was not carried over: %+v", worst)
	}
	if worst.DeadCodeFindingCount != 0 {
		t.Errorf("expected no dead code findings, got %d", worst.DeadCodeFindingCount)
	}

	quiet := modules[1]
	if quiet.FilePath != "src/b.ts" {
		t.Fatalf("expected src/b.ts second, got %q", quiet.FilePath)
	}
	if quiet.DeadCodeFindingCount != 4 || quiet.DeadCodeBlockCount != 2 {
		t.Errorf("dead code rollup was not carried over: %+v", quiet)
	}
	if quiet.ModuleName != "" {
		t.Errorf("expected no module name for a file dependency analysis did not name, got %q", quiet.ModuleName)
	}
}

func TestBuildModuleQualityIgnoresModulesNoAnalysisMeasured(t *testing.T) {
	_, _, depsResponse := moduleQualityFixtures()

	if modules := BuildModuleQuality(nil, nil, depsResponse); len(modules) != 0 {
		t.Errorf("expected dependency analysis alone to report no modules, got %d", len(modules))
	}
}

func TestBuildModuleQualityWithoutAnalysesIsEmpty(t *testing.T) {
	if modules := BuildModuleQuality(nil, nil, nil); len(modules) != 0 {
		t.Errorf("expected no modules, got %d", len(modules))
	}
}

func TestWriteAnalyzeJSONReportsModuleQualityAndDirectories(t *testing.T) {
	complexityResponse, deadCodeResponse, depsResponse := moduleQualityFixtures()
	complexityResponse.ByDirectory = domain.DirectoryComplexityMetricsList{
		{DirectoryPath: "src", FunctionCount: 4, AverageComplexity: 4.75, MaxComplexity: 14, HighRiskFunctionCount: 1, AverageNestingDepth: 1.5, MaxNestingDepth: 3},
	}

	var buf bytes.Buffer
	formatter := NewOutputFormatter()
	if err := formatter.WriteAnalyze(complexityResponse, deadCodeResponse, nil, nil, depsResponse, domain.OutputFormatJSON, &buf, time.Second); err != nil {
		t.Fatalf("WriteAnalyze failed: %v", err)
	}

	var report struct {
		Complexity struct {
			ByDirectory []domain.DirectoryComplexityMetrics `json:"by_directory"`
		} `json:"complexity"`
		ModuleQuality []domain.ModuleQualityMetrics `json:"module_quality"`
	}
	if err := json.Unmarshal(buf.Bytes(), &report); err != nil {
		t.Fatalf("failed to parse the report: %v", err)
	}

	if len(report.Complexity.ByDirectory) != 1 || report.Complexity.ByDirectory[0].DirectoryPath != "src" {
		t.Errorf("expected the directory rollup in the report, got %+v", report.Complexity.ByDirectory)
	}
	if len(report.ModuleQuality) != 2 || report.ModuleQuality[0].FilePath != "src/a.ts" {
		t.Errorf("expected the module quality join in the report, got %+v", report.ModuleQuality)
	}
	if report.ModuleQuality[0].LinesOfCode != 120 {
		t.Errorf("expected lines of code to survive serialization, got %d", report.ModuleQuality[0].LinesOfCode)
	}
}

func TestWriteAnalyzeTextListsModuleHotspotsAndDirectories(t *testing.T) {
	complexityResponse, deadCodeResponse, depsResponse := moduleQualityFixtures()
	complexityResponse.Functions = []domain.FunctionComplexity{{Name: "fn", FilePath: "src/a.ts", Metrics: domain.ComplexityMetrics{Complexity: 14}, RiskLevel: domain.RiskLevelHigh}}
	complexityResponse.ByDirectory = domain.DirectoryComplexityMetricsList{
		{DirectoryPath: "src", FunctionCount: 4, AverageComplexity: 4.75, MaxComplexity: 14, HighRiskFunctionCount: 1, AverageNestingDepth: 1.5, MaxNestingDepth: 3},
	}

	var buf bytes.Buffer
	formatter := NewOutputFormatter()
	if err := formatter.WriteAnalyze(complexityResponse, deadCodeResponse, nil, nil, depsResponse, domain.OutputFormatText, &buf, time.Second); err != nil {
		t.Fatalf("WriteAnalyze failed: %v", err)
	}

	report := buf.String()
	for _, expected := range []string{
		"Directory Complexity:",
		"  src\n",
		"Nesting: avg 1.50, max 3",
		"=== Module Quality Hotspots ===",
		"a (src/a.ts)",
		"Lines: 120, functions analyzed: 3",
		"Dead code: 4 findings, 2 blocks",
	} {
		if !strings.Contains(report, expected) {
			t.Errorf("expected the report to contain %q:\n%s", expected, report)
		}
	}
}

func TestWriteAnalyzeTextTruncatesTheModuleHotspotList(t *testing.T) {
	rollups := make(map[string]domain.ModuleComplexityMetrics, moduleQualityTextLimit+1)
	for index := 0; index <= moduleQualityTextLimit; index++ {
		rollups[string(rune('a'+index))+".ts"] = domain.ModuleComplexityMetrics{AnalyzedFunctionCount: 1}
	}

	var buf bytes.Buffer
	writeModuleQualityText(&buf, BuildModuleQuality(&domain.ComplexityResponse{ModuleRollups: rollups}, nil, nil))

	if !strings.Contains(buf.String(), "Showing top 10 of 11 modules") {
		t.Errorf("expected the truncation notice, got:\n%s", buf.String())
	}
}

func TestWriteAnalyzeCSVReportsRollupRows(t *testing.T) {
	complexityResponse, deadCodeResponse, depsResponse := moduleQualityFixtures()
	complexityResponse.Functions = []domain.FunctionComplexity{{Name: "fn", FilePath: "src/a.ts", Metrics: domain.ComplexityMetrics{Complexity: 14}, RiskLevel: domain.RiskLevelHigh}}
	complexityResponse.ByDirectory = domain.DirectoryComplexityMetricsList{
		{DirectoryPath: "src", FunctionCount: 4, AverageComplexity: 4.75, MaxComplexity: 14, HighRiskFunctionCount: 1, AverageNestingDepth: 1.5, MaxNestingDepth: 3},
	}

	var buf bytes.Buffer
	formatter := NewOutputFormatter()
	if err := formatter.WriteAnalyze(complexityResponse, deadCodeResponse, nil, nil, depsResponse, domain.OutputFormatCSV, &buf, time.Second); err != nil {
		t.Fatalf("WriteAnalyze failed: %v", err)
	}

	report := buf.String()
	for _, expected := range []string{
		"directory_complexity,src,4,4.75,14,1,1.50,3",
		"module_quality,a,src/a.ts,120,3,6.00,14,1,2,0,0",
	} {
		if !strings.Contains(report, expected) {
			t.Errorf("expected the CSV to contain %q:\n%s", expected, report)
		}
	}
}

func TestWriteHTMLRendersRollupTables(t *testing.T) {
	complexityResponse, deadCodeResponse, depsResponse := moduleQualityFixtures()
	complexityResponse.ByDirectory = domain.DirectoryComplexityMetricsList{
		{DirectoryPath: "src", FunctionCount: 4, AverageComplexity: 4.75, MaxComplexity: 14, HighRiskFunctionCount: 1, AverageNestingDepth: 1.5, MaxNestingDepth: 3},
	}

	var buf bytes.Buffer
	formatter := NewOutputFormatter()
	if err := formatter.WriteHTML(complexityResponse, deadCodeResponse, nil, nil, depsResponse, &buf, time.Second); err != nil {
		t.Fatalf("WriteHTML failed: %v", err)
	}

	report := buf.String()
	for _, expected := range []string{
		"Directory Complexity",
		"All modules",
		"Hotspot files",
		"sortModuleQuality(",
		"src/a.ts",
	} {
		if !strings.Contains(report, expected) {
			t.Errorf("expected the HTML report to contain %q", expected)
		}
	}
}

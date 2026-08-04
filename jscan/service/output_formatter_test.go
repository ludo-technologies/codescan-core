package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ludo-technologies/polyscan/jscan/domain"
)

func TestWriteJSON(t *testing.T) {
	data := map[string]interface{}{
		"name":  "test",
		"value": 42,
	}

	var buf bytes.Buffer
	err := WriteJSON(&buf, data)
	if err != nil {
		t.Fatalf("WriteJSON failed: %v", err)
	}

	// Check that it's valid JSON
	var result map[string]interface{}
	err = json.Unmarshal(buf.Bytes(), &result)
	if err != nil {
		t.Fatalf("Failed to parse output as JSON: %v", err)
	}

	if result["name"] != "test" {
		t.Errorf("Expected name to be 'test', got %v", result["name"])
	}
}

func TestCalculateDuplicationPercentageReportsUncappedRatio(t *testing.T) {
	response := &domain.CloneResponse{Statistics: &domain.CloneStatistics{
		TotalFragments: 100,
		TotalClones:    50,
	}}

	if got := calculateDuplicationPercentage(response); got != 50 {
		t.Fatalf("expected actual 50%% fragment ratio, got %.1f%%", got)
	}

	summary := BuildAnalyzeSummary(nil, nil, response, nil, nil)
	if summary.CodeDuplication != 50 {
		t.Fatalf("expected summary to retain actual ratio, got %.1f%%", summary.CodeDuplication)
	}
	if summary.DuplicationScore != 0 {
		t.Fatalf("expected duplication score to saturate at zero, got %d", summary.DuplicationScore)
	}
}

func TestBuildAnalyzeSummary_WiresProjectScale(t *testing.T) {
	complexityResponse := &domain.ComplexityResponse{
		Summary: domain.ComplexitySummary{
			TotalFunctions: 240,
			FilesAnalyzed:  123,
		},
	}
	cloneResponse := &domain.CloneResponse{Statistics: &domain.CloneStatistics{
		LinesAnalyzed: 7890,
	}}

	summary := BuildAnalyzeSummary(complexityResponse, nil, cloneResponse, nil, nil)

	if summary.ProjectScale != domain.ScaleMedium {
		t.Errorf("ProjectScale = %q, want %q", summary.ProjectScale, domain.ScaleMedium)
	}
	if summary.TotalLOC != 7890 {
		t.Errorf("TotalLOC = %d, want 7890", summary.TotalLOC)
	}

	want := "Medium (123 files, 240 functions, 7890 LOC)"
	if got := FormatProjectScale(summary); got != want {
		t.Errorf("FormatProjectScale() = %q, want %q", got, want)
	}
	if cli := FormatCLISummary(summary, time.Second); !strings.Contains(cli, "Project Scale: "+want) {
		t.Errorf("CLI summary missing project scale line:\n%s", cli)
	}
}

func TestFormatProjectScale_OmitsLOCWhenUnavailable(t *testing.T) {
	// Clone analysis is what supplies the line count, so a run without it
	// reports files and functions only.
	summary := BuildAnalyzeSummary(&domain.ComplexityResponse{
		Summary: domain.ComplexitySummary{
			TotalFunctions: 6,
			FilesAnalyzed:  4,
		},
	}, nil, nil, nil, nil)

	want := "Micro (4 files, 6 functions)"
	if got := FormatProjectScale(summary); got != want {
		t.Errorf("FormatProjectScale() = %q, want %q", got, want)
	}
}

func TestOutputFormatterWriteComplexityJSON(t *testing.T) {
	formatter := NewOutputFormatter()

	response := &domain.ComplexityResponse{
		Functions: []domain.FunctionComplexity{
			{
				Name:      "testFunc",
				FilePath:  "test.js",
				StartLine: 1,
				EndLine:   10,
				Metrics: domain.ComplexityMetrics{
					Complexity: 5,
					Nodes:      10,
					Edges:      15,
				},
				RiskLevel: domain.RiskLevelLow,
			},
		},
		Summary: domain.ComplexitySummary{
			TotalFunctions:    1,
			AverageComplexity: 5.0,
			MaxComplexity:     5,
			MinComplexity:     5,
			FilesAnalyzed:     1,
			LowRiskFunctions:  1,
		},
		GeneratedAt: time.Now().Format(time.RFC3339),
		Version:     "test",
	}

	var buf bytes.Buffer
	err := formatter.Write(response, domain.OutputFormatJSON, &buf)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Verify JSON structure
	var result ComplexityResponseJSON
	err = json.Unmarshal(buf.Bytes(), &result)
	if err != nil {
		t.Fatalf("Failed to parse output as JSON: %v", err)
	}

	if len(result.Functions) != 1 {
		t.Errorf("Expected 1 function, got %d", len(result.Functions))
	}
	if result.Functions[0].Name != "testFunc" {
		t.Errorf("Expected function name 'testFunc', got %s", result.Functions[0].Name)
	}
}

func TestOutputFormatterWriteComplexityText(t *testing.T) {
	formatter := NewOutputFormatter()

	response := &domain.ComplexityResponse{
		Functions: []domain.FunctionComplexity{
			{
				Name:      "testFunc",
				FilePath:  "test.js",
				StartLine: 1,
				EndLine:   10,
				Metrics: domain.ComplexityMetrics{
					Complexity: 5,
				},
				RiskLevel: domain.RiskLevelLow,
			},
		},
		Summary: domain.ComplexitySummary{
			TotalFunctions:    1,
			AverageComplexity: 5.0,
			MaxComplexity:     5,
			MinComplexity:     5,
			FilesAnalyzed:     1,
		},
		GeneratedAt: time.Now().Format(time.RFC3339),
		Version:     "test",
	}

	var buf bytes.Buffer
	err := formatter.Write(response, domain.OutputFormatText, &buf)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	output := buf.String()

	// Check for expected content
	if !strings.Contains(output, "Complexity Analysis") {
		t.Error("Expected output to contain 'Complexity Analysis'")
	}
	if !strings.Contains(output, "testFunc") {
		t.Error("Expected output to contain function name 'testFunc'")
	}
	if !strings.Contains(output, "Total functions: 1") {
		t.Error("Expected output to contain 'Total functions: 1'")
	}
}

func TestOutputFormatterWriteDeadCodeJSON(t *testing.T) {
	formatter := NewOutputFormatter()

	response := &domain.DeadCodeResponse{
		Files: []domain.FileDeadCode{
			{
				FilePath: "test.js",
				Functions: []domain.FunctionDeadCode{
					{
						Name:     "testFunc",
						FilePath: "test.js",
						Findings: []domain.DeadCodeFinding{
							{
								Location: domain.DeadCodeLocation{
									FilePath:  "test.js",
									StartLine: 5,
									EndLine:   5,
								},
								FunctionName: "testFunc",
								Reason:       "unreachable_after_return",
								Severity:     domain.DeadCodeSeverityWarning,
								Description:  "Code after return statement",
							},
						},
						CriticalCount: 0,
						WarningCount:  1,
						InfoCount:     0,
					},
				},
				TotalFindings: 1,
			},
		},
		Summary: domain.DeadCodeSummary{
			TotalFiles:      1,
			TotalFunctions:  1,
			TotalFindings:   1,
			WarningFindings: 1,
		},
		GeneratedAt: time.Now().Format(time.RFC3339),
		Version:     "test",
	}

	var buf bytes.Buffer
	err := formatter.WriteDeadCode(response, domain.OutputFormatJSON, &buf)
	if err != nil {
		t.Fatalf("WriteDeadCode failed: %v", err)
	}

	// Verify JSON structure
	var result DeadCodeResponseJSON
	err = json.Unmarshal(buf.Bytes(), &result)
	if err != nil {
		t.Fatalf("Failed to parse output as JSON: %v", err)
	}

	if len(result.Files) != 1 {
		t.Errorf("Expected 1 file, got %d", len(result.Files))
	}
	if result.Summary.TotalFindings != 1 {
		t.Errorf("Expected 1 finding, got %d", result.Summary.TotalFindings)
	}
}

func TestOutputFormatterWriteAnalyzeJSON(t *testing.T) {
	formatter := NewOutputFormatter()

	complexityResponse := &domain.ComplexityResponse{
		Functions: []domain.FunctionComplexity{
			{
				Name:      "testFunc",
				FilePath:  "test.js",
				Metrics:   domain.ComplexityMetrics{Complexity: 5},
				RiskLevel: domain.RiskLevelLow,
			},
		},
		Summary: domain.ComplexitySummary{
			TotalFunctions:    1,
			AverageComplexity: 5.0,
			FilesAnalyzed:     1,
		},
		GeneratedAt: time.Now().Format(time.RFC3339),
		Version:     "test",
	}

	var buf bytes.Buffer
	err := formatter.WriteAnalyze(complexityResponse, nil, nil, nil, nil, domain.OutputFormatJSON, &buf, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("WriteAnalyze failed: %v", err)
	}

	// Verify JSON structure
	var result AnalyzeResponseJSON
	err = json.Unmarshal(buf.Bytes(), &result)
	if err != nil {
		t.Fatalf("Failed to parse output as JSON: %v", err)
	}

	if result.Complexity == nil {
		t.Error("Expected complexity response to be present")
	}
	if result.Summary == nil {
		t.Error("Expected summary to be present")
	}
	if result.Summary.ComplexityEnabled != true {
		t.Error("Expected complexity to be enabled in summary")
	}
}

func TestOutputFormatterWriteAnalyzeJSON_CloneErrorIncluded(t *testing.T) {
	formatter := NewOutputFormatter()

	cloneResponse := &domain.CloneResponse{
		Success: false,
		Error:   "clone analysis completed with 1 file error(s)",
		Statistics: &domain.CloneStatistics{
			LinesAnalyzed: 10,
		},
	}

	var buf bytes.Buffer
	err := formatter.WriteAnalyze(nil, nil, cloneResponse, nil, nil, domain.OutputFormatJSON, &buf, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("WriteAnalyze failed: %v", err)
	}

	var result AnalyzeResponseJSON
	err = json.Unmarshal(buf.Bytes(), &result)
	if err != nil {
		t.Fatalf("Failed to parse output as JSON: %v", err)
	}

	if result.Clone == nil {
		t.Fatal("Expected clone response to be present")
	}
	if result.Clone.Success {
		t.Error("Expected clone success=false")
	}
	if result.Clone.Error == "" {
		t.Error("Expected clone error to be present")
	}
}

func TestOutputFormatterWriteHTML(t *testing.T) {
	formatter := NewOutputFormatter()

	complexityResponse := &domain.ComplexityResponse{
		Functions: []domain.FunctionComplexity{
			{
				Name:      "testFunc",
				FilePath:  "test.js",
				Metrics:   domain.ComplexityMetrics{Complexity: 5},
				RiskLevel: domain.RiskLevelLow,
			},
		},
		Summary: domain.ComplexitySummary{
			TotalFunctions:    1,
			AverageComplexity: 5.0,
			MaxComplexity:     5,
			FilesAnalyzed:     1,
		},
		GeneratedAt: time.Now().Format(time.RFC3339),
		Version:     "test",
	}

	deadCodeResponse := &domain.DeadCodeResponse{
		Summary: domain.DeadCodeSummary{
			TotalFiles:      1,
			TotalFunctions:  1,
			TotalFindings:   1,
			WarningFindings: 1,
		},
		GeneratedAt: time.Now().Format(time.RFC3339),
		Version:     "test",
	}

	var buf bytes.Buffer
	err := formatter.WriteAnalyze(complexityResponse, deadCodeResponse, nil, nil, nil, domain.OutputFormatHTML, &buf, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("WriteAnalyze with HTML failed: %v", err)
	}

	output := buf.String()

	// Check for expected HTML content
	if !strings.Contains(output, "<!DOCTYPE html>") {
		t.Error("Expected output to contain HTML doctype")
	}
	if !strings.Contains(output, "jscan Analysis Report") {
		t.Error("Expected output to contain 'jscan Analysis Report'")
	}
	if !strings.Contains(output, "Health Score") {
		t.Error("Expected output to contain 'Health Score'")
	}
	if !strings.Contains(output, "testFunc") {
		t.Error("Expected output to contain function name 'testFunc'")
	}
}

func TestOutputFormatterWriteHTML_CloneNilSafe(t *testing.T) {
	formatter := NewOutputFormatter()

	cloneResponse := &domain.CloneResponse{
		ClonePairs: []*domain.ClonePair{
			{ID: 1},
		},
	}

	var buf bytes.Buffer
	err := formatter.WriteAnalyze(nil, nil, cloneResponse, nil, nil, domain.OutputFormatHTML, &buf, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("WriteAnalyze with HTML failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Clone Detection") {
		t.Error("Expected output to contain clone section")
	}
}

func TestOutputFormatterWriteAnalyzeCSV_WithDeps(t *testing.T) {
	formatter := NewOutputFormatter()

	graph := domain.NewDependencyGraph()
	graph.AddNode(&domain.ModuleNode{ID: "src/a.ts", Name: "a", FilePath: "src/a.ts"})
	graph.AddNode(&domain.ModuleNode{ID: "src/b.ts", Name: "b", FilePath: "src/b.ts"})
	graph.AddEdge(&domain.DependencyEdge{
		From:     "src/a.ts",
		To:       "src/b.ts",
		EdgeType: domain.EdgeTypeImport,
		Weight:   1,
	})

	depsResponse := &domain.DependencyGraphResponse{
		Graph:       graph,
		GeneratedAt: time.Now().Format(time.RFC3339),
		Version:     "test",
	}

	var buf bytes.Buffer
	err := formatter.WriteAnalyze(nil, nil, nil, nil, depsResponse, domain.OutputFormatCSV, &buf, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("WriteAnalyze with CSV failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "type,from,to,edge_type,weight") {
		t.Error("Expected CSV header for deps output")
	}
	if !strings.Contains(output, "deps,src/a.ts,src/b.ts,import,1") {
		t.Error("Expected deps CSV row")
	}
}

func TestOutputFormatterUnsupportedFormat(t *testing.T) {
	formatter := NewOutputFormatter()

	response := &domain.ComplexityResponse{}
	var buf bytes.Buffer

	err := formatter.Write(response, domain.OutputFormatYAML, &buf)
	if err == nil {
		t.Error("Expected error for unsupported format")
	}
}

func TestBuildAnalyzeSummary_WiresMSD(t *testing.T) {
	graph := domain.NewDependencyGraph()
	for i := 0; i < 10; i++ {
		graph.AddNode(&domain.ModuleNode{ID: fmt.Sprintf("mod%d", i)})
	}

	depsResponse := &domain.DependencyGraphResponse{
		Graph: graph,
		Analysis: &domain.DependencyAnalysisResult{
			MaxDepth: 3,
			CouplingAnalysis: &domain.CouplingAnalysis{
				MainSequenceDeviation: 0.42,
			},
		},
	}

	summary := BuildAnalyzeSummary(nil, nil, nil, nil, depsResponse)

	if summary.DepsMainSequenceDeviation != 0.42 {
		t.Errorf("DepsMainSequenceDeviation = %f, want 0.42", summary.DepsMainSequenceDeviation)
	}
	if summary.DependencyScore >= 100 {
		t.Errorf("DependencyScore should be < 100 when MSD > 0, got %d", summary.DependencyScore)
	}
}

func TestBuildAnalyzeSummary_WiresCycles(t *testing.T) {
	graph := domain.NewDependencyGraph()
	for i := 0; i < 100; i++ {
		graph.AddNode(&domain.ModuleNode{ID: fmt.Sprintf("mod%d", i)})
	}

	depsResponse := &domain.DependencyGraphResponse{
		Graph: graph,
		Analysis: &domain.DependencyAnalysisResult{
			MaxDepth: 3,
			CircularDependencies: &domain.CircularDependencyAnalysis{
				TotalModulesInCycles: 10,
			},
		},
	}

	summary := BuildAnalyzeSummary(nil, nil, nil, nil, depsResponse)

	if summary.DepsModulesInCycles != 10 {
		t.Errorf("DepsModulesInCycles = %d, want 10", summary.DepsModulesInCycles)
	}
	if summary.DependencyScore >= 100 {
		t.Errorf("DependencyScore should be < 100 when cycles exist, got %d", summary.DependencyScore)
	}
}

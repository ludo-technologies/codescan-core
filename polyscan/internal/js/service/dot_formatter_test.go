package service

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ludo-technologies/polyscan/polyscan/internal/js/domain"
)

func TestEscapeDOTID(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple path",
			input:    "src/index",
			expected: "src__index",
		},
		{
			name:     "path with extension",
			input:    "src/index.ts",
			expected: "src__index_ts",
		},
		{
			name:     "path with dashes",
			input:    "src/my-component",
			expected: "src__my_component",
		},
		{
			name:     "path with @",
			input:    "@scope/package",
			expected: "_at_scope__package",
		},
		{
			name:     "starts with number",
			input:    "123abc",
			expected: "_123abc",
		},
		{
			name:     "path with dots",
			input:    "src.component.ts",
			expected: "src_component_ts",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := escapeDOTID(tc.input)
			if result != tc.expected {
				t.Errorf("escapeDOTID(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestEscapeDOTLabel(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple string",
			input:    "hello",
			expected: "hello",
		},
		{
			name:     "string with quotes",
			input:    `hello "world"`,
			expected: `hello \"world\"`,
		},
		{
			name:     "string with newline",
			input:    "hello\nworld",
			expected: `hello\nworld`,
		},
		{
			name:     "string with backslash",
			input:    `path\to\file`,
			expected: `path\\to\\file`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := escapeDOTLabel(tc.input)
			if result != tc.expected {
				t.Errorf("escapeDOTLabel(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestDOTFormatterBasic(t *testing.T) {
	// Create a simple graph
	graph := domain.NewDependencyGraph()

	graph.AddNode(&domain.ModuleNode{
		ID:           "src/index.ts",
		Name:         "index",
		FilePath:     "src/index.ts",
		IsEntryPoint: true,
	})
	graph.AddNode(&domain.ModuleNode{
		ID:       "src/app.ts",
		Name:     "app",
		FilePath: "src/app.ts",
	})
	graph.AddNode(&domain.ModuleNode{
		ID:     "src/utils.ts",
		Name:   "utils",
		IsLeaf: true,
	})

	graph.AddEdge(&domain.DependencyEdge{
		From:     "src/index.ts",
		To:       "src/app.ts",
		EdgeType: domain.EdgeTypeImport,
	})
	graph.AddEdge(&domain.DependencyEdge{
		From:     "src/app.ts",
		To:       "src/utils.ts",
		EdgeType: domain.EdgeTypeImport,
	})

	response := &domain.DependencyGraphResponse{
		Graph:    graph,
		Analysis: &domain.DependencyAnalysisResult{},
	}

	formatter := NewDOTFormatter(nil)
	result, err := formatter.FormatDependencyGraph(response)
	if err != nil {
		t.Fatalf("FormatDependencyGraph failed: %v", err)
	}

	// Check basic structure
	if !strings.Contains(result, "digraph dependencies {") {
		t.Error("Missing digraph declaration")
	}
	if !strings.Contains(result, "src__index_ts") {
		t.Error("Missing node for src/index.ts")
	}
	if !strings.Contains(result, "src__app_ts") {
		t.Error("Missing node for src/app.ts")
	}
	if !strings.Contains(result, "src__utils_ts") {
		t.Error("Missing node for src/utils.ts")
	}
	if !strings.Contains(result, "src__index_ts -> src__app_ts") {
		t.Error("Missing edge from index to app")
	}
	if !strings.Contains(result, "src__app_ts -> src__utils_ts") {
		t.Error("Missing edge from app to utils")
	}
}

func TestDOTFormatterWithRiskLevels(t *testing.T) {
	graph := domain.NewDependencyGraph()

	graph.AddNode(&domain.ModuleNode{
		ID:           "low_risk.ts",
		Name:         "low_risk",
		IsEntryPoint: true,
	})
	graph.AddNode(&domain.ModuleNode{
		ID:   "medium_risk.ts",
		Name: "medium_risk",
	})
	graph.AddNode(&domain.ModuleNode{
		ID:   "high_risk.ts",
		Name: "high_risk",
	})

	graph.AddEdge(&domain.DependencyEdge{
		From:     "low_risk.ts",
		To:       "medium_risk.ts",
		EdgeType: domain.EdgeTypeImport,
	})
	graph.AddEdge(&domain.DependencyEdge{
		From:     "medium_risk.ts",
		To:       "high_risk.ts",
		EdgeType: domain.EdgeTypeImport,
	})

	response := &domain.DependencyGraphResponse{
		Graph: graph,
		Analysis: &domain.DependencyAnalysisResult{
			ModuleMetrics: map[string]*domain.ModuleDependencyMetrics{
				"low_risk.ts":    {RiskLevel: domain.RiskLevelLow},
				"medium_risk.ts": {RiskLevel: domain.RiskLevelMedium},
				"high_risk.ts":   {RiskLevel: domain.RiskLevelHigh},
			},
		},
	}

	formatter := NewDOTFormatter(nil)
	result, err := formatter.FormatDependencyGraph(response)
	if err != nil {
		t.Fatalf("FormatDependencyGraph failed: %v", err)
	}

	// Check risk level colors
	if !strings.Contains(result, "#90EE90") { // Low risk fill
		t.Error("Missing low risk color")
	}
	if !strings.Contains(result, "#FFD700") { // Medium risk fill
		t.Error("Missing medium risk color")
	}
	if !strings.Contains(result, "#FF6B6B") { // High risk fill
		t.Error("Missing high risk color")
	}
}

func TestDOTFormatterWithCycles(t *testing.T) {
	graph := domain.NewDependencyGraph()

	graph.AddNode(&domain.ModuleNode{
		ID:           "a.ts",
		Name:         "a",
		IsEntryPoint: true,
	})
	graph.AddNode(&domain.ModuleNode{
		ID:   "b.ts",
		Name: "b",
	})

	graph.AddEdge(&domain.DependencyEdge{
		From:     "a.ts",
		To:       "b.ts",
		EdgeType: domain.EdgeTypeImport,
	})
	graph.AddEdge(&domain.DependencyEdge{
		From:     "b.ts",
		To:       "a.ts",
		EdgeType: domain.EdgeTypeImport,
	})

	response := &domain.DependencyGraphResponse{
		Graph: graph,
		Analysis: &domain.DependencyAnalysisResult{
			CircularDependencies: &domain.CircularDependencyAnalysis{
				HasCircularDependencies: true,
				TotalCycles:             1,
				CircularDependencies: []domain.CircularDependency{
					{
						Modules:  []string{"a.ts", "b.ts"},
						Severity: domain.CycleSeverityMedium,
					},
				},
			},
		},
	}

	config := DefaultDOTFormatterConfig()
	config.ClusterCycles = true
	formatter := NewDOTFormatter(config)
	result, err := formatter.FormatDependencyGraph(response)
	if err != nil {
		t.Fatalf("FormatDependencyGraph failed: %v", err)
	}

	// Check cycle clustering
	if !strings.Contains(result, "subgraph cluster_cycle_0") {
		t.Error("Missing cycle cluster")
	}
	if !strings.Contains(result, "#FFEEEE") { // Cycle fill color
		t.Error("Missing cycle fill color")
	}
	if !strings.Contains(result, "#DC143C") { // Cycle border color
		t.Error("Missing cycle border color")
	}
	if !strings.Contains(result, "penwidth=2") { // Cycle edge style
		t.Error("Missing cycle edge penwidth")
	}
}

func TestDOTFormatterEdgeTypes(t *testing.T) {
	graph := domain.NewDependencyGraph()

	graph.AddNode(&domain.ModuleNode{
		ID:           "main.ts",
		Name:         "main",
		IsEntryPoint: true,
	})
	graph.AddNode(&domain.ModuleNode{
		ID:   "dynamic.ts",
		Name: "dynamic",
	})
	graph.AddNode(&domain.ModuleNode{
		ID:   "types.ts",
		Name: "types",
	})
	graph.AddNode(&domain.ModuleNode{
		ID:   "reexport.ts",
		Name: "reexport",
	})

	graph.AddEdge(&domain.DependencyEdge{
		From:     "main.ts",
		To:       "dynamic.ts",
		EdgeType: domain.EdgeTypeDynamic,
	})
	graph.AddEdge(&domain.DependencyEdge{
		From:     "main.ts",
		To:       "types.ts",
		EdgeType: domain.EdgeTypeTypeOnly,
	})
	graph.AddEdge(&domain.DependencyEdge{
		From:     "main.ts",
		To:       "reexport.ts",
		EdgeType: domain.EdgeTypeReExport,
	})

	response := &domain.DependencyGraphResponse{
		Graph:    graph,
		Analysis: &domain.DependencyAnalysisResult{},
	}

	formatter := NewDOTFormatter(nil)
	result, err := formatter.FormatDependencyGraph(response)
	if err != nil {
		t.Fatalf("FormatDependencyGraph failed: %v", err)
	}

	// Check edge styles
	if !strings.Contains(result, "style=dashed") {
		t.Error("Missing dashed style for dynamic import")
	}
	if !strings.Contains(result, "style=dotted") {
		t.Error("Missing dotted style for type-only import")
	}
	if !strings.Contains(result, "style=bold") {
		t.Error("Missing bold style for re-export")
	}
	if !strings.Contains(result, "arrowhead=empty") {
		t.Error("Missing empty arrowhead for dynamic import")
	}
	if !strings.Contains(result, "arrowhead=odot") {
		t.Error("Missing odot arrowhead for type-only import")
	}
	if !strings.Contains(result, "arrowhead=diamond") {
		t.Error("Missing diamond arrowhead for re-export")
	}
}

func TestDOTFormatterWithLegend(t *testing.T) {
	graph := domain.NewDependencyGraph()
	graph.AddNode(&domain.ModuleNode{
		ID:           "test.ts",
		Name:         "test",
		IsEntryPoint: true,
	})

	response := &domain.DependencyGraphResponse{
		Graph:    graph,
		Analysis: &domain.DependencyAnalysisResult{},
	}

	// Test with legend enabled
	config := DefaultDOTFormatterConfig()
	config.ShowLegend = true
	formatter := NewDOTFormatter(config)
	result, err := formatter.FormatDependencyGraph(response)
	if err != nil {
		t.Fatalf("FormatDependencyGraph failed: %v", err)
	}

	if !strings.Contains(result, "subgraph cluster_legend") {
		t.Error("Missing legend when ShowLegend is true")
	}
	if !strings.Contains(result, "Low Risk") {
		t.Error("Missing Low Risk in legend")
	}
	if !strings.Contains(result, "Medium Risk") {
		t.Error("Missing Medium Risk in legend")
	}
	if !strings.Contains(result, "High Risk") {
		t.Error("Missing High Risk in legend")
	}

	// Test with legend disabled
	config.ShowLegend = false
	formatter = NewDOTFormatter(config)
	result, err = formatter.FormatDependencyGraph(response)
	if err != nil {
		t.Fatalf("FormatDependencyGraph failed: %v", err)
	}

	if strings.Contains(result, "subgraph cluster_legend") {
		t.Error("Legend present when ShowLegend is false")
	}
}

func TestDOTFormatterMinCouplingFilter(t *testing.T) {
	graph := domain.NewDependencyGraph()

	graph.AddNode(&domain.ModuleNode{
		ID:           "high_coupling.ts",
		Name:         "high_coupling",
		IsEntryPoint: true,
	})
	graph.AddNode(&domain.ModuleNode{
		ID:   "low_coupling.ts",
		Name: "low_coupling",
	})

	graph.AddEdge(&domain.DependencyEdge{
		From:     "high_coupling.ts",
		To:       "low_coupling.ts",
		EdgeType: domain.EdgeTypeImport,
	})

	response := &domain.DependencyGraphResponse{
		Graph: graph,
		Analysis: &domain.DependencyAnalysisResult{
			ModuleMetrics: map[string]*domain.ModuleDependencyMetrics{
				"high_coupling.ts": {
					AfferentCoupling: 5,
					EfferentCoupling: 3,
				},
				"low_coupling.ts": {
					AfferentCoupling: 1,
					EfferentCoupling: 0,
				},
			},
		},
	}

	config := DefaultDOTFormatterConfig()
	config.MinCoupling = 5 // Only include nodes with total coupling >= 5
	formatter := NewDOTFormatter(config)
	result, err := formatter.FormatDependencyGraph(response)
	if err != nil {
		t.Fatalf("FormatDependencyGraph failed: %v", err)
	}

	// high_coupling.ts has total coupling of 8, should be included
	if !strings.Contains(result, "high_coupling_ts") {
		t.Error("high_coupling.ts should be included (coupling=8)")
	}

	// low_coupling.ts has total coupling of 1, should be excluded
	if strings.Contains(result, "low_coupling_ts") {
		t.Error("low_coupling.ts should be excluded (coupling=1)")
	}
}

func TestDOTFormatterNilResponse(t *testing.T) {
	formatter := NewDOTFormatter(nil)

	_, err := formatter.FormatDependencyGraph(nil)
	if err == nil {
		t.Error("Expected error for nil response")
	}

	_, err = formatter.FormatDependencyGraph(&domain.DependencyGraphResponse{})
	if err == nil {
		t.Error("Expected error for nil graph")
	}
}

func TestDOTFormatterRankDir(t *testing.T) {
	graph := domain.NewDependencyGraph()
	graph.AddNode(&domain.ModuleNode{
		ID:           "test.ts",
		Name:         "test",
		IsEntryPoint: true,
	})

	response := &domain.DependencyGraphResponse{
		Graph:    graph,
		Analysis: &domain.DependencyAnalysisResult{},
	}

	testCases := []string{"TB", "LR", "BT", "RL"}

	for _, rankDir := range testCases {
		t.Run(rankDir, func(t *testing.T) {
			config := DefaultDOTFormatterConfig()
			config.RankDir = rankDir
			formatter := NewDOTFormatter(config)

			result, err := formatter.FormatDependencyGraph(response)
			if err != nil {
				t.Fatalf("FormatDependencyGraph failed: %v", err)
			}

			expected := "rankdir=" + rankDir
			if !strings.Contains(result, expected) {
				t.Errorf("Expected %s in output", expected)
			}
		})
	}
}

func TestDOTFormatterInvalidRankDir(t *testing.T) {
	graph := domain.NewDependencyGraph()
	graph.AddNode(&domain.ModuleNode{
		ID:           "test.ts",
		Name:         "test",
		IsEntryPoint: true,
	})

	response := &domain.DependencyGraphResponse{
		Graph:    graph,
		Analysis: &domain.DependencyAnalysisResult{},
	}

	config := DefaultDOTFormatterConfig()
	config.RankDir = "INVALID"
	formatter := NewDOTFormatter(config)

	_, err := formatter.FormatDependencyGraph(response)
	if err == nil {
		t.Error("Expected error for invalid RankDir")
	}
	if !strings.Contains(err.Error(), "invalid rank direction") {
		t.Errorf("Expected 'invalid rank direction' in error, got: %v", err)
	}
}

func TestDOTFormatterWriteDependencyGraph(t *testing.T) {
	graph := domain.NewDependencyGraph()
	graph.AddNode(&domain.ModuleNode{
		ID:           "test.ts",
		Name:         "test",
		IsEntryPoint: true,
	})

	response := &domain.DependencyGraphResponse{
		Graph:    graph,
		Analysis: &domain.DependencyAnalysisResult{},
	}

	formatter := NewDOTFormatter(nil)

	var buf bytes.Buffer
	err := formatter.WriteDependencyGraph(response, &buf)
	if err != nil {
		t.Fatalf("WriteDependencyGraph failed: %v", err)
	}

	result := buf.String()
	if !strings.Contains(result, "digraph dependencies") {
		t.Error("Output doesn't contain expected content")
	}
}

func TestShortenModuleName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"src/components/Button.tsx", "Button"},
		{"utils.ts", "utils"},
		{"src/index", "index"},
		{"", ""},
		{"single", "single"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := shortenModuleName(tc.input)
			if result != tc.expected {
				t.Errorf("shortenModuleName(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestDOTFormatterMaxDepthFilter(t *testing.T) {
	// Create a graph with depth: entry -> level1 -> level2 -> level3
	graph := domain.NewDependencyGraph()

	graph.AddNode(&domain.ModuleNode{
		ID:           "entry.ts",
		Name:         "entry",
		IsEntryPoint: true,
	})
	graph.AddNode(&domain.ModuleNode{
		ID:   "level1.ts",
		Name: "level1",
	})
	graph.AddNode(&domain.ModuleNode{
		ID:   "level2.ts",
		Name: "level2",
	})
	graph.AddNode(&domain.ModuleNode{
		ID:   "level3.ts",
		Name: "level3",
	})

	graph.AddEdge(&domain.DependencyEdge{
		From:     "entry.ts",
		To:       "level1.ts",
		EdgeType: domain.EdgeTypeImport,
	})
	graph.AddEdge(&domain.DependencyEdge{
		From:     "level1.ts",
		To:       "level2.ts",
		EdgeType: domain.EdgeTypeImport,
	})
	graph.AddEdge(&domain.DependencyEdge{
		From:     "level2.ts",
		To:       "level3.ts",
		EdgeType: domain.EdgeTypeImport,
	})

	response := &domain.DependencyGraphResponse{
		Graph:    graph,
		Analysis: &domain.DependencyAnalysisResult{},
	}

	t.Run("MaxDepth=1 shows entry and level1 only", func(t *testing.T) {
		config := DefaultDOTFormatterConfig()
		config.MaxDepth = 1
		config.ShowLegend = false
		formatter := NewDOTFormatter(config)

		result, err := formatter.FormatDependencyGraph(response)
		if err != nil {
			t.Fatalf("FormatDependencyGraph failed: %v", err)
		}

		// entry.ts (depth 0) should be included
		if !strings.Contains(result, "entry_ts") {
			t.Error("entry.ts should be included (depth=0)")
		}
		// level1.ts (depth 1) should be included
		if !strings.Contains(result, "level1_ts") {
			t.Error("level1.ts should be included (depth=1)")
		}
		// level2.ts (depth 2) should be excluded
		if strings.Contains(result, "level2_ts") {
			t.Error("level2.ts should be excluded (depth=2)")
		}
		// level3.ts (depth 3) should be excluded
		if strings.Contains(result, "level3_ts") {
			t.Error("level3.ts should be excluded (depth=3)")
		}
	})

	t.Run("MaxDepth=2 shows up to level2", func(t *testing.T) {
		config := DefaultDOTFormatterConfig()
		config.MaxDepth = 2
		config.ShowLegend = false
		formatter := NewDOTFormatter(config)

		result, err := formatter.FormatDependencyGraph(response)
		if err != nil {
			t.Fatalf("FormatDependencyGraph failed: %v", err)
		}

		if !strings.Contains(result, "entry_ts") {
			t.Error("entry.ts should be included")
		}
		if !strings.Contains(result, "level1_ts") {
			t.Error("level1.ts should be included")
		}
		if !strings.Contains(result, "level2_ts") {
			t.Error("level2.ts should be included (depth=2)")
		}
		if strings.Contains(result, "level3_ts") {
			t.Error("level3.ts should be excluded (depth=3)")
		}
	})

	t.Run("MaxDepth=0 (unlimited) shows all nodes", func(t *testing.T) {
		config := DefaultDOTFormatterConfig()
		config.MaxDepth = 0
		config.ShowLegend = false
		formatter := NewDOTFormatter(config)

		result, err := formatter.FormatDependencyGraph(response)
		if err != nil {
			t.Fatalf("FormatDependencyGraph failed: %v", err)
		}

		if !strings.Contains(result, "entry_ts") {
			t.Error("entry.ts should be included")
		}
		if !strings.Contains(result, "level1_ts") {
			t.Error("level1.ts should be included")
		}
		if !strings.Contains(result, "level2_ts") {
			t.Error("level2.ts should be included")
		}
		if !strings.Contains(result, "level3_ts") {
			t.Error("level3.ts should be included")
		}
	})
}

func TestDOTFormatterMaxDepthNoEntryPoints(t *testing.T) {
	// Test behavior when MaxDepth is set but there are no entry points
	graph := domain.NewDependencyGraph()

	graph.AddNode(&domain.ModuleNode{
		ID:           "a.ts",
		Name:         "a",
		IsEntryPoint: false, // Not an entry point
	})
	graph.AddNode(&domain.ModuleNode{
		ID:   "b.ts",
		Name: "b",
	})

	graph.AddEdge(&domain.DependencyEdge{
		From:     "a.ts",
		To:       "b.ts",
		EdgeType: domain.EdgeTypeImport,
	})

	response := &domain.DependencyGraphResponse{
		Graph:    graph,
		Analysis: &domain.DependencyAnalysisResult{},
	}

	config := DefaultDOTFormatterConfig()
	config.MaxDepth = 1
	config.ShowLegend = false
	formatter := NewDOTFormatter(config)

	result, err := formatter.FormatDependencyGraph(response)
	if err != nil {
		t.Fatalf("FormatDependencyGraph failed: %v", err)
	}

	// With no entry points, BFS won't find any nodes
	// Should produce empty graph message
	if !strings.Contains(result, "No modules match the filter criteria") {
		t.Error("Expected empty graph when MaxDepth is set but no entry points exist")
	}
}

func TestDOTFormatterEmptyGraph(t *testing.T) {
	graph := domain.NewDependencyGraph()
	// Add only external nodes which will be filtered out
	graph.AddNode(&domain.ModuleNode{
		ID:         "node_modules/lodash/index.js",
		Name:       "lodash",
		IsExternal: true,
	})

	response := &domain.DependencyGraphResponse{
		Graph:    graph,
		Analysis: &domain.DependencyAnalysisResult{},
	}

	formatter := NewDOTFormatter(nil)
	result, err := formatter.FormatDependencyGraph(response)
	if err != nil {
		t.Fatalf("FormatDependencyGraph failed: %v", err)
	}

	// Should produce empty graph with comment
	if !strings.Contains(result, "No modules match the filter criteria") {
		t.Error("Expected empty graph message")
	}
}

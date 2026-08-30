package analyzer

import (
	"strings"
	"testing"

	"github.com/ludo-technologies/polyscan/polyscan/internal/js/domain"
)

func TestNewCircularDependencyDetector(t *testing.T) {
	detector := NewCircularDependencyDetector()
	if detector == nil {
		t.Fatal("Expected detector to not be nil")
	}
}

func TestDetectCyclesNilGraph(t *testing.T) {
	detector := NewCircularDependencyDetector()
	result := detector.DetectCycles(nil)

	if result == nil {
		t.Fatal("Expected result to not be nil")
	}
	if result.HasCircularDependencies {
		t.Error("Expected no circular dependencies for nil graph")
	}
	if result.TotalCycles != 0 {
		t.Error("Expected 0 cycles for nil graph")
	}
}

func TestDetectCyclesEmptyGraph(t *testing.T) {
	detector := NewCircularDependencyDetector()
	graph := domain.NewDependencyGraph()
	result := detector.DetectCycles(graph)

	if result.HasCircularDependencies {
		t.Error("Expected no circular dependencies for empty graph")
	}
}

func TestDetectCyclesNoCycle(t *testing.T) {
	// A -> B -> C (linear, no cycle)
	graph := domain.NewDependencyGraph()
	graph.AddNode(&domain.ModuleNode{ID: "a"})
	graph.AddNode(&domain.ModuleNode{ID: "b"})
	graph.AddNode(&domain.ModuleNode{ID: "c"})
	graph.AddEdge(&domain.DependencyEdge{From: "a", To: "b", Weight: 1})
	graph.AddEdge(&domain.DependencyEdge{From: "b", To: "c", Weight: 1})

	detector := NewCircularDependencyDetector()
	result := detector.DetectCycles(graph)

	if result.HasCircularDependencies {
		t.Error("Expected no circular dependencies for linear graph")
	}
	if result.TotalCycles != 0 {
		t.Errorf("Expected 0 cycles, got %d", result.TotalCycles)
	}
}

func TestDetectCyclesSimpleCycle(t *testing.T) {
	// A -> B -> A (simple 2-node cycle)
	graph := domain.NewDependencyGraph()
	graph.AddNode(&domain.ModuleNode{ID: "a"})
	graph.AddNode(&domain.ModuleNode{ID: "b"})
	graph.AddEdge(&domain.DependencyEdge{From: "a", To: "b", Weight: 1})
	graph.AddEdge(&domain.DependencyEdge{From: "b", To: "a", Weight: 1})

	detector := NewCircularDependencyDetector()
	result := detector.DetectCycles(graph)

	if !result.HasCircularDependencies {
		t.Error("Expected circular dependencies to be detected")
	}
	if result.TotalCycles != 1 {
		t.Errorf("Expected 1 cycle, got %d", result.TotalCycles)
	}
	if result.TotalModulesInCycles != 2 {
		t.Errorf("Expected 2 modules in cycles, got %d", result.TotalModulesInCycles)
	}

	// Check cycle details
	if len(result.CircularDependencies) != 1 {
		t.Fatalf("Expected 1 cycle, got %d", len(result.CircularDependencies))
	}
	cycle := result.CircularDependencies[0]
	if cycle.Size != 2 {
		t.Errorf("Expected cycle size 2, got %d", cycle.Size)
	}
	if cycle.Severity != domain.CycleSeverityLow {
		t.Errorf("Expected severity Low for 2-node cycle, got %s", cycle.Severity)
	}
}

func TestDetectCyclesThreeNodeCycle(t *testing.T) {
	// A -> B -> C -> A (3-node cycle)
	graph := domain.NewDependencyGraph()
	graph.AddNode(&domain.ModuleNode{ID: "a"})
	graph.AddNode(&domain.ModuleNode{ID: "b"})
	graph.AddNode(&domain.ModuleNode{ID: "c"})
	graph.AddEdge(&domain.DependencyEdge{From: "a", To: "b", Weight: 1})
	graph.AddEdge(&domain.DependencyEdge{From: "b", To: "c", Weight: 1})
	graph.AddEdge(&domain.DependencyEdge{From: "c", To: "a", Weight: 1})

	detector := NewCircularDependencyDetector()
	result := detector.DetectCycles(graph)

	if !result.HasCircularDependencies {
		t.Error("Expected circular dependencies to be detected")
	}
	if result.TotalCycles != 1 {
		t.Errorf("Expected 1 cycle, got %d", result.TotalCycles)
	}
	if result.TotalModulesInCycles != 3 {
		t.Errorf("Expected 3 modules in cycles, got %d", result.TotalModulesInCycles)
	}

	cycle := result.CircularDependencies[0]
	if cycle.Severity != domain.CycleSeverityMedium {
		t.Errorf("Expected severity Medium for 3-node cycle, got %s", cycle.Severity)
	}
}

func TestDetectCyclesMultipleCycles(t *testing.T) {
	// Two separate cycles: A -> B -> A and C -> D -> C
	graph := domain.NewDependencyGraph()
	graph.AddNode(&domain.ModuleNode{ID: "a"})
	graph.AddNode(&domain.ModuleNode{ID: "b"})
	graph.AddNode(&domain.ModuleNode{ID: "c"})
	graph.AddNode(&domain.ModuleNode{ID: "d"})
	// First cycle
	graph.AddEdge(&domain.DependencyEdge{From: "a", To: "b", Weight: 1})
	graph.AddEdge(&domain.DependencyEdge{From: "b", To: "a", Weight: 1})
	// Second cycle
	graph.AddEdge(&domain.DependencyEdge{From: "c", To: "d", Weight: 1})
	graph.AddEdge(&domain.DependencyEdge{From: "d", To: "c", Weight: 1})

	detector := NewCircularDependencyDetector()
	result := detector.DetectCycles(graph)

	if !result.HasCircularDependencies {
		t.Error("Expected circular dependencies to be detected")
	}
	if result.TotalCycles != 2 {
		t.Errorf("Expected 2 cycles, got %d", result.TotalCycles)
	}
	if result.TotalModulesInCycles != 4 {
		t.Errorf("Expected 4 modules in cycles, got %d", result.TotalModulesInCycles)
	}
}

func TestDetectCyclesLargeCycle(t *testing.T) {
	// 7-node cycle: A -> B -> C -> D -> E -> F -> G -> A
	graph := domain.NewDependencyGraph()
	nodes := []string{"a", "b", "c", "d", "e", "f", "g"}
	for _, n := range nodes {
		graph.AddNode(&domain.ModuleNode{ID: n})
	}
	for i := 0; i < len(nodes); i++ {
		from := nodes[i]
		to := nodes[(i+1)%len(nodes)]
		graph.AddEdge(&domain.DependencyEdge{From: from, To: to, Weight: 1})
	}

	detector := NewCircularDependencyDetector()
	result := detector.DetectCycles(graph)

	if !result.HasCircularDependencies {
		t.Error("Expected circular dependencies to be detected")
	}

	cycle := result.CircularDependencies[0]
	if cycle.Severity != domain.CycleSeverityCritical {
		t.Errorf("Expected severity Critical for 7-node cycle, got %s", cycle.Severity)
	}
}

func TestDetectCyclesCoreInfrastructure(t *testing.T) {
	// Module 'x' is in two cycles: A -> B -> X -> A and C -> D -> X -> C
	graph := domain.NewDependencyGraph()
	graph.AddNode(&domain.ModuleNode{ID: "a"})
	graph.AddNode(&domain.ModuleNode{ID: "b"})
	graph.AddNode(&domain.ModuleNode{ID: "c"})
	graph.AddNode(&domain.ModuleNode{ID: "d"})
	graph.AddNode(&domain.ModuleNode{ID: "x"})
	// First cycle: A -> B -> X -> A
	graph.AddEdge(&domain.DependencyEdge{From: "a", To: "b", Weight: 1})
	graph.AddEdge(&domain.DependencyEdge{From: "b", To: "x", Weight: 1})
	graph.AddEdge(&domain.DependencyEdge{From: "x", To: "a", Weight: 1})
	// Second cycle: C -> D -> X -> C
	graph.AddEdge(&domain.DependencyEdge{From: "c", To: "d", Weight: 1})
	graph.AddEdge(&domain.DependencyEdge{From: "d", To: "x", Weight: 1})
	graph.AddEdge(&domain.DependencyEdge{From: "x", To: "c", Weight: 1})

	detector := NewCircularDependencyDetector()
	result := detector.DetectCycles(graph)

	// X should be in core infrastructure as it's in multiple cycles
	// Note: Due to Tarjan's algorithm, these might be detected as one larger SCC
	// depending on the graph structure
	if !result.HasCircularDependencies {
		t.Error("Expected circular dependencies to be detected")
	}
}

func TestCycleSeverity(t *testing.T) {
	detector := NewCircularDependencyDetector()

	testCases := []struct {
		size     int
		expected domain.CycleSeverity
	}{
		{1, domain.CycleSeverityLow}, // Technically not a cycle, but test the logic
		{2, domain.CycleSeverityLow},
		{3, domain.CycleSeverityMedium},
		{4, domain.CycleSeverityMedium},
		{5, domain.CycleSeverityHigh},
		{6, domain.CycleSeverityHigh},
		{7, domain.CycleSeverityCritical},
		{10, domain.CycleSeverityCritical},
	}

	for _, tc := range testCases {
		scc := make([]string, tc.size)
		for i := 0; i < tc.size; i++ {
			scc[i] = string(rune('a' + i))
		}
		severity := detector.calculateCycleSeverity(scc)
		if severity != tc.expected {
			t.Errorf("Size %d: expected severity %s, got %s", tc.size, tc.expected, severity)
		}
	}
}

func TestCycleBreakingSuggestions(t *testing.T) {
	// A -> B -> C -> A with different weights
	graph := domain.NewDependencyGraph()
	graph.AddNode(&domain.ModuleNode{ID: "a"})
	graph.AddNode(&domain.ModuleNode{ID: "b"})
	graph.AddNode(&domain.ModuleNode{ID: "c"})
	graph.AddEdge(&domain.DependencyEdge{From: "a", To: "b", Weight: 5})
	graph.AddEdge(&domain.DependencyEdge{From: "b", To: "c", Weight: 1}) // Lowest weight
	graph.AddEdge(&domain.DependencyEdge{From: "c", To: "a", Weight: 3})

	detector := NewCircularDependencyDetector()
	result := detector.DetectCycles(graph)

	if len(result.CycleBreakingSuggestions) == 0 {
		t.Error("Expected cycle breaking suggestions")
	}

	// Should suggest breaking the edge with lowest weight (b -> c)
	found := false
	for _, suggestion := range result.CycleBreakingSuggestions {
		if strings.Contains(suggestion, "b") && strings.Contains(suggestion, "c") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected suggestion to break edge from b to c (lowest weight)")
	}
}

func TestFindCyclePath(t *testing.T) {
	// A -> B -> C
	graph := domain.NewDependencyGraph()
	graph.AddNode(&domain.ModuleNode{ID: "a"})
	graph.AddNode(&domain.ModuleNode{ID: "b"})
	graph.AddNode(&domain.ModuleNode{ID: "c"})
	graph.AddEdge(&domain.DependencyEdge{From: "a", To: "b", Weight: 1})
	graph.AddEdge(&domain.DependencyEdge{From: "b", To: "c", Weight: 1})

	detector := NewCircularDependencyDetector()
	path := detector.FindCyclePath("a", "c", graph)

	if len(path) != 3 {
		t.Errorf("Expected path length 3, got %d", len(path))
	}
	if path[0] != "a" || path[1] != "b" || path[2] != "c" {
		t.Errorf("Expected path [a, b, c], got %v", path)
	}
}

func TestFindCyclePathNoPath(t *testing.T) {
	// A -> B, C (disconnected)
	graph := domain.NewDependencyGraph()
	graph.AddNode(&domain.ModuleNode{ID: "a"})
	graph.AddNode(&domain.ModuleNode{ID: "b"})
	graph.AddNode(&domain.ModuleNode{ID: "c"})
	graph.AddEdge(&domain.DependencyEdge{From: "a", To: "b", Weight: 1})

	detector := NewCircularDependencyDetector()
	path := detector.FindCyclePath("a", "c", graph)

	if path != nil {
		t.Errorf("Expected nil path for disconnected nodes, got %v", path)
	}
}

func TestGetModuleBaseName(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"src/components/Button.tsx", "Button.tsx"},
		{"/absolute/path/file.js", "file.js"},
		{"simple.js", "simple.js"},
		{"", ""},
	}

	for _, tc := range testCases {
		result := getModuleBaseName(tc.input)
		if result != tc.expected {
			t.Errorf("getModuleBaseName(%q) = %q, expected %q", tc.input, result, tc.expected)
		}
	}
}

func TestGenerateCycleDescription(t *testing.T) {
	detector := NewCircularDependencyDetector()

	scc := []string{"src/a.js", "src/b.js", "src/c.js"}
	description := detector.generateCycleDescription(scc)

	if description == "" {
		t.Error("Expected non-empty description")
	}
	if !strings.Contains(description, "3 modules") {
		t.Error("Expected description to mention 3 modules")
	}
}

func TestDetectCyclesSkipsDynamicEdges(t *testing.T) {
	// A -> B (static), B -> A (dynamic import only): a dynamic import is
	// evaluated at call time, not module load time, so this is not a
	// load-time cycle. See pyscn issue #460.
	graph := domain.NewDependencyGraph()
	graph.AddNode(&domain.ModuleNode{ID: "a"})
	graph.AddNode(&domain.ModuleNode{ID: "b"})
	graph.AddEdge(&domain.DependencyEdge{From: "a", To: "b", EdgeType: domain.EdgeTypeImport, Weight: 1})
	graph.AddEdge(&domain.DependencyEdge{From: "b", To: "a", EdgeType: domain.EdgeTypeDynamic, Weight: 1})

	detector := NewCircularDependencyDetector()
	result := detector.DetectCycles(graph)

	if result.HasCircularDependencies {
		t.Error("Expected no circular dependencies when back edge is dynamic-only")
	}
	if result.TotalCycles != 0 {
		t.Errorf("Expected 0 cycles, got %d", result.TotalCycles)
	}
}

func TestDetectCyclesKeepsCycleWithStaticAndDynamicEdges(t *testing.T) {
	// A -> B (static), B -> A (static AND dynamic): the static back edge
	// still forms a load-time cycle; the extra dynamic edge must not hide it.
	graph := domain.NewDependencyGraph()
	graph.AddNode(&domain.ModuleNode{ID: "a"})
	graph.AddNode(&domain.ModuleNode{ID: "b"})
	graph.AddEdge(&domain.DependencyEdge{From: "a", To: "b", EdgeType: domain.EdgeTypeImport, Weight: 1})
	graph.AddEdge(&domain.DependencyEdge{From: "b", To: "a", EdgeType: domain.EdgeTypeDynamic, Weight: 1})
	graph.AddEdge(&domain.DependencyEdge{From: "b", To: "a", EdgeType: domain.EdgeTypeImport, Weight: 1})

	detector := NewCircularDependencyDetector()
	result := detector.DetectCycles(graph)

	if !result.HasCircularDependencies {
		t.Error("Expected circular dependencies to be detected via the static back edge")
	}
	if result.TotalCycles != 1 {
		t.Errorf("Expected 1 cycle, got %d", result.TotalCycles)
	}
}

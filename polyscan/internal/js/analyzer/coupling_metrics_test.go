package analyzer

import (
	"context"
	"math"
	"testing"

	"github.com/ludo-technologies/polyscan/polyscan/internal/js/domain"
)

func TestDefaultCouplingMetricsConfig(t *testing.T) {
	config := DefaultCouplingMetricsConfig()

	if config.InstabilityHighThreshold != 0.7 {
		t.Errorf("Expected InstabilityHighThreshold 0.7, got %f", config.InstabilityHighThreshold)
	}
	if config.InstabilityLowThreshold != 0.3 {
		t.Errorf("Expected InstabilityLowThreshold 0.3, got %f", config.InstabilityLowThreshold)
	}
	if config.DistanceThreshold != 0.3 {
		t.Errorf("Expected DistanceThreshold 0.3, got %f", config.DistanceThreshold)
	}
}

func TestNewCouplingMetricsCalculator(t *testing.T) {
	calc := NewCouplingMetricsCalculator(nil)
	if calc == nil {
		t.Fatal("Expected calculator to not be nil")
	}
	if calc.config == nil {
		t.Fatal("Expected config to not be nil")
	}
}

func TestCalculateMetricsUsesCoreCouplingAndPreservesDuplicateEdges(t *testing.T) {
	graph := domain.NewDependencyGraph()
	graph.AddNode(&domain.ModuleNode{ID: "a"})
	graph.AddNode(&domain.ModuleNode{ID: "b"})
	graph.AddNode(&domain.ModuleNode{ID: "c"})
	graph.AddNode(&domain.ModuleNode{ID: "isolated"})
	graph.AddEdge(&domain.DependencyEdge{From: "a", To: "b", Weight: 1})
	graph.AddEdge(&domain.DependencyEdge{From: "a", To: "b", Weight: 1})
	graph.AddEdge(&domain.DependencyEdge{From: "c", To: "a", Weight: 1})

	calc := NewCouplingMetricsCalculator(nil)
	metrics := calc.CalculateMetrics(graph)

	if got := metrics["a"].AfferentCoupling; got != 1 {
		t.Errorf("A Ca = %d, want 1", got)
	}
	if got := metrics["a"].EfferentCoupling; got != 2 {
		t.Errorf("A Ce = %d, want 2", got)
	}
	if got := metrics["b"].AfferentCoupling; got != 2 {
		t.Errorf("B Ca = %d, want 2", got)
	}
	if got := metrics["a"].Instability; math.Abs(got-2.0/3.0) > 0.001 {
		t.Errorf("A instability = %f, want %f", got, 2.0/3.0)
	}
	if got := metrics["isolated"].Instability; got != 0 {
		t.Errorf("isolated instability = %f, want core semantics 0", got)
	}
	if got := metrics["isolated"].Distance; got != 1 {
		t.Errorf("isolated distance = %f, want 1", got)
	}
}

func TestClassifyStabilityZone(t *testing.T) {
	calc := NewCouplingMetricsCalculator(nil)

	testCases := []struct {
		instability  float64
		abstractness float64
		distance     float64
		afferent     int
		expected     string
	}{
		{0.5, 0.5, 0.0, 0, "main_sequence"},       // On main sequence
		{0.5, 0.5, 0.2, 0, "main_sequence"},       // Near main sequence
		{0.2, 0.2, 0.6, 2, "zone_of_pain"},        // Stable + concrete, depended on
		{0.1, 0.1, 0.8, 5, "zone_of_pain"},        // Very stable + concrete, depended on
		{0.2, 0.2, 0.6, 1, ""},                    // Stable + concrete but nothing depends on it
		{0.8, 0.8, 0.6, 0, "zone_of_uselessness"}, // Unstable + abstract
		{0.9, 0.9, 0.8, 0, "zone_of_uselessness"}, // Very unstable + abstract
		{0.5, 0.5, 0.35, 0, ""},                   // Off the main sequence but in no zone
	}

	for _, tc := range testCases {
		m := &domain.ModuleDependencyMetrics{
			Instability:      tc.instability,
			Abstractness:     tc.abstractness,
			Distance:         tc.distance,
			AfferentCoupling: tc.afferent,
		}
		result := calc.classifyStabilityZone(m)
		if result != tc.expected {
			t.Errorf("Zone(I=%f, A=%f, D=%f, Ca=%d) = %q, expected %q",
				tc.instability, tc.abstractness, tc.distance, tc.afferent, result, tc.expected)
		}
	}
}

func TestAssessRiskLevel(t *testing.T) {
	calc := NewCouplingMetricsCalculator(nil)

	testCases := []struct {
		ca       int
		ce       int
		distance float64
		expected domain.RiskLevel
	}{
		{1, 1, 0.1, domain.RiskLevelLow},     // Low coupling, on main sequence
		{3, 3, 0.2, domain.RiskLevelMedium},  // Medium coupling
		{5, 5, 0.1, domain.RiskLevelHigh},    // High coupling
		{1, 1, 0.6, domain.RiskLevelHigh},    // High distance
		{3, 2, 0.35, domain.RiskLevelMedium}, // Moderate distance
	}

	for _, tc := range testCases {
		result := calc.assessRiskLevel(tc.ca, tc.ce, tc.distance)
		if result != tc.expected {
			t.Errorf("Risk(Ca=%d, Ce=%d, D=%f) = %s, expected %s",
				tc.ca, tc.ce, tc.distance, result, tc.expected)
		}
	}
}

func TestCalculateMetricsEmptyGraph(t *testing.T) {
	calc := NewCouplingMetricsCalculator(nil)
	result := calc.CalculateMetrics(nil)

	if result == nil {
		t.Fatal("Expected result to not be nil")
	}
	if len(result) != 0 {
		t.Error("Expected empty result for nil graph")
	}
}

func TestCalculateMetrics(t *testing.T) {
	// Create a graph: A -> B -> C
	// A: Ce=1, Ca=0 (unstable)
	// B: Ce=1, Ca=1 (neutral)
	// C: Ce=0, Ca=1 (stable)
	graph := domain.NewDependencyGraph()
	graph.AddNode(&domain.ModuleNode{ID: "a", Name: "a", Exports: []string{"foo"}})
	graph.AddNode(&domain.ModuleNode{ID: "b", Name: "b", Exports: []string{"bar", "baz"}})
	graph.AddNode(&domain.ModuleNode{ID: "c", Name: "c", Exports: []string{}})
	graph.AddEdge(&domain.DependencyEdge{From: "a", To: "b", Weight: 1})
	graph.AddEdge(&domain.DependencyEdge{From: "b", To: "c", Weight: 1})

	calc := NewCouplingMetricsCalculator(nil)
	metrics := calc.CalculateMetrics(graph)

	if len(metrics) != 3 {
		t.Fatalf("Expected 3 metrics, got %d", len(metrics))
	}

	// Check A (unstable)
	metricsA := metrics["a"]
	if metricsA.AfferentCoupling != 0 {
		t.Errorf("A: Expected Ca=0, got %d", metricsA.AfferentCoupling)
	}
	if metricsA.EfferentCoupling != 1 {
		t.Errorf("A: Expected Ce=1, got %d", metricsA.EfferentCoupling)
	}
	if metricsA.Instability != 1.0 {
		t.Errorf("A: Expected I=1.0, got %f", metricsA.Instability)
	}

	// Check B (neutral)
	metricsB := metrics["b"]
	if metricsB.AfferentCoupling != 1 {
		t.Errorf("B: Expected Ca=1, got %d", metricsB.AfferentCoupling)
	}
	if metricsB.EfferentCoupling != 1 {
		t.Errorf("B: Expected Ce=1, got %d", metricsB.EfferentCoupling)
	}
	if metricsB.Instability != 0.5 {
		t.Errorf("B: Expected I=0.5, got %f", metricsB.Instability)
	}

	// Check C (stable)
	metricsC := metrics["c"]
	if metricsC.AfferentCoupling != 1 {
		t.Errorf("C: Expected Ca=1, got %d", metricsC.AfferentCoupling)
	}
	if metricsC.EfferentCoupling != 0 {
		t.Errorf("C: Expected Ce=0, got %d", metricsC.EfferentCoupling)
	}
	if metricsC.Instability != 0.0 {
		t.Errorf("C: Expected I=0.0, got %f", metricsC.Instability)
	}
}

func TestCalculateCouplingAnalysis(t *testing.T) {
	graph := domain.NewDependencyGraph()
	graph.AddNode(&domain.ModuleNode{ID: "a", Name: "a"})
	graph.AddNode(&domain.ModuleNode{ID: "b", Name: "b"})
	graph.AddNode(&domain.ModuleNode{ID: "c", Name: "c"})
	graph.AddEdge(&domain.DependencyEdge{From: "a", To: "b", Weight: 1})
	graph.AddEdge(&domain.DependencyEdge{From: "b", To: "c", Weight: 1})

	calc := NewCouplingMetricsCalculator(nil)
	metrics := calc.CalculateMetrics(graph)
	analysis := calc.CalculateCouplingAnalysis(graph, metrics)

	if analysis == nil {
		t.Fatal("Expected analysis to not be nil")
	}

	// Check averages are calculated
	if analysis.AverageCoupling <= 0 {
		t.Error("Expected positive average coupling")
	}
	if analysis.AverageInstability <= 0 || analysis.AverageInstability > 1 {
		t.Errorf("Expected average instability between 0-1, got %f", analysis.AverageInstability)
	}
}

func TestCalculateTransitiveDependencies(t *testing.T) {
	// A -> B -> C -> D
	graph := domain.NewDependencyGraph()
	graph.AddNode(&domain.ModuleNode{ID: "a"})
	graph.AddNode(&domain.ModuleNode{ID: "b"})
	graph.AddNode(&domain.ModuleNode{ID: "c"})
	graph.AddNode(&domain.ModuleNode{ID: "d"})
	graph.AddEdge(&domain.DependencyEdge{From: "a", To: "b", Weight: 1})
	graph.AddEdge(&domain.DependencyEdge{From: "b", To: "c", Weight: 1})
	graph.AddEdge(&domain.DependencyEdge{From: "c", To: "d", Weight: 1})

	calc := NewCouplingMetricsCalculator(nil)
	transitive := calc.CalculateTransitiveDependencies("a", graph)

	if len(transitive) != 3 {
		t.Errorf("Expected 3 transitive dependencies, got %d", len(transitive))
	}

	// Should contain b, c, d
	expected := map[string]bool{"b": true, "c": true, "d": true}
	for _, dep := range transitive {
		if !expected[dep] {
			t.Errorf("Unexpected dependency: %s", dep)
		}
	}
}

func TestCalculateMaxDepth(t *testing.T) {
	// A -> B -> C -> D (depth 3)
	graph := domain.NewDependencyGraph()
	graph.AddNode(&domain.ModuleNode{ID: "a"})
	graph.AddNode(&domain.ModuleNode{ID: "b"})
	graph.AddNode(&domain.ModuleNode{ID: "c"})
	graph.AddNode(&domain.ModuleNode{ID: "d"})
	graph.AddEdge(&domain.DependencyEdge{From: "a", To: "b", Weight: 1})
	graph.AddEdge(&domain.DependencyEdge{From: "b", To: "c", Weight: 1})
	graph.AddEdge(&domain.DependencyEdge{From: "c", To: "d", Weight: 1})

	calc := NewCouplingMetricsCalculator(nil)
	depth, err := calc.CalculateMaxDepth(context.Background(), graph)
	if err != nil {
		t.Fatalf("CalculateMaxDepth: %v", err)
	}

	if depth != 3 {
		t.Errorf("Expected max depth 3, got %d", depth)
	}
}

func TestCalculateMaxDepthWithCycle(t *testing.T) {
	// A -> B -> C -> A (cycle)
	graph := domain.NewDependencyGraph()
	graph.AddNode(&domain.ModuleNode{ID: "a"})
	graph.AddNode(&domain.ModuleNode{ID: "b"})
	graph.AddNode(&domain.ModuleNode{ID: "c"})
	graph.AddEdge(&domain.DependencyEdge{From: "a", To: "b", Weight: 1})
	graph.AddEdge(&domain.DependencyEdge{From: "b", To: "c", Weight: 1})
	graph.AddEdge(&domain.DependencyEdge{From: "c", To: "a", Weight: 1})

	calc := NewCouplingMetricsCalculator(nil)
	depth, err := calc.CalculateMaxDepth(context.Background(), graph)
	if err != nil {
		t.Fatalf("CalculateMaxDepth: %v", err)
	}

	// Should handle cycle gracefully
	if depth < 0 {
		t.Error("Expected non-negative depth even with cycle")
	}
}

// TestCalculateMaxDepthExcludesDynamicImports pins depth to the same load-time
// contract cycle detection uses: a dynamic import() runs when it is called, so
// it cannot deepen the layering the modules are loaded in.
func TestCalculateMaxDepthExcludesDynamicImports(t *testing.T) {
	// a -> b statically, b -> c only through a dynamic import().
	graph := domain.NewDependencyGraph()
	graph.AddNode(&domain.ModuleNode{ID: "a"})
	graph.AddNode(&domain.ModuleNode{ID: "b"})
	graph.AddNode(&domain.ModuleNode{ID: "c"})
	graph.AddEdge(&domain.DependencyEdge{From: "a", To: "b", Weight: 1})
	graph.AddEdge(&domain.DependencyEdge{From: "b", To: "c", Weight: 1, EdgeType: domain.EdgeTypeDynamic})

	calc := NewCouplingMetricsCalculator(nil)

	if depth, err := calc.CalculateMaxDepth(context.Background(), graph); err != nil || depth != 1 {
		t.Errorf("the dynamic edge must not count toward depth: got %d, %v; want 1", depth, err)
	}

	// Promoting the same pair to a static import restores the layer.
	graph.AddEdge(&domain.DependencyEdge{From: "b", To: "c", Weight: 1})
	if depth, err := calc.CalculateMaxDepth(context.Background(), graph); err != nil || depth != 2 {
		t.Errorf("a static edge alongside the dynamic one is a load-time dependency: got %d, %v; want 2", depth, err)
	}
}

func TestGetCouplingBucket(t *testing.T) {
	calc := NewCouplingMetricsCalculator(nil)

	testCases := []struct {
		coupling int
		expected int
	}{
		{0, 0},
		{1, 3},
		{3, 3},
		{4, 7},
		{7, 7},
		{8, 10},
		{10, 10},
		{15, 11},
		{100, 11},
	}

	for _, tc := range testCases {
		result := calc.getCouplingBucket(tc.coupling)
		if result != tc.expected {
			t.Errorf("Bucket(%d) = %d, expected %d", tc.coupling, result, tc.expected)
		}
	}
}

func TestGetDirectDependencies(t *testing.T) {
	graph := domain.NewDependencyGraph()
	graph.AddNode(&domain.ModuleNode{ID: "a"})
	graph.AddNode(&domain.ModuleNode{ID: "b"})
	graph.AddNode(&domain.ModuleNode{ID: "c"})
	graph.AddEdge(&domain.DependencyEdge{From: "a", To: "b", Weight: 1})
	graph.AddEdge(&domain.DependencyEdge{From: "a", To: "c", Weight: 1})

	calc := NewCouplingMetricsCalculator(nil)
	deps := calc.getDirectDependencies("a", graph)

	if len(deps) != 2 {
		t.Errorf("Expected 2 direct dependencies, got %d", len(deps))
	}
}

func TestGetDependents(t *testing.T) {
	graph := domain.NewDependencyGraph()
	graph.AddNode(&domain.ModuleNode{ID: "a"})
	graph.AddNode(&domain.ModuleNode{ID: "b"})
	graph.AddNode(&domain.ModuleNode{ID: "c"})
	graph.AddEdge(&domain.DependencyEdge{From: "b", To: "a", Weight: 1})
	graph.AddEdge(&domain.DependencyEdge{From: "c", To: "a", Weight: 1})

	calc := NewCouplingMetricsCalculator(nil)
	deps := calc.getDependents("a", graph)

	if len(deps) != 2 {
		t.Errorf("Expected 2 dependents, got %d", len(deps))
	}
}

func TestCalculateAbstractness(t *testing.T) {
	calc := NewCouplingMetricsCalculator(nil)

	testCases := []struct {
		exports  []string
		expected float64
	}{
		{nil, 0.0},
		{[]string{}, 0.0},
		{[]string{"a"}, 0.1},
		{[]string{"a", "b", "c", "d", "e"}, 0.5},
		{[]string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"}, 1.0},
		{[]string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l"}, 1.0}, // Capped at 1.0
	}

	for _, tc := range testCases {
		node := &domain.ModuleNode{Exports: tc.exports}
		result := calc.calculateAbstractness(node)
		if math.Abs(result-tc.expected) > 0.001 {
			t.Errorf("Abstractness(exports=%d) = %f, expected %f", len(tc.exports), result, tc.expected)
		}
	}
}

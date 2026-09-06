package service

import (
	"context"
	"testing"

	"github.com/ludo-technologies/polyscan/polyscan/internal/js/domain"
)

func TestAnalyzeDependencyGraph(t *testing.T) {
	graph := domain.NewDependencyGraph()
	for _, id := range []string{"a", "b", "c", "d"} {
		graph.AddNode(&domain.ModuleNode{ID: id, Name: id, FilePath: id + ".js", Abstractness: 0.5})
	}
	for _, edge := range [][2]string{{"a", "b"}, {"b", "c"}, {"c", "b"}, {"c", "d"}} {
		graph.AddEdge(&domain.DependencyEdge{From: edge[0], To: edge[1], EdgeType: domain.EdgeTypeImport})
	}
	graph.UpdateNodeFlags()

	analysis, err := AnalyzeDependencyGraph(context.Background(), graph, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if analysis.TotalModules != 4 || analysis.TotalDependencies != 4 {
		t.Errorf("modules=%d dependencies=%d, want 4 and 4", analysis.TotalModules, analysis.TotalDependencies)
	}
	if analysis.CircularDependencies == nil || analysis.CircularDependencies.TotalModulesInCycles != 2 {
		t.Errorf("cycles = %+v, want b and c in one cycle", analysis.CircularDependencies)
	}
	// The chain routes through the cycle once: a -> b -> c -> d.
	if analysis.MaxDepth != 3 {
		t.Errorf("max depth = %d, want 3", analysis.MaxDepth)
	}
	if got := analysis.RootModules; len(got) != 1 || got[0] != "a" {
		t.Errorf("roots = %v, want [a]", got)
	}
	if m := analysis.ModuleMetrics["b"]; m == nil || m.AfferentCoupling != 2 || m.EfferentCoupling != 1 || m.Abstractness != 0.5 {
		t.Errorf("metrics for b = %+v", m)
	}

	analysis, err = AnalyzeDependencyGraph(context.Background(), graph, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if analysis.CircularDependencies != nil {
		t.Errorf("cycles = %+v, want none when detection is off", analysis.CircularDependencies)
	}
}

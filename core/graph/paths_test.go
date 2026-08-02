package graph

import (
	"fmt"
	"testing"
)

func chainGraph(edges [][2]string, isolated ...string) *MapGraph {
	g := NewMapGraph()
	for _, node := range isolated {
		g.AddNode(node)
	}
	for _, edge := range edges {
		g.AddEdge(edge[0], edge[1])
	}
	return g
}

// isSimplePath verifies every step follows an edge and no node repeats.
func isSimplePath(g DirectedGraph, path []string) error {
	seen := make(map[string]struct{}, len(path))
	for i, node := range path {
		if !g.HasNode(node) {
			return fmt.Errorf("node %q is not in the graph", node)
		}
		if _, duplicate := seen[node]; duplicate {
			return fmt.Errorf("node %q repeats in the path", node)
		}
		seen[node] = struct{}{}

		if i == 0 {
			continue
		}
		linked := false
		for _, successor := range g.Successors(path[i-1]) {
			if successor == node {
				linked = true
				break
			}
		}
		if !linked {
			return fmt.Errorf("no edge from %q to %q", path[i-1], node)
		}
	}
	return nil
}

func TestLongestChainEmptyGraph(t *testing.T) {
	if chain := NewChainFinder(NewMapGraph()).LongestChain(); chain != nil {
		t.Errorf("LongestChain(empty) = %v, want nil", chain)
	}
	if chain := NewChainFinder(nil).LongestChain(); chain != nil {
		t.Errorf("NewChainFinder(nil).LongestChain() = %v, want nil", chain)
	}
}

func TestLongestChainSingleNode(t *testing.T) {
	chain := NewChainFinder(chainGraph(nil, "only")).LongestChain()
	if len(chain) != 1 || chain[0] != "only" {
		t.Errorf("LongestChain = %v, want [only]", chain)
	}
}

func TestLongestChainLinearPath(t *testing.T) {
	g := chainGraph([][2]string{{"a", "b"}, {"b", "c"}, {"c", "d"}})

	chain := NewChainFinder(g).LongestChain()
	want := []string{"a", "b", "c", "d"}
	if len(chain) != len(want) {
		t.Fatalf("LongestChain = %v, want %v", chain, want)
	}
	for i := range want {
		if chain[i] != want[i] {
			t.Fatalf("LongestChain = %v, want %v", chain, want)
		}
	}
}

func TestLongestChainPrefersDeeperBranch(t *testing.T) {
	// "a" branches into a short arm and a long arm.
	g := chainGraph([][2]string{
		{"a", "short"},
		{"a", "long1"}, {"long1", "long2"}, {"long2", "long3"},
	})

	chain := NewChainFinder(g).LongestChain()
	if err := isSimplePath(g, chain); err != nil {
		t.Fatalf("chain %v is not a simple path: %v", chain, err)
	}
	if len(chain) != 4 || chain[0] != "a" || chain[3] != "long3" {
		t.Errorf("LongestChain = %v, want a→long1→long2→long3", chain)
	}
}

func TestLongestChainRoutesThroughCycle(t *testing.T) {
	// entry → (x ⇄ y ⇄ z cycle) → exit
	g := chainGraph([][2]string{
		{"entry", "x"},
		{"x", "y"}, {"y", "z"}, {"z", "x"},
		{"z", "exit"},
	})

	chain := NewChainFinder(g).LongestChain()
	if err := isSimplePath(g, chain); err != nil {
		t.Fatalf("chain %v is not a simple path: %v", chain, err)
	}
	if chain[0] != "entry" || chain[len(chain)-1] != "exit" {
		t.Errorf("LongestChain = %v, want a chain from entry to exit", chain)
	}
}

func TestLongestChainWholeGraphIsOneCycle(t *testing.T) {
	g := chainGraph([][2]string{{"a", "b"}, {"b", "c"}, {"c", "a"}})

	chain := NewChainFinder(g).LongestChain()
	if err := isSimplePath(g, chain); err != nil {
		t.Fatalf("chain %v is not a simple path: %v", chain, err)
	}
	if len(chain) != 1 {
		t.Errorf("LongestChain = %v, want a single component representative", chain)
	}
}

func TestLongestChainIsDeterministic(t *testing.T) {
	g := chainGraph([][2]string{
		{"a", "b"}, {"a", "c"}, {"b", "d"}, {"c", "d"}, {"d", "e"},
		{"e", "f"}, {"f", "d"}, {"e", "g"},
	})

	first := NewChainFinder(g).LongestChain()
	for i := 0; i < 20; i++ {
		next := NewChainFinder(g).LongestChain()
		if len(next) != len(first) {
			t.Fatalf("run %d returned %v, want %v", i, next, first)
		}
		for j := range first {
			if next[j] != first[j] {
				t.Fatalf("run %d returned %v, want %v", i, next, first)
			}
		}
	}
}

// TestLongestChainCompletesOnDenseGraph pins the complexity guarantee: an
// exhaustive simple-path search over this graph would not terminate.
func TestLongestChainCompletesOnDenseGraph(t *testing.T) {
	const nodes = 300

	g := NewMapGraph()
	for i := 0; i < nodes; i++ {
		for j := i + 1; j < nodes && j < i+8; j++ {
			g.AddEdge(fmt.Sprintf("n%03d", i), fmt.Sprintf("n%03d", j))
		}
	}

	chain := NewChainFinder(g).LongestChain()
	if err := isSimplePath(g, chain); err != nil {
		t.Fatalf("chain is not a simple path: %v", err)
	}
	if len(chain) < 2 {
		t.Errorf("LongestChain returned %d nodes, want a multi-node chain", len(chain))
	}
}

func TestLongestChainFromStartsAtRequestedNode(t *testing.T) {
	g := chainGraph([][2]string{
		{"a", "b"}, {"b", "c"}, {"c", "d"},
		{"x", "b"},
	})
	finder := NewChainFinder(g)

	chain := finder.LongestChainFrom("x")
	if err := isSimplePath(g, chain); err != nil {
		t.Fatalf("chain %v is not a simple path: %v", chain, err)
	}
	if len(chain) == 0 || chain[0] != "x" {
		t.Fatalf("LongestChainFrom(x) = %v, want a chain starting at x", chain)
	}
	if chain[len(chain)-1] != "d" {
		t.Errorf("LongestChainFrom(x) = %v, want it to reach d", chain)
	}

	if chain := finder.LongestChainFrom("missing"); chain != nil {
		t.Errorf("LongestChainFrom(missing) = %v, want nil", chain)
	}
}

func TestLongestChainFromInsideCycleStaysSimple(t *testing.T) {
	g := chainGraph([][2]string{
		{"x", "y"}, {"y", "z"}, {"z", "x"},
		{"z", "tail"}, {"tail", "end"},
	})

	chain := NewChainFinder(g).LongestChainFrom("y")
	if err := isSimplePath(g, chain); err != nil {
		t.Fatalf("chain %v is not a simple path: %v", chain, err)
	}
	if chain[0] != "y" || chain[len(chain)-1] != "end" {
		t.Errorf("LongestChainFrom(y) = %v, want a chain from y to end", chain)
	}
}

func TestStronglyConnectedComponentsIncludesSingletons(t *testing.T) {
	g := chainGraph([][2]string{{"a", "b"}, {"b", "a"}, {"b", "c"}})

	components := StronglyConnectedComponents(g)
	if len(components) != 2 {
		t.Fatalf("got %d components (%v), want 2", len(components), components)
	}

	sizes := map[int]int{}
	for _, component := range components {
		sizes[len(component)]++
	}
	if sizes[1] != 1 || sizes[2] != 1 {
		t.Errorf("component sizes = %v, want one singleton and one pair", sizes)
	}
}

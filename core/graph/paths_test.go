package graph

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
)

func mustChainFinder(t *testing.T, g DirectedGraph) *ChainFinder {
	t.Helper()
	finder, err := NewChainFinder(context.Background(), g)
	if err != nil {
		t.Fatalf("NewChainFinder: %v", err)
	}
	return finder
}

func mustLongestChains(t *testing.T, g DirectedGraph, limit int) [][]string {
	t.Helper()
	chains, err := mustChainFinder(t, g).LongestChains(context.Background(), limit)
	if err != nil {
		t.Fatalf("LongestChains(%d): %v", limit, err)
	}
	return chains
}

func cancelledContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

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
	if chain := mustChainFinder(t, NewMapGraph()).LongestChain(); chain != nil {
		t.Errorf("LongestChain(empty) = %v, want nil", chain)
	}
	if chain := mustChainFinder(t, nil).LongestChain(); chain != nil {
		t.Errorf("NewChainFinder(nil).LongestChain() = %v, want nil", chain)
	}
}

func TestLongestChainSingleNode(t *testing.T) {
	chain := mustChainFinder(t, chainGraph(nil, "only")).LongestChain()
	if len(chain) != 1 || chain[0] != "only" {
		t.Errorf("LongestChain = %v, want [only]", chain)
	}
}

func TestLongestChainLinearPath(t *testing.T) {
	g := chainGraph([][2]string{{"a", "b"}, {"b", "c"}, {"c", "d"}})

	chain := mustChainFinder(t, g).LongestChain()
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

	chain := mustChainFinder(t, g).LongestChain()
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

	chain := mustChainFinder(t, g).LongestChain()
	if err := isSimplePath(g, chain); err != nil {
		t.Fatalf("chain %v is not a simple path: %v", chain, err)
	}
	if chain[0] != "entry" || chain[len(chain)-1] != "exit" {
		t.Errorf("LongestChain = %v, want a chain from entry to exit", chain)
	}
}

func TestLongestChainWholeGraphIsOneCycle(t *testing.T) {
	g := chainGraph([][2]string{{"a", "b"}, {"b", "c"}, {"c", "a"}})

	chain := mustChainFinder(t, g).LongestChain()
	if err := isSimplePath(g, chain); err != nil {
		t.Fatalf("chain %v is not a simple path: %v", chain, err)
	}
	// A graph that is nothing but a cycle still has depth: the chain must walk
	// the cycle's members rather than collapse to whichever node it started on.
	if len(chain) != 3 {
		t.Errorf("LongestChain = %v, want all three cycle members", chain)
	}
}

// TestLongestChainWalksTerminalCycle covers the shape that made chains collapse:
// the chain ends inside a cycle, so nothing constrains where the route stops.
func TestLongestChainWalksTerminalCycle(t *testing.T) {
	g := chainGraph([][2]string{
		{"entry", "x"},
		{"x", "y"}, {"y", "x"},
	})

	chain := mustChainFinder(t, g).LongestChain()
	if err := isSimplePath(g, chain); err != nil {
		t.Fatalf("chain %v is not a simple path: %v", chain, err)
	}
	if len(chain) != 3 {
		t.Errorf("LongestChain = %v, want entry plus both cycle members", chain)
	}
	if chain[0] != "entry" {
		t.Errorf("LongestChain = %v, want it to start at entry", chain)
	}
}

// TestLongestChainPrefersChainThroughCycle pins the weighting: two chains cross
// the same number of components, but one of those components is a cycle holding
// more modules, so it is the deeper dependency chain.
func TestLongestChainPrefersChainThroughCycle(t *testing.T) {
	g := chainGraph([][2]string{
		{"root", "plain"}, {"plain", "leaf"},
		{"root", "c1"}, {"c1", "c2"}, {"c2", "c3"}, {"c3", "c1"},
	})

	chain := mustChainFinder(t, g).LongestChain()
	if err := isSimplePath(g, chain); err != nil {
		t.Fatalf("chain %v is not a simple path: %v", chain, err)
	}
	if len(chain) != 4 {
		t.Errorf("LongestChain = %v, want root plus all three cycle members", chain)
	}
}

func TestLongestChainIsDeterministic(t *testing.T) {
	g := chainGraph([][2]string{
		{"a", "b"}, {"a", "c"}, {"b", "d"}, {"c", "d"}, {"d", "e"},
		{"e", "f"}, {"f", "d"}, {"e", "g"},
	})

	first := mustChainFinder(t, g).LongestChain()
	for i := 0; i < 20; i++ {
		next := mustChainFinder(t, g).LongestChain()
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

	chain := mustChainFinder(t, g).LongestChain()
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
	finder := mustChainFinder(t, g)

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

	chain := mustChainFinder(t, g).LongestChainFrom("y")
	if err := isSimplePath(g, chain); err != nil {
		t.Fatalf("chain %v is not a simple path: %v", chain, err)
	}
	if chain[0] != "y" || chain[len(chain)-1] != "end" {
		t.Errorf("LongestChainFrom(y) = %v, want a chain from y to end", chain)
	}
}

func TestNewChainFinderHonorsCancellation(t *testing.T) {
	g := chainGraph([][2]string{{"a", "b"}})

	finder, err := NewChainFinder(cancelledContext(), g)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("NewChainFinder(cancelled) = %v, %v; want context.Canceled", finder, err)
	}
}

func TestLongestChainsHonorsCancellation(t *testing.T) {
	finder := mustChainFinder(t, chainGraph([][2]string{{"a", "b"}}))

	chains, err := finder.LongestChains(cancelledContext(), 1)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("LongestChains(cancelled) = %v, %v; want context.Canceled", chains, err)
	}
}

func TestLongestChainsRanksEqualChainsByName(t *testing.T) {
	// entry reaches leaf through two arms of equal depth.
	g := chainGraph([][2]string{
		{"entry", "left"}, {"entry", "right"},
		{"left", "leaf"}, {"right", "leaf"},
	})

	want := [][]string{{"entry", "left", "leaf"}, {"entry", "right", "leaf"}}
	if got := mustLongestChains(t, g, 2); !reflect.DeepEqual(got, want) {
		t.Errorf("LongestChains(2) = %v, want %v", got, want)
	}
}

// TestLongestChainsRanksDeepBranchAboveShallowOnes pins that weight comes
// before name: many lexicographically earlier shallow arms must not push the
// one deep arm out of the ranking.
func TestLongestChainsRanksDeepBranchAboveShallowOnes(t *testing.T) {
	edges := [][2]string{
		{"root", "z_deep"}, {"z_deep", "z_middle"}, {"z_middle", "z_leaf"},
	}
	for i := 0; i < 10; i++ {
		edges = append(edges, [2]string{"root", fmt.Sprintf("a_shallow_%02d", i)})
	}
	g := chainGraph(edges)

	chains := mustLongestChains(t, g, 10)
	want := []string{"root", "z_deep", "z_middle", "z_leaf"}
	if len(chains) == 0 || !reflect.DeepEqual(chains[0], want) {
		t.Errorf("LongestChains(10)[0] = %v, want %v", chains, want)
	}
}

// TestLongestChainsIncludesTails pins that chains may start anywhere: the tail
// of the best chain is itself a chain, ranked below it.
func TestLongestChainsIncludesTails(t *testing.T) {
	g := chainGraph([][2]string{{"a", "b"}, {"b", "c"}, {"c", "d"}})

	want := [][]string{{"a", "b", "c", "d"}, {"b", "c", "d"}, {"c", "d"}}
	if got := mustLongestChains(t, g, 10); !reflect.DeepEqual(got, want) {
		t.Errorf("LongestChains(10) = %v, want %v", got, want)
	}
}

// TestLongestChainsKeepsSeveralChainsPerComponent pins the per-component
// ranking: every chain from root shares the start component, and the global
// top list must hold all of them, ordered by the components they cross, before
// any lighter tail.
func TestLongestChainsKeepsSeveralChainsPerComponent(t *testing.T) {
	g := chainGraph([][2]string{
		{"root", "a"}, {"root", "b"},
		{"a", "c"}, {"a", "d"}, {"b", "c"}, {"b", "d"},
	})

	want := [][]string{
		{"root", "a", "c"}, {"root", "a", "d"}, {"root", "b", "c"}, {"root", "b", "d"},
		{"a", "c"}, {"a", "d"},
	}
	if got := mustLongestChains(t, g, 6); !reflect.DeepEqual(got, want) {
		t.Errorf("LongestChains(6) = %v, want %v", got, want)
	}
}

func TestLongestChainsExpandsCycleComponents(t *testing.T) {
	g := chainGraph([][2]string{
		{"entry", "first_a"}, {"first_a", "first_b"}, {"first_b", "first_a"},
		{"first_b", "second_a"}, {"second_a", "second_b"}, {"second_b", "second_a"},
		{"second_b", "leaf"},
	})

	chains := mustLongestChains(t, g, 1)
	want := [][]string{{"entry", "first_a", "first_b", "second_a", "second_b", "leaf"}}
	if !reflect.DeepEqual(chains, want) {
		t.Errorf("LongestChains(1) = %v, want %v", chains, want)
	}
	if err := isSimplePath(g, chains[0]); err != nil {
		t.Errorf("chain %v is not a simple path: %v", chains[0], err)
	}
}

func TestLongestChainsAgreesWithLongestChain(t *testing.T) {
	g := chainGraph([][2]string{
		{"a", "b"}, {"a", "c"}, {"b", "d"}, {"c", "d"}, {"d", "e"},
		{"e", "f"}, {"f", "d"}, {"e", "g"},
	})
	finder := mustChainFinder(t, g)

	chains, err := finder.LongestChains(context.Background(), 1)
	if err != nil {
		t.Fatalf("LongestChains(1): %v", err)
	}
	if len(chains) != 1 || !reflect.DeepEqual(chains[0], finder.LongestChain()) {
		t.Errorf("LongestChains(1) = %v, want [%v]", chains, finder.LongestChain())
	}
}

func TestLongestChainsWithoutLimitOrEdges(t *testing.T) {
	if chains := mustLongestChains(t, chainGraph([][2]string{{"a", "b"}}), 0); chains != nil {
		t.Errorf("LongestChains(0) = %v, want nil", chains)
	}
	if chains := mustLongestChains(t, NewMapGraph(), 3); chains != nil {
		t.Errorf("LongestChains(empty graph) = %v, want nil", chains)
	}
	// Isolated nodes are not chains, even when the limit has room for them.
	if chains := mustLongestChains(t, chainGraph(nil, "only", "other"), 3); len(chains) != 0 {
		t.Errorf("LongestChains(isolated nodes) = %v, want none", chains)
	}
}

// TestLongestChainsWholeGraphIsOneCycle pins that a cycle counts as a chain
// even though it is a single component: its members depend on each other.
func TestLongestChainsWholeGraphIsOneCycle(t *testing.T) {
	g := chainGraph([][2]string{{"a", "b"}, {"b", "a"}}, "lonely")

	want := [][]string{{"a", "b"}}
	if got := mustLongestChains(t, g, 3); !reflect.DeepEqual(got, want) {
		t.Errorf("LongestChains(3) = %v, want %v", got, want)
	}
}

func TestStronglyConnectedComponentsHonorsCancellation(t *testing.T) {
	g := chainGraph([][2]string{{"a", "b"}})

	components, err := StronglyConnectedComponents(cancelledContext(), g)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("StronglyConnectedComponents(cancelled) = %v, %v; want context.Canceled", components, err)
	}
}

// TestStronglyConnectedComponentsAreInReverseTopologicalOrder pins the
// ordering contract the chain finder relies on: every component appears before
// any component it depends on, so one forward pass sees each component's
// successors already ranked.
func TestStronglyConnectedComponentsAreInReverseTopologicalOrder(t *testing.T) {
	g := chainGraph([][2]string{
		{"a", "b"}, {"b", "c"}, {"c", "b"}, {"c", "d"}, {"x", "d"}, {"x", "a"},
	})

	components, err := StronglyConnectedComponents(context.Background(), g)
	if err != nil {
		t.Fatalf("StronglyConnectedComponents: %v", err)
	}
	position := make(map[string]int)
	for index, component := range components {
		for _, node := range component {
			position[node] = index
		}
	}
	for _, from := range g.NodeIDs() {
		for _, to := range g.Successors(from) {
			if position[to] > position[from] {
				t.Errorf("component of %q (%d) comes after component of %q (%d)", to, position[to], from, position[from])
			}
		}
	}
}

func TestStronglyConnectedComponentsIncludesSingletons(t *testing.T) {
	g := chainGraph([][2]string{{"a", "b"}, {"b", "a"}, {"b", "c"}})

	components, err := StronglyConnectedComponents(context.Background(), g)
	if err != nil {
		t.Fatalf("StronglyConnectedComponents: %v", err)
	}
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

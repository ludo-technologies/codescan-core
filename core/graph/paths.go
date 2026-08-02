package graph

// noComponent marks the absence of a component index.
const noComponent = -1

// ChainFinder answers longest-dependency-chain queries over a directed graph.
//
// Finding the longest simple path in a graph that may contain cycles is
// NP-hard, so a ChainFinder walks the condensation instead: strongly connected
// components are collapsed to single vertices and the longest path through the
// resulting DAG is memoized in one linear pass. Each query then expands the
// component path into a concrete route through the members of every component
// it crosses, so every chain returned is a real simple path in the graph.
//
// Chains are compared by the number of components they cross. Ties are resolved
// by traversal order, which is derived from the graph's node ordering, so
// repeated queries over the same graph return the same chain.
type ChainFinder struct {
	dag  *condensation
	best []componentChain
}

// NewChainFinder builds the condensation of g and memoizes the longest chain
// reachable from every component. Construction is linear in the graph size;
// each subsequent query is linear in the length of the chain it returns.
func NewChainFinder(g DirectedGraph) *ChainFinder {
	if g == nil || g.NodeCount() == 0 {
		return &ChainFinder{}
	}

	dag := newCondensation(g)
	return &ChainFinder{dag: dag, best: dag.longestChainPerComponent()}
}

// LongestChainFrom returns the longest chain that starts at nodeID, or nil if
// the node is not in the graph.
func (f *ChainFinder) LongestChainFrom(nodeID string) []string {
	if f.dag == nil {
		return nil
	}
	start, known := f.dag.componentOf[nodeID]
	if !known {
		return nil
	}
	return f.dag.expand(f.componentPathFrom(start), nodeID)
}

// LongestChain returns the longest chain anywhere in the graph.
func (f *ChainFinder) LongestChain() []string {
	if f.dag == nil {
		return nil
	}

	start, depth := noComponent, 0
	for index, chain := range f.best {
		if chain.depth > depth {
			start, depth = index, chain.depth
		}
	}
	if start == noComponent {
		return nil
	}
	return f.dag.expand(f.componentPathFrom(start), "")
}

func (f *ChainFinder) componentPathFrom(start int) []int {
	path := make([]int, 0, f.best[start].depth)
	for index := start; index != noComponent; index = f.best[index].next {
		path = append(path, index)
	}
	return path
}

// condensation is the DAG of strongly connected components of a graph.
type condensation struct {
	graph      DirectedGraph
	components [][]string
	// componentOf maps a node ID to the index of its component.
	componentOf map[string]int
	// successors lists the component indices each component depends on.
	successors [][]int
	// crossing records, for an ordered component pair, one concrete edge
	// realizing it: crossing[[2]int{from, to}] = [2]string{fromNode, toNode}.
	crossing map[[2]int][2]string
}

func newCondensation(g DirectedGraph) *condensation {
	components := StronglyConnectedComponents(g)

	dag := &condensation{
		graph:       g,
		components:  components,
		componentOf: make(map[string]int, g.NodeCount()),
		successors:  make([][]int, len(components)),
		crossing:    make(map[[2]int][2]string),
	}
	for index, component := range components {
		for _, nodeID := range component {
			dag.componentOf[nodeID] = index
		}
	}

	for index, component := range components {
		seen := make(map[int]struct{})
		for _, nodeID := range component {
			for _, successor := range g.Successors(nodeID) {
				target, known := dag.componentOf[successor]
				if !known || target == index {
					continue
				}
				if _, duplicate := seen[target]; duplicate {
					continue
				}
				seen[target] = struct{}{}
				dag.successors[index] = append(dag.successors[index], target)
				dag.crossing[[2]int{index, target}] = [2]string{nodeID, successor}
			}
		}
	}

	return dag
}

// componentChain is the best chain reachable from one component: how many
// components it spans and which component continues it.
type componentChain struct {
	depth int
	next  int
}

// longestChainPerComponent memoizes, for every component, the longest chain
// starting there. The condensation is acyclic, so no component is reachable
// from itself and a single memoized pass suffices.
func (dag *condensation) longestChainPerComponent() []componentChain {
	best := make([]componentChain, len(dag.components))
	resolved := make([]bool, len(dag.components))

	var longestFrom func(index int) componentChain
	longestFrom = func(index int) componentChain {
		if resolved[index] {
			return best[index]
		}
		resolved[index] = true

		chain := componentChain{depth: 1, next: noComponent}
		for _, successor := range dag.successors[index] {
			if candidate := longestFrom(successor); candidate.depth+1 > chain.depth {
				chain = componentChain{depth: candidate.depth + 1, next: successor}
			}
		}

		best[index] = chain
		return chain
	}

	for index := range dag.components {
		longestFrom(index)
	}
	return best
}

// expand turns a component path into a concrete node path, routing through the
// members of every component the chain crosses. The path begins at startNode
// when given, and otherwise at the first component's first member.
func (dag *condensation) expand(componentPath []int, startNode string) []string {
	if len(componentPath) == 0 {
		return nil
	}

	var path []string
	entry := startNode
	for position, index := range componentPath {
		exit := ""
		nextEntry := ""
		if position+1 < len(componentPath) {
			edge := dag.crossing[[2]int{index, componentPath[position+1]}]
			exit, nextEntry = edge[0], edge[1]
		}

		path = append(path, dag.routeWithin(index, entry, exit)...)
		entry = nextEntry
	}

	return path
}

// routeWithin returns a simple path inside one component that starts at entry
// (or the component's first node when entry is empty) and ends at exit (or at
// entry itself when exit is empty). Every node of a strongly connected
// component is reachable from every other, so a route always exists.
func (dag *condensation) routeWithin(index int, entry, exit string) []string {
	component := dag.components[index]
	if entry == "" {
		entry = component[0]
	}
	if exit == "" || exit == entry {
		return []string{entry}
	}

	members := make(map[string]struct{}, len(component))
	for _, nodeID := range component {
		members[nodeID] = struct{}{}
	}

	// Breadth-first search stays inside the component, so the route never
	// revisits a node already contributed by an earlier component.
	previous := map[string]string{entry: ""}
	queue := []string{entry}
	for len(queue) > 0 {
		if _, reached := previous[exit]; reached {
			break
		}
		current := queue[0]
		queue = queue[1:]
		for _, successor := range dag.graph.Successors(current) {
			if _, inside := members[successor]; !inside {
				continue
			}
			if _, seen := previous[successor]; seen {
				continue
			}
			previous[successor] = current
			queue = append(queue, successor)
		}
	}

	route := []string{}
	for node := exit; node != ""; node = previous[node] {
		route = append(route, node)
	}
	for left, right := 0, len(route)-1; left < right; left, right = left+1, right-1 {
		route[left], route[right] = route[right], route[left]
	}
	return route
}

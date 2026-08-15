package graph

import (
	"context"
	"sort"
)

// ChainFinder answers longest-dependency-chain queries over a directed graph.
//
// Finding the longest simple path in a graph that may contain cycles is
// NP-hard, so a ChainFinder walks the condensation instead: strongly connected
// components are collapsed to single vertices and the heaviest chains through
// the resulting DAG are memoized in one linear pass, weighting each component
// by how many nodes it holds. Each query then expands a component chain into a
// concrete route through the members of every component it crosses, so every
// chain returned is a real simple path in the graph.
//
// A chain is maximal in dependency layers: no other simple path crosses more
// strongly connected components. Within a component the route is not guaranteed
// maximal — the final component is walked greedily through as many members as
// it can reach, while components the chain passes through are crossed by the
// shortest route between the edges that enter and leave them. Recovering the
// longest route through a cycle is the NP-hard problem again.
//
// Chains are ranked by weight (the node count of the components they cross)
// and, between equally heavy chains, by the lexicographic order of the
// components they cross, each component named by its smallest member. That
// order depends only on the graph, not on traversal, so repeated queries over
// the same graph return the same chains.
type ChainFinder struct {
	dag *condensation
	// best is the top-ranked chain starting at each component.
	best []*rankedChain
}

// NewChainFinder builds the condensation of g and memoizes the best chain
// reachable from every component. Construction is linear in the graph size;
// each subsequent query is linear in the length of the chain it returns.
//
// The condensation and the memoizing pass check ctx as they go and return
// ctx.Err() as soon as the context is cancelled.
func NewChainFinder(ctx context.Context, g DirectedGraph) (*ChainFinder, error) {
	if g == nil || g.NodeCount() == 0 {
		return &ChainFinder{}, nil
	}

	dag, err := newCondensation(ctx, g)
	if err != nil {
		return nil, err
	}
	ranked, err := dag.rankedChainsPerComponent(ctx, 1)
	if err != nil {
		return nil, err
	}
	best := make([]*rankedChain, len(ranked))
	for index, chains := range ranked {
		best[index] = chains[0]
	}
	return &ChainFinder{dag: dag, best: best}, nil
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
	return f.dag.expand(f.best[start], nodeID)
}

// LongestChain returns the longest chain anywhere in the graph.
func (f *ChainFinder) LongestChain() []string {
	if f.dag == nil {
		return nil
	}

	var top *rankedChain
	for _, chain := range f.best {
		if top == nil || f.dag.chainLess(chain, top) {
			top = chain
		}
	}
	return f.dag.expand(top, "")
}

// LongestChains returns the limit best-ranked chains in the graph, best first.
// Chains may start anywhere, so a reported chain can be the tail of another.
// Every chain contains at least one edge: a node that depends on nothing is
// not a chain, so a graph without edges has none. A limit of zero or less
// returns nil.
//
// The ranking pass is linear in the graph size times limit and checks ctx as it
// goes, returning ctx.Err() as soon as the context is cancelled.
func (f *ChainFinder) LongestChains(ctx context.Context, limit int) ([][]string, error) {
	if f.dag == nil || limit <= 0 {
		return nil, nil
	}

	ranked, err := f.dag.rankedChainsPerComponent(ctx, limit)
	if err != nil {
		return nil, err
	}

	var candidates []*rankedChain
	for _, chains := range ranked {
		for _, chain := range chains {
			// A lone node weighs 1; anything heavier crosses an edge, either
			// into another component or around its own cycle.
			if chain.nodes > 1 {
				candidates = append(candidates, chain)
			}
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		return f.dag.chainLess(candidates[i], candidates[j])
	})
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}

	chains := make([][]string, len(candidates))
	for index, candidate := range candidates {
		chains[index] = f.dag.expand(candidate, "")
	}
	return chains, nil
}

// condensation is the DAG of strongly connected components of a graph.
type condensation struct {
	graph DirectedGraph
	// components lists every component's members in sorted order. Components
	// come in reverse topological order, so every component a chain continues
	// into has a smaller index than the component it leaves.
	components [][]string
	// componentOf maps a node ID to the index of its component.
	componentOf map[string]int
	// successors lists the component indices each component depends on.
	successors [][]int
	// crossing records, for an ordered component pair, one concrete edge
	// realizing it: crossing[[2]int{from, to}] = [2]string{fromNode, toNode}.
	crossing map[[2]int][2]string
}

func newCondensation(ctx context.Context, g DirectedGraph) (*condensation, error) {
	components, err := StronglyConnectedComponents(ctx, g)
	if err != nil {
		return nil, err
	}
	// Sort each component's members so a chain that starts inside a cycle starts
	// at a predictable node rather than wherever Tarjan happened to pop it, and
	// so component[0] names the component when chains are compared. These
	// slices are this call's own; CycleDetector runs its own SCC pass, so its
	// reported cycle order is untouched.
	for _, component := range components {
		sort.Strings(component)
	}

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
		if err := ctx.Err(); err != nil {
			return nil, err
		}
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

	return dag, nil
}

// rankedChain is one chain through the condensation, stored as a linked list
// of components so that chains sharing a tail share its storage.
type rankedChain struct {
	component int
	// next continues the chain, or is nil when the chain ends here.
	next *rankedChain
	// nodes is the chain's weight: the node count of every component it crosses.
	nodes int
	// rank is the chain's position among the ranked chains starting at the same
	// component, which lets two chains be compared without walking them.
	rank int
}

// rankedChainsPerComponent memoizes, for every component, the limit best-ranked
// chains starting there. A component with no successors starts exactly one
// chain, itself; every other component's chains extend the memoized chains of
// its successors, so a chain is only ever a top-limit chain from its start when
// its tail is a top-limit chain from the component that follows. Components
// come in reverse topological order, so each one is ranked after every
// component it can continue into.
func (dag *condensation) rankedChainsPerComponent(ctx context.Context, limit int) ([][]*rankedChain, error) {
	ranked := make([][]*rankedChain, len(dag.components))
	for index, component := range dag.components {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		size := len(component)
		var candidates []*rankedChain
		if len(dag.successors[index]) == 0 {
			candidates = []*rankedChain{{component: index, nodes: size}}
		}
		for _, successor := range dag.successors[index] {
			for _, tail := range ranked[successor] {
				candidates = append(candidates, &rankedChain{
					component: index,
					next:      tail,
					nodes:     size + tail.nodes,
				})
			}
		}

		sort.Slice(candidates, func(i, j int) bool {
			return dag.chainLess(candidates[i], candidates[j])
		})
		if len(candidates) > limit {
			candidates = candidates[:limit]
		}
		for rank, chain := range candidates {
			chain.rank = rank
		}
		ranked[index] = candidates
	}
	return ranked, nil
}

// chainLess orders chains best first: heavier chains before lighter ones, then
// by the lexicographic order of the components crossed, each named by its
// smallest member. Two chains that leave the same start component through the
// same next component are ordered by that tail's rank, which was assigned by
// this same order, so the comparison never walks a chain.
func (dag *condensation) chainLess(left, right *rankedChain) bool {
	if left.nodes != right.nodes {
		return left.nodes > right.nodes
	}
	if left.component != right.component {
		return dag.components[left.component][0] < dag.components[right.component][0]
	}
	if left.next == nil || right.next == nil {
		return false
	}
	if left.next.component != right.next.component {
		return dag.components[left.next.component][0] < dag.components[right.next.component][0]
	}
	return left.next.rank < right.next.rank
}

// expand turns a component chain into a concrete node path, routing through the
// members of every component the chain crosses. The path begins at startNode
// when given, and otherwise at the first component's first member.
func (dag *condensation) expand(chain *rankedChain, startNode string) []string {
	var path []string
	entry := startNode
	for ; chain != nil; chain = chain.next {
		exit := ""
		nextEntry := ""
		if chain.next != nil {
			edge := dag.crossing[[2]int{chain.component, chain.next.component}]
			exit, nextEntry = edge[0], edge[1]
		}

		path = append(path, dag.routeWithin(chain.component, entry, exit)...)
		entry = nextEntry
	}

	return path
}

// routeWithin returns a simple path inside one component, starting at entry (or
// the component's first node when entry is empty).
//
// When exit is set the route ends there, by the shortest way through the
// component. When it is empty — the component ends the chain, so nothing
// constrains where the route stops — the route walks greedily through as many
// members as it can reach. That greedy walk is what keeps a chain ending in a
// cycle from collapsing to the single node it entered on.
func (dag *condensation) routeWithin(index int, entry, exit string) []string {
	component := dag.components[index]
	if entry == "" {
		entry = component[0]
	}

	members := make(map[string]struct{}, len(component))
	for _, nodeID := range component {
		members[nodeID] = struct{}{}
	}

	if exit == "" {
		return dag.walkWithin(entry, members)
	}
	if exit == entry {
		return []string{entry}
	}
	return dag.shortestRouteWithin(entry, exit, members)
}

// walkWithin follows unvisited successors from entry for as long as it can,
// staying inside members. Every step follows a real edge to a node not yet on
// the path, so the result is a simple path; it is not guaranteed to be the
// longest one, which is NP-hard to find.
func (dag *condensation) walkWithin(entry string, members map[string]struct{}) []string {
	path := []string{entry}
	visited := map[string]struct{}{entry: {}}

	for current := entry; ; {
		next := ""
		for _, successor := range dag.graph.Successors(current) {
			if _, inside := members[successor]; !inside {
				continue
			}
			if _, seen := visited[successor]; seen {
				continue
			}
			next = successor
			break
		}
		if next == "" {
			return path
		}
		visited[next] = struct{}{}
		path = append(path, next)
		current = next
	}
}

// shortestRouteWithin returns the shortest path from entry to exit that stays
// inside members. Every node of a strongly connected component is reachable
// from every other, so a route always exists.
func (dag *condensation) shortestRouteWithin(entry, exit string, members map[string]struct{}) []string {
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

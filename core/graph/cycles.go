package graph

import "context"

// CycleResult holds the result of cycle detection via Tarjan's SCC algorithm.
type CycleResult struct {
	// Cycles contains all strongly connected components with more than one node.
	Cycles [][]string
	// HasCycles is true if any cycle was found.
	HasCycles bool
	// AffectedNodes contains all nodes that participate in at least one cycle.
	AffectedNodes map[string]bool
}

// CycleDetector finds strongly connected components using Tarjan's algorithm.
type CycleDetector struct {
	ctx      context.Context
	index    int
	stack    []string
	onStack  map[string]bool
	indices  map[string]int
	lowlinks map[string]int
	// collect receives every completed component, including single-node ones.
	collect func(scc []string)
}

// NewCycleDetector creates a new CycleDetector.
func NewCycleDetector() *CycleDetector {
	return &CycleDetector{}
}

// DetectCycles finds all cycles (SCCs with size > 1) in the directed graph.
func (d *CycleDetector) DetectCycles(g DirectedGraph) *CycleResult {
	result := &CycleResult{
		AffectedNodes: make(map[string]bool),
	}

	// context.Background is never cancelled, so the SCC pass cannot fail here.
	components, err := StronglyConnectedComponents(context.Background(), g)
	if err != nil {
		panic(err)
	}

	// Only SCCs with more than one node are actual cycles.
	for _, scc := range components {
		if len(scc) <= 1 {
			continue
		}
		result.Cycles = append(result.Cycles, scc)
		for _, node := range scc {
			result.AffectedNodes[node] = true
		}
	}

	result.HasCycles = len(result.Cycles) > 0
	return result
}

// StronglyConnectedComponents returns every strongly connected component of g,
// including single-node components, using Tarjan's algorithm. Components are
// returned in reverse topological order: a component appears before any
// component it depends on.
//
// The pass checks ctx once per visited node and returns ctx.Err() as soon as
// the context is cancelled.
func StronglyConnectedComponents(ctx context.Context, g DirectedGraph) ([][]string, error) {
	d := &CycleDetector{
		ctx:      ctx,
		onStack:  make(map[string]bool),
		indices:  make(map[string]int),
		lowlinks: make(map[string]int),
	}

	var components [][]string
	d.collect = func(scc []string) { components = append(components, scc) }

	for _, nodeID := range g.NodeIDs() {
		if _, visited := d.indices[nodeID]; visited {
			continue
		}
		if err := d.strongConnect(g, nodeID); err != nil {
			return nil, err
		}
	}

	return components, nil
}

func (d *CycleDetector) strongConnect(g DirectedGraph, v string) error {
	if err := d.ctx.Err(); err != nil {
		return err
	}

	d.indices[v] = d.index
	d.lowlinks[v] = d.index
	d.index++
	d.stack = append(d.stack, v)
	d.onStack[v] = true

	for _, w := range g.Successors(v) {
		if _, visited := d.indices[w]; !visited {
			if err := d.strongConnect(g, w); err != nil {
				return err
			}
			if d.lowlinks[w] < d.lowlinks[v] {
				d.lowlinks[v] = d.lowlinks[w]
			}
		} else if d.onStack[w] {
			if d.indices[w] < d.lowlinks[v] {
				d.lowlinks[v] = d.indices[w]
			}
		}
	}

	if d.lowlinks[v] == d.indices[v] {
		var scc []string
		for {
			w := d.stack[len(d.stack)-1]
			d.stack = d.stack[:len(d.stack)-1]
			d.onStack[w] = false
			scc = append(scc, w)
			if w == v {
				break
			}
		}
		if d.collect != nil {
			d.collect(scc)
		}
	}
	return nil
}

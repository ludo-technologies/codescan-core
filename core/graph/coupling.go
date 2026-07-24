package graph

import "math"

// CouplingMetrics holds Robert Martin's package coupling metrics for a node.
type CouplingMetrics struct {
	NodeID       string
	Ca           int     // Afferent coupling (incoming edges)
	Ce           int     // Efferent coupling (outgoing edges)
	Instability  float64 // Ce / (Ca + Ce), 0 = maximally stable
	Abstractness float64 // Provided by language-specific callback
	Distance     float64 // |Abstractness + Instability - 1|
}

// CouplingConfig configures coupling metric computation.
type CouplingConfig struct {
	// AbstractnessFunc computes the abstractness (0.0–1.0) for a given node.
	// pyscn: ratio of public names matching abstract patterns.
	// jscan: export ratio.
	// If nil, Abstractness defaults to 0.0 for all nodes.
	AbstractnessFunc func(nodeID string) (float64, error)
}

// ComputeCouplingMetrics computes Robert Martin's coupling metrics for all
// nodes in the directed graph.
func ComputeCouplingMetrics(g DirectedGraph, config CouplingConfig) (map[string]*CouplingMetrics, error) {
	result := make(map[string]*CouplingMetrics)

	for _, nodeID := range g.NodeIDs() {
		ca := len(g.Predecessors(nodeID))
		ce := len(g.Successors(nodeID))

		var instability float64
		if ca+ce > 0 {
			instability = float64(ce) / float64(ca+ce)
		}

		var abstractness float64
		if config.AbstractnessFunc != nil {
			var err error
			abstractness, err = config.AbstractnessFunc(nodeID)
			if err != nil {
				return nil, err
			}
		}

		distance := math.Abs(abstractness + instability - 1.0)

		result[nodeID] = &CouplingMetrics{
			NodeID:       nodeID,
			Ca:           ca,
			Ce:           ce,
			Instability:  instability,
			Abstractness: abstractness,
			Distance:     distance,
		}
	}

	return result, nil
}

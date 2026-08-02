package analyzer

import (
	"sort"

	coregraph "github.com/ludo-technologies/polyscan/core/graph"
	"github.com/ludo-technologies/polyscan/jscan/domain"
)

const (
	mainSequenceMaxDistance = 0.2
	zoneMinDistance         = 0.5
	lowAbstractness         = 0.3
	highAbstractness        = 0.7
)

// CouplingMetricsConfig configures the CouplingMetricsCalculator
type CouplingMetricsConfig struct {
	// InstabilityHighThreshold defines the threshold for high instability (default: 0.7)
	InstabilityHighThreshold float64

	// InstabilityLowThreshold defines the threshold for low instability (default: 0.3)
	InstabilityLowThreshold float64

	// DistanceThreshold defines the threshold for main sequence deviation (default: 0.3)
	DistanceThreshold float64

	// CouplingHighThreshold defines the threshold for high coupling risk (default: 10)
	CouplingHighThreshold int

	// CouplingMediumThreshold defines the threshold for medium coupling risk (default: 5)
	CouplingMediumThreshold int
}

// DefaultCouplingMetricsConfig returns a config with sensible defaults
func DefaultCouplingMetricsConfig() *CouplingMetricsConfig {
	return &CouplingMetricsConfig{
		InstabilityHighThreshold: 0.7,
		InstabilityLowThreshold:  0.3,
		DistanceThreshold:        0.3,
		CouplingHighThreshold:    10,
		CouplingMediumThreshold:  5,
	}
}

// CouplingMetricsCalculator calculates coupling metrics for modules
type CouplingMetricsCalculator struct {
	config *CouplingMetricsConfig
}

// NewCouplingMetricsCalculator creates a new CouplingMetricsCalculator
func NewCouplingMetricsCalculator(config *CouplingMetricsConfig) *CouplingMetricsCalculator {
	if config == nil {
		config = DefaultCouplingMetricsConfig()
	}
	return &CouplingMetricsCalculator{config: config}
}

// CalculateMetrics computes coupling metrics for all modules in the graph
func (c *CouplingMetricsCalculator) CalculateMetrics(graph *domain.DependencyGraph) map[string]*domain.ModuleDependencyMetrics {
	if graph == nil || graph.NodeCount() == 0 {
		return make(map[string]*domain.ModuleDependencyMetrics)
	}

	coreMetrics, err := coregraph.ComputeCouplingMetrics(graph, coregraph.CouplingConfig{
		AbstractnessFunc: func(nodeID string) (float64, error) {
			return c.calculateAbstractness(graph.GetNode(nodeID)), nil
		},
	})
	if err != nil {
		return make(map[string]*domain.ModuleDependencyMetrics)
	}

	metrics := make(map[string]*domain.ModuleDependencyMetrics, len(coreMetrics))
	for _, nodeID := range graph.NodeIDs() {
		node := graph.GetNode(nodeID)
		m := coreMetrics[nodeID]

		// Stability zone is calculated in CalculateCouplingAnalysis

		// Assess risk level
		riskLevel := c.assessRiskLevel(m.Ca, m.Ce, m.Distance)

		// Get direct dependencies and dependents
		directDeps := c.getDirectDependencies(nodeID, graph)
		dependents := c.getDependents(nodeID, graph)

		metrics[nodeID] = &domain.ModuleDependencyMetrics{
			ModuleName:             node.Name,
			FilePath:               node.FilePath,
			AfferentCoupling:       m.Ca,
			EfferentCoupling:       m.Ce,
			Instability:            m.Instability,
			Abstractness:           m.Abstractness,
			Distance:               m.Distance,
			RiskLevel:              riskLevel,
			DirectDependencies:     directDeps,
			Dependents:             dependents,
			TransitiveDependencies: []string{}, // Can be computed separately if needed
		}
	}

	return metrics
}

// CalculateCouplingAnalysis generates a comprehensive coupling analysis
func (c *CouplingMetricsCalculator) CalculateCouplingAnalysis(graph *domain.DependencyGraph, metrics map[string]*domain.ModuleDependencyMetrics) *domain.CouplingAnalysis {
	if len(metrics) == 0 {
		return &domain.CouplingAnalysis{
			CouplingDistribution: make(map[int]int),
		}
	}

	var totalCoupling float64
	var totalInstability float64
	var totalDistance float64
	couplingDist := make(map[int]int)
	var highlyCoupled []string
	var looselyCoupled []string
	var stableModules []string
	var unstableModules []string
	var zoneOfPain []string
	var zoneOfUselessness []string
	var mainSequence []string

	// Walk modules in a fixed order: floating-point sums are order-dependent, so
	// map iteration order would otherwise perturb the averages below.
	nodeIDs := make([]string, 0, len(metrics))
	for nodeID := range metrics {
		nodeIDs = append(nodeIDs, nodeID)
	}
	sort.Strings(nodeIDs)

	for _, nodeID := range nodeIDs {
		m := metrics[nodeID]
		coupling := m.AfferentCoupling + m.EfferentCoupling
		totalCoupling += float64(coupling)
		totalInstability += m.Instability
		totalDistance += m.Distance

		// Distribution bucketing
		bucket := c.getCouplingBucket(coupling)
		couplingDist[bucket]++

		// Classify by coupling level
		if coupling >= c.config.CouplingHighThreshold {
			highlyCoupled = append(highlyCoupled, nodeID)
		} else if coupling <= 2 {
			looselyCoupled = append(looselyCoupled, nodeID)
		}

		// Classify by instability
		if m.Instability <= c.config.InstabilityLowThreshold {
			stableModules = append(stableModules, nodeID)
		} else if m.Instability >= c.config.InstabilityHighThreshold {
			unstableModules = append(unstableModules, nodeID)
		}

		// Classify by zone
		zone := c.classifyStabilityZone(m)
		switch zone {
		case "zone_of_pain":
			zoneOfPain = append(zoneOfPain, nodeID)
		case "zone_of_uselessness":
			zoneOfUselessness = append(zoneOfUselessness, nodeID)
		case "main_sequence":
			mainSequence = append(mainSequence, nodeID)
		}
	}

	count := float64(len(metrics))

	// Sort for deterministic output
	sort.Strings(highlyCoupled)
	sort.Strings(looselyCoupled)
	sort.Strings(stableModules)
	sort.Strings(unstableModules)
	sort.Strings(zoneOfPain)
	sort.Strings(zoneOfUselessness)
	sort.Strings(mainSequence)

	return &domain.CouplingAnalysis{
		AverageCoupling:       totalCoupling / count,
		CouplingDistribution:  couplingDist,
		HighlyCoupledModules:  highlyCoupled,
		LooselyCoupledModules: looselyCoupled,
		AverageInstability:    totalInstability / count,
		StableModules:         stableModules,
		InstableModules:       unstableModules,
		MainSequenceDeviation: totalDistance / count,
		ZoneOfPain:            zoneOfPain,
		ZoneOfUselessness:     zoneOfUselessness,
		MainSequence:          mainSequence,
	}
}

// calculateAbstractness calculates A = abstractions / total declarations
// Simplified: based on the ratio of exports to a baseline
// In a more complete implementation, this would analyze actual abstractions (interfaces, abstract classes)
func (c *CouplingMetricsCalculator) calculateAbstractness(node *domain.ModuleNode) float64 {
	if node == nil {
		return 0.0
	}

	// Simplified abstractness calculation based on exports
	// More exports generally indicate more abstraction
	exports := len(node.Exports)
	if exports == 0 {
		return 0.0 // Concrete - no exports
	}

	// Cap at 1.0, with 10+ exports being fully abstract
	abstractness := float64(exports) / 10.0
	if abstractness > 1.0 {
		abstractness = 1.0
	}
	return abstractness
}

// classifyStabilityZone classifies a module into a stability zone.
// Modules between the main sequence band and the zones belong to no zone
// and return an empty string.
func (c *CouplingMetricsCalculator) classifyStabilityZone(m *domain.ModuleDependencyMetrics) string {
	// Zone of Pain: stable concrete modules far from the main sequence that
	// other modules actually depend on. Hard to change, lots depend on it.
	if m.Distance >= zoneMinDistance &&
		m.AfferentCoupling >= 2 &&
		m.Instability <= c.config.InstabilityLowThreshold &&
		m.Abstractness <= lowAbstractness {
		return "zone_of_pain"
	}

	// Zone of Uselessness: unstable abstract modules far from the main
	// sequence. Abstract but nothing uses it.
	if m.Distance >= zoneMinDistance &&
		m.Instability >= c.config.InstabilityHighThreshold &&
		m.Abstractness >= highAbstractness {
		return "zone_of_uselessness"
	}

	// Main Sequence: modules close to A + I = 1
	if m.Distance <= mainSequenceMaxDistance {
		return "main_sequence"
	}

	return ""
}

// assessRiskLevel assesses the risk level based on coupling and distance
func (c *CouplingMetricsCalculator) assessRiskLevel(ca, ce int, distance float64) domain.RiskLevel {
	totalCoupling := ca + ce

	// High risk: high coupling or far from main sequence
	if totalCoupling >= c.config.CouplingHighThreshold || distance > 0.5 {
		return domain.RiskLevelHigh
	}

	// Medium risk: moderate coupling or moderate distance
	if totalCoupling >= c.config.CouplingMediumThreshold || distance > c.config.DistanceThreshold {
		return domain.RiskLevelMedium
	}

	return domain.RiskLevelLow
}

// getDirectDependencies returns the IDs of modules this module depends on
func (c *CouplingMetricsCalculator) getDirectDependencies(nodeID string, graph *domain.DependencyGraph) []string {
	return graph.Successors(nodeID)
}

// getDependents returns the IDs of modules that depend on this module
func (c *CouplingMetricsCalculator) getDependents(nodeID string, graph *domain.DependencyGraph) []string {
	return graph.Predecessors(nodeID)
}

// getCouplingBucket returns the bucket for coupling distribution
func (c *CouplingMetricsCalculator) getCouplingBucket(coupling int) int {
	switch {
	case coupling == 0:
		return 0
	case coupling <= 3:
		return 3
	case coupling <= 7:
		return 7
	case coupling <= 10:
		return 10
	default:
		return 11 // 11+ bucket
	}
}

// CalculateTransitiveDependencies calculates all transitive dependencies for a module
func (c *CouplingMetricsCalculator) CalculateTransitiveDependencies(nodeID string, graph *domain.DependencyGraph) []string {
	visited := make(map[string]bool)
	var result []string

	var dfs func(current string)
	dfs = func(current string) {
		edges := graph.GetOutgoingEdges(current)
		for _, edge := range edges {
			if !visited[edge.To] {
				visited[edge.To] = true
				result = append(result, edge.To)
				dfs(edge.To)
			}
		}
	}

	dfs(nodeID)
	sort.Strings(result)
	return result
}

// CalculateMaxDepth calculates the maximum dependency depth in the graph,
// measured in edges along the longest dependency chain.
//
// The chain comes from core/graph's condensation-based finder: cycles are
// collapsed before the search, so the depth is well defined even when modules
// import each other, and it is the same chain the report shows under
// LongestChains.
func (c *CouplingMetricsCalculator) CalculateMaxDepth(graph *domain.DependencyGraph) int {
	if graph == nil || graph.NodeCount() == 0 {
		return 0
	}

	chain := coregraph.NewChainFinder(graph).LongestChain()
	if len(chain) == 0 {
		return 0
	}
	return len(chain) - 1
}

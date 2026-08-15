package analyzer

import (
	"fmt"
	"sort"
	"strings"

	coregraph "github.com/ludo-technologies/polyscan/core/graph"
	"github.com/ludo-technologies/polyscan/jscan/domain"
)

// CircularDependencyDetector detects circular dependencies and enriches the
// core graph result with jscan-specific details and remediation suggestions.
type CircularDependencyDetector struct{}

// NewCircularDependencyDetector creates a new CircularDependencyDetector
func NewCircularDependencyDetector() *CircularDependencyDetector {
	return &CircularDependencyDetector{}
}

// isLoadTimeEdge reports whether an edge participates in module loading.
// Dynamic import() edges are evaluated only when the call executes, so they
// cannot form a load-time circular import and are excluded from cycle
// detection. A pair connected by both a static and a dynamic import is still
// a load-time dependency via its static edge. See pyscn issue #460.
func isLoadTimeEdge(edge *domain.DependencyEdge) bool {
	return edge != nil && edge.EdgeType != domain.EdgeTypeDynamic
}

// LoadTimeGraph is graph seen through its load-time imports alone. Every
// structural analysis of how modules are layered — cycles, depth, chains —
// reads the same view, so the reported depth cannot count an edge that the
// cycle report has already ruled out. Coupling metrics deliberately keep the
// dynamic edges: a lazy import is still a runtime dependency.
func LoadTimeGraph(graph *domain.DependencyGraph) coregraph.DirectedGraph {
	return loadTimeGraph{graph: graph}
}

// loadTimeGraph excludes dynamic imports from the graph seen by structural
// analysis while leaving the full dependency graph unchanged for reporting.
type loadTimeGraph struct {
	graph *domain.DependencyGraph
}

func (g loadTimeGraph) NodeIDs() []string { return g.graph.NodeIDs() }

func (g loadTimeGraph) Successors(nodeID string) []string {
	var ids []string
	for _, edge := range g.graph.GetOutgoingEdges(nodeID) {
		if isLoadTimeEdge(edge) && g.graph.HasNode(edge.To) {
			ids = append(ids, edge.To)
		}
	}
	sort.Strings(ids)
	return ids
}

func (g loadTimeGraph) Predecessors(nodeID string) []string {
	var ids []string
	for _, edge := range g.graph.GetIncomingEdges(nodeID) {
		if isLoadTimeEdge(edge) && g.graph.HasNode(edge.From) {
			ids = append(ids, edge.From)
		}
	}
	sort.Strings(ids)
	return ids
}

func (g loadTimeGraph) NodeCount() int             { return g.graph.NodeCount() }
func (g loadTimeGraph) HasNode(nodeID string) bool { return g.graph.HasNode(nodeID) }

// DetectCycles finds all load-time cycles using the shared core detector.
func (d *CircularDependencyDetector) DetectCycles(graph *domain.DependencyGraph) *domain.CircularDependencyAnalysis {
	if graph == nil || graph.NodeCount() == 0 {
		return &domain.CircularDependencyAnalysis{
			HasCircularDependencies: false,
			TotalCycles:             0,
			TotalModulesInCycles:    0,
			CircularDependencies:    []domain.CircularDependency{},
		}
	}

	result := coregraph.NewCycleDetector().DetectCycles(loadTimeGraph{graph: graph})
	sccs := result.Cycles
	for _, scc := range sccs {
		sort.Strings(scc)
	}
	sort.Slice(sccs, func(i, j int) bool {
		return strings.Join(sccs[i], "\x00") < strings.Join(sccs[j], "\x00")
	})

	// Filter to only SCCs with more than one node (actual cycles)
	var cycles []domain.CircularDependency
	var modulesInCycles = make(map[string]bool)
	var coreInfrastructure = make(map[string]int) // module -> count of cycles it's in

	for _, scc := range sccs {
		cycle := d.buildCycleInfo(scc, graph)
		cycles = append(cycles, cycle)

		for _, module := range scc {
			modulesInCycles[module] = true
			coreInfrastructure[module]++
		}
	}

	// Find core infrastructure (modules in multiple cycles)
	var coreModules []string
	for module, count := range coreInfrastructure {
		if count > 1 {
			coreModules = append(coreModules, module)
		}
	}
	sort.Strings(coreModules)

	// Generate cycle breaking suggestions
	suggestions := d.suggestCycleBreaking(cycles, graph)

	return &domain.CircularDependencyAnalysis{
		HasCircularDependencies:  len(cycles) > 0,
		TotalCycles:              len(cycles),
		TotalModulesInCycles:     len(modulesInCycles),
		CircularDependencies:     cycles,
		CycleBreakingSuggestions: suggestions,
		CoreInfrastructure:       coreModules,
	}
}

// buildCycleInfo creates a CircularDependency from an SCC
func (d *CircularDependencyDetector) buildCycleInfo(scc []string, graph *domain.DependencyGraph) domain.CircularDependency {
	// Find the cycle path (edges forming the cycle)
	var paths []domain.DependencyPath
	sccSet := make(map[string]bool)
	for _, module := range scc {
		sccSet[module] = true
	}

	// Find edges within the SCC
	for _, from := range scc {
		edges := graph.GetOutgoingEdges(from)
		for _, edge := range edges {
			if !isLoadTimeEdge(edge) {
				continue // lazy edges are not load-time dependencies (#460)
			}
			if sccSet[edge.To] {
				paths = append(paths, domain.DependencyPath{
					From:   from,
					To:     edge.To,
					Path:   []string{from, edge.To},
					Length: 1,
				})
			}
		}
	}

	// Calculate severity based on cycle size
	severity := d.calculateCycleSeverity(scc)

	// Generate description
	description := d.generateCycleDescription(scc)

	return domain.CircularDependency{
		Modules:      scc,
		Dependencies: paths,
		Severity:     severity,
		Size:         len(scc),
		Description:  description,
	}
}

// calculateCycleSeverity determines the severity of a cycle based on its size
func (d *CircularDependencyDetector) calculateCycleSeverity(scc []string) domain.CycleSeverity {
	size := len(scc)
	switch {
	case size <= 2:
		return domain.CycleSeverityLow
	case size <= 4:
		return domain.CycleSeverityMedium
	case size <= 6:
		return domain.CycleSeverityHigh
	default:
		return domain.CycleSeverityCritical
	}
}

// generateCycleDescription creates a human-readable description of the cycle
func (d *CircularDependencyDetector) generateCycleDescription(scc []string) string {
	if len(scc) == 0 {
		return ""
	}

	// Sort for consistent output
	sorted := make([]string, len(scc))
	copy(sorted, scc)
	sort.Strings(sorted)

	// Create a simple cycle representation
	var parts []string
	for _, module := range sorted {
		// Extract just the filename for readability
		parts = append(parts, getModuleBaseName(module))
	}

	return fmt.Sprintf("Circular dependency involving %d modules: %s", len(scc), strings.Join(parts, " <-> "))
}

// suggestCycleBreaking generates suggestions for breaking cycles
func (d *CircularDependencyDetector) suggestCycleBreaking(cycles []domain.CircularDependency, graph *domain.DependencyGraph) []string {
	var suggestions []string

	for _, cycle := range cycles {
		if len(cycle.Modules) == 0 {
			continue
		}

		// Find the best edge to break
		bestEdge := d.findBestEdgeToBreak(cycle, graph)
		if bestEdge != nil {
			suggestion := fmt.Sprintf(
				"Consider removing or inverting the dependency from '%s' to '%s' to break the cycle",
				getModuleBaseName(bestEdge.From),
				getModuleBaseName(bestEdge.To),
			)
			suggestions = append(suggestions, suggestion)
		}

		// Suggest interface extraction for larger cycles
		if len(cycle.Modules) >= 3 {
			suggestion := fmt.Sprintf(
				"Consider extracting interfaces for modules: %s",
				strings.Join(getModuleBaseNames(cycle.Modules), ", "),
			)
			suggestions = append(suggestions, suggestion)
		}
	}

	return suggestions
}

// findBestEdgeToBreak finds the edge with the lowest weight (least used) to break a cycle
func (d *CircularDependencyDetector) findBestEdgeToBreak(cycle domain.CircularDependency, graph *domain.DependencyGraph) *domain.DependencyEdge {
	var bestEdge *domain.DependencyEdge
	minWeight := int(^uint(0) >> 1) // Max int

	sccSet := make(map[string]bool)
	for _, module := range cycle.Modules {
		sccSet[module] = true
	}

	for _, module := range cycle.Modules {
		edges := graph.GetOutgoingEdges(module)
		for _, edge := range edges {
			if !isLoadTimeEdge(edge) {
				continue // breaking a lazy edge would not break the load-time cycle
			}
			if sccSet[edge.To] && edge.Weight < minWeight {
				minWeight = edge.Weight
				bestEdge = edge
			}
		}
	}

	return bestEdge
}

// getModuleBaseName extracts a readable module name from a path
func getModuleBaseName(path string) string {
	// Get the last component of the path
	parts := strings.Split(path, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return path
}

// getModuleBaseNames extracts readable names from multiple paths
func getModuleBaseNames(paths []string) []string {
	names := make([]string, len(paths))
	for i, path := range paths {
		names[i] = getModuleBaseName(path)
	}
	return names
}

// FindCyclePath finds a path from one module to another within a cycle
func (d *CircularDependencyDetector) FindCyclePath(from, to string, graph *domain.DependencyGraph) []string {
	if graph == nil {
		return nil
	}

	visited := make(map[string]bool)
	path := []string{}

	var dfs func(current string) bool
	dfs = func(current string) bool {
		if current == to {
			path = append(path, current)
			return true
		}

		if visited[current] {
			return false
		}
		visited[current] = true
		path = append(path, current)

		edges := graph.GetOutgoingEdges(current)
		for _, edge := range edges {
			if !isLoadTimeEdge(edge) {
				continue // lazy edges are not load-time dependencies (#460)
			}
			if dfs(edge.To) {
				return true
			}
		}

		// Backtrack
		path = path[:len(path)-1]
		return false
	}

	if dfs(from) {
		return path
	}
	return nil
}

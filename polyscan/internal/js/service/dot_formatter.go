package service

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/ludo-technologies/polyscan/polyscan/internal/js/domain"
	"github.com/ludo-technologies/polyscan/polyscan/internal/js/version"
)

// DOTFormatterConfig configures the DOT formatter behavior
type DOTFormatterConfig struct {
	// ClusterCycles groups cycles in subgraphs
	ClusterCycles bool

	// ShowLegend includes a legend subgraph
	ShowLegend bool

	// MaxDepth filters by depth (0 = unlimited)
	MaxDepth int

	// MinCoupling filters by minimum coupling level
	MinCoupling int

	// RankDir is the layout direction: TB, LR, BT, RL
	RankDir string
}

// DefaultDOTFormatterConfig returns a DOTFormatterConfig with sensible defaults
func DefaultDOTFormatterConfig() *DOTFormatterConfig {
	return &DOTFormatterConfig{
		ClusterCycles: true,
		ShowLegend:    true,
		MaxDepth:      0,
		MinCoupling:   0,
		RankDir:       "TB",
	}
}

// DOTFormatter formats dependency graphs as DOT for Graphviz
type DOTFormatter struct {
	config *DOTFormatterConfig
}

// NewDOTFormatter creates a new DOT formatter with the given configuration
func NewDOTFormatter(config *DOTFormatterConfig) *DOTFormatter {
	if config == nil {
		config = DefaultDOTFormatterConfig()
	}
	return &DOTFormatter{config: config}
}

// nodeColors defines the color scheme for nodes based on risk level.
// This is effectively a constant map and should not be modified at runtime.
var nodeColors = map[domain.RiskLevel]struct {
	fill   string
	border string
}{
	domain.RiskLevelLow:    {fill: "#90EE90", border: "#228B22"},
	domain.RiskLevelMedium: {fill: "#FFD700", border: "#FFA500"},
	domain.RiskLevelHigh:   {fill: "#FF6B6B", border: "#DC143C"},
}

// edgeStyles defines the visual style for edges based on dependency type.
// This is effectively a constant map and should not be modified at runtime.
var edgeStyles = map[domain.DependencyEdgeType]struct {
	style string
	arrow string
}{
	domain.EdgeTypeImport:   {style: "solid", arrow: "normal"},
	domain.EdgeTypeDynamic:  {style: "dashed", arrow: "empty"},
	domain.EdgeTypeTypeOnly: {style: "dotted", arrow: "odot"},
	domain.EdgeTypeReExport: {style: "bold", arrow: "diamond"},
}

// FormatDependencyGraph formats a dependency graph as DOT and returns the string
func (f *DOTFormatter) FormatDependencyGraph(response *domain.DependencyGraphResponse) (string, error) {
	var sb strings.Builder
	if err := f.WriteDependencyGraph(response, &sb); err != nil {
		return "", err
	}
	return sb.String(), nil
}

// validRankDirs contains the valid Graphviz rank directions
var validRankDirs = map[string]bool{
	"TB": true, // Top to Bottom
	"LR": true, // Left to Right
	"BT": true, // Bottom to Top
	"RL": true, // Right to Left
}

// WriteDependencyGraph writes a dependency graph as DOT to the writer
func (f *DOTFormatter) WriteDependencyGraph(response *domain.DependencyGraphResponse, writer io.Writer) error {
	if response == nil || response.Graph == nil {
		return fmt.Errorf("nil response or graph")
	}

	// Validate RankDir
	if !validRankDirs[f.config.RankDir] {
		return fmt.Errorf("invalid rank direction %q: must be one of TB, LR, BT, RL", f.config.RankDir)
	}

	graph := response.Graph
	analysis := response.Analysis

	// Collect nodes that pass filtering
	filteredNodes := f.filterNodes(graph, analysis)
	if len(filteredNodes) == 0 {
		// Empty graph
		fmt.Fprintf(writer, "/* jscan Dependency Graph - Generated: %s */\n", time.Now().Format(time.RFC3339))
		fmt.Fprintln(writer, "digraph dependencies {")
		fmt.Fprintln(writer, "    /* No modules match the filter criteria */")
		fmt.Fprintln(writer, "}")
		return nil
	}

	// Build set of modules in cycles for quick lookup
	cycleModules := make(map[string]int) // module -> cycle index
	var cycles []domain.CircularDependency

	if analysis != nil && analysis.CircularDependencies != nil &&
		analysis.CircularDependencies.HasCircularDependencies {
		cycles = analysis.CircularDependencies.CircularDependencies
		for i, cycle := range cycles {
			for _, mod := range cycle.Modules {
				if _, exists := cycleModules[mod]; !exists {
					cycleModules[mod] = i
				}
			}
		}
	}

	// Write header
	fmt.Fprintf(writer, "/* jscan Dependency Graph - Generated: %s */\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(writer, "/* Version: %s */\n", version.GetVersion())
	fmt.Fprintln(writer, "digraph dependencies {")
	fmt.Fprintf(writer, "    rankdir=%s;\n", f.config.RankDir)
	fmt.Fprintln(writer, "    node [shape=box, style=filled, fontname=\"Helvetica\"];")
	fmt.Fprintln(writer, "    edge [fontname=\"Helvetica\", fontsize=10];")
	fmt.Fprintln(writer)

	// Track which nodes have been written (to avoid duplicates when clustering)
	writtenNodes := make(map[string]bool)

	// Write cycle clusters if enabled
	if f.config.ClusterCycles && len(cycles) > 0 {
		for i, cycle := range cycles {
			// Only process if cycle has modules in the filtered set
			hasFilteredModules := false
			for _, mod := range cycle.Modules {
				if filteredNodes[mod] {
					hasFilteredModules = true
					break
				}
			}
			if !hasFilteredModules {
				continue
			}

			fmt.Fprintf(writer, "    // Cycle %d\n", i)
			fmt.Fprintf(writer, "    subgraph cluster_cycle_%d {\n", i)
			fmt.Fprintf(writer, "        label=\"Cycle: %s (%s)\";\n",
				f.formatCycleLabel(cycle), cycle.Severity)
			fmt.Fprintln(writer, "        style=filled;")
			fmt.Fprintln(writer, "        fillcolor=\"#FFEEEE\";")
			fmt.Fprintln(writer, "        color=\"#DC143C\";")
			fmt.Fprintln(writer)

			// Write nodes in this cycle
			for _, moduleID := range cycle.Modules {
				if !filteredNodes[moduleID] {
					continue
				}
				node := graph.GetNode(moduleID)
				if node == nil {
					continue
				}
				f.writeNode(writer, node, analysis, "        ")
				writtenNodes[moduleID] = true
			}

			fmt.Fprintln(writer, "    }")
			fmt.Fprintln(writer)
		}
	}

	// Write regular nodes (not in clusters)
	fmt.Fprintln(writer, "    // Regular nodes")
	// Sort node IDs for deterministic output
	var nodeIDs []string
	for id := range filteredNodes {
		nodeIDs = append(nodeIDs, id)
	}
	sort.Strings(nodeIDs)

	for _, nodeID := range nodeIDs {
		if writtenNodes[nodeID] {
			continue
		}
		node := graph.GetNode(nodeID)
		if node == nil {
			continue
		}
		f.writeNode(writer, node, analysis, "    ")
	}
	fmt.Fprintln(writer)

	// Write edges
	fmt.Fprintln(writer, "    // Edges")
	f.writeEdges(writer, graph, filteredNodes, cycleModules)
	fmt.Fprintln(writer)

	// Write legend if enabled
	if f.config.ShowLegend {
		f.writeLegend(writer)
	}

	fmt.Fprintln(writer, "}")

	return nil
}

// filterNodes returns a set of node IDs that pass the filter criteria
func (f *DOTFormatter) filterNodes(graph *domain.DependencyGraph, analysis *domain.DependencyAnalysisResult) map[string]bool {
	result := make(map[string]bool)

	for nodeID, node := range graph.Nodes {
		// Skip external modules unless explicitly included
		if node.IsExternal {
			continue
		}

		// Apply min coupling filter
		if f.config.MinCoupling > 0 && analysis != nil && analysis.ModuleMetrics != nil {
			metrics := analysis.ModuleMetrics[nodeID]
			if metrics != nil {
				totalCoupling := metrics.AfferentCoupling + metrics.EfferentCoupling
				if totalCoupling < f.config.MinCoupling {
					continue
				}
			}
		}

		result[nodeID] = true
	}

	// Apply max depth filter if specified
	if f.config.MaxDepth > 0 && analysis != nil {
		result = f.filterByDepth(graph, analysis, result)
	}

	return result
}

// filterByDepth filters nodes by maximum dependency depth from entry points.
// The analysis parameter is currently unused but reserved for future enhancements
// such as using pre-computed depth information from the analysis result.
func (f *DOTFormatter) filterByDepth(graph *domain.DependencyGraph, _ *domain.DependencyAnalysisResult, nodes map[string]bool) map[string]bool {
	result := make(map[string]bool)

	// Find entry points
	var entryPoints []string
	for nodeID := range nodes {
		node := graph.GetNode(nodeID)
		if node != nil && node.IsEntryPoint {
			entryPoints = append(entryPoints, nodeID)
		}
	}

	// BFS from entry points
	visited := make(map[string]int) // node -> depth
	queue := make([]struct {
		id    string
		depth int
	}, 0)

	for _, ep := range entryPoints {
		queue = append(queue, struct {
			id    string
			depth int
		}{ep, 0})
		visited[ep] = 0
	}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if current.depth > f.config.MaxDepth {
			continue
		}

		if nodes[current.id] {
			result[current.id] = true
		}

		edges := graph.GetOutgoingEdges(current.id)
		for _, edge := range edges {
			if _, seen := visited[edge.To]; !seen {
				visited[edge.To] = current.depth + 1
				queue = append(queue, struct {
					id    string
					depth int
				}{edge.To, current.depth + 1})
			}
		}
	}

	return result
}

// writeNode writes a single node in DOT format
func (f *DOTFormatter) writeNode(writer io.Writer, node *domain.ModuleNode, analysis *domain.DependencyAnalysisResult, indent string) {
	dotID := escapeDOTID(node.ID)
	label := node.Name
	if label == "" {
		label = node.ID
	}

	// Determine risk level and colors
	riskLevel := domain.RiskLevelLow
	var tooltip string

	if analysis != nil && analysis.ModuleMetrics != nil {
		metrics := analysis.ModuleMetrics[node.ID]
		if metrics != nil {
			riskLevel = metrics.RiskLevel
			tooltip = fmt.Sprintf("Ca: %d, Ce: %d\\nInstability: %.2f",
				metrics.AfferentCoupling, metrics.EfferentCoupling, metrics.Instability)
		}
	}

	// Add entry point / leaf info to tooltip
	var nodeType string
	if node.IsEntryPoint {
		nodeType = "Entry Point"
	} else if node.IsLeaf {
		nodeType = "Leaf Module"
	}
	if nodeType != "" {
		if tooltip != "" {
			tooltip = nodeType + "\\n" + tooltip
		} else {
			tooltip = nodeType
		}
	}

	colors := nodeColors[riskLevel]
	if colors.fill == "" {
		colors = nodeColors[domain.RiskLevelLow]
	}

	fmt.Fprintf(writer, "%s%s [label=\"%s\", fillcolor=\"%s\", color=\"%s\"",
		indent, dotID, escapeDOTLabel(label), colors.fill, colors.border)

	if tooltip != "" {
		fmt.Fprintf(writer, ", tooltip=\"%s\"", tooltip)
	}

	fmt.Fprintln(writer, "];")
}

// writeEdges writes all edges in DOT format
func (f *DOTFormatter) writeEdges(writer io.Writer, graph *domain.DependencyGraph, filteredNodes map[string]bool, cycleModules map[string]int) {
	// Collect and sort edges for deterministic output
	type edgeKey struct {
		from, to string
	}
	edges := make(map[edgeKey]*domain.DependencyEdge)
	var edgeKeys []edgeKey

	for nodeID := range filteredNodes {
		for _, edge := range graph.GetOutgoingEdges(nodeID) {
			if !filteredNodes[edge.To] {
				continue
			}
			key := edgeKey{edge.From, edge.To}
			if _, exists := edges[key]; !exists {
				edges[key] = edge
				edgeKeys = append(edgeKeys, key)
			}
		}
	}

	sort.Slice(edgeKeys, func(i, j int) bool {
		if edgeKeys[i].from != edgeKeys[j].from {
			return edgeKeys[i].from < edgeKeys[j].from
		}
		return edgeKeys[i].to < edgeKeys[j].to
	})

	for _, key := range edgeKeys {
		edge := edges[key]
		fromID := escapeDOTID(edge.From)
		toID := escapeDOTID(edge.To)

		style := edgeStyles[edge.EdgeType]
		if style.style == "" {
			style = edgeStyles[domain.EdgeTypeImport]
		}

		// Check if this edge is part of a cycle
		_, fromInCycle := cycleModules[edge.From]
		_, toInCycle := cycleModules[edge.To]
		isCycleEdge := fromInCycle && toInCycle

		fmt.Fprintf(writer, "    %s -> %s [style=%s, arrowhead=%s",
			fromID, toID, style.style, style.arrow)

		if isCycleEdge {
			fmt.Fprint(writer, ", penwidth=2, color=\"#DC143C\"")
		}

		if edge.EdgeType != domain.EdgeTypeImport {
			fmt.Fprintf(writer, ", label=\"%s\"", edge.EdgeType)
		}

		fmt.Fprintln(writer, "];")
	}
}

// writeLegend writes the legend subgraph
func (f *DOTFormatter) writeLegend(writer io.Writer) {
	fmt.Fprintln(writer, "    // Legend")
	fmt.Fprintln(writer, "    subgraph cluster_legend {")
	fmt.Fprintln(writer, "        label=\"Legend\";")
	fmt.Fprintln(writer, "        style=filled;")
	fmt.Fprintln(writer, "        fillcolor=\"#F5F5F5\";")
	fmt.Fprintln(writer, "        color=\"#CCCCCC\";")
	fmt.Fprintln(writer, "        fontsize=10;")
	fmt.Fprintln(writer)
	fmt.Fprintln(writer, "        // Risk levels")
	fmt.Fprintf(writer, "        legend_low [label=\"Low Risk\", fillcolor=\"%s\", color=\"%s\"];\n",
		nodeColors[domain.RiskLevelLow].fill, nodeColors[domain.RiskLevelLow].border)
	fmt.Fprintf(writer, "        legend_medium [label=\"Medium Risk\", fillcolor=\"%s\", color=\"%s\"];\n",
		nodeColors[domain.RiskLevelMedium].fill, nodeColors[domain.RiskLevelMedium].border)
	fmt.Fprintf(writer, "        legend_high [label=\"High Risk\", fillcolor=\"%s\", color=\"%s\"];\n",
		nodeColors[domain.RiskLevelHigh].fill, nodeColors[domain.RiskLevelHigh].border)
	fmt.Fprintln(writer)
	fmt.Fprintln(writer, "        // Edge types")
	fmt.Fprintln(writer, "        legend_import_a [label=\"\", style=invis, width=0, height=0];")
	fmt.Fprintln(writer, "        legend_import_b [label=\"import\", style=invis, width=0, height=0];")
	fmt.Fprintln(writer, "        legend_import_a -> legend_import_b [style=solid, arrowhead=normal, label=\"import\"];")
	fmt.Fprintln(writer)
	fmt.Fprintln(writer, "        legend_dynamic_a [label=\"\", style=invis, width=0, height=0];")
	fmt.Fprintln(writer, "        legend_dynamic_b [label=\"dynamic\", style=invis, width=0, height=0];")
	fmt.Fprintln(writer, "        legend_dynamic_a -> legend_dynamic_b [style=dashed, arrowhead=empty, label=\"dynamic\"];")
	fmt.Fprintln(writer)
	fmt.Fprintln(writer, "        legend_type_a [label=\"\", style=invis, width=0, height=0];")
	fmt.Fprintln(writer, "        legend_type_b [label=\"type_only\", style=invis, width=0, height=0];")
	fmt.Fprintln(writer, "        legend_type_a -> legend_type_b [style=dotted, arrowhead=odot, label=\"type_only\"];")
	fmt.Fprintln(writer)
	fmt.Fprintln(writer, "        legend_reexport_a [label=\"\", style=invis, width=0, height=0];")
	fmt.Fprintln(writer, "        legend_reexport_b [label=\"re_export\", style=invis, width=0, height=0];")
	fmt.Fprintln(writer, "        legend_reexport_a -> legend_reexport_b [style=bold, arrowhead=diamond, label=\"re_export\"];")
	fmt.Fprintln(writer)
	fmt.Fprintln(writer, "        // Cycle indicator")
	fmt.Fprintln(writer, "        legend_cycle_a [label=\"\", style=invis, width=0, height=0];")
	fmt.Fprintln(writer, "        legend_cycle_b [label=\"cycle\", style=invis, width=0, height=0];")
	fmt.Fprintln(writer, "        legend_cycle_a -> legend_cycle_b [penwidth=2, color=\"#DC143C\", label=\"cycle\"];")
	fmt.Fprintln(writer, "    }")
}

// formatCycleLabel creates a short label for a cycle
func (f *DOTFormatter) formatCycleLabel(cycle domain.CircularDependency) string {
	if len(cycle.Modules) == 0 {
		return "Empty Cycle"
	}
	if len(cycle.Modules) == 2 {
		return fmt.Sprintf("%s <-> %s",
			shortenModuleName(cycle.Modules[0]),
			shortenModuleName(cycle.Modules[1]))
	}
	return fmt.Sprintf("%s -> ... (%d modules)",
		shortenModuleName(cycle.Modules[0]), len(cycle.Modules))
}

// shortenModuleName extracts a short name from a module ID
func shortenModuleName(moduleID string) string {
	// Get the last component of the path
	parts := strings.Split(moduleID, "/")
	if len(parts) > 0 {
		name := parts[len(parts)-1]
		// Remove extension
		if idx := strings.LastIndex(name, "."); idx > 0 {
			name = name[:idx]
		}
		return name
	}
	return moduleID
}

// escapeDOTID escapes a string for use as a DOT node ID
func escapeDOTID(id string) string {
	// Replace characters that are problematic in DOT IDs
	replacer := strings.NewReplacer(
		"/", "__",
		".", "_",
		"-", "_",
		"@", "_at_",
		" ", "_",
		":", "_",
		"(", "_",
		")", "_",
		"[", "_",
		"]", "_",
		"{", "_",
		"}", "_",
	)
	escaped := replacer.Replace(id)

	// Ensure it starts with a letter or underscore
	if len(escaped) > 0 && !isValidDOTIDStart(escaped[0]) {
		escaped = "_" + escaped
	}

	return escaped
}

// escapeDOTLabel escapes a string for use as a DOT label
func escapeDOTLabel(label string) string {
	// Escape special characters in labels
	// Note: backslash must be first to avoid double-escaping
	replacer := strings.NewReplacer(
		"\\", "\\\\",
		"\"", "\\\"",
		"\n", "\\n",
		"\r", "",
		"\t", "\\t",
	)
	return replacer.Replace(label)
}

// isValidDOTIDStart checks if a character can start a DOT ID
func isValidDOTIDStart(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_'
}

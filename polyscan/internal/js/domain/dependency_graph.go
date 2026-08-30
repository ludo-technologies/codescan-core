package domain

import "sort"

// DependencyEdgeType represents the type of dependency relationship
type DependencyEdgeType string

const (
	// EdgeTypeImport represents static ES6/CommonJS import
	EdgeTypeImport DependencyEdgeType = "import"

	// EdgeTypeDynamic represents dynamic import()
	EdgeTypeDynamic DependencyEdgeType = "dynamic"

	// EdgeTypeTypeOnly represents TypeScript type-only import
	EdgeTypeTypeOnly DependencyEdgeType = "type_only"

	// EdgeTypeReExport represents export { } from
	EdgeTypeReExport DependencyEdgeType = "re_export"
)

// ModuleNode represents a node in the dependency graph
type ModuleNode struct {
	// ID is the unique identifier (normalized path)
	ID string `json:"id"`

	// Name is the module name (filename without extension)
	Name string `json:"name"`

	// FilePath is the full file path
	FilePath string `json:"file_path"`

	// ModuleType is the classification (relative, package, builtin, alias)
	ModuleType ModuleType `json:"module_type"`

	// IsExternal indicates if the module is not in the project (e.g., node_modules)
	IsExternal bool `json:"is_external"`

	// IsEntryPoint indicates if no other modules depend on this one
	IsEntryPoint bool `json:"is_entry_point"`

	// IsLeaf indicates if this module has no dependencies
	IsLeaf bool `json:"is_leaf"`

	// Exports lists the exported names from this module
	Exports []string `json:"exports,omitempty"`
}

// DependencyEdge represents a directed edge in the dependency graph
type DependencyEdge struct {
	// From is the source module ID
	From string `json:"from"`

	// To is the target module ID
	To string `json:"to"`

	// EdgeType is the type of dependency (import/dynamic/type_only/re_export)
	EdgeType DependencyEdgeType `json:"edge_type"`

	// Specifiers are the individual imported items
	Specifiers []string `json:"specifiers,omitempty"`

	// Location is the source code location of the import statement
	Location *SourceLocation `json:"location,omitempty"`

	// Weight is the number of uses (for coupling calculations)
	Weight int `json:"weight"`
}

// DependencyGraph represents the complete dependency graph
type DependencyGraph struct {
	// Nodes maps module ID to ModuleNode
	Nodes map[string]*ModuleNode `json:"nodes"`

	// Edges maps source module ID to its outgoing edges
	Edges map[string][]*DependencyEdge `json:"edges"`

	// ReverseEdges maps target module ID to incoming edges (for afferent coupling)
	ReverseEdges map[string][]*DependencyEdge `json:"-"`
}

// NewDependencyGraph creates a new empty DependencyGraph
func NewDependencyGraph() *DependencyGraph {
	return &DependencyGraph{
		Nodes:        make(map[string]*ModuleNode),
		Edges:        make(map[string][]*DependencyEdge),
		ReverseEdges: make(map[string][]*DependencyEdge),
	}
}

// AddNode adds a node to the graph
func (g *DependencyGraph) AddNode(node *ModuleNode) {
	if node == nil {
		return
	}
	g.Nodes[node.ID] = node
}

// AddEdge adds an edge to the graph and updates reverse edges
func (g *DependencyGraph) AddEdge(edge *DependencyEdge) {
	if edge == nil {
		return
	}
	g.Edges[edge.From] = append(g.Edges[edge.From], edge)
	g.ReverseEdges[edge.To] = append(g.ReverseEdges[edge.To], edge)
}

// GetNode returns a node by ID
func (g *DependencyGraph) GetNode(id string) *ModuleNode {
	return g.Nodes[id]
}

// GetOutgoingEdges returns all edges from a node (efferent)
func (g *DependencyGraph) GetOutgoingEdges(nodeID string) []*DependencyEdge {
	return g.Edges[nodeID]
}

// GetIncomingEdges returns all edges to a node (afferent)
func (g *DependencyGraph) GetIncomingEdges(nodeID string) []*DependencyEdge {
	return g.ReverseEdges[nodeID]
}

// NodeCount returns the number of nodes in the graph
func (g *DependencyGraph) NodeCount() int {
	return len(g.Nodes)
}

// EdgeCount returns the total number of edges in the graph
func (g *DependencyGraph) EdgeCount() int {
	count := 0
	for _, edges := range g.Edges {
		count += len(edges)
	}
	return count
}

// NodeIDs returns all node IDs sorted lexicographically.
func (g *DependencyGraph) NodeIDs() []string {
	ids := make([]string, 0, len(g.Nodes))
	for id := range g.Nodes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// Successors returns sorted target IDs for outgoing edges. Duplicate edges are
// intentionally preserved because jscan coupling counts import statements.
func (g *DependencyGraph) Successors(nodeID string) []string {
	edges := g.Edges[nodeID]
	if len(edges) == 0 {
		return nil
	}

	ids := make([]string, 0, len(edges))
	for _, edge := range edges {
		if edge != nil {
			ids = append(ids, edge.To)
		}
	}
	sort.Strings(ids)
	return ids
}

// Predecessors returns sorted source IDs for incoming edges. Duplicate edges
// are intentionally preserved because jscan coupling counts import statements.
func (g *DependencyGraph) Predecessors(nodeID string) []string {
	edges := g.ReverseEdges[nodeID]
	if len(edges) == 0 {
		return nil
	}

	ids := make([]string, 0, len(edges))
	for _, edge := range edges {
		if edge != nil {
			ids = append(ids, edge.From)
		}
	}
	sort.Strings(ids)
	return ids
}

// HasNode reports whether a node ID exists in the graph.
func (g *DependencyGraph) HasNode(nodeID string) bool {
	_, ok := g.Nodes[nodeID]
	return ok
}

// GetAllNodeIDs returns all node IDs in deterministic order.
func (g *DependencyGraph) GetAllNodeIDs() []string {
	return g.NodeIDs()
}

// UpdateNodeFlags updates IsEntryPoint and IsLeaf flags for all nodes
func (g *DependencyGraph) UpdateNodeFlags() {
	for _, node := range g.Nodes {
		// IsEntryPoint: no incoming edges (no dependents)
		node.IsEntryPoint = len(g.ReverseEdges[node.ID]) == 0

		// IsLeaf: no outgoing edges (no dependencies)
		node.IsLeaf = len(g.Edges[node.ID]) == 0
	}
}

// DependencyGraphRequest represents a request for dependency graph analysis
type DependencyGraphRequest struct {
	// Paths are the input files or directories to analyze
	Paths []string `json:"paths"`

	// OutputFormat specifies the output format
	OutputFormat OutputFormat `json:"output_format"`

	// OutputPath is the path to save output file
	OutputPath string `json:"output_path,omitempty"`

	// NoOpen prevents auto-opening HTML in browser
	NoOpen bool `json:"no_open"`

	// Recursive indicates whether to analyze directories recursively
	Recursive *bool `json:"recursive,omitempty"`

	// IncludePatterns are glob patterns for files to include
	IncludePatterns []string `json:"include_patterns,omitempty"`

	// ExcludePatterns are glob patterns for files to exclude
	ExcludePatterns []string `json:"exclude_patterns,omitempty"`

	// IncludeExternal indicates whether to include external modules (node_modules)
	IncludeExternal *bool `json:"include_external,omitempty"`

	// IncludeTypeImports indicates whether to include TypeScript type imports
	IncludeTypeImports *bool `json:"include_type_imports,omitempty"`

	// DetectCycles enables circular dependency detection
	DetectCycles *bool `json:"detect_cycles,omitempty"`

	// Thresholds for risk assessment
	InstabilityHighThreshold float64 `json:"instability_high_threshold,omitempty"`
	InstabilityLowThreshold  float64 `json:"instability_low_threshold,omitempty"`
	DistanceThreshold        float64 `json:"distance_threshold,omitempty"`
}

// DefaultDependencyGraphRequest returns a DependencyGraphRequest with default values
func DefaultDependencyGraphRequest() *DependencyGraphRequest {
	return &DependencyGraphRequest{
		OutputFormat:             OutputFormatText,
		Recursive:                BoolPtr(true),
		IncludePatterns:          []string{"**/*.js", "**/*.ts", "**/*.jsx", "**/*.tsx"},
		ExcludePatterns:          []string{"node_modules/**", "dist/**", "build/**"},
		IncludeExternal:          BoolPtr(false),
		IncludeTypeImports:       BoolPtr(true),
		DetectCycles:             BoolPtr(true),
		InstabilityHighThreshold: 0.7,
		InstabilityLowThreshold:  0.3,
		DistanceThreshold:        0.3,
	}
}

// DependencyGraphResponse represents the response from dependency graph analysis
type DependencyGraphResponse struct {
	// Graph is the complete dependency graph
	Graph *DependencyGraph `json:"graph"`

	// Analysis is the dependency analysis result
	Analysis *DependencyAnalysisResult `json:"analysis"`

	// Warnings contains any warnings from analysis
	Warnings []string `json:"warnings,omitempty"`

	// Errors contains any errors encountered during analysis
	Errors []string `json:"errors,omitempty"`

	// GeneratedAt is when the analysis was generated
	GeneratedAt string `json:"generated_at"`

	// Version is the tool version
	Version string `json:"version"`
}

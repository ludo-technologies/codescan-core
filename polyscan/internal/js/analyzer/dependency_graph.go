package analyzer

import (
	"path/filepath"
	"strings"

	"github.com/ludo-technologies/polyscan/polyscan/internal/js/domain"
	"github.com/ludo-technologies/polyscan/polyscan/internal/js/parser"
)

// DependencyGraphBuilderConfig configures the DependencyGraphBuilder
type DependencyGraphBuilderConfig struct {
	// IncludeExternal includes external modules (node_modules) in the graph
	IncludeExternal bool

	// IncludeTypeImports includes TypeScript type-only imports
	IncludeTypeImports bool

	// ProjectRoot is the root directory for path normalization
	ProjectRoot string
}

// DefaultDependencyGraphBuilderConfig returns a config with sensible defaults
func DefaultDependencyGraphBuilderConfig() *DependencyGraphBuilderConfig {
	return &DependencyGraphBuilderConfig{
		IncludeExternal:    false,
		IncludeTypeImports: true,
		ProjectRoot:        "",
	}
}

// DependencyGraphBuilder builds a dependency graph from module analysis results
type DependencyGraphBuilder struct {
	config         *DependencyGraphBuilderConfig
	moduleAnalyzer *ModuleAnalyzer
}

// NewDependencyGraphBuilder creates a new DependencyGraphBuilder
func NewDependencyGraphBuilder(config *DependencyGraphBuilderConfig) *DependencyGraphBuilder {
	if config == nil {
		config = DefaultDependencyGraphBuilderConfig()
	}

	moduleConfig := DefaultModuleAnalyzerConfig()
	moduleConfig.IncludeTypeImports = config.IncludeTypeImports

	return &DependencyGraphBuilder{
		config:         config,
		moduleAnalyzer: NewModuleAnalyzer(moduleConfig),
	}
}

// BuildGraph constructs a DependencyGraph from ModuleAnalysisResult
func (b *DependencyGraphBuilder) BuildGraph(moduleResult *domain.ModuleAnalysisResult) *domain.DependencyGraph {
	if moduleResult == nil {
		return domain.NewDependencyGraph()
	}

	graph := domain.NewDependencyGraph()

	// Create nodes for all analyzed files
	for filePath, moduleInfo := range moduleResult.Files {
		node := b.createModuleNode(filePath, moduleInfo)
		graph.AddNode(node)
	}

	// Build set of known node IDs for extension resolution
	knownNodeIDs := make(map[string]bool, len(graph.Nodes))
	for id := range graph.Nodes {
		knownNodeIDs[id] = true
	}

	// Create edges from imports
	for filePath, moduleInfo := range moduleResult.Files {
		fromID := b.normalizeModuleID(filePath)

		for _, imp := range moduleInfo.Imports {
			// Skip type-only imports if not configured
			if imp.IsTypeOnly && !b.config.IncludeTypeImports {
				continue
			}

			// Skip external modules if not configured
			if b.isExternalModule(imp.Source, imp.SourceType) && !b.config.IncludeExternal {
				continue
			}

			edge := b.createDependencyEdge(fromID, imp, filePath, knownNodeIDs)
			if edge != nil {
				// Ensure target node exists (for external or unresolved modules)
				toID := edge.To
				if graph.GetNode(toID) == nil {
					externalNode := b.createExternalNode(toID, imp.SourceType)
					graph.AddNode(externalNode)
				}
				graph.AddEdge(edge)
			}
		}
	}

	// Update node flags (IsEntryPoint, IsLeaf)
	graph.UpdateNodeFlags()

	return graph
}

// BuildGraphFromASTs constructs a DependencyGraph directly from parsed ASTs
func (b *DependencyGraphBuilder) BuildGraphFromASTs(asts map[string]*parser.Node) (*domain.DependencyGraph, error) {
	moduleResult, err := b.moduleAnalyzer.AnalyzeAll(asts)
	if err != nil {
		return nil, err
	}
	return b.BuildGraph(moduleResult), nil
}

// normalizeModuleID normalizes a file path to a module ID
func (b *DependencyGraphBuilder) normalizeModuleID(filePath string) string {
	// Use relative path if project root is set
	if b.config.ProjectRoot != "" {
		rel, err := filepath.Rel(b.config.ProjectRoot, filePath)
		if err == nil {
			return filepath.ToSlash(rel)
		}
	}
	return filepath.ToSlash(filePath)
}

// moduleDisplayName derives a module name from a path: the base name without
// its extension
func moduleDisplayName(path string) string {
	name := filepath.Base(path)
	ext := filepath.Ext(name)
	if ext != "" {
		name = strings.TrimSuffix(name, ext)
	}
	return name
}

// createModuleNode creates a ModuleNode from file path and module info
func (b *DependencyGraphBuilder) createModuleNode(filePath string, info *domain.ModuleInfo) *domain.ModuleNode {
	id := b.normalizeModuleID(filePath)
	name := moduleDisplayName(filePath)

	// Extract export names
	var exports []string
	if info != nil {
		for _, exp := range info.Exports {
			if exp.Name != "" {
				exports = append(exports, exp.Name)
			}
			for _, spec := range exp.Specifiers {
				exports = append(exports, spec.Exported)
			}
		}
	}

	return &domain.ModuleNode{
		ID:           id,
		Name:         name,
		FilePath:     filePath,
		ModuleType:   domain.ModuleTypeRelative,
		IsExternal:   false,
		Exports:      exports,
		Abstractness: ExportAbstractness(exports),
	}
}

// ExportAbstractness grades a JavaScript module's abstractness by its export
// count: more exports generally indicate more abstraction, and ten or more
// count as fully abstract. A module with no exports is concrete.
func ExportAbstractness(exports []string) float64 {
	return min(float64(len(exports))/10.0, 1.0)
}

// createExternalNode creates a ModuleNode for an external/unresolved module.
// The name comes from the resolved ID, not from the importing file's specifier:
// several files can reach the same unresolved module through different relative
// specifiers, and the first importer visited depends on map iteration order.
func (b *DependencyGraphBuilder) createExternalNode(id string, moduleType domain.ModuleType) *domain.ModuleNode {
	name := id
	switch moduleType {
	case domain.ModuleTypeRelative, domain.ModuleTypeAbsolute:
		// These IDs are paths, so show the file name like project nodes do.
		name = moduleDisplayName(id)
	}

	return &domain.ModuleNode{
		ID:         id,
		Name:       name,
		FilePath:   "",
		ModuleType: moduleType,
		IsExternal: true,
	}
}

// createDependencyEdge creates a DependencyEdge from an import
func (b *DependencyGraphBuilder) createDependencyEdge(fromID string, imp *domain.Import, fromFilePath string, knownNodeIDs map[string]bool) *domain.DependencyEdge {
	if imp == nil {
		return nil
	}

	// Determine edge type
	edgeType := b.getEdgeType(imp)

	// Resolve target module ID
	toID := b.resolveImportTarget(imp.Source, imp.SourceType, fromFilePath, knownNodeIDs)

	// Extract specifier names
	var specifiers []string
	for _, spec := range imp.Specifiers {
		specifiers = append(specifiers, spec.Local)
	}

	// Calculate weight (number of specifiers or 1 for namespace/default)
	weight := len(specifiers)
	if weight == 0 {
		weight = 1
	}

	return &domain.DependencyEdge{
		From:       fromID,
		To:         toID,
		EdgeType:   edgeType,
		Specifiers: specifiers,
		Location:   &imp.Location,
		Weight:     weight,
	}
}

// getEdgeType determines the edge type from an import
func (b *DependencyGraphBuilder) getEdgeType(imp *domain.Import) domain.DependencyEdgeType {
	if imp.IsDynamic {
		return domain.EdgeTypeDynamic
	}
	if imp.IsTypeOnly {
		return domain.EdgeTypeTypeOnly
	}
	return domain.EdgeTypeImport
}

// resolveImportTarget resolves an import source to a target module ID
func (b *DependencyGraphBuilder) resolveImportTarget(source string, sourceType domain.ModuleType, fromFilePath string, knownNodeIDs map[string]bool) string {
	switch sourceType {
	case domain.ModuleTypeRelative:
		// Resolve relative path from the importing file
		dir := filepath.Dir(fromFilePath)
		resolved := filepath.Join(dir, source)
		// Normalize the path
		resolved = filepath.Clean(resolved)
		normalized := b.normalizeModuleID(resolved)

		// If the normalized ID matches a known node, return it directly
		if knownNodeIDs[normalized] {
			return normalized
		}

		// Try appending common extensions
		extensions := []string{".ts", ".tsx", ".js", ".jsx", ".mts", ".cts", ".mjs", ".cjs"}
		for _, ext := range extensions {
			candidate := normalized + ext
			if knownNodeIDs[candidate] {
				return candidate
			}
		}

		// Try directory index files
		for _, ext := range extensions {
			candidate := normalized + "/index" + ext
			if knownNodeIDs[candidate] {
				return candidate
			}
		}

		// No match found; return normalized as-is (will become external)
		return normalized

	case domain.ModuleTypePackage, domain.ModuleTypeBuiltin:
		// Use the source as-is for packages and builtins
		return source

	case domain.ModuleTypeAlias:
		// Use the source as-is for aliases (resolution would require tsconfig)
		return source

	case domain.ModuleTypeAbsolute:
		return b.normalizeModuleID(source)

	default:
		return source
	}
}

// isExternalModule checks if a module is external (not part of the project)
func (b *DependencyGraphBuilder) isExternalModule(source string, sourceType domain.ModuleType) bool {
	switch sourceType {
	case domain.ModuleTypePackage, domain.ModuleTypeBuiltin:
		return true
	default:
		return false
	}
}

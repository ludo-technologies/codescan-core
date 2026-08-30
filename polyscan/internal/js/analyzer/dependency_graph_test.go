package analyzer

import (
	"reflect"
	"testing"

	coregraph "github.com/ludo-technologies/polyscan/core/graph"
	"github.com/ludo-technologies/polyscan/polyscan/internal/js/domain"
	"github.com/ludo-technologies/polyscan/polyscan/internal/js/parser"
)

var _ coregraph.DirectedGraph = (*domain.DependencyGraph)(nil)

func TestDefaultDependencyGraphBuilderConfig(t *testing.T) {
	config := DefaultDependencyGraphBuilderConfig()

	if config.IncludeExternal != false {
		t.Errorf("Expected IncludeExternal to be false, got %v", config.IncludeExternal)
	}
	if config.IncludeTypeImports != true {
		t.Errorf("Expected IncludeTypeImports to be true, got %v", config.IncludeTypeImports)
	}
}

func TestNewDependencyGraphBuilder(t *testing.T) {
	builder := NewDependencyGraphBuilder(nil)
	if builder == nil {
		t.Fatal("Expected builder to not be nil")
	}
	if builder.config == nil {
		t.Fatal("Expected config to not be nil")
	}
	if builder.moduleAnalyzer == nil {
		t.Fatal("Expected moduleAnalyzer to not be nil")
	}
}

func TestBuildGraphFromSimpleImports(t *testing.T) {
	source := `
import React from 'react';
import { useState, useEffect } from 'react';
import './utils';
import { helper } from './helpers';
`
	p := parser.NewParser()
	defer p.Close()

	ast, err := p.ParseString(source)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	moduleAnalyzer := NewModuleAnalyzer(DefaultModuleAnalyzerConfig())
	moduleInfo, err := moduleAnalyzer.AnalyzeFile(ast, "/src/app.js")
	if err != nil {
		t.Fatalf("Failed to analyze: %v", err)
	}

	moduleResult := &domain.ModuleAnalysisResult{
		Files: map[string]*domain.ModuleInfo{
			"/src/app.js": moduleInfo,
		},
	}

	config := DefaultDependencyGraphBuilderConfig()
	config.IncludeExternal = true
	builder := NewDependencyGraphBuilder(config)

	graph := builder.BuildGraph(moduleResult)

	if graph == nil {
		t.Fatal("Expected graph to not be nil")
	}

	// Should have the source file node
	if graph.GetNode("/src/app.js") == nil {
		t.Error("Expected source file node to exist")
	}

	// Should have edges for imports
	edges := graph.GetOutgoingEdges("/src/app.js")
	if len(edges) == 0 {
		t.Error("Expected at least one edge")
	}
}

func TestBuildGraphWithDynamicImports(t *testing.T) {
	source := `
const module = await import('./dynamic-module');
`
	p := parser.NewParser()
	defer p.Close()

	ast, err := p.ParseString(source)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	moduleAnalyzer := NewModuleAnalyzer(DefaultModuleAnalyzerConfig())
	moduleInfo, err := moduleAnalyzer.AnalyzeFile(ast, "/src/app.js")
	if err != nil {
		t.Fatalf("Failed to analyze: %v", err)
	}

	moduleResult := &domain.ModuleAnalysisResult{
		Files: map[string]*domain.ModuleInfo{
			"/src/app.js": moduleInfo,
		},
	}

	builder := NewDependencyGraphBuilder(nil)
	graph := builder.BuildGraph(moduleResult)

	edges := graph.GetOutgoingEdges("/src/app.js")

	if len(edges) != 1 {
		t.Fatalf("Expected 1 edge, got %d", len(edges))
	}
	if edges[0].EdgeType != domain.EdgeTypeDynamic {
		t.Errorf("Expected edge type %q, got %q", domain.EdgeTypeDynamic, edges[0].EdgeType)
	}
}

func TestBuildGraphWithTypeOnlyImports(t *testing.T) {
	source := `
import type { User } from './types';
import { normalImport } from './utils';
`
	// `import type` is TypeScript syntax, so the JavaScript grammar cannot
	// parse this source.
	p := parser.NewTypeScriptParser()
	defer p.Close()

	ast, err := p.ParseString(source)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	moduleAnalyzer := NewModuleAnalyzer(DefaultModuleAnalyzerConfig())
	moduleInfo, err := moduleAnalyzer.AnalyzeFile(ast, "/src/app.ts")
	if err != nil {
		t.Fatalf("Failed to analyze: %v", err)
	}

	// Check if any type-only imports were detected
	hasTypeOnlyImport := false
	for _, imp := range moduleInfo.Imports {
		if imp.IsTypeOnly {
			hasTypeOnlyImport = true
			break
		}
	}

	moduleResult := &domain.ModuleAnalysisResult{
		Files: map[string]*domain.ModuleInfo{
			"/src/app.ts": moduleInfo,
		},
	}

	// Test with type imports included
	config := DefaultDependencyGraphBuilderConfig()
	config.IncludeTypeImports = true
	builder := NewDependencyGraphBuilder(config)
	graph := builder.BuildGraph(moduleResult)

	edges := graph.GetOutgoingEdges("/src/app.ts")

	// Check for type-only edge if parser detected type-only imports
	if hasTypeOnlyImport {
		foundTypeOnly := false
		for _, edge := range edges {
			if edge.EdgeType == domain.EdgeTypeTypeOnly {
				foundTypeOnly = true
				break
			}
		}
		if !foundTypeOnly {
			t.Error("Expected to find a type-only import edge")
		}
	}

	// Test with type imports excluded
	config.IncludeTypeImports = false
	builder = NewDependencyGraphBuilder(config)
	graph = builder.BuildGraph(moduleResult)

	edges = graph.GetOutgoingEdges("/src/app.ts")
	for _, edge := range edges {
		if edge.EdgeType == domain.EdgeTypeTypeOnly {
			t.Error("Expected type-only import edge to be excluded")
			break
		}
	}
}

func TestBuildGraphExcludesExternalModules(t *testing.T) {
	source := `
import React from 'react';
import { helper } from './helpers';
`
	p := parser.NewParser()
	defer p.Close()

	ast, err := p.ParseString(source)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	moduleAnalyzer := NewModuleAnalyzer(DefaultModuleAnalyzerConfig())
	moduleInfo, err := moduleAnalyzer.AnalyzeFile(ast, "/src/app.js")
	if err != nil {
		t.Fatalf("Failed to analyze: %v", err)
	}

	moduleResult := &domain.ModuleAnalysisResult{
		Files: map[string]*domain.ModuleInfo{
			"/src/app.js": moduleInfo,
		},
	}

	// Test with external modules excluded (default)
	config := DefaultDependencyGraphBuilderConfig()
	config.IncludeExternal = false
	builder := NewDependencyGraphBuilder(config)
	graph := builder.BuildGraph(moduleResult)

	// Should not have react node
	if graph.GetNode("react") != nil {
		t.Error("Expected react node to not exist when external modules excluded")
	}

	// Test with external modules included
	config.IncludeExternal = true
	builder = NewDependencyGraphBuilder(config)
	graph = builder.BuildGraph(moduleResult)

	// Should have react node
	if graph.GetNode("react") == nil {
		t.Error("Expected react node to exist when external modules included")
	}
}

func TestBuildGraphUpdatesNodeFlags(t *testing.T) {
	// Test node flags using direct graph construction
	// This avoids path resolution issues with relative imports
	graph := domain.NewDependencyGraph()

	// Create nodes
	graph.AddNode(&domain.ModuleNode{ID: "a", Name: "a"})
	graph.AddNode(&domain.ModuleNode{ID: "b", Name: "b"})
	graph.AddNode(&domain.ModuleNode{ID: "c", Name: "c"})

	// Create edges: A -> B -> C
	graph.AddEdge(&domain.DependencyEdge{From: "a", To: "b", Weight: 1})
	graph.AddEdge(&domain.DependencyEdge{From: "b", To: "c", Weight: 1})

	// Update flags
	graph.UpdateNodeFlags()

	// A should be entry point (no incoming edges)
	nodeA := graph.GetNode("a")
	if nodeA == nil {
		t.Fatal("Expected node A to exist")
	}
	if !nodeA.IsEntryPoint {
		t.Error("Expected A to be an entry point")
	}
	if nodeA.IsLeaf {
		t.Error("Expected A to not be a leaf")
	}

	// B should be neither entry point nor leaf
	nodeB := graph.GetNode("b")
	if nodeB == nil {
		t.Fatal("Expected node B to exist")
	}
	if nodeB.IsEntryPoint {
		t.Error("Expected B to not be an entry point")
	}
	if nodeB.IsLeaf {
		t.Error("Expected B to not be a leaf")
	}

	// C should be a leaf (no outgoing edges)
	nodeC := graph.GetNode("c")
	if nodeC == nil {
		t.Fatal("Expected node C to exist")
	}
	if nodeC.IsEntryPoint {
		t.Error("Expected C to not be an entry point")
	}
	if !nodeC.IsLeaf {
		t.Error("Expected C to be a leaf")
	}
}

func TestNormalizeModuleID(t *testing.T) {
	config := DefaultDependencyGraphBuilderConfig()
	config.ProjectRoot = "/project"
	builder := NewDependencyGraphBuilder(config)

	testCases := []struct {
		input    string
		expected string
	}{
		{"/project/src/app.js", "src/app.js"},
		{"/project/lib/utils.js", "lib/utils.js"},
	}

	for _, tc := range testCases {
		result := builder.normalizeModuleID(tc.input)
		if result != tc.expected {
			t.Errorf("normalizeModuleID(%s) = %s, expected %s", tc.input, result, tc.expected)
		}
	}
}

func TestDependencyGraphNodeCount(t *testing.T) {
	graph := domain.NewDependencyGraph()

	if graph.NodeCount() != 0 {
		t.Error("Expected empty graph to have 0 nodes")
	}

	graph.AddNode(&domain.ModuleNode{ID: "a"})
	graph.AddNode(&domain.ModuleNode{ID: "b"})

	if graph.NodeCount() != 2 {
		t.Errorf("Expected 2 nodes, got %d", graph.NodeCount())
	}
}

func TestDependencyGraphEdgeCount(t *testing.T) {
	graph := domain.NewDependencyGraph()

	if graph.EdgeCount() != 0 {
		t.Error("Expected empty graph to have 0 edges")
	}

	graph.AddEdge(&domain.DependencyEdge{From: "a", To: "b"})
	graph.AddEdge(&domain.DependencyEdge{From: "b", To: "c"})

	if graph.EdgeCount() != 2 {
		t.Errorf("Expected 2 edges, got %d", graph.EdgeCount())
	}
}

func TestDependencyGraphReverseEdges(t *testing.T) {
	graph := domain.NewDependencyGraph()

	graph.AddNode(&domain.ModuleNode{ID: "a"})
	graph.AddNode(&domain.ModuleNode{ID: "b"})
	graph.AddEdge(&domain.DependencyEdge{From: "a", To: "b"})

	// Check forward edge
	outgoing := graph.GetOutgoingEdges("a")
	if len(outgoing) != 1 || outgoing[0].To != "b" {
		t.Error("Expected outgoing edge from a to b")
	}

	// Check reverse edge
	incoming := graph.GetIncomingEdges("b")
	if len(incoming) != 1 || incoming[0].From != "a" {
		t.Error("Expected incoming edge to b from a")
	}
}

func TestDependencyGraphDirectedGraphMethodsAreDeterministicAndPreserveDuplicates(t *testing.T) {
	graph := domain.NewDependencyGraph()
	graph.AddNode(&domain.ModuleNode{ID: "c"})
	graph.AddNode(&domain.ModuleNode{ID: "a"})
	graph.AddNode(&domain.ModuleNode{ID: "b"})
	graph.AddEdge(&domain.DependencyEdge{From: "a", To: "c"})
	graph.AddEdge(&domain.DependencyEdge{From: "a", To: "b"})
	graph.AddEdge(&domain.DependencyEdge{From: "a", To: "b"})

	if got, want := graph.NodeIDs(), []string{"a", "b", "c"}; !reflect.DeepEqual(got, want) {
		t.Errorf("NodeIDs() = %v, want %v", got, want)
	}
	if got, want := graph.Successors("a"), []string{"b", "b", "c"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Successors(a) = %v, want %v", got, want)
	}
	if got, want := graph.Predecessors("b"), []string{"a", "a"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Predecessors(b) = %v, want %v", got, want)
	}
	if !graph.HasNode("a") || graph.HasNode("missing") {
		t.Errorf("HasNode returned unexpected result")
	}
}

func TestBuildGraphFromNilResult(t *testing.T) {
	builder := NewDependencyGraphBuilder(nil)
	graph := builder.BuildGraph(nil)

	if graph == nil {
		t.Fatal("Expected graph to not be nil even with nil input")
	}

	if graph.NodeCount() != 0 {
		t.Error("Expected empty graph")
	}
}

func TestResolveImportTargetWithExtensionResolution(t *testing.T) {
	config := DefaultDependencyGraphBuilderConfig()
	config.ProjectRoot = "/project"
	builder := NewDependencyGraphBuilder(config)

	knownNodeIDs := map[string]bool{
		"lib/application.js": true,
		"lib/router.ts":      true,
		"src/utils.jsx":      true,
	}

	testCases := []struct {
		name         string
		source       string
		fromFilePath string
		expected     string
	}{
		{
			name:         "resolves .js extension",
			source:       "./application",
			fromFilePath: "/project/lib/index.js",
			expected:     "lib/application.js",
		},
		{
			name:         "resolves .ts extension",
			source:       "./router",
			fromFilePath: "/project/lib/index.js",
			expected:     "lib/router.ts",
		},
		{
			name:         "resolves .jsx extension",
			source:       "./utils",
			fromFilePath: "/project/src/app.js",
			expected:     "src/utils.jsx",
		},
		{
			name:         "keeps exact match with extension",
			source:       "./application.js",
			fromFilePath: "/project/lib/index.js",
			expected:     "lib/application.js",
		},
		{
			name:         "returns normalized path for unknown module",
			source:       "./unknown",
			fromFilePath: "/project/lib/index.js",
			expected:     "lib/unknown",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := builder.resolveImportTarget(tc.source, domain.ModuleTypeRelative, tc.fromFilePath, knownNodeIDs)
			if result != tc.expected {
				t.Errorf("resolveImportTarget(%q) = %q, expected %q", tc.source, result, tc.expected)
			}
		})
	}
}

func TestResolveImportTargetWithIndexResolution(t *testing.T) {
	config := DefaultDependencyGraphBuilderConfig()
	config.ProjectRoot = "/project"
	builder := NewDependencyGraphBuilder(config)

	knownNodeIDs := map[string]bool{
		"lib/middleware/index.js": true,
		"src/components/index.ts": true,
	}

	testCases := []struct {
		name         string
		source       string
		fromFilePath string
		expected     string
	}{
		{
			name:         "resolves directory to index.js",
			source:       "./middleware",
			fromFilePath: "/project/lib/app.js",
			expected:     "lib/middleware/index.js",
		},
		{
			name:         "resolves directory to index.ts",
			source:       "./components",
			fromFilePath: "/project/src/app.ts",
			expected:     "src/components/index.ts",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := builder.resolveImportTarget(tc.source, domain.ModuleTypeRelative, tc.fromFilePath, knownNodeIDs)
			if result != tc.expected {
				t.Errorf("resolveImportTarget(%q) = %q, expected %q", tc.source, result, tc.expected)
			}
		})
	}
}

func TestBuildGraphWithExtensionlessImports(t *testing.T) {
	// Build a graph where imports omit file extensions (common in Node.js)
	// and verify internal edges are correctly created without ghost external nodes

	moduleResult := &domain.ModuleAnalysisResult{
		Files: map[string]*domain.ModuleInfo{
			"/project/lib/app.js": {
				Imports: []*domain.Import{
					{
						Source:     "./router",
						SourceType: domain.ModuleTypeRelative,
					},
					{
						Source:     "./utils",
						SourceType: domain.ModuleTypeRelative,
					},
				},
			},
			"/project/lib/router.js": {
				Imports: []*domain.Import{
					{
						Source:     "./utils",
						SourceType: domain.ModuleTypeRelative,
					},
				},
			},
			"/project/lib/utils.js": {
				Imports: []*domain.Import{},
			},
		},
	}

	config := DefaultDependencyGraphBuilderConfig()
	config.ProjectRoot = "/project"
	builder := NewDependencyGraphBuilder(config)
	graph := builder.BuildGraph(moduleResult)

	// Should have exactly 3 internal nodes (no ghost external nodes)
	if graph.NodeCount() != 3 {
		t.Errorf("Expected 3 nodes, got %d", graph.NodeCount())
		for id, node := range graph.Nodes {
			t.Logf("  node: %s (external=%v)", id, node.IsExternal)
		}
	}

	// Verify no external nodes exist
	for id, node := range graph.Nodes {
		if node.IsExternal {
			t.Errorf("Unexpected external node: %s", id)
		}
	}

	// Verify edges connect to internal nodes
	appEdges := graph.GetOutgoingEdges("lib/app.js")
	if len(appEdges) != 2 {
		t.Errorf("Expected 2 edges from lib/app.js, got %d", len(appEdges))
	}
	for _, edge := range appEdges {
		if edge.To != "lib/router.js" && edge.To != "lib/utils.js" {
			t.Errorf("Unexpected edge target: %s", edge.To)
		}
	}

	routerEdges := graph.GetOutgoingEdges("lib/router.js")
	if len(routerEdges) != 1 {
		t.Errorf("Expected 1 edge from lib/router.js, got %d", len(routerEdges))
	}
	if len(routerEdges) > 0 && routerEdges[0].To != "lib/utils.js" {
		t.Errorf("Expected edge to lib/utils.js, got %s", routerEdges[0].To)
	}

	// utils.js should be a leaf (no outgoing edges)
	utilsEdges := graph.GetOutgoingEdges("lib/utils.js")
	if len(utilsEdges) != 0 {
		t.Errorf("Expected 0 edges from lib/utils.js, got %d", len(utilsEdges))
	}
}

func TestBuildGraphNamesUnresolvedModuleFromItsID(t *testing.T) {
	// Two files reach the same unresolved module through different relative
	// specifiers. The node name must not depend on which importer the map
	// iteration visits first.
	moduleResult := &domain.ModuleAnalysisResult{
		Files: map[string]*domain.ModuleInfo{
			"/project/lib/app.js": {
				Imports: []*domain.Import{
					{
						Source:     "./missing/public",
						SourceType: domain.ModuleTypeRelative,
					},
				},
			},
			"/project/lib/missing/router.js": {
				Imports: []*domain.Import{
					{
						Source:     "./public",
						SourceType: domain.ModuleTypeRelative,
					},
				},
			},
		},
	}

	config := DefaultDependencyGraphBuilderConfig()
	config.ProjectRoot = "/project"
	builder := NewDependencyGraphBuilder(config)

	for range 50 {
		graph := builder.BuildGraph(moduleResult)

		node := graph.GetNode("lib/missing/public")
		if node == nil {
			t.Fatal("Expected an unresolved node for lib/missing/public")
		}
		if node.Name != "public" {
			t.Fatalf("Expected name %q, got %q", "public", node.Name)
		}
	}
}

func TestBuildGraphNamesUnresolvedPackageFromItsSource(t *testing.T) {
	moduleResult := &domain.ModuleAnalysisResult{
		Files: map[string]*domain.ModuleInfo{
			"/project/lib/app.js": {
				Imports: []*domain.Import{
					{
						Source:     "@scope/pkg/sub",
						SourceType: domain.ModuleTypePackage,
					},
				},
			},
		},
	}

	config := DefaultDependencyGraphBuilderConfig()
	config.ProjectRoot = "/project"
	config.IncludeExternal = true
	builder := NewDependencyGraphBuilder(config)
	graph := builder.BuildGraph(moduleResult)

	node := graph.GetNode("@scope/pkg/sub")
	if node == nil {
		t.Fatal("Expected an external node for @scope/pkg/sub")
	}
	if node.Name != "@scope/pkg/sub" {
		t.Errorf("Expected name %q, got %q", "@scope/pkg/sub", node.Name)
	}
}

func TestBuildGraphFromASTs(t *testing.T) {
	source := `
import { helper } from './helper';
export const app = 1;
`
	p := parser.NewParser()
	defer p.Close()

	ast, err := p.ParseString(source)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	asts := map[string]*parser.Node{
		"/src/app.js": ast,
	}

	builder := NewDependencyGraphBuilder(nil)
	graph, err := builder.BuildGraphFromASTs(asts)

	if err != nil {
		t.Fatalf("Failed to build graph: %v", err)
	}

	if graph == nil {
		t.Fatal("Expected graph to not be nil")
	}

	if graph.NodeCount() == 0 {
		t.Error("Expected at least one node")
	}
}

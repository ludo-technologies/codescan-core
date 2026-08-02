package service

import (
	"context"
	"fmt"
	"os"
	"sort"
	"time"

	coregraph "github.com/ludo-technologies/polyscan/core/graph"
	"github.com/ludo-technologies/polyscan/jscan/domain"
	"github.com/ludo-technologies/polyscan/jscan/internal/analyzer"
	"github.com/ludo-technologies/polyscan/jscan/internal/parser"
	"github.com/ludo-technologies/polyscan/jscan/internal/version"
)

// DependencyGraphServiceImpl implements dependency graph analysis
type DependencyGraphServiceImpl struct {
	graphBuilderConfig *analyzer.DependencyGraphBuilderConfig
	couplingConfig     *analyzer.CouplingMetricsConfig
	includeTypeImports bool
	includeExternal    bool
}

// NewDependencyGraphService creates a new dependency graph service
func NewDependencyGraphService(includeExternal, includeTypeImports bool) *DependencyGraphServiceImpl {
	return &DependencyGraphServiceImpl{
		graphBuilderConfig: &analyzer.DependencyGraphBuilderConfig{
			IncludeExternal:    includeExternal,
			IncludeTypeImports: includeTypeImports,
		},
		couplingConfig:     analyzer.DefaultCouplingMetricsConfig(),
		includeTypeImports: includeTypeImports,
		includeExternal:    includeExternal,
	}
}

// NewDependencyGraphServiceWithDefaults creates a new service with default configuration
func NewDependencyGraphServiceWithDefaults() *DependencyGraphServiceImpl {
	return &DependencyGraphServiceImpl{
		graphBuilderConfig: analyzer.DefaultDependencyGraphBuilderConfig(),
		couplingConfig:     analyzer.DefaultCouplingMetricsConfig(),
		includeTypeImports: true,
		includeExternal:    false,
	}
}

// Analyze performs complete dependency graph analysis
func (s *DependencyGraphServiceImpl) Analyze(ctx context.Context, req domain.DependencyGraphRequest) (*domain.DependencyGraphResponse, error) {
	var warnings []string
	var errors []string

	// Apply request options to config
	config := *s.graphBuilderConfig
	if req.IncludeExternal != nil {
		config.IncludeExternal = *req.IncludeExternal
	}
	if req.IncludeTypeImports != nil {
		config.IncludeTypeImports = *req.IncludeTypeImports
	}

	// Parse all files
	asts, parseWarnings, parseErrors := s.parseFiles(ctx, req.Paths)
	warnings = append(warnings, parseWarnings...)
	errors = append(errors, parseErrors...)

	if len(asts) == 0 {
		return &domain.DependencyGraphResponse{
			Graph:       domain.NewDependencyGraph(),
			Analysis:    &domain.DependencyAnalysisResult{},
			Warnings:    warnings,
			Errors:      errors,
			GeneratedAt: time.Now().Format(time.RFC3339),
			Version:     version.GetVersion(),
		}, nil
	}

	// Build dependency graph
	graphBuilder := analyzer.NewDependencyGraphBuilder(&config)
	graph, err := graphBuilder.BuildGraphFromASTs(asts)
	if err != nil {
		errors = append(errors, fmt.Sprintf("Failed to build dependency graph: %v", err))
		return &domain.DependencyGraphResponse{
			Graph:       domain.NewDependencyGraph(),
			Analysis:    &domain.DependencyAnalysisResult{},
			Warnings:    warnings,
			Errors:      errors,
			GeneratedAt: time.Now().Format(time.RFC3339),
			Version:     version.GetVersion(),
		}, nil
	}

	// Detect cycles
	var circularDeps *domain.CircularDependencyAnalysis
	if req.DetectCycles == nil || *req.DetectCycles {
		cycleDetector := analyzer.NewCircularDependencyDetector()
		circularDeps = cycleDetector.DetectCycles(graph)
	}

	// Calculate coupling metrics
	couplingConfig := *s.couplingConfig
	if req.InstabilityHighThreshold > 0 {
		couplingConfig.InstabilityHighThreshold = req.InstabilityHighThreshold
	}
	if req.InstabilityLowThreshold > 0 {
		couplingConfig.InstabilityLowThreshold = req.InstabilityLowThreshold
	}
	if req.DistanceThreshold > 0 {
		couplingConfig.DistanceThreshold = req.DistanceThreshold
	}

	couplingCalc := analyzer.NewCouplingMetricsCalculator(&couplingConfig)
	moduleMetrics := couplingCalc.CalculateMetrics(graph)
	couplingAnalysis := couplingCalc.CalculateCouplingAnalysis(graph, moduleMetrics)

	// Calculate max depth
	maxDepth := couplingCalc.CalculateMaxDepth(graph)

	// Build analysis result
	analysis := s.buildAnalysisResult(graph, circularDeps, couplingAnalysis, moduleMetrics, maxDepth)

	return &domain.DependencyGraphResponse{
		Graph:       graph,
		Analysis:    analysis,
		Warnings:    warnings,
		Errors:      errors,
		GeneratedAt: time.Now().Format(time.RFC3339),
		Version:     version.GetVersion(),
	}, nil
}

// parseFiles parses all input files and returns their ASTs
func (s *DependencyGraphServiceImpl) parseFiles(ctx context.Context, paths []string) (map[string]*parser.Node, []string, []string) {
	asts := make(map[string]*parser.Node, len(paths))
	var warnings []string
	var errors []string

	results := analyzeFilesConcurrently(ctx, paths, nil,
		func(_ context.Context, filePath string) fileAnalysis[*parser.Node] {
			content, err := os.ReadFile(filePath)
			if err != nil {
				return fileAnalysis[*parser.Node]{
					errors: []string{fmt.Sprintf("Failed to read %s: %v", filePath, err)},
				}
			}

			// Each call builds its own parser: tree-sitter parsers are not safe
			// for concurrent use.
			p := parser.NewParser()
			defer p.Close()

			ast, err := p.ParseString(string(content))
			if err != nil {
				return fileAnalysis[*parser.Node]{
					warnings: []string{fmt.Sprintf("Failed to parse %s: %v", filePath, err)},
				}
			}

			return fileAnalysis[*parser.Node]{value: ast}
		})

	for index, result := range results {
		warnings = append(warnings, result.warnings...)
		errors = append(errors, result.errors...)
		if result.value != nil {
			asts[paths[index]] = result.value
		}
	}

	if ctx.Err() != nil {
		errors = append(errors, fmt.Sprintf("Parsing cancelled: %v", ctx.Err()))
	}

	return asts, warnings, errors
}

// buildAnalysisResult builds a DependencyAnalysisResult from the analysis components
func (s *DependencyGraphServiceImpl) buildAnalysisResult(
	graph *domain.DependencyGraph,
	circularDeps *domain.CircularDependencyAnalysis,
	couplingAnalysis *domain.CouplingAnalysis,
	moduleMetrics map[string]*domain.ModuleDependencyMetrics,
	maxDepth int,
) *domain.DependencyAnalysisResult {
	// Find root and leaf modules
	var rootModules []string
	var leafModules []string

	for nodeID, node := range graph.Nodes {
		if node.IsEntryPoint {
			rootModules = append(rootModules, nodeID)
		}
		if node.IsLeaf {
			leafModules = append(leafModules, nodeID)
		}
	}

	sort.Strings(rootModules)
	sort.Strings(leafModules)

	// Build dependency matrix
	dependencyMatrix := make(map[string]map[string]bool)
	for nodeID := range graph.Nodes {
		deps := make(map[string]bool)
		edges := graph.GetOutgoingEdges(nodeID)
		for _, edge := range edges {
			deps[edge.To] = true
		}
		if len(deps) > 0 {
			dependencyMatrix[nodeID] = deps
		}
	}

	// Find longest dependency chains
	longestChains := s.findLongestChains(graph, maxDepth)

	return &domain.DependencyAnalysisResult{
		TotalModules:         graph.NodeCount(),
		TotalDependencies:    graph.EdgeCount(),
		RootModules:          rootModules,
		LeafModules:          leafModules,
		ModuleMetrics:        moduleMetrics,
		DependencyMatrix:     dependencyMatrix,
		CircularDependencies: circularDeps,
		CouplingAnalysis:     couplingAnalysis,
		LongestChains:        longestChains,
		MaxDepth:             maxDepth,
	}
}

// maxReportedChains caps how many dependency chains the report carries.
const maxReportedChains = 5

// findLongestChains finds the longest dependency chains in the graph, one per
// entry-point module. Chains come from core/graph's condensation-based finder,
// which is linear in the graph size — an exhaustive simple-path search would be
// exponential on the cyclic import graphs real projects produce.
func (s *DependencyGraphServiceImpl) findLongestChains(graph *domain.DependencyGraph, maxDepth int) []domain.DependencyPath {
	if maxDepth == 0 {
		return nil
	}

	finder := coregraph.NewChainFinder(graph)

	var chains []domain.DependencyPath
	for _, nodeID := range graph.NodeIDs() {
		node := graph.GetNode(nodeID)
		if node == nil || !node.IsEntryPoint {
			continue
		}

		chain := finder.LongestChainFrom(nodeID)
		if len(chain) > 1 {
			chains = append(chains, domain.DependencyPath{
				From:   chain[0],
				To:     chain[len(chain)-1],
				Path:   chain,
				Length: len(chain) - 1,
			})
		}
	}

	// Sort by length (descending), then by starting module so equal-length
	// chains keep a stable order across runs.
	sort.Slice(chains, func(i, j int) bool {
		if chains[i].Length != chains[j].Length {
			return chains[i].Length > chains[j].Length
		}
		return chains[i].From < chains[j].From
	})

	if len(chains) > maxReportedChains {
		chains = chains[:maxReportedChains]
	}

	return chains
}

// AnalyzeSingleFile analyzes a single file and returns its dependency information
func (s *DependencyGraphServiceImpl) AnalyzeSingleFile(ctx context.Context, filePath string) (*domain.ModuleInfo, error) {
	// Read file
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Parse file
	p := parser.NewParser()
	defer p.Close()

	ast, err := p.ParseString(string(content))
	if err != nil {
		return nil, fmt.Errorf("failed to parse file: %w", err)
	}

	// Analyze module
	moduleConfig := analyzer.DefaultModuleAnalyzerConfig()
	moduleConfig.IncludeTypeImports = s.includeTypeImports
	moduleAnalyzer := analyzer.NewModuleAnalyzer(moduleConfig)

	return moduleAnalyzer.AnalyzeFile(ast, filePath)
}

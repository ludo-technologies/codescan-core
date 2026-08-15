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

// Analyze performs complete dependency graph analysis. Unlike the other
// analyses there is no per-file streaming variant: graph construction needs
// every module's AST at once, so a single-analysis run holds the same one set
// of parse trees a shared snapshot would.
func (s *DependencyGraphServiceImpl) Analyze(ctx context.Context, req domain.DependencyGraphRequest) (*domain.DependencyGraphResponse, error) {
	snapshot := BuildProjectSnapshot(ctx, req.Paths, nil)
	return s.AnalyzeSnapshot(ctx, snapshot, req)
}

// AnalyzeSnapshot performs dependency graph analysis on already parsed project
// files. The snapshot defines the analyzed file set; req.Paths, when set, must
// name the same files.
func (s *DependencyGraphServiceImpl) AnalyzeSnapshot(ctx context.Context, snapshot *ProjectSnapshot, req domain.DependencyGraphRequest) (*domain.DependencyGraphResponse, error) {
	if err := snapshot.validateRequestPaths(req.Paths); err != nil {
		return nil, err
	}

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

	// Collect the parsed files
	asts, parseWarnings, parseErrors := collectSnapshotASTs(ctx, snapshot)
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

	// The max depth and the reported chains are the same search over the same
	// condensation, so build the finder once and let both read from it. It runs
	// over the load-time graph, so a dynamic import() cannot add a layer that
	// the cycle report has already ruled out.
	chainFinder := coregraph.NewChainFinder(analyzer.LoadTimeGraph(graph))
	maxDepth := couplingCalc.CalculateMaxDepthFrom(chainFinder)

	// Build analysis result
	analysis := s.buildAnalysisResult(graph, circularDeps, couplingAnalysis, moduleMetrics, maxDepth, chainFinder)

	return &domain.DependencyGraphResponse{
		Graph:       graph,
		Analysis:    analysis,
		Warnings:    warnings,
		Errors:      errors,
		GeneratedAt: time.Now().Format(time.RFC3339),
		Version:     version.GetVersion(),
	}, nil
}

// collectSnapshotASTs gathers the snapshot's parsed files, reporting read
// failures as errors and parse failures as warnings, matching how this
// analysis has always classified them.
func collectSnapshotASTs(ctx context.Context, snapshot *ProjectSnapshot) (map[string]*parser.Node, []string, []string) {
	asts := make(map[string]*parser.Node, len(snapshot.Files))
	var warnings []string
	var errors []string

	for _, file := range snapshot.Files {
		if file.ReadErr != nil {
			errors = append(errors, fmt.Sprintf("Failed to read %s: %v", file.Path, file.ReadErr))
			continue
		}
		if file.ParseErr != nil {
			warnings = append(warnings, fmt.Sprintf("Failed to parse %s: %v", file.Path, file.ParseErr))
			continue
		}
		asts[file.Path] = file.AST
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
	chainFinder *coregraph.ChainFinder,
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
	longestChains := s.findLongestChains(graph, chainFinder, maxDepth)

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
// entry-point module. Chains come from the caller's condensation-based finder,
// which is linear in the graph size — an exhaustive simple-path search would be
// exponential on the cyclic import graphs real projects produce.
func (s *DependencyGraphServiceImpl) findLongestChains(graph *domain.DependencyGraph, finder *coregraph.ChainFinder, maxDepth int) []domain.DependencyPath {
	if maxDepth == 0 || finder == nil {
		return nil
	}

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

	// Parse with the language-appropriate grammar and the real file name, the
	// same way the snapshot parses for Analyze, so both entry points report
	// identical locations and module contents.
	ast, err := parser.ParseForLanguage(filePath, content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse file: %w", err)
	}

	// Analyze module
	moduleConfig := analyzer.DefaultModuleAnalyzerConfig()
	moduleConfig.IncludeTypeImports = s.includeTypeImports
	moduleAnalyzer := analyzer.NewModuleAnalyzer(moduleConfig)

	return moduleAnalyzer.AnalyzeFile(ast, filePath)
}

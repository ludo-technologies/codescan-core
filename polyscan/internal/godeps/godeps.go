package godeps

import (
	"context"
	"time"

	"github.com/ludo-technologies/polyscan/polyscan/internal/js/analyzer"
	"github.com/ludo-technologies/polyscan/polyscan/internal/js/domain"
	"github.com/ludo-technologies/polyscan/polyscan/internal/js/service"
	"github.com/ludo-technologies/polyscan/polyscan/internal/js/version"
)

// Analyze builds the package graph of files and derives the dependency
// analysis from it: coupling metrics, depth and chains, and the cycle report,
// which is always empty because the Go compiler rejects import cycles. The
// response is nil when no package could be placed in the graph, so a tree
// without a go.mod leaves the dependency dimension out rather than scoring
// it clean; the warnings say why.
func Analyze(files []string, display func(string) string) (*domain.DependencyGraphResponse, []string, error) {
	graph, warnings := Build(files, display)
	if graph.NodeCount() == 0 {
		return nil, warnings, nil
	}
	analysis, err := service.AnalyzeDependencyGraph(context.Background(), graph, analyzer.DefaultCouplingMetricsConfig(), true)
	if err != nil {
		return nil, warnings, err
	}
	return &domain.DependencyGraphResponse{
		Graph:       graph,
		Analysis:    analysis,
		GeneratedAt: time.Now().Format(time.RFC3339),
		Version:     version.GetVersion(),
	}, warnings, nil
}

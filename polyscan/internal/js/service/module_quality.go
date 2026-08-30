package service

import (
	"path/filepath"

	"github.com/ludo-technologies/polyscan/polyscan/internal/js/domain"
)

// BuildModuleQuality joins the per-module rollups the analyses own into the
// per-file view the unified report shows. Each analysis contributes only what
// it measured, so a run that skipped an analysis simply leaves those columns at
// zero instead of dropping the module.
//
// The join keys on the cleaned file path, which is what every analysis reports
// for the same file, and the modules come back ranked worst-first.
func BuildModuleQuality(
	complexityResponse *domain.ComplexityResponse,
	deadCodeResponse *domain.DeadCodeResponse,
	depsResponse *domain.DependencyGraphResponse,
) []domain.ModuleQualityMetrics {
	modules := make(map[string]*domain.ModuleQualityMetrics)

	moduleEntry := func(filePath string) *domain.ModuleQualityMetrics {
		key := filepath.Clean(filePath)
		module := modules[key]
		if module == nil {
			module = &domain.ModuleQualityMetrics{FilePath: key}
			modules[key] = module
		}
		return module
	}

	if complexityResponse != nil {
		for filePath, complexity := range complexityResponse.ModuleRollups {
			moduleEntry(filePath).ModuleComplexityMetrics = complexity
		}
	}

	if deadCodeResponse != nil {
		for filePath, deadCode := range deadCodeResponse.ModuleRollups {
			moduleEntry(filePath).ModuleDeadCodeMetrics = deadCode
		}
	}

	// Dependency analysis is the only source of a module's name. It never
	// introduces a module of its own: a file no other analysis measured has
	// nothing to report in this table.
	if depsResponse != nil && depsResponse.Analysis != nil {
		for _, metrics := range depsResponse.Analysis.ModuleMetrics {
			if metrics == nil {
				continue
			}
			if module, ok := modules[filepath.Clean(metrics.FilePath)]; ok {
				module.ModuleName = metrics.ModuleName
			}
		}
	}

	result := make([]domain.ModuleQualityMetrics, 0, len(modules))
	for _, module := range modules {
		result = append(result, *module)
	}
	domain.SortModuleQuality(result)
	return result
}

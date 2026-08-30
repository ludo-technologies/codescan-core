package domain

import (
	"path/filepath"
	"sort"
)

// ModuleComplexityMetrics is the canonical module-level complexity contract.
// AnalyzedFunctionCount includes every function-level complexity record before
// presentation filters.
//
// LinesOfCode belongs to the same rollup because the complexity service is the
// only per-file reader that already holds the file content: counting lines
// there costs nothing, while a separate pass would re-read the project.
//
// pyscn also reports AverageCognitiveComplexity here. jscan has no cognitive
// complexity metric, so the field is absent rather than reported as zero.
type ModuleComplexityMetrics struct {
	LinesOfCode           int     `json:"lines_of_code" yaml:"lines_of_code"`
	AnalyzedFunctionCount int     `json:"analyzed_function_count" yaml:"analyzed_function_count"`
	AverageComplexity     float64 `json:"average_complexity" yaml:"average_complexity"`
	MaxComplexity         int     `json:"max_complexity" yaml:"max_complexity"`
	HighRiskFunctionCount int     `json:"high_risk_function_count" yaml:"high_risk_function_count"`
	ExceptionHandlerCount int     `json:"exception_handler_count" yaml:"exception_handler_count"`
}

// ModuleDeadCodeMetrics is the canonical module-level dead-code contract.
// Both counts describe findings the detectors produced before severity
// filtering, so they do not shrink when a report raises min_severity.
type ModuleDeadCodeMetrics struct {
	DeadCodeFindingCount int `json:"dead_code_finding_count" yaml:"dead_code_finding_count"`
	DeadCodeBlockCount   int `json:"dead_code_block_count" yaml:"dead_code_block_count"`
}

// ModuleQualityMetrics is the public per-file view assembled by unified
// analysis. ModuleName is only known when dependency analysis ran.
type ModuleQualityMetrics struct {
	ModuleName              string `json:"module_name,omitempty" yaml:"module_name,omitempty"`
	FilePath                string `json:"file_path" yaml:"file_path"`
	ModuleComplexityMetrics `yaml:",inline"`
	ModuleDeadCodeMetrics   `yaml:",inline"`
}

type moduleComplexityAccumulator struct {
	metrics         ModuleComplexityMetrics
	totalComplexity int
}

// AggregateComplexityByModule derives module metrics from the complete,
// pre-filter complexity population owned by the complexity service.
func AggregateComplexityByModule(functions []FunctionComplexity) map[string]ModuleComplexityMetrics {
	modules := make(map[string]*moduleComplexityAccumulator)
	for _, function := range functions {
		key := filepath.Clean(function.FilePath)
		module := modules[key]
		if module == nil {
			module = &moduleComplexityAccumulator{}
			modules[key] = module
		}

		module.metrics.AnalyzedFunctionCount++
		module.totalComplexity += function.Metrics.Complexity
		module.metrics.ExceptionHandlerCount += function.Metrics.ExceptionHandlers
		if function.Metrics.Complexity > module.metrics.MaxComplexity {
			module.metrics.MaxComplexity = function.Metrics.Complexity
		}
		if function.RiskLevel == RiskLevelHigh {
			module.metrics.HighRiskFunctionCount++
		}
	}

	result := make(map[string]ModuleComplexityMetrics, len(modules))
	for path, module := range modules {
		if module.metrics.AnalyzedFunctionCount > 0 {
			module.metrics.AverageComplexity = float64(module.totalComplexity) / float64(module.metrics.AnalyzedFunctionCount)
		}
		result[path] = module.metrics
	}
	return result
}

// SortModuleQuality ranks the worst modules first so that a truncated report
// still shows the ones worth acting on.
func SortModuleQuality(modules []ModuleQualityMetrics) {
	sort.Slice(modules, func(i, j int) bool {
		left, right := modules[i], modules[j]
		if left.HighRiskFunctionCount != right.HighRiskFunctionCount {
			return left.HighRiskFunctionCount > right.HighRiskFunctionCount
		}
		if left.MaxComplexity != right.MaxComplexity {
			return left.MaxComplexity > right.MaxComplexity
		}
		if left.AverageComplexity != right.AverageComplexity {
			return left.AverageComplexity > right.AverageComplexity
		}
		if left.DeadCodeFindingCount != right.DeadCodeFindingCount {
			return left.DeadCodeFindingCount > right.DeadCodeFindingCount
		}
		return left.FilePath < right.FilePath
	})
}

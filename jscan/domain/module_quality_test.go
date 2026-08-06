package domain

import "testing"

func TestAggregateComplexityByModuleSummarizesEachFile(t *testing.T) {
	rollups := AggregateComplexityByModule([]FunctionComplexity{
		{
			FilePath:  "./src/a.ts",
			Metrics:   ComplexityMetrics{Complexity: 4, ExceptionHandlers: 1},
			RiskLevel: RiskLevelLow,
		},
		{
			FilePath:  "src/a.ts",
			Metrics:   ComplexityMetrics{Complexity: 12, ExceptionHandlers: 2},
			RiskLevel: RiskLevelHigh,
		},
		{
			FilePath:  "src/b.ts",
			Metrics:   ComplexityMetrics{Complexity: 2},
			RiskLevel: RiskLevelLow,
		},
	})

	if len(rollups) != 2 {
		t.Fatalf("expected 2 modules, got %d", len(rollups))
	}

	// Both spellings of the same file are one module.
	module := rollups["src/a.ts"]
	if module.AnalyzedFunctionCount != 2 {
		t.Errorf("expected 2 analyzed functions, got %d", module.AnalyzedFunctionCount)
	}
	if module.AverageComplexity != 8 {
		t.Errorf("expected average complexity 8, got %.2f", module.AverageComplexity)
	}
	if module.MaxComplexity != 12 {
		t.Errorf("expected max complexity 12, got %d", module.MaxComplexity)
	}
	if module.HighRiskFunctionCount != 1 {
		t.Errorf("expected 1 high-risk function, got %d", module.HighRiskFunctionCount)
	}
	if module.ExceptionHandlerCount != 3 {
		t.Errorf("expected 3 exception handlers, got %d", module.ExceptionHandlerCount)
	}
}

func TestAggregateComplexityByModuleWithoutFunctionsIsEmpty(t *testing.T) {
	if rollups := AggregateComplexityByModule(nil); len(rollups) != 0 {
		t.Errorf("expected no modules, got %d", len(rollups))
	}
}

func TestSortModuleQualityRanksTheWorstModulesFirst(t *testing.T) {
	modules := []ModuleQualityMetrics{
		{FilePath: "quiet.ts"},
		{FilePath: "dead.ts", ModuleDeadCodeMetrics: ModuleDeadCodeMetrics{DeadCodeFindingCount: 5}},
		{FilePath: "complex.ts", ModuleComplexityMetrics: ModuleComplexityMetrics{MaxComplexity: 30}},
		{FilePath: "risky.ts", ModuleComplexityMetrics: ModuleComplexityMetrics{HighRiskFunctionCount: 1, MaxComplexity: 21}},
	}

	SortModuleQuality(modules)

	expected := []string{"risky.ts", "complex.ts", "dead.ts", "quiet.ts"}
	for index, filePath := range expected {
		if modules[index].FilePath != filePath {
			t.Errorf("expected %q at position %d, got %q", filePath, index, modules[index].FilePath)
		}
	}
}

package domain

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func directoryFunction(filePath string, complexity, nesting int, risk RiskLevel) FunctionComplexity {
	return FunctionComplexity{
		Name:     "fn",
		FilePath: filePath,
		Metrics: ComplexityMetrics{
			Complexity:   complexity,
			NestingDepth: nesting,
		},
		RiskLevel: risk,
	}
}

func TestComplexityDirectoryRootIsTheCommonAncestorOfTheAnalyzedFiles(t *testing.T) {
	root, err := ComplexityDirectoryRoot([]string{"src/a/one.ts", "src/b/two.ts", "src/index.ts"})
	if err != nil {
		t.Fatalf("ComplexityDirectoryRoot failed: %v", err)
	}

	expected, err := filepath.Abs("src")
	if err != nil {
		t.Fatalf("failed to resolve expected root: %v", err)
	}
	if root != expected {
		t.Errorf("expected root %q, got %q", expected, root)
	}
}

func TestComplexityDirectoryRootOfOneFileIsItsDirectory(t *testing.T) {
	root, err := ComplexityDirectoryRoot([]string{"src/a/one.ts"})
	if err != nil {
		t.Fatalf("ComplexityDirectoryRoot failed: %v", err)
	}

	expected, err := filepath.Abs(filepath.Join("src", "a"))
	if err != nil {
		t.Fatalf("failed to resolve expected root: %v", err)
	}
	if root != expected {
		t.Errorf("expected root %q, got %q", expected, root)
	}
}

func TestComplexityDirectoryRootRequiresAtLeastOneFile(t *testing.T) {
	if _, err := ComplexityDirectoryRoot(nil); err == nil {
		t.Error("expected an error when no files were analyzed")
	}
}

func TestAggregateComplexityByDirectoryGroupsByDirectAncestor(t *testing.T) {
	root, err := filepath.Abs("src")
	if err != nil {
		t.Fatalf("failed to resolve root: %v", err)
	}

	directories, err := AggregateComplexityByDirectory([]FunctionComplexity{
		directoryFunction("src/a/one.ts", 4, 2, RiskLevelLow),
		directoryFunction("src/a/two.ts", 10, 4, RiskLevelHigh),
		directoryFunction("src/index.ts", 2, 1, RiskLevelLow),
	}, root)
	if err != nil {
		t.Fatalf("AggregateComplexityByDirectory failed: %v", err)
	}

	if len(directories) != 2 {
		t.Fatalf("expected 2 directories, got %d", len(directories))
	}

	// The high-risk directory ranks first.
	worst := directories[0]
	if worst.DirectoryPath != "a" {
		t.Errorf("expected directory %q first, got %q", "a", worst.DirectoryPath)
	}
	if worst.FunctionCount != 2 {
		t.Errorf("expected 2 functions, got %d", worst.FunctionCount)
	}
	if worst.AverageComplexity != 7 {
		t.Errorf("expected average complexity 7, got %.2f", worst.AverageComplexity)
	}
	if worst.MaxComplexity != 10 {
		t.Errorf("expected max complexity 10, got %d", worst.MaxComplexity)
	}
	if worst.HighRiskFunctionCount != 1 {
		t.Errorf("expected 1 high-risk function, got %d", worst.HighRiskFunctionCount)
	}
	if worst.AverageNestingDepth != 3 {
		t.Errorf("expected average nesting depth 3, got %.2f", worst.AverageNestingDepth)
	}
	if worst.MaxNestingDepth != 4 {
		t.Errorf("expected max nesting depth 4, got %d", worst.MaxNestingDepth)
	}

	if directories[1].DirectoryPath != "." {
		t.Errorf("expected the root directory to be reported as %q, got %q", ".", directories[1].DirectoryPath)
	}
}

func TestAggregateComplexityByDirectoryRejectsFilesOutsideTheRoot(t *testing.T) {
	root, err := filepath.Abs("src")
	if err != nil {
		t.Fatalf("failed to resolve root: %v", err)
	}

	if _, err := AggregateComplexityByDirectory([]FunctionComplexity{
		directoryFunction("vendor/other.ts", 1, 0, RiskLevelLow),
	}, root); err == nil {
		t.Error("expected an error for a function outside the directory root")
	}
}

func TestAggregateComplexityByDirectoryWithoutFunctionsIsEmpty(t *testing.T) {
	directories, err := AggregateComplexityByDirectory(nil, "src")
	if err != nil {
		t.Fatalf("AggregateComplexityByDirectory failed: %v", err)
	}
	if len(directories) != 0 {
		t.Errorf("expected no directories, got %d", len(directories))
	}
}

func TestDirectoryComplexityMetricsListEncodesNilAsEmptyArray(t *testing.T) {
	encoded, err := json.Marshal(struct {
		ByDirectory DirectoryComplexityMetricsList `json:"by_directory"`
	}{})
	if err != nil {
		t.Fatalf("failed to encode: %v", err)
	}

	if string(encoded) != `{"by_directory":[]}` {
		t.Errorf("expected an empty array, got %s", encoded)
	}
}

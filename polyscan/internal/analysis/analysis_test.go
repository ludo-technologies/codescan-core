package analysis

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ludo-technologies/polyscan/core/domain"
)

// fixtures copies the Go fixtures into a temporary directory and adds a
// file that does not parse. The broken file is generated here rather than
// committed so that gofmt and go vet never trip over it.
func fixtures(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	sample, err := os.ReadFile("../../testdata/go/sample.go")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sample.go"), sample, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "broken.go"), []byte("package broken\n\nfunc Unclosed() {\n\tif true {\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestAnalyzeComplexity(t *testing.T) {
	report, err := Analyze([]string{fixtures(t)}, Options{Complexity: true})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	if report.Files != (Files{Total: 2, Analyzed: 1, Skipped: 1}) {
		t.Errorf("files = %+v, want 2/1/1", report.Files)
	}
	if len(report.Errors) != 1 || !strings.Contains(report.Errors[0], "broken.go: syntax error") {
		t.Errorf("errors = %v, want the broken file's syntax error", report.Errors)
	}
	if report.Clones != nil {
		t.Error("clones were not selected but are present")
	}

	summary := report.Complexity.Summary
	if summary.TotalFunctions != 6 || summary.MaxComplexity != 8 || summary.MinComplexity != 1 {
		t.Errorf("summary = %+v", summary)
	}
	if summary.LowRiskFunctions != 6 || summary.MediumRiskFunctions != 0 || summary.HighRiskFunctions != 0 {
		t.Errorf("risk distribution = %d/%d/%d, want 6/0/0", summary.LowRiskFunctions, summary.MediumRiskFunctions, summary.HighRiskFunctions)
	}

	functions := report.Complexity.Functions
	first := functions[0]
	if first.Name != "Server.Handle" || first.Complexity != 8 {
		t.Errorf("first function = %s (%d), want Server.Handle (8)", first.Name, first.Complexity)
	}
	if first.Language != "Go" || filepath.Base(first.FilePath) != "sample.go" {
		t.Errorf("first function language=%q path=%q", first.Language, first.FilePath)
	}
	for i := 1; i < len(functions); i++ {
		if functions[i].Complexity > functions[i-1].Complexity {
			t.Errorf("functions are not sorted by descending complexity at %d", i)
		}
	}
}

func TestAnalyzeClones(t *testing.T) {
	report, err := Analyze([]string{"../../testdata/go/clones"}, Options{Clones: true})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if report.Complexity != nil {
		t.Error("complexity was not selected but is present")
	}

	// sum_test.go holds a third copy but test files are not compared.
	stats := report.Clones.Statistics
	if stats.TotalFragments != 3 || stats.TotalClonePairs != 3 || stats.TotalCloneGroups != 1 || stats.TotalClones != 3 {
		t.Errorf("statistics = %+v", stats)
	}
	if stats.ClonesByType["Type-1"] != 1 {
		t.Errorf("clones by type = %v, want one Type-1 pair for the exact copy", stats.ClonesByType)
	}

	exact := report.Clones.Pairs[0]
	if exact.Type != domain.Type1Clone || exact.Fragment1.Name != "SumPositive" || exact.Fragment2.Name != "SumPositive" {
		t.Errorf("strongest pair = %+v, want the Type-1 SumPositive pair", exact)
	}
	if filepath.Dir(exact.Fragment1.FilePath) == filepath.Dir(exact.Fragment2.FilePath) {
		t.Errorf("pair paths = %q, %q, want the copies in different directories", exact.Fragment1.FilePath, exact.Fragment2.FilePath)
	}
	if group := report.Clones.Groups[0]; len(group.Fragments) != 3 {
		t.Errorf("group = %+v, want all three functions", group)
	}
}

func TestAnalyzeWithoutSupportedFiles(t *testing.T) {
	if _, err := Analyze([]string{t.TempDir()}, Options{Complexity: true}); err == nil || !strings.Contains(err.Error(), "no supported source files") {
		t.Errorf("err = %v, want no supported source files", err)
	}
}

func TestRiskLevel(t *testing.T) {
	cases := map[int]domain.RiskLevel{
		1: domain.RiskLevelLow, 9: domain.RiskLevelLow,
		10: domain.RiskLevelMedium, 19: domain.RiskLevelMedium,
		20: domain.RiskLevelHigh,
	}
	for complexity, want := range cases {
		if got := RiskLevel(complexity); got != want {
			t.Errorf("RiskLevel(%d) = %s, want %s", complexity, got, want)
		}
	}
}

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

func TestAnalyzeTestdata(t *testing.T) {
	report, err := Analyze([]string{fixtures(t)})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	summary := report.Summary
	if summary.TotalFiles != 2 || summary.FilesAnalyzed != 1 || summary.SkippedFiles != 1 {
		t.Errorf("files: total=%d analyzed=%d skipped=%d, want 2/1/1", summary.TotalFiles, summary.FilesAnalyzed, summary.SkippedFiles)
	}
	if len(report.Errors) != 1 || !strings.Contains(report.Errors[0], "broken.go: syntax error") {
		t.Errorf("errors = %v, want the broken file's syntax error", report.Errors)
	}
	if summary.TotalFunctions != 6 || summary.MaxComplexity != 8 || summary.MinComplexity != 1 {
		t.Errorf("summary = %+v", summary)
	}
	if summary.LowRiskFunctions != 6 || summary.MediumRiskFunctions != 0 || summary.HighRiskFunctions != 0 {
		t.Errorf("risk distribution = %d/%d/%d, want 6/0/0", summary.LowRiskFunctions, summary.MediumRiskFunctions, summary.HighRiskFunctions)
	}

	first := report.Functions[0]
	if first.Name != "Server.Handle" || first.Complexity != 8 {
		t.Errorf("first function = %s (%d), want Server.Handle (8)", first.Name, first.Complexity)
	}
	if first.Language != "Go" || filepath.Base(first.FilePath) != "sample.go" {
		t.Errorf("first function language=%q path=%q", first.Language, first.FilePath)
	}
	for i := 1; i < len(report.Functions); i++ {
		if report.Functions[i].Complexity > report.Functions[i-1].Complexity {
			t.Errorf("functions are not sorted by descending complexity at %d", i)
		}
	}
}

func TestAnalyzeWithoutSupportedFiles(t *testing.T) {
	if _, err := Analyze([]string{t.TempDir()}); err == nil || !strings.Contains(err.Error(), "no supported source files") {
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

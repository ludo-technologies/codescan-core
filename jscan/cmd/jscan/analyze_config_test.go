package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ludo-technologies/polyscan/jscan/internal/config"
)

// complexityFixture defines three functions with complexity 1, 2 and 3 so that
// min_complexity filtering can be observed without relying on report_unchanged.
const complexityFixture = `
function trivial(a) {
  return a;
}

function branching(a) {
  if (a) {
    return 1;
  }
  return 0;
}

function nested(a, b) {
  if (a) {
    return 1;
  }
  if (b) {
    return 2;
  }
  return 0;
}
`

func writeComplexityFixture(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "fixture.js")
	if err := os.WriteFile(path, []byte(complexityFixture), 0o644); err != nil {
		t.Fatalf("failed to write fixture: %v", err)
	}
	return path
}

// TestRunComplexityAnalysisInternal_MinComplexityFromConfig ensures output.min_complexity
// from the configuration file reaches the complexity request (regression for #22).
func TestRunComplexityAnalysisInternal_MinComplexityFromConfig(t *testing.T) {
	file := writeComplexityFixture(t)

	cfg := config.DefaultConfig()
	cfg.Complexity.ReportUnchanged = true

	baseline, err := runComplexityAnalysisInternal([]string{file}, cfg)
	if err != nil {
		t.Fatalf("baseline analysis failed: %v", err)
	}
	if len(baseline.Functions) != 3 {
		t.Fatalf("fixture should yield 3 functions, got %d", len(baseline.Functions))
	}

	cfg.Output.MinComplexity = 3
	filtered, err := runComplexityAnalysisInternal([]string{file}, cfg)
	if err != nil {
		t.Fatalf("filtered analysis failed: %v", err)
	}

	for _, fn := range filtered.Functions {
		if fn.Metrics.Complexity < cfg.Output.MinComplexity {
			t.Errorf("function %s with complexity %d should have been filtered out",
				fn.Name, fn.Metrics.Complexity)
		}
	}
	if len(filtered.Functions) >= len(baseline.Functions) {
		t.Errorf("min_complexity=3 should drop functions: got %d of %d",
			len(filtered.Functions), len(baseline.Functions))
	}
	if filtered.Summary.TotalFunctions != len(filtered.Functions) {
		t.Errorf("TotalFunctions should be the post-filter count, got %d for %d functions",
			filtered.Summary.TotalFunctions, len(filtered.Functions))
	}
	if filtered.Summary.FunctionsParsed != len(baseline.Functions) {
		t.Errorf("FunctionsParsed should stay at the pre-filter count %d, got %d",
			len(baseline.Functions), filtered.Summary.FunctionsParsed)
	}
}

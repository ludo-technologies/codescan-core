package report

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/ludo-technologies/polyscan/core/domain"
	"github.com/ludo-technologies/polyscan/polyscan/internal/analysis"
)

func document(minComplexity int) *Document {
	return &Document{
		Version:     "test",
		GeneratedAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		Complexity: &analysis.Report{
			Functions: []analysis.Function{
				{Name: "Big", FilePath: "a.go", StartLine: 1, EndLine: 40, Complexity: 25, Decisions: map[string]int{"branch": 24}, RiskLevel: domain.RiskLevelHigh},
				{Name: "Small", FilePath: "b.go", StartLine: 3, EndLine: 5, Complexity: 1, Decisions: map[string]int{}, RiskLevel: domain.RiskLevelLow},
			},
			Summary: analysis.Summary{TotalFunctions: 2, AverageComplexity: 13, MaxComplexity: 25, MinComplexity: 1, FilesAnalyzed: 2, TotalFiles: 3, SkippedFiles: 1, LowRiskFunctions: 1, HighRiskFunctions: 1},
			Errors:  []string{"c.go: syntax error at line 9"},
		},
		MinComplexity: minComplexity,
	}
}

func TestWriteText(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, document(1), domain.OutputFormatText); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"=== Complexity Analysis ===",
		"Generated: 2026-01-02T03:04:05Z",
		"Files analyzed: 2",
		"Files skipped: 1",
		"High risk: 1",
		"Functions:\n  Big: 25 [HIGH]\n    File: a.go:1-40\n  Small: 1\n    File: b.go:3-5\n",
		"Errors:\n  - c.go: syntax error at line 9\n",
	} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("text output lacks %q:\n%s", want, buf.String())
		}
	}
}

func TestWriteTextFiltersListedFunctionsOnly(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, document(10), domain.OutputFormatText); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "Functions (complexity >= 10):") || strings.Contains(out, "Small") {
		t.Errorf("filtered text output is wrong:\n%s", out)
	}
	if !strings.Contains(out, "Total functions: 2") {
		t.Errorf("summary must cover every function:\n%s", out)
	}
}

func TestWriteJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, document(10), domain.OutputFormatJSON); err != nil {
		t.Fatal(err)
	}
	var decoded Document
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if len(decoded.Complexity.Functions) != 1 || decoded.Complexity.Functions[0].Name != "Big" {
		t.Errorf("functions = %+v, want only Big", decoded.Complexity.Functions)
	}
	if decoded.Complexity.Summary.TotalFunctions != 2 {
		t.Errorf("summary total = %d, want 2", decoded.Complexity.Summary.TotalFunctions)
	}
	if !strings.Contains(buf.String(), `"decisions": {`) || !strings.Contains(buf.String(), `"risk_level": "high"`) {
		t.Errorf("JSON lacks expected fields:\n%s", buf.String())
	}
}

func TestWriteRejectsUnknownFormat(t *testing.T) {
	if err := Write(&bytes.Buffer{}, document(1), domain.OutputFormatHTML); err == nil {
		t.Error("expected an error for html")
	}
}

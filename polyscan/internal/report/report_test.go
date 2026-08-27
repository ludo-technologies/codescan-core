package report

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/ludo-technologies/polyscan/core/domain"
	"github.com/ludo-technologies/polyscan/polyscan/internal/analysis"
	"github.com/ludo-technologies/polyscan/polyscan/internal/clone"
)

func document(minComplexity int) *Document {
	a := clone.Fragment{ID: 0, Name: "Big", FilePath: "a.go", StartLine: 1, EndLine: 40, LineCount: 40, NodeCount: 90, Content: "func Big() {\n\tif a < b {\n\t}\n}"}
	b := clone.Fragment{ID: 1, Name: "Copy", FilePath: "c.go", StartLine: 10, EndLine: 49, LineCount: 40, NodeCount: 90}
	return &Document{
		Version:     "test",
		GeneratedAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		Report: &analysis.Report{
			Files: analysis.Files{Total: 3, Analyzed: 2, Skipped: 1},
			Complexity: &analysis.Complexity{
				Functions: []analysis.Function{
					{Name: "Big", FilePath: "a.go", StartLine: 1, EndLine: 40, Complexity: 25, Decisions: map[string]int{"branch": 24}, RiskLevel: domain.RiskLevelHigh},
					{Name: "Small", FilePath: "b.go", StartLine: 3, EndLine: 5, Complexity: 1, Decisions: map[string]int{}, RiskLevel: domain.RiskLevelLow},
				},
				Summary: analysis.ComplexitySummary{TotalFunctions: 2, AverageComplexity: 13, MaxComplexity: 25, MinComplexity: 1, LowRiskFunctions: 1, HighRiskFunctions: 1},
			},
			Clones: &clone.Report{
				Pairs:      []clone.Pair{{ID: 0, Type: domain.Type1Clone, Similarity: 1, Confidence: 1, Fragment1: a, Fragment2: b}},
				Groups:     []clone.Group{{ID: 0, Type: domain.Type1Clone, Similarity: 1, Fragments: []clone.Fragment{a, b}}},
				Statistics: clone.Statistics{TotalFragments: 2, TotalClones: 2, TotalClonePairs: 1, TotalCloneGroups: 1, ClonesByType: map[string]int{"Type-1": 1}, AverageSimilarity: 1},
			},
			Errors: []string{"d.go: syntax error at line 9"},
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
		"Generated: 2026-01-02T03:04:05Z",
		"Files analyzed: 2\nFiles skipped: 1\n",
		"=== Complexity Analysis ===",
		"High risk: 1",
		"Functions:\n  Big: 25 [HIGH]\n    File: a.go:1-40\n  Small: 1\n    File: b.go:3-5\n",
		"=== Clone Detection ===",
		"Total clone pairs: 1",
		"Clone Types:\n  Type-1: 1\n",
		"Group 1: Type-1, 2 fragments, 100.0% similar\n    Big (a.go:1-40)\n    Copy (c.go:10-49)\n",
		"Top Clone Pairs:\n  Type-1: Big (a.go:1-40) <-> Copy (c.go:10-49) (100.0% similar)\n",
		"Errors:\n  - d.go: syntax error at line 9\n",
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

func TestWriteTextWithoutClones(t *testing.T) {
	doc := document(1)
	doc.Clones = nil
	var buf bytes.Buffer
	if err := Write(&buf, doc, domain.OutputFormatText); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "Clone Detection") {
		t.Errorf("unselected clone section is present:\n%s", buf.String())
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
	if decoded.Complexity.Summary.TotalFunctions != 2 || decoded.Files.Skipped != 1 {
		t.Errorf("summary total = %d, files = %+v", decoded.Complexity.Summary.TotalFunctions, decoded.Files)
	}
	if len(decoded.Clones.Pairs) != 1 || decoded.Clones.Pairs[0].Fragment2.Name != "Copy" {
		t.Errorf("clone pairs = %+v", decoded.Clones.Pairs)
	}
	for _, want := range []string{`"decisions": {`, `"risk_level": "high"`, `"clone": {`, `"clones_by_type": {`} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("JSON lacks %s:\n%s", want, buf.String())
		}
	}
}

func TestWriteHTML(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, document(10), domain.OutputFormatHTML); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		"<title>polyscan report</title>",
		"2 of 3 files analyzed, 1 skipped",
		`<td class="mono">Big</td>`,
		"complexity 10 and above",
		"Type-1 Exact · 1 pairs",
		"Group 1",
		"if a &lt; b {", // the preview is escaped
		"d.go: syntax error at line 9",
		"CC 20&#43;: 1 functions", // html/template escapes the plus
	} {
		if !strings.Contains(out, want) {
			t.Errorf("HTML lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Small") {
		t.Errorf("HTML lists a function below the filter:\n%s", out)
	}
}

func TestWriteRejectsUnknownFormat(t *testing.T) {
	if err := Write(&bytes.Buffer{}, document(1), domain.OutputFormatYAML); err == nil {
		t.Error("expected an error for yaml")
	}
}

func TestHistogramBins(t *testing.T) {
	functions := []analysis.Function{{Complexity: 1}, {Complexity: 3}, {Complexity: 9}, {Complexity: 10}, {Complexity: 19}, {Complexity: 20}, {Complexity: 20}}
	hist := buildHistogram(functions)
	counts := []int{}
	for _, bin := range hist.Bins {
		counts = append(counts, bin.Count)
	}
	if want := []int{1, 1, 1, 2, 2}; len(counts) != len(want) || counts[0] != 1 || counts[1] != 1 || counts[2] != 1 || counts[3] != 2 || counts[4] != 2 {
		t.Errorf("bin counts = %v, want %v", counts, want)
	}
	if hist.Bins[3].Band != "warn" || hist.Bins[4].Band != "bad" || hist.Bins[2].Band != "" {
		t.Errorf("bands = %+v", hist.Bins)
	}
	if buildHistogram(nil) != nil {
		t.Error("empty population must have no histogram")
	}
}

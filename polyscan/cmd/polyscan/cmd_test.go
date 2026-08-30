package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func run(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := rootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

// analyzeJSON is the part of the unified JSON shape the tests read.
type analyzeJSON struct {
	Complexity *struct {
		Functions []struct {
			Name     string `json:"name"`
			Language string `json:"language"`
		} `json:"functions"`
		Summary struct {
			TotalFunctions int `json:"total_functions"`
			TotalFiles     int `json:"total_files"`
			SkippedFiles   int `json:"skipped_files"`
		} `json:"summary"`
	} `json:"complexity"`
	DeadCode json.RawMessage `json:"dead_code"`
	Clone    *struct {
		ClonePairs []struct {
			Clone1 struct {
				Language string `json:"language"`
			} `json:"clone1"`
		} `json:"clone_pairs"`
		Statistics struct {
			TotalClonePairs int `json:"total_clone_pairs"`
		} `json:"statistics"`
	} `json:"clone"`
	Summary *struct {
		HealthScore       int    `json:"health_score"`
		Grade             string `json:"grade"`
		ComplexityEnabled bool   `json:"complexity_enabled"`
		DeadCodeEnabled   bool   `json:"dead_code_enabled"`
		CloneEnabled      bool   `json:"clone_enabled"`
	} `json:"summary"`
}

func TestAnalyzeText(t *testing.T) {
	out, err := run(t, "analyze", "--format", "text", "../../testdata/go/sample.go")
	if err != nil {
		t.Fatalf("analyze: %v\n%s", err, out)
	}
	for _, want := range []string{"polyscan Analysis Report", "Server.Handle: 8", "Health Score"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
}

func TestAnalyzeJSON(t *testing.T) {
	out, err := run(t, "analyze", "--format", "json", "--min-complexity", "5", "../../testdata/go")
	if err != nil {
		t.Fatalf("analyze: %v\n%s", err, out)
	}
	doc := decodeAnalyzeJSON(t, out)
	if len(doc.Complexity.Functions) != 3 || doc.Complexity.Summary.TotalFunctions != 10 {
		t.Errorf("listed %d functions of %d, want 3 of 10",
			len(doc.Complexity.Functions), doc.Complexity.Summary.TotalFunctions)
	}
	for _, fn := range doc.Complexity.Functions {
		if fn.Language != "Go" {
			t.Errorf("function %s has language %q, want Go", fn.Name, fn.Language)
		}
	}
	if doc.Clone == nil || doc.Clone.Statistics.TotalClonePairs != 3 {
		t.Errorf("clone = %+v, want 3 pairs", doc.Clone)
	}
	for _, pair := range doc.Clone.ClonePairs {
		if pair.Clone1.Language != "Go" {
			t.Errorf("clone fragment has language %q, want Go", pair.Clone1.Language)
		}
	}
	if doc.Summary == nil || doc.Summary.Grade == "" {
		t.Errorf("summary = %+v, want a health score", doc.Summary)
	}
}

// decodeAnalyzeJSON parses the analyze output, which carries the CLI summary
// after the JSON document on structured formats.
func decodeAnalyzeJSON(t *testing.T, out string) *analyzeJSON {
	t.Helper()
	doc := &analyzeJSON{}
	if err := json.NewDecoder(strings.NewReader(out)).Decode(doc); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	return doc
}

func TestAnalyzeHTML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.html")
	out, err := run(t, "analyze", "--no-open", "-o", path, "../../testdata/go")
	if err != nil {
		t.Fatalf("analyze: %v\n%s", err, out)
	}
	if !strings.Contains(out, "HTML report written to ") || !strings.Contains(out, "Health Score:") {
		t.Errorf("unexpected output:\n%s", out)
	}
	html, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"<title>polyscan Analysis Report", "sample.go", "Duplication"} {
		if !strings.Contains(string(html), want) {
			t.Errorf("HTML report lacks %q", want)
		}
	}
}

func TestAnalyzeSelect(t *testing.T) {
	out, err := run(t, "analyze", "--format", "text", "--select", "clone", "../../testdata/go/clones")
	if err != nil {
		t.Fatalf("analyze: %v\n%s", err, out)
	}
	if strings.Contains(out, "Complexity Analysis") || !strings.Contains(out, "Clone Detection") {
		t.Errorf("unexpected output:\n%s", out)
	}
	// Deselected dimensions are left out of the health score, so their
	// category lines must not appear as clean results.
	if strings.Contains(out, "Complexity: ") || !strings.Contains(out, "Code Duplication:") {
		t.Errorf("category scores should list only the selected analyses:\n%s", out)
	}
}

func TestAnalyzeRejectsBadArguments(t *testing.T) {
	for _, args := range [][]string{
		{"analyze"},
		{"analyze", "--select", "lint", "../../testdata/go"},
		{"analyze", "--select=", "../../testdata/go"},
		{"analyze", "--format", "yaml", "../../testdata/go"},
		{"analyze", "--min-complexity", "0", "../../testdata/go"},
		{"analyze", "does-not-exist"},
	} {
		if _, err := run(t, args...); err == nil {
			t.Errorf("%v: expected an error", args)
		}
	}
}

// TestAnalyzeJavaScriptOnlySelection covers a selection that leaves nothing
// for the generic engine: a Go-only tree then has no analysis at all.
func TestAnalyzeJavaScriptOnlySelection(t *testing.T) {
	if _, err := run(t, "analyze", "--select", "deadcode", "../../testdata/go"); err == nil {
		t.Error("a JavaScript-only selection over a Go tree should find no files")
	}
	out, err := run(t, "analyze", "--format", "text", "--select", "deadcode", "../../testdata/javascript")
	if err != nil {
		t.Fatalf("analyze: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Dead Code Analysis") {
		t.Errorf("unexpected output:\n%s", out)
	}
}

func TestVersion(t *testing.T) {
	out, err := run(t, "version")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, "polyscan version ") {
		t.Errorf("unexpected output %q", out)
	}
}

func TestAnalyzeJavaScriptOnly(t *testing.T) {
	out, err := run(t, "analyze", "--format", "text", "../../testdata/javascript")
	if err != nil {
		t.Fatalf("analyze: %v\n%s", err, out)
	}
	if !strings.Contains(out, "polyscan Analysis Report") || !strings.Contains(out, "Health Score") {
		t.Errorf("expected the unified text report, got:\n%s", out)
	}
}

func TestAnalyzeIgnoresJSConfigWithoutJSFiles(t *testing.T) {
	dir := t.TempDir()
	src, err := os.ReadFile("../../testdata/go/sample.go")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sample.go"), src, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "jscan.config.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := run(t, "analyze", "--format", "text", dir)
	if err != nil {
		t.Fatalf("a JavaScript configuration must not fail a tree without JavaScript: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Server.Handle: 8") {
		t.Errorf("unexpected output:\n%s", out)
	}
}

func TestAnalyzeMixedTreeJSON(t *testing.T) {
	out, err := run(t, "analyze", "--format", "json", "../../testdata")
	if err != nil {
		t.Fatalf("analyze: %v\n%s", err, out)
	}
	doc := decodeAnalyzeJSON(t, out)
	languages := map[string]int{}
	for _, fn := range doc.Complexity.Functions {
		languages[fn.Language]++
	}
	for _, language := range []string{"Go", "Rust", "C++", "JavaScript"} {
		if languages[language] == 0 {
			t.Errorf("no %s function in the unified complexity report, got %v", language, languages)
		}
	}
	if len(doc.DeadCode) == 0 {
		t.Errorf("the unified report should carry the JavaScript dead code analysis")
	}
	if doc.Summary == nil || !doc.Summary.ComplexityEnabled || !doc.Summary.DeadCodeEnabled || !doc.Summary.CloneEnabled {
		t.Errorf("summary = %+v, want every run dimension enabled", doc.Summary)
	}
}

func TestAnalyzeMixedTreeHTML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.html")
	out, err := run(t, "analyze", "--no-open", "-o", path, "../../testdata")
	if err != nil {
		t.Fatalf("analyze: %v\n%s", err, out)
	}
	if !strings.Contains(out, "HTML report written to ") {
		t.Errorf("unexpected output:\n%s", out)
	}
	if strings.Contains(out, "JavaScript HTML report") {
		t.Errorf("the report is unified; there should be no separate JavaScript report:\n%s", out)
	}
	html, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"<title>polyscan Analysis Report", "sample.go", "deadcode.js"} {
		if !strings.Contains(string(html), want) {
			t.Errorf("HTML report lacks %q", want)
		}
	}
}

func TestFileURL(t *testing.T) {
	for path, want := range map[string]string{
		"/tmp/report.html":        "file:///tmp/report.html",
		"/tmp/a b#1/report?.html": "file:///tmp/a%20b%231/report%3F.html",
	} {
		if got := fileURL(path); got != want {
			t.Errorf("fileURL(%q) = %q, want %q", path, got, want)
		}
	}
}

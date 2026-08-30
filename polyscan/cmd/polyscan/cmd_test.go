package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ludo-technologies/polyscan/polyscan/internal/report"
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

func TestAnalyzeText(t *testing.T) {
	out, err := run(t, "analyze", "--format", "text", "../../testdata/go/sample.go")
	if err != nil {
		t.Fatalf("analyze: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Server.Handle: 8") {
		t.Errorf("unexpected output:\n%s", out)
	}
}

func TestAnalyzeJSON(t *testing.T) {
	out, err := run(t, "analyze", "--format", "json", "--min-complexity", "5", "../../testdata/go")
	if err != nil {
		t.Fatalf("analyze: %v\n%s", err, out)
	}
	var doc report.Document
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if len(doc.Complexity.Functions) != 3 || doc.Complexity.Summary.TotalFunctions != 10 || doc.Files.Skipped != 0 {
		t.Errorf("listed %d functions of %d, want 3 of 10", len(doc.Complexity.Functions), doc.Complexity.Summary.TotalFunctions)
	}
	if doc.Clones == nil || doc.Clones.Statistics.TotalClonePairs != 3 {
		t.Errorf("clones = %+v, want 3 pairs", doc.Clones)
	}
}

func TestAnalyzeHTML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.html")
	out, err := run(t, "analyze", "--no-open", "-o", path, "../../testdata/go")
	if err != nil {
		t.Fatalf("analyze: %v\n%s", err, out)
	}
	if !strings.Contains(out, "HTML report written to ") {
		t.Errorf("unexpected output:\n%s", out)
	}
	html, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"<title>polyscan report</title>", "Server.Handle", "Clone groups", "SumPositive"} {
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
}

func TestAnalyzeRejectsBadArguments(t *testing.T) {
	for _, args := range [][]string{
		{"analyze"},
		{"analyze", "--select", "deadcode", "../../testdata/go"},
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
	if !strings.Contains(out, "jscan Analysis Report") || !strings.Contains(out, "Health Score") {
		t.Errorf("expected jscan's text report, got:\n%s", out)
	}
	if strings.Contains(out, "=== polyscan ===") {
		t.Errorf("a JavaScript-only tree should not print an empty polyscan report:\n%s", out)
	}
}

func TestAnalyzeMixedTreeJSON(t *testing.T) {
	out, err := run(t, "analyze", "--format", "json", "../../testdata")
	if err != nil {
		t.Fatalf("analyze: %v\n%s", err, out)
	}
	var doc report.Document
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if doc.Report == nil || doc.Complexity.Summary.TotalFunctions == 0 {
		t.Errorf("the engine languages should keep their report, got %+v", doc.Report)
	}
	var javascript struct {
		Complexity *struct {
			Summary struct {
				TotalFunctions int `json:"total_functions"`
			} `json:"summary"`
		} `json:"complexity"`
		DeadCode json.RawMessage `json:"dead_code"`
	}
	if err := json.Unmarshal(doc.JavaScript, &javascript); err != nil {
		t.Fatalf("invalid javascript section: %v\n%s", err, doc.JavaScript)
	}
	if javascript.Complexity == nil || javascript.Complexity.Summary.TotalFunctions == 0 {
		t.Errorf("the javascript section should carry jscan's complexity output:\n%s", doc.JavaScript)
	}
	if len(javascript.DeadCode) == 0 {
		t.Errorf("the javascript section should carry jscan's dead code output:\n%s", doc.JavaScript)
	}
}

func TestAnalyzeMixedTreeHTML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.html")
	out, err := run(t, "analyze", "--no-open", "-o", path, "../../testdata")
	if err != nil {
		t.Fatalf("analyze: %v\n%s", err, out)
	}
	if !strings.Contains(out, "HTML report written to ") || !strings.Contains(out, "JavaScript HTML report written to ") {
		t.Errorf("unexpected output:\n%s", out)
	}
	html, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(html), "<title>polyscan report</title>") {
		t.Errorf("the main report should stay the polyscan one")
	}
	jsHTML, err := os.ReadFile(jsReportPath(path))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(jsHTML), "jscan Analysis Report") {
		t.Errorf("the JavaScript report should be jscan's")
	}
}

func TestJSReportPath(t *testing.T) {
	if got := jsReportPath("polyscan-report.html"); got != "polyscan-report.js.html" {
		t.Errorf("jsReportPath = %q", got)
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

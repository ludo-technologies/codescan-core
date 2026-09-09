package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
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
	CBO      *struct {
		Classes []struct {
			Name     string `json:"Name"`
			Language string `json:"language"`
			Metrics  struct {
				CouplingCount    int      `json:"CouplingCount"`
				DependentClasses []string `json:"DependentClasses"`
			} `json:"Metrics"`
		} `json:"classes"`
		Summary struct {
			TotalClasses int `json:"TotalClasses"`
		} `json:"summary"`
	} `json:"cbo"`
	Clone *struct {
		ClonePairs []struct {
			Clone1 struct {
				Language string `json:"language"`
			} `json:"clone1"`
		} `json:"clone_pairs"`
		Statistics struct {
			TotalClonePairs int `json:"total_clone_pairs"`
		} `json:"statistics"`
	} `json:"clone"`
	Deps *struct {
		Analysis struct {
			TotalModules int `json:"TotalModules"`
			MaxDepth     int `json:"MaxDepth"`
		} `json:"analysis"`
		Warnings []string `json:"warnings"`
	} `json:"deps"`
	LCOM *struct {
		Classes []struct {
			Name     string `json:"name"`
			Language string `json:"language"`
			Metrics  struct {
				LCOM4 int `json:"lcom4"`
			} `json:"metrics"`
		} `json:"classes"`
		Summary struct {
			TotalClasses int `json:"total_classes"`
		} `json:"summary"`
	} `json:"lcom"`
	Summary *struct {
		HealthScore       int    `json:"health_score"`
		CohesionScore     int    `json:"cohesion_score"`
		LCOMEnabled       bool   `json:"lcom_enabled"`
		CBOEnabled        bool   `json:"cbo_enabled"`
		CBOClasses        int    `json:"cbo_classes"`
		DependencyScore   int    `json:"dependency_score"`
		DepsEnabled       bool   `json:"deps_enabled"`
		Grade             string `json:"grade"`
		TotalFiles        int    `json:"total_files"`
		SkippedFiles      int    `json:"skipped_files"`
		ComplexityEnabled bool   `json:"complexity_enabled"`
		DeadCodeEnabled   bool   `json:"dead_code_enabled"`
		CloneEnabled      bool   `json:"clone_enabled"`
	} `json:"summary"`
}

func TestRuntimeErrorDoesNotPrintUsage(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	out, err := run(t, "analyze", missing)
	if err == nil {
		t.Fatal("analyze returned nil error for a missing path")
	}
	if strings.Contains(out, "Usage:") {
		t.Fatalf("runtime error printed usage:\n%s", out)
	}
}

func TestInvalidAnalysisReturnsClearError(t *testing.T) {
	_, err := run(t, "analyze", "--select", "foo", "../../testdata/go")
	if err == nil {
		t.Fatal("invalid analysis returned nil error")
	}
	if !strings.Contains(err.Error(), "foo") {
		t.Fatalf("error %q does not identify the invalid analysis", err)
	}
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

// TestAnalyzeChargesParseErrorsWithoutComplexity covers #92: a run that
// leaves complexity out still reports and charges the files it could not
// parse.
func TestAnalyzeChargesParseErrorsWithoutComplexity(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string]string{
		"ok.js":     "export function ok(a) { return a; }\n",
		"broken.js": "function broken( {\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	out, err := run(t, "analyze", "--format", "json", "--select", "deadcode", dir)
	if err != nil {
		t.Fatalf("analyze: %v\n%s", err, out)
	}
	doc := decodeAnalyzeJSON(t, out)
	if doc.Summary == nil || doc.Summary.TotalFiles != 2 || doc.Summary.SkippedFiles != 1 {
		t.Fatalf("summary = %+v, want 2 files with 1 skipped", doc.Summary)
	}
	if doc.Summary.HealthScore >= 100 {
		t.Errorf("health score = %d, want the parse-error penalty applied", doc.Summary.HealthScore)
	}
	for _, want := range []string{"1 of 2 files skipped (parse errors)", "broken.js: syntax error"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
}

func TestAnalyzeGoDependencies(t *testing.T) {
	out, err := run(t, "analyze", "--format", "json", "--select", "deps", "../../testdata/godeps")
	if err != nil {
		t.Fatal(err)
	}
	doc := decodeAnalyzeJSON(t, out)
	if doc.Deps == nil {
		t.Fatal("expected a deps section for a Go module")
	}
	if doc.Deps.Analysis.TotalModules != 3 || doc.Deps.Analysis.MaxDepth != 2 {
		t.Errorf("deps analysis = %+v, want 3 packages at depth 2", doc.Deps.Analysis)
	}
	if doc.Summary == nil || !doc.Summary.DepsEnabled {
		t.Errorf("summary = %+v, want the dependency dimension enabled", doc.Summary)
	}

	out, err = run(t, "analyze", "--format", "text", "--select", "deps", "../../testdata/godeps")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Total modules: 3", "Max depth: 2", "Dependencies:"} {
		if !strings.Contains(out, want) {
			t.Errorf("text output lacks %q:\n%s", want, out)
		}
	}
}

func TestAnalyzeGoDependenciesWithoutModule(t *testing.T) {
	// The fixture lies inside polyscan's own module, so it has to move out
	// to lose its go.mod.
	dir := t.TempDir()
	content, err := os.ReadFile("../../testdata/go/sample.go")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sample.go"), content, 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := run(t, "analyze", "--format", "json", "--select", "deps,complexity", dir)
	if err != nil {
		t.Fatal(err)
	}
	doc := decodeAnalyzeJSON(t, out)
	if doc.Deps != nil {
		t.Errorf("deps = %+v, want none outside a module", doc.Deps)
	}
	if doc.Summary == nil || doc.Summary.DepsEnabled {
		t.Errorf("summary = %+v, want the dependency dimension left out", doc.Summary)
	}
	if !strings.Contains(out, "no go.mod above it") {
		t.Errorf("output lacks the go.mod warning:\n%s", out)
	}
}

func TestAnalyzeGoDependenciesOnlyCountsSkippedFiles(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root can read an unreadable file")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/skip\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.go"), []byte("package main\n"), 0o000); err != nil {
		t.Fatal(err)
	}

	out, err := run(t, "analyze", "--format", "json", "--select", "deps", dir)
	if err != nil {
		t.Fatal(err)
	}
	doc := decodeAnalyzeJSON(t, out)
	if doc.Summary == nil || doc.Summary.TotalFiles != 2 || doc.Summary.SkippedFiles != 1 {
		t.Errorf("summary = %+v, want 2 files with 1 skipped whichever analyses ran", doc.Summary)
	}
}

func TestAnalyzeCohesion(t *testing.T) {
	dir := t.TempDir()
	// A Go type next to a JavaScript file: lcom alone must not start the
	// JavaScript pipeline with nothing selected.
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package p\n\ntype T struct{ a, b int }\n\nfunc (t *T) A() { t.a++ }\nfunc (t *T) B() { t.b++ }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.js"), []byte("export function f() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := run(t, "analyze", "--format", "json", "--select", "lcom", dir)
	if err != nil {
		t.Fatalf("analyze: %v\n%s", err, out)
	}
	doc := decodeAnalyzeJSON(t, out)
	if doc.LCOM == nil || len(doc.LCOM.Classes) != 1 {
		t.Fatalf("lcom = %+v, want one class", doc.LCOM)
	}
	class := doc.LCOM.Classes[0]
	if class.Name != "T" || class.Language != "Go" || class.Metrics.LCOM4 != 2 {
		t.Errorf("class = %+v, want T (Go) with LCOM4 2", class)
	}
	if doc.Summary == nil || !doc.Summary.LCOMEnabled || doc.Summary.CohesionScore != 100 {
		t.Errorf("summary = %+v, want cohesion enabled at 100", doc.Summary)
	}

	out, err = run(t, "analyze", "--format", "text", "--select", "lcom", dir)
	if err != nil {
		t.Fatalf("analyze text: %v\n%s", err, out)
	}
	for _, want := range []string{"=== LCOM Analysis ===", "T: LCOM4=2 [low]", "Cohesion:         100/100"} {
		if !strings.Contains(out, want) {
			t.Errorf("text output lacks %q:\n%s", want, out)
		}
	}
}

func TestAnalyzeCoupling(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"go.mod":        "module example.com/app\n",
		"app.go":        "package app\n\nimport \"example.com/app/model\"\n\ntype Server struct {\n\tusers []model.User\n\tcfg   Config\n}\n\ntype Config struct{}\n",
		"model/user.go": "package model\n\ntype User struct{}\n",
		"a.js":          "import { z } from 'zod';\nexport function f() { return z; }\n",
	}
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	out, err := run(t, "analyze", "--format", "json", "--select", "cbo", dir)
	if err != nil {
		t.Fatalf("analyze: %v\n%s", err, out)
	}
	doc := decodeAnalyzeJSON(t, out)
	if doc.CBO == nil || len(doc.CBO.Classes) != 2 || doc.CBO.Summary.TotalClasses != 2 {
		t.Fatalf("cbo = %+v, want the Go type and the JavaScript module", doc.CBO)
	}
	server := doc.CBO.Classes[0]
	if server.Name != "Server" || server.Language != "Go" || server.Metrics.CouplingCount != 2 ||
		!reflect.DeepEqual(server.Metrics.DependentClasses, []string{"Config", "model.User"}) {
		t.Errorf("class = %+v, want Server (Go) coupled to Config and model.User", server)
	}
	if module := doc.CBO.Classes[1]; module.Name != "a" || module.Language != "" {
		t.Errorf("class = %+v, want the JavaScript module a", module)
	}
	if doc.Summary == nil || !doc.Summary.CBOEnabled || doc.Summary.CBOClasses != 2 {
		t.Errorf("summary = %+v, want coupling enabled over 2 classes", doc.Summary)
	}

	out, err = run(t, "analyze", "--format", "text", "--select", "cbo", dir)
	if err != nil {
		t.Fatalf("analyze text: %v\n%s", err, out)
	}
	for _, want := range []string{"=== CBO Analysis ===", "Server: CBO=2 [low]", "Coupling:         100/100"} {
		if !strings.Contains(out, want) {
			t.Errorf("text output lacks %q:\n%s", want, out)
		}
	}
}

func TestAnalyzeOutputPath(t *testing.T) {
	for _, format := range []string{"json", "text"} {
		t.Run(format, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "report."+format)
			if err := os.WriteFile(path, bytes.Repeat([]byte("stale"), 10000), 0600); err != nil {
				t.Fatal(err)
			}
			cmd := rootCmd()
			var stdout, stderr bytes.Buffer
			cmd.SetOut(&stdout)
			cmd.SetErr(&stderr)
			cmd.SetArgs([]string{"analyze", "--select", "complexity", "--format", format, "--output", path, "../../testdata/go/sample.go"})
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if stdout.Len() != 0 {
				t.Errorf("report leaked to stdout: %s", stdout.String())
			}
			if bytes.Contains(data, []byte("stale")) {
				t.Fatal("output retained stale file contents")
			}
			if format == "json" {
				var doc analyzeJSON
				if err := json.Unmarshal(data, &doc); err != nil {
					t.Fatal(err)
				}
				if doc.Complexity == nil || doc.Summary == nil {
					t.Fatal("missing analysis or summary")
				}
				if !strings.Contains(stderr.String(), "Health Score:") {
					t.Error("missing stderr summary")
				}
			} else if !bytes.Contains(data, []byte("polyscan Analysis Report")) || !bytes.Contains(data, []byte("Server.Handle: 8")) {
				t.Errorf("missing text report: %s", data)
			}
		})
	}
}

func TestAnalyzeOutputPathError(t *testing.T) {
	for _, format := range []string{"json", "text", "html"} {
		t.Run(format, func(t *testing.T) {
			out, err := run(t, "analyze", "--select", "complexity", "--format", format, "--no-open", "--output", t.TempDir(), "../../testdata/go/sample.go")
			if err == nil {
				t.Fatal("directory output path returned no error")
			}
			if strings.Contains(out, "Health Score:") {
				t.Errorf("reported success after write failure: %s", out)
			}
		})
	}
}

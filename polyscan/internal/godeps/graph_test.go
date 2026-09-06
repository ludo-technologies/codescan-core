package godeps

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ludo-technologies/polyscan/core/source"
	"github.com/ludo-technologies/polyscan/polyscan/internal/js/domain"
)

const fixture = "../../testdata/godeps"

func fixtureFiles(t *testing.T) []string {
	t.Helper()
	files, err := source.CollectFiles([]string{fixture}, source.FileFilter{IncludePatterns: []string{"*.go"}, Recursive: true})
	if err != nil {
		t.Fatal(err)
	}
	var kept []string
	for _, file := range files {
		if !strings.HasSuffix(file, "_test.go") {
			kept = append(kept, file)
		}
	}
	return kept
}

func identity(path string) string { return path }

func TestBuildFixture(t *testing.T) {
	graph, warnings := Build(fixtureFiles(t), identity)
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}

	want := []string{"example.com/godeps/app", "example.com/godeps/lib", "example.com/godeps/model"}
	if got := graph.NodeIDs(); !reflect.DeepEqual(got, want) {
		t.Fatalf("nodes = %v, want %v", got, want)
	}

	edges := map[string][]string{}
	for _, from := range graph.NodeIDs() {
		edges[from] = graph.Successors(from)
	}
	wantEdges := map[string][]string{
		"example.com/godeps/app":   {"example.com/godeps/lib", "example.com/godeps/model"},
		"example.com/godeps/lib":   {"example.com/godeps/model"},
		"example.com/godeps/model": nil,
	}
	if !reflect.DeepEqual(edges, wantEdges) {
		t.Errorf("edges = %v, want %v", edges, wantEdges)
	}

	// app imports lib from two files and model from one; each pair keeps
	// one edge whose weight counts the files.
	for _, edge := range graph.GetOutgoingEdges("example.com/godeps/app") {
		want := 1
		if edge.To == "example.com/godeps/lib" {
			want = 2
		}
		if edge.Weight != want || edge.EdgeType != domain.EdgeTypeImport {
			t.Errorf("edge app -> %s: weight %d type %s, want weight %d type import", edge.To, edge.Weight, edge.EdgeType, want)
		}
	}

	app := graph.GetNode("example.com/godeps/app")
	model := graph.GetNode("example.com/godeps/model")
	if app.Name != "main" || !app.IsEntryPoint || app.IsLeaf {
		t.Errorf("app = %+v, want package main entry point", app)
	}
	if model.Name != "model" || model.IsEntryPoint || !model.IsLeaf {
		t.Errorf("model = %+v, want a leaf", model)
	}
	if model.Abstractness != 0.5 {
		t.Errorf("model abstractness = %v, want 0.5 (Store among Store and Record)", model.Abstractness)
	}
	if app.Abstractness != 0 {
		t.Errorf("app abstractness = %v, want 0", app.Abstractness)
	}
	if want := filepath.Join("godeps", "model"); !strings.HasSuffix(model.FilePath, want) {
		t.Errorf("model file path = %q, want a path ending in %q", model.FilePath, want)
	}
}

func TestBuildOutsideModule(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a", "a.go"), "package a\n")
	writeFile(t, filepath.Join(dir, "a", "b.go"), "package a\n")
	writeFile(t, filepath.Join(dir, "broken", "broken.go"), "package broken\n")
	writeFile(t, filepath.Join(dir, "broken", "go.mod"), "module example.com/broken\n")
	writeFile(t, filepath.Join(dir, "broken", "bad.go"), "func f() {}\n")

	graph, warnings := Build([]string{
		filepath.Join(dir, "a", "a.go"),
		filepath.Join(dir, "a", "b.go"),
		filepath.Join(dir, "broken", "broken.go"),
		filepath.Join(dir, "broken", "bad.go"),
	}, identity)

	if want := []string{"example.com/broken"}; !reflect.DeepEqual(graph.NodeIDs(), want) {
		t.Errorf("nodes = %v, want %v", graph.NodeIDs(), want)
	}
	if len(warnings) != 2 {
		t.Fatalf("warnings = %v, want one per directory without go.mod and one per unparsable file", warnings)
	}
	if !strings.Contains(warnings[0], "no go.mod") || !strings.Contains(warnings[0], filepath.Join(dir, "a")) {
		t.Errorf("warning[0] = %q", warnings[0])
	}
	if !strings.Contains(warnings[1], "bad.go") || !strings.Contains(warnings[1], "no package clause") {
		t.Errorf("warning[1] = %q", warnings[1])
	}
}

func TestAnalyzeFixture(t *testing.T) {
	response, warnings, err := Analyze(fixtureFiles(t), identity)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
	analysis := response.Analysis
	if analysis.TotalModules != 3 || analysis.TotalDependencies != 3 || analysis.MaxDepth != 2 {
		t.Errorf("modules=%d dependencies=%d depth=%d, want 3, 3, 2", analysis.TotalModules, analysis.TotalDependencies, analysis.MaxDepth)
	}
	if analysis.CircularDependencies == nil || analysis.CircularDependencies.HasCircularDependencies {
		t.Errorf("cycles = %+v, want an empty cycle report", analysis.CircularDependencies)
	}
	model := analysis.ModuleMetrics["example.com/godeps/model"]
	if model.AfferentCoupling != 2 || model.EfferentCoupling != 0 || model.Instability != 0 || model.Abstractness != 0.5 || model.Distance != 0.5 {
		t.Errorf("model metrics = %+v", model)
	}
	if len(analysis.LongestChains) != 1 || analysis.LongestChains[0].Length != 2 {
		t.Errorf("chains = %+v, want one chain of length 2 from app", analysis.LongestChains)
	}
}

func TestAnalyzeWithoutPackages(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.go"), "package a\n")
	response, warnings, err := Analyze([]string{filepath.Join(dir, "a.go")}, identity)
	if err != nil {
		t.Fatal(err)
	}
	if response != nil {
		t.Errorf("response = %+v, want nil when no package resolves", response)
	}
	if len(warnings) != 1 {
		t.Errorf("warnings = %v, want one", warnings)
	}
}

package report

import (
	"reflect"
	"testing"

	coredomain "github.com/ludo-technologies/polyscan/core/domain"
	"github.com/ludo-technologies/polyscan/polyscan/internal/analysis"
	"github.com/ludo-technologies/polyscan/polyscan/internal/clone"
	"github.com/ludo-technologies/polyscan/polyscan/internal/js"
	"github.com/ludo-technologies/polyscan/polyscan/internal/js/domain"
)

func genericReport() *analysis.Report {
	fragment1 := clone.Fragment{ID: 0, Name: "Sum", Language: "Go", FilePath: "a.go", StartLine: 1, EndLine: 10, LineCount: 8, NodeCount: 20, Content: "func Sum() {}"}
	fragment2 := clone.Fragment{ID: 1, Name: "Add", Language: "Go", FilePath: "b.go", StartLine: 5, EndLine: 14, LineCount: 8, NodeCount: 20}
	return &analysis.Report{
		Files: analysis.Files{Total: 3, Analyzed: 2, Skipped: 1},
		Complexity: &analysis.Complexity{
			Functions: []analysis.Function{
				{Name: "Sum", FilePath: "a.go", Language: "Go", StartLine: 1, EndLine: 10, Complexity: 12, RiskLevel: coredomain.RiskLevelMedium},
				{Name: "Add", FilePath: "b.go", Language: "Go", StartLine: 5, EndLine: 14, Complexity: 2, RiskLevel: coredomain.RiskLevelLow},
			},
			Summary: analysis.ComplexitySummary{
				TotalFunctions:      2,
				AverageComplexity:   7,
				MaxComplexity:       12,
				MinComplexity:       2,
				LowRiskFunctions:    1,
				MediumRiskFunctions: 1,
			},
		},
		Clones: &clone.Report{
			Pairs: []clone.Pair{{
				ID: 0, Type: coredomain.Type1Clone, Similarity: 1,
				Fragment1: fragment1, Fragment2: fragment2,
			}},
			Groups: []clone.Group{{
				ID: 0, Type: coredomain.Type1Clone, Similarity: 1,
				Fragments: []clone.Fragment{fragment1, fragment2},
			}},
			Statistics: clone.Statistics{
				TotalFragments: 4, TotalClones: 2, TotalClonePairs: 1, TotalCloneGroups: 1,
				ClonesByType: map[string]int{"Type-1": 1}, AverageSimilarity: 1,
				LinesAnalyzed: 140, FilesAnalyzed: 2,
			},
		},
		FileLines: map[string]int{"a.go": 100, "b.go": 40},
		Errors:    []string{"c.go: read error"},
	}
}

func TestCombineGenericOnly(t *testing.T) {
	responses, err := Combine(genericReport(), nil)
	if err != nil {
		t.Fatal(err)
	}

	complexity := responses.Complexity
	if complexity == nil {
		t.Fatal("no complexity response")
	}
	if len(complexity.Functions) != 2 || complexity.Functions[0].Name != "Sum" {
		t.Fatalf("functions = %+v", complexity.Functions)
	}
	first := complexity.Functions[0]
	if first.Language != "Go" || first.Metrics.Complexity != 12 || first.RiskLevel != domain.RiskLevelMedium {
		t.Errorf("converted function = %+v", first)
	}
	summary := complexity.Summary
	if summary.TotalFunctions != 2 || summary.TotalFiles != 3 || summary.FilesAnalyzed != 2 || summary.SkippedFiles != 1 {
		t.Errorf("summary = %+v", summary)
	}
	if summary.ComplexityDistribution[12] != 1 || summary.ComplexityDistribution[2] != 1 {
		t.Errorf("distribution = %v", summary.ComplexityDistribution)
	}
	if rollup := complexity.ModuleRollups["a.go"]; rollup.LinesOfCode != 100 || rollup.AnalyzedFunctionCount != 1 || rollup.MaxComplexity != 12 {
		t.Errorf("rollup = %+v", rollup)
	}
	if len(complexity.Errors) != 1 {
		t.Errorf("errors = %v", complexity.Errors)
	}

	clones := responses.Clone
	if clones == nil {
		t.Fatal("no clone response")
	}
	if len(clones.ClonePairs) != 1 || len(clones.CloneGroups) != 1 || len(clones.Clones) != 2 {
		t.Fatalf("clones = %d pairs, %d groups, %d fragments",
			len(clones.ClonePairs), len(clones.CloneGroups), len(clones.Clones))
	}
	pair := clones.ClonePairs[0]
	if pair.Clone1.Language != "Go" || pair.Clone1.Location.FilePath != "a.go" || pair.Type != domain.Type1Clone {
		t.Errorf("pair = %+v", pair)
	}
	// A fragment keeps one identity across the pairs and groups it appears in.
	if pair.Clone1 != clones.CloneGroups[0].Clones[0] {
		t.Error("pair and group must share fragment instances")
	}
	stats := clones.Statistics
	if stats.TotalFragments != 4 || stats.TotalClones != 2 || stats.LinesAnalyzed != 140 || stats.FilesAnalyzed != 2 {
		t.Errorf("statistics = %+v", stats)
	}
	if responses.DeadCode != nil || responses.CBO != nil || responses.Deps != nil {
		t.Error("the generic engine has no dead code, CBO or dependency analysis")
	}
	if !reflect.DeepEqual(responses.Files, domain.FileAccounting{Total: 3, Skipped: 1, Errors: []string{"c.go: read error"}}) {
		t.Errorf("files = %+v", responses.Files)
	}
}

func javascriptResult() *js.Result {
	fragment := &domain.Clone{
		ID: 0, Type: domain.Type2Clone, Language: "JavaScript",
		Location: &domain.CloneLocation{FilePath: "app.js", StartLine: 1, EndLine: 12},
	}
	other := &domain.Clone{
		ID: 1, Type: domain.Type2Clone, Language: "JavaScript",
		Location: &domain.CloneLocation{FilePath: "app.js", StartLine: 20, EndLine: 31},
	}
	return &js.Result{
		Files: domain.FileAccounting{Total: 1},
		Complexity: &domain.ComplexityResponse{
			Functions: []domain.FunctionComplexity{{
				Name: "handler", FilePath: "app.js", Language: "JavaScript", StartLine: 1, EndLine: 30,
				Metrics: domain.ComplexityMetrics{Complexity: 5}, RiskLevel: domain.RiskLevelLow,
			}},
			ModuleRollups: map[string]domain.ModuleComplexityMetrics{
				"app.js": {LinesOfCode: 30, AnalyzedFunctionCount: 1, AverageComplexity: 5, MaxComplexity: 5},
			},
			Summary: domain.ComplexitySummary{
				TotalFunctions: 1, FunctionsParsed: 1, AverageComplexity: 5,
				MaxComplexity: 5, MinComplexity: 5, FilesAnalyzed: 1, TotalFiles: 1,
				LowRiskFunctions: 1, ComplexityDistribution: map[int]int{5: 1},
			},
			Config: map[string]interface{}{"low_threshold": 9, "medium_threshold": 19},
		},
		DeadCode: &domain.DeadCodeResponse{},
		Clones: &domain.CloneResponse{
			Clones: []*domain.Clone{fragment, other},
			ClonePairs: []*domain.ClonePair{{
				ID: 0, Clone1: fragment, Clone2: other, Similarity: 0.9, Type: domain.Type2Clone,
			}},
			Statistics: &domain.CloneStatistics{
				TotalFragments: 3, TotalClones: 2, TotalClonePairs: 1,
				ClonesByType: map[string]int{"Type-2": 1}, AverageSimilarity: 0.9,
				LinesAnalyzed: 60, FilesAnalyzed: 1,
			},
			Success: true,
		},
	}
}

func TestCombineMergesLanguages(t *testing.T) {
	responses, err := Combine(genericReport(), javascriptResult())
	if err != nil {
		t.Fatal(err)
	}

	complexity := responses.Complexity
	if len(complexity.Functions) != 3 {
		t.Fatalf("functions = %+v", complexity.Functions)
	}
	// Merged functions stay ranked by complexity across languages.
	names := []string{complexity.Functions[0].Name, complexity.Functions[1].Name, complexity.Functions[2].Name}
	if names[0] != "Sum" || names[1] != "handler" || names[2] != "Add" {
		t.Errorf("ranked functions = %v", names)
	}
	summary := complexity.Summary
	if summary.TotalFunctions != 3 || summary.TotalFiles != 4 || summary.FilesAnalyzed != 3 || summary.SkippedFiles != 1 {
		t.Errorf("summary = %+v", summary)
	}
	if responses.Files.Total != 4 || responses.Files.Skipped != 1 {
		t.Errorf("files = %+v, want both languages' files counted", responses.Files)
	}
	if want := (7.0*2 + 5.0) / 3; summary.AverageComplexity != want {
		t.Errorf("AverageComplexity = %v, want %v", summary.AverageComplexity, want)
	}
	if summary.MaxComplexity != 12 || summary.MinComplexity != 2 {
		t.Errorf("max/min = %d/%d", summary.MaxComplexity, summary.MinComplexity)
	}
	if summary.ComplexityDistribution[5] != 1 || summary.ComplexityDistribution[12] != 1 {
		t.Errorf("distribution = %v", summary.ComplexityDistribution)
	}
	if len(complexity.ModuleRollups) != 3 {
		t.Errorf("rollups = %+v", complexity.ModuleRollups)
	}

	clones := responses.Clone
	if len(clones.ClonePairs) != 2 || len(clones.Clones) != 4 {
		t.Fatalf("clones = %d pairs, %d fragments", len(clones.ClonePairs), len(clones.Clones))
	}
	// Fragment IDs stay unique after the JavaScript response is rebased, and
	// pairs are re-ranked (highest similarity first) and renumbered.
	seen := map[int]bool{}
	for _, fragment := range clones.Clones {
		if seen[fragment.ID] {
			t.Errorf("duplicate fragment ID %d", fragment.ID)
		}
		seen[fragment.ID] = true
	}
	if clones.ClonePairs[0].Similarity != 1 || clones.ClonePairs[0].ID != 0 || clones.ClonePairs[1].ID != 1 {
		t.Errorf("pair ranking = %+v", clones.ClonePairs)
	}
	stats := clones.Statistics
	if stats.TotalFragments != 7 || stats.TotalClones != 4 || stats.TotalClonePairs != 2 ||
		stats.LinesAnalyzed != 200 || stats.FilesAnalyzed != 3 {
		t.Errorf("statistics = %+v", stats)
	}
	if want := (1.0 + 0.9) / 2; stats.AverageSimilarity != want {
		t.Errorf("AverageSimilarity = %v, want %v", stats.AverageSimilarity, want)
	}
	if stats.ClonesByType["Type-1"] != 1 || stats.ClonesByType["Type-2"] != 1 {
		t.Errorf("clones by type = %v", stats.ClonesByType)
	}

	if responses.DeadCode == nil {
		t.Error("the JavaScript dead code response must pass through")
	}
}

func TestCombineJavaScriptOnly(t *testing.T) {
	javascript := javascriptResult()
	responses, err := Combine(nil, javascript)
	if err != nil {
		t.Fatal(err)
	}
	if responses.Complexity != javascript.Complexity || responses.Clone != javascript.Clones {
		t.Error("a JavaScript-only run must pass its responses through unchanged")
	}
}

func TestCombineSelectionWithoutComplexity(t *testing.T) {
	generic := genericReport()
	generic.Complexity = nil
	responses, err := Combine(generic, nil)
	if err != nil {
		t.Fatal(err)
	}
	if responses.Complexity != nil {
		t.Error("no complexity analysis ran, so there must be no complexity response")
	}
	if responses.Clone == nil {
		t.Error("the clone response must survive without complexity")
	}
	if responses.Files.Skipped != 1 {
		t.Errorf("files = %+v, want the skipped file charged without complexity", responses.Files)
	}
}

// TestCombineJavaScriptSkippedFilesWithoutComplexity covers #92 on the
// JavaScript side: a dead-code-only run still carries its unparsable files.
func TestCombineJavaScriptSkippedFilesWithoutComplexity(t *testing.T) {
	javascript := &js.Result{
		Files:    domain.FileAccounting{Total: 2, Skipped: 1, Errors: []string{"broken.js: syntax error at line 1"}},
		DeadCode: &domain.DeadCodeResponse{},
	}
	responses, err := Combine(nil, javascript)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(responses.Files, javascript.Files) {
		t.Errorf("files = %+v, want %+v", responses.Files, javascript.Files)
	}
}

package analysis

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ludo-technologies/polyscan/core/domain"
	"github.com/ludo-technologies/polyscan/polyscan/internal/clone"
)

// fixtures copies the Go fixtures into a temporary directory and adds a
// file that does not parse and one that cannot be read. The broken file is
// generated here rather than committed so that gofmt and go vet never trip
// over it.
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
	if err := os.WriteFile(filepath.Join(dir, "broken.go"), []byte("package broken\n\nfunc Broken() {\n\tif {\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "unreadable.go"), []byte("package p\n"), 0o000); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestAnalyzeComplexity(t *testing.T) {
	report, err := Analyze([]string{fixtures(t)}, Options{Complexity: true})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	if report.Files != (Files{Total: 3, Analyzed: 2, Partial: 1, Skipped: 1}) {
		t.Errorf("files = %+v, want total 3, analyzed 2, partial 1, skipped 1", report.Files)
	}
	if len(report.Warnings) != 1 || !strings.Contains(report.Warnings[0], "broken.go: syntax error at line 4") {
		t.Errorf("warnings = %v, want the broken file's syntax error", report.Warnings)
	}
	if len(report.Errors) != 1 || !strings.Contains(report.Errors[0], "unreadable.go") {
		t.Errorf("errors = %v, want the unreadable file", report.Errors)
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
	if first.NestingDepth != 1 {
		t.Errorf("Server.Handle nesting depth = %d, want 1", first.NestingDepth)
	}
	for _, fn := range functions {
		if fn.Name == "Closure" && fn.NestingDepth != 2 {
			t.Errorf("Closure nesting depth = %d, want 2 (the literal's for and if count toward it)", fn.NestingDepth)
		}
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

func TestAnalyzeRust(t *testing.T) {
	report, err := Analyze([]string{"../../testdata/rust"}, Options{Complexity: true, Clones: true})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	byName := map[string]Function{}
	for _, fn := range report.Complexity.Functions {
		byName[fn.Name] = fn
	}
	if fn := byName["Server::handle"]; fn.Complexity != 8 || fn.Language != "Rust" {
		t.Errorf("Server::handle = %+v, want complexity 8 in Rust", fn)
	}
	if _, ok := byName["tests::sums_positive_values"]; !ok {
		t.Error("test functions must still be analyzed for complexity")
	}

	if _, ok := byName["roundtrip"]; !ok {
		t.Error("functions in tests.rs must still be analyzed for complexity")
	}

	// sums_positive_values is a copy of sum_positive but lies in #[cfg(test)],
	// and roundtrip is another copy but lies in tests.rs.
	stats := report.Clones.Statistics
	if stats.TotalFragments != 3 || stats.TotalClonePairs != 1 || stats.ClonesByType["Type-2"] != 1 {
		t.Errorf("statistics = %+v, want one Type-2 pair among three fragments", stats)
	}
}

func TestAnalyzeMergesLanguages(t *testing.T) {
	report, err := Analyze([]string{"../../testdata/go/clones", "../../testdata/rust"}, Options{Clones: true})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	pairs := report.Clones.Pairs
	if len(pairs) != 4 || report.Clones.Statistics.TotalFragments != 6 {
		t.Fatalf("pairs = %d, fragments = %d, want 4 and 6", len(pairs), report.Clones.Statistics.TotalFragments)
	}
	// The Type-1 Go pair outranks the Type-2 pairs whatever the language order.
	if pairs[0].Type != domain.Type1Clone || pairs[0].Fragment1.Language != "Go" {
		t.Errorf("first pair = %+v, want the Go Type-1 pair", pairs[0])
	}
	seen := map[int]string{}
	for i, pair := range pairs {
		if pair.ID != i || i > 0 && pair.Similarity > pairs[i-1].Similarity {
			t.Errorf("pair %d is out of order: %+v", i, pair)
		}
		for _, fragment := range []clone.Fragment{pair.Fragment1, pair.Fragment2} {
			if name, ok := seen[fragment.ID]; ok && name != fragment.Name {
				t.Errorf("fragment ID %d names both %s and %s", fragment.ID, name, fragment.Name)
			}
			seen[fragment.ID] = fragment.Name
			if fragment.Language == "" {
				t.Errorf("fragment %+v has no language", fragment)
			}
		}
	}
	for i, group := range report.Clones.Groups {
		if group.ID != i {
			t.Errorf("group %d has ID %d", i, group.ID)
		}
	}
}

func TestRankOrdersGroupsLikeCore(t *testing.T) {
	fragment := func(path string, line int) clone.Fragment { return clone.Fragment{FilePath: path, StartLine: line} }
	report := &clone.Report{
		Groups: []clone.Group{
			{Similarity: 0.9, Fragments: []clone.Fragment{fragment("a.rs", 1), fragment("b.rs", 1)}},
			{Similarity: 0.9 + 1e-12, Fragments: []clone.Fragment{fragment("z.go", 1), fragment("z.go", 50), fragment("y.go", 1)}},
			{Similarity: 1, Fragments: []clone.Fragment{fragment("c.rs", 1), fragment("d.rs", 1)}},
		},
		Pairs: []clone.Pair{
			{Similarity: 0.8, Fragment1: fragment("b.go", 1), Fragment2: fragment("c.go", 1)},
			{Similarity: 0.8, Fragment1: fragment("a.rs", 1), Fragment2: fragment("c.rs", 1)},
			{Similarity: 1, Fragment1: fragment("x.go", 1), Fragment2: fragment("y.go", 1)},
		},
	}
	rank(report)

	var groups []string
	for _, group := range report.Groups {
		groups = append(groups, group.Fragments[0].FilePath)
	}
	// Equal similarity within epsilon: the larger group comes first.
	if want := []string{"c.rs", "z.go", "a.rs"}; strings.Join(groups, ",") != strings.Join(want, ",") {
		t.Errorf("groups = %v, want %v", groups, want)
	}
	var pairs []string
	for _, pair := range report.Pairs {
		pairs = append(pairs, pair.Fragment1.FilePath)
	}
	if want := []string{"x.go", "a.rs", "b.go"}; strings.Join(pairs, ",") != strings.Join(want, ",") {
		t.Errorf("pairs = %v, want %v", pairs, want)
	}
	for i, group := range report.Groups {
		if group.ID != i {
			t.Errorf("group %d has ID %d", i, group.ID)
		}
	}
}

func TestAnalyzeCpp(t *testing.T) {
	report, err := Analyze([]string{"../../testdata/cpp"}, Options{Complexity: true, Clones: true})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	byName := map[string]Function{}
	for _, fn := range report.Complexity.Functions {
		byName[fn.Name] = fn
	}
	if fn := byName["Server::handle"]; fn.Complexity != 9 || fn.Language != "C++" {
		t.Errorf("Server::handle = %+v, want complexity 9 in C++", fn)
	}
	if _, ok := byName["sumPositiveAgain"]; !ok {
		t.Error("functions in test files must still be analyzed for complexity")
	}
	// sumPositiveAgain is a copy of sumPositive but lies in sample_test.cpp.
	stats := report.Clones.Statistics
	if stats.TotalFragments != 3 || stats.TotalClonePairs != 1 || stats.ClonesByType["Type-2"] != 1 {
		t.Errorf("statistics = %+v, want one Type-2 pair among three fragments", stats)
	}
}

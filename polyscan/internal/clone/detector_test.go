package clone

import (
	"math"
	"strings"
	"testing"

	"github.com/ludo-technologies/polyscan/core/apted"
	"github.com/ludo-technologies/polyscan/core/domain"
	"github.com/ludo-technologies/polyscan/polyscan/internal/engine"
	"github.com/ludo-technologies/polyscan/polyscan/internal/lang/golang"
)

func TestCostModelTiers(t *testing.T) {
	model := newCostModel(engine.CloneSpec{
		Structural:  []string{"function_declaration"},
		ControlFlow: []string{"if_statement", "for_statement"},
		Expressions: []string{"call_expression"},
		Related:     [][2]string{{"if_statement", "expression_switch_statement"}},
	})
	node := func(label string) *apted.TreeNode { return apted.NewTreeNode(0, label) }

	for label, want := range map[string]float64{
		"function_declaration": structuralMultiplier,
		"if_statement":         controlFlowMultiplier,
		"call_expression":      expressionMultiplier,
		"identifier(x)":        defaultMultiplier,
	} {
		if got := model.Insert(node(label)); got != want {
			t.Errorf("Insert(%s) = %v, want %v", label, got, want)
		}
		if got := model.Delete(node(label)); got != want {
			t.Errorf("Delete(%s) = %v, want %v", label, got, want)
		}
	}

	for _, tc := range []struct {
		a, b string
		want float64
	}{
		{"identifier(x)", "identifier(x)", 0},
		{"identifier(x)", "identifier(y)", 1 - sameBaseTypeSimilarity},
		{"if_statement", "expression_switch_statement", 1 - relatedTypeSimilarity},
		{"if_statement", "for_statement", 1 - sameCategorySimilarity},
		{"if_statement", "call_expression", 1},
		{"identifier(x)", "int_literal(1)", 1},
	} {
		if got := model.Rename(node(tc.a), node(tc.b)); math.Abs(got-tc.want) > 1e-9 {
			t.Errorf("Rename(%s, %s) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

const body = `
	total := 0
	for _, value := range values {
		if value > 0 {
			total += value
		} else if value < -10 {
			total -= value
		}
		fmt.Println(value)
		fmt.Println(total)
	}
	return total
}
`

func detect(t *testing.T, sources ...string) *Report {
	t.Helper()
	return detectWith(t, DefaultConfig(), sources...)
}

func detectWith(t *testing.T, config Config, sources ...string) *Report {
	t.Helper()
	detector := NewDetector(golang.Language.Clone, config)
	for i, source := range sources {
		functions, err := golang.Language.Analyze([]byte("package p\n\nimport \"fmt\"\n\n" + source))
		if err != nil {
			t.Fatal(err)
		}
		for _, fn := range functions {
			detector.Add(fn, "file"+string(rune('a'+i))+".go")
		}
	}
	return detector.Detect()
}

func TestDetectClassifiesPairs(t *testing.T) {
	renamed := strings.NewReplacer("total", "sum", "value", "v", "values", "xs").Replace(body)
	report := detect(t,
		"func Sum(values []int) int {"+body,
		"func Sum(values []int) int { // a comment\n"+body,
		"func Sum(xs []int) int {"+renamed,
	)

	if len(report.Pairs) != 3 || len(report.Groups) != 1 || len(report.Groups[0].Fragments) != 3 {
		t.Fatalf("pairs = %+v, groups = %+v", report.Pairs, report.Groups)
	}
	exact := report.Pairs[0]
	if exact.Type != domain.Type1Clone || exact.Similarity != 1 || exact.Fragment1.FilePath != "filea.go" || exact.Fragment2.FilePath != "fileb.go" {
		t.Errorf("strongest pair = %+v, want the Type-1 pair of the first two files", exact)
	}
	for _, pair := range report.Pairs[1:] {
		if pair.Type != domain.Type2Clone {
			t.Errorf("renamed pair = %+v, want Type-2", pair)
		}
	}
	if report.Statistics.ClonesByType["Type-1"] != 1 || report.Statistics.ClonesByType["Type-2"] != 2 || report.Statistics.TotalClones != 3 {
		t.Errorf("statistics = %+v", report.Statistics)
	}
}

func TestDetectSkipsSmallAndUnrelatedFunctions(t *testing.T) {
	report := detect(t,
		"func Sum(values []int) int {"+body,
		"func Small(values []int) int {\n\treturn len(values)\n}\n",
		"func Other(name string) error {\n\tf, err := os.Open(name)\n\tif err != nil {\n\t\treturn err\n\t}\n\tdefer f.Close()\n\tb := make([]byte, 10)\n\t_, err = f.Read(b)\n\tif err != nil {\n\t\treturn err\n\t}\n\tfmt.Println(string(b))\n\treturn nil\n}\n",
	)
	if report.Statistics.TotalFragments != 2 {
		t.Errorf("fragments = %d, want Sum and Other only", report.Statistics.TotalFragments)
	}
	if len(report.Pairs) != 0 || len(report.Groups) != 0 {
		t.Errorf("unexpected clones: %+v", report.Pairs)
	}
}

// TestDetectCappedLSHCandidatesChainEveryFragment forces the LSH path with
// a candidate cap smaller than the number of identical functions. Each
// fragment is paired with the members that follow it in the bucket, so the
// last two are compared with each other even though they lie beyond the
// cap of every earlier fragment.
func TestDetectCappedLSHCandidatesChainEveryFragment(t *testing.T) {
	config := DefaultConfig()
	config.MaxPairs = 12 // Six fragments make 15 pairs, so the LSH path runs.
	config.LSH.MaxCandidates = 3

	sources := make([]string, 6)
	for i := range sources {
		sources[i] = "func Sum(values []int) int {" + body
	}
	report := detectWith(t, config, sources...)

	// 3+3+3+2+1 pairs: every fragment with the next three.
	if len(report.Pairs) != 12 || report.Statistics.TotalClones != 6 {
		t.Errorf("pairs = %d, clones = %d, want 12 and 6", len(report.Pairs), report.Statistics.TotalClones)
	}
	last := false
	for _, pair := range report.Pairs {
		if pair.Type != domain.Type1Clone {
			t.Errorf("pair = %+v, want Type-1", pair)
		}
		if pair.Fragment1.ID == 4 && pair.Fragment2.ID == 5 {
			last = true
		}
	}
	if !last {
		t.Error("the last two fragments were not compared with each other")
	}
	if len(report.Groups) != 1 || len(report.Groups[0].Fragments) != 6 {
		t.Errorf("groups = %+v, want one group of six", report.Groups)
	}
}

func TestDetectIgnoresCommentsAndSpacing(t *testing.T) {
	commented := "// Sum adds the values.\n//\n// It skips nothing.\nfunc Sum(values []int) int { // start\n" +
		strings.ReplaceAll(strings.ReplaceAll(body, "\n\t\t", "\n\t\t// a comment line\n\n\t\t"), "return total", "return/* the result */total")
	report := detect(t, "func Sum(values []int) int {"+body, commented)

	if len(report.Pairs) != 1 || report.Pairs[0].Type != domain.Type1Clone {
		t.Fatalf("pairs = %+v, want one Type-1 pair", report.Pairs)
	}
	if a, b := report.Pairs[0].Fragment1.LineCount, report.Pairs[0].Fragment2.LineCount; a != b {
		t.Errorf("line counts %d and %d differ, want lines of code only", a, b)
	}
}

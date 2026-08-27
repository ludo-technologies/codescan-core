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

// TestDetectCappedLSHCandidatesReachEveryFragment forces the LSH path with
// a candidate cap smaller than the number of identical functions. A capped
// query lists the lowest indexes, so without the reverse visit the later
// fragments would never be paired with anything.
func TestDetectCappedLSHCandidatesReachEveryFragment(t *testing.T) {
	config := DefaultConfig()
	config.MaxPairs = 10 // Six fragments make 15 pairs, so the LSH path runs.
	config.LSH.MaxCandidates = 3

	sources := make([]string, 6)
	for i := range sources {
		sources[i] = "func Sum(values []int) int {" + body
	}
	report := detectWith(t, config, sources...)

	if report.Statistics.TotalClones != 6 {
		t.Errorf("clones = %d, want every fragment paired", report.Statistics.TotalClones)
	}
	if len(report.Pairs) != config.MaxPairs {
		t.Errorf("pairs = %d, want the MaxPairs strongest", len(report.Pairs))
	}
	for _, pair := range report.Pairs {
		if pair.Type != domain.Type1Clone {
			t.Errorf("pair = %+v, want Type-1", pair)
		}
	}
}

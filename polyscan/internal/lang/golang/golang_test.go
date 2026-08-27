package golang

import (
	"strings"
	"testing"

	"github.com/ludo-technologies/polyscan/polyscan/internal/engine"
)

func analyze(t *testing.T, source string) map[string]engine.Function {
	t.Helper()
	functions, err := Language.Analyze([]byte(source))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	byName := map[string]engine.Function{}
	for _, fn := range functions {
		byName[fn.Name] = fn
	}
	return byName
}

func TestExtractsFunctionsAndMethods(t *testing.T) {
	functions := analyze(t, `package p

func Plain() {}

type T struct{}
type G[K any] struct{}

func (t T) Value() {}
func (t *T) Pointer() {}
func (g G[K]) Generic() {}
func (g *G[K]) GenericPointer() {}
func (t (T)) Parenthesized() {}
func (t (*T)) ParenthesizedPointer() {}
func (g (*G[K])) ParenthesizedGeneric() {}

var closure = func() {}
`)

	want := map[string]string{
		"Plain":                  "function",
		"T.Value":                "method",
		"T.Pointer":              "method",
		"G.Generic":              "method",
		"G.GenericPointer":       "method",
		"T.Parenthesized":        "method",
		"T.ParenthesizedPointer": "method",
		"G.ParenthesizedGeneric": "method",
	}
	if len(functions) != len(want) {
		t.Fatalf("got %d functions, want %d: %v", len(functions), len(want), functions)
	}
	for name, kind := range want {
		fn, ok := functions[name]
		if !ok {
			t.Errorf("missing function %q", name)
			continue
		}
		if fn.Kind != kind {
			t.Errorf("%s: kind = %q, want %q", name, fn.Kind, kind)
		}
		if fn.Complexity != 1 {
			t.Errorf("%s: complexity = %d, want 1", name, fn.Complexity)
		}
	}
}

func TestPositions(t *testing.T) {
	functions := analyze(t, "package p\n\nfunc F() {\n\treturn\n}\n")
	fn := functions["F"]
	if fn.StartLine != 3 || fn.StartColumn != 1 || fn.EndLine != 5 {
		t.Errorf("F at %d:%d-%d, want 3:1-5", fn.StartLine, fn.StartColumn, fn.EndLine)
	}
}

func TestComplexityCountsDecisionPoints(t *testing.T) {
	cases := []struct {
		name      string
		body      string
		want      int
		decisions map[string]int
	}{
		{"none", `return`, 1, map[string]int{}},
		{"if", `if a { return }`, 2, map[string]int{"branch": 1}},
		{"if else", `if a { return } else { return }`, 2, map[string]int{"branch": 1}},
		{"else if chain", `if a { } else if b { } else { }`, 3, map[string]int{"branch": 2}},
		{"for", `for i := 0; i < 1; i++ { }`, 2, map[string]int{"loop": 1}},
		{"for range", `for range xs { }`, 2, map[string]int{"loop": 1}},
		{"for ever", `for { break }`, 2, map[string]int{"loop": 1}},
		{"switch cases without default", `switch n { case 1: case 2, 3: default: }`, 3, map[string]int{"case": 2}},
		{"type switch", `switch v.(type) { case int: case string: default: }`, 3, map[string]int{"case": 2}},
		{"select", `select { case <-ch: default: }`, 2, map[string]int{"case": 1}},
		{"logical operators", `if a && b || c { }`, 4, map[string]int{"branch": 1, "logical_operator": 2}},
		{"bitwise operators do not count", `n = n & 1 | 2`, 1, map[string]int{}},
		{"closure counts toward its enclosing function", `f := func() { if a { } }; f()`, 2, map[string]int{"branch": 1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source := "package p\n\nvar a, b, c bool\nvar n int\nvar v interface{}\nvar xs []int\nvar ch chan int\n\nfunc F() {\n\t" + tc.body + "\n}\n"
			fn := analyze(t, source)["F"]
			if fn.Complexity != tc.want {
				t.Errorf("complexity = %d, want %d", fn.Complexity, tc.want)
			}
			if len(fn.Decisions) != len(tc.decisions) {
				t.Errorf("decisions = %v, want %v", fn.Decisions, tc.decisions)
			}
			for kind, count := range tc.decisions {
				if fn.Decisions[kind] != count {
					t.Errorf("decisions[%q] = %d, want %d", kind, fn.Decisions[kind], count)
				}
			}
		})
	}
}

func TestTopLevelDecisionsAreIgnored(t *testing.T) {
	functions := analyze(t, "package p\n\nvar a, b bool\nvar c = a && b\n\nfunc F() {}\n")
	if got := functions["F"].Complexity; got != 1 {
		t.Errorf("F complexity = %d, want 1", got)
	}
}

func TestSyntaxErrorIsReported(t *testing.T) {
	_, err := Language.Analyze([]byte("package p\n\nfunc F() {\n\tif {\n"))
	if err == nil {
		t.Fatal("expected a syntax error")
	}
	if !strings.Contains(err.Error(), "syntax error at line 3") {
		t.Errorf("error = %q, want the first error line", err)
	}
}

func TestContentDropsCommentsButKeepsTokensApart(t *testing.T) {
	fn := analyze(t, "package p\n\n// F returns one.\nfunc F() int {\n\t// a line comment\n\n\treturn/* one */1 /* trailing */\n}\n")["F"]
	want := "func F() int {\n\t \n\n\treturn 1  \n}"
	if fn.Content != want {
		t.Errorf("content = %q, want %q", fn.Content, want)
	}
	if fn.CodeLines != 3 {
		t.Errorf("code lines = %d, want 3", fn.CodeLines)
	}
}

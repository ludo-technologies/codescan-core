package golang

import (
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/ludo-technologies/polyscan/polyscan/internal/engine"
)

func analyze(t *testing.T, source string) map[string]engine.Function {
	t.Helper()
	result, err := Language.Analyze([]byte(source))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.SyntaxError != nil {
		t.Fatalf("syntax error: %v", result.SyntaxError)
	}
	byName := map[string]engine.Function{}
	for _, fn := range result.Functions {
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

func TestSyntaxErrorKeepsCleanFunctions(t *testing.T) {
	result, err := Language.Analyze([]byte("package p\n\nfunc Clean() {\n\tif true {\n\t}\n}\n\nfunc Broken() {\n\tif {\n}\n\nfunc AlsoClean() {}\n"))
	if err != nil {
		t.Fatal(err)
	}
	if result.SyntaxError == nil || !strings.Contains(result.SyntaxError.Error(), "syntax error at line 9") {
		t.Errorf("syntax error = %v, want the first error line", result.SyntaxError)
	}
	var names []string
	for _, fn := range result.Functions {
		names = append(names, fn.Name)
	}
	if strings.Join(names, ",") != "Clean,AlsoClean" {
		t.Errorf("functions = %v, want the ones without errors", names)
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

func TestNestingDepth(t *testing.T) {
	functions := analyze(t, `package p

func Flat(a bool) {
	if a {
	}
	for {
	}
}

func ElseIfContinuesTheChain(a, b bool) {
	if a {
	} else if b {
		for {
		}
	} else {
		if a {
			if b {
			}
		}
	}
}

func Deep(a bool, xs []int) {
	if a {
		for _, x := range xs {
			switch x {
			case 1:
				select {
				default:
				}
			}
		}
	}
}

func Closure(a, b bool) func() {
	return func() {
		for {
			if a {
				if b {
				}
			}
		}
	}
}
`)
	want := map[string]int{
		"Flat":                    1,
		"ElseIfContinuesTheChain": 3,
		"Deep":                    4,
		"Closure":                 3,
	}
	for name, depth := range want {
		if got := functions[name].NestingDepth; got != depth {
			t.Errorf("%s: nesting depth = %d, want %d", name, got, depth)
		}
	}
}

func TestMembers(t *testing.T) {
	functions := analyze(t, `package p

type Base struct{}

func (Base) Promoted() {}

type T struct {
	Base
	a, b int
	cb   func()
}

func (t *T) Fields() {
	t.a = 1
	t.b.c = 2
	other.x = 3
	go func() { t.a++ }()
}

func (t *T) Calls() {
	t.Fields()
	t.cb()
	t.Base.Promoted()
	t.Promoted()
	other.Fields()
}

func (T) Static() {}

func (_ T) Blank() {}

func Free() {}
`)

	cases := []struct {
		name     string
		receiver string
		hasSelf  bool
		fields   []string
		calls    []string
	}{
		{"Base.Promoted", "Base", false, nil, nil},
		{"T.Fields", "T", true, []string{"a", "b"}, []string{}},
		{"T.Calls", "T", true, []string{"Base"}, []string{"Fields", "Promoted", "cb"}},
		{"T.Static", "T", false, nil, nil},
		{"T.Blank", "T", false, nil, nil},
		{"Free", "", false, nil, nil},
	}
	for _, tc := range cases {
		fn, ok := functions[tc.name]
		if !ok {
			t.Errorf("missing function %q", tc.name)
			continue
		}
		if fn.Receiver != tc.receiver || fn.HasSelf != tc.hasSelf {
			t.Errorf("%s: receiver %q hasSelf %v, want %q %v", tc.name, fn.Receiver, fn.HasSelf, tc.receiver, tc.hasSelf)
		}
		if got := slices.Sorted(maps.Keys(fn.Fields)); !equalStrings(got, tc.fields) {
			t.Errorf("%s: fields = %v, want %v", tc.name, got, tc.fields)
		}
		if got := slices.Sorted(maps.Keys(fn.Calls)); !equalStrings(got, tc.calls) {
			t.Errorf("%s: calls = %v, want %v", tc.name, got, tc.calls)
		}
	}
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func TestMembersIgnoreShadowedReceiver(t *testing.T) {
	functions := analyze(t, `package p

type License struct{ TTL int; items []License; ch chan License }

// Package-level names are the ones a receiver shadows.
var x = newThing()

const c = "x"

func (c *License) Package() int {
	c.Warm()
	return c.TTL
}

func (x *License) Default(v int) {
	switch v {
	default:
		var x = 1
		_ = x.Def
	}
}

// The right-hand side of the declaration still sees the receiver.
func (x *License) GetText() string {
	if x, ok := x.GetSource().(*Text); ok {
		return x.Text
	}
	return ""
}

func (x *License) Range() {
	for _, x := range x.items {
		x.Do()
	}
}

func (x *License) ForClause() {
	for x := 0; x < 3; x++ {
		_ = x.Idx
	}
}

func (x *License) Block() {
	{
		x := other()
		x.Inner()
	}
	x.Outer()
}

func (x *License) Var() {
	var x = 1
	var (
		y = x.Y
	)
	_ = y
}

func (x *License) TypeSwitch(v interface{}) {
	switch x := x.Value().(type) {
	case int:
		_ = x.Int
	}
}

func (x *License) Case(v int) {
	switch v {
	case 1:
		x := 2
		_ = x.One
	case 2:
		_ = x.Two
	}
}

func (x *License) Select() {
	select {
	case x := <-x.ch:
		x.Recv()
	}
}

func (x *License) Closure() {
	f := func(x int, rest ...int) (r int) { return x.Param + r.Result }
	g := func() { x.Captured() }
	f(1)
	g()
}
`)

	cases := []struct {
		name   string
		fields []string
		calls  []string
	}{
		{"License.Package", []string{"TTL"}, []string{"Warm"}},
		{"License.Default", []string{}, []string{}},
		{"License.GetText", []string{}, []string{"GetSource"}},
		{"License.Range", []string{"items"}, []string{}},
		{"License.ForClause", []string{}, []string{}},
		{"License.Block", []string{}, []string{"Outer"}},
		{"License.Var", []string{}, []string{}},
		{"License.TypeSwitch", []string{}, []string{"Value"}},
		{"License.Case", []string{"Two"}, []string{}},
		{"License.Select", []string{"ch"}, []string{}},
		{"License.Closure", []string{}, []string{"Captured"}},
	}
	for _, tc := range cases {
		fn, ok := functions[tc.name]
		if !ok {
			t.Errorf("missing function %q", tc.name)
			continue
		}
		if got := slices.Sorted(maps.Keys(fn.Fields)); !equalStrings(got, tc.fields) {
			t.Errorf("%s: fields = %v, want %v", tc.name, got, tc.fields)
		}
		if got := slices.Sorted(maps.Keys(fn.Calls)); !equalStrings(got, tc.calls) {
			t.Errorf("%s: calls = %v, want %v", tc.name, got, tc.calls)
		}
	}
}

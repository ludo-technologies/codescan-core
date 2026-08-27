package rust

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
	functions := analyze(t, `
fn free() {}
struct S;
struct G<T>(T);
impl S { fn method(&self) {} }
impl<T> G<T> { fn generic(&self) {} }
impl fmt::Display for S { fn fmt(&self) {} }
trait Tr { fn provided(&self) {} fn required(&self); }
mod m { fn inner() { fn nested() {} } }
fn with_closure() { let f = |x| x; f(1); }
`)

	want := map[string]string{
		"free":         "function",
		"S.method":     "method",
		"G<T>.generic": "method",
		"S.fmt":        "method",
		"Tr.provided":  "method",
		"inner":        "function",
		"nested":       "function",
		"with_closure": "function",
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
	}
}

func TestComplexityCountsDecisionPoints(t *testing.T) {
	cases := []struct {
		name      string
		body      string
		want      int
		decisions map[string]int
	}{
		{"none", `a`, 1, map[string]int{}},
		{"if", `if a { 1 } else { 2 }`, 2, map[string]int{"branch": 1}},
		{"else if chain", `if a { 1 } else if b { 2 } else { 3 }`, 3, map[string]int{"branch": 2}},
		{"if let", `if let Some(x) = o { x } else { 0 }`, 2, map[string]int{"branch": 1}},
		{"match arms except the last", `match n { 1 => 1, 2 | 3 => 2, _ => 0 }`, 3, map[string]int{"case": 2}},
		{"single arm match", `match n { _ => 0 }`, 1, map[string]int{}},
		{"for", `for i in 0..3 { }`, 2, map[string]int{"loop": 1}},
		{"while", `while a { }`, 2, map[string]int{"loop": 1}},
		{"while let", `while let Some(x) = o { }`, 2, map[string]int{"loop": 1}},
		{"loop", `loop { break; }`, 2, map[string]int{"loop": 1}},
		{"logical operators", `if a && b || c { }`, 4, map[string]int{"branch": 1, "logical_operator": 2}},
		{"bitwise operators do not count", `n & 1 | 2`, 1, map[string]int{}},
		{"question mark", `let v = r?; v`, 2, map[string]int{"try_operator": 1}},
		{"match guard", `match o { Some(v) if v > 0 => v, _ => 0 }`, 3, map[string]int{"case": 1, "branch": 1}},
		{"let else", `let Some(v) = o else { return 0 }; v`, 2, map[string]int{"branch": 1}},
		{"let chain", `if let Some(v) = o && v > 0 && a { v } else { 0 }`, 4, map[string]int{"branch": 1, "logical_operator": 2}},
		{"closure counts toward its enclosing function", `let f = |x| if x { 1 } else { 2 }; f(a)`, 2, map[string]int{"branch": 1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fn := analyze(t, "fn f(a: bool, b: bool, c: bool, n: i32, o: Option<i32>, r: Result<i32, ()>) -> i32 {\n\t"+tc.body+"\n}\n")["f"]
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

func TestTestCodeIsMarked(t *testing.T) {
	functions := analyze(t, `
fn production() {}

#[test]
fn unit() {}

#[cfg(test)]
mod tests {
    use super::*;
    fn helper() {}
    #[test]
    fn in_module() {}
}

#[cfg(feature = "x")]
mod feature { fn gated() {} }

#[test]
#[cfg(feature = "alloc")]
fn test_then_cfg() {}

#[cfg(feature = "alloc")]
#[test]
fn cfg_then_test() {}

#[allow(dead_code)]
#[cfg(test)]
mod more_tests { fn helper2() {} }
`)
	for name, want := range map[string]bool{
		"production": false, "unit": true, "helper": true, "in_module": true, "gated": false,
		"test_then_cfg": true, "cfg_then_test": true, "helper2": true,
	} {
		if got := functions[name].IsTest; got != want {
			t.Errorf("%s: IsTest = %v, want %v", name, got, want)
		}
	}
}

func TestContentDropsComments(t *testing.T) {
	fn := analyze(t, "/// Doc.\nfn f() -> i32 {\n\t// line\n\t1 /* block */\n}\n")["f"]
	if strings.Contains(fn.Content, "line") || strings.Contains(fn.Content, "block") || strings.Contains(fn.Content, "Doc") {
		t.Errorf("content keeps comments: %q", fn.Content)
	}
	if fn.CodeLines != 3 {
		t.Errorf("code lines = %d, want 3", fn.CodeLines)
	}
}

func TestSyntaxErrorIsReported(t *testing.T) {
	_, err := Language.Analyze([]byte("fn f() {\n\tif {\n"))
	if err == nil || !strings.Contains(err.Error(), "syntax error") {
		t.Fatalf("err = %v, want a syntax error", err)
	}
}

func TestTestFiles(t *testing.T) {
	for path, want := range map[string]bool{
		"src/lib.rs":                   false,
		"src/parser/tests.rs":          true,
		"src/bin/integration_tests.rs": true,
		"src/tests/mod.rs":             true,
		"tests/integration.rs":         true,
		"src/contests/x.rs":            false,
	} {
		if got := Language.IsTestFile(path); got != want {
			t.Errorf("IsTestFile(%q) = %v, want %v", path, got, want)
		}
	}
}

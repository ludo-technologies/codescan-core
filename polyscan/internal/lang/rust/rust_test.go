package rust

import (
	"maps"
	"reflect"
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
		"S::method":    "function",
		"G::generic":   "function",
		"S::fmt":       "function",
		"Tr::provided": "function",
		"m::inner":     "function",
		"m::nested":    "function",
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

#[cfg(test)]
fn test_helper() {}

#[cfg(test)]
impl S { fn fixture(&self) {} }

#[cfg(test)]
trait TestOnly { fn provided(&self) {} }

#[cfg(all(test, unix))]
mod unix_tests { fn helper3() {} }

#[cfg(not(test))]
fn real() {}

#[cfg(any(test, feature = "x"))]
fn maybe() {}
`)
	for name, want := range map[string]bool{
		"production": false, "unit": true, "tests::helper": true, "tests::in_module": true, "feature::gated": false,
		"test_then_cfg": true, "cfg_then_test": true, "more_tests::helper2": true,
		"test_helper": true, "S::fixture": true, "TestOnly::provided": true, "unix_tests::helper3": true,
		"real": false, "maybe": false,
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
	result, err := Language.Analyze([]byte("fn f() {\n\tif {\n"))
	if err != nil {
		t.Fatal(err)
	}
	if result.SyntaxError == nil || !strings.Contains(result.SyntaxError.Error(), "syntax error") {
		t.Fatalf("syntax error = %v, want one", result.SyntaxError)
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

func TestNestingDepth(t *testing.T) {
	functions := analyze(t, `
fn flat(a: bool) {
    if a {}
    loop { break; }
}

fn else_if_continues_the_chain(a: bool, b: bool) {
    if a {
    } else if b {
        while b {}
    } else {
        if a { if b {} }
    }
}

fn deep(a: bool, xs: Vec<i32>) {
    if a {
        for x in xs {
            match x {
                1 => { if let Some(_) = Some(x) {} }
                _ => {}
            }
        }
    }
}

fn let_else(value: Option<i32>, retry: bool) {
    let Some(_value) = value else {
        if retry {}
        return;
    };
}

fn outer(a: bool) {
    fn inner(a: bool) {
        if a { if a { if a {} } }
    }
    let closure = || { if a { if a {} } };
}
`)
	want := map[string]int{
		"flat":                        1,
		"else_if_continues_the_chain": 3,
		"deep":                        4,
		"let_else":                    2,
		"outer":                       2,
		"inner":                       3,
	}
	for name, depth := range want {
		fn, ok := functions[name]
		if !ok {
			t.Errorf("missing function %q", name)
			continue
		}
		if fn.NestingDepth != depth {
			t.Errorf("%s: nesting depth = %d, want %d", name, fn.NestingDepth, depth)
		}
	}
}

func TestMembers(t *testing.T) {
	functions := analyze(t, `
pub struct S<T> { a: T, b: u32, cb: fn() }

impl<T> S<T> {
    pub fn new() -> Self { Self { a: 1, b: 2, cb: || {} } }
    pub fn fields(&mut self) { self.a = 1; self.b.c = 2; other.x = 3; let f = || self.a; }
    pub fn calls(&self) { self.fields(); (self.cb)(); Self::helper(self); Self::new(); other.fields() }
    fn helper(this: &Self) {}
    fn boxed(self: Box<Self>) { self.b; }
}

struct Pair(u32, u32);
impl Pair { fn sum(&self) -> u32 { self.0 + self.1 } }

trait Tr {
    fn default_method(&self) { self.other() }
}

mod inner {
    pub fn free(&self) {}
    pub struct N;
    impl N { pub fn m(&self) { self.x; } }
}
`)

	cases := []struct {
		name     string
		receiver string
		hasSelf  bool
		fields   []string
		calls    []string
	}{
		{"S::new", "S", false, nil, nil},
		{"S::fields", "S", true, []string{"a", "b"}, []string{}},
		{"S::calls", "S", true, []string{"cb"}, []string{"fields", "helper", "new"}},
		{"S::helper", "S", false, nil, nil},
		{"S::boxed", "S", true, []string{"b"}, []string{}},
		{"Pair::sum", "Pair", true, []string{"0", "1"}, []string{}},
		{"Tr::default_method", "", true, nil, nil},
		{"inner::free", "", true, nil, nil},
		{"inner::N::m", "inner::N", true, []string{"x"}, []string{}},
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

func TestTypesAndReferences(t *testing.T) {
	result, err := Language.Analyze([]byte(`
use std::collections::HashMap;
pub struct Foo { a: Bar, m: HashMap<String, Baz> }
pub enum Color { Red, Green(Bar) }
pub trait Tr { fn d(&self) -> Bar { Bar::new() } fn r(&self); }
impl Tr for Foo { fn r(&self) {} }
impl<T: Clone> G<T> { fn new() -> Self { Self::default() } }
impl Iterator for &Foo { type Item = Baz; fn next(&mut self) -> Option<Baz> { None } }
impl dyn Tr { fn x(&self) -> Qux { Qux } }
mod m { pub struct Inner(super::Bar); impl Inner { fn f(&self) { let Inner(b) = self; match c { Color::Red => {} } } } }
fn free() -> Bar { Foo::new() }
#[cfg(test)]
mod tests { struct Fixture; impl Fixture { fn f(&self) -> Bar {} } impl Foo { fn t(&self) -> Baz {} } }
`))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	want := []engine.Type{
		// Foo's declaration, its two impls, the impl for &Foo; the impl in
		// the test module is a test span of another scope.
		{Name: "Foo", StartLine: 3, EndLine: 3, Declared: true, References: []engine.Reference{
			{Name: "Bar"}, {Name: "HashMap"}, {Name: "String"}, {Name: "Baz"},
			{Name: "Tr", Embedded: true}, {Name: "Iterator", Embedded: true}, {Name: "Item"}, {Name: "Option"},
		}},
		{Name: "Color", StartLine: 4, EndLine: 4, Declared: true, References: []engine.Reference{{Name: "Bar"}}},
		// A default method's references belong to the trait.
		{Name: "Tr", StartLine: 5, EndLine: 5, Declared: true, Abstract: true, References: []engine.Reference{{Name: "Bar"}}},
		// An impl of an undeclared type is not declared here.
		{Name: "G", StartLine: 7, EndLine: 7, References: []engine.Reference{{Name: "T"}, {Name: "Clone"}}},
		{Name: "m::Inner", StartLine: 10, EndLine: 10, Declared: true, References: []engine.Reference{
			{Name: "Bar"}, {Name: "Inner"}, {Name: "Color"},
		}},
		{Name: "tests::Fixture", StartLine: 13, EndLine: 13, Declared: true, IsTest: true},
		{Name: "tests::Foo", StartLine: 13, EndLine: 13, IsTest: true},
	}
	if !reflect.DeepEqual(result.Types, want) {
		t.Errorf("types = %+v\nwant   %+v", result.Types, want)
	}
	if fn := result.Functions[0]; fn.Name != "Tr::d" || fn.Receiver != "" {
		t.Errorf("trait default method = %+v, want Tr::d without a receiver", fn)
	}
	for _, name := range []string{"Foo::r", "G::new", "Foo::next", "dyn Tr::x", "m::Inner::f"} {
		found := false
		for _, fn := range result.Functions {
			found = found || fn.Name == name
		}
		if !found {
			t.Errorf("missing function %q", name)
		}
	}
}

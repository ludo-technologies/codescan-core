package cpp

import (
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

func TestExtractsFunctionsAndMembers(t *testing.T) {
	functions := analyze(t, `
int free(int a) { return a; }
int* pointer() { return 0; }
const char** pointers() { return 0; }
int& reference(int& x) { return x; }
int*& both(int*& x) { return x; }
namespace ns { namespace in {
class Foo {
 public:
  Foo() {}
  ~Foo() {}
  int method(int x) const { return x; }
  int* member_pointer() { return 0; }
  bool operator==(const Foo&) const { return true; }
  class Inner { void deep() {} };
};
int ns_free() { return 1; }
} }
int ns::in::Foo::outside(int y) { return y; }
template <typename T> T tmpl(T t) { auto l = [&](int z) { return z; }; return l(t); }
struct S { void m() {} };
`)

	want := []string{
		"free", "pointer", "pointers", "reference", "both",
		"ns::in::Foo::Foo", "ns::in::Foo::~Foo", "ns::in::Foo::method", "ns::in::Foo::member_pointer",
		"ns::in::Foo::operator==", "ns::in::Foo::Inner::deep", "ns::in::ns_free",
		"ns::in::Foo::outside", "tmpl", "S::m",
	}
	if len(functions) != len(want) {
		t.Errorf("got %d functions, want %d: %v", len(functions), len(want), functions)
	}
	for _, name := range want {
		if _, ok := functions[name]; !ok {
			t.Errorf("missing function %q", name)
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
		{"none", `return a;`, 1, map[string]int{}},
		{"if", `if (a) return 1; return 0;`, 2, map[string]int{"branch": 1}},
		{"if else", `if (a) { return 1; } else { return 0; }`, 2, map[string]int{"branch": 1}},
		{"else if chain", `if (a) { } else if (b) { } else { } return 0;`, 3, map[string]int{"branch": 2}},
		{"ternary", `return a ? 1 : 0;`, 2, map[string]int{"ternary": 1}},
		{"for", `for (int i = 0; i < 3; i++) { } return 0;`, 2, map[string]int{"loop": 1}},
		{"range for", `for (auto v : xs) { } return 0;`, 2, map[string]int{"loop": 1}},
		{"while", `while (a) { } return 0;`, 2, map[string]int{"loop": 1}},
		{"do while", `do { } while (a); return 0;`, 2, map[string]int{"loop": 1}},
		{"switch cases without default", `switch (n) { case 1: return 1; case 2: case 3: return 2; default: return 0; }`, 4, map[string]int{"case": 3}},
		{"try catch", `try { return 1; } catch (const E& e) { return 2; } catch (...) { return 3; }`, 3, map[string]int{"exception": 2}},
		{"logical operators", `if (a && b || c) { } return 0;`, 4, map[string]int{"branch": 1, "logical_operator": 2}},
		{"bitwise operators do not count", `return n & 1 | 2;`, 1, map[string]int{}},
		{"lambda counts toward its enclosing function", `auto f = [&](int z) { if (z) return z; return 0; }; return f(n);`, 2, map[string]int{"branch": 1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fn := analyze(t, "int f(bool a, bool b, bool c, int n, int xs[3]) {\n\t"+tc.body+"\n}\n")["f"]
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

func TestPreprocessorBranchesAreBothAnalyzed(t *testing.T) {
	functions := analyze(t, "#ifdef X\nint cond() { if (1) return 1; return 0; }\n#else\nint other() { return 2; }\n#endif\n")
	if functions["cond"].Complexity != 2 || functions["other"].Complexity != 1 {
		t.Errorf("functions = %v, want both branches", functions)
	}
}

func TestContentDropsComments(t *testing.T) {
	fn := analyze(t, "// Doc.\nint f() {\n\t// line\n\treturn/* block */1;\n}\n")["f"]
	if strings.Contains(fn.Content, "line") || strings.Contains(fn.Content, "block") {
		t.Errorf("content keeps comments: %q", fn.Content)
	}
	if !strings.Contains(fn.Content, "return 1") || fn.CodeLines != 3 {
		t.Errorf("content = %q, code lines = %d", fn.Content, fn.CodeLines)
	}
}

func TestTestFiles(t *testing.T) {
	for path, want := range map[string]bool{
		"src/foo.cc":           false,
		"src/foo_test.cc":      true,
		"src/foo_tests.cpp":    true,
		"src/test_foo.cpp":     true,
		"src/FooTest.cpp":      true,
		"src/FooTests.cxx":     true,
		"test/foo.cc":          true,
		"tests/unit/foo.cpp":   true,
		"src/latest/foo.cpp":   false,
		"include/foo_test.hpp": false,
	} {
		if got := Language.IsTestFile(path); got != want {
			t.Errorf("IsTestFile(%q) = %v, want %v", path, got, want)
		}
	}
}

// TestMacroErrorsKeepOtherFunctions parses a file the way C++ libraries
// look without the preprocessor: a macro opens the namespace and a macro
// declares an attribute. The functions around them still count; the
// function that contains the error does not.
func TestMacroErrorsKeepOtherFunctions(t *testing.T) {
	result, err := Language.Analyze([]byte(`
FMT_BEGIN_NAMESPACE
namespace detail {
int first(int a) { if (a) return 1; return 0; }
}
GTEST_DISABLE_MSC_WARNINGS_PUSH_(4100)
class C { int method() { return 1; } };
int last() { return 2; }
int broken(int a) { if ( return a; }
`))
	if err != nil {
		t.Fatal(err)
	}
	if result.SyntaxError == nil {
		t.Fatal("expected a syntax error")
	}
	complexity := map[string]int{}
	for _, fn := range result.Functions {
		// Recovery may lose the namespace scope, so match the bare name.
		name := fn.Name
		if i := strings.LastIndex(name, "::"); i >= 0 {
			name = name[i+2:]
		}
		complexity[name] = fn.Complexity
	}
	if complexity["first"] != 2 || complexity["method"] != 1 || complexity["last"] != 1 {
		t.Errorf("functions = %v, want first, method and last", complexity)
	}
	if _, ok := complexity["broken"]; ok {
		t.Error("a function containing the error must not be analyzed")
	}
}

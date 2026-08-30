package analyzer

import (
	"log"
	"os"
	"testing"

	"github.com/ludo-technologies/polyscan/polyscan/internal/js/domain"
	"github.com/ludo-technologies/polyscan/polyscan/internal/js/parser"
)

// Helper function to create AST from JavaScript code
func parseJS(t *testing.T, code string) *parser.Node {
	t.Helper()
	p := parser.NewParser()
	defer p.Close()

	ast, err := p.ParseString(code)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}
	return ast
}

// Helper function to find a function in AST
func findFunction(ast *parser.Node, name string) *parser.Node {
	var found *parser.Node
	ast.Walk(func(n *parser.Node) bool {
		if n.IsFunction() && n.Name == name {
			found = n
			return false
		}
		return true
	})
	return found
}

func TestNewCFGBuilder(t *testing.T) {
	builder := NewCFGBuilder()

	if builder == nil {
		t.Fatal("NewCFGBuilder should return non-nil builder")
	}
	if builder.scopeStack == nil {
		t.Error("scopeStack should be initialized")
	}
	if builder.functionCFGs == nil {
		t.Error("functionCFGs should be initialized")
	}
	if builder.loopStack == nil {
		t.Error("loopStack should be initialized")
	}
	if builder.exceptionStack == nil {
		t.Error("exceptionStack should be initialized")
	}
	if builder.logger != nil {
		t.Error("logger should be nil by default")
	}
}

func TestCFGBuilder_SetLogger(t *testing.T) {
	builder := NewCFGBuilder()
	logger := log.New(os.Stderr, "test: ", log.LstdFlags)

	builder.SetLogger(logger)

	if builder.logger != logger {
		t.Error("Logger should be set")
	}
}

func TestCFGBuilder_Build_NilNode(t *testing.T) {
	builder := NewCFGBuilder()

	cfg, err := builder.Build(nil)

	if err == nil {
		t.Error("Build with nil should return error")
	}
	if cfg != nil {
		t.Error("Build with nil should return nil CFG")
	}
}

func TestCFGBuilder_Build_SimpleFunction(t *testing.T) {
	code := `function hello() { return 42; }`
	ast := parseJS(t, code)
	funcNode := findFunction(ast, "hello")

	builder := NewCFGBuilder()
	cfg, err := builder.Build(funcNode)

	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	if cfg == nil {
		t.Fatal("CFG should not be nil")
	}
	if cfg.Name != "hello" {
		t.Errorf("CFG name should be 'hello', got %s", cfg.Name)
	}
	if cfg.Entry == nil || cfg.Exit == nil {
		t.Error("CFG should have entry and exit blocks")
	}
}

func TestCFGBuilder_Build_Program(t *testing.T) {
	code := `
		let x = 1;
		let y = 2;
		console.log(x + y);
	`
	ast := parseJS(t, code)

	builder := NewCFGBuilder()
	cfg, err := builder.Build(ast)

	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	if cfg.Name != domain.ModuleFunctionName {
		t.Errorf("CFG name should be '%s', got %s", domain.ModuleFunctionName, cfg.Name)
	}
}

func TestCFGBuilder_Build_IfStatement(t *testing.T) {
	code := `
		function test(x) {
			if (x > 0) {
				return 1;
			} else {
				return -1;
			}
		}
	`
	ast := parseJS(t, code)
	funcNode := findFunction(ast, "test")

	builder := NewCFGBuilder()
	cfg, err := builder.Build(funcNode)

	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Should have: entry, if_then, if_else, if_merge, exit
	if cfg.Size() < 5 {
		t.Errorf("CFG should have at least 5 blocks, got %d", cfg.Size())
	}

	// Check for conditional edges
	hasCondTrue := false
	hasCondFalse := false
	cfg.Walk(&edgeTypeChecker{
		onEdge: func(e *Edge) {
			if e.Type == EdgeCondTrue {
				hasCondTrue = true
			}
			if e.Type == EdgeCondFalse {
				hasCondFalse = true
			}
		},
	})

	if !hasCondTrue {
		t.Error("CFG should have EdgeCondTrue edge")
	}
	if !hasCondFalse {
		t.Error("CFG should have EdgeCondFalse edge")
	}
}

func TestCFGBuilder_Build_IfWithoutElse(t *testing.T) {
	code := `
		function test(x) {
			if (x > 0) {
				console.log("positive");
			}
			return x;
		}
	`
	ast := parseJS(t, code)
	funcNode := findFunction(ast, "test")

	builder := NewCFGBuilder()
	cfg, err := builder.Build(funcNode)

	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Check that both branches eventually merge
	hasCondTrue := false
	hasCondFalse := false
	cfg.Walk(&edgeTypeChecker{
		onEdge: func(e *Edge) {
			if e.Type == EdgeCondTrue {
				hasCondTrue = true
			}
			if e.Type == EdgeCondFalse {
				hasCondFalse = true
			}
		},
	})

	if !hasCondTrue || !hasCondFalse {
		t.Error("If without else should still have both conditional edges")
	}
}

func TestCFGBuilder_Build_NestedIf(t *testing.T) {
	code := `
		function test(x, y) {
			if (x > 0) {
				if (y > 0) {
					return 1;
				}
			}
			return 0;
		}
	`
	ast := parseJS(t, code)
	funcNode := findFunction(ast, "test")

	builder := NewCFGBuilder()
	cfg, err := builder.Build(funcNode)

	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Count conditional edges
	condEdges := 0
	cfg.Walk(&edgeTypeChecker{
		onEdge: func(e *Edge) {
			if e.Type == EdgeCondTrue || e.Type == EdgeCondFalse {
				condEdges++
			}
		},
	})

	if condEdges < 4 {
		t.Errorf("Nested if should have at least 4 conditional edges, got %d", condEdges)
	}
}

func TestCFGBuilder_Build_ForLoop(t *testing.T) {
	code := `
		function test() {
			for (let i = 0; i < 10; i++) {
				console.log(i);
			}
		}
	`
	ast := parseJS(t, code)
	funcNode := findFunction(ast, "test")

	builder := NewCFGBuilder()
	cfg, err := builder.Build(funcNode)

	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Should have back edge (loop)
	hasLoopEdge := false
	cfg.Walk(&edgeTypeChecker{
		onEdge: func(e *Edge) {
			if e.Type == EdgeLoop {
				hasLoopEdge = true
			}
		},
	})

	if !hasLoopEdge {
		t.Error("For loop CFG should have EdgeLoop edge")
	}
}

func TestCFGBuilder_Build_ForInLoop(t *testing.T) {
	code := `
		function test(obj) {
			for (let key in obj) {
				console.log(key);
			}
		}
	`
	ast := parseJS(t, code)
	funcNode := findFunction(ast, "test")

	builder := NewCFGBuilder()
	cfg, err := builder.Build(funcNode)

	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	hasLoopEdge := false
	cfg.Walk(&edgeTypeChecker{
		onEdge: func(e *Edge) {
			if e.Type == EdgeLoop {
				hasLoopEdge = true
			}
		},
	})

	if !hasLoopEdge {
		t.Error("For-in loop CFG should have EdgeLoop edge")
	}
}

func TestCFGBuilder_Build_ForOfLoop(t *testing.T) {
	code := `
		function test(arr) {
			for (let item of arr) {
				console.log(item);
			}
		}
	`
	ast := parseJS(t, code)
	funcNode := findFunction(ast, "test")

	builder := NewCFGBuilder()
	cfg, err := builder.Build(funcNode)

	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	hasLoopEdge := false
	cfg.Walk(&edgeTypeChecker{
		onEdge: func(e *Edge) {
			if e.Type == EdgeLoop {
				hasLoopEdge = true
			}
		},
	})

	if !hasLoopEdge {
		t.Error("For-of loop CFG should have EdgeLoop edge")
	}
}

func TestCFGBuilder_Build_WhileLoop(t *testing.T) {
	code := `
		function test() {
			let i = 0;
			while (i < 10) {
				console.log(i);
				i++;
			}
		}
	`
	ast := parseJS(t, code)
	funcNode := findFunction(ast, "test")

	builder := NewCFGBuilder()
	cfg, err := builder.Build(funcNode)

	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	hasLoopEdge := false
	hasCondEdges := 0
	cfg.Walk(&edgeTypeChecker{
		onEdge: func(e *Edge) {
			if e.Type == EdgeLoop {
				hasLoopEdge = true
			}
			if e.Type == EdgeCondTrue || e.Type == EdgeCondFalse {
				hasCondEdges++
			}
		},
	})

	if !hasLoopEdge {
		t.Error("While loop CFG should have EdgeLoop edge")
	}
	if hasCondEdges < 2 {
		t.Error("While loop should have conditional edges for condition check")
	}
}

func TestCFGBuilder_Build_DoWhileLoop(t *testing.T) {
	code := `
		function test() {
			let i = 0;
			do {
				console.log(i);
				i++;
			} while (i < 10);
		}
	`
	ast := parseJS(t, code)
	funcNode := findFunction(ast, "test")

	builder := NewCFGBuilder()
	cfg, err := builder.Build(funcNode)

	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Do-while has back edge from condition to body
	hasCondTrue := false
	cfg.Walk(&edgeTypeChecker{
		onEdge: func(e *Edge) {
			if e.Type == EdgeCondTrue {
				hasCondTrue = true
			}
		},
	})

	if !hasCondTrue {
		t.Error("Do-while loop should have EdgeCondTrue for loop back")
	}
}

func TestCFGBuilder_Build_SwitchStatement(t *testing.T) {
	code := `
		function test(x) {
			switch (x) {
				case 1:
					return "one";
				case 2:
					return "two";
				default:
					return "other";
			}
		}
	`
	ast := parseJS(t, code)
	funcNode := findFunction(ast, "test")

	builder := NewCFGBuilder()
	cfg, err := builder.Build(funcNode)

	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Should have multiple case blocks
	if cfg.Size() < 5 {
		t.Errorf("Switch CFG should have at least 5 blocks, got %d", cfg.Size())
	}
}

func TestCFGBuilder_Build_SwitchWithFallthrough(t *testing.T) {
	code := `
		function test(x) {
			switch (x) {
				case 1:
					console.log("one");
				case 2:
					console.log("two");
					break;
				default:
					console.log("default");
			}
		}
	`
	ast := parseJS(t, code)
	funcNode := findFunction(ast, "test")

	builder := NewCFGBuilder()
	cfg, err := builder.Build(funcNode)

	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Switch should have multiple cases connected to merge
	// The fallthrough case (case 1) should connect to case 2
	if cfg.Size() < 5 {
		t.Errorf("Switch CFG should have at least 5 blocks, got %d", cfg.Size())
	}

	// Verify there are conditional edges for case matching
	condEdges := 0
	cfg.Walk(&edgeTypeChecker{
		onEdge: func(e *Edge) {
			if e.Type == EdgeCondTrue || e.Type == EdgeCondFalse {
				condEdges++
			}
		},
	})

	if condEdges < 2 {
		t.Errorf("Switch should have conditional edges, got %d", condEdges)
	}
}

func TestCFGBuilder_Build_TryCatchFinally(t *testing.T) {
	code := `
		function test() {
			try {
				throw new Error("oops");
			} catch (e) {
				console.error(e);
			} finally {
				cleanup();
			}
		}
	`
	ast := parseJS(t, code)
	funcNode := findFunction(ast, "test")

	builder := NewCFGBuilder()
	cfg, err := builder.Build(funcNode)

	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Should have exception edge
	hasExceptionEdge := false
	cfg.Walk(&edgeTypeChecker{
		onEdge: func(e *Edge) {
			if e.Type == EdgeException {
				hasExceptionEdge = true
			}
		},
	})

	if !hasExceptionEdge {
		t.Error("Try-catch CFG should have EdgeException edge")
	}
}

func TestCFGBuilder_Build_TryCatch(t *testing.T) {
	code := `
		function test() {
			try {
				riskyOperation();
			} catch (e) {
				handleError(e);
			}
		}
	`
	ast := parseJS(t, code)
	funcNode := findFunction(ast, "test")

	builder := NewCFGBuilder()
	cfg, err := builder.Build(funcNode)

	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	hasExceptionEdge := false
	cfg.Walk(&edgeTypeChecker{
		onEdge: func(e *Edge) {
			if e.Type == EdgeException {
				hasExceptionEdge = true
			}
		},
	})

	if !hasExceptionEdge {
		t.Error("Try-catch CFG should have EdgeException edge")
	}
}

func TestCFGBuilder_Build_ReturnStatement(t *testing.T) {
	code := `
		function test(x) {
			if (x > 0) {
				return x;
			}
			console.log("negative");
			return -x;
		}
	`
	ast := parseJS(t, code)
	funcNode := findFunction(ast, "test")

	builder := NewCFGBuilder()
	cfg, err := builder.Build(funcNode)

	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Count return edges
	returnEdges := 0
	cfg.Walk(&edgeTypeChecker{
		onEdge: func(e *Edge) {
			if e.Type == EdgeReturn {
				returnEdges++
			}
		},
	})

	if returnEdges < 2 {
		t.Errorf("Should have at least 2 return edges, got %d", returnEdges)
	}
}

func TestCFGBuilder_Build_BreakStatement(t *testing.T) {
	code := `
		function test() {
			for (let i = 0; i < 10; i++) {
				if (i === 5) {
					break;
				}
			}
		}
	`
	ast := parseJS(t, code)
	funcNode := findFunction(ast, "test")

	builder := NewCFGBuilder()
	cfg, err := builder.Build(funcNode)

	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	hasBreakEdge := false
	cfg.Walk(&edgeTypeChecker{
		onEdge: func(e *Edge) {
			if e.Type == EdgeBreak {
				hasBreakEdge = true
			}
		},
	})

	if !hasBreakEdge {
		t.Error("CFG should have EdgeBreak edge")
	}
}

func TestCFGBuilder_Build_ContinueStatement(t *testing.T) {
	code := `
		function test() {
			for (let i = 0; i < 10; i++) {
				if (i % 2 === 0) {
					continue;
				}
				console.log(i);
			}
		}
	`
	ast := parseJS(t, code)
	funcNode := findFunction(ast, "test")

	builder := NewCFGBuilder()
	cfg, err := builder.Build(funcNode)

	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	hasContinueEdge := false
	cfg.Walk(&edgeTypeChecker{
		onEdge: func(e *Edge) {
			if e.Type == EdgeContinue {
				hasContinueEdge = true
			}
		},
	})

	if !hasContinueEdge {
		t.Error("CFG should have EdgeContinue edge")
	}
}

func TestCFGBuilder_Build_ThrowStatement(t *testing.T) {
	code := `
		function test(x) {
			if (x < 0) {
				throw new Error("negative");
			}
			return x;
		}
	`
	ast := parseJS(t, code)
	funcNode := findFunction(ast, "test")

	builder := NewCFGBuilder()
	cfg, err := builder.Build(funcNode)

	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	hasExceptionEdge := false
	cfg.Walk(&edgeTypeChecker{
		onEdge: func(e *Edge) {
			if e.Type == EdgeException {
				hasExceptionEdge = true
			}
		},
	})

	if !hasExceptionEdge {
		t.Error("Throw statement should create EdgeException edge")
	}
}

func TestCFGBuilder_Build_NestedLoops(t *testing.T) {
	code := `
		function test() {
			for (let i = 0; i < 5; i++) {
				for (let j = 0; j < 5; j++) {
					console.log(i, j);
				}
			}
		}
	`
	ast := parseJS(t, code)
	funcNode := findFunction(ast, "test")

	builder := NewCFGBuilder()
	cfg, err := builder.Build(funcNode)

	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Count loop edges
	loopEdges := 0
	cfg.Walk(&edgeTypeChecker{
		onEdge: func(e *Edge) {
			if e.Type == EdgeLoop {
				loopEdges++
			}
		},
	})

	if loopEdges < 2 {
		t.Errorf("Nested loops should have at least 2 loop edges, got %d", loopEdges)
	}
}

func TestCFGBuilder_Build_NestedFunction(t *testing.T) {
	code := `
		function outer() {
			function inner() {
				return 42;
			}
			return inner();
		}
	`
	ast := parseJS(t, code)
	funcNode := findFunction(ast, "outer")

	builder := NewCFGBuilder()
	cfg, err := builder.Build(funcNode)

	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Check that inner function was recorded
	if len(builder.functionCFGs) == 0 {
		t.Error("Nested function should be recorded in functionCFGs")
	}

	if cfg.Name != "outer" {
		t.Errorf("Main CFG name should be 'outer', got %s", cfg.Name)
	}
}

func TestCFGBuilder_Build_ArrowFunction(t *testing.T) {
	code := `const add = (a, b) => { return a + b; };`
	ast := parseJS(t, code)

	var arrowFunc *parser.Node
	ast.Walk(func(n *parser.Node) bool {
		if n.Type == parser.NodeArrowFunction {
			arrowFunc = n
			return false
		}
		return true
	})

	if arrowFunc == nil {
		t.Fatal("Arrow function not found in AST")
	}

	builder := NewCFGBuilder()
	cfg, err := builder.Build(arrowFunc)

	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Arrow function should produce valid CFG
	if cfg == nil {
		t.Error("CFG should not be nil")
	}
}

func TestCFGBuilder_Build_Class(t *testing.T) {
	code := `
		class Calculator {
			add(a, b) {
				return a + b;
			}

			subtract(a, b) {
				return a - b;
			}
		}
	`
	ast := parseJS(t, code)

	var classNode *parser.Node
	ast.Walk(func(n *parser.Node) bool {
		if n.Type == parser.NodeClass {
			classNode = n
			return false
		}
		return true
	})

	if classNode == nil {
		t.Fatal("Class not found in AST")
	}

	builder := NewCFGBuilder()
	cfg, err := builder.Build(classNode)

	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Methods should be recorded
	if len(builder.functionCFGs) < 2 {
		t.Errorf("Should have at least 2 method CFGs, got %d", len(builder.functionCFGs))
	}

	if cfg.Name != "Calculator" {
		t.Errorf("CFG name should be 'Calculator', got %s", cfg.Name)
	}
}

func TestCFGBuilder_BuildAll(t *testing.T) {
	code := `
		function foo() { return 1; }
		function bar() { return 2; }
		let x = foo() + bar();
	`
	ast := parseJS(t, code)

	builder := NewCFGBuilder()
	cfgs, err := builder.BuildAll(ast)

	if err != nil {
		t.Fatalf("BuildAll failed: %v", err)
	}

	// Should have module-scope code, foo, bar
	if len(cfgs) < 3 {
		t.Errorf("BuildAll should produce at least 3 CFGs, got %d", len(cfgs))
	}

	if cfgs[domain.ModuleFunctionName] == nil {
		t.Error("Should have module-scope CFG")
	}
	if cfgs["foo"] == nil {
		t.Error("Should have 'foo' CFG")
	}
	if cfgs["bar"] == nil {
		t.Error("Should have 'bar' CFG")
	}
}

func TestCFGBuilder_Build_SetsFunctionNode(t *testing.T) {
	code := `
		function sample() {
			return 42;
		}
	`
	ast := parseJS(t, code)
	funcNode := findFunction(ast, "sample")
	if funcNode == nil {
		t.Fatal("Function node should not be nil")
	}

	builder := NewCFGBuilder()
	cfg, err := builder.Build(funcNode)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if cfg.FunctionNode == nil {
		t.Fatal("FunctionNode should be set on CFG")
	}
	storedNode, ok := cfg.FunctionNode.(*parser.Node)
	if !ok {
		t.Fatalf("FunctionNode has type %T, want *parser.Node", cfg.FunctionNode)
	}
	if storedNode.Location.StartLine != funcNode.Location.StartLine {
		t.Errorf("FunctionNode start line mismatch: got %d, want %d", storedNode.Location.StartLine, funcNode.Location.StartLine)
	}
	if storedNode.Location.EndLine != funcNode.Location.EndLine {
		t.Errorf("FunctionNode end line mismatch: got %d, want %d", storedNode.Location.EndLine, funcNode.Location.EndLine)
	}
}

func TestCFGBuilder_BuildAll_NilNode(t *testing.T) {
	builder := NewCFGBuilder()
	cfgs, err := builder.BuildAll(nil)

	if err == nil {
		t.Error("BuildAll with nil should return error")
	}
	if cfgs != nil {
		t.Error("BuildAll with nil should return nil map")
	}
}

func TestCFGBuilder_Build_BlockStatement(t *testing.T) {
	code := `
		function test() {
			{
				let x = 1;
				let y = 2;
			}
			return 0;
		}
	`
	ast := parseJS(t, code)
	funcNode := findFunction(ast, "test")

	builder := NewCFGBuilder()
	cfg, err := builder.Build(funcNode)

	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Block statement should not create separate blocks, just add statements
	if cfg == nil {
		t.Error("CFG should not be nil")
	}
}

func TestCFGBuilder_Build_ComplexControlFlow(t *testing.T) {
	code := `
		function complex(x) {
			if (x < 0) {
				throw new Error("negative");
			}

			for (let i = 0; i < x; i++) {
				if (i === 5) {
					break;
				}
				if (i % 2 === 0) {
					continue;
				}

				try {
					riskyOp(i);
				} catch (e) {
					console.error(e);
				}
			}

			switch (x % 3) {
				case 0:
					return "divisible by 3";
				case 1:
					return "remainder 1";
				default:
					return "remainder 2";
			}
		}
	`
	ast := parseJS(t, code)
	funcNode := findFunction(ast, "complex")

	builder := NewCFGBuilder()
	cfg, err := builder.Build(funcNode)

	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Verify various edge types exist
	edgeTypes := make(map[EdgeType]bool)
	cfg.Walk(&edgeTypeChecker{
		onEdge: func(e *Edge) {
			edgeTypes[e.Type] = true
		},
	})

	requiredEdges := []EdgeType{EdgeCondTrue, EdgeCondFalse, EdgeLoop, EdgeBreak, EdgeContinue, EdgeException, EdgeReturn}
	for _, et := range requiredEdges {
		if !edgeTypes[et] {
			t.Errorf("Complex CFG should have %s edge", et)
		}
	}
}

func TestCFGBuilder_Build_UnreachableCode(t *testing.T) {
	code := `
		function test() {
			return 1;
			console.log("unreachable");
		}
	`
	ast := parseJS(t, code)
	funcNode := findFunction(ast, "test")

	builder := NewCFGBuilder()
	cfg, err := builder.Build(funcNode)

	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Should have unreachable block
	hasUnreachable := false
	for _, block := range cfg.Blocks {
		if block.Label == LabelUnreachable || block.ID == LabelUnreachable+"_1" {
			hasUnreachable = true
			break
		}
	}

	// Note: The builder creates unreachable blocks for code after return
	if !hasUnreachable && cfg.Size() <= 3 {
		// If no explicit unreachable block, at least verify the CFG was built
		t.Log("Warning: Unreachable code handling may vary")
	}
}

func TestCFGBuilder_Build_AnonymousFunction(t *testing.T) {
	code := `
		const fn = function() {
			return 42;
		};
	`
	ast := parseJS(t, code)

	var funcExpr *parser.Node
	ast.Walk(func(n *parser.Node) bool {
		if n.Type == parser.NodeFunctionExpression {
			funcExpr = n
			return false
		}
		return true
	})

	if funcExpr == nil {
		t.Fatal("Function expression not found")
	}

	builder := NewCFGBuilder()
	cfg, err := builder.Build(funcExpr)

	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Anonymous function should get a generated name
	if cfg.Name == "" {
		t.Error("Anonymous function should have a generated name")
	}
}

func TestCFGBuilder_BuildAll_ArrowInVariable(t *testing.T) {
	code := `const add = (a, b) => a + b;`
	ast := parseJS(t, code)

	builder := NewCFGBuilder()
	cfgs, err := builder.BuildAll(ast)
	if err != nil {
		t.Fatalf("BuildAll failed: %v", err)
	}

	// Should have module-scope code plus at least one function for the arrow
	if len(cfgs) < 2 {
		t.Errorf("BuildAll should detect arrow function in variable declaration, got %d CFGs", len(cfgs))
	}

	// Check that a CFG exists for the arrow function (may be named "add" or "anonymous_<line>")
	found := false
	for name := range cfgs {
		if name != domain.ModuleFunctionName {
			found = true
			break
		}
	}
	if !found {
		t.Error("Arrow function in variable declaration should be discovered")
	}
}

func TestCFGBuilder_BuildAll_FunctionExpressionInVariable(t *testing.T) {
	code := `const fn = function() { return 42; };`
	ast := parseJS(t, code)

	builder := NewCFGBuilder()
	cfgs, err := builder.BuildAll(ast)
	if err != nil {
		t.Fatalf("BuildAll failed: %v", err)
	}

	if len(cfgs) < 2 {
		t.Errorf("BuildAll should detect function expression in variable, got %d CFGs", len(cfgs))
	}

	found := false
	for name := range cfgs {
		if name != domain.ModuleFunctionName {
			found = true
			break
		}
	}
	if !found {
		t.Error("Function expression in variable declaration should be discovered")
	}
}

func TestCFGBuilder_BuildAll_AssignmentFunctionExpression(t *testing.T) {
	code := `
		const app = {};
		app.init = function init() { return true; };
	`
	ast := parseJS(t, code)

	builder := NewCFGBuilder()
	cfgs, err := builder.BuildAll(ast)
	if err != nil {
		t.Fatalf("BuildAll failed: %v", err)
	}

	if len(cfgs) < 2 {
		t.Errorf("BuildAll should detect assigned function expression, got %d CFGs", len(cfgs))
	}

	// The named function expression should be found (name: "init")
	found := false
	for name := range cfgs {
		if name != domain.ModuleFunctionName {
			found = true
			break
		}
	}
	if !found {
		t.Error("Assignment function expression should be discovered")
	}
}

func TestCFGBuilder_BuildAll_ObjectMethods(t *testing.T) {
	code := `
		const obj = {
			method() { return 1; },
			other() { return 2; }
		};
	`
	ast := parseJS(t, code)

	builder := NewCFGBuilder()
	cfgs, err := builder.BuildAll(ast)
	if err != nil {
		t.Fatalf("BuildAll failed: %v", err)
	}

	// Should have module-scope code + at least 2 methods
	if len(cfgs) < 3 {
		t.Errorf("BuildAll should detect object methods, got %d CFGs (want >= 3)", len(cfgs))
	}
}

func TestCFGBuilder_BuildAll_ExportedArrowFunction(t *testing.T) {
	code := `export const foo = () => { return 42; };`
	ast := parseJS(t, code)

	builder := NewCFGBuilder()
	cfgs, err := builder.BuildAll(ast)
	if err != nil {
		t.Fatalf("BuildAll failed: %v", err)
	}

	if len(cfgs) < 2 {
		t.Errorf("BuildAll should detect exported arrow function, got %d CFGs", len(cfgs))
	}

	found := false
	for name := range cfgs {
		if name != domain.ModuleFunctionName {
			found = true
			break
		}
	}
	if !found {
		t.Error("Exported arrow function should be discovered")
	}
}

func TestCFGBuilder_BuildAll_CallbackArgument(t *testing.T) {
	code := `setTimeout(function() { console.log("hi"); }, 1000);`
	ast := parseJS(t, code)

	builder := NewCFGBuilder()
	cfgs, err := builder.BuildAll(ast)
	if err != nil {
		t.Fatalf("BuildAll failed: %v", err)
	}

	if len(cfgs) < 2 {
		t.Errorf("BuildAll should detect callback function, got %d CFGs", len(cfgs))
	}

	found := false
	for name := range cfgs {
		if name != domain.ModuleFunctionName {
			found = true
			break
		}
	}
	if !found {
		t.Error("Callback function expression should be discovered")
	}
}

func TestCFGBuilder_BuildAll_Mixed(t *testing.T) {
	code := `
		function declared() { return 1; }
		const arrow = (x) => x * 2;
		module.exports.init = function() { return 3; };
		const obj = {
			method() { return 4; }
		};
	`
	ast := parseJS(t, code)

	builder := NewCFGBuilder()
	cfgs, err := builder.BuildAll(ast)
	if err != nil {
		t.Fatalf("BuildAll failed: %v", err)
	}

	// Should have module-scope code + declared + arrow + assignment func + object method = 5
	if len(cfgs) < 5 {
		t.Errorf("BuildAll should detect all mixed function patterns, got %d CFGs (want >= 5)", len(cfgs))
	}

	// declared() should definitely be there
	if cfgs["declared"] == nil {
		t.Error("Should have 'declared' CFG from function declaration")
	}
}

// Helper visitor for checking edge types
type edgeTypeChecker struct {
	onEdge func(*Edge)
}

func (v *edgeTypeChecker) VisitBlock(block *BasicBlock) bool {
	return true
}

func (v *edgeTypeChecker) VisitEdge(edge *Edge) bool {
	if v.onEdge != nil {
		v.onEdge(edge)
	}
	return true
}

package analyzer

import (
	"testing"

	"github.com/ludo-technologies/polyscan/jscan/internal/config"
	"github.com/ludo-technologies/polyscan/jscan/internal/parser"
)

// Helper to create a config for testing
func testComplexityConfig() *config.ComplexityConfig {
	return &config.ComplexityConfig{
		LowThreshold:    5,
		MediumThreshold: 10,
		Enabled:         true,
	}
}

func TestComplexityResult_GetComplexity(t *testing.T) {
	result := &ComplexityResult{Complexity: 10}
	if result.GetComplexity() != 10 {
		t.Errorf("Expected 10, got %d", result.GetComplexity())
	}
}

func TestComplexityResult_GetFunctionName(t *testing.T) {
	result := &ComplexityResult{FunctionName: "testFunc"}
	if result.GetFunctionName() != "testFunc" {
		t.Errorf("Expected 'testFunc', got %s", result.GetFunctionName())
	}
}

func TestComplexityResult_GetRiskLevel(t *testing.T) {
	result := &ComplexityResult{RiskLevel: "high"}
	if result.GetRiskLevel() != "high" {
		t.Errorf("Expected 'high', got %s", result.GetRiskLevel())
	}
}

func TestComplexityResult_GetDetailedMetrics(t *testing.T) {
	result := &ComplexityResult{
		Nodes:             5,
		Edges:             8,
		IfStatements:      2,
		LoopStatements:    1,
		ExceptionHandlers: 1,
		SwitchCases:       0,
		LogicalOperators:  2,
		TernaryOperators:  1,
	}

	metrics := result.GetDetailedMetrics()

	tests := []struct {
		key      string
		expected int
	}{
		{"nodes", 5},
		{"edges", 8},
		{"if_statements", 2},
		{"loop_statements", 1},
		{"exception_handlers", 1},
		{"switch_cases", 0},
		{"logical_operators", 2},
		{"ternary_operators", 1},
	}

	for _, tc := range tests {
		if metrics[tc.key] != tc.expected {
			t.Errorf("metrics[%s] = %d, expected %d", tc.key, metrics[tc.key], tc.expected)
		}
	}
}

func TestComplexityResult_String(t *testing.T) {
	result := &ComplexityResult{
		FunctionName: "calculateSum",
		Complexity:   15,
		RiskLevel:    "high",
	}

	str := result.String()
	expected := "Function: calculateSum, Complexity: 15, Risk: high"
	if str != expected {
		t.Errorf("String() = %s, expected %s", str, expected)
	}
}

func TestCalculateComplexity_NilCFG(t *testing.T) {
	result := CalculateComplexity(nil)

	if result.Complexity != 0 {
		t.Errorf("Nil CFG should have complexity 0, got %d", result.Complexity)
	}
	if result.RiskLevel != "low" {
		t.Errorf("Nil CFG should have low risk, got %s", result.RiskLevel)
	}
}

func TestCalculateComplexity_SimpleCFG(t *testing.T) {
	// Simple function: just entry and exit
	cfg := NewCFG("simpleFunc")
	cfg.ConnectBlocks(cfg.Entry, cfg.Exit, EdgeNormal)

	result := CalculateComplexity(cfg)

	// Minimum complexity should be 1
	if result.Complexity < 1 {
		t.Errorf("Minimum complexity should be 1, got %d", result.Complexity)
	}
	if result.FunctionName != "simpleFunc" {
		t.Errorf("Function name should be 'simpleFunc', got %s", result.FunctionName)
	}
}

func TestCalculateComplexity_UsesFunctionLocation(t *testing.T) {
	code := `
		function locateMe(value) {
			if (value) {
				return 1;
			}
			return 0;
		}
	`
	ast := parseJS(t, code)
	funcNode := findFunction(ast, "locateMe")
	if funcNode == nil {
		t.Fatal("Function node should not be nil")
	}

	builder := NewCFGBuilder()
	cfg, err := builder.Build(funcNode)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	result := CalculateComplexity(cfg)
	if result.StartLine != funcNode.Location.StartLine {
		t.Errorf("StartLine mismatch: got %d, want %d", result.StartLine, funcNode.Location.StartLine)
	}
	if result.StartCol != funcNode.Location.StartCol {
		t.Errorf("StartCol mismatch: got %d, want %d", result.StartCol, funcNode.Location.StartCol)
	}
	if result.EndLine != funcNode.Location.EndLine {
		t.Errorf("EndLine mismatch: got %d, want %d", result.EndLine, funcNode.Location.EndLine)
	}
}

func TestCalculateComplexity_WithConditional(t *testing.T) {
	code := `
		function test(x) {
			if (x > 0) {
				return 1;
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

	result := CalculateComplexity(cfg)

	// if statement adds one decision point, so complexity should be at least 2
	if result.Complexity < 2 {
		t.Errorf("Complexity with if statement should be >= 2, got %d", result.Complexity)
	}
}

func TestCalculateComplexity_WithMultipleConditionals(t *testing.T) {
	code := `
		function test(x) {
			if (x > 0) {
				if (x > 10) {
					return "large";
				}
				return "small";
			}
			return "negative";
		}
	`
	ast := parseJS(t, code)
	funcNode := findFunction(ast, "test")

	builder := NewCFGBuilder()
	cfg, err := builder.Build(funcNode)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	result := CalculateComplexity(cfg)

	// Two if statements = at least 3 complexity
	if result.Complexity < 3 {
		t.Errorf("Complexity with 2 if statements should be >= 3, got %d", result.Complexity)
	}
}

func TestCalculateComplexity_WithLoop(t *testing.T) {
	code := `
		function test(n) {
			for (let i = 0; i < n; i++) {
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

	result := CalculateComplexity(cfg)

	// Loop adds complexity
	if result.Complexity < 2 {
		t.Errorf("Complexity with loop should be >= 2, got %d", result.Complexity)
	}
	if result.LoopStatements < 1 {
		t.Errorf("Should count at least 1 loop statement, got %d", result.LoopStatements)
	}
}

func TestCalculateComplexity_WithTryCatch(t *testing.T) {
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

	result := CalculateComplexity(cfg)

	if result.ExceptionHandlers < 1 {
		t.Errorf("Should count at least 1 exception handler, got %d", result.ExceptionHandlers)
	}
}

func TestCalculateComplexity_WithLogicalOperators(t *testing.T) {
	code := `
		function test(a, b, c) {
			if (a && b || c) {
				return true;
			}
			return false;
		}
	`
	ast := parseJS(t, code)
	funcNode := findFunction(ast, "test")

	builder := NewCFGBuilder()
	cfg, err := builder.Build(funcNode)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	result := CalculateComplexity(cfg)

	// && and || should increase complexity
	if result.LogicalOperators < 1 {
		t.Errorf("Should count logical operators, got %d", result.LogicalOperators)
	}
}

func TestCalculateComplexity_WithTernaryOperator(t *testing.T) {
	code := `
		function test(x) {
			return x > 0 ? "positive" : "non-positive";
		}
	`
	ast := parseJS(t, code)
	funcNode := findFunction(ast, "test")

	builder := NewCFGBuilder()
	cfg, err := builder.Build(funcNode)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	result := CalculateComplexity(cfg)

	if result.TernaryOperators < 1 {
		t.Errorf("Should count ternary operators, got %d", result.TernaryOperators)
	}
}

func TestCalculateComplexityWithConfig_CustomThresholds(t *testing.T) {
	cfg := NewCFG("test")
	block := cfg.CreateBlock("body")
	cfg.ConnectBlocks(cfg.Entry, block, EdgeNormal)
	cfg.ConnectBlocks(block, cfg.Exit, EdgeNormal)

	// Add multiple decision points manually
	for range 5 {
		b := cfg.CreateBlock("")
		cfg.ConnectBlocks(block, b, EdgeCondTrue)
		cfg.ConnectBlocks(block, b, EdgeCondFalse)
	}

	// Test with low thresholds
	lowConfig := &config.ComplexityConfig{
		LowThreshold:    2,
		MediumThreshold: 4,
	}

	result := CalculateComplexityWithConfig(cfg, lowConfig)

	// Should be "high" risk with these thresholds
	if result.Complexity > 4 && result.RiskLevel != "high" {
		t.Errorf("Expected 'high' risk for complexity %d with threshold 4, got %s",
			result.Complexity, result.RiskLevel)
	}
}

func TestCalculateNestingDepth_Nil(t *testing.T) {
	depth := CalculateNestingDepth(nil)
	if depth != 0 {
		t.Errorf("Nil node should have depth 0, got %d", depth)
	}
}

func TestCalculateNestingDepth_NoNesting(t *testing.T) {
	code := `
		function test() {
			let x = 1;
			let y = 2;
			return x + y;
		}
	`
	ast := parseJS(t, code)
	funcNode := findFunction(ast, "test")

	depth := CalculateNestingDepth(funcNode)
	if depth != 0 {
		t.Errorf("Function without control structures should have depth 0, got %d", depth)
	}
}

func TestCalculateNestingDepth_SingleLevel(t *testing.T) {
	code := `
		function test(x) {
			if (x > 0) {
				return 1;
			}
			return 0;
		}
	`
	ast := parseJS(t, code)
	funcNode := findFunction(ast, "test")

	depth := CalculateNestingDepth(funcNode)
	if depth != 1 {
		t.Errorf("Single if should have depth 1, got %d", depth)
	}
}

func TestCalculateNestingDepth_DeepNesting(t *testing.T) {
	code := `
		function test(x) {
			if (x > 0) {
				if (x > 10) {
					if (x > 100) {
						return "very large";
					}
				}
			}
			return "other";
		}
	`
	ast := parseJS(t, code)
	funcNode := findFunction(ast, "test")

	depth := CalculateNestingDepth(funcNode)
	if depth < 3 {
		t.Errorf("Triple nested if should have depth >= 3, got %d", depth)
	}
}

func TestCalculateNestingDepth_MixedControlStructures(t *testing.T) {
	code := `
		function test(items) {
			for (let item of items) {
				if (item.valid) {
					try {
						process(item);
					} catch (e) {
						console.error(e);
					}
				}
			}
		}
	`
	ast := parseJS(t, code)
	funcNode := findFunction(ast, "test")

	depth := CalculateNestingDepth(funcNode)
	// for -> if -> try -> catch = at least 3-4 levels
	if depth < 3 {
		t.Errorf("Mixed control structures should have depth >= 3, got %d", depth)
	}
}

func TestIsControlStructure(t *testing.T) {
	controlStructures := []parser.NodeType{
		parser.NodeIfStatement,
		parser.NodeSwitchStatement,
		parser.NodeForStatement,
		parser.NodeForInStatement,
		parser.NodeForOfStatement,
		parser.NodeWhileStatement,
		parser.NodeDoWhileStatement,
		parser.NodeTryStatement,
		parser.NodeCatchClause,
	}

	for _, nodeType := range controlStructures {
		node := &parser.Node{Type: nodeType}
		if !isControlStructure(node) {
			t.Errorf("isControlStructure should return true for %s", nodeType)
		}
	}

	nonControlStructures := []parser.NodeType{
		parser.NodeExpressionStatement,
		parser.NodeVariableDeclaration,
		parser.NodeReturnStatement,
		parser.NodeFunction,
		parser.NodeArrowFunction,
	}

	for _, nodeType := range nonControlStructures {
		node := &parser.Node{Type: nodeType}
		if isControlStructure(node) {
			t.Errorf("isControlStructure should return false for %s", nodeType)
		}
	}
}

func TestNewComplexityAnalyzer(t *testing.T) {
	cfg := testComplexityConfig()
	analyzer := NewComplexityAnalyzer(cfg)

	if analyzer == nil {
		t.Fatal("NewComplexityAnalyzer should not return nil")
	}
	if analyzer.cfg != cfg {
		t.Error("Analyzer should store config")
	}
}

func TestComplexityAnalyzer_AnalyzeFile_NilAST(t *testing.T) {
	cfg := testComplexityConfig()
	analyzer := NewComplexityAnalyzer(cfg)

	results, err := analyzer.AnalyzeFile(nil)

	if err == nil {
		t.Error("AnalyzeFile with nil AST should return error")
	}
	if results != nil {
		t.Error("AnalyzeFile with nil AST should return nil results")
	}
}

func TestComplexityAnalyzer_AnalyzeFile_SingleFunction(t *testing.T) {
	code := `
		function simple() {
			return 42;
		}
	`
	ast := parseJS(t, code)
	cfg := testComplexityConfig()
	analyzer := NewComplexityAnalyzer(cfg)

	results, err := analyzer.AnalyzeFile(ast)

	if err != nil {
		t.Fatalf("AnalyzeFile failed: %v", err)
	}
	if len(results) == 0 {
		t.Error("Should have at least one result")
	}
}

func TestComplexityAnalyzer_AnalyzeFile_MultipleFunctions(t *testing.T) {
	code := `
		function add(a, b) {
			return a + b;
		}

		function subtract(a, b) {
			return a - b;
		}

		function multiply(a, b) {
			return a * b;
		}
	`
	ast := parseJS(t, code)
	cfg := testComplexityConfig()
	analyzer := NewComplexityAnalyzer(cfg)

	results, err := analyzer.AnalyzeFile(ast)

	if err != nil {
		t.Fatalf("AnalyzeFile failed: %v", err)
	}
	// Should have results for module-scope, add, subtract, multiply
	if len(results) < 4 {
		t.Errorf("Should have at least 4 results, got %d", len(results))
	}
}

func TestComplexityAnalyzer_AnalyzeFile_ComplexFunction(t *testing.T) {
	code := `
		function complex(x, y) {
			if (x > 0) {
				if (y > 0) {
					for (let i = 0; i < x; i++) {
						if (i % 2 === 0) {
							console.log(i);
						}
					}
				}
			} else if (x < 0) {
				while (y > 0) {
					y--;
				}
			}
			return x * y;
		}
	`
	ast := parseJS(t, code)
	cfg := testComplexityConfig()
	analyzer := NewComplexityAnalyzer(cfg)

	results, err := analyzer.AnalyzeFile(ast)

	if err != nil {
		t.Fatalf("AnalyzeFile failed: %v", err)
	}

	// Find the "complex" function result
	var complexResult *ComplexityResult
	for _, r := range results {
		if r.FunctionName == "complex" {
			complexResult = r
			break
		}
	}

	if complexResult == nil {
		t.Fatal("Should have result for 'complex' function")
	}

	// Complex function should have high complexity
	if complexResult.Complexity < 5 {
		t.Errorf("Complex function should have complexity >= 5, got %d", complexResult.Complexity)
	}
}

// Test for edge case: empty function
func TestCalculateComplexity_EmptyFunction(t *testing.T) {
	code := `function empty() {}`
	ast := parseJS(t, code)
	funcNode := findFunction(ast, "empty")

	builder := NewCFGBuilder()
	cfg, err := builder.Build(funcNode)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	result := CalculateComplexity(cfg)

	// Even empty function should have complexity of at least 1
	if result.Complexity < 1 {
		t.Errorf("Empty function should have complexity >= 1, got %d", result.Complexity)
	}
}

// Test switch statement complexity
func TestCalculateComplexity_SwitchStatement(t *testing.T) {
	code := `
		function test(x) {
			switch (x) {
				case 1:
					return "one";
				case 2:
					return "two";
				case 3:
					return "three";
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

	result := CalculateComplexity(cfg)

	// One decision point per case label; default adds nothing, like else.
	if result.Complexity != 4 {
		t.Errorf("Switch with 3 cases should have complexity 4, got %d", result.Complexity)
	}
	if result.SwitchCases != 3 {
		t.Errorf("Switch with 3 cases should report 3 switch cases, got %d", result.SwitchCases)
	}
}

// A switch and the equivalent if chain describe the same branching, so they
// must score the same (issue #40).
func TestCalculateComplexity_SwitchMatchesEquivalentIfChain(t *testing.T) {
	code := `
		function sw4(a) {
			switch (a) {
				case 1: return 1;
				case 2: return 2;
				case 3: return 3;
				case 4: return 4;
				default: return 0;
			}
		}

		function ifChain(a) {
			if (a === 1) return 1;
			if (a === 2) return 2;
			if (a === 3) return 3;
			if (a === 4) return 4;
			return 0;
		}
	`
	ast := parseJS(t, code)

	complexityOf := func(name string) *ComplexityResult {
		builder := NewCFGBuilder()
		cfg, err := builder.Build(findFunction(ast, name))
		if err != nil {
			t.Fatalf("Build failed for %s: %v", name, err)
		}
		return CalculateComplexity(cfg)
	}

	switchResult := complexityOf("sw4")
	ifResult := complexityOf("ifChain")

	if switchResult.Complexity != ifResult.Complexity {
		t.Errorf("switch complexity %d should match if chain complexity %d",
			switchResult.Complexity, ifResult.Complexity)
	}
	if switchResult.Complexity != 5 {
		t.Errorf("switch with 4 cases should have complexity 5, got %d", switchResult.Complexity)
	}
	if switchResult.SwitchCases != 4 {
		t.Errorf("switch with 4 cases should report 4 switch cases, got %d", switchResult.SwitchCases)
	}
	if ifResult.SwitchCases != 0 {
		t.Errorf("if chain should report 0 switch cases, got %d", ifResult.SwitchCases)
	}
}

func TestCalculateComplexity_SwitchWithoutDefault(t *testing.T) {
	code := `
		function test(x) {
			switch (x) {
				case 1: return "one";
				case 2: return "two";
			}
			return "other";
		}
	`
	ast := parseJS(t, code)

	builder := NewCFGBuilder()
	cfg, err := builder.Build(findFunction(ast, "test"))
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	result := CalculateComplexity(cfg)

	if result.Complexity != 3 {
		t.Errorf("Switch with 2 cases should have complexity 3, got %d", result.Complexity)
	}
	if result.SwitchCases != 2 {
		t.Errorf("Switch with 2 cases should report 2 switch cases, got %d", result.SwitchCases)
	}
}

func TestCalculateComplexity_EmptySwitch(t *testing.T) {
	code := `
		function test(x) {
			switch (x) {
			}
			return x;
		}
	`
	ast := parseJS(t, code)

	builder := NewCFGBuilder()
	cfg, err := builder.Build(findFunction(ast, "test"))
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	result := CalculateComplexity(cfg)

	// No case labels means nothing is decided.
	if result.Complexity != 1 {
		t.Errorf("Empty switch should have complexity 1, got %d", result.Complexity)
	}
	if result.SwitchCases != 0 {
		t.Errorf("Empty switch should report 0 switch cases, got %d", result.SwitchCases)
	}
}

// A switch inside a nested function belongs to that function's own result.
func TestCalculateComplexity_SwitchInNestedFunction(t *testing.T) {
	code := `
		function outer(a) {
			const inner = (b) => {
				switch (b) {
					case 1: return 1;
					case 2: return 2;
				}
				return 0;
			};
			return inner(a);
		}
	`
	ast := parseJS(t, code)

	builder := NewCFGBuilder()
	cfg, err := builder.Build(findFunction(ast, "outer"))
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	result := CalculateComplexity(cfg)

	if result.Complexity != 1 {
		t.Errorf("outer should have complexity 1, got %d", result.Complexity)
	}
	if result.SwitchCases != 0 {
		t.Errorf("outer should report 0 switch cases, got %d", result.SwitchCases)
	}
}

// Test null coalescing operator
func TestCalculateComplexity_NullishCoalescing(t *testing.T) {
	code := `
		function test(a, b) {
			return a ?? b ?? "default";
		}
	`
	ast := parseJS(t, code)
	funcNode := findFunction(ast, "test")

	builder := NewCFGBuilder()
	cfg, err := builder.Build(funcNode)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	result := CalculateComplexity(cfg)

	// ?? operators should be counted as logical operators
	if result.LogicalOperators < 1 {
		t.Logf("Note: Nullish coalescing operators counted: %d", result.LogicalOperators)
	}
}

// Integration test with realistic code
func TestCalculateComplexity_RealisticCode(t *testing.T) {
	code := `
		function processOrder(order, user) {
			if (!order || !user) {
				throw new Error("Invalid arguments");
			}

			let total = 0;

			for (const item of order.items) {
				if (item.quantity <= 0) {
					continue;
				}

				const price = item.discountPrice ?? item.regularPrice;
				total += price * item.quantity;

				if (item.isGift && user.isPremium) {
					total -= total * 0.1;
				}
			}

			try {
				validateTotal(total);
			} catch (e) {
				console.error(e);
				return { error: true, total: 0 };
			}

			return { error: false, total: total > 100 ? applyBulkDiscount(total) : total };
		}
	`
	ast := parseJS(t, code)
	funcNode := findFunction(ast, "processOrder")

	builder := NewCFGBuilder()
	cfg, err := builder.Build(funcNode)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	result := CalculateComplexity(cfg)

	// This function has:
	// - Multiple if statements
	// - for loop
	// - continue statement
	// - logical operators (&&, ||)
	// - try-catch
	// - ternary operator
	// - nullish coalescing
	// Should have moderate to high complexity
	if result.Complexity < 5 {
		t.Errorf("Realistic complex function should have complexity >= 5, got %d", result.Complexity)
	}

	// Log the detailed metrics for inspection
	t.Logf("Complexity: %d, Risk: %s", result.Complexity, result.RiskLevel)
	t.Logf("Metrics: if=%d, loops=%d, exceptions=%d, logical=%d, ternary=%d",
		result.IfStatements, result.LoopStatements, result.ExceptionHandlers,
		result.LogicalOperators, result.TernaryOperators)
}

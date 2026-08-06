package analyzer

import (
	"fmt"

	corecfg "github.com/ludo-technologies/polyscan/core/cfg"
	"github.com/ludo-technologies/polyscan/jscan/internal/config"
	"github.com/ludo-technologies/polyscan/jscan/internal/parser"
)

// ComplexityResult holds cyclomatic complexity metrics for a function or method
type ComplexityResult struct {
	// McCabe cyclomatic complexity
	Complexity int

	// Raw CFG metrics
	Edges               int
	Nodes               int
	ConnectedComponents int

	// Function/method information
	FunctionName string
	StartLine    int
	StartCol     int
	EndLine      int

	// Nesting depth
	NestingDepth int

	// Decision points breakdown
	IfStatements      int
	LoopStatements    int
	ExceptionHandlers int
	SwitchCases       int
	LogicalOperators  int // JavaScript-specific: &&, ||, ??
	TernaryOperators  int // JavaScript-specific: ? :

	// Risk assessment based on complexity thresholds
	RiskLevel string // "low", "medium", "high"
}

// Interface methods for reporter compatibility

func (cr *ComplexityResult) GetComplexity() int {
	return cr.Complexity
}

func (cr *ComplexityResult) GetFunctionName() string {
	return cr.FunctionName
}

func (cr *ComplexityResult) GetRiskLevel() string {
	return cr.RiskLevel
}

func (cr *ComplexityResult) GetDetailedMetrics() map[string]int {
	return map[string]int{
		"nodes":              cr.Nodes,
		"edges":              cr.Edges,
		"if_statements":      cr.IfStatements,
		"loop_statements":    cr.LoopStatements,
		"exception_handlers": cr.ExceptionHandlers,
		"switch_cases":       cr.SwitchCases,
		"logical_operators":  cr.LogicalOperators,
		"ternary_operators":  cr.TernaryOperators,
	}
}

// String returns a human-readable representation of the complexity result
func (cr *ComplexityResult) String() string {
	return fmt.Sprintf("Function: %s, Complexity: %d, Risk: %s",
		cr.FunctionName, cr.Complexity, cr.RiskLevel)
}

// isFunctionNode returns true if the node represents a function boundary
func isFunctionNode(n *parser.Node) bool {
	switch n.Type {
	case parser.NodeFunction, parser.NodeFunctionExpression, parser.NodeArrowFunction,
		parser.NodeAsyncFunction, parser.NodeGeneratorFunction, parser.NodeMethodDefinition:
		return true
	}
	return false
}

// CalculateComplexity computes McCabe cyclomatic complexity for a CFG using default thresholds
func CalculateComplexity(cfg *CFG) *ComplexityResult {
	defaultConfig := config.DefaultConfig()
	return CalculateComplexityWithConfig(cfg, &defaultConfig.Complexity)
}

// CalculateComplexityWithConfig computes McCabe cyclomatic complexity using provided configuration
func CalculateComplexityWithConfig(cfg *CFG, complexityConfig *config.ComplexityConfig) *ComplexityResult {
	if cfg == nil {
		return &ComplexityResult{
			Complexity: 0,
			RiskLevel:  "low",
		}
	}

	coreResult, err := corecfg.ComputeComplexity(cfg, corecfg.ComplexityConfig{
		Contributor: javaScriptComplexityContributor{},
	})
	if err != nil {
		return &ComplexityResult{RiskLevel: "low"}
	}

	edges := 0
	for _, count := range coreResult.EdgeBreakdown {
		edges += count
	}
	logicalOperators := 0
	ternaryOperators := 0
	for _, contribution := range coreResult.Contributions {
		switch contribution.Description {
		case "logical_operator":
			logicalOperators += contribution.Count
		case "ternary":
			ternaryOperators += contribution.Count
		}
	}
	nodes := 0
	for _, block := range cfg.Blocks {
		if !block.IsEntry && !block.IsExit {
			nodes++
		}
	}

	// Determine risk level based on thresholds
	riskLevel := complexityConfig.AssessRiskLevel(coreResult.McCabe)

	result := &ComplexityResult{
		Complexity:        coreResult.McCabe,
		Edges:             edges,
		Nodes:             nodes,
		IfStatements:      coreResult.DecisionPoints,
		LoopStatements:    coreResult.EdgeBreakdown[corecfg.EdgeLoop],
		ExceptionHandlers: coreResult.EdgeBreakdown[corecfg.EdgeException],
		LogicalOperators:  logicalOperators,
		TernaryOperators:  ternaryOperators,
		RiskLevel:         riskLevel,
		FunctionName:      cfg.Name,
	}

	if functionNode, ok := jsNode(cfg.FunctionNode); ok {
		result.StartLine = functionNode.Location.StartLine
		result.StartCol = functionNode.Location.StartCol
		result.EndLine = functionNode.Location.EndLine
		result.SwitchCases = countSwitchCases(functionNode)
		result.NestingDepth = CalculateNestingDepth(functionNode)
	}

	return result
}

// countSwitchCases counts the case clauses of every switch statement owned by
// this function. Default clauses are excluded, matching the treatment of else,
// and nested functions are skipped because they get their own CFG and result.
func countSwitchCases(functionNode *parser.Node) int {
	switchCases := 0
	functionNode.Walk(func(current *parser.Node) bool {
		if current != functionNode && isFunctionNode(current) {
			return false
		}
		if current.Type == parser.NodeCaseClause {
			switchCases++
		}
		return true
	})
	return switchCases
}

// CalculateNestingDepth returns the deepest chain of nested control structures
// inside a function. The function body itself is depth 0, so a single loop or
// branch is depth 1.
//
// Nested functions are skipped because they get their own CFG and result, the
// same boundary countSwitchCases uses.
func CalculateNestingDepth(node *parser.Node) int {
	if node == nil {
		return 0
	}

	deepest := 0
	for _, child := range parser.OrderedChildren(node) {
		if depth := nestingDepthOf(child, 0); depth > deepest {
			deepest = depth
		}
	}
	return deepest
}

// nestingDepthOf returns the deepest level reached inside node, which sits at
// the given level before its own contribution is counted.
func nestingDepthOf(node *parser.Node, level int) int {
	if node == nil || isFunctionNode(node) {
		return level
	}

	if isControlStructure(node) {
		level++
	}

	deepest := level
	for _, child := range parser.OrderedChildren(node) {
		childLevel := level
		// `else if` continues the chain its outer if opened rather than
		// starting a deeper one, so the whole chain reads as one level. A
		// plain `else { ... }` block is left alone: code nested in it really
		// is one level deeper.
		if node.Type == parser.NodeIfStatement && child == node.Alternate && isElseIf(child) {
			childLevel = level - 1
		}
		if depth := nestingDepthOf(child, childLevel); depth > deepest {
			deepest = depth
		}
	}
	return deepest
}

// isElseIf reports whether an if statement's alternate continues an else-if
// chain. The parser keeps the else clause as a wrapper node, so the chain is
// recognized by the clause holding an if statement instead of a block.
func isElseIf(alternate *parser.Node) bool {
	if alternate == nil {
		return false
	}
	if alternate.Type == parser.NodeIfStatement {
		return true
	}
	for _, child := range parser.OrderedChildren(alternate) {
		switch child.Type {
		case parser.NodeIfStatement:
			return true
		case parser.NodeBlockStatement:
			return false
		}
	}
	return false
}

// isControlStructure checks if a node opens a nesting level. A catch clause
// does not: it is part of the try statement that already opened one.
func isControlStructure(node *parser.Node) bool {
	switch node.Type {
	case parser.NodeIfStatement, parser.NodeSwitchStatement,
		parser.NodeForStatement, parser.NodeForInStatement, parser.NodeForOfStatement,
		parser.NodeWhileStatement, parser.NodeDoWhileStatement,
		parser.NodeTryStatement:
		return true
	}
	return false
}

// ComplexityAnalyzer analyzes complexity for multiple functions
type ComplexityAnalyzer struct {
	cfg *config.ComplexityConfig
}

// NewComplexityAnalyzer creates a new complexity analyzer
func NewComplexityAnalyzer(cfg *config.ComplexityConfig) *ComplexityAnalyzer {
	return &ComplexityAnalyzer{
		cfg: cfg,
	}
}

// AnalyzeFile analyzes complexity for all functions in a file
func (ca *ComplexityAnalyzer) AnalyzeFile(ast *parser.Node) ([]*ComplexityResult, error) {
	if ast == nil {
		return nil, fmt.Errorf("AST is nil")
	}

	// Build CFGs for all functions
	builder := NewCFGBuilder()
	cfgs, err := builder.BuildAll(ast)
	if err != nil {
		return nil, fmt.Errorf("failed to build CFGs: %w", err)
	}

	// Calculate complexity for each function
	var results []*ComplexityResult
	for _, cfg := range cfgs {
		result := CalculateComplexityWithConfig(cfg, ca.cfg)
		results = append(results, result)
	}

	return results, nil
}

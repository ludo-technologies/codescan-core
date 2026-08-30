package analyzer

import (
	corecfg "github.com/ludo-technologies/polyscan/core/cfg"
	"github.com/ludo-technologies/polyscan/polyscan/internal/js/parser"
)

type javaScriptCFGClassifier struct{}

var (
	_ corecfg.StatementClassifier = javaScriptCFGClassifier{}
	_ corecfg.NoOpClassifier      = javaScriptCFGClassifier{}
)

func jsNode(value any) (*parser.Node, bool) {
	node, ok := value.(*parser.Node)
	return node, ok && node != nil
}

func (javaScriptCFGClassifier) IsReturn(stmt any) bool {
	node, ok := jsNode(stmt)
	return ok && node.Type == parser.NodeReturnStatement
}

func (javaScriptCFGClassifier) IsBreak(stmt any) bool {
	node, ok := jsNode(stmt)
	return ok && node.Type == parser.NodeBreakStatement
}

func (javaScriptCFGClassifier) IsContinue(stmt any) bool {
	node, ok := jsNode(stmt)
	return ok && node.Type == parser.NodeContinueStatement
}

func (javaScriptCFGClassifier) IsThrow(stmt any) bool {
	node, ok := jsNode(stmt)
	return ok && node.Type == parser.NodeThrowStatement
}

func (javaScriptCFGClassifier) IsNoOp(stmt any) bool {
	node, ok := jsNode(stmt)
	return ok && node.Type == parser.NodeEmptyStatement
}

type javaScriptComplexityContributor struct{}

var _ corecfg.ComplexityContributor = javaScriptComplexityContributor{}

func (javaScriptComplexityContributor) ContributeComplexity(block *corecfg.BasicBlock) ([]corecfg.ComplexityContribution, error) {
	logicalOperators := 0
	ternaryOperators := 0

	for _, statement := range block.Statements {
		node, ok := jsNode(statement)
		if !ok || isFunctionNode(node) {
			continue
		}

		node.Walk(func(current *parser.Node) bool {
			if current != node && isFunctionNode(current) {
				return false
			}
			switch current.Type {
			case parser.NodeLogicalExpression:
				logicalOperators++
			case parser.NodeConditionalExpression:
				ternaryOperators++
			}
			return true
		})
	}

	contributions := make([]corecfg.ComplexityContribution, 0, 2)
	if logicalOperators > 0 {
		contributions = append(contributions, corecfg.ComplexityContribution{
			Count:       logicalOperators,
			Description: "logical_operator",
		})
	}
	if ternaryOperators > 0 {
		contributions = append(contributions, corecfg.ComplexityContribution{
			Count:       ternaryOperators,
			Description: "ternary",
		})
	}
	return contributions, nil
}

func addJSStatements(block *corecfg.BasicBlock, nodes []*parser.Node) {
	for _, node := range nodes {
		block.AddStatement(node)
	}
}

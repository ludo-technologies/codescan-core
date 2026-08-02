package analyzer

import (
	"fmt"

	"github.com/ludo-technologies/polyscan/core/apted"
	"github.com/ludo-technologies/polyscan/jscan/internal/parser"
)

// TreeConverter converts parser AST nodes to APTED tree nodes
type TreeConverter struct {
	nextID int
}

// NewTreeConverter creates a new tree converter
func NewTreeConverter() *TreeConverter {
	return &TreeConverter{nextID: 0}
}

// ConvertAST converts a parser AST node to an APTED tree
func (tc *TreeConverter) ConvertAST(astNode *parser.Node) *apted.TreeNode {
	if astNode == nil {
		return nil
	}

	// Create tree node with simplified label
	label := tc.getNodeLabel(astNode)
	treeNode := apted.NewTreeNode(tc.nextID, label)
	tc.nextID++

	// TreeNode.OriginalNode is deliberately left empty. Nothing in jscan reads
	// back the parser node, and because parser nodes carry a parent pointer,
	// storing one on every converted node would pin whole file ASTs in memory
	// for as long as clone detection holds the trees.

	for _, child := range parser.OrderedChildren(astNode) {
		if childNode := tc.ConvertAST(child); childNode != nil {
			treeNode.AddChild(childNode)
		}
	}

	return treeNode
}

// getNodeLabel extracts a meaningful label from the AST node
func (tc *TreeConverter) getNodeLabel(astNode *parser.Node) string {
	// Use the node type as the primary label
	label := string(astNode.Type)

	// For some node types, include additional information
	switch astNode.Type {
	case parser.NodeIdentifier:
		if astNode.Name != "" {
			label = fmt.Sprintf("Identifier(%s)", astNode.Name)
		}
	case parser.NodeLiteral, parser.NodeStringLiteral, parser.NodeNumberLiteral:
		if astNode.Value != nil {
			label = fmt.Sprintf("Literal(%v)", astNode.Value)
		}
	case parser.NodeFunction, parser.NodeAsyncFunction, parser.NodeArrowFunction:
		if astNode.Name != "" {
			label = fmt.Sprintf("Function(%s)", astNode.Name)
		}
	case parser.NodeClass, parser.NodeClassExpression:
		if astNode.Name != "" {
			label = fmt.Sprintf("Class(%s)", astNode.Name)
		}
	case parser.NodeBinaryExpression, parser.NodeUnaryExpression, parser.NodeLogicalExpression,
		parser.NodeAssignmentExpression, parser.NodeUpdateExpression:
		if astNode.Operator != "" {
			label = fmt.Sprintf("%s(%s)", astNode.Type, astNode.Operator)
		}
	case parser.NodeVariableDeclaration:
		if astNode.Kind != "" {
			label = fmt.Sprintf("VariableDeclaration(%s)", astNode.Kind)
		}
	}

	return label
}

package analyzer

import (
	"strings"

	"github.com/ludo-technologies/polyscan/core/apted"
)

// JavaScriptCostModel implements a JavaScript-aware cost model with different costs for different node types
type JavaScriptCostModel struct {
	// Base costs for different operations
	BaseInsertCost float64
	BaseDeleteCost float64
	BaseRenameCost float64

	// Whether to ignore differences in literal values
	IgnoreLiterals bool

	// Whether to ignore differences in identifier names
	IgnoreIdentifiers bool
}

var _ apted.CostModel = (*JavaScriptCostModel)(nil)

// NewJavaScriptCostModel creates a new JavaScript-aware cost model with default settings
func NewJavaScriptCostModel() *JavaScriptCostModel {
	return &JavaScriptCostModel{
		BaseInsertCost:    1.0,
		BaseDeleteCost:    1.0,
		BaseRenameCost:    1.0,
		IgnoreLiterals:    false,
		IgnoreIdentifiers: false,
	}
}

// NewJavaScriptCostModelWithConfig creates a JavaScript cost model with custom configuration
func NewJavaScriptCostModelWithConfig(ignoreLiterals, ignoreIdentifiers bool) *JavaScriptCostModel {
	return &JavaScriptCostModel{
		BaseInsertCost:    1.0,
		BaseDeleteCost:    1.0,
		BaseRenameCost:    1.0,
		IgnoreLiterals:    ignoreLiterals,
		IgnoreIdentifiers: ignoreIdentifiers,
	}
}

// nodeCategory classifies a node label into the cost tiers this model prices.
// Labels are looked up rather than prefix-scanned because APTED evaluates the
// cost model once per node per comparison, across millions of comparisons.
type nodeCategory int

const (
	categoryOther nodeCategory = iota
	categoryStructural
	categoryControlFlow
	categoryExpression
	categoryLiteral
	categoryIdentifier
)

// Cost multipliers per category. Structural and control-flow nodes carry the
// shape of the program, so editing them is priced above the default; expression
// nodes below it. Literals and identifiers are only discounted when the caller
// asked for them to be ignored.
const (
	structuralMultiplier        = 1.5
	controlFlowMultiplier       = 1.3
	expressionMultiplier        = 0.8
	ignoredLiteralMultiplier    = 0.1
	ignoredIdentifierMultiplier = 0.2
	defaultMultiplier           = 1.0
)

// Label similarity tiers, from identical base type down to no relation.
const (
	sameBaseTypeSimilarity = 0.8
	relatedTypeSimilarity  = 0.5
	sameCategorySimilarity = 0.3
	unrelatedSimilarity    = 0.0
)

// nodeCategories maps a node label's base type to its category. Categories are
// disjoint and looked up in the order the cost tiers are defined.
var nodeCategories = buildNodeCategories()

func buildNodeCategories() map[string]nodeCategory {
	categories := map[string]nodeCategory{}
	add := func(category nodeCategory, labels ...string) {
		for _, label := range labels {
			categories[label] = category
		}
	}

	add(categoryStructural,
		"FunctionDeclaration", "FunctionExpression", "ArrowFunctionExpression",
		"AsyncFunctionDeclaration", "GeneratorFunctionDeclaration",
		"ClassDeclaration", "ClassExpression", "MethodDefinition",
		"Program", "Module")
	add(categoryControlFlow,
		"IfStatement", "SwitchStatement", "SwitchCase",
		"ForStatement", "ForInStatement", "ForOfStatement",
		"WhileStatement", "DoWhileStatement",
		"TryStatement", "CatchClause", "FinallyClause",
		"BreakStatement", "ContinueStatement", "ReturnStatement", "ThrowStatement")
	add(categoryExpression,
		"BinaryExpression", "UnaryExpression", "LogicalExpression",
		"ConditionalExpression", "CallExpression", "MemberExpression",
		"AssignmentExpression", "UpdateExpression", "NewExpression",
		"ArrayExpression", "ObjectExpression", "SequenceExpression",
		"AwaitExpression", "YieldExpression", "SpreadElement",
		"TemplateLiteral")
	add(categoryLiteral,
		"StringLiteral", "NumberLiteral", "BooleanLiteral",
		"NullLiteral", "RegExpLiteral")

	return categories
}

// parenthesizedCategories maps the base types that are only meaningful when the
// label carries a parenthesized detail, as produced by TreeConverter.getNodeLabel
// (for example "Function(name)" or "Identifier(name)").
var parenthesizedCategories = map[string]nodeCategory{
	"Function":   categoryStructural,
	"Class":      categoryStructural,
	"Literal":    categoryLiteral,
	"Identifier": categoryIdentifier,
}

// splitLabel separates a label's base node type from its parenthesized detail.
func splitLabel(label string) (baseType string, hasDetail bool) {
	if idx := strings.Index(label, "("); idx >= 0 {
		return label[:idx], true
	}
	return label, false
}

// categoryOf classifies a full node label.
func categoryOf(label string) nodeCategory {
	baseType, hasDetail := splitLabel(label)
	if category, ok := nodeCategories[baseType]; ok {
		return category
	}
	if hasDetail {
		if category, ok := parenthesizedCategories[baseType]; ok {
			return category
		}
	}
	return categoryOther
}

// Insert returns the cost of inserting a node
func (c *JavaScriptCostModel) Insert(node *apted.TreeNode) float64 {
	if node == nil {
		return c.BaseInsertCost
	}

	// Different costs based on node type
	multiplier := c.getNodeTypeMultiplier(node.Label)
	return c.BaseInsertCost * multiplier
}

// Delete returns the cost of deleting a node
func (c *JavaScriptCostModel) Delete(node *apted.TreeNode) float64 {
	if node == nil {
		return c.BaseDeleteCost
	}

	// Different costs based on node type
	multiplier := c.getNodeTypeMultiplier(node.Label)
	return c.BaseDeleteCost * multiplier
}

// Rename returns the cost of renaming node1 to node2
func (c *JavaScriptCostModel) Rename(node1, node2 *apted.TreeNode) float64 {
	if node1 == nil || node2 == nil {
		return c.BaseRenameCost
	}

	// If labels are identical, no cost
	if node1.Label == node2.Label {
		return 0.0
	}

	// Apply ignore patterns
	if c.shouldIgnoreDifference(node1.Label, node2.Label) {
		return 0.0
	}

	// Check if both nodes are of similar types
	similarity := c.calculateLabelSimilarity(node1.Label, node2.Label)

	// Scale rename cost based on similarity
	return c.BaseRenameCost * (1.0 - similarity)
}

// getNodeTypeMultiplier returns a cost multiplier based on the node type
func (c *JavaScriptCostModel) getNodeTypeMultiplier(label string) float64 {
	switch categoryOf(label) {
	case categoryStructural:
		return structuralMultiplier
	case categoryControlFlow:
		return controlFlowMultiplier
	case categoryExpression:
		return expressionMultiplier
	case categoryLiteral:
		if c.IgnoreLiterals {
			return ignoredLiteralMultiplier
		}
	case categoryIdentifier:
		if c.IgnoreIdentifiers {
			return ignoredIdentifierMultiplier
		}
	}

	return defaultMultiplier
}

// shouldIgnoreDifference determines if the difference between two labels should be ignored
func (c *JavaScriptCostModel) shouldIgnoreDifference(label1, label2 string) bool {
	category1, category2 := categoryOf(label1), categoryOf(label2)

	// Ignore literal differences if configured
	if c.IgnoreLiterals && category1 == categoryLiteral && category2 == categoryLiteral {
		return true
	}

	// Ignore identifier differences if configured
	if c.IgnoreIdentifiers && category1 == categoryIdentifier && category2 == categoryIdentifier {
		return true
	}

	return false
}

// calculateLabelSimilarity calculates similarity between two node labels
func (c *JavaScriptCostModel) calculateLabelSimilarity(label1, label2 string) float64 {
	// Extract base node types (remove parenthetical content)
	baseType1, _ := splitLabel(label1)
	baseType2, _ := splitLabel(label2)

	// If base types are identical, high similarity
	if baseType1 == baseType2 {
		return sameBaseTypeSimilarity
	}

	// Check for related node types
	if areRelatedNodeTypes(baseType1, baseType2) {
		return relatedTypeSimilarity
	}

	// Check for same category
	if areSameCategory(baseType1, baseType2) {
		return sameCategorySimilarity
	}

	return unrelatedSimilarity
}

// relatedNodeTypes pairs node types that express the same construct in
// different syntactic forms, so renaming between them is cheaper than renaming
// across unrelated types.
var relatedNodeTypes = buildRelatedNodeTypes()

func buildRelatedNodeTypes() map[[2]string]struct{} {
	pairs := [][2]string{
		{"FunctionDeclaration", "FunctionExpression"},
		{"FunctionDeclaration", "ArrowFunctionExpression"},
		{"FunctionExpression", "ArrowFunctionExpression"},
		{"FunctionDeclaration", "AsyncFunctionDeclaration"},
		{"ClassDeclaration", "ClassExpression"},
		{"ForStatement", "ForInStatement"},
		{"ForStatement", "ForOfStatement"},
		{"ForInStatement", "ForOfStatement"},
		{"WhileStatement", "DoWhileStatement"},
		{"BinaryExpression", "UnaryExpression"},
		{"BinaryExpression", "LogicalExpression"},
		{"ArrayExpression", "ObjectExpression"},
		{"IfStatement", "ConditionalExpression"},
	}

	related := make(map[[2]string]struct{}, len(pairs)*2)
	for _, pair := range pairs {
		related[pair] = struct{}{}
		related[[2]string{pair[1], pair[0]}] = struct{}{}
	}
	return related
}

// areRelatedNodeTypes checks if two node types are related
func areRelatedNodeTypes(type1, type2 string) bool {
	_, ok := relatedNodeTypes[[2]string{type1, type2}]
	return ok
}

// areSameCategory checks if two node types belong to the same category.
// Literals and identifiers are excluded: their categories only carry a discount
// when the caller opted into ignoring them, not a claim of structural kinship.
func areSameCategory(type1, type2 string) bool {
	category1, category2 := categoryOf(type1), categoryOf(type2)
	if category1 != category2 {
		return false
	}

	switch category1 {
	case categoryStructural, categoryControlFlow, categoryExpression:
		return true
	default:
		return false
	}
}

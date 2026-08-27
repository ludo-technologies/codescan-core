package clone

import (
	"strings"

	"github.com/ludo-technologies/polyscan/core/apted"
	"github.com/ludo-technologies/polyscan/polyscan/internal/engine"
)

// category is the cost tier of a node type.
type category int

const (
	categoryOther category = iota
	categoryStructural
	categoryControlFlow
	categoryExpression
)

// Cost multipliers per tier. Structural and control-flow nodes carry the
// shape of the program, so editing them is priced above the default and
// expression nodes below it.
const (
	structuralMultiplier  = 1.5
	controlFlowMultiplier = 1.3
	expressionMultiplier  = 0.8
	defaultMultiplier     = 1.0
)

// Label similarity tiers, from identical base type down to no relation. A
// rename costs 1 minus the similarity.
const (
	sameBaseTypeSimilarity = 0.8
	relatedTypeSimilarity  = 0.5
	sameCategorySimilarity = 0.3
)

// costModel prices tree edits from a language's CloneSpec, following the
// cost models of pyscn and jscan.
type costModel struct {
	categories map[string]category
	related    map[[2]string]struct{}
}

var _ apted.CostModel = (*costModel)(nil)

func newCostModel(spec engine.CloneSpec) *costModel {
	model := &costModel{
		categories: map[string]category{},
		related:    map[[2]string]struct{}{},
	}
	for _, name := range spec.Structural {
		model.categories[name] = categoryStructural
	}
	for _, name := range spec.ControlFlow {
		model.categories[name] = categoryControlFlow
	}
	for _, name := range spec.Expressions {
		model.categories[name] = categoryExpression
	}
	for _, pair := range spec.Related {
		model.related[pair] = struct{}{}
		model.related[[2]string{pair[1], pair[0]}] = struct{}{}
	}
	return model
}

func (m *costModel) Insert(node *apted.TreeNode) float64 {
	return m.multiplier(node.Label)
}

func (m *costModel) Delete(node *apted.TreeNode) float64 {
	return m.multiplier(node.Label)
}

func (m *costModel) Rename(node1, node2 *apted.TreeNode) float64 {
	if node1.Label == node2.Label {
		return 0
	}
	return 1 - m.labelSimilarity(node1.Label, node2.Label)
}

func (m *costModel) multiplier(label string) float64 {
	switch m.categories[baseType(label)] {
	case categoryStructural:
		return structuralMultiplier
	case categoryControlFlow:
		return controlFlowMultiplier
	case categoryExpression:
		return expressionMultiplier
	default:
		return defaultMultiplier
	}
}

func (m *costModel) labelSimilarity(label1, label2 string) float64 {
	base1, base2 := baseType(label1), baseType(label2)
	if base1 == base2 {
		return sameBaseTypeSimilarity
	}
	if _, ok := m.related[[2]string{base1, base2}]; ok {
		return relatedTypeSimilarity
	}
	if c := m.categories[base1]; c != categoryOther && c == m.categories[base2] {
		return sameCategorySimilarity
	}
	return 0
}

// baseType strips the parenthesized detail the engine appends to a label,
// so "identifier(count)" and "identifier(total)" share a base type.
func baseType(label string) string {
	if idx := strings.IndexByte(label, '('); idx >= 0 {
		return label[:idx]
	}
	return label
}

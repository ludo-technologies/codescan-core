package analyzer

import (
	"testing"

	"github.com/ludo-technologies/polyscan/core/apted"
	"github.com/ludo-technologies/polyscan/polyscan/internal/js/parser"
)

func TestJavaScriptCostModel(t *testing.T) {
	costModel := NewJavaScriptCostModel()

	tests := []struct {
		name      string
		label     string
		wantAbove float64
		wantBelow float64
	}{
		{name: "structural", label: "FunctionDeclaration", wantAbove: 1.0},
		{name: "control flow", label: "IfStatement", wantAbove: 1.0},
		{name: "expression", label: "BinaryExpression", wantBelow: 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cost := costModel.Insert(apted.NewTreeNode(1, tt.label))
			if tt.wantAbove > 0 && cost <= tt.wantAbove {
				t.Fatalf("Insert(%q) = %f, want > %f", tt.label, cost, tt.wantAbove)
			}
			if tt.wantBelow > 0 && cost >= tt.wantBelow {
				t.Fatalf("Insert(%q) = %f, want < %f", tt.label, cost, tt.wantBelow)
			}
		})
	}

	left := apted.NewTreeNode(2, "Identifier(foo)")
	right := apted.NewTreeNode(3, "Identifier(bar)")
	if cost := costModel.Rename(left, right); cost >= 1.0 {
		t.Fatalf("same-base-type rename cost = %f, want < 1.0", cost)
	}
}

func TestJavaScriptCostModelIgnore(t *testing.T) {
	tests := []struct {
		name       string
		costModel  *JavaScriptCostModel
		leftLabel  string
		rightLabel string
	}{
		{
			name:       "literals",
			costModel:  NewJavaScriptCostModelWithConfig(true, false),
			leftLabel:  "Literal(42)",
			rightLabel: "Literal(100)",
		},
		{
			name:       "identifiers",
			costModel:  NewJavaScriptCostModelWithConfig(false, true),
			leftLabel:  "Identifier(foo)",
			rightLabel: "Identifier(bar)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			left := apted.NewTreeNode(1, tt.leftLabel)
			right := apted.NewTreeNode(2, tt.rightLabel)
			if cost := tt.costModel.Rename(left, right); cost != 0.0 {
				t.Fatalf("Rename(%q, %q) = %f, want 0", tt.leftLabel, tt.rightLabel, cost)
			}
		})
	}
}

func TestTreeConverterConvertAST(t *testing.T) {
	converter := NewTreeConverter()
	if tree := converter.ConvertAST(nil); tree != nil {
		t.Fatal("ConvertAST(nil) should return nil")
	}

	root := parser.NewNode(parser.NodeVariableDeclaration)
	root.Kind = "const"
	identifier := parser.NewNode(parser.NodeIdentifier)
	identifier.Name = "answer"
	root.Children = append(root.Children, identifier)

	tree := converter.ConvertAST(root)
	if tree.Label != "VariableDeclaration(const)" {
		t.Fatalf("root label = %q", tree.Label)
	}
	// The converter must not pin the parser node: parser nodes carry a parent
	// pointer, so retaining one would keep the whole file AST alive for as long
	// as clone detection holds the converted tree.
	if tree.OriginalNode != nil {
		t.Fatal("converter retained the parser node, pinning the source AST")
	}
	if len(tree.Children) != 1 || tree.Children[0].Label != "Identifier(answer)" {
		t.Fatalf("converted children = %#v", tree.Children)
	}
	if tree.Children[0].Parent != tree {
		t.Fatal("converted child parent was not set")
	}
}

// Expression fields are not always present in parser.Node.Children, so this
// verifies the language adapter includes parser.OrderedChildren in APTED input.
func TestConvertASTIncludesExpressionFieldsInAPTEDDistance(t *testing.T) {
	buildIf := func(operator string) *parser.Node {
		test := parser.NewNode(parser.NodeBinaryExpression)
		test.Operator = operator
		test.Left = &parser.Node{Type: parser.NodeIdentifier, Name: "x"}
		test.Right = &parser.Node{Type: parser.NodeIdentifier, Name: "y"}

		ifStatement := parser.NewNode(parser.NodeIfStatement)
		ifStatement.Test = test
		ifStatement.Consequent = &parser.Node{
			Type: parser.NodeBlockStatement,
			Body: []*parser.Node{{Type: parser.NodeReturnStatement}},
		}
		return ifStatement
	}

	converter := NewTreeConverter()
	left := converter.ConvertAST(buildIf("<"))
	right := converter.ConvertAST(buildIf(">"))
	if left.Size() <= 2 {
		t.Fatalf("expression fields were not converted, tree size = %d", left.Size())
	}

	analyzer := apted.NewAPTEDAnalyzerWithNormalization(apted.NewDefaultCostModel(), apted.NormalizeByMax)
	distance, similarity := analyzer.ComputeDistanceAndSimilarity(left, right)
	if distance != 1.0 {
		t.Fatalf("operator-only distance = %f, want 1", distance)
	}
	wantSimilarity := 1.0 - 1.0/float64(left.Size())
	if similarity != wantSimilarity {
		t.Fatalf("operator-only similarity = %f, want %f", similarity, wantSimilarity)
	}
}

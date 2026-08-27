// Package golang defines how polyscan analyzes Go.
package golang

import (
	"github.com/ludo-technologies/polyscan/polyscan/internal/engine"
	tsgo "github.com/smacker/go-tree-sitter/golang"
)

// Language is the Go definition. Function literals are not extracted on
// their own: their decision points count toward the enclosing function, the
// same way gocyclo reports them.
var Language = &engine.Language{
	Name:       "Go",
	Extensions: []string{".go"},
	Grammar:    tsgo.GetLanguage(),
	TestFiles:  []string{"*_test.go"},
	// The definition patterns of tree-sitter-go's queries/tags.scm, with the
	// receiver captured so methods report as "Type.Method". The Go
	// specification allows a receiver type of T or *T, where T may carry
	// type parameters, and the whole type may be parenthesized, which gofmt
	// removes but the compiler accepts.
	Definitions: `
(function_declaration
  name: (identifier) @name) @definition.function

(method_declaration
  receiver: (parameter_list
    (parameter_declaration
      type: [
        (type_identifier) @receiver
        (pointer_type (type_identifier) @receiver)
        (generic_type type: (type_identifier) @receiver)
        (pointer_type (generic_type type: (type_identifier) @receiver))
        (parenthesized_type [
          (type_identifier) @receiver
          (pointer_type (type_identifier) @receiver)
          (generic_type type: (type_identifier) @receiver)
          (pointer_type (generic_type type: (type_identifier) @receiver))
        ])
      ]))
  name: (field_identifier) @name) @definition.method
`,
	// default_case is deliberately absent: the default arm is the no-match
	// path that the other cases already branch away from.
	Decisions: `
(if_statement) @branch
(for_statement) @loop
(expression_case) @case
(type_case) @case
(communication_case) @case
(binary_expression operator: ["&&" "||"]) @logical_operator
`,
	Clone: engine.CloneSpec{
		Identifiers: []string{"identifier", "field_identifier", "type_identifier", "package_identifier", "label_name"},
		Literals: []string{
			"int_literal", "float_literal", "imaginary_literal", "rune_literal",
			"interpreted_string_literal", "raw_string_literal",
			"true", "false", "nil", "iota",
		},
		Patterns: []string{
			"if_statement", "for_statement", "expression_switch_statement", "type_switch_statement",
			"select_statement", "return_statement", "defer_statement", "go_statement",
			"break_statement", "continue_statement", "labeled_statement", "send_statement",
			"func_literal", "call_expression", "selector_expression", "index_expression",
			"slice_expression", "type_assertion_expression", "binary_expression", "unary_expression",
			"assignment_statement", "short_var_declaration", "var_declaration", "const_declaration",
			"composite_literal",
		},
		Structural: []string{
			"source_file", "function_declaration", "method_declaration", "func_literal",
			"type_declaration", "type_spec", "struct_type", "interface_type",
		},
		ControlFlow: []string{
			"if_statement", "for_statement", "for_clause", "range_clause",
			"expression_switch_statement", "type_switch_statement", "select_statement",
			"expression_case", "type_case", "communication_case", "default_case",
			"return_statement", "break_statement", "continue_statement", "goto_statement",
			"fallthrough_statement", "defer_statement", "go_statement", "labeled_statement",
		},
		Expressions: []string{
			"binary_expression", "unary_expression", "call_expression", "selector_expression",
			"index_expression", "slice_expression", "type_assertion_expression",
			"type_conversion_expression", "type_instantiation_expression", "parenthesized_expression",
			"composite_literal", "literal_value", "keyed_element",
			"assignment_statement", "short_var_declaration", "inc_statement", "dec_statement",
			"send_statement",
		},
		Related: [][2]string{
			{"expression_switch_statement", "type_switch_statement"},
			{"expression_case", "type_case"},
			{"expression_case", "communication_case"},
			{"assignment_statement", "short_var_declaration"},
			{"var_declaration", "short_var_declaration"},
			{"for_clause", "range_clause"},
			{"binary_expression", "unary_expression"},
			{"inc_statement", "dec_statement"},
			{"go_statement", "defer_statement"},
		},
	},
}

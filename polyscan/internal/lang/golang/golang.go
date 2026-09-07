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
	// receiver captured so methods report as "Type.Method" and its name, when
	// the receiver has one, as the variable the method reaches its fields
	// through. The Go specification allows a receiver type of T or *T, where
	// T may carry type parameters, and the whole type may be parenthesized,
	// which gofmt removes but the compiler accepts.
	Definitions: `
(function_declaration
  name: (identifier) @name) @definition.function

(method_declaration
  receiver: (parameter_list
    (parameter_declaration
      name: (identifier)? @self
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
	// A field is anything selected from the receiver variable, so a promoted
	// method of an embedded type counts as the embedded field it comes
	// through; a sibling method is a call through the receiver. The engine
	// discards a match whose @object is not the method's receiver.
	Members: `
(selector_expression operand: (identifier) @object field: (field_identifier) @field)
(call_expression function: (selector_expression operand: (identifier) @object field: (field_identifier) @call))
`,
	// A local declaration hides the receiver from its end to the end of
	// the block, case clause, if, for or switch statement that holds it; a
	// type switch alias from the end of the switched value; a function
	// literal's parameters throughout the literal. The var and const
	// patterns also match package-level declarations, whose scope is the
	// file; the engine drops those, as a receiver shadows them instead.
	Bindings: `
(_ (short_var_declaration left: (expression_list (identifier) @binding)) @declaration) @scope
(for_statement (for_clause initializer: (short_var_declaration left: (expression_list (identifier) @binding)) @declaration)) @scope
(_ (range_clause left: (expression_list (identifier) @binding)) @declaration) @scope
(_ (var_declaration (var_spec name: (identifier) @binding)) @declaration) @scope
(_ (var_declaration (var_spec_list (var_spec name: (identifier) @binding))) @declaration) @scope
(_ (const_declaration (const_spec name: (identifier) @binding)) @declaration) @scope
(type_switch_statement alias: (expression_list (identifier) @binding) value: (_) @declaration) @scope
(_ (receive_statement left: (expression_list (identifier) @binding)) @declaration) @scope
(func_literal parameters: (parameter_list [(parameter_declaration name: (identifier) @binding) (variadic_parameter_declaration name: (identifier) @binding)] @declaration)) @scope
(func_literal result: (parameter_list (parameter_declaration name: (identifier) @binding) @declaration)) @scope
`,
	// Every type_spec is a type; an alias (type_alias) only names another.
	// The span is the spec rather than the whole declaration, since a
	// grouped declaration holds several specs. An interface matches both
	// patterns and the engine keeps one span, abstract.
	Types: `
(type_spec name: (type_identifier) @name) @type
(type_spec name: (type_identifier) @name type: (interface_type)) @abstract
`,
	// Every type_identifier is a reference, including predeclared types and
	// type parameters, which the analysis drops because nothing declares
	// them; the qualified form adds the package. An embedded field is a
	// field_declaration without a name, and the star of an embedded *T is a
	// bare token, so the type is the identifier itself. An embedded
	// interface is a type_elem of one term; a type set such as int | MyInt
	// is a constraint, not embedding.
	References: `
(type_identifier) @reference
(qualified_type package: (package_identifier) @package name: (type_identifier) @reference)
(field_declaration !name type: [
  (type_identifier) @embedded
  (qualified_type package: (package_identifier) @package name: (type_identifier) @embedded)
  (generic_type type: (type_identifier) @embedded)
  (generic_type type: (qualified_type package: (package_identifier) @package name: (type_identifier) @embedded))
])
(interface_type (type_elem . (type_identifier) @embedded .))
(interface_type (type_elem . (qualified_type package: (package_identifier) @package name: (type_identifier) @embedded) .))
`,
	// Methods of one type may be spread over the files of its package.
	TypeSpansDirectory: true,
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
	// An if in the else arm of another if continues that if's chain.
	Nesting: `
(if_statement) @nesting
(if_statement alternative: (if_statement) @continuation)
(for_statement) @nesting
(expression_switch_statement) @nesting
(type_switch_statement) @nesting
(select_statement) @nesting
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

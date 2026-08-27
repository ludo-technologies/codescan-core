// Package rust defines how polyscan analyzes Rust.
package rust

import (
	"github.com/ludo-technologies/polyscan/polyscan/internal/engine"
	tsrust "github.com/smacker/go-tree-sitter/rust"
)

// Language is the Rust definition. Closures are not extracted on their
// own: their decision points count toward the enclosing function, as Go's
// function literals do. Macro invocations parse as token trees, so the
// code inside a macro call contributes tokens but no structure, which
// lowers clone detection recall on macro-heavy code.
var Language = &engine.Language{
	Name:       "Rust",
	Extensions: []string{".rs"},
	Grammar:    tsrust.GetLanguage(),
	// A test module split into its own file is declared as
	// #[cfg(test)] mod tests; in the parent, which a single file cannot
	// see, so the conventional names stand in for the attribute: tests.rs and
	// *_tests.rs, and a tests directory, which also holds Cargo's
	// integration tests.
	TestFiles: []string{"tests.rs", "*_tests.rs", "tests/"},
	// Tests kept next to the code are found by attribute: #[test]
	// functions and any item under #[cfg(test)] or #[cfg(all(test, ...))],
	// with any other attributes between the marker and the item. In the
	// direct form the flag is a child of the argument list, so
	// #[cfg(not(test))], whose test sits one level deeper, does not match;
	// the nested form is restricted to all(...), because code under
	// any(test, ...) is also built without tests.
	TestCode: `
((attribute_item (attribute (identifier) @attr)) . (attribute_item)* .
  (function_item) @test
  (#eq? @attr "test"))
((attribute_item (attribute (identifier) @attr arguments: (token_tree (identifier) @flag))) . (attribute_item)* .
  [(function_item) (impl_item) (trait_item) (mod_item)] @test
  (#eq? @attr "cfg") (#eq? @flag "test"))
((attribute_item (attribute (identifier) @attr arguments: (token_tree (identifier) @combinator (token_tree (identifier) @flag)))) . (attribute_item)* .
  [(function_item) (impl_item) (trait_item) (mod_item)] @test
  (#eq? @attr "cfg") (#eq? @combinator "all") (#eq? @flag "test"))
`,
	// The function pattern of tree-sitter-rust's queries/tags.scm, plus
	// the same node inside an impl or trait body, where the implemented
	// type or the trait becomes the receiver. The engine merges the two
	// matches of one function node and keeps the receiver.
	Definitions: `
(function_item name: (identifier) @name) @definition.function

(impl_item type: (_) @receiver
  body: (declaration_list (function_item name: (identifier) @name) @definition.method))

(trait_item name: (type_identifier) @receiver
  body: (declaration_list (function_item name: (identifier) @name) @definition.method))
`,
	// A match is exhaustive, so its last arm is the path the other arms
	// branch away from and only arms followed by another arm count; an
	// arm's guard is a further branch. let-else branches on the pattern.
	// In a let chain the && are tokens of the chain rather than binary
	// expressions. The ? operator is an early return and counts like the
	// exception edge core/cfg counts.
	Decisions: `
(if_expression) @branch
(match_pattern condition: (_)) @branch
(let_declaration alternative: (_)) @branch
(match_block (match_arm) @case . (match_arm))
(for_expression) @loop
(while_expression) @loop
(loop_expression) @loop
(binary_expression operator: ["&&" "||"]) @logical_operator
(let_chain "&&" @logical_operator)
(try_expression) @try_operator
`,
	Clone: engine.CloneSpec{
		Identifiers: []string{
			"identifier", "field_identifier", "type_identifier", "shorthand_field_identifier",
			"primitive_type", "lifetime", "metavariable",
		},
		Literals: []string{
			"integer_literal", "float_literal", "string_literal", "raw_string_literal",
			"char_literal", "boolean_literal", "negative_literal",
		},
		Patterns: []string{
			"if_expression", "match_expression", "match_arm", "for_expression", "while_expression",
			"loop_expression", "return_expression", "break_expression", "continue_expression",
			"try_expression", "await_expression", "closure_expression", "call_expression",
			"field_expression", "index_expression", "binary_expression", "unary_expression",
			"assignment_expression", "compound_assignment_expr", "let_declaration",
			"struct_expression", "macro_invocation", "reference_expression", "unsafe_block",
			"async_block", "range_expression", "tuple_expression", "array_expression",
			"type_cast_expression",
		},
		Structural: []string{
			"source_file", "function_item", "closure_expression", "impl_item", "trait_item",
			"struct_item", "enum_item", "union_item", "mod_item", "type_item",
		},
		ControlFlow: []string{
			"if_expression", "else_clause", "let_condition", "let_chain",
			"match_expression", "match_block", "match_arm",
			"for_expression", "while_expression", "loop_expression",
			"return_expression", "break_expression", "continue_expression", "try_expression",
		},
		Expressions: []string{
			"binary_expression", "unary_expression", "call_expression", "arguments",
			"field_expression", "index_expression", "assignment_expression",
			"compound_assignment_expr", "reference_expression", "struct_expression",
			"tuple_expression", "array_expression", "range_expression", "type_cast_expression",
			"parenthesized_expression", "await_expression", "macro_invocation",
		},
		Related: [][2]string{
			{"for_expression", "while_expression"},
			{"for_expression", "loop_expression"},
			{"while_expression", "loop_expression"},
			{"if_expression", "match_expression"},
			{"assignment_expression", "compound_assignment_expr"},
			{"let_declaration", "assignment_expression"},
			{"binary_expression", "unary_expression"},
			{"struct_expression", "tuple_expression"},
			{"return_expression", "try_expression"},
		},
	},
}

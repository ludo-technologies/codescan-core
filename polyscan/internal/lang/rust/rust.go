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
	Name:           "Rust",
	Extensions:     []string{".rs"},
	Grammar:        tsrust.GetLanguage(),
	ScopeSeparator: "::",
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
	// The function pattern of tree-sitter-rust's queries/tags.scm, with the
	// self parameter captured in both its shorthand and typed forms. Impl
	// and trait bodies are scopes, so their functions read Type::method,
	// and so are modules. Only an impl makes its functions methods: a trait
	// has no fields for its default methods to share, and a function of a
	// module is free.
	Definitions: `
(function_item
  name: (identifier) @name
  parameters: (parameters [(self_parameter) (parameter pattern: (self))]? @self)) @definition.function
`,
	// An impl names its type by the bare identifier, so impl G<T> and
	// impl G<i32> are blocks of one type G, a path keeps its last segment,
	// and a reference such as &G is G, since its methods still touch self.
	// An impl for any other form, a trait object, tuple, array or pointer,
	// keeps its text as a module so its functions stay qualified without
	// becoming methods of a type.
	Scopes: `
(impl_item type: [
  (type_identifier) @receiver
  (generic_type type: (type_identifier) @receiver)
  (scoped_type_identifier name: (type_identifier) @receiver)
  (generic_type type: (scoped_type_identifier name: (type_identifier) @receiver))
  (reference_type type: [
    (type_identifier) @receiver
    (generic_type type: (type_identifier) @receiver)
    (scoped_type_identifier name: (type_identifier) @receiver)
    (generic_type type: (scoped_type_identifier name: (type_identifier) @receiver))
  ])
  [(dynamic_type) (tuple_type) (array_type) (pointer_type) (abstract_type) (function_type) (unit_type)] @module
] body: (declaration_list) @scope)
(trait_item name: (type_identifier) @module body: (declaration_list) @scope)
(mod_item name: (identifier) @module body: (declaration_list) @scope)
`,
	// Structs, enums and unions are types, a trait an abstract one, and an
	// impl adds to the type it names without declaring it; an alias
	// (type_item) only names another type.
	Types: `
(struct_item name: (type_identifier) @name) @type
(enum_item name: (type_identifier) @name) @type
(union_item name: (type_identifier) @name) @type
(trait_item name: (type_identifier) @name) @abstract
(impl_item type: [
  (type_identifier) @name
  (generic_type type: (type_identifier) @name)
  (scoped_type_identifier name: (type_identifier) @name)
  (generic_type type: (scoped_type_identifier name: (type_identifier) @name))
  (reference_type type: [
    (type_identifier) @name
    (generic_type type: (type_identifier) @name)
    (scoped_type_identifier name: (type_identifier) @name)
    (generic_type type: (scoped_type_identifier name: (type_identifier) @name))
  ])
]) @impl
`,
	// Every type_identifier is a reference except Self; primitive types are
	// their own node and never match. A path expression such as Foo::new()
	// or Color::Red names its type in the path's identifiers, and a tuple
	// struct pattern names it in the pattern; module segments and generic
	// parameters caught this way are dropped by the analysis because
	// nothing declares them. The trait an impl implements is inheritance.
	References: `
((type_identifier) @reference (#not-eq? @reference "Self"))
((scoped_identifier path: [(identifier) @reference (scoped_identifier name: (identifier) @reference)])
  (#not-eq? @reference "Self"))
(tuple_struct_pattern type: (identifier) @reference)
(impl_item trait: [
  (type_identifier) @embedded
  (generic_type type: (type_identifier) @embedded)
  (scoped_type_identifier name: (type_identifier) @embedded)
])
`,
	// A field is anything reached from self, including a tuple struct's
	// numbered fields; a sibling method is a method call on self or an
	// associated function called through Self.
	Members: `
(field_expression value: (self) field: [(field_identifier) (integer_literal)] @field)
(call_expression function: (field_expression value: (self) field: (field_identifier) @call))
((call_expression function: (scoped_identifier path: (identifier) @path name: (identifier) @call))
  (#eq? @path "Self"))
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
	// An if in the else arm of another if continues that if's chain. The
	// else block of a let-else is one level deep, as it branches.
	Nesting: `
(if_expression) @nesting
(if_expression alternative: (else_clause (if_expression) @continuation))
(let_declaration alternative: (_)) @nesting
(match_expression) @nesting
(for_expression) @nesting
(while_expression) @nesting
(loop_expression) @nesting
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

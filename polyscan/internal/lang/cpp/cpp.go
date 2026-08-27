// Package cpp defines how polyscan analyzes C++.
package cpp

import (
	"github.com/ludo-technologies/polyscan/polyscan/internal/engine"
	tscpp "github.com/smacker/go-tree-sitter/cpp"
)

// declarators matches a function's declarator: the function_declarator
// itself, or one wrapped in the pointer and reference declarators of a
// pointer, pointer-to-pointer, reference or reference-to-pointer return
// type.
const declarators = `[
  (function_declarator declarator: ` + names + `)
  (pointer_declarator declarator: (function_declarator declarator: ` + names + `))
  (pointer_declarator declarator: (pointer_declarator declarator: (function_declarator declarator: ` + names + `)))
  (reference_declarator (function_declarator declarator: ` + names + `))
  (pointer_declarator declarator: (reference_declarator (function_declarator declarator: ` + names + `)))
]`

// names are the nodes that name a function: a plain or member name, a
// destructor, an operator, or a qualified name such as ns::Type::method.
const names = `[(identifier) (field_identifier) (destructor_name) (operator_name) (qualified_identifier)] @name`

// Language is the C++ definition. Files are parsed one at a time without
// the preprocessor, so every branch of a conditional inclusion is
// analyzed and a file whose syntax only makes sense after macro expansion
// is reported as a syntax error. Headers are analyzed as C++; C code
// parses under the C++ grammar with few differences. Lambdas are not
// extracted on their own: their decision points count toward the
// enclosing function, as Go's function literals do.
var Language = &engine.Language{
	Name:           "C++",
	Extensions:     []string{".cpp", ".cc", ".cxx", ".hpp", ".hh", ".hxx", ".h", ".ipp", ".inl"},
	Grammar:        tscpp.GetLanguage(),
	ScopeSeparator: "::",
	// The conventions of GoogleTest, Catch2 and doctest: test files next to
	// the code or in a test directory.
	TestFiles: []string{
		"*_test.cc", "*_test.cpp", "*_test.cxx", "*_tests.cc", "*_tests.cpp", "*_tests.cxx",
		"test_*.cc", "test_*.cpp", "test_*.cxx",
		"*Test.cc", "*Test.cpp", "*Test.cxx", "*Tests.cc", "*Tests.cpp", "*Tests.cxx",
		"test/", "tests/",
	},
	// The function pattern of tree-sitter-cpp's queries/tags.scm, widened
	// to pointer and reference return types. Members defined inside their
	// class carry a plain name that the scopes query qualifies.
	Definitions: `(function_definition declarator: ` + declarators + `) @definition.function`,
	Scopes: `
(class_specifier name: (type_identifier) @receiver body: (field_declaration_list) @scope)
(struct_specifier name: (type_identifier) @receiver body: (field_declaration_list) @scope)
(union_specifier name: (type_identifier) @receiver body: (field_declaration_list) @scope)
(namespace_definition name: (namespace_identifier) @receiver body: (declaration_list) @scope)
`,
	// A case without a value is the default, the path the other cases
	// branch away from. A catch clause is the exception edge core/cfg
	// counts.
	Decisions: `
(if_statement) @branch
(conditional_expression) @ternary
(for_statement) @loop
(for_range_loop) @loop
(while_statement) @loop
(do_statement) @loop
(case_statement value: (_)) @case
(catch_clause) @exception
(binary_expression operator: ["&&" "||"]) @logical_operator
`,
	Clone: engine.CloneSpec{
		Identifiers: []string{
			"identifier", "field_identifier", "type_identifier", "namespace_identifier",
			"statement_identifier", "primitive_type", "this", "auto",
		},
		Literals: []string{
			"number_literal", "string_literal", "char_literal", "raw_string_literal",
			"user_defined_literal", "true", "false", "null",
		},
		Patterns: []string{
			"if_statement", "for_statement", "for_range_loop", "while_statement", "do_statement",
			"switch_statement", "case_statement", "try_statement", "catch_clause", "throw_statement",
			"return_statement", "break_statement", "continue_statement", "goto_statement",
			"call_expression", "field_expression", "subscript_expression", "binary_expression",
			"unary_expression", "update_expression", "assignment_expression", "conditional_expression",
			"cast_expression", "new_expression", "delete_expression", "lambda_expression",
			"declaration", "init_declarator", "initializer_list", "pointer_expression",
			"sizeof_expression", "co_await_expression", "template_function",
		},
		Structural: []string{
			"translation_unit", "function_definition", "lambda_expression",
			"class_specifier", "struct_specifier", "union_specifier", "enum_specifier",
			"namespace_definition", "template_declaration",
		},
		ControlFlow: []string{
			"if_statement", "else_clause", "condition_clause",
			"for_statement", "for_range_loop", "while_statement", "do_statement",
			"switch_statement", "case_statement", "try_statement", "catch_clause", "throw_statement",
			"return_statement", "break_statement", "continue_statement", "goto_statement",
			"labeled_statement", "co_return_statement",
		},
		Expressions: []string{
			"binary_expression", "unary_expression", "update_expression", "assignment_expression",
			"conditional_expression", "call_expression", "argument_list", "field_expression",
			"subscript_expression", "cast_expression", "pointer_expression", "new_expression",
			"delete_expression", "sizeof_expression", "parenthesized_expression", "comma_expression",
			"initializer_list", "compound_literal_expression", "co_await_expression",
		},
		Related: [][2]string{
			{"for_statement", "for_range_loop"},
			{"for_statement", "while_statement"},
			{"for_range_loop", "while_statement"},
			{"while_statement", "do_statement"},
			{"if_statement", "conditional_expression"},
			{"if_statement", "switch_statement"},
			{"binary_expression", "unary_expression"},
			{"assignment_expression", "update_expression"},
			{"class_specifier", "struct_specifier"},
		},
	},
}

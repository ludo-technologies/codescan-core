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
}

// Package parser wraps tree-sitter and turns JavaScript and TypeScript source
// into the syntax tree the analyzers work on.
//
// tree-sitter is error tolerant, so a file with a syntax error still yields a
// usable tree rather than nothing. That is what lets jscan analyze a codebase
// mid-refactor. It also means no type information is available anywhere in
// jscan: nothing here invokes tsc or reads tsconfig.json, so every finding
// comes from syntax alone.
//
// The parser handles .js, .jsx, .mjs, .cjs, .ts, .tsx, .mts, and .cts. Vue
// single-file components are not supported, so the script block inside a .vue
// file is never parsed.
//
// Node is jscan's own syntax tree node, built from the tree-sitter parse tree
// rather than wrapping it. It carries the children, source location, parent
// link, and the declaration fields the analyzers need, such as Name, Params,
// and Body. Its Type is a NodeType, whose values are jscan's own names for the
// constructs the analyzers match on, for example NodeIfStatement and
// NodeReturnStatement, rather than the underlying tree-sitter node kinds.
//
// Because tree-sitter is a C library reached through cgo, this package is the
// reason jscan cannot be cross-compiled and the reason building from source
// needs a C compiler.
package parser

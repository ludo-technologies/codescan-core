// Package engine runs declarative, per-language tree-sitter queries and turns
// their captures into per-function metrics. A language contributes no Go
// code beyond its Language value: which grammar to load, which query finds
// its functions, which query marks its decision points, and which node
// types matter for clone detection.
package engine

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/ludo-technologies/polyscan/core/apted"
	sitter "github.com/smacker/go-tree-sitter"
)

// Language describes how one language is analyzed.
type Language struct {
	// Name is the user-facing language name, such as "Go".
	Name string
	// Extensions lists the file extensions, with the leading dot, that
	// belong to this language.
	Extensions []string
	// Grammar is the tree-sitter grammar the queries are written against.
	Grammar *sitter.Language
	// Definitions is a tree-sitter query that matches every function once.
	// Its @definition.<kind> capture spans the whole function and @name its
	// name. An optional @receiver capture holds a receiver type declared on
	// the function itself, as Go methods do, and is prefixed to the name
	// with the ScopeSeparator. An optional @self capture holds the receiver
	// parameter of a method that has one.
	Definitions string
	// Scopes is an optional tree-sitter query for the named scopes that
	// enclose functions, such as classes, impl blocks and namespaces:
	// @scope spans the scope and @receiver or @module holds its name. A
	// function is named after the scopes that contain it, outermost first,
	// so a member defined inside its class and one defined outside it with a
	// qualified name read the same. A @receiver scope is a type whose
	// functions are its methods; a @module scope only qualifies names.
	Scopes string
	// ScopeSeparator joins scope and receiver names to function names;
	// empty means ".".
	ScopeSeparator string
	// Decisions is a tree-sitter query in which every capture is one
	// decision point. The point is attributed to the innermost function
	// that contains it and counted under the capture's name, so the capture
	// names double as the breakdown reported next to the complexity.
	Decisions string
	// Members is an optional tree-sitter query for what a method does with
	// its own type: @field captures the name of a field it reads or writes
	// and @call the name of a sibling method it calls. A match may also
	// capture @object, the variable the access goes through, and then only
	// counts when that variable is the method's receiver, as Go's named
	// receivers require; a language whose receiver is a keyword, as Rust's
	// self is, leaves @object out. The Definitions query marks the receiver
	// with @self: a function with a receiver type but no @self, such as a
	// Rust associated function, is a method that cannot touch instance
	// state and is left out of cohesion. A language without a Members
	// query has no cohesion analysis.
	Members string
	// Bindings is an optional tree-sitter query for the local declarations
	// that can shadow the receiver a Members match goes through: @binding
	// is the declared name, @declaration the node after whose end the name
	// is in scope, as the right-hand side of a short variable declaration
	// still sees the outer name, and @scope the node at whose end it goes
	// out of scope. A Members match whose @object is shadowed at that point
	// is not an access to the receiver.
	Bindings string
	// Types is an optional tree-sitter query for the named types a file
	// declares, which the coupling (CBO) analysis measures: @name is the
	// type's name and the capture that spans the match says what it is,
	// @type a concrete declaration, @abstract an interface or trait, @impl a
	// block that adds to a type without declaring it, as a Rust impl does. A
	// language without a Types query has no coupling analysis.
	Types string
	// References is an optional tree-sitter query, paired with Types, for
	// the places a type is referred to: @reference is the name node, and an
	// optional @package the qualifier it is reached through, as in Go's
	// pkg.T. A match may capture @embedded instead of @reference for an
	// embedded field or interface or an implemented trait, which is
	// inheritance rather than use. A reference belongs to the type whose
	// method or declaration contains it.
	References string
	// Nesting is a tree-sitter query whose captures are the constructs that
	// open a nesting level: branches, loops, switches and try blocks. A
	// capture named @continuation marks a construct that continues the
	// level its parent opened rather than deepening it, as an if in the
	// else arm of another if does. A function's nesting depth is the
	// longest chain of these constructs inside it, with the function body
	// at depth 0. Code inside a function that is extracted on its own
	// belongs to that function; code inside a closure that is not counts
	// toward the enclosing function, as its decision points do.
	Nesting string
	// Clone names the node types clone detection prices and compares.
	Clone CloneSpec
	// TestFiles names test files: a glob matches the file name, and a
	// pattern ending in a slash names a directory anywhere on the path.
	// TestCode is an optional tree-sitter query whose captures span test
	// code inside a file, such as attribute-marked test functions or
	// modules. Both are analyzed for complexity but excluded from clone
	// detection, where the shared skeleton of test functions swamps the
	// report.
	TestFiles []string
	TestCode  string
	// TypeSpansDirectory reports that the methods of one type may be
	// declared across the files of a directory, as Go's may be across the
	// files of a package, so cohesion is computed per directory and type
	// rather than per file and type.
	TypeSpansDirectory bool

	compileOnce sync.Once
	compileErr  error
	definitions *sitter.Query
	decisions   *sitter.Query
	nesting     *sitter.Query
	scopes      *sitter.Query
	members     *sitter.Query
	bindings    *sitter.Query
	types       *sitter.Query
	references  *sitter.Query
	testCode    *sitter.Query
	identifiers map[string]struct{}
	literals    map[string]struct{}
}

// CloneSpec names the node types clone detection needs to know about. Node
// types are the tree-sitter names, such as "if_statement". Anything not
// listed is handled with the default cost and no special feature.
type CloneSpec struct {
	// Identifiers are node types whose text is a name. Their tree label
	// carries the name, as in "identifier(count)", so an exact structural
	// match with different names is still told apart from an exact clone.
	Identifiers []string
	// Literals are node types whose text is a literal value, labeled the
	// same way. A literal node becomes a leaf of the tree.
	Literals []string
	// Patterns are node types whose presence in a fragment is a structural
	// feature for similarity hashing and the Jaccard pre-filter.
	Patterns []string
	// Structural, ControlFlow and Expressions are the cost tiers of the tree
	// edit distance: editing a structural or control-flow node is priced
	// above the default, editing an expression node below it.
	Structural  []string
	ControlFlow []string
	Expressions []string
	// Related pairs node types that express the same construct in different
	// syntactic forms, so renaming between them is cheaper than between
	// unrelated types.
	Related [][2]string
}

// Function is one function or method found in a source file.
type Function struct {
	// Name is the function name, prefixed with its receiver for methods.
	Name string
	// Kind is the suffix of the @definition capture that matched, such as
	// "function" or "method".
	Kind string
	// StartLine, StartColumn, EndLine and EndColumn are 1-based source
	// positions.
	StartLine   int
	StartColumn int
	EndLine     int
	EndColumn   int
	// Complexity is the McCabe cyclomatic complexity: one plus the number of
	// decision points, following the same conventions core/cfg applies to a
	// control flow graph. A two-way branch or loop counts one, a switch
	// counts one per case with the default excluded, and each short-circuit
	// operator counts one.
	Complexity int
	// Decisions breaks the decision points down by the name of the capture
	// that produced them.
	Decisions map[string]int
	// NestingDepth is the deepest chain of nested control structures inside
	// the function, per the language's Nesting query. It is 0 for a
	// language without one.
	NestingDepth int
	// Tree is the function's syntax tree in the form the tree edit distance
	// consumes: named nodes only, comments dropped, labeled per CloneSpec.
	Tree *apted.TreeNode
	// Content is the function's source text with each comment replaced by
	// a space, for exact-match (Type-1) clone classification.
	Content string
	// CodeLines counts the lines of Content that are not blank, so that
	// comments and spacing do not change how large a function is.
	CodeLines int
	// IsTest reports that the function lies in a span the language's
	// TestCode query captured.
	IsTest bool
	// Receiver is the qualified name of the type the function is a method
	// of, or empty for a free function.
	Receiver string
	// HasSelf reports that the method has a receiver parameter through
	// which it can reach instance state. Fields and Calls are the names of
	// the fields it accesses and the sibling methods it calls, per the
	// language's Members query.
	HasSelf bool
	Fields  map[string]bool
	Calls   map[string]bool

	startByte uint32
	endByte   uint32
	hasError  bool
	self      string
}

// Type is one named type of a source file, with every reference to another
// type that its declaration and its methods in the file make.
type Type struct {
	// Name is the type name, prefixed with its enclosing scopes the way
	// function names are.
	Name string
	// StartLine and EndLine span the declaration, or the first block that
	// adds to the type when the file declares it elsewhere or not at all.
	StartLine int
	EndLine   int
	// Declared reports that the file declares the type. A file may only add
	// methods to a type, as Go files of one package and Rust impl blocks do;
	// such a type is not Declared here.
	Declared bool
	// Abstract reports an interface or trait.
	Abstract bool
	// IsTest reports that every part of the type lies in test code.
	IsTest bool
	// References lists the types referred to, in source order without
	// duplicates.
	References []Reference
}

// Reference is one type named by another type's declaration or methods.
type Reference struct {
	// Name is the referenced type's name; Package the qualifier it is
	// reached through, or empty for an unqualified name.
	Name    string
	Package string
	// Embedded marks an embedded field or interface or an implemented trait.
	Embedded bool
}

const (
	definitionPrefix    = "definition."
	nameCapture         = "name"
	receiverCapture     = "receiver"
	selfCapture         = "self"
	scopeCapture        = "scope"
	moduleCapture       = "module"
	fieldCapture        = "field"
	callCapture         = "call"
	objectCapture       = "object"
	bindingCapture      = "binding"
	declarationCapture  = "declaration"
	continuationCapture = "continuation"
	typeCapture         = "type"
	abstractCapture     = "abstract"
	implCapture         = "impl"
	referenceCapture    = "reference"
	embeddedCapture     = "embedded"
	packageCapture      = "package"
	operatorField       = "operator"
)

// HasCohesion reports whether the language declares what a method does
// with its type, which the cohesion (LCOM4) analysis needs.
func (l *Language) HasCohesion() bool {
	return l.Members != ""
}

// HasCoupling reports whether the language declares its types and their
// references, which the coupling (CBO) analysis needs.
func (l *Language) HasCoupling() bool {
	return l.Types != ""
}

// Separator is the ScopeSeparator, or "." when none is set.
func (l *Language) Separator() string {
	if l.ScopeSeparator == "" {
		return "."
	}
	return l.ScopeSeparator
}

// IsTestFile reports whether the path matches one of the language's
// TestFiles patterns.
func (l *Language) IsTestFile(path string) bool {
	components := strings.Split(filepath.ToSlash(filepath.Clean(path)), "/")
	base := components[len(components)-1]
	for _, pattern := range l.TestFiles {
		if dir, ok := strings.CutSuffix(pattern, "/"); ok {
			for _, component := range components[:len(components)-1] {
				if component == dir {
					return true
				}
			}
			continue
		}
		if matched, err := filepath.Match(pattern, base); err == nil && matched {
			return true
		}
	}
	return false
}

// Result is the analysis of one file.
type Result struct {
	Functions []Function
	// Types lists the file's named types with their references, per the
	// language's Types and References queries, in order of first appearance.
	// It is nil for a language without a Types query.
	Types []Type
	// SyntaxError is the first syntax error in the file, or nil when the
	// file parsed cleanly. A tree-sitter parse always yields a tree and
	// recovers around what it cannot parse, so the functions that contain
	// no error are still reported; the ones that do are left out, because
	// their metrics would describe only the fragments the grammar managed
	// to salvage. This matters most for C++, where a macro that opens a
	// namespace or declares an attribute is a syntax error to a parser
	// that does not run the preprocessor.
	SyntaxError error
}

// Analyze parses source and returns every function the language defines,
// with its complexity computed.
func (l *Language) Analyze(source []byte) (*Result, error) {
	if err := l.compile(); err != nil {
		return nil, err
	}

	parser := sitter.NewParser()
	defer parser.Close()
	parser.SetLanguage(l.Grammar)

	tree, err := parser.ParseCtx(context.Background(), nil, source)
	if err != nil {
		return nil, err
	}
	defer tree.Close()

	root := tree.RootNode()
	result := &Result{SyntaxError: checkSyntax(root)}

	functions := l.extractFunctions(root, source)
	scopes := l.collectScopes(root, source)
	applyScopes(scopes, functions, l.Separator())
	l.countDecisions(root, source, functions)
	l.measureNesting(root, source, functions)
	l.collectMembers(root, source, functions)
	tests := l.testSpans(root, source)
	markTests(tests, functions)
	result.Types = l.collectTypes(root, source, functions, scopes, tests)
	for i := range functions {
		functions[i].Complexity = 1
		for _, count := range functions[i].Decisions {
			functions[i].Complexity += count
		}
	}
	for _, fn := range functions {
		if !fn.hasError {
			result.Functions = append(result.Functions, fn)
		}
	}
	return result, nil
}

// compile builds the queries once. A query that does not compile against
// the grammar is a language definition bug, and it is reported on first use
// rather than being silently skipped.
func (l *Language) compile() error {
	l.compileOnce.Do(func() {
		definitions, err := sitter.NewQuery([]byte(l.Definitions), l.Grammar)
		if err != nil {
			l.compileErr = fmt.Errorf("%s definitions query: %w", l.Name, err)
			return
		}
		decisions, err := sitter.NewQuery([]byte(l.Decisions), l.Grammar)
		if err != nil {
			l.compileErr = fmt.Errorf("%s decisions query: %w", l.Name, err)
			return
		}
		if l.Nesting != "" {
			nesting, err := sitter.NewQuery([]byte(l.Nesting), l.Grammar)
			if err != nil {
				l.compileErr = fmt.Errorf("%s nesting query: %w", l.Name, err)
				return
			}
			l.nesting = nesting
		}
		if l.Scopes != "" {
			scopes, err := sitter.NewQuery([]byte(l.Scopes), l.Grammar)
			if err != nil {
				l.compileErr = fmt.Errorf("%s scopes query: %w", l.Name, err)
				return
			}
			l.scopes = scopes
		}
		if l.Members != "" {
			members, err := sitter.NewQuery([]byte(l.Members), l.Grammar)
			if err != nil {
				l.compileErr = fmt.Errorf("%s members query: %w", l.Name, err)
				return
			}
			l.members = members
		}
		if l.Bindings != "" {
			bindings, err := sitter.NewQuery([]byte(l.Bindings), l.Grammar)
			if err != nil {
				l.compileErr = fmt.Errorf("%s bindings query: %w", l.Name, err)
				return
			}
			l.bindings = bindings
		}
		if l.Types != "" {
			types, err := sitter.NewQuery([]byte(l.Types), l.Grammar)
			if err != nil {
				l.compileErr = fmt.Errorf("%s types query: %w", l.Name, err)
				return
			}
			references, err := sitter.NewQuery([]byte(l.References), l.Grammar)
			if err != nil {
				l.compileErr = fmt.Errorf("%s references query: %w", l.Name, err)
				return
			}
			l.types, l.references = types, references
		}
		if l.TestCode != "" {
			testCode, err := sitter.NewQuery([]byte(l.TestCode), l.Grammar)
			if err != nil {
				l.compileErr = fmt.Errorf("%s test code query: %w", l.Name, err)
				return
			}
			l.testCode = testCode
		}
		l.definitions = definitions
		l.decisions = decisions
		l.identifiers = set(l.Clone.Identifiers)
		l.literals = set(l.Clone.Literals)
	})
	return l.compileErr
}

func set(names []string) map[string]struct{} {
	result := make(map[string]struct{}, len(names))
	for _, name := range names {
		result[name] = struct{}{}
	}
	return result
}

func (l *Language) extractFunctions(root *sitter.Node, source []byte) []Function {
	var functions []Function
	ForEachMatch(l.definitions, root, source, func(match *sitter.QueryMatch) {
		var fn Function
		var node *sitter.Node
		var receiver string
		for _, capture := range match.Captures {
			name := l.definitions.CaptureNameForId(capture.Index)
			switch {
			case strings.HasPrefix(name, definitionPrefix):
				node = capture.Node
				fn.Kind = strings.TrimPrefix(name, definitionPrefix)
			case name == nameCapture:
				fn.Name = capture.Node.Content(source)
			case name == receiverCapture:
				receiver = capture.Node.Content(source)
			case name == selfCapture:
				fn.self = capture.Node.Content(source)
			}
		}
		if node == nil || fn.Name == "" {
			return
		}
		if receiver != "" {
			fn.Name = receiver + l.Separator() + fn.Name
			fn.Receiver = receiver
		}
		// A blank receiver name cannot be referred to, so the method has no
		// way to reach instance state.
		fn.HasSelf = fn.self != "" && fn.self != "_"
		fn.StartLine = int(node.StartPoint().Row) + 1
		fn.StartColumn = int(node.StartPoint().Column) + 1
		fn.EndLine = int(node.EndPoint().Row) + 1
		fn.EndColumn = int(node.EndPoint().Column) + 1
		fn.startByte = node.StartByte()
		fn.endByte = node.EndByte()
		fn.hasError = node.HasError()
		fn.Decisions = map[string]int{}

		converter := treeConverter{language: l, source: source}
		fn.Tree = converter.convert(node)
		fn.Content = converter.contentWithoutComments(node)
		fn.CodeLines = countCodeLines(fn.Content)
		functions = append(functions, fn)
	})
	sort.Slice(functions, func(i, j int) bool {
		return functions[i].startByte < functions[j].startByte
	})
	return functions
}

// treeConverter builds the tree edit distance tree of one function and
// remembers where its comments were.
type treeConverter struct {
	language *Language
	source   []byte
	nextID   int
	comments [][2]uint32
}

// convert turns a syntax node into a labeled tree of its named descendants.
// Comments, which tree-sitter marks as extra nodes, are left out and their
// byte ranges recorded. A literal becomes a leaf because its label already
// carries the value.
func (c *treeConverter) convert(node *sitter.Node) *apted.TreeNode {
	tree := apted.NewTreeNode(c.nextID, c.label(node))
	c.nextID++
	if _, isLiteral := c.language.literals[node.Type()]; isLiteral {
		return tree
	}
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child.IsExtra() {
			c.comments = append(c.comments, [2]uint32{child.StartByte(), child.EndByte()})
			continue
		}
		tree.AddChild(c.convert(child))
	}
	return tree
}

// label is the node type, carrying the text of identifiers and literals and
// the operator of any node that has one, as in "binary_expression(+)".
func (c *treeConverter) label(node *sitter.Node) string {
	nodeType := node.Type()
	if _, ok := c.language.identifiers[nodeType]; ok {
		return nodeType + "(" + node.Content(c.source) + ")"
	}
	if _, ok := c.language.literals[nodeType]; ok {
		return nodeType + "(" + node.Content(c.source) + ")"
	}
	if operator := node.ChildByFieldName(operatorField); operator != nil {
		return nodeType + "(" + operator.Type() + ")"
	}
	return nodeType
}

// contentWithoutComments returns the node's source text with each comment
// that convert recorded replaced by a space, so that the tokens around a
// comment stay apart. It must run after convert.
func (c *treeConverter) contentWithoutComments(node *sitter.Node) string {
	var b strings.Builder
	cursor := node.StartByte()
	for _, comment := range c.comments {
		b.Write(c.source[cursor:comment[0]])
		b.WriteByte(' ')
		cursor = comment[1]
	}
	b.Write(c.source[cursor:node.EndByte()])
	return b.String()
}

// countCodeLines counts the lines that hold something other than whitespace.
func countCodeLines(content string) int {
	lines := 0
	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) != "" {
			lines++
		}
	}
	return lines
}

func (l *Language) countDecisions(root *sitter.Node, source []byte, functions []Function) {
	ForEachMatch(l.decisions, root, source, func(match *sitter.QueryMatch) {
		for _, capture := range match.Captures {
			fn := innermost(functions, capture.Node.StartByte(), capture.Node.EndByte())
			if fn == nil {
				continue
			}
			fn.Decisions[l.decisions.CaptureNameForId(capture.Index)]++
		}
	})
}

// measureNesting sets each function's NestingDepth to the deepest chain of
// nesting constructs inside it. Every construct the Nesting query captures
// is attributed to the innermost function that contains it, and its depth
// is the number of level-opening constructs on the path from it up to that
// function, itself included. A construct in a nested function belongs to
// the nested function alone, because that function is the innermost one
// containing it.
func (l *Language) measureNesting(root *sitter.Node, source []byte, functions []Function) {
	if l.nesting == nil {
		return
	}
	// levels maps each captured node to the levels it opens: one, or zero
	// for a continuation.
	levels := map[uintptr]int{}
	ForEachMatch(l.nesting, root, source, func(match *sitter.QueryMatch) {
		for _, capture := range match.Captures {
			id := capture.Node.ID()
			if l.nesting.CaptureNameForId(capture.Index) == continuationCapture {
				levels[id] = 0
			} else if _, seen := levels[id]; !seen {
				levels[id] = 1
			}
		}
	})
	ForEachMatch(l.nesting, root, source, func(match *sitter.QueryMatch) {
		for _, capture := range match.Captures {
			node := capture.Node
			fn := innermost(functions, node.StartByte(), node.EndByte())
			if fn == nil {
				continue
			}
			depth := 0
			for n := node; n != nil && (n.StartByte() != fn.startByte || n.EndByte() != fn.endByte); n = n.Parent() {
				depth += levels[n.ID()]
			}
			fn.NestingDepth = max(fn.NestingDepth, depth)
		}
	})
}

// scope is a named span from the Scopes query. isType marks a @receiver
// scope, whose functions are methods.
type scope struct {
	start, end uint32
	name       string
	isType     bool
}

// collectScopes gathers the file's named scopes in start order, or nothing
// for a language without a Scopes query.
func (l *Language) collectScopes(root *sitter.Node, source []byte) []scope {
	if l.scopes == nil {
		return nil
	}
	var scopes []scope
	ForEachMatch(l.scopes, root, source, func(match *sitter.QueryMatch) {
		var s scope
		for _, capture := range match.Captures {
			switch l.scopes.CaptureNameForId(capture.Index) {
			case scopeCapture:
				s.start, s.end = capture.Node.StartByte(), capture.Node.EndByte()
			case receiverCapture:
				s.name = capture.Node.Content(source)
				s.isType = true
			case moduleCapture:
				s.name = capture.Node.Content(source)
			}
		}
		if s.name != "" && s.end > s.start {
			scopes = append(scopes, s)
		}
	})
	sort.Slice(scopes, func(i, j int) bool { return scopes[i].start < scopes[j].start })
	return scopes
}

// enclosing returns the names of the scopes that contain the byte range,
// outermost first, and whether the innermost of them is a type.
func enclosing(scopes []scope, start, end uint32) (names []string, innermostIsType bool) {
	for _, s := range scopes {
		if s.start > start {
			break
		}
		if s.start <= start && s.end >= end {
			names = append(names, s.name)
			innermostIsType = s.isType
		}
	}
	return names, innermostIsType
}

// applyScopes prefixes each function with the names of the scopes that
// contain it, outermost first, and makes a function whose innermost scope
// is a type a method of that type, named with the same prefix.
func applyScopes(scopes []scope, functions []Function, separator string) {
	for i := range functions {
		fn := &functions[i]
		names, innermostIsType := enclosing(scopes, fn.startByte, fn.endByte)
		if len(names) == 0 {
			continue
		}
		prefix := strings.Join(names, separator)
		fn.Name = prefix + separator + fn.Name
		switch {
		case fn.Receiver != "":
			fn.Receiver = prefix + separator + fn.Receiver
		case innermostIsType:
			fn.Receiver = prefix
		}
	}
}

// collectMembers records, for each method with a receiver parameter, the
// fields it accesses and the sibling methods it calls; a free function or a
// method without one keeps nil maps. A name captured as
// both, as the callee of a method call is by a field pattern that also
// matches it, is a call.
func (l *Language) collectMembers(root *sitter.Node, source []byte, functions []Function) {
	if l.members == nil {
		return
	}
	for i := range functions {
		if functions[i].Receiver != "" && functions[i].HasSelf {
			functions[i].Fields = map[string]bool{}
			functions[i].Calls = map[string]bool{}
		}
	}
	type member struct {
		fn   *Function
		name string
	}
	fields := map[uintptr]member{}
	calls := map[uintptr]struct{}{}
	bindings := l.collectBindings(root, source)
	ForEachMatch(l.members, root, source, func(match *sitter.QueryMatch) {
		var fn *Function
		var node, object *sitter.Node
		var kind string
		for _, capture := range match.Captures {
			switch l.members.CaptureNameForId(capture.Index) {
			case fieldCapture, callCapture:
				kind = l.members.CaptureNameForId(capture.Index)
				node = capture.Node
				fn = innermost(functions, node.StartByte(), node.EndByte())
			case objectCapture:
				object = capture.Node
			}
		}
		if fn == nil || fn.Fields == nil {
			return
		}
		if object != nil {
			name := object.Content(source)
			if name != fn.self || bindings.shadows(name, object.StartByte()) {
				return
			}
		}
		name := node.Content(source)
		if kind == callCapture {
			fn.Calls[name] = true
			calls[node.ID()] = struct{}{}
			return
		}
		fields[node.ID()] = member{fn, name}
	})
	for id, field := range fields {
		if _, isCall := calls[id]; !isCall {
			field.fn.Fields[field.name] = true
		}
	}
}

// binding is one local declaration from the Bindings query: the name is in
// scope from the end of its declaration to the end of its scope.
type binding struct {
	declarationEnd, scopeStart, scopeEnd uint32
}

type bindings map[string][]binding

// collectBindings gathers the file's local declarations by name.
func (l *Language) collectBindings(root *sitter.Node, source []byte) bindings {
	result := bindings{}
	if l.bindings == nil {
		return result
	}
	ForEachMatch(l.bindings, root, source, func(match *sitter.QueryMatch) {
		var name string
		var b binding
		var scope *sitter.Node
		for _, capture := range match.Captures {
			switch l.bindings.CaptureNameForId(capture.Index) {
			case bindingCapture:
				name = capture.Node.Content(source)
			case declarationCapture:
				b.declarationEnd = capture.Node.EndByte()
			case scopeCapture:
				scope = capture.Node
				b.scopeStart, b.scopeEnd = scope.StartByte(), scope.EndByte()
			}
		}
		// A declaration whose scope is the whole file is a package-level
		// one, which a receiver shadows rather than the other way around.
		if name != "" && scope != nil && scope.Parent() != nil {
			result[name] = append(result[name], b)
		}
	})
	return result
}

// shadows reports whether a local declaration of name is in scope at the
// byte position.
func (b bindings) shadows(name string, position uint32) bool {
	for _, binding := range b[name] {
		if position >= binding.declarationEnd && position >= binding.scopeStart && position < binding.scopeEnd {
			return true
		}
	}
	return false
}

// span is a byte range of the source.
type span struct {
	start, end uint32
}

func (s span) contains(start, end uint32) bool {
	return s.start <= start && s.end >= end
}

// testSpans returns the spans the TestCode query captures.
func (l *Language) testSpans(root *sitter.Node, source []byte) []span {
	if l.testCode == nil {
		return nil
	}
	var spans []span
	ForEachMatch(l.testCode, root, source, func(match *sitter.QueryMatch) {
		for _, capture := range match.Captures {
			spans = append(spans, span{capture.Node.StartByte(), capture.Node.EndByte()})
		}
	})
	return spans
}

func inTest(tests []span, start, end uint32) bool {
	for _, test := range tests {
		if test.contains(start, end) {
			return true
		}
	}
	return false
}

// markTests flags every function inside a test span.
func markTests(tests []span, functions []Function) {
	for i := range functions {
		if inTest(tests, functions[i].startByte, functions[i].endByte) {
			functions[i].IsTest = true
		}
	}
}

// typeSpan is one match of the Types query: a declaration or a block that
// adds to a type.
type typeSpan struct {
	span
	startLine, endLine int
	name               string
	declared           bool
	abstract           bool
}

// collectTypes gathers the file's types and attributes every reference to
// the type whose declaration or method contains it. A reference inside a
// method belongs to the method's receiver type; inside a function without a
// receiver, such as a trait's default method, to the innermost type span
// around that function; outside any function, to the innermost type span
// around it; and a reference in none of these, as in a free function or a
// package-level variable, belongs to no type. A type declared inside a
// function is local to it and is not a type of the file. References in test
// code are dropped, and a type is a test type when every span of it is.
func (l *Language) collectTypes(root *sitter.Node, source []byte, functions []Function, scopes []scope, tests []span) []Type {
	if l.types == nil {
		return nil
	}
	separator := l.Separator()

	// A declaration that two patterns match, as an interface does when one
	// pattern matches every type and another only interfaces, is one span.
	byNode := map[uintptr]int{}
	names := map[uintptr]struct{}{}
	var spans []typeSpan
	ForEachMatch(l.types, root, source, func(match *sitter.QueryMatch) {
		var node, name *sitter.Node
		var kind string
		for _, capture := range match.Captures {
			switch captureName := l.types.CaptureNameForId(capture.Index); captureName {
			case nameCapture:
				name = capture.Node
			case typeCapture, abstractCapture, implCapture:
				node = capture.Node
				kind = captureName
			}
		}
		if node == nil || name == nil {
			return
		}
		names[name.ID()] = struct{}{}
		if i, ok := byNode[node.ID()]; ok {
			spans[i].abstract = spans[i].abstract || kind == abstractCapture
			return
		}
		if innermost(functions, node.StartByte(), node.EndByte()) != nil {
			return
		}
		s := typeSpan{
			span:      span{node.StartByte(), node.EndByte()},
			startLine: int(node.StartPoint().Row) + 1,
			endLine:   int(node.EndPoint().Row) + 1,
			name:      name.Content(source),
			declared:  kind != implCapture,
			abstract:  kind == abstractCapture,
		}
		if prefix, _ := enclosing(scopes, s.start, s.end); len(prefix) > 0 {
			s.name = strings.Join(prefix, separator) + separator + s.name
		}
		byNode[node.ID()] = len(spans)
		spans = append(spans, s)
	})
	sort.SliceStable(spans, func(i, j int) bool { return spans[i].start < spans[j].start })

	var types []Type
	index := map[string]int{}
	typeOf := func(name string) *Type {
		if i, ok := index[name]; ok {
			return &types[i]
		}
		index[name] = len(types)
		types = append(types, Type{Name: name, IsTest: true})
		return &types[len(types)-1]
	}
	for _, s := range spans {
		t := typeOf(s.name)
		t.IsTest = t.IsTest && inTest(tests, s.start, s.end)
		if s.declared && !t.Declared || t.StartLine == 0 {
			t.StartLine, t.EndLine = s.startLine, s.endLine
		}
		t.Declared = t.Declared || s.declared
		t.Abstract = t.Abstract || s.abstract
	}
	for i := range functions {
		if fn := &functions[i]; fn.Receiver != "" && !fn.IsTest {
			t := typeOf(fn.Receiver)
			t.IsTest = false
			if t.StartLine == 0 {
				t.StartLine, t.EndLine = fn.StartLine, fn.EndLine
			}
		}
	}

	// owner returns the type a reference at the byte range belongs to.
	owner := func(start, end uint32) *Type {
		if fn := innermost(functions, start, end); fn != nil {
			if fn.IsTest {
				return nil
			}
			if fn.Receiver != "" {
				return typeOf(fn.Receiver)
			}
			start, end = fn.startByte, fn.endByte
		}
		var found *typeSpan
		for i := range spans {
			if spans[i].start > start {
				break
			}
			if spans[i].contains(start, end) {
				found = &spans[i]
			}
		}
		if found == nil || inTest(tests, found.start, found.end) {
			return nil
		}
		return typeOf(found.name)
	}

	type reference struct {
		node     *sitter.Node
		pkg      *sitter.Node
		embedded bool
	}
	refs := map[uintptr]reference{}
	var order []uintptr
	ForEachMatch(l.references, root, source, func(match *sitter.QueryMatch) {
		var ref reference
		for _, capture := range match.Captures {
			switch l.references.CaptureNameForId(capture.Index) {
			case referenceCapture:
				ref.node = capture.Node
			case embeddedCapture:
				ref.node = capture.Node
				ref.embedded = true
			case packageCapture:
				ref.pkg = capture.Node
			}
		}
		if ref.node == nil {
			return
		}
		if _, isName := names[ref.node.ID()]; isName {
			return
		}
		// The plain pattern also matches the name inside a qualified or
		// embedded form; the more specific match wins.
		if existing, ok := refs[ref.node.ID()]; ok {
			if existing.embedded || (existing.pkg != nil && ref.pkg == nil) {
				return
			}
		} else {
			order = append(order, ref.node.ID())
		}
		refs[ref.node.ID()] = ref
	})
	seen := map[string]map[Reference]bool{}
	for _, id := range order {
		ref := refs[id]
		t := owner(ref.node.StartByte(), ref.node.EndByte())
		if t == nil {
			continue
		}
		r := Reference{Name: ref.node.Content(source), Embedded: ref.embedded}
		if ref.pkg != nil {
			r.Package = ref.pkg.Content(source)
		}
		if seen[t.Name] == nil {
			seen[t.Name] = map[Reference]bool{}
		}
		if !seen[t.Name][r] {
			seen[t.Name][r] = true
			t.References = append(t.References, r)
		}
	}
	return types
}

// innermost returns the function whose span most tightly contains the byte
// range, or nil when the range lies outside every function. Functions are
// sorted by start, so among the containing spans the last one is the
// innermost.
func innermost(functions []Function, start, end uint32) *Function {
	var found *Function
	for i := range functions {
		fn := &functions[i]
		if fn.startByte > start {
			break
		}
		if fn.endByte >= end {
			found = fn
		}
	}
	return found
}

// ForEachMatch runs query over the tree under root and calls visit with each
// match, its predicates already applied.
func ForEachMatch(query *sitter.Query, root *sitter.Node, source []byte, visit func(*sitter.QueryMatch)) {
	cursor := sitter.NewQueryCursor()
	defer cursor.Close()
	cursor.Exec(query, root)
	for {
		match, ok := cursor.NextMatch()
		if !ok {
			return
		}
		visit(cursor.FilterPredicates(match, source))
	}
}

// checkSyntax reports the first syntax problem in the tree, or nil when the
// source is valid.
func checkSyntax(root *sitter.Node) error {
	if !root.HasError() {
		return nil
	}
	if node := firstErrorNode(root); node != nil {
		return fmt.Errorf("syntax error at line %d", int(node.StartPoint().Row)+1)
	}
	return fmt.Errorf("syntax errors found in source code")
}

// firstErrorNode returns the first ERROR or MISSING node in source order,
// descending only into subtrees that carry an error.
func firstErrorNode(node *sitter.Node) *sitter.Node {
	if node.IsError() || node.IsMissing() {
		return node
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil || !child.HasError() {
			continue
		}
		if found := firstErrorNode(child); found != nil {
			return found
		}
	}
	return nil
}

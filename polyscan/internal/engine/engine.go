// Package engine runs declarative, per-language tree-sitter queries and turns
// their captures into per-function metrics. A language contributes no Go
// code beyond its Language value: which grammar to load, which query finds
// its functions, which query marks its decision points, and which node
// types matter for clone detection.
package engine

import (
	"context"
	"fmt"
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
	// name. An optional @receiver capture holds the receiver type of a
	// method and is prefixed to the name as "Receiver.Name".
	Definitions string
	// A function node that several patterns match is reported once, under
	// the match that captured a receiver when there is one, so a grammar
	// that uses one node type for functions and methods can name methods
	// through their enclosing type.
	//
	// Decisions is a tree-sitter query in which every capture is one
	// decision point. The point is attributed to the innermost function
	// that contains it and counted under the capture's name, so the capture
	// names double as the breakdown reported next to the complexity.
	Decisions string
	// Clone names the node types clone detection prices and compares.
	Clone CloneSpec
	// TestFiles are file name globs of test files, and TestCode is an
	// optional tree-sitter query whose captures span test code inside a
	// file, such as attribute-marked test functions or modules. Both are
	// analyzed for complexity but excluded from clone detection, where the
	// shared skeleton of test functions swamps the report.
	TestFiles []string
	TestCode  string

	compileOnce sync.Once
	compileErr  error
	definitions *sitter.Query
	decisions   *sitter.Query
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

	startByte uint32
	endByte   uint32
}

const (
	definitionPrefix = "definition."
	nameCapture      = "name"
	receiverCapture  = "receiver"
	operatorField    = "operator"
)

// Analyze parses source and returns every function the language defines,
// with its complexity computed. Source that does not parse is an error: a
// tree-sitter parse always yields a tree, so the syntax check is what keeps
// a broken file from being reported as if its salvageable fragments were
// the whole program.
func (l *Language) Analyze(source []byte) ([]Function, error) {
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
	if err := checkSyntax(root); err != nil {
		return nil, err
	}

	functions := l.extractFunctions(root, source)
	l.countDecisions(root, source, functions)
	l.markTests(root, source, functions)
	for i := range functions {
		functions[i].Complexity = 1
		for _, count := range functions[i].Decisions {
			functions[i].Complexity += count
		}
	}
	return functions, nil
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
	byStart := map[uint32]int{}
	forEachMatch(l.definitions, root, source, func(match *sitter.QueryMatch) {
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
			}
		}
		if node == nil || fn.Name == "" {
			return
		}
		if receiver != "" {
			fn.Name = receiver + "." + fn.Name
		}
		if index, seen := byStart[node.StartByte()]; seen {
			if receiver != "" {
				functions[index].Name = fn.Name
				functions[index].Kind = fn.Kind
			}
			return
		}
		byStart[node.StartByte()] = len(functions)
		fn.StartLine = int(node.StartPoint().Row) + 1
		fn.StartColumn = int(node.StartPoint().Column) + 1
		fn.EndLine = int(node.EndPoint().Row) + 1
		fn.EndColumn = int(node.EndPoint().Column) + 1
		fn.startByte = node.StartByte()
		fn.endByte = node.EndByte()
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
	forEachMatch(l.decisions, root, source, func(match *sitter.QueryMatch) {
		for _, capture := range match.Captures {
			fn := innermost(functions, capture.Node.StartByte(), capture.Node.EndByte())
			if fn == nil {
				continue
			}
			fn.Decisions[l.decisions.CaptureNameForId(capture.Index)]++
		}
	})
}

// markTests flags every function inside a span the TestCode query captures.
func (l *Language) markTests(root *sitter.Node, source []byte, functions []Function) {
	if l.testCode == nil {
		return
	}
	forEachMatch(l.testCode, root, source, func(match *sitter.QueryMatch) {
		for _, capture := range match.Captures {
			start, end := capture.Node.StartByte(), capture.Node.EndByte()
			for i := range functions {
				if functions[i].startByte >= start && functions[i].endByte <= end {
					functions[i].IsTest = true
				}
			}
		}
	})
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

func forEachMatch(query *sitter.Query, root *sitter.Node, source []byte, visit func(*sitter.QueryMatch)) {
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

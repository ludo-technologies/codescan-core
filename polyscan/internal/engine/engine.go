// Package engine runs declarative, per-language tree-sitter queries and turns
// their captures into per-function metrics. A language contributes no Go
// code beyond its Language value: which grammar to load, which query finds
// its functions, and which query marks its decision points.
package engine

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

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
	// Decisions is a tree-sitter query in which every capture is one
	// decision point. The point is attributed to the innermost function
	// that contains it and counted under the capture's name, so the capture
	// names double as the breakdown reported next to the complexity.
	Decisions string

	compileOnce sync.Once
	compileErr  error
	definitions *sitter.Query
	decisions   *sitter.Query
}

// Function is one function or method found in a source file.
type Function struct {
	// Name is the function name, prefixed with its receiver for methods.
	Name string
	// Kind is the suffix of the @definition capture that matched, such as
	// "function" or "method".
	Kind string
	// StartLine, StartColumn and EndLine are 1-based source positions.
	StartLine   int
	StartColumn int
	EndLine     int
	// Complexity is the McCabe cyclomatic complexity: one plus the number of
	// decision points, following the same conventions core/cfg applies to a
	// control flow graph. A two-way branch or loop counts one, a switch
	// counts one per case with the default excluded, and each short-circuit
	// operator counts one.
	Complexity int
	// Decisions breaks the decision points down by the name of the capture
	// that produced them.
	Decisions map[string]int

	startByte uint32
	endByte   uint32
}

const (
	definitionPrefix = "definition."
	nameCapture      = "name"
	receiverCapture  = "receiver"
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
		l.definitions = definitions
		l.decisions = decisions
	})
	return l.compileErr
}

func (l *Language) extractFunctions(root *sitter.Node, source []byte) []Function {
	var functions []Function
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
		fn.StartLine = int(node.StartPoint().Row) + 1
		fn.StartColumn = int(node.StartPoint().Column) + 1
		fn.EndLine = int(node.EndPoint().Row) + 1
		fn.startByte = node.StartByte()
		fn.endByte = node.EndByte()
		fn.Decisions = map[string]int{}
		functions = append(functions, fn)
	})
	sort.Slice(functions, func(i, j int) bool {
		return functions[i].startByte < functions[j].startByte
	})
	return functions
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

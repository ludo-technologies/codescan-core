package parser

import (
	"context"
	"fmt"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/javascript"
	"github.com/smacker/go-tree-sitter/typescript/tsx"
)

// Parser wraps tree-sitter parser for JavaScript/TypeScript
type Parser struct {
	parser   *sitter.Parser
	language *sitter.Language
	isTS     bool
}

// NewParser creates a new JavaScript parser
func NewParser() *Parser {
	parser := sitter.NewParser()
	lang := javascript.GetLanguage()
	parser.SetLanguage(lang)

	return &Parser{
		parser:   parser,
		language: lang,
		isTS:     false,
	}
}

// NewTypeScriptParser creates a new TypeScript parser
func NewTypeScriptParser() *Parser {
	parser := sitter.NewParser()
	lang := tsx.GetLanguage()
	parser.SetLanguage(lang)

	return &Parser{
		parser:   parser,
		language: lang,
		isTS:     true,
	}
}

// ParseFile parses a JavaScript/TypeScript file
func (p *Parser) ParseFile(filename string, source []byte) (*Node, error) {
	tree, err := p.parser.ParseCtx(context.Background(), nil, source)
	if tree == nil {
		return nil, fmt.Errorf("failed to parse file %s: %v", filename, err)
	}
	defer tree.Close()

	rootNode := tree.RootNode()
	if rootNode == nil {
		return nil, fmt.Errorf("no root node in parse tree for %s", filename)
	}

	// Reported without the filename: every caller already has the path and
	// prefixes it, so repeating it here only doubles it in the output.
	if err := checkSyntax(rootNode); err != nil {
		return nil, err
	}

	// Build our internal AST from tree-sitter CST
	builder := NewASTBuilder(filename, source)
	ast := builder.Build(rootNode)

	return ast, nil
}

// checkSyntax reports the first syntax problem in the parsed tree, or nil when
// the source is valid. tree-sitter always returns a tree: source it cannot
// parse comes back with ERROR and MISSING nodes rather than an error, so
// without this check a file no JavaScript engine would load is analyzed as if
// it were valid, and every metric silently describes only the fragments the
// grammar managed to salvage.
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

	for index := 0; index < int(node.ChildCount()); index++ {
		child := node.Child(index)
		if child == nil || !child.HasError() {
			continue
		}
		if found := firstErrorNode(child); found != nil {
			return found
		}
	}
	return nil
}

// Parse parses JavaScript/TypeScript source code
func (p *Parser) Parse(source []byte) (*Node, error) {
	return p.ParseFile("<input>", source)
}

// ParseString parses JavaScript/TypeScript source code from a string
func (p *Parser) ParseString(source string) (*Node, error) {
	return p.Parse([]byte(source))
}

// IsTypeScript returns true if this parser is configured for TypeScript
func (p *Parser) IsTypeScript() bool {
	return p.isTS
}

// Close closes the parser and frees resources
func (p *Parser) Close() {
	if p.parser != nil {
		p.parser.Close()
	}
}

// ParseForLanguage automatically selects JavaScript or TypeScript parser based on file extension
func ParseForLanguage(filename string, source []byte) (*Node, error) {
	// Determine language from file extension
	isTS := false
	if len(filename) > 3 {
		ext := filename[len(filename)-3:]
		if ext == ".ts" || ext == "tsx" {
			isTS = true
		}
	}
	if len(filename) > 4 {
		ext := filename[len(filename)-4:]
		if ext == ".tsx" || ext == ".mts" || ext == ".cts" {
			isTS = true
		}
	}

	var parser *Parser
	if isTS {
		parser = NewTypeScriptParser()
	} else {
		parser = NewParser()
	}
	defer parser.Close()

	return parser.ParseFile(filename, source)
}

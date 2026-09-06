package godeps

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/ludo-technologies/polyscan/polyscan/internal/engine"
	"github.com/ludo-technologies/polyscan/polyscan/internal/lang/golang"
	sitter "github.com/smacker/go-tree-sitter"
)

// fileInfo is what the dependency graph needs from one Go file.
type fileInfo struct {
	// Package is the name in the package clause.
	Package string
	// Imports are the import paths, in source order, with duplicates kept.
	Imports []string
	// ExportedTypes counts the exported type declarations and
	// ExportedInterfaces the ones among them that declare an interface.
	// Type aliases are not declarations of their own and are not counted.
	ExportedTypes      int
	ExportedInterfaces int
}

// query captures the package clause, every import path and every declared
// type. An interface type matches both type patterns, so the two captures are
// counted separately rather than the second subtracted from the first.
const query = `
(package_clause (package_identifier) @package)
(import_spec path: [(interpreted_string_literal) (raw_string_literal)] @path)
(type_declaration (type_spec name: (type_identifier) @type))
(type_declaration (type_spec name: (type_identifier) @interface type: (interface_type)))
`

var (
	compileOnce sync.Once
	compiled    *sitter.Query
	compileErr  error
)

func compile() (*sitter.Query, error) {
	compileOnce.Do(func() {
		compiled, compileErr = sitter.NewQuery([]byte(query), golang.Language.Grammar)
		if compileErr != nil {
			compileErr = fmt.Errorf("Go dependency query: %w", compileErr)
		}
	})
	return compiled, compileErr
}

// parseFile extracts the package clause, imports and type declarations of one
// Go source. A file without a package clause, which a syntax error at the top
// of the file can cause, is an error: nothing places it in a package.
func parseFile(source []byte) (*fileInfo, error) {
	q, err := compile()
	if err != nil {
		return nil, err
	}
	parser := sitter.NewParser()
	defer parser.Close()
	parser.SetLanguage(golang.Language.Grammar)
	tree, err := parser.ParseCtx(context.Background(), nil, source)
	if err != nil {
		return nil, err
	}
	defer tree.Close()

	info := &fileInfo{}
	engine.ForEachMatch(q, tree.RootNode(), source, func(match *sitter.QueryMatch) {
		for _, capture := range match.Captures {
			text := capture.Node.Content(source)
			switch q.CaptureNameForId(capture.Index) {
			case "package":
				if info.Package == "" {
					info.Package = text
				}
			case "path":
				info.Imports = append(info.Imports, strings.Trim(text, "\"`"))
			case "type":
				if isExported(text) {
					info.ExportedTypes++
				}
			case "interface":
				if isExported(text) {
					info.ExportedInterfaces++
				}
			}
		}
	})
	if info.Package == "" {
		return nil, fmt.Errorf("no package clause")
	}
	return info, nil
}

func isExported(name string) bool {
	r, _ := utf8.DecodeRuneInString(name)
	return unicode.IsUpper(r)
}

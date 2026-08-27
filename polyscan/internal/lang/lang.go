// Package lang registers the supported languages and resolves a file to its
// language by extension.
package lang

import (
	"path/filepath"

	"github.com/ludo-technologies/polyscan/polyscan/internal/engine"
	"github.com/ludo-technologies/polyscan/polyscan/internal/lang/golang"
)

// All lists every supported language.
var All = []*engine.Language{golang.Language}

// ByPath returns the language that owns the file's extension.
func ByPath(path string) (*engine.Language, bool) {
	ext := filepath.Ext(path)
	for _, language := range All {
		for _, candidate := range language.Extensions {
			if candidate == ext {
				return language, true
			}
		}
	}
	return nil, false
}

// IncludePatterns returns one glob per supported extension, for file
// collection.
func IncludePatterns() []string {
	var patterns []string
	for _, language := range All {
		for _, ext := range language.Extensions {
			patterns = append(patterns, "*"+ext)
		}
	}
	return patterns
}

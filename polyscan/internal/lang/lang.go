// Package lang registers the supported languages and resolves a file to its
// language by extension.
package lang

import (
	"path/filepath"

	"github.com/ludo-technologies/polyscan/polyscan/internal/engine"
	"github.com/ludo-technologies/polyscan/polyscan/internal/lang/cpp"
	"github.com/ludo-technologies/polyscan/polyscan/internal/lang/golang"
	"github.com/ludo-technologies/polyscan/polyscan/internal/lang/rust"
)

// All lists every supported language.
var All = []*engine.Language{golang.Language, rust.Language, cpp.Language}

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

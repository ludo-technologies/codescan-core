package domain

import (
	"path/filepath"
	"strings"
)

// Language labels reported on every function and clone fragment. The other
// polyscan languages report theirs the same way ("Go", "Rust", "C++"), so the
// unified report can tell every result's language apart.
const (
	LanguageJavaScript = "JavaScript"
	LanguageTypeScript = "TypeScript"
)

// LanguageForPath reports the language of an analyzed file from its
// extension. Every file that reaches an analysis is JavaScript or TypeScript,
// so anything that is not a TypeScript extension is JavaScript.
func LanguageForPath(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".ts", ".tsx", ".mts", ".cts":
		return LanguageTypeScript
	default:
		return LanguageJavaScript
	}
}

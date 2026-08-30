package analyzer

import (
	"strings"

	coreclone "github.com/ludo-technologies/polyscan/core/clone"
)

// removeJSComments strips JavaScript comments while preserving quoted content.
func removeJSComments(content string) string {
	var b strings.Builder
	b.Grow(len(content))
	const (
		code = iota
		lineComment
		blockComment
		quoted
	)
	state := code
	var quote byte
	escaped := false
	for i := 0; i < len(content); i++ {
		ch := content[i]
		switch state {
		case code:
			if ch == '/' && i+1 < len(content) && content[i+1] == '/' {
				state = lineComment
				i++
				continue
			}
			if ch == '/' && i+1 < len(content) && content[i+1] == '*' {
				state = blockComment
				i++
				continue
			}
			if ch == '\'' || ch == '"' || ch == '`' {
				state, quote = quoted, ch
			}
			b.WriteByte(ch)
		case lineComment:
			if ch == '\n' {
				state = code
				b.WriteByte(ch)
			}
		case blockComment:
			if ch == '*' && i+1 < len(content) && content[i+1] == '/' {
				state = code
				i++
			}
		case quoted:
			b.WriteByte(ch)
			if escaped {
				escaped = false
			} else if ch == '\\' {
				escaped = true
			} else if ch == quote {
				state = code
			}
		}
	}
	return b.String()
}

var fragmentHashNormalizer = coreclone.NewTextualSimilarityAnalyzer(removeJSComments)

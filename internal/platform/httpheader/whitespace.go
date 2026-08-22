package httpheader

import "unicode"

// HasNonHTTPWhitespace reports Unicode whitespace that HTTP field grammars do
// not treat as SP or HTAB. Whitespace inside a quoted string remains field
// content and is not classified as syntax whitespace.
func HasNonHTTPWhitespace(value string) bool {
	quoted, escaped := false, false
	for _, character := range value {
		switch {
		case escaped:
			escaped = false
		case quoted && character == '\\':
			escaped = true
		case character == '"':
			quoted = !quoted
		case !quoted && character != ' ' && character != '\t' && unicode.IsSpace(character):
			return true
		}
	}
	return false
}

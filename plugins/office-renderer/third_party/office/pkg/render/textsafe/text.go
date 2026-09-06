// Package textsafe provides the final visible-text boundary shared by Office
// renderers. Format parsers are still responsible for converting meaningful
// controls into semantic events; this package guarantees that unresolved C0
// syntax cannot leak into HTML as visible replacement glyphs.
package textsafe

import "strings"

// Visible removes unresolved C0 controls from visible text while preserving
// whitespace that is valid textual content across Office families. Newline and
// carriage return are intentionally preserved for spreadsheet/presentation
// rich text; Word converts them to paragraph/break events before rendering.
func Visible(text string) string {
	if text == "" {
		return ""
	}
	var out strings.Builder
	out.Grow(len(text))
	for _, r := range text {
		if r < 0x20 {
			switch r {
			case '\t', '\n', '\r':
				out.WriteRune(r)
			default:
				continue
			}
			continue
		}
		if r == 0x7f {
			continue
		}
		out.WriteRune(r)
	}
	return out.String()
}

package word97

import "strings"

// controlTextCursor is the final textual normalization stage for Word97 runs.
// Semantic decorators (fields, drawings, references, bookmarks and sections)
// must run before it so their control markers have the first chance to become
// typed events. Anything left here is textual Word control syntax and must not
// leak into Unicode/HTML as a replacement-square glyph.
type controlTextCursor struct {
	inner Cursor
}

func NewControlTextCursor(inner Cursor) Cursor {
	if inner == nil {
		return nil
	}
	return &controlTextCursor{inner: inner}
}

func (c *controlTextCursor) Next() (Run, bool, error) {
	for {
		run, ok, err := c.inner.Next()
		if err != nil || !ok {
			return run, ok, err
		}
		if run.Kind != "" || run.Text == "" {
			return run, true, nil
		}
		run.Text = normalizeWordControlText(run.Text)
		if run.Text == "" {
			continue
		}
		return run, true, nil
	}
}

func normalizeWordControlText(text string) string {
	if text == "" {
		return ""
	}
	var out strings.Builder
	out.Grow(len(text))
	for _, ch := range text {
		switch ch {
		case '\n':
			// Some legacy producers use LF for a manual line break. Word's
			// canonical manual-break character is VT, already handled by Walk.
			out.WriteByte('\v')
		case '\x0e':
			// Preserve the existing binary compatibility convention: 0x0e is
			// promoted to a form-feed so the break pipeline paginates it.
			out.WriteByte('\f')
		case '\x1e':
			// Word nonbreaking hyphen.
			out.WriteRune('\u2011')
		case '\x1f':
			// Word optional hyphen.
			out.WriteRune('\u00ad')
		case '\x00', '\x01', '\x02', '\x03', '\x04', '\x05', '\x06', '\x08':
			// Post-semantic object/annotation markers. If they survived the
			// typed decorators they have no textual representation.
			continue
		default:
			// Keep Word controls with explicit downstream semantics: cell mark
			// (0x07), tab, VT, FF, CR and field begin/separator/end (0x13-15).
			// Visible Unicode is passed through unchanged and escaped by HTML.
			out.WriteRune(ch)
		}
	}
	return out.String()
}

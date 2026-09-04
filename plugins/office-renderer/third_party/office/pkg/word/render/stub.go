package render

import (
	"fmt"
	"strings"

	resolve "office/pkg/word/ir"
)

// MathMLOptions controls MathML rendering behaviour.
type MathMLOptions struct {
	// InlineCSS embeds minimal CSS for basic math layout when true.
	// When false (default), MathML is returned as raw XML.
	InlineCSS bool
	// WrapHTML returns the MathML wrapped in a minimal HTML document shell
	// when true. When false (default), only the concatenated MathML is
	// returned.
	WrapHTML bool
	// Title is used as the HTML document title when WrapHTML is true.
	Title string
}

// MathML renders the document's MathML content with a best-effort layout
// approximation. It collects math runs from the body and all referenced
// stories (headers, footers, footnotes, endnotes, and comments) and, when
// requested, wraps them in a minimal HTML document with MathML-friendly CSS.
func MathML(doc *resolve.Document, opts MathMLOptions) (string, error) {
	maths := collectMathMLRuns(doc)
	if len(maths) == 0 {
		if opts.WrapHTML {
			return wrapMathMLHTML("", opts), nil
		}
		return "", nil
	}

	var sb strings.Builder
	for _, m := range maths {
		if isDisplayMath(m) {
			sb.WriteString(`<div class="math-display">`)
			sb.WriteString(m)
			sb.WriteString("</div>\n")
		} else {
			sb.WriteString("<span>")
			sb.WriteString(m)
			sb.WriteString("</span>\n")
		}
	}

	if opts.WrapHTML {
		return wrapMathMLHTML(sb.String(), opts), nil
	}
	return sb.String(), nil
}

// collectMathMLRuns walks the document body and all stories, returning the
// raw MathML content of every math run in document order.
func collectMathMLRuns(doc *resolve.Document) []string {
	if doc == nil {
		return nil
	}
	var out []string
	out = collectMathMLFromBlocks(out, doc.Blocks)
	for _, story := range doc.Headers {
		out = collectMathMLFromBlocks(out, story.Blocks)
	}
	for _, story := range doc.Footers {
		out = collectMathMLFromBlocks(out, story.Blocks)
	}
	for _, story := range doc.Footnotes {
		out = collectMathMLFromBlocks(out, story.Blocks)
	}
	for _, story := range doc.Endnotes {
		out = collectMathMLFromBlocks(out, story.Blocks)
	}
	for _, story := range doc.Comments {
		out = collectMathMLFromBlocks(out, story.Blocks)
	}
	return out
}

// collectMathMLFromBlocks appends math-run MathML found in blocks to out.
func collectMathMLFromBlocks(out []string, blocks []resolve.Block) []string {
	for _, b := range blocks {
		switch b.Kind {
		case "paragraph":
			if b.Paragraph == nil {
				continue
			}
			for _, r := range b.Paragraph.Runs {
				if r.Kind == "math" && r.MathML != "" {
					out = append(out, r.MathML)
				}
			}
		case "table":
			if b.Table == nil {
				continue
			}
			for _, row := range b.Table.Rows {
				for _, cell := range row.Cells {
					out = collectMathMLFromBlocks(out, cell.Blocks)
				}
			}
		}
	}
	return out
}

// isDisplayMath reports whether a raw MathML fragment should be treated as
// display (block) math rather than inline math.
func isDisplayMath(m string) bool {
	trimmed := strings.TrimSpace(m)
	if strings.HasPrefix(trimmed, "<m:display>") {
		return true
	}
	if strings.Contains(trimmed, `display="block"`) {
		return true
	}
	return false
}

// wrapMathMLHTML wraps the supplied math markup in a minimal HTML document.
func wrapMathMLHTML(mathMarkup string, opts MathMLOptions) string {
	title := opts.Title
	if title == "" {
		title = "MathML"
	}
	var css strings.Builder
	if opts.InlineCSS {
		css.WriteString(`<style>
	math { font-family: "Cambria Math", "Latin Modern Math", "STIX Two Math", serif; }
	.math-display { display: block; text-align: center; margin: 1em 0; }
	body { font-family: sans-serif; margin: 1em; }
</style>`)
	}
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
	<meta charset="UTF-8">
	<title>%s</title>
%s
</head>
<body>
%s
</body>
</html>
`, htmlEscape(title), css.String(), mathMarkup)
}

// htmlEscape escapes a value for use in HTML text/attributes.
func htmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}

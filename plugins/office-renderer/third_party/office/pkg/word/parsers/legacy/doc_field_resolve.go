package legacy

import (
	"strings"

	wordir "office/pkg/word/ir"
)

// resolveDocCrossReferenceFields performs a post-processing pass on a fully
// parsed legacy document to complete bookmark-aware REF and STYLEREF linkage:
//
//  1. REF fields: clears Anchor when the referenced bookmark name is not
//     present in bookmarkStarts, preventing broken intra-document links
//     when field instructions reference bookmarks that do not exist.
//
//  2. STYLEREF fields: substitutes the run Text with the plain text of the
//     first body paragraph carrying the referenced style name, making the
//     resolved value available to non-HTML output paths (the HTML renderer
//     computes this independently via resolveStyleRef).
func resolveDocCrossReferenceFields(doc *wordir.Document, bookmarkStarts map[int][]string) {
	if doc == nil {
		return
	}

	// Build the set of known bookmark names from the loaded PlcfBkf table.
	knownBookmarks := make(map[string]bool, len(bookmarkStarts))
	for _, names := range bookmarkStarts {
		for _, n := range names {
			knownBookmarks[n] = true
		}
	}

	// Collect the first non-empty paragraph text for each style name found in
	// the main document body (top-level paragraphs, then tables in order).
	styleFirstText := collectStyleFirstText(doc.Paragraphs, doc.Tables)

	// Apply REF/STYLEREF fixups across all run-bearing containers.
	applyDocRefFixups(doc.Paragraphs, doc.Tables, knownBookmarks, styleFirstText)

	for key, story := range doc.Headers {
		applyDocRefFixups(story.Paragraphs, story.Tables, knownBookmarks, styleFirstText)
		doc.Headers[key] = story
	}
	for key, story := range doc.Footers {
		applyDocRefFixups(story.Paragraphs, story.Tables, knownBookmarks, styleFirstText)
		doc.Footers[key] = story
	}
	for key, story := range doc.Footnotes {
		applyDocRefFixups(story.Paragraphs, story.Tables, knownBookmarks, styleFirstText)
		doc.Footnotes[key] = story
	}
	for key, story := range doc.Endnotes {
		applyDocRefFixups(story.Paragraphs, story.Tables, knownBookmarks, styleFirstText)
		doc.Endnotes[key] = story
	}
}

// collectStyleFirstText returns a map from lowercase style name to the plain
// text of the first paragraph carrying that style found during a depth-first
// walk of the supplied paragraphs and tables.
func collectStyleFirstText(paras []wordir.Paragraph, tables []wordir.Table) map[string]string {
	result := map[string]string{}
	var walk func([]wordir.Paragraph, []wordir.Table)
	walk = func(paras []wordir.Paragraph, tables []wordir.Table) {
		for _, p := range paras {
			if name := strings.TrimSpace(p.StyleName); name != "" {
				lname := strings.ToLower(name)
				if _, seen := result[lname]; !seen {
					var sb strings.Builder
					for _, r := range p.Runs {
						if r.Revision != "deleted" {
							sb.WriteString(r.Text)
						}
					}
					if t := strings.TrimSpace(sb.String()); t != "" {
						result[lname] = t
					}
				}
			}
		}
		for _, tbl := range tables {
			for _, row := range tbl.Rows {
				for _, cell := range row.Cells {
					walk(cell.Paragraphs, cell.Tables)
				}
			}
		}
	}
	walk(paras, tables)
	return result
}

// applyDocRefFixups walks paragraphs and tables recursively, modifying REF and
// STYLEREF field runs in-place.
func applyDocRefFixups(paras []wordir.Paragraph, tables []wordir.Table, knownBookmarks map[string]bool, styleFirstText map[string]string) {
	for i := range paras {
		for j := range paras[i].Runs {
			r := &paras[i].Runs[j]
			switch r.FieldKind {
			case "REF":
				// Clear the anchor when the bookmark is not present in the
				// document; this prevents the renderer from emitting a broken link.
				if r.Anchor != "" && !knownBookmarks[r.Anchor] {
					r.Anchor = ""
				}
			case "STYLEREF":
				// Substitute display text with the first matching paragraph body
				// text so non-HTML output paths get the resolved value.
				if r.FieldArg != "" {
					if text, ok := styleFirstText[strings.ToLower(r.FieldArg)]; ok {
						r.Text = text
					}
				}
			}
		}
	}
	for i := range tables {
		for ri := range tables[i].Rows {
			for ci := range tables[i].Rows[ri].Cells {
				cell := &tables[i].Rows[ri].Cells[ci]
				applyDocRefFixups(cell.Paragraphs, cell.Tables, knownBookmarks, styleFirstText)
			}
		}
	}
}
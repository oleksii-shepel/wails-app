package word97

import (
	"strings"

	wordir "office/pkg/word/ir"
)

// ApplyDocumentPostProjection completes document-level semantics that require a
// fully assembled Word IR. The binary state used to reach this point is already
// canonical; this pass owns the remaining Word97 document-wide compatibility
// behavior rather than routing it back through parsers/legacy.
func ApplyDocumentPostProjection(doc *wordir.Document, bookmarkStarts map[int][]string) {
	if doc == nil {
		return
	}
	applyDocumentGridSemantics(doc)
	resolveCrossReferenceFields(doc, bookmarkStarts)
}

func applyDocumentGridSemantics(doc *wordir.Document) {
	for _, sec := range doc.Sections {
		if sec.DocGrid == nil || sec.DocGrid.Type == "" || sec.DocGrid.Type == "default" {
			continue
		}
		for i := range doc.Paragraphs {
			doc.Paragraphs[i].Format.SnapToGrid = true
		}
		return
	}
}

// resolveCrossReferenceFields preserves the mature DOC behavior for REF and
// STYLEREF after the complete document has been assembled.
//
// REF anchors that do not exist in PlcfBkf are cleared so renderers do not emit
// broken intra-document links. STYLEREF display text is resolved from the first
// non-empty body paragraph carrying the requested style.
func resolveCrossReferenceFields(doc *wordir.Document, bookmarkStarts map[int][]string) {
	knownBookmarks := make(map[string]bool, len(bookmarkStarts))
	for _, names := range bookmarkStarts {
		for _, name := range names {
			knownBookmarks[name] = true
		}
	}

	styleFirstText := collectStyleFirstText(doc.Paragraphs, doc.Tables)
	applyReferenceFixups(doc.Paragraphs, doc.Tables, knownBookmarks, styleFirstText)

	for key, story := range doc.Headers {
		applyReferenceFixups(story.Paragraphs, story.Tables, knownBookmarks, styleFirstText)
		doc.Headers[key] = story
	}
	for key, story := range doc.Footers {
		applyReferenceFixups(story.Paragraphs, story.Tables, knownBookmarks, styleFirstText)
		doc.Footers[key] = story
	}
	for key, story := range doc.Footnotes {
		applyReferenceFixups(story.Paragraphs, story.Tables, knownBookmarks, styleFirstText)
		doc.Footnotes[key] = story
	}
	for key, story := range doc.Endnotes {
		applyReferenceFixups(story.Paragraphs, story.Tables, knownBookmarks, styleFirstText)
		doc.Endnotes[key] = story
	}
}

func collectStyleFirstText(paragraphs []wordir.Paragraph, tables []wordir.Table) map[string]string {
	result := map[string]string{}
	var walk func([]wordir.Paragraph, []wordir.Table)
	walk = func(paragraphs []wordir.Paragraph, tables []wordir.Table) {
		for _, paragraph := range paragraphs {
			name := strings.TrimSpace(paragraph.StyleName)
			if name == "" {
				continue
			}
			key := strings.ToLower(name)
			if _, seen := result[key]; seen {
				continue
			}
			var text strings.Builder
			for _, run := range paragraph.Runs {
				if run.Revision != "deleted" {
					text.WriteString(run.Text)
				}
			}
			if value := strings.TrimSpace(text.String()); value != "" {
				result[key] = value
			}
		}
		for _, table := range tables {
			for _, row := range table.Rows {
				for _, cell := range row.Cells {
					walk(cell.Paragraphs, cell.Tables)
				}
			}
		}
	}
	walk(paragraphs, tables)
	return result
}

func applyReferenceFixups(
	paragraphs []wordir.Paragraph,
	tables []wordir.Table,
	knownBookmarks map[string]bool,
	styleFirstText map[string]string,
) {
	for i := range paragraphs {
		for j := range paragraphs[i].Runs {
			run := &paragraphs[i].Runs[j]
			switch run.FieldKind {
			case "REF":
				if run.Anchor != "" && !knownBookmarks[run.Anchor] {
					run.Anchor = ""
				}
			case "STYLEREF":
				if run.FieldArg == "" {
					continue
				}
				if text, ok := styleFirstText[strings.ToLower(run.FieldArg)]; ok {
					run.Text = text
				}
			}
		}
	}
	for i := range tables {
		for ri := range tables[i].Rows {
			for ci := range tables[i].Rows[ri].Cells {
				cell := &tables[i].Rows[ri].Cells[ci]
				applyReferenceFixups(cell.Paragraphs, cell.Tables, knownBookmarks, styleFirstText)
			}
		}
	}
}

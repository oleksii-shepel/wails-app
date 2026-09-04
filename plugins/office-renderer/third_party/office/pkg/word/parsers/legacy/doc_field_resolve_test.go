package legacy

import (
	"testing"

	wordir "office/pkg/word/ir"
)

// TestResolveDocCrossReferenceFieldsClearsUnknownRefAnchor verifies that a REF
// field whose bookmark name does not appear in bookmarkStarts gets its Anchor
// cleared so the renderer does not emit a broken intra-document link.
func TestResolveDocCrossReferenceFieldsClearsUnknownRefAnchor(t *testing.T) {
	doc := &wordir.Document{
		Paragraphs: []wordir.Paragraph{
			{
				Runs: []wordir.Run{
					{Text: "see here", FieldKind: "REF", Anchor: "MissingBookmark"},
				},
			},
		},
	}
	// bookmarkStarts contains a different bookmark, not "MissingBookmark".
	bookmarkStarts := map[int][]string{0: {"OtherBookmark"}}

	resolveDocCrossReferenceFields(doc, bookmarkStarts)

	run := doc.Paragraphs[0].Runs[0]
	if run.Anchor != "" {
		t.Errorf("Anchor = %q, want empty (bookmark not in bookmarkStarts)", run.Anchor)
	}
	if run.FieldKind != "REF" {
		t.Errorf("FieldKind = %q, want REF", run.FieldKind)
	}
	if run.Text != "see here" {
		t.Errorf("Text = %q, want %q (text should be preserved)", run.Text, "see here")
	}
}

// TestResolveDocCrossReferenceFieldsKeepsKnownRefAnchor verifies that a REF
// field whose bookmark name IS present in bookmarkStarts keeps its Anchor.
func TestResolveDocCrossReferenceFieldsKeepsKnownRefAnchor(t *testing.T) {
	doc := &wordir.Document{
		Paragraphs: []wordir.Paragraph{
			{
				Runs: []wordir.Run{
					{Text: "see here", FieldKind: "REF", Anchor: "ExistingBookmark"},
				},
			},
		},
	}
	bookmarkStarts := map[int][]string{5: {"ExistingBookmark"}}

	resolveDocCrossReferenceFields(doc, bookmarkStarts)

	run := doc.Paragraphs[0].Runs[0]
	if run.Anchor != "ExistingBookmark" {
		t.Errorf("Anchor = %q, want ExistingBookmark", run.Anchor)
	}
}

// TestResolveDocCrossReferenceFieldsSubstitutesStylerefText verifies that a
// STYLEREF field gets its display text replaced with the text of the first
// main-body paragraph carrying the referenced style name.
func TestResolveDocCrossReferenceFieldsSubstitutesStylerefText(t *testing.T) {
	doc := &wordir.Document{
		Paragraphs: []wordir.Paragraph{
			// First paragraph: carries "Heading 1" style with body text.
			{
				StyleName: "Heading 1",
				Runs:      []wordir.Run{{Text: "Chapter One"}},
			},
			// Second paragraph: STYLEREF field targeting "Heading 1".
			{
				Runs: []wordir.Run{
					{Text: "stale cached text", FieldKind: "STYLEREF", FieldArg: "Heading 1"},
				},
			},
		},
	}

	resolveDocCrossReferenceFields(doc, nil)

	run := doc.Paragraphs[1].Runs[0]
	if run.Text != "Chapter One" {
		t.Errorf("STYLEREF run Text = %q, want %q", run.Text, "Chapter One")
	}
	if run.FieldKind != "STYLEREF" {
		t.Errorf("FieldKind = %q, want STYLEREF", run.FieldKind)
	}
}

// TestResolveDocCrossReferenceFieldsStylerefCaseInsensitive verifies that
// style name matching is case-insensitive (e.g. "heading 1" matches "Heading 1").
func TestResolveDocCrossReferenceFieldsStylerefCaseInsensitive(t *testing.T) {
	doc := &wordir.Document{
		Paragraphs: []wordir.Paragraph{
			{
				StyleName: "Heading 1",
				Runs:      []wordir.Run{{Text: "Introduction"}},
			},
			{
				Runs: []wordir.Run{
					{Text: "old", FieldKind: "STYLEREF", FieldArg: "heading 1"},
				},
			},
		},
	}

	resolveDocCrossReferenceFields(doc, nil)

	if got := doc.Paragraphs[1].Runs[0].Text; got != "Introduction" {
		t.Errorf("STYLEREF Text = %q, want %q", got, "Introduction")
	}
}

// TestResolveDocCrossReferenceFieldsStylerefNoMatchPreservesText verifies that
// when no paragraph carries the referenced style the original display text is
// left unchanged.
func TestResolveDocCrossReferenceFieldsStylerefNoMatchPreservesText(t *testing.T) {
	doc := &wordir.Document{
		Paragraphs: []wordir.Paragraph{
			{
				StyleName: "Normal",
				Runs:      []wordir.Run{{Text: "body text"}},
			},
			{
				Runs: []wordir.Run{
					{Text: "cached value", FieldKind: "STYLEREF", FieldArg: "Heading 1"},
				},
			},
		},
	}

	resolveDocCrossReferenceFields(doc, nil)

	if got := doc.Paragraphs[1].Runs[0].Text; got != "cached value" {
		t.Errorf("STYLEREF Text = %q, want %q (no match should preserve original)", got, "cached value")
	}
}

// TestResolveDocCrossReferenceFieldsAppliesToHeadersAndFooters verifies that
// the resolve pass also fixes REF/STYLEREF runs inside header and footer stories.
func TestResolveDocCrossReferenceFieldsAppliesToHeadersAndFooters(t *testing.T) {
	doc := &wordir.Document{
		Paragraphs: []wordir.Paragraph{
			{
				StyleName: "Heading 1",
				Runs:      []wordir.Run{{Text: "First Heading"}},
			},
		},
		Headers: map[string]wordir.Story{
			"default": {
				Paragraphs: []wordir.Paragraph{
					{
						Runs: []wordir.Run{
							// REF pointing at a bookmark that doesn't exist.
							{Text: "ref text", FieldKind: "REF", Anchor: "Ghost"},
							// STYLEREF that should pick up "First Heading".
							{Text: "stale", FieldKind: "STYLEREF", FieldArg: "Heading 1"},
						},
					},
				},
			},
		},
	}

	resolveDocCrossReferenceFields(doc, map[int][]string{})

	hdrPara := doc.Headers["default"].Paragraphs[0]
	if hdrPara.Runs[0].Anchor != "" {
		t.Errorf("header REF Anchor = %q, want empty (bookmark not in table)", hdrPara.Runs[0].Anchor)
	}
	if hdrPara.Runs[1].Text != "First Heading" {
		t.Errorf("header STYLEREF Text = %q, want %q", hdrPara.Runs[1].Text, "First Heading")
	}
}
package legacytoooxml

import (
	"strings"
	"testing"

	"office/pkg/ooxml/chart"
	dml "office/pkg/ooxml/drawing"
	wordir "office/pkg/word/ir"
	wordparts "office/pkg/word/parsers/ooxml/parts"
)

func TestConvertRunPropertiesWritesCharacterStyleID(t *testing.T) {
	rp := (&converter{}).convertRunProperties(&wordir.Run{StyleID: "Emphasis"})
	if rp == nil || rp.RunStyle == nil || rp.RunStyle.Val != "Emphasis" {
		t.Fatalf("run properties=%+v, want rStyle Emphasis", rp)
	}
}

func TestConvertNilDocument(t *testing.T) {
	_, err := Convert(nil)
	if err == nil {
		t.Fatal("expected error for nil document")
	}
}

func TestConvertEmptyDocument(t *testing.T) {
	doc := &wordir.Document{}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pkg == nil {
		t.Fatal("expected non-nil package")
	}
	if pkg.Document == nil {
		t.Fatal("expected non-nil document part")
	}
}

func TestConvertParagraphs(t *testing.T) {
	doc := &wordir.Document{
		Paragraphs: []wordir.Paragraph{
			{
				Index:   0,
				StyleID: "Normal",
				Format: wordir.ParagraphFormat{
					Alignment: "center",
				},
				Runs: []wordir.Run{
					{Text: "Hello ", Format: wordir.RunFormat{Bold: boolPtr(true)}},
					{Text: "World", Format: wordir.RunFormat{Italic: boolPtr(true)}},
				},
			},
		},
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pkg.Document.Body.Paragraphs) != 1 {
		t.Fatalf("expected 1 paragraph, got %d", len(pkg.Document.Body.Paragraphs))
	}
	p := pkg.Document.Body.Paragraphs[0]
	if p.Properties == nil || p.Properties.Justification == nil || p.Properties.Justification.Val != "center" {
		t.Fatal("expected center alignment")
	}
	if len(p.Runs) != 2 {
		t.Fatalf("expected 2 runs, got %d", len(p.Runs))
	}
	if p.Runs[0].Properties == nil || p.Runs[0].Properties.Bold == nil {
		t.Fatal("expected bold on first run")
	}
	if p.Runs[1].Properties == nil || p.Runs[1].Properties.Italic == nil {
		t.Fatal("expected italic on second run")
	}
}

func TestConvertTables(t *testing.T) {
	widthBefore := 120
	widthAfter := 240
	doc := &wordir.Document{
		Tables: []wordir.Table{
			{
				Index:            0,
				PropertyRevision: &wordir.RevisionMark{Author: "TableAuthor", Date: "2026-05-08"},
				Format: wordir.TableFormat{
					Alignment:     "center",
					CellSpacing:   100,
					CellPadTop:    20,
					CellPadBottom: 40,
					CellPadLeft:   60,
					CellPadRight:  80,
				},
				Rows: []wordir.TableRow{
					{
						PropertyRevision: &wordir.RevisionMark{Author: "RowAuthor", Date: "2026-05-09"},
						Format: wordir.TableRowFormat{
							WidthBefore:     &widthBefore,
							WidthBeforeType: "dxa",
							WidthAfter:      &widthAfter,
							WidthAfterType:  "dxa",
						},
						Cells: []wordir.TableCell{
							{
								PropertyRevision: &wordir.RevisionMark{Author: "CellAuthor", Date: "2026-05-10"},
								Paragraphs: []wordir.Paragraph{
									{Runs: []wordir.Run{{Text: "Cell1"}}},
								},
							},
						},
					},
				},
			},
		},
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pkg.Document.Body.Tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(pkg.Document.Body.Tables))
	}
	tbl := pkg.Document.Body.Tables[0]
	if tbl.Properties == nil || tbl.Properties.Alignment == nil || tbl.Properties.Alignment.Val != "center" {
		t.Fatal("expected center table alignment")
	}
	if tbl.Properties.PropertyChange == nil || tbl.Properties.PropertyChange.Author != "TableAuthor" || tbl.Properties.PropertyChange.Date != "2026-05-08" {
		t.Fatalf("table PropertyChange = %#v, want TableAuthor/2026-05-08", tbl.Properties.PropertyChange)
	}
	if tbl.Properties.CellSpacing == nil || tbl.Properties.CellSpacing.Width != 100 || tbl.Properties.CellSpacing.Type != "dxa" {
		t.Fatalf("table cell spacing = %#v, want 100 dxa", tbl.Properties.CellSpacing)
	}
	if tbl.Properties.CellMargin == nil || tbl.Properties.CellMargin.Top == nil || tbl.Properties.CellMargin.Top.Width != 20 {
		t.Fatalf("table top cell margin = %#v, want 20", tbl.Properties.CellMargin)
	}
	if tbl.Properties.CellMargin.Bottom == nil || tbl.Properties.CellMargin.Bottom.Width != 40 {
		t.Fatalf("table bottom cell margin = %#v, want 40", tbl.Properties.CellMargin)
	}
	if tbl.Properties.CellMargin.Left == nil || tbl.Properties.CellMargin.Left.Width != 60 {
		t.Fatalf("table left cell margin = %#v, want 60", tbl.Properties.CellMargin)
	}
	if tbl.Properties.CellMargin.Right == nil || tbl.Properties.CellMargin.Right.Width != 80 {
		t.Fatalf("table right cell margin = %#v, want 80", tbl.Properties.CellMargin)
	}
	if len(tbl.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(tbl.Rows))
	}
	if tbl.Rows[0].Properties == nil || tbl.Rows[0].Properties.WidthBefore == nil || tbl.Rows[0].Properties.WidthBefore.Width != 120 {
		t.Fatalf("row WidthBefore = %#v, want 120", tbl.Rows[0].Properties)
	}
	if tbl.Rows[0].Properties.WidthAfter == nil || tbl.Rows[0].Properties.WidthAfter.Width != 240 {
		t.Fatalf("row WidthAfter = %#v, want 240", tbl.Rows[0].Properties)
	}
	if tbl.Rows[0].Properties.PropertyChange == nil || tbl.Rows[0].Properties.PropertyChange.Author != "RowAuthor" || tbl.Rows[0].Properties.PropertyChange.Date != "2026-05-09" {
		t.Fatalf("row PropertyChange = %#v, want RowAuthor/2026-05-09", tbl.Rows[0].Properties.PropertyChange)
	}
	if len(tbl.Rows[0].Cells) != 1 || tbl.Rows[0].Cells[0].Properties == nil || tbl.Rows[0].Cells[0].Properties.PropertyChange == nil || tbl.Rows[0].Cells[0].Properties.PropertyChange.Author != "CellAuthor" || tbl.Rows[0].Cells[0].Properties.PropertyChange.Date != "2026-05-10" {
		t.Fatalf("cell PropertyChange = %#v, want CellAuthor/2026-05-10", tbl.Rows[0].Cells)
	}
}

func TestConvertCoreProperties(t *testing.T) {
	doc := &wordir.Document{
		Title:       "Test",
		Author:      "Author",
		Description: "Desc",
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pkg.CoreProperties == nil {
		t.Fatal("expected core properties")
	}
	if pkg.CoreProperties.Title != "Test" {
		t.Fatalf("expected title Test, got %q", pkg.CoreProperties.Title)
	}
	if pkg.CoreProperties.Creator != "Author" {
		t.Fatalf("expected creator Author, got %q", pkg.CoreProperties.Creator)
	}
}

func TestConvertStyles(t *testing.T) {
	doc := &wordir.Document{
		Styles: map[string]wordir.Style{
			"Normal": {
				ID:      "Normal",
				Name:    "Normal",
				Default: true,
				Format: wordir.ParagraphFormat{
					Alignment: "left",
				},
				RunFormat: wordir.RunFormat{
					FontASCII: "Arial",
				},
			},
		},
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pkg.Styles == nil {
		t.Fatal("expected styles part")
	}
	if len(pkg.Styles.Styles) != 1 {
		t.Fatalf("expected 1 style, got %d", len(pkg.Styles.Styles))
	}
}

func TestConvertFootnotes(t *testing.T) {
	doc := &wordir.Document{
		Footnotes: map[int]wordir.Story{
			1: {
				Paragraphs: []wordir.Paragraph{
					{Runs: []wordir.Run{{Text: "Footnote text"}}},
				},
			},
		},
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pkg.Footnotes == nil {
		t.Fatal("expected footnotes part")
	}
	if len(pkg.Footnotes.Notes) != 1 {
		t.Fatalf("expected 1 footnote, got %d", len(pkg.Footnotes.Notes))
	}
}

func TestConvertComments(t *testing.T) {
	doc := &wordir.Document{
		Comments: map[int]wordir.Story{
			0: {
				Author: "Reviewer",
				Date:   "2026-01-01",
				Paragraphs: []wordir.Paragraph{
					{Runs: []wordir.Run{{Text: "Comment text"}}},
				},
			},
		},
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pkg.Comments == nil {
		t.Fatal("expected comments part")
	}
	if len(pkg.Comments.Comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(pkg.Comments.Comments))
	}
	if pkg.Comments.Comments[0].Author != "Reviewer" {
		t.Fatalf("expected author Reviewer, got %q", pkg.Comments.Comments[0].Author)
	}
}

func TestConvertSectionMapsPageNumberFormatAndStart(t *testing.T) {
	start := 7
	doc := &wordir.Document{
		Sections: []wordir.Section{
			{
				PageNumberFormat: "upperRoman",
				PageNumberStart:  &start,
			},
		},
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pkg.Document == nil || pkg.Document.Body == nil || pkg.Document.Body.Section == nil {
		t.Fatal("expected document section properties")
	}
	if pkg.Document.Body.Section.PageNumType == nil {
		t.Fatal("expected w:pgNumType to be written")
	}
	if got := pkg.Document.Body.Section.PageNumType.Format; got != "upperRoman" {
		t.Fatalf("PageNumType.Format = %q, want upperRoman", got)
	}
	if got := pkg.Document.Body.Section.PageNumType.Start; got != 7 {
		t.Fatalf("PageNumType.Start = %d, want 7", got)
	}
}

func TestConvertRunMapsNoProof(t *testing.T) {
	noProof := true
	doc := &wordir.Document{
		Paragraphs: []wordir.Paragraph{
			{
				Runs: []wordir.Run{
					{
						Text:   "proofing off",
						Format: wordir.RunFormat{NoProof: &noProof},
					},
				},
			},
		},
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pkg.Document == nil || pkg.Document.Body == nil || len(pkg.Document.Body.Paragraphs) != 1 {
		t.Fatal("expected one paragraph")
	}
	para := pkg.Document.Body.Paragraphs[0]
	if len(para.Runs) != 1 {
		t.Fatal("expected one paragraph run")
	}
	if para.Runs[0].Properties == nil || para.Runs[0].Properties.NoProof == nil || para.Runs[0].Properties.NoProof.Val == nil || !bool(*para.Runs[0].Properties.NoProof.Val) {
		t.Fatalf("run.Properties.NoProof = %#v, want true", para.Runs[0].Properties)
	}
}

func TestConvertSettingsMapsFootnoteAndEndnoteProperties(t *testing.T) {
	doc := &wordir.Document{
		FootnoteNumberFormat: "upperLetter",
		FootnoteNumberStart:  3,
		FootnoteRestart:      "eachPage",
		EndnotePosition:      "sectEnd",
		EndnoteNumberFormat:  "lowerRoman",
		EndnoteNumberStart:   5,
		EndnoteRestart:       "eachSect",
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pkg.Settings == nil {
		t.Fatal("expected settings part")
	}
	if pkg.Settings.FootnoteProps == nil {
		t.Fatal("expected footnote settings")
	}
	if pkg.Settings.FootnoteProps.Format == nil || pkg.Settings.FootnoteProps.Format.Val != "upperLetter" {
		t.Fatalf("footnote format = %#v, want upperLetter", pkg.Settings.FootnoteProps.Format)
	}
	if pkg.Settings.FootnoteProps.Start == nil || pkg.Settings.FootnoteProps.Start.Val != 3 {
		t.Fatalf("footnote start = %#v, want 3", pkg.Settings.FootnoteProps.Start)
	}
	if pkg.Settings.FootnoteProps.Restart == nil || pkg.Settings.FootnoteProps.Restart.Val != "eachPage" {
		t.Fatalf("footnote restart = %#v, want eachPage", pkg.Settings.FootnoteProps.Restart)
	}
	if pkg.Settings.EndnoteProps == nil {
		t.Fatal("expected endnote settings")
	}
	if pkg.Settings.EndnoteProps.Position == nil || pkg.Settings.EndnoteProps.Position.Val != "sectEnd" {
		t.Fatalf("endnote position = %#v, want sectEnd", pkg.Settings.EndnoteProps.Position)
	}
	if pkg.Settings.EndnoteProps.Format == nil || pkg.Settings.EndnoteProps.Format.Val != "lowerRoman" {
		t.Fatalf("endnote format = %#v, want lowerRoman", pkg.Settings.EndnoteProps.Format)
	}
	if pkg.Settings.EndnoteProps.Start == nil || pkg.Settings.EndnoteProps.Start.Val != 5 {
		t.Fatalf("endnote start = %#v, want 5", pkg.Settings.EndnoteProps.Start)
	}
	if pkg.Settings.EndnoteProps.Restart == nil || pkg.Settings.EndnoteProps.Restart.Val != "eachSect" {
		t.Fatalf("endnote restart = %#v, want eachSect", pkg.Settings.EndnoteProps.Restart)
	}
}

func TestConvertNoteSeparatorStories(t *testing.T) {
	doc := &wordir.Document{
		FootnoteSeparator: wordir.Story{
			Paragraphs: []wordir.Paragraph{{Runs: []wordir.Run{{Text: "Footnote separator"}}}},
		},
		FootnoteContinuationSep: wordir.Story{
			Paragraphs: []wordir.Paragraph{{Runs: []wordir.Run{{Text: "Footnote continuation separator"}}}},
		},
		EndnoteSeparator: wordir.Story{
			Paragraphs: []wordir.Paragraph{{Runs: []wordir.Run{{Text: "Endnote separator"}}}},
		},
		EndnoteContinuationNotice: wordir.Story{
			Paragraphs: []wordir.Paragraph{{Runs: []wordir.Run{{Text: "Endnote continuation notice"}}}},
		},
		Footnotes: map[int]wordir.Story{
			4: {Paragraphs: []wordir.Paragraph{{Runs: []wordir.Run{{Text: "Normal footnote"}}}}},
		},
		Endnotes: map[int]wordir.Story{
			9: {Paragraphs: []wordir.Paragraph{{Runs: []wordir.Run{{Text: "Normal endnote"}}}}},
		},
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pkg.Footnotes == nil || len(pkg.Footnotes.Notes) != 3 {
		t.Fatalf("footnote note count = %d, want 3", len(pkg.Footnotes.Notes))
	}
	if pkg.Footnotes.Notes[0].Type != "separator" || pkg.Footnotes.Notes[1].Type != "continuationSeparator" || pkg.Footnotes.Notes[2].Type != "normal" {
		t.Fatalf("footnote note types = %q, %q, %q", pkg.Footnotes.Notes[0].Type, pkg.Footnotes.Notes[1].Type, pkg.Footnotes.Notes[2].Type)
	}
	if pkg.Endnotes == nil || len(pkg.Endnotes.Notes) != 3 {
		t.Fatalf("endnote note count = %d, want 3", len(pkg.Endnotes.Notes))
	}
	if pkg.Endnotes.Notes[0].Type != "separator" || pkg.Endnotes.Notes[1].Type != "continuationNotice" || pkg.Endnotes.Notes[2].Type != "normal" {
		t.Fatalf("endnote note types = %q, %q, %q", pkg.Endnotes.Notes[0].Type, pkg.Endnotes.Notes[1].Type, pkg.Endnotes.Notes[2].Type)
	}
}

func boolPtr(b bool) *bool {
	return &b
}

func TestConvertBodyBlockOrdering(t *testing.T) {
	doc := &wordir.Document{
		Blocks: []wordir.Block{
			{Kind: "paragraph", Paragraph: &wordir.Paragraph{Runs: []wordir.Run{{Text: "P1"}}}},
			{Kind: "table", Table: &wordir.Table{Rows: []wordir.TableRow{{Cells: []wordir.TableCell{{Paragraphs: []wordir.Paragraph{{Runs: []wordir.Run{{Text: "C1"}}}}}}}}}},
			{Kind: "paragraph", Paragraph: &wordir.Paragraph{Runs: []wordir.Run{{Text: "P2"}}}},
		},
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pkg.Document.Body.Paragraphs) != 2 {
		t.Fatalf("expected 2 paragraphs, got %d", len(pkg.Document.Body.Paragraphs))
	}
	if len(pkg.Document.Body.Tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(pkg.Document.Body.Tables))
	}
	if len(pkg.Document.Body.Blocks) != 3 {
		t.Fatalf("expected 3 blocks, got %d", len(pkg.Document.Body.Blocks))
	}
	if pkg.Document.Body.Blocks[0].Kind != wordparts.BodyBlockParagraph {
		t.Fatal("expected block 0 to be paragraph")
	}
	if pkg.Document.Body.Blocks[1].Kind != wordparts.BodyBlockTable {
		t.Fatal("expected block 1 to be table")
	}
	if pkg.Document.Body.Blocks[2].Kind != wordparts.BodyBlockParagraph {
		t.Fatal("expected block 2 to be paragraph")
	}
}

func TestConvertSectionBreakKind(t *testing.T) {
	doc := &wordir.Document{
		Paragraphs: []wordir.Paragraph{{Runs: []wordir.Run{{Text: "Hello"}}}},
		Sections: []wordir.Section{
			{BreakKind: "continuous"},
		},
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sp := pkg.Document.Body.Section
	if sp == nil || sp.Type == nil || sp.Type.Val != "continuous" {
		t.Fatal("expected section type 'continuous'")
	}
}

func TestConvertSectionHeadersAndFooters(t *testing.T) {
	doc := &wordir.Document{
		Paragraphs: []wordir.Paragraph{{Runs: []wordir.Run{{Text: "Hello"}}}},
		Sections: []wordir.Section{
			{
				Headers: []wordir.Reference{
					{Type: "default", PartName: "header1"},
				},
				Footers: []wordir.Reference{
					{Type: "default", PartName: "footer1"},
				},
			},
		},
		Headers: map[string]wordir.Story{
			"header1": {Paragraphs: []wordir.Paragraph{{Runs: []wordir.Run{{Text: "Header text"}}}}},
		},
		Footers: map[string]wordir.Story{
			"footer1": {Paragraphs: []wordir.Paragraph{{Runs: []wordir.Run{{Text: "Footer text"}}}}},
		},
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sp := pkg.Document.Body.Section
	if sp == nil {
		t.Fatal("expected section properties")
	}
	if len(sp.HeaderRefs) != 1 {
		t.Fatalf("expected 1 header ref, got %d", len(sp.HeaderRefs))
	}
	if sp.HeaderRefs[0].Type != "default" {
		t.Fatalf("expected header type 'default', got %q", sp.HeaderRefs[0].Type)
	}
	if len(sp.FooterRefs) != 1 {
		t.Fatalf("expected 1 footer ref, got %d", len(sp.FooterRefs))
	}
	if sp.FooterRefs[0].Type != "default" {
		t.Fatalf("expected footer type 'default', got %q", sp.FooterRefs[0].Type)
	}
	if len(pkg.Headers) != 1 {
		t.Fatalf("expected 1 header part, got %d", len(pkg.Headers))
	}
	if len(pkg.Footers) != 1 {
		t.Fatalf("expected 1 footer part, got %d", len(pkg.Footers))
	}
	// Verify relationship was created.
	mainRels := pkg.PartRels[pkg.MainPartName]
	if mainRels == nil || len(mainRels.Relationships) != 2 {
		t.Fatalf("expected 2 relationships, got %d", len(mainRels.Relationships))
	}
}

func TestConvertSectionTitlePageAndBidi(t *testing.T) {
	doc := &wordir.Document{
		Paragraphs: []wordir.Paragraph{{Runs: []wordir.Run{{Text: "Hello"}}}},
		Sections: []wordir.Section{
			{TitlePage: true, Bidi: true},
		},
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sp := pkg.Document.Body.Section
	if sp == nil {
		t.Fatal("expected section properties")
	}
	if sp.TitlePage == nil || !*sp.TitlePage.Val {
		t.Fatal("expected titlePage")
	}
	if sp.Bidi == nil || !*sp.Bidi.Val {
		t.Fatal("expected bidi")
	}
}

func TestConvertHyperlink(t *testing.T) {
	doc := &wordir.Document{
		Paragraphs: []wordir.Paragraph{{
			Runs: []wordir.Run{
				{Text: "Visit "},
				{Text: "example", Kind: "hyperlink", Target: "https://example.com", Format: wordir.RunFormat{Underline: "single"}},
				{Text: " for more."},
			},
		}},
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	p := pkg.Document.Body.Paragraphs[0]
	if len(p.Runs) != 2 {
		t.Fatalf("expected 2 regular runs, got %d", len(p.Runs))
	}
	if len(p.Hyperlinks) != 1 {
		t.Fatalf("expected 1 hyperlink, got %d", len(p.Hyperlinks))
	}
	hl := p.Hyperlinks[0]
	if hl.ID == "" {
		t.Fatal("expected hyperlink relationship ID")
	}
	if len(hl.Runs) != 1 {
		t.Fatalf("expected 1 run in hyperlink, got %d", len(hl.Runs))
	}
	if pkg.HyperlinkTarget(pkg.MainPartName, hl.ID) != "https://example.com" {
		t.Fatalf("expected URL https://example.com for rel %s, got %q", hl.ID, pkg.HyperlinkTarget(pkg.MainPartName, hl.ID))
	}
}

func TestConvertMultipleHyperlinks(t *testing.T) {
	doc := &wordir.Document{
		Paragraphs: []wordir.Paragraph{{
			Runs: []wordir.Run{
				{Text: "A", Kind: "hyperlink", Target: "https://a.com"},
				{Text: "B", Kind: "hyperlink", Target: "https://a.com"},
				{Text: "C"},
				{Text: "D", Kind: "hyperlink", Target: "https://b.com"},
			},
		}},
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	p := pkg.Document.Body.Paragraphs[0]
	if len(p.Hyperlinks) != 2 {
		t.Fatalf("expected 2 hyperlinks, got %d", len(p.Hyperlinks))
	}
	if len(p.Hyperlinks[0].Runs) != 2 {
		t.Fatalf("expected 2 runs in first hyperlink, got %d", len(p.Hyperlinks[0].Runs))
	}
	if len(p.Hyperlinks[1].Runs) != 1 {
		t.Fatalf("expected 1 run in second hyperlink, got %d", len(p.Hyperlinks[1].Runs))
	}
	if pkg.HyperlinkTarget(pkg.MainPartName, p.Hyperlinks[0].ID) != "https://a.com" {
		t.Fatal("expected first hyperlink URL to be https://a.com")
	}
	if pkg.HyperlinkTarget(pkg.MainPartName, p.Hyperlinks[1].ID) != "https://b.com" {
		t.Fatal("expected second hyperlink URL to be https://b.com")
	}
}

func TestConvertExtendedProperties(t *testing.T) {
	doc := &wordir.Document{
		Paragraphs:     []wordir.Paragraph{{Runs: []wordir.Run{{Text: "Hello"}}}},
		Company:        "Acme Corp",
		Manager:        "Alice Smith",
		AppVersion:     "16.0000",
		Template:       "Normal.dotm",
		Application:    "Microsoft Office Word",
		WordCount:      42,
		CharCount:      256,
		PageCount:      2,
		ParagraphCount: 5,
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pkg.ExtendedProperties == nil {
		t.Fatal("expected extended properties")
	}
	if pkg.ExtendedProperties.Company != "Acme Corp" {
		t.Fatalf("company = %q, want Acme Corp", pkg.ExtendedProperties.Company)
	}
	if pkg.ExtendedProperties.Manager != "Alice Smith" {
		t.Fatalf("manager = %q, want Alice Smith", pkg.ExtendedProperties.Manager)
	}
	if pkg.ExtendedProperties.AppVersion != "16.0000" {
		t.Fatalf("appVersion = %q, want 16.0000", pkg.ExtendedProperties.AppVersion)
	}
	if pkg.ExtendedProperties.Template != "Normal.dotm" {
		t.Fatalf("template = %q, want Normal.dotm", pkg.ExtendedProperties.Template)
	}
	if pkg.ExtendedProperties.Words != 42 {
		t.Fatalf("words = %d, want 42", pkg.ExtendedProperties.Words)
	}
}

func TestConvertCustomProperties(t *testing.T) {
	doc := &wordir.Document{
		Paragraphs: []wordir.Paragraph{{Runs: []wordir.Run{{Text: "Hello"}}}},
		CustomProperties: map[string]string{
			"ProjectID": "12345",
			"Status":    "Draft",
		},
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pkg.CustomProperties == nil {
		t.Fatal("expected custom properties")
	}
	if len(pkg.CustomProperties.Properties) != 2 {
		t.Fatalf("expected 2 custom properties, got %d", len(pkg.CustomProperties.Properties))
	}
	found := make(map[string]string)
	for _, p := range pkg.CustomProperties.Properties {
		found[p.Name] = p.Value
	}
	if found["ProjectID"] != "12345" {
		t.Fatalf("ProjectID = %q, want 12345", found["ProjectID"])
	}
	if found["Status"] != "Draft" {
		t.Fatalf("Status = %q, want Draft", found["Status"])
	}
}

func TestConvertDocumentVariables(t *testing.T) {
	doc := &wordir.Document{
		Paragraphs: []wordir.Paragraph{{Runs: []wordir.Run{{Text: "Hello"}}}},
		DocumentVariables: map[string]string{
			"Version":     "1.0",
			"Environment": "staging",
		},
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pkg.Settings == nil {
		t.Fatal("expected settings")
	}
	if len(pkg.Settings.DocVars) != 2 {
		t.Fatalf("expected 2 document variables, got %d", len(pkg.Settings.DocVars))
	}
	found := make(map[string]string, len(pkg.Settings.DocVars))
	for _, v := range pkg.Settings.DocVars {
		found[v.Name] = v.Value
	}
	if found["Version"] != "1.0" {
		t.Fatalf("Version = %q, want 1.0", found["Version"])
	}
	if found["Environment"] != "staging" {
		t.Fatalf("Environment = %q, want staging", found["Environment"])
	}
}

func TestConvertCustomXMLParts(t *testing.T) {
	doc := &wordir.Document{
		Paragraphs: []wordir.Paragraph{{Runs: []wordir.Run{{Text: "Hello"}}}},
		CustomXMLParts: map[string][]byte{
			"{STORE-1}": []byte(`<root><customer>Bound Customer</customer></root>`),
		},
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pkg.CustomXMLParts) != 1 {
		t.Fatalf("custom XML parts = %d, want 1", len(pkg.CustomXMLParts))
	}
	if got := string(pkg.CustomXMLParts["{STORE-1}"]); got != `<root><customer>Bound Customer</customer></root>` {
		t.Fatalf("custom XML part = %q, want payload", got)
	}
}

func TestConvertNumberingDefinitionParagraphStyle(t *testing.T) {
	doc := &wordir.Document{
		Paragraphs: []wordir.Paragraph{{Runs: []wordir.Run{{Text: "Hello"}}}},
		NumberingDefinitions: []wordir.NumberingDefinition{
			{
				NumID:         1,
				AbstractNumID: 10,
				Levels: []wordir.NumberingLevel{
					{Level: 0, Format: "decimal", Text: "%1.", StyleID: "ListParagraph"},
				},
			},
		},
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pkg.Numbering == nil || len(pkg.Numbering.AbstractNums) != 1 {
		t.Fatalf("numbering = %+v, want one abstractNum", pkg.Numbering)
	}
	lvl := pkg.Numbering.AbstractNums[0].Levels[0]
	if lvl.PStyle == nil || lvl.PStyle.Val != "ListParagraph" {
		t.Fatalf("pStyle = %+v, want ListParagraph", lvl.PStyle)
	}
}

func TestConvertSettingsDefaultTabStopAndZoom(t *testing.T) {
	doc := &wordir.Document{
		Paragraphs:     []wordir.Paragraph{{Runs: []wordir.Run{{Text: "Hello"}}}},
		DefaultTabStop: 720,
		ZoomPercent:    150,
		ViewMode:       "web",
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pkg.Settings == nil {
		t.Fatal("expected settings")
	}
	if pkg.Settings.DefaultTabStop == nil || pkg.Settings.DefaultTabStop.Val != 720 {
		t.Fatalf("defaultTabStop = %v, want 720", pkg.Settings.DefaultTabStop)
	}
	if pkg.Settings.Zoom == nil || pkg.Settings.Zoom.Percent != 150 {
		t.Fatalf("zoom = %v, want 150", pkg.Settings.Zoom)
	}
	if pkg.Settings.View == nil || pkg.Settings.View.Val != "web" {
		t.Fatalf("view = %v, want web", pkg.Settings.View)
	}
}

func TestConvertSettingsTrackRevisionsAndProtection(t *testing.T) {
	doc := &wordir.Document{
		Paragraphs:          []wordir.Paragraph{{Runs: []wordir.Run{{Text: "Hello"}}}},
		TrackChangesEnabled: true,
		ProtectionEnabled:   true,
		DocumentProtection: &wordir.DocumentProtection{
			Edit:        "forms",
			Enforcement: true,
		},
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pkg.Settings == nil {
		t.Fatal("expected settings")
	}
	if pkg.Settings.TrackRevisions == nil || !*pkg.Settings.TrackRevisions.Val {
		t.Fatal("expected trackRevisions")
	}
	if pkg.Settings.DocumentProtection == nil {
		t.Fatal("expected documentProtection")
	}
	if pkg.Settings.DocumentProtection.Edit != "forms" {
		t.Fatalf("edit = %q, want forms", pkg.Settings.DocumentProtection.Edit)
	}
}

func TestConvertSettingsCharacterSpacingControl(t *testing.T) {
	doc := &wordir.Document{
		Paragraphs:              []wordir.Paragraph{{Runs: []wordir.Run{{Text: "Hello"}}}},
		CharacterSpacingControl: "compressPunctuation",
		AutoHyphenation:         true,
		EvenAndOddHeaders:       true,
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pkg.Settings == nil {
		t.Fatal("expected settings")
	}
	if pkg.Settings.CharacterSpacingControl == nil || pkg.Settings.CharacterSpacingControl.Val != "compressPunctuation" {
		t.Fatal("expected characterSpacingControl")
	}
	if pkg.Settings.AutoHyphenation == nil || !*pkg.Settings.AutoHyphenation.Val {
		t.Fatal("expected autoHyphenation")
	}
	if pkg.Settings.EvenAndOddHeaders == nil || !*pkg.Settings.EvenAndOddHeaders.Val {
		t.Fatal("expected evenAndOddHeaders")
	}
}

func TestConvertTableCellHMerge(t *testing.T) {
	doc := &wordir.Document{
		Tables: []wordir.Table{{
			Rows: []wordir.TableRow{{
				Cells: []wordir.TableCell{{
					Format: wordir.TableCellFormat{HMerge: true},
					Paragraphs: []wordir.Paragraph{{
						Runs: []wordir.Run{{Text: "A"}},
					}},
				}},
			}},
		}},
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cell := pkg.Document.Body.Tables[0].Rows[0].Cells[0]
	if cell.Properties == nil || cell.Properties.HMerge == nil {
		t.Fatal("expected hMerge")
	}
	if cell.Properties.HMerge.Val != "restart" {
		t.Fatalf("hMerge val = %q, want restart", cell.Properties.HMerge.Val)
	}
}

func TestConvertSectionPageBorders(t *testing.T) {
	doc := &wordir.Document{
		Paragraphs: []wordir.Paragraph{{Runs: []wordir.Run{{Text: "Hello"}}}},
		Sections: []wordir.Section{{
			PageBorders: &wordir.PageBorders{
				Top:        &wordir.BorderSide{Val: "single", SizePt: 1.5, Color: "FF0000"},
				Bottom:     &wordir.BorderSide{Val: "double", SizePt: 2, Color: "00FF00"},
				Display:    "firstPage",
				OffsetFrom: "text",
			},
		}},
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sp := pkg.Document.Body.Section
	if sp == nil || sp.PageBorders == nil {
		t.Fatal("expected page borders")
	}
	if sp.PageBorders.Top == nil || sp.PageBorders.Top.Val != "single" {
		t.Fatal("expected top border")
	}
	if sp.PageBorders.Bottom == nil || sp.PageBorders.Bottom.Val != "double" {
		t.Fatal("expected bottom border")
	}
	if sp.PageBorders.Display != "firstPage" {
		t.Fatalf("display = %q, want firstPage", sp.PageBorders.Display)
	}
}

func TestConvertSectionLineNumbering(t *testing.T) {
	doc := &wordir.Document{
		Paragraphs: []wordir.Paragraph{{Runs: []wordir.Run{{Text: "Hello"}}}},
		Sections: []wordir.Section{{
			LineNumbering: &wordir.LineNumbering{
				CountBy:  5,
				Start:    1,
				Restart:  "newPage",
				Distance: 720,
			},
		}},
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sp := pkg.Document.Body.Section
	if sp == nil || sp.LnNumType == nil {
		t.Fatal("expected line numbering")
	}
	if sp.LnNumType.CountBy != 5 {
		t.Fatalf("countBy = %d, want 5", sp.LnNumType.CountBy)
	}
	if sp.LnNumType.Restart != "newPage" {
		t.Fatalf("restart = %q, want newPage", sp.LnNumType.Restart)
	}
}

func TestConvertSectionDocGrid(t *testing.T) {
	doc := &wordir.Document{
		Paragraphs: []wordir.Paragraph{{Runs: []wordir.Run{{Text: "Hello"}}}},
		Sections: []wordir.Section{{
			DocGrid: &wordir.DocGrid{
				LinePitch: 360,
				CharSpace: -2048,
				Type:      "linesAndChars",
			},
		}},
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sp := pkg.Document.Body.Section
	if sp == nil || sp.DocGrid == nil {
		t.Fatal("expected doc grid")
	}
	if sp.DocGrid.LinePitch != 360 {
		t.Fatalf("linePitch = %d, want 360", sp.DocGrid.LinePitch)
	}
	if sp.DocGrid.Type != "linesAndChars" {
		t.Fatalf("type = %q, want linesAndChars", sp.DocGrid.Type)
	}
}

func TestConvertSectionVerticalAlign(t *testing.T) {
	doc := &wordir.Document{
		Paragraphs: []wordir.Paragraph{{Runs: []wordir.Run{{Text: "Hello"}}}},
		Sections: []wordir.Section{{
			VerticalAlign: "center",
		}},
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sp := pkg.Document.Body.Section
	if sp == nil || sp.VAlign == nil || sp.VAlign.Val != "center" {
		t.Fatal("expected vertical align center")
	}
}

func TestConvertSectionPaperSource(t *testing.T) {
	doc := &wordir.Document{
		Paragraphs: []wordir.Paragraph{{Runs: []wordir.Run{{Text: "Hello"}}}},
		Sections: []wordir.Section{{
			PaperSource: &wordir.PaperSource{First: 15, Other: 16},
		}},
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sp := pkg.Document.Body.Section
	if sp == nil || sp.PaperSrc == nil {
		t.Fatal("expected paper source")
	}
	if sp.PaperSrc.First != 15 {
		t.Fatalf("first = %d, want 15", sp.PaperSrc.First)
	}
}

func TestConvertSectionFootnoteEndnoteProps(t *testing.T) {
	doc := &wordir.Document{
		Paragraphs: []wordir.Paragraph{{Runs: []wordir.Run{{Text: "Hello"}}}},
		Sections: []wordir.Section{{
			FootnotePos:      "belowText",
			FootnoteNumFmt:   "lowerRoman",
			FootnoteNumStart: 3,
			FootnoteRestart:  "eachSect",
			EndnoteNumFmt:    "upperRoman",
			EndnoteNumStart:  1,
			EndnoteRestart:   "eachSect",
		}},
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sp := pkg.Document.Body.Section
	if sp == nil {
		t.Fatal("expected section properties")
	}
	if sp.FootnoteProps == nil {
		t.Fatal("expected footnote props")
	}
	if sp.FootnoteProps.Position == nil || sp.FootnoteProps.Position.Val != "belowText" {
		t.Fatal("expected footnote position belowText")
	}
	if sp.FootnoteProps.Format == nil || sp.FootnoteProps.Format.Val != "lowerRoman" {
		t.Fatal("expected footnote format lowerRoman")
	}
	if sp.EndnoteProps == nil {
		t.Fatal("expected endnote props")
	}
	if sp.EndnoteProps.Format == nil || sp.EndnoteProps.Format.Val != "upperRoman" {
		t.Fatal("expected endnote format upperRoman")
	}
}

func TestConvertParagraphTabStops(t *testing.T) {
	doc := &wordir.Document{
		Paragraphs: []wordir.Paragraph{{
			Runs: []wordir.Run{{Text: "Hello"}},
			Format: wordir.ParagraphFormat{
				TabStops: []wordir.TabStop{
					{Pos: 720, Algn: "left"},
					{Pos: 1440, Algn: "center"},
					{Pos: 2160, Algn: "right"},
				},
			},
		}},
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	p := pkg.Document.Body.Paragraphs[0]
	if len(p.Properties.Tabs) != 3 {
		t.Fatalf("expected 3 tab stops, got %d", len(p.Properties.Tabs))
	}
	if p.Properties.Tabs[0].Pos != 720 || p.Properties.Tabs[0].Val != "left" {
		t.Fatalf("tab[0] = %+v", p.Properties.Tabs[0])
	}
	if p.Properties.Tabs[2].Pos != 2160 || p.Properties.Tabs[2].Val != "right" {
		t.Fatalf("tab[2] = %+v", p.Properties.Tabs[2])
	}
}

func TestConvertParagraphOutlineLevel(t *testing.T) {
	level := 2
	doc := &wordir.Document{
		Paragraphs: []wordir.Paragraph{{
			Runs: []wordir.Run{{Text: "Hello"}},
			Format: wordir.ParagraphFormat{
				OutlineLevel: &level,
			},
		}},
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	p := pkg.Document.Body.Paragraphs[0]
	if p.Properties.OutlineLvl == nil || p.Properties.OutlineLvl.Val != 2 {
		t.Fatalf("outlineLvl = %v, want 2", p.Properties.OutlineLvl)
	}
}

func TestConvertParagraphSuppressLineNumbers(t *testing.T) {
	doc := &wordir.Document{
		Paragraphs: []wordir.Paragraph{{
			Runs: []wordir.Run{{Text: "Hello"}},
			Format: wordir.ParagraphFormat{
				NoLineNumber: true,
			},
		}},
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	p := pkg.Document.Body.Paragraphs[0]
	if p.Properties.SuppressLineNumbers == nil || !*p.Properties.SuppressLineNumbers.Val {
		t.Fatal("expected suppressLineNumbers")
	}
}

func TestConvertRunDoubleStrike(t *testing.T) {
	doc := &wordir.Document{
		Paragraphs: []wordir.Paragraph{{
			Runs: []wordir.Run{{
				Text: "Hello",
				Format: wordir.RunFormat{
					DoubleStrike: boolPtr(true),
				},
			}},
		}},
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rp := pkg.Document.Body.Paragraphs[0].Runs[0].Properties
	if rp == nil || rp.DoubleStrike == nil || !*rp.DoubleStrike.Val {
		t.Fatal("expected doubleStrike")
	}
}

func TestConvertRunWebHiddenKerningCharScale(t *testing.T) {
	doc := &wordir.Document{
		Paragraphs: []wordir.Paragraph{{
			Runs: []wordir.Run{{
				Text: "Hello",
				Format: wordir.RunFormat{
					WebHidden:    boolPtr(true),
					Kerning:      ptrInt(14),
					CharScale:    ptrInt(150),
					FitTextWidth: ptrInt(7200),
					EmphasisMark: "dot",
				},
			}},
		}},
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rp := pkg.Document.Body.Paragraphs[0].Runs[0].Properties
	if rp == nil || rp.WebHidden == nil || !*rp.WebHidden.Val {
		t.Fatal("expected webHidden")
	}
	if rp.Kerning == nil || rp.Kerning.Val != 14 {
		t.Fatalf("kerning = %v, want 14", rp.Kerning)
	}
	if rp.CharScale == nil || rp.CharScale.Val != 150 {
		t.Fatalf("charScale = %v, want 150", rp.CharScale)
	}
	if rp.FitText == nil || rp.FitText.Val != 7200 {
		t.Fatalf("fitText = %v, want 7200", rp.FitText)
	}
	if rp.EmphasisMark == nil || rp.EmphasisMark.Val != "dot" {
		t.Fatalf("emphasisMark = %v, want dot", rp.EmphasisMark)
	}
}

func TestConvertTableBidiVisualOverlapCaption(t *testing.T) {
	doc := &wordir.Document{
		Tables: []wordir.Table{{
			Format: wordir.TableFormat{
				BidiVisual:  true,
				Overlap:     "never",
				Caption:     "Table 1",
				Description: "A sample table",
			},
			Rows: []wordir.TableRow{{
				Cells: []wordir.TableCell{{
					Paragraphs: []wordir.Paragraph{{Runs: []wordir.Run{{Text: "A"}}}},
				}},
			}},
		}},
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tp := pkg.Document.Body.Tables[0].Properties
	if tp == nil {
		t.Fatal("expected table properties")
	}
	if tp.BidiVisual == nil || !*tp.BidiVisual.Val {
		t.Fatal("expected bidiVisual")
	}
	if tp.Overlap == nil || tp.Overlap.Val != "never" {
		t.Fatalf("overlap = %v, want never", tp.Overlap)
	}
	if tp.Caption != "Table 1" {
		t.Fatalf("caption = %q, want Table 1", tp.Caption)
	}
	if tp.Description != "A sample table" {
		t.Fatalf("description = %q, want A sample table", tp.Description)
	}
}

func TestConvertInlineImage(t *testing.T) {
	doc := &wordir.Document{
		Paragraphs: []wordir.Paragraph{{
			Runs: []wordir.Run{{
				Kind: "image",
				Image: &wordir.Image{
					MIMEType: "image/png",
					Data:     []byte("fake"),
					Width:    914400,
					Height:   914400,
					Alt:      "Test Image",
					Title:    "Title",
				},
			}},
		}},
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rc := pkg.Document.Body.Paragraphs[0].Runs[0].Contents[0]
	if rc.Kind != wordparts.RunContentDrawing {
		t.Fatalf("kind = %v, want drawing", rc.Kind)
	}
	if rc.Drawing == nil || rc.Drawing.Inline == nil {
		t.Fatal("expected inline drawing")
	}
	if rc.Drawing.Anchor != nil {
		t.Fatal("expected no anchor")
	}
	if rc.Drawing.Inline.Extent.CX != 914400 || rc.Drawing.Inline.Extent.CY != 914400 {
		t.Fatalf("extent = %v, want 914400x914400", rc.Drawing.Inline.Extent)
	}
	if rc.Drawing.Inline.DocPr.Name != "Test Image" {
		t.Fatalf("docPr name = %q, want Test Image", rc.Drawing.Inline.DocPr.Name)
	}
	if rc.Drawing.Inline.DocPr.Title != "Title" {
		t.Fatalf("docPr title = %q, want Title", rc.Drawing.Inline.DocPr.Title)
	}
}

func TestConvertImageStretchAndTile(t *testing.T) {
	doc := &wordir.Document{
		Paragraphs: []wordir.Paragraph{{
			Runs: []wordir.Run{{
				Kind: "image",
				Image: &wordir.Image{
					MIMEType: "image/png",
					Data:     []byte("fake"),
					Width:    914400,
					Height:   914400,
					Tile:     true,
					Stretch:  false,
				},
			}, {
				Kind: "image",
				Image: &wordir.Image{
					MIMEType: "image/png",
					Data:     []byte("fake"),
					Width:    914400,
					Height:   914400,
					Tile:     false,
					Stretch:  true,
				},
			}},
		}},
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	runs := pkg.Document.Body.Paragraphs[0].Runs
	if len(runs) != 2 {
		t.Fatalf("expected 2 runs, got %d", len(runs))
	}
	// First image: Tile=true, Stretch=false
	pic1 := runs[0].Contents[0].Drawing.Inline.Graphic.GraphicData.Picture
	if pic1 == nil || pic1.BlipFill == nil {
		t.Fatal("expected picture with blipFill")
	}
	if pic1.BlipFill.Tile == nil {
		t.Error("expected Tile set on first image")
	}
	if pic1.BlipFill.Stretch != nil {
		t.Error("expected no Stretch on first image")
	}
	// Second image: Tile=false, Stretch=true
	pic2 := runs[1].Contents[0].Drawing.Inline.Graphic.GraphicData.Picture
	if pic2 == nil || pic2.BlipFill == nil {
		t.Fatal("expected picture with blipFill")
	}
	if pic2.BlipFill.Tile != nil {
		t.Error("expected no Tile on second image")
	}
	if pic2.BlipFill.Stretch == nil {
		t.Error("expected Stretch set on second image")
	}
}

func TestConvertFloatingImage(t *testing.T) {
	doc := &wordir.Document{
		Paragraphs: []wordir.Paragraph{{
			Runs: []wordir.Run{{
				Kind: "image",
				Image: &wordir.Image{
					MIMEType:      "image/png",
					Data:          []byte("fake"),
					Width:         914400,
					Height:        914400,
					Floating:      true,
					X:             1000000,
					Y:             2000000,
					XRelative:     "column",
					YRelative:     "paragraph",
					ZIndex:        5,
					BehindDoc:     true,
					WrapType:      "square",
					WrapSide:      "bothSides",
					DistTop:       10,
					DistBottom:    10,
					DistLeft:      15,
					DistRight:     15,
					SimplePosX:    500000,
					SimplePosY:    500000,
					EffectExtentL: 1000,
					EffectExtentT: 2000,
					EffectExtentR: 3000,
					EffectExtentB: 4000,
					Locked:        true,
				},
			}},
		}},
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rc := pkg.Document.Body.Paragraphs[0].Runs[0].Contents[0]
	if rc.Kind != wordparts.RunContentDrawing {
		t.Fatalf("kind = %v, want drawing", rc.Kind)
	}
	if rc.Drawing == nil || rc.Drawing.Anchor == nil {
		t.Fatal("expected anchor drawing")
	}
	if rc.Drawing.Inline != nil {
		t.Fatal("expected no inline")
	}

	a := rc.Drawing.Anchor
	if a.RelativeHeight != 5 {
		t.Fatalf("relativeHeight = %d, want 5", a.RelativeHeight)
	}
	if a.BehindDoc != 1 {
		t.Fatalf("behindDoc = %d, want 1", a.BehindDoc)
	}
	if a.Locked != true {
		t.Fatal("expected locked")
	}
	if a.Extent.CX != 914400 || a.Extent.CY != 914400 {
		t.Fatalf("extent = %v, want 914400x914400", a.Extent)
	}
	if a.DistT != 127000 {
		t.Fatalf("distT = %d, want 127000", a.DistT)
	}
	if a.DistB != 127000 {
		t.Fatalf("distB = %d, want 127000", a.DistB)
	}
	if a.DistL != 190500 {
		t.Fatalf("distL = %d, want 190500", a.DistL)
	}
	if a.DistR != 190500 {
		t.Fatalf("distR = %d, want 190500", a.DistR)
	}
	if a.SimplePos == nil || a.SimplePos.X != 500000 || a.SimplePos.Y != 500000 {
		t.Fatalf("simplePos = %v, want 500000,500000", a.SimplePos)
	}
	if !a.SimplePosAttr {
		t.Fatal("expected simplePos attr")
	}
	if a.PositionH == nil || a.PositionH.PosOffset != 1000000 || a.PositionH.RelativeFrom != "column" {
		t.Fatalf("positionH = %v, want posOffset 1000000 relativeFrom column", a.PositionH)
	}
	if a.PositionV == nil || a.PositionV.PosOffset != 2000000 || a.PositionV.RelativeFrom != "paragraph" {
		t.Fatalf("positionV = %v, want posOffset 2000000 relativeFrom paragraph", a.PositionV)
	}
	if a.EffectExtent == nil || a.EffectExtent.L != 1000 || a.EffectExtent.T != 2000 || a.EffectExtent.R != 3000 || a.EffectExtent.B != 4000 {
		t.Fatalf("effectExtent = %v, want 1000,2000,3000,4000", a.EffectExtent)
	}
	if a.WrapSquare == nil {
		t.Fatal("expected wrapSquare")
	}
	if a.WrapSquare.WrapText != "bothSides" {
		t.Fatalf("wrapText = %q, want bothSides", a.WrapSquare.WrapText)
	}
}

func TestConvertFloatingImageAlign(t *testing.T) {
	doc := &wordir.Document{
		Paragraphs: []wordir.Paragraph{{
			Runs: []wordir.Run{{
				Kind: "image",
				Image: &wordir.Image{
					MIMEType:  "image/png",
					Data:      []byte("fake"),
					Width:     914400,
					Height:    914400,
					Floating:  true,
					XAlign:    "right",
					XRelative: "margin",
					YAlign:    "bottom",
					YRelative: "page",
				},
			}},
		}},
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	a := pkg.Document.Body.Paragraphs[0].Runs[0].Contents[0].Drawing.Anchor
	if a.PositionH == nil || a.PositionH.Align != "right" || a.PositionH.RelativeFrom != "margin" {
		t.Fatalf("positionH = %v, want align right relativeFrom margin", a.PositionH)
	}
	if a.PositionV == nil || a.PositionV.Align != "bottom" || a.PositionV.RelativeFrom != "page" {
		t.Fatalf("positionV = %v, want align bottom relativeFrom page", a.PositionV)
	}
}

func TestConvertFloatingImageWrapTypes(t *testing.T) {
	cases := []struct {
		wrap    string
		wantNil string // field that must be nil
		wantSet string // field that must be non-nil
	}{
		{"none", "WrapSquare", "WrapNone"},
		{"square", "WrapNone", "WrapSquare"},
		{"tight", "WrapSquare", "WrapTight"},
		{"through", "WrapTight", "WrapThrough"},
		{"topAndBottom", "WrapThrough", "WrapTopAndBottom"},
	}

	for _, tc := range cases {
		t.Run(tc.wrap, func(t *testing.T) {
			doc := &wordir.Document{
				Paragraphs: []wordir.Paragraph{{
					Runs: []wordir.Run{{
						Kind: "image",
						Image: &wordir.Image{
							MIMEType:    "image/png",
							Data:        []byte("fake"),
							Width:       914400,
							Height:      914400,
							Floating:    true,
							WrapType:    tc.wrap,
							WrapPolygon: [][2]float64{{0, 0}, {1, 0}, {1, 1}, {0, 1}},
						},
					}},
				}},
			}
			pkg, err := Convert(doc)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			a := pkg.Document.Body.Paragraphs[0].Runs[0].Contents[0].Drawing.Anchor

			// Dynamically check nil/set using reflection is overkill; switch on wantSet.
			switch tc.wantSet {
			case "WrapNone":
				if a.WrapNone == nil {
					t.Fatal("expected WrapNone")
				}
			case "WrapSquare":
				if a.WrapSquare == nil {
					t.Fatal("expected WrapSquare")
				}
			case "WrapTight":
				if a.WrapTight == nil {
					t.Fatal("expected WrapTight")
				}
				if a.WrapTight.Polygon == nil {
					t.Fatal("expected wrap polygon")
				}
			case "WrapThrough":
				if a.WrapThrough == nil {
					t.Fatal("expected WrapThrough")
				}
				if a.WrapThrough.Polygon == nil {
					t.Fatal("expected wrap polygon")
				}
			case "WrapTopAndBottom":
				if a.WrapTopAndBottom == nil {
					t.Fatal("expected WrapTopAndBottom")
				}
			}
		})
	}
}

func TestConvertCompatSettings(t *testing.T) {
	doc := &wordir.Document{
		Compatibility: &wordir.DocCompatibility{
			DoNotUseHTMLParagraphAutoSpacing: true,
			AlignBordersAndEdges:             true,
			LineWrapLikeWord6:                false,
			NoLeading:                        true,
			SpaceLikeWW8:                     true,
			GrowAutofit:                      true,
		},
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pkg.Settings == nil || pkg.Settings.Compat == nil {
		t.Fatal("expected compat settings")
	}
	settings := pkg.Settings.Compat.Settings
	if len(settings) != 5 {
		t.Fatalf("compat settings count = %d, want 5", len(settings))
	}
	names := make(map[string]bool)
	for _, s := range settings {
		names[s.Name] = true
		if s.URI != "http://schemas.microsoft.com/office/word" {
			t.Fatalf("uri = %q, want MS Office word uri", s.URI)
		}
		if s.Value != "1" {
			t.Fatalf("val for %s = %q, want 1", s.Name, s.Value)
		}
	}
	want := []string{
		"doNotUseHTMLParagraphAutoSpacing",
		"alignBordersAndEdges",
		"noLeading",
		"spaceLikeWW8",
		"growAutofit",
	}
	for _, w := range want {
		if !names[w] {
			t.Fatalf("missing compat setting %q", w)
		}
	}
}

func TestConvertCompatSettingsNone(t *testing.T) {
	doc := &wordir.Document{}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pkg.Settings == nil || pkg.Settings.Compat != nil {
		t.Fatal("expected no compat settings when compatibility is nil")
	}
}

func TestConvertCompatSettingsNewFlags(t *testing.T) {
	doc := &wordir.Document{
		Compatibility: &wordir.DocCompatibility{
			AdjustLineHeightInTable:           true,
			AlignTablesRowByRow:               true,
			AllowPNG:                          true,
			AutoSpaceLikeWord95:               true,
			ConvMailMergeEsc:                  true,
			DoNotSuppressParagraphBorders:     true,
			DoNotUseEastAsianBreakRules:       true,
			DoNotWrapTextWithPunct:            true,
			ForgetLastTabAlignment:            true,
			LayoutRawTableWidth:               true,
			LayoutTableRowsApart:              true,
			MwSmallCaps:                       true,
			NoExtraLineSpacing:                true,
			NoSpaceRaiseLower:                 true,
			PrintBodyTextBeforeHeader:         true,
			PrintColBlack:                     true,
			SelectFldWithFirstOrLastChar:      true,
			ShowBreaksInFrames:                true,
			SpacingInWholePoints:              true,
			SubFontBySize:                     true,
			SuppressBottomSpacing:             true,
			SuppressSpBfAfterPgBrk:            true,
			SuppressSpacingAtTopOfPage:        true,
			SuppressTopSpacing:                true,
			SuppressTopSpacingWP:              true,
			SwapBordersFacingPages:            true,
			TruncateFontHeightsLikeWP6:        true,
			UseFELayout:                       true,
			UsePrinterMetrics:                 true,
			UseSingleBorderforContiguousCells: true,
			UseWord97LineBreakRules:           true,
			WpJustification:                   true,
			WpSpaceWidth:                      true,
			WrapTrailSpaces:                   true,
		},
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pkg.Settings == nil || pkg.Settings.Compat == nil {
		t.Fatal("expected compat settings")
	}
	settings := pkg.Settings.Compat.Settings
	want := []string{
		"adjustLineHeightInTable",
		"alignTablesRowByRow",
		"allowPNG",
		"autoSpaceLikeWord95",
		"convMailMergeEsc",
		"doNotSuppressParagraphBorders",
		"doNotUseEastAsianBreakRules",
		"doNotWrapTextWithPunct",
		"forgetLastTabAlignment",
		"layoutRawTableWidth",
		"layoutTableRowsApart",
		"mwSmallCaps",
		"noExtraLineSpacing",
		"noSpaceRaiseLower",
		"printBodyTextBeforeHeader",
		"printColBlack",
		"selectFldWithFirstOrLastChar",
		"showBreaksInFrames",
		"spacingInWholePoints",
		"subFontBySize",
		"suppressBottomSpacing",
		"suppressSpBfAfterPgBrk",
		"suppressSpacingAtTopOfPage",
		"suppressTopSpacing",
		"suppressTopSpacingWP",
		"swapBordersFacingPages",
		"truncateFontHeightsLikeWP6",
		"useFELayout",
		"usePrinterMetrics",
		"useSingleBorderforContiguousCells",
		"useWord97LineBreakRules",
		"wpJustification",
		"wpSpaceWidth",
		"wrapTrailSpaces",
	}
	if len(settings) != len(want) {
		t.Fatalf("compat settings count = %d, want %d", len(settings), len(want))
	}
	names := make(map[string]bool)
	for _, s := range settings {
		names[s.Name] = true
		if s.URI != "http://schemas.microsoft.com/office/word" {
			t.Fatalf("uri = %q, want MS Office word uri", s.URI)
		}
		if s.Value != "1" {
			t.Fatalf("val for %s = %q, want 1", s.Name, s.Value)
		}
	}
	for _, w := range want {
		if !names[w] {
			t.Fatalf("missing compat setting %q", w)
		}
	}
}

func TestConvertTableRowJustificationConditional(t *testing.T) {
	doc := &wordir.Document{
		Tables: []wordir.Table{{
			Rows: []wordir.TableRow{{
				Format: wordir.TableRowFormat{
					Justification: "center",
					Conditional:   "101000001100",
					Hidden:        true,
				},
				Cells: []wordir.TableCell{{
					Paragraphs: []wordir.Paragraph{{Runs: []wordir.Run{{Text: "A"}}}},
				}},
			}},
		}},
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	trp := pkg.Document.Body.Tables[0].Rows[0].Properties
	if trp == nil {
		t.Fatal("expected row properties")
	}
	if trp.Justification == nil || trp.Justification.Val != "center" {
		t.Fatalf("justification = %v, want center", trp.Justification)
	}
	if trp.Conditional == nil || trp.Conditional.Val != "101000001100" {
		t.Fatalf("conditional = %v, want 101000001100", trp.Conditional)
	}
	if trp.Hidden == nil || !*trp.Hidden.Val {
		t.Fatal("expected hidden row")
	}
}

func TestConvertParagraphBetweenAndBarBorders(t *testing.T) {
	doc := &wordir.Document{
		Paragraphs: []wordir.Paragraph{{
			Format: wordir.ParagraphFormat{
				Borders: &wordir.ParagraphBorders{
					Top:     &wordir.BorderSide{Val: "single", SizePt: 6, Color: "FF0000"},
					Between: &wordir.BorderSide{Val: "double", SizePt: 4, Color: "00FF00"},
					Bar:     &wordir.BorderSide{Val: "dashed", SizePt: 3, Color: "0000FF"},
				},
			},
			Runs: []wordir.Run{{Text: "hello"}},
		}},
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	b := pkg.Document.Body.Paragraphs[0].Properties.Borders
	if b == nil {
		t.Fatal("expected borders")
	}
	if b.Top == nil || b.Top.Val != "single" {
		t.Fatalf("top border = %v, want single", b.Top)
	}
	if b.Between == nil || b.Between.Val != "double" {
		t.Fatalf("between border = %v, want double", b.Between)
	}
	if b.Bar == nil || b.Bar.Val != "dashed" {
		t.Fatalf("bar border = %v, want dashed", b.Bar)
	}
}

func TestConvertTableStyle(t *testing.T) {
	doc := &wordir.Document{
		Styles: map[string]wordir.Style{
			"TableNormal": {
				ID:   "TableNormal",
				Name: "Table Normal",
				TableStyles: []wordir.ConditionalTableStyle{
					{Type: "firstRow", CellFormat: wordir.TableCellFormat{Shading: "CCCCCC"}, RunFormat: wordir.RunFormat{Bold: boolPtr(true)}},
					{Type: "lastRow", CellFormat: wordir.TableCellFormat{VertAlign: "bottom"}},
					{Type: "band1H", ParaFormat: wordir.ParagraphFormat{Alignment: "center"}},
				},
			},
		},
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pkg.Styles == nil || len(pkg.Styles.Styles) != 1 {
		t.Fatalf("expected 1 style, got %d", len(pkg.Styles.Styles))
	}
	s := pkg.Styles.Styles[0]
	if s.Type != "table" {
		t.Fatalf("type = %q, want table", s.Type)
	}
	if s.StyleID != "TableNormal" {
		t.Fatalf("styleId = %q, want TableNormal", s.StyleID)
	}
	if len(s.TableStyles) != 3 {
		t.Fatalf("tblStylePr count = %d, want 3", len(s.TableStyles))
	}
	if s.TableStyles[0].Type != "firstRow" {
		t.Fatalf("first tblStylePr type = %q, want firstRow", s.TableStyles[0].Type)
	}
	if s.TableStyles[0].CellProps == nil || s.TableStyles[0].CellProps.Shading == nil || s.TableStyles[0].CellProps.Shading.Fill != "CCCCCC" {
		t.Fatalf("firstRow cell shading = %v, want CCCCCC", s.TableStyles[0].CellProps)
	}
	if s.TableStyles[0].RunProps == nil || s.TableStyles[0].RunProps.Bold == nil || !*s.TableStyles[0].RunProps.Bold.Val {
		t.Fatal("expected firstRow run bold")
	}
	if s.TableStyles[2].Type != "band1H" {
		t.Fatalf("third tblStylePr type = %q, want band1H", s.TableStyles[2].Type)
	}
	if s.TableStyles[2].ParaProps == nil || s.TableStyles[2].ParaProps.Justification == nil || s.TableStyles[2].ParaProps.Justification.Val != "center" {
		t.Fatalf("band1H para alignment = %v, want center", s.TableStyles[2].ParaProps)
	}
}

func TestConvertStyleMetadata(t *testing.T) {
	doc := &wordir.Document{
		Styles: map[string]wordir.Style{
			"Heading1": {
				ID:             "Heading1",
				Name:           "Heading 1",
				BasedOn:        "Normal",
				NextStyleID:    "Normal",
				UIPriority:     9,
				RSID:           "12345678",
				UnhideWhenUsed: true,
				QFormat:        true,
				SemiHidden:     true,
				AutoRedefine:   true,
				Format: wordir.ParagraphFormat{
					Alignment: "left",
				},
			},
		},
		CharacterStyles: map[string]wordir.CharacterStyle{
			"Emphasis": {
				ID:             "Emphasis",
				Name:           "Emphasis",
				BasedOn:        "DefaultParagraphFont",
				NextStyleID:    "Emphasis",
				UIPriority:     20,
				RSID:           "87654321",
				UnhideWhenUsed: true,
				QFormat:        true,
				SemiHidden:     true,
				AutoRedefine:   true,
			},
		},
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pkg.Styles == nil || len(pkg.Styles.Styles) != 2 {
		t.Fatalf("expected 2 styles, got %d", len(pkg.Styles.Styles))
	}
	var ps, cs *wordparts.Style
	for _, s := range pkg.Styles.Styles {
		if s.StyleID == "Heading1" {
			ps = s
		}
		if s.StyleID == "Emphasis" {
			cs = s
		}
	}
	if ps == nil {
		t.Fatal("missing Heading1 style")
	}
	if ps.Next == nil || ps.Next.Val != "Normal" {
		t.Fatalf("next = %v, want Normal", ps.Next)
	}
	if ps.UIPriority == nil || ps.UIPriority.Val != 9 {
		t.Fatalf("uiPriority = %v, want 9", ps.UIPriority)
	}
	if ps.RSID != "12345678" {
		t.Fatalf("rsid = %q, want 12345678", ps.RSID)
	}
	if ps.Unhide == nil || ps.Unhide.Val == nil || !*ps.Unhide.Val {
		t.Fatal("expected unhideWhenUsed = true")
	}
	if ps.QuickFormat == nil || ps.QuickFormat.Val == nil || !*ps.QuickFormat.Val {
		t.Fatal("expected qFormat = true")
	}
	if ps.SemiHidden == nil || ps.SemiHidden.Val == nil || !*ps.SemiHidden.Val {
		t.Fatal("expected semiHidden = true")
	}
	if ps.AutoRedef == nil || ps.AutoRedef.Val == nil || !*ps.AutoRedef.Val {
		t.Fatal("expected autoRedefine = true")
	}
	if cs == nil {
		t.Fatal("missing Emphasis style")
	}
	if cs.Next == nil || cs.Next.Val != "Emphasis" {
		t.Fatalf("next = %v, want Emphasis", cs.Next)
	}
	if cs.UIPriority == nil || cs.UIPriority.Val != 20 {
		t.Fatalf("uiPriority = %v, want 20", cs.UIPriority)
	}
	if cs.RSID != "87654321" {
		t.Fatalf("rsid = %q, want 87654321", cs.RSID)
	}
	if cs.Unhide == nil || cs.Unhide.Val == nil || !*cs.Unhide.Val {
		t.Fatal("expected unhideWhenUsed = true")
	}
}

func TestConvertParagraphStyleWithoutTableStyles(t *testing.T) {
	doc := &wordir.Document{
		Styles: map[string]wordir.Style{
			"Normal": {
				ID:   "Normal",
				Name: "Normal",
				Format: wordir.ParagraphFormat{
					Alignment: "left",
				},
			},
		},
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pkg.Styles.Styles) != 1 {
		t.Fatalf("expected 1 style, got %d", len(pkg.Styles.Styles))
	}
	if pkg.Styles.Styles[0].Type != "paragraph" {
		t.Fatalf("type = %q, want paragraph", pkg.Styles.Styles[0].Type)
	}
	if len(pkg.Styles.Styles[0].TableStyles) != 0 {
		t.Fatalf("expected no table styles, got %d", len(pkg.Styles.Styles[0].TableStyles))
	}
}

func TestConvertField(t *testing.T) {
	doc := &wordir.Document{
		Paragraphs: []wordir.Paragraph{{
			Runs: []wordir.Run{{
				Text:       "1",
				FieldInstr: " PAGE ",
			}},
		}},
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rc := pkg.Document.Body.Paragraphs[0].Runs[0].Contents
	if len(rc) != 5 {
		t.Fatalf("expected 5 run contents, got %d", len(rc))
	}
	if rc[0].Kind != wordparts.RunContentFldChar || rc[0].FldChar == nil || rc[0].FldChar.Type != "begin" {
		t.Fatalf("content[0] = %v, want fldChar begin", rc[0])
	}
	if rc[1].Kind != wordparts.RunContentInstrText || rc[1].InstrText != " PAGE " {
		t.Fatalf("content[1] = %v, want instrText ' PAGE '", rc[1])
	}
	if rc[2].Kind != wordparts.RunContentFldChar || rc[2].FldChar == nil || rc[2].FldChar.Type != "separate" {
		t.Fatalf("content[2] = %v, want fldChar separate", rc[2])
	}
	if rc[3].Kind != wordparts.RunContentText || rc[3].Text == nil || rc[3].Text.Value != "1" {
		t.Fatalf("content[3] = %v, want text '1'", rc[3])
	}
	if rc[4].Kind != wordparts.RunContentFldChar || rc[4].FldChar == nil || rc[4].FldChar.Type != "end" {
		t.Fatalf("content[4] = %v, want fldChar end", rc[4])
	}
}

func TestConvertFieldWithoutText(t *testing.T) {
	doc := &wordir.Document{
		Paragraphs: []wordir.Paragraph{{
			Runs: []wordir.Run{{
				FieldInstr: " DATE ",
			}},
		}},
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rc := pkg.Document.Body.Paragraphs[0].Runs[0].Contents
	if len(rc) != 4 {
		t.Fatalf("expected 4 run contents, got %d", len(rc))
	}
	if rc[0].Kind != wordparts.RunContentFldChar || rc[0].FldChar.Type != "begin" {
		t.Fatalf("content[0] = %v, want fldChar begin", rc[0])
	}
	if rc[1].Kind != wordparts.RunContentInstrText || rc[1].InstrText != " DATE " {
		t.Fatalf("content[1] = %v, want instrText ' DATE '", rc[1])
	}
	if rc[2].Kind != wordparts.RunContentFldChar || rc[2].FldChar.Type != "separate" {
		t.Fatalf("content[2] = %v, want fldChar separate", rc[2])
	}
	if rc[3].Kind != wordparts.RunContentFldChar || rc[3].FldChar.Type != "end" {
		t.Fatalf("content[3] = %v, want fldChar end", rc[3])
	}
}

func TestConvertFormFieldText(t *testing.T) {
	doc := &wordir.Document{
		Paragraphs: []wordir.Paragraph{{
			Runs: []wordir.Run{{
				Kind: "formField",
				FormField: &wordir.FormField{
					Type:        "text",
					Name:        "Text1",
					DefaultText: "Hello",
					MaxLength:   50,
					TextType:    "regular",
				},
				Text: "Hello",
			}},
		}},
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rc := pkg.Document.Body.Paragraphs[0].Runs[0].Contents
	if len(rc) != 4 {
		t.Fatalf("expected 4 run contents, got %d", len(rc))
	}
	if rc[0].Kind != wordparts.RunContentFldChar || rc[0].FldChar == nil || rc[0].FldChar.Type != "begin" {
		t.Fatalf("content[0] = %v, want fldChar begin", rc[0])
	}
	if rc[0].FldChar.FormField == nil {
		t.Fatal("expected form field data")
	}
	ff := rc[0].FldChar.FormField
	if ff.Name != "Text1" {
		t.Fatalf("name = %q, want Text1", ff.Name)
	}
	if ff.Kind != "text" {
		t.Fatalf("kind = %q, want text", ff.Kind)
	}
	if ff.DefaultTxt != "Hello" {
		t.Fatalf("defaultTxt = %q, want Hello", ff.DefaultTxt)
	}
	if ff.MaxLength != 50 {
		t.Fatalf("maxLength = %d, want 50", ff.MaxLength)
	}
	if rc[1].Kind != wordparts.RunContentFldChar || rc[1].FldChar.Type != "separate" {
		t.Fatalf("content[1] = %v, want fldChar separate", rc[1])
	}
	if rc[2].Kind != wordparts.RunContentText || rc[2].Text.Value != "Hello" {
		t.Fatalf("content[2] = %v, want text Hello", rc[2])
	}
	if rc[3].Kind != wordparts.RunContentFldChar || rc[3].FldChar.Type != "end" {
		t.Fatalf("content[3] = %v, want fldChar end", rc[3])
	}
}

func TestConvertFormFieldCheckbox(t *testing.T) {
	doc := &wordir.Document{
		Paragraphs: []wordir.Paragraph{{
			Runs: []wordir.Run{{
				Kind: "formField",
				FormField: &wordir.FormField{
					Type:             "checkbox",
					Name:             "Check1",
					Checked:          true,
					CheckboxSizeAuto: true,
				},
			}},
		}},
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rc := pkg.Document.Body.Paragraphs[0].Runs[0].Contents
	if len(rc) != 3 {
		t.Fatalf("expected 3 run contents, got %d", len(rc))
	}
	ff := rc[0].FldChar.FormField
	if ff.Kind != "checkbox" {
		t.Fatalf("kind = %q, want checkbox", ff.Kind)
	}
	if !ff.Checked {
		t.Fatal("expected checked")
	}
	if !ff.CheckboxSizeAuto {
		t.Fatal("expected sizeAuto")
	}
}

func TestConvertFormFieldDropdown(t *testing.T) {
	doc := &wordir.Document{
		Paragraphs: []wordir.Paragraph{{
			Runs: []wordir.Run{{
				Kind: "formField",
				FormField: &wordir.FormField{
					Type:       "dropdown",
					Name:       "Drop1",
					Items:      []string{"A", "B", "C"},
					DefaultIdx: 1,
				},
				Text: "B",
			}},
		}},
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rc := pkg.Document.Body.Paragraphs[0].Runs[0].Contents
	if len(rc) != 4 {
		t.Fatalf("expected 4 run contents, got %d", len(rc))
	}
	ff := rc[0].FldChar.FormField
	if len(ff.DropItems) != 3 || ff.DropItems[1] != "B" {
		t.Fatalf("dropItems = %v, want [A B C]", ff.DropItems)
	}
	if ff.DropDefIdx != 1 {
		t.Fatalf("dropDefIdx = %d, want 1", ff.DropDefIdx)
	}
}

func TestConvertParagraphMiscProperties(t *testing.T) {
	doc := &wordir.Document{
		Paragraphs: []wordir.Paragraph{{
			Runs: []wordir.Run{{Text: "hello"}},
			Format: wordir.ParagraphFormat{
				Kinsoku:        true,
				WordWrap:       boolPtr(true),
				OverflowPunct:  true,
				TopLinePunct:   true,
				AutoSpaceDE:    true,
				AutoSpaceDN:    true,
				MirrorIndents:  true,
				NoAllowOverlap: true,
				AdjustRightInd: true,
				FontAlign:      "center",
			},
		}},
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	pp := pkg.Document.Body.Paragraphs[0].Properties
	if pp.Kinsoku == nil || !*pp.Kinsoku.Val {
		t.Fatal("expected kinsoku")
	}
	if pp.WordWrap == nil || !*pp.WordWrap.Val {
		t.Fatal("expected wordWrap")
	}
	if pp.OverflowPunct == nil || !*pp.OverflowPunct.Val {
		t.Fatal("expected overflowPunct")
	}
	if pp.TopLinePunct == nil || !*pp.TopLinePunct.Val {
		t.Fatal("expected topLinePunct")
	}
	if pp.AutoSpaceDE == nil || !*pp.AutoSpaceDE.Val {
		t.Fatal("expected autoSpaceDE")
	}
	if pp.AutoSpaceDN == nil || !*pp.AutoSpaceDN.Val {
		t.Fatal("expected autoSpaceDN")
	}
	if pp.MirrorIndents == nil || !*pp.MirrorIndents.Val {
		t.Fatal("expected mirrorIndents")
	}
	if pp.NoAllowOverlap == nil || !*pp.NoAllowOverlap.Val {
		t.Fatal("expected noAllowOverlap")
	}
	if pp.AdjustRightInd == nil || !*pp.AdjustRightInd.Val {
		t.Fatal("expected adjustRightInd")
	}
	if pp.TextAlignment == nil || pp.TextAlignment.Val != "center" {
		t.Fatalf("textAlignment = %v, want center", pp.TextAlignment)
	}
}

func TestConvertRunMiscProperties(t *testing.T) {
	doc := &wordir.Document{
		Paragraphs: []wordir.Paragraph{{
			Runs: []wordir.Run{{
				Text: "hello",
				Format: wordir.RunFormat{
					Outline:       boolPtr(true),
					Shadow:        boolPtr(true),
					Emboss:        boolPtr(true),
					Imprint:       boolPtr(true),
					ComplexScript: boolPtr(true),
					Borders: &wordir.RunBorders{
						Top: &wordir.BorderSide{Val: "single", SizePt: 4, Color: "FF0000"},
					},
				},
			}},
		}},
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rp := pkg.Document.Body.Paragraphs[0].Runs[0].Properties
	if rp.OutlineToggle == nil || !*rp.OutlineToggle.Val {
		t.Fatal("expected outline")
	}
	if rp.ShadowToggle == nil || !*rp.ShadowToggle.Val {
		t.Fatal("expected shadow")
	}
	if rp.Emboss == nil || !*rp.Emboss.Val {
		t.Fatal("expected emboss")
	}
	if rp.Imprint == nil || !*rp.Imprint.Val {
		t.Fatal("expected imprint")
	}
	if rp.CS == nil || !*rp.CS.Val {
		t.Fatal("expected cs")
	}
	if rp.Borders == nil || rp.Borders.Val != "single" {
		t.Fatalf("run borders = %v, want single", rp.Borders)
	}
}

func TestConvertRuby(t *testing.T) {
	doc := &wordir.Document{
		Paragraphs: []wordir.Paragraph{{
			Runs: []wordir.Run{{
				Kind: "ruby",
				Ruby: &wordir.RubyContent{
					Base: "東京",
					Text: "とうきょう",
				},
			}},
		}},
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rc := pkg.Document.Body.Paragraphs[0].Runs[0].Contents
	if len(rc) != 1 {
		t.Fatalf("expected 1 run content, got %d", len(rc))
	}
	if rc[0].Kind != wordparts.RunContentRuby {
		t.Fatalf("kind = %v, want ruby", rc[0].Kind)
	}
	if rc[0].Ruby == nil {
		t.Fatal("expected ruby content")
	}
	if rc[0].Ruby.Base != "東京" {
		t.Fatalf("base = %q, want 東京", rc[0].Ruby.Base)
	}
	if rc[0].Ruby.Text != "とうきょう" {
		t.Fatalf("text = %q, want とうきょう", rc[0].Ruby.Text)
	}
}

func TestConvertBookmarks(t *testing.T) {
	doc := &wordir.Document{
		Paragraphs: []wordir.Paragraph{{
			Runs:      []wordir.Run{{Text: "Hello"}},
			Bookmarks: []string{"bm1", "bm2"},
		}},
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	p := pkg.Document.Body.Paragraphs[0]
	if len(p.BookmarkStartElems) != 2 {
		t.Fatalf("expected 2 bookmark starts, got %d", len(p.BookmarkStartElems))
	}
	if p.BookmarkStartElems[0].Name != "bm1" {
		t.Errorf("bookmark[0] name = %q, want bm1", p.BookmarkStartElems[0].Name)
	}
	if p.BookmarkStartElems[1].Name != "bm2" {
		t.Errorf("bookmark[1] name = %q, want bm2", p.BookmarkStartElems[1].Name)
	}
	if len(p.BookmarkEndElems) != 2 {
		t.Fatalf("expected 2 bookmark ends, got %d", len(p.BookmarkEndElems))
	}
	if p.BookmarkEndElems[0].ID != p.BookmarkStartElems[0].ID {
		t.Errorf("bookmark end id mismatch: end=%d start=%d", p.BookmarkEndElems[0].ID, p.BookmarkStartElems[0].ID)
	}
}

func TestConvertMath(t *testing.T) {
	doc := &wordir.Document{
		Paragraphs: []wordir.Paragraph{{
			Runs: []wordir.Run{{
				Kind:   "math",
				MathML: `<m:r><m:t>x</m:t></m:r>`,
			}},
		}},
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rc := pkg.Document.Body.Paragraphs[0].Runs[0].Contents
	if len(rc) != 1 {
		t.Fatalf("expected 1 run content, got %d", len(rc))
	}
	if rc[0].Kind != wordparts.RunContentMath {
		t.Fatalf("kind = %v, want math", rc[0].Kind)
	}
	if rc[0].MathML != `<m:r><m:t>x</m:t></m:r>` {
		t.Errorf("MathML = %q, want raw math", rc[0].MathML)
	}
}

func TestConvertFontTable(t *testing.T) {
	doc := &wordir.Document{
		FontFamilies: map[string]wordir.FontFamilyInfo{
			"Arial":           {Family: "swiss", Pitch: "variable", Charset: "00", Panose1: "020B0604020202020204"},
			"Times New Roman": {Family: "roman", Pitch: "variable"},
		},
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pkg.FontTable == nil {
		t.Fatal("expected font table")
	}
	if len(pkg.FontTable.Fonts) != 2 {
		t.Fatalf("expected 2 fonts, got %d", len(pkg.FontTable.Fonts))
	}
	fontMap := make(map[string]*wordparts.Font)
	for _, f := range pkg.FontTable.Fonts {
		fontMap[f.Name] = f
	}
	arial := fontMap["Arial"]
	if arial == nil {
		t.Fatal("missing Arial font")
	}
	if arial.Family == nil || arial.Family.Val != "swiss" {
		t.Errorf("Arial family = %v, want swiss", arial.Family)
	}
	if arial.Pitch == nil || arial.Pitch.Val != "variable" {
		t.Errorf("Arial pitch = %v, want variable", arial.Pitch)
	}
	if arial.Panose1 == nil || arial.Panose1.Val != "020B0604020202020204" {
		t.Errorf("Arial panose1 = %v", arial.Panose1)
	}
}

func TestConvertEmbeddedFonts(t *testing.T) {
	doc := &wordir.Document{
		FontFamilies: map[string]wordir.FontFamilyInfo{
			"CustomFont": {Family: "swiss"},
		},
		EmbeddedFonts: []*wordir.EmbeddedFont{
			{
				Family: "CustomFont",
				Faces: []*wordir.EmbeddedFontFace{
					{Weight: 400, Style: "normal", ContentType: "application/vnd.ms-fontobject", Data: []byte("regular")},
					{Weight: 700, Style: "normal", ContentType: "application/vnd.ms-fontobject", Data: []byte("bold")},
				},
			},
		},
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pkg.EmbeddedFonts) != 1 {
		t.Fatalf("expected 1 embedded font data, got %d", len(pkg.EmbeddedFonts))
	}
	if pkg.EmbeddedFonts[0].Regular == nil {
		t.Error("expected regular font part")
	}
	if pkg.EmbeddedFonts[0].Bold == nil {
		t.Error("expected bold font part")
	}
	// Check font table has embed refs
	var customFont *wordparts.Font
	for _, f := range pkg.FontTable.Fonts {
		if f.Name == "CustomFont" {
			customFont = f
			break
		}
	}
	if customFont == nil {
		t.Fatal("missing CustomFont in font table")
	}
	if customFont.EmbedRegular == nil || customFont.EmbedRegular.RelID == "" {
		t.Error("expected EmbedRegular relID")
	}
	if customFont.EmbedBold == nil || customFont.EmbedBold.RelID == "" {
		t.Error("expected EmbedBold relID")
	}
}

func TestConvertTheme(t *testing.T) {
	doc := &wordir.Document{}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pkg.Theme == nil {
		t.Fatal("expected theme")
	}
	if pkg.Theme.Elements == nil {
		t.Fatal("expected theme elements")
	}
	if pkg.Theme.Elements.ColorScheme == nil {
		t.Fatal("expected color scheme")
	}
	if pkg.Theme.Elements.FontScheme == nil {
		t.Fatal("expected font scheme")
	}
}

func TestConvertSettingsCompleteness(t *testing.T) {
	doc := &wordir.Document{
		Paragraphs:             []wordir.Paragraph{{Runs: []wordir.Run{{Text: "Hello"}}}},
		ConsecutiveHyphenLimit: 2,
		HyphenationZone:        360,
		DoNotHyphenateCaps:     true,
		UpdateFields:           true,
		ThemeFontLang:          &wordir.ThemeFontLang{Val: "en-US", EastAsia: "ja-JP", Bidi: "ar-SA"},
		ProofState:             &wordir.ProofState{Spelling: "clean", Grammar: "dirty"},
		MailMerge: &wordir.MailMergeSettings{
			MainDocumentType:        "letter",
			LinkToQuery:             true,
			DataType:                "database",
			ConnectString:           "Provider=SQLOLEDB",
			Query:                   "SELECT * FROM Customers",
			DataSource:              "data.csv",
			HeaderSource:            "header.csv",
			Destination:             "printer",
			ActiveRecord:            "4",
			AddressFieldName:        "MailingAddress",
			CheckErrors:             "2",
			DoNotSuppressBlankLines: true,
			DynamicAddress:          true,
			EntryMacro:              "OnMergeStart",
			ExitMacro:               "OnMergeEnd",
			DataSourceHasHeaders:    true,
			LinkedToFile:            true,
			MailAsAttachment:        true,
			MailSubject:             "Quarterly Update",
			MappedName:              "RecipientName",
			SourceFileName:          "records.csv",
			StoreMappedDataAs:       "text",
			SubDoc:                  "BatchA",
			UDL:                     "Provider=SQLOLEDB",
			ViewMergedData:          true,
			Personal:                true,
			PersonalCompose:         true,
			PersonalReply:           true,
			Types:                   []string{"personalizedLetters", "email"},
			Recipients: []wordir.RecipientData{
				{Active: true},
				{Active: false},
			},
			ODSO: &wordir.ODSOSettings{
				ColDelim:       124,
				DynamicAddress: true,
				FHdr:           true,
				SourceFileName: "odso.csv",
				UDL:            "Provider=ODBC",
				FieldMapData: []wordir.FieldMapData{
					{MappedName: "FirstName", Name: "first_name", Lang: "en-US", Val: "firstName", Column: 1},
					{MappedName: "LastName", Name: "last_name", Lang: "en-US", Val: "lastName", Column: 2},
				},
				RecipientData: []wordir.RecipientData{
					{Active: true},
					{Active: false},
				},
			},
		},
		DocumentType: "letter",
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := pkg.Settings
	if s == nil {
		t.Fatal("expected settings")
	}
	if s.ConsecutiveHyphenLimit == nil || s.ConsecutiveHyphenLimit.Val != 2 {
		t.Errorf("ConsecutiveHyphenLimit = %v, want 2", s.ConsecutiveHyphenLimit)
	}
	if s.HyphenationZone == nil || s.HyphenationZone.Val != 360 {
		t.Errorf("HyphenationZone = %v, want 360", s.HyphenationZone)
	}
	if s.DoNotHyphenateCaps == nil || !bool(*s.DoNotHyphenateCaps.Val) {
		t.Errorf("DoNotHyphenateCaps = %v, want true", s.DoNotHyphenateCaps)
	}
	if s.UpdateFields == nil || !bool(*s.UpdateFields.Val) {
		t.Errorf("UpdateFields = %v, want true", s.UpdateFields)
	}
	if s.ThemeFontLang == nil || s.ThemeFontLang.Val != "en-US" {
		t.Errorf("ThemeFontLang = %v, want en-US", s.ThemeFontLang)
	}
	if s.ProofState == nil || s.ProofState.Spelling != "clean" {
		t.Errorf("ProofState = %v", s.ProofState)
	}
	if s.MailMerge == nil || s.MailMerge.MainDocumentType != "letter" {
		t.Errorf("MailMerge = %v", s.MailMerge)
	}
	if s.MailMerge == nil || !s.MailMerge.LinkToQuery || s.MailMerge.DataType != "database" || s.MailMerge.ConnectString != "Provider=SQLOLEDB" {
		t.Errorf("MailMerge core fields = %+v", s.MailMerge)
	}
	if s.MailMerge == nil || s.MailMerge.Query != "SELECT * FROM Customers" || s.MailMerge.HeaderSource != "header.csv" {
		t.Errorf("MailMerge query/header = %+v", s.MailMerge)
	}
	if s.MailMerge == nil || s.MailMerge.ActiveRecord == nil || s.MailMerge.ActiveRecord.Val != "4" {
		t.Errorf("MailMerge.ActiveRecord = %+v", s.MailMerge)
	}
	if s.MailMerge == nil || s.MailMerge.AddressFieldName == nil || s.MailMerge.AddressFieldName.Val != "MailingAddress" {
		t.Errorf("MailMerge.AddressFieldName = %+v", s.MailMerge)
	}
	if s.MailMerge == nil || s.MailMerge.CheckErrors == nil || s.MailMerge.CheckErrors.Val != "2" {
		t.Errorf("MailMerge.CheckErrors = %+v", s.MailMerge)
	}
	if s.MailMerge == nil || s.MailMerge.DoNotSuppressBlankLines == nil || !bool(*s.MailMerge.DoNotSuppressBlankLines.Val) {
		t.Errorf("MailMerge.DoNotSuppressBlankLines = %+v", s.MailMerge)
	}
	if s.MailMerge == nil || s.MailMerge.DynamicAddress == nil || !bool(*s.MailMerge.DynamicAddress.Val) {
		t.Errorf("MailMerge.DynamicAddress = %+v", s.MailMerge)
	}
	if s.MailMerge == nil || s.MailMerge.EntryMacro == nil || s.MailMerge.EntryMacro.Val != "OnMergeStart" {
		t.Errorf("MailMerge.EntryMacro = %+v", s.MailMerge)
	}
	if s.MailMerge == nil || s.MailMerge.ExitMacro == nil || s.MailMerge.ExitMacro.Val != "OnMergeEnd" {
		t.Errorf("MailMerge.ExitMacro = %+v", s.MailMerge)
	}
	if s.MailMerge == nil || s.MailMerge.FHdr == nil || !bool(*s.MailMerge.FHdr.Val) {
		t.Errorf("MailMerge.FHdr = %+v", s.MailMerge)
	}
	if s.MailMerge == nil || s.MailMerge.LinkedToFile == nil || !bool(*s.MailMerge.LinkedToFile.Val) {
		t.Errorf("MailMerge.LinkedToFile = %+v", s.MailMerge)
	}
	if s.MailMerge == nil || s.MailMerge.MailAsAttachment == nil || !bool(*s.MailMerge.MailAsAttachment.Val) {
		t.Errorf("MailMerge.MailAsAttachment = %+v", s.MailMerge)
	}
	if s.MailMerge == nil || s.MailMerge.MailSubject == nil || s.MailMerge.MailSubject.Val != "Quarterly Update" {
		t.Errorf("MailMerge.MailSubject = %+v", s.MailMerge)
	}
	if s.MailMerge == nil || s.MailMerge.MappedName == nil || s.MailMerge.MappedName.Val != "RecipientName" {
		t.Errorf("MailMerge.MappedName = %+v", s.MailMerge)
	}
	if s.MailMerge == nil || s.MailMerge.SourceFileName == nil || s.MailMerge.SourceFileName.Val != "records.csv" {
		t.Errorf("MailMerge.SourceFileName = %+v", s.MailMerge)
	}
	if s.MailMerge == nil || s.MailMerge.StoreMappedDataAs == nil || s.MailMerge.StoreMappedDataAs.Val != "text" {
		t.Errorf("MailMerge.StoreMappedDataAs = %+v", s.MailMerge)
	}
	if s.MailMerge == nil || s.MailMerge.SubDoc == nil || s.MailMerge.SubDoc.Val != "BatchA" {
		t.Errorf("MailMerge.SubDoc = %+v", s.MailMerge)
	}
	if s.MailMerge == nil || s.MailMerge.UDL == nil || s.MailMerge.UDL.Val != "Provider=SQLOLEDB" {
		t.Errorf("MailMerge.UDL = %+v", s.MailMerge)
	}
	if s.MailMerge == nil || s.MailMerge.ViewMergedData == nil || !bool(*s.MailMerge.ViewMergedData.Val) {
		t.Errorf("MailMerge.ViewMergedData = %+v", s.MailMerge)
	}
	if s.MailMerge == nil || s.MailMerge.Personal == nil || !bool(*s.MailMerge.Personal.Val) {
		t.Errorf("MailMerge.Personal = %+v", s.MailMerge)
	}
	if s.MailMerge == nil || s.MailMerge.PersonalCompose == nil || !bool(*s.MailMerge.PersonalCompose.Val) {
		t.Errorf("MailMerge.PersonalCompose = %+v", s.MailMerge)
	}
	if s.MailMerge == nil || s.MailMerge.PersonalReply == nil || !bool(*s.MailMerge.PersonalReply.Val) {
		t.Errorf("MailMerge.PersonalReply = %+v", s.MailMerge)
	}
	if s.MailMerge == nil || s.MailMerge.Types == nil || len(s.MailMerge.Types.Types) != 2 || s.MailMerge.Types.Types[0].Val != "personalizedLetters" || s.MailMerge.Types.Types[1].Val != "email" {
		t.Errorf("MailMerge.Types = %+v", s.MailMerge)
	}
	if s.MailMerge == nil || s.MailMerge.Recipients == nil || len(s.MailMerge.Recipients.RecipientData) != 2 || s.MailMerge.Recipients.RecipientData[0].Active == nil || !bool(*s.MailMerge.Recipients.RecipientData[0].Active.Val) || s.MailMerge.Recipients.RecipientData[1].Active != nil {
		t.Errorf("MailMerge.Recipients = %+v", s.MailMerge)
	}
	if s.MailMerge == nil || s.MailMerge.ODSO == nil {
		t.Fatal("expected MailMerge.ODSO")
	}
	if s.MailMerge.ODSO.ColDelim == nil || s.MailMerge.ODSO.ColDelim.Val != 124 {
		t.Errorf("MailMerge.ODSO.ColDelim = %+v", s.MailMerge.ODSO)
	}
	if s.MailMerge.ODSO.DynamicAddress == nil || !bool(*s.MailMerge.ODSO.DynamicAddress.Val) || s.MailMerge.ODSO.FHdr == nil || !bool(*s.MailMerge.ODSO.FHdr.Val) {
		t.Errorf("MailMerge.ODSO flags = %+v", s.MailMerge.ODSO)
	}
	if s.MailMerge.ODSO.SourceFileName == nil || s.MailMerge.ODSO.SourceFileName.Val != "odso.csv" || s.MailMerge.ODSO.UDL == nil || s.MailMerge.ODSO.UDL.Val != "Provider=ODBC" {
		t.Errorf("MailMerge.ODSO source/udl = %+v", s.MailMerge.ODSO)
	}
	if len(s.MailMerge.ODSO.FieldMapData) != 2 || s.MailMerge.ODSO.FieldMapData[0].MappedName != "FirstName" || s.MailMerge.ODSO.FieldMapData[1].MappedName != "LastName" {
		t.Errorf("MailMerge.ODSO.FieldMapData = %+v", s.MailMerge.ODSO.FieldMapData)
	}
	if len(s.MailMerge.ODSO.RecipientData) != 2 || s.MailMerge.ODSO.RecipientData[0].Active == nil || !bool(*s.MailMerge.ODSO.RecipientData[0].Active.Val) || s.MailMerge.ODSO.RecipientData[1].Active != nil {
		t.Errorf("MailMerge.ODSO.RecipientData = %+v", s.MailMerge.ODSO.RecipientData)
	}
	if s.DocumentType == nil || s.DocumentType.Val != "letter" {
		t.Errorf("DocumentType = %v, want letter", s.DocumentType)
	}
}

func TestConvertSDTRun(t *testing.T) {
	doc := &wordir.Document{
		Paragraphs: []wordir.Paragraph{{
			Runs: []wordir.Run{
				{Text: "before"},
				{
					Text: "sdt text",
					ContentControl: &wordir.ContentControl{
						ID:    42,
						Tag:   "tag1",
						Title: "My SDT",
						Type:  "plainText",
					},
				},
				{Text: "after"},
			},
		}},
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	p := pkg.Document.Body.Paragraphs[0]
	if len(p.Inlines) != 3 {
		t.Fatalf("expected 3 inlines, got %d", len(p.Inlines))
	}
	if p.Inlines[0].Kind != wordparts.ParagraphInlineRun {
		t.Errorf("inline[0] kind = %v, want run", p.Inlines[0].Kind)
	}
	if p.Inlines[1].Kind != wordparts.ParagraphInlineSDT {
		t.Errorf("inline[1] kind = %v, want sdt", p.Inlines[1].Kind)
	}
	if p.Inlines[1].SdtProps == nil {
		t.Fatal("expected SDT properties")
	}
	if p.Inlines[1].SdtProps.ID != 42 {
		t.Errorf("SDT ID = %d, want 42", p.Inlines[1].SdtProps.ID)
	}
	if p.Inlines[1].SdtProps.Tag != "tag1" {
		t.Errorf("SDT Tag = %q, want tag1", p.Inlines[1].SdtProps.Tag)
	}
	if p.Inlines[2].Kind != wordparts.ParagraphInlineRun {
		t.Errorf("inline[2] kind = %v, want run", p.Inlines[2].Kind)
	}
}

func TestConvertRevisionMarks(t *testing.T) {
	doc := &wordir.Document{
		Paragraphs: []wordir.Paragraph{{
			Runs: []wordir.Run{
				{Text: "normal"},
				{
					Text:           "inserted",
					Revision:       "inserted",
					RevisionAuthor: "Alice",
					RevisionDate:   "2024-01-01",
				},
				{
					Text:           "deleted",
					Revision:       "deleted",
					RevisionAuthor: "Bob",
					RevisionDate:   "2024-01-02",
				},
			},
		}},
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	p := pkg.Document.Body.Paragraphs[0]
	if len(p.Inlines) != 3 {
		t.Fatalf("expected 3 inlines, got %d", len(p.Inlines))
	}
	if p.Inlines[0].Kind != wordparts.ParagraphInlineRun {
		t.Errorf("inline[0] kind = %v, want run", p.Inlines[0].Kind)
	}
	if p.Inlines[1].Kind != wordparts.ParagraphInlineIns {
		t.Errorf("inline[1] kind = %v, want ins", p.Inlines[1].Kind)
	}
	if p.Inlines[1].RevisionAuthor != "Alice" {
		t.Errorf("ins author = %q, want Alice", p.Inlines[1].RevisionAuthor)
	}
	if p.Inlines[2].Kind != wordparts.ParagraphInlineDel {
		t.Errorf("inline[2] kind = %v, want del", p.Inlines[2].Kind)
	}
	if p.Inlines[2].RevisionAuthor != "Bob" {
		t.Errorf("del author = %q, want Bob", p.Inlines[2].RevisionAuthor)
	}
}

func TestConvertIntermediateSectionBreak(t *testing.T) {
	doc := &wordir.Document{
		Blocks: []wordir.Block{
			{Kind: "paragraph", Paragraph: &wordir.Paragraph{Runs: []wordir.Run{{Text: "First"}}}},
			{Kind: "section", Section: &wordir.Section{BreakKind: "nextPage"}},
			{Kind: "paragraph", Paragraph: &wordir.Paragraph{Runs: []wordir.Run{{Text: "Second"}}}},
			{Kind: "section", Section: &wordir.Section{BreakKind: "continuous"}},
		},
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// First paragraph should have sectPr in its properties
	p1 := pkg.Document.Body.Paragraphs[0]
	if p1.Properties == nil || p1.Properties.Section == nil {
		t.Fatal("expected first paragraph to have section properties")
	}
	if p1.Properties.Section.Type == nil || p1.Properties.Section.Type.Val != "nextPage" {
		t.Errorf("first paragraph section type = %v, want nextPage", p1.Properties.Section.Type)
	}
	// Second paragraph should NOT have section properties
	p2 := pkg.Document.Body.Paragraphs[1]
	if p2.Properties != nil && p2.Properties.Section != nil {
		t.Error("expected second paragraph to NOT have section properties")
	}
	// Body should have exactly 1 section block (the trailing one)
	sectCount := 0
	for _, b := range pkg.Document.Body.Blocks {
		if b.Kind == wordparts.BodyBlockSection {
			sectCount++
		}
	}
	if sectCount != 1 {
		t.Fatalf("expected 1 section block, got %d", sectCount)
	}
}

func TestConvertShapeInline(t *testing.T) {
	doc := &wordir.Document{
		Paragraphs: []wordir.Paragraph{{
			Runs: []wordir.Run{{
				Kind: "shape",
				Shape: &wordir.Shape{
					Floating:    false,
					Width:       914400,
					Height:      457200,
					FillColor:   "FF0000",
					StrokeColor: "000000",
					StrokeWidth: 12700,
					Geometry:    &dml.PresetGeometry{Name: "rect"},
				},
			}},
		}},
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rc := pkg.Document.Body.Paragraphs[0].Runs[0].Contents
	if len(rc) != 1 {
		t.Fatalf("expected 1 run content, got %d", len(rc))
	}
	if rc[0].Kind != wordparts.RunContentDrawing {
		t.Fatalf("kind = %v, want drawing", rc[0].Kind)
	}
	xmlStr := string(rc[0].DrawingXML)
	if !strings.Contains(xmlStr, "<wp:inline") {
		t.Errorf("expected inline element, got: %s", xmlStr)
	}
	if !strings.Contains(xmlStr, `cx="914400"`) {
		t.Errorf("expected width 914400, got: %s", xmlStr)
	}
	if !strings.Contains(xmlStr, `prst="rect"`) {
		t.Errorf("expected rect geometry, got: %s", xmlStr)
	}
	if !strings.Contains(xmlStr, `val="FF0000"`) {
		t.Errorf("expected fill color, got: %s", xmlStr)
	}
}

func TestConvertShapeFloating(t *testing.T) {
	doc := &wordir.Document{
		Paragraphs: []wordir.Paragraph{{
			Runs: []wordir.Run{{
				Kind: "shape",
				Shape: &wordir.Shape{
					Floating:  true,
					Width:     914400,
					Height:    457200,
					XAlign:    "left",
					XRelative: "margin",
					YAlign:    "top",
					YRelative: "margin",
					WrapType:  "square",
					ZIndex:    1,
				},
			}},
		}},
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rc := pkg.Document.Body.Paragraphs[0].Runs[0].Contents
	xmlStr := string(rc[0].DrawingXML)
	if !strings.Contains(xmlStr, "<wp:anchor") {
		t.Errorf("expected anchor element, got: %s", xmlStr)
	}
	if !strings.Contains(xmlStr, `relativeFrom="margin"`) {
		t.Errorf("expected margin relative, got: %s", xmlStr)
	}
	if !strings.Contains(xmlStr, "<wp:align>left</wp:align>") {
		t.Errorf("expected left align, got: %s", xmlStr)
	}
}

func TestConvertParagraphPropertyRevision(t *testing.T) {
	doc := &wordir.Document{
		Paragraphs: []wordir.Paragraph{{
			Runs: []wordir.Run{{Text: "hello"}},
			PropertyRevision: &wordir.RevisionMark{
				Author: "Alice",
				Date:   "2024-01-15T09:00:00Z",
			},
		}},
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	p := pkg.Document.Body.Paragraphs[0]
	if p.Properties == nil {
		t.Fatal("expected paragraph properties")
	}
	if p.Properties.PropertyChange == nil {
		t.Fatal("expected PropertyChange")
	}
	if p.Properties.PropertyChange.Author != "Alice" {
		t.Errorf("author = %q, want Alice", p.Properties.PropertyChange.Author)
	}
}

func TestConvertRunPropertyRevision(t *testing.T) {
	doc := &wordir.Document{
		Paragraphs: []wordir.Paragraph{{
			Runs: []wordir.Run{{
				Text: "hello",
				PropertyRevision: &wordir.RevisionMark{
					Author: "Bob",
					Date:   "2024-02-01T10:30:00Z",
				},
			}},
		}},
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	r := pkg.Document.Body.Paragraphs[0].Runs[0]
	if r.Properties == nil {
		t.Fatal("expected run properties")
	}
	if r.Properties.PropertyChange == nil {
		t.Fatal("expected PropertyChange")
	}
	if r.Properties.PropertyChange.Author != "Bob" {
		t.Errorf("author = %q, want Bob", r.Properties.PropertyChange.Author)
	}
}

func TestConvertMoveRevisions(t *testing.T) {
	doc := &wordir.Document{
		Paragraphs: []wordir.Paragraph{{
			Runs: []wordir.Run{
				{Text: "old text", Revision: "movedFrom", RevisionAuthor: "Alice", RevisionDate: "2024-01-01", MoveID: 7},
				{Text: "new text", Revision: "movedTo", RevisionAuthor: "Alice", RevisionDate: "2024-01-01", MoveID: 7},
			},
		}},
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	p := pkg.Document.Body.Paragraphs[0]
	if len(p.Inlines) != 2 {
		t.Fatalf("expected 2 inlines, got %d", len(p.Inlines))
	}
	if p.Inlines[0].Kind != wordparts.ParagraphInlineMoveFrom {
		t.Errorf("inline[0] kind = %v, want MoveFrom", p.Inlines[0].Kind)
	}
	if p.Inlines[0].MoveID != 7 {
		t.Errorf("inline[0] moveID = %d, want 7", p.Inlines[0].MoveID)
	}
	if p.Inlines[1].Kind != wordparts.ParagraphInlineMoveTo {
		t.Errorf("inline[1] kind = %v, want MoveTo", p.Inlines[1].Kind)
	}
	if p.Inlines[1].MoveID != 7 {
		t.Errorf("inline[1] moveID = %d, want 7", p.Inlines[1].MoveID)
	}
}

func TestConvertImageCropRotationFlip(t *testing.T) {
	doc := &wordir.Document{
		Paragraphs: []wordir.Paragraph{{
			Runs: []wordir.Run{{
				Kind: "image",
				Image: &wordir.Image{
					Data:        []byte("PNG"),
					MIMEType:    "image/png",
					Width:       914400,
					Height:      457200,
					CropLeft:    0.1,
					CropTop:     0.2,
					CropRight:   0.3,
					CropBottom:  0.4,
					RotationDeg: 90,
					FlipH:       true,
					FlipV:       false,
				},
			}},
		}},
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rc := pkg.Document.Body.Paragraphs[0].Runs[0].Contents
	if len(rc) != 1 || rc[0].Kind != wordparts.RunContentDrawing {
		t.Fatal("expected drawing run content")
	}
	drawing := rc[0].Drawing
	if drawing == nil {
		t.Fatal("expected Drawing")
	}
	inline := drawing.Inline
	if inline == nil {
		t.Fatal("expected inline")
	}
	graphic := inline.Graphic
	if graphic == nil || graphic.GraphicData == nil || graphic.GraphicData.Picture == nil {
		t.Fatal("expected picture")
	}
	pic := graphic.GraphicData.Picture
	if pic.BlipFill == nil || pic.BlipFill.SrcRect == nil {
		t.Fatal("expected srcRect for crop")
	}
	sr := pic.BlipFill.SrcRect
	if sr.L != 10000 || sr.T != 20000 || sr.R != 30000 || sr.B != 40000 {
		t.Errorf("srcRect = %+v, want l=10000 t=20000 r=30000 b=40000", sr)
	}
	if pic.ShapeProps == nil || pic.ShapeProps.Xfrm == nil {
		t.Fatal("expected xfrm for rotation/flip")
	}
	xfrm := pic.ShapeProps.Xfrm
	if xfrm.Rot != 5400000 {
		t.Errorf("rot = %d, want 5400000", xfrm.Rot)
	}
	if !xfrm.FlipH {
		t.Error("expected FlipH")
	}
	if xfrm.FlipV {
		t.Error("did not expect FlipV")
	}
}

func TestConvertBlockLevelSDT(t *testing.T) {
	cc := &wordir.ContentControl{ID: 42, Title: "Title", Tag: "tag", Type: "richText"}
	doc := &wordir.Document{
		Blocks: []wordir.Block{
			{Kind: "paragraph", Paragraph: &wordir.Paragraph{Runs: []wordir.Run{{Text: "para1"}}}, ContentControl: cc},
			{Kind: "paragraph", Paragraph: &wordir.Paragraph{Runs: []wordir.Run{{Text: "para2"}}}, ContentControl: cc},
			{Kind: "table", Table: &wordir.Table{Rows: []wordir.TableRow{{Cells: []wordir.TableCell{{Paragraphs: []wordir.Paragraph{{Runs: []wordir.Run{{Text: "cell"}}}}}}}}}, ContentControl: cc},
		},
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	blocks := pkg.Document.Body.Blocks
	if len(blocks) != 1 {
		t.Fatalf("expected 1 top-level block, got %d", len(blocks))
	}
	if blocks[0].Kind != wordparts.BodyBlockSDT {
		t.Fatalf("expected SDT block, got %v", blocks[0].Kind)
	}
	if blocks[0].SdtProps == nil || blocks[0].SdtProps.ID != 42 {
		t.Errorf("sdt props id = %v, want 42", blocks[0].SdtProps)
	}
	sdtBlocks := blocks[0].SdtBlocks
	if len(sdtBlocks) != 3 {
		t.Fatalf("expected 3 sdt inner blocks, got %d", len(sdtBlocks))
	}
	if sdtBlocks[0].Kind != wordparts.BodyBlockParagraph {
		t.Errorf("sdt block[0] kind = %v, want paragraph", sdtBlocks[0].Kind)
	}
	if sdtBlocks[2].Kind != wordparts.BodyBlockTable {
		t.Errorf("sdt block[2] kind = %v, want table", sdtBlocks[2].Kind)
	}
}

func TestConvertShapeAdvancedEffects(t *testing.T) {
	doc := &wordir.Document{
		Paragraphs: []wordir.Paragraph{{
			Runs: []wordir.Run{{
				Kind: "shape",
				Shape: &wordir.Shape{
					Floating: false,
					Width:    914400,
					Height:   457200,
					Fill: dml.GradientFill{
						GradientType: "linear",
						Angle:        90,
						Stops: []dml.GradientStop{
							{Position: 0, Color: "FF0000"},
							{Position: 1, Color: "0000FF"},
						},
					},
					LineStyle: &dml.LineStyle{
						Width: 2,
						Color: "00FF00",
						Dash:  "dash",
						Cap:   "round",
					},
					Effects: dml.EffectList{
						Effects: []dml.Effect{
							dml.Shadow{Color: "000000", Alpha: 0.5, Blur: 2, Distance: 3, Angle: 45},
							dml.ThreeD{
								CameraPreset:   "legacyPerspectiveTopLeft",
								LightRig:       "threePt",
								BevelTopWidth:  3,
								BevelTopHeight: 1,
								ExtrusionDepth: 2,
							},
						},
					},
				},
			}},
		}},
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rc := pkg.Document.Body.Paragraphs[0].Runs[0].Contents
	xmlStr := string(rc[0].DrawingXML)

	if !strings.Contains(xmlStr, "<a:gradFill") {
		t.Errorf("expected gradient fill, got: %s", xmlStr)
	}
	if !strings.Contains(xmlStr, `<a:gs pos="0"`) {
		t.Errorf("expected gradient stop at 0, got: %s", xmlStr)
	}
	if !strings.Contains(xmlStr, `<a:ln w="25400" cap="round"`) {
		t.Errorf("expected line with width and cap, got: %s", xmlStr)
	}
	if !strings.Contains(xmlStr, `<a:prstDash val="dash"`) {
		t.Errorf("expected dash pattern, got: %s", xmlStr)
	}
	if !strings.Contains(xmlStr, "<a:effectLst>") {
		t.Errorf("expected effect list, got: %s", xmlStr)
	}
	if !strings.Contains(xmlStr, "<a:outerShdw") {
		t.Errorf("expected outer shadow, got: %s", xmlStr)
	}
	if !strings.Contains(xmlStr, "<a:scene3d>") {
		t.Errorf("expected scene3d, got: %s", xmlStr)
	}
	if !strings.Contains(xmlStr, "<a:sp3d") {
		t.Errorf("expected sp3d, got: %s", xmlStr)
	}
	if !strings.Contains(xmlStr, "<a:bevelT") {
		t.Errorf("expected bevelT, got: %s", xmlStr)
	}
}

func TestConvertShapeBodyPr(t *testing.T) {
	doc := &wordir.Document{
		Paragraphs: []wordir.Paragraph{{
			Runs: []wordir.Run{{
				Kind: "shape",
				Shape: &wordir.Shape{
					Floating:  false,
					Width:     914400,
					Height:    457200,
					FillColor: "FF0000",
					TextBox: &wordir.TextBox{
						PaddingLeft:    10,
						PaddingTop:     5,
						PaddingRight:   10,
						PaddingBottom:  5,
						Anchor:         "center",
						AnchorCenter:   true,
						Autofit:        "norm",
						FontScale:      75000,
						LnSpcReduction: 20000,
						PrstTxWarp:     "arc",
						FlatTx:         true,
						NumCol:         2,
						SpcCol:         12,
						RtlCol:         true,
						Blocks: []wordir.Block{{
							Kind: "paragraph",
							Paragraph: &wordir.Paragraph{
								Runs: []wordir.Run{{
									Kind: "text",
									Text: "Hello",
								}},
							},
						}},
					},
				},
			}},
		}},
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rc := pkg.Document.Body.Paragraphs[0].Runs[0].Contents
	if len(rc) != 1 {
		t.Fatalf("expected 1 run content, got %d", len(rc))
	}
	xmlStr := string(rc[0].DrawingXML)
	if !strings.Contains(xmlStr, `<wps:bodyPr`) {
		t.Errorf("expected wps:bodyPr element, got: %s", xmlStr)
	}
	if !strings.Contains(xmlStr, `lIns="127000"`) {
		t.Errorf("expected lIns, got: %s", xmlStr)
	}
	if !strings.Contains(xmlStr, `anchor="ctr"`) {
		t.Errorf("expected anchor ctr, got: %s", xmlStr)
	}
	if !strings.Contains(xmlStr, `anchorCtr="1"`) {
		t.Errorf("expected anchorCtr, got: %s", xmlStr)
	}
	if !strings.Contains(xmlStr, `<a:normAutofit fontScale="75000" lnSpcReduction="20000"/>`) {
		t.Errorf("expected normAutofit, got: %s", xmlStr)
	}
	if !strings.Contains(xmlStr, `<a:prstTxWarp prst="arc"/>`) {
		t.Errorf("expected prstTxWarp, got: %s", xmlStr)
	}
	if !strings.Contains(xmlStr, `<a:flatTx/>`) {
		t.Errorf("expected flatTx, got: %s", xmlStr)
	}
	if !strings.Contains(xmlStr, `numCol="2"`) {
		t.Errorf("expected numCol, got: %s", xmlStr)
	}
	if !strings.Contains(xmlStr, `rtlCol="1"`) {
		t.Errorf("expected rtlCol, got: %s", xmlStr)
	}
}

func TestConvertChart(t *testing.T) {
	doc := &wordir.Document{
		Paragraphs: []wordir.Paragraph{{
			Runs: []wordir.Run{{
				Kind: "chart",
				Chart: &chart.Chart{
					Title:     "Sales",
					ChartType: chart.ChartBar,
					Series: []chart.Series{
						{Name: "Q1", Values: []float64{10, 20, 30}},
					},
					Categories: []string{"A", "B", "C"},
				},
			}},
		}},
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pkg.ChartPartsFor(pkg.MainPartName)) != 1 {
		t.Fatalf("expected 1 chart part, got %d", len(pkg.ChartPartsFor(pkg.MainPartName)))
	}
	rc := pkg.Document.Body.Paragraphs[0].Runs[0].Contents
	if len(rc) != 1 || rc[0].Kind != wordparts.RunContentDrawing {
		t.Fatal("expected drawing run content for chart")
	}
	xmlStr := string(rc[0].DrawingXML)
	if !strings.Contains(xmlStr, "<c:chart") {
		t.Errorf("expected c:chart element, got: %s", xmlStr)
	}
}

func TestConvertCustomTheme(t *testing.T) {
	doc := &wordir.Document{
		Theme: &wordir.Theme{
			Name: "My Theme",
			ColorScheme: map[string]string{
				"dk1":      "111111",
				"lt1":      "EEEEEE",
				"accent1":  "AABBCC",
				"hlink":    "112233",
				"folHlink": "445566",
			},
			FontScheme: wordir.ThemeFontScheme{
				MajorFont: wordir.ThemeFontSet{
					Latin:         "Major Latin",
					EastAsia:      "Major EA",
					ComplexScript: "Major CS",
				},
				MinorFont: wordir.ThemeFontSet{
					Latin:         "Minor Latin",
					EastAsia:      "Minor EA",
					ComplexScript: "Minor CS",
				},
			},
		},
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pkg.Theme == nil {
		t.Fatal("expected theme")
	}
	if pkg.Theme.Name != "My Theme" {
		t.Errorf("theme name = %q, want %q", pkg.Theme.Name, "My Theme")
	}
	if pkg.Theme.Elements == nil {
		t.Fatal("expected theme elements")
	}
	cs := pkg.Theme.Elements.ColorScheme
	if cs == nil {
		t.Fatal("expected color scheme")
	}
	if cs.Dk1 == nil || cs.Dk1.SRGB == nil || cs.Dk1.SRGB.Val != "111111" {
		t.Errorf("dk1 = %v, want SRGB 111111", cs.Dk1)
	}
	if cs.Lt1 == nil || cs.Lt1.SRGB == nil || cs.Lt1.SRGB.Val != "EEEEEE" {
		t.Errorf("lt1 = %v, want SRGB EEEEEE", cs.Lt1)
	}
	if cs.Accent1 == nil || cs.Accent1.SRGB == nil || cs.Accent1.SRGB.Val != "AABBCC" {
		t.Errorf("accent1 = %v, want SRGB AABBCC", cs.Accent1)
	}
	if cs.Hlink == nil || cs.Hlink.SRGB == nil || cs.Hlink.SRGB.Val != "112233" {
		t.Errorf("hlink = %v, want SRGB 112233", cs.Hlink)
	}
	if cs.FolHlink == nil || cs.FolHlink.SRGB == nil || cs.FolHlink.SRGB.Val != "445566" {
		t.Errorf("folHlink = %v, want SRGB 445566", cs.FolHlink)
	}
	fs := pkg.Theme.Elements.FontScheme
	if fs == nil {
		t.Fatal("expected font scheme")
	}
	if fs.MajorFont == nil || fs.MajorFont.Latin.Typeface != "Major Latin" {
		t.Errorf("major latin = %q, want %q", fs.MajorFont.Latin.Typeface, "Major Latin")
	}
	if len(fs.MajorFont.EastAsia) != 1 || fs.MajorFont.EastAsia[0].Typeface != "Major EA" {
		t.Errorf("major eastAsia = %v, want Major EA", fs.MajorFont.EastAsia)
	}
	if len(fs.MajorFont.ComplexScript) != 1 || fs.MajorFont.ComplexScript[0].Typeface != "Major CS" {
		t.Errorf("major cs = %v, want Major CS", fs.MajorFont.ComplexScript)
	}
	if fs.MinorFont == nil || fs.MinorFont.Latin.Typeface != "Minor Latin" {
		t.Errorf("minor latin = %q, want %q", fs.MinorFont.Latin.Typeface, "Minor Latin")
	}
	if len(fs.MinorFont.EastAsia) != 1 || fs.MinorFont.EastAsia[0].Typeface != "Minor EA" {
		t.Errorf("minor eastAsia = %v, want Minor EA", fs.MinorFont.EastAsia)
	}
	if len(fs.MinorFont.ComplexScript) != 1 || fs.MinorFont.ComplexScript[0].Typeface != "Minor CS" {
		t.Errorf("minor cs = %v, want Minor CS", fs.MinorFont.ComplexScript)
	}
}

func TestConvertThemeFallbackWhenNil(t *testing.T) {
	doc := &wordir.Document{}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pkg.Theme == nil {
		t.Fatal("expected theme")
	}
	cs := pkg.Theme.Elements.ColorScheme
	if cs == nil {
		t.Fatal("expected color scheme")
	}
	if cs.Dk1 == nil || cs.Dk1.SysClr == nil || cs.Dk1.SysClr.LastClr != "000000" {
		t.Errorf("dk1 fallback = %v, want SysClr 000000", cs.Dk1)
	}
	fs := pkg.Theme.Elements.FontScheme
	if fs == nil {
		t.Fatal("expected font scheme")
	}
	if fs.MinorFont == nil || fs.MinorFont.Latin.Typeface != "Calibri" {
		t.Errorf("minor latin fallback = %q, want %q", fs.MinorFont.Latin.Typeface, "Calibri")
	}
}

func TestConvertStructuralAutoTextFieldSpansBlockResult(t *testing.T) {
	p1 := wordir.Paragraph{Index: 0, Runs: []wordir.Run{{Kind: "run", Text: "Heading", FieldKind: "AUTOTEXT", FieldArg: "ClauseBlock"}}, Text: "Heading"}
	cellA := wordir.TableCell{Paragraphs: []wordir.Paragraph{{Index: 1, Runs: []wordir.Run{{Kind: "run", Text: "Cell A", FieldKind: "AUTOTEXT", FieldArg: "ClauseBlock"}}, Text: "Cell A"}}, Text: "Cell A"}
	tbl := wordir.Table{Index: 0, Rows: []wordir.TableRow{{Cells: []wordir.TableCell{cellA}}}, Text: "Cell A"}
	p2 := wordir.Paragraph{Index: 2, Runs: []wordir.Run{{Kind: "run", Text: "Tail", FieldKind: "AUTOTEXT", FieldArg: "ClauseBlock"}}, Text: "Tail"}
	doc := &wordir.Document{
		Blocks: []wordir.Block{
			{Index: 0, Kind: "paragraph", Paragraph: &p1, FieldKind: "AUTOTEXT", FieldIndex: 0},
			{Index: 1, Kind: "table", Table: &tbl, FieldKind: "AUTOTEXT", FieldIndex: 0},
			{Index: 2, Kind: "paragraph", Paragraph: &p2, FieldKind: "AUTOTEXT", FieldIndex: 0},
		},
		Fields: []wordir.StoryField{{Kind: "AUTOTEXT", Instr: `AUTOTEXT "ClauseBlock"`, Arg: "ClauseBlock"}},
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	blocks := pkg.Document.Body.Blocks
	if len(blocks) != 3 || blocks[0].Paragraph == nil || blocks[1].Table == nil || blocks[2].Paragraph == nil {
		t.Fatalf("converted block structure = %+v", blocks)
	}
	first := blocks[0].Paragraph.Runs[0]
	if len(first.Contents) != 3 || first.Contents[0].FldChar == nil || first.Contents[0].FldChar.Type != "begin" || first.Contents[1].InstrText != `AUTOTEXT "ClauseBlock"` || first.Contents[2].FldChar == nil || first.Contents[2].FldChar.Type != "separate" {
		t.Fatalf("first AUTOTEXT field marker run = %+v", first.Contents)
	}
	lastRuns := blocks[2].Paragraph.Runs
	if len(lastRuns) == 0 {
		t.Fatal("missing last paragraph runs")
	}
	last := lastRuns[len(lastRuns)-1]
	if len(last.Contents) != 1 || last.Contents[0].FldChar == nil || last.Contents[0].FldChar.Type != "end" {
		t.Fatalf("last AUTOTEXT field marker run = %+v", last.Contents)
	}
}

func TestConvertTableCellTrackedChanges(t *testing.T) {
	cell := wordir.TableCell{
		PropertyRevision: &wordir.RevisionMark{Author: "Props", Date: "2026-05-01"},
		CellIns:          &wordir.RevisionMark{Author: "Insert", Date: "2026-05-02"},
		CellDel:          &wordir.RevisionMark{Author: "Delete", Date: "2026-05-03"},
		CellMerge:        &wordir.RevisionMark{Author: "Merge", Date: "2026-05-04"},
		Paragraphs:       []wordir.Paragraph{{Index: 0, Text: "Cell", Runs: []wordir.Run{{Kind: "run", Text: "Cell"}}}},
		Text:             "Cell",
	}
	table := wordir.Table{Index: 0, Rows: []wordir.TableRow{{Cells: []wordir.TableCell{cell}}}, Text: "Cell"}
	doc := &wordir.Document{
		Tables: []wordir.Table{table},
		Blocks: []wordir.Block{{Index: 0, Kind: "table", Table: &table}},
	}
	pkg, err := Convert(doc)
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if len(pkg.Document.Body.Blocks) != 1 || pkg.Document.Body.Blocks[0].Table == nil {
		t.Fatalf("missing table block: %+v", pkg.Document.Body.Blocks)
	}
	tcPr := pkg.Document.Body.Blocks[0].Table.Rows[0].Cells[0].Properties
	if tcPr.PropertyChange == nil || tcPr.PropertyChange.Author != "Props" {
		t.Fatalf("tcPrChange = %+v", tcPr.PropertyChange)
	}
	if tcPr.CellIns == nil || tcPr.CellIns.Author != "Insert" {
		t.Fatalf("cellIns = %+v", tcPr.CellIns)
	}
	if tcPr.CellDel == nil || tcPr.CellDel.Author != "Delete" {
		t.Fatalf("cellDel = %+v", tcPr.CellDel)
	}
	if tcPr.CellMerge == nil || tcPr.CellMerge.Author != "Merge" {
		t.Fatalf("cellMerge = %+v", tcPr.CellMerge)
	}
}

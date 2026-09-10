package legacytoooxml

import (
	"testing"

	sheetir "office/pkg/sheet/ir"
	sheetparts "office/pkg/sheet/parsers/ooxml/parts"
)

func TestPhase3InteractiveMetadataMapsBackToOOXML(t *testing.T) {
	c := &converter{}
	sheet := &sheetir.Sheet{
		Hyperlinks: []*sheetir.Hyperlink{{Ref: "A1:B1", URL: "https://example.com/docs", Display: "Open docs", Tooltip: "Documentation"}},
		Notes: map[string]*sheetir.CommentNote{
			"A1": {Ref: "A1", Author: "Ada", Text: "Review this", RichText: []*sheetir.RichTextRun{{Text: "Review ", Bold: true}, {Text: "this"}}},
		},
	}
	ws := &sheetparts.Worksheet{}
	urls := c.mapHyperlinks(sheet, ws)
	if len(ws.Hyperlinks) != 1 {
		t.Fatalf("hyperlinks = %d, want 1", len(ws.Hyperlinks))
	}
	hl := ws.Hyperlinks[0]
	if hl.Ref != "A1:B1" || hl.Display != "Open docs" || hl.Tooltip != "Documentation" || urls[hl.RelID] != "https://example.com/docs" {
		t.Fatalf("hyperlink round-trip metadata = %#v urls=%#v", hl, urls)
	}
	comments := c.mapComments(sheet)
	if comments == nil || len(comments.Authors) != 1 || comments.Authors[0] != "Ada" || len(comments.CommentList) != 1 {
		t.Fatalf("comments metadata = %#v", comments)
	}
	comment := comments.CommentList[0]
	if comment.Ref != "A1" || comment.Text.PlainValue() != "Review this" || len(comment.Text.Runs) != 2 || comment.Text.Runs[0].Properties == nil || comment.Text.Runs[0].Properties.Bold == nil {
		t.Fatalf("comment rich text = %#v", comment)
	}
}

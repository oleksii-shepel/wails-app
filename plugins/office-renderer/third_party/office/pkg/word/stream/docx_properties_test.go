package stream

import (
	"testing"

	wordsax "office/pkg/word/parsers/ooxml/sax"
)

func intp(v int) *int { return &v }
func boolp(v bool) *bool { return &v }

func TestParagraphFormatFromSAX(t *testing.T) {
	got := paragraphFormatFromSAX(wordsax.ParagraphProperties{
		Alignment: "center", IndentLeft: intp(720), SpacingAfter: intp(240),
		KeepNext: true, KeepLines: true, PageBreakBefore: true, WidowControl: boolp(false),
	})
	if got.Alignment != "center" || got.IndentLeft == nil || *got.IndentLeft != 720 || got.SpacingAfter == nil || *got.SpacingAfter != 240 {
		t.Fatalf("paragraph format = %#v", got)
	}
	if !got.KeepNext || !got.KeepLines || !got.PageBreakBefore || got.WidowControl {
		t.Fatalf("paragraph pagination flags = %#v", got)
	}
}

func TestRunFormatFromSAX(t *testing.T) {
	got := runFormatFromSAX(wordsax.RunProperties{
		Bold: boolp(true), Italic: boolp(false), Underline: "double", Color: "112233",
		FontFamily: "Aptos", FontSizeHalfPoints: intp(24),
	})
	if got.Bold == nil || !*got.Bold || got.Italic == nil || *got.Italic {
		t.Fatalf("run flags = %#v", got)
	}
	if got.Underline != "double" || got.Color != "112233" || got.FontASCII != "Aptos" || got.FontHAnsi != "Aptos" {
		t.Fatalf("run presentation = %#v", got)
	}
	if got.FontSize == nil || *got.FontSize != 24 {
		t.Fatalf("font size = %#v, want 24 half-points", got.FontSize)
	}
}

func TestSectionFromSAX(t *testing.T) {
	got := sectionFromSAX(wordsax.SectionProperties{
		Type: "nextPage", PageWidth: intp(12240), PageHeight: intp(15840),
		MarginTop: intp(1440), MarginRight: intp(1080), MarginBottom: intp(1440), MarginLeft: intp(1080),
		MarginHeader: intp(720), MarginFooter: intp(720), Columns: intp(2), ColumnSpace: intp(720), TitlePage: true,
	})
	if got.Type != "nextPage" || got.PageSize == nil || got.PageSize.Width != 12240 || got.PageSize.Height != 15840 {
		t.Fatalf("section page size = %#v", got)
	}
	if got.PageMargin == nil || got.PageMargin.Top != 1440 || got.PageMargin.Header != 720 {
		t.Fatalf("section margins = %#v", got.PageMargin)
	}
	if got.Columns == nil || got.Columns.Num != 2 || got.Columns.Space != 720 || !got.TitlePage {
		t.Fatalf("section columns/title = %#v", got)
	}
}

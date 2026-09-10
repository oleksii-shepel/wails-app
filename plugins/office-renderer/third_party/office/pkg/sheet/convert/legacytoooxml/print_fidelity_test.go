package legacytoooxml

import (
	"testing"

	sheetir "office/pkg/sheet/ir"
	sheetparts "office/pkg/sheet/parsers/ooxml/parts"
)

func TestPhase4PageSetupPreservesExtendedPrintSemantics(t *testing.T) {
	sheet := &sheetir.Sheet{
		PageSetup: &sheetir.PageSetup{
			PaperSize: 9, Orientation: "landscape", Scale: 85,
			FitToPage: true, FitToWidth: 1, FitToHeight: 2,
			FirstPageNumber: 7, UseFirstPageNumber: true,
			PageOrder: "overThenDown", BlackAndWhite: true, Draft: true,
			HorizontalDPI: 600, VerticalDPI: 600, Copies: 3,
			MarginLeft: .25, MarginRight: .25, MarginTop: .5, MarginBottom: .5,
			MarginHeader: .2, MarginFooter: .2,
			CenterHorizontally: true, PrintGridlines: true,
		},
		HeaderFooter: &sheetir.HeaderFooter{
			OddHeader: "&CReport", ScaleWithDoc: true, AlignWithMargins: true,
		},
	}
	ws := &sheetparts.Worksheet{}
	(&converter{}).mapPageSetup(sheet, ws)
	if ws.PageSetup == nil {
		t.Fatal("page setup was not emitted")
	}
	ps := ws.PageSetup
	if ps.FirstPageNumber != 7 || !ps.UseFirstPageNumber || ps.PageOrder != "overThenDown" ||
		!ps.BlackAndWhite || !ps.Draft || ps.HorizontalDPI != 600 || ps.VerticalDPI != 600 || ps.Copies != 3 {
		t.Fatalf("extended page setup lost: %+v", ps)
	}
	if ws.HeaderFooter == nil || !ws.HeaderFooter.ScaleWithDoc || !ws.HeaderFooter.AlignWithMargins {
		t.Fatalf("header/footer print semantics lost: %+v", ws.HeaderFooter)
	}
}

package ir

import "testing"

func TestPrintPagesHonorsAreaTitlesAndManualBreaks(t *testing.T) {
	s := &Sheet{PrintArea: "$A$1:$D$20", PrintTitleRows: "$1:$2", RowPageBreaks: []int{11}, DefaultColWidthChars: 8.43, DefaultRowHeightPt: 15, PageSetup: &PageSetup{PaperSize: 9, Orientation: "portrait", Scale: 100, MarginLeft: .7, MarginRight: .7, MarginTop: .75, MarginBottom: .75}}
	p := s.PrintPages()
	if len(p) < 2 {
		t.Fatalf("pages=%d want >=2", len(p))
	}
	if p[0].FirstRow != 1 || p[0].LastRow != 10 || p[1].FirstRow != 11 {
		t.Fatalf("manual break not honored: %#v", p[:2])
	}
	if p[0].RepeatRowsFrom != 1 || p[0].RepeatRowsTo != 2 {
		t.Fatalf("titles=%d..%d", p[0].RepeatRowsFrom, p[0].RepeatRowsTo)
	}
}
func TestPrintPagesHonorsFitToWidth(t *testing.T) {
	s := &Sheet{PrintArea: "$A$1:$Z$10", DefaultColWidthChars: 12, DefaultRowHeightPt: 15, PageSetup: &PageSetup{PaperSize: 9, FitToPage: true, FitToWidth: 1, FitToHeight: 0, MarginLeft: .5, MarginRight: .5, MarginTop: .5, MarginBottom: .5}}
	p := s.PrintPages()
	if len(p) == 0 {
		t.Fatal("no pages")
	}
	first := p[0]
	for _, x := range p {
		if x.FirstCol != first.FirstCol || x.LastCol != first.LastCol {
			t.Fatalf("fit-to-width produced multiple column slices: %#v", p)
		}
	}
	if first.Scale >= 1 {
		t.Fatalf("scale=%v want shrink", first.Scale)
	}
}
func TestPrintPagesSkipsHiddenGeometry(t *testing.T) {
	s := &Sheet{PrintArea: "$A$1:$C$2", ColumnWidths: map[int]float64{1: 100, 2: 100, 3: 100}, HiddenCols: map[int]bool{2: true}, DefaultRowHeightPt: 15, PageSetup: &PageSetup{PaperSize: 9, Scale: 100, MarginLeft: 2.5, MarginRight: 2.5, MarginTop: .5, MarginBottom: .5}}
	p := s.PrintPages()
	if len(p) != 2 {
		t.Fatalf("pages=%d want 2 column pages with hidden B collapsed: %#v", len(p), p)
	}
	if p[0].FirstCol != 1 || p[0].LastCol != 2 || p[1].FirstCol != 3 {
		t.Fatalf("unexpected slices %#v", p)
	}
}

func TestPrintPagesOwnsPhysicalPaperAndMarginGeometry(t *testing.T) {
	s := &Sheet{PrintArea: "$A$1:$B$2", DefaultColWidthChars: 8.43, DefaultRowHeightPt: 15,
		PageSetup: &PageSetup{PaperSize: 9, Orientation: "landscape", MarginLeft: .4, MarginRight: .5, MarginTop: .6, MarginBottom: .7, MarginHeader: .2, MarginFooter: .25}}
	p := s.PrintPages()
	if len(p) != 1 {
		t.Fatalf("pages=%d want 1", len(p))
	}
	got := p[0]
	if got.PageWidthPt < got.PageHeightPt {
		t.Fatalf("landscape geometry = %.2fx%.2f", got.PageWidthPt, got.PageHeightPt)
	}
	if got.MarginLeftPt != .4*72 || got.MarginRightPt != .5*72 || got.HeaderMarginPt != .2*72 || got.FooterMarginPt != .25*72 {
		t.Fatalf("physical margins not canonical: %#v", got)
	}
}

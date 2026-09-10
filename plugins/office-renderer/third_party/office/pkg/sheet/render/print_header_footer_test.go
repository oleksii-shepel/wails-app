package render

import (
	"strings"
	"testing"

	"office/pkg/sheet/parsers/ooxml/resolve"
)

func TestPrintHeaderFooterSelectsFirstEvenOddAndExpandsFields(t *testing.T) {
	s := &resolve.Sheet{Name: "Report", HeaderFooter: &resolve.HeaderFooter{
		OddHeader: "&LOdd&C&P/&N&R&A", EvenHeader: "&LEven", FirstHeader: "&LFirst",
		DifferentOddEven: true, DifferentFirst: true,
	}}
	h, _ := printHeaderFooterForPage(s, 0, 5, 3)
	if h.Left != "First" {
		t.Fatalf("first header = %#v", h)
	}
	h, _ = printHeaderFooterForPage(s, 1, 6, 3)
	if h.Left != "Even" {
		t.Fatalf("even header = %#v", h)
	}
	h, _ = printHeaderFooterForPage(s, 2, 7, 3)
	if h.Left != "Odd" || h.Center != "7/3" || h.Right != "Report" {
		t.Fatalf("odd header = %#v", h)
	}
}

func TestPrintHeaderFooterEscapesText(t *testing.T) {
	var b strings.Builder
	renderPrintHeaderFooter(&b, "header", printHFSections{Left: "A < B & C"}, nil, nil)
	got := b.String()
	if !strings.Contains(got, "A &lt; B &amp; C") {
		t.Fatalf("header was not escaped: %s", got)
	}
}

func TestExpandPrintHeaderFooterPreservesUnknownFields(t *testing.T) {
	got := expandPrintHFFields("&& &P &N &A &D", "S", 2, 9)
	if got != "& 2 9 S &D" {
		t.Fatalf("expanded = %q", got)
	}
}

func TestPrintHeaderFooterRendersCanonicalImagePlaceholder(t *testing.T) {
	var b strings.Builder
	hf := &resolve.HeaderFooter{Images: []resolve.HFImage{{ContentType: "image/png", Data: []byte{1, 2, 3}}}}
	idx := 0
	renderPrintHeaderFooter(&b, "header", printHFSections{Center: "Logo &G"}, hf, &idx)
	got := b.String()
	if !strings.Contains(got, `class="hf-image"`) || !strings.Contains(got, `data:image/png;base64,AQID`) {
		t.Fatalf("header/footer image missing: %s", got)
	}
	if idx != 1 {
		t.Fatalf("consumed images=%d want 1", idx)
	}
}

package layout

import (
	"testing"

	resolve "office/pkg/word/ir"
)

func letterSection() []resolve.Section {
	return []resolve.Section{{
		PageSize:   &resolve.PageSize{Width: 12240, Height: 15840},
		PageMargin: &resolve.PageMargin{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440},
	}}
}

func TestFragmentFitLetsTrailingLeadingHangPastRegion(t *testing.T) {
	m := paragraphFlowMetrics{
		lineHeights:     []float64{14, 14, 14, 14},
		trailingLeading: 1,
	}
	// Three lines measure 42pt but their text ends at 41pt.
	gotLines, gotHeight := m.fragmentFit(0, 41.5, false)
	if gotLines != 3 || !almostEqual(gotHeight, 42) {
		t.Fatalf("fragment = (%d, %.2fpt), want (3, 42pt)", gotLines, gotHeight)
	}
}

func TestEmptyParagraphLineUsesParagraphMarkFontFamily(t *testing.T) {
	size := 22
	p := &resolve.Paragraph{
		Format:           resolve.ParagraphFormat{LineRule: "auto", LineSpacing: intPtr(240)},
		DefaultRunFormat: &resolve.RunFormat{FontSize: &size, FontASCII: "Calibri", FontHAnsi: "Calibri"},
	}
	doc := &resolve.Document{DefaultRunFormat: resolve.RunFormat{FontSize: &size}}
	got := resolveTextLayoutMetrics(doc, p, PageGeometry{}, 0, 0).EmptyHeightPt
	withoutMark := paragraphLineHeightPt(&resolve.Paragraph{Format: p.Format}, PageGeometry{}, 11)
	if almostEqual(got, withoutMark) {
		t.Fatalf("empty paragraph height = %.3fpt, want the paragraph mark's Calibri metrics instead of the generic fallback", got)
	}
	if want := paragraphLineHeightPt(&resolve.Paragraph{Format: p.Format, Runs: []resolve.Run{{Kind: "run", Text: "x", Format: *p.DefaultRunFormat}}}, PageGeometry{}, 11); !almostEqual(got, want) {
		t.Fatalf("empty paragraph height = %.3fpt, want the same line as Calibri text %.3fpt", got, want)
	}
}

func TestInlinePictureLineAddsMultipleLeadingAndEffectExtent(t *testing.T) {
	size := 22
	p := &resolve.Paragraph{
		Format:           resolve.ParagraphFormat{LineRule: "auto", LineSpacing: intPtr(480)},
		DefaultRunFormat: &resolve.RunFormat{FontSize: &size, FontASCII: "Calibri"},
		Runs: []resolve.Run{{Kind: "image", Image: &resolve.Image{
			Width: 100 * emuPerPoint, Height: 200 * emuPerPoint, EffectExtentB: 2 * emuPerPoint,
		}}},
	}
	single := 11 * paragraphLineHeightRatio(p)
	got := paragraphLineHeightPt(p, PageGeometry{}, 11)
	// Double spacing adds one single line of leading; the picture is not doubled.
	if want := 202 + single; !almostEqual(got, want) {
		t.Fatalf("picture line = %.3fpt, want %.3fpt", got, want)
	}
}

func TestPaginateBreaksOversizedLinesInEmptyRegionDespiteWidowControl(t *testing.T) {
	picture := func() resolve.Run {
		return resolve.Run{Kind: "image", Image: &resolve.Image{Width: 300 * emuPerPoint, Height: 400 * emuPerPoint}}
	}
	doc := &resolve.Document{
		Sections: letterSection(),
		Blocks: []resolve.Block{{Kind: "paragraph", Paragraph: &resolve.Paragraph{
			Format: resolve.ParagraphFormat{WidowControl: true},
			Runs: []resolve.Run{
				picture(), {Kind: "break"},
				picture(), {Kind: "break"},
				picture(),
			},
		}}},
	}
	pages := Paginate(doc)
	if len(pages) != 3 {
		t.Fatalf("pages = %d, want one 400pt picture line per 648pt page", len(pages))
	}
	for i, page := range pages {
		if len(page.Blocks) != 1 || page.Blocks[0].Fragment == nil {
			t.Fatalf("page %d blocks = %#v, want one paragraph fragment", i+1, page.Blocks)
		}
		if got := page.Blocks[0].Fragment.EndLine - page.Blocks[0].Fragment.StartLine; got != 1 {
			t.Fatalf("page %d fragment lines = %d, want 1", i+1, got)
		}
	}
}

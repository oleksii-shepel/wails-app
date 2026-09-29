package layout

import (
	"testing"

	resolve "office/pkg/word/ir"
)

func TestResolveTextLayoutMetricsOwnsEmptyParagraphHeight(t *testing.T) {
	fontSize := 24 // half-points = 12pt
	p := &resolve.Paragraph{
		Runs: []resolve.Run{{Kind: "run", Format: resolve.RunFormat{FontSize: &fontSize}}},
	}
	metrics := resolveTextLayoutMetrics(&resolve.Document{}, p, PageGeometry{}, 3, 5)
	if metrics.FontSizePt != 12 {
		t.Fatalf("FontSizePt = %.2f, want 12", metrics.FontSizePt)
	}
	if metrics.LineHeightPt <= 0 || metrics.EmptyHeightPt != metrics.LineHeightPt {
		t.Fatalf("empty height = %.2f, line height = %.2f; want same positive metric", metrics.EmptyHeightPt, metrics.LineHeightPt)
	}
	if got := paragraphContentHeightFromMetrics(metrics, nil); got != metrics.EmptyHeightPt {
		t.Fatalf("empty paragraph content height = %.2f, want %.2f", got, metrics.EmptyHeightPt)
	}
}

func TestEmptyParagraphUsesParagraphMarkDefaultRunFormat(t *testing.T) {
	docFontSize := 24  // 12pt
	markFontSize := 40 // 20pt
	doc := &resolve.Document{DefaultRunFormat: resolve.RunFormat{FontSize: &docFontSize}}
	p := &resolve.Paragraph{
		DefaultRunFormat: &resolve.RunFormat{FontSize: &markFontSize},
	}

	metrics := resolveTextLayoutMetrics(doc, p, PageGeometry{}, 0, 0)
	want := 20.0 * wordSingleLineRatio
	if !almostEqual(metrics.EmptyHeightPt, want) {
		t.Fatalf("empty paragraph height = %.3f, want paragraph-mark height %.3f", metrics.EmptyHeightPt, want)
	}
}

func TestEmptyParagraphFallsBackToDocumentDefaultRunFormat(t *testing.T) {
	docFontSize := 36 // 18pt
	doc := &resolve.Document{DefaultRunFormat: resolve.RunFormat{FontSize: &docFontSize}}
	p := &resolve.Paragraph{}

	metrics := resolveTextLayoutMetrics(doc, p, PageGeometry{}, 0, 0)
	want := 18.0 * wordSingleLineRatio
	if !almostEqual(metrics.EmptyHeightPt, want) {
		t.Fatalf("empty paragraph height = %.3f, want document-default height %.3f", metrics.EmptyHeightPt, want)
	}
}

func TestDrawingHostUsesParagraphMarkHeightNotDrawingRunFormatting(t *testing.T) {
	docFontSize := 24     // 12pt
	markFontSize := 32    // 16pt
	drawingFontSize := 72 // 36pt; must not control the empty anchor line
	doc := &resolve.Document{DefaultRunFormat: resolve.RunFormat{FontSize: &docFontSize}}
	p := &resolve.Paragraph{
		DefaultRunFormat: &resolve.RunFormat{FontSize: &markFontSize},
		Runs: []resolve.Run{{
			Kind:   "image",
			Format: resolve.RunFormat{FontSize: &drawingFontSize},
			Image:  &resolve.Image{Floating: true},
		}},
	}

	if got := resolveParagraphFlowRole(p); got != paragraphFlowRoleDrawingHost {
		t.Fatalf("flow role = %v, want drawing host", got)
	}
	metrics := resolveTextLayoutMetrics(doc, p, PageGeometry{}, 0, 0)
	want := 16.0 * wordSingleLineRatio
	if !almostEqual(metrics.EmptyHeightPt, want) {
		t.Fatalf("drawing-host empty height = %.3f, want paragraph-mark height %.3f", metrics.EmptyHeightPt, want)
	}
}

func TestMixedDrawingParagraphKeepsVisibleTextLineMetrics(t *testing.T) {
	markFontSize := 40 // 20pt
	textFontSize := 24 // 12pt
	p := &resolve.Paragraph{
		DefaultRunFormat: &resolve.RunFormat{FontSize: &markFontSize},
		Runs: []resolve.Run{
			{Kind: "run", Text: "visible", Format: resolve.RunFormat{FontSize: &textFontSize}},
			{Kind: "image", Image: &resolve.Image{Floating: true}},
		},
	}

	if got := resolveParagraphFlowRole(p); got != paragraphFlowRoleMixed {
		t.Fatalf("flow role = %v, want mixed", got)
	}
	metrics := resolveTextLayoutMetrics(&resolve.Document{}, p, PageGeometry{}, 0, 0)
	want := 12.0 * wordSingleLineRatio
	if !almostEqual(metrics.LineHeightPt, want) || !almostEqual(metrics.EmptyHeightPt, want) {
		t.Fatalf("mixed metrics = line %.3f empty %.3f, want visible text %.3f", metrics.LineHeightPt, metrics.EmptyHeightPt, want)
	}
}

func TestParagraphMarkHeightPreservesAuthoredExactSpacing(t *testing.T) {
	markFontSize := 40
	exact := 240 // 12pt
	p := &resolve.Paragraph{
		DefaultRunFormat: &resolve.RunFormat{FontSize: &markFontSize},
		Format: resolve.ParagraphFormat{
			LineRule:    "exact",
			LineSpacing: &exact,
		},
	}

	metrics := resolveTextLayoutMetrics(&resolve.Document{}, p, PageGeometry{}, 0, 0)
	if !almostEqual(metrics.EmptyHeightPt, 12) {
		t.Fatalf("exact empty paragraph height = %.3f, want 12", metrics.EmptyHeightPt)
	}
}

func TestParagraphHeightFromMetricsOwnsVerticalComposition(t *testing.T) {
	metrics := TextLayoutMetrics{EmptyHeightPt: 14, SpacingBefore: 3, SpacingAfter: 5}
	if got, want := paragraphHeightFromMetrics(metrics, []float64{12, 13}, 2, 7, 4), 46.0; got != want {
		t.Fatalf("paragraph height = %.2f, want %.2f", got, want)
	}
	if got, want := paragraphHeightFromMetrics(metrics, nil, 2, 7, 4), 35.0; got != want {
		t.Fatalf("empty paragraph height = %.2f, want %.2f", got, want)
	}
}

func TestResolveParagraphSpacingMetricsOwnsContextualSpacing(t *testing.T) {
	before := 120
	after := 160
	p1 := &resolve.Paragraph{StyleID: "Body", Format: resolve.ParagraphFormat{SpacingAfter: &after, ContextualSpacing: true}}
	p2 := &resolve.Paragraph{StyleID: "Body", Format: resolve.ParagraphFormat{SpacingBefore: &before, ContextualSpacing: true}}

	first := resolveParagraphSpacingMetrics(&resolve.Document{}, p1, nil, p2, PageGeometry{}, true)
	second := resolveParagraphSpacingMetrics(&resolve.Document{}, p2, p1, nil, PageGeometry{}, false)
	if first.SpacingBefore != 0 || first.SpacingAfter != 0 {
		t.Fatalf("first contextual spacing = before %.2f after %.2f, want 0/0", first.SpacingBefore, first.SpacingAfter)
	}
	if second.SpacingBefore != 0 {
		t.Fatalf("second contextual spacing-before = %.2f, want 0", second.SpacingBefore)
	}
}

func TestParagraphIntrinsicHeightUsesSameResolvedMetrics(t *testing.T) {
	metrics := TextLayoutMetrics{EmptyHeightPt: 15, SpacingBefore: 2, SpacingAfter: 3}
	got := paragraphIntrinsicHeightFromMetrics(metrics, []float64{11, 12}, 1, 2)
	want := 31.0
	if got != want {
		t.Fatalf("intrinsic paragraph height = %.2f, want %.2f", got, want)
	}
}

func TestFullParagraphPhysicalFragmentOwnsOrdinaryLines(t *testing.T) {
	fontSize := 24
	p := &resolve.Paragraph{
		Text: "ordinary paragraph geometry belongs to layout",
		Runs: []resolve.Run{{
			Kind: "run",
			Text: "ordinary paragraph geometry belongs to layout",
			Format: resolve.RunFormat{
				FontSize: &fontSize,
			},
		}},
	}

	geom := PageGeometry{
		Width:         612,
		Height:        792,
		MarginTop:     72,
		MarginBottom:  72,
		MarginLeft:    72,
		MarginRight:   72,
		ColumnCount:   1,
		ColumnWidthPt: 468,
	}

	fragment := fullParagraphPhysicalFragment(
		&resolve.Document{},
		p,
		geom,
		0,
		0,
		0,
		0,
		nil,
	)
	if fragment == nil {
		t.Fatal("ordinary paragraph physical fragment = nil, want canonical physical lines")
	}
	if len(fragment.Lines) == 0 {
		t.Fatal("ordinary paragraph must retain layout-owned physical lines")
	}
	if fragment.SpatialRendering {
		t.Fatal("ordinary paragraph must not require absolute spatial rendering")
	}
}

func TestFullParagraphPhysicalFragmentOwnsConstrainedLines(t *testing.T) {
	geom := PageGeometry{
		Width:         300,
		Height:        400,
		MarginTop:     50,
		MarginRight:   50,
		MarginBottom:  50,
		MarginLeft:    50,
		ColumnCount:   1,
		ColumnWidthPt: 200,
	}
	p := &resolve.Paragraph{Runs: []resolve.Run{{
		Kind: "run",
		Text: "alpha beta gamma delta epsilon zeta eta theta iota kappa lambda mu nu xi omicron pi rho sigma tau",
	}}}
	active := []FloatBox{{
		Image:  &resolve.Image{Floating: true, WrapType: "square"},
		X:      50,
		Y:      50,
		Width:  105,
		Height: 70,
	}}
	fragment := fullParagraphPhysicalFragment(&resolve.Document{}, p, geom, 0, 0, 0, 0, active)
	if fragment == nil {
		t.Fatal("constrained paragraph must carry canonical physical lines")
	}
	if !fragment.SpatialRendering {
		t.Fatal("constrained paragraph must use spatial rendering")
	}
}

func TestPhysicalLinesMarkExplicitBreakTopologyAsSpatial(t *testing.T) {
	p := &resolve.Paragraph{Runs: []resolve.Run{
		{Kind: "run", Text: "before"},
		{Kind: "break", BreakType: "line"},
		{Kind: "run", Text: "after"},
	}}
	if !paragraphPhysicalLinesNeedSpatialRendering(p, PageGeometry{}, 0, nil) {
		t.Fatal("explicit line break must retain physical topology")
	}
}

func TestParagraphSpacingCollapsesAdjacentBeforeAndAfter(t *testing.T) {
	prevAfter := 240     // 12pt
	currentBefore := 360 // 18pt
	prev := &resolve.Paragraph{Format: resolve.ParagraphFormat{SpacingAfter: &prevAfter}}
	current := &resolve.Paragraph{Format: resolve.ParagraphFormat{SpacingBefore: &currentBefore}}

	// The preceding block already owns 12pt. The current block contributes only
	// the remaining 6pt so the inter-paragraph gap is max(12pt, 18pt), not 30pt.
	if got, want := ParagraphSpacingBefore(&resolve.Document{}, current, prev, PageGeometry{}, false), 6.0; got != want {
		t.Fatalf("collapsed spacing-before = %.2f, want %.2f", got, want)
	}

	currentBefore = 120 // 6pt, fully covered by the preceding 12pt after-space.
	if got := ParagraphSpacingBefore(&resolve.Document{}, current, prev, PageGeometry{}, false); got != 0 {
		t.Fatalf("covered spacing-before = %.2f, want 0", got)
	}
}

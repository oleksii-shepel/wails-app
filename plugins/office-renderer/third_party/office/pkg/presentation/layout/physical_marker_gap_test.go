package layout

import (
	ir "office/pkg/presentation/ir"
	"testing"
)

func TestParagraphVisualPlanOwnsMarkerGap(t *testing.T) {
	p := &ir.Paragraph{
		BulletChar: "•", BulletFontFamily: "Arial", BulletSize: 1200,
		Runs: []*ir.Run{{Text: "Item", FontSize: 1200, FontFamily: "Arial"}},
	}
	plan := ResolveParagraphVisualPlan(p, 100, 1, 1)
	if plan.Marker.GapPt <= 0 {
		t.Fatalf("physical marker gap = %v, want measured positive gap", plan.Marker.GapPt)
	}
}

func TestPhysicalMarkerConsumesAuthoredHangingIndentBeforeText(t *testing.T) {
	p := &ir.Paragraph{
		HasMarginLeft: true, MarginLeftEMU: 457200, // 36pt
		HasTextIndent: true, TextIndentEMU: -381000, // -30pt hanging marker region
		BulletChar: "●", BulletFontFamily: "Arial", BulletSize: 2400,
		Runs: []*ir.Run{{Text: "Reporting tab", FontSize: 2400, FontFamily: "Arial"}},
	}
	plan := ResolveParagraphVisualPlan(p, 300, 1, 1.2)
	markerWidth := measurePlainTextWidthPt("●", 24, "Arial", false, false)
	wantGap := 30.0 - markerWidth
	if wantGap < 0 {
		wantGap = 0
	}
	if diff := plan.Marker.GapPt - wantGap; diff < -0.01 || diff > 0.01 {
		t.Fatalf("marker gap = %.3fpt, want hanging-indent remainder %.3fpt", plan.Marker.GapPt, wantGap)
	}
	// FirstLineAvailableWidthPt includes the negative indent. After the marker
	// plus its physical gap consume exactly 30pt, text gets the same width as
	// continuation lines instead of an accidental extra hanging-indent width.
	if len(plan.Lines) == 0 {
		t.Fatal("expected measured physical line")
	}
}

func TestPictureBulletFitsInsideAuthoredHangingRegion(t *testing.T) {
	p := &ir.Paragraph{
		HasMarginLeft: true, MarginLeftEMU: 343080, // 27pt
		HasTextIndent: true, TextIndentEMU: -343080,
		BulletImageData: []byte("PNG-BULLET"), BulletImageMIME: "image/png", BulletSizePct: 113071,
		Runs: []*ir.Run{{Text: "The policy context", FontSize: 2800, FontFamily: "Palatino"}},
	}
	plan := ResolveParagraphVisualPlan(p, 636, 1, 1)
	if plan.Marker.Kind != "image" {
		t.Fatalf("marker kind = %q, want image", plan.Marker.Kind)
	}
	want := (343080.0 / 12700.0) * 0.8
	if diff := plan.Marker.FontSizePt - want; diff < -0.01 || diff > 0.01 {
		t.Fatalf("picture marker size = %.3fpt, want %.3fpt inside authored hanging region", plan.Marker.FontSizePt, want)
	}
}

func TestPictureBulletSmallerThanHangingRegionKeepsAuthoredSize(t *testing.T) {
	p := &ir.Paragraph{
		HasTextIndent: true, TextIndentEMU: -381000, // 30pt
		BulletImageData: []byte("PNG-BULLET"), BulletSize: 1200,
		Runs: []*ir.Run{{Text: "Item", FontSize: 2400, FontFamily: "Arial"}},
	}
	plan := ResolveParagraphVisualPlan(p, 300, 1, 1)
	if plan.Marker.FontSizePt != 12 {
		t.Fatalf("picture marker size = %.3fpt, want authored 12pt", plan.Marker.FontSizePt)
	}
}

package layout

import (
	"math"
	"testing"

	ir "office/pkg/presentation/ir"
)

func TestMeasuredListMarkerSharesFirstPhysicalLineBaseline(t *testing.T) {
	p := &ir.Paragraph{
		BulletChar:       "•",
		BulletFontFamily: "Times New Roman",
		BulletSize:       1800,
		Runs:             []*ir.Run{{Text: "Mixed marker baseline", FontFamily: "Arial", FontSize: 2800}},
	}
	plan := ResolveParagraphVisualPlan(p, 300, 1, 1)
	if !plan.LinesMeasured || len(plan.Lines) == 0 {
		t.Fatal("paragraph did not materialize physical lines")
	}
	m := plan.Marker
	if m.AscentPt <= 0 || m.DescentPt <= 0 {
		t.Fatalf("marker baseline extents not materialized: %+v", m)
	}
	// The marker's emitted box is centred on its face's own ascent/descent, so
	// the offset anchors that baseline onto the first line's baseline.
	want := math.Max(0, plan.Lines[0].AscentPt-m.AscentPt)
	if fm := ResolveFontAdvanceMetrics(m.FontFamily); fm.FaceAscent > 0 && fm.FaceDescent > 0 && m.FontSizePt > 0 {
		boxHeightPt := m.AscentPt + m.DescentPt
		faceBaselinePt := (boxHeightPt-(fm.FaceAscent+fm.FaceDescent)*m.FontSizePt)/2 + fm.FaceAscent*m.FontSizePt
		want = plan.Lines[0].AscentPt - faceBaselinePt
	}
	if math.Abs(m.TopOffsetPt-want) > .001 {
		t.Fatalf("marker top offset = %.4f, want %.4f from shared baseline", m.TopOffsetPt, want)
	}
	if m.TopOffsetPt <= 0 {
		t.Fatalf("smaller marker top offset = %.4f, want positive placement inside taller first line", m.TopOffsetPt)
	}
}

package layout

import (
	"math"
	"testing"

	ir "office/pkg/presentation/ir"
)

func TestMixedFontFragmentsOwnPhysicalTopOffsets(t *testing.T) {
	p := &ir.Paragraph{Runs: []*ir.Run{
		{Text: "A", FontSize: 2400, FontFamily: "Arial"},
		{Text: "g", FontSize: 1800, FontFamily: "Times New Roman"},
	}}
	metrics := ResolveParagraphMetrics(p, 300, 24, 1)
	got := LayoutParagraphLines(p, metrics, 1, "", 0)
	if len(got.Lines) != 1 || len(got.Lines[0].Fragments) != 2 {
		t.Fatalf("lines/fragments = %d/%d", len(got.Lines), len(got.Lines[0].Fragments))
	}
	line := got.Lines[0]
	for i := range line.Fragments {
		f := &line.Fragments[i]
		if f.AscentPt <= 0 || f.DescentPt <= 0 {
			t.Fatalf("fragment %d missing baseline extents: %.3f/%.3f", i, f.AscentPt, f.DescentPt)
		}
		// The emitted child box is centred on the face's own ascent/descent, so
		// the offset has to move that baseline onto the layout-owned one. Faces
		// without face metrics fall back to the clamped box-ascent offset.
		fm := ResolveFontAdvanceMetrics(fragmentFontFamily(f))
		want := math.Max(0, line.AscentPt-f.AscentPt)
		if fm.FaceAscent > 0 && fm.FaceDescent > 0 && f.Visual.FontSizePt > 0 {
			boxHeightPt := f.AscentPt + f.DescentPt
			faceBaselinePt := (boxHeightPt-(fm.FaceAscent+fm.FaceDescent)*f.Visual.FontSizePt)/2 + fm.FaceAscent*f.Visual.FontSizePt
			want = line.AscentPt - faceBaselinePt
		}
		if math.Abs(f.TopOffsetPt-want) > .001 {
			t.Fatalf("fragment %d top = %.3f, want %.3f", i, f.TopOffsetPt, want)
		}
	}
	if !(line.Fragments[1].TopOffsetPt > line.Fragments[0].TopOffsetPt) {
		t.Fatalf("smaller run should sit lower in shared baseline box: %+v", line.Fragments)
	}
}

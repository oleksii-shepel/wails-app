package legacy

import (
	"encoding/binary"
	"testing"

	officeartescher "office/pkg/officeart/legacy/escher"
	presentationir "office/pkg/presentation/ir"
)

func TestClientPlacementPreservesAuthoredAnchorEdges(t *testing.T) {
	body := make([]byte, 8)
	binary.LittleEndian.PutUint16(body[0:2], uint16(int16(2880))) // top = 60% of 4800
	binary.LittleEndian.PutUint16(body[2:4], uint16(int16(1000)))
	binary.LittleEndian.PutUint16(body[4:6], uint16(int16(2000)))
	binary.LittleEndian.PutUint16(body[6:8], uint16(int16(3168))) // bottom; height = 6%

	bounds, edges, ok := parseLegacyPptClientPlacementAnchorWithEdges(body, true)
	if !ok {
		t.Fatal("client anchor did not decode")
	}
	if edges.Top != 2880 || edges.Bottom != 3168 || edges.height() != 288 {
		t.Fatalf("authored Y edges = top:%d bottom:%d height:%d", edges.Top, edges.Bottom, edges.height())
	}
	if bounds.Y != edges.Top*defaultSlideHeightEMU/legacyPptMasterHeight {
		t.Fatalf("canonical Y = %d, want authored top conversion", bounds.Y)
	}
	if bounds.H != edges.height()*defaultSlideHeightEMU/legacyPptMasterHeight {
		t.Fatalf("canonical H = %d, want authored edge delta conversion", bounds.H)
	}
}

func TestResolvedOLEPlacementCarriesClientAnchorProvenance(t *testing.T) {
	body := make([]byte, 8)
	binary.LittleEndian.PutUint16(body[0:2], 2880)
	binary.LittleEndian.PutUint16(body[2:4], 1000)
	binary.LittleEndian.PutUint16(body[4:6], 2000)
	binary.LittleEndian.PutUint16(body[6:8], 3168)
	p := resolveLegacyPptShapePlacement(&officeartescher.ShapeNode{ClientAnchor: body}, legacyPptGroupFrame{})
	if !p.valid() {
		t.Fatal("placement invalid")
	}
	if p.Anchor.Top != 2880 || p.Anchor.Bottom != 3168 {
		t.Fatalf("placement lost raw Y1/Y2: %+v", p.Anchor)
	}
}

func TestEmptyParagraphUsesParagraphMarkCharacterMetrics(t *testing.T) {
	cf := stpaCFRun{CharCount: 1, FontSize: 1200, FontFamily: "Times New Roman", Color: "FFFFFF"}
	runs := buildRunsFromCF("", []stpaCFRun{cf}, 0, 1)
	if len(runs) != 1 {
		t.Fatalf("runs = %d, want 1", len(runs))
	}
	if runs[0].Text != "" {
		t.Fatalf("empty paragraph text = %q", runs[0].Text)
	}
	if runs[0].FontSize != 1200 || runs[0].FontFamily != "Times New Roman" || runs[0].Color != "FFFFFF" {
		t.Fatalf("paragraph-mark character metrics lost: %+v", runs[0])
	}
}

// TestLegacyParagraphSemanticsCannotRewriteOLEBounds is a guard for the
// slide-14 parity work.  PPTX is a useful oracle for paragraph/master
// semantics, but legacy OLE placement remains owned exclusively by the PPT
// OfficeArt anchor.  Text-flow fixes must never normalize or otherwise derive
// equation coordinates from the text layout (or from an OOXML counterpart).
func TestLegacyParagraphSemanticsCannotRewriteOLEBounds(t *testing.T) {
	authored := presentationir.Rect{X: 1012345, Y: 2345678, W: 3456789, H: 456789}
	shape := buildLegacyPptPictureOrExternalObjectShape(
		14,
		legacyPptPictureShape{bounds: authored, shapeID: 1401},
		&legacyPptPicture{},
		legacyPptExternalObject{Kind: presentationir.ShapeOleObject, ProgID: "Equation.3"},
	)
	if shape == nil {
		t.Fatal("OLE shape was not materialized")
	}
	if shape.Bounds != authored {
		t.Fatalf("OLE bounds were rewritten: got %+v, want authored PPT bounds %+v", shape.Bounds, authored)
	}
}

func TestEmptyParagraphUsesCharacterMetricsAfterParagraphMarkBoundary(t *testing.T) {
	// A marked paragraph mark owns a dedicated single-character CF run that
	// starts at the CR: PowerPoint's own conversion reproduces its size as
	// endParaRPr. The covering run (a continuation of the preceding paragraph's
	// text) must not win over it.
	marked := []stpaCFRun{
		{CharCount: 1, FontSize: 2800, FontFamily: "Times New Roman", Color: "FFFFFF"},
		{CharCount: 1, FontSize: 1200, FontFamily: "Times New Roman", Color: "FFFFFF"},
	}
	runs := buildRunsFromCF("", marked, 1, 2)
	if len(runs) != 1 {
		t.Fatalf("runs = %d, want 1", len(runs))
	}
	if runs[0].FontSize != 1200 {
		t.Fatalf("empty paragraph font size = %d, want the paragraph mark's own CF size 1200", runs[0].FontSize)
	}

	// Without a dedicated run at the CR the boundary after the mark describes
	// the mark, which is what kept the slide-14 spacer metrics (28pt -> 12pt).
	covering := []stpaCFRun{
		{CharCount: 2, FontSize: 2800, FontFamily: "Times New Roman", Color: "FFFFFF"},
		{CharCount: 1, FontSize: 1200, FontFamily: "Times New Roman", Color: "FFFFFF"},
	}
	runs = buildRunsFromCF("", covering, 1, 2)
	if len(runs) != 1 || runs[0].FontSize != 1200 {
		t.Fatalf("covering-run empty paragraph = %+v, want the following CF size 1200", runs)
	}
}

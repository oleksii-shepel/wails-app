package legacy

import (
	"testing"

	"office/pkg/ooxml/drawing"
	presentationir "office/pkg/presentation/ir"
)

func TestMasterArtworkOwnershipRejectsInvisibleTextboxScaffolding(t *testing.T) {
	shape := &presentationir.Shape{
		Kind:       presentationir.ShapeText,
		Bounds:     presentationir.Rect{X: 100, Y: 100, W: 999999999, H: 100},
		Paragraphs: []*presentationir.Paragraph{{}},
	}
	if legacyPptShapePaintsMasterArtwork(shape) {
		t.Fatal("empty unpainted master textbox must not materialize as artwork")
	}
}

func TestMasterArtworkOwnershipKeepsTextAndPaintedDecorations(t *testing.T) {
	text := &presentationir.Shape{
		Kind:       presentationir.ShapeText,
		Paragraphs: []*presentationir.Paragraph{{Runs: []*presentationir.Run{{Text: "Risk Assessment & Control"}}}},
	}
	if !legacyPptShapePaintsMasterArtwork(text) {
		t.Fatal("master text must remain artwork")
	}

	decoration := &presentationir.Shape{
		Kind: presentationir.ShapeText,
		Fill: &drawing.SolidFill{Color: "000099", Alpha: 1},
	}
	if !legacyPptShapePaintsMasterArtwork(decoration) {
		t.Fatal("painted empty master shape must remain artwork")
	}
}

func TestMasterArtworkOwnershipKeepsGroupWithVisibleDescendant(t *testing.T) {
	group := &presentationir.Shape{
		Kind: presentationir.ShapeGroup,
		Children: []*presentationir.Shape{
			{Kind: presentationir.ShapeText},
			{Kind: presentationir.ShapeConnector},
		},
	}
	if !legacyPptShapePaintsMasterArtwork(group) {
		t.Fatal("group with visible descendant must remain artwork")
	}
}

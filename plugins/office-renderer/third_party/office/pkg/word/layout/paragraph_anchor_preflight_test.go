package layout

import (
	"testing"

	wordir "office/pkg/word/ir"
)

func TestParagraphDrawingPreflightMovesLateParagraphRelativeTextbox(t *testing.T) {
	shape := &wordir.Shape{
		Floating: true,
		Width: 1270000,
		Height: 508000,
		Y: 108 * 12700,
		YRelative: "paragraph",
	}
	p := &wordir.Paragraph{Runs: []wordir.Run{{Kind: "shape", Shape: shape}}}
	if !paragraphDrawingRequiresNextPage(p, PageGeometry{Width: 612, Height: 792}, 700, 23.2) {
		t.Fatal("late paragraph-relative textbox must move with its anchor to the next physical page")
	}
	if paragraphDrawingRequiresNextPage(p, PageGeometry{Width: 612, Height: 792}, 100, 23.2) {
		t.Fatal("textbox that fits on the current physical page must stay with its anchor")
	}
}

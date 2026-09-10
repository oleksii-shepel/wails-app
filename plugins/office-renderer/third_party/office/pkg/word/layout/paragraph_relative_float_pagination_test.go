package layout

import (
	"testing"

	wordir "office/pkg/word/ir"
)

func TestParagraphRelativeFloatOverflowRequiresNextPhysicalPage(t *testing.T) {
	shape := &wordir.Shape{
		Floating: true,
		YRelative: "paragraph",
	}
	fl := FloatBox{Y: 770, Height: 30, Shape: shape}
	if !paragraphRelativeFloatOverflowsPage([]FloatBox{fl}, PageGeometry{Height: 792}) {
		t.Fatal("paragraph-relative float crossing page bottom must move with its anchor")
	}
}

func TestPageRelativeFloatOverflowDoesNotRepaginateAnchor(t *testing.T) {
	shape := &wordir.Shape{
		Floating: true,
		YRelative: "page",
	}
	fl := FloatBox{Y: 770, Height: 30, Shape: shape}
	if paragraphRelativeFloatOverflowsPage([]FloatBox{fl}, PageGeometry{Height: 792}) {
		t.Fatal("page-relative float must retain authored physical-page placement")
	}
}

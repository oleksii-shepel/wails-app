package legacy

import (
	officeartlegacy "office/pkg/officeart/legacy"
	wordir "office/pkg/word/ir"
	"testing"

	officeartescher "office/pkg/officeart/legacy/escher"
)

func TestLegacyOfficeArtProjectsGeometryTextToTextPath(t *testing.T) {
	props := legacyOfficeArtShapeProps{}
	legacyApplyOfficeArtProperties([]officeartescher.Property{
		{
			ID:        officeartescher.PropGTextUnicode,
			IsComplex: true,
			Complex:   []byte{'W', 0, 'o', 0, 'r', 0, 'd', 0, 'A', 0, 'r', 0, 't', 0, 0, 0},
		},
	}, &props)
	if !props.HasGeometryText || props.GeometryText.Text != "WordArt" {
		t.Fatalf("shape props = %+v", props)
	}
	shape := legacyCloneDrawingShapeByID(77, map[uint32]legacyOfficeArtShapeProps{77: props}, nil)
	if shape == nil || shape.TextPath == nil || shape.TextPath.On != "t" || shape.TextPath.String != "WordArt" {
		t.Fatalf("shape = %#v", shape)
	}
}

func TestLegacyGeometryTextPathProjectsWordArtStyling(t *testing.T) {
	got := legacyGeometryTextPath(officeartlegacy.GeometryText{
		Present: true, Text: "Styled", Font: "Arial Black",
		Size: 36 << 16, Bold: true, Italic: true, Vertical: true,
	})
	if got == nil || got.String != "Styled" || got.Font != "Arial Black" ||
		got.SizePt != 36 || !got.Bold || !got.Italic || !got.Vertical {
		t.Fatalf("text path = %+v", got)
	}
}

func TestLegacyNormalizeDrawingGroupChildrenUsesSharedGroupMapping(t *testing.T) {
	shape := &wordir.Shape{
		Kind:   "group",
		Width:  43200,
		Height: 21600,
		Children: []wordir.Shape{{
			X: 5400, Y: 5400, Width: 10800, Height: 10800,
		}},
	}
	legacyNormalizeDrawingGroupChildren(shape)

	child := shape.Children[0]
	if child.X != 10800 || child.Y != 5400 || child.Width != 21600 || child.Height != 10800 {
		t.Fatalf("mapped child = x:%d y:%d w:%d h:%d", child.X, child.Y, child.Width, child.Height)
	}
}

func TestLegacyNormalizeDrawingGroupChildrenSkipsZeroSizedGroup(t *testing.T) {
	shape := &wordir.Shape{
		Kind:     "group",
		Children: []wordir.Shape{{X: 1, Y: 2, Width: 3, Height: 4}},
	}
	legacyNormalizeDrawingGroupChildren(shape)
	child := shape.Children[0]
	if child.X != 1 || child.Y != 2 || child.Width != 3 || child.Height != 4 {
		t.Fatalf("unexpected mutation: %+v", child)
	}
}

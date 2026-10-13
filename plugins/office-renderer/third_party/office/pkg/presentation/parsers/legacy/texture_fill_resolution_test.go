package legacy

import (
	"testing"

	officeartir "office/pkg/officeart/ir"
	"office/pkg/ooxml/drawing"
	presentationir "office/pkg/presentation/ir"
)

func TestResolveLegacyPptShapePictureFillMaterializesTexture(t *testing.T) {
	shape := &presentationir.Shape{
		Fill: drawing.PictureFill{Tile: true, TileScaleX: 100000, TileScaleY: 100000},
		OfficeArt: &officeartir.Shape{Fill: officeartir.Fill{
			Kind:    officeartir.FillPicture,
			Picture: &officeartir.PictureFill{BlipRef: 4, Tile: &officeartir.Tile{ScaleX: 1, ScaleY: 1}},
		}},
	}
	store := map[uint32]*legacyPptPicture{4: {Data: []byte{0xFF, 0xD8, 0xFF}, MIME: "image/jpeg"}}

	resolveLegacyPptShapePictureFill(shape, store)

	fill, ok := shape.Fill.(drawing.PictureFill)
	if !ok {
		t.Fatalf("fill = %T, want drawing.PictureFill", shape.Fill)
	}
	if len(fill.ImageData) != 3 || fill.ContentType != "image/jpeg" {
		t.Fatalf("picture fill = %#v, want the stored JPEG", fill)
	}
	if !fill.Tile || fill.TileScaleX != 100000 {
		t.Fatalf("tile semantics lost: %#v", fill)
	}
}

func TestResolveLegacyPptShapePictureFillKeepsAuthoredBytes(t *testing.T) {
	shape := &presentationir.Shape{
		Fill: drawing.PictureFill{ImageData: []byte{1, 2, 3}, ContentType: "image/png"},
		OfficeArt: &officeartir.Shape{Fill: officeartir.Fill{
			Kind:    officeartir.FillPicture,
			Picture: &officeartir.PictureFill{BlipRef: 4},
		}},
	}
	resolveLegacyPptShapePictureFill(shape, map[uint32]*legacyPptPicture{4: {Data: []byte{9, 9}, MIME: "image/jpeg"}})

	fill := shape.Fill.(drawing.PictureFill)
	if len(fill.ImageData) != 3 || fill.ContentType != "image/png" {
		t.Fatalf("authored picture data replaced: %#v", fill)
	}
}

func TestResolveLegacyPptShapePictureFillWalksGroupChildren(t *testing.T) {
	child := &presentationir.Shape{
		Fill: drawing.PictureFill{Tile: true},
		OfficeArt: &officeartir.Shape{Fill: officeartir.Fill{
			Kind:    officeartir.FillPicture,
			Picture: &officeartir.PictureFill{BlipRef: 1},
		}},
	}
	group := &presentationir.Shape{Children: []*presentationir.Shape{child}}

	resolveLegacyPptShapePictureFills([]*presentationir.Shape{group}, map[uint32]*legacyPptPicture{1: {Data: []byte{7}, MIME: "image/png"}})

	fill := child.Fill.(drawing.PictureFill)
	if len(fill.ImageData) != 1 {
		t.Fatalf("grouped child fill = %#v, want resolved bytes", fill)
	}
}

func TestResolveLegacyPptShapePictureFillIgnoresMissingBlip(t *testing.T) {
	shape := &presentationir.Shape{
		Fill: drawing.PictureFill{Tile: true},
		OfficeArt: &officeartir.Shape{Fill: officeartir.Fill{
			Kind:    officeartir.FillPicture,
			Picture: &officeartir.PictureFill{BlipRef: 3},
		}},
	}
	resolveLegacyPptShapePictureFill(shape, map[uint32]*legacyPptPicture{1: {Data: []byte{7}, MIME: "image/png"}})

	if fill := shape.Fill.(drawing.PictureFill); len(fill.ImageData) != 0 {
		t.Fatalf("unresolved fill gained bytes: %#v", fill)
	}
}

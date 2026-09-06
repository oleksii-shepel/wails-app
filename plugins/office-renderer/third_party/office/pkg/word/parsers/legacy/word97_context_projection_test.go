package legacy

import (
	"testing"

	wordir "office/pkg/word/ir"
	word97 "office/pkg/word/parsers/doc/word97"
)

func TestApplySequentialContextCarriesDrawingProjection(t *testing.T) {
	image := &wordir.Image{}
	textbox := &wordir.TextBox{}
	ole := &wordir.Run{Kind: "olePlaceholder"}
	ctx := &word97.Context{
		Drawings: word97.DrawingProjection{
			InlineImages: []*wordir.Image{image},
			TextBoxes:    []*wordir.TextBox{textbox},
			OLEByCP:      map[int]*wordir.Run{3: ole},
		},
	}
	core := &legacyDocStreamCore{}

	applySequentialContext(core, ctx)

	if len(core.InlineImages) != 1 || core.InlineImages[0] != image {
		t.Fatalf("inline images were not projected: %#v", core.InlineImages)
	}
	if len(core.TextBoxes) != 1 || core.TextBoxes[0] != textbox {
		t.Fatalf("textboxes were not projected: %#v", core.TextBoxes)
	}
	if core.OLEByCP[3] != ole {
		t.Fatalf("OLE projection was not carried: %#v", core.OLEByCP)
	}
}

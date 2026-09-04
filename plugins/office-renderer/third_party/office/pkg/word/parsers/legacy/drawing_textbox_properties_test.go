package legacy

import (
	"testing"

	wordir "office/pkg/word/ir"
)

func TestLegacyClientTextboxCreatesPlaceholderWithoutTextFlow(t *testing.T) {
	props := map[uint32]legacyOfficeArtShapeProps{
		42: {
			HasTextBox:    true,
			HasTextLeft:   true,
			TextLeft:      12,
			HasTextAnchor: true,
			TextAnchor:    "center",
		},
	}
	shape := legacyCloneDrawingShapeByID(42, props, nil)
	if shape == nil || shape.TextBox == nil {
		t.Fatal("ClientTextbox shape did not create textbox placeholder")
	}
	if shape.TextBox.LegacyShapeID != 42 {
		t.Fatalf("LegacyShapeID=%d want 42", shape.TextBox.LegacyShapeID)
	}
	if shape.TextBox.PaddingLeft != 12 || !shape.TextBox.HasPaddingLeft {
		t.Fatalf("left inset=%v present=%v", shape.TextBox.PaddingLeft, shape.TextBox.HasPaddingLeft)
	}
	if shape.TextBox.Anchor != "center" || !shape.TextBox.HasAnchorSetting {
		t.Fatalf("anchor=%q present=%v", shape.TextBox.Anchor, shape.TextBox.HasAnchorSetting)
	}
}

func TestMergeLegacyTextBoxPropertiesPreservesAuthoredZeroInset(t *testing.T) {
	incoming := &wordir.TextBox{
		PaddingLeft: 7.2,
		NoWrap:      true,
	}
	placeholder := &wordir.TextBox{
		PaddingLeft:    0,
		HasPaddingLeft: true,
		NoWrap:         false,
		HasWrapSetting: true,
	}
	mergeLegacyTextBoxProperties(incoming, placeholder)
	if incoming.PaddingLeft != 0 || !incoming.HasPaddingLeft {
		t.Fatalf("left inset=%v present=%v", incoming.PaddingLeft, incoming.HasPaddingLeft)
	}
	if incoming.NoWrap {
		t.Fatal("authored wrap=false was lost")
	}
}

func TestAttachLegacyShapeTextBoxesUsesShapeID(t *testing.T) {
	shape := &wordir.Shape{
		LegacyShapeID: 20,
		TextBox:       &wordir.TextBox{LegacyShapeID: 20},
	}
	first := &wordir.TextBox{LegacyShapeID: 10, Blocks: []wordir.Block{{Kind: "paragraph"}}}
	second := &wordir.TextBox{LegacyShapeID: 20, Blocks: []wordir.Block{{Kind: "paragraph"}}}

	remaining := attachLegacyShapeTextBoxes(shape, []*wordir.TextBox{first, second})
	if shape.TextBox != second {
		t.Fatal("shape did not attach matching FTXBXS.lid/FSP.spid story")
	}
	if len(remaining) != 1 || remaining[0] != first {
		t.Fatalf("remaining=%#v", remaining)
	}
}

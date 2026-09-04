package legacy

import (
	"encoding/binary"
	"testing"

	wordir "office/pkg/word/ir"
)

func TestParseLegacyTextboxPLCUsesFTXBXSLid(t *testing.T) {
	plc := make([]byte, 4*2+22)
	binary.LittleEndian.PutUint32(plc[0:4], 3)
	binary.LittleEndian.PutUint32(plc[4:8], 9)
	binary.LittleEndian.PutUint32(plc[8+14:8+18], 4242)

	got := parseLegacyTextboxPLC(plc, false)
	if len(got) != 1 {
		t.Fatalf("got %#v", got)
	}
	if got[0].StartCP != 3 || got[0].EndCP != 9 || got[0].ShapeID != 4242 {
		t.Fatalf("got %#v", got[0])
	}
}

func TestAttachLegacyShapeTextBoxesMatchesByShapeID(t *testing.T) {
	shape := &wordir.Shape{
		LegacyShapeID: 2,
		TextBox:       &wordir.TextBox{Vert: "vert"},
	}
	boxes := []*wordir.TextBox{
		{LegacyShapeID: 1, Blocks: []wordir.Block{{Kind: "paragraph"}}},
		{LegacyShapeID: 2, Blocks: []wordir.Block{{Kind: "paragraph"}}},
	}
	remaining := attachLegacyShapeTextBoxes(shape, boxes)
	if shape.TextBox == nil || shape.TextBox.LegacyShapeID != 2 {
		t.Fatalf("wrong textbox attached: %#v", shape.TextBox)
	}
	if len(remaining) != 1 || remaining[0].LegacyShapeID != 1 {
		t.Fatalf("remaining = %#v", remaining)
	}
}

package word97

import (
	"encoding/binary"
	"strings"
	"testing"

	wordir "office/pkg/word/ir"
)

func TestDrawingCursorConsumesResolvedAndUnresolvedAnchors(t *testing.T) {
	shape := &wordir.Shape{Kind: "rect"}
	inner := &sliceCursor{runs: []Run{{Text: "a\x08b\x08c", StartCP: 10, Spec: true}}}
	c := NewDrawingCursor(inner, map[int]Drawing{11: {Shape: shape}}, nil, nil, nil)
	got := collectCursor(t, c)
	if len(got) != 4 || got[0].Text != "a" || got[1].Kind != "shape" || got[1].Shape != shape || got[2].Text != "b" || got[3].Text != "c" {
		t.Fatalf("drawing stream=%#v", got)
	}
}

func TestDrawingCursorInjectsInlineImageAndOLE(t *testing.T) {
	img := &wordir.Image{}
	oleByCP := map[int]*wordir.Run{}
	inner := &sliceCursor{runs: []Run{
		{Text: "\x01", StartCP: 4, Spec: true, HasPicLocation: true, PicLocation: 12},
		{Text: "\x01", StartCP: 8, Obj: true},
	}}
	c := NewDrawingCursorWithPictureLocations(inner, nil, nil, map[int]*wordir.Image{12: img}, []wordir.Run{{Kind: "olePlaceholder", Text: "Object", OLEType: "Embed"}}, oleByCP)
	got := collectCursor(t, c)
	if len(got) != 2 || got[0].Kind != "image" || got[0].Image != img || got[1].Kind != "olePlaceholder" {
		t.Fatalf("drawing stream=%#v", got)
	}
	if oleByCP[8] == nil || oleByCP[8].Text != "Object" {
		t.Fatalf("oleByCP=%#v", oleByCP)
	}
}

func TestDrawingCursorRejectsMissingInlinePicture(t *testing.T) {
	c := NewDrawingCursorWithPictureLocations(
		&sliceCursor{runs: []Run{{Text: "\x01", StartCP: 2, Spec: true, HasPicLocation: true, PicLocation: 12}}},
		nil, nil, nil, nil, nil,
	)
	_, ok, err := c.Next()
	if ok || err == nil {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	unsupported, yes := err.(UnsupportedError)
	if !yes || !strings.HasPrefix(unsupported.Feature, "picture/data anchor") {
		t.Fatalf("err=%T %v", err, err)
	}
}

func TestDrawingCursorConsumesNonRenderableInlinePicturePlaceholder(t *testing.T) {
	data := make([]byte, inlinePICFHeaderSize+50)
	binary.LittleEndian.PutUint32(data[0:4], uint32(len(data)))
	binary.LittleEndian.PutUint16(data[4:6], inlinePICFHeaderSize)
	binary.LittleEndian.PutUint16(data[6:8], 0x0064)

	c := NewDrawingCursorWithPictureData(
		&sliceCursor{runs: []Run{{Text: "before\x01after", StartCP: 2, Spec: true, HasPicLocation: true}}},
		nil, nil, data, nil, nil,
	)
	got := collectCursor(t, c)
	if len(got) != 2 || got[0].Text != "before" || got[1].Text != "after" {
		t.Fatalf("placeholder stream = %#v", got)
	}
}

func TestDrawingCursorDoesNotTreatFDataAsPicture(t *testing.T) {
	c := NewDrawingCursor(&sliceCursor{runs: []Run{{Text: "\x01", StartCP: 2, Spec: true, Data: true}}}, nil, nil, nil, nil)
	run, ok, err := c.Next()
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if run.Text != "\x01" || !run.Data {
		t.Fatalf("run=%#v", run)
	}
}

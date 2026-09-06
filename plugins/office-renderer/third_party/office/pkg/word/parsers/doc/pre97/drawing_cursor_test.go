package pre97

import (
	"reflect"
	"testing"

	wordir "office/pkg/word/ir"
)

type sliceCursor struct {
	runs []Run
	idx  int
}

func (c *sliceCursor) Next() (Run, bool, error) {
	if c.idx >= len(c.runs) {
		return Run{}, false, nil
	}
	run := c.runs[c.idx]
	c.idx++
	return run, true, nil
}

func TestDrawingCursorProjectsShapeAndPreservesSurroundingText(t *testing.T) {
	shape := &wordir.Shape{Kind: "rect"}
	cursor := NewDrawingCursor(&sliceCursor{runs: []Run{{
		Text: "before" + string(DrawingAnchorChar) + "after", StartCP: 11, Spec: true,
	}}}, DrawingProjection{Drawings: map[int]Drawing{17: {Shape: shape}}})

	first, ok, err := cursor.Next()
	if err != nil || !ok || first.Text != "before" || first.StartCP != 11 {
		t.Fatalf("first = %#v, ok=%v err=%v", first, ok, err)
	}
	drawing, ok, err := cursor.Next()
	if err != nil || !ok || drawing.Kind != "shape" || drawing.Shape != shape || drawing.StartCP != 17 {
		t.Fatalf("drawing = %#v, ok=%v err=%v", drawing, ok, err)
	}
	last, ok, err := cursor.Next()
	if err != nil || !ok || last.Text != "after" || last.StartCP != 18 {
		t.Fatalf("last = %#v, ok=%v err=%v", last, ok, err)
	}
}

func TestDrawingCursorConsumesUnresolvedSpecialAnchor(t *testing.T) {
	cursor := NewDrawingCursor(&sliceCursor{runs: []Run{{
		Text: "before" + string(DrawingAnchorChar) + "after", StartCP: 3, Spec: true,
	}}}, DrawingProjection{})
	first, ok, err := cursor.Next()
	if err != nil || !ok || first.Text != "before" || first.StartCP != 3 {
		t.Fatalf("first = %#v, ok=%v err=%v", first, ok, err)
	}
	second, ok, err := cursor.Next()
	if err != nil || !ok || second.Text != "after" || second.StartCP != 10 {
		t.Fatalf("second = %#v, ok=%v err=%v", second, ok, err)
	}
}

func TestDrawingCursorProjectsOLEPayloadByCP(t *testing.T) {
	oleByCP := map[int]*wordir.Run{}
	cursor := NewDrawingCursor(&sliceCursor{runs: []Run{{Text: "\x01", StartCP: 23, Spec: true, Obj: true}}}, DrawingProjection{
		OLEObjects: []wordir.Run{{Kind: "olePlaceholder", Text: "Sheet", OLEProgID: "Excel.Sheet", OLEData: []byte{1, 2, 3}, OLEMIME: "application/octet-stream"}},
		OLEByCP: oleByCP,
	})
	run, ok, err := cursor.Next()
	if err != nil || !ok || run.Kind != "olePlaceholder" || run.StartCP != 23 || run.Obj {
		t.Fatalf("run = %#v, ok=%v err=%v", run, ok, err)
	}
	resolved := oleByCP[23]
	if resolved == nil || resolved.OLEProgID != "Excel.Sheet" || !reflect.DeepEqual(resolved.OLEData, []byte{1, 2, 3}) {
		t.Fatalf("resolved OLE = %#v", resolved)
	}
}

func TestDrawingCursorRejectsMissingInlineImage(t *testing.T) {
	cursor := NewDrawingCursor(&sliceCursor{runs: []Run{{Text: "\x01", StartCP: 7, Spec: true, Data: true}}}, DrawingProjection{})
	_, ok, err := cursor.Next()
	if ok {
		t.Fatal("unexpected run")
	}
	unsupported, ok := err.(UnsupportedError)
	if !ok || unsupported.Feature != "picture/data anchor" {
		t.Fatalf("err = %v", err)
	}
}

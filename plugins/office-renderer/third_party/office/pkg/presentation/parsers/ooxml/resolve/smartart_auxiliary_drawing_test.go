package resolve

import (
	"testing"

	"office/pkg/ooxml/drawing"
	smartartrender "office/pkg/ooxml/smartart/render"
)

func TestBuildAuxiliarySmartArtShapePreservesDrawingPrimitive(t *testing.T) {
	info := &smartartrender.DrawingShapeInfo{
		ModelID: "aux-circle", X: 100, Y: 200, W: 300, H: 300,
		Geometry: "ellipse", DrawingOrder: 7,
		Fill: drawing.SolidFill{Color: "AABBCC"},
		Line: &drawing.LineStyle{Color: "FFFFFF", Width: 1},
	}
	got := buildAuxiliarySmartArtShapeFromDrawing(info, 0.5, 10, 20)
	if got == nil || got.Kind != ShapeSmartArt || got.Geometry != "ellipse" {
		t.Fatalf("auxiliary primitive = %#v", got)
	}
	if got.DrawingOrder != 7 || got.SmartArtNodeID != "aux-circle" {
		t.Fatalf("identity/order lost: %#v", got)
	}
	if got.Bounds.X != int64((10+50)*emuPerPt) || got.Bounds.Y != int64((20+100)*emuPerPt) {
		t.Fatalf("bounds = %+v", got.Bounds)
	}
}

func TestSmartArtDrawingTransformNormalizesNonZeroDrawingOrigin(t *testing.T) {
	drawingXML := []byte(`<dsp:drawing xmlns:dsp="http://schemas.microsoft.com/office/drawing/2008/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><dsp:spTree><dsp:sp modelId="aux"><dsp:nvSpPr><dsp:cNvPr id="1" name=""/><dsp:cNvSpPr/></dsp:nvSpPr><dsp:spPr><a:xfrm><a:off x="12700" y="25400"/><a:ext cx="12700" cy="12700"/></a:xfrm><a:prstGeom prst="ellipse"><a:avLst/></a:prstGeom></dsp:spPr></dsp:sp></dsp:spTree></dsp:drawing>`)
	frame := &smartartrender.FrameBounds{X: 127000, Y: 254000, W: 12700, H: 12700}
	scale, ox, oy := smartArtConnectorTransform(drawingXML, frame, nil)
	if scale <= 0 {
		t.Fatalf("scale = %v", scale)
	}
	// The only drawing primitive must land exactly at the frame origin even
	// though the drawing coordinate system itself starts away from zero.
	x := ox + 12700*scale
	y := oy + 25400*scale
	if x != 10 || y != 20 {
		t.Fatalf("mapped origin = (%v,%v), want (10,20)", x, y)
	}
}

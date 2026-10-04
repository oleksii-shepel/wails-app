package legacytoooxml

import (
	"bytes"
	"math"
	"testing"

	"office/pkg/ooxml/drawing"
	"office/pkg/ooxml/opc"
	sheetir "office/pkg/sheet/ir"
	sheetparts "office/pkg/sheet/parsers/ooxml/parts"
	sheetresolve "office/pkg/sheet/parsers/ooxml/resolve"
)

func TestLegacyShapeTransformAndShadowSurviveXLSXConversion(t *testing.T) {
	wb := &sheetir.Workbook{Sheets: []*sheetir.Sheet{{Name: "Sheet1", Drawings: []*sheetir.Drawing{{
		Name: "Legacy Shape", Type: sheetir.DrawingTypeShape,
		Position: sheetir.DrawingPosition{Anchor: sheetir.AnchorAbsolute, X: 10, Y: 20, Width: 1000000, Height: 500000},
		Shape: &sheetir.DrawingShape{Geometry: "rect", FillColor: "FF0000", RotationDeg: 37.5, FlipH: true, FlipV: true,
			Effects: drawing.EffectList{Effects: []drawing.Effect{drawing.Shadow{Color: "112233", Alpha: .5, Blur: 3, Distance: 2, Angle: 45}}}},
	}}}}}
	pkg, err := Convert(wb)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := sheetparts.WritePackage(pkg, &buf); err != nil {
		t.Fatal(err)
	}
	op, err := opc.OpenReaderAt(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := sheetparts.LoadPackage(op)
	if err != nil {
		t.Fatal(err)
	}
	got, err := sheetresolve.ResolvePackage(parsed)
	if err != nil {
		t.Fatal(err)
	}
	shape := got.Sheets[0].Drawings[0].Shape
	if math.Abs(shape.RotationDeg-37.5) > .001 || !shape.FlipH || !shape.FlipV {
		t.Fatalf("transform = rot:%v h:%t v:%t", shape.RotationDeg, shape.FlipH, shape.FlipV)
	}
	if len(shape.Effects.Effects) == 0 {
		t.Fatal("shadow effect was lost")
	}
	shadow, ok := shape.Effects.Effects[0].(drawing.Shadow)
	if !ok {
		t.Fatalf("effect = %T", shape.Effects.Effects[0])
	}
	if shadow.Color != "112233" || math.Abs(shadow.Alpha-.5) > .01 {
		t.Fatalf("shadow = %#v", shadow)
	}
}

func TestLegacySoftEdgeAndBevelSurviveXLSXConversion(t *testing.T) {
	wb := &sheetir.Workbook{Sheets: []*sheetir.Sheet{{Name: "Sheet1", Drawings: []*sheetir.Drawing{{
		Name: "Legacy Home button", Type: sheetir.DrawingTypeShape,
		Position: sheetir.DrawingPosition{Anchor: sheetir.AnchorAbsolute, X: 10, Y: 20, Width: 1000000, Height: 500000},
		Shape: &sheetir.DrawingShape{Geometry: "rect", FillColor: "C6D9F1", Effects: drawing.EffectList{Effects: []drawing.Effect{
			drawing.SoftEdge{Radius: 5},
			drawing.ThreeD{CameraPreset: "orthographicFront", LightRig: "threePt", LightDirection: "t", BevelTopWidth: 13, BevelTopHeight: 3, BevelTopPreset: "coolSlant"},
		}}},
	}}}}}
	pkg, err := Convert(wb)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := sheetparts.WritePackage(pkg, &buf); err != nil {
		t.Fatal(err)
	}
	op, err := opc.OpenReaderAt(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := sheetparts.LoadPackage(op)
	if err != nil {
		t.Fatal(err)
	}
	got, err := sheetresolve.ResolvePackage(parsed)
	if err != nil {
		t.Fatal(err)
	}
	effects := got.Sheets[0].Drawings[0].Shape.Effects.Effects
	if len(effects) != 2 {
		t.Fatalf("effects = %#v, want soft edge and 3-D", effects)
	}
	var softEdge drawing.SoftEdge
	var threeD drawing.ThreeD
	for _, effect := range effects {
		switch effect := effect.(type) {
		case drawing.SoftEdge:
			softEdge = effect
		case drawing.ThreeD:
			threeD = effect
		}
	}
	if softEdge.Radius != 5 {
		t.Fatalf("soft edge = %#v, want 5pt", softEdge)
	}
	if threeD.BevelTopWidth != 13 || threeD.BevelTopHeight != 3 || threeD.BevelTopPreset != "coolSlant" {
		t.Fatalf("3-D = %#v, want coolSlant 13pt by 3pt bevel", threeD)
	}
}

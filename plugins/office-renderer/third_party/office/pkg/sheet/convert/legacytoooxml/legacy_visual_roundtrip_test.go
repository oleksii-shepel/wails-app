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

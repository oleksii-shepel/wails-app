package drawingml

import (
	"testing"

	"office/pkg/officeart/ir"
	dml "office/pkg/ooxml/drawing"
)

func TestShapeMapsDrawingMLIntoSharedIR(t *testing.T) {
	line := dml.DefaultLineStyle()
	line.Color = "112233"
	line.Width = 2.5
	line.HeadEnd = &dml.Arrowhead{Type: dml.ArrowheadTriangle, Size: dml.ArrowheadSizeLarge, Length: dml.ArrowheadSizeSmall}
	line.TailEnd = &dml.Arrowhead{Type: dml.ArrowheadOval, Size: dml.ArrowheadSizeMedium, Length: dml.ArrowheadSizeLarge}

	spec := ShapeSpec{
		ID:     7,
		Name:   "Connector",
		Bounds: ir.Rect{X: 10, Y: 20, W: 300, H: 100},
		Line:   &line,
		Preset: &dml.PresetGeometry{Name: "bentConnector3", Adjustments: []float64{0.25}},
		Connector: &ConnectorSpec{
			Type:        "bentConnector3",
			Adjustments: []int{25000},
		},
	}
	got := Shape(spec)
	if got.Kind != ir.ShapeConnector {
		t.Fatalf("kind=%q", got.Kind)
	}
	if got.Geometry.Preset != "bentConnector3" || len(got.Geometry.Adjustments) != 1 {
		t.Fatalf("geometry=%#v", got.Geometry)
	}
	if got.Line.StartArrow.Type != "triangle" || got.Line.EndArrow.Type != "oval" {
		t.Fatalf("line=%#v", got.Line)
	}
	if got.Connector == nil || len(got.Connector.Adjustments) != 1 || got.Connector.Adjustments[0] != 0.25 {
		t.Fatalf("connector=%#v", got.Connector)
	}
}

func TestFillMapsAllCoreDrawingMLFillKinds(t *testing.T) {
	tests := []struct {
		name string
		in   dml.Fill
		want ir.FillKind
	}{
		{"none", dml.NoFill{}, ir.FillNone},
		{"solid", dml.SolidFill{Color: "FF0000", Alpha: 0.5}, ir.FillSolid},
		{"gradient", dml.GradientFill{
			GradientType: dml.GradientTypeLinear,
			Stops:        []dml.GradientStop{{Position: 0, Color: "000000", Alpha: 1}, {Position: 1, Color: "FFFFFF", Alpha: 1}},
		}, ir.FillGradient},
		{"pattern", dml.PatternFill{PatternType: dml.PatternTypeSmallGrid, Foreground: "111111", Background: "EEEEEE"}, ir.FillPattern},
		{"picture", dml.PictureFill{ImageData: []byte{1, 2, 3}, ContentType: "image/png"}, ir.FillPicture},
	}
	for _, tt := range tests {
		if got := Fill(tt.in); got.Kind != tt.want {
			t.Fatalf("%s: kind=%q want=%q", tt.name, got.Kind, tt.want)
		}
	}
}

func TestTextBodyMapsLayoutAndRuns(t *testing.T) {
	in := dml.TextBody{
		BodyProps: dml.TextBodyProperties{
			Wrap: "none", Anchor: "ctr", Vert: "vert270",
			InsetL: 100, InsetT: 200, InsetR: 300, InsetB: 400,
			Rot: 60000, AutofitType: "normAutofit", Columns: 2, ColumnSpacingEMU: 500,
		},
		Paragraphs: []dml.TextParagraph{{
			Props: dml.ParagraphProperties{Align: "ctr", SpaceBefore: 120, SpaceAfter: 80, LineSpacing: 1500},
			Runs: []dml.TextRun{{
				Text:  "Hello",
				Props: dml.RunProperties{FontFamily: "Calibri", FontSizePt: 12, Bold: true, Color: "123456"},
			}},
		}},
	}
	got := TextBody(in)
	if got.Wrap != ir.TextWrapNone || got.Anchor != ir.TextAnchorCenter || got.Vert != ir.TextVertical270 {
		t.Fatalf("body=%#v", got)
	}
	if got.RotationDeg != 1 || got.AutoFit != ir.AutoFitNormal || got.Columns != 2 {
		t.Fatalf("layout=%#v", got)
	}
	if len(got.Paragraphs) != 1 || len(got.Paragraphs[0].Runs) != 1 || got.Paragraphs[0].Runs[0].Text != "Hello" {
		t.Fatalf("paragraphs=%#v", got.Paragraphs)
	}
}

func TestEffectsMapCoreDrawingMLEffects(t *testing.T) {
	got := Effects(dml.EffectList{Effects: []dml.Effect{
		dml.Shadow{Color: "111111", Alpha: .5, Blur: 3, Distance: 4, Angle: 45},
		dml.Glow{Color: "FF0000", Alpha: .7, Radius: 6},
		dml.SoftEdge{Radius: 2},
	}})
	if got.Shadow == nil || got.Glow == nil || got.Blur == nil {
		t.Fatalf("effects=%#v", got)
	}
}

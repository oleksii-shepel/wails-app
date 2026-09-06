package stream

import (
	"testing"

	wordsax "office/pkg/word/parsers/ooxml/sax"
	wordparts "office/pkg/word/parsers/ooxml/parts"
)

func TestDOCXShapeFromDrawingBuildsWordAndOfficeArtIR(t *testing.T) {
	raw := wordsax.Event{
		DrawingGraphicKind: "shape",
		DrawingKind:        "anchor",
		DrawingName:        "Rounded text box",
		WidthEMU:           1800000,
		HeightEMU:          900000,
		PartName:           "word/document.xml",
		RawXML: `<w:drawing xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"
			xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing"
			xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
			xmlns:wps="http://schemas.microsoft.com/office/word/2010/wordprocessingShape">
			<wp:anchor relativeHeight="7" behindDoc="1" distT="12700" distB="25400" distL="38100" distR="50800">
				<wps:wsp>
					<wps:spPr>
						<a:xfrm rot="5400000" flipH="1"/>
						<a:prstGeom prst="roundRect"/>
						<a:solidFill><a:srgbClr val="336699"/></a:solidFill>
						<a:ln w="25400"><a:solidFill><a:srgbClr val="112233"/></a:solidFill></a:ln>
					</wps:spPr>
					<wps:txbx><w:txbxContent>
						<w:p><w:pPr><w:jc w:val="center"/></w:pPr><w:r><w:rPr><w:b/><w:color w:val="FF0000"/></w:rPr><w:t>Hello</w:t></w:r></w:p>
					</w:txbxContent></wps:txbx>
					<wps:bodyPr wrap="none" anchor="ctr" lIns="12700" tIns="25400" rIns="38100" bIns="50800"/>
				</wps:wsp>
			</wp:anchor>
		</w:drawing>`,
		DrawingModel: &wordparts.Drawing{Anchor: &wordparts.DrawingAnchor{
			RelativeHeight: 7,
			BehindDoc:      1,
			DistT:          12700,
			DistB:          25400,
			DistL:          38100,
			DistR:          50800,
			PositionH:      &wordparts.DrawingPosition{RelativeFrom: "page", PosOffset: 100000},
			PositionV:      &wordparts.DrawingPosition{RelativeFrom: "paragraph", PosOffset: 200000},
			WrapSquare:     &wordparts.DrawingWrapSided{WrapText: "bothSides"},
		}},
	}

	shape, err := docxShapeFromDrawing(raw)
	if err != nil {
		t.Fatal(err)
	}
	if shape == nil {
		t.Fatal("shape is nil")
	}
	if shape.Kind != "roundRect" || shape.Geometry == nil || shape.Geometry.Name != "roundRect" {
		t.Fatalf("geometry = kind %q geometry %#v", shape.Kind, shape.Geometry)
	}
	if !shape.Floating || shape.X != 100000 || shape.Y != 200000 || shape.XRelative != "page" || shape.YRelative != "paragraph" {
		t.Fatalf("anchor = floating=%v x=%d y=%d xRel=%q yRel=%q", shape.Floating, shape.X, shape.Y, shape.XRelative, shape.YRelative)
	}
	if shape.WrapType != "square" || shape.WrapSide != "bothSides" || !shape.BehindDoc || shape.ZIndex != 7 {
		t.Fatalf("wrap/z = %q %q behind=%v z=%d", shape.WrapType, shape.WrapSide, shape.BehindDoc, shape.ZIndex)
	}
	if shape.FillColor != "336699" || shape.StrokeColor != "112233" || shape.StrokeWidth != 25400 {
		t.Fatalf("style = fill %q stroke %q width %d", shape.FillColor, shape.StrokeColor, shape.StrokeWidth)
	}
	if shape.RotationDeg != 90 || !shape.FlipH {
		t.Fatalf("transform = rotation %v flipH=%v", shape.RotationDeg, shape.FlipH)
	}
	if shape.TextBox == nil || len(shape.TextBox.Blocks) != 1 || shape.TextBox.Blocks[0].Paragraph == nil {
		t.Fatalf("textbox = %#v", shape.TextBox)
	}
	p := shape.TextBox.Blocks[0].Paragraph
	if p.Text != "Hello" || p.Format.Alignment != "center" || len(p.Runs) != 1 {
		t.Fatalf("textbox paragraph = %#v", p)
	}
	if p.Runs[0].Format.Bold == nil || !*p.Runs[0].Format.Bold || p.Runs[0].Format.Color != "FF0000" {
		t.Fatalf("textbox run format = %#v", p.Runs[0].Format)
	}
	if shape.TextBox.NoWrap != true || shape.TextBox.Anchor != "ctr" {
		t.Fatalf("textbox body props = %#v", shape.TextBox)
	}
	if shape.OfficeArt == nil {
		t.Fatal("canonical OfficeArt shape is nil")
	}
	if shape.OfficeArt.Geometry.Preset != "roundRect" {
		t.Fatalf("OfficeArt preset = %q", shape.OfficeArt.Geometry.Preset)
	}
	if shape.OfficeArt.Transform.Rect.W != 1800000 || shape.OfficeArt.Transform.Rect.H != 900000 {
		t.Fatalf("OfficeArt bounds = %#v", shape.OfficeArt.Transform.Rect)
	}
}

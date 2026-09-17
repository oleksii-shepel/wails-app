package sax_test

import (
	"testing"

	"office/pkg/presentation/parsers/ooxml/sax"
)

func TestWalkBytesEmitsAuthoredVisualFactsWithoutResolvingThem(t *testing.T) {
	data := buildPPTX(t, map[string]string{
		"[Content_Types].xml":             `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/><Override PartName="/ppt/slides/slide1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/></Types>`,
		"_rels/.rels":                     `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/></Relationships>`,
		"ppt/presentation.xml":            `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:sldIdLst><p:sldId id="256" r:id="rIdSlide1"/></p:sldIdLst></p:presentation>`,
		"ppt/_rels/presentation.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rIdSlide1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/></Relationships>`,
		"ppt/slides/slide1.xml":           `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:cSld><p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name="root"/></p:nvGrpSpPr><p:grpSpPr/><p:sp><p:nvSpPr><p:cNvPr id="7" name="Title"/><p:nvPr><p:ph type="title" idx="2" orient="vert" sz="half"/></p:nvPr></p:nvSpPr><p:spPr><a:xfrm rot="60000" flipH="1"><a:off x="100" y="200"/><a:ext cx="300" cy="400"/></a:xfrm><a:prstGeom prst="roundRect"/><a:solidFill><a:schemeClr val="accent1"/></a:solidFill><a:ln w="12700" cap="round" cmpd="dbl"><a:solidFill><a:srgbClr val="112233"><a:alpha val="75000"/></a:srgbClr></a:solidFill><a:prstDash val="dashDot"/><a:bevel/><a:headEnd type="triangle" w="lg" len="sm"/><a:tailEnd type="oval" w="sm" len="lg"/></a:ln><a:effectLst><a:outerShdw blurRad="25400" dist="12700" dir="5400000" rotWithShape="1"><a:srgbClr val="445566"><a:alpha val="50000"/></a:srgbClr></a:outerShdw><a:glow rad="38100"><a:schemeClr val="accent1"/></a:glow></a:effectLst></p:spPr></p:sp><p:pic><p:nvPicPr><p:cNvPr id="8" name="Picture"/></p:nvPicPr><p:blipFill><a:blip r:embed="rIdImg"/><a:srcRect l="1000" t="2000" r="3000" b="4000"/></p:blipFill></p:pic></p:spTree></p:cSld></p:sld>`,
	})

	var transform *sax.Event
	var placeholder *sax.Event
	var geometry *sax.Event
	var fill *sax.Event
	var line *sax.Event
	var blips []sax.Event
	var effects []sax.Event
	err := sax.WalkBytes(data, sax.HandlerFunc(func(ev sax.Event) error {
		switch ev.Type {
		case sax.EventTransform:
			if ev.ShapeID == 7 {
				v := ev
				transform = &v
			}
		case sax.EventPlaceholder:
			if ev.ShapeID == 7 {
				v := ev
				placeholder = &v
			}
		case sax.EventGeometry:
			if ev.ShapeID == 7 {
				v := ev
				geometry = &v
			}
		case sax.EventFill:
			if ev.ShapeID == 7 {
				v := ev
				fill = &v
			}
		case sax.EventLine:
			if ev.ShapeID == 7 {
				v := ev
				line = &v
			}
		case sax.EventEffect:
			if ev.ShapeID == 7 {
				effects = append(effects, ev)
			}
		case sax.EventBlip:
			if ev.ShapeID == 8 {
				blips = append(blips, ev)
			}
		}
		return nil
	}), sax.Options{})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if transform == nil || transform.Transform.X != 100 || transform.Transform.Y != 200 || transform.Transform.CX != 300 || transform.Transform.CY != 400 || transform.Transform.Rotation != 60000 || !transform.Transform.FlipH {
		t.Fatalf("transform = %#v", transform)
	}
	if placeholder == nil || placeholder.Placeholder != "title" || placeholder.PlaceholderIdx != 2 || placeholder.PlaceholderOrient != "vert" || placeholder.PlaceholderSize != "half" {
		t.Fatalf("placeholder = %#v", placeholder)
	}
	if geometry == nil || geometry.Geometry != "roundRect" {
		t.Fatalf("geometry = %#v", geometry)
	}
	if fill == nil || fill.Color.Kind != "schemeClr" || fill.Color.Value != "accent1" {
		t.Fatalf("fill = %#v", fill)
	}
	if line == nil || line.LineWidthEMU != 12700 || line.Line.Cap != "round" || line.Line.Compound != "dbl" || line.Line.Dash != "dashDot" || line.Line.Join != "bevel" || line.Line.HeadType != "triangle" || line.Line.TailType != "oval" || !line.Line.Color.HasAlpha || line.Line.Color.Alpha != 75000 {
		t.Fatalf("line = %#v", line)
	}
	if len(effects) != 2 || effects[0].Effect.Kind != "outerShdw" || effects[0].Effect.Color.Value != "445566" || effects[0].Effect.Color.Alpha != 50000 || effects[1].Effect.Kind != "glow" || effects[1].Effect.Rad != 38100 {
		t.Fatalf("effects = %#v", effects)
	}
	if len(blips) != 2 || blips[0].RelID != "rIdImg" || blips[1].CropLeft != 1000 || blips[1].CropBottom != 4000 {
		t.Fatalf("blips = %#v", blips)
	}
}

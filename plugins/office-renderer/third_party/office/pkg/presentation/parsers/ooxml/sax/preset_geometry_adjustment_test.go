package sax_test

import (
	"math"
	"testing"

	"office/pkg/presentation/parsers/ooxml/sax"
)

func TestContextualBuilderPreservesPresetGeometryAdjustments(t *testing.T) {
	data := buildPPTX(t, map[string]string{
		"[Content_Types].xml":             `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/></Types>`,
		"_rels/.rels":                     `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="r" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/></Relationships>`,
		"ppt/presentation.xml":            `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:sldIdLst><p:sldId id="256" r:id="s"/></p:sldIdLst></p:presentation>`,
		"ppt/_rels/presentation.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="s" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/></Relationships>`,
		"ppt/slides/slide1.xml":           `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><p:cSld><p:spTree><p:sp><p:nvSpPr><p:cNvPr id="2" name="Atmosphere"/></p:nvSpPr><p:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="240" cy="5472"/></a:xfrm><a:prstGeom prst="moon"><a:avLst><a:gd name="adj" fmla="val 50000"/></a:avLst></a:prstGeom></p:spPr></p:sp></p:spTree></p:cSld></p:sld>`,
	})

	slides, err := sax.ParseSlidesBytes(data, sax.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(slides) != 1 || len(slides[0].Shapes) != 1 {
		t.Fatalf("slides = %#v", slides)
	}
	shape := slides[0].Shapes[0]
	if shape.Geometry != "moon" {
		t.Fatalf("geometry = %q, want moon", shape.Geometry)
	}
	if len(shape.GeometryAdjustments) != 1 || math.Abs(shape.GeometryAdjustments[0]-0.5) > 1e-9 {
		t.Fatalf("geometry adjustments = %#v, want [0.5]", shape.GeometryAdjustments)
	}
}

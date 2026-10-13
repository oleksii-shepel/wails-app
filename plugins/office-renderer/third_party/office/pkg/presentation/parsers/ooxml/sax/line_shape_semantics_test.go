package sax_test

import (
	"testing"

	"office/pkg/presentation/ir"
	"office/pkg/presentation/parsers/ooxml/sax"
)

func TestPlainShapeWithLineGeometryNormalizesToConnector(t *testing.T) {
	data := buildPPTX(t, map[string]string{
		"[Content_Types].xml":              `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/></Types>`,
		"_rels/.rels":                      `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/></Relationships>`,
		"ppt/presentation.xml":             `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:sldIdLst><p:sldId id="256" r:id="rIdSlide1"/></p:sldIdLst></p:presentation>`,
		"ppt/_rels/presentation.xml.rels":  `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rIdSlide1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/></Relationships>`,
		"ppt/slides/slide1.xml":            `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><p:cSld><p:spTree><p:sp><p:nvSpPr><p:cNvPr id="2" name="line"/></p:nvSpPr><p:spPr><a:xfrm flipH="1" flipV="1"><a:off x="100" y="200"/><a:ext cx="300" cy="400"/></a:xfrm><a:prstGeom prst="line"/><a:ln w="38100"><a:solidFill><a:srgbClr val="000000"/></a:solidFill><a:tailEnd type="triangle"/></a:ln></p:spPr><p:txBody><a:bodyPr/><a:p/></p:txBody></p:sp></p:spTree></p:cSld></p:sld>`,
		"ppt/slides/_rels/slide1.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"></Relationships>`,
	})
	slides, err := sax.ParseSlidesBytes(data, sax.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(slides) != 1 || len(slides[0].Shapes) != 1 {
		t.Fatalf("slides=%#v", slides)
	}
	got := slides[0].Shapes[0]
	if got.Kind != ir.ShapeConnector {
		t.Fatalf("kind=%v, want connector", got.Kind)
	}
	if got.Geometry != "line" || got.ConnectorType != "straightConnector1" {
		t.Fatalf("geometry/type=%q/%q", got.Geometry, got.ConnectorType)
	}
	if !got.ConnectorFlipH || !got.ConnectorFlipV {
		t.Fatalf("connector flips=(%v,%v), want both true", got.ConnectorFlipH, got.ConnectorFlipV)
	}
	if got.ArrowTail != "triangle" {
		t.Fatalf("tail=%q, want triangle", got.ArrowTail)
	}
}

func TestPlainShapeWithLineGeometryAndTextRemainsTextShape(t *testing.T) {
	data := buildPPTX(t, map[string]string{
		"[Content_Types].xml":              `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/></Types>`,
		"_rels/.rels":                      `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/></Relationships>`,
		"ppt/presentation.xml":             `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:sldIdLst><p:sldId id="256" r:id="rIdSlide1"/></p:sldIdLst></p:presentation>`,
		"ppt/_rels/presentation.xml.rels":  `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rIdSlide1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/></Relationships>`,
		"ppt/slides/slide1.xml":            `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><p:cSld><p:spTree><p:sp><p:nvSpPr><p:cNvPr id="2" name="line with text"/></p:nvSpPr><p:spPr><a:xfrm><a:off x="100" y="200"/><a:ext cx="300" cy="400"/></a:xfrm><a:prstGeom prst="line"/></p:spPr><p:txBody><a:bodyPr/><a:p><a:r><a:rPr lang="en-US"/><a:t>Label</a:t></a:r></a:p></p:txBody></p:sp></p:spTree></p:cSld></p:sld>`,
		"ppt/slides/_rels/slide1.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"></Relationships>`,
	})
	slides, err := sax.ParseSlidesBytes(data, sax.Options{})
	if err != nil {
		t.Fatal(err)
	}
	got := slides[0].Shapes[0]
	if got.Kind != ir.ShapeText {
		t.Fatalf("kind=%v, want text because line shape contains meaningful text", got.Kind)
	}
	if len(got.Paragraphs) == 0 || len(got.Paragraphs[0].Runs) == 0 || got.Paragraphs[0].Runs[0].Text != "Label" {
		t.Fatalf("paragraphs=%#v", got.Paragraphs)
	}
}

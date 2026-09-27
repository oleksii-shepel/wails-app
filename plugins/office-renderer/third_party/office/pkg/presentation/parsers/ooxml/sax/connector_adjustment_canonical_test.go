package sax_test

import (
	"testing"

	"office/pkg/presentation/parsers/ooxml/sax"
)

func TestConnectorPresetAdjustmentsReachCanonicalConnectorIR(t *testing.T) {
	data := buildPPTX(t, map[string]string{
		"[Content_Types].xml":              `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/></Types>`,
		"_rels/.rels":                      `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/></Relationships>`,
		"ppt/presentation.xml":             `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:sldIdLst><p:sldId id="256" r:id="rIdSlide1"/></p:sldIdLst></p:presentation>`,
		"ppt/_rels/presentation.xml.rels":  `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rIdSlide1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/></Relationships>`,
		"ppt/slides/slide1.xml":            `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><p:cSld><p:spTree><p:cxnSp><p:nvCxnSpPr><p:cNvPr id="2" name="elbow"/><p:cNvCxnSpPr/></p:nvCxnSpPr><p:spPr><a:xfrm flipV="1"><a:off x="100" y="200"/><a:ext cx="300" cy="400"/></a:xfrm><a:prstGeom prst="bentConnector3"><a:avLst><a:gd name="adj1" fmla="val 100519"/></a:avLst></a:prstGeom><a:ln/></p:spPr></p:cxnSp></p:spTree></p:cSld></p:sld>`,
		"ppt/slides/_rels/slide1.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"></Relationships>`,
	})
	slides, err := sax.BuildContextualSlidesBytes(data, sax.Options{})
	if err != nil {
		t.Fatal(err)
	}
	got := slides[0].Shapes[0]
	if got.ConnectorType != "bentConnector3" || len(got.ConnectorAdjustments) != 1 || got.ConnectorAdjustments[0] != 100519 {
		t.Fatalf("connector = type %q adjustments %v, want bentConnector3/[100519]", got.ConnectorType, got.ConnectorAdjustments)
	}
}

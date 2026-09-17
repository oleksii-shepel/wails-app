package sax_test

import (
	"testing"

	"office/pkg/presentation/parsers/ooxml/sax"
)

func TestEndParaRPrSolidFillDoesNotBecomeShapeFill(t *testing.T) {
	data := buildPPTX(t, map[string]string{
		"[Content_Types].xml":             `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/><Override PartName="/ppt/slides/slide1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/></Types>`,
		"_rels/.rels":                     `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/></Relationships>`,
		"ppt/presentation.xml":            `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:sldIdLst><p:sldId id="256" r:id="s"/></p:sldIdLst></p:presentation>`,
		"ppt/_rels/presentation.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="s" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/></Relationships>`,
		"ppt/slides/slide1.xml":           `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><p:cSld><p:spTree><p:nvGrpSpPr/><p:grpSpPr/><p:sp><p:nvSpPr><p:cNvPr id="7" name="body"/></p:nvSpPr><p:spPr><a:noFill/></p:spPr><p:txBody><a:bodyPr/><a:lstStyle/><a:p><a:r><a:t>text</a:t></a:r><a:endParaRPr sz="2400"><a:solidFill><a:srgbClr val="424242"/></a:solidFill></a:endParaRPr></a:p></p:txBody></p:sp></p:spTree></p:cSld></p:sld>`,
	})
	var fills, noFills int
	if err := sax.WalkBytes(data, sax.HandlerFunc(func(ev sax.Event) error {
		if ev.ShapeID != 7 {
			return nil
		}
		if ev.Type == sax.EventFill {
			fills++
		}
		if ev.Type == sax.EventNoFill {
			noFills++
		}
		return nil
	}), sax.Options{}); err != nil {
		t.Fatal(err)
	}
	if fills != 0 {
		t.Fatalf("endParaRPr emitted %d shape fills, want 0", fills)
	}
	if noFills != 1 {
		t.Fatalf("shape noFill events = %d, want 1", noFills)
	}
}

func TestRunShadowDoesNotBecomeShapeEffect(t *testing.T) {
	data := buildPPTX(t, map[string]string{
		"[Content_Types].xml":             `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/><Override PartName="/ppt/slides/slide1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/></Types>`,
		"_rels/.rels":                     `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/></Relationships>`,
		"ppt/presentation.xml":            `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:sldIdLst><p:sldId id="256" r:id="s"/></p:sldIdLst></p:presentation>`,
		"ppt/_rels/presentation.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="s" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/></Relationships>`,
		"ppt/slides/slide1.xml":           `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><p:cSld><p:spTree><p:nvGrpSpPr/><p:grpSpPr/><p:sp><p:nvSpPr><p:cNvPr id="7" name="body"/></p:nvSpPr><p:spPr/><p:txBody><a:bodyPr/><a:lstStyle/><a:p><a:r><a:rPr><a:effectLst><a:outerShdw blurRad="50800" dist="38100" dir="18900000"><a:srgbClr val="000000"/></a:outerShdw></a:effectLst></a:rPr><a:t>text</a:t></a:r></a:p></p:txBody></p:sp></p:spTree></p:cSld></p:sld>`,
	})
	var effects int
	if err := sax.WalkBytes(data, sax.HandlerFunc(func(ev sax.Event) error {
		if ev.ShapeID == 7 && ev.Type == sax.EventEffect {
			effects++
		}
		return nil
	}), sax.Options{}); err != nil {
		t.Fatal(err)
	}
	if effects != 0 {
		t.Fatalf("run shadow emitted %d shape effects, want 0", effects)
	}
}

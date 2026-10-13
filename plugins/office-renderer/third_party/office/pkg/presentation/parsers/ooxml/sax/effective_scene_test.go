package sax_test

import (
	"testing"

	"office/pkg/presentation/parsers/ooxml/sax"
)

func TestContextualBuilderMaterializesMasterAndLayoutDecorativeShapes(t *testing.T) {
	data := buildPPTX(t, map[string]string{
		"[Content_Types].xml":                          `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/></Types>`,
		"_rels/.rels":                                  `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="r" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/></Relationships>`,
		"ppt/presentation.xml":                         `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:sldIdLst><p:sldId id="256" r:id="s"/></p:sldIdLst></p:presentation>`,
		"ppt/_rels/presentation.xml.rels":              `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="s" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/></Relationships>`,
		"ppt/slides/slide1.xml":                        `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:cSld><p:spTree><p:sp><p:nvSpPr><p:cNvPr id="30" name="slide-local"/></p:nvSpPr></p:sp></p:spTree></p:cSld></p:sld>`,
		"ppt/slides/_rels/slide1.xml.rels":             `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="l" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/></Relationships>`,
		"ppt/slideLayouts/slideLayout1.xml":            `<p:sldLayout xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:cSld><p:spTree><p:sp><p:nvSpPr><p:cNvPr id="20" name="layout-decoration"/></p:nvSpPr></p:sp><p:sp><p:nvSpPr><p:cNvPr id="21" name="layout-title-template"/><p:nvPr><p:ph type="title"/></p:nvPr></p:nvSpPr></p:sp></p:spTree></p:cSld></p:sldLayout>`,
		"ppt/slideLayouts/_rels/slideLayout1.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="m" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="../slideMasters/slideMaster1.xml"/></Relationships>`,
		"ppt/slideMasters/slideMaster1.xml":            `<p:sldMaster xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:cSld><p:spTree><p:sp><p:nvSpPr><p:cNvPr id="10" name="master-decoration"/></p:nvSpPr></p:sp><p:sp><p:nvSpPr><p:cNvPr id="11" name="master-body-template"/><p:nvPr><p:ph type="body"/></p:nvPr></p:nvSpPr></p:sp></p:spTree></p:cSld></p:sldMaster>`,
		"ppt/slideMasters/_rels/slideMaster1.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"></Relationships>`,
	})
	slides, err := sax.ParseSlidesBytes(data, sax.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(slides) != 1 {
		t.Fatalf("slides = %d, want 1", len(slides))
	}
	got := slides[0].Shapes
	if len(got) != 3 {
		t.Fatalf("effective shapes = %d, want master + layout + slide = 3: %#v", len(got), got)
	}
	if got[0].Name != "master-decoration" || got[1].Name != "layout-decoration" || got[2].Name != "slide-local" {
		t.Fatalf("effective paint order = %q, %q, %q", got[0].Name, got[1].Name, got[2].Name)
	}
	if len(slides[0].Master.Shapes) != 2 || len(slides[0].Layout.Shapes) != 2 {
		t.Fatalf("placeholder template collections changed: master=%d layout=%d", len(slides[0].Master.Shapes), len(slides[0].Layout.Shapes))
	}
}

func TestContextualBuilderDoesNotPaintMasterEditingPromptAsArtwork(t *testing.T) {
	data := buildPPTX(t, map[string]string{
		"[Content_Types].xml":                          `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/></Types>`,
		"_rels/.rels":                                  `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="r" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/></Relationships>`,
		"ppt/presentation.xml":                         `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:sldIdLst><p:sldId id="256" r:id="s"/></p:sldIdLst></p:presentation>`,
		"ppt/_rels/presentation.xml.rels":              `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="s" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/></Relationships>`,
		"ppt/slides/slide1.xml":                        `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:cSld><p:spTree><p:sp><p:nvSpPr><p:cNvPr id="30" name="slide-local"/></p:nvSpPr></p:sp></p:spTree></p:cSld></p:sld>`,
		"ppt/slides/_rels/slide1.xml.rels":             `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="l" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/></Relationships>`,
		"ppt/slideLayouts/slideLayout1.xml":            `<p:sldLayout xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:cSld><p:spTree/></p:cSld></p:sldLayout>`,
		"ppt/slideLayouts/_rels/slideLayout1.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="m" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="../slideMasters/slideMaster1.xml"/></Relationships>`,
		"ppt/slideMasters/slideMaster1.xml":            `<p:sldMaster xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><p:cSld><p:spTree><p:sp><p:nvSpPr><p:cNvPr id="10" name="master-editing-prompt"/></p:nvSpPr><p:txBody><a:bodyPr/><a:lstStyle/><a:p><a:r><a:t>Click to edit Master text styles</a:t></a:r></a:p><a:p><a:r><a:t>Second level</a:t></a:r></a:p></p:txBody></p:sp><p:sp><p:nvSpPr><p:cNvPr id="11" name="real-master-label"/></p:nvSpPr><p:txBody><a:bodyPr/><a:lstStyle/><a:p><a:r><a:t>University</a:t></a:r></a:p></p:txBody></p:sp></p:spTree></p:cSld></p:sldMaster>`,
		"ppt/slideMasters/_rels/slideMaster1.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"></Relationships>`,
	})
	slides, err := sax.ParseSlidesBytes(data, sax.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(slides) != 1 {
		t.Fatalf("slides = %d, want 1", len(slides))
	}
	got := slides[0].Shapes
	if len(got) != 2 {
		t.Fatalf("effective shapes = %d, want real master artwork + slide shape", len(got))
	}
	if got[0].Name != "real-master-label" || got[1].Name != "slide-local" {
		t.Fatalf("effective shapes = %q, %q", got[0].Name, got[1].Name)
	}
	if len(slides[0].Master.Shapes) != 2 {
		t.Fatalf("master template graph changed: %d shapes", len(slides[0].Master.Shapes))
	}
}

func TestContextualBuilderHonorsSlideShowMasterShapesFalse(t *testing.T) {
	data := buildPPTX(t, map[string]string{
		"[Content_Types].xml":                          `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/></Types>`,
		"_rels/.rels":                                  `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="r" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/></Relationships>`,
		"ppt/presentation.xml":                         `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:sldIdLst><p:sldId id="256" r:id="s"/></p:sldIdLst></p:presentation>`,
		"ppt/_rels/presentation.xml.rels":              `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="s" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/></Relationships>`,
		"ppt/slides/slide1.xml":                        `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" showMasterSp="0"><p:cSld><p:spTree><p:sp><p:nvSpPr><p:cNvPr id="30" name="slide-local"/></p:nvSpPr></p:sp></p:spTree></p:cSld></p:sld>`,
		"ppt/slides/_rels/slide1.xml.rels":             `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="l" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/></Relationships>`,
		"ppt/slideLayouts/slideLayout1.xml":            `<p:sldLayout xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:cSld><p:spTree><p:sp><p:nvSpPr><p:cNvPr id="20" name="layout-decoration"/></p:nvSpPr></p:sp></p:spTree></p:cSld></p:sldLayout>`,
		"ppt/slideLayouts/_rels/slideLayout1.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="m" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="../slideMasters/slideMaster1.xml"/></Relationships>`,
		"ppt/slideMasters/slideMaster1.xml":            `<p:sldMaster xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:cSld><p:spTree><p:sp><p:nvSpPr><p:cNvPr id="10" name="master-decoration"/></p:nvSpPr></p:sp></p:spTree></p:cSld></p:sldMaster>`,
		"ppt/slideMasters/_rels/slideMaster1.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"></Relationships>`,
	})
	slides, err := sax.ParseSlidesBytes(data, sax.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(slides) != 1 {
		t.Fatalf("slides = %d, want 1", len(slides))
	}
	got := slides[0].Shapes
	if len(got) != 1 {
		t.Fatalf("effective shapes = %d, want slide-local paint only", len(got))
	}
	if got[0].Name != "slide-local" {
		t.Fatalf("effective shape = %q; slide showMasterSp=0 must suppress inherited layout/master artwork", got[0].Name)
	}
	if len(slides[0].Master.Shapes) != 1 {
		t.Fatalf("master template graph changed: %d shapes", len(slides[0].Master.Shapes))
	}
}

func TestContextualBuilderHonorsLayoutShowMasterShapesFalse(t *testing.T) {
	data := buildPPTX(t, map[string]string{
		"[Content_Types].xml":                          `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/></Types>`,
		"_rels/.rels":                                  `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="r" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/></Relationships>`,
		"ppt/presentation.xml":                         `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:sldIdLst><p:sldId id="256" r:id="s"/></p:sldIdLst></p:presentation>`,
		"ppt/_rels/presentation.xml.rels":              `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="s" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/></Relationships>`,
		"ppt/slides/slide1.xml":                        `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:cSld><p:spTree><p:sp><p:nvSpPr><p:cNvPr id="30" name="slide-local"/></p:nvSpPr></p:sp></p:spTree></p:cSld></p:sld>`,
		"ppt/slides/_rels/slide1.xml.rels":             `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="l" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/></Relationships>`,
		"ppt/slideLayouts/slideLayout1.xml":            `<p:sldLayout xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" showMasterSp="0"><p:cSld><p:spTree><p:sp><p:nvSpPr><p:cNvPr id="20" name="layout-decoration"/></p:nvSpPr></p:sp></p:spTree></p:cSld></p:sldLayout>`,
		"ppt/slideLayouts/_rels/slideLayout1.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="m" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="../slideMasters/slideMaster1.xml"/></Relationships>`,
		"ppt/slideMasters/slideMaster1.xml":            `<p:sldMaster xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:cSld><p:spTree><p:sp><p:nvSpPr><p:cNvPr id="10" name="master-decoration"/></p:nvSpPr></p:sp></p:spTree></p:cSld></p:sldMaster>`,
		"ppt/slideMasters/_rels/slideMaster1.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"></Relationships>`,
	})
	slides, err := sax.ParseSlidesBytes(data, sax.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(slides) != 1 {
		t.Fatalf("slides = %d, want 1", len(slides))
	}
	got := slides[0].Shapes
	if len(got) != 2 {
		t.Fatalf("effective shapes = %d, want layout + slide only", len(got))
	}
	if got[0].Name != "layout-decoration" || got[1].Name != "slide-local" {
		t.Fatalf("effective shapes = %q, %q; layout showMasterSp=0 must suppress master artwork", got[0].Name, got[1].Name)
	}
	if len(slides[0].Master.Shapes) != 1 {
		t.Fatalf("master template graph changed: %d shapes", len(slides[0].Master.Shapes))
	}
}

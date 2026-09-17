package sax_test

import (
	"office/pkg/presentation/parsers/ooxml/sax"
	"testing"
)

func TestBuildContextualPresentationOwnsCanvasTransitionAndNotes(t *testing.T) {
	data := buildPPTX(t, map[string]string{
		"[Content_Types].xml":              `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/><Override PartName="/ppt/slides/slide1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/></Types>`,
		"_rels/.rels":                      `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/></Relationships>`,
		"ppt/presentation.xml":             `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:sldSz cx="12192000" cy="6858000"/><p:sldIdLst><p:sldId id="256" r:id="rId1"/></p:sldIdLst></p:presentation>`,
		"ppt/_rels/presentation.xml.rels":  `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/></Relationships>`,
		"ppt/slides/slide1.xml":            `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:cSld><p:spTree/></p:cSld><p:transition spd="slow" advClick="0" advTm="1250"><p:fade/></p:transition></p:sld>`,
		"ppt/slides/_rels/slide1.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rIdN" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/notesSlide" Target="../notesSlides/notesSlide1.xml"/></Relationships>`,
		"ppt/notesSlides/notesSlide1.xml":  `<p:notes xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><p:cSld><p:spTree><p:sp><p:nvSpPr><p:cNvPr id="9" name="Notes"/></p:nvSpPr><p:txBody><a:p><a:r><a:t>speaker note</a:t></a:r></a:p></p:txBody></p:sp></p:spTree></p:cSld></p:notes>`,
	})
	got, err := sax.BuildContextualPresentationBytes(data, sax.Options{IncludeNotes: true, IncludeHiddenSlides: true})
	if err != nil {
		t.Fatal(err)
	}
	if got.SlideWidthEMU != 12192000 || got.SlideHeightEMU != 6858000 {
		t.Fatalf("canvas=%dx%d", got.SlideWidthEMU, got.SlideHeightEMU)
	}
	if len(got.Slides) != 1 || got.Slides[0].Transition == nil {
		t.Fatalf("transition missing: %#v", got.Slides)
	}
	tr := got.Slides[0].Transition
	if tr.Type != "fade" || tr.Spd != "slow" || tr.AdvClick || tr.AdvTmMs != 1250 {
		t.Fatalf("transition=%+v", *tr)
	}
	if got.Slides[0].Notes == nil || len(got.Slides[0].Notes.Shapes) != 1 {
		t.Fatalf("notes=%#v", got.Slides[0].Notes)
	}
}

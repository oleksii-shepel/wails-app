package edit

import (
	"bytes"
	"encoding/xml"
	"strings"
	"testing"

	"office/pkg/ooxml/opc"
	"office/pkg/ooxml/rels"
	presir "office/pkg/presentation/ir"
)

func TestEditSavePrunesRemovedManagedPartsAndPreservesOpaqueExtensions(t *testing.T) {
	p := NewPresentation()
	s1 := p.AddBlankSlide()
	pic := s1.AddImage(minimalPNG(), "image/png", 100, 100, 1000, 1000)
	if pic == nil {
		t.Fatal("add image")
	}
	link := s1.AddTextBox(200, 200, 1200, 400)
	link.SetText("link")
	link.SetHyperlinkURL("https://example.com")
	s2 := p.AddBlankSlide()
	s2.AddTextBox(100, 100, 1000, 300).SetText("remove this slide")

	var base bytes.Buffer
	if err := p.SaveAsPPTX(&base); err != nil {
		t.Fatalf("save base: %v", err)
	}
	op, err := opc.OpenBytes(base.Bytes())
	if err != nil {
		t.Fatalf("open base: %v", err)
	}
	const customName = "ppt/customXml/edit-vendor.xml"
	op.AddPart(customName, "application/xml", []byte(`<vendor xmlns="urn:edit:test">keep</vendor>`))
	doc, ok, err := op.Relationships("ppt/slides/slide1.xml")
	if err != nil || !ok {
		t.Fatalf("slide rels: ok=%v err=%v", ok, err)
	}
	doc.Relationships = append(doc.Relationships, rels.Relationship{
		ID: "rId999", Type: "urn:edit:test/relationship", Target: "../customXml/edit-vendor.xml",
	})
	relsData, err := xml.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal source rels: %v", err)
	}
	op.AddPart("ppt/slides/_rels/slide1.xml.rels", "application/vnd.openxmlformats-package.relationships+xml", relsData)
	sourceBytes, err := op.WriteToBytes()
	if err != nil {
		t.Fatalf("write source: %v", err)
	}

	edited, err := LoadFromPPTXBytes(sourceBytes)
	if err != nil {
		t.Fatalf("load editable: %v", err)
	}
	if edited.SlideCount() != 2 {
		t.Fatalf("slide count = %d, want 2", edited.SlideCount())
	}
	first := edited.Slide(0)
	for _, sh := range first.Shapes() {
		switch sh.Kind() {
		case presir.ShapeImage:
			if !first.RemoveShape(sh) {
				t.Fatal("remove image shape")
			}
		case presir.ShapeText:
			sh.RemoveHyperlink()
		}
	}
	edited.RemoveSlide(1)

	var outBuf bytes.Buffer
	if err := edited.SaveAsPPTX(&outBuf); err != nil {
		t.Fatalf("save edited: %v", err)
	}
	out, err := opc.OpenBytes(outBuf.Bytes())
	if err != nil {
		t.Fatalf("open edited output: %v", err)
	}
	if _, ok := out.Part(customName); !ok {
		t.Fatal("opaque vendor part was lost through high-level edit/save")
	}
	if _, ok := out.Part("ppt/slides/slide2.xml"); ok {
		t.Fatal("removed slide part remains in package")
	}
	for _, part := range out.Parts() {
		if strings.HasPrefix(part.Name, "ppt/media/") {
			t.Fatalf("orphaned media part remains: %s", part.Name)
		}
	}
	relsDoc, ok, err := out.Relationships("ppt/slides/slide1.xml")
	if err != nil || !ok {
		t.Fatalf("output slide rels: ok=%v err=%v", ok, err)
	}
	var vendorFound bool
	for _, rel := range relsDoc.Relationships {
		if rel.Type == "urn:edit:test/relationship" {
			vendorFound = true
		}
		if strings.HasSuffix(rel.Type, "/image") || strings.HasSuffix(rel.Type, "/hyperlink") {
			t.Fatalf("removed managed relationship was resurrected: %+v", rel)
		}
	}
	if !vendorFound {
		t.Fatal("opaque vendor relationship was lost")
	}

	reloaded, err := LoadFromPPTXBytes(outBuf.Bytes())
	if err != nil {
		t.Fatalf("reload edited output: %v", err)
	}
	if reloaded.SlideCount() != 1 {
		t.Fatalf("reloaded slide count = %d, want 1", reloaded.SlideCount())
	}
}

func TestEditImagePayloadRoundTripsAndCanBeReplaced(t *testing.T) {
	p := NewPresentation()
	shape := p.AddBlankSlide().AddImage(minimalPNG(), "image/png", 0, 0, 1000, 1000)
	var first bytes.Buffer
	if err := p.SaveAsPPTX(&first); err != nil {
		t.Fatalf("save image: %v", err)
	}
	firstPkg, err := opc.OpenBytes(first.Bytes())
	if err != nil {
		t.Fatalf("open image package: %v", err)
	}
	var firstMedia []byte
	for _, part := range firstPkg.Parts() {
		if strings.HasPrefix(part.Name, "ppt/media/") {
			firstMedia = append([]byte(nil), part.Data...)
		}
	}
	if !bytes.Equal(firstMedia, minimalPNG()) {
		t.Fatalf("embedded image payload not written, len=%d", len(firstMedia))
	}

	loaded, err := LoadFromPPTXBytes(first.Bytes())
	if err != nil {
		t.Fatalf("load image: %v", err)
	}
	replacement := append([]byte(nil), minimalPNG()...)
	replacement[len(replacement)-12] ^= 0x01
	loadedShape := loaded.Slide(0).Shapes()[0]
	loadedShape.SetImageData(replacement, "image/png")

	var second bytes.Buffer
	if err := loaded.SaveAsPPTX(&second); err != nil {
		t.Fatalf("save replacement: %v", err)
	}
	secondPkg, err := opc.OpenBytes(second.Bytes())
	if err != nil {
		t.Fatalf("open replacement package: %v", err)
	}
	mediaCount := 0
	for _, part := range secondPkg.Parts() {
		if strings.HasPrefix(part.Name, "ppt/media/") {
			mediaCount++
			if !bytes.Equal(part.Data, replacement) {
				t.Fatal("image replacement did not update embedded payload")
			}
		}
	}
	if mediaCount != 1 {
		t.Fatalf("media parts = %d, want 1", mediaCount)
	}
	_ = shape
}

func TestEditSavePrunesRemovedCommentsAndNotes(t *testing.T) {
	p := NewPresentation()
	s := p.AddBlankSlide()
	s.AddComment("remove me", "Ada", "A", 10, 20)
	s.SetNotesText("remove notes")
	var first bytes.Buffer
	if err := p.SaveAsPPTX(&first); err != nil {
		t.Fatalf("save source: %v", err)
	}
	loaded, err := LoadFromPPTXBytes(first.Bytes())
	if err != nil {
		t.Fatalf("load source: %v", err)
	}
	loaded.Slide(0).ClearComments()
	loaded.Slide(0).RemoveNotes()
	var second bytes.Buffer
	if err := loaded.SaveAsPPTX(&second); err != nil {
		t.Fatalf("save edited: %v", err)
	}
	out, err := opc.OpenBytes(second.Bytes())
	if err != nil {
		t.Fatalf("open edited: %v", err)
	}
	for _, part := range out.Parts() {
		if strings.HasPrefix(part.Name, "ppt/comments/") ||
			strings.HasPrefix(part.Name, "ppt/notesSlides/") ||
			strings.HasPrefix(part.Name, "ppt/notesMasters/") ||
			part.Name == "ppt/commentAuthors.xml" {
			t.Fatalf("removed comment/notes part remains: %s", part.Name)
		}
	}
}

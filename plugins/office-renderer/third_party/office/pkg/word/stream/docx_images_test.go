package stream

import (
	"archive/zip"
	"bytes"
	"testing"
)

func makeDOCXPictureFixture(t *testing.T, drawing string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	write := func(name, content string) {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	write("[Content_Types].xml", `<?xml version="1.0" encoding="UTF-8"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Default Extension="png" ContentType="image/png"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/></Types>`)
	write("_rels/.rels", `<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>`)
	write("word/_rels/document.xml.rels", `<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rIdImg" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/image1.png"/></Relationships>`)
	write("word/document.xml", `<?xml version="1.0" encoding="UTF-8"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:pic="http://schemas.openxmlformats.org/drawingml/2006/picture" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><w:body><w:p><w:r><w:t>before</w:t></w:r><w:r>`+drawing+`</w:r><w:r><w:t>after</w:t></w:r></w:p></w:body></w:document>`)
	write("word/media/image1.png", "\x89PNG\r\n\x1a\nstream-picture")
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func pictureGraphicXML() string {
	return `<a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/picture"><pic:pic><pic:nvPicPr><pic:cNvPr id="0" name="image1.png"/><pic:cNvPicPr/></pic:nvPicPr><pic:blipFill><a:blip r:embed="rIdImg"/><a:stretch><a:fillRect/></a:stretch></pic:blipFill><pic:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="914400" cy="457200"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></pic:spPr></pic:pic></a:graphicData></a:graphic>`
}

func TestDOCXLayoutStreamsInlinePicture(t *testing.T) {
	drawing := `<w:drawing><wp:inline><wp:extent cx="914400" cy="457200"/><wp:docPr id="1" name="Picture 1" descr="inline alt" title="inline title"/>` + pictureGraphicXML() + `</wp:inline></w:drawing>`
	data := makeDOCXPictureFixture(t, drawing)
	source := NewDOCXSource(bytes.NewReader(data), int64(len(data)), DOCXOptions{})
	session := NewLayoutSession(1, 0)
	if err := source.Walk(session); err != nil {
		t.Fatalf("Walk: %v", err)
	}
	doc := session.Document()
	if doc == nil || len(doc.Paragraphs) != 1 {
		t.Fatalf("document = %#v, want one paragraph", doc)
	}
	var imageRunFound bool
	for _, run := range doc.Paragraphs[0].Runs {
		if run.Kind != "image" || run.Image == nil {
			continue
		}
		imageRunFound = true
		if run.Image.Floating {
			t.Fatal("inline picture marked floating")
		}
		if run.Image.MIMEType != "image/png" || len(run.Image.Data) == 0 {
			t.Fatalf("image payload = %q/%d bytes", run.Image.MIMEType, len(run.Image.Data))
		}
		if run.Image.Width != 914400 || run.Image.Height != 457200 {
			t.Fatalf("image extent = %d x %d", run.Image.Width, run.Image.Height)
		}
		if run.Image.Alt != "inline alt" || run.Image.Title != "inline title" {
			t.Fatalf("image metadata = alt %q title %q", run.Image.Alt, run.Image.Title)
		}
	}
	if !imageRunFound {
		t.Fatalf("runs = %#v, want image run", doc.Paragraphs[0].Runs)
	}
}

func TestDOCXLayoutStreamsAnchoredPicture(t *testing.T) {
	drawing := `<w:drawing><wp:anchor distT="12700" distB="25400" distL="38100" distR="50800" simplePos="0" relativeHeight="42" behindDoc="1"><wp:simplePos x="0" y="0"/><wp:positionH relativeFrom="page"><wp:posOffset>100000</wp:posOffset></wp:positionH><wp:positionV relativeFrom="paragraph"><wp:posOffset>200000</wp:posOffset></wp:positionV><wp:extent cx="914400" cy="457200"/><wp:wrapSquare wrapText="right"/><wp:docPr id="2" name="Floating Picture" descr="floating alt"/>` + pictureGraphicXML() + `</wp:anchor></w:drawing>`
	data := makeDOCXPictureFixture(t, drawing)
	source := NewDOCXSource(bytes.NewReader(data), int64(len(data)), DOCXOptions{})
	session := NewLayoutSession(1, 0)
	if err := source.Walk(session); err != nil {
		t.Fatalf("Walk: %v", err)
	}
	doc := session.Document()
	if doc == nil || len(doc.Paragraphs) != 1 {
		t.Fatalf("document = %#v, want one paragraph", doc)
	}
	for _, run := range doc.Paragraphs[0].Runs {
		if run.Kind != "image" || run.Image == nil {
			continue
		}
		img := run.Image
		if !img.Floating || !img.BehindDoc || img.ZIndex != 42 {
			t.Fatalf("anchor flags = floating %v behind %v z %d", img.Floating, img.BehindDoc, img.ZIndex)
		}
		if img.X != 100000 || img.Y != 200000 || img.XRelative != "page" || img.YRelative != "paragraph" {
			t.Fatalf("anchor position = (%d,%d) relative %q/%q", img.X, img.Y, img.XRelative, img.YRelative)
		}
		if img.WrapType != "square" || img.WrapSide != "right" {
			t.Fatalf("anchor wrap = %q/%q", img.WrapType, img.WrapSide)
		}
		if img.DistTop != 1 || img.DistBottom != 2 || img.DistLeft != 3 || img.DistRight != 4 {
			t.Fatalf("anchor distances = %v %v %v %v", img.DistTop, img.DistBottom, img.DistLeft, img.DistRight)
		}
		return
	}
	t.Fatalf("runs = %#v, want floating image run", doc.Paragraphs[0].Runs)
}

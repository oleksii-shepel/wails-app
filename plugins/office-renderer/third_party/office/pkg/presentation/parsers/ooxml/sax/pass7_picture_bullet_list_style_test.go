package sax

import (
	"archive/zip"
	"bytes"
	"testing"

	"office/pkg/presentation/ir"
)

func TestPass7ShapeListStylePictureBulletMaterializesBeforePhysicalLayout(t *testing.T) {
	data := buildPass7PPTX(t, map[string]string{
		"[Content_Types].xml":              `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Default Extension="png" ContentType="image/png"/><Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/><Override PartName="/ppt/slides/slide1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/></Types>`,
		"_rels/.rels":                      `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/></Relationships>`,
		"ppt/presentation.xml":             `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:sldIdLst><p:sldId id="256" r:id="rIdSlide1"/></p:sldIdLst></p:presentation>`,
		"ppt/_rels/presentation.xml.rels":  `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rIdSlide1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/></Relationships>`,
		"ppt/slides/slide1.xml":            `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:cSld><p:spTree><p:sp><p:nvSpPr><p:cNvPr id="2" name="Body"/></p:nvSpPr><p:txBody><a:bodyPr/><a:lstStyle><a:lvl1pPr marL="343080" indent="-343080"><a:buSzPct val="113071"/><a:buBlip><a:blip r:embed="rIdBullet"/></a:buBlip></a:lvl1pPr></a:lstStyle><a:p><a:r><a:rPr sz="2800"/><a:t>The policy context</a:t></a:r></a:p></p:txBody></p:sp></p:spTree></p:cSld></p:sld>`,
		"ppt/slides/_rels/slide1.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rIdBullet" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="../media/bullet.png"/></Relationships>`,
		"ppt/media/bullet.png":             "PNG-BULLET",
	})

	pres, err := ParseBytes(data, Options{})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	p := pres.Slides[0].Shapes[0].Paragraphs[0]
	if p.BulletNone || p.BulletImageRelID != "rIdBullet" {
		t.Fatalf("list-style picture bullet semantics = none:%v rel:%q", p.BulletNone, p.BulletImageRelID)
	}
	if !bytes.Equal(p.BulletImageData, []byte("PNG-BULLET")) || p.BulletImageMIME != "image/png" {
		t.Fatalf("list-style picture bullet payload = %q/%q", string(p.BulletImageData), p.BulletImageMIME)
	}
	if !p.HasMarginLeft || !p.HasTextIndent || p.MarginLeftEMU != 343080 || p.TextIndentEMU != -343080 || p.BulletSizePct != 113071 {
		t.Fatalf("list-style picture bullet geometry = %#v", p)
	}
}

func TestPass7PlaceholderInheritanceCarriesPictureBulletPayload(t *testing.T) {
	src := &ir.Paragraph{
		BulletImageRelID: "rId7",
		BulletImageData:  []byte("PNG-BULLET"),
		BulletImageMIME:  "image/png",
		BulletSizePct:    113071,
	}
	dst := &ir.Paragraph{}

	inheritParagraphFacts(dst, src)

	if dst.BulletNone || dst.BulletImageRelID != "rId7" || !bytes.Equal(dst.BulletImageData, src.BulletImageData) || dst.BulletImageMIME != "image/png" {
		t.Fatalf("inherited picture bullet = %#v", dst)
	}
}

func buildPass7PPTX(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

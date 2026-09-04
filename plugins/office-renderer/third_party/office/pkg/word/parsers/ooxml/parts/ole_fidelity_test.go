package parts

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"strings"
	"testing"

	"office/pkg/ooxml/opc"
)

func TestOLEObjectRunPreservesPreviewAndMetadataOnRoundTrip(t *testing.T) {
	input := `<w:r xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:v="urn:schemas-microsoft-com:vml" xmlns:o="urn:schemas-microsoft-com:office:office"><w:object><v:shape><v:imagedata r:id="rPreview1"/></v:shape><o:OLEObject ProgID="Excel.Sheet.12" Type="Embed" DrawAspect="Content" ObjectID="_123" UpdateMode="Always" r:id="rOle1"/></w:object></w:r>`
	var run Run
	if err := xml.Unmarshal([]byte(input), &run); err != nil {
		t.Fatal(err)
	}
	if len(run.Contents) != 1 || run.Contents[0].OLE == nil {
		t.Fatalf("contents = %#v", run.Contents)
	}
	ole := run.Contents[0].OLE
	if ole.RelID != "rOle1" || ole.PreviewRelID != "rPreview1" || ole.ProgID != "Excel.Sheet.12" || ole.Type != "Embed" || ole.DrawAspect != "Content" || ole.ObjectID != "_123" || ole.UpdateMode != "Always" {
		t.Fatalf("OLE = %#v", ole)
	}
	data, err := xml.Marshal(&run)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{"OLEObject", "rOle1", "rPreview1", "Excel.Sheet.12", "DrawAspect=\"Content\""} {
		if !strings.Contains(text, want) {
			t.Fatalf("marshal missing %q: %s", want, text)
		}
	}
	var again Run
	if err := xml.Unmarshal(data, &again); err != nil {
		t.Fatal(err)
	}
	if len(again.Contents) != 1 || again.Contents[0].OLE == nil || again.Contents[0].OLE.PreviewRelID != "rPreview1" {
		t.Fatalf("round trip = %#v", again.Contents)
	}
}

func TestLoadPackageLoadsEmbeddedAndLinkedOLERelationships(t *testing.T) {
	data := buildOLETestPackage(t, map[string]string{
		"[Content_Types].xml":            `<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/><Override PartName="/word/embeddings/oleObject1.bin" ContentType="application/vnd.openxmlformats-officedocument.oleObject"/></Types>`,
		"_rels/.rels":                    `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>`,
		"word/document.xml":              `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:o="urn:schemas-microsoft-com:office:office"><w:body><w:p><w:r><w:object><o:OLEObject ProgID="Excel.Sheet.12" r:id="rEmbed"/></w:object></w:r><w:r><w:object><o:OLEObject ProgID="Word.Document.12" Type="Link" r:id="rLink"/></w:object></w:r></w:p></w:body></w:document>`,
		"word/_rels/document.xml.rels":   `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rEmbed" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/oleObject" Target="embeddings/oleObject1.bin"/><Relationship Id="rLink" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/oleObject" Target="file:///C:/Data/source.xlsx" TargetMode="External"/></Relationships>`,
		"word/embeddings/oleObject1.bin": "OLE-PAYLOAD",
	})
	opcPkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := LoadPackage(opcPkg)
	if err != nil {
		t.Fatal(err)
	}
	embedded, ok := pkg.OLEPartFor(pkg.MainPartName, "rEmbed")
	if !ok || string(embedded.Data) != "OLE-PAYLOAD" {
		t.Fatalf("embedded = %#v %v", embedded, ok)
	}
	link, ok := pkg.OLELinkFor(pkg.MainPartName, "rLink")
	if !ok || link != "file:///C:/Data/source.xlsx" {
		t.Fatalf("link = %q %v", link, ok)
	}
}

func buildOLETestPackage(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestWritePackageRoundTripsEmbeddedOLEPayload(t *testing.T) {
	pkg := &Package{
		Document: &Document{Body: &Body{Paragraphs: []*Paragraph{{Runs: []*Run{{Contents: []RunContent{{Kind: RunContentOLE, OLE: &OLEObjectRun{ProgID: "Excel.Sheet.12", RelID: "rOle1", Type: "Embed"}}}}}}}}},
	}
	pkg.SetOLEPart("word/document.xml", "rOle1", OLEPart{ContentType: "application/vnd.openxmlformats-officedocument.oleObject", Data: []byte("OLE-BYTES")}, "")
	var buf bytes.Buffer
	if err := WritePackage(pkg, &buf); err != nil {
		t.Fatal(err)
	}
	opcPkg, err := opc.OpenBytes(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadPackage(opcPkg)
	if err != nil {
		t.Fatal(err)
	}
	part, ok := loaded.OLEPartFor(loaded.MainPartName, "rOle1")
	if !ok || string(part.Data) != "OLE-BYTES" {
		t.Fatalf("OLE part = %#v, %v", part, ok)
	}
	if len(loaded.Document.Body.Paragraphs) != 1 || len(loaded.Document.Body.Paragraphs[0].Runs) != 1 || len(loaded.Document.Body.Paragraphs[0].Runs[0].Contents) != 1 || loaded.Document.Body.Paragraphs[0].Runs[0].Contents[0].OLE == nil {
		t.Fatalf("loaded OLE run missing: %#v", loaded.Document.Body)
	}
}
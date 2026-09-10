package sax

import (
	"archive/zip"
	"bytes"
	"testing"
	"strings"
)

func TestWalkBytesDoesNotDiscardAlternateContentWordprocessingTextBox(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	parts := map[string]string{
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006" xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:wps="http://schemas.microsoft.com/office/word/2010/wordprocessingShape" xmlns:v="urn:schemas-microsoft-com:vml"><w:body><w:p><w:r><mc:AlternateContent><mc:Choice Requires="wps"><w:drawing><wp:anchor><wp:extent cx="1270000" cy="508000"/><a:graphic><a:graphicData uri="http://schemas.microsoft.com/office/word/2010/wordprocessingShape"><wps:wsp><wps:txbx><w:txbxContent><w:p><w:r><w:t>VISIBLE</w:t></w:r></w:p></w:txbxContent></wps:txbx></wps:wsp></a:graphicData></a:graphic></wp:anchor></w:drawing></mc:Choice><mc:Fallback><w:pict><v:shape><v:textbox><w:txbxContent><w:p><w:r><w:t>VISIBLE</w:t></w:r></w:p></w:txbxContent></v:textbox></v:shape></w:pict></mc:Fallback></mc:AlternateContent></w:r></w:p></w:body></w:document>`,
	}
	for name, body := range parts {
		w, err := zw.Create(name); if err != nil { t.Fatal(err) }
		if _, err := w.Write([]byte(body)); err != nil { t.Fatal(err) }
	}
	if err := zw.Close(); err != nil { t.Fatal(err) }

	count := 0
	var got Event
	if err := WalkBytes(buf.Bytes(), HandlerFunc(func(e Event) error {
		if e.Type == EventDrawing && e.DrawingHasTextBox {
			count++
			got = e
		}
		return nil
	}), Options{}); err != nil { t.Fatal(err) }

	if count != 1 {
		t.Fatalf("textbox drawing count = %d, want exactly 1 compatibility branch", count)
	}
	if got.DrawingGraphicKind != "shape" || !strings.Contains(got.RawXML, "VISIBLE") {
		t.Fatalf("textbox event = %#v", got)
	}
}

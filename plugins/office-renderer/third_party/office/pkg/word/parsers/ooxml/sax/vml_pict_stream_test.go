package sax

import (
	"archive/zip"
	"bytes"
	"testing"
)

func TestStreamingPreservesVMLPictAsDrawingEvent(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	parts := map[string]string{
		"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:v="urn:schemas-microsoft-com:vml"><w:body><w:p><w:r><w:pict><v:shape style="position:absolute;width:100pt;height:40pt"><v:textbox><w:txbxContent><w:p><w:r><w:t>VISIBLE</w:t></w:r></w:p></w:txbxContent></v:textbox></v:shape></w:pict></w:r></w:p></w:body></w:document>`,
	}
	for name, body := range parts {
		w, err := zw.Create(name)
		if err != nil { t.Fatal(err) }
		if _, err := w.Write([]byte(body)); err != nil { t.Fatal(err) }
	}
	if err := zw.Close(); err != nil { t.Fatal(err) }

	var got *Event
	err := WalkBytes(buf.Bytes(), HandlerFunc(func(e Event) error {
		if e.Type == EventDrawing {
			copy := e
			got = &copy
		}
		return nil
	}), Options{})
	if err != nil { t.Fatal(err) }
	if got == nil || got.DrawingGraphicKind != "shape" || !got.DrawingHasTextBox || !bytes.Contains([]byte(got.RawXML), []byte("VISIBLE")) {
		t.Fatalf("VML pict event = %#v", got)
	}
}

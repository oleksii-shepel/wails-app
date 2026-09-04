package legacy

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"strings"
	"testing"

	wordir "office/pkg/word/ir"
)

func TestLegacyExtractOfficeArtEMFAndConvertToSVG(t *testing.T) {
	emf := make([]byte, 128)
	binary.LittleEndian.PutUint32(emf[0:4], emrHeader)
	binary.LittleEndian.PutUint32(emf[4:8], 108)
	binary.LittleEndian.PutUint32(emf[8:12], 0)
	binary.LittleEndian.PutUint32(emf[12:16], 0)
	binary.LittleEndian.PutUint32(emf[16:20], 256)
	binary.LittleEndian.PutUint32(emf[20:24], 52)
	binary.LittleEndian.PutUint32(emf[108:112], emrEOF)
	binary.LittleEndian.PutUint32(emf[112:116], 20)

	var compressed bytes.Buffer
	zw := zlib.NewWriter(&compressed)
	if _, err := zw.Write(emf); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	payload := make([]byte, 50+compressed.Len())
	binary.LittleEndian.PutUint32(payload[16:20], uint32(len(emf)))
	binary.LittleEndian.PutUint32(payload[44:48], uint32(compressed.Len()))
	copy(payload[50:], compressed.Bytes())

	extracted, ok := legacyExtractOfficeArtMetafile(legacyOfficeArtBlipEMF, payload)
	if !ok || !bytes.Equal(extracted, emf) {
		t.Fatalf("metafile extraction failed: ok=%v len=%d", ok, len(extracted))
	}
	svg, w, h, ok := legacyEMFToSVG(extracted)
	if !ok || w != 256 || h != 52 {
		t.Fatalf("EMF->SVG geometry = ok:%v %.1fx%.1f", ok, w, h)
	}
	if !strings.Contains(string(svg), `viewBox="0 0 256.000 52.000"`) {
		t.Fatalf("SVG = %s", svg)
	}
}

func TestLegacyEMFLooksLikeTextGrid(t *testing.T) {
	var data []byte
	appendRecord := func(kind uint32) {
		rec := make([]byte, 8)
		binary.LittleEndian.PutUint32(rec[0:4], kind)
		binary.LittleEndian.PutUint32(rec[4:8], 8)
		data = append(data, rec...)
	}
	for i := 0; i < 6; i++ {
		appendRecord(emrExtTextOutW)
	}
	for i := 0; i < 4; i++ {
		appendRecord(emrLineTo)
	}
	if !legacyEMFLooksLikeTextGrid(data) {
		t.Fatal("text/grid EMF was not recognized")
	}
	if legacyEMFLooksLikeTextGrid(data[:5*8]) {
		t.Fatal("short non-grid EMF was incorrectly recognized")
	}
}

func TestAttachLegacyOLEMetafilePreviewPrefersCenteredObjectParagraph(t *testing.T) {
	preview := &wordir.Image{MIMEType: "image/svg+xml", Data: []byte("<svg/>"), Width: 100, Height: 50}
	doc := &wordir.Document{Blocks: []wordir.Block{
		{Kind: "paragraph", Paragraph: &wordir.Paragraph{Runs: []wordir.Run{{Kind: "olePlaceholder", Text: "Embedded Object"}}}},
		{Kind: "paragraph", Paragraph: &wordir.Paragraph{Runs: []wordir.Run{{Text: "before"}, {Kind: "olePlaceholder", Text: "Embedded Object"}, {Text: "after"}}}},
		{Kind: "paragraph", Paragraph: &wordir.Paragraph{Format: wordir.ParagraphFormat{Alignment: "center"}, Runs: []wordir.Run{{Kind: "olePlaceholder", Text: "Embedded Object"}}}},
	}}

	attachLegacyOLEMetafilePreviews(doc, []*wordir.Image{preview})

	if got := doc.Blocks[0].Paragraph.Runs[0].OLEPreview; got != nil {
		t.Fatalf("first structural OLE anchor received preview: %+v", got)
	}
	if got := doc.Blocks[1].Paragraph.Runs[1].OLEPreview; got != nil {
		t.Fatalf("inline structural OLE anchor received preview: %+v", got)
	}
	got := doc.Blocks[2].Paragraph.Runs[0].OLEPreview
	if got == nil || got.MIMEType != "image/svg+xml" || string(got.Data) != "<svg/>" {
		t.Fatalf("centered visible OLE preview = %+v", got)
	}
	if &got.Data[0] == &preview.Data[0] {
		t.Fatal("preview payload was not cloned")
	}
}

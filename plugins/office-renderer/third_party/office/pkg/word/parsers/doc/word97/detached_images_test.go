package word97

import (
	"bytes"
	"testing"

	wordir "office/pkg/word/ir"
)

func TestAppendDetachedDrawingImagesSkipsCanonicalDuplicate(t *testing.T) {
	data := []byte{1, 2, 3, 4}
	doc := &wordir.Document{Paragraphs: []wordir.Paragraph{{
		Runs: []wordir.Run{{Kind: "image", Image: &wordir.Image{Data: append([]byte(nil), data...)}}},
	}}}
	AppendDetachedDrawingImages(doc, []*wordir.Image{{Data: data}})
	if len(doc.Paragraphs) != 1 {
		t.Fatalf("paragraphs=%d, want duplicate suppressed", len(doc.Paragraphs))
	}
}

func TestAppendDetachedDrawingImagesPreservesUncorrelatedBLIP(t *testing.T) {
	data := []byte{5, 6, 7, 8}
	doc := &wordir.Document{}
	source := &wordir.Image{MIMEType: "image/png", Data: data}
	AppendDetachedDrawingImages(doc, []*wordir.Image{source})
	if len(doc.Paragraphs) != 1 || len(doc.Blocks) != 1 {
		t.Fatalf("detached image missing: paragraphs=%d blocks=%d", len(doc.Paragraphs), len(doc.Blocks))
	}
	got := doc.Paragraphs[0].Runs[0].Image
	if got == nil || !got.Floating || got.MIMEType != "image/png" || !bytes.Equal(got.Data, data) {
		t.Fatalf("image=%#v", got)
	}
	if got == source || &got.Data[0] == &source.Data[0] {
		t.Fatal("detached projection must own its image payload copy")
	}
}

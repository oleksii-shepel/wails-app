package media_test

import (
	"archive/zip"
	"bytes"
	"testing"

	"office/pkg/ooxml/media"
	"office/pkg/ooxml/opc"
)

func TestFromPackageDiscoversMediaParts(t *testing.T) {
	data := buildPackage(t, map[string][]byte{
		"[Content_Types].xml": []byte(`<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Default Extension="png" ContentType="image/png"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
</Types>`),
		"_rels/.rels": []byte(`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`),
		"word/document.xml":      []byte(`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body/></w:document>`),
		"word/media/image1.png":  []byte{0x89, 0x50, 0x4E, 0x47},
		"custom/media/audio.bin": []byte{0x01, 0x02},
		"docProps/app.xml":       []byte(`<Properties/>`),
	})

	pkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}

	registry := media.FromPackage(pkg)
	parts := registry.Parts()
	if got := len(parts); got != 2 {
		t.Fatalf("media part count = %d, want 2", got)
	}
	if _, ok := registry.Part("word/media/image1.png"); !ok {
		t.Fatal("expected image part lookup to succeed")
	}
	if _, ok := registry.Part("/custom/media/audio.bin"); !ok {
		t.Fatal("expected path normalization for media lookup")
	}
}

func buildPackage(t *testing.T, files map[string][]byte) []byte {
	t.Helper()

	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, data := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		if _, err := entry.Write(data); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buffer.Bytes()
}

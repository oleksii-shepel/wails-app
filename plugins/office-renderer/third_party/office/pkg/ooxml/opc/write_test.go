package opc_test

import (
	"bytes"
	"testing"

	"office/pkg/ooxml/opc"
)

func TestWriteRoundTrip(t *testing.T) {
	original := buildPackage(t, map[string]string{
		"[Content_Types].xml": contentTypesXML(
			defaultEntry("rels", "application/vnd.openxmlformats-package.relationships+xml"),
			defaultEntry("xml", "application/xml"),
			defaultEntry("png", "image/png"),
			overrideEntry("/word/document.xml", "application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"),
		),
		"_rels/.rels":                  rootRelsXML("word/document.xml"),
		"word/document.xml":            `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>Hello</w:t></w:r></w:p></w:body></w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/image1.png"/></Relationships>`,
		"word/media/image1.png":        "png-data",
	})

	// Load original.
	pkg, err := opc.OpenBytes(original)
	if err != nil {
		t.Fatalf("open original: %v", err)
	}

	// Write to bytes.
	written, err := pkg.WriteToBytes()
	if err != nil {
		t.Fatalf("write: %v", err)
	}

	// Reload.
	pkg2, err := opc.OpenBytes(written)
	if err != nil {
		t.Fatalf("open written: %v", err)
	}

	// Verify package type preserved.
	if got, want := pkg2.PackageType(), opc.PackageTypeWord; got != want {
		t.Fatalf("package type = %q, want %q", got, want)
	}

	// Verify parts exist.
	for _, name := range []string{"word/document.xml", "word/media/image1.png", "_rels/.rels", "word/_rels/document.xml.rels"} {
		if _, ok := pkg2.Part(name); !ok {
			t.Fatalf("missing part %q after round-trip", name)
		}
	}

	// Verify relationship resolution still works.
	part, _, ok, err := pkg2.ResolveRelationship("word/document.xml", "rId1")
	if err != nil {
		t.Fatalf("resolve relationship: %v", err)
	}
	if !ok {
		t.Fatal("expected image relationship to resolve after round-trip")
	}
	if part.Name != "word/media/image1.png" {
		t.Fatalf("resolved part = %q, want word/media/image1.png", part.Name)
	}

	// Verify content types.
	if ct := pkg2.ContentType("word/document.xml"); ct != "application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml" {
		t.Fatalf("content type = %q", ct)
	}
}

func TestAddPartAndWrite(t *testing.T) {
	original := buildPackage(t, map[string]string{
		"[Content_Types].xml": contentTypesXML(
			defaultEntry("rels", "application/vnd.openxmlformats-package.relationships+xml"),
			defaultEntry("xml", "application/xml"),
			overrideEntry("/word/document.xml", "application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"),
		),
		"_rels/.rels":       rootRelsXML("word/document.xml"),
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>Hello</w:t></w:r></w:p></w:body></w:document>`,
	})

	pkg, err := opc.OpenBytes(original)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	// Add a new part.
	pkg.AddPart("word/media/image2.png", "image/png", []byte("new-png-data"))

	// Verify added.
	part, ok := pkg.Part("word/media/image2.png")
	if !ok {
		t.Fatal("expected added part to exist")
	}
	if !bytes.Equal(part.Data, []byte("new-png-data")) {
		t.Fatal("added part data mismatch")
	}

	// Write and reload.
	written, err := pkg.WriteToBytes()
	if err != nil {
		t.Fatalf("write: %v", err)
	}

	pkg2, err := opc.OpenBytes(written)
	if err != nil {
		t.Fatalf("open written: %v", err)
	}

	part2, ok := pkg2.Part("word/media/image2.png")
	if !ok {
		t.Fatal("expected added part after round-trip")
	}
	if !bytes.Equal(part2.Data, []byte("new-png-data")) {
		t.Fatal("added part data mismatch after round-trip")
	}

	// Content type should be preserved.
	if ct := pkg2.ContentType("word/media/image2.png"); ct != "image/png" {
		t.Fatalf("content type = %q, want image/png", ct)
	}
}

func TestRemovePartAndWrite(t *testing.T) {
	original := buildPackage(t, map[string]string{
		"[Content_Types].xml": contentTypesXML(
			defaultEntry("rels", "application/vnd.openxmlformats-package.relationships+xml"),
			defaultEntry("xml", "application/xml"),
			defaultEntry("png", "image/png"),
			overrideEntry("/word/document.xml", "application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"),
		),
		"_rels/.rels":                  rootRelsXML("word/document.xml"),
		"word/document.xml":            `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>Hello</w:t></w:r></w:p></w:body></w:document>`,
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/image1.png"/></Relationships>`,
		"word/media/image1.png":        "png-data",
	})

	pkg, err := opc.OpenBytes(original)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	// Remove image part.
	pkg.RemovePart("word/media/image1.png")

	// Verify removed.
	if _, ok := pkg.Part("word/media/image1.png"); ok {
		t.Fatal("expected removed part to not exist")
	}

	// Write and reload.
	written, err := pkg.WriteToBytes()
	if err != nil {
		t.Fatalf("write: %v", err)
	}

	pkg2, err := opc.OpenBytes(written)
	if err != nil {
		t.Fatalf("open written: %v", err)
	}

	if _, ok := pkg2.Part("word/media/image1.png"); ok {
		t.Fatal("expected removed part to not exist after round-trip")
	}
}

func TestDocumentPartCommitter(t *testing.T) {
	original := buildPackage(t, map[string]string{
		"[Content_Types].xml": contentTypesXML(
			defaultEntry("rels", "application/vnd.openxmlformats-package.relationships+xml"),
			defaultEntry("xml", "application/xml"),
			overrideEntry("/word/document.xml", "application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"),
		),
		"_rels/.rels":       rootRelsXML("word/document.xml"),
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>Hello</w:t></w:r></w:p></w:body></w:document>`,
	})

	pkg, err := opc.OpenBytes(original)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	// Use a custom DocumentPart.
	part := &testPart{
		name:        "word/styles.xml",
		contentType: "application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml",
		data:        []byte(`<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"></w:styles>`),
	}

	if err := pkg.AddPartFromCommitter(part); err != nil {
		t.Fatalf("add part from committer: %v", err)
	}

	written, err := pkg.WriteToBytes()
	if err != nil {
		t.Fatalf("write: %v", err)
	}

	pkg2, err := opc.OpenBytes(written)
	if err != nil {
		t.Fatalf("open written: %v", err)
	}

	p, ok := pkg2.Part("word/styles.xml")
	if !ok {
		t.Fatal("expected committed part after round-trip")
	}
	if !bytes.Equal(p.Data, part.data) {
		t.Fatal("committed part data mismatch")
	}
}

type testPart struct {
	name        string
	contentType string
	data        []byte
}

func (t *testPart) PartName() string        { return t.name }
func (t *testPart) ContentType() string     { return t.contentType }
func (t *testPart) Commit() ([]byte, error) { return t.data, nil }

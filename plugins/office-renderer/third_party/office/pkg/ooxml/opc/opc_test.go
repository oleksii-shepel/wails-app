package opc_test

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"office/pkg/ooxml/media"
	"office/pkg/ooxml/opc"
	"office/pkg/ooxml/xmltree"
)

func TestOpenBytesDetectsWordPackage(t *testing.T) {
	data := buildPackage(t, map[string]string{
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

	pkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open bytes: %v", err)
	}

	if got, want := pkg.PackageType(), opc.PackageTypeWord; got != want {
		t.Fatalf("package type = %q, want %q", got, want)
	}

	part, _, ok, err := pkg.ResolveRelationship("word/document.xml", "rId1")
	if err != nil {
		t.Fatalf("resolve relationship: %v", err)
	}
	if !ok {
		t.Fatal("expected image relationship target to resolve")
	}
	if part.Name != "word/media/image1.png" {
		t.Fatalf("resolved part = %q", part.Name)
	}

	registry := media.FromPackage(pkg)
	if got := len(registry.Parts()); got != 1 {
		t.Fatalf("media parts = %d, want 1", got)
	}

	doc, ok := pkg.Part("WORD/DOCUMENT.XML")
	if !ok {
		t.Fatal("case-insensitive lookup failed")
	}
	tree, err := xmltree.Parse(doc.Data)
	if err != nil {
		t.Fatalf("parse xml tree: %v", err)
	}
	if tree.Name.Local != "document" {
		t.Fatalf("root = %q, want document", tree.Name.Local)
	}
}

func TestOpenBytesDetectsSpreadsheetPackage(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": contentTypesXML(
			defaultEntry("rels", "application/vnd.openxmlformats-package.relationships+xml"),
			defaultEntry("xml", "application/xml"),
			overrideEntry("/xl/workbook.xml", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"),
		),
		"_rels/.rels":     rootRelsXML("xl/workbook.xml"),
		"xl/workbook.xml": `<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheets/></workbook>`,
	})

	pkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open bytes: %v", err)
	}

	if got, want := pkg.PackageType(), opc.PackageTypeSpreadsheet; got != want {
		t.Fatalf("package type = %q, want %q", got, want)
	}
}

func TestOpenBytesDetectsPresentationPackage(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": contentTypesXML(
			defaultEntry("rels", "application/vnd.openxmlformats-package.relationships+xml"),
			defaultEntry("xml", "application/xml"),
			overrideEntry("/ppt/presentation.xml", "application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"),
		),
		"_rels/.rels":          rootRelsXML("ppt/presentation.xml"),
		"ppt/presentation.xml": `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"/>`,
	})

	pkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open bytes: %v", err)
	}

	if got, want := pkg.PackageType(), opc.PackageTypePresentation; got != want {
		t.Fatalf("package type = %q, want %q", got, want)
	}
}

func TestOpenBytesDetectsPresentationPackageWithStrictOfficeDocumentRelationship(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": contentTypesXML(
			defaultEntry("rels", "application/vnd.openxmlformats-package.relationships+xml"),
			defaultEntry("xml", "application/xml"),
			overrideEntry("/ppt/presentation.xml", "application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"),
		),
		"_rels/.rels":          `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://purl.oclc.org/ooxml/officeDocument/relationships/officeDocument" Target="ppt/presentation.xml"/></Relationships>`,
		"ppt/presentation.xml": `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"/>`,
	})

	pkg, err := opc.OpenBytes(data)
	if err != nil {
		t.Fatalf("open bytes: %v", err)
	}
	if got, want := pkg.PackageType(), opc.PackageTypePresentation; got != want {
		t.Fatalf("package type = %q, want %q", got, want)
	}
}

func TestOpenStreamDetectsWordPackage(t *testing.T) {
	data := buildPackage(t, map[string]string{
		"[Content_Types].xml": contentTypesXML(
			defaultEntry("rels", "application/vnd.openxmlformats-package.relationships+xml"),
			defaultEntry("xml", "application/xml"),
			overrideEntry("/word/document.xml", "application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"),
		),
		"_rels/.rels":       rootRelsXML("word/document.xml"),
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>Hello</w:t></w:r></w:p></w:body></w:document>`,
	})

	pkg, err := opc.OpenStream(bytes.NewBuffer(data))
	if err != nil {
		t.Fatalf("open stream: %v", err)
	}
	if got, want := pkg.PackageType(), opc.PackageTypeWord; got != want {
		t.Fatalf("package type = %q, want %q", got, want)
	}
	if _, ok := pkg.Part("word/document.xml"); !ok {
		t.Fatal("expected word/document.xml in package")
	}
}

func TestOpenStreamDecryptsEncryptedOLEPackage(t *testing.T) {
	oldPassword := opc.Password
	opc.Password = "solrcell"
	defer func() { opc.Password = oldPassword }()

	file, err := os.Open(repoPath(t, "testdata", "docx", "bug53475-password-is-solrcell.docx"))
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer func() {
		_ = file.Close()
	}()

	pkg, err := opc.OpenStream(io.LimitReader(file, 1<<62))
	if err != nil {
		t.Fatalf("open encrypted stream: %v", err)
	}
	if got, want := pkg.PackageType(), opc.PackageTypeWord; got != want {
		t.Fatalf("package type = %q, want %q", got, want)
	}
	if _, ok := pkg.Part("word/document.xml"); !ok {
		t.Fatal("expected word/document.xml in decrypted package")
	}
}

func TestOpenDecryptsStandardEncryptedOLEPackage(t *testing.T) {
	oldPassword := opc.Password
	opc.Password = "solrcell"
	defer func() { opc.Password = oldPassword }()

	pkg, err := opc.Open(repoPath(t, "testdata", "docx", "bug53475-password-is-solrcell.docx"))
	if err != nil {
		t.Fatalf("open encrypted standard package: %v", err)
	}
	if got, want := pkg.PackageType(), opc.PackageTypeWord; got != want {
		t.Fatalf("package type = %q, want %q", got, want)
	}
	if _, ok := pkg.Part("word/document.xml"); !ok {
		t.Fatal("expected word/document.xml in decrypted package")
	}
}

func TestOpenDecryptsAgileEncryptedOLEPackage(t *testing.T) {
	oldPassword := opc.Password
	opc.Password = ""
	defer func() { opc.Password = oldPassword }()

	pkg, err := opc.Open(repoPath(t, "testdata", "docx", "protected_agile.docx"))
	if err != nil {
		t.Fatalf("open encrypted agile package: %v", err)
	}
	if got, want := pkg.PackageType(), opc.PackageTypeWord; got != want {
		t.Fatalf("package type = %q, want %q", got, want)
	}
	if _, ok := pkg.Part("word/document.xml"); !ok {
		t.Fatal("expected word/document.xml in decrypted package")
	}
}

func TestOpenDecryptsPasswordProtectedOLEPackage(t *testing.T) {
	oldPassword := opc.Password
	opc.Password = "Test001!!"
	defer func() { opc.Password = oldPassword }()

	pkg, err := opc.Open(repoPath(t, "testdata", "xlsx", "60320-protected.xlsx"))
	if err != nil {
		t.Fatalf("open password-protected package: %v", err)
	}
	if got, want := pkg.PackageType(), opc.PackageTypeSpreadsheet; got != want {
		t.Fatalf("package type = %q, want %q", got, want)
	}
	if _, ok := pkg.Part("xl/workbook.xml"); !ok {
		t.Fatal("expected xl/workbook.xml in decrypted package")
	}
}

func TestOpenDecryptsStandardAES256OLEPackage(t *testing.T) {
	oldPassword := opc.Password
	opc.Password = "pass"
	defer func() { opc.Password = oldPassword }()

	pkg, err := opc.Open(repoPath(t, "testdata", "docx", "bug53475-password-is-pass.docx"))
	if err != nil {
		t.Fatalf("open encrypted standard AES-256 package: %v", err)
	}
	if got, want := pkg.PackageType(), opc.PackageTypeWord; got != want {
		t.Fatalf("package type = %q, want %q", got, want)
	}
	if _, ok := pkg.Part("word/document.xml"); !ok {
		t.Fatal("expected word/document.xml in decrypted package")
	}
}

func TestOpenDecryptsStandardOLEPackageWithTikaPassword(t *testing.T) {
	oldPassword := opc.Password
	opc.Password = "tika"
	defer func() { opc.Password = oldPassword }()

	pkg, err := opc.Open(repoPath(t, "testdata", "xlsx", "protected_passtika.xlsx"))
	if err != nil {
		t.Fatalf("open protected_passtika.xlsx: %v", err)
	}
	if got, want := pkg.PackageType(), opc.PackageTypeSpreadsheet; got != want {
		t.Fatalf("package type = %q, want %q", got, want)
	}
	if _, ok := pkg.Part("xl/workbook.xml"); !ok {
		t.Fatal("expected xl/workbook.xml in decrypted package")
	}
}

func TestOpenDecryptsStandardOLEPackageWithPwd123(t *testing.T) {
	oldPassword := opc.Password
	opc.Password = "pwd123"
	defer func() { opc.Password = oldPassword }()

	pkg, err := opc.Open(repoPath(t, "testdata", "xlsx", "extenxls_pwd123.xlsx"))
	if err != nil {
		t.Fatalf("open extenxls_pwd123.xlsx: %v", err)
	}
	if got, want := pkg.PackageType(), opc.PackageTypeSpreadsheet; got != want {
		t.Fatalf("package type = %q, want %q", got, want)
	}
	if _, ok := pkg.Part("xl/workbook.xml"); !ok {
		t.Fatal("expected xl/workbook.xml in decrypted package")
	}
}

func TestOpenDecryptsAgileSHA512OLEPackage(t *testing.T) {
	oldPassword := opc.Password
	opc.Password = "this is a test"
	defer func() { opc.Password = oldPassword }()

	pkg, err := opc.Open(repoPath(t, "testdata", "xlsx", "protected_sha512.xlsx"))
	if err != nil {
		t.Fatalf("open protected_sha512.xlsx: %v", err)
	}
	if got, want := pkg.PackageType(), opc.PackageTypeSpreadsheet; got != want {
		t.Fatalf("package type = %q, want %q", got, want)
	}
	if _, ok := pkg.Part("xl/workbook.xml"); !ok {
		t.Fatal("expected xl/workbook.xml in decrypted package")
	}
}

func TestOpenRejectsWrongOOXMLPassword(t *testing.T) {
	oldPassword := opc.Password
	opc.Password = "wrong"
	defer func() { opc.Password = oldPassword }()

	_, err := opc.Open(repoPath(t, "testdata", "xlsx", "60320-protected.xlsx"))
	if err == nil {
		t.Fatal("expected invalid password error")
	}
	if !strings.Contains(err.Error(), "invalid password") {
		t.Fatalf("expected invalid password error, got %v", err)
	}
}

func TestOpenRejectsWrongStandardOOXMLPassword(t *testing.T) {
	oldPassword := opc.Password
	opc.Password = "wrong"
	defer func() { opc.Password = oldPassword }()

	_, err := opc.Open(repoPath(t, "testdata", "docx", "bug53475-password-is-solrcell.docx"))
	if err == nil {
		t.Fatal("expected invalid password error")
	}
	if !strings.Contains(err.Error(), "invalid password") {
		t.Fatalf("expected invalid password error, got %v", err)
	}
}

func buildPackage(t *testing.T, files map[string]string) []byte {
	t.Helper()

	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)

	for name, data := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatalf("create zip entry %s: %v", name, err)
		}
		if _, err := entry.Write([]byte(data)); err != nil {
			t.Fatalf("write zip entry %s: %v", name, err)
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}

	return buffer.Bytes()
}

func contentTypesXML(entries ...string) string {
	return `<?xml version="1.0" encoding="UTF-8"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
		join(entries...) +
		`</Types>`
}

func defaultEntry(ext, contentType string) string {
	return `<Default Extension="` + ext + `" ContentType="` + contentType + `"/>`
}

func overrideEntry(partName, contentType string) string {
	return `<Override PartName="` + partName + `" ContentType="` + contentType + `"/>`
}

func rootRelsXML(target string) string {
	return `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="` + target + `"/></Relationships>`
}

func join(values ...string) string {
	var out string
	for _, value := range values {
		out += value
	}
	return out
}

func repoPath(t *testing.T, elems ...string) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	parts := append([]string{filepath.Dir(file), "..", "..", ".."}, elems...)
	return filepath.Clean(filepath.Join(parts...))
}

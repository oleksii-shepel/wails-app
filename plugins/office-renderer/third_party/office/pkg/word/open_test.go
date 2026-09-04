package word

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestDetectFormat(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want Format
	}{
		{name: "report.docx", want: FormatDOCX},
		{name: "report.docm", want: FormatDOCX},
		{name: "template.dotx", want: FormatDOCX},
		{name: "legacy.doc", want: FormatDOC},
		{name: "unknown.bin", data: []byte("PK\x03\x04rest"), want: FormatDOCX},
		{name: "unknown.bin", data: []byte{0xd0, 0xcf, 0x11, 0xe0, 0xa1, 0xb1, 0x1a, 0xe1}, want: FormatDOC},
		{name: "unknown.bin", data: []byte("plain text"), want: FormatUnknown},
	}
	for _, tt := range tests {
		if got := DetectFormat(tt.name, tt.data); got != tt.want {
			t.Fatalf("DetectFormat(%q) = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestOpenBytesDOCXFacade(t *testing.T) {
	data := makeDOCXFixture(t, "hello docx")
	doc, err := OpenBytes("fixture.docx", data)
	if err != nil {
		t.Fatalf("OpenBytes DOCX: %v", err)
	}
	assertDocumentContainsText(t, doc, "hello docx")

	html, err := HTML(doc)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(html, "hello docx") {
		t.Fatalf("HTML output lost fixture text: %s", html)
	}
}

func TestOpenBytesDOCFacadePreservesLegacySource(t *testing.T) {
	data := makeDOCFixture(t, "hello doc")
	doc, err := OpenBytes("fixture.doc", data)
	if err != nil {
		t.Fatalf("OpenBytes DOC: %v", err)
	}
	assertDocumentContainsText(t, doc, "hello doc")
	if doc.Source == nil {
		t.Fatal("OpenBytes DOC lost legacy Document.Source")
	}
	if doc.Source.Version == 0 {
		t.Fatal("OpenBytes DOC lost legacy FIB version")
	}

	html, err := HTML(doc)
	if err != nil {
		t.Fatalf("HTML legacy DOC: %v", err)
	}
	if !strings.Contains(html, "hello doc") {
		t.Fatalf("legacy DOC HTML output lost fixture text: %s", html)
	}
}

func TestConvertBytesToDOCXFromLegacyDOC(t *testing.T) {
	legacyData := makeDOCFixture(t, "converted legacy doc")
	var out bytes.Buffer
	if err := ConvertBytesToDOCX("fixture.doc", legacyData, &out); err != nil {
		t.Fatalf("ConvertBytesToDOCX: %v", err)
	}
	if out.Len() == 0 {
		t.Fatal("ConvertBytesToDOCX produced empty output")
	}

	converted, err := OpenBytes("converted.docx", out.Bytes())
	if err != nil {
		t.Fatalf("reopen converted DOCX: %v", err)
	}
	assertDocumentContainsText(t, converted, "converted legacy doc")
	if converted.Source != nil && converted.Source.Version != 0 {
		t.Fatalf("converted DOCX unexpectedly retained legacy FIB version: %#v", converted.Source)
	}
}

func TestConvertBytesToDOCXAcceptsDOCXInput(t *testing.T) {
	source := makeDOCXFixture(t, "already docx")
	var out bytes.Buffer
	if err := ConvertBytesToDOCX("fixture.docx", source, &out); err != nil {
		t.Fatalf("ConvertBytesToDOCX DOCX input: %v", err)
	}
	converted, err := OpenBytes("converted.docx", out.Bytes())
	if err != nil {
		t.Fatalf("reopen converted DOCX: %v", err)
	}
	assertDocumentContainsText(t, converted, "already docx")
}

func TestConvertBytesToDOCXRejectsUnsupportedInput(t *testing.T) {
	var out bytes.Buffer
	err := ConvertBytesToDOCX("fixture.bin", []byte("not a Word document"), &out)
	if !errors.Is(err, ErrUnsupportedFormat) {
		t.Fatalf("ConvertBytesToDOCX error = %v, want ErrUnsupportedFormat", err)
	}
}

func TestOpenBytesRejectsUnsupportedContainer(t *testing.T) {
	_, err := OpenBytes("fixture.bin", []byte("not a Word document"))
	if !errors.Is(err, ErrUnsupportedFormat) {
		t.Fatalf("OpenBytes error = %v, want ErrUnsupportedFormat", err)
	}
}

func TestEditFacadeWrapsResolvedDocument(t *testing.T) {
	doc, err := OpenBytes("fixture.docx", makeDOCXFixture(t, "before"))
	if err != nil {
		t.Fatalf("OpenBytes: %v", err)
	}
	editor := Edit(doc)
	if editor == nil || editor.IR != doc {
		t.Fatal("Edit did not wrap the supplied IR document")
	}
}

func makeDOCXFixture(t *testing.T, text string) []byte {
	t.Helper()
	editor := New()
	p := editor.AddParagraph()
	p.AddRun().SetText(text)
	var buf bytes.Buffer
	if err := editor.SaveAsDOCX(&buf); err != nil {
		t.Fatalf("SaveAsDOCX: %v", err)
	}
	return buf.Bytes()
}

func makeDOCFixture(t *testing.T, text string) []byte {
	t.Helper()
	editor := New()
	p := editor.AddParagraph()
	p.AddRun().SetText(text)
	var buf bytes.Buffer
	if err := editor.SaveAsDOC(&buf); err != nil {
		t.Fatalf("SaveAsDOC: %v", err)
	}
	return buf.Bytes()
}

func assertDocumentContainsText(t *testing.T, doc *Document, want string) {
	t.Helper()
	if doc == nil {
		t.Fatal("document is nil")
	}
	for _, block := range doc.Blocks {
		if block.Paragraph == nil {
			continue
		}
		for _, run := range block.Paragraph.Runs {
			if strings.Contains(run.Text, want) {
				return
			}
		}
	}
	t.Fatalf("document does not contain %q", want)
}

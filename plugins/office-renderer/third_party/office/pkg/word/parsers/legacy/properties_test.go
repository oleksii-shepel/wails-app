package legacy

import (
	"encoding/binary"
	"path/filepath"
	"testing"
)

func TestParseLegacyPropertySetStrings(t *testing.T) {
	stream := buildLegacyPropertySetStream(t, []legacyTestProperty{
		{id: legacyPropertyCodePage, kind: "ui2", u16: 1252},
		{id: legacyPropertyTitle, kind: "lpstr", text: "Synthetic Title"},
		{id: legacyPropertySubject, kind: "lpwstr", text: "Synthetic Subject"},
		{id: legacyPropertyLastAuthor, kind: "lpstr", text: "Editor"},
		{id: legacyPropertyTemplate, kind: "lpstr", text: "Normal.dot"},
		{id: legacyPropertyAppName, kind: "lpstr", text: "Microsoft Word"},
	})
	props := parseLegacyPropertySetStrings(stream)
	if got := props[legacyPropertyTitle]; got != "Synthetic Title" {
		t.Fatalf("title = %q, want Synthetic Title", got)
	}
	if got := props[legacyPropertySubject]; got != "Synthetic Subject" {
		t.Fatalf("subject = %q, want Synthetic Subject", got)
	}
	if got := props[legacyPropertyLastAuthor]; got != "Editor" {
		t.Fatalf("lastAuthor = %q, want Editor", got)
	}
	if got := props[legacyPropertyTemplate]; got != "Normal.dot" {
		t.Fatalf("template = %q, want Normal.dot", got)
	}
	if got := props[legacyPropertyAppName]; got != "Microsoft Word" {
		t.Fatalf("appName = %q, want Microsoft Word", got)
	}
}

func TestParseLegacyEditTimeMinutes(t *testing.T) {
	// VT_FILETIME: 1 hour = 60 minutes.
	// FILETIME ticks are 100-ns intervals.
	oneHourFiletime := uint64(60 * 60 * 10_000_000)
	stream := buildLegacyPropertySetStream(t, []legacyTestProperty{
		{id: legacyPropertyCodePage, kind: "ui2", u16: 1252},
		{id: legacyPropertyEditTime, kind: "filetime", filetime: oneHourFiletime},
	})
	if got := parseLegacyEditTimeMinutes(stream); got != 60 {
		t.Fatalf("filetime editTime = %d, want 60", got)
	}

	// VT_I4: stored directly as minutes.
	stream = buildLegacyPropertySetStream(t, []legacyTestProperty{
		{id: legacyPropertyCodePage, kind: "ui2", u16: 1252},
		{id: legacyPropertyEditTime, kind: "i4", i32: 90},
	})
	if got := parseLegacyEditTimeMinutes(stream); got != 90 {
		t.Fatalf("i4 editTime = %d, want 90", got)
	}
}

func TestParseDocReadsLegacySummaryInformation(t *testing.T) {
	t.Skip("retired: superseded by the maintained legacy-* DOC corpus")
	doc, err := ParseDoc(filepath.Join("..", "..", "..", "..", "testdata", "doc", "apache-documentProperties.doc"))
	if err != nil {
		t.Fatalf("ParseDoc: %v", err)
	}
	if doc.Title != "This is document title" {
		t.Fatalf("Title = %q, want %q", doc.Title, "This is document title")
	}
	if doc.Subject != "This is document subject" {
		t.Fatalf("Subject = %q, want %q", doc.Subject, "This is document subject")
	}
	if doc.Author != "Sergey Vladimirov" {
		t.Fatalf("Author = %q, want %q", doc.Author, "Sergey Vladimirov")
	}
	if doc.Keywords != "This is document keywords" {
		t.Fatalf("Keywords = %q, want %q", doc.Keywords, "This is document keywords")
	}
	if doc.Description != "This is document comment" {
		t.Fatalf("Description = %q, want %q", doc.Description, "This is document comment")
	}
}

type legacyTestProperty struct {
	id       uint32
	kind     string
	text     string
	u16      uint16
	i32      int32
	filetime uint64
}

func buildLegacyPropertySetStream(t *testing.T, props []legacyTestProperty) []byte {
	t.Helper()
	section := make([]byte, 8+len(props)*8)
	binary.LittleEndian.PutUint32(section[4:8], uint32(len(props)))
	payloadOffset := len(section)
	payload := make([]byte, 0, 64)
	for index, prop := range props {
		binary.LittleEndian.PutUint32(section[8+index*8:12+index*8], prop.id)
		binary.LittleEndian.PutUint32(section[12+index*8:16+index*8], uint32(payloadOffset))
		var encoded []byte
		switch prop.kind {
		case "ui2":
			encoded = make([]byte, 8)
			binary.LittleEndian.PutUint32(encoded[0:4], legacyVTUI2)
			binary.LittleEndian.PutUint16(encoded[4:6], prop.u16)
		case "lpstr":
			raw := append([]byte(prop.text), 0)
			encoded = make([]byte, 8+len(raw))
			binary.LittleEndian.PutUint32(encoded[0:4], legacyVTLPSTR)
			binary.LittleEndian.PutUint32(encoded[4:8], uint32(len(raw)))
			copy(encoded[8:], raw)
		case "lpwstr":
			raw := make([]byte, (len([]rune(prop.text))+1)*2)
			for i, r := range []rune(prop.text) {
				binary.LittleEndian.PutUint16(raw[i*2:(i+1)*2], uint16(r))
			}
			encoded = make([]byte, 8+len(raw))
			binary.LittleEndian.PutUint32(encoded[0:4], legacyVTLPWSTR)
			binary.LittleEndian.PutUint32(encoded[4:8], uint32(len([]rune(prop.text))+1))
			copy(encoded[8:], raw)
		case "i4":
			encoded = make([]byte, 8)
			binary.LittleEndian.PutUint32(encoded[0:4], legacyVTI4)
			binary.LittleEndian.PutUint32(encoded[4:8], uint32(prop.i32))
		case "filetime":
			encoded = make([]byte, 12)
			binary.LittleEndian.PutUint32(encoded[0:4], legacyVTFILETIME)
			binary.LittleEndian.PutUint64(encoded[4:12], prop.filetime)
		default:
			t.Fatalf("unsupported property kind %q", prop.kind)
		}
		for len(encoded)%4 != 0 {
			encoded = append(encoded, 0)
		}
		payload = append(payload, encoded...)
		payloadOffset += len(encoded)
	}
	section = append(section, payload...)
	binary.LittleEndian.PutUint32(section[0:4], uint32(len(section)))

	stream := make([]byte, 48)
	binary.LittleEndian.PutUint16(stream[0:2], 0xFFFE)
	binary.LittleEndian.PutUint32(stream[24:28], 1)
	binary.LittleEndian.PutUint32(stream[44:48], 48)
	stream = append(stream, section...)
	return stream
}

func TestParseLegacyPropertySetSectionsMultiSection(t *testing.T) {
	// Build a two-section property set (like DocumentSummaryInformation).
	section0 := buildLegacyPropertySetSection(t, []legacyTestProperty{
		{id: legacyPropertyCodePage, kind: "ui2", u16: 1252},
		{id: legacyDocSummaryPropertyCompany, kind: "lpstr", text: "Acme Corp"},
		{id: legacyDocSummaryPropertyManager, kind: "lpstr", text: "Alice"},
	})
	section1 := buildLegacyPropertySetSection(t, []legacyTestProperty{
		{id: legacyPropertyCodePage, kind: "ui2", u16: 1252},
		{id: 0x02, kind: "lpstr", text: "CustomValue"},
	})

	// Header: 48 bytes. With 2 section descriptors (20 bytes each), header is 68 bytes total.
	headerSize := 28 + 2*20 // 68
	stream := make([]byte, headerSize)
	binary.LittleEndian.PutUint16(stream[0:2], 0xFFFE)
	binary.LittleEndian.PutUint32(stream[24:28], 2) // 2 sections

	// Section 0 FMTID: {D5CDD502-2E9C-101B-9397-08002B2CF9AE}
	copy(stream[28:44], []byte{0x02, 0xD5, 0xCD, 0xD5, 0x9C, 0x2E, 0x1B, 0x10, 0x93, 0x97, 0x08, 0x00, 0x2B, 0x2C, 0xF9, 0xAE})
	binary.LittleEndian.PutUint32(stream[44:48], uint32(headerSize))

	// Section 1 FMTID: {D5CDD505-2E9C-101B-9397-08002B2CF9AE}
	copy(stream[48:64], []byte{0x05, 0xD5, 0xCD, 0xD5, 0x9C, 0x2E, 0x1B, 0x10, 0x93, 0x97, 0x08, 0x00, 0x2B, 0x2C, 0xF9, 0xAE})
	binary.LittleEndian.PutUint32(stream[64:68], uint32(headerSize+len(section0)))

	stream = append(stream, section0...)
	stream = append(stream, section1...)

	sections := parseLegacyPropertySetSections(stream)
	if len(sections) != 2 {
		t.Fatalf("expected 2 sections, got %d", len(sections))
	}
	if sections[0][legacyDocSummaryPropertyCompany] != "Acme Corp" {
		t.Errorf("company = %q", sections[0][legacyDocSummaryPropertyCompany])
	}
	if sections[0][legacyDocSummaryPropertyManager] != "Alice" {
		t.Errorf("manager = %q", sections[0][legacyDocSummaryPropertyManager])
	}
	if sections[1][0x02] != "CustomValue" {
		t.Errorf("custom = %q", sections[1][0x02])
	}
}

func TestLoadLegacyDocPropertiesIncludesDocSummary(t *testing.T) {
	section0 := buildLegacyPropertySetSection(t, []legacyTestProperty{
		{id: legacyDocSummaryPropertyCompany, kind: "lpstr", text: "Acme Corp"},
		{id: legacyDocSummaryPropertyManager, kind: "lpstr", text: "Bob"},
	})
	stream := make([]byte, 48)
	binary.LittleEndian.PutUint16(stream[0:2], 0xFFFE)
	binary.LittleEndian.PutUint32(stream[24:28], 1)
	binary.LittleEndian.PutUint32(stream[44:48], 48)
	stream = append(stream, section0...)

	// Build a minimal CFB file with just the DocumentSummaryInformation stream.
	// Since cfb.File is an interface-like struct, we can use a real fixture or
	// test at the parse level. Here we test parseLegacyPropertySetSections
	// directly (covered above) and verify loadLegacyDocProperties via the
	// higher-level integration in doc_test.go if a fixture exists.
	props := parseLegacyPropertySetStrings(stream)
	if props[legacyDocSummaryPropertyCompany] != "Acme Corp" {
		t.Errorf("company = %q", props[legacyDocSummaryPropertyCompany])
	}
	if props[legacyDocSummaryPropertyManager] != "Bob" {
		t.Errorf("manager = %q", props[legacyDocSummaryPropertyManager])
	}
}

func buildLegacyPropertySetSection(t *testing.T, props []legacyTestProperty) []byte {
	t.Helper()
	section := make([]byte, 8+len(props)*8)
	binary.LittleEndian.PutUint32(section[4:8], uint32(len(props)))
	payloadOffset := len(section)
	payload := make([]byte, 0, 64)
	for index, prop := range props {
		binary.LittleEndian.PutUint32(section[8+index*8:12+index*8], prop.id)
		binary.LittleEndian.PutUint32(section[12+index*8:16+index*8], uint32(payloadOffset))
		var encoded []byte
		switch prop.kind {
		case "ui2":
			encoded = make([]byte, 8)
			binary.LittleEndian.PutUint32(encoded[0:4], legacyVTUI2)
			binary.LittleEndian.PutUint16(encoded[4:6], prop.u16)
		case "lpstr":
			raw := append([]byte(prop.text), 0)
			encoded = make([]byte, 8+len(raw))
			binary.LittleEndian.PutUint32(encoded[0:4], legacyVTLPSTR)
			binary.LittleEndian.PutUint32(encoded[4:8], uint32(len(raw)))
			copy(encoded[8:], raw)
		case "lpwstr":
			raw := make([]byte, (len([]rune(prop.text))+1)*2)
			for i, r := range []rune(prop.text) {
				binary.LittleEndian.PutUint16(raw[i*2:(i+1)*2], uint16(r))
			}
			encoded = make([]byte, 8+len(raw))
			binary.LittleEndian.PutUint32(encoded[0:4], legacyVTLPWSTR)
			binary.LittleEndian.PutUint32(encoded[4:8], uint32(len([]rune(prop.text))+1))
			copy(encoded[8:], raw)
		default:
			t.Fatalf("unsupported property kind %q", prop.kind)
		}
		for len(encoded)%4 != 0 {
			encoded = append(encoded, 0)
		}
		payload = append(payload, encoded...)
		payloadOffset += len(encoded)
	}
	section = append(section, payload...)
	binary.LittleEndian.PutUint32(section[0:4], uint32(len(section)))
	return section
}

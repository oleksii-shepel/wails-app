package legacy

import (
	"bytes"
	"encoding/binary"
	"testing"

	wordir "office/pkg/word/ir"
)

// buildXstz creates a counted Unicode string with null terminator (Xstz).
func buildXstz(s string) []byte {
	var b bytes.Buffer
	binary.Write(&b, binary.LittleEndian, uint16(len(s)))
	for _, r := range s {
		binary.Write(&b, binary.LittleEndian, uint16(r))
	}
	binary.Write(&b, binary.LittleEndian, uint16(0))
	return b.Bytes()
}

// buildSttb creates an extended-character string table (STTB).
func buildSttb(items []string) []byte {
	var b bytes.Buffer
	binary.Write(&b, binary.LittleEndian, uint16(0xFFFF)) // fExtend
	binary.Write(&b, binary.LittleEndian, uint16(len(items)))
	binary.Write(&b, binary.LittleEndian, uint16(0)) // cbExtra
	for _, item := range items {
		binary.Write(&b, binary.LittleEndian, uint16(len(item)))
		for _, r := range item {
			binary.Write(&b, binary.LittleEndian, uint16(r))
		}
	}
	return b.Bytes()
}

// buildNilPICF wraps binary payload in a NilPICFAndBinData structure.
func buildNilPICF(payload []byte) []byte {
	cbHeader := 0x44
	lcb := cbHeader + len(payload)
	b := make([]byte, lcb)
	binary.LittleEndian.PutUint32(b[0:4], uint32(lcb))
	binary.LittleEndian.PutUint16(b[4:6], uint16(cbHeader))
	copy(b[cbHeader:], payload)
	return b
}

// buildFFDataText creates a complete FFData payload for a text form field.
func buildFFDataText(name, defaultText string, maxLen int) []byte {
	var b bytes.Buffer
	// FFDataBase (10 bytes)
	binary.Write(&b, binary.LittleEndian, uint32(0xFFFFFFFF)) // version
	binary.Write(&b, binary.LittleEndian, uint16(0x0000))     // bits: iType=0 (text)
	binary.Write(&b, binary.LittleEndian, uint16(maxLen))     // cch
	binary.Write(&b, binary.LittleEndian, uint16(0))          // hps
	// xstzName
	b.Write(buildXstz(name))
	// xstzTextDef
	b.Write(buildXstz(defaultText))
	// wDef (not present for text)
	// xstzTextFormat
	b.Write(buildXstz(""))
	// xstzHelpText
	b.Write(buildXstz(""))
	// xstzStatText
	b.Write(buildXstz(""))
	// xstzEntryMcr
	b.Write(buildXstz(""))
	// xstzExitMcr
	b.Write(buildXstz(""))
	return b.Bytes()
}

// buildFFDataCheckbox creates a complete FFData payload for a checkbox form field.
func buildFFDataCheckbox(name string, checked bool) []byte {
	var b bytes.Buffer
	// FFDataBase (10 bytes)
	binary.Write(&b, binary.LittleEndian, uint32(0xFFFFFFFF)) // version
	binary.Write(&b, binary.LittleEndian, uint16(0x0001))     // bits: iType=1 (checkbox)
	binary.Write(&b, binary.LittleEndian, uint16(0))          // cch
	binary.Write(&b, binary.LittleEndian, uint16(0))          // hps
	// xstzName
	b.Write(buildXstz(name))
	// xstzTextDef (not present for checkbox)
	// wDef
	wDef := uint16(0)
	if checked {
		wDef = 1
	}
	binary.Write(&b, binary.LittleEndian, wDef)
	// xstzTextFormat
	b.Write(buildXstz(""))
	// xstzHelpText
	b.Write(buildXstz(""))
	// xstzStatText
	b.Write(buildXstz(""))
	// xstzEntryMcr
	b.Write(buildXstz(""))
	// xstzExitMcr
	b.Write(buildXstz(""))
	return b.Bytes()
}

// buildFFDataDropdown creates a complete FFData payload for a dropdown form field.
func buildFFDataDropdown(name string, defaultIdx int, items []string) []byte {
	var b bytes.Buffer
	// FFDataBase (10 bytes)
	binary.Write(&b, binary.LittleEndian, uint32(0xFFFFFFFF)) // version
	binary.Write(&b, binary.LittleEndian, uint16(0x0002))     // bits: iType=2 (dropdown)
	binary.Write(&b, binary.LittleEndian, uint16(0))          // cch
	binary.Write(&b, binary.LittleEndian, uint16(0))          // hps
	// xstzName
	b.Write(buildXstz(name))
	// xstzTextDef (not present for dropdown)
	// wDef
	binary.Write(&b, binary.LittleEndian, uint16(defaultIdx))
	// xstzTextFormat
	b.Write(buildXstz(""))
	// xstzHelpText
	b.Write(buildXstz(""))
	// xstzStatText
	b.Write(buildXstz(""))
	// xstzEntryMcr
	b.Write(buildXstz(""))
	// xstzExitMcr
	b.Write(buildXstz(""))
	// hsttbDropList
	b.Write(buildSttb(items))
	return b.Bytes()
}

func TestReadLegacyNilPICFAndBinData(t *testing.T) {
	payload := []byte{0x01, 0x02, 0x03}
	data := buildNilPICF(payload)
	got, ok := readLegacyNilPICFAndBinData(data, 0)
	if !ok {
		t.Fatal("expected ok")
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("payload = %x, want %x", got, payload)
	}

	// Invalid: cbHeader != 0x44
	bad := make([]byte, 10)
	binary.LittleEndian.PutUint32(bad[0:4], 100)
	binary.LittleEndian.PutUint16(bad[4:6], 0x40)
	_, ok = readLegacyNilPICFAndBinData(bad, 0)
	if ok {
		t.Fatal("expected not ok for bad cbHeader")
	}

	// Invalid: offset out of range
	_, ok = readLegacyNilPICFAndBinData(data, len(data)-1)
	if ok {
		t.Fatal("expected not ok for out-of-range offset")
	}
}

func TestReadLegacyXstz(t *testing.T) {
	data := buildXstz("Hello")
	s, n := readLegacyXstz(data, 0)
	if s != "Hello" {
		t.Fatalf("s = %q, want Hello", s)
	}
	if n != 2+5*2+2 {
		t.Fatalf("n = %d, want %d", n, 2+5*2+2)
	}

	// Empty string
	data = buildXstz("")
	s, n = readLegacyXstz(data, 0)
	if s != "" {
		t.Fatalf("s = %q, want empty", s)
	}
	if n != 4 {
		t.Fatalf("n = %d, want 4", n)
	}

	// Truncated
	_, n = readLegacyXstz([]byte{0x05, 0x00}, 0)
	if n != 0 {
		t.Fatal("expected 0 for truncated data")
	}
}

func TestReadLegacySttb(t *testing.T) {
	items := []string{"Apple", "Banana", "Cherry"}
	data := buildSttb(items)
	got, n := readLegacySttb(data, 0)
	if n == 0 {
		t.Fatal("expected non-zero size")
	}
	if len(got) != len(items) {
		t.Fatalf("len(got) = %d, want %d", len(got), len(items))
	}
	for i, want := range items {
		if got[i] != want {
			t.Fatalf("got[%d] = %q, want %q", i, got[i], want)
		}
	}

	// Empty STTB
	data = buildSttb(nil)
	got, n = readLegacySttb(data, 0)
	if len(got) != 0 {
		t.Fatalf("len(got) = %d, want 0", len(got))
	}

	// Non-extended (should fail gracefully)
	got, n = readLegacySttb([]byte{0x00, 0x00}, 0)
	if n != 0 {
		t.Fatal("expected 0 for non-extended STTB")
	}
}

func TestReadLegacyFFDataText(t *testing.T) {
	payload := buildFFDataText("FieldName", "DefaultValue", 50)
	dataStream := buildNilPICF(payload)
	ff := readLegacyFFData(dataStream, 0)
	if ff == nil {
		t.Fatal("expected non-nil FormField")
	}
	if ff.Type != "text" {
		t.Fatalf("Type = %q, want text", ff.Type)
	}
	if ff.Name != "FieldName" {
		t.Fatalf("Name = %q, want FieldName", ff.Name)
	}
	if ff.DefaultText != "DefaultValue" {
		t.Fatalf("DefaultText = %q, want DefaultValue", ff.DefaultText)
	}
	if ff.MaxLength != 50 {
		t.Fatalf("MaxLength = %d, want 50", ff.MaxLength)
	}
}

func TestReadLegacyFFDataCheckbox(t *testing.T) {
	payload := buildFFDataCheckbox("CheckField", true)
	dataStream := buildNilPICF(payload)
	ff := readLegacyFFData(dataStream, 0)
	if ff == nil {
		t.Fatal("expected non-nil FormField")
	}
	if ff.Type != "checkbox" {
		t.Fatalf("Type = %q, want checkbox", ff.Type)
	}
	if ff.Name != "CheckField" {
		t.Fatalf("Name = %q, want CheckField", ff.Name)
	}
	if !ff.Checked {
		t.Fatal("Checked = false, want true")
	}

	// Unchecked
	payload = buildFFDataCheckbox("CheckField", false)
	dataStream = buildNilPICF(payload)
	ff = readLegacyFFData(dataStream, 0)
	if ff.Checked {
		t.Fatal("Checked = true, want false")
	}
}

func TestReadLegacyFFDataDropdown(t *testing.T) {
	items := []string{"Red", "Green", "Blue"}
	payload := buildFFDataDropdown("ColorField", 1, items)
	dataStream := buildNilPICF(payload)
	ff := readLegacyFFData(dataStream, 0)
	if ff == nil {
		t.Fatal("expected non-nil FormField")
	}
	if ff.Type != "dropdown" {
		t.Fatalf("Type = %q, want dropdown", ff.Type)
	}
	if ff.Name != "ColorField" {
		t.Fatalf("Name = %q, want ColorField", ff.Name)
	}
	if ff.DefaultIdx != 1 {
		t.Fatalf("DefaultIdx = %d, want 1", ff.DefaultIdx)
	}
	if len(ff.Items) != 3 {
		t.Fatalf("Items = %v, want 3 items", ff.Items)
	}
	for i, want := range items {
		if ff.Items[i] != want {
			t.Fatalf("Items[%d] = %q, want %q", i, ff.Items[i], want)
		}
	}
}

func TestReadLegacyFFDataInvalid(t *testing.T) {
	// Too short
	if ff := readLegacyFFData([]byte{0x00}, 0); ff != nil {
		t.Fatal("expected nil for too-short data")
	}
	// Bad version
	bad := make([]byte, 14)
	binary.LittleEndian.PutUint32(bad[0:4], 0x12345678)
	if ff := readLegacyFFData(buildNilPICF(bad), 0); ff != nil {
		t.Fatal("expected nil for bad version")
	}
	// Bad cbHeader
	badHeader := make([]byte, 10)
	binary.LittleEndian.PutUint32(badHeader[0:4], 100)
	binary.LittleEndian.PutUint16(badHeader[4:6], 0x40)
	if _, ok := readLegacyNilPICFAndBinData(badHeader, 0); ok {
		t.Fatal("expected not ok for bad cbHeader")
	}
}

func TestApplyDocFieldsToRunsWithDataStreamTextFormField(t *testing.T) {
	payload := buildFFDataText("MyField", "BinaryDefault", 25)
	picOffset := 100
	dataStream := make([]byte, picOffset)
	copy(dataStream[picOffset:], buildNilPICF(payload))
	dataStream = append(dataStream, buildNilPICF(payload)...)

	runs := []docRun{
		{Text: "\x13", Format: wordir.RunFormat{PicLocation: picOffset}},
		{Text: " FORMTEXT ", Format: wordir.RunFormat{PicLocation: picOffset}},
		{Text: "\x14", Format: wordir.RunFormat{PicLocation: picOffset}},
		{Text: "CachedResult", Format: wordir.RunFormat{PicLocation: picOffset}},
		{Text: "\x15", Format: wordir.RunFormat{PicLocation: picOffset}},
	}
	result := applyDocFieldsToRunsWithDataStream(runs, dataStream)
	if len(result) != 1 {
		t.Fatalf("len(result) = %d, want 1", len(result))
	}
	ff := result[0].FormField
	if ff == nil {
		t.Fatal("FormField = nil")
	}
	if ff.Type != "text" {
		t.Fatalf("Type = %q, want text", ff.Type)
	}
	if ff.Name != "MyField" {
		t.Fatalf("Name = %q, want MyField", ff.Name)
	}
	if ff.DefaultText != "BinaryDefault" {
		t.Fatalf("DefaultText = %q, want BinaryDefault", ff.DefaultText)
	}
	if ff.MaxLength != 25 {
		t.Fatalf("MaxLength = %d, want 25", ff.MaxLength)
	}
}

func TestApplyDocFieldsToRunsWithDataStreamDropdownFormField(t *testing.T) {
	items := []string{"OptionA", "OptionB", "OptionC"}
	payload := buildFFDataDropdown("DropField", 2, items)
	picOffset := 100
	dataStream := make([]byte, picOffset)
	dataStream = append(dataStream, buildNilPICF(payload)...)

	runs := []docRun{
		{Text: "\x13", Format: wordir.RunFormat{PicLocation: picOffset}},
		{Text: " FORMDROPDOWN ", Format: wordir.RunFormat{PicLocation: picOffset}},
		{Text: "\x14", Format: wordir.RunFormat{PicLocation: picOffset}},
		{Text: "Cached", Format: wordir.RunFormat{PicLocation: picOffset}},
		{Text: "\x15", Format: wordir.RunFormat{PicLocation: picOffset}},
	}
	result := applyDocFieldsToRunsWithDataStream(runs, dataStream)
	if len(result) != 1 {
		t.Fatalf("len(result) = %d, want 1", len(result))
	}
	ff := result[0].FormField
	if ff == nil {
		t.Fatal("FormField = nil")
	}
	if ff.Type != "dropdown" {
		t.Fatalf("Type = %q, want dropdown", ff.Type)
	}
	if len(ff.Items) != 3 {
		t.Fatalf("Items = %v, want 3", ff.Items)
	}
	if ff.DefaultIdx != 2 {
		t.Fatalf("DefaultIdx = %d, want 2", ff.DefaultIdx)
	}
}

func TestApplyDocFieldsToRunsWithDataStreamCheckboxFormField(t *testing.T) {
	payload := buildFFDataCheckbox("BoxField", true)
	picOffset := 100
	dataStream := make([]byte, picOffset)
	dataStream = append(dataStream, buildNilPICF(payload)...)

	runs := []docRun{
		{Text: "\x13", Format: wordir.RunFormat{PicLocation: picOffset}},
		{Text: " FORMCHECKBOX ", Format: wordir.RunFormat{PicLocation: picOffset}},
		{Text: "\x14", Format: wordir.RunFormat{PicLocation: picOffset}},
		{Text: "0", Format: wordir.RunFormat{PicLocation: picOffset}},
		{Text: "\x15", Format: wordir.RunFormat{PicLocation: picOffset}},
	}
	result := applyDocFieldsToRunsWithDataStream(runs, dataStream)
	if len(result) != 1 {
		t.Fatalf("len(result) = %d, want 1", len(result))
	}
	ff := result[0].FormField
	if ff == nil {
		t.Fatal("FormField = nil")
	}
	if ff.Type != "checkbox" {
		t.Fatalf("Type = %q, want checkbox", ff.Type)
	}
	if !ff.Checked {
		t.Fatal("Checked = false, want true (binary overrides display text)")
	}
}

func TestApplyDocFieldsToRunsWithDataStreamFallbackWhenNoDataStream(t *testing.T) {
	runs := []docRun{
		{Text: "\x13"},
		{Text: " FORMTEXT "},
		{Text: "\x14"},
		{Text: "FallbackText"},
		{Text: "\x15"},
	}
	result := applyDocFieldsToRunsWithDataStream(runs, nil)
	if len(result) != 1 {
		t.Fatalf("len(result) = %d, want 1", len(result))
	}
	ff := result[0].FormField
	if ff == nil {
		t.Fatal("FormField = nil")
	}
	if ff.Type != "text" {
		t.Fatalf("Type = %q, want text", ff.Type)
	}
	if ff.DefaultText != "FallbackText" {
		t.Fatalf("DefaultText = %q, want FallbackText", ff.DefaultText)
	}
	if ff.Name != "" {
		t.Fatalf("Name = %q, want empty (no binary data)", ff.Name)
	}
}

func TestApplyDocFieldsToRunsWithDataStreamFallbackWhenPicLocationZero(t *testing.T) {
	payload := buildFFDataText("MyField", "BinaryDefault", 25)
	dataStream := buildNilPICF(payload)

	runs := []docRun{
		{Text: "\x13", Format: wordir.RunFormat{PicLocation: 0}}, // PicLocation = 0, so binary won't be read
		{Text: " FORMTEXT "},
		{Text: "\x14"},
		{Text: "FallbackText"},
		{Text: "\x15"},
	}
	result := applyDocFieldsToRunsWithDataStream(runs, dataStream)
	if len(result) != 1 {
		t.Fatalf("len(result) = %d, want 1", len(result))
	}
	ff := result[0].FormField
	if ff == nil {
		t.Fatal("FormField = nil")
	}
	if ff.DefaultText != "FallbackText" {
		t.Fatalf("DefaultText = %q, want FallbackText", ff.DefaultText)
	}
}
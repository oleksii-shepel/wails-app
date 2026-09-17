package cfb_test

import (
	"bytes"
	"testing"

	"office/pkg/ooxml/cfb"
)

// buildMinimalCFB constructs a minimal, well-formed CFB file in memory.
// It contains a single root storage and one stream named streamName
// containing the bytes of payload.  The payload is stored using the regular
// (non-mini) FAT so no mini-stream infrastructure is required; payloads
// smaller than 4096 bytes are padded to exactly 4096 bytes (one FAT chain
// spanning 8 × 512-byte sectors) so the miniStreamCutoff (0x1000) is not
// triggered.
func buildMinimalCFB(streamName string, payload []byte) []byte {
	const sectorSize = 512
	const dirEntrySize = 128
	const miniCutoff = 0x1000 // default cutoff

	// Ensure payload is at least miniCutoff bytes so it uses regular FAT.
	if len(payload) < miniCutoff {
		padded := make([]byte, miniCutoff)
		copy(padded, payload)
		payload = padded
	}

	// Number of data sectors needed.
	numDataSectors := (len(payload) + sectorSize - 1) / sectorSize
	// Total sectors: 1 (FAT) + 1 (directory) + numDataSectors.
	totalSectors := 2 + numDataSectors
	fileSize := 512 + totalSectors*sectorSize
	file := make([]byte, fileSize)

	// ── header ──────────────────────────────────────────────────────────────
	magic := []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}
	copy(file[0:], magic)
	// Minor version
	le16(file, 24, 0x003E)
	// Major version (3)
	le16(file, 26, 0x0003)
	// Byte order
	le16(file, 28, 0xFFFE)
	// Sector size power: 9 → 512
	le16(file, 30, 9)
	// Mini sector size power: 6 → 64
	le16(file, 32, 6)
	// reserved[34:40] = 0
	// Dir sector count (v3 must be 0)
	le32(file, 40, 0)
	// FAT sector count = 1
	le32(file, 44, 1)
	// Root dir first sector = 1
	le32(file, 48, 1)
	// Transaction signature = 0
	le32(file, 52, 0)
	// Mini stream cutoff (spec requires 0x1000).
	le32(file, 56, miniCutoff)
	// First mini FAT sector = end-of-chain (no mini FAT)
	le32(file, 60, 0xFFFFFFFE)
	// Mini FAT sector count = 0
	le32(file, 64, 0)
	// First DIFAT sector = end-of-chain (no DIFAT chain)
	le32(file, 68, 0xFFFFFFFE)
	// DIFAT sector count = 0
	le32(file, 72, 0)
	// DIFAT[0] = 0 (sector 0 is the FAT sector)
	le32(file, 76, 0)
	// DIFAT[1..108] = 0xFFFFFFFF (free)
	for i := 1; i < 109; i++ {
		le32(file, 76+i*4, 0xFFFFFFFF)
	}

	// ── sector 0: FAT ───────────────────────────────────────────────────────
	fatOff := 512 + 0*sectorSize
	// fat[0] = 0xFFFFFFFD (FAT sector)
	le32(file, fatOff+0*4, 0xFFFFFFFD)
	// fat[1] = 0xFFFFFFFE (end of directory chain)
	le32(file, fatOff+1*4, 0xFFFFFFFE)
	// Data sectors: fat[2] → fat[3] → … → fat[2+numDataSectors-1] → end.
	for i := 0; i < numDataSectors; i++ {
		sect := 2 + i
		if i < numDataSectors-1 {
			le32(file, fatOff+sect*4, uint32(sect+1)) // next sector
		} else {
			le32(file, fatOff+sect*4, 0xFFFFFFFE) // end of chain
		}
	}
	// Remaining FAT entries = free.
	for i := 2 + numDataSectors; i < sectorSize/4; i++ {
		le32(file, fatOff+i*4, 0xFFFFFFFF)
	}

	// ── sector 1: directory ──────────────────────────────────────────────────
	dirOff := 512 + 1*sectorSize

	// Entry 0: Root Entry
	writeEntry(file, dirOff+0*dirEntrySize, "Root Entry", 5 /*root*/, 0xFFFFFFFF, 0xFFFFFFFF, 1 /*child*/, 0xFFFFFFFE, 0)
	// Entry 1: the named stream
	writeEntry(file, dirOff+1*dirEntrySize, streamName, 2 /*stream*/, 0xFFFFFFFF, 0xFFFFFFFF, 0xFFFFFFFF, 2 /*startSect*/, uint32(len(payload)))
	// Entry 2, 3: empty (objType = 0) — already zeroed.

	// ── data sectors ─────────────────────────────────────────────────────────
	for i := 0; i < numDataSectors; i++ {
		dataOff := 512 + (2+i)*sectorSize
		start := i * sectorSize
		end := start + sectorSize
		if end > len(payload) {
			end = len(payload)
		}
		if start < len(payload) {
			copy(file[dataOff:], payload[start:end])
		}
	}

	return file
}

// writeEntry writes a 128-byte directory entry at offset off in file.
func writeEntry(file []byte, off int, name string, objType byte, left, right, child, startSect, size uint32) {
	// Encode name as UTF-16LE.
	runes := []rune(name)
	nameLen := len(runes)*2 + 2 // include null terminator
	for i, r := range runes {
		file[off+i*2] = byte(r)
		file[off+i*2+1] = 0
	}
	// Null terminator.
	file[off+len(runes)*2] = 0
	file[off+len(runes)*2+1] = 0
	// Name length in bytes.
	le16(file, off+64, uint16(nameLen))
	// Object type.
	file[off+66] = objType
	// Color flag (1 = black).
	file[off+67] = 1
	// Left, right, child.
	le32(file, off+68, left)
	le32(file, off+72, right)
	le32(file, off+76, child)
	// Start sector.
	le32(file, off+116, startSect)
	// Size (low 32 bits).
	le32(file, off+120, size)
	// Size high 32 bits (v3: must be 0).
	le32(file, off+124, 0)
}

func le16(b []byte, off int, v uint16) {
	b[off] = byte(v)
	b[off+1] = byte(v >> 8)
}

func le32(b []byte, off int, v uint32) {
	b[off] = byte(v)
	b[off+1] = byte(v >> 8)
	b[off+2] = byte(v >> 16)
	b[off+3] = byte(v >> 24)
}

// ─── tests ────────────────────────────────────────────────────────────────────

func TestOpenBytes_InvalidMagic(t *testing.T) {
	_, err := cfb.OpenBytes([]byte("not a cfb file"))
	if err == nil {
		t.Fatal("expected error for invalid magic, got nil")
	}
}

func TestOpenBytes_TooSmall(t *testing.T) {
	_, err := cfb.OpenBytes([]byte{0xD0, 0xCF})
	if err == nil {
		t.Fatal("expected error for too-small input, got nil")
	}
}

func TestOpenBytes_NilOrEmpty(t *testing.T) {
	_, err := cfb.OpenBytes(nil)
	if err == nil {
		t.Fatal("expected error for nil input, got nil")
	}
	_, err = cfb.OpenBytes([]byte{})
	if err == nil {
		t.Fatal("expected error for empty input, got nil")
	}
}

func TestStream_HappyPath(t *testing.T) {
	payload := []byte("Hello, CFB world!")
	raw := buildMinimalCFB("TestStream", payload)

	f, err := cfb.OpenBytes(raw)
	if err != nil {
		t.Fatalf("OpenBytes: %v", err)
	}

	data, ok := f.Stream("TestStream")
	if !ok {
		t.Fatal("Stream(\"TestStream\"): not found")
	}
	// buildMinimalCFB pads payload to ≥ 4096 bytes; the stream size field
	// stores the padded length, so the returned slice is that length.
	// Verify the original content appears at the start.
	if len(data) < len(payload) {
		t.Fatalf("returned data too short: got %d bytes, want at least %d", len(data), len(payload))
	}
	if string(data[:len(payload)]) != string(payload) {
		t.Fatalf("Stream content prefix mismatch: got %q, want %q", data[:len(payload)], payload)
	}
}

func TestStream_CaseInsensitive(t *testing.T) {
	payload := []byte("case test")
	raw := buildMinimalCFB("WordDocument", payload)

	f, err := cfb.OpenBytes(raw)
	if err != nil {
		t.Fatalf("OpenBytes: %v", err)
	}

	_, ok := f.Stream("worddocument") // lowercase lookup
	if !ok {
		t.Fatal("Stream lookup should be case-insensitive")
	}
	_, ok = f.Stream("WORDDOCUMENT") // uppercase lookup
	if !ok {
		t.Fatal("Stream lookup should be case-insensitive")
	}
}

func TestStream_Missing(t *testing.T) {
	raw := buildMinimalCFB("ExistingStream", []byte("data"))
	f, err := cfb.OpenBytes(raw)
	if err != nil {
		t.Fatalf("OpenBytes: %v", err)
	}
	_, ok := f.Stream("MissingStream")
	if ok {
		t.Fatal("expected false for missing stream, got true")
	}
}

func TestStreamNames(t *testing.T) {
	raw := buildMinimalCFB("MyStream", []byte("data"))
	f, err := cfb.OpenBytes(raw)
	if err != nil {
		t.Fatalf("OpenBytes: %v", err)
	}
	names := f.StreamNames()
	if len(names) != 1 || names[0] != "MyStream" {
		t.Fatalf("StreamNames() = %v, want [\"MyStream\"]", names)
	}
}

func TestNilFile(t *testing.T) {
	var f *cfb.File
	_, ok := f.Stream("anything")
	if ok {
		t.Fatal("nil File.Stream should return false")
	}
	if got := f.StreamNames(); got != nil {
		t.Fatalf("nil File.StreamNames should return nil, got %v", got)
	}
}

func TestOpenBytes_V3IgnoresReservedHighStreamSizeDWORD(t *testing.T) {
	const sectorSize = 512
	raw := buildMinimalCFB("PowerPoint Document", []byte("ppt-data"))
	dirOff := sectorSize + 1*sectorSize
	entryOff := dirOff + 1*128
	le32(raw, entryOff+124, 0xD2FCF2CB)

	f, err := cfb.OpenBytes(raw)
	if err != nil {
		t.Fatalf("OpenBytes: %v", err)
	}
	got, ok := f.Stream("PowerPoint Document")
	if !ok {
		t.Fatal("PowerPoint Document stream not found")
	}
	if len(got) != 4096 {
		t.Fatalf("stream len = %d, want 4096", len(got))
	}
	if string(got[:8]) != "ppt-data" {
		t.Fatalf("stream prefix = %q, want ppt-data", got[:8])
	}
}

func TestOpenReaderAt_V3IgnoresReservedHighStreamSizeDWORD(t *testing.T) {
	const sectorSize = 512
	raw := buildMinimalCFB("PowerPoint Document", []byte("ppt-data"))
	dirOff := sectorSize + 1*sectorSize
	entryOff := dirOff + 1*128
	le32(raw, entryOff+124, 0xD2FCF2CB)

	f, err := cfb.OpenReaderAt(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatalf("OpenReaderAt: %v", err)
	}
	got, ok := f.Stream("PowerPoint Document")
	if !ok {
		t.Fatal("PowerPoint Document stream not found")
	}
	if len(got) != 4096 {
		t.Fatalf("stream len = %d, want 4096", len(got))
	}
	if string(got[:8]) != "ppt-data" {
		t.Fatalf("stream prefix = %q, want ppt-data", got[:8])
	}
}

func TestStreamFallsBackToReadablePrefixForTruncatedRegularStream(t *testing.T) {
	raw := buildMinimalCFB("EncryptedPackage", []byte("package-data"))

	// Make the directory claim more data than the FAT chain can provide.
	const sectorSize = 512
	dirOff := sectorSize + sectorSize
	entryOff := dirOff + 128
	le32(raw, entryOff+120, 8872)
	le32(raw, entryOff+124, 0)

	f, err := cfb.OpenBytes(raw)
	if err != nil {
		t.Fatalf("OpenBytes: %v", err)
	}

	if _, ok, err := f.StreamWithError("EncryptedPackage"); !ok || err == nil {
		t.Fatalf("StreamWithError = ok=%v err=%v, want present stream with truncation error", ok, err)
	}

	got, ok := f.Stream("EncryptedPackage")
	if !ok {
		t.Fatal("Stream reported EncryptedPackage as missing")
	}
	if len(got) == 0 {
		t.Fatal("Stream returned no readable prefix for truncated EncryptedPackage")
	}
	if string(got[:12]) != "package-data" {
		t.Fatalf("Stream prefix = %q, want package-data", got[:12])
	}
}

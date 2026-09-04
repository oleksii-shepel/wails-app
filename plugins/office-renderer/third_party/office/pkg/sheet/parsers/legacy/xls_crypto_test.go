package legacy

import (
	"encoding/binary"
	"fmt"
	"path/filepath"
	"testing"

	"office/pkg/ooxml/crypt"
	sheetir "office/pkg/sheet/ir"
)

func TestCreateXorVerifier1(t *testing.T) {
	tests := []struct {
		password string
		want     int
	}{
		{"", 0x0000},
		{"abc", 0xCC1A},
	}
	for _, tc := range tests {
		got := crypt.CreateXorVerifier1(tc.password)
		if got != tc.want {
			t.Errorf("CreateXorVerifier1(%q) = 0x%04X, want 0x%04X", tc.password, got, tc.want)
		}
	}
}

func TestCreateXorKey1(t *testing.T) {
	tests := []struct {
		password string
		want     int
	}{
		{"", 0x0000},
		{"abc", 0x514A},
	}
	for _, tc := range tests {
		got := crypt.CreateXorKey1(tc.password)
		if got != tc.want {
			t.Errorf("CreateXorKey1(%q) = 0x%04X, want 0x%04X", tc.password, got, tc.want)
		}
	}
}

func TestVerifyXORPassword(t *testing.T) {
	// xor-encryption-abc.xls stores key=0x514A verifier=0xCC1A
	if !verifyXORPassword(0x514A, 0xCC1A, "abc") {
		t.Error("verifyXORPassword(0x514A, 0xCC1A, abc) = false, want true")
	}
	if verifyXORPassword(0x514A, 0xCC1A, "wrong") {
		t.Error("verifyXORPassword(0x514A, 0xCC1A, wrong) = true, want false")
	}
	if !verifyXORPassword(0, 0, "") {
		t.Error("verifyXORPassword(0, 0, empty) = false, want true")
	}
}

func TestDecryptXORStreamRoundTrip(t *testing.T) {
	// Build a tiny BIFF stream with one encrypted record payload.
	// Record: SID=0x0203 (Number), size=8, payload=8 bytes of plaintext.
	// After encryption the payload bytes are transformed.
	// We encrypt manually, then decrypt and verify the payload is restored.
	plainPayload := []byte{0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	data := make([]byte, 4+len(plainPayload))
	binary.LittleEndian.PutUint16(data[0:], 0x0203)
	binary.LittleEndian.PutUint16(data[2:], uint16(len(plainPayload)))
	copy(data[4:], plainPayload)

	xorArray := crypt.CreateXorArray1("abc")

	// Encrypt in place (reverse of decryptXORStream)
	// The reverse of: dec = rotateLeft(enc, 3) ^ xorArray[idx]
	// is: enc = rotateRight(dec ^ xorArray[idx], 3)
	for i := range plainPayload {
		streamOffset := 4 + i
		idx := (streamOffset + len(plainPayload)) % 16
		b := data[streamOffset]
		b ^= xorArray[idx]
		b = (b >> 3) | (b << 5) // rotate right by 3
		data[streamOffset] = b
	}

	// Now decrypt
	decryptXORStream(data, 0, xorArray)

	for i, want := range plainPayload {
		if data[4+i] != want {
			t.Fatalf("decrypt round-trip byte %d = 0x%02X, want 0x%02X", i, data[4+i], want)
		}
	}
}

func TestDecryptXORStreamLeavesNeverEncryptedRecordsAlone(t *testing.T) {
	// BOF payload should not be touched.
	bofPayload := []byte{0x08, 0x09, 0x10, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	data := make([]byte, 4+len(bofPayload))
	binary.LittleEndian.PutUint16(data[0:], biffBOF)
	binary.LittleEndian.PutUint16(data[2:], uint16(len(bofPayload)))
	copy(data[4:], bofPayload)

	xorArray := crypt.CreateXorArray1("abc")
	decryptXORStream(data, 0, xorArray)

	for i, want := range bofPayload {
		if data[4+i] != want {
			t.Fatalf("BOF payload byte %d was modified: got 0x%02X, want 0x%02X", i, data[4+i], want)
		}
	}
}

func TestMaybeDecryptBIFFStreamNoFilePass(t *testing.T) {
	// Simple unencrypted workbook globals: BOF + EOF
	data := make([]byte, 0, 16)
	data = append(data, biffRecord(biffBOF, make([]byte, 16))...)
	data = append(data, biffRecord(biffEOF, nil)...)

	orig := make([]byte, len(data))
	copy(orig, data)

	out, err := maybeDecryptBIFFStream(data)
	if err != nil {
		t.Fatalf("maybeDecryptBIFFStream: %v", err)
	}
	for i := range orig {
		if out[i] != orig[i] {
			t.Fatalf("unencrypted stream was modified at byte %d", i)
		}
	}
}

func TestMaybeDecryptBIFFStreamXorWrongPassword(t *testing.T) {
	data := make([]byte, 0, 32)
	data = append(data, biffRecord(biffBOF, make([]byte, 16))...)
	// FilePass XOR: encryptionType=0, key=0x514A, verifier=0xCC1A
	fpPayload := []byte{0x00, 0x00, 0x4A, 0x51, 0x1A, 0xCC}
	data = append(data, biffRecord(biffFilePass, fpPayload)...)

	oldPwd := XlsPassword
	XlsPassword = "wrong"
	defer func() { XlsPassword = oldPwd }()

	_, err := maybeDecryptBIFFStream(data)
	if err == nil {
		t.Fatal("expected error for wrong password, got nil")
	}
}

func TestParseXlsXorEncryptedFixture(t *testing.T) {
	oldPwd := XlsPassword
	XlsPassword = "abc"
	defer func() { XlsPassword = oldPwd }()

	wb, err := ParseXls(filepath.Join("..", "..", "..", "..", "testdata", "xls", "xor-encryption-abc.xls"))
	if err != nil {
		t.Fatalf("ParseXls: %v", err)
	}
	if len(wb.Sheets) != 1 {
		t.Fatalf("expected 1 sheet, got %d", len(wb.Sheets))
	}
	sheet := wb.Sheets[0]
	if len(sheet.Rows) < 3 {
		t.Fatalf("expected at least 3 rows, got %d", len(sheet.Rows))
	}
	for rowIdx, want := range []string{"1", "2", "3"} {
		if rowIdx >= len(sheet.Rows) {
			t.Fatalf("missing row at index %d", rowIdx)
		}
		row := sheet.Rows[rowIdx]
		if row == nil {
			t.Fatalf("nil row at index %d", rowIdx)
		}
		var found bool
		for _, cell := range row.Cells {
			if cell.Col == 1 {
				found = true
				got := fmt.Sprintf("%.0f", cell.NumValue)
				if got != want {
					t.Errorf("A%d value = %q, want %q", rowIdx+1, got, want)
				}
				break
			}
		}
		if !found {
			t.Fatalf("missing cell A%d", rowIdx+1)
		}
	}
}

func TestParseXlsXorEncryptedFixtureWithoutPassword(t *testing.T) {
	oldPwd := XlsPassword
	XlsPassword = ""
	defer func() { XlsPassword = oldPwd }()

	_, err := ParseXls(filepath.Join("..", "..", "..", "..", "testdata", "xls", "xor-encryption-abc.xls"))
	if err == nil {
		t.Fatal("expected error when opening encrypted file without password")
	}
}

func TestParseXlsBinaryRC4EncryptedFixture(t *testing.T) {
	oldPwd := XlsPassword
	XlsPassword = "password"
	defer func() { XlsPassword = oldPwd }()

	wb, err := ParseXls(filepath.Join("..", "..", "..", "..", "testdata", "xls", "password.xls"))
	if err != nil {
		t.Fatalf("ParseXls: %v", err)
	}
	if len(wb.Sheets) == 0 {
		t.Fatal("expected at least one sheet")
	}
	cell := findCellInSheetByRef(wb.Sheets[0], "D5")
	if cell == nil {
		t.Fatal("expected cell D5 in first sheet")
	}
	if cell.StrValue == "" {
		t.Fatal("expected decrypted Binary RC4 fixture text in D5")
	}
	const want = "A ZIP bomb is a variant of mail-bombing."
	if len(cell.StrValue) < len(want) || cell.StrValue[:len(want)] != want {
		t.Fatalf("D5 = %q, want prefix %q", cell.StrValue, want)
	}
}

func TestParseXlsCryptoAPIEncryptedFixture(t *testing.T) {
	oldPwd := XlsPassword
	XlsPassword = "freedom"
	defer func() { XlsPassword = oldPwd }()

	wb, err := ParseXls(filepath.Join("..", "..", "..", "..", "testdata", "xls", "35897-type4.xls"))
	if err != nil {
		t.Fatalf("ParseXls: %v", err)
	}
	if len(wb.Sheets) == 0 {
		t.Fatal("expected at least one sheet")
	}
	cell := findCellInSheetByRef(wb.Sheets[0], "A1")
	if cell == nil {
		t.Fatal("expected cell A1 in first sheet")
	}
	if cell.StrValue != "hello there!" {
		t.Fatalf("A1 = %q, want %q", cell.StrValue, "hello there!")
	}
}

func TestParseXlsBinaryRC4EncryptedFixtureWrongPassword(t *testing.T) {
	oldPwd := XlsPassword
	XlsPassword = "wrong"
	defer func() { XlsPassword = oldPwd }()

	_, err := ParseXls(filepath.Join("..", "..", "..", "..", "testdata", "xls", "password.xls"))
	if err == nil {
		t.Fatal("expected error when opening Binary RC4 encrypted file with wrong password")
	}
}

func TestParseXlsCryptoAPIEncryptedFixtureWrongPassword(t *testing.T) {
	oldPwd := XlsPassword
	XlsPassword = "wrong"
	defer func() { XlsPassword = oldPwd }()

	_, err := ParseXls(filepath.Join("..", "..", "..", "..", "testdata", "xls", "35897-type4.xls"))
	if err == nil {
		t.Fatal("expected error when opening CryptoAPI encrypted file with wrong password")
	}
}

func findCellInSheetByRef(sheet *sheetir.Sheet, ref string) *sheetir.Cell {
	if sheet == nil {
		return nil
	}
	for _, row := range sheet.Rows {
		if row == nil {
			continue
		}
		for _, cell := range row.Cells {
			if cell != nil && cell.Ref == ref {
				return cell
			}
		}
	}
	return nil
}

func biffRecord(sid uint16, payload []byte) []byte {
	b := make([]byte, 4+len(payload))
	binary.LittleEndian.PutUint16(b[0:], sid)
	binary.LittleEndian.PutUint16(b[2:], uint16(len(payload)))
	copy(b[4:], payload)
	return b
}

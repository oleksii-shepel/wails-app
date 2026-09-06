package doc

import (
	"encoding/binary"
	"testing"
)

func TestClassifyFIBRecognizesWord97(t *testing.T) {
	header := make([]byte, 18)
	binary.LittleEndian.PutUint16(header[0:2], fibSignature)
	binary.LittleEndian.PutUint16(header[2:4], fibWord97)

	got := classifyFIB(header)
	if !got.Valid || got.Version != fibWord97 || got.Encrypted {
		t.Fatalf("classifyFIB = %+v, want valid Word97 unencrypted", got)
	}
	if !got.word97OrNewer() {
		t.Fatalf("word97OrNewer = false, want true")
	}
}

func TestClassifyFIBPreservesEncryptionFlag(t *testing.T) {
	header := make([]byte, 18)
	binary.LittleEndian.PutUint16(header[0:2], fibSignature)
	binary.LittleEndian.PutUint16(header[2:4], fibWord97)
	binary.LittleEndian.PutUint16(header[10:12], flagEncrypted)

	got := classifyFIB(header)
	if !got.Valid || !got.Encrypted {
		t.Fatalf("classifyFIB = %+v, want encrypted valid FIB", got)
	}
}

func TestClassifyFIBRecognizesWord95(t *testing.T) {
	header := make([]byte, 18)
	binary.LittleEndian.PutUint16(header[0:2], fibSignature)
	binary.LittleEndian.PutUint16(header[2:4], fibWord95)

	got := classifyFIB(header)
	if !got.word95() || got.word97OrNewer() {
		t.Fatalf("classifyFIB = %+v, want Word95 only", got)
	}
}

func TestClassifyFIBRejectsInvalidHeader(t *testing.T) {
	if got := classifyFIB(make([]byte, 18)); got.Valid {
		t.Fatalf("classifyFIB = %+v, want invalid", got)
	}
	if got := classifyFIB([]byte{0xEC}); got.Valid {
		t.Fatalf("short classifyFIB = %+v, want invalid", got)
	}
}

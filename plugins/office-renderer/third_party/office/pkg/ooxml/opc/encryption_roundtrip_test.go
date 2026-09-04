package opc_test

import (
	"bytes"
	"testing"

	"office/pkg/ooxml/cfb"
	"office/pkg/ooxml/opc"
)

func TestRoundTripStandardEncryption(t *testing.T) {
	password := "testpassword123"
	plaintext := []byte("hello world zip content here")

	info, encrypted, err := opc.EncryptStandardPackage(password, plaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	decrypted, err := opc.DecryptEncryptedPackageWithPassword(info, encrypted, password)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Fatalf("decrypted %q != plaintext %q", decrypted, plaintext)
	}
}

func TestRoundTripAgileEncryption(t *testing.T) {
	password := "testpassword456"
	plaintext := []byte("hello world agile zip content here with more data to span segments maybe")

	info, encrypted, err := opc.EncryptAgilePackage(password, plaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	decrypted, err := opc.DecryptEncryptedPackageWithPassword(info, encrypted, password)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Fatalf("decrypted %q != plaintext %q", decrypted, plaintext)
	}
}

// TestWriteEncryptedAgilePackage_RoundTrip verifies the CFB-wrapped agile
// container parses and decrypts back to the original plaintext.
func TestWriteEncryptedAgilePackage_RoundTrip(t *testing.T) {
	password := "round-trip-agile-pw"
	plaintext := bytes.Repeat([]byte("agile-payload-x"), 600)

	container, err := opc.WriteEncryptedAgilePackage(password, plaintext)
	if err != nil {
		t.Fatalf("WriteEncryptedAgilePackage: %v", err)
	}

	f, err := cfb.OpenBytes(container)
	if err != nil {
		t.Fatalf("cfb.OpenBytes: %v", err)
	}
	info, ok := f.Stream("EncryptionInfo")
	if !ok {
		t.Fatal("EncryptionInfo stream missing from container")
	}
	encrypted, ok := f.Stream("EncryptedPackage")
	if !ok {
		t.Fatal("EncryptedPackage stream missing from container")
	}

	decrypted, err := opc.DecryptEncryptedPackageWithPassword(info, encrypted, password)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if !bytes.Equal(decrypted, plaintext) {
		t.Fatalf("decrypted plaintext mismatch: got %d bytes, want %d", len(decrypted), len(plaintext))
	}
}

// TestWriteEncryptedStandardPackage_ContainerShape verifies the CFB-wrapped
// standard container exposes both the EncryptionInfo and EncryptedPackage
// streams as direct children of the root storage. End-to-end decryption is
// covered by the standard-mode encryption tests; this test only exercises
// the CFB wrapping that WriteEncryptedStandardPackage adds on top.
func TestWriteEncryptedStandardPackage_ContainerShape(t *testing.T) {
	plaintext := []byte("standard-mode plaintext")
	container, err := opc.WriteEncryptedStandardPackage("pw", plaintext)
	if err != nil {
		t.Fatalf("WriteEncryptedStandardPackage: %v", err)
	}

	f, err := cfb.OpenBytes(container)
	if err != nil {
		t.Fatalf("cfb.OpenBytes: %v", err)
	}
	if _, ok := f.Stream("EncryptionInfo"); !ok {
		t.Fatal("EncryptionInfo stream missing from container")
	}
	enc, ok := f.Stream("EncryptedPackage")
	if !ok {
		t.Fatal("EncryptedPackage stream missing from container")
	}
	if len(enc) < 8 {
		t.Fatalf("EncryptedPackage too short (%d bytes); missing 8-byte plaintext length prefix", len(enc))
	}
}

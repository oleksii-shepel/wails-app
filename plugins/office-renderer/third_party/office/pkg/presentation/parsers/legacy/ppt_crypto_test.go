package legacy

import (
	"bytes"
	"crypto/md5"
	"crypto/rc4"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"office/pkg/ooxml/cfb"
	"office/pkg/ooxml/crypt"
	"office/pkg/presentation/ir"
)

func extractSlideText(slide *ir.Slide) string {
	var parts []string
	for _, shape := range slide.Shapes {
		for _, para := range shape.Paragraphs {
			for _, run := range para.Runs {
				parts = append(parts, run.Text)
			}
		}
	}
	return strings.Join(parts, "")
}

func TestParsePpt_EncryptedHello(t *testing.T) {
	PptPassword = "hello"
	defer func() { PptPassword = "" }()

	pres, err := ParsePpt("../../../../testdata/ppt/apache-Password_Protected-hello.ppt")
	if err != nil {
		t.Fatalf("ParsePpt failed: %v", err)
	}
	if pres == nil {
		t.Fatal("expected non-nil presentation")
	}
	if len(pres.Slides) == 0 {
		t.Fatal("expected at least one slide")
	}
	text := extractSlideText(pres.Slides[0])
	if !strings.Contains(text, "hello") {
		t.Fatalf("expected slide text to contain 'hello', got %q", text)
	}
}

func TestParsePpt_Encrypted56Hello(t *testing.T) {
	PptPassword = "hello"
	defer func() { PptPassword = "" }()

	pres, err := ParsePpt("../../../../testdata/ppt/apache-Password_Protected-56-hello.ppt")
	if err != nil {
		t.Fatalf("ParsePpt failed: %v", err)
	}
	if pres == nil {
		t.Fatal("expected non-nil presentation")
	}
	if len(pres.Slides) == 0 {
		t.Fatal("expected at least one slide")
	}
	text := extractSlideText(pres.Slides[0])
	if !strings.Contains(text, "hello") {
		t.Fatalf("expected slide text to contain 'hello', got %q", text)
	}
}

func TestParsePpt_EncryptedCryptoapiProc2356(t *testing.T) {
	PptPassword = "crypto"
	defer func() { PptPassword = "" }()

	pres, err := ParsePpt("../../../../testdata/ppt/apache-cryptoapi-proc2356.ppt")
	if err != nil {
		t.Fatalf("ParsePpt failed: %v", err)
	}
	if pres == nil {
		t.Fatal("expected non-nil presentation")
	}
	if len(pres.Slides) == 0 {
		t.Fatal("expected at least one slide")
	}
	text := extractSlideText(pres.Slides[0])
	if !strings.Contains(text, "Dominic") {
		t.Fatalf("expected slide text to contain 'Dominic', got %q", text)
	}
}

func TestParsePpt_EncryptedNoPassword(t *testing.T) {
	PptPassword = ""
	_, err := ParsePpt("../../../../testdata/ppt/apache-Password_Protected-hello.ppt")
	if err == nil {
		t.Fatal("expected error when no password is provided")
	}
	if !strings.Contains(err.Error(), "no password was provided") {
		t.Fatalf("expected 'no password' error, got: %v", err)
	}
}

func TestParsePpt_EncryptedWrongPassword(t *testing.T) {
	PptPassword = "wrong"
	defer func() { PptPassword = "" }()

	_, err := ParsePpt("../../../../testdata/ppt/apache-Password_Protected-hello.ppt")
	if err == nil {
		t.Fatal("expected error for wrong password")
	}
	if !strings.Contains(err.Error(), "invalid password") {
		t.Fatalf("expected 'invalid password' error, got: %v", err)
	}
}

func TestParsePpt_PP40SyntheticXOR(t *testing.T) {
	path := writeSyntheticPP40Fixture(t, "secret", []string{
		"Encrypted PP40 Title",
		"Question 1",
		"Answer 1",
	})
	PptPassword = "secret"
	defer func() { PptPassword = "" }()

	pres, err := ParsePpt(path)
	if err != nil {
		t.Fatalf("ParsePpt failed: %v", err)
	}
	if !legacyPresentationContainsText(pres, "Encrypted PP40 Title") {
		t.Fatal("presentation text missing decrypted PP40 title")
	}
	if !legacyPresentationContainsText(pres, "Answer 1") {
		t.Fatal("presentation text missing decrypted PP40 body")
	}
}

func TestParsePpt_PP40SyntheticXORNoPassword(t *testing.T) {
	path := writeSyntheticPP40Fixture(t, "secret", []string{"Encrypted PP40 Title"})
	PptPassword = ""

	_, err := ParsePpt(path)
	if err == nil {
		t.Fatal("expected error when no password is provided")
	}
	if !strings.Contains(err.Error(), "no password was provided") {
		t.Fatalf("expected 'no password' error, got: %v", err)
	}
}

func TestParsePpt_PP40SyntheticXORWrongPassword(t *testing.T) {
	path := writeSyntheticPP40Fixture(t, "secret", []string{"Encrypted PP40 Title"})
	PptPassword = "wrong"
	defer func() { PptPassword = "" }()

	_, err := ParsePpt(path)
	if err == nil {
		t.Fatal("expected error for wrong password")
	}
	if !strings.Contains(err.Error(), "invalid password") {
		t.Fatalf("expected 'invalid password' error, got: %v", err)
	}
}

func TestParsePptEncryptionInfo_BinaryRC4(t *testing.T) {
	password := "secret"
	salt := []byte("0123456789ABCDEF")
	verifier := []byte("FEDCBA9876543210")
	stream := buildPptAtomRecord(0, pptRTDocumentEncryptionAtom, buildBinaryRC4PptEncryptionPayload(password, salt, verifier))

	info, ok := parsePptEncryptionInfo(stream, 0)
	if !ok {
		t.Fatal("parsePptEncryptionInfo returned ok=false")
	}
	if info.mode != pptModeBinaryRC4 {
		t.Fatalf("mode = %v, want %v", info.mode, pptModeBinaryRC4)
	}
	if info.keyBits != 40 {
		t.Fatalf("keyBits = %d, want 40", info.keyBits)
	}
	if !bytes.Equal(info.salt, salt) {
		t.Fatalf("salt = %x, want %x", info.salt, salt)
	}

	PptPassword = password
	defer func() { PptPassword = "" }()

	keyMaterial, err := verifyPptPassword(info)
	if err != nil {
		t.Fatalf("verifyPptPassword: %v", err)
	}
	wantKey := crypt.DeriveBinaryRC4PasswordKey(password, salt)
	if !bytes.Equal(keyMaterial, wantKey) {
		t.Fatalf("keyMaterial = %x, want %x", keyMaterial, wantKey)
	}
}

func TestDecryptRecord_BinaryRC4(t *testing.T) {
	password := "secret"
	salt := []byte("0123456789ABCDEF")
	info := &pptEncryptionInfo{
		mode:                  pptModeBinaryRC4,
		hashAlg:               crypt.HashMD5,
		keyBits:               40,
		salt:                  append([]byte(nil), salt...),
		encryptedVerifier:     nil,
		encryptedVerifierHash: nil,
		verifierHashSize:      16,
	}
	keyMaterial := crypt.DeriveBinaryRC4PasswordKey(password, salt)
	persistID := 7
	plaintext := buildPptAtomRecord(0, pptRTTextBytesAtom, []byte("hello"))
	ciphertext := append([]byte(nil), plaintext...)
	blockKey := crypt.DeriveBinaryRC4BlockKey(keyMaterial, persistID)
	cipher, err := rc4.NewCipher(blockKey)
	if err != nil {
		t.Fatalf("rc4.NewCipher: %v", err)
	}
	cipher.XORKeyStream(ciphertext, ciphertext)

	decryptRecord(ciphertext, 0, persistID, keyMaterial, info)
	if !bytes.Equal(ciphertext, plaintext) {
		t.Fatalf("decryptRecord output = %x, want %x", ciphertext, plaintext)
	}
}

func buildBinaryRC4PptEncryptionPayload(password string, salt, verifier []byte) []byte {
	keyMaterial := crypt.DeriveBinaryRC4PasswordKey(password, salt)
	blockKey := crypt.DeriveBinaryRC4BlockKey(keyMaterial, 0)
	cipher, _ := rc4.NewCipher(blockKey)

	encryptedVerifier := make([]byte, len(verifier))
	cipher.XORKeyStream(encryptedVerifier, verifier)

	verifierHash := md5.Sum(verifier)
	encryptedVerifierHash := make([]byte, len(verifierHash))
	cipher.XORKeyStream(encryptedVerifierHash, verifierHash[:])

	payload := make([]byte, 52)
	binary.LittleEndian.PutUint16(payload[0:2], 1)
	binary.LittleEndian.PutUint16(payload[2:4], 1)
	copy(payload[4:20], salt)
	copy(payload[20:36], encryptedVerifier)
	copy(payload[36:52], encryptedVerifierHash)
	return payload
}

func writeSyntheticPP40Fixture(t *testing.T, password string, texts []string) string {
	t.Helper()
	var plain bytes.Buffer
	plain.Write([]byte{0xED, 0xDE, 0xAD, 0x0B})
	for _, text := range texts {
		plain.Write(buildSyntheticPP40TextRecord(text))
	}
	stream := buildSyntheticPre97PptXORStream(password, plain.Bytes())

	w := cfb.NewWriter()
	if err := w.AddStream("PP40", stream); err != nil {
		t.Fatalf("AddStream(PP40): %v", err)
	}
	raw, err := w.Bytes()
	if err != nil {
		t.Fatalf("Writer.Bytes: %v", err)
	}
	path := filepath.Join(t.TempDir(), "synthetic-pp40-xor.ppt")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("os.WriteFile: %v", err)
	}
	return path
}

func buildSyntheticPP40TextRecord(text string) []byte {
	payload := []byte(text)
	record := make([]byte, 6+len(payload))
	binary.LittleEndian.PutUint16(record[0:2], 1)
	binary.LittleEndian.PutUint32(record[2:6], uint32(len(payload)))
	copy(record[6:], payload)
	return record
}

func buildSyntheticPre97PptXORStream(password string, plaintext []byte) []byte {
	headerLen := len(syntheticPre97PptXORMagic) + 4
	out := make([]byte, headerLen+len(plaintext))
	copy(out, []byte(syntheticPre97PptXORMagic))
	binary.LittleEndian.PutUint16(out[len(syntheticPre97PptXORMagic):], uint16(crypt.CreateXorKey1(password)))
	binary.LittleEndian.PutUint16(out[len(syntheticPre97PptXORMagic)+2:], uint16(crypt.CreateXorVerifier1(password)))
	encryptSyntheticPre97PptXORPayload(out[headerLen:], plaintext, password)
	return out
}

func encryptSyntheticPre97PptXORPayload(dst, plaintext []byte, password string) {
	xorArray := crypt.CreateXorArray1(password)
	for i, b := range plaintext {
		b ^= xorArray[i%16]
		dst[i] = (b >> 3) | (b << 5)
	}
}

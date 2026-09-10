package legacy

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	wordir "office/pkg/word/ir"
)

func extractDocText(doc *wordir.Document) string {
	var parts []string
	for _, p := range doc.Paragraphs {
		parts = append(parts, p.Text)
	}
	return strings.Join(parts, "\n")
}

func legacyEncryptedFixturePassword(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "testdata", "passwords.json"))
	if err != nil {
		t.Fatalf("read encrypted fixture credentials: %v", err)
	}
	var passwords map[string]string
	if err := json.Unmarshal(data, &passwords); err != nil {
		t.Fatalf("parse encrypted fixture credentials: %v", err)
	}
	password, ok := passwords[filepath.ToSlash(filepath.Join("doc", name))]
	if !ok || password == "" {
		t.Fatalf("no password registered for doc/%s", name)
	}
	return password
}

func requireLegacyEncryptedFixture(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join("..", "..", "..", "..", "testdata", "doc", name)
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			t.Skipf("encrypted DOC fixture %s is not present", name)
		}
		t.Fatalf("stat encrypted DOC fixture: %v", err)
	}
	return path
}

func testEncryptedDocFixture(t *testing.T, name, expectedText string) {
	t.Helper()
	path := requireLegacyEncryptedFixture(t, name)
	password := legacyEncryptedFixturePassword(t, name)

	doc, err := ParseDocWithOptions(path, ParseOptions{Password: password})
	if err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	text := extractDocText(doc)
	if !strings.Contains(text, expectedText) {
		t.Fatalf("%s text = %q, want substring %q", name, text, expectedText)
	}
}

func TestDecryptDoc_BinaryRC4(t *testing.T) {
	testEncryptedDocFixture(t, "apache-password_tika_binaryrc4.doc", "This is an encrypted Word 2007 File.")
}

func TestDecryptDoc_CryptoAPI(t *testing.T) {
	testEncryptedDocFixture(t, "apache-password_password_cryptoapi.doc", "This is a test")
}

func TestDecryptDocGeneratedWord97Fixture(t *testing.T) {
	testEncryptedDocFixture(t, "encrypted-word97-password.doc", "Encrypted legacy DOC fixture")
}

func TestEncryptedDocRequiresExplicitPassword(t *testing.T) {
	const name = "encrypted-word97-password.doc"
	path := requireLegacyEncryptedFixture(t, name)

	_, err := ParseDocWithOptions(path, ParseOptions{})
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "password") {
		t.Fatalf("ParseDocWithOptions without password error = %v, want password error", err)
	}
}

func TestEncryptedDocRejectsWrongPasswordWithoutRawFallback(t *testing.T) {
	const name = "encrypted-word97-password.doc"
	path := requireLegacyEncryptedFixture(t, name)

	doc, err := ParseDocWithOptions(path, ParseOptions{Password: "definitely-wrong"})
	if err == nil {
		t.Fatalf("wrong password unexpectedly parsed document: %#v", doc)
	}
	lower := strings.ToLower(err.Error())
	if !strings.Contains(lower, "password") && !strings.Contains(lower, "verifier") {
		t.Fatalf("wrong password error = %v, want password/verifier error", err)
	}
}



func TestLegacyDocEncryptionErrorsAreAuthoritative(t *testing.T) {
	cases := []struct {
		name   string
		table  []byte
		flags  uint16
		lKey   uint32
		needle string
	}{
		{name: "truncated header", table: []byte{1, 0, 1}, flags: docFlagEncrypted, lKey: 3, needle: "header too short"},
		{name: "header exceeds stream", table: []byte{1, 0, 1, 0}, flags: docFlagEncrypted, lKey: 52, needle: "exceeds table stream"},
		{name: "unsupported version", table: []byte{9, 0, 9, 0}, flags: docFlagEncrypted, lKey: 4, needle: "unsupported encryption version"},
		{name: "truncated binary rc4", table: []byte{1, 0, 1, 0}, flags: docFlagEncrypted, lKey: 4, needle: "Binary RC4 encryption header too short"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseDocEncryptionInfo(nil, tc.table, tc.flags, tc.lKey)
			if err == nil {
				t.Fatal("parseDocEncryptionInfo error = nil")
			}
			if !isLegacyDocEncryptionError(err) {
				t.Fatalf("encryption error was not classified as authoritative: %v", err)
			}
			if !strings.Contains(err.Error(), tc.needle) {
				t.Fatalf("error = %q, want substring %q", err, tc.needle)
			}
		})
	}
}

func TestOrdinaryLegacyDocErrorsAreNotEncryptionErrors(t *testing.T) {
	err := fmt.Errorf("legacy doc: missing WordDocument stream")
	if isLegacyDocEncryptionError(err) {
		t.Fatalf("ordinary parser error was classified as encryption error: %v", err)
	}
}

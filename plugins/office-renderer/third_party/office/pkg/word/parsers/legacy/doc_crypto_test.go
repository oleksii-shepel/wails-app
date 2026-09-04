package legacy

import (
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

func TestDecryptDoc_BinaryRC4(t *testing.T) {
	t.Skip("retired: encrypted Apache fixture is no longer in the legacy-* corpus")
	DocPassword = "tika"
	defer func() { DocPassword = "" }()

	doc, err := ParseDoc("../../../../testdata/doc/apache-password_tika_binaryrc4.doc")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	text := extractDocText(doc)
	if !strings.Contains(text, "This is an encrypted Word 2007 File.") {
		t.Fatalf("expected encrypted text, got: %q", text)
	}
}

func TestDecryptDoc_CryptoAPI(t *testing.T) {
	t.Skip("retired: encrypted Apache fixture is no longer in the legacy-* corpus")
	DocPassword = "password"
	defer func() { DocPassword = "" }()

	doc, err := ParseDoc("../../../../testdata/doc/apache-password_password_cryptoapi.doc")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	text := extractDocText(doc)
	if !strings.Contains(text, "This is a test") {
		t.Fatalf("expected encrypted text, got: %q", text)
	}
}

package word

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

)

func TestOpenWithOptionsEncryptedDOCUsesPerCallPassword(t *testing.T) {
	path := wordFixturePath(t, "doc", "encrypted-word97-password.doc")
	password := wordFixturePassword(t, "doc/encrypted-word97-password.doc")
	doc, err := OpenWithOptions(path, OpenOptions{Password: password})
	if err != nil {
		t.Fatalf("OpenWithOptions encrypted DOC: %v", err)
	}
	if text := wordDocumentText(doc); !strings.Contains(text, "Encrypted legacy DOC fixture") {
		t.Fatalf("encrypted DOC text = %q", text)
	}
}

func TestOpenBytesWithOptionsEncryptedDOCUsesPerCallPassword(t *testing.T) {
	path := wordFixturePath(t, "doc", "encrypted-word97-password.doc")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	password := wordFixturePassword(t, "doc/encrypted-word97-password.doc")
	doc, err := OpenBytesWithOptions("encrypted-word97-password.doc", data, OpenOptions{Password: password})
	if err != nil {
		t.Fatalf("OpenBytesWithOptions encrypted DOC: %v", err)
	}
	if text := wordDocumentText(doc); !strings.Contains(text, "Encrypted legacy DOC fixture") {
		t.Fatalf("encrypted DOC text = %q", text)
	}
}

func TestOpenWithOptionsEncryptedDOCXUsesPerCallPassword(t *testing.T) {
	path := wordFixturePath(t, "docx", "bug53475-password-is-pass.docx")
	password := wordFixturePassword(t, "docx/bug53475-password-is-pass.docx")


	doc, err := OpenWithOptions(path, OpenOptions{Password: password})
	if err != nil {
		t.Fatalf("OpenWithOptions encrypted DOCX: %v", err)
	}
	if doc == nil {
		t.Fatal("OpenWithOptions encrypted DOCX returned nil document")
	}
}

func TestOpenBytesWithOptionsEncryptedDOCXUsesPerCallPassword(t *testing.T) {
	path := wordFixturePath(t, "docx", "bug53475-password-is-pass.docx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	password := wordFixturePassword(t, "docx/bug53475-password-is-pass.docx")


	doc, err := OpenBytesWithOptions("bug53475-password-is-pass.docx", data, OpenOptions{Password: password})
	if err != nil {
		t.Fatalf("OpenBytesWithOptions encrypted DOCX: %v", err)
	}
	if doc == nil {
		t.Fatal("OpenBytesWithOptions encrypted DOCX returned nil document")
	}
}

func wordFixturePassword(t *testing.T, key string) string {
	t.Helper()
	data, err := os.ReadFile(wordRepoPath(t, "testdata", "passwords.json"))
	if err != nil {
		t.Fatalf("read passwords.json: %v", err)
	}
	var passwords map[string]string
	if err := json.Unmarshal(data, &passwords); err != nil {
		t.Fatalf("parse passwords.json: %v", err)
	}
	password := passwords[key]
	if password == "" {
		t.Fatalf("no fixture password for %s", key)
	}
	return password
}

func wordFixturePath(t *testing.T, elems ...string) string {
	t.Helper()
	path := wordRepoPath(t, append([]string{"testdata"}, elems...)...)
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			t.Skipf("fixture not found: %s", path)
		}
		t.Fatal(err)
	}
	return path
}

func wordRepoPath(t *testing.T, elems ...string) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
	return filepath.Join(append([]string{root}, elems...)...)
}

func wordDocumentText(doc *Document) string {
	if doc == nil {
		return ""
	}
	var b strings.Builder
	for _, p := range doc.Paragraphs {
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(p.Text)
	}
	return b.String()
}

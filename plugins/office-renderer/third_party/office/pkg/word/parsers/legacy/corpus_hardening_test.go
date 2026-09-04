package legacy

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"office/pkg/word/render"
)

// TestLegacyDOCCorpusParseRender keeps real-world DOC files on the production
// path. Encrypted/password-protected files are capability cases rather than
// corruption failures; every other corpus entry must parse and render.
func TestLegacyDOCCorpusParseRender(t *testing.T) {
	root := legacyRepoPath(t, "testdata", "doc")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Skipf("legacy DOC corpus not available: %v", err)
	}
	checked := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".doc") {
			continue
		}
		checked++
		entry := entry
		t.Run(entry.Name(), func(t *testing.T) {
			doc, err := ParseDocLossless(filepath.Join(root, entry.Name()))
			if err != nil {
				lower := strings.ToLower(err.Error())
				if strings.Contains(lower, "encrypt") || strings.Contains(lower, "password") {
					t.Skip(err)
				}
				t.Fatalf("parse legacy DOC: %v", err)
			}
			if doc == nil {
				t.Fatal("parsed document is nil")
			}
			if _, err := render.HTML(doc); err != nil {
				t.Fatalf("render legacy DOC: %v", err)
			}
		})
	}
	if checked == 0 {
		t.Skip("no legacy DOC corpus fixtures found")
	}
}

func legacyRepoPath(t *testing.T, elems ...string) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", "..", ".."))
	return filepath.Join(append([]string{root}, elems...)...)
}

package render_test

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"office/pkg/ooxml/opc"
	word "office/pkg/word"
)

// TestDocxCorpusLoadResolveRender exercises the same streaming open/render path
// used by the public API and CLI. The save round trip is covered separately by
// the test below, which shares that reader.
func TestDocxCorpusLoadResolveRender(t *testing.T) {
	fixturesDir := repoPath(t, "testdata", "docx")
	entries, err := os.ReadDir(fixturesDir)
	if err != nil {
		t.Fatalf("read fixtures dir: %v", err)
	}

	var fixtureCount int
	var totalDiagnostics int
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".docx") {
			continue
		}
		// Word drops an owner lock file next to an open document. It is not a
		// ZIP package, so a fixture left open in the editor must not fail the run.
		if strings.HasPrefix(entry.Name(), "~$") {
			continue
		}
		fixtureCount++
		path := filepath.Join(fixturesDir, entry.Name())
		t.Run(entry.Name(), func(t *testing.T) {
			password := docxFixturePassword(t, entry.Name())
			doc, err := word.OpenWithOptions(path, word.OpenOptions{Password: password})
			if err != nil {
				if opc.IsEncryptedPackageError(err) && password == "" {
					t.Fatalf("encrypted DOCX fixture %s has no password in testdata/passwords.json", entry.Name())
				}
				t.Fatalf("open word document: %v", err)
			}
			if doc == nil {
				t.Fatal("resolved document is nil")
			}
			totalDiagnostics += len(doc.Diagnostics)

			html, err := word.HTML(doc)
			if err != nil {
				t.Fatalf("render HTML: %v", err)
			}
			if !strings.Contains(html, "<!DOCTYPE html>") || !strings.Contains(html, "<body>") {
				t.Fatalf("unexpected HTML envelope")
			}

			// Snapshot equality is intentionally owned by TestDocxHTMLSnapshots.
			// Keep this corpus gate focused on production-path open/resolve/render
			// health and diagnostics so one fidelity change does not report the
			// same baseline mismatch twice.
		})
	}

	if fixtureCount == 0 {
		t.Fatal("no docx fixtures found")
	}
	if totalDiagnostics != 0 {
		t.Fatalf("total corpus diagnostics = %d, want 0", totalDiagnostics)
	}
}

func repoPath(t *testing.T, elems ...string) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", ".."))
	pathParts := append([]string{root}, elems...)
	return filepath.Join(pathParts...)
}

func categorizeDocxDiag(code string) string {
	switch {
	case containsStr(code, "chart", "diagram"):
		return "chart"
	case containsStr(code, "drawing", "vml", "shape", "image"):
		return "drawing"
	case containsStr(code, "field", "formula"):
		return "field"
	case containsStr(code, "style", "font", "color"):
		return "style"
	case containsStr(code, "footnote", "endnote"):
		return "note"
	case containsStr(code, "section", "page"):
		return "pagination"
	default:
		return "other"
	}
}

func containsStr(s string, substrs ...string) bool {
	for _, substr := range substrs {
		if strings.Contains(s, substr) {
			return true
		}
	}
	return false
}

// TestDocxCorpusSaveReopenResolve exercises the supported save round trip: the
// sequential reader ingests the source, the editor writes a DOCX package, and
// the saved bytes are read and rendered again. Both legs use the streaming
// reader, so Strict packages (which only that reader understands) are covered
// alongside the transitional corpus.
func TestDocxCorpusSaveReopenResolve(t *testing.T) {
	fixturesDir := repoPath(t, "testdata", "docx")
	entries, err := os.ReadDir(fixturesDir)
	if err != nil {
		t.Skipf("DOCX corpus not available: %v", err)
	}
	var checked int
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".docx") {
			continue
		}
		// Skip Word's owner lock file; it is not a ZIP package.
		if strings.HasPrefix(entry.Name(), "~$") {
			continue
		}
		checked++
		entry := entry
		t.Run(entry.Name(), func(t *testing.T) {
			password := docxFixturePassword(t, entry.Name())
			source := filepath.Join(fixturesDir, entry.Name())
			doc, err := word.OpenWithOptions(source, word.OpenOptions{Password: password})
			if err != nil {
				if opc.IsEncryptedPackageError(err) && password == "" {
					t.Fatalf("encrypted DOCX fixture %s has no password in testdata/passwords.json", entry.Name())
				}
				t.Fatalf("open source: %v", err)
			}
			if doc == nil {
				t.Fatal("source document is nil")
			}
			var saved bytes.Buffer
			if err := word.Edit(doc).SaveAsDOCX(&saved); err != nil {
				t.Fatalf("save: %v", err)
			}
			reopened, err := word.OpenBytesWithOptions(entry.Name(), saved.Bytes(), word.OpenOptions{})
			if err != nil {
				t.Fatalf("reopen save: %v", err)
			}
			if reopened == nil {
				t.Fatal("reopened saved document is nil")
			}
			if _, err := word.HTML(reopened); err != nil {
				t.Fatalf("render saved document: %v", err)
			}
		})
	}
	if checked == 0 {
		t.Skip("no DOCX corpus fixtures found")
	}
}

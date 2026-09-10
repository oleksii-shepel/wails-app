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
	"office/pkg/word/parsers/ooxml/parts"
	"office/pkg/word/parsers/ooxml/resolve"
	"office/pkg/word/render"
)

// TestDocxCorpusLoadResolveRender exercises the same streaming open/render path
// used by the public API and CLI. Materialized OOXML remains covered separately
// by the save/reopen preservation test below.
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

// TestDocxCorpusSaveReopenResolve intentionally exercises the materialized
// OOXML package path because package preservation/editing is a different
// boundary from production rendering.
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
		checked++
		entry := entry
		t.Run(entry.Name(), func(t *testing.T) {
			password := docxFixturePassword(t, entry.Name())
			source, err := opc.OpenWithOptions(filepath.Join(fixturesDir, entry.Name()), opc.OpenOptions{Password: password})
			if err != nil {
				if opc.IsEncryptedPackageError(err) && password == "" {
					t.Fatalf("encrypted DOCX fixture %s has no password in testdata/passwords.json", entry.Name())
				}
				t.Fatal(err)
			}
			pkg, err := parts.LoadPackage(source)
			if err != nil {
				t.Fatalf("load source: %v", err)
			}
			var saved bytes.Buffer
			if err := parts.WritePackage(pkg, &saved); err != nil {
				t.Fatalf("save: %v", err)
			}
			reopened, err := opc.OpenBytes(saved.Bytes())
			if err != nil {
				t.Fatalf("reopen save: %v", err)
			}
			reloaded, err := parts.LoadPackage(reopened)
			if err != nil {
				t.Fatalf("reload save: %v", err)
			}
			doc, err := resolve.ResolvePackage(reloaded)
			if err != nil {
				t.Fatalf("resolve save: %v", err)
			}
			if doc == nil {
				t.Fatal("resolved saved document is nil")
			}
			if _, err := render.HTML(doc); err != nil {
				t.Fatalf("render saved document: %v", err)
			}
		})
	}
	if checked == 0 {
		t.Skip("no DOCX corpus fixtures found")
	}
}

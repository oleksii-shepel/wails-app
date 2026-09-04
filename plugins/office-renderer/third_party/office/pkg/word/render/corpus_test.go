package render_test

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"office/pkg/ooxml/opc"
	"office/pkg/word/parsers/ooxml/parts"
	"office/pkg/word/parsers/ooxml/resolve"
	"office/pkg/word/render"
)

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
			pkg, err := opc.Open(path)
			if err != nil {
				if opc.IsEncryptedPackageError(err) {
					t.Skip("encrypted package requires password")
				}
				t.Fatalf("open package: %v", err)
			}

			wordPkg, err := parts.LoadPackage(pkg)
			if err != nil {
				t.Fatalf("load word package: %v", err)
			}

			doc, err := resolve.ResolvePackage(wordPkg)
			if err != nil {
				t.Fatalf("resolve package: %v", err)
			}
			if doc == nil {
				t.Fatal("resolved document is nil")
			}
			if len(wordPkg.Diagnostics) > 0 && len(doc.Diagnostics) == 0 {
				t.Fatalf("expected resolved diagnostics to carry package diagnostics: %+v", wordPkg.Diagnostics)
			}

			// Accumulate diagnostics seen in the package for corpus-level assertion.
			totalDiagnostics += len(wordPkg.Diagnostics)

			html, err := render.HTML(doc)
			if err != nil {
				t.Fatalf("render HTML: %v", err)
			}
			if !strings.Contains(html, "<!DOCTYPE html>") || !strings.Contains(html, "<body>") {
				t.Fatalf("unexpected HTML envelope")
			}

			// If a snapshot exists for this fixture under testdata/docx-html/<basename>.html, compare.
			base := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
			snapPath := repoPath(t, "testdata", "docx-html", base+".html")
			if _, err := os.Stat(snapPath); err == nil {
				want, err := os.ReadFile(snapPath)
				if err != nil {
					t.Fatalf("read snapshot: %v", err)
				}
				got := strings.TrimSpace(strings.ReplaceAll(html, "\r\n", "\n"))
				wantStr := strings.TrimSpace(strings.ReplaceAll(string(want), "\r\n", "\n"))
				if got != wantStr {
					t.Fatalf("HTML snapshot mismatch for %s: output differs from %s", entry.Name(), snapPath)
				}
			}
		})
	}

	if fixtureCount == 0 {
		t.Fatal("no docx fixtures found")
	}

	// Corpus-level assertion: keep the DOCX fixture set diagnostics at 0.
	// Any new unsupported markup must either be handled or explicitly allowed here.
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
	parts := append([]string{root}, elems...)
	return filepath.Join(parts...)
}

// categorizeDocxDiag categorizes diagnostic codes by feature area for corpus analysis
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
			source, err := opc.Open(filepath.Join(fixturesDir, entry.Name()))
			if err != nil {
				if opc.IsEncryptedPackageError(err) {
					t.Skip("encrypted package requires password")
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

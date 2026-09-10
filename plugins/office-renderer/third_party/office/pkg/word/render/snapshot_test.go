package render_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	word "office/pkg/word"
)

func TestDocxHTMLSnapshots(t *testing.T) {
	fixturesDir := repoPath(t, "testdata", "docx")
	snapshotsDir := repoPath(t, "testdata", "docx-html")

	entries, err := os.ReadDir(fixturesDir)
	if err != nil {
		t.Fatalf("read DOCX fixtures: %v", err)
	}

	var checked int
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".docx") {
			continue
		}

		name := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		snapPath := filepath.Join(snapshotsDir, name+".html")
		if _, err := os.Stat(snapPath); err != nil {
			if os.IsNotExist(err) {
				t.Fatalf("DOCX corpus fixture %s has no canonical HTML snapshot %s", entry.Name(), snapPath)
			}
			t.Fatalf("stat snapshot %s: %v", snapPath, err)
		}

		checked++
		docxName := entry.Name()
		t.Run(name, func(t *testing.T) {
			got := strings.TrimSpace(renderFixtureHTML(t, docxName))

			if os.Getenv("UPDATE_WORD_SNAPSHOTS") == "1" {
				if err := os.WriteFile(snapPath, []byte(got+"\n"), 0o644); err != nil {
					t.Fatalf("write snapshot %s: %v", snapPath, err)
				}
				return
			}

			wantBytes, err := os.ReadFile(snapPath)
			if err != nil {
				t.Fatalf("read snapshot %s: %v", snapPath, err)
			}
			want := normalizeWordHTML(string(wantBytes))
			if got != want {
				t.Fatalf("snapshot mismatch for %s", name)
			}
		})
	}

	if checked == 0 {
		t.Fatal("no DOCX fixtures with HTML snapshots found")
	}
}

// renderFixtureHTML deliberately uses the public production path. Snapshot
// tests must exercise the same SAX/sequential parser and semantic IR used by the
// CLI rather than a second materialized OOXML resolver.
func renderFixtureHTML(t *testing.T, docxName string) string {
	t.Helper()
	docxPath := repoPath(t, "testdata", "docx", docxName)
	if _, err := os.Stat(docxPath); os.IsNotExist(err) {
		t.Skipf("fixture not found: %s", docxPath)
	}
	password := docxFixturePassword(t, docxName)
	doc, err := word.OpenWithOptions(docxPath, word.OpenOptions{Password: password})
	if err != nil {
		t.Fatalf("open word document: %v", err)
	}
	html, err := word.HTML(doc)
	if err != nil {
		t.Fatalf("render html: %v", err)
	}
	return html
}


func normalizeWordHTML(html string) string {
	return strings.TrimSpace(strings.ReplaceAll(html, "\r\n", "\n"))
}

package render_test

import (
	"os"
	"strings"
	"testing"

	"office/pkg/ooxml/opc"
	"office/pkg/word/parsers/ooxml/parts"
	"office/pkg/word/parsers/ooxml/resolve"
	"office/pkg/word/render"
)

func TestDocxHTMLSnapshots(t *testing.T) {
	cases := []string{
		"flux-file-manager-requirements",
		"protected_agile",
		"sample-files.com-basic-text",
		"sample-files.com-formatted-report",
		"sample-files.com-image-document",
		"sample-files.com-large-document",
		"sample-files.com-lists",
		"sample-files.com-multi-column",
		"sample-files.com-table-document",
		"sample-files.com-template",
		"sample-files.com-tracked-changes",
		"sample1",
		"sample2",
		"sample3",
		"sample4",
		"shepel-frontend-developer",
	}

	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			got := strings.TrimSpace(renderFixtureHTML(t, name+".docx"))
			snapPath := repoPath(t, "testdata", "docx-html", name+".html")
			wantBytes, err := os.ReadFile(snapPath)
			if err != nil {
				t.Fatalf("read snapshot %s: %v", snapPath, err)
			}
			want := strings.TrimSpace(strings.ReplaceAll(string(wantBytes), "\r\n", "\n"))
			if got != want {
				t.Fatalf("snapshot mismatch for %s", name)
			}
		})
	}
}

func renderFixtureHTML(t *testing.T, docxName string) string {
	t.Helper()

	docxPath := repoPath(t, "testdata", "docx", docxName)
	if _, err := os.Stat(docxPath); os.IsNotExist(err) {
		t.Skipf("fixture not found: %s", docxPath)
	}
	pkg, err := opc.Open(docxPath)
	if err != nil {
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
	html, err := render.HTML(doc)
	if err != nil {
		t.Fatalf("render html: %v", err)
	}
	return html
}

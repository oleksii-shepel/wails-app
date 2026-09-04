package render_test

import (
	"os"
	"strings"
	"testing"

	"office/pkg/ooxml/opc"
	"office/pkg/sheet/parsers/ooxml/parts"
	"office/pkg/sheet/parsers/ooxml/resolve"
	"office/pkg/sheet/render"
)

func TestXlsxHTMLSnapshots(t *testing.T) {
	cases := []string{
		"ExcelTables",
		"sample1",
		"sample-1",
		"sample2",
		"sample-2",
		"sample3",
		"sample-3",
		"sample-4",
		"sample-5",
		"tableStyle",
	}

	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			got := strings.TrimSpace(renderFixtureHTML(t, name+".xlsx"))
			snapPath := repoPath(t, "testdata", "xlsx-html", name+".html")
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

func renderFixtureHTML(t *testing.T, xlsxName string) string {
	t.Helper()

	xlsxPath := repoPath(t, "testdata", "xlsx", xlsxName)
	if _, err := os.Stat(xlsxPath); os.IsNotExist(err) {
		t.Skipf("fixture not found: %s", xlsxPath)
	}
	pkg, err := opc.Open(xlsxPath)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}
	sheetPkg, err := parts.LoadPackage(pkg)
	if err != nil {
		t.Fatalf("load sheet package: %v", err)
	}
	wb, err := resolve.ResolvePackage(sheetPkg)
	if err != nil {
		t.Fatalf("resolve package: %v", err)
	}
	html, err := render.HTML(wb)
	if err != nil {
		t.Fatalf("render html: %v", err)
	}
	return html
}

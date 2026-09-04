package render_test

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"office/pkg/ooxml/opc"
	"office/pkg/presentation/parsers/ooxml/parts"
	"office/pkg/presentation/parsers/ooxml/resolve"
	"office/pkg/presentation/render"
)

func TestPptxHTMLSnapshots(t *testing.T) {
	cases := []string{
		"sample-1",
		"sample-2",
		"sample-3",
		"sample-4",
		"sample-5",
		"sample-8",
		"sample-9",
	}

	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			got := strings.TrimSpace(renderPptxHTML(t, name+".pptx"))
			snapPath := repoPath(t, "testdata", "pptx-html", name+".html")
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

func TestPptxHTMLRunSpansKeepExplicitColor(t *testing.T) {
	cases := []string{
		"sample-1",
		"sample-2",
		"sample-3",
		"sample-4",
		"sample-5",
		"sample-8",
		"sample-9",
	}
	spanRE := regexp.MustCompile(`<span style="([^"]*)">`)
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			html := renderPptxHTML(t, name+".pptx")
			matches := spanRE.FindAllStringSubmatch(html, -1)
			for _, m := range matches {
				if !strings.Contains(m[1], "color:#") {
					t.Fatalf("span missing explicit color in %s: %q", name, m[0])
				}
			}
		})
	}
}

func renderPptxHTML(t *testing.T, pptxName string) string {
	t.Helper()
	return renderPptxHTMLWithOptions(t, pptxName, render.HTMLOptions{})
}

func renderPptxHTMLWithOptions(t *testing.T, pptxName string, opts render.HTMLOptions) string {
	t.Helper()
	pkg, err := opc.Open(repoPath(t, "testdata", "pptx", pptxName))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	pptPkg, err := parts.LoadPackage(pkg)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	pres, err := resolve.ResolvePackage(pptPkg)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	html, _, err := render.HTMLWithOptions(pres, opts)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	return html
}
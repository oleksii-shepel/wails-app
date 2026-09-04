package render_test

import (
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"office/pkg/ooxml/opc"
	"office/pkg/word/parsers/ooxml/parts"
	"office/pkg/word/parsers/ooxml/resolve"
	"office/pkg/word/render"
)

func benchRepoPath(elems ...string) string {
	_, filename, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", ".."))
	return filepath.Join(append([]string{root}, elems...)...)
}

func BenchmarkDocxPipeline(b *testing.B) {
	cases := []string{
		"sample-files.com-basic-text.docx",
		"sample-files.com-large-document.docx",
		"sample-files.com-table-document.docx",
	}
	for _, name := range cases {
		b.Run(name, func(b *testing.B) {
			path := benchRepoPath("testdata", "docx", name)
			b.ResetTimer()
			for range b.N {
				pkg, err := opc.Open(path)
				if err != nil {
					b.Fatal(err)
				}
				wordPkg, err := parts.LoadPackage(pkg)
				if err != nil {
					b.Fatal(err)
				}
				doc, err := resolve.ResolvePackage(wordPkg)
				if err != nil {
					b.Fatal(err)
				}
				if _, err := render.HTML(doc); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}


// TestDocxPipelinePerformanceGuardrails asserts that the full parse+resolve+render
// pipeline completes within documented time budgets on the standard fixture set.
// These are soft upper-bound checks rather than strict benchmarks; the goal is to
// catch catastrophic regressions in CI without requiring -bench flags.
func TestDocxPipelinePerformanceGuardrails(t *testing.T) {
	cases := []struct {
		name    string
		budgetS float64 // wall-clock budget in seconds per full pipeline run
	}{
		{"sample-files.com-basic-text.docx", 2.0},
		{"sample-files.com-large-document.docx", 5.0},
		{"sample-files.com-table-document.docx", 3.0},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			path := benchRepoPath("testdata", "docx", tc.name)
			start := time.Now()
			pkg, err := opc.Open(path)
			if err != nil {
				t.Skipf("fixture not available: %v", err)
			}
			wordPkg, err := parts.LoadPackage(pkg)
			if err != nil {
				t.Fatalf("load: %v", err)
			}
			doc, err := resolve.ResolvePackage(wordPkg)
			if err != nil {
				t.Fatalf("resolve: %v", err)
			}
			if _, err := render.HTML(doc); err != nil {
				t.Fatalf("render: %v", err)
			}
			elapsed := time.Since(start).Seconds()
			if elapsed > tc.budgetS {
				t.Errorf("pipeline took %.2fs, budget %.2fs — potential performance regression", elapsed, tc.budgetS)
			}
		})
	}
}
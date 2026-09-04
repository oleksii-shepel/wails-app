package render_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"office/pkg/ooxml/opc"
	"office/pkg/presentation/parsers/ooxml/parts"
	"office/pkg/presentation/render"
	"office/pkg/presentation/parsers/ooxml/resolve"
)

func benchRepoPath(elems ...string) string {
	_, filename, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", ".."))
	return filepath.Join(append([]string{root}, elems...)...)
}

func BenchmarkPptxPipeline(b *testing.B) {
	cases := []string{"sample-1.pptx", "sample-2.pptx", "sample-3.pptx"}
	for _, name := range cases {
		b.Run(name, func(b *testing.B) {
			path := benchRepoPath("testdata", "pptx", name)
			b.ResetTimer()
			for range b.N {
				pkg, err := opc.Open(path)
				if err != nil {
					b.Fatal(err)
				}
				pptPkg, err := parts.LoadPackage(pkg)
				if err != nil {
					b.Fatal(err)
				}
				pres, err := resolve.ResolvePackage(pptPkg)
				if err != nil {
					b.Fatal(err)
				}
				if _, err := render.HTML(pres); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
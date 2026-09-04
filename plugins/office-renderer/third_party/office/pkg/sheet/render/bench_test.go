package render_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"office/pkg/ooxml/opc"
	"office/pkg/sheet/parsers/ooxml/parts"
	"office/pkg/sheet/parsers/ooxml/resolve"
	"office/pkg/sheet/render"
)

func benchRepoPath(elems ...string) string {
	_, filename, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", ".."))
	return filepath.Join(append([]string{root}, elems...)...)
}

func BenchmarkXlsxPipeline(b *testing.B) {
	cases := []string{"sample1.xlsx", "sample2.xlsx", "sample3.xlsx"}
	for _, name := range cases {
		b.Run(name, func(b *testing.B) {
			path := benchRepoPath("testdata", "xlsx", name)
			b.ResetTimer()
			for range b.N {
				pkg, err := opc.Open(path)
				if err != nil {
					b.Fatal(err)
				}
				xlPkg, err := parts.LoadPackage(pkg)
				if err != nil {
					b.Fatal(err)
				}
				wb, err := resolve.ResolvePackage(xlPkg)
				if err != nil {
					b.Fatal(err)
				}
				if _, err := render.HTML(wb); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

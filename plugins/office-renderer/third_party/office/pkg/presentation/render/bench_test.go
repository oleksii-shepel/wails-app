package render_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"os"

	"office/pkg/presentation/edit"
	"office/pkg/presentation/render"
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
				data, err := os.ReadFile(path)
				if err != nil {
					b.Fatal(err)
				}
				edited, err := edit.LoadFromPPTXBytes(data)
				if err != nil {
					b.Fatal(err)
				}
				if _, err := render.HTML(edited.IR); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

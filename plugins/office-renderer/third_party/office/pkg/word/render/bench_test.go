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
	// Budgets are ratios against a calibration fixture measured in this same
	// process, not wall-clock seconds: the suite runs packages in parallel and
	// CI hardware varies, so absolute seconds would fail on load alone. A
	// genuine algorithmic regression slows the fixture relative to the
	// calibration and still trips the alarm.
	const calibrationFixture = "sample-files.com-basic-text.docx"
	cases := []struct {
		name        string
		ratioBudget float64 // multiple of the calibration pipeline's runtime
	}{
		{calibrationFixture, 8},
		{"sample-files.com-large-document.docx", 60},
		{"sample-files.com-table-document.docx", 8},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			before := pipelineSeconds(t, calibrationFixture)
			elapsed := pipelineSeconds(t, tc.name)
			after := pipelineSeconds(t, calibrationFixture)
			calibration := before
			if after > calibration {
				calibration = after
			}
			budget := calibration * tc.ratioBudget
			t.Logf("pipeline %s took %.3fs; calibration %.3fs; budget %.3fs (%.0fx)",
				tc.name, elapsed, calibration, budget, tc.ratioBudget)
			if elapsed > budget {
				t.Errorf("pipeline took %.2fs, budget %.2fs (%.0f x %.3fs calibration) — potential performance regression",
					elapsed, budget, tc.ratioBudget, calibration)
			}
		})
	}
}

// pipelineSeconds opens, resolves and renders one fixture, returning the
// wall-clock time. It skips the test when the fixture is unavailable.
func pipelineSeconds(t *testing.T, name string) float64 {
	t.Helper()
	path := benchRepoPath("testdata", "docx", name)
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
	return time.Since(start).Seconds()
}
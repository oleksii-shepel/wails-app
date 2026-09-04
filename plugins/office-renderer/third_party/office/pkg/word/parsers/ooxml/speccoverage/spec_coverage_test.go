package speccoverage

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestBuildDocxSpecCoverageReport(t *testing.T) {
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(here), "..", "..", "..", "..", ".."))
	report, err := BuildDocxSpecCoverageReport(repoRoot)
	if err != nil {
		t.Fatalf("BuildDocxSpecCoverageReport: %v", err)
	}
	if report.Summary.SpecElementCount <= 0 {
		t.Fatalf("SpecElementCount = %d, want > 0", report.Summary.SpecElementCount)
	}
	if report.Summary.BindingElementCount <= 0 {
		t.Fatalf("BindingElementCount = %d, want > 0", report.Summary.BindingElementCount)
	}
	if report.Summary.MatchedElementCount <= 0 {
		t.Fatalf("MatchedElementCount = %d, want > 0", report.Summary.MatchedElementCount)
	}
	if len(report.Packages) == 0 {
		t.Fatal("expected at least one package entry")
	}
	foundWordParts := false
	for _, pkg := range report.Packages {
		if pkg.Package == "word/parsers/ooxml/parts" {
			foundWordParts = true
			break
		}
	}
	if !foundWordParts {
		t.Fatal("missing word/parsers/ooxml/parts package entry")
	}
	foundWordMain := false
	for _, ns := range report.Namespaces {
		if ns.Namespace == nsWordMain {
			foundWordMain = true
			if ns.SpecElementCount <= 0 || ns.BindingElementCount <= 0 {
				t.Fatalf("word namespace stats = %+v, want positive counts", ns)
			}
			break
		}
	}
	if !foundWordMain {
		t.Fatal("missing WordprocessingML namespace entry")
	}
	if len(report.Structures) < 8 {
		t.Fatalf("structures = %d, want >= 8", len(report.Structures))
	}
}
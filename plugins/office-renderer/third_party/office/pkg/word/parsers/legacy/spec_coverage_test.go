package legacy

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestBuildDocSpecCoverageReport(t *testing.T) {
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(here), "..", "..", "..", ".."))
	report, err := BuildDocSpecCoverageReport(repoRoot)
	if err != nil {
		t.Fatalf("BuildDocSpecCoverageReport: %v", err)
	}
	if len(report.SprmGroups) != 3 {
		t.Fatalf("SPRMs groups = %d, want 3", len(report.SprmGroups))
	}
	if report.Summary.CanonicalSPRMs <= 0 {
		t.Fatalf("CanonicalSPRMs = %d, want > 0", report.Summary.CanonicalSPRMs)
	}
	if report.Summary.HandledSPRMs <= 0 {
		t.Fatalf("HandledSPRMs = %d, want > 0", report.Summary.HandledSPRMs)
	}
	if report.Summary.ParserCoveragePct <= 0 {
		t.Fatalf("ParserCoveragePct = %.2f, want > 0", report.Summary.ParserCoveragePct)
	}
	if len(report.Structures) < 8 {
		t.Fatalf("structures = %d, want >= 8", len(report.Structures))
	}
}
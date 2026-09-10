package legacy

import (
	"os"
	"strings"
	"testing"
)

func TestWord97FieldsMaterializeBeforeLegacyProjection(t *testing.T) {
	p, err := os.ReadFile("word97_projection.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(p)
	if strings.Contains(s, "applyPreWord97FieldsToRunsWithDataStream(") {
		t.Fatal("Word97 projection must not invoke legacy field materialization")
	}
	if !strings.Contains(s, "word97.EnrichMaterializedRuns(") {
		t.Fatal("Word97 projection must consume canonical enriched/materialized runs")
	}
	d, err := os.ReadFile("doc.go")
	if err != nil {
		t.Fatal(err)
	}
	ds := string(d)
	if strings.Contains(ds, "applyDocFieldsToRunsWithDataStream(") {
		t.Fatal("legacy field materializer must remain explicitly pre-97 named")
	}
	if !strings.Contains(ds, "data.Assembly.Runs") || !strings.Contains(ds, "applyPreWord97FieldsToRunsWithDataStream(") {
		t.Fatal("pre-97 compatibility branch must retain explicitly pre-97 field materialization")
	}
}

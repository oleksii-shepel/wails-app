package legacy

import (
	"testing"

	wordir "office/pkg/word/ir"
	word97 "office/pkg/word/parsers/doc/word97"
)

func TestLegacyListTablesFromCanonicalPreservesLevelAndOverride(t *testing.T) {
	start := 7
	restart := true
	restartOn := 0
	bold := true
	catalog := &word97.NumberingCatalog{
		Abstract: map[int][]word97.NumberingLevel{
			42: {{
				Level: 1, Start: 1, Format: "decimal", Text: "%1.%2.",
				IndentLeft: 720, IndentHanging: 360, RestartOnLevel: 0,
				RunFormat: &wordir.RunFormat{Bold: &bold},
			}},
		},
		Instances: map[int]word97.NumberingInstance{
			3: {
				NumID: 3, AbstractNumID: 42,
				Overrides: map[int]wordir.LegacyNumberingOverride{
					1: {Start: &start, Format: "upperRoman", Restart: &restart, RestartOnLevel: &restartOn},
				},
			},
		},
	}

	tables := legacyListTablesFromCanonical(catalog)
	if tables == nil {
		t.Fatal("tables = nil")
	}
	if got := tables.abstractNumIDFor(3); got != 42 {
		t.Fatalf("abstract num id = %d, want 42", got)
	}
	if got := tables.formatFor(3, 1); got != "upperRoman" {
		t.Fatalf("format = %q, want upperRoman", got)
	}
	if got := tables.startFor(3, 1); got != 7 {
		t.Fatalf("start = %d, want 7", got)
	}
	if got := tables.templateFor(3, 1); got != "%1.%2." {
		t.Fatalf("template = %q", got)
	}
	if got, ok := tables.indentLeftFor(3, 1); !ok || got != 720 {
		t.Fatalf("indent left = %d,%v, want 720,true", got, ok)
	}
	if got, ok := tables.indentFirstLineFor(3, 1); !ok || got != -360 {
		t.Fatalf("first line = %d,%v, want -360,true", got, ok)
	}
	if rf := tables.runFormatFor(3, 1); rf == nil || rf.Bold == nil || !*rf.Bold {
		t.Fatalf("run format = %#v, want bold", rf)
	}
	override := tables.overrideFor(3, 1)
	if override == nil || override.Start == nil || *override.Start != 7 || override.Format != "upperRoman" {
		t.Fatalf("override = %#v", override)
	}
}

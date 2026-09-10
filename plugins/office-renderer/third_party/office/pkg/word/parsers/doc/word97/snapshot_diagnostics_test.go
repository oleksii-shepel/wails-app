package word97

import (
	"testing"

	"office/pkg/diag"
	wordir "office/pkg/word/ir"
	docformat "office/pkg/word/parsers/doc/format"
)

func TestApplySnapshotDiagnosticsProjectsCanonicalFormattingAndSections(t *testing.T) {
	doc := &wordir.Document{
		Sections: []wordir.Section{{
			Diagnostics: diag.List{{Code: "section"}},
		}},
	}
	snapshot := &Snapshot{Core: &OpenCore{
		CHPX: []docformat.CHPXEntry{{
			Diagnostics: diag.List{{Code: "chpx"}},
		}},
		PAPX: []docformat.PAPXEntry{{
			Meta: docformat.PAPXMetadata{
				Diagnostics: diag.List{{Code: "papx"}},
			},
		}},
	}}
	ApplySnapshotDiagnostics(doc, snapshot)
	if len(doc.Diagnostics) != 3 {
		t.Fatalf("diagnostics = %d, want 3", len(doc.Diagnostics))
	}
	if doc.Diagnostics[0].Code != "chpx" ||
		doc.Diagnostics[1].Code != "papx" ||
		doc.Diagnostics[2].Code != "section" {
		t.Fatalf("diagnostic order = %#v", doc.Diagnostics)
	}
}

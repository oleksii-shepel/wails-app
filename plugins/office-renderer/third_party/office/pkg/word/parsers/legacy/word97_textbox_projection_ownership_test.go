package legacy

import (
	"os"
	"strings"
	"testing"
)

func TestWord97DetachedTextboxProjectionIsCanonical(t *testing.T) {
	data, err := os.ReadFile("word97_projection.go")
	if err != nil {
		t.Fatal(err)
	}
	projection := string(data)
	if !strings.Contains(projection, "word97.ApplySnapshotDocumentState(doc, snapshot)") {
		t.Fatal("Word97 projection must delegate snapshot-owned textbox projection to canonical word97")
	}
	for _, forbidden := range []string{
		"word97.ProjectDetachedDrawingTextBoxes(",
		"appendLegacyDetachedDrawingTextBoxes(",
		"appendPreWord97DetachedDrawingTextBoxes(",
	} {
		if strings.Contains(projection, forbidden) {
			t.Fatalf("Word97 projection retained direct/legacy textbox ownership %q", forbidden)
		}
	}
	if _, err := os.Stat("drawing_textboxes.go"); !os.IsNotExist(err) {
		if err == nil {
			t.Fatal("legacy detached textbox projection file must be removed")
		}
		t.Fatalf("stat drawing_textboxes.go: %v", err)
	}
}

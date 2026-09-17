package edit

import (
	"os"
	"strings"
	"testing"
)

// The compatibility membrane is allowed to enrich feature families that have
// not migrated yet, but it must not reacquire facts already owned by SAX.
func TestComposerMembraneCannotReacquireSAXOwnedCoreFacts(t *testing.T) {
	data, err := os.ReadFile("sax_shape_ownership.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	forbidden := []string{
		"dst.PlaceholderHasCustomPrompt = old.PlaceholderHasCustomPrompt",
		"dst.HyperlinkURL = old.HyperlinkURL",
		"dst.ActionLink = old.ActionLink",
		"dst.Bounds = old.Bounds",
		"dst.Fill = old.Fill",
		"dst.LineStyle = old.LineStyle",
		"dst.Geometry = old.Geometry",
		"dst.TextBodyProps = old.TextBodyProps",
	}
	for _, needle := range forbidden {
		if strings.Contains(src, needle) {
			t.Fatalf("composer membrane reacquired SAX-owned fact %q", needle)
		}
	}
}

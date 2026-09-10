package doc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSharedWordEventPackageOwnsIRMaterialization(t *testing.T) {
	data, err := os.ReadFile("materialize.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "wordevent.NewMaterializer(name)") {
		t.Fatal("DOC materialization must delegate to the shared word/event materializer")
	}
	for _, forbidden := range []string{
		"type materializeTableFrame struct",
		"func (m *materializer) Handle(",
		"func (m *materializer) finishParagraph(",
		"func runFromEvent(",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("DOC package retained event materializer ownership %q", forbidden)
		}
	}

	sharedPath := filepath.Join("..", "..", "event", "materialize.go")
	shared, err := os.ReadFile(sharedPath)
	if err != nil {
		t.Fatal(err)
	}
	sharedText := string(shared)
	for _, required := range []string{
		"type Materializer struct",
		"func NewMaterializer(",
		"func (m *Materializer) Handle(",
		"func (m *Materializer) Document(",
	} {
		if !strings.Contains(sharedText, required) {
			t.Fatalf("shared word/event materializer missing %q", required)
		}
	}
}

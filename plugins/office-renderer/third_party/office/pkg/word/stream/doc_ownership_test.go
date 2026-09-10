package stream

import (
	"os"
	"strings"
	"testing"
)

func TestDOCStreamDoesNotMislabelParserFailuresAsLegacy(t *testing.T) {
	data, err := os.ReadFile("doc.go")
	if err != nil {
		t.Fatalf("read doc.go: %v", err)
	}
	text := string(data)
	if strings.Contains(text, `EventType("legacy:" + unsupported.Feature)`) {
		t.Fatal("DOC stream bridge must not label canonical Word97/pre97 parser failures as legacy")
	}
	if !strings.Contains(text, `EventType("doc:" + unsupported.Feature)`) {
		t.Fatal("DOC stream bridge must preserve DOC parser ownership in unsupported-event labels")
	}
}

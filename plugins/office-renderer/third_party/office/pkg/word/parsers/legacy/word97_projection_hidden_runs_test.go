package legacy

import (
	"os"
	"strings"
	"testing"
)

func TestWord97ProjectionPreservesVanishRunsUntilParagraphReconciliation(t *testing.T) {
	data, err := os.ReadFile("word97_projection.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)
	if strings.Contains(src, "visible := runs[:0]") || strings.Contains(src, "word97BinaryDocumentInput(snapshot, visible)") {
		t.Fatal("Word97 projection must not delete Vanish runs before paragraph reconciliation")
	}
	if !strings.Contains(src, "word97BinaryDocumentInput(snapshot, runs)") {
		t.Fatal("Word97 projection must pass the complete canonical run stream to paragraph assembly")
	}
}

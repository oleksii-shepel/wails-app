package word97

import (
	"os"
	"strings"
	"testing"
)

func TestOpenPipelineConsumesCanonicalSnapshot(t *testing.T) {
	data, err := os.ReadFile("open_pipeline.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	if !strings.Contains(source, "OpenSnapshotReaderAt(") {
		t.Fatal("open pipeline must consume canonical snapshot")
	}
	if strings.Contains(source, "LoadDocumentContextReaderAt(") {
		t.Fatal("open pipeline must not orchestrate document context separately")
	}
}

func TestSnapshotOwnsCoreAndContextComposition(t *testing.T) {
	data, err := os.ReadFile("snapshot.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, want := range []string{
		"OpenCoreReaderAt(",
		"LoadDocumentContextReaderAt(",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("snapshot must own %q", want)
		}
	}
}

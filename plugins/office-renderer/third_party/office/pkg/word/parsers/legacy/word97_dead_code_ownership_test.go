package legacy

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLegacyDocDoesNotRegainWord97RunAssembly(t *testing.T) {
	data, err := os.ReadFile("doc.go")
	if err != nil {
		t.Fatalf("read doc.go: %v", err)
	}
	source := string(data)
	for _, forbidden := range []string{
		"func assembleDocRunsFromClxRange(", "func assembleDocRunsFromClx(",
		"func assembleDocRunsFromPlcPcdRange(", "func assembleDocRunsFromPlcPcdRangeWithCharset(",
		"func assembleDocRunsFromPlcPcd(", "func assembleDocRunsFromPlcPcdWithCharset(",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("legacy doc regained superseded Word97 run assembly %q", forbidden)
		}
	}
}

func TestRetiredWord97RevisionCompatibilityPassStaysRemoved(t *testing.T) {
	for _, name := range []string{"word97_revisions.go", "word97_revisions_test.go"} {
		if _, err := os.Stat(name); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("retired Word97 revision compatibility pass must stay removed: %s: %v", name, err)
		}
	}
}

func TestRetiredWord97RawRangeAdapterStaysRemoved(t *testing.T) {
	path := filepath.Join("..", "doc", "word97", "resolved_range.go")
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("raw CLX compatibility range adapter must stay removed: %v", err)
	}
}

func TestLegacyDOPBinaryAdapterStaysRemoved(t *testing.T) {
	if _, err := os.Stat("dop.go"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("dead legacy DOP adapter must stay removed: %v", err)
	}
}

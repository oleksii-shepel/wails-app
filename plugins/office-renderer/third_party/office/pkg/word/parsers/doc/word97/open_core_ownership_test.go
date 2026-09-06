package word97

import (
	"os"
	"strings"
	"testing"
)

func TestOpenCoreOwnsDocumentWideStoryDiscovery(t *testing.T) {
	data, err := os.ReadFile("open_readerat.go")
	if err != nil {
		t.Fatalf("read open_readerat.go: %v", err)
	}
	source := string(data)
	for _, want := range []string{
		"LoadSideStoryRanges(",
		"LoadHeaderFooterRanges(",
		"LoadSectionRecords(",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("open_readerat.go must own %q", want)
		}
	}
	if !strings.Contains(source, "sections := LoadSectionRecords(") {
		t.Fatal("open_readerat.go must discover sections once before header/footer ranges")
	}
	if !strings.Contains(source, "LoadHeaderFooterRanges(table, tableSize, fib, layout, metadata.StoryCounts, len(sections))") {
		t.Fatal("header/footer discovery must use the cached section count")
	}
	if !strings.Contains(source, "Sections:        sections") {
		t.Fatal("OpenCore must retain the same section catalog used by header/footer discovery")
	}
	if !strings.Contains(source, "LoadFontNamesReaderAt(") {
		t.Fatal("open_readerat.go must use canonical ordered font-name resolution")
	}
	if strings.Contains(source, "func loadOrderedFontNamesReaderAt(") {
		t.Fatal("open_readerat.go must not own a second FFN name scanner")
	}
}

func TestContextReusesOpenCoreStoryDiscovery(t *testing.T) {
	data, err := os.ReadFile("context_readerat.go")
	if err != nil {
		t.Fatalf("read context_readerat.go: %v", err)
	}
	source := string(data)
	for _, forbidden := range []string{
		"LoadSideStoryRanges(",
		"LoadHeaderFooterRanges(",
		"LoadSectionRecords(",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("context_readerat.go must reuse OpenCore instead of calling %s", forbidden)
		}
	}
	for _, want := range []string{
		"opened.SideStories",
		"opened.HeaderFooters",
		"opened.Sections",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("context_readerat.go must reuse %q", want)
		}
	}
}

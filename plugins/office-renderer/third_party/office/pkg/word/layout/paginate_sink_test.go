package layout

import (
	"testing"

	wordir "office/pkg/word/ir"
)

func TestPaginateToSinkStopsBeforeFullDocument(t *testing.T) {
	blocks := make([]wordir.Block, 1000)
	for i := range blocks {
		blocks[i] = wordir.Block{Paragraph: &wordir.Paragraph{Text: "x"}}
	}
	doc := &wordir.Document{Blocks: blocks}

	maxBlocksSeen := 0
	paginate := func(partial *wordir.Document) []Page {
		if len(partial.Blocks) > maxBlocksSeen {
			maxBlocksSeen = len(partial.Blocks)
		}
		pageCount := (len(partial.Blocks) + 63) / 64
		pages := make([]Page, pageCount)
		for i := range pages {
			pages[i].Number = i + 1
		}
		return pages
	}

	emitted := make([]int, 0, 2)
	paginateToSink(doc, PageSinkFunc(func(page Page) bool {
		emitted = append(emitted, page.Number)
		return page.Number < 2
	}), paginate)

	if len(emitted) != 2 || emitted[0] != 1 || emitted[1] != 2 {
		t.Fatalf("emitted pages = %v, want [1 2]", emitted)
	}
	if maxBlocksSeen >= len(blocks) {
		t.Fatalf("pagination consumed %d blocks, want early stop before %d", maxBlocksSeen, len(blocks))
	}
}

func TestPaginateToSinkDefersDocEndEndnotesOnTemporaryPrefixes(t *testing.T) {
	blocks := make([]wordir.Block, 400)
	for i := range blocks {
		blocks[i] = wordir.Block{Paragraph: &wordir.Paragraph{Text: "x"}}
	}
	doc := &wordir.Document{
		Blocks: blocks,
		Endnotes: map[int]wordir.Story{
			1: {},
		},
	}

	maxBlocksSeen := 0
	sawSuppressedEndnotes := false
	paginate := func(partial *wordir.Document) []Page {
		if len(partial.Blocks) > maxBlocksSeen {
			maxBlocksSeen = len(partial.Blocks)
		}
		if len(partial.Blocks) < len(blocks) && len(partial.Endnotes) == 0 {
			sawSuppressedEndnotes = true
		}
		pageCount := (len(partial.Blocks) + 63) / 64
		pages := make([]Page, pageCount)
		for i := range pages {
			pages[i].Number = i + 1
		}
		return pages
	}

	paginateToSink(doc, PageSinkFunc(func(page Page) bool {
		return page.Number < 2
	}), paginate)

	if !sawSuppressedEndnotes {
		t.Fatal("docEnd endnotes were not suppressed on a temporary prefix")
	}
	if maxBlocksSeen >= len(blocks) {
		t.Fatalf("docEnd pagination consumed %d blocks, want early stop before %d", maxBlocksSeen, len(blocks))
	}
}

func TestPaginateToSinkKeepsSectionEndEndnotesOnFullPath(t *testing.T) {
	blocks := make([]wordir.Block, 400)
	for i := range blocks {
		blocks[i] = wordir.Block{Paragraph: &wordir.Paragraph{Text: "x"}}
	}
	doc := &wordir.Document{
		Blocks:          blocks,
		EndnotePosition: "sectEnd",
		Endnotes: map[int]wordir.Story{
			1: {},
		},
	}

	calls := 0
	paginateToSink(doc, PageSinkFunc(func(Page) bool { return false }), func(partial *wordir.Document) []Page {
		calls++
		if len(partial.Blocks) != len(blocks) {
			t.Fatalf("sectEnd fallback saw %d blocks, want full %d", len(partial.Blocks), len(blocks))
		}
		if len(partial.Endnotes) != 1 {
			t.Fatalf("sectEnd fallback endnotes = %d, want 1", len(partial.Endnotes))
		}
		return []Page{{Number: 1}}
	})
	if calls != 1 {
		t.Fatalf("paginate calls = %d, want 1 full-document call", calls)
	}
}

func TestExtendPaginationPrefixThroughKeepNext(t *testing.T) {
	blocks := make([]wordir.Block, 5)
	for i := range blocks {
		blocks[i].Paragraph = &wordir.Paragraph{}
	}
	blocks[1].Paragraph.Format.KeepNext = true
	blocks[2].Paragraph.Format.KeepNext = true

	if got := extendPaginationPrefixThroughKeepNext(blocks, 2); got != 4 {
		t.Fatalf("prefix end = %d, want 4", got)
	}
	if got := extendPaginationPrefixThroughKeepNext(blocks, 5); got != 5 {
		t.Fatalf("final prefix end = %d, want 5", got)
	}
}

func TestPaginateToSinkNilDocumentDoesNothing(t *testing.T) {
	called := false
	paginateToSink(nil, PageSinkFunc(func(Page) bool {
		called = true
		return true
	}), func(*wordir.Document) []Page {
		t.Fatal("paginate should not be called for nil document")
		return nil
	})
	if called {
		t.Fatal("sink was called for nil document")
	}
}

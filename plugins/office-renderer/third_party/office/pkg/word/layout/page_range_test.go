package layout

import (
	"reflect"
	"testing"

	wordir "office/pkg/word/ir"
)

func testPagedDocument(paragraphCount int) *wordir.Document {
	blocks := make([]wordir.Block, 0, paragraphCount)
	for i := 0; i < paragraphCount; i++ {
		runs := []wordir.Run{{Kind: "run", Text: "Stable pagination content."}}
		// Force deterministic pagination rather than relying on font metrics or
		// default page geometry. Every twentieth paragraph ends with a hard page
		// break, so the fixture remains stable as layout measurements evolve.
		if (i+1)%20 == 0 && i+1 < paragraphCount {
			runs = append(runs, wordir.Run{Kind: "break", BreakType: "page"})
		}
		p := &wordir.Paragraph{
			Text: "Stable pagination content.",
			Runs: runs,
		}
		blocks = append(blocks, wordir.Block{Kind: "paragraph", Paragraph: p})
	}
	return &wordir.Document{Blocks: blocks}
}

func TestPaginateRangeMatchesFullPagination(t *testing.T) {
	doc := testPagedDocument(60)
	all := Paginate(doc)
	if len(all) < 2 {
		t.Fatalf("fixture produced %d pages, want at least 2", len(all))
	}

	got := PaginateRange(doc, 2, 2)
	if len(got) != 1 {
		t.Fatalf("PaginateRange returned %d pages, want 1", len(got))
	}
	if !reflect.DeepEqual(got[0], all[1]) {
		t.Fatal("PaginateRange page differs from full Paginate result")
	}
}

func TestPaginateRangeMatchesFullPaginationWithDocEndEndnote(t *testing.T) {
	doc := testPagedDocument(400)
	last := doc.Blocks[len(doc.Blocks)-1].Paragraph
	last.Runs = append(last.Runs, wordir.Run{Kind: "endnoteRef", NoteID: 1})
	doc.Endnotes = map[int]wordir.Story{
		1: {
			Blocks: []wordir.Block{{
				Kind: "paragraph",
				Paragraph: &wordir.Paragraph{
					Text: "Document end note.",
					Runs: []wordir.Run{{Kind: "run", Text: "Document end note."}},
				},
			}},
		},
	}

	all := Paginate(doc)
	if len(all) < 3 {
		t.Fatalf("fixture produced %d pages, want at least 3", len(all))
	}
	got := PaginateRange(doc, 2, 2)
	if len(got) != 1 {
		t.Fatalf("PaginateRange returned %d pages, want 1", len(got))
	}
	if !reflect.DeepEqual(got[0], all[1]) {
		t.Fatal("docEnd PaginateRange page differs from full Paginate result")
	}
}

func TestPaginateRangePreservesPhysicalPageNumbers(t *testing.T) {
	doc := testPagedDocument(60)
	all := Paginate(doc)
	if len(all) < 2 {
		t.Fatalf("fixture produced %d pages, want at least 2", len(all))
	}

	got := PaginateRange(doc, 2, 0)
	if len(got) != len(all)-1 {
		t.Fatalf("PaginateRange returned %d pages, want %d", len(got), len(all)-1)
	}
	if got[0].Number != 2 {
		t.Fatalf("first retained page number = %d, want 2", got[0].Number)
	}
}

func TestPaginatePageNormalizesAndReportsMissingPage(t *testing.T) {
	doc := testPagedDocument(20)
	all := Paginate(doc)
	if len(all) == 0 {
		t.Fatal("fixture produced no pages")
	}

	first, ok := PaginatePage(doc, 0)
	if !ok {
		t.Fatal("PaginatePage(0) unexpectedly reported missing page")
	}
	if !reflect.DeepEqual(first, all[0]) {
		t.Fatal("PaginatePage(0) did not normalize to the first page")
	}

	if _, ok := PaginatePage(doc, len(all)+1); ok {
		t.Fatal("PaginatePage beyond the document unexpectedly succeeded")
	}
}

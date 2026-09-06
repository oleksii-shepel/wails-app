package layout

import resolve "office/pkg/word/ir"

// PaginatePage returns one 1-based page from the document. A page number below
// one is normalized to the first page. When the requested page is beyond the
// document, the second return value is false.
func PaginatePage(doc *resolve.Document, pageNumber int) (Page, bool) {
	if pageNumber < 1 {
		pageNumber = 1
	}
	pages := PaginateRange(doc, pageNumber, pageNumber)
	if len(pages) == 0 {
		return Page{}, false
	}
	return pages[0], true
}

// PaginateRange returns a 1-based inclusive range of pages. start values below
// one are normalized to one. end <= 0 means unbounded. If a positive end is
// below start it is normalized to start.
//
// Returned Page.Number values remain the document's physical page numbers;
// they are not rebased to the beginning of the requested range.
func PaginateRange(doc *resolve.Document, start, end int) []Page {
	sink := NewPageWindowSink(start, end)
	PaginateToSink(doc, sink)
	return sink.Pages()
}

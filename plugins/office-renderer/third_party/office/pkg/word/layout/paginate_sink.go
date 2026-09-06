package layout

import (
	"strings"

	resolve "office/pkg/word/ir"
)

const incrementalPaginationBlockChunk = 128

// PaginateToSink emits finalized pages to sink in physical page order.
// Returning false from the sink stops pagination as soon as the requested page
// has become stable.
//
// The existing Paginate implementation still owns Word pagination semantics.
// This adapter feeds it progressively larger, safe block prefixes and withholds
// the mutable tail page. As soon as a following page exists, the preceding page
// can be emitted and discarded by a bounded sink. This gives page-oriented
// callers real early termination without maintaining a second pagination
// implementation.
//
// docEnd endnotes are suppressed on temporary prefixes and restored only for
// the final prefix. They cannot affect body-page layout before the document end,
// so finalized body pages remain safe to emit. sectEnd endnotes are different:
// they participate in intermediate section boundaries and therefore still use
// the full-document compatibility path until section-complete checkpoints are
// available.
func PaginateToSink(doc *resolve.Document, sink PageSink) {
	paginateToSink(doc, sink, Paginate)
}

func paginateToSink(doc *resolve.Document, sink PageSink, paginate func(*resolve.Document) []Page) {
	if sink == nil || paginate == nil || doc == nil {
		return
	}
	if len(doc.Blocks) == 0 || hasSectionEndEndnotes(doc) {
		emitPaginatedPages(paginate(doc), sink, 0)
		return
	}

	emitted := 0
	prefixEnd := 0
	for prefixEnd < len(doc.Blocks) {
		nextEnd := prefixEnd + incrementalPaginationBlockChunk
		if nextEnd > len(doc.Blocks) {
			nextEnd = len(doc.Blocks)
		}
		nextEnd = extendPaginationPrefixThroughKeepNext(doc.Blocks, nextEnd)
		if nextEnd <= prefixEnd {
			nextEnd = len(doc.Blocks)
		}

		partial := *doc
		partial.Blocks = doc.Blocks[:nextEnd]
		finalPrefix := nextEnd == len(doc.Blocks)
		if !finalPrefix && len(doc.Endnotes) > 0 {
			// docEnd notes belong after the complete body. Materializing them for a
			// temporary prefix would create artificial tail pages that are not yet
			// stable. References remain in the body IR; only note-story expansion is
			// deferred until EOF is known.
			partial.Endnotes = nil
		}
		pages := paginate(&partial)

		stableCount := len(pages)
		if !finalPrefix && stableCount > 0 {
			// The last page is still open to later body flow. Keep it as bounded
			// lookahead and emit it only after another page exists or EOF is known.
			stableCount--
		}
		for emitted < stableCount {
			if !sink.Emit(pages[emitted]) {
				return
			}
			emitted++
		}
		prefixEnd = nextEnd
	}
}

func hasSectionEndEndnotes(doc *resolve.Document) bool {
	return doc != nil && len(doc.Endnotes) > 0 && strings.EqualFold(doc.EndnotePosition, "sectEnd")
}

func emitPaginatedPages(pages []Page, sink PageSink, start int) int {
	if start < 0 {
		start = 0
	}
	for i := start; i < len(pages); i++ {
		if !sink.Emit(pages[i]) {
			return i + 1
		}
	}
	return len(pages)
}

// extendPaginationPrefixThroughKeepNext avoids ending a temporary document in
// the middle of a keep-with-next chain. Such a cut can make the temporary tail
// page move when the next block arrives and would invalidate the page before it.
func extendPaginationPrefixThroughKeepNext(blocks []resolve.Block, end int) int {
	if len(blocks) == 0 {
		return 0
	}
	if end < 1 {
		end = 1
	}
	if end > len(blocks) {
		return len(blocks)
	}
	for end < len(blocks) {
		block := &blocks[end-1]
		if block.Paragraph == nil || !block.Paragraph.Format.KeepNext {
			break
		}
		end++
	}
	return end
}

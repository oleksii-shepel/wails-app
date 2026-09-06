package word97

import (
	"io"

	docbinary "office/pkg/word/parsers/doc/binary"
	docformat "office/pkg/word/parsers/doc/format"
)

// MaterializeResolvedRunRangeFromCLX is the final compatibility-facing range
// adapter for callers that retain raw CLX plus canonical CHPX/PAPX entries. It
// resolves tracked-change attribution in the canonical Word97 layer before
// returning runs.
func MaterializeResolvedRunRangeFromCLX(
	word io.ReaderAt,
	wordSize int64,
	clx []byte,
	chpx []docformat.CHPXEntry,
	papx []docformat.PAPXEntry,
	authors []string,
	r StoryRange,
) []Run {
	complex := docbinary.ParseComplexFileTable(clx)
	if complex == nil || len(complex.PlcPcd) == 0 {
		return nil
	}
	return MaterializeRunRange(word, wordSize, &OpenCore{
		Complex:         complex,
		CHPX:            chpx,
		PAPX:            papx,
		RevisionAuthors: append([]string(nil), authors...),
	}, r)
}

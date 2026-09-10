package word97

import (
	"fmt"
	"io"
	"strings"
	"unicode/utf16"

	"golang.org/x/text/encoding/charmap"
	docbinary "office/pkg/word/parsers/doc/binary"
	docformat "office/pkg/word/parsers/doc/format"
)

// PieceReaderAtCursor walks a Word97 PieceTable in logical CP order and splits
// pieces at canonical CHPX/PAPX boundaries without materializing the document.
type PieceReaderAtCursor struct {
	wordReader       io.ReaderAt
	wordSize         int64
	table            *docbinary.PieceTable
	chpxEntries      []docformat.CHPXEntry
	papxEntries      []docformat.PAPXEntry
	preferredCharset *charmap.Charmap
	pieceIndex       int
	pieceRuns        []Run
	runIndex         int
}

func NewPieceReaderAtCursor(wordReader io.ReaderAt, wordSize int64, plc []byte, chpxEntries []docformat.CHPXEntry, papxEntries []docformat.PAPXEntry, preferredCharset *charmap.Charmap) (*PieceReaderAtCursor, error) {
	if wordReader == nil {
		return nil, fmt.Errorf("doc word97 stream: nil WordDocument ReaderAt")
	}
	table, err := docbinary.ParsePieceTable(plc)
	if err != nil {
		return nil, err
	}
	return &PieceReaderAtCursor{wordReader: wordReader, wordSize: wordSize, table: table, chpxEntries: chpxEntries, papxEntries: papxEntries, preferredCharset: preferredCharset}, nil
}

func (c *PieceReaderAtCursor) Next() (Run, bool, error) {
	if c == nil {
		return Run{}, false, nil
	}
	for {
		if c.runIndex < len(c.pieceRuns) {
			r := c.pieceRuns[c.runIndex]
			c.runIndex++
			return r, true, nil
		}
		if c.table == nil || c.pieceIndex >= c.table.Count() {
			return Run{}, false, nil
		}
		if err := c.loadNextPiece(); err != nil {
			return Run{}, false, err
		}
	}
}

func (c *PieceReaderAtCursor) loadNextPiece() error {
	i := c.pieceIndex
	c.pieceIndex++
	c.pieceRuns = nil
	c.runIndex = 0
	piece, ok := c.table.Piece(i)
	if !ok {
		return fmt.Errorf("doc word97 stream: missing PieceTable entry %d", i)
	}
	count := piece.CharCount()
	if count <= 0 {
		return nil
	}
	c.pieceRuns = SplitPieceReaderAt(c.wordReader, c.wordSize, piece.ByteOffset, piece.RawFC, count, piece.Compressed, piece.StartCP, c.chpxEntries, c.papxEntries, c.preferredCharset)
	return nil
}

func SplitPieceReaderAt(wordReader io.ReaderAt, wordSize int64, byteOff int64, rawFC uint32, charCount int, compressed bool, pieceStartCP int, chpx []docformat.CHPXEntry, papx []docformat.PAPXEntry, preferredCharset *charmap.Charmap) []Run {
	pieceFC, fcStep := pieceFC(rawFC, compressed)
	chpxBase, chpxStep := chpxFCBasis(rawFC, pieceFC, fcStep, compressed, charCount, chpx)
	papxBase, papxStep := papxFCBasis(rawFC, pieceFC, fcStep, compressed, charCount, papx)
	var result []Run
	for pos := 0; pos < charCount; {
		chpxFC := chpxBase + uint32(pos)*chpxStep
		papxFC := papxBase + uint32(pos)*papxStep
		ci := docformat.FindCHPXEntry(chpx, chpxFC)
		pi := docformat.FindPAPXEntry(papx, papxFC)
		var run Run
		if ci >= 0 {
			applyCHPX(&run, chpx[ci])
		}
		if pi >= 0 {
			applyPAPX(&run, papx[pi])
		}
		end := charCount
		cn := ci + 1
		if ci < 0 {
			cn = 0
		}
		if cn < len(chpx) && chpx[cn].FC > chpxBase {
			d := int((chpx[cn].FC - chpxBase) / chpxStep)
			if d > pos && d < end {
				end = d
			}
		}
		pn := pi + 1
		if pi < 0 {
			pn = 0
		}
		if pn < len(papx) && papx[pn].FC > papxBase {
			d := int((papx[pn].FC - papxBase) / papxStep)
			if d > pos && d < end {
				end = d
			}
		}
		count := end - pos
		off := byteOff + int64(pos)
		if !compressed {
			off = byteOff + int64(pos*2)
		}
		text := decodePieceTextReaderAt(wordReader, wordSize, off, count, compressed, preferredCharset)
		if text != "" {
			run.Text = text
			run.StartCP = pieceStartCP + pos
			if run.LegacyInTableHint && strings.HasSuffix(text, "\a") {
				run.ParagraphFormat.IsTableCellTrailingParagraph = true
			}
			if run.LegacyRowMarkHint && strings.HasSuffix(text, "\a") {
				run.ParagraphFormat.IsTableTrailingParagraph = true
			}
			result = append(result, run)
		}
		pos = end
	}
	return result
}

func applyCHPX(r *Run, e docformat.CHPXEntry) {
	r.Format = e.Format
	r.Revision = e.Revision
	r.Spec = e.Spec
	r.Data = e.Data
	r.Obj = e.Obj
	r.IbstRMark = e.IbstRMark
	r.HasIbstRMark = e.HasIbstRMark
	r.DttmRMark = e.DttmRMark
	r.HasDttmRMark = e.HasDttmRMark
	r.ChpxSprms = e.Sprms
	if picLocation, ok := picLocationFromCHPX(e.Sprms); ok {
		r.PicLocation = picLocation
		r.HasPicLocation = true
		r.Format.PicLocation = picLocation
	}
	if p := e.PropertyRevision; p != nil {
		r.HasRunPropRMark = p.Present
		r.RunPropRMarkIbst = p.Ibst
		r.HasRunPropRMarkIbst = p.HasIbst
		r.RunPropRMarkDttm = p.Dttm
		r.HasRunPropRMarkDttm = p.HasDttm
	}
	if p := e.DisplayFieldRevision; p != nil {
		r.HasDispFldRMark = p.Present
		r.DispFldRMarkIbst = p.Ibst
		r.HasDispFldRMarkIbst = p.HasIbst
		r.DispFldRMarkDttm = p.Dttm
		r.HasDispFldRMarkDttm = p.HasDttm
	}
}
func applyPAPX(r *Run, e docformat.PAPXEntry) {
	r.ParagraphFormat = e.Format
	r.TableDepth = e.Meta.TableDepth
	r.TableProps = e.Meta.TableRowProps
	r.LegacyInTableHint = e.LegacyInTableHint
	r.LegacyRowMarkHint = e.LegacyRowMarkHint
	r.PapxSprms = e.Sprms
	if p := e.Meta.PropertyRevision; p != nil {
		r.HasPropRMark = p.Present
		r.PropRMarkIbst = p.Ibst
		r.HasPropRMarkIbst = p.HasIbst
		r.PropRMarkDttm = p.Dttm
		r.HasPropRMarkDttm = p.HasDttm
	}
}
func pieceFC(raw uint32, compressed bool) (uint32, uint32) {
	v := raw & 0x3fffffff
	if compressed {
		return v / 2, 1
	}
	return v, 2
}
func chpxFCBasis(raw, byteFC, byteStep uint32, compressed bool, charCount int, entries []docformat.CHPXEntry) (uint32, uint32) {
	return entryFCBasis(raw, byteFC, byteStep, compressed, charCount, len(entries), func(i int) uint32 { return entries[i].FC })
}

func papxFCBasis(raw, byteFC, byteStep uint32, compressed bool, charCount int, entries []docformat.PAPXEntry) (uint32, uint32) {
	return entryFCBasis(raw, byteFC, byteStep, compressed, charCount, len(entries), func(i int) uint32 { return entries[i].FC })
}

// entryFCBasis resolves the historical compressed-piece FC ambiguity using
// formatting boundaries that actually intersect this piece. The old code used
// the first FKP entry in the whole document; once that entry was far away from
// the current piece it could select the wrong coordinate system for unrelated
// text. We retain compatibility with documents whose FKP boundaries use the
// encoded PCD coordinate system, but choose it only when this piece's interval
// provides stronger evidence.
func entryFCBasis(raw, byteFC, byteStep uint32, compressed bool, charCount, count int, fcAt func(int) uint32) (uint32, uint32) {
	if !compressed || count == 0 || charCount <= 0 {
		return byteFC, byteStep
	}
	byteScore := formattingBoundariesInPiece(byteFC, byteStep, charCount, count, fcAt)
	rawScore := formattingBoundariesInPiece(raw, 2, charCount, count, fcAt)
	if rawScore > byteScore {
		return raw, 2
	}
	return byteFC, byteStep
}

func formattingBoundariesInPiece(base, step uint32, charCount, count int, fcAt func(int) uint32) int {
	if step == 0 || charCount <= 0 {
		return 0
	}
	end := base + uint32(charCount)*step
	score := 0
	for i := 0; i < count; i++ {
		fc := fcAt(i)
		if fc >= base && fc < end {
			score++
		}
	}
	return score
}

func decodePieceTextReaderAt(r io.ReaderAt, size, off int64, count int, compressed bool, preferred *charmap.Charmap) string {
	if r == nil || off < 0 || off >= size || count <= 0 {
		return ""
	}
	want := count
	if !compressed {
		want *= 2
	}
	if int64(want) > size-off {
		want = int(size - off)
	}
	if want <= 0 {
		return ""
	}
	buf := make([]byte, want)
	n, err := r.ReadAt(buf, off)
	if err != nil && err != io.EOF {
		return ""
	}
	buf = buf[:n]
	if compressed {
		if preferred != nil {
			if out, err := preferred.NewDecoder().Bytes(buf); err == nil {
				return string(out)
			}
		}
		if out, err := charmap.Windows1252.NewDecoder().Bytes(buf); err == nil {
			return string(out)
		}
		return string(buf)
	}
	if len(buf)%2 != 0 {
		buf = buf[:len(buf)-1]
	}
	u := make([]uint16, 0, len(buf)/2)
	for i := 0; i+1 < len(buf); i += 2 {
		u = append(u, uint16(buf[i])|uint16(buf[i+1])<<8)
	}
	return string(utf16.Decode(u))
}

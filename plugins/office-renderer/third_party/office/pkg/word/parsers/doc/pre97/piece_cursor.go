package pre97

import (
	"bytes"

	docformat "office/pkg/word/parsers/doc/format"
	word97 "office/pkg/word/parsers/doc/word97"
)

// PieceCursor is the canonical pre-Word97 main-story cursor. It reuses the
// bounded PieceTable reader shared with Word97, but owns the old-format
// CHPX/PAPX projection and exposes only the pre97.Run contract.
type PieceCursor struct {
	inner word97.Cursor
}

func NewPieceCursor(word []byte, core *Core) (*PieceCursor, error) {
	if core == nil {
		return nil, UnsupportedError{Feature: "missing pre-Word97 core"}
	}
	inner, err := word97.NewPieceReaderAtCursor(
		bytes.NewReader(word), int64(len(word)), core.PiecePLC,
		pieceCHPX(core), piecePAPX(core), core.PreferredCharset,
	)
	if err != nil {
		return nil, err
	}
	return &PieceCursor{inner: inner}, nil
}

func (c *PieceCursor) Next() (Run, bool, error) {
	if c == nil || c.inner == nil {
		return Run{}, false, nil
	}
	run, ok, err := c.inner.Next()
	if err != nil || !ok {
		return Run{}, ok, err
	}
	return Run{
		Text: run.Text, Kind: run.Kind, Format: run.Format, ParagraphFormat: run.ParagraphFormat,
		TableDepth: run.TableDepth, LegacyInTableHint: run.LegacyInTableHint,
		LegacyRowMarkHint: run.LegacyRowMarkHint, TableProps: run.TableProps,
		StartCP: run.StartCP, FieldKind: run.FieldKind, FieldInstr: run.FieldInstr, FieldArg: run.FieldArg,
		Target: run.Target, Anchor: run.Anchor, NoteID: run.NoteID, CommentID: run.CommentID,
		Image: run.Image, Shape: run.Shape, Spec: run.Spec, Data: run.Data, Obj: run.Obj,
		ChpxSprms: append([]byte(nil), run.ChpxSprms...), PapxSprms: append([]byte(nil), run.PapxSprms...),
	}, true, nil
}

func pieceCHPX(core *Core) []docformat.CHPXEntry {
	if core == nil || len(core.CHPX) == 0 {
		return nil
	}
	out := make([]docformat.CHPXEntry, 0, len(core.CHPX))
	for _, entry := range core.CHPX {
		out = append(out, docformat.CHPXEntry{
			FC: entry.FC,
			Format: DecodeCHPXGrpprl(entry.Sprms, core.FontNames),
			Sprms: append([]byte(nil), entry.Sprms...),
		})
	}
	return out
}

func piecePAPX(core *Core) []docformat.PAPXEntry {
	if core == nil || len(core.PAPX) == 0 {
		return nil
	}
	out := make([]docformat.PAPXEntry, 0, len(core.PAPX))
	for _, entry := range core.PAPX {
		if entry.EndFC <= entry.StartFC {
			continue
		}
		format, semantic := DecodePAPXGrpprl(entry.Sprms)
		if entry.StyleIndex != nil {
			style := *entry.StyleIndex
			format.BinaryStyleIndex = &style
		}
		out = append(out, docformat.PAPXEntry{
			FC: uint32(entry.StartFC), Format: format,
			Meta: docformat.PAPXMetadata{PreWord97ListStart: semantic.ListStart},
			LegacyInTableHint: semantic.InTable, LegacyRowMarkHint: semantic.RowMark,
			Sprms: append([]byte(nil), entry.Sprms...),
		})
	}
	return out
}

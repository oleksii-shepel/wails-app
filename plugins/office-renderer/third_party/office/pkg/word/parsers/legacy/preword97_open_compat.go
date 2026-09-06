package legacy

import (
	"fmt"

	"office/pkg/ooxml/cfb"
	pre97 "office/pkg/word/parsers/doc/pre97"
)

// PreWord97Compatibility now supplies only the remaining binary compatibility
// decoding needed to build canonical pre97 state. The returned cursor and
// context are canonical and no longer round-trip through legacyDocStreamCore.
type PreWord97Compatibility struct{}

var _ pre97.CompatibilityServices = PreWord97Compatibility{}

func (PreWord97Compatibility) OpenOpenedCFB(f *cfb.File) (pre97.Cursor, *pre97.Context, error) {
	if f == nil {
		return nil, nil, fmt.Errorf("legacy doc stream: nil cfb")
	}
	wordStream, ok := f.Stream("WordDocument")
	if !ok {
		wordStream, ok = f.Stream("Ole")
	}
	if !ok {
		return nil, nil, pre97.ErrNotApplicable
	}
	version, valid := legacyDocFibVersion(wordStream)
	if !((valid && version == legacyDocFibWord95) || legacyDocLooksPreWord97(wordStream)) {
		return nil, nil, pre97.ErrNotApplicable
	}

	return newCanonicalPreWord97StreamCursor(wordStream, f)
}

type pre97ToLegacyCursor struct{ inner pre97.Cursor }

func (c *pre97ToLegacyCursor) Next() (docRun, bool, error) {
	if c == nil || c.inner == nil {
		return docRun{}, false, nil
	}
	run, ok, err := c.inner.Next()
	if err != nil {
		return docRun{}, false, err
	}
	if !ok {
		return docRun{}, false, nil
	}
	return docRun{
		Text: run.Text, Kind: run.Kind, Format: run.Format, ParaFormat: run.ParagraphFormat,
		TableDepth: run.TableDepth, LegacyInTableHint: run.LegacyInTableHint, StartCP: run.StartCP,
		FieldKind: run.FieldKind, FieldInstr: run.FieldInstr, FieldArg: run.FieldArg,
		Target: run.Target, Anchor: run.Anchor, NoteID: run.NoteID, CommentID: run.CommentID,
		Image: run.Image, Shape: run.Shape, Spec: run.Spec, Data: run.Data, Obj: run.Obj,
	}, true, nil
}

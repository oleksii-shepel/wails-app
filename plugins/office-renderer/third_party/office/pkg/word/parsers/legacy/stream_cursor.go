package legacy

import (
	"strings"

	wordir "office/pkg/word/ir"
)

// legacyRunCursor is the streaming boundary between binary DOC decoding and
// semantic event emission. PieceTable/PAPX/CHPX readers implement this directly
// so the event state machine does not depend on whole-document run slices.
type legacyRunCursor interface {
	Next() (docRun, bool, error)
}

type legacyRunSliceCursor struct {
	runs  []docRun
	index int
}

func newLegacyRunSliceCursor(runs []docRun) legacyRunCursor {
	return &legacyRunSliceCursor{runs: runs}
}

func (c *legacyRunSliceCursor) Next() (docRun, bool, error) {
	if c == nil || c.index >= len(c.runs) {
		return docRun{}, false, nil
	}
	run := c.runs[c.index]
	c.index++
	return run, true, nil
}

// legacySemanticRunCursor layers compact stylesheet semantics over raw PAPX /
// CHPX runs as they are produced. This mirrors the mature whole-slice style
// projection without requiring a document-wide post pass.
type legacySemanticRunCursor struct {
	inner      legacyRunCursor
	stylesheet *legacyStyleSheet
}

func newLegacySemanticRunCursor(inner legacyRunCursor, stylesheet *legacyStyleSheet) legacyRunCursor {
	if inner == nil {
		return nil
	}
	return &legacySemanticRunCursor{inner: inner, stylesheet: stylesheet}
}

func (c *legacySemanticRunCursor) Next() (docRun, bool, error) {
	run, ok, err := c.inner.Next()
	if err != nil || !ok {
		return run, ok, err
	}
	if c.stylesheet != nil && run.ParaFormat.BinaryStyleIndex != nil {
		istd := *run.ParaFormat.BinaryStyleIndex
		if style, exists := c.stylesheet.paragraphByIndex[istd]; exists {
			styleFormat := style.Format
			applyStyleFormatToRunIfUnset(&run, styleFormat)

			// Keep style-derived binary list metadata explicit. The mature merge
			// path carries these pointers, while applyStyleFormatToRunIfUnset is
			// intentionally focused on visual paragraph properties.
			if run.ParaFormat.BinaryListOverride == nil && styleFormat.BinaryListOverride != nil {
				v := *styleFormat.BinaryListOverride
				run.ParaFormat.BinaryListOverride = &v
			}
			if run.ParaFormat.BinaryListLevel == nil && styleFormat.BinaryListLevel != nil {
				v := *styleFormat.BinaryListLevel
				run.ParaFormat.BinaryListLevel = &v
			}
			if run.ParaFormat.BinaryListFormat == nil && styleFormat.BinaryListFormat != nil {
				v := *styleFormat.BinaryListFormat
				run.ParaFormat.BinaryListFormat = &v
			}

			// Preserve the derived table hints used by the mature parser. The
			// style merge may already have set ParaFormat.InTable, so derive the
			// legacy hint from the effective value rather than from the pre-merge
			// state alone.
			if run.ParaFormat.InTable || styleFormat.InTable {
				run.ParaFormat.InTable = true
				run.LegacyInTableHint = true
				if strings.HasSuffix(run.Text, "\a") {
					run.ParaFormat.IsTableCellTrailingParagraph = true
				}
			}
		}
	}
	return run, true, nil
}

// legacyStreamNumberingReference promotes the binary ilfo/ilvl pair into the
// format-neutral Word numbering reference carried by startParagraph events.
func legacyStreamNumberingReference(run docRun) *wordir.NumberingReference {
	if run.ParaFormat.BinaryListOverride == nil || run.ParaFormat.BinaryListLevel == nil {
		return nil
	}
	ilfo := *run.ParaFormat.BinaryListOverride
	level := *run.ParaFormat.BinaryListLevel
	if ilfo <= 0 || level < 0 {
		return nil
	}
	return &wordir.NumberingReference{NumID: ilfo, Level: level}
}

func validateLegacyStreamRun(run docRun) error {
	// sprmCFData is a generic data-bearing run flag, not proof of a picture.
	// Word97 fixed pictures are identified and resolved earlier from fSpec plus
	// sprmCPicLocation. Rejecting every unresolved fData run here turns valid
	// field/form data into a false picture/data-anchor layout failure.
	if run.Spec && run.Obj {
		return StreamUnsupportedError{Feature: "OLE object anchor"}
	}
	// Raw PieceTable cursors have not yet applied the stateful legacy field
	// rewriter. Refuse them explicitly rather than exposing FLDBEGIN/SEP/END as
	// visible control characters.
	if strings.ContainsAny(run.Text, "\x13\x14\x15") {
		return StreamUnsupportedError{Feature: "field"}
	}
	return nil
}

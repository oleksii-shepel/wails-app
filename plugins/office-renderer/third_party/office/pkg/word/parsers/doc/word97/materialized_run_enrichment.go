package word97

import (
	"fmt"

	"office/pkg/ooxml/cfb"
)

type materializedRunCursor struct {
	runs  []Run
	index int
}

func (c *materializedRunCursor) Next() (Run, bool, error) {
	if c == nil || c.index >= len(c.runs) {
		return Run{}, false, nil
	}
	run := c.runs[c.index]
	c.index++
	return run, true, nil
}

// EnrichMaterializedRuns applies CP-sensitive document-wide Word97 semantics to
// an already materialized run range. This is the compatibility-safe counterpart
// of the normal streaming pipeline.
//
// Reference insertion deliberately precedes drawing expansion here. The mature
// legacy adapter used that order, which preserves a reference even when a PLCF
// insertion CP coincides with a drawing/object anchor CP.
func EnrichMaterializedRuns(runs []Run, metadata Metadata, context *Context, f *cfb.File) ([]Run, error) {
	if len(runs) == 0 {
		return runs, nil
	}

	var cursor Cursor = &materializedRunCursor{runs: runs}

	footnoteRefs := metadata.FootnoteRefs
	endnoteRefs := metadata.EndnoteRefs
	commentRefs := metadata.CommentRefs
	if context != nil {
		if len(context.FootnoteRefs) > 0 {
			footnoteRefs = context.FootnoteRefs
		}
		if len(context.EndnoteRefs) > 0 {
			endnoteRefs = context.EndnoteRefs
		}
		if len(context.CommentRefs) > 0 {
			commentRefs = context.CommentRefs
		}
	}
	cursor = NewReferenceCursor(cursor, footnoteRefs, endnoteRefs, commentRefs)

	if context != nil {
		var pictureData []byte
		if f != nil {
			if data, ok := f.Stream("Data"); ok && len(data) > 0 {
				pictureData = data
			} else if data, ok := f.Stream("DataStream"); ok && len(data) > 0 {
				pictureData = data
			}
		}
		drawings := context.Drawings
		cursor = NewCompatibilityDrawingCursorWithPictureData(
			cursor,
			drawings.Drawings,
			drawings.InlineImages,
			pictureData,
			drawings.OLEObjects,
			drawings.OLEByCP,
		)
	}

	materialized := make([]Run, 0, len(runs))
	for {
		run, ok, err := cursor.Next()
		if err != nil {
			return nil, fmt.Errorf("word97 enrich materialized runs: %w", err)
		}
		if !ok {
			break
		}
		materialized = append(materialized, run)
	}
	var dataStream []byte
	if f != nil {
		if data, ok := f.Stream("Data"); ok && len(data) > 0 {
			dataStream = data
		} else if data, ok := f.Stream("DataStream"); ok && len(data) > 0 {
			dataStream = data
		}
	}
	return MaterializeFields(materialized, dataStream), nil
}

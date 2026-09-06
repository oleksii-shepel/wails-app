package word97

import (
	"fmt"
	"io"

	"office/pkg/ooxml/cfb"
)

// ValidateContext verifies that canonical side-story metadata and materialized
// stories agree. Recognized-but-unresolved stories are reported as
// UnsupportedError so outer parser layers can translate them at their API
// boundary without owning Word97 validation semantics.
func ValidateContext(opened *OpenCore, context *Context) error {
	if opened == nil || context == nil {
		return fmt.Errorf("doc word97 open: invalid canonical context")
	}
	counts := opened.Metadata.StoryCounts
	footnoteRefs := context.FootnoteRefs
	if len(footnoteRefs) == 0 { footnoteRefs = opened.Metadata.FootnoteRefs }
	endnoteRefs := context.EndnoteRefs
	if len(endnoteRefs) == 0 { endnoteRefs = opened.Metadata.EndnoteRefs }
	commentRefs := context.CommentRefs
	if len(commentRefs) == 0 { commentRefs = opened.Metadata.CommentRefs }
	if counts.Ftn > 0 && (len(footnoteRefs) == 0 || len(context.Footnotes) == 0) { return UnsupportedError{Feature: "footnote story"} }
	if counts.Edn > 0 && (len(endnoteRefs) == 0 || len(context.Endnotes) == 0) { return UnsupportedError{Feature: "endnote story"} }
	if counts.Atn > 0 && (len(commentRefs) == 0 || len(context.Comments) == 0) { return UnsupportedError{Feature: "comment story"} }
	if counts.Hdr > 0 && len(context.Headers) == 0 && len(context.Footers) == 0 { return UnsupportedError{Feature: "header/footer story"} }
	return nil
}

// OpenReaderAt owns the complete normal Word97 read pipeline, including
// revision attribution and drawing/media projection. No parsers/legacy
// compatibility service participates in the normal Word97 path.
func OpenReaderAt(word io.ReaderAt, wordSize int64, f *cfb.File) (Cursor, *Context, *OpenCore, error) {
	snapshot, err := OpenSnapshotReaderAt(word, wordSize, f)
	if err != nil { return nil, nil, nil, err }
	if snapshot == nil || snapshot.Core == nil || snapshot.Core.Cursor == nil {
		var opened *OpenCore
		if snapshot != nil { opened = snapshot.Core }
		return nil, nil, opened, fmt.Errorf("doc word97 open: missing canonical cursor")
	}
	opened := snapshot.Core
	context := snapshot.Context
	if err := ValidateContext(opened, context); err != nil { return nil, context, opened, err }

	counts := opened.Metadata.StoryCounts
	cursor := mainStoryCursor(opened.Cursor, counts.Text)
	cursor = NewRevisionCursor(cursor, opened.RevisionAuthors)
	cursor = NewFieldCursor(cursor, ParseFieldInstruction)
	if opened.Styles != nil { cursor = NewStyleCursor(cursor, opened.Styles.ResolveParagraphFormat, nil) }

	drawings := context.Drawings
	if counts.Txbx > 0 || counts.HdrTxbx > 0 {
		if drawings.ExpectedTextboxes == 0 || drawings.ResolvedTextboxes < drawings.ExpectedTextboxes {
			return nil, context, opened, UnsupportedError{Feature: "textbox"}
		}
	}
	var pictureData []byte
	if f != nil {
		if data, ok := f.Stream("Data"); ok && len(data) > 0 {
			pictureData = data
		} else if data, ok := f.Stream("DataStream"); ok && len(data) > 0 {
			pictureData = data
		}
	}
	cursor = NewDrawingCursorWithPictureData(cursor, drawings.Drawings, drawings.InlineImages, pictureData, drawings.OLEObjects, drawings.OLEByCP)

	footnoteRefs := context.FootnoteRefs
	if len(footnoteRefs) == 0 { footnoteRefs = opened.Metadata.FootnoteRefs }
	endnoteRefs := context.EndnoteRefs
	if len(endnoteRefs) == 0 { endnoteRefs = opened.Metadata.EndnoteRefs }
	commentRefs := context.CommentRefs
	if len(commentRefs) == 0 { commentRefs = opened.Metadata.CommentRefs }
	cursor = NewReferenceCursor(cursor, footnoteRefs, endnoteRefs, commentRefs)
	cursor = NewBookmarkCursor(cursor, opened.Metadata.BookmarkStarts, opened.Metadata.BookmarkEnds)
	cursor = NewSectionBoundaryCursor(cursor, context.Sections)
	// Normalize only after every CP-sensitive semantic decorator has consumed
	// its Word control markers. Raw C0 controls must never reach text/HTML.
	cursor = NewControlTextCursor(cursor)
	return cursor, context, opened, nil
}

package legacy

import (
	"fmt"

	"office/pkg/ooxml/cfb"
	wordir "office/pkg/word/ir"
	docofficeart "office/pkg/word/parsers/doc/officeart"
	pre97 "office/pkg/word/parsers/doc/pre97"
	word97 "office/pkg/word/parsers/doc/word97"
)

// newLegacyPreWord97StreamCursor remains only for the legacy Stream API. The
// complete pre-Word97 semantic pipeline is canonical before this final adapter.
func newLegacyPreWord97StreamCursor(wordStream []byte, f *cfb.File) (legacyRunCursor, *legacyDocStreamCore, error) {
	cursor, context, err := newCanonicalPreWord97StreamCursor(wordStream, f)
	if err != nil {
		return nil, nil, err
	}
	return &pre97ToLegacyCursor{inner: cursor}, legacyStreamCoreFromPre97Context(context), nil
}

func newCanonicalPreWord97StreamCursor(wordStream []byte, f *cfb.File) (pre97.Cursor, *pre97.Context, error) {
	oldCore, ok := loadPreWord97DocCore(wordStream)
	if !ok || oldCore == nil || oldCore.Canonical == nil {
		return nil, nil, fmt.Errorf("legacy doc stream: unavailable pre-Word97 core")
	}
	if len(oldCore.PiecePlc) == 0 {
		if oldCore.Span.FComplex {
			return nil, nil, fmt.Errorf("legacy doc stream: unavailable pre-Word97 complex PieceTable")
		}
		return nil, nil, fmt.Errorf("legacy doc stream: unavailable pre-Word97 PieceTable")
	}

	pieceCursor, err := pre97.NewPieceCursor(wordStream, oldCore.Canonical)
	if err != nil {
		return nil, nil, err
	}
	canonicalCursor := pre97.NewFieldCursor(pieceCursor)
	canonicalCursor = pre97.NewStyleCursor(canonicalCursor, oldCore.Canonical.Styles)
	canonicalCursor = pre97.NewGridCursor(canonicalCursor)

	sections := preWord97SectionsFromCanonical(oldCore.Canonical.Sections)
	if len(sections) == 0 {
		sections = loadPreWord97Sections(wordStream)
	}
	context := &pre97.Context{Sections: sections, OLEByCP: make(map[int]*wordir.Run)}

	if notes := preWord97NoteDataFromRanges(wordStream, oldCore, oldCore.Canonical.Stories.Footnotes); notes != nil {
		context.FootnoteRefs = notes.RefByCP
		context.Footnotes = preWord97NoteDataAsStoryMap(notes)
	} else if notes := loadPreWord97FootnoteStories(wordStream); notes != nil {
		context.FootnoteRefs = notes.RefByCP
		context.Footnotes = preWord97NoteDataAsStoryMap(notes)
	}
	if notes := preWord97NoteDataFromRanges(wordStream, oldCore, oldCore.Canonical.Stories.Endnotes); notes != nil {
		context.EndnoteRefs = notes.RefByCP
		context.Endnotes = preWord97NoteDataAsStoryMap(notes)
	} else if notes := loadPreWord97EndnoteStories(wordStream); notes != nil {
		context.EndnoteRefs = notes.RefByCP
		context.Endnotes = preWord97NoteDataAsStoryMap(notes)
	}
	if notes := preWord97NoteDataFromRanges(wordStream, oldCore, oldCore.Canonical.Stories.Comments); notes != nil {
		context.CommentRefs = notes.RefByCP
		context.Comments = preWord97NoteDataAsStoryMap(notes)
	} else if notes := loadPreWord97AnnotationStories(wordStream); notes != nil {
		context.CommentRefs = notes.RefByCP
		context.Comments = preWord97NoteDataAsStoryMap(notes)
	}

	context.BookmarkStarts = oldCore.Canonical.Bookmarks.Starts
	context.BookmarkEnds = oldCore.Canonical.Bookmarks.Ends
	if len(context.BookmarkStarts) == 0 {
		context.BookmarkStarts, context.BookmarkEnds = preWord97StreamBookmarks(wordStream)
	}

	// Reuse the remaining compatibility header/preservation adapters behind a
	// temporary boundary object, then project only semantic context out.
	tmp := &legacyDocStreamCore{Sections: context.Sections, OLEByCP: context.OLEByCP}
	applyCanonicalPreWord97Headers(wordStream, oldCore, tmp)

	inlineImages := word97.LoadInlineImages(f)
	oleObjects := word97.LoadOLEObjects(f)
	if f != nil && len(oleObjects) > 0 {
		if data, ok := f.Stream("Data"); ok && len(data) > 0 {
			docofficeart.AttachOLEPreviews(oleObjects, data)
		} else if data, ok := f.Stream("DataStream"); ok && len(data) > 0 {
			docofficeart.AttachOLEPreviews(oleObjects, data)
		}
	}
	tmp.InlineImages = inlineImages
	tmp.OLEObjects = oleObjects
	mergeLegacyStreamPreservationMeta(f, tmp)

	context.Sections = tmp.Sections
	context.Headers = tmp.Headers
	context.Footers = tmp.Footers
	context.FootnoteSeparator = tmp.FootnoteSeparator
	context.FootnoteContinuationSep = tmp.FootnoteContinuationSep
	context.FootnoteContinuationNotice = tmp.FootnoteContinuationNotice
	context.EndnoteSeparator = tmp.EndnoteSeparator
	context.EndnoteContinuationSep = tmp.EndnoteContinuationSep
	context.EndnoteContinuationNotice = tmp.EndnoteContinuationNotice
	context.OLEByCP = tmp.OLEByCP
	if len(tmp.Footnotes) > 0 {
		context.Footnotes = tmp.Footnotes
	}
	if len(tmp.Endnotes) > 0 {
		context.Endnotes = tmp.Endnotes
	}
	if len(tmp.Comments) > 0 {
		context.Comments = tmp.Comments
	}

	drawings := make(map[int]pre97.Drawing, len(tmp.Drawings))
	for cp, drawing := range tmp.Drawings {
		drawings[cp] = pre97.Drawing{Image: drawing.Image, Shape: drawing.Shape}
	}
	canonicalCursor = pre97.NewDrawingCursor(canonicalCursor, pre97.DrawingProjection{
		Drawings: drawings, InlineImages: inlineImages, OLEObjects: oleObjects, OLEByCP: context.OLEByCP,
	})
	return canonicalCursor, context, nil
}

func legacyStreamCoreFromPre97Context(context *pre97.Context) *legacyDocStreamCore {
	if context == nil {
		return nil
	}
	return &legacyDocStreamCore{
		Sections: context.Sections,
		Headers: context.Headers,
		Footers: context.Footers,
		Footnotes: context.Footnotes,
		Endnotes: context.Endnotes,
		Comments: context.Comments,
		FootnoteSeparator: context.FootnoteSeparator,
		FootnoteContinuationSep: context.FootnoteContinuationSep,
		FootnoteContinuationNotice: context.FootnoteContinuationNotice,
		EndnoteSeparator: context.EndnoteSeparator,
		EndnoteContinuationSep: context.EndnoteContinuationSep,
		EndnoteContinuationNotice: context.EndnoteContinuationNotice,
		OLEByCP: context.OLEByCP,
	}
}

func preWord97NoteDataFromRanges(wordStream []byte, oldCore *preWord97DocCore, ranges []pre97.ReferencedStoryRange) *legacyNoteData {
	if oldCore == nil || len(ranges) == 0 { return nil }
	data := &legacyNoteData{RefByCP: make(map[int]int, len(ranges)), Stories: make(map[int]wordir.Story, len(ranges))}
	for _, item := range ranges {
		if item.ID <= 0 || item.Range.EndCP <= item.Range.StartCP { continue }
		data.RefByCP[item.ReferenceCP] = item.ID
		start := oldCore.Span.Start + item.Range.StartCP
		end := oldCore.Span.Start + item.Range.EndCP
		if start < 0 || end > len(wordStream) || end <= start { continue }
		raw := decodePreWord97SingleByteTextWithCharset(wordStream[start:end], oldCore.PreferredCharset)
		raw = stripPreWord97NoteMarker(raw)
		if raw != "" { data.Stories[item.ID] = wordir.Story{Text: raw} }
	}
	if len(data.RefByCP) == 0 && len(data.Stories) == 0 { return nil }
	return data
}

func preWord97SectionsFromCanonical(records []pre97.SectionRecord) []wordir.Section {
	if len(records) == 0 { return nil }
	out := make([]wordir.Section, 0, len(records))
	for _, record := range records {
		section := parsePreWord97SepxGrpprl(record.Sepx)
		section.Source = &wordir.SectionSource{SepxSprms: append([]byte(nil), record.Sepx...), StartCP: record.StartCP, EndCP: record.EndCP}
		out = append(out, section)
	}
	return out
}

func applyCanonicalPreWord97Headers(wordStream []byte, oldCore *preWord97DocCore, core *legacyDocStreamCore) {
	if oldCore == nil || oldCore.Canonical == nil || core == nil { return }
	var group [6]wordir.Story
	hasContent := false
	for i, r := range oldCore.Canonical.HeaderFooter.Slots {
		if r.EndCP <= r.StartCP { continue }
		start := oldCore.Span.Start + r.StartCP
		end := oldCore.Span.Start + r.EndCP
		if start < 0 || end > len(wordStream) || end <= start { continue }
		raw := decodePreWord97SingleByteTextWithCharset(wordStream[start:end], oldCore.PreferredCharset)
		group[i] = buildHdrFtrStoryFromRaw(raw)
		if group[i].Text != "" { hasContent = true }
	}
	if !hasContent { applyPreWord97StreamHeaders(wordStream, core); return }
	var noteStories [6]wordir.Story
	hf := buildLegacyHeaderFooterDataFromSlots(noteStories, [][6]wordir.Story{group}, 1)
	if hf == nil { return }
	core.Headers = hf.Headers; core.Footers = hf.Footers
	if len(core.Sections) == 0 && (len(hf.SectionHeaders) > 0 || len(hf.SectionFooters) > 0) { core.Sections = []wordir.Section{{}} }
	for i := range core.Sections {
		if i < len(hf.SectionHeaders) { core.Sections[i].Headers = append([]wordir.Reference(nil), hf.SectionHeaders[i]...) }
		if i < len(hf.SectionFooters) { core.Sections[i].Footers = append([]wordir.Reference(nil), hf.SectionFooters[i]...) }
	}
}

func preWord97StreamBookmarks(wordStream []byte) (map[int][]string, map[int][]string) {
	starts, endsByName := loadPreWord97BookmarkData(wordStream)
	if len(endsByName) == 0 { return starts, nil }
	ends := make(map[int][]string, len(endsByName)); for name, cp := range endsByName { ends[cp] = append(ends[cp], name) }; return starts, ends
}

func applyPreWord97StreamHeaders(wordStream []byte, core *legacyDocStreamCore) {
	if core == nil { return }
	hf := loadPreWord97HeaderFooterStories(wordStream); if hf == nil { return }
	core.Headers = hf.Headers; core.Footers = hf.Footers
	core.FootnoteSeparator = hf.FootnoteSeparator; core.FootnoteContinuationSep = hf.FootnoteContinuationSep; core.FootnoteContinuationNotice = hf.FootnoteContinuationNotice
	core.EndnoteSeparator = hf.EndnoteSeparator; core.EndnoteContinuationSep = hf.EndnoteContinuationSep; core.EndnoteContinuationNotice = hf.EndnoteContinuationNotice
	if len(core.Sections) == 0 && (len(hf.SectionHeaders) > 0 || len(hf.SectionFooters) > 0) { core.Sections = []wordir.Section{{}} }
	for i := range core.Sections {
		if i < len(hf.SectionHeaders) { core.Sections[i].Headers = append([]wordir.Reference(nil), hf.SectionHeaders[i]...) }
		if i < len(hf.SectionFooters) { core.Sections[i].Footers = append([]wordir.Reference(nil), hf.SectionFooters[i]...) }
	}
}

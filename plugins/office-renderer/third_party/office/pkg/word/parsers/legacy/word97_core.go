package legacy

import (
	"bytes"

	"office/pkg/diag"
	"office/pkg/ooxml/cfb"
	wordir "office/pkg/word/ir"
	docbinary "office/pkg/word/parsers/doc/binary"
	word97 "office/pkg/word/parsers/doc/word97"
)

// legacyDocStreamCore is the temporary compatibility context consumed only by
// the legacy Stream API adapters. It is version-neutral so pre-Word97 no longer
// depends conceptually on the old full-buffer Word97 parser.
type legacyDocStreamCore struct {
	DataStream      []byte
	CcpText         int
	ClxData         []byte
	ChpxEntries     []chpxEntry
	PapxEntries     []papxEntry
	ListTables      *legacyListTables
	StyleSheet      *legacyStyleSheet
	DocSettings     legacyDocSettings
	Compatibility   *wordir.DocCompatibility
	FontFamilies    map[string]wordir.FontFamilyInfo
	EmbeddedFonts   []*wordir.EmbeddedFont
	BookmarkStarts  map[int][]string
	BookmarkEnds    map[int][]string
	FootnoteRefs    map[int]int
	EndnoteRefs     map[int]int
	CommentRefs     map[int]int
	Footnotes       map[int]wordir.Story
	Endnotes        map[int]wordir.Story
	Comments        map[int]wordir.Story
	Sections        []wordir.Section
	Headers         map[string]wordir.Story
	Footers         map[string]wordir.Story
	FootnoteSeparator          wordir.Story
	FootnoteContinuationSep    wordir.Story
	FootnoteContinuationNotice wordir.Story
	EndnoteSeparator           wordir.Story
	EndnoteContinuationSep     wordir.Story
	EndnoteContinuationNotice  wordir.Story
	Drawings                    map[int]word97.Drawing
	InlineImages                []*wordir.Image
	TextBoxes                   []*wordir.TextBox
	OLEObjects                  []wordir.Run
	OLEByCP                     map[int]*wordir.Run
	RevisionAuthors             []string
	RawDOP                      []byte
	Diagnostics                 diag.List
	Version                     uint16
}

func legacyStoryCountsFromSequential(counts docbinary.StoryCounts) legacyStoryCounts {
	return legacyStoryCounts{
		Text: counts.Text, Ftn: counts.Ftn, Hdr: counts.Hdr, Mcr: counts.Mcr,
		Atn: counts.Atn, Edn: counts.Edn, Txbx: counts.Txbx, HdrTxbx: counts.HdrTxbx,
	}
}

func applySequentialMetadata(core *legacyDocStreamCore, metadata word97.Metadata) {
	if core == nil {
		return
	}
	core.BookmarkStarts = metadata.BookmarkStarts
	core.BookmarkEnds = metadata.BookmarkEnds
	core.FootnoteRefs = metadata.FootnoteRefs
	core.EndnoteRefs = metadata.EndnoteRefs
	core.CommentRefs = metadata.CommentRefs
}

func applySequentialContext(core *legacyDocStreamCore, ctx *word97.Context) {
	if core == nil || ctx == nil {
		return
	}
	core.Headers = ctx.Headers
	core.Footers = ctx.Footers
	core.Sections = ctx.Sections
	if len(ctx.FootnoteRefs) > 0 {
		core.FootnoteRefs = ctx.FootnoteRefs
	}
	if len(ctx.EndnoteRefs) > 0 {
		core.EndnoteRefs = ctx.EndnoteRefs
	}
	if len(ctx.CommentRefs) > 0 {
		core.CommentRefs = ctx.CommentRefs
	}
	core.Footnotes = ctx.Footnotes
	core.Endnotes = ctx.Endnotes
	core.Comments = ctx.Comments
	projection := ctx.Drawings
	core.Drawings = projection.Drawings
	core.InlineImages = projection.InlineImages
	core.TextBoxes = projection.TextBoxes
	core.OLEByCP = projection.OLEByCP
	if ctx.FootnoteSeparator != nil {
		core.FootnoteSeparator = *ctx.FootnoteSeparator
	}
	if ctx.FootnoteContinuationSep != nil {
		core.FootnoteContinuationSep = *ctx.FootnoteContinuationSep
	}
	if ctx.FootnoteContinuationNotice != nil {
		core.FootnoteContinuationNotice = *ctx.FootnoteContinuationNotice
	}
	if ctx.EndnoteSeparator != nil {
		core.EndnoteSeparator = *ctx.EndnoteSeparator
	}
	if ctx.EndnoteContinuationSep != nil {
		core.EndnoteContinuationSep = *ctx.EndnoteContinuationSep
	}
	if ctx.EndnoteContinuationNotice != nil {
		core.EndnoteContinuationNotice = *ctx.EndnoteContinuationNotice
	}
}

func legacyDocSettingsFromSequential(s word97.DocumentSettings) legacyDocSettings {
	return legacyDocSettings{
		DefaultTabStop: s.DefaultTabStop, FootnoteNumberFormat: s.FootnoteNumberFormat,
		FootnoteNumberStart: s.FootnoteNumberStart, FootnoteRestart: s.FootnoteRestart,
		EndnotePosition: s.EndnotePosition, EndnoteNumberFormat: s.EndnoteNumberFormat,
		EndnoteNumberStart: s.EndnoteNumberStart, EndnoteRestart: s.EndnoteRestart,
		ZoomPercent: s.ZoomPercent, ViewMode: s.ViewMode, ProtectionEnabled: s.ProtectionEnabled,
		DocumentProtection: s.DocumentProtection, CreatedAt: s.CreatedAt, ModifiedAt: s.ModifiedAt,
		LastPrintedAt: s.LastPrintedAt, WordCount: s.WordCount, CharCount: s.CharCount,
		PageCount: s.PageCount, ParagraphCount: s.ParagraphCount, MirrorMargins: s.MirrorMargins,
		TrackChangesEnabled: s.TrackChangesEnabled,
	}
}

func loadLegacyWord97DocCore(wordStream []byte, f *cfb.File) (*legacyDocStreamCore, bool) {
	wordReader := bytes.NewReader(wordStream)
	snapshot, err := word97.OpenSnapshotReaderAt(wordReader, int64(len(wordStream)), f)
	if err != nil || snapshot == nil || snapshot.Core == nil {
		return nil, false
	}
	opened := snapshot.Core
	var dataStream []byte
	if ds, ok := f.Stream("Data"); ok && len(ds) > 0 {
		dataStream = ds
	} else if ds, ok = f.Stream("DataStream"); ok && len(ds) > 0 {
		dataStream = ds
	}
	counts := legacyStoryCountsFromSequential(opened.Metadata.StoryCounts)
	chpx := appendSequentialCHPX(nil, opened.CHPX)
	papx := appendSequentialPAPX(nil, opened.PAPX)
	core := buildLegacyDocStreamCore(
		dataStream,
		counts,
		opened.RawCLX,
		chpx,
		papx,
		legacyListTablesFromCanonical(opened.Numbering),
		opened.Layout.Version,
	)
	core.DocSettings = legacyDocSettingsFromSequential(opened.Settings)
	core.Compatibility = opened.Settings.Compatibility
	core.FontFamilies = opened.FontFamilies
	core.EmbeddedFonts = opened.EmbeddedFonts
	core.RevisionAuthors = opened.RevisionAuthors
	core.RawDOP = opened.RawDOP
	for _, entry := range chpx {
		core.Diagnostics = append(core.Diagnostics, entry.Diagnostics...)
	}
	for _, entry := range papx {
		core.Diagnostics = append(core.Diagnostics, entry.Meta.Diagnostics...)
	}
	core.StyleSheet = legacyStyleSheetFromCatalog(opened.Styles)
	applySequentialMetadata(core, opened.Metadata)
	applySequentialContext(core, snapshot.Context)
	mergeLegacyStreamPreservationMeta(f, core)
	return core, true
}

func buildLegacyDocStreamCore(
	dataStream []byte,
	counts legacyStoryCounts,
	clxData []byte,
	chpxEntries []chpxEntry,
	papxEntries []papxEntry,
	listTables *legacyListTables,
	nFib uint16,
) *legacyDocStreamCore {
	return &legacyDocStreamCore{
		DataStream:  dataStream,
		CcpText:     counts.Text,
		ClxData:     clxData,
		ChpxEntries: chpxEntries,
		PapxEntries: papxEntries,
		ListTables:  listTables,
		Version:     nFib,
	}
}

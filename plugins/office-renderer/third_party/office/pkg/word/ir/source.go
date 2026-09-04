package ir

// ParagraphSource preserves the raw binary-level provenance for a paragraph
// so that the IR remains lossless. Projection layers use this to (re)compute
// effective formatting, resolve style inheritance, or round-trip to binary.
type ParagraphSource struct {
	// StartCP and EndCP preserve the paragraph character-position span in the main story.
	StartCP int
	EndCP   int

	// PapxSprms stores the raw PAPX grpprl bytes from the FKP or Prc.
	PapxSprms []byte

	// StyleIndex is the raw istd (style index) from the PAPX. nil when the
	// document default style applies.
	StyleIndex *int

	// TableDepth is the nesting depth from sprmPItap / sprmPDtap.
	TableDepth int

	// LegacyInTableHint is set by pre-Word-97 heuristic table detection.
	LegacyInTableHint bool

	// LegacyRowMarkHint is true for table row end marks.
	LegacyRowMarkHint bool

	// PropertyRevision carries raw tracked-change indices for paragraph
	// property revisions (sprmPPropRMark).
	PropertyRevision *SourcePropertyRevision
}

// RunSource preserves the raw binary-level provenance for a run.
type RunSource struct {
	// ChpxSprms stores the raw CHPX grpprl bytes from the FKP or Prc.
	ChpxSprms []byte

	// StyleIndex is the raw character-style istd from sprmCIstd.
	StyleIndex *int

	// Revision is the raw tracked-change state: "inserted", "deleted", or "".
	Revision string

	// Spec is true when the run carries the fSpec flag.
	Spec bool

	// IbstRMark is the raw SttbfRMark index for the revision author.
	IbstRMark    uint16
	HasIbstRMark bool

	// DttmRMark is the raw date-time stamp for the revision.
	DttmRMark    uint32
	HasDttmRMark bool

	// PropertyRevision carries raw indices for run property revisions
	// (sprmCPropRMark).
	PropertyRevision *SourcePropertyRevision

	// RunPropertyRevision carries raw indices for run-format property revisions.
	RunPropertyRevision *SourcePropertyRevision

	// DisplayFieldRevision carries raw indices for display-field revisions
	// (sprmCDispFldRMark).
	DisplayFieldRevision *SourcePropertyRevision
}

// SourcePropertyRevision holds the raw indices for a property revision mark.
type SourcePropertyRevision struct {
	Ibst    uint16
	HasIbst bool
	Dttm    uint32
	HasDttm bool
}

// SectionSource preserves the raw binary-level provenance for a section.
type SectionSource struct {
	// StartCP and EndCP preserve the PlcfSed character-position span for this section.
	StartCP int
	EndCP   int

	// SepxSprms stores the raw SEPX grpprl bytes.
	SepxSprms []byte
}

// StyleSource preserves the raw binary-level provenance for a style.
type StyleSource struct {
	// RawSprms stores the style's own grpprl bytes (paragraph properties for
	// paragraph styles, character properties for character styles).
	RawSprms []byte

	// RunSprms stores character-property grpprl bytes defined on paragraph
	// styles (the "style's run properties").
	RunSprms []byte
}

// DocCompatibility stores document-wide compatibility flags that affect
// layout and rendering. These come from the DOP compatibility flags
// (legacy .doc) or w:settings/w:compat (OOXML).
type DocCompatibility struct {
	// DoNotUseHTMLParagraphAutoSpacing disables the HTML-like auto-spacing
	// between paragraphs. When true, spacing-before/after are applied exactly
	// as specified rather than being collapsed like in HTML.
	DoNotUseHTMLParagraphAutoSpacing bool

	// AlignBordersAndEdges aligns paragraph borders and edges with the page
	// margins rather than the text.
	AlignBordersAndEdges bool

	// LineWrapLikeWord6 uses Word 6.0 line-breaking rules instead of modern
	// Unicode line breaking.
	LineWrapLikeWord6 bool

	// ShapeLayoutLikeWW8 uses Word 97 shape layout and anchoring rules.
	ShapeLayoutLikeWW8 bool

	// FootnoteLayoutLikeWW8 uses Word 6.x/95/97 footnote layout rules.
	FootnoteLayoutLikeWW8 bool

	// TableWrapLikeWW8 uses Word 6.x table text wrapping rules.
	TableWrapLikeWW8 bool

	// NoTabHangInd prevents hanging indentation from being affected by custom
	// tab stops.
	NoTabHangInd bool

	// NoLeading applies extra leading (line spacing) exactly as specified
	// rather than adding Word's default leading.
	NoLeading bool

	// SpaceLikeWW8 uses Word 95 spacing rules for paragraphs.
	SpaceLikeWW8 bool

	// NoColumnBalance prevents columns from being balanced on the last page.
	NoColumnBalance bool

	// DoNotBreakWrappedTables prevents tables from breaking across pages
	// when they have text wrapping.
	DoNotBreakWrappedTables bool

	// DoNotSnapToGridInCell disables snap-to-grid for table cells.
	DoNotSnapToGridInCell bool

	// DoNotAllowFieldEndSelect prevents selection of the field end character.
	DoNotAllowFieldEndSelect bool

	// ApplyBreakingRules applies East Asian breaking rules.
	ApplyBreakingRules bool

	// DoNotBreakConstrainedForcedTables prevents breaking of tables that are
	// constrained to not break across pages.
	DoNotBreakConstrainedForcedTables bool

	// DoNotVertAlignCellWithSp disables vertical alignment of table cells
	// that contain floating objects.
	DoNotVertAlignCellWithSp bool

	// UseWord2002TableRules uses Word 2002 table layout rules.
	UseWord2002TableRules bool

	// GrowAutofit grows autofit tables to fill the available width.
	GrowAutofit bool

	// UseNormalStyleForListNumber applies the Normal style to list numbers.
	UseNormalStyleForListNumber bool

	// DoNotUseIndentAsNumberingTabStop prevents indentation from being used
	// as a tab stop for numbering.
	DoNotUseIndentAsNumberingTabStop bool

	// UseAltKinsokuLineBreakRules uses alternative Japanese kinsoku rules.
	UseAltKinsokuLineBreakRules bool

	// AllowSpaceOfSameStyleInTable allows spacing of the same style in tables.
	AllowSpaceOfSameStyleInTable bool

	// DoNotSuppressIndentation does not suppress indentation for body text.
	DoNotSuppressIndentation bool

	// DoNotAutofitConstrainedTables does not autofit tables that are
	// constrained to a specific width.
	DoNotAutofitConstrainedTables bool

	// AutofitToFirstFixedWidthCell autofits tables to the first fixed-width
	// cell.
	AutofitToFirstFixedWidthCell bool

	// UnderlineTabInNumberingList underlines the tab in numbering lists.
	UnderlineTabInNumberingList bool

	// DisplayHangulFixedWidth uses fixed-width for Hangul characters.
	DisplayHangulFixedWidth bool

	// SplitPgBreakAndParaMark splits page breaks and paragraph marks into
	// separate paragraphs.
	SplitPgBreakAndParaMark bool

	// DoNotVertAlignInTxbx disables vertical alignment in text boxes.
	DoNotVertAlignInTxbx bool

	// UseAnsiKerningPairs uses ANSI kerning pairs.
	UseAnsiKerningPairs bool

	// CachedFieldBalance balances field results across lines.
	CachedFieldBalance bool

	// UICompat97To2003 uses UI compatibility mode for Word 97-2003.
	UICompat97To2003 bool

	// AdjustLineHeightInTable adjusts line height within table cells to match
	// the line height of the surrounding text (w:adjustLineHeightInTable).
	AdjustLineHeightInTable bool

	// AlignTablesRowByRow aligns table rows independently rather than aligning
	// the entire table as a unit (w:alignTablesRowByRow).
	AlignTablesRowByRow bool

	// AllowPNG allows PNG images to be used in the document (w:allowPNG).
	AllowPNG bool

	// AutoSpaceLikeWord95 emulates Word 95 auto-spacing behavior.
	AutoSpaceLikeWord95 bool

	// ConvMailMergeEsc converts mail-merge escape characters.
	ConvMailMergeEsc bool

	// DoNotSuppressParagraphBorders prevents suppression of paragraph borders.
	DoNotSuppressParagraphBorders bool

	// DoNotUseEastAsianBreakRules disables East Asian breaking rules.
	DoNotUseEastAsianBreakRules bool

	// DoNotWrapTextWithPunct prevents text wrapping at punctuation.
	DoNotWrapTextWithPunct bool

	// ForgetLastTabAlignment forgets the last tab alignment.
	ForgetLastTabAlignment bool

	// LayoutRawTableWidth uses raw table width for layout.
	LayoutRawTableWidth bool

	// LayoutTableRowsApart lays out table rows apart from each other.
	LayoutTableRowsApart bool

	// MwSmallCaps emulates WordPerfect small caps behavior.
	MwSmallCaps bool

	// NoExtraLineSpacing disables extra line spacing.
	NoExtraLineSpacing bool

	// NoSpaceRaiseLower prevents space adjustment for raised/lowered text.
	NoSpaceRaiseLower bool

	// PrintBodyTextBeforeHeader prints body text before the header.
	PrintBodyTextBeforeHeader bool

	// PrintColBlack prints color text as black.
	PrintColBlack bool

	// SelectFldWithFirstOrLastChar selects fields with first/last character.
	SelectFldWithFirstOrLastChar bool

	// ShowBreaksInFrames shows breaks in frames.
	ShowBreaksInFrames bool

	// SpacingInWholePoints uses whole points for spacing.
	SpacingInWholePoints bool

	// SubFontBySize substitutes fonts by size.
	SubFontBySize bool

	// SuppressBottomSpacing suppresses spacing at the bottom of pages.
	SuppressBottomSpacing bool

	// SuppressSpBfAfterPgBrk suppresses space before after page breaks.
	SuppressSpBfAfterPgBrk bool

	// SuppressSpacingAtTopOfPage suppresses spacing at the top of pages.
	SuppressSpacingAtTopOfPage bool

	// SuppressTopSpacing suppresses top spacing.
	SuppressTopSpacing bool

	// SuppressTopSpacingWP suppresses top spacing (WordPerfect emulation).
	SuppressTopSpacingWP bool

	// SwapBordersFacingPages swaps borders on facing pages.
	SwapBordersFacingPages bool

	// TruncateFontHeightsLikeWP6 truncates font heights like WordPerfect 6.
	TruncateFontHeightsLikeWP6 bool

	// UseFELayout uses Far East layout rules.
	UseFELayout bool

	// UsePrinterMetrics uses printer metrics for layout.
	UsePrinterMetrics bool

	// UseSingleBorderforContiguousCells uses a single border for contiguous cells.
	UseSingleBorderforContiguousCells bool

	// UseWord97LineBreakRules uses Word 97 line-breaking rules.
	UseWord97LineBreakRules bool

	// WpJustification emulates WordPerfect justification.
	WpJustification bool

	// WpSpaceWidth emulates WordPerfect space width.
	WpSpaceWidth bool

	// WrapTrailSpaces wraps trailing spaces to the next line.
	WrapTrailSpaces bool
}

// DocumentSource preserves document-level raw binary provenance.
type DocumentSource struct {
	// DOP stores the raw Document Properties bytes.
	DOP []byte

	// Version is the nFib value from the FIB.
	Version uint16

	// ComplexFileTable stores the raw Clx bytes (piece table + Prc data).
	ComplexFileTable []byte

	// FontTable stores the raw SttbfFfn bytes.
	FontTable []byte

	// ListTable stores the raw LSTF/LVLF/LFO bytes.
	ListTable []byte

	// StyleSheet stores the raw STSH bytes.
	StyleSheet []byte
}

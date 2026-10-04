package render

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"html"
	"math"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"office/pkg/diag"
	"office/pkg/fontmetrics"
	dml "office/pkg/ooxml/drawing"
	"office/pkg/ooxml/fontdata"
	"office/pkg/ooxml/htmlasset"
	drawingcss "office/pkg/render/drawingcss"
	wordfontmetrics "office/pkg/word/fontmetrics"
	"office/pkg/word/inlineposition"
	resolve "office/pkg/word/ir"
	"office/pkg/word/layout"
	wordtextmetrics "office/pkg/word/textmetrics"
)

type HTMLImageMode string

type HTMLFontMode string

const (
	HTMLImageModeInline   HTMLImageMode = "inline"
	HTMLImageModeExternal HTMLImageMode = "external"

	HTMLFontModeInline   HTMLFontMode = "inline"
	HTMLFontModeExternal HTMLFontMode = "external"
)

type HTMLAsset = htmlasset.Asset

type HTMLOptions struct {
	ImageMode        HTMLImageMode
	ExternalImageDir string
	FontMode         HTMLFontMode
	ExternalFontDir  string
	// FastPreview renders semantic flow without page pagination. Floating objects
	// are kept in that flow so later-page drawings cannot be attached to the
	// first visible page as absolute overlays.
	FastPreview bool
	// ShowFormattingMarks renders normally invisible characters (tab arrows,
	// paragraph marks, page-break indicators) as visible glyphs.
	ShowFormattingMarks bool
	// MergeData provides values for MERGEFIELD substitution during rendering.
	// When a MERGEFIELD name exists as a key, the corresponding value is used
	// instead of the placeholder. If nil or the key is absent, the field falls
	// back to its cached text or the «Name» placeholder.
	MergeData map[string]string
	// MergeRecords provides a multi-record data source for mail-merge fields.
	// When non-empty, MERGEFIELD looks up values in the record at
	// MergeRecordIndex instead of MergeData. MergeData is still used as a
	// fallback when MergeRecords is empty.
	MergeRecords []map[string]string
	// MergeRecordIndex is the zero-based index of the current record within
	// MergeRecords. It is incremented by NEXT fields during rendering.
	MergeRecordIndex int
	// EditableContentControls renders supported SDTs as editable HTML controls
	// instead of static readonly/disabled approximations.
	EditableContentControls bool
	// InterpretPrivateFields parses PRIVATE field instructions and exposes the
	// app name and payload as data-private-app / data-private-data attributes
	// on the field's semantic span. When false (default) PRIVATE fields render
	// only their cached fallback text.
	InterpretPrivateFields bool
	// DatabaseExecutor provides an optional callback for executing DATABASE
	// field queries against external data sources. When nil (default) DATABASE
	// fields fall back to their cached text result.
	DatabaseExecutor func(connectionString, sqlQuery string) (string, error)
	// RenderTime fixes the evaluation time for volatile DATE/TIME fields. When nil,
	// rendering uses the current time, matching Word field refresh behavior. Tests and
	// snapshot callers can provide a stable value.
	RenderTime *time.Time
}

// HTML renders a resolved Word document to a self-contained HTML5 string.
func HTML(doc *resolve.Document) (string, error) {
	html, _, err := HTMLWithOptions(doc, HTMLOptions{})
	return html, err
}

// HTMLWithOptions renders a resolved Word document and returns any sidecar assets.
func HTMLWithOptions(doc *resolve.Document, opts HTMLOptions) (string, []HTMLAsset, error) {
	if doc == nil {
		return "", nil, fmt.Errorf("word render: document is nil")
	}
	if err := opts.validate(); err != nil {
		return "", nil, err
	}
	r := &htmlRenderer{
		doc:                 doc,
		zIndexMap:           buildZIndexMap(doc),
		embeddedFamilies:    buildEmbeddedFamilyIndex(doc),
		fontMetrics:         buildWordFontMetricsCatalog(doc),
		options:             opts.normalized(),
		seqCounters:         make(map[string]int),
		currentMergeRecord:  opts.MergeRecordIndex,
		mergeSeqCount:       opts.MergeRecordIndex,
		showFormattingMarks: opts.ShowFormattingMarks,
		openFieldIndex:      -1,
	}
	return r.render()
}

// buildZIndexMap collects all non-zero raw relativeHeight values from floating
// objects across the full document and maps each unique value to a small
// sequential integer starting at 1, preserving relative order.
// This converts opaque EMU-scale integers like 251633152 into tidy 1, 2, 3…
func buildZIndexMap(doc *resolve.Document) map[int64]int {
	seen := map[int64]struct{}{}
	var collectBlocks func(blocks []resolve.Block)
	collectBlocks = func(blocks []resolve.Block) {
		for _, b := range blocks {
			switch b.Kind {
			case "paragraph":
				if b.Paragraph == nil {
					continue
				}
				for _, run := range b.Paragraph.Runs {
					if run.Image != nil && run.Image.ZIndex > 0 {
						seen[run.Image.ZIndex] = struct{}{}
					}
					if run.TextBox != nil {
						if run.TextBox.ZIndex > 0 {
							seen[run.TextBox.ZIndex] = struct{}{}
						}
						collectBlocks(run.TextBox.Blocks)
					}
					if run.Shape != nil && run.Shape.ZIndex > 0 {
						seen[run.Shape.ZIndex] = struct{}{}
					}
				}
			case "table":
				if b.Table == nil {
					continue
				}
				for _, row := range b.Table.Rows {
					for _, cell := range row.Cells {
						collectBlocks(cell.Blocks)
					}
				}
			}
		}
	}
	collectBlocks(doc.Blocks)
	for _, story := range doc.Headers {
		collectBlocks(story.Blocks)
	}
	for _, story := range doc.Footers {
		collectBlocks(story.Blocks)
	}

	sorted := make([]int64, 0, len(seen))
	for v := range seen {
		sorted = append(sorted, v)
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	m := make(map[int64]int, len(sorted))
	for i, v := range sorted {
		m[v] = i + 1 // 1-based; 0 is reserved for behindDoc objects
	}
	return m
}

// normalizeZIndex converts a raw relativeHeight (EMU integer) to its small
// sequential CSS z-index value. Returns 0 when zRaw is 0 (unset).
func (r *htmlRenderer) normalizeZIndex(zRaw int64) int {
	if zRaw == 0 {
		return 0
	}
	if v, ok := r.zIndexMap[zRaw]; ok {
		return v
	}
	return 1
}

// HTMLDebug renders like HTML but tints each paragraph's background by its
// resolved style ID so style distribution is immediately visible in a browser.
func HTMLDebug(doc *resolve.Document) (string, error) {
	r := &htmlRenderer{doc: doc, zIndexMap: buildZIndexMap(doc), embeddedFamilies: buildEmbeddedFamilyIndex(doc), debugStyleColors: true}
	html, _, err := r.render()
	return html, err
}

// headingPara tracks a paragraph that acts as a heading for SEQ \s restart.
type headingPara struct {
	index int // paragraph.Index
	level int // heading level 1–6
}

type htmlRenderer struct {
	pageTopFloatAnchor *resolve.Paragraph
	pageTopFloatImages []*resolve.Image
	renderedTopFloats  map[*resolve.Image]struct{}
	doc                *resolve.Document
	embeddedFamilies   map[string]struct{}
	fontMetrics        *fontmetrics.Catalog
	options            HTMLOptions
	assets             []HTMLAsset
	nextImageIndex     int
	nextFontIndex      int
	nextWordArtIndex   int
	buf                bytes.Buffer
	overlay            bytes.Buffer
	behindOverlay      bytes.Buffer
	listStack          []listFrame
	currentParagraph   *resolve.Paragraph
	// physicalInlineListMarker is true while a layout-owned paragraph fragment
	// renders its marker inside the first physical line. This keeps centered
	// Word bullets attached to their text instead of forcing every marker into
	// one page-wide gutter column.
	physicalInlineListMarker bool
	pageIndex                int
	sectionIndex             int
	pageOpen                 bool
	pendingBreak             bool
	pageWidthPt              float64
	pageHeightPt             float64
	marginTopPt              float64
	marginLeftPt             float64
	marginRightPt            float64
	marginBottomPt           float64
	columnCount              int
	columnGapPt              float64
	columnWidthsPt           []float64
	columnGapsPt             []float64
	columnSep                bool
	physicalColumnPage       bool
	mixedColumnPage          bool
	emittedPages             int
	basePageWidthPt          float64
	basePageHeightPt         float64
	baseMarginTopPt          float64
	baseMarginLeftPt         float64
	baseMarginRightPt        float64
	baseMarginBottomPt       float64
	baseColumnCount          int
	baseColumnGapPt          float64
	baseColumnWidthsPt       []float64
	baseColumnGapsPt         []float64
	baseColumnSep            bool
	sectionTextDirection     string // "tbRl", "btLr", or "" (default lrTb)
	sectionBidi              bool   // section-level RTL layout (w:bidi)
	sectionVerticalAlign     string // "top", "center", "both", "bottom", or "" (default top)
	pageBorders              *resolve.PageBorders
	docGrid                  *resolve.DocGrid // section document grid (for snap-to-grid)
	zIndexMap                map[int64]int    // raw relativeHeight → normalized sequential z-index
	// Tab rendering state (reset per paragraph).
	tabStops      []resolve.TabStop
	tabDefault    int // default tab interval in twips
	tabIndex      int
	tabEstimatedX float64 // estimated current inline position in points
	// ADVANCE offset state (reset per paragraph).
	advanceState        inlineposition.AdvanceState
	debugStyleColors    bool // set by HTMLDebug; tints paragraphs by resolved style ID
	pages               []layout.Page
	showFormattingMarks bool // render visible glyphs for tabs, paragraph marks, etc.
	// frameDepth tracks nested rendering contexts (table cells, text boxes,
	// headers, footers).  During nesting (depth > 0) frame paragraphs render
	// inline with relative positioning instead of to the page overlay.
	frameDepth int
	// openFieldIndex tracks the StoryField whose container is currently open.
	// -1 means no field container is open.
	openFieldIndex int
	sdtBlockTags   []string
	sdtInlineTags  []string
	textAnimations map[string]bool
	// Layout-backed position maps (populated once in render()).
	// All coordinates are page-absolute (from page top-left) in points.
	floatByImage         map[*resolve.Image]layout.FloatBox
	floatByTextBox       map[*resolve.TextBox]layout.FloatBox
	floatByShape         map[*resolve.Shape]layout.FloatBox
	floatByLegacyShapeID map[uint32]layout.FloatBox
	floatByFramePara     map[*resolve.Paragraph]layout.FloatBox
	floatByTable         map[*resolve.Table]layout.FloatBox
	paraBoxes            map[int]layout.BlockBox
	paraHeights          map[int]float64 // paragraph.Index → computed height in points
	bookmarkPages        map[string]int
	bookmarkText         map[string]string
	bookmarkNoteRefs     map[string]noteFieldTarget
	bookmarkParaIndex    map[string]int // bookmark name → paragraph.Index
	bookmarkScopes       map[string]bookmarkScope
	footnoteNumbers      map[int]string
	endnoteNumbers       map[int]string
	sectionPageCounts    map[int]int
	seqCounters          map[string]int
	autonumCounter       int
	seqRunValues         map[seqRunKey]int
	seqValuesByName      map[string][]seqPositionValue
	headingParagraphs    []headingPara
	currentMergeRecord   int
	mergeSeqCount        int
	paragraphNumbers     map[int]string // paragraph.Index → full context number (e.g. "1.2.3")
	paragraphLevelNum    map[int]string // paragraph.Index → current level number only (e.g. "3")
	paragraphSeq         []*resolve.Paragraph
	paragraphSeqByIdx    map[int]int // paragraph.Index → position in paragraphSeq
	// Line numbering state (from current section's LineNumbering setting).
	lineNumbering     *resolve.LineNumbering // nil when disabled
	lineNumber        int                    // current line number (1-based)
	pendingNoteTarget *resolve.Block
	// noteBodyKind is "footnote" or "endnote" while rendering the body story of
	// a materialized note. Note-reference runs inside note bodies print as plain
	// superscript marks in Word; the body's own mark must not emit a second
	// self-referencing backlink or duplicate the body's ref anchor id.
	noteBodyKind string
}

// listFrame tracks a currently open list element.
type listFrame struct {
	tag    string // "ul" or "ol"
	numID  int    // OOXML numId; used to detect list boundaries at the same depth
	liOpen bool   // nested lists must remain inside their parent <li>
}

type noteFieldTarget struct {
	kind   string
	noteID int
}

type bookmarkScope struct {
	startSeq  int
	endSeq    int
	startPage int
	endPage   int
}

type seqRunKey struct {
	paraIdx int
	runIdx  int
}

type seqPositionValue struct {
	position int
	value    int
}

func (r *htmlRenderer) render() (string, []HTMLAsset, error) {
	// Build paragraph adjacency so bookmark anchors can be emitted only at span
	// starts and span-boundary markers can be emitted at starts/ends.
	r.initParagraphSequence()
	r.initSequenceValues()
	r.initParagraphNumbers()
	r.initBookmarkText()
	r.initNoteFieldState()
	// Pre-compute page-absolute float positions and paragraph heights from the
	// layout layer. These are used by renderImageTo / renderTextBoxTo /
	// renderShapeTo to place all floating objects in the page overlay at
	// accurate coordinates instead of relying on CSS position:relative on
	// anchor paragraphs.
	if r.options.FastPreview {
		r.initFastPreviewLayout()
	} else {
		r.initLayoutMaps()
	}
	r.initBookmarkScopes()
	r.textAnimations = docCollectTextAnimations(r.doc)
	if len(r.pages) > 0 {
		r.applyGeometry(r.pages[0].Geometry)
	}
	r.captureBaseSectionState()
	r.buf.WriteString("<!DOCTYPE html>\n")
	r.buf.WriteString("<html lang=\"en\">\n")
	r.buf.WriteString("<head>\n")
	r.buf.WriteString("<meta charset=\"utf-8\">\n")
	r.buf.WriteString("<title>Document</title>\n")
	r.renderDocumentMeta()
	r.buf.WriteString("<style>\n")
	r.writeCSS()
	r.buf.WriteString("</style>\n")
	r.buf.WriteString("</head>\n")
	r.buf.WriteString("<body>\n")
	r.buf.WriteString("<div class=\"document\">\n")
	r.renderDiagnostics()
	r.renderProtectionNotice()
	if r.options.FastPreview {
		r.ensurePage()
		r.renderBlocks(r.doc.Blocks)
		r.finishPage()
	} else if len(r.pages) == 0 {
		r.ensurePage()
		r.finishPage()
	} else {
		for i := range r.pages {
			if i > 0 {
				r.pendingBreak = true
			}
			page := r.pages[i]
			r.pageIndex = i
			r.sectionIndex = page.SectionIndex
			r.physicalColumnPage = r.doc.Source != nil && pageHasPhysicalColumns(page)
			r.mixedColumnPage = pageHasMixedColumnGeometry(page)
			r.applyGeometry(page.Geometry)
			r.columnSep = page.Section.Columns != nil && page.Section.Columns.Sep
			r.applyLineNumbering(page.Section.LineNumbering)
			r.sectionBidi = page.Section.Bidi
			r.sectionTextDirection = page.Section.TextDirection
			r.sectionVerticalAlign = page.Section.VerticalAlign
			r.pageBorders = page.Section.PageBorders
			r.docGrid = page.Section.DocGrid
			r.ensurePage()
			r.renderPageStories(page)
			r.renderPageBlocks(page)
			r.finishPage()
		}
	}
	r.renderNotes()
	if documentEndsWithExplicitPageBreak(r.doc) {
		if r.showFormattingMarks {
			r.buf.WriteString(`<div class="fmt-mark page-break-label" aria-hidden="true">——— Page Break ———</div>` + "\n")
		}
		r.buf.WriteString("<hr class=\"page-break\">\n")
	}
	r.buf.WriteString("</div>\n")
	r.buf.WriteString("</body>\n")
	r.buf.WriteString("</html>\n")
	return r.buf.String(), r.assetsCopy(), nil
}

type diagnosticGroup struct {
	diagnostic diag.Diagnostic
	count      int
}

func groupedDiagnostics(diagnostics diag.List) []diagnosticGroup {
	if len(diagnostics) == 0 {
		return nil
	}
	type key struct {
		severity diag.Severity
		code     string
		message  string
		part     string
	}
	groups := make([]diagnosticGroup, 0, len(diagnostics))
	index := make(map[key]int, len(diagnostics))
	for _, diagnostic := range diagnostics {
		part := diagnostic.Location.NormalizedPart()
		k := key{diagnostic.Severity, diagnostic.Code, diagnostic.Message, part}
		if groupIndex, ok := index[k]; ok {
			groups[groupIndex].count++
			continue
		}
		diagnostic.Location.Part = part
		groups = append(groups, diagnosticGroup{diagnostic: diagnostic, count: 1})
		index[k] = len(groups) - 1
	}
	return groups
}

func (r *htmlRenderer) renderDiagnostics() {
	if r == nil || len(r.doc.Diagnostics) == 0 {
		return
	}
	r.buf.WriteString("<details class=\"diagnostics\">\n")
	r.buf.WriteString("<summary>Unsupported Features</summary>\n")
	r.buf.WriteString("<ul>\n")
	for _, group := range groupedDiagnostics(r.doc.Diagnostics) {
		diagnostic := group.diagnostic
		severityClass := diagnosticSeverityClass(diagnostic.Severity)
		message := html.EscapeString(diagnostic.Message)
		if diagnostic.Code != "" {
			message = "[" + html.EscapeString(diagnostic.Code) + "] " + message
		}
		if diagnostic.Location.Part != "" {
			message += " (" + html.EscapeString(diagnostic.Location.Part) + ")"
		}
		if group.count > 1 {
			message += fmt.Sprintf(" <span class=\"diagnostic-count\">×%d</span>", group.count)
		}
		fmt.Fprintf(&r.buf, "<li class=%q>%s</li>\n", severityClass, message)
	}
	r.buf.WriteString("</ul>\n")
	r.buf.WriteString("</details>\n")
}

func (r *htmlRenderer) renderProtectionNotice() {
	if r.doc == nil || !r.doc.ProtectionEnabled || r.doc.DocumentProtection == nil {
		return
	}
	edit := r.doc.DocumentProtection.Edit
	if edit == "" {
		edit = "restricted"
	}
	r.buf.WriteString(`<div class="protection-banner">` + "\n")
	fmt.Fprintf(&r.buf, "This document is protected (%s editing).</div>\n", html.EscapeString(edit))
}

func diagnosticSeverityClass(severity diag.Severity) string {
	switch severity {
	case diag.SeverityError:
		return "diagnostic-error"
	case diag.SeverityInfo:
		return "diagnostic-info"
	default:
		return "diagnostic-warning"
	}
}

func (r *htmlRenderer) writeCSS() {
	defaults := r.documentDefaultCSS()
	r.writeEmbeddedFontCSS()
	pageBackground := "white"
	// DisplayBackgroundShape nil = show background when present; explicit false = suppress.
	showBg := r.doc.DisplayBackgroundShape == nil || *r.doc.DisplayBackgroundShape
	if showBg {
		if r.doc.BackgroundColor != "" {
			pageBackground = "#" + r.doc.BackgroundColor
		}
		if r.doc.BackgroundImage != nil && len(r.doc.BackgroundImage.Data) > 0 {
			pageBackground = fmt.Sprintf("%s url(%q) center center / cover no-repeat", pageBackground, r.imageSource(r.doc.BackgroundImage))
		}
	}
	fmt.Fprintf(&r.buf,
		`* { box-sizing: border-box; margin: 0; padding: 0; }
body { background: #e8e8e8; font-family: %s; font-size: %.1fpt; color: %s; }
.document { width: %.1fpt; margin: 0 auto; }
.page { position: relative; isolation: isolate; background: %s; width: %.1fpt; height: %.1fpt; min-height: %.1fpt; margin: 20pt auto; overflow: hidden; }
.page-num { font: 11px sans-serif; color: #888; margin: 12pt auto -16pt; user-select: none; }
.page-body { position: relative; z-index: 1; height: %.1fpt; min-height: %.1fpt; padding: %.1fpt %.1fpt %.1fpt %.1fpt; overflow: hidden; }
.page-overlay { position: absolute; z-index: 2; inset: 0; overflow: hidden; pointer-events: none; }
.page-overlay-behind { z-index: 0; }
.word-wrap-proxy { visibility: hidden; pointer-events: none; }
.page-header, .page-footer, .page-footnotes { position: absolute; overflow: hidden; pointer-events: auto; user-select: text; }
/* The footnote separator is laid out as a real block: it carries Word's own
   two-inch rule geometry and reserves the band it occupies. A ::before rule
   here would double it and push the note body off the layout's geometry. */
.story-overlay { position: absolute; inset: 0; overflow: hidden; }
p, h1, h2, h3, h4, h5, h6 { line-height: 1.2; margin: 0; }
/* Larger headings use progressively tighter leading. An explicit Word
   w:spacing/@line setting is emitted inline and deliberately wins. */
h1 { font-size: 2em; font-weight: bold; line-height: 1.10; }
h2 { font-size: 1.5em; font-weight: bold; line-height: 1.15; }
h3 { font-size: 1.17em; font-weight: bold; line-height: 1.20; }
h4 { font-size: 1em; font-weight: bold; }
h5 { font-size: 0.83em; font-weight: bold; }
h6 { font-size: 0.67em; font-weight: bold; }
table { border-collapse: collapse; width: 100%%; margin: 4pt 0; }
td, th { padding: 2pt 4pt; vertical-align: top; }
ul, ol { padding: 0; margin: 0; }
li { line-height: 1.2; }
.word-list-marker { display: inline-block; min-width: 2.2em; margin-left: -2.2em; padding-right: 0.35em; text-align: right; white-space: nowrap; }
li > ul, li > ol { padding-top: var(--word-list-item-flow-height, 0pt); }
.word-physical-list-marker { position: absolute; right: calc(100%% + 3pt); left: auto; top: 0; width: max-content; min-width: 0; margin-left: 0; padding-right: 0; }
.word-physical-inline-list-marker { display: inline-block; min-width: 0; margin: 0 3pt 0 0; padding: 0; white-space: nowrap; vertical-align: baseline; }
a { color: inherit; text-decoration: inherit; }
.floating-textbox { position: absolute; z-index: 2; overflow: visible; line-height: 1.2; }
.floating-textbox p, .floating-textbox h1, .floating-textbox h2, .floating-textbox h3, .floating-textbox h4, .floating-textbox h5, .floating-textbox h6 { margin: 0; line-height: inherit; }
.floating-textbox li { line-height: inherit; }
.floating-textbox ul, .floating-textbox ol { padding: 0; margin: 0; }
.shape-line { position: absolute; z-index: 1; border-top-style: solid; }
.shape-line-v { position: absolute; z-index: 1; border-left-style: solid; }
.comment-ref { cursor: help; position:absolute; width:1px; height:1px; padding:0; margin:-1px; overflow:hidden; clip:rect(0,0,0,0); white-space:nowrap; border:0; }
.footnote-ref, .endnote-ref { text-decoration: none; }
.note-target { display: inline; }
.note-backlink { text-decoration: none; margin-right: 0.35em; font-size: 0.85em; }
.word-comment-range-start, .word-comment-range-end { display: none; }
@media print { .diagnostics, .note-backlink, .comment-ref, aside.notes .comments, .page-num { display:none !important; } }
.comment-meta { font-size: 0.85em; color: #666; margin-right: 0.4em; }
.comment-author { font-weight: bold; margin-right: 0.3em; }
.comment-date { font-style: italic; }
.ole-object { display: inline-block; padding: 1pt 4pt; background: #f0f0f0; border: 1px solid #bbb; border-radius: 2pt; font-style: italic; color: #555; cursor: default; }
.ole-object-preview { padding: 0; background: transparent; border: 0; border-radius: 0; font-style: normal; color: inherit; line-height: 0; }
.ole-object-preview img { display: block; max-width: 100%%; }
.page-break { border: none; border-top: 1px dashed #aaa; margin: 8pt 0; }
ins { text-decoration: underline; color: #22863a; background-color: #f0fff4; }
del { text-decoration: line-through; color: #cb2431; background-color: #ffeef0; }
ins[data-revision-type="movedTo"] { text-decoration: underline; color: #22863a; background-color: #ddf4ff; }
del[data-revision-type="movedFrom"] { text-decoration: line-through; color: #cb2431; background-color: #fff5e6; }
.word-field { font-style: inherit; color: inherit; }
.word-field-toc { display: inline-block; min-width: 100%%; white-space: pre-line; font-style: normal; }
.word-field-index { display: inline-block; min-width: 100%%; white-space: pre-line; font-style: normal; }
.word-field-xe, .word-field-tc { display: none; }
.word-bookmark-span-start, .word-bookmark-span-end { display: none; }
.word-move-from-range-start, .word-move-from-range-end, .word-move-to-range-start, .word-move-to-range-end,
.word-perm-range-start, .word-perm-range-end,
.word-custom-xml-ins-range-start, .word-custom-xml-ins-range-end,
.word-custom-xml-del-range-start, .word-custom-xml-del-range-end,
.word-custom-xml-move-from-range-start, .word-custom-xml-move-from-range-end,
.word-custom-xml-move-to-range-start, .word-custom-xml-move-to-range-end { display: none; }
.word-sdt { display: contents; }
.word-sdt-placeholder { color: #aaa; font-style: italic; }
.word-sdt-checkbox { vertical-align: middle; }
.word-sdt-text { border: none; background: transparent; font-family: inherit; font-size: inherit; color: inherit; }
.word-sdt-rich { background: transparent; font-family: inherit; font-size: inherit; color: inherit; }
.ln { position: absolute; left: -3em; min-width: 2.5em; text-align: right; color: #888; font-size: 0.85em; user-select: none; pointer-events: none; }
.fmt-mark { color: #888; font-size: 0.85em; user-select: none; pointer-events: none; margin-left: 0.2em; }
.page-break-label { text-align: center; margin: 4pt 0; }
header.doc-header, footer.doc-footer { width: %.1fpt; margin: 0 auto; padding: 4pt %.1fpt; font-size: 0.85em; color: #555; }
`,
		defaults.FontFamily,
		defaults.FontSizePt,
		defaults.Color,
		r.pageWidthPt,
		pageBackground,
		r.pageWidthPt,
		r.pageHeightPt,
		r.pageHeightPt,
		r.pageHeightPt,
		r.pageHeightPt,
		r.marginTopPt,
		r.marginRightPt,
		r.marginBottomPt,
		r.marginLeftPt,
		r.pageWidthPt,
		r.marginLeftPt,
	)
	if len(r.doc.Diagnostics) > 0 {
		fmt.Fprintf(&r.buf, ".diagnostics { width: %.1fpt; margin: 12pt auto 0; padding: 8pt 12pt; background: #fffbe6; border: 1px solid #d9c87c; border-radius: 4pt; }\n", r.pageWidthPt)
		r.buf.WriteString(".diagnostics summary { cursor: pointer; font-weight: bold; }\n")
		r.buf.WriteString(".diagnostics ul { margin: 8pt 0 0 18pt; padding: 0; }\n")
		r.buf.WriteString(".diagnostics li { margin: 4pt 0; }\n")
		r.buf.WriteString(".diagnostic-count { opacity: .65; white-space: nowrap; }\n")
		r.buf.WriteString(".diagnostic-error { color: #a94442; }\n")
		r.buf.WriteString(".diagnostic-warning { color: #8a6d3b; }\n")
		r.buf.WriteString(".diagnostic-info { color: #31708f; }\n")
	}
	if r.doc.ProtectionEnabled && r.doc.DocumentProtection != nil {
		fmt.Fprintf(&r.buf, `.protection-banner { width: %.1fpt; margin: 8pt auto; padding: 6pt 10pt; background: #fff3cd; border: 1px solid #ffc107; border-radius: 3pt; color: #856404; font-size: 0.9em; text-align: center; }
`, r.pageWidthPt)
	}
	if r.textAnimations["blinkBackground"] {
		r.buf.WriteString(`@keyframes word-blink { 50% { opacity: 0; } }
`)
	}
	if r.textAnimations["lights"] {
		r.buf.WriteString(`@keyframes word-lights { from { text-shadow: 0 0 4px rgba(255,255,200,0.3); } to { text-shadow: 0 0 12px rgba(255,255,0,0.9); } }
`)
	}
	if r.textAnimations["antsBlack"] {
		r.buf.WriteString(`@keyframes word-ants-black { 0% { outline-offset:0; } 100% { outline-offset:4px; } }
`)
	}
	if r.textAnimations["antsRed"] {
		r.buf.WriteString(`@keyframes word-ants-red { 0% { outline-offset:0; } 100% { outline-offset:4px; } }
`)
	}
	if r.textAnimations["shimmer"] {
		r.buf.WriteString(`@keyframes word-shimmer { from { text-shadow: 0 0 2px currentColor; } to { text-shadow: 0 0 10px currentColor, 0 0 20px currentColor; } }
`)
	}
	if r.textAnimations["sparkle"] {
		r.buf.WriteString(`@keyframes word-sparkle { 0%,100% { opacity:1; } 50% { opacity:0.6; text-shadow: 0 0 8px gold, 0 0 16px orange; } }
`)
	}
	if r.doc.AutoHyphenation {
		r.buf.WriteString(".page-body { hyphens: auto")
		if r.doc.ConsecutiveHyphenLimit > 0 {
			fmt.Fprintf(&r.buf, "; -webkit-hyphenate-limit-lines: %d; hyphenate-limit-lines: %d", r.doc.ConsecutiveHyphenLimit, r.doc.ConsecutiveHyphenLimit)
		}
		if r.doc.HyphenationZone > 0 {
			fmt.Fprintf(&r.buf, "; -webkit-hyphenate-limit-zone: %.1fpt; hyphenate-limit-zone: %.1fpt", twipsToPoints(r.doc.HyphenationZone), twipsToPoints(r.doc.HyphenationZone))
		}
		r.buf.WriteString(" }\n")
	}
}

type documentDefaultCSS struct {
	FontFamily string
	FontSizePt float64
	Color      string
}

func (r *htmlRenderer) documentDefaultCSS() documentDefaultCSS {
	defaults := documentDefaultCSS{
		FontFamily: `"Times New Roman", Times, serif`,
		FontSizePt: 12.0,
		Color:      "#000",
	}
	if r == nil || r.doc == nil {
		return defaults
	}
	format := r.doc.DefaultRunFormat
	if format.FontSize != nil && *format.FontSize > 0 {
		defaults.FontSizePt = float64(*format.FontSize) / 2.0
	}
	if fontFamily := effectiveFontFamily(format); fontFamily != "" {
		defaults.FontFamily = r.cssFontFamily(fontFamily, !r.isEmbeddedFont(fontFamily))
	}
	if color := cssHexColor(format.Color); color != "" {
		defaults.Color = color
	}
	return defaults
}

func cssHexColor(color string) string {
	color = strings.TrimSpace(color)
	if color == "" || strings.EqualFold(color, "auto") {
		return ""
	}
	if strings.HasPrefix(color, "#") {
		return color
	}
	return "#" + color
}

func docCollectTextAnimations(doc *resolve.Document) map[string]bool {
	result := make(map[string]bool)
	if doc == nil {
		return result
	}
	for _, block := range doc.Blocks {
		if block.Kind != "paragraph" || block.Paragraph == nil {
			continue
		}
		for _, run := range block.Paragraph.Runs {
			if run.Format.TextAnimation != "" {
				result[run.Format.TextAnimation] = true
			}
		}
	}
	return result
}

func (opts HTMLOptions) normalized() HTMLOptions {
	imageMode := opts.ImageMode
	if imageMode == "" {
		imageMode = HTMLImageModeInline
	}
	fontMode := opts.FontMode
	if fontMode == "" {
		fontMode = HTMLFontModeInline
	}
	imageDir := htmlasset.NormalizeDir(opts.ExternalImageDir)
	if imageMode == HTMLImageModeExternal && imageDir == "" {
		imageDir = "images"
	}
	fontDir := htmlasset.NormalizeDir(opts.ExternalFontDir)
	if fontMode == HTMLFontModeExternal && fontDir == "" {
		fontDir = "fonts"
	}
	return HTMLOptions{
		ImageMode:               imageMode,
		ExternalImageDir:        imageDir,
		FontMode:                fontMode,
		ExternalFontDir:         fontDir,
		FastPreview:             opts.FastPreview,
		ShowFormattingMarks:     opts.ShowFormattingMarks,
		MergeData:               opts.MergeData,
		MergeRecords:            opts.MergeRecords,
		MergeRecordIndex:        opts.MergeRecordIndex,
		EditableContentControls: opts.EditableContentControls,
		InterpretPrivateFields:  opts.InterpretPrivateFields,
		DatabaseExecutor:        opts.DatabaseExecutor,
		RenderTime:              opts.RenderTime,
	}
}

func (opts HTMLOptions) validate() error {
	normalized := opts.normalized()
	switch normalized.ImageMode {
	case HTMLImageModeInline, HTMLImageModeExternal:
	default:
		return fmt.Errorf("word render: unsupported html image mode %q", normalized.ImageMode)
	}
	switch normalized.FontMode {
	case HTMLFontModeInline, HTMLFontModeExternal:
		return nil
	default:
		return fmt.Errorf("word render: unsupported html font mode %q", normalized.FontMode)
	}
}

func (r *htmlRenderer) assetsCopy() []HTMLAsset {
	if r == nil || len(r.assets) == 0 {
		return nil
	}
	assets := make([]HTMLAsset, len(r.assets))
	copy(assets, r.assets)
	return assets
}

type htmlSectionSpan struct {
	Section resolve.Section
	Blocks  []resolve.Block
}

func htmlSectionSpans(doc *resolve.Document) []htmlSectionSpan {
	if doc == nil {
		return nil
	}
	if len(doc.Blocks) == 0 {
		if len(doc.Sections) == 0 {
			return nil
		}
		return []htmlSectionSpan{{Section: doc.Sections[0]}}
	}
	spans := make([]htmlSectionSpan, 0, len(doc.Sections))
	current := make([]resolve.Block, 0, len(doc.Blocks))
	sectionIndex := 0
	flush := func(section *resolve.Section) {
		resolved := resolve.Section{}
		if section != nil {
			resolved = *section
		} else if sectionIndex < len(doc.Sections) {
			resolved = doc.Sections[sectionIndex]
		} else if len(doc.Sections) > 0 {
			resolved = doc.Sections[len(doc.Sections)-1]
		}
		spans = append(spans, htmlSectionSpan{Section: resolved, Blocks: append([]resolve.Block(nil), current...)})
		current = current[:0]
		sectionIndex++
	}
	for _, block := range doc.Blocks {
		if block.Kind == "section" {
			flush(block.Section)
			continue
		}
		current = append(current, block)
	}
	if len(current) > 0 || len(spans) == 0 {
		resolved := resolve.Section{}
		if sectionIndex < len(doc.Sections) {
			resolved = doc.Sections[sectionIndex]
		} else if len(doc.Sections) > 0 {
			resolved = doc.Sections[len(doc.Sections)-1]
		}
		spans = append(spans, htmlSectionSpan{Section: resolved, Blocks: append([]resolve.Block(nil), current...)})
	}
	return spans
}

func firstHTMLSection(spans []htmlSectionSpan) *resolve.Section {
	if len(spans) == 0 {
		return nil
	}
	return &spans[0].Section
}

func lastHTMLSection(spans []htmlSectionSpan) *resolve.Section {
	if len(spans) == 0 {
		return nil
	}
	return &spans[len(spans)-1].Section
}

func (r *htmlRenderer) captureBaseSectionState() {
	r.basePageWidthPt = r.pageWidthPt
	r.basePageHeightPt = r.pageHeightPt
	r.baseMarginTopPt = r.marginTopPt
	r.baseMarginLeftPt = r.marginLeftPt
	r.baseMarginRightPt = r.marginRightPt
	r.baseMarginBottomPt = r.marginBottomPt
	r.baseColumnCount = r.columnCount
	r.baseColumnGapPt = r.columnGapPt
	r.baseColumnWidthsPt = append([]float64(nil), r.columnWidthsPt...)
	r.baseColumnGapsPt = append([]float64(nil), r.columnGapsPt...)
	r.baseColumnSep = r.columnSep
}

// applyLineNumbering updates the renderer's line numbering state for a new page/section.
// Per the OOXML spec, line numbering restarts on each new page (default "newPage") unless
// configured as "continuous" or "newSection".
func (r *htmlRenderer) applyLineNumbering(ln *resolve.LineNumbering) {
	if ln == nil {
		r.lineNumbering = nil
		r.lineNumber = 0
		return
	}
	// If numbering is new (section changed or first page), reset counter.
	if r.lineNumbering == nil || r.lineNumbering.Restart != "continuous" {
		r.lineNumber = ln.Start
	}
	r.lineNumbering = ln
}

func (r *htmlRenderer) withLineNumberingSuppressed(fn func()) {
	if fn == nil {
		return
	}
	savedLineNumbering := r.lineNumbering
	savedLineNumber := r.lineNumber
	r.lineNumbering = nil
	fn()
	r.lineNumbering = savedLineNumbering
	r.lineNumber = savedLineNumber
}

func (r *htmlRenderer) ensurePage() {
	if r.pageOpen {
		return
	}
	if r.pendingBreak {
		if r.showFormattingMarks {
			r.buf.WriteString(`<div class="fmt-mark page-break-label" aria-hidden="true">——— Page Break ———</div>` + "\n")
		}
		r.buf.WriteString("<hr class=\"page-break\">\n")
		r.pendingBreak = false
	}
	pageStyle := r.pageStyleAttr()
	r.emittedPages++
	fmt.Fprintf(&r.buf, "<div class=\"page-num\">Page %d</div>\n", r.emittedPages)
	fmt.Fprintf(&r.buf, "<section class=\"page\" id=\"page-%d\" data-page=\"%d\"%s>\n", r.emittedPages, r.emittedPages, pageStyle)
	bodyStyle := r.documentBodyStyleAttr()
	borderStyle := r.pageBorderStyleAttr()
	if borderStyle != "" {
		if bodyStyle != "" {
			bodyStyle = strings.TrimSuffix(bodyStyle, `"`) + ";" + borderStyle + `"`
		} else {
			bodyStyle = fmt.Sprintf(` style=%q`, borderStyle)
		}
	}
	bidiAttr := ""
	if r.sectionBidi {
		bidiAttr = ` dir="rtl"`
	}
	if bodyStyle != "" {
		fmt.Fprintf(&r.buf, "<div class=\"page-body\"%s%s>\n", bodyStyle, bidiAttr)
	} else if bidiAttr != "" {
		fmt.Fprintf(&r.buf, "<div class=\"page-body\"%s>\n", bidiAttr)
	} else {
		r.buf.WriteString("<div class=\"page-body\">\n")
	}
	r.pageOpen = true
}

func (r *htmlRenderer) finishPage() {
	if !r.pageOpen {
		return
	}
	r.closeAllLists()
	r.buf.WriteString("</div>\n")
	if r.behindOverlay.Len() > 0 {
		r.buf.WriteString("<div class=\"page-overlay page-overlay-behind\">\n")
		r.buf.Write(r.behindOverlay.Bytes())
		r.buf.WriteString("</div>\n")
		r.behindOverlay.Reset()
	}
	if r.overlay.Len() > 0 {
		r.buf.WriteString("<div class=\"page-overlay\">\n")
		r.buf.Write(r.overlay.Bytes())
		r.buf.WriteString("</div>\n")
		r.overlay.Reset()
	}
	r.buf.WriteString("</section>\n")
	r.pageOpen = false
}

func (r *htmlRenderer) pageStyleAttr() string {
	if almostSameFloat(r.pageWidthPt, r.basePageWidthPt) &&
		almostSameFloat(r.pageHeightPt, r.basePageHeightPt) {
		return ""
	}
	return fmt.Sprintf(" style=%q", fmt.Sprintf("width:%.1fpt;height:%.1fpt;min-height:%.1fpt", r.pageWidthPt, r.pageHeightPt, r.pageHeightPt))
}

func (r *htmlRenderer) documentBodyStyleAttr() string {
	dirStyles := sectionTextDirectionCSS(r.sectionTextDirection)
	vaStyles := sectionVerticalAlignCSS(r.sectionVerticalAlign)
	if almostSameFloat(r.pageHeightPt, r.basePageHeightPt) &&
		almostSameFloat(r.marginTopPt, r.baseMarginTopPt) &&
		almostSameFloat(r.marginRightPt, r.baseMarginRightPt) &&
		almostSameFloat(r.marginBottomPt, r.baseMarginBottomPt) &&
		almostSameFloat(r.marginLeftPt, r.baseMarginLeftPt) &&
		r.columnCount == r.baseColumnCount &&
		almostSameFloat(r.columnGapPt, r.baseColumnGapPt) &&
		almostSameFloatSlice(r.columnWidthsPt, r.baseColumnWidthsPt) &&
		almostSameFloatSlice(r.columnGapsPt, r.baseColumnGapsPt) &&
		r.columnSep == r.baseColumnSep {
		if r.columnCount <= 1 && len(dirStyles) == 0 && len(vaStyles) == 0 {
			return ""
		}
		var styles []string
		if r.columnCount > 1 && !r.hasExplicitPhysicalColumns() && !r.mixedColumnPage {
			styles = append(styles,
				fmt.Sprintf("column-count:%d", r.columnCount),
				fmt.Sprintf("column-gap:%.1fpt", r.columnGapPt),
			)
			if r.columnSep {
				styles = append(styles, "column-rule:1px solid #bbb")
			}
		}
		styles = append(styles, dirStyles...)
		styles = append(styles, vaStyles...)
		return fmt.Sprintf(" style=%q", strings.Join(styles, ";"))
	}
	styles := []string{
		fmt.Sprintf("height:%.1fpt", r.pageHeightPt),
		fmt.Sprintf("min-height:%.1fpt", r.pageHeightPt),
		fmt.Sprintf("padding:%.1fpt %.1fpt %.1fpt %.1fpt", r.marginTopPt, r.marginRightPt, r.marginBottomPt, r.marginLeftPt),
	}
	if r.columnCount > 1 && !r.hasExplicitPhysicalColumns() && !r.mixedColumnPage {
		styles = append(styles,
			fmt.Sprintf("column-count:%d", r.columnCount),
			fmt.Sprintf("column-gap:%.1fpt", r.columnGapPt),
		)
		if r.columnSep {
			styles = append(styles, "column-rule:1px solid #bbb")
		}
	}
	styles = append(styles, dirStyles...)
	styles = append(styles, vaStyles...)
	return fmt.Sprintf(" style=%q", strings.Join(styles, ";"))
}

func (r *htmlRenderer) hasExplicitPhysicalColumns() bool {
	if r.physicalColumnPage && r.columnCount > 1 {
		return true
	}
	return hasExplicitPhysicalColumns(layout.PageGeometry{
		ColumnCount:     r.columnCount,
		ColumnGapPt:     r.columnGapPt,
		ColumnWidthsPt:  r.columnWidthsPt,
		ExplicitColumns: len(r.columnWidthsPt) >= r.columnCount && r.columnCount > 1,
		ColumnGapsPt:    r.columnGapsPt,
		ColumnSep:       r.columnSep,
	})
}

// sectionVerticalAlignCSS returns CSS flexbox properties for section-level
// vertical page alignment (w:vAlign / sprmSVjc).
func sectionVerticalAlignCSS(align string) []string {
	switch align {
	case "center":
		return []string{"display:flex", "flex-direction:column", "justify-content:center"}
	case "both":
		return []string{"display:flex", "flex-direction:column", "justify-content:space-between"}
	case "bottom":
		return []string{"display:flex", "flex-direction:column", "justify-content:flex-end"}
	default:
		return nil
	}
}

// sectionTextDirectionCSS returns CSS writing-mode/text-orientation properties
// for section-level text direction (w:textDirection / sprmSTextFlow).
func sectionTextDirectionCSS(direction string) []string {
	switch direction {
	case "tbRl", "tbRlV":
		return []string{"writing-mode:vertical-rl", "text-orientation:mixed"}
	case "btLr":
		return []string{"writing-mode:vertical-lr", "text-orientation:mixed"}
	default:
		return nil
	}
}

func almostSameFloat(left, right float64) bool {
	return left-right < 0.001 && right-left < 0.001
}

func maxFloat(left, right float64) float64 {
	if left > right {
		return left
	}
	return right
}

// relativeSizeStyle returns a CSS width/height declaration for an object sized
// relative to page, margin, or column. pct is in thousandths of a percent
// (e.g. 50000 = 50%).
func (r *htmlRenderer) relativeSizeStyle(pct int64, relativeFrom string, isWidth bool) string {
	property := "height"
	if isWidth {
		property = "width"
	}
	if pct <= 0 {
		return ""
	}
	fraction := float64(pct) / 100000.0
	var reference float64
	if isWidth {
		switch strings.ToLower(strings.TrimSpace(relativeFrom)) {
		case "page":
			reference = r.pageWidthPt
		case "leftmargin", "insidemargin":
			reference = r.marginLeftPt
		case "rightmargin", "outsidemargin":
			reference = r.marginRightPt
		case "column":
			if len(r.columnWidthsPt) > 0 && r.columnWidthsPt[0] > 0 {
				reference = r.columnWidthsPt[0]
			} else {
				reference = r.pageWidthPt - r.marginLeftPt - r.marginRightPt
				if r.columnCount > 1 {
					reference = (reference - float64(r.columnCount-1)*r.columnGapPt) / float64(r.columnCount)
				}
			}
		default:
			reference = r.pageWidthPt - r.marginLeftPt - r.marginRightPt
		}
	} else {
		switch strings.ToLower(strings.TrimSpace(relativeFrom)) {
		case "page":
			reference = r.pageHeightPt
		case "topmargin", "insidemargin":
			reference = r.marginTopPt
		case "bottommargin", "outsidemargin":
			reference = r.marginBottomPt
		default:
			reference = r.pageHeightPt - r.marginTopPt - r.marginBottomPt
		}
	}
	if reference < 0 {
		reference = 0
	}
	return fmt.Sprintf("%s:%.1fpt", property, reference*fraction)
}

// shapeOutsidePolygonCSS builds a CSS shape-outside polygon value from a
// wrap polygon. The polygon points are in points relative to the object's
// top-left corner; they are converted to percentages based on the object's
// width and height in EMU so the polygon scales with the rendered element.
func shapeOutsidePolygonCSS(poly [][2]float64, widthEMU, heightEMU int64) string {
	if len(poly) == 0 || widthEMU <= 0 || heightEMU <= 0 {
		return ""
	}
	widthPt := emuToPoints(widthEMU)
	heightPt := emuToPoints(heightEMU)
	if widthPt <= 0 || heightPt <= 0 {
		return ""
	}
	var coords []string
	for _, pt := range poly {
		xPct := pt[0] / widthPt * 100
		yPct := pt[1] / heightPt * 100
		coords = append(coords, fmt.Sprintf("%.2f%% %.2f%%", xPct, yPct))
	}
	return "polygon(" + strings.Join(coords, ", ") + ")"
}

// rotatedProxyGeometry returns the axis-aligned proxy box that encloses an
// object rotated around its center. The proxy is used only for browser text
// wrapping; the visible object keeps Word's original left/top/size and rotates
// around that box. For tight/through wrapping, polygonCSS traces the rotated
// rectangle inside the expanded proxy box.
func rotatedProxyGeometry(x, y, width, height, rotationDeg float64) (proxyX, proxyY, proxyW, proxyH float64, polygonCSS string) {
	proxyX, proxyY, proxyW, proxyH = x, y, width, height
	if width <= 0 || height <= 0 {
		return
	}
	angle := math.Mod(rotationDeg, 360)
	if angle < 0 {
		angle += 360
	}
	if math.Abs(angle) < 1e-9 || math.Abs(angle-360) < 1e-9 {
		return
	}
	rad := angle * math.Pi / 180
	cosA, sinA := math.Cos(rad), math.Sin(rad)
	cx, cy := x+width/2, y+height/2
	points := [][2]float64{{-width / 2, -height / 2}, {width / 2, -height / 2}, {width / 2, height / 2}, {-width / 2, height / 2}}
	left, top := math.Inf(1), math.Inf(1)
	right, bottom := math.Inf(-1), math.Inf(-1)
	for i := range points {
		dx, dy := points[i][0], points[i][1]
		px := cx + dx*cosA - dy*sinA
		py := cy + dx*sinA + dy*cosA
		points[i] = [2]float64{px, py}
		left, right = math.Min(left, px), math.Max(right, px)
		top, bottom = math.Min(top, py), math.Max(bottom, py)
	}
	proxyX, proxyY = left, top
	proxyW, proxyH = right-left, bottom-top
	if proxyW <= 0 || proxyH <= 0 {
		return x, y, width, height, ""
	}
	coords := make([]string, 0, len(points))
	for _, point := range points {
		xPct := (point[0] - proxyX) / proxyW * 100
		yPct := (point[1] - proxyY) / proxyH * 100
		coords = append(coords, fmt.Sprintf("%.2f%% %.2f%%", xPct, yPct))
	}
	polygonCSS = "polygon(" + strings.Join(coords, ", ") + ")"
	return
}

// wrapPolygonProxyCSS projects an authored Word wrap polygon through the same
// resolved size and rotation used by layout. The browser proxy is the rotated
// axis-aligned box, so polygon percentages must be relative to that proxy rather
// than to the unrotated intrinsic image box.
func wrapPolygonProxyCSS(poly [][2]float64, widthEMU, heightEMU int64, renderedW, renderedH, proxyX, proxyY, proxyW, proxyH, objectX, objectY, rotationDeg float64) string {
	if len(poly) == 0 || renderedW <= 0 || renderedH <= 0 || proxyW <= 0 || proxyH <= 0 {
		return ""
	}
	sourceW := emuToPoints(widthEMU)
	sourceH := emuToPoints(heightEMU)
	if sourceW <= 0 || sourceH <= 0 {
		return ""
	}
	sx := renderedW / sourceW
	sy := renderedH / sourceH
	cx, cy := objectX+renderedW/2, objectY+renderedH/2
	angle := math.Mod(rotationDeg, 360)
	if angle < 0 {
		angle += 360
	}
	rad := angle * math.Pi / 180
	cosA, sinA := math.Cos(rad), math.Sin(rad)
	coords := make([]string, 0, len(poly))
	for _, point := range poly {
		x := objectX + point[0]*sx
		y := objectY + point[1]*sy
		if math.Abs(angle) > 1e-9 && math.Abs(angle-360) > 1e-9 {
			dx, dy := x-cx, y-cy
			x = cx + dx*cosA - dy*sinA
			y = cy + dx*sinA + dy*cosA
		}
		xPct := (x - proxyX) / proxyW * 100
		yPct := (y - proxyY) / proxyH * 100
		coords = append(coords, fmt.Sprintf("%.2f%% %.2f%%", xPct, yPct))
	}
	return "polygon(" + strings.Join(coords, ", ") + ")"
}

// initLayoutMaps runs layout.Paginate once and builds lookup tables so that
// all floating objects can be positioned at page-absolute coordinates without
// relying on CSS position:relative on anchor paragraphs.
func (r *htmlRenderer) initParagraphSequence() {
	r.paragraphSeq = r.paragraphSeq[:0]
	r.paragraphSeqByIdx = make(map[int]int)
	r.headingParagraphs = r.headingParagraphs[:0]
	var collectParagraph func(p *resolve.Paragraph)
	collectParagraph = func(p *resolve.Paragraph) {
		if p == nil {
			return
		}
		if _, seen := r.paragraphSeqByIdx[p.Index]; seen {
			return
		}
		r.paragraphSeqByIdx[p.Index] = len(r.paragraphSeq)
		r.paragraphSeq = append(r.paragraphSeq, p)
		if level := headingLevel(p.StyleID, p.StyleName); level > 0 {
			r.headingParagraphs = append(r.headingParagraphs, headingPara{index: p.Index, level: level})
		}
	}
	var collectBlocks func(blocks []resolve.Block)
	collectBlocks = func(blocks []resolve.Block) {
		for _, block := range blocks {
			switch block.Kind {
			case "paragraph":
				collectParagraph(block.Paragraph)
				if block.Paragraph == nil {
					continue
				}
				for _, run := range block.Paragraph.Runs {
					if run.TextBox != nil {
						collectBlocks(run.TextBox.Blocks)
					}
				}
			case "table":
				if block.Table == nil {
					continue
				}
				for _, row := range block.Table.Rows {
					for _, cell := range row.Cells {
						collectBlocks(cell.Blocks)
					}
				}
			}
		}
	}
	if r.doc != nil && len(r.doc.Blocks) > 0 {
		collectBlocks(r.doc.Blocks)
		return
	}
	for _, page := range r.pages {
		for _, box := range page.Blocks {
			if box.Block == nil {
				continue
			}
			collectBlocks([]resolve.Block{*box.Block})
		}
	}
}

func (r *htmlRenderer) initSequenceValues() {
	if r.seqRunValues != nil && r.seqValuesByName != nil {
		return
	}
	if len(r.paragraphSeq) == 0 {
		r.initParagraphSequence()
	}
	r.seqRunValues = make(map[seqRunKey]int)
	r.seqValuesByName = make(map[string][]seqPositionValue)
	seqCounters := make(map[string]int)
	for _, p := range r.paragraphSeq {
		if p == nil {
			continue
		}
		position, ok := r.paragraphSeqByIdx[p.Index]
		if !ok {
			continue
		}
		for runIdx, run := range p.Runs {
			if run.FieldKind != "SEQ" {
				continue
			}
			name := normalizeSequenceName(run.FieldArg)
			if name == "" {
				continue
			}
			control := r.seqControlForRun(run)
			counterKey := name
			if control.hasRestartHeading {
				hIdx := r.nearestPrecedingHeadingIdx(p.Index, control.restartHeading)
				counterKey = fmt.Sprintf("%s#h=%d", name, hIdx)
			}
			value := 0
			if control.hasRestart {
				seqCounters[counterKey] = control.restart
				value = control.restart
			} else if control.repeat {
				value = seqCounters[counterKey]
			} else {
				seqCounters[counterKey]++
				value = seqCounters[counterKey]
			}
			if value <= 0 {
				continue
			}
			r.seqRunValues[seqRunKey{paraIdx: p.Index, runIdx: runIdx}] = value
			r.seqValuesByName[name] = append(r.seqValuesByName[name], seqPositionValue{position: position, value: value})
		}
	}
}

func (r *htmlRenderer) sequenceRunValue(paraIdx, runIdx int) (int, bool) {
	if r.seqRunValues == nil {
		r.initSequenceValues()
	}
	value, ok := r.seqRunValues[seqRunKey{paraIdx: paraIdx, runIdx: runIdx}]
	return value, ok
}

// sequenceTotalCount returns the total number of SEQ fields with the given name.
func (r *htmlRenderer) sequenceTotalCount(name string) int {
	if r.seqValuesByName == nil {
		r.initSequenceValues()
	}
	return len(r.seqValuesByName[normalizeSequenceName(name)])
}

func (r *htmlRenderer) sequenceValueAtParagraph(name string, paraIdx int) (int, bool) {
	if paraIdx < 0 {
		return 0, false
	}
	if r.seqValuesByName == nil {
		r.initSequenceValues()
	}
	position, ok := r.paragraphSeqByIdx[paraIdx]
	if !ok {
		return 0, false
	}
	values := r.seqValuesByName[normalizeSequenceName(name)]
	for i := len(values) - 1; i >= 0; i-- {
		if values[i].position <= position {
			return values[i].value, true
		}
	}
	return 0, false
}

func normalizeCaptionSequenceName(name string) string {
	name = normalizeSequenceName(name)
	if strings.HasSuffix(name, "S") && len(name) > 1 {
		name = strings.TrimSuffix(name, "S")
	}
	return name
}

func captionSequenceMatches(seqName, captionName string) bool {
	return normalizeCaptionSequenceName(seqName) != "" && normalizeCaptionSequenceName(seqName) == normalizeCaptionSequenceName(captionName)
}

// initParagraphNumbers computes list numbering values for every paragraph that
// has w:numPr. It populates paragraphNumbers (full context like "1.2.3") and
// paragraphLevelNum (current level only like "3").
func (r *htmlRenderer) initParagraphNumbers() {
	r.paragraphNumbers = make(map[int]string)
	r.paragraphLevelNum = make(map[int]string)
	counters := make(map[int]map[int]int)     // numID -> level -> counter
	starts := make(map[int]map[int]int)       // numID -> level -> start value
	formats := make(map[int]map[int]string)   // numID -> level -> format
	templates := make(map[int]map[int]string) // numID -> level -> template text

	var scan func(blocks []resolve.Block)
	scan = func(blocks []resolve.Block) {
		for _, block := range blocks {
			switch block.Kind {
			case "paragraph":
				if block.Paragraph == nil {
					continue
				}
				p := block.Paragraph
				if p.Numbering != nil {
					numID := p.Numbering.NumID
					level := p.Numbering.Level
					if counters[numID] == nil {
						counters[numID] = make(map[int]int)
						starts[numID] = make(map[int]int)
						formats[numID] = make(map[int]string)
						templates[numID] = make(map[int]string)
					}
					if p.Numbering.Start > 0 {
						starts[numID][level] = p.Numbering.Start
					}
					if p.Numbering.Format != "" {
						formats[numID][level] = p.Numbering.Format
					}
					if p.List != nil && p.List.Template != "" {
						templates[numID][level] = p.List.Template
					} else if strings.Contains(p.Numbering.Text, "%") {
						templates[numID][level] = p.Numbering.Text
					}
					if _, ok := counters[numID][level]; !ok {
						if s, hasStart := starts[numID][level]; hasStart {
							counters[numID][level] = s
						} else {
							counters[numID][level] = 1
						}
					} else {
						counters[numID][level]++
					}
					for childLevel := level + 1; childLevel <= 8; childLevel++ {
						delete(counters[numID], childLevel)
					}
					fullNum := ""
					template := templates[numID][level]
					// A semantic list template is authoritative when present. Legacy DOC
					// also stores an eagerly-resolved Numbering.Text, but that value may
					// have been computed before list-instance/profile reconciliation. Using
					// the template here reconstructs ancestor counters at render time and
					// prevents deep labels from collapsing to a stale current-level value.
					if strings.Contains(template, "%") {
						fullNum = substituteNumberTemplate(template, numID, counters, formats)
					} else {
						// Numbering.Text may be an eagerly resolved marker copied from the
						// source (for example "3."). Reusing that literal for every
						// paragraph freezes an ordered list at one number, but discarding
						// it loses the marker's punctuation. Keep the literal's shape and
						// run only the current level's counter through it.
						template = resolvedMarkerTemplate(p.Numbering.Text, level+1)
						fullNum = substituteNumberTemplate(template, numID, counters, formats)
					}
					// Some older DOC producers store only the punctuation suffix in the
					// resolved number text (for example ")") while the LVLF placeholder
					// offset is absent/ambiguous. Reconstruct the missing current-level
					// counter instead of rendering a bare suffix.
					if formats[numID][level] != "bullet" && listMarkerIsPunctuationOnly(fullNum) {
						fullNum = formatListNumber(counters[numID][level], formats[numID][level]) + fullNum
					}
					r.paragraphNumbers[p.Index] = fullNum
					r.paragraphLevelNum[p.Index] = formatListNumber(counters[numID][level], formats[numID][level])
				}
				for _, run := range p.Runs {
					if run.TextBox != nil {
						scan(run.TextBox.Blocks)
					}
				}
			case "table":
				if block.Table == nil {
					continue
				}
				for _, row := range block.Table.Rows {
					for _, cell := range row.Cells {
						scan(cell.Blocks)
					}
				}
			}
		}
	}
	if r.doc != nil {
		scan(r.doc.Blocks)
	}
}

func listMarkerIsPunctuationOnly(marker string) bool {
	marker = strings.TrimSpace(marker)
	if marker == "" {
		return false
	}
	for _, ch := range marker {
		if unicode.IsLetter(ch) || unicode.IsDigit(ch) {
			return false
		}
	}
	return true
}

// substituteNumberTemplate replaces %1, %2, etc. in a list template with the
// formatted counter values for the corresponding levels.
func substituteNumberTemplate(template string, numID int, counters map[int]map[int]int, formats map[int]map[int]string) string {
	result := template
	for level := 0; level <= 8; level++ {
		placeholder := "%" + strconv.Itoa(level+1)
		if !strings.Contains(result, placeholder) {
			continue
		}
		val := ""
		if count, ok := counters[numID][level]; ok {
			val = formatListNumber(count, formats[numID][level])
		}
		result = strings.ReplaceAll(result, placeholder, val)
	}
	return result
}

// formatListNumber formats a single list counter value according to the Word
// numbering format (decimal, lowerLetter, upperLetter, lowerRoman, upperRoman).
func formatListNumber(n int, format string) string {
	switch strings.ToLower(format) {
	case "decimal":
		return strconv.Itoa(n)
	case "lowerletter":
		return strings.ToLower(intToLetters(n))
	case "upperletter":
		return strings.ToUpper(intToLetters(n))
	case "lowerroman":
		return strings.ToLower(intToRoman(n))
	case "upperroman":
		return strings.ToUpper(intToRoman(n))
	case "ordinal":
		return ordinalNumber(n)
	default:
		return strconv.Itoa(n)
	}
}

func (r *htmlRenderer) initBookmarkText() {
	r.bookmarkText = make(map[string]string)
	r.bookmarkParaIndex = make(map[string]int)
	var collectBlocks func(blocks []resolve.Block)
	collectBlocks = func(blocks []resolve.Block) {
		for _, block := range blocks {
			switch block.Kind {
			case "paragraph":
				if block.Paragraph == nil {
					continue
				}
				text := paragraphBookmarkText(block.Paragraph)
				if text != "" {
					for _, bookmark := range block.Paragraph.Bookmarks {
						if bookmark == "" {
							continue
						}
						if _, ok := r.bookmarkText[bookmark]; !ok {
							r.bookmarkText[bookmark] = text
						}
						if _, ok := r.bookmarkParaIndex[bookmark]; !ok {
							r.bookmarkParaIndex[bookmark] = block.Paragraph.Index
						}
					}
				}
				for _, run := range block.Paragraph.Runs {
					if run.TextBox != nil {
						collectBlocks(run.TextBox.Blocks)
					}
				}
			case "table":
				if block.Table == nil {
					continue
				}
				for _, row := range block.Table.Rows {
					for _, cell := range row.Cells {
						collectBlocks(cell.Blocks)
					}
				}
			}
		}
	}
	collectBlocks(r.doc.Blocks)
	for _, story := range r.doc.Headers {
		collectBlocks(story.Blocks)
	}
	for _, story := range r.doc.Footers {
		collectBlocks(story.Blocks)
	}
	for _, story := range r.doc.Footnotes {
		collectBlocks(story.Blocks)
	}
	for _, story := range r.doc.Endnotes {
		collectBlocks(story.Blocks)
	}
	for _, story := range r.doc.Comments {
		collectBlocks(story.Blocks)
	}
}

func paragraphBookmarkText(p *resolve.Paragraph) string {
	if p == nil {
		return ""
	}
	if strings.TrimSpace(p.Text) != "" {
		return strings.TrimSpace(p.Text)
	}
	var sb strings.Builder
	for _, run := range p.Runs {
		sb.WriteString(run.Text)
	}
	return strings.TrimSpace(sb.String())
}

func (r *htmlRenderer) initBookmarkScopes() {
	r.bookmarkScopes = make(map[string]bookmarkScope)
	for name, paraIndex := range r.bookmarkParaIndex {
		r.extendBookmarkScope(name, paraIndex, 0)
	}
	for _, page := range r.pages {
		for _, box := range page.Blocks {
			if box.Block == nil {
				continue
			}
			r.collectBookmarkScopesFromBlocks([]resolve.Block{*box.Block}, page.Number)
		}
	}
}

func (r *htmlRenderer) collectBookmarkScopesFromBlocks(blocks []resolve.Block, pageNumber int) {
	for _, block := range blocks {
		switch block.Kind {
		case "paragraph":
			if block.Paragraph == nil {
				continue
			}
			for _, bookmark := range block.Paragraph.Bookmarks {
				r.extendBookmarkScope(bookmark, block.Paragraph.Index, pageNumber)
			}
			for _, run := range block.Paragraph.Runs {
				if run.TextBox != nil {
					r.collectBookmarkScopesFromBlocks(run.TextBox.Blocks, pageNumber)
				}
			}
		case "table":
			if block.Table == nil {
				continue
			}
			for _, row := range block.Table.Rows {
				for _, cell := range row.Cells {
					r.collectBookmarkScopesFromBlocks(cell.Blocks, pageNumber)
				}
			}
		}
	}
}

func (r *htmlRenderer) extendBookmarkScope(name string, paraIndex, pageNumber int) {
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	scope, ok := r.bookmarkScopes[name]
	if !ok {
		scope = bookmarkScope{startSeq: -1, endSeq: -1}
	}
	if seq, ok := r.paragraphSeqByIdx[paraIndex]; ok {
		if scope.startSeq < 0 || seq < scope.startSeq {
			scope.startSeq = seq
		}
		if scope.endSeq < 0 || seq > scope.endSeq {
			scope.endSeq = seq
		}
	}
	if pageNumber > 0 {
		if scope.startPage == 0 || pageNumber < scope.startPage {
			scope.startPage = pageNumber
		}
		if pageNumber > scope.endPage {
			scope.endPage = pageNumber
		}
	}
	r.bookmarkScopes[name] = scope
}

func (r *htmlRenderer) bookmarkScopeFor(name string) (bookmarkScope, bool) {
	name = strings.TrimSpace(name)
	if name == "" {
		return bookmarkScope{}, false
	}
	if r.bookmarkScopes == nil {
		r.initParagraphSequence()
		r.initBookmarkScopes()
	}
	scope, ok := r.bookmarkScopes[name]
	if !ok {
		return bookmarkScope{}, false
	}
	return scope, scope.startSeq >= 0 || scope.startPage > 0
}

func (r *htmlRenderer) paragraphInBookmarkScope(p *resolve.Paragraph, scope bookmarkScope) bool {
	if p == nil {
		return false
	}
	if scope.startSeq >= 0 && scope.endSeq >= scope.startSeq {
		if seq, ok := r.paragraphSeqByIdx[p.Index]; ok {
			return seq >= scope.startSeq && seq <= scope.endSeq
		}
	}
	return false
}

func (r *htmlRenderer) initNoteFieldState() {
	r.bookmarkNoteRefs = make(map[string]noteFieldTarget)
	r.footnoteNumbers = make(map[int]string)
	r.endnoteNumbers = make(map[int]string)
	footnoteNext := 1
	if r.doc != nil && r.doc.FootnoteNumberStart > 0 {
		footnoteNext = r.doc.FootnoteNumberStart
	}
	endnoteNext := 1
	if r.doc != nil && r.doc.EndnoteNumberStart > 0 {
		endnoteNext = r.doc.EndnoteNumberStart
	}
	var collectBlocks func(blocks []resolve.Block)
	collectBlocks = func(blocks []resolve.Block) {
		for _, block := range blocks {
			switch block.Kind {
			case "paragraph":
				if block.Paragraph == nil {
					continue
				}
				var firstTarget noteFieldTarget
				for _, run := range block.Paragraph.Runs {
					switch run.Kind {
					case "footnoteRef":
						if _, ok := r.footnoteNumbers[run.NoteID]; !ok {
							r.footnoteNumbers[run.NoteID] = formatNoteNumber(footnoteNext, r.doc.FootnoteNumberFormat)
							footnoteNext++
						}
						if firstTarget.noteID == 0 {
							firstTarget = noteFieldTarget{kind: "footnote", noteID: run.NoteID}
						}
					case "endnoteRef":
						if _, ok := r.endnoteNumbers[run.NoteID]; !ok {
							r.endnoteNumbers[run.NoteID] = formatNoteNumber(endnoteNext, r.doc.EndnoteNumberFormat)
							endnoteNext++
						}
						if firstTarget.noteID == 0 {
							firstTarget = noteFieldTarget{kind: "endnote", noteID: run.NoteID}
						}
					}
					if run.TextBox != nil {
						collectBlocks(run.TextBox.Blocks)
					}
				}
				if firstTarget.noteID != 0 {
					for _, bookmark := range block.Paragraph.Bookmarks {
						if bookmark == "" {
							continue
						}
						if _, ok := r.bookmarkNoteRefs[bookmark]; !ok {
							r.bookmarkNoteRefs[bookmark] = firstTarget
						}
					}
				}
			case "table":
				if block.Table == nil {
					continue
				}
				for _, row := range block.Table.Rows {
					for _, cell := range row.Cells {
						collectBlocks(cell.Blocks)
					}
				}
			}
		}
	}
	collectBlocks(r.doc.Blocks)
	for _, story := range r.doc.Headers {
		collectBlocks(story.Blocks)
	}
	for _, story := range r.doc.Footers {
		collectBlocks(story.Blocks)
	}
	for _, story := range r.doc.Footnotes {
		collectBlocks(story.Blocks)
	}
	for _, story := range r.doc.Endnotes {
		collectBlocks(story.Blocks)
	}
	for _, story := range r.doc.Comments {
		collectBlocks(story.Blocks)
	}
}

func (r *htmlRenderer) collectBookmarkPagesFromBlocks(blocks []resolve.Block, pageNumber int) {
	for _, block := range blocks {
		switch block.Kind {
		case "paragraph":
			if block.Paragraph == nil {
				continue
			}
			for _, bookmark := range block.Paragraph.Bookmarks {
				if bookmark == "" {
					continue
				}
				if _, ok := r.bookmarkPages[bookmark]; !ok {
					r.bookmarkPages[bookmark] = pageNumber
				}
			}
			for _, run := range block.Paragraph.Runs {
				if run.TextBox != nil {
					r.collectBookmarkPagesFromBlocks(run.TextBox.Blocks, pageNumber)
				}
			}
		case "table":
			if block.Table == nil {
				continue
			}
			for _, row := range block.Table.Rows {
				for _, cell := range row.Cells {
					r.collectBookmarkPagesFromBlocks(cell.Blocks, pageNumber)
				}
			}
		}
	}
}

func (r *htmlRenderer) initLayoutMaps() {
	r.initLayoutMapsForPages(layout.Paginate(r.doc))
}

func (r *htmlRenderer) initLayoutMapsForPages(pages []layout.Page) {
	r.pages = pages
	r.floatByImage = make(map[*resolve.Image]layout.FloatBox)
	r.floatByTextBox = make(map[*resolve.TextBox]layout.FloatBox)
	r.floatByShape = make(map[*resolve.Shape]layout.FloatBox)
	r.floatByLegacyShapeID = make(map[uint32]layout.FloatBox)
	r.floatByFramePara = make(map[*resolve.Paragraph]layout.FloatBox)
	r.floatByTable = make(map[*resolve.Table]layout.FloatBox)
	r.paraBoxes = make(map[int]layout.BlockBox)
	r.paraHeights = make(map[int]float64)
	r.bookmarkPages = make(map[string]int)
	r.sectionPageCounts = make(map[int]int)
	for _, page := range pages {
		r.sectionPageCounts[page.SectionIndex]++
		for _, fb := range page.Floats {
			switch {
			case fb.Image != nil:
				r.floatByImage[fb.Image] = fb
			case fb.TextBox != nil:
				r.floatByTextBox[fb.TextBox] = fb
			case fb.Shape != nil:
				r.floatByShape[fb.Shape] = fb
				if fb.Shape.LegacyShapeID != 0 {
					r.floatByLegacyShapeID[fb.Shape.LegacyShapeID] = fb
				}
			case fb.FrameParagraph != nil:
				r.floatByFramePara[fb.FrameParagraph] = fb
			case fb.FloatTable != nil:
				r.floatByTable[fb.FloatTable] = fb
			}
		}
		for _, bb := range page.Blocks {
			if bb.Block == nil {
				continue
			}
			if bb.Block.Paragraph != nil {
				if _, ok := r.paraBoxes[bb.Block.Paragraph.Index]; !ok {
					r.paraBoxes[bb.Block.Paragraph.Index] = bb
				}
				r.paraHeights[bb.Block.Paragraph.Index] = bb.Height
			}
			r.collectBookmarkPagesFromBlocks([]resolve.Block{*bb.Block}, page.Number)
		}
	}
}

// initFastPreviewLayout supplies a stable browser page without running the
// expensive Word pagination engine. Floating object placement is deliberately
// resolved in normal document flow by the renderer.
func (r *htmlRenderer) initFastPreviewLayout() {
	r.pages = nil
	r.floatByImage = make(map[*resolve.Image]layout.FloatBox)
	r.floatByTextBox = make(map[*resolve.TextBox]layout.FloatBox)
	r.floatByShape = make(map[*resolve.Shape]layout.FloatBox)
	r.floatByLegacyShapeID = make(map[uint32]layout.FloatBox)
	r.floatByFramePara = make(map[*resolve.Paragraph]layout.FloatBox)
	r.floatByTable = make(map[*resolve.Table]layout.FloatBox)
	r.paraBoxes = make(map[int]layout.BlockBox)
	r.paraHeights = make(map[int]float64)
	r.bookmarkPages = make(map[string]int)
	r.sectionPageCounts = make(map[int]int)
	r.applyGeometry(layout.PageGeometry{
		Width: 612, Height: 792,
		MarginTop: 72, MarginRight: 72, MarginBottom: 72, MarginLeft: 72,
		ColumnCount: 1, ColumnWidthPt: 468,
	})
}

func (r *htmlRenderer) applyGeometry(geom layout.PageGeometry) {
	r.pageWidthPt = geom.Width
	r.pageHeightPt = geom.Height
	r.marginTopPt = geom.MarginTop
	r.marginRightPt = geom.MarginRight
	r.marginBottomPt = geom.MarginBottom
	r.marginLeftPt = geom.MarginLeft
	r.columnCount = geom.ColumnCount
	r.columnGapPt = geom.ColumnGapPt
	r.columnWidthsPt = append(r.columnWidthsPt[:0], geom.ColumnWidthsPt...)
	r.columnGapsPt = append(r.columnGapsPt[:0], geom.ColumnGapsPt...)
	r.columnSep = geom.ColumnSep
	if r.columnCount < 1 {
		r.columnCount = 1
	}
}

func (r *htmlRenderer) withLocalLayout(blocks []resolve.Block, geom layout.PageGeometry, fn func([]layout.BlockBox)) {
	savedOverlay := r.overlay
	savedBehindOverlay := r.behindOverlay
	savedPageWidth, savedPageHeight := r.pageWidthPt, r.pageHeightPt
	savedMargins := [4]float64{r.marginTopPt, r.marginRightPt, r.marginBottomPt, r.marginLeftPt}
	savedColumns := [2]float64{float64(r.columnCount), r.columnGapPt}
	savedColumnWidths := append([]float64(nil), r.columnWidthsPt...)
	savedColumnGaps := append([]float64(nil), r.columnGapsPt...)
	savedColumnSep := r.columnSep
	savedListStack := append([]listFrame(nil), r.listStack...)
	savedFloatByImage := r.floatByImage
	savedFloatByTextBox := r.floatByTextBox
	savedFloatByShape := r.floatByShape
	savedFloatByLegacyShapeID := r.floatByLegacyShapeID
	savedFloatByFramePara := r.floatByFramePara
	savedFloatByTable := r.floatByTable
	savedParaBoxes := r.paraBoxes
	savedParaHeights := r.paraHeights
	savedPageTopAnchor := r.pageTopFloatAnchor
	savedPageTopImages := r.pageTopFloatImages
	savedRenderedTopFloats := r.renderedTopFloats

	r.overlay = bytes.Buffer{}
	r.behindOverlay = bytes.Buffer{}
	r.applyGeometry(geom)
	r.listStack = nil
	r.pageTopFloatAnchor = nil
	r.pageTopFloatImages = nil
	r.renderedTopFloats = nil
	r.floatByImage = make(map[*resolve.Image]layout.FloatBox)
	r.floatByTextBox = make(map[*resolve.TextBox]layout.FloatBox)
	r.floatByShape = make(map[*resolve.Shape]layout.FloatBox)
	r.floatByLegacyShapeID = make(map[uint32]layout.FloatBox)
	r.floatByFramePara = make(map[*resolve.Paragraph]layout.FloatBox)
	r.floatByTable = make(map[*resolve.Table]layout.FloatBox)
	r.paraBoxes = make(map[int]layout.BlockBox)
	r.paraHeights = make(map[int]float64)

	localBoxes, localFloats := layout.LayoutBlocksForRender(r.doc, flattenSDTBlocks(blocks), geom)
	for _, fb := range localFloats {
		switch {
		case fb.Image != nil:
			r.floatByImage[fb.Image] = fb
		case fb.TextBox != nil:
			r.floatByTextBox[fb.TextBox] = fb
		case fb.Shape != nil:
			r.floatByShape[fb.Shape] = fb
			if fb.Shape.LegacyShapeID != 0 {
				r.floatByLegacyShapeID[fb.Shape.LegacyShapeID] = fb
			}
		case fb.FrameParagraph != nil:
			r.floatByFramePara[fb.FrameParagraph] = fb
		case fb.FloatTable != nil:
			r.floatByTable[fb.FloatTable] = fb
		}
	}
	for _, bb := range localBoxes {
		if bb.Block != nil && bb.Block.Paragraph != nil {
			if _, ok := r.paraBoxes[bb.Block.Paragraph.Index]; !ok {
				r.paraBoxes[bb.Block.Paragraph.Index] = bb
			}
			r.paraHeights[bb.Block.Paragraph.Index] = bb.Height
		}
	}

	defer func() {
		r.overlay = savedOverlay
		r.behindOverlay = savedBehindOverlay
		r.pageWidthPt = savedPageWidth
		r.pageHeightPt = savedPageHeight
		r.marginTopPt = savedMargins[0]
		r.marginRightPt = savedMargins[1]
		r.marginBottomPt = savedMargins[2]
		r.marginLeftPt = savedMargins[3]
		r.columnCount = int(savedColumns[0])
		r.columnGapPt = savedColumns[1]
		r.columnWidthsPt = savedColumnWidths
		r.columnGapsPt = savedColumnGaps
		r.columnSep = savedColumnSep
		r.listStack = savedListStack
		r.floatByImage = savedFloatByImage
		r.floatByTextBox = savedFloatByTextBox
		r.floatByShape = savedFloatByShape
		r.floatByLegacyShapeID = savedFloatByLegacyShapeID
		r.floatByFramePara = savedFloatByFramePara
		r.floatByTable = savedFloatByTable
		r.paraBoxes = savedParaBoxes
		r.paraHeights = savedParaHeights
		r.pageTopFloatAnchor = savedPageTopAnchor
		r.pageTopFloatImages = savedPageTopImages
		r.renderedTopFloats = savedRenderedTopFloats
	}()

	r.frameDepth++
	fn(localBoxes)
	r.frameDepth--
}

func (r *htmlRenderer) renderPageBlocks(page layout.Page) {
	boxes := page.Blocks
	if len(boxes) == 0 {
		return
	}
	r.pageTopFloatAnchor, r.pageTopFloatImages = r.collectPageTopFlowImages(page)
	r.renderedTopFloats = make(map[*resolve.Image]struct{}, len(r.pageTopFloatImages))
	defer func() {
		r.pageTopFloatAnchor = nil
		r.pageTopFloatImages = nil
		r.renderedTopFloats = nil
	}()
	baseGeom := page.Geometry
	if r.mixedColumnPage {
		// The page body is single-column; every column grid gets its own wrapper.
		baseGeom = singleColumnGeometry(baseGeom)
	}
	currentWrappedGeom := baseGeom
	sectionWrapperOpen := false
	columnWrapperOpen := false
	currentWrappedColumn := -1
	closeColumnWrapper := func() {
		if !columnWrapperOpen {
			return
		}
		r.closeAllLists()
		r.buf.WriteString("</div>\n")
		columnWrapperOpen = false
		currentWrappedColumn = -1
	}
	closeSectionWrapper := func() {
		if !sectionWrapperOpen {
			return
		}
		closeColumnWrapper()
		r.closeAllLists()
		r.buf.WriteString("</div>\n")
		sectionWrapperOpen = false
	}
	for i, box := range boxes {
		if box.Block == nil {
			continue
		}
		r.pendingNoteTarget = box.Block
		// Note bodies print their own reference mark as a plain superscript.
		// Ordinary blocks carry an empty NoteKind and reset the flag.
		r.noteBodyKind = box.Block.NoteKind
		// Field container transitions.
		if box.Block.FieldKind != "" && box.Block.FieldIndex != r.openFieldIndex {
			if r.openFieldIndex != -1 {
				r.closeFieldContainer()
			}
			r.openFieldContainer(box.Block.FieldKind)
			r.openFieldIndex = box.Block.FieldIndex
		} else if box.Block.FieldKind == "" && r.openFieldIndex != -1 {
			r.closeFieldContainer()
			r.openFieldIndex = -1
		}
		gridGeom := box.Geometry
		if r.physicalColumnPage {
			gridGeom = physicalColumnGridGeometry(gridGeom)
		}
		if pageBodySectionNeedsWrapper(baseGeom, gridGeom) {
			if !sectionWrapperOpen || !samePageBodySectionGeometry(currentWrappedGeom, box.Geometry) {
				closeSectionWrapper()
				fmt.Fprintf(&r.buf, "<div class=\"page-body-section\"%s>\n", pageBodySectionStyleAttr(gridGeom))
				sectionWrapperOpen = true
				currentWrappedGeom = box.Geometry
			}
			if hasExplicitPhysicalColumns(gridGeom) && (!columnWrapperOpen || currentWrappedColumn != box.Column) {
				closeColumnWrapper()
				fmt.Fprintf(&r.buf, "<div class=\"word-column\" data-word-column=\"%d\"%s>\n", box.Column, pageBodyColumnStyleAttr(gridGeom, box.Column))
				columnWrapperOpen = true
				currentWrappedColumn = box.Column
			}
		} else {
			closeSectionWrapper()
			currentWrappedGeom = baseGeom
		}
		if box.Block.CustomXml != nil {
			r.closeAllLists()
			r.openCustomXmlBlock(box.Block.CustomXml)
		}
		if box.Block.ContentControl != nil {
			r.closeAllLists()
			r.openSdtBlock(box.Block.ContentControl)
		}
		switch box.Block.Kind {
		case "paragraph":
			p := box.Block.Paragraph
			if p != nil && p.Frame != nil {
				if isDropCapFrameParagraph(p) {
					r.closeAllLists()
					r.renderDropCapFramePara(p)
					if box.Block.ContentControl != nil {
						r.closeSdtBlock()
					}
					if box.Block.CustomXml != nil {
						r.closeCustomXmlBlock()
					}
					continue
				}
				r.renderFrameParaTo(&r.overlay, p)
				if box.Block.ContentControl != nil {
					r.closeSdtBlock()
				}
				if box.Block.CustomXml != nil {
					r.closeCustomXmlBlock()
				}
				continue
			}
			r.renderPageParagraphBox(boxes, i)
			if box.Block.ContentControl != nil {
				r.closeSdtBlock()
			}
			if box.Block.CustomXml != nil {
				r.closeCustomXmlBlock()
			}
		case "table":
			t := box.Block.Table
			if t != nil && t.Format.FloatProps != nil {
				r.ensurePage()
				if r.renderFloatTableToOverlay(t) {
					// Table rendered to overlay with layout-computed absolute coordinates.
				} else {
					r.renderInlineFloatTable(t)
				}
				if box.Block.ContentControl != nil {
					r.closeSdtBlock()
				}
				if box.Block.CustomXml != nil {
					r.closeCustomXmlBlock()
				}
				continue
			}
			r.closeAllLists()
			if box.TableFragment != nil {
				r.renderTableFragment(t, box.TableFragment)
			} else {
				r.renderTable(t)
			}
			if box.Block.ContentControl != nil {
				r.closeSdtBlock()
			}
			if box.Block.CustomXml != nil {
				r.closeCustomXmlBlock()
			}
		}
	}
	closeSectionWrapper()
	r.closeAllLists()
	if r.openFieldIndex != -1 {
		r.closeFieldContainer()
		r.openFieldIndex = -1
	}
}

// pageHasPhysicalColumns selects the layout-owned column grid only where text
// has already been paginated into physical fragments. Browser multicolumn flow
// would otherwise redistribute those fragments and can create a third column.
func pageHasPhysicalColumns(page layout.Page) bool {
	for _, box := range page.Blocks {
		if box.Geometry.ColumnCount > 1 && box.Fragment != nil && len(box.Fragment.Lines) > 0 {
			return true
		}
	}
	return false
}

func physicalColumnGridGeometry(geom layout.PageGeometry) layout.PageGeometry {
	if geom.ColumnCount <= 1 || hasExplicitPhysicalColumns(geom) {
		return geom
	}
	width := geom.ColumnWidthPt
	if width <= 0 {
		width = (geom.Width - geom.MarginLeft - geom.MarginRight - float64(geom.ColumnCount-1)*geom.ColumnGapPt) / float64(geom.ColumnCount)
	}
	if width <= 0 {
		return geom
	}
	geom.ColumnWidthsPt = make([]float64, geom.ColumnCount)
	for i := range geom.ColumnWidthsPt {
		geom.ColumnWidthsPt[i] = width
	}
	geom.ExplicitColumns = true
	return geom
}

// pageHasMixedColumnGeometry reports whether continuous section breaks put
// blocks with different column grids on one page.
func pageHasMixedColumnGeometry(page layout.Page) bool {
	for _, box := range page.Blocks {
		if box.Block != nil && !samePageBodySectionGeometry(page.Geometry, box.Geometry) {
			return true
		}
	}
	return false
}

func singleColumnGeometry(geom layout.PageGeometry) layout.PageGeometry {
	geom.ColumnCount = 1
	geom.ColumnWidthPt = 0
	geom.ColumnWidthsPt = nil
	geom.ColumnGapsPt = nil
	geom.ExplicitColumns = false
	geom.ColumnSep = false
	return geom
}

func samePageBodySectionGeometry(left, right layout.PageGeometry) bool {
	if left.ColumnCount <= 1 && right.ColumnCount <= 1 {
		return true
	}
	return left.ColumnCount == right.ColumnCount &&
		left.ExplicitColumns == right.ExplicitColumns &&
		almostSameFloat(left.ColumnGapPt, right.ColumnGapPt) &&
		left.ColumnSep == right.ColumnSep &&
		almostSameFloatSlice(left.ColumnWidthsPt, right.ColumnWidthsPt) &&
		almostSameFloatSlice(left.ColumnGapsPt, right.ColumnGapsPt)
}

func almostSameFloatSlice(left, right []float64) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if !almostSameFloat(left[i], right[i]) {
			return false
		}
	}
	return true
}

func pageBodySectionNeedsWrapper(pageGeom, blockGeom layout.PageGeometry) bool {
	// Explicit unequal Word columns cannot be represented faithfully by CSS
	// multi-column layout: column-width is only a preferred width and adjacent
	// gaps are necessarily uniform. Keep those sections in their own wrapper so
	// renderPageBlocks can materialize the physical column grid from layout.
	return hasExplicitPhysicalColumns(blockGeom) || !samePageBodySectionGeometry(pageGeom, blockGeom)
}

func hasExplicitPhysicalColumns(geom layout.PageGeometry) bool {
	if geom.ColumnCount <= 1 || len(geom.ColumnWidthsPt) < geom.ColumnCount {
		return false
	}
	// A physical grid supplied by Word is authoritative even when all widths
	// happen to be equal. Browser multicolumn flow balances content and can
	// move paragraphs between columns, while Word's layout already assigned
	// each block to a concrete column.
	if geom.ExplicitColumns {
		return true
	}
	first := geom.ColumnWidthsPt[0]
	for i := 1; i < geom.ColumnCount; i++ {
		if !almostSameFloat(first, geom.ColumnWidthsPt[i]) {
			return true
		}
	}
	if len(geom.ColumnGapsPt) > 1 {
		firstGap := geom.ColumnGapsPt[0]
		for i := 1; i < len(geom.ColumnGapsPt); i++ {
			if !almostSameFloat(firstGap, geom.ColumnGapsPt[i]) {
				return true
			}
		}
	}
	return false
}

func explicitColumnGridTemplate(geom layout.PageGeometry) string {
	if !hasExplicitPhysicalColumns(geom) {
		return ""
	}
	tracks := make([]string, 0, geom.ColumnCount*2-1)
	for i := 0; i < geom.ColumnCount; i++ {
		tracks = append(tracks, fmt.Sprintf("%.1fpt", geom.ColumnWidthsPt[i]))
		if i >= geom.ColumnCount-1 {
			continue
		}
		gap := geom.ColumnGapPt
		if i < len(geom.ColumnGapsPt) {
			gap = geom.ColumnGapsPt[i]
		}
		tracks = append(tracks, fmt.Sprintf("%.1fpt", gap))
	}
	return strings.Join(tracks, " ")
}

func pageBodySectionStyleAttr(geom layout.PageGeometry) string {
	styles := make([]string, 0, 4)
	if grid := explicitColumnGridTemplate(geom); grid != "" {
		styles = append(styles,
			"display:grid",
			"grid-template-columns:"+grid,
			"align-items:start",
		)
	} else if geom.ColumnCount > 1 {
		styles = append(styles,
			fmt.Sprintf("column-count:%d", geom.ColumnCount),
			fmt.Sprintf("column-gap:%.1fpt", geom.ColumnGapPt),
		)
		if geom.ColumnSep {
			styles = append(styles, "column-rule:1px solid #bbb")
		}
	}
	if len(styles) == 0 {
		return ""
	}
	return fmt.Sprintf(" style=%q", strings.Join(styles, ";"))
}

func pageBodyColumnStyleAttr(geom layout.PageGeometry, column int) string {
	if explicitColumnGridTemplate(geom) == "" {
		return ""
	}
	if column < 0 {
		column = 0
	}
	if column >= geom.ColumnCount {
		column = geom.ColumnCount - 1
	}
	// Width and gap tracks alternate, so Word column n occupies grid track 2n+1.
	track := column*2 + 1
	styles := []string{fmt.Sprintf("grid-column:%d", track), "min-width:0"}
	if geom.ColumnSep && column > 0 {
		styles = append(styles, "border-left:1px solid #bbb")
		gap := geom.ColumnGapPt
		if column-1 < len(geom.ColumnGapsPt) {
			gap = geom.ColumnGapsPt[column-1]
		}
		if gap > 0 {
			styles = append(styles, fmt.Sprintf("padding-left:%.1fpt", gap/2))
		}
	}
	return fmt.Sprintf(" style=%q", strings.Join(styles, ";"))
}

func (r *htmlRenderer) renderPageParagraphBox(boxes []layout.BlockBox, index int) {
	box := boxes[index]
	if box.Block == nil || box.Block.Paragraph == nil {
		return
	}
	p := box.Block.Paragraph
	if box.Fragment != nil {
		// Paragraph fragments own the text that belongs to this page/column, but
		// physical-line projection deliberately omits drawing anchors. Preserve
		// those source-owned floating objects independently from the fragment text
		// so choosing a fragment rendering strategy can never make an image,
		// textbox, or shape disappear. Anchors already present in the fragment are
		// skipped to avoid duplicate overlay emission.
		// Once layout has projected physical lines, those lines are authoritative
		// for rendering as well as pagination. Do not hand an ordinary full-width
		// fragment back to the browser merely because it did not need constrained
		// Square/Tight geometry: doing so creates a second line-breaking engine and
		// allows pagination and rendered line topology to drift apart.
		if len(box.Fragment.Lines) > 0 {
			r.renderPhysicalParagraphFragment(box, boxes, index)
			return
		}
		// Compatibility fallback only for structural fragments that genuinely have
		// no physical-line projection yet.
		renderPageParagraphFragmentRuns(r, p, box.Fragment, !box.Fragment.IsContinuation, box.Fragment.IsContinuation, !box.Fragment.IsLast, box.Fragment.IsContinuation, box.Fragment.IsContinuation, box.Fragment.StartsNewColumn)
		return
	}
	suppressBefore := layout.ContextualSpacingSuppresses(p, adjacentPageParagraph(boxes, index, -1))
	suppressAfter := layout.ContextualSpacingSuppresses(p, adjacentPageParagraph(boxes, index, +1))
	renderPageParagraphRuns(r, p, p.Runs, true, suppressBefore, suppressAfter, false, false, false)
}

// renderPageParagraphRuns keeps layout progression and renderer progression as
// separate concerns for drawing-host paragraphs. Layout has already consumed the
// paragraph's measured height and used its Y as the reference for paragraph-
// relative floats. The page body therefore must not create a second CSS flow box
// for a paragraph whose only content is floating anchors. The anchor runs still
// need to be visited so their objects are emitted to the page overlay.
func renderPageParagraphRuns(r *htmlRenderer, p *resolve.Paragraph, runs []resolve.Run, includeBookmarks bool, suppressBefore, suppressAfter, suppressListMarker, suppressFirstLineIndent, forceColumnBreakBefore bool) {
	if paragraphIsFloatingAnchorOnly(runs) && !paragraphHasOnlyBehindTextAnchors(runs) {
		r.renderParagraphSegmentContent(
			p, runs, includeBookmarks, suppressBefore, suppressAfter, suppressListMarker, suppressFirstLineIndent, forceColumnBreakBefore,
			"display:contents",
			func() { r.renderRuns(runs) },
		)
		return
	}
	r.renderParagraphSegment(p, runs, includeBookmarks, suppressBefore, suppressAfter, suppressListMarker, suppressFirstLineIndent, forceColumnBreakBefore)
}

func renderPageParagraphFragmentRuns(r *htmlRenderer, p *resolve.Paragraph, fragment *layout.ParagraphFragment, includeBookmarks bool, suppressBefore, suppressAfter, suppressListMarker, suppressFirstLineIndent, forceColumnBreakBefore bool) {
	if fragment == nil {
		return
	}
	r.renderParagraphSegmentContent(
		p, fragment.FallbackRuns, includeBookmarks, suppressBefore, suppressAfter, suppressListMarker, suppressFirstLineIndent, forceColumnBreakBefore,
		"",
		func() {
			// Physical-line projection intentionally excludes drawing anchors. Emit
			// source-owned floats while currentParagraph is active and before the
			// fragment text. This preserves the normal wrap/block semantics for the
			// common Word anchor-before-text form.
			r.renderMissingFragmentFloatingContent(p.Runs, fragment.FallbackRuns)
			r.renderRuns(fragment.FallbackRuns)
		},
	)
}

func (r *htmlRenderer) renderMissingFragmentFloatingContent(sourceRuns, fragmentRuns []resolve.Run) {
	images := make(map[*resolve.Image]struct{})
	textBoxes := make(map[*resolve.TextBox]struct{})
	shapes := make(map[*resolve.Shape]struct{})
	for _, run := range fragmentRuns {
		if run.Image != nil && run.Image.Floating {
			images[run.Image] = struct{}{}
		}
		if run.TextBox != nil && run.TextBox.Floating {
			textBoxes[run.TextBox] = struct{}{}
		}
		if run.Shape != nil && run.Shape.Floating {
			shapes[run.Shape] = struct{}{}
		}
	}
	for _, run := range sourceRuns {
		switch {
		case run.Image != nil && run.Image.Floating:
			if _, ok := images[run.Image]; !ok {
				r.renderRun(run)
			}
		case run.TextBox != nil && run.TextBox.Floating:
			if _, ok := textBoxes[run.TextBox]; !ok {
				r.renderRun(run)
			}
		case run.Shape != nil && run.Shape.Floating:
			if _, ok := shapes[run.Shape]; !ok {
				r.renderRun(run)
			}
		}
	}
}

func adjacentPageParagraph(boxes []layout.BlockBox, i, dir int) *resolve.Paragraph {
	for j := i + dir; j >= 0 && j < len(boxes); j += dir {
		if boxes[j].Block == nil {
			continue
		}
		if boxes[j].Block.Kind == "paragraph" {
			if boxes[j].Block == boxes[i].Block {
				break
			}
			return boxes[j].Block.Paragraph
		}
		break
	}
	return nil
}

func (r *htmlRenderer) renderPageStories(page layout.Page) {
	r.renderPageHeaderFooter(page, "header")
	r.renderPageFootnotes(page)
	r.renderPageHeaderFooter(page, "footer")
}

func (r *htmlRenderer) renderPageHeaderFooter(page layout.Page, kind string) {
	var refs []resolve.Reference
	var stories map[string]resolve.Story
	var top float64
	var height float64
	if kind == "header" {
		refs = page.Section.Headers
		stories = r.doc.Headers
		if story, ok := layoutStoryForPage(stories, refs, page.Number, r.doc.EvenAndOddHeaders, page.Section.TitlePage); ok {
			top = 0
			if page.Section.PageMargin != nil {
				top = twipsToPoints(page.Section.PageMargin.Header)
			}
			height = estimateStoryFrameHeight(r.doc, story, page.Geometry.Width-page.Geometry.MarginLeft-page.Geometry.MarginRight)
			r.renderStoryFrame(&r.overlay, storyBlocks(story), "page-header", page.Geometry.MarginLeft, top, page.Geometry.Width-page.Geometry.MarginLeft-page.Geometry.MarginRight, height)
		}
		return
	}
	refs = page.Section.Footers
	stories = r.doc.Footers
	story, ok := layoutStoryForPage(stories, refs, page.Number, r.doc.EvenAndOddHeaders, page.Section.TitlePage)
	if !ok {
		return
	}
	height = estimateStoryFrameHeight(r.doc, story, page.Geometry.Width-page.Geometry.MarginLeft-page.Geometry.MarginRight)
	bottom := 0.0
	if page.Section.PageMargin != nil {
		bottom = twipsToPoints(page.Section.PageMargin.Footer)
	}
	top = page.Geometry.Height - bottom - height
	r.renderStoryFrame(&r.overlay, storyBlocks(story), "page-footer", page.Geometry.MarginLeft, top, page.Geometry.Width-page.Geometry.MarginLeft-page.Geometry.MarginRight, height)
}

func (r *htmlRenderer) renderPageFootnotes(page layout.Page) {
	if len(page.Footnotes) == 0 {
		return
	}
	blocks := make([]resolve.Block, 0, len(page.Footnotes))
	frameGeom := page.Geometry
	startY := page.Footnotes[0].Y
	endY := page.Footnotes[0].Y + page.Footnotes[0].Height
	for _, box := range page.Footnotes {
		if box.Block == nil {
			continue
		}
		blocks = append(blocks, *box.Block)
		if box.Geometry.ColumnWidthPt > 0 {
			frameGeom = box.Geometry
		}
		if box.Y < startY {
			startY = box.Y
		}
		if box.Y+box.Height > endY {
			endY = box.Y + box.Height
		}
	}
	if len(blocks) == 0 {
		return
	}
	r.renderStoryFrame(&r.overlay, blocks, "page-footnotes", frameGeom.MarginLeft, frameGeom.MarginTop+startY, frameGeom.ColumnWidthPt, endY-startY)
}

func footerBlocksWithCanonicalTabs(blocks []resolve.Block, widthPt float64) []resolve.Block {
	if widthPt <= 0 {
		return blocks
	}
	out := append([]resolve.Block(nil), blocks...)
	for i := range out {
		p := out[i].Paragraph
		if out[i].Kind != "paragraph" || p == nil || len(p.Format.TabStops) != 0 {
			continue
		}
		tabs := 0
		for _, run := range p.Runs {
			if run.Kind == "tab" || run.Text == "\t" {
				tabs++
			}
		}
		if tabs < 2 {
			continue
		}
		copyParagraph := *p
		copyParagraph.Format.TabStops = []resolve.TabStop{
			{Pos: int(widthPt * 10), Algn: "center"}, // points -> twips, half width
			{Pos: int(widthPt * 20), Algn: "right"},  // points -> twips, right edge
		}
		out[i].Paragraph = &copyParagraph
	}
	return out
}

func (r *htmlRenderer) renderStoryFrame(target *bytes.Buffer, blocks []resolve.Block, className string, left, top, width, height float64) {
	if len(blocks) == 0 || target == nil {
		return
	}
	if width <= 0 {
		width = r.pageWidthPt
	}
	if height <= 0 {
		height = estimateStoryFrameHeight(r.doc, resolve.Story{Blocks: blocks}, width)
	}
	styles := []string{
		"position:absolute",
		fmt.Sprintf("left:%.1fpt", left),
		fmt.Sprintf("top:%.1fpt", top),
		fmt.Sprintf("width:%.1fpt", width),
		fmt.Sprintf("min-height:%.1fpt", height),
		"overflow:hidden",
	}
	frameBuf := bytes.Buffer{}
	geom := layout.PageGeometry{Width: width, Height: height, ColumnCount: 1, ColumnWidthPt: width}
	if className == "page-footer" {
		blocks = footerBlocksWithCanonicalTabs(blocks, width)
	}
	r.withLocalLayout(blocks, geom, func(localBoxes []layout.BlockBox) {
		r.withOutputBuffer(&frameBuf, func() {
			fmt.Fprintf(&r.buf, `<div class=%q style=%q>`, className, strings.Join(styles, ";"))
			r.withLineNumberingSuppressed(func() {
				// Header/footer stories must render the same physical paragraph
				// topology that LayoutBlocksForRender measured. In particular this
				// preserves Word tab-stop geometry for left/center/right footer fields.
				r.renderPageBlocks(layout.Page{Geometry: geom, Blocks: localBoxes})
			})
			r.closeAllLists()
			if r.overlay.Len() > 0 {
				r.buf.WriteString(`<div class="story-overlay">`)
				r.buf.Write(r.overlay.Bytes())
				r.buf.WriteString(`</div>`)
			}
			r.buf.WriteString(`</div>`)
		})
	})
	target.Write(frameBuf.Bytes())
}

func documentEndsWithExplicitPageBreak(doc *resolve.Document) bool {
	if doc == nil {
		return false
	}
	for i := len(doc.Blocks) - 1; i >= 0; i-- {
		block := doc.Blocks[i]
		if block.Kind == "section" {
			continue
		}
		if block.Kind != "paragraph" || block.Paragraph == nil {
			return false
		}
		runs := block.Paragraph.Runs
		for j := len(runs) - 1; j >= 0; j-- {
			if runs[j].Kind == "break" && runs[j].BreakType == "page" {
				return true
			}
			if strings.TrimSpace(runs[j].Text) != "" {
				return false
			}
		}
		return false
	}
	return false
}

func storyBlocks(story resolve.Story) []resolve.Block {
	blocks := make([]resolve.Block, 0, len(story.Blocks)+len(story.Paragraphs)+len(story.Tables))
	if len(story.Blocks) > 0 {
		for _, block := range story.Blocks {
			if block.Kind == "section" {
				continue
			}
			blocks = append(blocks, block)
		}
	} else {
		for i := range story.Paragraphs {
			paragraph := story.Paragraphs[i]
			blocks = append(blocks, resolve.Block{Kind: "paragraph", Paragraph: &paragraph})
		}
		for i := range story.Tables {
			table := story.Tables[i]
			blocks = append(blocks, resolve.Block{Kind: "table", Table: &table})
		}
	}

	// Word stories always terminate with a paragraph mark. A trailing empty
	// paragraph is structural, not an extra visible footer/header line. Counting
	// it in the frame height moves a bottom-anchored footer upward by one full
	// line and makes it overlap the body. Trim only terminal paragraphs that have
	// no visible or floating content.
	for len(blocks) > 0 {
		last := blocks[len(blocks)-1]
		if last.Kind != "paragraph" || !storyParagraphIsVisuallyEmpty(last.Paragraph) {
			break
		}
		blocks = blocks[:len(blocks)-1]
	}
	return blocks
}

func storyParagraphIsVisuallyEmpty(p *resolve.Paragraph) bool {
	if p == nil {
		return true
	}
	if strings.TrimSpace(p.Text) != "" {
		return false
	}
	for _, run := range p.Runs {
		if strings.TrimSpace(run.Text) != "" {
			return false
		}
		switch run.Kind {
		case "image", "drawing", "textbox", "shape", "chart", "ole", "field", "footnoteRef", "endnoteRef":
			return false
		}
	}
	return true
}

func layoutStoryForPage(stories map[string]resolve.Story, refs []resolve.Reference, pageNumber int, evenAndOddHeaders, titlePage bool) (resolve.Story, bool) {
	if titlePage && pageNumber <= 1 {
		if story, ok := storyByType(stories, refs, "first"); ok {
			return story, true
		}
	}
	if evenAndOddHeaders && pageNumber > 0 && pageNumber%2 == 0 {
		if story, ok := storyByType(stories, refs, "even"); ok {
			return story, true
		}
	}
	return storyByType(stories, refs, "default")
}

func storyByType(stories map[string]resolve.Story, refs []resolve.Reference, storyType string) (resolve.Story, bool) {
	for _, ref := range refs {
		if storyType == "default" {
			if !strings.EqualFold(ref.Type, "default") && ref.Type != "" {
				continue
			}
		} else if !strings.EqualFold(ref.Type, storyType) {
			continue
		}
		story, ok := stories[ref.PartName]
		if ok {
			return story, true
		}
	}
	return resolve.Story{}, false
}

func estimateStoryFrameHeight(doc *resolve.Document, story resolve.Story, widthPt float64) float64 {
	if widthPt <= 0 {
		widthPt = 1
	}
	total := 0.0
	blocks := storyBlocks(story)
	for i, block := range blocks {
		switch block.Kind {
		case "paragraph":
			p := block.Paragraph
			prev := adjacentParagraph(blocks, i, -1)
			next := adjacentParagraph(blocks, i, +1)
			total += layout.ParagraphSpacingBefore(doc, p, prev, layout.PageGeometry{}, i == 0)
			total += layout.ParagraphHeightInWidthForRender(doc, p, widthPt)
			total += layout.ParagraphSpacingAfter(doc, p, next, layout.PageGeometry{})
		case "table":
			total += layout.TableHeightForRender(doc, block.Table, widthPt)
		}
	}
	return total
}

func (r *htmlRenderer) openFieldContainer(kind string) {
	if kind == "" {
		return
	}
	className := "field-" + strings.ToLower(kind)
	r.buf.WriteString(`<div class=` + strconv.Quote(className) + `>` + "\n")
}

func (r *htmlRenderer) closeFieldContainer() {
	r.closeAllLists()
	r.buf.WriteString("</div>\n")
}

func (r *htmlRenderer) renderBlocks(blocks []resolve.Block) {
	for i, block := range blocks {
		r.pendingNoteTarget = &blocks[i]
		// Field container transitions.
		if block.FieldKind != "" && block.FieldIndex != r.openFieldIndex {
			if r.openFieldIndex != -1 {
				r.closeFieldContainer()
			}
			r.openFieldContainer(block.FieldKind)
			r.openFieldIndex = block.FieldIndex
		} else if block.FieldKind == "" && r.openFieldIndex != -1 {
			r.closeFieldContainer()
			r.openFieldIndex = -1
		}
		if block.ContentControl != nil {
			r.closeAllLists()
			r.openSdtBlock(block.ContentControl)
		}
		switch block.Kind {
		case "paragraph":
			p := block.Paragraph
			if p != nil && p.Frame != nil {
				if isDropCapFrameParagraph(p) {
					r.closeAllLists()
					r.renderDropCapFramePara(p)
					if block.ContentControl != nil {
						r.closeSdtBlock()
					}
					continue
				}
				// In nested contexts (table cells, text boxes, headers,
				// footers) render frame paragraphs inline with relative
				// positioning so they stay within their container instead
				// of floating at the page overlay level.
				if r.frameDepth > 0 {
					r.renderFrameParaInline(p)
					if block.ContentControl != nil {
						r.closeSdtBlock()
					}
					continue
				}
				r.renderFrameParaTo(&r.overlay, p)
				if block.ContentControl != nil {
					r.closeSdtBlock()
				}
				continue
			}
			suppressBefore := layout.ContextualSpacingSuppresses(p, adjacentParagraph(blocks, i, -1))
			suppressAfter := layout.ContextualSpacingSuppresses(p, adjacentParagraph(blocks, i, +1))
			r.renderParagraph(p, suppressBefore, suppressAfter)
		case "table":
			r.closeAllLists()
			r.renderPendingMaterializedNoteTarget()
			r.renderTable(block.Table)
		case "altChunk":
			r.closeAllLists()
			r.renderPendingMaterializedNoteTarget()
			r.renderAltChunk(block.AltChunk)
		}
		if block.ContentControl != nil {
			r.closeSdtBlock()
		}
		r.pendingNoteTarget = nil
	}
	if r.openFieldIndex != -1 {
		r.closeFieldContainer()
		r.openFieldIndex = -1
	}
}

func (r *htmlRenderer) renderPendingMaterializedNoteTarget() {
	if r.pendingNoteTarget == nil {
		return
	}
	block := *r.pendingNoteTarget
	r.pendingNoteTarget = nil
	r.renderMaterializedNoteTarget(block)
}

func (r *htmlRenderer) renderMaterializedNoteTarget(block resolve.Block) {
	if block.NoteID <= 0 {
		return
	}
	var prefix, role, label string
	switch block.NoteKind {
	case "footnote":
		prefix, role, label = "fn", "doc-footnote", "Back to footnote reference"
	case "endnote":
		prefix, role, label = "en", "doc-endnote", "Back to endnote reference"
	default:
		return
	}
	fmt.Fprintf(&r.buf, `<span class="note-target" id="%s%d" role="%s"><a class="note-backlink" href="#%sref%d" aria-label=%q>↩</a></span>`, prefix, block.NoteID, role, prefix, block.NoteID, label)
}

func (r *htmlRenderer) renderAltChunk(ac *resolve.AltChunk) {
	if ac == nil || len(ac.Data) == 0 {
		return
	}
	switch strings.ToLower(strings.TrimSpace(ac.ContentType)) {
	case "text/html", "application/xhtml+xml":
		// Emit raw HTML content directly. We wrap it in a div so it stays
		// within the document flow, but otherwise pass it through unchanged.
		r.buf.WriteString(`<div class="word-altchunk">`)
		r.buf.Write(ac.Data)
		r.buf.WriteString("</div>\n")
	case "text/plain":
		r.buf.WriteString(`<div class="word-altchunk"><pre>`)
		r.buf.WriteString(html.EscapeString(string(ac.Data)))
		r.buf.WriteString("</pre></div>\n")
	default:
		// Unsupported altChunk content type — emit a placeholder comment.
		fmt.Fprintf(&r.buf, "<!-- altChunk (%s) -->\n", html.EscapeString(ac.ContentType))
	}
}

// adjacentParagraph returns the nearest Paragraph in blocks at offset dir (-1 or +1) from i,
// skipping non-paragraph blocks.
func adjacentParagraph(blocks []resolve.Block, i, dir int) *resolve.Paragraph {
	j := i + dir
	if j >= 0 && j < len(blocks) && blocks[j].Kind == "paragraph" {
		return blocks[j].Paragraph
	}
	return nil
}

func (r *htmlRenderer) renderParagraph(p *resolve.Paragraph, suppressBefore, suppressAfter bool) {
	if p == nil {
		return
	}
	r.renderParagraphSegment(p, resolve.CanonicalVisibleRuns(p), true, suppressBefore, suppressAfter, false, false, false)
}

func (r *htmlRenderer) renderParagraphSegment(p *resolve.Paragraph, runs []resolve.Run, includeBookmarks bool, suppressBefore, suppressAfter, suppressListMarker, suppressFirstLineIndent, forceColumnBreakBefore bool) {
	r.renderParagraphSegmentContent(p, runs, includeBookmarks, suppressBefore, suppressAfter, suppressListMarker, suppressFirstLineIndent, forceColumnBreakBefore, "", func() {
		r.renderRuns(runs)
	})
}

func (r *htmlRenderer) renderParagraphSegmentContent(p *resolve.Paragraph, runs []resolve.Run, includeBookmarks bool, suppressBefore, suppressAfter, suppressListMarker, suppressFirstLineIndent, forceColumnBreakBefore bool, extraStyle string, renderContent func()) {
	if p == nil {
		return
	}
	if p.DefaultRunFormat != nil && p.DefaultRunFormat.Vanish != nil && *p.DefaultRunFormat.Vanish {
		return
	}
	r.ensurePage()

	// List items take their own path.
	if p.List != nil && !suppressListMarker {
		r.renderListItemSegmentContent(p, runs, includeBookmarks, extraStyle, renderContent)
		return
	}

	r.closeAllLists()

	needsContainer := paragraphNeedsBlockContainerRuns(runs)
	level := headingLevel(p.StyleID, p.StyleName)
	tag := "p"
	if p.Format.BlockQuote {
		tag = "blockquote"
	} else if level > 0 {
		tag = "h" + strconv.Itoa(level)
	} else if needsContainer {
		tag = "div"
	}

	styleAttr := paragraphStyleAttrForParagraph(p, suppressBefore, suppressAfter, suppressFirstLineIndent)
	if p.Format.SnapToGrid && docGridIsActive(r.docGrid) {
		if r.docGrid.LinePitch > 0 {
			styleAttr = appendInlineStyleAttr(styleAttr, fmt.Sprintf("line-height:%.2fpt", float64(r.docGrid.LinePitch)/20.0))
		}
		if r.docGrid.CharSpace != 0 {
			styleAttr = appendInlineStyleAttr(styleAttr, fmt.Sprintf("letter-spacing:%.2fpt", float64(r.docGrid.CharSpace)/20.0))
		}
	}
	if forceColumnBreakBefore {
		styleAttr = appendInlineStyleAttr(styleAttr, "break-before:column")
	}
	if extraStyle != "" {
		styleAttr = appendInlineStyleAttr(styleAttr, extraStyle)
	}
	if r.debugStyleColors {
		styleAttr = appendInlineStyleAttr(styleAttr, styleDebugBgColor(p.Index))
	}
	dirAttr := ""
	if p.Format.Bidi {
		dirAttr = ` dir="rtl"`
	}
	// All floating objects are now routed to the page overlay with layout-computed
	// page-absolute coordinates, so paragraphs never need position:relative for
	// anchoring. Empty/anchor-only paragraphs still need min-height so they occupy
	// the same vertical space Word allocates for them.
	if paragraphIsEmptyOrAnchorOnly(runs) {
		styleAttr = appendInlineStyleAttr(styleAttr, fmt.Sprintf("min-height:%.2fpt", r.emptyParaMinHeightPt(p)))
	}
	dataStyle := ""
	if p.StyleID != "" {
		dataStyle = fmt.Sprintf(" data-style=%q", p.StyleID)
	}
	propRev := revisionAttrs(p.PropertyRevision)

	fmt.Fprintf(&r.buf, "<%s%s%s%s%s>", tag, styleAttr, dirAttr, dataStyle, propRev)
	r.renderPendingMaterializedNoteTarget()
	r.renderMoveRangeBoundaries(p, true)
	if includeBookmarks {
		r.renderCommentRangeStarts(p)
		r.renderBookmarkAnchorsAndSpanMarkers(p)
	}
	// Emit line number annotation when line numbering is active and this is
	// the start (not a continuation fragment) of a paragraph.
	if r.lineNumbering != nil && includeBookmarks && !p.Format.NoLineNumber {
		ln := r.lineNumbering
		n := r.lineNumber
		r.lineNumber++
		countBy := ln.CountBy
		if countBy <= 0 {
			countBy = 1
		}
		// Only display the number on lines that are multiples of countBy.
		display := ""
		if (n-ln.Start)%countBy == 0 {
			display = strconv.Itoa(n)
		}
		fmt.Fprintf(&r.buf, "<span class=\"ln\" aria-hidden=\"true\">%s</span>", html.EscapeString(display))
	}
	savedParagraph := r.currentParagraph
	r.currentParagraph = p
	r.resetTabState(p)
	defer r.clearTabState()
	r.advanceState.Reset(r.tabEstimatedX)
	r.renderPageTopFlowImagesForCurrentParagraph()
	if renderContent != nil {
		renderContent()
	}
	if r.showFormattingMarks {
		if p.DefaultRunFormat != nil {
			styles := r.runFormatStyles(*p.DefaultRunFormat)
			if len(styles) > 0 {
				fmt.Fprintf(&r.buf, `<span class="fmt-mark" aria-hidden="true" style=%q>¶</span>`, strings.Join(styles, ";"))
			} else {
				r.buf.WriteString(`<span class="fmt-mark" aria-hidden="true">¶</span>`)
			}
		} else {
			r.buf.WriteString(`<span class="fmt-mark" aria-hidden="true">¶</span>`)
		}
	}
	r.renderMoveRangeBoundaries(p, false)
	if includeBookmarks {
		r.renderCommentRangeEnds(p)
	}
	r.currentParagraph = savedParagraph
	fmt.Fprintf(&r.buf, "</%s>\n", tag)
}

func (r *htmlRenderer) renderListItemSegment(p *resolve.Paragraph, runs []resolve.Run, includeBookmarks bool) {
	r.renderListItemSegmentContent(p, runs, includeBookmarks, "", func() {
		r.renderRuns(runs)
	})
}

func (r *htmlRenderer) renderListItemSegmentContent(p *resolve.Paragraph, runs []resolve.Run, includeBookmarks bool, extraStyle string, renderContent func()) {
	r.ensurePage()
	list := p.List
	tag := "ul"
	if list.Kind == "ordered" {
		tag = "ol"
	}
	depth := list.Level + 1 // 1-based
	if depth < 1 {
		depth = 1
	}

	numID := 0
	if p.Numbering != nil {
		numID = p.Numbering.NumID
	}

	// Close deeper lists first. Each nested list belongs inside the parent
	// list item, so close the child <li> before its list container and leave
	// the parent's <li> open until the next sibling (or the whole list) closes.
	for len(r.listStack) > depth {
		top := r.listStack[len(r.listStack)-1]
		if top.liOpen {
			r.buf.WriteString("</li>\n")
		}
		fmt.Fprintf(&r.buf, "</%s>\n", top.tag)
		r.listStack = r.listStack[:len(r.listStack)-1]
	}

	// A new item at the same depth is a sibling: finish the previous <li>.
	if len(r.listStack) == depth && depth > 0 {
		top := &r.listStack[depth-1]
		if top.numID != numID || top.tag != tag {
			if top.liOpen {
				r.buf.WriteString("</li>\n")
			}
			fmt.Fprintf(&r.buf, "</%s>\n", top.tag)
			r.listStack = r.listStack[:depth-1]
		} else if top.liOpen {
			r.buf.WriteString("</li>\n")
			top.liOpen = false
		}
	}

	// Open missing levels. In well-formed Word numbering, a deeper level
	// follows a parent item, which remains open and therefore owns this list.
	for len(r.listStack) < depth {
		atTarget := len(r.listStack) == depth-1
		listOpenTag := listOpenElement(tag, list, atTarget)
		fmt.Fprint(&r.buf, listOpenTag)
		r.listStack = append(r.listStack, listFrame{tag: tag, numID: numID})
	}

	liStyle := paragraphStyleAttrWithHangingForSource(p.Format, true, true, false, true, p.Source != nil)
	// Always render Word list markers explicitly. Native browser markers are
	// positioned relative to the CSS list box and interact badly with Word's
	// hanging-indent geometry (especially legacy DOC bullets), which can place
	// the bullet on top of the first characters of the item text.
	liStyle = appendInlineStyleAttr(liStyle, "list-style-type:none")
	if extraStyle != "" {
		liStyle = appendInlineStyleAttr(liStyle, extraStyle)
	}
	if p.Format.SnapToGrid && docGridIsActive(r.docGrid) {
		if r.docGrid.LinePitch > 0 {
			liStyle = appendInlineStyleAttr(liStyle, fmt.Sprintf("line-height:%.2fpt", float64(r.docGrid.LinePitch)/20.0))
		}
		if r.docGrid.CharSpace != 0 {
			liStyle = appendInlineStyleAttr(liStyle, fmt.Sprintf("letter-spacing:%.2fpt", float64(r.docGrid.CharSpace)/20.0))
		}
	}
	if r.debugStyleColors {
		liStyle = appendInlineStyleAttr(liStyle, styleDebugBgColor(p.Index))
	}

	fmt.Fprintf(&r.buf, "<li%s>", liStyle)
	r.listStack[len(r.listStack)-1].liOpen = true
	r.renderPendingMaterializedNoteTarget()

	// Word list labels are rendered explicitly for both ordered and bullet lists.
	// Besides supporting compound numbering, this keeps bullet placement in the
	// same Word hanging-indent coordinate system as the item text instead of
	// delegating marker geometry to the browser. Marker run styling is isolated
	// to the marker so symbol-font bullets (Wingdings/Symbol) keep their glyph.
	marker, bullet := r.resolvedListMarker(p)
	if p.List != nil && p.List.Picture != nil && !r.physicalInlineListMarker {
		markerClass := "word-list-marker"
		if strings.Contains(extraStyle, "position:relative") {
			markerClass += " word-physical-list-marker"
		}
		r.renderPictureBullet(p.List.Picture, markerClass+" word-list-bullet")
	} else if marker != "" && !r.physicalInlineListMarker {
		markerStyle := r.listMarkerStyle(p, runs)
		markerClass := "word-list-marker"
		// Physical paragraph slots that cannot host an inline marker keep the
		// legacy gutter fallback. The normal physical paragraph path paints the
		// marker inside its first measured line instead.
		if strings.Contains(extraStyle, "position:relative") {
			markerClass += " word-physical-list-marker"
		}
		if bullet {
			markerClass += " word-list-bullet"
		}
		ariaHidden := ""
		if bullet {
			ariaHidden = ` aria-hidden="true"`
		}
		if markerStyle != "" {
			fmt.Fprintf(&r.buf, `<span class=%q style=%q%s>%s</span>`, markerClass, markerStyle, ariaHidden, html.EscapeString(marker))
		} else {
			fmt.Fprintf(&r.buf, `<span class=%q%s>%s</span>`, markerClass, ariaHidden, html.EscapeString(marker))
		}
	}

	if includeBookmarks {
		r.renderCommentRangeStarts(p)
		r.renderBookmarkAnchorsAndSpanMarkers(p)
	}
	savedParagraph := r.currentParagraph
	r.currentParagraph = p
	r.resetTabState(p)
	defer r.clearTabState()
	r.advanceState.Reset(r.tabEstimatedX)
	r.renderPageTopFlowImagesForCurrentParagraph()
	if renderContent != nil {
		renderContent()
	}
	if includeBookmarks {
		r.renderCommentRangeEnds(p)
	}
	r.currentParagraph = savedParagraph
	// Deliberately keep </li> open. A following deeper list must be a child of
	// this item; the sibling/ascend/closeAllLists paths close it later.
}

func (r *htmlRenderer) renderPictureBullet(img *resolve.Image, className string) {
	if img == nil || !isBrowserImage(img.MIMEType) || len(img.Data) == 0 {
		return
	}
	width := float64(img.Width) / 12700.0
	height := float64(img.Height) / 12700.0
	if width <= 0 {
		width = 9
	}
	if height <= 0 {
		height = 9
	}
	fmt.Fprintf(&r.buf, `<img class=%q src=%q alt="" aria-hidden="true" style="display:inline-block;width:%.2fpt;height:%.2fpt;object-fit:contain;vertical-align:middle">`, className, r.imageSource(img), width, height)
}

func (r *htmlRenderer) bookmarkRenderedOnAdjacentParagraph(p *resolve.Paragraph, bookmark string, dir int) bool {
	if p == nil || bookmark == "" {
		return false
	}
	pos, ok := r.paragraphSeqByIdx[p.Index]
	if !ok {
		return false
	}
	next := pos + dir
	if next < 0 || next >= len(r.paragraphSeq) {
		return false
	}
	adj := r.paragraphSeq[next]
	if adj == nil {
		return false
	}
	for _, bm := range adj.Bookmarks {
		if bm == bookmark {
			return true
		}
	}
	return false
}

// emitInlineMoveBoundary emits an empty <span> marker for a single inline
// move-range boundary. isStart=true emits the start marker, otherwise the end.
func (r *htmlRenderer) emitInlineMoveBoundary(imr resolve.InlineMoveRange, isStart bool) {
	if isStart {
		cls := "word-move-from-range-start"
		if r.hasMoveToRangeWithID(imr.ID) {
			cls = "word-move-to-range-start"
		}
		fmt.Fprintf(&r.buf, `<span class=%q data-move-id="%d" data-move-author=%q data-move-date=%q data-move-name=%q></span>`,
			cls, imr.ID, html.EscapeString(imr.Author), html.EscapeString(imr.Date), html.EscapeString(imr.Name))
	} else {
		cls := "word-move-from-range-end"
		if r.hasMoveToRangeWithID(imr.ID) {
			cls = "word-move-to-range-end"
		}
		fmt.Fprintf(&r.buf, `<span class=%q data-move-id="%d"></span>`, cls, imr.ID)
	}
}

// hasMoveToRangeWithID returns true if the current paragraph has a MoveToRange
// with the given ID, used to distinguish move-to from move-from boundaries.
func (r *htmlRenderer) hasMoveToRangeWithID(id int) bool {
	if r.currentParagraph == nil {
		return false
	}
	for _, mrs := range r.currentParagraph.MoveToRangeStarts {
		if mrs.ID == id {
			return true
		}
	}
	return false
}

// renderMoveRangeBoundaries emits empty <span> markers for tracked-change move
// range boundaries that appear in this paragraph. When start=true it emits starts
// (w:moveFromRangeStart / w:moveToRangeStart); when start=false it emits ends.
// When inline position correlation is available (InlineMoveRanges non-empty),
// paragraph-boundary emission is skipped to avoid duplicate markers.
func (r *htmlRenderer) renderMoveRangeBoundaries(p *resolve.Paragraph, start bool) {
	if p == nil {
		return
	}
	if len(p.InlineMoveRanges) > 0 {
		return
	}
	if start {
		for _, mrs := range p.MoveFromRangeStarts {
			fmt.Fprintf(&r.buf, `<span class="word-move-from-range-start" data-move-id="%d" data-move-author=%q data-move-date=%q data-move-name=%q></span>`, mrs.ID, html.EscapeString(mrs.Author), html.EscapeString(mrs.Date), html.EscapeString(mrs.Name))
		}
		for _, mrs := range p.MoveToRangeStarts {
			fmt.Fprintf(&r.buf, `<span class="word-move-to-range-start" data-move-id="%d" data-move-author=%q data-move-date=%q data-move-name=%q></span>`, mrs.ID, html.EscapeString(mrs.Author), html.EscapeString(mrs.Date), html.EscapeString(mrs.Name))
		}
		for _, mrs := range p.CustomXmlInsRangeStarts {
			fmt.Fprintf(&r.buf, `<span class="word-custom-xml-ins-range-start" data-range-id="%d" data-range-author=%q data-range-date=%q></span>`, mrs.ID, html.EscapeString(mrs.Author), html.EscapeString(mrs.Date))
		}
		for _, mrs := range p.CustomXmlDelRangeStarts {
			fmt.Fprintf(&r.buf, `<span class="word-custom-xml-del-range-start" data-range-id="%d" data-range-author=%q data-range-date=%q></span>`, mrs.ID, html.EscapeString(mrs.Author), html.EscapeString(mrs.Date))
		}
		for _, mrs := range p.CustomXmlMoveFromRangeStarts {
			fmt.Fprintf(&r.buf, `<span class="word-custom-xml-move-from-range-start" data-range-id="%d" data-range-author=%q data-range-date=%q data-range-name=%q></span>`, mrs.ID, html.EscapeString(mrs.Author), html.EscapeString(mrs.Date), html.EscapeString(mrs.Name))
		}
		for _, mrs := range p.CustomXmlMoveToRangeStarts {
			fmt.Fprintf(&r.buf, `<span class="word-custom-xml-move-to-range-start" data-range-id="%d" data-range-author=%q data-range-date=%q data-range-name=%q></span>`, mrs.ID, html.EscapeString(mrs.Author), html.EscapeString(mrs.Date), html.EscapeString(mrs.Name))
		}
	} else {
		for _, id := range p.MoveFromRangeEnds {
			fmt.Fprintf(&r.buf, `<span class="word-move-from-range-end" data-move-id="%d"></span>`, id)
		}
		for _, id := range p.MoveToRangeEnds {
			fmt.Fprintf(&r.buf, `<span class="word-move-to-range-end" data-move-id="%d"></span>`, id)
		}
		for _, pr := range p.PermRangeStarts {
			fmt.Fprintf(&r.buf, `<span class="word-perm-range-start" data-perm-id="%d" data-perm-ed-grp=%q data-perm-ed=%q data-perm-col-first="%d" data-perm-col-last="%d"></span>`, pr.ID, html.EscapeString(pr.EdGrp), html.EscapeString(pr.Ed), pr.ColFirst, pr.ColLast)
		}
		for _, id := range p.PermRangeEnds {
			fmt.Fprintf(&r.buf, `<span class="word-perm-range-end" data-perm-id="%d"></span>`, id)
		}
		for _, id := range p.CustomXmlInsRangeEnds {
			fmt.Fprintf(&r.buf, `<span class="word-custom-xml-ins-range-end" data-range-id="%d"></span>`, id)
		}
		for _, id := range p.CustomXmlDelRangeEnds {
			fmt.Fprintf(&r.buf, `<span class="word-custom-xml-del-range-end" data-range-id="%d"></span>`, id)
		}
		for _, id := range p.CustomXmlMoveFromRangeEnds {
			fmt.Fprintf(&r.buf, `<span class="word-custom-xml-move-from-range-end" data-range-id="%d"></span>`, id)
		}
		for _, id := range p.CustomXmlMoveToRangeEnds {
			fmt.Fprintf(&r.buf, `<span class="word-custom-xml-move-to-range-end" data-range-id="%d"></span>`, id)
		}
	}
}

func (r *htmlRenderer) renderCommentRangeStarts(p *resolve.Paragraph) {
	if p == nil {
		return
	}
	for _, id := range p.CommentRangeStarts {
		fmt.Fprintf(&r.buf, `<span class="word-comment-range-start" data-comment-id="%d"></span>`, id)
	}
}

func (r *htmlRenderer) renderCommentRangeEnds(p *resolve.Paragraph) {
	if p == nil {
		return
	}
	for _, id := range p.CommentRangeEnds {
		fmt.Fprintf(&r.buf, `<span class="word-comment-range-end" data-comment-id="%d"></span>`, id)
	}
}

func (r *htmlRenderer) renderBookmarkAnchorsAndSpanMarkers(p *resolve.Paragraph) {
	if p == nil || len(p.Bookmarks) == 0 {
		return
	}
	seen := map[string]bool{}
	for _, bm := range p.Bookmarks {
		if bm == "" || seen[bm] {
			continue
		}
		seen[bm] = true
		isStart := !r.bookmarkRenderedOnAdjacentParagraph(p, bm, -1)
		isEnd := !r.bookmarkRenderedOnAdjacentParagraph(p, bm, +1)
		if isStart {
			fmt.Fprintf(&r.buf, "<a id=%q></a>", bm)
			fmt.Fprintf(&r.buf, `<span class="word-bookmark-span-start" data-bookmark=%q></span>`, bm)
		}
		if isEnd {
			fmt.Fprintf(&r.buf, `<span class="word-bookmark-span-end" data-bookmark=%q></span>`, bm)
		}
	}
}

func (r *htmlRenderer) closeAllLists() {
	for i := len(r.listStack) - 1; i >= 0; i-- {
		if r.listStack[i].liOpen {
			r.buf.WriteString("</li>\n")
		}
		fmt.Fprintf(&r.buf, "</%s>\n", r.listStack[i].tag)
	}
	r.listStack = r.listStack[:0]
}

// renderNotes emits notes that are not already rendered in the paginated page body.
// Footnotes and endnotes are materialized into page frames by the layout layer;
// only comments remain in the trailing aside.
func (r *htmlRenderer) renderNotes() {
	hasNotes := len(r.doc.Comments) > 0
	if !hasNotes {
		return
	}
	r.buf.WriteString("<aside class=\"notes\">\n")
	if len(r.doc.Comments) > 0 {
		r.buf.WriteString("<section class=\"comments\">\n<hr>\n<ol>\n")
		ids := make([]int, 0, len(r.doc.Comments))
		for id := range r.doc.Comments {
			ids = append(ids, id)
		}
		sortInts(ids)
		for _, id := range ids {
			story := r.doc.Comments[id]
			fmt.Fprintf(&r.buf, "<li id=\"comment%d\" role=\"note\"><a class=\"note-backlink\" href=\"#commentref%d\" aria-label=\"Back to comment reference\">↩</a> ", id, id)
			if story.Author != "" || story.Date != "" {
				r.buf.WriteString("<span class=\"comment-meta\">")
				if story.Author != "" {
					fmt.Fprintf(&r.buf, "<span class=\"comment-author\">%s</span>", html.EscapeString(story.Author))
				}
				if story.Date != "" {
					fmt.Fprintf(&r.buf, "<time class=\"comment-date\">%s</time>", html.EscapeString(story.Date))
				}
				r.buf.WriteString("</span> ")
			}
			r.renderNoteStory(story)
			r.buf.WriteString("</li>\n")
		}
		r.buf.WriteString("</ol>\n</section>\n")
	}
	r.buf.WriteString("</aside>\n")
}

func (r *htmlRenderer) renderNoteStory(story resolve.Story) {
	for _, block := range story.Blocks {
		switch block.Kind {
		case "paragraph":
			r.advanceState.Reset(r.tabEstimatedX)
			r.renderRuns(block.Paragraph.Runs)
		case "table":
			r.renderTable(block.Table)
		}
	}
	// Fallback for stories without a Blocks slice.
	if len(story.Blocks) == 0 {
		for _, p := range story.Paragraphs {
			r.advanceState.Reset(r.tabEstimatedX)
			r.renderRuns(p.Runs)
		}
	}
}

// sortInts sorts a slice of ints in ascending order (avoid importing sort for a simple case).
func sortInts(s []int) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

func (r *htmlRenderer) renderTableFragment(t *resolve.Table, fragment *layout.TableFragment) {
	if t == nil || fragment == nil {
		r.renderTable(t)
		return
	}
	start := fragment.StartRow
	end := fragment.EndRow
	if start < 0 {
		start = 0
	}
	if end > len(t.Rows) {
		end = len(t.Rows)
	}
	if start >= end {
		return
	}

	rows := make([]resolve.TableRow, 0, end-start+4)
	if fragment.RepeatHeaders {
		headerEnd := 0
		for headerEnd < len(t.Rows) && t.Rows[headerEnd].Format.IsHeader {
			headerEnd++
		}
		rows = append(rows, t.Rows[:headerEnd]...)
	}
	rows = append(rows, t.Rows[start:end]...)

	copyTable := *t
	copyTable.Rows = rows
	if fragment.IsContinuation {
		// Captions belong to the table as a whole and must not be repeated on
		// continuation pages. Repeated table-header rows are handled above.
		copyTable.Format.Caption = ""
	}
	r.renderTable(&copyTable)
}

func (r *htmlRenderer) renderTable(t *resolve.Table) {
	r.renderPendingMaterializedNoteTarget()
	if t == nil {
		return
	}
	r.ensurePage()

	var tableStyles []string
	var tableAttrs []string
	f := t.Format
	availableWidthPt := r.pageWidthPt - r.marginLeftPt - r.marginRightPt
	effectiveWidthPt := layout.TableWidthPtForDocument(r.doc, t, availableWidthPt)
	legacyWidthClamped := r.doc != nil && r.doc.Source != nil && r.doc.Source.Version != 0 && r.doc.Source.Version < 0x00C1 &&
		availableWidthPt > 0 && effectiveWidthPt+0.01 < layout.TableWidthPt(t, availableWidthPt)
	if legacyWidthClamped {
		tableStyles = append(tableStyles, fmt.Sprintf("width:%.1fpt", effectiveWidthPt))
	} else if f.Width != nil && f.WidthType == "dxa" {
		tableStyles = append(tableStyles, fmt.Sprintf("width:%.1fpt", twipsToPoints(*f.Width)))
	} else if f.Width != nil && f.WidthType == "pct" {
		tableStyles = append(tableStyles, fmt.Sprintf("width:%.1f%%", float64(*f.Width)/50.0))
	}
	if f.Layout == "fixed" || len(f.ColumnWidths) > 0 {
		tableStyles = append(tableStyles, "table-layout:fixed")
	}
	switch f.Alignment {
	case "center":
		tableStyles = append(tableStyles, "margin-left:auto;margin-right:auto")
	case "right":
		tableStyles = append(tableStyles, "margin-left:auto")
	}
	if f.Shading != "" {
		tableStyles = append(tableStyles, fmt.Sprintf("background-color:#%s", f.Shading))
	}
	if f.Indent != 0 {
		tableStyles = append(tableStyles, fmt.Sprintf("margin-left:%.1fpt", twipsToPoints(f.Indent)))
	}
	if hasVisibleTableBorders(f.Borders) {
		tableStyles = append(tableStyles, "border-collapse:collapse")
		tableStyles = appendTableBorderStyles(tableStyles, f.Borders)
	}
	if f.BidiVisual {
		tableStyles = append(tableStyles, "direction:rtl")
	}
	if f.Description != "" {
		tableAttrs = append(tableAttrs, fmt.Sprintf(` title=%q`, f.Description))
	}

	r.buf.WriteString("<table")
	if len(tableStyles) > 0 {
		fmt.Fprintf(&r.buf, " style=%q", strings.Join(tableStyles, ";"))
	}
	for _, attr := range tableAttrs {
		r.buf.WriteString(attr)
	}
	if t.PropertyRevision != nil {
		r.buf.WriteString(revisionAttrs(t.PropertyRevision))
	}
	r.buf.WriteString(">\n")
	if f.Caption != "" {
		fmt.Fprintf(&r.buf, "<caption>%s</caption>\n", html.EscapeString(f.Caption))
	}
	if len(f.ColumnWidths) > 0 {
		r.buf.WriteString("<colgroup>")
		columnScale := 1.0
		if legacyWidthClamped {
			rawWidthPt := 0.0
			for _, w := range f.ColumnWidths {
				rawWidthPt += twipsToPoints(w)
			}
			if rawWidthPt > 0 {
				columnScale = effectiveWidthPt / rawWidthPt
			}
		}
		for _, w := range f.ColumnWidths {
			fmt.Fprintf(&r.buf, `<col style="width:%.1fpt">`, twipsToPoints(w)*columnScale)
		}
		r.buf.WriteString("</colgroup>\n")
	}
	r.withLineNumberingSuppressed(func() {
		rowSpans := tableVMergeRowSpans(t.Rows)
		// Find boundary between header and body rows.
		headerEnd := 0
		for headerEnd < len(t.Rows) && t.Rows[headerEnd].Format.IsHeader {
			headerEnd++
		}
		if headerEnd > 0 {
			r.buf.WriteString("<thead>\n")
			for i := 0; i < headerEnd; i++ {
				r.renderTableRow(t.Rows[i], rowSpans[i], f, i, len(t.Rows), true)
			}
			r.buf.WriteString("</thead>\n")
		}
		if headerEnd < len(t.Rows) {
			r.buf.WriteString("<tbody>\n")
			for i := headerEnd; i < len(t.Rows); i++ {
				r.renderTableRow(t.Rows[i], rowSpans[i], f, i, len(t.Rows), false)
			}
			r.buf.WriteString("</tbody>\n")
		}
	})
	r.buf.WriteString("</table>\n")
}

func (r *htmlRenderer) renderTableRow(row resolve.TableRow, rowSpans []int, tf resolve.TableFormat, rowIndex, rowCount int, isHeader bool) {
	if row.Format.Hidden {
		return
	}
	var rowStyles []string
	rf := row.Format
	if rf.CantSplit {
		rowStyles = append(rowStyles, "break-inside:avoid-page")
	}
	if rf.Height != nil && *rf.Height > 0 {
		switch rf.HeightRule {
		case "exact":
			rowStyles = append(rowStyles, fmt.Sprintf("height:%.1fpt", twipsToPoints(*rf.Height)))
		default:
			rowStyles = append(rowStyles, fmt.Sprintf("min-height:%.1fpt", twipsToPoints(*rf.Height)))
		}
	}
	if row.Override != nil && row.Override.Shading != "" {
		rowStyles = append(rowStyles, fmt.Sprintf("background-color:#%s", row.Override.Shading))
	}
	switch rf.Justification {
	case "center":
		rowStyles = append(rowStyles, "text-align:center")
	case "right":
		rowStyles = append(rowStyles, "text-align:right")
	}
	if len(rowStyles) > 0 {
		fmt.Fprintf(&r.buf, "<tr style=%q", strings.Join(rowStyles, ";"))
	} else {
		r.buf.WriteString("<tr")
	}
	if row.PropertyRevision != nil {
		r.buf.WriteString(revisionAttrs(row.PropertyRevision))
	}
	r.buf.WriteString(">\n")

	r.renderGridGapCell(row.Format.GridBefore, row.Format.WidthBefore, row.Format.WidthBeforeType, isHeader)

	// Compute effective colspans accounting for HMerge.
	effectiveSpans := make([]int, len(row.Cells))
	for i, cell := range row.Cells {
		span := cell.Format.GridSpan
		if span <= 0 {
			span = 1
		}
		effectiveSpans[i] = span
	}
	for i, cell := range row.Cells {
		if cell.Format.HMerge {
			effectiveSpans[i] = 0
			for j := i - 1; j >= 0; j-- {
				if effectiveSpans[j] > 0 {
					effectiveSpans[j]++
					break
				}
			}
		}
	}
	// Border inheritance and span clamping must use physical table-grid
	// positions, not row slice indexes. Merges and grid gaps make those differ.
	gridCols := len(tf.ColumnWidths)
	totalGridCols := 0
	for _, span := range effectiveSpans {
		if span > 0 {
			totalGridCols += span
		}
	}
	if gridCols > 0 {
		totalGridCols = gridCols
	}
	gridCol := rf.GridBefore
	for cellIndex, cell := range row.Cells {
		span := effectiveSpans[cellIndex]
		if span == 0 {
			continue
		}
		if gridCols > 0 {
			remaining := gridCols - rf.GridAfter - gridCol
			if remaining <= 0 {
				continue
			}
			if span > remaining {
				span = remaining
			}
		}
		rowSpan := 1
		if cellIndex < len(rowSpans) {
			rowSpan = rowSpans[cellIndex]
		}
		r.renderTableCell(cell, rowSpan, span, tf, rowIndex, rowCount, gridCol, totalGridCols, rf.HeightRule, rf.CellSpacing, row.Override, isHeader)
		gridCol += span
	}
	r.renderGridGapCell(row.Format.GridAfter, row.Format.WidthAfter, row.Format.WidthAfterType, isHeader)
	r.buf.WriteString("</tr>\n")
}

func (r *htmlRenderer) renderGridGapCell(span int, width *int, widthType string, isHeader bool) {
	if span <= 0 {
		return
	}
	colspan := ""
	if span > 1 {
		colspan = fmt.Sprintf(" colspan=\"%d\"", span)
	}
	var styles []string
	if width != nil && widthType == "dxa" {
		styles = append(styles, fmt.Sprintf("width:%.1fpt", twipsToPoints(*width)))
	} else if width != nil && widthType == "pct" {
		styles = append(styles, fmt.Sprintf("width:%.1f%%", float64(*width)/50.0))
	}
	styles = append(styles, "padding:0", "border:none")
	tag := "td"
	if isHeader {
		tag = "th"
	}
	if len(styles) > 0 {
		fmt.Fprintf(&r.buf, "<%s%s style=%q></%s>\n", tag, colspan, strings.Join(styles, ";"), tag)
		return
	}
	fmt.Fprintf(&r.buf, "<%s%s></%s>\n", tag, colspan, tag)
}

func (r *htmlRenderer) renderTableCell(cell resolve.TableCell, rowSpan int, colSpan int, tf resolve.TableFormat, rowIndex, rowCount, gridCol, gridColCount int, heightRule string, rowCellSpacing int, rowOverride *resolve.TableRowOverride, isHeader bool) {
	if rowSpan == 0 {
		return
	}
	var cellStyles []string
	cf := cell.Format
	if cf.Width != nil && cf.WidthType == "dxa" {
		cellStyles = append(cellStyles, fmt.Sprintf("width:%.1fpt", twipsToPoints(*cf.Width)))
	} else if cf.Width != nil && cf.WidthType == "pct" {
		cellStyles = append(cellStyles, fmt.Sprintf("width:%.1f%%", float64(*cf.Width)/50.0))
	}
	switch cf.VertAlign {
	case "center":
		cellStyles = append(cellStyles, "vertical-align:middle")
	case "bottom":
		cellStyles = append(cellStyles, "vertical-align:bottom")
	}
	cellStyles = append(cellStyles, wordTableCellTextDirectionCSS(cf.TextDirection)...)
	if cf.Shading != "" {
		cellStyles = append(cellStyles, fmt.Sprintf("background-color:#%s", cf.Shading))
	}
	effectiveTableBorders := tf.Borders
	if rowOverride != nil && rowOverride.Borders != nil {
		effectiveTableBorders = rowOverride.Borders
	}
	if borders := effectiveTableCellBorders(cf.Borders, effectiveTableBorders, rowIndex, rowCount, gridCol, gridColCount, colSpan); borders != nil {
		cellStyles = append(cellStyles, appendBorderStyleIfSet(nil, "border-top", borders.Top)...)
		cellStyles = append(cellStyles, appendBorderStyleIfSet(nil, "border-left", effectiveCellBorderSide(borders.Left, borders.Start))...)
		cellStyles = append(cellStyles, appendBorderStyleIfSet(nil, "border-bottom", borders.Bottom)...)
		cellStyles = append(cellStyles, appendBorderStyleIfSet(nil, "border-right", effectiveCellBorderSide(borders.Right, borders.End))...)
		cellStyles = append(cellStyles, cellDiagonalBorderCSS(borders)...)
	}
	// Apply cell-level margins if present, otherwise fall back to table-level defaults.
	// Horizontal padding uses dxaGapHalf (= CellSpacing/2) as a fallback when no
	// explicit left/right default is set, following POI WordToHtmlUtils.addTableCellProperties.
	padTop := tf.CellPadTop
	padBottom := tf.CellPadBottom
	padLeft := tf.CellPadLeft
	padRight := tf.CellPadRight
	// Cell margins are independent overrides; changing one side must not clear
	// table defaults on the other three sides.
	if cf.CellPadTop != 0 {
		padTop = cf.CellPadTop
	}
	if cf.CellPadBottom != 0 {
		padBottom = cf.CellPadBottom
	}
	if cf.CellPadLeft != 0 {
		padLeft = cf.CellPadLeft
	}
	if cf.CellPadRight != 0 {
		padRight = cf.CellPadRight
	}
	// Row-level tblPrEx cell margin overrides take precedence over table defaults.
	if rowOverride != nil {
		if rowOverride.CellPadTop != 0 {
			padTop = rowOverride.CellPadTop
		}
		if rowOverride.CellPadBottom != 0 {
			padBottom = rowOverride.CellPadBottom
		}
		if rowOverride.CellPadLeft != 0 {
			padLeft = rowOverride.CellPadLeft
		}
		if rowOverride.CellPadRight != 0 {
			padRight = rowOverride.CellPadRight
		}
	}
	effectiveCellSpacing := tf.CellSpacing
	if rowCellSpacing > 0 {
		effectiveCellSpacing = rowCellSpacing
	}
	if padLeft == 0 && effectiveCellSpacing > 0 {
		padLeft = effectiveCellSpacing / 2
	}
	if padRight == 0 && effectiveCellSpacing > 0 {
		padRight = effectiveCellSpacing / 2
	}
	if padTop > 0 || padBottom > 0 || padLeft > 0 || padRight > 0 {
		cellStyles = append(cellStyles, fmt.Sprintf("padding:%.1fpt %.1fpt %.1fpt %.1fpt",
			twipsToPoints(padTop), twipsToPoints(padRight),
			twipsToPoints(padBottom), twipsToPoints(padLeft)))
	}
	if cf.NoWrap {
		cellStyles = append(cellStyles, "white-space:nowrap")
	}
	if c3d := cf.Cell3D; c3d != nil {
		cellStyles = append(cellStyles, cell3DBoxShadowCSS(c3d)...)
	}

	if heightRule == "exact" {
		cellStyles = append(cellStyles, "overflow:hidden")
	}
	span := ""
	if colSpan > 1 {
		span = fmt.Sprintf(" colspan=\"%d\"", colSpan)
	}
	rowSpanAttr := ""
	if rowSpan > 1 {
		rowSpanAttr = fmt.Sprintf(" rowspan=\"%d\"", rowSpan)
	}

	tag := "td"
	if isHeader {
		tag = "th"
	}
	revAttr := revisionAttrs(cell.PropertyRevision)
	trackAttr := cellTrackingAttrs(cell)
	if len(cellStyles) > 0 {
		fmt.Fprintf(&r.buf, "<%s%s%s style=%q%s%s>", tag, span, rowSpanAttr, strings.Join(cellStyles, ";"), revAttr, trackAttr)
	} else {
		fmt.Fprintf(&r.buf, "<%s%s%s%s%s>", tag, span, rowSpanAttr, revAttr, trackAttr)
	}
	r.frameDepth++
	blocks := cell.Blocks
	if cf.HideMark && len(blocks) > 0 {
		last := blocks[len(blocks)-1]
		if last.Kind == "paragraph" && last.Paragraph != nil &&
			len(last.Paragraph.Runs) == 0 && last.Paragraph.Text == "" {
			blocks = blocks[:len(blocks)-1]
		}
	}
	r.renderBlocks(blocks)
	r.closeAllLists()
	fmt.Fprintf(&r.buf, "</%s>\n", tag)
	r.frameDepth--
}

func hasVisibleTableBorders(borders *resolve.TableBorders) bool {
	if borders == nil {
		return false
	}
	for _, side := range []*resolve.BorderSide{borders.Top, borders.Left, borders.Start, borders.Bottom, borders.Right, borders.End, borders.InsideH, borders.InsideV} {
		if side != nil && side.Val != "none" && side.Val != "nil" {
			return true
		}
	}
	return false
}

func appendTableBorderStyles(styles []string, borders *resolve.TableBorders) []string {
	if borders == nil {
		return styles
	}
	styles = appendBorderStyleIfSet(styles, "border-top", borders.Top)
	styles = appendBorderStyleIfSet(styles, "border-left", effectiveTableBorderSide(borders.Left, borders.Start))
	styles = appendBorderStyleIfSet(styles, "border-bottom", borders.Bottom)
	styles = appendBorderStyleIfSet(styles, "border-right", effectiveTableBorderSide(borders.Right, borders.End))
	return styles
}

func appendBorderStyleIfSet(styles []string, prop string, side *resolve.BorderSide) []string {
	if side == nil {
		return styles
	}
	return append(styles, borderSideCSS(prop, side)...)
}

func effectiveTableCellBorders(cellBorders *resolve.CellBorders, tableBorders *resolve.TableBorders, rowIndex, rowCount, gridCol, gridColCount, gridSpan int) *resolve.CellBorders {
	if cellBorders == nil && tableBorders == nil {
		return nil
	}
	out := cloneTableCellBorders(cellBorders)
	if tableBorders == nil {
		return out
	}
	if out == nil {
		out = &resolve.CellBorders{}
	}
	if out.Top == nil {
		if rowIndex == 0 {
			out.Top = cloneTableBorderSide(tableBorders.Top)
		} else {
			out.Top = cloneTableBorderSide(tableBorders.InsideH)
		}
	}
	if out.Bottom == nil {
		if rowIndex == rowCount-1 {
			out.Bottom = cloneTableBorderSide(tableBorders.Bottom)
		} else {
			out.Bottom = cloneTableBorderSide(tableBorders.InsideH)
		}
	}
	if out.Left == nil {
		if gridCol == 0 {
			out.Left = cloneTableBorderSide(tableBorders.Left)
		} else {
			out.Left = cloneTableBorderSide(tableBorders.InsideV)
		}
	}
	if out.Right == nil {
		if gridCol+gridSpan >= gridColCount {
			out.Right = cloneTableBorderSide(tableBorders.Right)
		} else {
			out.Right = cloneTableBorderSide(tableBorders.InsideV)
		}
	}
	if out.Top == nil && out.Left == nil && out.Bottom == nil && out.Right == nil && out.TL2BR == nil && out.TR2BL == nil {
		return nil
	}
	return out
}

func cloneTableCellBorders(borders *resolve.CellBorders) *resolve.CellBorders {
	if borders == nil {
		return nil
	}
	return &resolve.CellBorders{
		Top:    cloneTableBorderSide(borders.Top),
		Left:   cloneTableBorderSide(borders.Left),
		Start:  cloneTableBorderSide(borders.Start),
		Bottom: cloneTableBorderSide(borders.Bottom),
		Right:  cloneTableBorderSide(borders.Right),
		End:    cloneTableBorderSide(borders.End),
		TL2BR:  cloneTableBorderSide(borders.TL2BR),
		TR2BL:  cloneTableBorderSide(borders.TR2BL),
	}
}

func cloneTableBorderSide(side *resolve.BorderSide) *resolve.BorderSide {
	if side == nil {
		return nil
	}
	out := *side
	return &out
}

// cellDiagonalBorderCSS returns CSS declarations for table-cell diagonal borders
// (TL2BR and TR2BL) rendered as SVG background images.
func cellDiagonalBorderCSS(borders *resolve.CellBorders) []string {
	if borders == nil {
		return nil
	}
	var lines []string
	if line := svgDiagonalLine("0", "0", "100%", "100%", borders.TL2BR); line != "" {
		lines = append(lines, line)
	}
	if line := svgDiagonalLine("100%", "0", "0", "100%", borders.TR2BL); line != "" {
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		return nil
	}
	svg := `<svg xmlns="http://www.w3.org/2000/svg" width="100%" height="100%" preserveAspectRatio="none">` + strings.Join(lines, "") + `</svg>`
	uri := "url('data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte(svg)) + "')"
	return []string{
		"background-image:" + uri,
		"background-size:100% 100%",
		"background-repeat:no-repeat",
	}
}

// svgDiagonalLine returns an SVG <line> element for a single diagonal border side.
func svgDiagonalLine(x1, y1, x2, y2 string, side *resolve.BorderSide) string {
	if side == nil || side.Val == "none" || side.Val == "nil" {
		return ""
	}
	width := side.SizePt
	dashAttr := ""
	switch side.Val {
	case "dashed":
		dashAttr = fmt.Sprintf(` stroke-dasharray="%gpt,%gpt"`, width*4, width*2)
	case "dotted":
		dashAttr = fmt.Sprintf(` stroke-dasharray="0,%gpt" stroke-linecap="round"`, width*3)
	case "double":
		// Approximate double diagonal with a thicker line.
		width *= 1.5
	}
	color := "#000"
	if side.Color != "" {
		color = "#" + side.Color
	}
	return fmt.Sprintf(`<line x1="%s" y1="%s" x2="%s" y2="%s" stroke="%s" stroke-width="%gpt"%s/>`,
		x1, y1, x2, y2, color, width, dashAttr)
}

// effectiveCellBorderSide returns the physical side if set, otherwise the
// logical start/end side. This maps OOXML logical borders into CSS physical
// borders (start -> left, end -> right in LTR contexts).
func effectiveCellBorderSide(physical, logical *resolve.BorderSide) *resolve.BorderSide {
	if physical != nil {
		return physical
	}
	return logical
}

// effectiveTableBorderSide is the table-level equivalent of effectiveCellBorderSide.
func effectiveTableBorderSide(physical, logical *resolve.BorderSide) *resolve.BorderSide {
	if physical != nil {
		return physical
	}
	return logical
}

// effectiveParagraphBorderLeft returns the effective left border for a paragraph,
// preferring physical Left, then logical Start, then Bar.
func effectiveParagraphBorderLeft(b *resolve.ParagraphBorders) *resolve.BorderSide {
	if b.Left != nil {
		return b.Left
	}
	if b.Start != nil {
		return b.Start
	}
	return b.Bar
}

// effectiveParagraphBorderRight returns the effective right border for a paragraph,
// preferring physical Right, then logical End.
func effectiveParagraphBorderRight(b *resolve.ParagraphBorders) *resolve.BorderSide {
	if b.Right != nil {
		return b.Right
	}
	return b.End
}

// effectivePageBorderLeft returns the effective left border for a page,
// preferring physical Left, then logical Start.
func effectivePageBorderLeft(b *resolve.PageBorders) *resolve.BorderSide {
	if b.Left != nil {
		return b.Left
	}
	return b.Start
}

// effectivePageBorderRight returns the effective right border for a page,
// preferring physical Right, then logical End.
func effectivePageBorderRight(b *resolve.PageBorders) *resolve.BorderSide {
	if b.Right != nil {
		return b.Right
	}
	return b.End
}

type vMergeAnchor struct {
	rowIndex  int
	cellIndex int
}

func tableVMergeRowSpans(rows []resolve.TableRow) [][]int {
	rowSpans := make([][]int, len(rows))
	active := map[int]vMergeAnchor{}
	for rowIndex, row := range rows {
		rowSpans[rowIndex] = make([]int, len(row.Cells))
		gridIndex := row.Format.GridBefore
		for cellIndex, cell := range row.Cells {
			span := cell.Format.GridSpan
			if span <= 0 {
				span = 1
			}
			switch cell.Format.VMerge {
			case "continue":
				rowSpans[rowIndex][cellIndex] = 0
				if anchor, ok := active[gridIndex]; ok {
					rowSpans[anchor.rowIndex][anchor.cellIndex]++
				}
			case "restart":
				rowSpans[rowIndex][cellIndex] = 1
				anchor := vMergeAnchor{rowIndex: rowIndex, cellIndex: cellIndex}
				for offset := 0; offset < span; offset++ {
					active[gridIndex+offset] = anchor
				}
			default:
				rowSpans[rowIndex][cellIndex] = 1
				for offset := 0; offset < span; offset++ {
					delete(active, gridIndex+offset)
				}
			}
			gridIndex += span
		}
	}
	return rowSpans
}

// runFormatStyles returns the CSS style declarations for a resolved RunFormat.
// It is used both for normal text runs and for the paragraph-mark (pilcrow) span.
func (r *htmlRenderer) runFormatStyles(f resolve.RunFormat) []string {
	var styles []string
	if f.Bold != nil && *f.Bold {
		styles = append(styles, "font-weight:bold")
	}
	if f.BoldCS != nil && *f.BoldCS && ((f.RTL != nil && *f.RTL) || (f.ComplexScript != nil && *f.ComplexScript)) {
		styles = append(styles, "font-weight:bold")
	}
	if f.Italic != nil && *f.Italic {
		styles = append(styles, "font-style:italic")
	}
	if f.ItalicCS != nil && *f.ItalicCS && ((f.RTL != nil && *f.RTL) || (f.ComplexScript != nil && *f.ComplexScript)) {
		styles = append(styles, "font-style:italic")
	}
	if f.SmallCaps != nil && *f.SmallCaps {
		styles = append(styles, "font-variant:small-caps")
	}
	if f.Caps != nil && *f.Caps {
		styles = append(styles, "text-transform:uppercase")
	}
	if f.Strike != nil && *f.Strike {
		styles = append(styles, "text-decoration:line-through")
	}
	if f.DoubleStrike != nil && *f.DoubleStrike {
		styles = append(styles, "text-decoration:line-through double")
	}
	if f.Underline != "" && f.Underline != "none" {
		u := "text-decoration:underline"
		if us := cssUnderlineStyle(f.Underline); us != "" {
			u += " " + us
		}
		if f.UnderlineColor != "" {
			u += fmt.Sprintf(" #%s", f.UnderlineColor)
		}
		styles = append(styles, u)
	}
	if f.Color != "" && f.Color != "auto" {
		styles = append(styles, fmt.Sprintf("color:#%s", f.Color))
	}
	if fillColor := cssTextFillColor(f.TextFill); fillColor != "" {
		styles = append(styles, "color:"+fillColor)
	}
	if f.FontSize != nil {
		styles = append(styles, fmt.Sprintf("font-size:%.1fpt", float64(*f.FontSize)/2.0))
	}
	if f.FontSizeCS != nil && (f.FontSize == nil || *f.FontSizeCS != *f.FontSize) &&
		((f.RTL != nil && *f.RTL) || (f.ComplexScript != nil && *f.ComplexScript)) {
		styles = append(styles, fmt.Sprintf("font-size:%.1fpt", float64(*f.FontSizeCS)/2.0))
	}
	if fontFamily := effectiveFontFamily(f); fontFamily != "" {
		if inferredWeight := inferredFontFamilyWeight(fontFamily); inferredWeight != "" && (f.Bold == nil || !*f.Bold) {
			styles = append(styles, "font-weight:"+inferredWeight)
		}
		styles = append(styles, "font-family:"+r.cssFontFamily(fontFamily, !r.isEmbeddedFont(fontFamily)))
	}
	if f.Highlight != "" && f.Highlight != "none" {
		styles = append(styles, fmt.Sprintf("background-color:%s", highlightColor(f.Highlight)))
	}
	if f.Background != "" {
		styles = append(styles, fmt.Sprintf("background-color:#%s", f.Background))
	}
	if f.CharacterSpacing != nil && *f.CharacterSpacing != 0 {
		// w:spacing is in twentieths of a point; CSS letter-spacing uses pt.
		styles = append(styles, fmt.Sprintf("letter-spacing:%.2fpt", float64(*f.CharacterSpacing)/20.0))
	}
	if r.currentParagraph != nil && r.paragraphNeedsFontAlignCSS(r.currentParagraph) &&
		r.currentParagraph.Format.FontAlign != "" && r.currentParagraph.Format.FontAlign != "auto" &&
		(f.Position == nil || *f.Position == 0) {
		if va := cssFontAlign(r.currentParagraph.Format.FontAlign); va != "" {
			styles = append(styles, fmt.Sprintf("vertical-align:%s", va))
		}
	}
	if f.Position != nil && *f.Position != 0 {
		// w:position is in half-points; positive = raised, negative = lowered.
		styles = append(styles, fmt.Sprintf("vertical-align:%.1fpt", float64(*f.Position)/2.0))
	}
	if f.RTL != nil && *f.RTL {
		// w:rtl marks the run as right-to-left complex-script text.
		styles = append(styles, "direction:rtl;unicode-bidi:embed")
	}
	if stroke := cssTextOutlineStroke(f.TextOutline); stroke != "" {
		styles = append(styles, "-webkit-text-stroke:"+stroke)
	}
	if f.Outline != nil && *f.Outline {
		styles = append(styles, "-webkit-text-stroke:0.75pt currentColor")
	}
	var textShadows []string
	if shadow := cssRunTextShadow(f.TextShadow, f.TextGlow); shadow != "" {
		textShadows = append(textShadows, shadow)
	}
	if f.Shadow != nil && *f.Shadow {
		textShadows = append(textShadows, "1px 1px 2px rgba(0,0,0,0.4)")
	}
	if f.Emboss != nil && *f.Emboss {
		textShadows = append(textShadows, "-1px -1px 1px rgba(255,255,255,0.7), 1px 1px 1px rgba(0,0,0,0.5)")
	}
	if f.Imprint != nil && *f.Imprint {
		textShadows = append(textShadows, "1px 1px 1px rgba(255,255,255,0.7), -1px -1px 1px rgba(0,0,0,0.5)")
	}
	if len(textShadows) > 0 {
		styles = append(styles, "text-shadow:"+strings.Join(textShadows, ", "))
	}
	if f.TextSoftEdge != nil && f.TextSoftEdge.Radius > 0 {
		styles = append(styles, fmt.Sprintf("filter:blur(%.1fpt)", f.TextSoftEdge.Radius))
	}
	if reflect := cssTextReflection(f.TextReflection); reflect != "" {
		styles = append(styles, reflect)
	}
	if ligatures := cssLigatureVariant(f.Ligatures); ligatures != "" {
		styles = append(styles, "font-variant-ligatures:"+ligatures)
	}
	if numeric := cssNumericSpacingVariant(f.NumberSpacing); numeric != "" {
		styles = append(styles, "font-variant-numeric:"+numeric)
	}
	if featureSettings := cssStylisticSetFeatureSettings(f.StylisticSets); featureSettings != "" {
		styles = append(styles, "font-feature-settings:"+featureSettings)
	}
	styles = append(styles, cssTextEmphasis(f)...)
	switch f.TextAnimation {
	case "blinkBackground":
		styles = append(styles, "animation:word-blink 1s step-end infinite")
	case "lights":
		styles = append(styles, "animation:word-lights 1.5s ease-in-out infinite alternate")
	case "antsBlack":
		styles = append(styles, "outline:1px dashed black;animation:word-ants-black 0.6s linear infinite")
	case "antsRed":
		styles = append(styles, "outline:1px dashed red;animation:word-ants-red 0.6s linear infinite")
	case "shimmer":
		styles = append(styles, "animation:word-shimmer 2s ease-in-out infinite alternate")
	case "sparkle":
		styles = append(styles, "animation:word-sparkle 1.5s ease-in-out infinite")
	}
	if f.FELayout != nil {
		if f.FELayout.Vertical || f.FELayout.VerticalCompressed {
			styles = append(styles, "writing-mode:vertical-rl")
		}
		if f.FELayout.CombineCharacters {
			styles = append(styles, "text-combine-upright:all")
		}
	}
	if f.WebHidden != nil && *f.WebHidden {
		styles = append(styles, "display:none")
	}
	if f.Kerning != nil && *f.Kerning > 0 {
		styles = append(styles, "font-kerning:normal")
	}
	if f.CharScale != nil && *f.CharScale > 0 && *f.CharScale != 100 {
		styles = append(styles, fmt.Sprintf("transform:scaleX(%.2f)", float64(*f.CharScale)/100.0))
	}
	if f.Borders != nil {
		styles = append(styles, borderSideCSS("border-top", f.Borders.Top)...)
		styles = append(styles, borderSideCSS("border-left", f.Borders.Left)...)
		styles = append(styles, borderSideCSS("border-bottom", f.Borders.Bottom)...)
		styles = append(styles, borderSideCSS("border-right", f.Borders.Right)...)
	}
	return styles
}

func (r *htmlRenderer) renderRuns(runs []resolve.Run) {
	// Group consecutive runs with the same revision metadata so they share a
	// single <ins> / <del> wrapper instead of one per run.
	var currentRev struct {
		tag   string
		extra string
	}
	open := false
	closeRevision := func() {
		if open {
			fmt.Fprintf(&r.buf, "</%s>", currentRev.tag)
			open = false
		}
	}
	// Build lookup maps for inline move-range boundaries when available.
	var moveStarts, moveEnds map[int][]resolve.InlineMoveRange
	if r.currentParagraph != nil && len(r.currentParagraph.InlineMoveRanges) > 0 {
		moveStarts = make(map[int][]resolve.InlineMoveRange)
		moveEnds = make(map[int][]resolve.InlineMoveRange)
		for _, imr := range r.currentParagraph.InlineMoveRanges {
			moveStarts[imr.StartRunIndex] = append(moveStarts[imr.StartRunIndex], imr)
			moveEnds[imr.EndRunIndex] = append(moveEnds[imr.EndRunIndex], imr)
		}
	}
	for i := 0; i < len(runs); i++ {
		run := r.effectiveRunForRendering(runs, i)
		if run.Format.Vanish != nil && *run.Format.Vanish {
			closeRevision()
			continue
		}
		if run.Format.SpecVanish != nil && *run.Format.SpecVanish {
			closeRevision()
			continue
		}
		if run.Kind == "tab" {
			// Tabs are semantic layout content even when the paragraph has no custom
			// stops. renderTabAligned falls back to the document/default 0.5-inch
			// interval, so never let an ordinary-flow tab fall through to renderRun.
			fieldWidth, decimalPrefix := r.peekAheadTabFieldMetrics(runs, i+1)
			r.renderTabAligned(fieldWidth, decimalPrefix)
			continue
		}
		// Handle ADVANCE field: update offset state and emit a zero-size marker.
		if run.FieldKind == "ADVANCE" {
			closeRevision()
			r.applyAdvanceField(run)
			continue
		}
		tag, extra := r.revisionTagFor(run)
		if tag == "" {
			closeRevision()
		} else if !open || currentRev.tag != tag || currentRev.extra != extra {
			closeRevision()
			fmt.Fprintf(&r.buf, "<%s%s>", tag, extra)
			currentRev.tag = tag
			currentRev.extra = extra
			open = true
		}
		// Emit inline move-range start boundaries before this run.
		for _, imr := range moveStarts[i] {
			r.emitInlineMoveBoundary(imr, true)
		}
		// Apply accumulated ADVANCE offset to this run.
		if r.advanceState.HorizontalPt != 0 || r.advanceState.VerticalPt != 0 {
			r.openAdvanceOffsetWrapper()
			r.renderRun(run)
			r.closeAdvanceOffsetWrapper()
		} else {
			r.renderRun(run)
		}
		// Emit inline move-range end boundaries after this run.
		for _, imr := range moveEnds[i] {
			r.emitInlineMoveBoundary(imr, false)
		}
	}
	closeRevision()
}

func (r *htmlRenderer) effectiveRunForRendering(runs []resolve.Run, idx int) resolve.Run {
	run := runs[idx]
	if run.FieldKind != "SEQ" {
		return run
	}
	control := r.seqControlForRun(run)
	if !control.charFormat || !runFormatIsZero(run.Format) {
		return run
	}
	prev, ok := previousRenderableRun(runs, idx)
	if !ok {
		return run
	}
	run.Format = prev.Format
	return run
}

func previousRenderableRun(runs []resolve.Run, idx int) (resolve.Run, bool) {
	for i := idx - 1; i >= 0; i-- {
		run := runs[i]
		if run.Format.Vanish != nil && *run.Format.Vanish {
			continue
		}
		if run.Format.SpecVanish != nil && *run.Format.SpecVanish {
			continue
		}
		if run.FieldKind == "ADVANCE" {
			continue
		}
		if run.Kind == "tab" {
			continue
		}
		if run.Kind == "run" && strings.TrimSpace(run.Text) == "" && runFormatIsZero(run.Format) {
			continue
		}
		if run.Kind == "break" && run.Text == "" {
			continue
		}
		return run, true
	}
	return resolve.Run{}, false
}

func runFormatIsZero(f resolve.RunFormat) bool {
	return reflect.DeepEqual(f, resolve.RunFormat{})
}

// revisionAttrs returns HTML data attributes for a property revision mark.
// When nil it returns an empty string.
func revisionAttrs(mark *resolve.RevisionMark) string {
	if mark == nil {
		return ""
	}
	var parts []string
	if mark.Author != "" {
		parts = append(parts, fmt.Sprintf(`data-prop-rev-author=%q`, mark.Author))
	}
	if mark.Date != "" {
		parts = append(parts, fmt.Sprintf(`data-prop-rev-date=%q`, mark.Date))
	}
	if mark.ID != 0 {
		parts = append(parts, fmt.Sprintf(`data-prop-rev-id="%d"`, mark.ID))
	}
	if len(parts) > 0 {
		parts = append([]string{`class="word-prop-revision"`}, parts...)
	}
	return " " + strings.Join(parts, " ")
}

// cellTrackingAttrs returns HTML data attributes for cell insertion/deletion/merge tracking.
func cellTrackingAttrs(cell resolve.TableCell) string {
	var parts []string
	if cell.CellIns != nil {
		parts = append(parts, `data-cell-track="ins"`)
		if cell.CellIns.Author != "" {
			parts = append(parts, fmt.Sprintf(`data-cell-track-author=%q`, cell.CellIns.Author))
		}
		if cell.CellIns.Date != "" {
			parts = append(parts, fmt.Sprintf(`data-cell-track-date=%q`, cell.CellIns.Date))
		}
	} else if cell.CellDel != nil {
		parts = append(parts, `data-cell-track="del"`)
		if cell.CellDel.Author != "" {
			parts = append(parts, fmt.Sprintf(`data-cell-track-author=%q`, cell.CellDel.Author))
		}
		if cell.CellDel.Date != "" {
			parts = append(parts, fmt.Sprintf(`data-cell-track-date=%q`, cell.CellDel.Date))
		}
	} else if cell.CellMerge != nil {
		parts = append(parts, `data-cell-track="merge"`)
		if cell.CellMerge.Author != "" {
			parts = append(parts, fmt.Sprintf(`data-cell-track-author=%q`, cell.CellMerge.Author))
		}
		if cell.CellMerge.Date != "" {
			parts = append(parts, fmt.Sprintf(`data-cell-track-date=%q`, cell.CellMerge.Date))
		}
	}
	if len(parts) > 0 {
		return " " + strings.Join(parts, " ")
	}
	return ""
}

func (r *htmlRenderer) revisionTagFor(run resolve.Run) (tag string, extra string) {
	if run.Revision != "inserted" && run.Revision != "deleted" && run.Revision != "movedFrom" && run.Revision != "movedTo" {
		return "", ""
	}
	tag = "ins"
	switch run.Revision {
	case "deleted", "movedFrom":
		tag = "del"
	}
	var b strings.Builder
	if run.Revision == "movedFrom" || run.Revision == "movedTo" {
		fmt.Fprintf(&b, ` data-revision-type=%q`, run.Revision)
	}
	if run.RevisionAuthor != "" || run.RevisionDate != "" {
		var titleParts []string
		if run.RevisionAuthor != "" {
			titleParts = append(titleParts, html.EscapeString(run.RevisionAuthor))
			fmt.Fprintf(&b, ` data-revision-author=%q`, run.RevisionAuthor)
		}
		if run.RevisionDate != "" {
			titleParts = append(titleParts, html.EscapeString(run.RevisionDate))
			fmt.Fprintf(&b, ` data-revision-date=%q`, run.RevisionDate)
		}
		fmt.Fprintf(&b, ` title=%q`, strings.Join(titleParts, ", "))
	}
	return tag, b.String()
}

func (r *htmlRenderer) renderRun(run resolve.Run) {
	if run.Format.Hidden() {
		// Hidden text is not displayed and takes no layout space; layout already
		// excludes it, so the semantic path must skip it too.
		return
	}
	if run.Image != nil && r.topFloatAlreadyRendered(run.Image) {
		return
	}
	if run.Revision == "deleted" && run.Kind == "olePlaceholder" {
		return
	}
	// Page breaks and images don't need text content.
	if run.Kind == "break" && run.BreakType == "page" {
		return
	}
	if run.Kind == "break" && run.BreakType == "line" {
		if run.BreakClear != "" && run.BreakClear != "none" {
			if clearCSS := breakClearCSS(run.BreakClear); clearCSS != "" {
				fmt.Fprintf(&r.buf, `<br style=%q>`, clearCSS)
				return
			}
		}
		r.buf.WriteString("<br>")
		return
	}
	if run.Kind == "image" && run.Image != nil {
		if run.Image.Floating {
			if r.options.FastPreview {
				r.renderImage(run.Image, run.ContentControl)
				return
			}
			if r.renderParagraphFloatImage(run.Image) {
				return
			}
			if r.renderParagraphBlockFloatImage(run.Image) {
				return
			}
			// All floating images go to the page overlay with layout-computed
			// page-absolute coordinates. No paragraph-relative anchoring needed.
			r.renderImageTo(r.imageOverlayTarget(run.Image), run.Image, run.ContentControl)
			return
		}
		r.renderImage(run.Image, run.ContentControl)
		return
	}
	if run.Kind == "chart" && run.Image != nil {
		// Charts are rendered as SVG images
		if run.Image.Floating {
			if r.options.FastPreview {
				r.renderImage(run.Image, nil)
				return
			}
			if r.renderParagraphFloatImage(run.Image) {
				return
			}
			if r.renderParagraphBlockFloatImage(run.Image) {
				return
			}
			r.renderImageTo(r.imageOverlayTarget(run.Image), run.Image, nil)
			return
		}
		r.renderImage(run.Image, nil)
		return
	}
	if run.Kind == "textbox" && run.TextBox != nil {
		if run.TextBox.Floating {
			if r.renderParagraphFloatTextBox(run.TextBox) {
				return
			}
			if r.renderParagraphBlockFloatTextBox(run.TextBox) {
				return
			}
			r.renderTextBoxTo(&r.overlay, run.TextBox)
			return
		}
		r.closeAllLists()
		r.renderTextBox(run.TextBox)
		return
	}
	if run.SignatureLine != nil {
		r.renderSignatureLine(run.SignatureLine)
		return
	}
	if run.Kind == "shape" && run.Shape != nil {
		if run.Shape.Floating {
			if r.renderParagraphBlockFloatShape(run.Shape) {
				return
			}
			r.renderShapeTo(&r.overlay, run.Shape)
			return
		}
		r.renderShape(run.Shape)
		return
	}
	if run.Kind == "math" && run.MathML != "" {
		r.buf.WriteString(run.MathML)
		return
	}
	if run.Kind == "ruby" && run.Ruby != nil {
		r.renderRuby(run)
		return
	}
	if run.Kind == "formField" && run.FormField != nil {
		r.renderFormField(run)
		return
	}
	if run.CustomXml != nil {
		r.openCustomXmlInline(run.CustomXml)
		defer r.closeCustomXmlInline()
	}
	if run.SmartTag != nil {
		r.openSmartTagInline(run.SmartTag)
		defer r.closeSmartTagInline()
	}
	if run.ContentControl != nil {
		if r.renderSdtRun(run) {
			return
		}
		r.openSdtInline(run.ContentControl)
		defer r.closeSdtInline()
	}
	if run.Kind == "olePlaceholder" {
		// A bare legacy object anchor is an implementation marker, not visible
		// document text. Word hides these anchors when no OLE payload, preview,
		// link, or ProgID can be resolved. Rendering "[Embedded Object]" here
		// leaks binary control structure into otherwise ordinary paragraphs.
		if run.OLEPreview == nil && len(run.OLEData) == 0 && run.OLEProgID == "" && run.OLERelID == "" && run.OLELinkURL == "" {
			return
		}
		label := oleProgIDName(firstNonEmptyString(run.OLEProgID, run.Text))
		className := "ole-object"
		if run.OLEPreview != nil && len(run.OLEPreview.Data) > 0 {
			className += " ole-object-preview"
		}
		attrs := fmt.Sprintf(`class=%q title="%s"`, className, html.EscapeString(firstNonEmptyString(run.OLEProgID, run.Text)))
		if run.OLERelID != "" {
			attrs += fmt.Sprintf(` data-ole-relid="%s"`, html.EscapeString(run.OLERelID))
		}
		if run.OLEProgID != "" {
			attrs += fmt.Sprintf(` data-ole-progid="%s"`, html.EscapeString(run.OLEProgID))
		}
		if run.OLEType != "" {
			attrs += fmt.Sprintf(` data-ole-type="%s"`, html.EscapeString(run.OLEType))
		}
		link := safeOLEHref(run.OLELinkURL)
		if link != "" {
			fmt.Fprintf(&r.buf, `<a class="ole-object-link" href="%s">`, html.EscapeString(link))
		}
		fmt.Fprintf(&r.buf, `<span %s>`, attrs)
		if run.OLEPreview != nil && len(run.OLEPreview.Data) > 0 {
			r.renderImage(run.OLEPreview, nil)
		} else {
			fmt.Fprintf(&r.buf, `[%s]`, html.EscapeString(label))
		}
		r.buf.WriteString(`</span>`)
		if link != "" {
			r.buf.WriteString(`</a>`)
		}
		return
	}
	if run.Kind == "footnoteRef" {
		id := run.NoteID
		if r.noteBodyKind != "" {
			// The note's own mark: plain superscript, no self-referencing link
			// and no duplicated ref anchor id.
			fmt.Fprintf(&r.buf, `<sup>%s</sup>`, html.EscapeString(r.noteDisplayNumber("footnote", id)))
			return
		}
		fmt.Fprintf(&r.buf, `<a href="#fn%d" id="fnref%d"><sup>%s</sup></a>`, id, id, html.EscapeString(r.noteDisplayNumber("footnote", id)))
		return
	}
	if run.Kind == "endnoteRef" {
		id := run.NoteID
		if r.noteBodyKind != "" {
			fmt.Fprintf(&r.buf, `<sup>%s</sup>`, html.EscapeString(r.noteDisplayNumber("endnote", id)))
			return
		}
		fmt.Fprintf(&r.buf, `<a href="#en%d" id="enref%d"><sup>%s</sup></a>`, id, id, html.EscapeString(r.noteDisplayNumber("endnote", id)))
		return
	}
	if run.Kind == "commentRef" {
		id := run.CommentID
		preview := ""
		if story, ok := r.doc.Comments[id]; ok && story.Text != "" {
			preview = fmt.Sprintf(" title=%q", story.Text)
		}
		fmt.Fprintf(&r.buf, `<a class="comment-ref" href="#comment%d" id="commentref%d" aria-describedby="comment%d"%s><sup>[%d]</sup></a>`, id, id, id, preview, id)
		return
	}
	if run.Kind == "annotationRef" {
		id := run.CommentID
		fmt.Fprintf(&r.buf, `<sup>[%d]</sup>`, id)
		return
	}
	if run.Kind == "horzRule" {
		if run.Text != "" {
			fmt.Fprintf(&r.buf, "<hr style=\"border-color:%s\">\n", html.EscapeString(run.Text))
		} else {
			r.buf.WriteString("<hr>\n")
		}
		return
	}

	// If the run is a pure field container (no text, no flat FieldKind),
	// evaluate each nested field and render its calculated value or cached
	// children. This allows dynamic fields (PAGE, IF, DATE, etc.) inside
	// nested structures to be evaluated rather than silently skipped.
	if run.Text == "" && run.FieldKind == "" && len(run.Fields) > 0 {
		for _, field := range run.Fields {
			synthRun := r.synthesizeFieldRun(field)
			calculated := r.calculateFieldValue(synthRun)
			if calculated != "" {
				synthRun.Text = calculated
				r.renderRun(synthRun)
			} else {
				r.renderRuns(field.Children)
			}
		}
		return
	}

	calculatedFieldValue := ""
	hasCalculatedFieldValue := false
	if run.FieldKind != "" {
		calculatedFieldValue = r.calculateFieldValue(run)
		hasCalculatedFieldValue = true
	}

	if run.Text == "" && run.FieldKind != "TOC" && run.FieldKind != "INDEX" && run.FieldKind != "FILLIN" && run.FieldKind != "ASK" && run.FieldKind != "FORMULA" && run.FieldKind != "MERGEFIELD" && run.FieldKind != "EMBED" && run.FieldKind != "LINK" && run.FieldKind != "DOCPROPERTY" && run.FieldKind != "DOCVARIABLE" {
		// Don't suppress calculated fields (PAGE, DATE, SECTION, etc.) —
		// they may have a dynamic value even when the cached text is empty.
		if run.FieldKind == "" || !hasCalculatedFieldValue || calculatedFieldValue == "" {
			return
		}
	}

	// Tab characters are handled separately using paragraph tab stops.
	if run.Kind == "tab" {
		r.renderTab()
		return
	}

	f := run.Format

	// Apply caps transformation before HTML escaping.
	rawText := run.Text
	if f.Caps != nil && *f.Caps {
		rawText = strings.ToUpper(rawText)
	}

	escaped := html.EscapeString(rawText)
	// Map run-level newlines (from w:br) to HTML equivalents.
	if run.Kind == "break" && run.BreakClear != "" && run.BreakClear != "none" {
		if clearCSS := breakClearCSS(run.BreakClear); clearCSS != "" {
			escaped = strings.ReplaceAll(escaped, "\n", fmt.Sprintf(`<br style="%s">`, clearCSS))
		} else {
			escaped = strings.ReplaceAll(escaped, "\n", "<br>")
		}
	} else {
		escaped = strings.ReplaceAll(escaped, "\n", "<br>")
	}

	styles := r.runFormatStyles(f)

	// FitText: estimate horizontal scale to fit text into target width.
	if f.FitTextWidth != nil && *f.FitTextWidth > 0 && rawText != "" {
		fitWidthPt := float64(*f.FitTextWidth) / 20.0
		fontSizePt := 11.0 // default estimate
		if f.FontSize != nil && *f.FontSize > 0 {
			fontSizePt = float64(*f.FontSize) / 2.0
		}
		charCount := float64(len([]rune(rawText)))
		if charCount > 0 {
			// Approximate average character width as 0.55em for proportional fonts.
			naturalWidthPt := charCount * fontSizePt * 0.55
			scale := fitWidthPt / naturalWidthPt
			if scale > 0 && (scale < 0.99 || scale > 1.01) {
				styles = append(styles, fmt.Sprintf("transform:scaleX(%.3f)", scale))
				styles = append(styles, "transform-origin:left center")
				styles = append(styles, "display:inline-block")
			}
		}
	}

	// Superscript / subscript use semantic tags.
	wrapTag := "span"
	switch f.VertAlignVal {
	case "superscript":
		wrapTag = "sup"
	case "subscript":
		wrapTag = "sub"
	}
	if f.Bdo != "" {
		wrapTag = "bdo"
	}

	// Hyperlink outer wrapper.
	if run.Kind == "hyperlink" && run.Target != "" {
		href := html.EscapeString(run.Target)
		if run.Anchor != "" && !strings.Contains(run.Target, "#") {
			href += "#" + html.EscapeString(run.Anchor)
		}
		fmt.Fprintf(&r.buf, "<a href=%q>", href)
	}

	// REF field: wrap display text in an anchor link to the bookmark unless
	// the resolver already materialized the run as a hyperlink or the \f switch
	// suppresses the hyperlink.
	if run.FieldKind == "REF" && run.Anchor != "" && !refHasSwitch(run.FieldInstr, "f") && !(run.Kind == "hyperlink" && run.Target != "") {
		fmt.Fprintf(&r.buf, "<a href=%q>", "#"+html.EscapeString(run.Anchor))
	}

	// PAGEREF field: wrap display text in an anchor link when \h is present.
	if run.FieldKind == "PAGEREF" && run.Anchor != "" && refHasSwitch(run.FieldInstr, "h") && !(run.Kind == "hyperlink" && run.Target != "") {
		fmt.Fprintf(&r.buf, "<a href=%q>", "#"+html.EscapeString(run.Anchor))
	}

	// FILLIN field: render as an HTML text input with the default value.
	if run.FieldKind == "FILLIN" {
		fmt.Fprintf(&r.buf, `<input type="text" class="word-field word-field-fillin" value=%q>`, html.EscapeString(run.FieldArg))
		return
	}

	// ASK field: render as an HTML text input with the current bookmark value.
	if run.FieldKind == "ASK" {
		value := ""
		if run.FieldArg != "" {
			value = r.bookmarkText[run.FieldArg]
		}
		fmt.Fprintf(&r.buf, `<input type="text" class="word-field word-field-ask" value=%q>`, html.EscapeString(value))
		return
	}

	// Field semantic wrapper: emits a span with class="word-field word-field-{kind}".
	// For calculated fields (PAGE, DATE, etc.), compute the actual value.
	if run.FieldKind != "" {
		fieldClass := "word-field word-field-" + strings.ToLower(run.FieldKind)
		dataAttr := ""
		if run.FieldArg != "" {
			dataAttr = fmt.Sprintf(` data-field-arg=%q`, html.EscapeString(run.FieldArg))
		}
		styleAttr := ""
		if run.FieldKind == "INDEX" {
			if cols := parseIndexColumns(run.FieldInstr); cols > 1 {
				styleAttr = fmt.Sprintf(` style="column-count:%d"`, cols)
			}
		}
		privateAttr := ""
		if run.FieldKind == "PRIVATE" && r.options.InterpretPrivateFields {
			if app, payload := parsePrivateField(run.FieldInstr); app != "" {
				privateAttr = fmt.Sprintf(` data-private-app=%q data-private-data=%q`, html.EscapeString(app), html.EscapeString(payload))
			}
		}
		fmt.Fprintf(&r.buf, `<span class=%q%s%s%s>`, fieldClass, dataAttr, styleAttr, privateAttr)
		// Calculate field value if needed
		if calculatedFieldValue != "" {
			escaped = html.EscapeString(calculatedFieldValue)
		}
	}

	noProofAttr := ""
	if f.NoProof != nil && *f.NoProof {
		noProofAttr = ` spellcheck="false"`
	}
	needsWrap := len(styles) > 0 || wrapTag != "span" || f.Lang != "" || f.Dir != "" || f.Bdo != "" || noProofAttr != ""
	if needsWrap {
		langAttr := ""
		if f.Lang != "" {
			langAttr = fmt.Sprintf(" lang=%q", f.Lang)
		}
		dirAttr := ""
		if f.Bdo != "" {
			dirAttr = fmt.Sprintf(` dir=%q`, f.Bdo)
		} else if f.Dir != "" {
			dirAttr = fmt.Sprintf(` dir=%q`, f.Dir)
		}
		if len(styles) > 0 {
			fmt.Fprintf(&r.buf, "<%s style=%q%s%s%s>", wrapTag, strings.Join(styles, ";"), langAttr, dirAttr, noProofAttr)
		} else {
			fmt.Fprintf(&r.buf, "<%s%s%s%s>", wrapTag, langAttr, dirAttr, noProofAttr)
		}
	}

	// Track approximate text width for tab-stop and ADVANCE calculations.
	displayText := rawText
	if run.FieldKind != "" && calculatedFieldValue != "" {
		displayText = calculatedFieldValue
	}
	r.tabEstimatedX += r.estimateTextWidthForRun(displayText, f)

	r.buf.WriteString(escaped)

	if needsWrap {
		fmt.Fprintf(&r.buf, "</%s>", wrapTag)
	}

	if run.FieldKind != "" {
		r.buf.WriteString("</span>")
	}

	if run.FieldKind == "REF" && run.Anchor != "" && !refHasSwitch(run.FieldInstr, "f") && !(run.Kind == "hyperlink" && run.Target != "") {
		r.buf.WriteString("</a>")
	}

	if run.FieldKind == "PAGEREF" && run.Anchor != "" && refHasSwitch(run.FieldInstr, "h") && !(run.Kind == "hyperlink" && run.Target != "") {
		r.buf.WriteString("</a>")
	}

	if run.Kind == "hyperlink" && run.Target != "" {
		r.buf.WriteString("</a>")
	}
}

func flattenSDTBlocks(blocks []resolve.Block) []resolve.Block {
	return blocks
}

func (r *htmlRenderer) openSdtBlock(cc *resolve.ContentControl) {
	tag, extra := sdtTag(cc, true)
	r.sdtBlockTags = append(r.sdtBlockTags, tag)
	fmt.Fprintf(&r.buf, `<%s class=%q%s%s>`, tag, sdtClass(cc), sdtAttrs(cc), extra)
}

func (r *htmlRenderer) closeSdtBlock() {
	tag := "div"
	if n := len(r.sdtBlockTags); n > 0 {
		tag = r.sdtBlockTags[n-1]
		r.sdtBlockTags = r.sdtBlockTags[:n-1]
	}
	fmt.Fprintf(&r.buf, `</%s>`, tag)
}

func (r *htmlRenderer) openSdtInline(cc *resolve.ContentControl) {
	tag, extra := sdtTag(cc, false)
	r.sdtInlineTags = append(r.sdtInlineTags, tag)
	fmt.Fprintf(&r.buf, `<%s class=%q%s%s>`, tag, sdtClass(cc), sdtAttrs(cc), extra)
}

func (r *htmlRenderer) closeSdtInline() {
	tag := "span"
	if n := len(r.sdtInlineTags); n > 0 {
		tag = r.sdtInlineTags[n-1]
		r.sdtInlineTags = r.sdtInlineTags[:n-1]
	}
	fmt.Fprintf(&r.buf, `</%s>`, tag)
}

func (r *htmlRenderer) openCustomXmlBlock(cx *resolve.CustomXmlInfo) {
	fmt.Fprintf(&r.buf, `<div data-custom-xml-uri=%q data-custom-xml-element=%q`, cx.URI, cx.Element)
	for _, attr := range cx.Attributes {
		fmt.Fprintf(&r.buf, ` data-custom-xml-attr-%s=%q`, attr.Name, attr.Value)
	}
	r.buf.WriteString(`>`)
}

func (r *htmlRenderer) closeCustomXmlBlock() {
	r.buf.WriteString(`</div>`)
}

func (r *htmlRenderer) openCustomXmlInline(cx *resolve.CustomXmlInfo) {
	fmt.Fprintf(&r.buf, `<span data-custom-xml-uri=%q data-custom-xml-element=%q`, cx.URI, cx.Element)
	for _, attr := range cx.Attributes {
		fmt.Fprintf(&r.buf, ` data-custom-xml-attr-%s=%q`, attr.Name, attr.Value)
	}
	r.buf.WriteString(`>`)
}

func (r *htmlRenderer) closeCustomXmlInline() {
	r.buf.WriteString(`</span>`)
}

func (r *htmlRenderer) openSmartTagInline(st *resolve.SmartTagInfo) {
	fmt.Fprintf(&r.buf, `<span data-smart-tag-uri=%q data-smart-tag-element=%q>`, st.URI, st.Element)
}

func (r *htmlRenderer) closeSmartTagInline() {
	r.buf.WriteString("</span>")
}

func sdtAllowsContentEditing(cc *resolve.ContentControl) bool {
	if cc == nil {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(cc.Lock)) {
	case "contentlocked", "sdtcontentlocked":
		return false
	default:
		return true
	}
}

// renderSdtRun handles inline w:sdt runs that have behaviorally distinct types.
// Returns true if the run was fully consumed (no further rendering needed).
func (r *htmlRenderer) renderSdtRun(run resolve.Run) bool {
	cc := run.ContentControl
	if cc == nil {
		return false
	}
	switch cc.Type {
	case "checkBox":
		checked := ""
		if cc.Checked {
			checked = " checked"
		}
		disabled := " disabled"
		if r.options.EditableContentControls && sdtAllowsContentEditing(cc) {
			disabled = ""
		}
		title := ""
		if cc.Title != "" {
			title = fmt.Sprintf(" title=%q", cc.Title)
		}
		fmt.Fprintf(&r.buf, `<input type="checkbox" class=%q%s%s%s%s>`, sdtClass(cc), sdtAttrs(cc), checked, title, disabled)
		return true
	case "date":
		// Prefer formatting the semantic fullDate value when available so the HTML
		// reflects the control's configured date picture instead of only cached text.
		attrs := sdtAttrs(cc)
		displayText := formatSdtDateText(cc, run.Text)
		if r.options.EditableContentControls && sdtAllowsContentEditing(cc) {
			if parsed := parseISODate(cc.DateFullDate); !parsed.IsZero() {
				fmt.Fprintf(&r.buf, `<input type="date" class=%q%s value=%q>`, sdtClass(cc), attrs, parsed.Format("2006-01-02"))
			} else {
				fmt.Fprintf(&r.buf, `<input type="text" class=%q%s value=%q>`, sdtClass(cc), attrs, displayText)
			}
			return true
		}
		dateTime := ""
		if cc.DateFullDate != "" {
			dateTime = fmt.Sprintf(` datetime=%q`, html.EscapeString(cc.DateFullDate))
		}
		fmt.Fprintf(&r.buf, `<time class=%q%s%s>%s</time>`, sdtClass(cc), dateTime, attrs, html.EscapeString(displayText))
		return true
	case "comboBox", "dropDownList":
		attrs := sdtAttrs(cc)
		currentValue := sdtDisplayText(cc, run.Text)
		// Build a <select> with the list items; pre-select the item matching run.Text.
		var buf strings.Builder
		disabled := " disabled"
		if r.options.EditableContentControls && sdtAllowsContentEditing(cc) {
			disabled = ""
		}
		fmt.Fprintf(&buf, `<select class=%q%s%s>`, sdtClass(cc), attrs, disabled)
		matched := false
		for _, item := range cc.ListItems {
			sel := ""
			// Data-bound list controls commonly store the item value while the
			// document cache shows its display label. Match either representation.
			if !matched && (item.DisplayText == currentValue || (item.Value != "" && item.Value == currentValue)) {
				sel = ` selected`
				matched = true
			}
			valAttr := ""
			if item.Value != "" {
				valAttr = fmt.Sprintf(` value=%q`, html.EscapeString(item.Value))
			}
			fmt.Fprintf(&buf, `<option%s%s>%s</option>`, valAttr, sel, html.EscapeString(item.DisplayText))
		}
		if !matched && currentValue != "" {
			// Current value not in list — add it as selected.
			fmt.Fprintf(&buf, `<option selected>%s</option>`, html.EscapeString(currentValue))
		}
		buf.WriteString(`</select>`)
		r.buf.WriteString(buf.String())
		return true
	case "plainText":
		attrs := sdtAttrs(cc)
		displayText := sdtDisplayText(cc, run.Text)
		readonly := " readonly"
		if r.options.EditableContentControls && sdtAllowsContentEditing(cc) {
			readonly = ""
		}
		if cc.PlainTextMultiLine || strings.Contains(displayText, "\n") {
			fmt.Fprintf(&r.buf, `<textarea class=%q%s%s>%s</textarea>`, sdtClass(cc), readonly, attrs, html.EscapeString(displayText))
		} else {
			fmt.Fprintf(&r.buf, `<input type="text" class=%q%s value=%q%s>`, sdtClass(cc), readonly, html.EscapeString(displayText), attrs)
		}
		return true
	case "richText":
		attrs := sdtAttrs(cc)
		text := html.EscapeString(sdtDisplayText(cc, run.Text))
		editable := ""
		if r.options.EditableContentControls && sdtAllowsContentEditing(cc) {
			editable = ` contenteditable="true"`
		}
		fmt.Fprintf(&r.buf, `<span class=%q%s%s>%s</span>`, sdtClass(cc), editable, attrs, text)
		return true
	}
	return false
}

func sdtTag(cc *resolve.ContentControl, block bool) (tag string, extra string) {
	if block {
		tag = "div"
	} else {
		tag = "span"
	}
	if cc == nil {
		return tag, ""
	}
	switch cc.Type {
	case "citation":
		if !block {
			tag = "cite"
		}
	case "equation":
		extra = ` role="math"`
	case "group":
		extra = ` role="group"`
	case "repeatingSection":
		if block {
			tag = "section"
		}
		extra = ` data-sdt-repeat="section"`
	case "repeatingSectionItem":
		extra = ` data-sdt-repeat="item"`
	case "contentPart":
		extra = ` data-sdt-content-part="true"`
	case "docPartObj", "docPartList":
		extra = ` data-sdt-doc-part="true"`
	}
	return tag, extra
}

func formatSdtDateText(cc *resolve.ContentControl, fallback string) string {
	text := sdtDisplayText(cc, fallback)
	if cc == nil || cc.ShowingPlaceholder {
		return text
	}
	if cc.DateFullDate == "" {
		return text
	}
	t := parseISODate(cc.DateFullDate)
	if t.IsZero() {
		return text
	}
	if cc.DateFormat != "" {
		return t.Format(convertWordDateFormat(cc.DateFormat))
	}
	if text != "" {
		return text
	}
	return t.Format("2006-01-02")
}

func sdtDisplayText(cc *resolve.ContentControl, fallback string) string {
	text := strings.TrimSpace(fallback)
	if cc == nil || cc.ShowingPlaceholder || strings.TrimSpace(cc.DataValue) == "" {
		return text
	}
	return strings.TrimSpace(cc.DataValue)
}

// renderRuby emits a w:ruby phonetic guide as an HTML <ruby> element.
func (r *htmlRenderer) renderRuby(run resolve.Run) {
	if run.Ruby == nil {
		return
	}
	ruby := run.Ruby
	var styles []string
	if ruby.Align != "" {
		switch ruby.Align {
		case "center":
			styles = append(styles, "ruby-align:center")
		case "distributeLetter":
			styles = append(styles, "ruby-align:space-between")
		case "distributeSpace":
			styles = append(styles, "ruby-align:space-around")
		case "left":
			styles = append(styles, "ruby-align:start")
		case "right", "rightVertical":
			styles = append(styles, "ruby-align:end")
		}
	}
	if ruby.FontSize > 0 {
		styles = append(styles, fmt.Sprintf("font-size:%.2fpt", ruby.FontSize))
	}
	if ruby.Raise > 0 {
		styles = append(styles, fmt.Sprintf("margin-top:%.2fpt", ruby.Raise))
	}
	if ruby.Lang != "" {
		styles = append(styles, fmt.Sprintf("lang=%q", ruby.Lang))
	}
	if len(styles) > 0 {
		fmt.Fprintf(&r.buf, `<ruby style=%q>`, strings.Join(styles, ";"))
	} else {
		r.buf.WriteString("<ruby>")
	}
	r.buf.WriteString(html.EscapeString(ruby.Base))
	var rtStyles []string
	if ruby.FontSize > 0 {
		rtStyles = append(rtStyles, fmt.Sprintf("font-size:%.2fpt", ruby.FontSize))
	}
	if len(rtStyles) > 0 {
		fmt.Fprintf(&r.buf, `<rt style=%q>%s</rt>`, strings.Join(rtStyles, ";"), html.EscapeString(ruby.Text))
	} else {
		fmt.Fprintf(&r.buf, "<rt>%s</rt>", html.EscapeString(ruby.Text))
	}
	r.buf.WriteString("</ruby>")
}

// renderDocumentMeta emits <meta> tags for document metadata that is resolved
// into IR but not otherwise rendered visually.
func (r *htmlRenderer) renderDocumentMeta() {
	if r.doc == nil {
		return
	}
	if r.doc.Author != "" {
		fmt.Fprintf(&r.buf, `<meta name="author" content=%q>%s`, html.EscapeString(r.doc.Author), "\n")
	}
	if r.doc.Subject != "" {
		fmt.Fprintf(&r.buf, `<meta name="description" content=%q>%s`, html.EscapeString(r.doc.Subject), "\n")
	}
	if r.doc.Keywords != "" {
		fmt.Fprintf(&r.buf, `<meta name="keywords" content=%q>%s`, html.EscapeString(r.doc.Keywords), "\n")
	}
	if r.doc.Template != "" {
		fmt.Fprintf(&r.buf, `<meta name="template" content=%q>%s`, html.EscapeString(r.doc.Template), "\n")
	}
	if r.doc.RevNum != "" {
		fmt.Fprintf(&r.buf, `<meta name="revision" content=%q>%s`, html.EscapeString(r.doc.RevNum), "\n")
	}
	if r.doc.ProofState != nil {
		spelling := r.doc.ProofState.Spelling
		if spelling == "" {
			spelling = "default"
		}
		grammar := r.doc.ProofState.Grammar
		if grammar == "" {
			grammar = "default"
		}
		fmt.Fprintf(&r.buf, `<meta name="proof-state" content="spelling:%s;grammar:%s">%s`, spelling, grammar, "\n")
	}
	if r.doc.ThemeFontLang != nil {
		parts := []string{}
		if r.doc.ThemeFontLang.Val != "" {
			parts = append(parts, "val:"+r.doc.ThemeFontLang.Val)
		}
		if r.doc.ThemeFontLang.EastAsia != "" {
			parts = append(parts, "eastAsia:"+r.doc.ThemeFontLang.EastAsia)
		}
		if r.doc.ThemeFontLang.Bidi != "" {
			parts = append(parts, "bidi:"+r.doc.ThemeFontLang.Bidi)
		}
		if len(parts) > 0 {
			fmt.Fprintf(&r.buf, `<meta name="theme-font-lang" content=%q>%s`, strings.Join(parts, ";"), "\n")
		}
	}
}

// renderFormField emits a legacy Word form field (FORMTEXT, FORMCHECKBOX, FORMDROPDOWN)
// as a read-only HTML form element.
func (r *htmlRenderer) renderFormField(run resolve.Run) {
	ff := run.FormField
	if ff == nil {
		return
	}
	switch ff.Type {
	case "text":
		var attrs strings.Builder
		attrs.WriteString(`type="text"`)
		if ff.DefaultText != "" {
			fmt.Fprintf(&attrs, ` value=%q`, ff.DefaultText)
		}
		if ff.MaxLength > 0 {
			fmt.Fprintf(&attrs, ` maxlength="%d"`, ff.MaxLength)
		}
		if ff.Name != "" {
			fmt.Fprintf(&attrs, ` name=%q`, ff.Name)
		}
		fmt.Fprintf(&r.buf, `<input class="word-form-field" %s disabled>`, attrs.String())
	case "checkbox":
		var attrs strings.Builder
		attrs.WriteString(`type="checkbox"`)
		if ff.Checked {
			attrs.WriteString(` checked`)
		}
		if ff.Name != "" {
			fmt.Fprintf(&attrs, ` name=%q`, ff.Name)
		}
		fmt.Fprintf(&r.buf, `<input class="word-form-field" %s disabled>`, attrs.String())
	case "dropdown":
		nameAttr := ""
		if ff.Name != "" {
			nameAttr = fmt.Sprintf(` name=%q`, ff.Name)
		}
		fmt.Fprintf(&r.buf, `<select class="word-form-field"%s disabled>`, nameAttr)
		for i, item := range ff.Items {
			if i == ff.DefaultIdx {
				fmt.Fprintf(&r.buf, `<option selected>%s</option>`, html.EscapeString(item))
			} else {
				fmt.Fprintf(&r.buf, `<option>%s</option>`, html.EscapeString(item))
			}
		}
		r.buf.WriteString(`</select>`)
	}
}

func sdtAttrs(cc *resolve.ContentControl) string {
	if cc == nil {
		return ""
	}
	var attrs strings.Builder
	if cc.Tag != "" {
		fmt.Fprintf(&attrs, ` data-sdt-tag=%q`, cc.Tag)
	}
	if cc.Type != "" {
		fmt.Fprintf(&attrs, ` data-sdt-type=%q`, cc.Type)
	}
	if cc.Title != "" {
		fmt.Fprintf(&attrs, ` data-sdt-title=%q`, cc.Title)
	}
	if cc.ID != 0 {
		fmt.Fprintf(&attrs, ` data-sdt-id=%q`, strconv.Itoa(cc.ID))
	}
	if cc.Checked {
		attrs.WriteString(` data-sdt-checked="true"`)
	}
	if cc.DateFormat != "" {
		fmt.Fprintf(&attrs, ` data-sdt-date-format=%q`, cc.DateFormat)
	}
	if cc.DateFullDate != "" {
		fmt.Fprintf(&attrs, ` data-sdt-date-full=%q`, cc.DateFullDate)
	}
	if cc.DateLanguage != "" {
		fmt.Fprintf(&attrs, ` data-sdt-date-language=%q`, cc.DateLanguage)
	}
	if cc.DateCalendar != "" {
		fmt.Fprintf(&attrs, ` data-sdt-date-calendar=%q`, cc.DateCalendar)
	}
	if cc.DocPartGallery != "" {
		fmt.Fprintf(&attrs, ` data-sdt-doc-part-gallery=%q`, cc.DocPartGallery)
	}
	if cc.DocPartCategory != "" {
		fmt.Fprintf(&attrs, ` data-sdt-doc-part-category=%q`, cc.DocPartCategory)
	}
	if cc.DocPartUnique {
		attrs.WriteString(` data-sdt-doc-part-unique="true"`)
	}
	if cc.ShowingPlaceholder {
		attrs.WriteString(` data-sdt-placeholder="true"`)
	}
	if cc.PlaceholderDocPart != "" {
		fmt.Fprintf(&attrs, ` data-sdt-placeholder-doc-part=%q`, cc.PlaceholderDocPart)
	}
	if cc.PlaceholderDocPartGallery != "" {
		fmt.Fprintf(&attrs, ` data-sdt-placeholder-gallery=%q`, cc.PlaceholderDocPartGallery)
	}
	if cc.DataXPath != "" {
		fmt.Fprintf(&attrs, ` data-sdt-xpath=%q`, cc.DataXPath)
	}
	if cc.DataStoreID != "" {
		fmt.Fprintf(&attrs, ` data-sdt-store-item-id=%q`, cc.DataStoreID)
	}
	if cc.DataValue != "" {
		fmt.Fprintf(&attrs, ` data-sdt-value=%q`, cc.DataValue)
	}
	if cc.Lock != "" {
		fmt.Fprintf(&attrs, ` data-sdt-lock=%q`, cc.Lock)
	}
	if cc.Temporary {
		attrs.WriteString(` data-sdt-temporary="true"`)
	}
	if cc.Appearance != "" {
		fmt.Fprintf(&attrs, ` data-sdt-appearance=%q`, cc.Appearance)
	}
	if cc.Color != "" {
		fmt.Fprintf(&attrs, ` data-sdt-color=%q`, cc.Color)
	}
	if cc.PlainTextMultiLine {
		attrs.WriteString(` data-sdt-multiline="true"`)
	}
	if len(cc.ListItems) > 0 {
		var itemTexts []string
		for _, item := range cc.ListItems {
			itemTexts = append(itemTexts, item.DisplayText)
		}
		fmt.Fprintf(&attrs, ` data-sdt-items=%q`, strings.Join(itemTexts, "|"))
	}
	return attrs.String()
}

func sdtClass(cc *resolve.ContentControl) string {
	cls := "word-sdt"
	if cc != nil {
		if cc.ShowingPlaceholder {
			cls += " word-sdt-placeholder"
		}
		switch cc.Type {
		case "checkBox":
			cls += " word-sdt-checkbox"
		case "comboBox", "dropDownList":
			cls += " word-sdt-list"
		case "plainText":
			cls += " word-sdt-text"
		case "richText":
			cls += " word-sdt-rich"
		case "date":
			cls += " word-sdt-date"
		case "picture":
			cls += " word-sdt-picture"
		case "citation":
			cls += " word-sdt-citation"
		case "equation":
			cls += " word-sdt-equation"
		case "group":
			cls += " word-sdt-group"
		case "repeatingSection", "repeatingSectionItem":
			cls += " word-sdt-repeating-section"
		case "contentPart":
			cls += " word-sdt-content-part"
		case "docPartObj", "docPartList":
			cls += " word-sdt-doc-part"
		}
	}
	return cls
}

func (r *htmlRenderer) collectPageTopFlowImages(page layout.Page) (*resolve.Paragraph, []*resolve.Image) {
	var anchor *resolve.Paragraph
	var images []*resolve.Image
	seen := make(map[*resolve.Image]struct{})
	for _, box := range page.Blocks {
		if box.Block == nil || box.Block.Paragraph == nil {
			continue
		}
		p := box.Block.Paragraph
		for _, run := range p.Runs {
			if run.Image == nil || !pageTopFlowImageEligible(run.Image) {
				continue
			}
			if anchor == nil {
				anchor = p
			}
			if _, ok := seen[run.Image]; ok {
				continue
			}
			seen[run.Image] = struct{}{}
			images = append(images, run.Image)
		}
	}
	sort.Slice(images, func(i, j int) bool {
		left, leftOK := r.floatByImage[images[i]]
		right, rightOK := r.floatByImage[images[j]]
		if leftOK && rightOK && !almostSameFloat(left.X, right.X) {
			return left.X < right.X
		}
		return images[i].X < images[j].X
	})
	return anchor, images
}

func pageTopFlowImageEligible(img *resolve.Image) bool {
	if img == nil || !img.Floating || img.BehindDoc || img.Hidden {
		return false
	}
	if strings.EqualFold(img.WrapType, "none") || strings.EqualFold(img.WrapType, "topAndBottom") {
		return false
	}
	if !strings.EqualFold(img.YAlign, "top") {
		return false
	}
	switch strings.ToLower(img.YRelative) {
	case "margin", "page":
		return true
	default:
		return false
	}
}

func (r *htmlRenderer) renderPageTopFlowImagesForCurrentParagraph() {
	if r.currentParagraph == nil || r.pageTopFloatAnchor == nil || r.currentParagraph != r.pageTopFloatAnchor {
		return
	}
	for _, img := range r.pageTopFloatImages {
		if img == nil || r.topFloatAlreadyRendered(img) {
			continue
		}
		if r.renderParagraphFloatImage(img) {
			r.renderedTopFloats[img] = struct{}{}
		}
	}
}

func (r *htmlRenderer) topFloatAlreadyRendered(img *resolve.Image) bool {
	if img == nil || r.renderedTopFloats == nil {
		return false
	}
	_, ok := r.renderedTopFloats[img]
	return ok
}

// paragraphStyleAttr builds an inline style= attribute string from paragraph formatting.
// suppressBefore/suppressAfter suppress space-before/space-after when contextualSpacing is active.
// Returns an empty string when no properties are set.
func paragraphStyleAttr(f resolve.ParagraphFormat, suppressBefore, suppressAfter, suppressFirstLineIndent bool) string {
	return paragraphStyleAttrWithHanging(f, suppressBefore, suppressAfter, suppressFirstLineIndent, false)
}

func paragraphStyleAttrForParagraph(p *resolve.Paragraph, suppressBefore, suppressAfter, suppressFirstLineIndent bool) string {
	if p == nil {
		return ""
	}
	return paragraphStyleAttrWithHangingForSource(p.Format, suppressBefore, suppressAfter, suppressFirstLineIndent, false, p.Source != nil)
}

// paragraphStyleAttrWithHanging is like paragraphStyleAttr but additionally
// allows suppressing hanging indent (IndentHanging) independently from
// first-line indent (IndentFirstLine). This is useful for list items where
// the browser's built-in list marker already provides hanging indent behavior.
func paragraphStyleAttrWithHanging(f resolve.ParagraphFormat, suppressBefore, suppressAfter, suppressFirstLineIndent, suppressHangingIndent bool) string {
	return paragraphStyleAttrWithHangingForSource(f, suppressBefore, suppressAfter, suppressFirstLineIndent, suppressHangingIndent, true)
}

func paragraphStyleAttrWithHangingForSource(f resolve.ParagraphFormat, suppressBefore, suppressAfter, suppressFirstLineIndent, suppressHangingIndent, legacyBinary bool) string {
	var styles []string
	paddingTopPt := 0.0
	paddingRightPt := 0.0
	paddingBottomPt := 0.0
	paddingLeftPt := 0.0
	if f.Shading != "" {
		styles = append(styles, fmt.Sprintf("background-color:#%s", f.Shading))
	}
	switch f.Alignment {
	case "center":
		styles = append(styles, "text-align:center")
	case "right":
		styles = append(styles, "text-align:right")
	case "both", "distribute":
		styles = append(styles, "text-align:justify")
	}
	styles = append(styles, wordTableCellTextDirectionCSS(f.TextDirection)...)
	if !suppressBefore && !f.SpacingBeforeAuto {
		if f.SpacingBeforeLines != nil && *f.SpacingBeforeLines > 0 {
			styles = append(styles, fmt.Sprintf("padding-top:%.4fem", float64(*f.SpacingBeforeLines)/100.0))
		} else if f.SpacingBefore != nil && *f.SpacingBefore > 0 {
			paddingTopPt += twipsToPoints(*f.SpacingBefore)
		}
	}
	if !suppressAfter && !f.SpacingAfterAuto {
		if f.SpacingAfterLines != nil && *f.SpacingAfterLines > 0 {
			styles = append(styles, fmt.Sprintf("padding-bottom:%.4fem", float64(*f.SpacingAfterLines)/100.0))
		} else if f.SpacingAfter != nil && *f.SpacingAfter > 0 {
			paddingBottomPt += twipsToPoints(*f.SpacingAfter)
		}
	}
	if f.Borders != nil {
		paddingTopPt += borderOutsetCSSPt(f.Borders.Top)
		leftBorder := effectiveParagraphBorderLeft(f.Borders)
		paddingLeftPt += borderOutsetCSSPt(leftBorder)
		if f.Borders.Bottom != nil {
			paddingBottomPt += borderOutsetCSSPt(f.Borders.Bottom)
		} else {
			paddingBottomPt += borderOutsetCSSPt(f.Borders.Between)
		}
		rightBorder := effectiveParagraphBorderRight(f.Borders)
		paddingRightPt += borderOutsetCSSPt(rightBorder)
		styles = append(styles, borderSideCSS("border-top", f.Borders.Top)...)
		styles = append(styles, borderSideCSS("border-left", leftBorder)...)
		if f.Borders.Bottom != nil {
			styles = append(styles, borderSideCSS("border-bottom", f.Borders.Bottom)...)
		} else {
			styles = append(styles, borderSideCSS("border-bottom", f.Borders.Between)...)
		}
		styles = append(styles, borderSideCSS("border-right", rightBorder)...)
	}
	if paddingTopPt > 0 {
		styles = append(styles, fmt.Sprintf("padding-top:%.1fpt", paddingTopPt))
	}
	if paddingRightPt > 0 {
		styles = append(styles, fmt.Sprintf("padding-right:%.1fpt", paddingRightPt))
	}
	if paddingBottomPt > 0 {
		styles = append(styles, fmt.Sprintf("padding-bottom:%.1fpt", paddingBottomPt))
	}
	if paddingLeftPt > 0 {
		styles = append(styles, fmt.Sprintf("padding-left:%.1fpt", paddingLeftPt))
	}
	if f.MirrorIndents {
		if f.IndentLeft != nil && *f.IndentLeft > 0 {
			styles = append(styles, fmt.Sprintf("margin-inline-start:%.1fpt", twipsToPoints(*f.IndentLeft)))
		}
		if f.IndentRight != nil && *f.IndentRight > 0 {
			styles = append(styles, fmt.Sprintf("margin-inline-end:%.1fpt", twipsToPoints(*f.IndentRight)))
		}
	} else {
		if f.IndentLeft != nil && *f.IndentLeft > 0 {
			styles = append(styles, fmt.Sprintf("margin-left:%.1fpt", twipsToPoints(*f.IndentLeft)))
		}
		if f.IndentRight != nil && *f.IndentRight > 0 {
			styles = append(styles, fmt.Sprintf("margin-right:%.1fpt", twipsToPoints(*f.IndentRight)))
		}
	}
	if !suppressFirstLineIndent && f.IndentFirstLine != nil {
		styles = append(styles, fmt.Sprintf("text-indent:%.1fpt", twipsToPoints(*f.IndentFirstLine)))
	} else if !suppressFirstLineIndent && !suppressHangingIndent && f.IndentHanging != nil && *f.IndentHanging > 0 {
		styles = append(styles, fmt.Sprintf("text-indent:-%.1fpt", twipsToPoints(*f.IndentHanging)))
	}
	if f.LineSpacing != nil && *f.LineSpacing > 0 {
		switch f.LineRule {
		case "auto", "":
			multiple := float64(*f.LineSpacing) / 240.0
			// Binary DOC and OOXML both encode 240=single, 360=1.5 and
			// 480=double. Keep browser-normal 1.2 as the single-line floor, but
			// do not multiply authored larger multiples by 1.2 again.
			multiple = math.Max(1.2, multiple)
			styles = append(styles, fmt.Sprintf("line-height:%.3f", multiple))
		case "exact":
			styles = append(styles, fmt.Sprintf("line-height:%.1fpt", twipsToPoints(*f.LineSpacing)))
		case "atLeast":
			styles = append(styles, fmt.Sprintf("min-height:%.1fpt", twipsToPoints(*f.LineSpacing)))
		}
	}
	if f.KeepNext {
		styles = append(styles, "break-after:avoid")
	}
	if f.KeepLines {
		styles = append(styles, "break-inside:avoid-page")
	}
	if f.PageBreakBefore {
		styles = append(styles, "break-before:page")
	}
	if f.WidowControl {
		styles = append(styles, "widows:2")
		styles = append(styles, "orphans:2")
	}
	if f.NoAutoHyphenation {
		styles = append(styles, "hyphens:none")
	}
	if f.WordWrap != nil && !*f.WordWrap {
		styles = append(styles, "word-break:break-all")
	}
	if f.Kinsoku {
		styles = append(styles, "line-break:strict")
	}
	if f.OverflowPunct {
		styles = append(styles, "hanging-punctuation:allow-end")
	}
	if f.TopLinePunct {
		styles = append(styles, "hanging-punctuation:first")
	}
	if autoSpace := paragraphAutoSpaceCSS(f.AutoSpaceDE, f.AutoSpaceDN); autoSpace != "" {
		styles = append(styles, autoSpace)
	}
	if len(styles) == 0 {
		return ""
	}
	return fmt.Sprintf(" style=%q", strings.Join(styles, ";"))
}

// paragraphAutoSpaceCSS returns a CSS text-autospace declaration for East-Asian
// auto-spacing settings (w:autoSpaceDE / w:autoSpaceDN). Returns "" when both
// are false (the Word default).
func paragraphAutoSpaceCSS(autoSpaceDE, autoSpaceDN bool) string {
	if autoSpaceDE && autoSpaceDN {
		return "text-autospace:normal"
	}
	if autoSpaceDE {
		return "text-autospace:ideograph-alpha"
	}
	if autoSpaceDN {
		return "text-autospace:ideograph-numeric"
	}
	return ""
}

func borderOutsetCSSPt(side *resolve.BorderSide) float64 {
	if side == nil {
		return 0
	}
	outset := 0.0
	if side.SpacePt > 0 {
		outset += side.SpacePt
	}
	if side.SizePt > 0 {
		outset += side.SizePt
	}
	return outset
}

// headingLevel maps a style ID or name to an h1–h6 level.
// Returns 0 if the style is not a heading.
func headingLevel(styleID, styleName string) int {
	for _, s := range []string{strings.ToLower(styleName), strings.ToLower(styleID)} {
		for _, prefix := range []string{"heading ", "heading"} {
			if strings.HasPrefix(s, prefix) {
				rest := strings.TrimSpace(strings.TrimPrefix(s, prefix))
				if n, err := strconv.Atoi(rest); err == nil && n >= 1 && n <= 6 {
					return n
				}
			}
		}
	}
	return 0
}

// nearestPrecedingHeadingIdx returns the index of the nearest heading paragraph
// at level <= maxLevel that comes before the paragraph with the given index.
// Returns -1 when no such heading exists.
func (r *htmlRenderer) nearestPrecedingHeadingIdx(paraIdx, maxLevel int) int {
	result := -1
	for _, h := range r.headingParagraphs {
		if h.index >= paraIdx {
			break
		}
		if h.level <= maxLevel {
			result = h.index
		}
	}
	return result
}

// highlightColor maps a WordprocessingML highlight name to a CSS color value.
func highlightColor(name string) string {
	switch name {
	case "yellow":
		return "#FFFF00"
	case "green":
		return "#00FF00"
	case "cyan":
		return "#00FFFF"
	case "magenta":
		return "#FF00FF"
	case "blue":
		return "#0000FF"
	case "red":
		return "#FF0000"
	case "darkBlue":
		return "#00008B"
	case "darkCyan":
		return "#008B8B"
	case "darkGreen":
		return "#006400"
	case "darkMagenta":
		return "#8B008B"
	case "darkRed":
		return "#8B0000"
	case "darkYellow":
		return "#808000"
	case "darkGray":
		return "#A9A9A9"
	case "lightGray":
		return "#D3D3D3"
	case "black":
		return "#000000"
	case "white":
		return "#FFFFFF"
	default:
		return "#FFFF00"
	}
}

// renderImage emits an inline (non-floating) <img> element.
// Browser-displayable types get a base64 data URI; others get a placeholder.
func (r *htmlRenderer) renderImage(img *resolve.Image, cc *resolve.ContentControl) {
	r.renderImageTo(&r.buf, img, cc)
}

func (r *htmlRenderer) renderParagraphFloatImage(img *resolve.Image) bool {
	styleAttrs, ok := r.paragraphFloatImageStyleAttrs(img)
	if !ok {
		return false
	}
	// The picture itself must own the CSS float.  A hidden proxy reserves the
	// right amount of space, but its separately positioned page-overlay image
	// can still paint over the text when the browser's line metrics differ from
	// the pagination metrics.  Keeping both the geometry and the visible bitmap
	// in the same flow box gives square, tight, and through anchors real wrapping.
	styleAttrs = append(styleAttrs, fmt.Sprintf("position:relative;z-index:%d", r.normalizeZIndex(img.ZIndex)))
	r.writeImageElement(&r.buf, img, styleAttrs, nil)
	return true
}

func (r *htmlRenderer) imageOverlayTarget(img *resolve.Image) *bytes.Buffer {
	if img != nil && img.BehindDoc {
		return &r.behindOverlay
	}
	return &r.overlay
}

func (r *htmlRenderer) paragraphFloatImageStyleAttrs(img *resolve.Image) ([]string, bool) {
	if img == nil || !img.Floating || img.BehindDoc || img.Hidden || r.currentParagraph == nil {
		return nil, false
	}
	if strings.EqualFold(img.WrapType, "none") || strings.EqualFold(img.WrapType, "topAndBottom") {
		return nil, false
	}
	// If the anchor paragraph has no visible inline text there is nothing for a
	// CSS float to wrap around. Prefer the overlay path (position:absolute) which
	// uses the layout-computed page-absolute coordinate and is not subject to
	// CSS-flow drift. Without this, icons in anchor-only paragraphs end up hidden
	// behind the page-overlay div which renders on top of page-body content.
	if paragraphIsEmptyOrAnchorOnly(r.currentParagraph.Runs) {
		return nil, false
	}
	// Accept column-, margin-, and page-relative horizontal anchoring.
	// Character-relative images are essentially inline and excluded.
	if strings.EqualFold(img.XRelative, "character") || strings.EqualFold(img.XRelative, "insideMargin") || strings.EqualFold(img.XRelative, "outsideMargin") {
		return nil, false
	}
	// Accept paragraph-, line-, margin-, and page-relative vertical anchoring.
	if strings.EqualFold(img.YRelative, "line") {
		return nil, false
	}
	if img.YAlign != "" && img.YAlign != "top" {
		return nil, false
	}
	fb, ok := r.floatByImage[img]
	if !ok {
		return nil, false
	}
	paraBox, ok := r.paraBoxes[r.currentParagraph.Index]
	if !ok || paraBox.Page != fb.Page {
		return nil, false
	}
	columnLeftPt := paraBox.Geometry.MarginLeft + float64(paraBox.Column)*(paraBox.Geometry.ColumnWidthPt+paraBox.Geometry.ColumnGapPt)
	proxyX, proxyY, proxyW, proxyH, rotatedShapeOutside := rotatedProxyGeometry(fb.X, fb.Y, fb.Width, fb.Height, img.RotationDeg)
	relLeftPt := proxyX - columnLeftPt
	paraTopPt := paraBox.Geometry.MarginTop + paraBox.Y
	relTopPt := proxyY - paraTopPt
	proxyBox := fb
	proxyBox.X, proxyBox.Y, proxyBox.Width, proxyBox.Height = proxyX, proxyY, proxyW, proxyH
	floatSide := r.paragraphFloatDirection(img.WrapSide, img.XAlign, proxyBox, paraBox)
	styleAttrs := []string{fmt.Sprintf("float:%s", floatSide), "display:block"}
	if proxyW > 0 {
		styleAttrs = append(styleAttrs, fmt.Sprintf("width:%.1fpt", proxyW))
	} else if img.SizeRelHPct > 0 {
		styleAttrs = append(styleAttrs, r.relativeSizeStyle(img.SizeRelHPct, img.SizeRelHFrom, true))
	} else if img.Width > 0 {
		styleAttrs = append(styleAttrs, fmt.Sprintf("width:%.1fpt", emuToPoints(img.Width)))
	}
	if proxyH > 0 {
		styleAttrs = append(styleAttrs, fmt.Sprintf("height:%.1fpt", proxyH))
	} else if img.SizeRelVPct > 0 {
		styleAttrs = append(styleAttrs, r.relativeSizeStyle(img.SizeRelVPct, img.SizeRelVFrom, false))
	} else if img.Height > 0 {
		styleAttrs = append(styleAttrs, fmt.Sprintf("height:%.1fpt", emuToPoints(img.Height)))
	}
	if relTopPt > 0 {
		styleAttrs = append(styleAttrs, fmt.Sprintf("margin-top:%.1fpt", relTopPt))
	}
	if img.DistBottom > 0 {
		styleAttrs = append(styleAttrs, fmt.Sprintf("margin-bottom:%.1fpt", img.DistBottom))
	}
	if floatSide == "left" {
		if !almostSameFloat(relLeftPt, 0) {
			styleAttrs = append(styleAttrs, fmt.Sprintf("margin-left:%.1fpt", relLeftPt))
		}
		if img.DistRight > 0 {
			styleAttrs = append(styleAttrs, fmt.Sprintf("margin-right:%.1fpt", img.DistRight))
		}
	} else {
		rightInsetPt := paraBox.Geometry.ColumnWidthPt - (relLeftPt + proxyW)
		if !almostSameFloat(rightInsetPt, 0) {
			styleAttrs = append(styleAttrs, fmt.Sprintf("margin-right:%.1fpt", rightInsetPt))
		}
		if img.DistLeft > 0 {
			styleAttrs = append(styleAttrs, fmt.Sprintf("margin-left:%.1fpt", img.DistLeft))
		}
	}
	if img.BorderColor != "" && img.BorderWidth > 0 {
		styleAttrs = append(styleAttrs, fmt.Sprintf("border:%.2fpt solid #%s", img.BorderWidth, img.BorderColor))
	}
	styleAttrs = append(styleAttrs, pictureFrameGeometryStyles(img)...)
	if strings.EqualFold(img.WrapType, "tight") || strings.EqualFold(img.WrapType, "through") {
		shapeOutside := ""
		if len(img.WrapPolygon) > 0 {
			shapeOutside = wrapPolygonProxyCSS(
				img.WrapPolygon, img.Width, img.Height,
				fb.Width, fb.Height,
				proxyX, proxyY, proxyW, proxyH,
				fb.X, fb.Y, img.RotationDeg,
			)
		} else if img.RotationDeg != 0 {
			shapeOutside = rotatedShapeOutside
		}
		if shapeOutside != "" {
			styleAttrs = append(styleAttrs, "shape-outside:"+shapeOutside)
		}
	}
	return styleAttrs, true
}

func (r *htmlRenderer) renderParagraphBlockFloatImage(img *resolve.Image) bool {
	styleAttrs, ok := r.paragraphBlockFloatImageStyleAttrs(img)
	if !ok {
		return false
	}
	r.writeImageElement(&r.buf, img, styleAttrs, nil)
	return true
}

func (r *htmlRenderer) renderParagraphFloatTextBox(tb *resolve.TextBox) bool {
	styleAttrs, widthPt, heightPt, ok := r.paragraphFloatTextBoxStyleAttrs(tb)
	if !ok {
		return false
	}
	r.writeTextBoxElement(tb, styleAttrs, widthPt, heightPt)
	return true
}

func (r *htmlRenderer) renderParagraphBlockFloatTextBox(tb *resolve.TextBox) bool {
	styleAttrs, widthPt, heightPt, ok := r.paragraphBlockFloatTextBoxStyleAttrs(tb)
	if !ok {
		return false
	}
	r.writeTextBoxElement(tb, styleAttrs, widthPt, heightPt)
	return true
}

func (r *htmlRenderer) paragraphFloatTextBoxStyleAttrs(tb *resolve.TextBox) ([]string, float64, float64, bool) {
	if tb == nil || !tb.Floating || tb.BehindDoc || r.currentParagraph == nil {
		return nil, 0, 0, false
	}
	wrapType := strings.ToLower(strings.TrimSpace(tb.WrapType))
	if wrapType == "none" || wrapType == "topandbottom" {
		return nil, 0, 0, false
	}
	if wrapType != "square" && wrapType != "tight" && wrapType != "through" {
		return nil, 0, 0, false
	}
	if paragraphIsEmptyOrAnchorOnly(r.currentParagraph.Runs) {
		return nil, 0, 0, false
	}
	if strings.EqualFold(tb.XRelative, "character") || strings.EqualFold(tb.XRelative, "insideMargin") || strings.EqualFold(tb.XRelative, "outsideMargin") {
		return nil, 0, 0, false
	}
	if !strings.EqualFold(tb.YRelative, "paragraph") {
		return nil, 0, 0, false
	}
	if tb.YAlign != "" && tb.YAlign != "top" {
		return nil, 0, 0, false
	}
	fb, ok := r.floatByTextBox[tb]
	if !ok {
		return nil, 0, 0, false
	}
	paraBox, ok := r.paraBoxes[r.currentParagraph.Index]
	if !ok || paraBox.Page != fb.Page {
		return nil, 0, 0, false
	}
	columnLeftPt := paraBox.Geometry.MarginLeft + float64(paraBox.Column)*(paraBox.Geometry.ColumnWidthPt+paraBox.Geometry.ColumnGapPt)
	relLeftPt := fb.X - columnLeftPt
	paraTopPt := paraBox.Geometry.MarginTop + paraBox.Y
	relTopPt := fb.Y - paraTopPt
	floatWidthPt := boxWidthPt(fb.Width, emuToPoints(tb.Width))
	floatHeightPt := boxHeightPt(fb.Height, emuToPoints(tb.Height))
	floatSide := r.paragraphFloatDirection(tb.WrapSide, tb.XAlign, fb, paraBox)
	styles := []string{fmt.Sprintf("float:%s", floatSide), "position:relative"}
	styles = r.appendTextBoxBoxStyles(styles, tb, floatWidthPt, floatHeightPt)
	if relTopPt > 0 {
		styles = append(styles, fmt.Sprintf("margin-top:%.1fpt", relTopPt))
	}
	if tb.DistBottom > 0 {
		styles = append(styles, fmt.Sprintf("margin-bottom:%.1fpt", tb.DistBottom))
	}
	if floatSide == "left" {
		if !almostSameFloat(relLeftPt, 0) {
			styles = append(styles, fmt.Sprintf("margin-left:%.1fpt", relLeftPt))
		}
		if tb.DistRight > 0 {
			styles = append(styles, fmt.Sprintf("margin-right:%.1fpt", tb.DistRight))
		}
	} else {
		rightInsetPt := paraBox.Geometry.ColumnWidthPt - (relLeftPt + floatWidthPt)
		if !almostSameFloat(rightInsetPt, 0) {
			styles = append(styles, fmt.Sprintf("margin-right:%.1fpt", rightInsetPt))
		}
		if tb.DistLeft > 0 {
			styles = append(styles, fmt.Sprintf("margin-left:%.1fpt", tb.DistLeft))
		}
	}
	if (wrapType == "tight" || wrapType == "through") && len(tb.WrapPolygon) > 0 {
		if shapeOutside := shapeOutsidePolygonCSS(tb.WrapPolygon, tb.Width, tb.Height); shapeOutside != "" {
			styles = append(styles, "shape-outside:"+shapeOutside)
		}
	}
	return styles, floatWidthPt, floatHeightPt, true
}

func (r *htmlRenderer) paragraphBlockFloatTextBoxStyleAttrs(tb *resolve.TextBox) ([]string, float64, float64, bool) {
	if tb == nil || !tb.Floating || tb.BehindDoc || r.currentParagraph == nil {
		return nil, 0, 0, false
	}
	if !strings.EqualFold(tb.WrapType, "topAndBottom") {
		return nil, 0, 0, false
	}
	if !paragraphBlockFloatTextBoxEligible(r.currentParagraph, tb) {
		return nil, 0, 0, false
	}

	widthPt := emuToPoints(tb.Width)
	heightPt := emuToPoints(tb.Height)
	styleAttrs := []string{"display:block", "clear:both"}
	if fb, ok := r.floatByTextBox[tb]; ok {
		widthPt = boxWidthPt(fb.Width, widthPt)
		heightPt = boxHeightPt(fb.Height, heightPt)
	}
	styleAttrs = r.appendTextBoxBoxStyles(styleAttrs, tb, widthPt, heightPt)
	if tb.DistTop > 0 {
		styleAttrs = append(styleAttrs, fmt.Sprintf("margin-top:%.1fpt", tb.DistTop))
	}
	if tb.DistBottom > 0 {
		styleAttrs = append(styleAttrs, fmt.Sprintf("margin-bottom:%.1fpt", tb.DistBottom))
	}

	fb, ok := r.floatByTextBox[tb]
	if !ok {
		if strings.EqualFold(tb.XAlign, "center") {
			styleAttrs = append(styleAttrs, "margin-left:auto", "margin-right:auto")
		} else if strings.EqualFold(tb.XAlign, "right") {
			styleAttrs = append(styleAttrs, "margin-left:auto")
		}
		return styleAttrs, widthPt, heightPt, true
	}
	paraBox, ok := r.paraBoxes[r.currentParagraph.Index]
	if !ok || paraBox.Page != fb.Page {
		return styleAttrs, widthPt, heightPt, true
	}

	columnLeftPt := paraBox.Geometry.MarginLeft + float64(paraBox.Column)*(paraBox.Geometry.ColumnWidthPt+paraBox.Geometry.ColumnGapPt)
	relLeftPt := fb.X - columnLeftPt
	rightInsetPt := paraBox.Geometry.ColumnWidthPt - (relLeftPt + fb.Width)
	if strings.EqualFold(tb.XAlign, "center") || almostSameFloat(relLeftPt, rightInsetPt) {
		styleAttrs = append(styleAttrs, "margin-left:auto", "margin-right:auto")
	} else if strings.EqualFold(tb.XAlign, "right") || rightInsetPt < relLeftPt {
		styleAttrs = append(styleAttrs, "margin-left:auto")
		if tb.DistLeft > 0 {
			styleAttrs = append(styleAttrs, fmt.Sprintf("margin-left:%.1fpt", tb.DistLeft))
		}
	} else {
		if relLeftPt > 0 {
			styleAttrs = append(styleAttrs, fmt.Sprintf("margin-left:%.1fpt", relLeftPt))
		}
		if tb.DistRight > 0 {
			styleAttrs = append(styleAttrs, fmt.Sprintf("margin-right:%.1fpt", tb.DistRight))
		}
	}
	return styleAttrs, widthPt, heightPt, true
}

func (r *htmlRenderer) renderParagraphBlockFloatShape(shape *resolve.Shape) bool {
	styleAttrs, widthPt, heightPt, ok := r.paragraphBlockFloatShapeStyleAttrs(shape)
	if !ok {
		return false
	}
	r.renderShapeWithStyleAttrs(&r.buf, shape, styleAttrs, widthPt, heightPt)
	return true
}

func (r *htmlRenderer) paragraphBlockFloatShapeStyleAttrs(shape *resolve.Shape) ([]string, float64, float64, bool) {
	if shape == nil || !shape.Floating || shape.BehindDoc || r.currentParagraph == nil {
		return nil, 0, 0, false
	}
	if !strings.EqualFold(shape.WrapType, "topAndBottom") {
		return nil, 0, 0, false
	}
	if !paragraphBlockFloatShapeEligible(r.currentParagraph, shape) {
		return nil, 0, 0, false
	}

	widthPt := emuToPoints(shape.Width)
	heightPt := emuToPoints(shape.Height)
	styleAttrs := []string{"display:block", "clear:both", "position:relative"}
	if fb, ok := r.shapeFloatBox(shape); ok {
		widthPt = boxWidthPt(fb.Width, widthPt)
		heightPt = boxHeightPt(fb.Height, heightPt)
	}
	if shape.DistTop > 0 {
		styleAttrs = append(styleAttrs, fmt.Sprintf("margin-top:%.1fpt", shape.DistTop))
	}
	if shape.DistBottom > 0 {
		styleAttrs = append(styleAttrs, fmt.Sprintf("margin-bottom:%.1fpt", shape.DistBottom))
	}

	fb, ok := r.shapeFloatBox(shape)
	if !ok {
		if strings.EqualFold(shape.XAlign, "center") {
			styleAttrs = append(styleAttrs, "margin-left:auto", "margin-right:auto")
		} else if strings.EqualFold(shape.XAlign, "right") {
			styleAttrs = append(styleAttrs, "margin-left:auto")
		}
		return styleAttrs, widthPt, heightPt, true
	}
	paraBox, ok := r.paraBoxes[r.currentParagraph.Index]
	if !ok || paraBox.Page != fb.Page {
		return styleAttrs, widthPt, heightPt, true
	}

	columnLeftPt := paraBox.Geometry.MarginLeft + float64(paraBox.Column)*(paraBox.Geometry.ColumnWidthPt+paraBox.Geometry.ColumnGapPt)
	relLeftPt := fb.X - columnLeftPt
	rightInsetPt := paraBox.Geometry.ColumnWidthPt - (relLeftPt + fb.Width)
	if strings.EqualFold(shape.XAlign, "center") || almostSameFloat(relLeftPt, rightInsetPt) {
		styleAttrs = append(styleAttrs, "margin-left:auto", "margin-right:auto")
	} else if strings.EqualFold(shape.XAlign, "right") || rightInsetPt < relLeftPt {
		styleAttrs = append(styleAttrs, "margin-left:auto")
		if shape.DistLeft > 0 {
			styleAttrs = append(styleAttrs, fmt.Sprintf("margin-left:%.1fpt", shape.DistLeft))
		}
	} else {
		if relLeftPt > 0 {
			styleAttrs = append(styleAttrs, fmt.Sprintf("margin-left:%.1fpt", relLeftPt))
		}
		if shape.DistRight > 0 {
			styleAttrs = append(styleAttrs, fmt.Sprintf("margin-right:%.1fpt", shape.DistRight))
		}
	}
	return styleAttrs, widthPt, heightPt, true
}

func (r *htmlRenderer) paragraphBlockFloatImageStyleAttrs(img *resolve.Image) ([]string, bool) {
	if img == nil || !img.Floating || img.BehindDoc || img.Hidden || r.currentParagraph == nil {
		return nil, false
	}
	if !strings.EqualFold(img.WrapType, "topAndBottom") {
		return nil, false
	}
	if !paragraphBlockFloatImageEligible(r.currentParagraph, img) {
		return nil, false
	}

	styleAttrs := []string{"display:block", "clear:both"}
	if img.Width > 0 {
		styleAttrs = append(styleAttrs, fmt.Sprintf("width:%.1fpt", emuToPoints(img.Width)))
	}
	if img.Height > 0 {
		styleAttrs = append(styleAttrs, fmt.Sprintf("height:%.1fpt", emuToPoints(img.Height)))
	}
	if img.DistTop > 0 {
		styleAttrs = append(styleAttrs, fmt.Sprintf("margin-top:%.1fpt", img.DistTop))
	}
	if img.DistBottom > 0 {
		styleAttrs = append(styleAttrs, fmt.Sprintf("margin-bottom:%.1fpt", img.DistBottom))
	}

	fb, ok := r.floatByImage[img]
	if !ok {
		if strings.EqualFold(img.XAlign, "center") {
			styleAttrs = append(styleAttrs, "margin-left:auto", "margin-right:auto")
		} else if strings.EqualFold(img.XAlign, "right") {
			styleAttrs = append(styleAttrs, "margin-left:auto")
		}
		return styleAttrs, true
	}
	paraBox, ok := r.paraBoxes[r.currentParagraph.Index]
	if !ok || paraBox.Page != fb.Page {
		return styleAttrs, true
	}

	columnLeftPt := paraBox.Geometry.MarginLeft + float64(paraBox.Column)*(paraBox.Geometry.ColumnWidthPt+paraBox.Geometry.ColumnGapPt)
	relLeftPt := fb.X - columnLeftPt
	rightInsetPt := paraBox.Geometry.ColumnWidthPt - (relLeftPt + fb.Width)
	if strings.EqualFold(img.XAlign, "center") || almostSameFloat(relLeftPt, rightInsetPt) {
		styleAttrs = append(styleAttrs, "margin-left:auto", "margin-right:auto")
	} else if strings.EqualFold(img.XAlign, "right") || rightInsetPt < relLeftPt {
		styleAttrs = append(styleAttrs, "margin-left:auto")
		if img.DistLeft > 0 {
			styleAttrs = append(styleAttrs, fmt.Sprintf("margin-left:%.1fpt", img.DistLeft))
		}
	} else {
		if relLeftPt > 0 {
			styleAttrs = append(styleAttrs, fmt.Sprintf("margin-left:%.1fpt", relLeftPt))
		}
		if img.DistRight > 0 {
			styleAttrs = append(styleAttrs, fmt.Sprintf("margin-right:%.1fpt", img.DistRight))
		}
	}
	return styleAttrs, true
}

func (r *htmlRenderer) paragraphFloatDirection(wrapSide, xAlign string, fb layout.FloatBox, paraBox layout.BlockBox) string {
	switch strings.ToLower(wrapSide) {
	case "left":
		return "right"
	case "right":
		return "left"
	case "largest":
		columnLeftPt := paraBox.Geometry.MarginLeft + float64(paraBox.Column)*(paraBox.Geometry.ColumnWidthPt+paraBox.Geometry.ColumnGapPt)
		relLeftPt := fb.X - columnLeftPt
		rightInsetPt := paraBox.Geometry.ColumnWidthPt - (relLeftPt + fb.Width)
		if rightInsetPt > relLeftPt {
			return "left"
		}
		return "right"
	}
	if strings.EqualFold(xAlign, "right") {
		return "right"
	}
	if strings.EqualFold(xAlign, "left") {
		return "left"
	}
	columnLeftPt := paraBox.Geometry.MarginLeft + float64(paraBox.Column)*(paraBox.Geometry.ColumnWidthPt+paraBox.Geometry.ColumnGapPt)
	relLeftPt := fb.X - columnLeftPt
	if relLeftPt+(fb.Width/2.0) > paraBox.Geometry.ColumnWidthPt/2.0 {
		return "right"
	}
	return "left"
}

func boxWidthPt(layoutWidth, emuWidth float64) float64 {
	if layoutWidth > 0 {
		return layoutWidth
	}
	return emuWidth
}

func boxHeightPt(layoutHeight, emuHeight float64) float64 {
	if layoutHeight > 0 {
		return layoutHeight
	}
	return emuHeight
}

func (r *htmlRenderer) appendTextBoxBoxStyles(styles []string, tb *resolve.TextBox, widthPt, heightPt float64) []string {
	if widthPt > 0 {
		styles = append(styles, fmt.Sprintf("width:%.1fpt", widthPt))
	} else if tb.SizeRelHPct > 0 {
		styles = append(styles, r.relativeSizeStyle(tb.SizeRelHPct, tb.SizeRelHFrom, true))
	}
	if heightPt > 0 {
		styles = append(styles, fmt.Sprintf("height:%.1fpt", heightPt))
	} else if tb.SizeRelVPct > 0 {
		styles = append(styles, r.relativeSizeStyle(tb.SizeRelVPct, tb.SizeRelVFrom, false))
	}
	if tb.FillColor != "" {
		styles = append(styles, "background-color:#"+tb.FillColor)
	}
	if tb.BorderColor != "" && tb.BorderWidth > 0 {
		styles = append(styles, fmt.Sprintf("border:%.2fpt solid #%s", tb.BorderWidth, tb.BorderColor))
	}
	pl, pt, pr, pb := tb.PaddingLeft, tb.PaddingTop, tb.PaddingRight, tb.PaddingBottom
	if pl != 0 || pt != 0 || pr != 0 || pb != 0 {
		if pl == pr && pt == pb && pl == pt {
			styles = append(styles, fmt.Sprintf("padding:%.2fpt", pl))
		} else if pl == pr && pt == pb {
			styles = append(styles, fmt.Sprintf("padding:%.2fpt %.2fpt", pt, pl))
		} else {
			styles = append(styles, fmt.Sprintf("padding:%.2fpt %.2fpt %.2fpt %.2fpt", pt, pr, pb, pl))
		}
	}
	if tb.BehindDoc {
		styles = append(styles, "z-index:0")
	} else {
		styles = append(styles, fmt.Sprintf("z-index:%d", r.normalizeZIndex(tb.ZIndex)))
	}
	if tb.NoWrap {
		styles = append(styles, "white-space:nowrap")
	}
	if tb.Locked {
		styles = append(styles, "pointer-events:none;user-select:none")
	}
	switch tb.Autofit {
	case "none":
		styles = append(styles, "overflow:visible")
	case "shape":
		styles = append(styles, "overflow:hidden")
	case "norm":
		if tb.FontScale > 0 {
			scale := float64(tb.FontScale) / 100000.0
			styles = append(styles, fmt.Sprintf("font-size:%.2f%%", scale*100))
		}
		if tb.LnSpcReduction > 0 {
			reduction := float64(tb.LnSpcReduction) / 100000.0
			lineHeight := 1.0 - reduction
			if lineHeight < 0.1 {
				lineHeight = 0.1
			}
			styles = append(styles, fmt.Sprintf("line-height:%.2f", lineHeight))
		}
	}
	if tb.FlatTx {
		styles = append(styles, "transform-style:flat")
	}
	textTransform := ""
	switch tb.Vert {
	case "vert":
		styles = append(styles, "writing-mode:vertical-rl", "text-orientation:mixed")
	case "vert270":
		styles = append(styles, "writing-mode:vertical-lr", "text-orientation:mixed")
		textTransform = "rotate(180deg)"
	}
	baseTransform := boxTransformCSS(tb.RotationDeg, tb.FlipH, tb.FlipV)
	if textTransform != "" {
		if baseTransform != "" {
			baseTransform = textTransform + " " + baseTransform
		} else {
			baseTransform = textTransform
		}
	}
	if effectStyles := textBoxEffectStyleProps(tb); len(effectStyles) > 0 {
		styles = append(styles, "overflow:visible")
		transform, origin := mergeBoxTransformWithEffects(baseTransform, effectStyles)
		if transform != "" {
			styles = append(styles, "transform:"+transform)
			if origin != "" {
				styles = append(styles, "transform-origin:"+origin)
			} else {
				styles = append(styles, "transform-origin:50% 50%")
			}
		} else if origin != "" {
			styles = append(styles, "transform-origin:"+origin)
		}
		styles = appendSortedStyleProps(styles, effectStyles)
	} else if baseTransform != "" {
		transform := baseTransform
		styles = append(styles, "transform:"+transform, "transform-origin:50% 50%")
	}
	return styles
}

func wordTableCellTextDirectionCSS(direction string) []string {
	switch strings.TrimSpace(direction) {
	case "tbRl", "tbRlV":
		return []string{"writing-mode:vertical-rl", "text-orientation:mixed"}
	case "btLr":
		return []string{"writing-mode:vertical-lr", "text-orientation:mixed", "transform:rotate(180deg)", "transform-origin:center center"}
	default:
		return nil
	}
}

// cell3DBoxShadowCSS approximates DrawingML cell 3D bevel/extrusion with CSS
// box-shadow. EMU values are converted to pixels (1 px ≈ 9525 EMUs).
func cell3DBoxShadowCSS(c3d *resolve.Cell3D) []string {
	if c3d == nil {
		return nil
	}
	const emuPerPx = 9525.0
	var shadows []string
	// Top bevel approximated as an inset highlight.
	if c3d.BevelTopWidth > 0 || c3d.BevelTopHeight > 0 {
		wPx := maxFloat(1.0, float64(c3d.BevelTopWidth)/emuPerPx)
		hPx := maxFloat(1.0, float64(c3d.BevelTopHeight)/emuPerPx)
		shadows = append(shadows, fmt.Sprintf("inset %.1fpx %.1fpx rgba(255,255,255,0.4)", wPx*0.5, hPx*0.5))
		shadows = append(shadows, fmt.Sprintf("inset -%.1fpx -%.1fpx rgba(0,0,0,0.15)", wPx*0.5, hPx*0.5))
	}
	// Bottom bevel / extrusion approximated as an outer shadow.
	if c3d.BevelBottomWidth > 0 || c3d.BevelBottomHeight > 0 {
		wPx := maxFloat(1.0, float64(c3d.BevelBottomWidth)/emuPerPx)
		hPx := maxFloat(1.0, float64(c3d.BevelBottomHeight)/emuPerPx)
		color := c3d.ExtrusionColor
		if color == "" {
			color = "000000"
		}
		shadows = append(shadows, fmt.Sprintf("%.1fpx %.1fpx %.1fpx #%s", wPx*0.5, hPx*0.5, math.Max(wPx, hPx)*0.5, color))
	}
	if len(shadows) == 0 {
		return nil
	}
	return []string{fmt.Sprintf("box-shadow:%s", strings.Join(shadows, ","))}
}

func textBoxViewportSize(widthPt, heightPt, pageWidthPt, pageHeightPt float64) (float64, float64) {
	w := widthPt
	if w <= 0 {
		w = pageWidthPt
	}
	h := heightPt
	if h <= 0 {
		h = pageHeightPt
	}
	if w <= 0 {
		w = 1
	}
	if h <= 0 {
		h = 1
	}
	return w, h
}

func trimTerminalEmptyTextBoxBlocks(blocks []resolve.Block) []resolve.Block {
	end := len(blocks)
	for end > 0 {
		b := blocks[end-1]
		if b.Paragraph == nil || !paragraphRendersNothing(b.Paragraph) {
			break
		}
		end--
	}
	return blocks[:end]
}

// paragraphRendersNothing reports whether a paragraph contributes no content at
// all. A paragraph that only anchors a floating drawing is not empty here: the
// drawing belongs to the current story's own overlay, so trimming the
// paragraph would drop the object from the output.
func paragraphRendersNothing(p *resolve.Paragraph) bool {
	if p == nil {
		return true
	}
	if strings.TrimSpace(p.Text) != "" {
		return false
	}
	for _, run := range p.Runs {
		if strings.TrimSpace(run.Text) != "" {
			return false
		}
		switch run.Kind {
		case "image", "drawing", "textbox", "shape", "chart", "olePlaceholder",
			"horzRule", "break", "tab", "footnoteRef", "endnoteRef", "commentRef",
			"annotationRef", "math", "ruby", "formField", "symbol":
			return false
		}
		if run.ContentControl != nil || run.Image != nil || run.TextBox != nil || run.Shape != nil || run.Chart != nil {
			return false
		}
	}
	return true
}

func (r *htmlRenderer) renderTextBoxContentHTML(tb *resolve.TextBox, widthPt, heightPt float64) string {
	var content bytes.Buffer

	// A textbox is its own layout viewport. Never expand its story geometry to
	// page size: doing so makes paragraph line breaking happen at page width
	// while the DOM is visually clipped to the much smaller shape rectangle.
	tbWidth, tbHeight := textBoxViewportSize(widthPt, heightPt, r.pageWidthPt, r.pageHeightPt)

	// The outer textbox element owns the DrawingML body insets as CSS padding.
	// Local story layout therefore operates in the already-inset content box.
	// Re-applying the insets as PageGeometry margins would shift spatial story
	// content (and nested floats) a second time; this is especially visible in
	// vertically centred one-line textboxes.
	tbContentW := tbWidth - tb.PaddingLeft - tb.PaddingRight
	if tbContentW < 1 {
		tbContentW = tbWidth
	}
	tbContentH := tbHeight - tb.PaddingTop - tb.PaddingBottom
	if tbContentH < 1 {
		tbContentH = tbHeight
	}
	geom := layout.PageGeometry{
		Width:         tbContentW,
		Height:        tbContentH,
		ColumnCount:   1,
		ColumnWidthPt: tbContentW,
	}
	blocks := trimTerminalEmptyTextBoxBlocks(tb.Blocks)
	r.withOutputBuffer(&content, func() {
		r.withLocalLayout(blocks, geom, func(_ []layout.BlockBox) {
			r.withLineNumberingSuppressed(func() {
				r.renderBlocks(blocks)
			})
			r.closeAllLists()
			if r.overlay.Len() > 0 {
				r.buf.WriteString(`<div class="story-overlay">`)
				r.buf.Write(r.overlay.Bytes())
				r.buf.WriteString(`</div>`)
			}
		})
	})
	return content.String()
}

func stripIDAttributes(markup string) string {
	re := regexp.MustCompile(`\s+id="[^"]*"`)
	return re.ReplaceAllString(markup, "")
}

func extractTextBoxReflection(tb *resolve.TextBox) *dml.Reflection {
	if tb == nil {
		return nil
	}
	for _, effect := range tb.Effects.Effects {
		if refl, ok := effect.(dml.Reflection); ok {
			return &refl
		}
		if refl, ok := effect.(*dml.Reflection); ok {
			return refl
		}
	}
	return nil
}

func textBoxReflectionStyles(tb *resolve.TextBox, widthPt, heightPt float64) []string {
	refl := extractTextBoxReflection(tb)
	if refl == nil || widthPt <= 0 || heightPt <= 0 {
		return nil
	}
	scaleY := refl.Size
	if scaleY <= 0 || scaleY > 1 {
		scaleY = 1
	}
	rad := refl.Direction * math.Pi / 180
	offX := refl.Distance * math.Cos(rad)
	offY := refl.Distance * math.Sin(rad)
	translateY := heightPt + offY + scaleY*heightPt
	alpha := refl.Alpha
	if alpha <= 0 {
		alpha = 0.3
	}
	styles := []string{
		"position:absolute",
		"pointer-events:none",
		"left:0",
		"top:0",
		"width:100%",
		"height:100%",
		fmt.Sprintf("transform:translate(%.2fpt, %.2fpt) scaleY(-%.4f)", offX, translateY, scaleY),
		"transform-origin:top left",
		fmt.Sprintf("opacity:%.4f", alpha),
		"overflow:visible",
	}
	if refl.Blur > 0 {
		styles = append(styles, fmt.Sprintf("filter:blur(%.1fpt)", refl.Blur))
	}
	if mask := shapeReflectionMaskGradientCSS(refl); mask != "" {
		styles = append(styles, "mask-image:"+mask)
		styles = append(styles, "-webkit-mask-image:"+mask)
	}
	return styles
}

func (r *htmlRenderer) writeTextBoxElement(tb *resolve.TextBox, styleAttrs []string, widthPt, heightPt float64) {
	fmt.Fprintf(&r.buf, `<div class="floating-textbox" style=%q>`, strings.Join(styleAttrs, ";"))
	contentHTML := r.renderTextBoxContentHTML(tb, widthPt, heightPt)
	if reflectionStyles := textBoxReflectionStyles(tb, widthPt, heightPt); len(reflectionStyles) > 0 {
		fmt.Fprintf(&r.buf, `<div class="textbox-reflection" aria-hidden="true" style=%q>`, strings.Join(reflectionStyles, ";"))
		r.buf.WriteString(stripIDAttributes(contentHTML))
		r.buf.WriteString(`</div>`)
	}
	innerStyles := textBoxInnerStyles(tb)
	if len(innerStyles) > 0 {
		fmt.Fprintf(&r.buf, `<div style=%q>`, strings.Join(innerStyles, ";"))
	}
	r.buf.WriteString(contentHTML)
	if len(innerStyles) > 0 {
		r.buf.WriteString(`</div>`)
	}
	r.buf.WriteString("</div>")
}

// textBoxInnerStyles returns CSS style declarations for the inner content
// wrapper of a text box. It handles vertical alignment (anchor), horizontal
// alignment, text rotation, and upright text. Paragraphs retain horizontal alignment.
func textBoxInnerStyles(tb *resolve.TextBox) []string {
	var styles []string
	switch tb.Anchor {
	case "center":
		styles = append(styles, "display:grid", "align-items:center", "height:100%")
	case "bottom":
		styles = append(styles, "display:grid", "align-items:end", "height:100%")
	}
	if tb.AnchorCenter {
		// a:bodyPr@anchorCtr centers the text box's text horizontally; the
		// paragraphs keep their own alignment inside that anchor.
		styles = append(styles, "text-align:center")
	}
	if tb.NumCol > 1 {
		styles = append(styles, fmt.Sprintf("column-count:%d", tb.NumCol))
		if tb.SpcCol > 0 {
			styles = append(styles, fmt.Sprintf("column-gap:%.1fpt", tb.SpcCol))
		}
		if tb.RtlCol {
			styles = append(styles, "direction:rtl")
		}
	}
	var transforms []string
	if tb.Upright && tb.RotationDeg != 0 {
		transforms = append(transforms, fmt.Sprintf("rotate(%.2fdeg)", -tb.RotationDeg))
	}
	if tb.TextRotation != 0 {
		transforms = append(transforms, fmt.Sprintf("rotate(%.2fdeg)", tb.TextRotation))
	}
	if len(transforms) > 0 {
		styles = append(styles, "transform:"+strings.Join(transforms, " "), "transform-origin:50% 50%")
	}
	return styles
}

// renderImageTo writes an image to buf. Floating images use layout-computed
// page-absolute coordinates; inline images carry no positioning.
func (r *htmlRenderer) renderImageTo(buf *bytes.Buffer, img *resolve.Image, cc *resolve.ContentControl) {
	if img != nil && img.Hidden {
		return
	}
	if img != nil && img.Floating && r.options.FastPreview {
		r.renderFastPreviewImage(buf, img, cc)
		return
	}
	var styleAttrs []string
	if img.Locked {
		styleAttrs = append(styleAttrs, "pointer-events:none;user-select:none")
	}
	if img.Floating {
		if fb, ok := r.floatByImage[img]; ok && fb.Width > 0 {
			styleAttrs = append(styleAttrs, fmt.Sprintf("width:%.1fpt", fb.Width))
		} else if img.SizeRelHPct > 0 {
			styleAttrs = append(styleAttrs, r.relativeSizeStyle(img.SizeRelHPct, img.SizeRelHFrom, true))
		} else if img.Width > 0 {
			styleAttrs = append(styleAttrs, fmt.Sprintf("width:%.1fpt", emuToPoints(img.Width)))
		}
		if fb, ok := r.floatByImage[img]; ok && fb.Height > 0 {
			styleAttrs = append(styleAttrs, fmt.Sprintf("height:%.1fpt", fb.Height))
		} else if img.SizeRelVPct > 0 {
			styleAttrs = append(styleAttrs, r.relativeSizeStyle(img.SizeRelVPct, img.SizeRelVFrom, false))
		} else if img.Height > 0 {
			styleAttrs = append(styleAttrs, fmt.Sprintf("height:%.1fpt", emuToPoints(img.Height)))
		}
	} else {
		if img.SizeRelHPct > 0 {
			styleAttrs = append(styleAttrs, r.relativeSizeStyle(img.SizeRelHPct, img.SizeRelHFrom, true))
		} else if img.Width > 0 {
			styleAttrs = append(styleAttrs, fmt.Sprintf("width:%.1fpt", emuToPoints(img.Width)))
		}
		if img.SizeRelVPct > 0 {
			styleAttrs = append(styleAttrs, r.relativeSizeStyle(img.SizeRelVPct, img.SizeRelVFrom, false))
		} else if img.Height > 0 {
			styleAttrs = append(styleAttrs, fmt.Sprintf("height:%.1fpt", emuToPoints(img.Height)))
		}
		if img.EffectExtentL > 0 {
			// An inline drawing's flow box is wp:effectExtent + wp:extent; the
			// bitmap itself is painted inset by the left effect extent inside
			// that box. Layout advanced the full box, so restore the inset here.
			styleAttrs = append(styleAttrs, fmt.Sprintf("margin-left:%.2fpt", emuToPoints(img.EffectExtentL)))
		}
	}
	if img.Floating {
		if img.BehindDoc {
			styleAttrs = append(styleAttrs, "position:absolute;z-index:0")
		} else {
			styleAttrs = append(styleAttrs, fmt.Sprintf("position:absolute;z-index:%d", r.normalizeZIndex(img.ZIndex)))
		}
		if img.SimplePosAttr {
			styleAttrs = append(styleAttrs, fmt.Sprintf("left:%.1fpt", emuToPoints(img.SimplePosX)))
			styleAttrs = append(styleAttrs, fmt.Sprintf("top:%.1fpt", emuToPoints(img.SimplePosY)))
		} else if fb, ok := r.floatByImage[img]; ok {
			styleAttrs = append(styleAttrs, fmt.Sprintf("left:%.1fpt", fb.X))
			styleAttrs = append(styleAttrs, fmt.Sprintf("top:%.1fpt", fb.Y))
		} else {
			styleAttrs = append(styleAttrs, fmt.Sprintf("left:%.1fpt", r.resolveFloatLeft(img.X, img.XAlign, img.XRelative, img.Width, false)))
			styleAttrs = append(styleAttrs, fmt.Sprintf("top:%.1fpt", r.resolveFloatTop(img.Y, img.YAlign, img.YRelative, img.Height, false)))
		}
		// Note: DistTop/DistRight/DistBottom/DistLeft are text-wrapping clearances
		// for surrounding flow text. They must not be applied as CSS margins on
		// absolutely-positioned overlay images (left/top encode the exact page position).
	}
	if img.FillColor != "" {
		styleAttrs = append(styleAttrs, "background-color:#"+img.FillColor)
	}
	if img.BorderColor != "" && img.BorderWidth > 0 {
		styleAttrs = append(styleAttrs, fmt.Sprintf("border:%.2fpt solid #%s", img.BorderWidth, img.BorderColor))
	}
	styleAttrs = append(styleAttrs, pictureFrameGeometryStyles(img)...)
	r.writeImageElement(buf, img, styleAttrs, cc)
}

// renderFastPreviewImage keeps floating drawings in semantic flow. Fast preview
// has no page coordinate map, so emitting position:absolute would pin every
// later-page image to the visible page's origin and make it cover the text.
func (r *htmlRenderer) renderFastPreviewImage(buf *bytes.Buffer, img *resolve.Image, cc *resolve.ContentControl) {
	styles := []string{"display:block", "max-width:100%"}
	if img.Width > 0 {
		styles = append(styles, fmt.Sprintf("width:%.1fpt", emuToPoints(img.Width)))
	}
	if img.Height > 0 {
		styles = append(styles, fmt.Sprintf("height:%.1fpt", emuToPoints(img.Height)))
	}
	if img.DistTop > 0 {
		styles = append(styles, fmt.Sprintf("margin-top:%.1fpt", img.DistTop))
	}
	if img.DistBottom > 0 {
		styles = append(styles, fmt.Sprintf("margin-bottom:%.1fpt", img.DistBottom))
	}
	// Preserve the useful part of Word's square/tight wrapping in the one-page
	// preview. A CSS float stays in document flow, unlike the page-absolute
	// placement used by the fully paginated renderer, so anchors from later
	// pages cannot cover this preview's content.
	wrapType := strings.ToLower(strings.TrimSpace(img.WrapType))
	wrapsText := wrapType == "square" || wrapType == "tight" || wrapType == "through"
	if wrapsText && !img.BehindDoc {
		side := "left"
		if strings.EqualFold(img.XAlign, "right") || (!strings.EqualFold(img.XAlign, "center") && img.X > 0) {
			side = "right"
		}
		styles = append(styles, "float:"+side)
		if side == "left" && img.DistRight > 0 {
			styles = append(styles, fmt.Sprintf("margin-right:%.1fpt", img.DistRight))
		}
		if side == "right" && img.DistLeft > 0 {
			styles = append(styles, fmt.Sprintf("margin-left:%.1fpt", img.DistLeft))
		}
	} else {
		switch strings.ToLower(img.XAlign) {
		case "right":
			styles = append(styles, "margin-left:auto")
		case "center":
			styles = append(styles, "margin-left:auto", "margin-right:auto")
		}
	}
	if img.BorderColor != "" && img.BorderWidth > 0 {
		styles = append(styles, fmt.Sprintf("border:%.2fpt solid #%s", img.BorderWidth, img.BorderColor))
	}
	styles = append(styles, pictureFrameGeometryStyles(img)...)
	r.writeImageElement(buf, img, styles, cc)
}

// pictureFrameGeometryStyles renders the authored DrawingML picture frame.
// The bitmap remains ordinary rectangular image data; pic:spPr/a:prstGeom
// defines the frame into which Word paints it. Frame geometry never invents
// a border: pic:spPr/a:ln remains the sole owner of an authored outline.
func pictureFrameGeometryStyles(img *resolve.Image) []string {
	if img == nil {
		return nil
	}
	switch strings.ToLower(strings.TrimSpace(img.FrameGeometry)) {
	case "ellipse", "flowchartconnector":
		// Word paints the bitmap through the picture-frame geometry. Keep
		// border-radius for normal HTML layout and use clip-path as the
		// authoritative bitmap clip (including absolutely positioned images).
		return []string{
			"border-radius:50%",
			"clip-path:ellipse(50% 50% at 50% 50%)",
			"overflow:hidden",
		}
	default:
		return nil
	}
}

func (r *htmlRenderer) writeImageElement(buf *bytes.Buffer, img *resolve.Image, styleAttrs []string, cc *resolve.ContentControl) {
	if cc != nil && cc.Type != "picture" {
		var inner bytes.Buffer
		r.writeImageElement(&inner, img, styleAttrs, nil)
		fmt.Fprintf(buf, `<span class=%q%s>%s</span>`, sdtClass(cc), sdtAttrs(cc), inner.String())
		return
	}
	alt := html.EscapeString(img.Alt)
	sdtAttrs := pictureSdtAttrs(cc)
	titleAttr := ""
	if img.Title != "" {
		titleAttr = fmt.Sprintf(` title=%q`, html.EscapeString(img.Title))
	}

	if isBrowserImage(img.MIMEType) && len(img.Data) > 0 {
		src := r.imageSource(img)
		if img.Tile {
			// Tiled images render as a <div> with background-repeat.
			tileStyles := append([]string(nil), styleAttrs...)
			tileStyles = append(tileStyles, fmt.Sprintf("background-image:url(%s)", src))
			tileStyles = append(tileStyles, "background-repeat:repeat")
			// Apply tile scale as background-size (EMU -> px at 96 DPI)
			if img.Width > 0 && img.Height > 0 {
				sx := float64(img.TileScaleX) / 100000.0
				sy := float64(img.TileScaleY) / 100000.0
				if sx <= 0 {
					sx = 1.0
				}
				if sy <= 0 {
					sy = 1.0
				}
				wPx := float64(img.Width) / 9525.0 * sx
				hPx := float64(img.Height) / 9525.0 * sy
				tileStyles = append(tileStyles, fmt.Sprintf("background-size:%.2fpx %.2fpx", wPx, hPx))
			}
			// Apply tile offset or alignment as background-position
			if img.TileOffsetX != 0 || img.TileOffsetY != 0 {
				xPx := float64(img.TileOffsetX) / 9525.0
				yPx := float64(img.TileOffsetY) / 9525.0
				tileStyles = append(tileStyles, fmt.Sprintf("background-position:%.2fpx %.2fpx", xPx, yPx))
			} else if pos := tileAlignToCSS(img.TileAlign); pos != "" {
				tileStyles = append(tileStyles, fmt.Sprintf("background-position:%s", pos))
			}
			if filter := imageCombinedFilter(img); filter != "" {
				tileStyles = append(tileStyles, "filter:"+filter)
			}
			if opacity := imageCombinedOpacity(img); opacity != "" {
				tileStyles = append(tileStyles, opacity)
			}
			if bs := imageBoxShadow(img); bs != "" {
				tileStyles = append(tileStyles, "box-shadow:"+bs)
			}
			classAttr := ""
			if sdtAttrs != "" {
				classAttr = ` class="word-sdt-picture"`
			}
			altAttr := ""
			if alt != "" {
				altAttr = fmt.Sprintf(` aria-label=%q`, alt)
			}
			fmt.Fprintf(buf, "<div%s%s%s%s%s></div>", classAttr, sdtAttrs, titleAttr, altAttr, inlineStyleAttr(tileStyles))
			return
		}
		if imageNeedsWrapper(img) {
			outerStyles, innerStyles := wrappedImageStyles(img, styleAttrs)
			frameClass := "image-frame"
			if sdtAttrs != "" {
				frameClass += " word-sdt word-sdt-picture"
			}
			fmt.Fprintf(buf, `<span class=%q%s%s><img src=%q alt=%q%s%s></span>`,
				frameClass, sdtAttrs, inlineStyleAttr(outerStyles), src, alt, titleAttr, inlineStyleAttr(innerStyles))
			return
		}
		classAttr := ""
		if sdtAttrs != "" {
			classAttr = ` class="word-sdt-picture"`
		}
		imgStyles := append([]string(nil), styleAttrs...)
		if !img.Stretch {
			imgStyles = append(imgStyles, "object-fit:contain")
		}
		if filter := imageCombinedFilter(img); filter != "" {
			imgStyles = append(imgStyles, "filter:"+filter)
		}
		if opacity := imageCombinedOpacity(img); opacity != "" {
			imgStyles = append(imgStyles, opacity)
		}
		if bs := imageBoxShadow(img); bs != "" {
			imgStyles = append(imgStyles, "box-shadow:"+bs)
		}
		fmt.Fprintf(buf, "<img src=%q alt=%q%s%s%s%s>", src, alt, classAttr, sdtAttrs, titleAttr, inlineStyleAttr(imgStyles))
	} else {
		// Non-displayable type (EMF, WMF, TIFF, …): show a labelled placeholder.
		label := img.MIMEType
		if label == "" {
			label = "image"
		}
		className := "image-placeholder"
		if sdtAttrs != "" {
			className += " word-sdt word-sdt-picture"
		}
		placeholderTitle := img.Alt
		if img.Title != "" {
			placeholderTitle = img.Title
		}
		fmt.Fprintf(buf, "<span class=%q title=%q%s%s>[%s]</span>",
			className, html.EscapeString(placeholderTitle), sdtAttrs, inlineStyleAttr(styleAttrs), html.EscapeString(label))
	}
}

func pictureSdtAttrs(cc *resolve.ContentControl) string {
	if cc == nil || cc.Type != "picture" {
		return ""
	}
	return sdtAttrs(cc)
}

func (r *htmlRenderer) imageSource(img *resolve.Image) string {
	if r != nil && r.options.ImageMode == HTMLImageModeExternal {
		return r.addImageAsset(img)
	}
	return "data:" + img.MIMEType + ";base64," + base64.StdEncoding.EncodeToString(img.Data)
}

func (r *htmlRenderer) addImageAsset(img *resolve.Image) string {
	if r == nil || img == nil {
		return ""
	}
	r.nextImageIndex++
	assetPath := htmlasset.AssetPath(r.options.ExternalImageDir, "image", r.nextImageIndex, htmlasset.ImageFileExtension(img.MIMEType))
	r.assets = append(r.assets, HTMLAsset{
		Path:        assetPath,
		ContentType: img.MIMEType,
		Data:        append([]byte(nil), img.Data...),
	})
	return assetPath
}

func inlineStyleAttr(styleAttrs []string) string {
	if len(styleAttrs) == 0 {
		return ""
	}
	return fmt.Sprintf(" style=%q", strings.Join(styleAttrs, ";"))
}

func appendSortedStyleProps(dst []string, props map[string]string) []string {
	if len(props) == 0 {
		return dst
	}
	keys := make([]string, 0, len(props))
	for key := range props {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		dst = append(dst, key+":"+props[key])
	}
	return dst
}

func shapeEffectStyleProps(shape *resolve.Shape, includeReflection bool) map[string]string {
	if shape == nil || len(shape.Effects.Effects) == 0 {
		return nil
	}
	if includeReflection {
		return effectStylePropsFromEffects(shape.Effects.Effects)
	}
	filtered := make([]dml.Effect, 0, len(shape.Effects.Effects))
	for _, effect := range shape.Effects.Effects {
		if isReflectionEffect(effect) {
			continue
		}
		filtered = append(filtered, effect)
	}
	return effectStylePropsFromEffects(filtered)
}

func textBoxEffectStyleProps(tb *resolve.TextBox) map[string]string {
	if tb == nil || len(tb.Effects.Effects) == 0 {
		return nil
	}
	filtered := make([]dml.Effect, 0, len(tb.Effects.Effects))
	for _, effect := range tb.Effects.Effects {
		if isReflectionEffect(effect) {
			continue
		}
		filtered = append(filtered, effect)
	}
	return effectStylePropsFromEffects(filtered)
}

func effectStylePropsFromEffects(effects []dml.Effect) map[string]string {
	if len(effects) == 0 {
		return nil
	}
	return drawingcss.EffectList(dml.EffectList{Effects: effects})
}

func mergeBoxTransformWithEffects(base string, props map[string]string) (string, string) {
	if len(props) == 0 {
		return base, ""
	}
	effectTransform := strings.TrimSpace(props["transform"])
	delete(props, "transform")
	origin := strings.TrimSpace(props["transform-origin"])
	delete(props, "transform-origin")
	if effectTransform == "" {
		return base, origin
	}
	if base == "" {
		return effectTransform, origin
	}
	return strings.TrimSpace(base + " " + effectTransform), origin
}

func imageNeedsWrapper(img *resolve.Image) bool {
	if img == nil {
		return false
	}
	return hasImageCrop(img) || img.RotationDeg != 0 || img.FlipH || img.FlipV || img.FillColor != ""
}

func hasImageCrop(img *resolve.Image) bool {
	if img == nil {
		return false
	}
	return img.CropLeft > 0 || img.CropTop > 0 || img.CropRight > 0 || img.CropBottom > 0
}

func wrappedImageStyles(img *resolve.Image, styleAttrs []string) ([]string, []string) {
	outer := append([]string(nil), styleAttrs...)
	if !styleDeclsContainProperty(outer, "display") {
		outer = append(outer, "display:inline-block")
	}
	if !styleDeclsContainProperty(outer, "position") {
		outer = append(outer, "position:relative")
	}
	overflow := "overflow:hidden"
	if imageBoxShadow(img) != "" && !hasImageCrop(img) {
		overflow = "overflow:visible"
	}
	if !styleDeclsContainProperty(outer, "overflow") {
		outer = append(outer, overflow)
	}
	if !styleDeclsContainProperty(outer, "width") && img.Width > 0 {
		outer = append(outer, fmt.Sprintf("width:%.1fpt", emuToPoints(img.Width)))
	}
	if !styleDeclsContainProperty(outer, "height") && img.Height > 0 {
		outer = append(outer, fmt.Sprintf("height:%.1fpt", emuToPoints(img.Height)))
	}

	visibleWidth := 1.0 - img.CropLeft - img.CropRight
	visibleHeight := 1.0 - img.CropTop - img.CropBottom
	if visibleWidth <= 0 || visibleWidth > 1 {
		visibleWidth = 1
	}
	if visibleHeight <= 0 || visibleHeight > 1 {
		visibleHeight = 1
	}

	inner := []string{
		"position:absolute",
		"display:block",
		fmt.Sprintf("width:%.4f%%", 100.0/visibleWidth),
		fmt.Sprintf("height:%.4f%%", 100.0/visibleHeight),
		fmt.Sprintf("left:%.4f%%", -100.0*img.CropLeft/visibleWidth),
		fmt.Sprintf("top:%.4f%%", -100.0*img.CropTop/visibleHeight),
	}
	if transform := imageTransformCSS(img); transform != "" {
		inner = append(inner, "transform:"+transform, "transform-origin:50% 50%")
	}
	if filter := imageCombinedFilter(img); filter != "" {
		inner = append(inner, "filter:"+filter)
	}
	if opacity := imageCombinedOpacity(img); opacity != "" {
		inner = append(inner, opacity)
	}
	if bs := imageBoxShadow(img); bs != "" {
		inner = append(inner, "box-shadow:"+bs)
	}
	return outer, inner
}

func imageTransformCSS(img *resolve.Image) string {
	if img == nil {
		return ""
	}
	return boxTransformCSS(img.RotationDeg, img.FlipH, img.FlipV)
}

func imageFilterCSS(img *resolve.Image) string {
	if img == nil {
		return ""
	}
	var filters []string
	if adjustment := pictureAdjustmentFilterCSS(img.Brightness, img.Contrast); adjustment != "" {
		filters = append(filters, adjustment)
	}
	if img.Grayscale {
		filters = append(filters, "grayscale(100%)")
	}
	if img.BiLevel {
		// Approximate black-and-white with grayscale + brightness + extreme contrast.
		// BiLevelThreshold is in thousandths of a percent (50000 = 50% default).
		threshold := img.BiLevelThreshold
		if threshold <= 0 {
			threshold = 50000
		}
		brightnessPct := 50000.0 / float64(threshold) * 100
		filters = append(filters, fmt.Sprintf("grayscale(100%%) brightness(%.0f%%) contrast(1000%%)", brightnessPct))
	}
	if len(filters) == 0 {
		return ""
	}
	return strings.Join(filters, " ")
}

// pictureAdjustmentFilterCSS renders an authored picture brightness/contrast
// correction. Word shifts levels for brightness and scales them for contrast,
// so the correction is the affine map
//
//	out = scale*in + (1-scale)/2 + offset*(1+scale)/2
//
// where brightness arrives as the 1+offset factor. A plain brightness is
// therefore "in + offset", which the CSS brightness() shorthand cannot express
// (it scales the levels instead of shifting them); the equivalent
// contrast()+brightness() pair reproduces the map exactly, because contrast()
// supplies both the scale and the mid-grey pivot while brightness() restores
// the level shift. The intermediate contrast() output always stays inside
// [0,1], so the pair clamps exactly where Word clamps.
func pictureAdjustmentFilterCSS(brightness, contrast float64) string {
	offset := 0.0
	if brightness > 0 {
		offset = brightness - 1
	}
	scale := 1.0
	if contrast > 0 {
		scale = contrast
	}
	if offset == 0 && scale == 1 {
		return ""
	}
	if offset == 0 {
		return fmt.Sprintf("contrast(%s)", filterPercentCSS(scale*100))
	}
	intercept := (1-scale)/2 + offset*(1+scale)/2
	if intercept < 0 {
		// A darkened picture combined with a strong contrast scale maps dark
		// levels below black; CSS filter functions cannot express the negative
		// intercept, so keep the multiplicative approximation.
		return fmt.Sprintf("brightness(%s) contrast(%s)", filterPercentCSS((1+offset)*100), filterPercentCSS(scale*100))
	}
	return fmt.Sprintf("contrast(%s) brightness(%s)", filterPercentCSS(scale/(2*intercept+scale)*100), filterPercentCSS((2*intercept+scale)*100))
}

// filterPercentCSS formats a CSS filter percentage without trailing zeros.
// Two decimals keep equivalent OfficeArt and DrawingML corrections byte-identical
// (a 16.16 fixed-point contrast and its percentage form can differ in the fourth).
func filterPercentCSS(v float64) string {
	s := strconv.FormatFloat(v, 'f', 2, 64)
	s = strings.TrimRight(s, "0")
	s = strings.TrimSuffix(s, ".")
	return s + "%"
}

func imageOpacityCSS(img *resolve.Image) string {
	if img == nil {
		return ""
	}
	if img.HasAlpha {
		if img.Alpha >= 1 {
			return ""
		}
		return fmt.Sprintf("opacity:%.4f", math.Max(0, img.Alpha))
	}
	if img.Alpha <= 0 || img.Alpha >= 1 {
		return ""
	}
	return fmt.Sprintf("opacity:%.4f", img.Alpha)
}

// imageCombinedFilter merges blip adjustment filters with DrawingML effect filters.
func imageCombinedFilter(img *resolve.Image) string {
	filter := imageFilterCSS(img)
	if img == nil || len(img.Effects.Effects) == 0 {
		return filter
	}
	effectCSS := drawingcss.EffectList(img.Effects)
	if ef := effectCSS["filter"]; ef != "" {
		if filter != "" {
			return filter + " " + ef
		}
		return ef
	}
	return filter
}

// imageCombinedOpacity merges blip alpha with DrawingML effect opacity.
func imageCombinedOpacity(img *resolve.Image) string {
	if img == nil {
		return ""
	}
	alpha := 1.0
	if img.HasAlpha {
		alpha = math.Max(0, math.Min(1, img.Alpha))
	} else if img.Alpha > 0 && img.Alpha < 1 {
		alpha = img.Alpha
	}
	if len(img.Effects.Effects) > 0 {
		effectCSS := drawingcss.EffectList(img.Effects)
		if eo := effectCSS["opacity"]; eo != "" {
			if val, err := strconv.ParseFloat(eo, 64); err == nil {
				alpha *= val
			}
		}
	}
	if alpha >= 0 && alpha < 1 {
		return fmt.Sprintf("opacity:%.4f", alpha)
	}
	return ""
}

// imageBoxShadow returns the box-shadow value from DrawingML effects, if any.
func imageBoxShadow(img *resolve.Image) string {
	if img == nil || len(img.Effects.Effects) == 0 {
		return ""
	}
	effectCSS := drawingcss.EffectList(img.Effects)
	return effectCSS["box-shadow"]
}

// tileAlignToCSS maps OOXML ST_RectAlignment values to CSS background-position.
func tileAlignToCSS(algn string) string {
	switch algn {
	case "tl":
		return "left top"
	case "t":
		return "center top"
	case "tr":
		return "right top"
	case "l":
		return "left center"
	case "ctr":
		return "center center"
	case "r":
		return "right center"
	case "bl":
		return "left bottom"
	case "b":
		return "center bottom"
	case "br":
		return "right bottom"
	default:
		return ""
	}
}

func styleDeclsContainProperty(styleAttrs []string, property string) bool {
	prefix := strings.ToLower(strings.TrimSpace(property)) + ":"
	for _, attr := range styleAttrs {
		for _, decl := range strings.Split(attr, ";") {
			if strings.HasPrefix(strings.ToLower(strings.TrimSpace(decl)), prefix) {
				return true
			}
		}
	}
	return false
}

func (r *htmlRenderer) renderTextBox(tb *resolve.TextBox) {
	r.renderTextBoxTo(&r.buf, tb)
}

// renderTextBoxTo writes a text box to buf. Floating text boxes use
// layout-computed page-absolute coordinates.
func (r *htmlRenderer) renderTextBoxTo(buf *bytes.Buffer, tb *resolve.TextBox) {
	if tb == nil {
		return
	}
	r.withOutputBuffer(buf, func() {
		if tb.Floating {
			var styles []string
			widthPt := emuToPoints(tb.Width)
			heightPt := emuToPoints(tb.Height)
			if tb.SimplePosAttr {
				styles = append(styles, fmt.Sprintf("left:%.1fpt", emuToPoints(tb.SimplePosX)))
				styles = append(styles, fmt.Sprintf("top:%.1fpt", emuToPoints(tb.SimplePosY)))
			} else if fb, ok := r.floatByTextBox[tb]; ok {
				styles = append(styles, fmt.Sprintf("left:%.1fpt", fb.X))
				styles = append(styles, fmt.Sprintf("top:%.1fpt", fb.Y))
				widthPt = boxWidthPt(fb.Width, widthPt)
				heightPt = boxHeightPt(fb.Height, heightPt)
			} else {
				styles = append(styles, fmt.Sprintf("left:%.1fpt", r.resolveFloatLeft(tb.X, tb.XAlign, tb.XRelative, tb.Width, false)))
				styles = append(styles, fmt.Sprintf("top:%.1fpt", r.resolveFloatTop(tb.Y, tb.YAlign, tb.YRelative, tb.Height, false)))
			}
			styles = r.appendTextBoxBoxStyles(styles, tb, widthPt, heightPt)
			// Note: DistTop/DistRight/DistBottom/DistLeft are text-wrapping
			// clearances for surrounding flow text — they are NOT positional
			// offsets and must not be applied as CSS margins on absolutely-
			// positioned overlay elements (left/top already encode the exact
			// page-absolute position).
			r.writeTextBoxElement(tb, styles, widthPt, heightPt)
			return
		}
		r.withLineNumberingSuppressed(func() {
			r.renderBlocks(tb.Blocks)
		})
	})
}

// renderFrameParaTo renders a w:framePr frame paragraph to buf as an
// absolutely positioned div using layout-computed coordinates.
func (r *htmlRenderer) renderFrameParaTo(buf *bytes.Buffer, p *resolve.Paragraph) {
	if p == nil {
		return
	}
	r.withOutputBuffer(buf, func() {
		var styles []string
		styles = append(styles, "position:absolute")
		if p.Frame != nil && p.Frame.TextDirection != "" {
			styles = append(styles, wordTableCellTextDirectionCSS(p.Frame.TextDirection)...)
		}
		if fb, ok := r.floatByFramePara[p]; ok {
			styles = append(styles, fmt.Sprintf("left:%.1fpt", fb.X))
			styles = append(styles, fmt.Sprintf("top:%.1fpt", fb.Y))
			if fb.Width > 0 {
				styles = append(styles, fmt.Sprintf("width:%.1fpt", fb.Width))
			}
			if fb.Height > 0 {
				styles = append(styles, fmt.Sprintf("height:%.1fpt", fb.Height))
			}
		}
		fmt.Fprintf(&r.buf, `<div class="frame-paragraph" style=%q>`, strings.Join(styles, ";"))
		r.withLineNumberingSuppressed(func() {
			r.renderParagraph(p, false, false)
		})
		r.buf.WriteString("</div>")
	})
}

func (r *htmlRenderer) renderDropCapFramePara(p *resolve.Paragraph) {
	if p == nil {
		return
	}
	var styles []string
	// A drop cap's float box is its wrap region, not a clipping window: the
	// glyph's ink can extend past its advance width, so the frame must not
	// hide the overhang.
	styles = append(styles, "float:left", "overflow:visible")
	if fb, ok := r.floatByFramePara[p]; ok {
		if fb.Width > 0 {
			styles = append(styles, fmt.Sprintf("width:%.1fpt", fb.Width))
		}
		if fb.Height > 0 {
			styles = append(styles, fmt.Sprintf("min-height:%.1fpt", fb.Height))
		}
	}
	if p.Frame != nil {
		if p.Frame.TextDirection != "" {
			styles = append(styles, wordTableCellTextDirectionCSS(p.Frame.TextDirection)...)
		}
		if gapPt := dropCapGapRightPt(p.Frame); gapPt > 0 {
			styles = append(styles, fmt.Sprintf("margin-right:%.1fpt", gapPt))
		}
		if p.Frame.VSpace > 0 {
			styles = append(styles, fmt.Sprintf("margin-bottom:%.1fpt", float64(p.Frame.VSpace)/20.0))
		}
	}
	fmt.Fprintf(&r.buf, `<div class="frame-paragraph" style=%q>`, strings.Join(styles, ";"))
	r.withLineNumberingSuppressed(func() {
		r.renderParagraphSegment(p, p.Runs, true, false, false, false, true, false)
	})
	r.buf.WriteString("</div>")
}

func isDropCapFrameParagraph(p *resolve.Paragraph) bool {
	return p != nil && p.Frame != nil && p.Frame.DropCap != "" && p.Frame.DropCap != "none"
}

// renderFrameParaInline renders a frame paragraph as an inline div with
// relative positioning for nested contexts (table cells, text boxes, etc.).
// Unlike overlay frames that use absolute positioning, inline frames use
// margins to approximate the frame offset from the paragraph's natural
// position in the flow.
func (r *htmlRenderer) renderFrameParaInline(p *resolve.Paragraph) {
	if p == nil || p.Frame == nil {
		return
	}
	var styles []string
	fp := p.Frame

	// Width
	if fp.W > 0 {
		styles = append(styles, fmt.Sprintf("width:%.1fpt", twipsToPoints(fp.W)))
	} else {
		styles = append(styles, "width:auto")
	}

	// Height
	if fp.H > 0 {
		styles = append(styles, fmt.Sprintf("height:%.1fpt", twipsToPoints(fp.H)))
		styles = append(styles, "overflow:hidden")
	}

	// Horizontal offset from normal flow (mimics the frame's X offset)
	if fp.X > 0 {
		styles = append(styles, fmt.Sprintf("margin-left:%.1fpt", twipsToPoints(fp.X)))
	} else if fp.XAlign == "right" {
		styles = append(styles, "margin-left:auto;margin-right:0")
	} else if fp.XAlign == "center" {
		styles = append(styles, "margin-left:auto;margin-right:auto")
	}

	// Horizontal spacing (w:hSpace → margin-right)
	if fp.HSpace > 0 {
		styles = append(styles, fmt.Sprintf("margin-right:%.1fpt", twipsToPoints(fp.HSpace)))
	}
	// Vertical spacing (w:vSpace → margin-bottom)
	if fp.VSpace > 0 {
		styles = append(styles, fmt.Sprintf("margin-bottom:%.1fpt", twipsToPoints(fp.VSpace)))
	}

	// Text direction
	if fp.TextDirection != "" {
		styles = append(styles, wordTableCellTextDirectionCSS(fp.TextDirection)...)
	}

	r.closeAllLists()
	fmt.Fprintf(&r.buf, `<div class="frame-paragraph frame-inline" style=%q>`, strings.Join(styles, ";"))
	r.withLineNumberingSuppressed(func() {
		r.renderParagraph(p, false, false)
	})
	r.buf.WriteString("</div>\n")
}

func dropCapGapRightPt(fp *resolve.FrameProps) float64 {
	if fp == nil {
		return 0
	}
	if fp.HSpace > 0 {
		return float64(fp.HSpace) / 20.0
	}
	return 6.0
}

func paragraphContainsOnlyFloatingImage(p *resolve.Paragraph, target *resolve.Image) bool {
	if p == nil || target == nil {
		return false
	}
	found := false
	for _, run := range p.Runs {
		switch {
		case run.Kind == "image" && run.Image == target:
			found = true
		case run.Kind == "image" && run.Image != nil:
			return false
		case strings.TrimSpace(run.Text) != "":
			return false
		case run.Kind == "textbox" || run.Kind == "shape" || run.Kind == "hyperlink":
			return false
		}
	}
	return found
}

func paragraphBlockFloatImageEligible(p *resolve.Paragraph, target *resolve.Image) bool {
	if paragraphContainsOnlyFloatingImage(p, target) {
		return true
	}
	if p == nil || target == nil {
		return false
	}
	targetIndex := -1
	for i, run := range p.Runs {
		if run.Kind == "image" && run.Image == target {
			targetIndex = i
			break
		}
	}
	if targetIndex < 0 {
		return false
	}
	for i := 0; i < targetIndex; i++ {
		run := p.Runs[i]
		switch {
		case strings.TrimSpace(run.Text) == "":
			continue
		case run.Kind == "image" && run.Image != nil && run.Image.Floating:
			continue
		default:
			return false
		}
	}
	return true
}

func paragraphContainsOnlyFloatingTextBox(p *resolve.Paragraph, target *resolve.TextBox) bool {
	if p == nil || target == nil {
		return false
	}
	found := false
	for _, run := range p.Runs {
		switch {
		case run.Kind == "textbox" && run.TextBox == target:
			found = true
		case run.Kind == "textbox" && run.TextBox != nil:
			return false
		case strings.TrimSpace(run.Text) != "":
			return false
		case run.Kind == "shape" || run.Kind == "hyperlink":
			return false
		}
	}
	return found
}

func paragraphBlockFloatTextBoxEligible(p *resolve.Paragraph, target *resolve.TextBox) bool {
	if paragraphContainsOnlyFloatingTextBox(p, target) {
		return true
	}
	if p == nil || target == nil {
		return false
	}
	targetIndex := -1
	for i, run := range p.Runs {
		if run.Kind == "textbox" && run.TextBox == target {
			targetIndex = i
			break
		}
	}
	if targetIndex < 0 {
		return false
	}
	for i := 0; i < targetIndex; i++ {
		run := p.Runs[i]
		switch {
		case strings.TrimSpace(run.Text) == "":
			continue
		case run.Kind == "image" && run.Image != nil && run.Image.Floating:
			continue
		case run.Kind == "textbox" && run.TextBox != nil && run.TextBox.Floating:
			continue
		case run.Kind == "shape" && run.Shape != nil && run.Shape.Floating:
			continue
		default:
			return false
		}
	}
	return true
}

func paragraphContainsOnlyFloatingShape(p *resolve.Paragraph, target *resolve.Shape) bool {
	if p == nil || target == nil {
		return false
	}
	found := false
	for _, run := range p.Runs {
		switch {
		case run.Kind == "shape" && run.Shape == target:
			found = true
		case run.Kind == "shape" && run.Shape != nil:
			return false
		case strings.TrimSpace(run.Text) != "":
			return false
		case run.Kind == "textbox" || run.Kind == "image" || run.Kind == "hyperlink":
			return false
		}
	}
	return found
}

func paragraphBlockFloatShapeEligible(p *resolve.Paragraph, target *resolve.Shape) bool {
	if paragraphContainsOnlyFloatingShape(p, target) {
		return true
	}
	if p == nil || target == nil {
		return false
	}
	targetIndex := -1
	for i, run := range p.Runs {
		if run.Kind == "shape" && run.Shape == target {
			targetIndex = i
			break
		}
	}
	if targetIndex < 0 {
		return false
	}
	for i := 0; i < targetIndex; i++ {
		run := p.Runs[i]
		switch {
		case strings.TrimSpace(run.Text) == "":
			continue
		case run.Kind == "shape" && run.Shape != nil && run.Shape.Floating:
			continue
		default:
			return false
		}
	}
	return true
}

// renderFloatTableToOverlay renders a page- or margin-anchored floating table
// to the page overlay using layout-computed absolute coordinates.
// Returns true if the table was rendered to the overlay.
func (r *htmlRenderer) renderFloatTableToOverlay(t *resolve.Table) bool {
	if t == nil || t.Format.FloatProps == nil {
		return false
	}
	fp := t.Format.FloatProps
	// Only use overlay positioning for page- or margin-anchored tables
	// where layout-computed coordinates are available.
	// OOXML defaults both anchors to "text" when absent.
	horz := strings.ToLower(strings.TrimSpace(fp.HorzAnchor))
	if horz == "" {
		horz = "text"
	}
	vert := strings.ToLower(strings.TrimSpace(fp.VertAnchor))
	if vert == "" {
		vert = "text"
	}
	if horz == "text" && vert == "text" {
		return false
	}
	fb, ok := r.floatByTable[t]
	if !ok {
		return false
	}
	r.withOutputBuffer(&r.overlay, func() {
		var styles []string
		styles = append(styles, "position:absolute")
		styles = append(styles, fmt.Sprintf("left:%.1fpt", fb.X))
		styles = append(styles, fmt.Sprintf("top:%.1fpt", fb.Y))
		if fb.Width > 0 {
			styles = append(styles, fmt.Sprintf("width:%.1fpt", fb.Width))
		}
		if fb.Height > 0 {
			styles = append(styles, fmt.Sprintf("height:%.1fpt", fb.Height))
		}
		fmt.Fprintf(&r.buf, `<div class="float-table" style=%q>`, strings.Join(styles, ";"))
		r.renderTable(t)
		r.buf.WriteString("</div>")
	})
	return true
}

// renderInlineFloatTable renders a w:tblpPr floating table inline using CSS
// float so that surrounding body text wraps around it naturally.
func (r *htmlRenderer) renderInlineFloatTable(t *resolve.Table) {
	if t == nil {
		return
	}
	fp := t.Format.FloatProps

	var styles []string
	floatDir := floatTableCSSDir(fp)
	if floatDir != "" {
		styles = append(styles, "float:"+floatDir)
	}
	if t.Format.Overlap == "never" {
		styles = append(styles, "clear:both")
	}
	availableWidthPt := r.pageWidthPt - r.marginLeftPt - r.marginRightPt
	widthPt := layout.TableWidthPt(t, availableWidthPt)
	if widthPt > 0 {
		styles = append(styles, fmt.Sprintf("width:%.1fpt", widthPt))
	}
	if fp != nil {
		if floatDir == "left" && fp.RightFromText > 0 {
			styles = append(styles, fmt.Sprintf("margin-right:%.1fpt", float64(fp.RightFromText)/20.0))
		} else if floatDir == "right" && fp.LeftFromText > 0 {
			styles = append(styles, fmt.Sprintf("margin-left:%.1fpt", float64(fp.LeftFromText)/20.0))
		}
		if fp.TopFromText > 0 {
			styles = append(styles, fmt.Sprintf("margin-top:%.1fpt", float64(fp.TopFromText)/20.0))
		}
		if fp.BottomFromText > 0 {
			styles = append(styles, fmt.Sprintf("margin-bottom:%.1fpt", float64(fp.BottomFromText)/20.0))
		}
	}
	fmt.Fprintf(&r.buf, `<div class="float-table" style=%q>`, strings.Join(styles, ";"))
	r.renderTable(t)
	r.buf.WriteString("</div>")
}

// floatTableCSSDir returns "left" or "right" for the CSS float property
// based on w:tblpPr horizontal anchor/alignment.
func floatTableCSSDir(fp *resolve.TableFloatProps) string {
	if fp == nil {
		return "left"
	}
	switch strings.ToLower(fp.XAlign) {
	case "right", "outside":
		return "right"
	case "left", "inside", "":
		// default: left
	}
	// No explicit alignment: infer from anchor
	// horzAnchor="text" with no X offset → float left
	return "left"
}

func (r *htmlRenderer) renderSignatureLine(sl *resolve.SignatureLine) {
	if sl == nil {
		return
	}
	r.buf.WriteString(`<div style="border:1px solid #000;padding:8px;max-width:300px;font-family:sans-serif">`)
	r.buf.WriteString(`<div style="border-bottom:1px solid #000;padding-bottom:4px;margin-bottom:4px">`)
	r.buf.WriteString(`<strong>X</strong> `)
	if sl.SuggestedSigner != "" {
		r.buf.WriteString(html.EscapeString(sl.SuggestedSigner))
	} else {
		r.buf.WriteString("Signature")
	}
	r.buf.WriteString(`</div>`)
	if sl.SuggestedSigner2 != "" {
		r.buf.WriteString(`<div style="font-size:0.9em;color:#555">`)
		r.buf.WriteString(html.EscapeString(sl.SuggestedSigner2))
		r.buf.WriteString(`</div>`)
	}
	if sl.SuggestedSignerEmail != "" {
		r.buf.WriteString(`<div style="font-size:0.85em;color:#777">`)
		r.buf.WriteString(html.EscapeString(sl.SuggestedSignerEmail))
		r.buf.WriteString(`</div>`)
	}
	if sl.SigningInstructions != "" {
		r.buf.WriteString(`<div style="font-size:0.8em;color:#999;margin-top:4px">`)
		r.buf.WriteString(html.EscapeString(sl.SigningInstructions))
		r.buf.WriteString(`</div>`)
	}
	r.buf.WriteString(`</div>`)
}

func (r *htmlRenderer) renderShape(shape *resolve.Shape) {
	r.renderShapeTo(&r.buf, shape)
}

func (r *htmlRenderer) appendShapeEffectTransformStyles(styles []string, shape *resolve.Shape, includeReflection bool) []string {
	return appendEffectTransformStyles(styles, boxTransformCSS(shape.RotationDeg, shape.FlipH, shape.FlipV), shapeEffectStyleProps(shape, includeReflection))
}

func appendEffectTransformStyles(styles []string, baseTransform string, effectStyles map[string]string) []string {
	if len(effectStyles) > 0 {
		styles = append(styles, "overflow:visible")
		transform, origin := mergeBoxTransformWithEffects(baseTransform, effectStyles)
		if transform != "" {
			styles = append(styles, "transform:"+transform)
			if origin != "" {
				styles = append(styles, "transform-origin:"+origin)
			} else {
				styles = append(styles, "transform-origin:50% 50%")
			}
		} else if origin != "" {
			styles = append(styles, "transform-origin:"+origin)
		}
		styles = appendSortedStyleProps(styles, effectStyles)
		return styles
	}
	if transform := baseTransform; transform != "" {
		styles = append(styles, "transform:"+transform, "transform-origin:50% 50%")
	}
	return styles
}

func threeDCameraTransformCSS(td *dml.ThreeD) string {
	if td == nil {
		return ""
	}
	preset := strings.ToLower(strings.TrimSpace(td.CameraPreset))
	perspective := 960.0
	rx, ry, rz := 0.0, 0.0, 0.0
	switch {
	case strings.Contains(preset, "perspective"):
		perspective = 760
	case strings.Contains(preset, "orthographic"):
		perspective = 2400
	}
	if strings.Contains(preset, "top") {
		rx = 18
	}
	if strings.Contains(preset, "bottom") {
		rx = -18
	}
	if strings.Contains(preset, "left") {
		ry = -18
	}
	if strings.Contains(preset, "right") {
		ry = 18
	}
	if strings.Contains(preset, "above") {
		rx = 12
	}
	if strings.Contains(preset, "below") {
		rx = -12
	}
	if strings.Contains(preset, "oblique") {
		rz = 4
	}
	parts := []string{fmt.Sprintf("perspective(%.0fpx)", perspective)}
	if rx != 0 {
		parts = append(parts, fmt.Sprintf("rotateX(%.2fdeg)", rx))
	}
	if ry != 0 {
		parts = append(parts, fmt.Sprintf("rotateY(%.2fdeg)", ry))
	}
	if rz != 0 {
		parts = append(parts, fmt.Sprintf("rotateZ(%.2fdeg)", rz))
	}
	return strings.Join(parts, " ")
}

// threeDExtrusionProjection converts DrawingML scene metadata into a portable
// 2.5D SVG projection. It deliberately keeps the front face as the authored
// vector geometry while drawing translated silhouettes behind it.
func threeDExtrusionProjection(td *dml.ThreeD) (steps int, dx, dy float64) {
	if td == nil || td.ExtrusionDepth <= 0 {
		return 0, 0, 0
	}
	depth := td.ExtrusionDepth
	steps = int(math.Ceil(depth / 1.5))
	if steps < 1 {
		steps = 1
	}
	if steps > 24 {
		steps = 24
	}
	angle := 45.0
	switch strings.ToLower(strings.TrimSpace(td.LightDirection)) {
	case "tl":
		angle = 45
	case "t":
		angle = 90
	case "tr":
		angle = 135
	case "l":
		angle = 0
	case "r":
		angle = 180
	case "bl":
		angle = -45
	case "b":
		angle = -90
	case "br":
		angle = -135
	}
	rad := angle * math.Pi / 180
	dx = -depth * math.Cos(rad) / float64(steps)
	dy = depth * math.Sin(rad) / float64(steps)
	return steps, dx, dy
}

func threeDExtrusionPrimitive(p dml.VectorPrimitive, td *dml.ThreeD) dml.VectorPrimitive {
	color := strings.TrimSpace(td.ExtrusionColor)
	if color == "" {
		color = "595959"
	}
	p.Style.Fill = &dml.SolidFill{Color: color, Alpha: 0.82}
	p.Style.FillColor = color
	p.Style.Stroke = nil
	p.Style.StrokeColor = ""
	p.Style.StrokeWidth = 0
	return p
}

func writeThreeDExtrusionLayers(b *strings.Builder, primitives []dml.VectorPrimitive, td *dml.ThreeD) bool {
	steps, dx, dy := threeDExtrusionProjection(td)
	if steps == 0 {
		return false
	}
	b.WriteString(`<g class="shape__3d-extrusion" aria-hidden="true">`)
	for step := steps; step >= 1; step-- {
		opacity := 0.40 + 0.45*float64(step)/float64(steps)
		fmt.Fprintf(b, `<g transform="translate(%.4f %.4f)" opacity="%.3f">`, dx*float64(step), dy*float64(step), opacity)
		for _, primitive := range primitives {
			writeSVGPrimitive(b, threeDExtrusionPrimitive(primitive, td), "")
		}
		b.WriteString(`</g>`)
	}
	b.WriteString(`</g>`)
	return true
}

func threeDBevelOverlay(td *dml.ThreeD, width, height float64) string {
	if td == nil || (td.BevelTopWidth <= 0 && td.BevelTopHeight <= 0 && td.ContourWidth <= 0) {
		return ""
	}
	inset := math.Max(td.BevelTopWidth, td.BevelTopHeight)
	if inset <= 0 {
		inset = td.ContourWidth
	}
	if inset <= 0 {
		return ""
	}
	if inset > math.Min(width, height)/3 {
		inset = math.Min(width, height) / 3
	}
	return fmt.Sprintf(`<g class="shape__3d-bevel" aria-hidden="true"><rect x="%.3f" y="%.3f" width="%.3f" height="%.3f" rx="%.3f" fill="none" stroke="rgba(255,255,255,0.42)" stroke-width="%.3f"/><rect x="%.3f" y="%.3f" width="%.3f" height="%.3f" rx="%.3f" fill="none" stroke="rgba(0,0,0,0.18)" stroke-width="%.3f"/></g>`, inset/2, inset/2, math.Max(0, width-inset), math.Max(0, height-inset), inset/2, math.Max(.5, inset/2), inset, inset, math.Max(0, width-2*inset), math.Max(0, height-2*inset), inset/2, math.Max(.5, inset/3))
}

func (r *htmlRenderer) renderVectorShapeWithStyleAttrs(buf *bytes.Buffer, shape *resolve.Shape, styleAttrs []string, width, height float64) bool {
	if shape == nil {
		return false
	}

	requiresVector := shape.GeometryData != nil || shape.Geometry != nil
	renderFill := r.renderableVectorFill(shape.Fill)
	requiresVectorFill := drawingcss.NeedsSVGDefinition(renderFill)
	if !requiresVector && !requiresVectorFill {
		return false
	}
	if width <= 0 || height <= 0 {
		return false
	}

	preset := "rect"
	if shape.Geometry != nil && shape.Geometry.Name != "" {
		preset = shape.Geometry.Name
	}

	strokeWidthPt := 0.0
	if shape.StrokeWidth > 0 {
		strokeWidthPt = emuToPoints(shape.StrokeWidth)
	}

	primitives, err := dml.BuildShapePrimitives(dml.ShapeGeometrySpec{
		Geometry:    shape.GeometryData,
		Preset:      &dml.PresetGeometry{Name: preset},
		Width:       width,
		Height:      height,
		Fill:        renderFill,
		FillColor:   shape.FillColor,
		Stroke:      shape.LineStyle,
		StrokeColor: shape.StrokeColor,
		StrokeWidth: strokeWidthPt,
	})
	if err != nil {
		return false
	}
	if len(primitives) == 0 {
		return false
	}

	fillDefID := ""
	fillDef := ""
	if drawingcss.NeedsSVGDefinition(renderFill) {
		fillDefID = "shape-fill"
		fillDef = drawingcss.SVGDefinition(renderFill, fillDefID)
	}
	plan := buildShapeEffectRenderPlan(shape)
	threeD := extractShapeThreeD(shape)
	threeDLightOverlays := drawingcss.ThreeDLightOverlays(threeD)
	originalOverlayInput := append([]dml.FillOverlay{}, plan.originalOverlays()...)
	originalOverlayInput = append(originalOverlayInput, threeDLightOverlays...)
	reflectionOverlayInput := append([]dml.FillOverlay{}, plan.reflectionOverlays()...)
	reflectionOverlayInput = append(reflectionOverlayInput, threeDLightOverlays...)
	originalOverlays := r.buildShapeFillOverlayRenders(originalOverlayInput, "shape-fill-overlay")
	reflectionOverlays := r.buildShapeFillOverlayRenders(reflectionOverlayInput, "shape-reflection-overlay")
	originalEffectStyles := effectStylePropsFromEffects(plan.OriginalEffects)
	reflectionEffectStyles := effectStylePropsFromEffects(plan.ReflectionEffects)
	// Position/size/geometry transform belong to one common host. The SVG and
	// textbox are children of that host so they cannot drift apart.
	hostStyles := append([]string{}, styleAttrs...)
	hasPosition := false
	for _, style := range hostStyles {
		if strings.HasPrefix(style, "position:") {
			hasPosition = true
			break
		}
	}
	if !hasPosition {
		hostStyles = append(hostStyles, "position:relative")
	}
	if width > 0 {
		hostStyles = append(hostStyles, fmt.Sprintf("width:%.4fpt", width))
	}
	if height > 0 {
		hostStyles = append(hostStyles, fmt.Sprintf("height:%.4fpt", height))
	}
	baseTransform := boxTransformCSS(shape.RotationDeg, shape.FlipH, shape.FlipV)
	cameraTransform := threeDCameraTransformCSS(threeD)
	combinedTransform := strings.TrimSpace(strings.TrimSpace(baseTransform + " " + cameraTransform))
	if combinedTransform != "" {
		hostStyles = append(hostStyles, "transform:"+combinedTransform, "transform-origin:50% 50%", "transform-style:preserve-3d")
	}
	if len(originalEffectStyles) > 0 || len(reflectionEffectStyles) > 0 || plan.Reflection != nil || len(originalOverlays) > 0 || len(reflectionOverlays) > 0 {
		hostStyles = append(hostStyles, "overflow:visible")
	}

	svgStyles := []string{"position:absolute", "inset:0", "display:block"}
	originalContentStyles := appendEffectTransformStyles(nil, "", originalEffectStyles)
	reflectionContentStyles := appendEffectTransformStyles(nil, "", reflectionEffectStyles)

	var svgBuf strings.Builder
	fmt.Fprintf(&svgBuf, `<div class="shape-vector-host" style=%q>`, strings.Join(hostStyles, ";"))
	fmt.Fprintf(&svgBuf, `<svg class="shape__vector" viewBox="0 0 %.4f %.4f" width="%.4fpt" height="%.4fpt" preserveAspectRatio="none" aria-hidden="true"`, width, height, width, height)
	if threeD != nil {
		fmt.Fprintf(&svgBuf, ` data-3d-camera=%q data-3d-light=%q data-3d-material=%q`, threeD.CameraPreset, threeD.LightRig, threeD.Material)
	}
	if len(svgStyles) > 0 {
		fmt.Fprintf(&svgBuf, ` style=%q`, strings.Join(svgStyles, ";"))
	}
	fmt.Fprintf(&svgBuf, ` xmlns="http://www.w3.org/2000/svg">`)
	// Check for SVG-renderable reflection effect.
	refl := plan.Reflection
	filterDef := ""
	maskDef := ""
	if refl != nil {
		filterDef = buildSVGReflectionFilter(refl)
		maskDef = buildSVGReflectionMask(refl)
	}
	if fillDef != "" || filterDef != "" || maskDef != "" || len(originalOverlays) > 0 || len(reflectionOverlays) > 0 {
		svgBuf.WriteString(`<defs>`)
		if fillDef != "" {
			svgBuf.WriteString(fillDef)
		}
		for _, overlay := range originalOverlays {
			if overlay.Def != "" {
				svgBuf.WriteString(overlay.Def)
			}
		}
		for _, overlay := range reflectionOverlays {
			if overlay.Def != "" {
				svgBuf.WriteString(overlay.Def)
			}
		}
		if filterDef != "" {
			svgBuf.WriteString(filterDef)
		}
		if maskDef != "" {
			svgBuf.WriteString(maskDef)
		}
		svgBuf.WriteString(`</defs>`)
	}
	if threeD != nil {
		writeThreeDExtrusionLayers(&svgBuf, primitives, threeD)
	}
	svgBuf.WriteString(`<g class="shape__vector-content"`)
	if len(originalContentStyles) > 0 {
		fmt.Fprintf(&svgBuf, ` style=%q`, strings.Join(originalContentStyles, ";"))
	}
	svgBuf.WriteString(`>`)
	writeShapeSVGPrimitives(&svgBuf, primitives, fillDefID, originalOverlays)
	svgBuf.WriteString(`</g>`)
	if bevel := threeDBevelOverlay(threeD, width, height); bevel != "" {
		svgBuf.WriteString(bevel)
	}
	if refl != nil {
		alpha := refl.Alpha
		if alpha <= 0 {
			alpha = 0.3
		}
		alpha = mergeEffectOpacityIntoReflection(alpha, reflectionEffectStyles)
		reflectionContentStyles = appendEffectTransformStyles(nil, "", reflectionEffectStyles)
		scaleY := refl.Size
		if scaleY <= 0 || scaleY > 1 {
			scaleY = 1
		}
		rad := refl.Direction * math.Pi / 180
		offX := refl.Distance * math.Cos(rad)
		offY := refl.Distance * math.Sin(rad)
		translateY := height + offY + scaleY*height
		// Render reflected copy with vertical flip plus size/distance offset.
		fmt.Fprintf(&svgBuf, `<g transform="translate(%.4f, %.4f) scale(1, -%.4f)" opacity="%.2f"`, offX, translateY, scaleY, alpha)
		if filterDef != "" {
			svgBuf.WriteString(` filter="url(#word-reflection-filter)"`)
		}
		if maskDef != "" {
			svgBuf.WriteString(` mask="url(#word-reflection-mask)"`)
		}
		svgBuf.WriteString(`>`)
		svgBuf.WriteString(`<g class="shape__vector-reflection-content"`)
		if len(reflectionContentStyles) > 0 {
			fmt.Fprintf(&svgBuf, ` style=%q`, strings.Join(reflectionContentStyles, ";"))
		}
		svgBuf.WriteString(`>`)
		writeShapeSVGPrimitives(&svgBuf, primitives, fillDefID, reflectionOverlays)
		svgBuf.WriteString(`</g>`)
		svgBuf.WriteString(`</g>`)
	}
	fmt.Fprintf(&svgBuf, `</svg>`)
	buf.WriteString(svgBuf.String())

	// Text is deliberately emitted before closing shape-vector-host. Absolute
	// inset:0 is therefore resolved against the actual shape rectangle, and the
	// host's rotation/flip applies to both the rectangle and its text.
	r.withOutputBuffer(buf, func() {
		r.renderShapeText(shape, width, height)
	})
	buf.WriteString(`</div>`)
	return true
}

// parseRefDelimiter extracts the \d switch value from a REF instruction.
// Returns "" when absent.
func parseRefDelimiter(instr string) string {
	upper := strings.ToUpper(instr)
	idx := strings.Index(upper, `\D`)
	if idx < 0 {
		return ""
	}
	value, ok := extractFieldSwitchValue(instr[idx+len(`\D`):])
	if !ok {
		return ""
	}
	return strings.Trim(value, `"`)
}

// replaceNumberDelimiters replaces common delimiters in a paragraph number
// with the specified separator.
func replaceNumberDelimiters(num, sep string) string {
	result := num
	for _, d := range []string{".", "-", ")", "("} {
		result = strings.ReplaceAll(result, d, sep)
	}
	return result
}

// stripNonNumberText removes alphabetic prefixes and suffixes from a paragraph
// number string, keeping digits and common delimiters. Used by REF \t.
func stripNonNumberText(s string) string {
	isNumOrDelim := func(r byte) bool {
		return (r >= '0' && r <= '9') || r == '.' || r == '-' || r == ')' || r == '('
	}
	// Trim leading non-digit/delim text.
	start := 0
	for start < len(s) && !isNumOrDelim(s[start]) {
		start++
	}
	// Trim trailing non-digit/delim text.
	end := len(s) - 1
	for end >= start && !isNumOrDelim(s[end]) {
		end--
	}
	if start > end {
		return s
	}
	return s[start : end+1]
}

// parseRefNumberFormat extracts the \# switch value from a REF instruction.
func parseRefNumberFormat(instr string) string {
	upper := strings.ToUpper(instr)
	idx := strings.Index(upper, `\#`)
	if idx < 0 {
		return ""
	}
	rest := strings.TrimSpace(instr[idx+2:])
	if strings.HasPrefix(rest, `"`) {
		fmt, _ := extractFieldString(rest)
		return fmt
	}
	fields := strings.Fields(rest)
	if len(fields) > 0 && !strings.HasPrefix(fields[0], `\`) {
		return fields[0]
	}
	return ""
}

// parseFieldCaseFormat extracts the \* upper/lower/caps/firstcap switch value from
// any field instruction. Returns "" when absent.
func parseFieldCaseFormat(instr string) string {
	upper := strings.ToUpper(instr)
	idx := strings.Index(upper, `\*`)
	if idx < 0 {
		return ""
	}
	rest := strings.TrimSpace(instr[idx+2:])
	token, _ := extractFieldToken(rest)
	switch strings.ToLower(token) {
	case "upper", "lower", "caps", "firstcap":
		return token
	}
	return ""
}

func (r *htmlRenderer) seqControlForRun(run resolve.Run) seqControl {
	instr := strings.TrimSpace(run.FieldInstr)
	if len(run.FieldInstrRuns) > 0 {
		if expanded := strings.TrimSpace(r.resolveFieldInstructionRuns(run.FieldInstrRuns)); expanded != "" {
			instr = expanded
		}
	}
	return parseSeqControl(instr)
}

func (r *htmlRenderer) formatSequenceDisplayValue(n int, control seqControl, paragraph *resolve.Paragraph) string {
	if control.mergeFormat && control.format == "" && paragraph != nil && paragraph.Numbering != nil && paragraph.Numbering.Format != "" {
		return applyFieldCaseFormat(formatListNumber(n, paragraph.Numbering.Format), control.caseFormat)
	}
	return formatSequenceValue(n, control.format, control.caseFormat)
}

// formatRefNumber applies a Word number-format picture to a string value.
// Best-effort: supports 0, #, comma, decimal point, and currency prefixes.
func formatRefNumber(s, picture string) string {
	s = strings.TrimSpace(s)
	num, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return s
	}
	picture = strings.TrimSpace(strings.Trim(picture, `"`))
	if picture == "" {
		return s
	}

	// Locate the numeric pattern so authored currency/text prefixes and suffixes
	// survive formatting (for example "$#,##0.00" or "0.0 kg").
	isPatternByte := func(ch byte) bool {
		return ch == '0' || ch == '#' || ch == ',' || ch == '.'
	}
	first, last := -1, -1
	for i := 0; i < len(picture); i++ {
		if isPatternByte(picture[i]) {
			if first < 0 {
				first = i
			}
			last = i
		}
	}
	if first < 0 {
		return s
	}
	prefix := picture[:first]
	suffix := picture[last+1:]
	pattern := picture[first : last+1]

	intPattern := pattern
	fracPattern := ""
	if dot := strings.Index(pattern, "."); dot >= 0 {
		intPattern = pattern[:dot]
		fracPattern = pattern[dot+1:]
	}
	precision := 0
	for _, ch := range fracPattern {
		if ch == '0' || ch == '#' {
			precision++
		}
	}
	minIntDigits := strings.Count(intPattern, "0")
	groupThousands := strings.Contains(intPattern, ",")

	negative := num < 0
	if negative {
		num = -num
	}
	formatted := fmt.Sprintf("%.*f", precision, num)
	parts := strings.SplitN(formatted, ".", 2)
	integer := parts[0]
	if len(integer) < minIntDigits {
		integer = strings.Repeat("0", minIntDigits-len(integer)) + integer
	}
	if groupThousands {
		integer = groupDecimalThousands(integer)
	}
	formatted = integer
	if len(parts) == 2 {
		formatted += "." + parts[1]
	}
	if negative {
		formatted = "-" + formatted
	}
	return prefix + formatted + suffix
}

func groupDecimalThousands(integer string) string {
	if len(integer) <= 3 {
		return integer
	}
	first := len(integer) % 3
	if first == 0 {
		first = 3
	}
	var b strings.Builder
	b.WriteString(integer[:first])
	for i := first; i < len(integer); i += 3 {
		b.WriteByte(',')
		b.WriteString(integer[i : i+3])
	}
	return b.String()
}

// extractShapeReflection returns the first Reflection effect from a shape's
// effect list, or nil if none is present.
func extractShapeReflection(shape *resolve.Shape) *dml.Reflection {
	if shape == nil {
		return nil
	}
	for _, effect := range shape.Effects.Effects {
		if refl, ok := effect.(dml.Reflection); ok {
			return &refl
		}
		if refl, ok := effect.(*dml.Reflection); ok {
			return refl
		}
	}
	return nil
}

func extractShapeThreeD(shape *resolve.Shape) *dml.ThreeD {
	if shape == nil {
		return nil
	}
	for _, effect := range shape.Effects.Effects {
		if threeD, ok := effect.(dml.ThreeD); ok {
			return &threeD
		}
		if threeD, ok := effect.(*dml.ThreeD); ok {
			return threeD
		}
	}
	return nil
}

// buildSVGReflectionFilter builds an SVG filter definition for a reflection
// blur effect. Returns empty string when no blur is needed.
func buildSVGReflectionFilter(refl *dml.Reflection) string {
	if refl == nil || refl.Blur <= 0 {
		return ""
	}
	stdDev := refl.Blur / 2 // approximate conversion from radius to stdDeviation
	return fmt.Sprintf(`<filter id="word-reflection-filter" x="-50%%" y="-50%%" width="200%%" height="200%%"><feGaussianBlur stdDeviation="%.2f"/></filter>`, stdDev)
}

// buildSVGReflectionMask builds a simple fade mask for reflected vector shapes.
// It is an approximation of Word's reflection fade behavior.
func buildSVGReflectionMask(refl *dml.Reflection) string {
	if refl == nil || !refl.Fade {
		return ""
	}
	rad := refl.Direction * math.Pi / 180
	offX := math.Cos(rad)
	offY := math.Sin(rad)
	x1, y1, x2, y2 := "0%", "0%", "0%", "100%"
	if math.Abs(offX) > math.Abs(offY) {
		if offX >= 0 {
			x1, x2 = "0%", "100%"
		} else {
			x1, x2 = "100%", "0%"
		}
		y1, y2 = "0%", "0%"
	} else if offY < 0 {
		y1, y2 = "100%", "0%"
	}
	return fmt.Sprintf(`<mask id="word-reflection-mask"><linearGradient id="word-reflection-gradient" x1=%q y1=%q x2=%q y2=%q><stop offset="0%%" stop-color="white" stop-opacity="1"/><stop offset="100%%" stop-color="white" stop-opacity="0"/></linearGradient><rect x="0" y="0" width="100%%" height="100%%" fill="url(#word-reflection-gradient)"/></mask>`, x1, y1, x2, y2)
}

type shapeEffectRenderPlan struct {
	Reflection               *dml.Reflection
	OverlaysBeforeReflection []dml.FillOverlay
	OverlaysAfterReflection  []dml.FillOverlay
	OriginalEffects          []dml.Effect
	ReflectionEffects        []dml.Effect
}

func buildShapeEffectRenderPlan(shape *resolve.Shape) shapeEffectRenderPlan {
	var plan shapeEffectRenderPlan
	if shape == nil {
		return plan
	}
	sawReflection := false
	for _, effect := range shape.Effects.Effects {
		switch current := effect.(type) {
		case dml.Reflection:
			if !sawReflection {
				refl := current
				plan.Reflection = &refl
				sawReflection = true
			}
		case *dml.Reflection:
			if !sawReflection && current != nil {
				refl := *current
				plan.Reflection = &refl
				sawReflection = true
			}
		case dml.FillOverlay:
			if current.Fill == nil {
				continue
			}
			if sawReflection || strings.EqualFold(strings.TrimSpace(current.ContainerType), "sib") {
				plan.OverlaysAfterReflection = append(plan.OverlaysAfterReflection, current)
			} else {
				plan.OverlaysBeforeReflection = append(plan.OverlaysBeforeReflection, current)
			}
		case *dml.FillOverlay:
			if current == nil || current.Fill == nil {
				continue
			}
			if sawReflection || strings.EqualFold(strings.TrimSpace(current.ContainerType), "sib") {
				plan.OverlaysAfterReflection = append(plan.OverlaysAfterReflection, *current)
			} else {
				plan.OverlaysBeforeReflection = append(plan.OverlaysBeforeReflection, *current)
			}
		default:
			if isReflectionEffect(effect) {
				continue
			}
			plan.OriginalEffects = append(plan.OriginalEffects, effect)
			if !sawReflection && effectReflectsIntoReflection(effect) {
				plan.ReflectionEffects = append(plan.ReflectionEffects, effect)
			}
		}
	}
	return plan
}

func (p shapeEffectRenderPlan) reflectionOverlays() []dml.FillOverlay {
	if p.Reflection == nil {
		return nil
	}
	return p.OverlaysBeforeReflection
}

func (p shapeEffectRenderPlan) originalOverlays() []dml.FillOverlay {
	if len(p.OverlaysAfterReflection) == 0 {
		return p.OverlaysBeforeReflection
	}
	overlays := make([]dml.FillOverlay, 0, len(p.OverlaysBeforeReflection)+len(p.OverlaysAfterReflection))
	overlays = append(overlays, p.OverlaysBeforeReflection...)
	overlays = append(overlays, p.OverlaysAfterReflection...)
	return overlays
}

func isReflectionEffect(effect dml.Effect) bool {
	switch effect.(type) {
	case dml.Reflection, *dml.Reflection:
		return true
	default:
		return false
	}
}

func effectReflectsIntoReflection(effect dml.Effect) bool {
	return !strings.EqualFold(strings.TrimSpace(effectContainerType(effect)), "sib")
}

func effectContainerType(effect dml.Effect) string {
	switch current := effect.(type) {
	case dml.AlphaBiLevel:
		return current.ContainerType
	case *dml.AlphaBiLevel:
		if current != nil {
			return current.ContainerType
		}
	case dml.AlphaMod:
		return current.ContainerType
	case *dml.AlphaMod:
		if current != nil {
			return current.ContainerType
		}
	case dml.AlphaOff:
		return current.ContainerType
	case *dml.AlphaOff:
		if current != nil {
			return current.ContainerType
		}
	case dml.AlphaModFix:
		return current.ContainerType
	case *dml.AlphaModFix:
		if current != nil {
			return current.ContainerType
		}
	case dml.AlphaRepl:
		return current.ContainerType
	case *dml.AlphaRepl:
		if current != nil {
			return current.ContainerType
		}
	case dml.AlphaOutset:
		return current.ContainerType
	case *dml.AlphaOutset:
		if current != nil {
			return current.ContainerType
		}
	case dml.AlphaInv:
		return current.ContainerType
	case *dml.AlphaInv:
		if current != nil {
			return current.ContainerType
		}
	case dml.AlphaCeiling:
		return current.ContainerType
	case *dml.AlphaCeiling:
		if current != nil {
			return current.ContainerType
		}
	case dml.AlphaFloor:
		return current.ContainerType
	case *dml.AlphaFloor:
		if current != nil {
			return current.ContainerType
		}
	case dml.Shadow:
		return current.ContainerType
	case *dml.Shadow:
		if current != nil {
			return current.ContainerType
		}
	case dml.InnerShadow:
		return current.ContainerType
	case *dml.InnerShadow:
		if current != nil {
			return current.ContainerType
		}
	case dml.Glow:
		return current.ContainerType
	case *dml.Glow:
		if current != nil {
			return current.ContainerType
		}
	case dml.SoftEdge:
		return current.ContainerType
	case *dml.SoftEdge:
		if current != nil {
			return current.ContainerType
		}
	case dml.Blur:
		return current.ContainerType
	case *dml.Blur:
		if current != nil {
			return current.ContainerType
		}
	case dml.FillOverlay:
		return current.ContainerType
	case *dml.FillOverlay:
		if current != nil {
			return current.ContainerType
		}
	case dml.BlendMode:
		return current.ContainerType
	case *dml.BlendMode:
		if current != nil {
			return current.ContainerType
		}
	case dml.RelativeOffset:
		return current.ContainerType
	case *dml.RelativeOffset:
		if current != nil {
			return current.ContainerType
		}
	case dml.Tint:
		return current.ContainerType
	case *dml.Tint:
		if current != nil {
			return current.ContainerType
		}
	case dml.TransformEffect:
		return current.ContainerType
	case *dml.TransformEffect:
		if current != nil {
			return current.ContainerType
		}
	}
	return ""
}

func mergeEffectOpacityIntoReflection(alpha float64, effectStyles map[string]string) float64 {
	if len(effectStyles) == 0 {
		return alpha
	}
	rawOpacity := strings.TrimSpace(effectStyles["opacity"])
	if rawOpacity == "" {
		return alpha
	}
	effectAlpha, err := strconv.ParseFloat(rawOpacity, 64)
	if err != nil {
		return alpha
	}
	delete(effectStyles, "opacity")
	return alpha * effectAlpha
}

func writeShapeSVGPrimitives(b *strings.Builder, primitives []dml.VectorPrimitive, fillDefID string, overlays []drawingcss.SVGOverlay) {
	for _, primitive := range primitives {
		writeSVGPrimitive(b, primitive, fillDefID)
	}
	for _, overlay := range overlays {
		if overlay.Fill == nil {
			continue
		}
		blendStyle := drawingcss.SVGOverlayBlendStyle(overlay.BlendMode)
		if blendStyle != "" {
			fmt.Fprintf(b, `<g style=%q>`, blendStyle)
		} else {
			b.WriteString(`<g>`)
		}
		for _, primitive := range primitives {
			writeSVGPrimitive(b, overlayVectorPrimitiveFill(primitive, overlay.Fill), overlay.DefID)
		}
		b.WriteString(`</g>`)
	}
}

func overlayVectorPrimitiveFill(primitive dml.VectorPrimitive, fill dml.Fill) dml.VectorPrimitive {
	primitive.Style.Fill = fill
	primitive.Style.FillColor = ""
	primitive.Style.Stroke = nil
	primitive.Style.StrokeColor = ""
	primitive.Style.StrokeWidth = 0
	return primitive
}

func (r *htmlRenderer) buildShapeFillOverlayRenders(overlays []dml.FillOverlay, defPrefix string) []drawingcss.SVGOverlay {
	renders := make([]drawingcss.SVGOverlay, 0, len(overlays))
	for i, overlay := range overlays {
		renderOverlayFill := r.renderableVectorFill(overlay.Fill)
		if renderOverlayFill == nil {
			continue
		}
		rendered := drawingcss.SVGOverlay{
			Fill:      renderOverlayFill,
			BlendMode: overlay.BlendMode,
		}
		if drawingcss.NeedsSVGDefinition(renderOverlayFill) {
			rendered.DefID = fmt.Sprintf("%s-%d", defPrefix, i)
			rendered.Def = drawingcss.SVGDefinition(renderOverlayFill, rendered.DefID)
		}
		renders = append(renders, rendered)
	}
	return renders
}

func shapeFillOverlayBoxStyles(fill dml.Fill, blendMode string) []string {
	if fill == nil {
		return nil
	}
	props := drawingcss.Fill(fill)
	if len(props) == 0 {
		return nil
	}
	styles := []string{
		"position:absolute",
		"inset:0",
		"pointer-events:none",
	}
	if fillValue := strings.TrimSpace(props["fill"]); fillValue != "" && fillValue != "none" {
		if strings.HasPrefix(fillValue, "url(") {
			styles = append(styles, "background-image:"+fillValue)
			styles = append(styles, "background-repeat:no-repeat")
			styles = append(styles, "background-size:100% 100%")
		} else {
			styles = append(styles, "background:"+fillValue)
		}
	}
	if opacity := strings.TrimSpace(props["opacity"]); opacity != "" {
		styles = append(styles, "opacity:"+opacity)
	}
	if filter := strings.TrimSpace(props["filter"]); filter != "" {
		styles = append(styles, "filter:"+filter)
	}
	if blendStyle := drawingcss.SVGOverlayBlendStyle(blendMode); blendStyle != "" {
		styles = append(styles, blendStyle)
	}
	return styles
}

func (r *htmlRenderer) writeShapeFillOverlayDivs(overlays []dml.FillOverlay, className string) {
	for _, overlay := range overlays {
		renderFill := r.renderableVectorFill(overlay.Fill)
		overlayStyles := shapeFillOverlayBoxStyles(renderFill, overlay.BlendMode)
		if len(overlayStyles) == 0 {
			continue
		}
		fmt.Fprintf(&r.buf, `<div class=%q aria-hidden="true" style=%q></div>`, className, strings.Join(overlayStyles, ";"))
	}
}

func shapeReflectionMaskGradientCSS(refl *dml.Reflection) string {
	if refl == nil || !refl.Fade {
		return ""
	}
	rad := refl.Direction * math.Pi / 180
	offX := math.Cos(rad)
	offY := math.Sin(rad)
	direction := "to bottom"
	if math.Abs(offX) > math.Abs(offY) {
		if offX >= 0 {
			direction = "to right"
		} else {
			direction = "to left"
		}
	} else if offY < 0 {
		direction = "to top"
	}
	return fmt.Sprintf("linear-gradient(%s, rgba(255,255,255,1), rgba(255,255,255,0))", direction)
}

func shapeReflectionBoxStyles(shape *resolve.Shape, refl *dml.Reflection, widthPt, heightPt float64, effectStyles map[string]string) []string {
	if refl == nil || widthPt <= 0 || heightPt <= 0 {
		return nil
	}
	scaleY := refl.Size
	if scaleY <= 0 || scaleY > 1 {
		scaleY = 1
	}
	rad := refl.Direction * math.Pi / 180
	offX := refl.Distance * math.Cos(rad)
	offY := refl.Distance * math.Sin(rad)
	translateY := heightPt + offY + scaleY*heightPt
	alpha := refl.Alpha
	if alpha <= 0 {
		alpha = 0.3
	}
	alpha = mergeEffectOpacityIntoReflection(alpha, effectStyles)

	styles := []string{
		"position:absolute",
		"pointer-events:none",
		fmt.Sprintf("width:%.1fpt", widthPt),
		fmt.Sprintf("height:%.1fpt", heightPt),
		"left:0",
		"top:0",
		fmt.Sprintf("transform:translate(%.2fpt, %.2fpt) scaleY(-%.4f)", offX, translateY, scaleY),
		"transform-origin:top left",
		fmt.Sprintf("opacity:%.4f", alpha),
	}
	if shape.FillColor != "" {
		styles = append(styles, "background-color:#"+shape.FillColor)
	}
	if shape.StrokeColor != "" {
		w := "1pt"
		if shape.StrokeWidth > 0 {
			w = fmt.Sprintf("%.2fpt", emuToPoints(shape.StrokeWidth))
		}
		styles = append(styles, fmt.Sprintf("border:%s solid #%s", w, shape.StrokeColor))
	}
	if refl.Blur > 0 {
		styles = append(styles, fmt.Sprintf("filter:blur(%.1fpt)", refl.Blur))
	}
	if mask := shapeReflectionMaskGradientCSS(refl); mask != "" {
		styles = append(styles, "mask-image:"+mask)
		styles = append(styles, "-webkit-mask-image:"+mask)
	}
	return appendEffectTransformStyles(styles, "", effectStyles)
}

func shapeLineReflectionStyles(baseStyles []string, shape *resolve.Shape, refl *dml.Reflection, widthPt, heightPt float64, effectStyles map[string]string) []string {
	if refl == nil || shape == nil {
		return nil
	}
	styles := append([]string{}, baseStyles...)
	boxHeight := heightPt
	strokePt := 1.0
	if shape.StrokeWidth > 0 {
		strokePt = emuToPoints(shape.StrokeWidth)
	}
	if boxHeight <= 0 {
		boxHeight = strokePt
	}
	scaleY := refl.Size
	if scaleY <= 0 || scaleY > 1 {
		scaleY = 1
	}
	rad := refl.Direction * math.Pi / 180
	offX := refl.Distance * math.Cos(rad)
	offY := refl.Distance * math.Sin(rad)
	translateY := boxHeight + offY + scaleY*boxHeight
	alpha := refl.Alpha
	if alpha <= 0 {
		alpha = 0.3
	}
	alpha = mergeEffectOpacityIntoReflection(alpha, effectStyles)
	reflectionTransform := fmt.Sprintf("translate(%.2fpt, %.2fpt) scaleY(-%.4f)", offX, translateY, scaleY)
	if baseTransform := boxTransformCSS(shape.RotationDeg, shape.FlipH, shape.FlipV); baseTransform != "" {
		reflectionTransform = baseTransform + " " + reflectionTransform
	}
	styles = append(styles, "pointer-events:none")
	styles = append(styles, "overflow:visible")
	styles = append(styles, "transform:"+reflectionTransform)
	styles = append(styles, "transform-origin:top left")
	styles = append(styles, fmt.Sprintf("opacity:%.4f", alpha))
	if refl.Blur > 0 {
		styles = append(styles, fmt.Sprintf("filter:blur(%.1fpt)", refl.Blur))
	}
	if mask := shapeReflectionMaskGradientCSS(refl); mask != "" {
		styles = append(styles, "mask-image:"+mask)
		styles = append(styles, "-webkit-mask-image:"+mask)
	}
	return appendEffectTransformStyles(styles, "", effectStyles)
}

func (r *htmlRenderer) renderableVectorFill(fill dml.Fill) dml.Fill {
	switch f := fill.(type) {
	case dml.PictureFill:
		return r.renderablePictureFill(f)
	case *dml.PictureFill:
		if f == nil {
			return fill
		}
		pf := r.renderablePictureFill(*f)
		return &pf
	default:
		return fill
	}
}

func (r *htmlRenderer) renderablePictureFill(fill dml.PictureFill) dml.PictureFill {
	if r == nil || r.options.ImageMode != HTMLImageModeExternal || len(fill.ImageData) == 0 {
		return fill
	}
	if strings.TrimSpace(fill.ImageURL) != "" {
		return fill
	}
	assetPath := r.addImageAsset(&resolve.Image{
		MIMEType: fill.ContentType,
		Data:     fill.ImageData,
	})
	if assetPath == "" {
		return fill
	}
	fill.ImageURL = assetPath
	fill.ImageData = nil
	return fill
}

func writeSVGPrimitive(b *strings.Builder, primitive dml.VectorPrimitive, fillDefID string) {
	if b == nil {
		return
	}
	switch primitive.Kind {
	case dml.PrimitiveEllipse:
		if primitive.Ellipse == nil {
			return
		}
		fill := svgPrimitiveFillValue(primitive.Style, fillDefID)
		fmt.Fprintf(b, `<ellipse cx="%.4f" cy="%.4f" rx="%.4f" ry="%.4f" fill=%q`,
			primitive.Ellipse.Center.X, primitive.Ellipse.Center.Y, primitive.Ellipse.RX, primitive.Ellipse.RY, fill)
		if primitive.Style.StrokeColor != "" && primitive.Style.StrokeWidth > 0 {
			writeSVGStrokeAttrs(b, primitive.Style.Stroke, "#"+primitive.Style.StrokeColor, primitive.Style.StrokeWidth)
		}
		b.WriteString(`/>`)
	case dml.PrimitivePath:
		if primitive.Path == nil {
			return
		}
		fmt.Fprintf(b, `<path d=%q fill=%q`, svgPathData(*primitive.Path), svgPrimitiveFillValue(primitive.Style, fillDefID))
		if primitive.Style.StrokeColor != "" && primitive.Style.StrokeWidth > 0 {
			writeSVGStrokeAttrs(b, primitive.Style.Stroke, "#"+primitive.Style.StrokeColor, primitive.Style.StrokeWidth)
		} else if primitive.Style.Stroke != nil {
			writeSVGStrokeAttrs(b, primitive.Style.Stroke, "none", 0)
		}
		if primitive.Path.FillRule != "" {
			fmt.Fprintf(b, ` fill-rule=%q`, primitive.Path.FillRule)
		}
		b.WriteString(`/>`)
	}
}

func svgPrimitiveFillValue(style dml.VectorStyle, fillDefID string) string {
	if style.Fill != nil {
		if fillDefID != "" && drawingcss.NeedsSVGDefinition(style.Fill) {
			return "url(#" + fillDefID + ")"
		}
		if fill, ok := drawingcss.Fill(style.Fill)["fill"]; ok && fill != "" {
			return fill
		}
	}
	if style.FillColor != "" {
		return "#" + style.FillColor
	}
	return "none"
}

func writeSVGStrokeAttrs(b *strings.Builder, style *dml.LineStyle, fallbackColor string, fallbackWidth float64) {
	if b == nil {
		return
	}
	color := fallbackColor
	width := fallbackWidth
	cap := ""
	join := ""
	miterLimit := 0.0
	dashArray := ""
	if style != nil {
		if style.Color != "" {
			color = "#" + style.Color
		}
		if style.Width > 0 {
			width = style.Width
		}
		switch style.Cap {
		case dml.LineCapRound:
			cap = "round"
		case dml.LineCapSquare:
			cap = "square"
		default:
			cap = "butt"
		}
		switch style.Join {
		case dml.LineJoinRound:
			join = "round"
		case dml.LineJoinBevel:
			join = "bevel"
		default:
			join = "miter"
		}
		miterLimit = style.MiterLimit
		dashArray = svgDashArray(style.Dash)
	}
	if color == "" || color == "none" || width <= 0 {
		return
	}
	fmt.Fprintf(b, ` stroke=%q stroke-width="%.2f"`, color, width)
	if cap != "" {
		fmt.Fprintf(b, ` stroke-linecap=%q`, cap)
	}
	if join != "" {
		fmt.Fprintf(b, ` stroke-linejoin=%q`, join)
	}
	if join == "miter" && miterLimit > 0 {
		fmt.Fprintf(b, ` stroke-miterlimit="%.2f"`, miterLimit)
	}
	if dashArray != "" {
		fmt.Fprintf(b, ` stroke-dasharray=%q`, dashArray)
	}
}

func svgDashArray(dash dml.DashPattern) string {
	switch dash {
	case dml.DashPatternDash, dml.DashPatternSystemDash:
		return "3, 3"
	case dml.DashPatternDot, dml.DashPatternSystemDot:
		return "1, 3"
	case dml.DashPatternDashDot, dml.DashPatternSystemDashDot:
		return "3, 3, 1, 3"
	case dml.DashPatternLongDash:
		return "5, 3"
	case dml.DashPatternLongDashDot:
		return "5, 3, 1, 3"
	case dml.DashPatternLongDashDotDot, dml.DashPatternSystemDashDotDot:
		return "5, 3, 1, 3, 1, 3"
	default:
		return ""
	}
}

func svgPathData(path dml.Path) string {
	var parts []string
	for _, cmd := range path.Commands {
		switch cmd.Type {
		case "M", "L":
			if len(cmd.Points) == 0 {
				continue
			}
			parts = append(parts, fmt.Sprintf("%s %.4f %.4f", cmd.Type, cmd.Points[0].X, cmd.Points[0].Y))
		case "C":
			if len(cmd.Points) < 3 {
				continue
			}
			parts = append(parts, fmt.Sprintf("C %.4f %.4f %.4f %.4f %.4f %.4f",
				cmd.Points[0].X, cmd.Points[0].Y,
				cmd.Points[1].X, cmd.Points[1].Y,
				cmd.Points[2].X, cmd.Points[2].Y,
			))
		case "Q":
			if len(cmd.Points) < 2 {
				continue
			}
			parts = append(parts, fmt.Sprintf("Q %.4f %.4f %.4f %.4f",
				cmd.Points[0].X, cmd.Points[0].Y,
				cmd.Points[1].X, cmd.Points[1].Y,
			))
		case "A":
			if len(cmd.Points) == 0 || len(cmd.Args) < 5 {
				continue
			}
			parts = append(parts, fmt.Sprintf("A %.4f %.4f %.4f %.0f %.0f %.4f %.4f",
				cmd.Args[0], cmd.Args[1], cmd.Args[2], cmd.Args[3], cmd.Args[4],
				cmd.Points[0].X, cmd.Points[0].Y,
			))
		case "Z":
			parts = append(parts, "Z")
		}
	}
	return strings.Join(parts, " ")
}

// renderLineSVG renders a line shape with arrowheads as inline SVG when the
// line has head/tail ends or a non-solid dash pattern. Returns true if SVG was
// emitted.
func (r *htmlRenderer) renderLineSVG(shape *resolve.Shape, styleAttrs []string, widthPt, heightPt float64) bool {
	if shape == nil || shape.LineStyle == nil {
		return false
	}
	ls := shape.LineStyle
	hasArrow := (ls.HeadEnd != nil && ls.HeadEnd.Type != dml.ArrowheadNone) ||
		(ls.TailEnd != nil && ls.TailEnd.Type != dml.ArrowheadNone)
	hasDash := ls.Dash != "" && ls.Dash != dml.DashPatternSolid
	if !hasArrow && !hasDash {
		return false
	}

	color := shape.StrokeColor
	if color == "" {
		color = "000000"
	}
	strokeWPt := 1.0
	if ls.Width > 0 {
		strokeWPt = ls.Width
	}

	var svgW, svgH float64
	var x1, y1, x2, y2 float64
	isVertical := heightPt > 0 && widthPt == 0
	if isVertical {
		svgW = strokeWPt * 4
		svgH = heightPt
		x1 = svgW / 2
		y1 = 0
		x2 = svgW / 2
		y2 = svgH
	} else {
		svgW = widthPt
		svgH = strokeWPt * 4
		x1 = 0
		y1 = svgH / 2
		x2 = svgW
		y2 = svgH / 2
	}

	var defs []string
	var paths []string
	strokeAttr := fmt.Sprintf(`stroke="#%s" stroke-width="%.2f"`, color, strokeWPt)
	if hasDash {
		strokeAttr += fmt.Sprintf(` stroke-dasharray="%s"`, ls.DashArray())
	}
	markerStart := ""
	markerEnd := ""

	if ls.HeadEnd != nil && ls.HeadEnd.Type != dml.ArrowheadNone {
		id := fmt.Sprintf("ah-%p", ls.HeadEnd)
		defs = append(defs, arrowheadMarkerDef(id, ls.HeadEnd, color, strokeWPt, false))
		markerStart = fmt.Sprintf(` marker-start="url(#%s)"`, id)
	}
	if ls.TailEnd != nil && ls.TailEnd.Type != dml.ArrowheadNone {
		id := fmt.Sprintf("at-%p", ls.TailEnd)
		defs = append(defs, arrowheadMarkerDef(id, ls.TailEnd, color, strokeWPt, false))
		markerEnd = fmt.Sprintf(` marker-end="url(#%s)"`, id)
	}

	paths = append(paths, fmt.Sprintf(`<line x1="%.2f" y1="%.2f" x2="%.2f" y2="%.2f" %s%s%s fill="none"/>`,
		x1, y1, x2, y2, strokeAttr, markerStart, markerEnd))

	var b strings.Builder
	b.WriteString(`<svg xmlns="http://www.w3.org/2000/svg"`)
	fmt.Fprintf(&b, ` width="%.2f" height="%.2f"`, svgW, svgH)
	b.WriteString(` style="display:block`)
	if t := boxTransformCSS(shape.RotationDeg, shape.FlipH, shape.FlipV); t != "" {
		fmt.Fprintf(&b, `;transform:%s`, t)
	}
	b.WriteString(`"`)
	b.WriteString(`>`)
	if len(defs) > 0 {
		b.WriteString(`<defs>`)
		for _, d := range defs {
			b.WriteString(d)
		}
		b.WriteString(`</defs>`)
	}
	for _, p := range paths {
		b.WriteString(p)
	}
	b.WriteString(`</svg>`)

	styles := append([]string{}, styleAttrs...)
	if isVertical {
		styles = append(styles, fmt.Sprintf("height:%.1fpt", heightPt))
	} else {
		styles = append(styles, fmt.Sprintf("width:%.1fpt", widthPt))
	}
	plan := buildShapeEffectRenderPlan(shape)
	originalEffectStyles := effectStylePropsFromEffects(plan.OriginalEffects)
	styles = appendEffectTransformStyles(styles, "", originalEffectStyles)
	fmt.Fprintf(&r.buf, `<div class="shape-line-svg" style=%q>%s</div>`, strings.Join(styles, ";"), b.String())
	return true
}

func arrowheadMarkerDef(id string, ah *dml.Arrowhead, color string, lineWidth float64, open bool) string {
	path := ah.PathForArrowhead(lineWidth)
	if path == "" {
		return ""
	}
	w := ah.Width()
	l := ah.LengthPoints()
	refX := l
	refY := w / 2
	viewW := l
	viewH := w
	fill := "none"
	if !open {
		fill = "#" + color
	}
	return fmt.Sprintf(`<marker id=%q markerWidth="%.2f" markerHeight="%.2f" refX="%.2f" refY="%.2f" orient="auto" markerUnits="userSpaceOnUse"><path d=%q fill=%q stroke="#%s" stroke-width="%.2f"/></marker>`,
		id, viewW, viewH, refX, refY, path, fill, color, lineWidth*0.5)
}

func (r *htmlRenderer) renderShapeWithStyleAttrs(buf *bytes.Buffer, shape *resolve.Shape, styleAttrs []string, widthPt, heightPt float64) {
	if shape == nil {
		return
	}
	r.withOutputBuffer(buf, func() {
		// Geometry text/WordArt is not a rectangular textbox. It is painted as
		// text following a geometry path, so it owns the shape host before the
		// ordinary vector/rect fallbacks get a chance to draw a box behind it.
		if shape.TextBox == nil && shape.TextPath != nil && shape.TextPath.String != "" {
			if r.renderWordArtShapeWithStyleAttrs(&r.buf, shape, styleAttrs, widthPt, heightPt) {
				return
			}
		}
		if r.renderVectorShapeWithStyleAttrs(&r.buf, shape, styleAttrs, widthPt, heightPt) {
			// Vector renderer owns the positioned host and renders shape text inside
			// that same host. Rendering text here would make it a page-level sibling.
			return
		}
		switch shape.Kind {
		case "line":
			if r.renderLineSVG(shape, styleAttrs, widthPt, heightPt) {
				break
			}
			baseStyles := append([]string{}, styleAttrs...)
			color := shape.StrokeColor
			if color == "" {
				color = "000000"
			}
			strokeW := "1pt"
			if shape.StrokeWidth > 0 {
				strokeW = fmt.Sprintf("%.2fpt", emuToPoints(shape.StrokeWidth))
			}
			plan := buildShapeEffectRenderPlan(shape)
			originalEffectStyles := effectStylePropsFromEffects(plan.OriginalEffects)
			reflectionEffectStyles := effectStylePropsFromEffects(plan.ReflectionEffects)
			if heightPt > 0 && widthPt == 0 {
				baseStyles = append(baseStyles, fmt.Sprintf("height:%.1fpt", heightPt))
				baseStyles = append(baseStyles, "border-left-color:#"+color)
				baseStyles = append(baseStyles, "border-left-width:"+strokeW)
				if reflectionStyles := shapeLineReflectionStyles(baseStyles, shape, plan.Reflection, widthPt, heightPt, reflectionEffectStyles); len(reflectionStyles) > 0 {
					fmt.Fprintf(&r.buf, `<div class="shape-line-v shape-line-reflection" aria-hidden="true" style=%q></div>`, strings.Join(reflectionStyles, ";"))
				}
				styles := appendEffectTransformStyles(append([]string{}, baseStyles...), boxTransformCSS(shape.RotationDeg, shape.FlipH, shape.FlipV), originalEffectStyles)
				fmt.Fprintf(&r.buf, `<div class="shape-line-v" style=%q></div>`, strings.Join(styles, ";"))
			} else {
				if widthPt > 0 {
					baseStyles = append(baseStyles, fmt.Sprintf("width:%.1fpt", widthPt))
				}
				baseStyles = append(baseStyles, "border-top-color:#"+color)
				baseStyles = append(baseStyles, "border-top-width:"+strokeW)
				if reflectionStyles := shapeLineReflectionStyles(baseStyles, shape, plan.Reflection, widthPt, heightPt, reflectionEffectStyles); len(reflectionStyles) > 0 {
					fmt.Fprintf(&r.buf, `<div class="shape-line shape-line-reflection" aria-hidden="true" style=%q></div>`, strings.Join(reflectionStyles, ";"))
				}
				styles := appendEffectTransformStyles(append([]string{}, baseStyles...), boxTransformCSS(shape.RotationDeg, shape.FlipH, shape.FlipV), originalEffectStyles)
				fmt.Fprintf(&r.buf, `<div class="shape-line" style=%q></div>`, strings.Join(styles, ";"))
			}
		case "rect":
			plan := buildShapeEffectRenderPlan(shape)
			originalEffectStyles := effectStylePropsFromEffects(plan.OriginalEffects)
			reflectionEffectStyles := effectStylePropsFromEffects(plan.ReflectionEffects)
			overlays := plan.originalOverlays()
			reflectionStyles := shapeReflectionBoxStyles(shape, plan.Reflection, widthPt, heightPt, reflectionEffectStyles)
			if len(overlays) == 0 && len(reflectionStyles) == 0 {
				plainStyles := append([]string{}, styleAttrs...)
				if widthPt > 0 {
					plainStyles = append(plainStyles, fmt.Sprintf("width:%.1fpt", widthPt))
				}
				if heightPt > 0 {
					plainStyles = append(plainStyles, fmt.Sprintf("height:%.1fpt", heightPt))
				}
				if shape.FillColor != "" {
					plainStyles = append(plainStyles, "background-color:#"+shape.FillColor)
				}
				if shape.StrokeColor != "" {
					w := "1pt"
					if shape.StrokeWidth > 0 {
						w = fmt.Sprintf("%.2fpt", emuToPoints(shape.StrokeWidth))
					}
					plainStyles = append(plainStyles, fmt.Sprintf("border:%s solid #%s", w, shape.StrokeColor))
				}
				if shape.DistTop > 0 || shape.DistRight > 0 || shape.DistBottom > 0 || shape.DistLeft > 0 {
					plainStyles = append(plainStyles, fmt.Sprintf("margin:%.1fpt %.1fpt %.1fpt %.1fpt", shape.DistTop, shape.DistRight, shape.DistBottom, shape.DistLeft))
				}
				plainStyles = appendEffectTransformStyles(plainStyles, boxTransformCSS(shape.RotationDeg, shape.FlipH, shape.FlipV), originalEffectStyles)
				fmt.Fprintf(&r.buf, `<div class="shape-rect" style=%q>`, strings.Join(plainStyles, ";"))
				r.renderShapeText(shape, widthPt, heightPt)
				r.buf.WriteString(`</div>`)
				return
			}
			styles := append([]string{}, styleAttrs...)
			if widthPt > 0 {
				styles = append(styles, fmt.Sprintf("width:%.1fpt", widthPt))
			}
			if heightPt > 0 {
				styles = append(styles, fmt.Sprintf("height:%.1fpt", heightPt))
			}
			if transform := boxTransformCSS(shape.RotationDeg, shape.FlipH, shape.FlipV); transform != "" {
				styles = append(styles, "transform:"+transform, "transform-origin:50% 50%")
			}
			styles = append(styles, "overflow:visible")
			if shape.DistTop > 0 || shape.DistRight > 0 || shape.DistBottom > 0 || shape.DistLeft > 0 {
				styles = append(styles, fmt.Sprintf("margin:%.1fpt %.1fpt %.1fpt %.1fpt", shape.DistTop, shape.DistRight, shape.DistBottom, shape.DistLeft))
			}
			contentStyles := []string{"position:absolute", "inset:0"}
			if shape.FillColor != "" {
				contentStyles = append(contentStyles, "background-color:#"+shape.FillColor)
			}
			if shape.StrokeColor != "" {
				w := "1pt"
				if shape.StrokeWidth > 0 {
					w = fmt.Sprintf("%.2fpt", emuToPoints(shape.StrokeWidth))
				}
				contentStyles = append(contentStyles, fmt.Sprintf("border:%s solid #%s", w, shape.StrokeColor))
			}
			contentStyles = appendEffectTransformStyles(contentStyles, "", originalEffectStyles)
			fmt.Fprintf(&r.buf, `<div class="shape-rect" style=%q>`, strings.Join(styles, ";"))
			if len(reflectionStyles) > 0 {
				fmt.Fprintf(&r.buf, `<div class="shape-reflection" aria-hidden="true" style=%q>`, strings.Join(reflectionStyles, ";"))
				r.writeShapeFillOverlayDivs(plan.reflectionOverlays(), "shape-fill-overlay shape-fill-overlay--reflection")
				r.buf.WriteString(`</div>`)
			}
			fmt.Fprintf(&r.buf, `<div class="shape-rect__content" style=%q>`, strings.Join(contentStyles, ";"))
			r.writeShapeFillOverlayDivs(overlays, "shape-fill-overlay")
			r.renderShapeText(shape, widthPt, heightPt)
			r.buf.WriteString(`</div>`)
			r.buf.WriteString(`</div>`)
		case "group":
			groupStyles := append([]string{}, styleAttrs...)
			if widthPt > 0 {
				groupStyles = append(groupStyles, fmt.Sprintf("width:%.1fpt", widthPt))
			}
			if heightPt > 0 {
				groupStyles = append(groupStyles, fmt.Sprintf("height:%.1fpt", heightPt))
			}
			hasPosition := false
			for _, s := range groupStyles {
				if strings.HasPrefix(s, "position:") {
					hasPosition = true
					break
				}
			}
			if !hasPosition {
				groupStyles = append(groupStyles, "position:relative")
			}
			if transform := boxTransformCSS(shape.RotationDeg, shape.FlipH, shape.FlipV); transform != "" {
				groupStyles = append(groupStyles, "transform:"+transform, "transform-origin:50% 50%")
			}
			groupStyles = append(groupStyles, "overflow:visible")
			fmt.Fprintf(&r.buf, `<div class="shape-group" style=%q>`, strings.Join(groupStyles, ";"))
			for i := range shape.Children {
				child := &shape.Children[i]
				childWidthPt := emuToPoints(child.Width)
				childHeightPt := emuToPoints(child.Height)
				childStyles := []string{"position:absolute"}
				childStyles = append(childStyles, fmt.Sprintf("left:%.1fpt", emuToPoints(child.X-shape.X)))
				childStyles = append(childStyles, fmt.Sprintf("top:%.1fpt", emuToPoints(child.Y-shape.Y)))
				if childWidthPt > 0 {
					childStyles = append(childStyles, fmt.Sprintf("width:%.1fpt", childWidthPt))
				}
				if childHeightPt > 0 {
					childStyles = append(childStyles, fmt.Sprintf("height:%.1fpt", childHeightPt))
				}
				r.renderShapeWithStyleAttrs(&r.buf, child, childStyles, childWidthPt, childHeightPt)
			}
			r.buf.WriteString(`</div>`)
		}
	})
}

// renderWordArtShapeWithStyleAttrs renders OfficeArt geometry text as SVG
// text-on-path. This is deliberately separate from textbox rendering: WordArt
// follows/deforms with the shape geometry instead of treating the shape as a
// rectangular text viewport.
func (r *htmlRenderer) renderWordArtShapeWithStyleAttrs(buf *bytes.Buffer, shape *resolve.Shape, styleAttrs []string, widthPt, heightPt float64) bool {
	if shape == nil || shape.TextPath == nil || strings.TrimSpace(shape.TextPath.String) == "" {
		return false
	}
	path := shape.TextPath
	if widthPt <= 0 {
		widthPt = 1
	}
	if heightPt <= 0 {
		heightPt = 1
	}

	hostStyles := append([]string{}, styleAttrs...)
	hostStyles = append(hostStyles,
		fmt.Sprintf("width:%.1fpt", widthPt),
		fmt.Sprintf("height:%.1fpt", heightPt),
		"overflow:visible",
	)
	plan := buildShapeEffectRenderPlan(shape)
	hostStyles = appendEffectTransformStyles(hostStyles, boxTransformCSS(shape.RotationDeg, shape.FlipH, shape.FlipV), effectStylePropsFromEffects(plan.OriginalEffects))

	pathID := fmt.Sprintf("wordart-path-%d", r.nextWordArtIndex)
	r.nextWordArtIndex++
	pathData := wordArtPathData(path.Warp, path.Vertical)
	startOffset, textAnchor := wordArtAlignment(path.Align)

	textStyles := []string{"dominant-baseline:middle", "paint-order:stroke fill"}
	if path.Font != "" {
		textStyles = append(textStyles, "font-family:'"+strings.ReplaceAll(path.Font, "'", "\\'")+"'")
	}
	if path.SizePt > 0 {
		// The viewBox is 100x100. Keep authored size as CSS points so the
		// browser preserves Word's font metrics before path deformation.
		textStyles = append(textStyles, fmt.Sprintf("font-size:%.2fpt", path.SizePt))
	}
	if path.Bold {
		textStyles = append(textStyles, "font-weight:bold")
	}
	if path.Italic {
		textStyles = append(textStyles, "font-style:italic")
	}
	if path.SmallCaps {
		textStyles = append(textStyles, "font-variant:small-caps")
	}
	decorations := make([]string, 0, 2)
	if path.Underline {
		decorations = append(decorations, "underline")
	}
	if path.Strikethrough {
		decorations = append(decorations, "line-through")
	}
	if len(decorations) > 0 {
		textStyles = append(textStyles, "text-decoration:"+strings.Join(decorations, " "))
	}
	if path.Kerning {
		textStyles = append(textStyles, "font-kerning:normal")
	}
	if path.Spacing > 0 && math.Abs(path.Spacing-1) > 0.0001 {
		// gtextSpacing is a 16.16 scaling factor. CSS has no direct equivalent;
		// tracking is best represented as an em-relative delta around 1.0.
		textStyles = append(textStyles, fmt.Sprintf("letter-spacing:%.4fem", path.Spacing-1))
	}
	if path.Shadow {
		textStyles = append(textStyles, "filter:drop-shadow(1px 1px 1px rgba(0,0,0,.45))")
	}

	fill := "#000000"
	if shape.FillColor != "" {
		fill = "#" + shape.FillColor
	}
	stroke := ""
	if shape.StrokeColor != "" {
		stroke = "#" + shape.StrokeColor
	}

	fmt.Fprintf(buf, `<div class="shape-wordart-host" style=%q>`, strings.Join(hostStyles, ";"))
	fmt.Fprintf(buf, `<svg class="shape-wordart" viewBox="0 0 100 100" width="100%%" height="100%%" preserveAspectRatio="none" role="img" aria-label=%q xmlns="http://www.w3.org/2000/svg">`, path.String)
	fmt.Fprintf(buf, `<defs><path id=%q d=%q pathLength="100"/></defs>`, pathID, pathData)
	fmt.Fprintf(buf, `<text fill=%q text-anchor=%q style=%q`, fill, textAnchor, strings.Join(textStyles, ";"))
	if stroke != "" {
		fmt.Fprintf(buf, ` stroke=%q stroke-width="0.6"`, stroke)
	}
	buf.WriteString(`>`)
	fmt.Fprintf(buf, `<textPath href="#%s" startOffset=%q`, pathID, startOffset)
	if path.Stretch || path.BestFit {
		buf.WriteString(` textLength="90" lengthAdjust="spacingAndGlyphs"`)
	}
	buf.WriteString(`>`)
	buf.WriteString(html.EscapeString(path.String))
	buf.WriteString(`</textPath></text></svg></div>`)
	return true
}

func wordArtAlignment(align uint32) (startOffset, textAnchor string) {
	// OfficeArt geometry-text documents encountered in Word use 1=center,
	// 2=left/start and 3=right/end. Other modes (stretch/letter/word justify)
	// are centered here and are further approximated by textLength when the
	// authored best-fit/stretch flags are present.
	switch align {
	case 2:
		return "5%", "start"
	case 3:
		return "95%", "end"
	default:
		return "50%", "middle"
	}
}

func wordArtPathData(warp string, vertical bool) string {
	if vertical {
		return "M 50 5 L 50 95"
	}
	switch strings.ToLower(strings.TrimSpace(warp)) {
	case "textarchupcurve", "textarchuppour", "textcurveup", "textoncurve":
		return "M 5 78 Q 50 5 95 78"
	case "textarchdowncurve", "textarchdownpour", "textcurvedown":
		return "M 5 22 Q 50 95 95 22"
	case "textcirclecurve", "textcirclepour", "textring", "textonring", "textringinside", "textringoutside":
		return "M 5 55 A 45 45 0 0 1 95 55"
	case "textbuttoncurve", "textbuttonpour", "textcanup":
		return "M 5 62 Q 50 18 95 62"
	case "textcandown":
		return "M 5 38 Q 50 82 95 38"
	case "textwave", "textwave1", "textwave2", "textwave3", "textwave4":
		return "M 3 52 C 18 18 32 86 50 52 C 68 18 82 86 97 52"
	case "textcascadeup", "textslantup", "textfadeup":
		return "M 5 78 L 95 22"
	case "textcascadedown", "textslantdown", "textfadedown":
		return "M 5 22 L 95 78"
	case "texttriangle", "textchevron":
		return "M 5 75 L 50 20 L 95 75"
	case "texttriangleinverted", "textchevroninverted":
		return "M 5 25 L 50 80 L 95 25"
	default:
		return "M 5 50 L 95 50"
	}
}

// renderShapeText renders the text content of a shape inside its geometry.
//
// The shape owns geometric placement/rotation. The textbox story gets a local
// layout viewport exactly widthPt x heightPt, so wrapping, paragraph widths and
// vertical alignment are computed against the rectangle rather than the page.
func (r *htmlRenderer) renderShapeText(shape *resolve.Shape, widthPt, heightPt float64) {
	tb := shape.TextBox
	if tb == nil && shape.TextPath == nil {
		return
	}
	if tb == nil && shape.TextPath != nil {
		styles := shapeTextPathStyles(shape.TextPath)
		fmt.Fprintf(&r.buf, `<div class="shape-text shape-text-path" style=%q>`, strings.Join(styles, ";"))
		r.buf.WriteString(html.EscapeString(shape.TextPath.String))
		r.buf.WriteString(`</div>`)
		return
	}

	styles := shapeTextBoxStyles(tb)
	if shape.LegacyFlowAnchor && tb.Vert == "vert" {
		// OfficeArt's vertical text flow is sideways Latin text, not upright
		// glyphs in a CSS vertical writing mode.
		styles = removeCSSDeclaration(styles, "writing-mode:")
		styles = removeCSSDeclaration(styles, "text-orientation:")
		transform := "rotate(-90deg)"
		if shape.LegacyUprightText {
			transform += " " + legacyUprightTextTransformCSS(shape)
		}
		styles = append(styles, "transform:"+transform, "transform-origin:50% 50%")
	} else if shape.LegacyUprightText {
		// Word 97-era text boxes rotate their outline around the anchor
		// rectangle, but leave their text horizontal in that rectangle.
		styles = append(styles, "transform:"+legacyUprightTextTransformCSS(shape), "transform-origin:50% 50%")
	}
	fmt.Fprintf(&r.buf, `<div class="shape-text" style=%q>`, strings.Join(styles, ";"))

	contentHTML := r.renderTextBoxContentHTML(tb, widthPt, heightPt)
	innerStyles := textBoxInnerStyles(tb)
	if len(innerStyles) > 0 {
		fmt.Fprintf(&r.buf, `<div class="shape-text__inner" style=%q>`, strings.Join(innerStyles, ";"))
	}
	r.buf.WriteString(contentHTML)
	if len(innerStyles) > 0 {
		r.buf.WriteString(`</div>`)
	}
	r.buf.WriteString(`</div>`)
}

// legacyUprightTextTransformCSS cancels the OfficeArt outline transform on a
// nested legacy textbox. The outline transform is scale(s), then rotate(r), so
// the nested text uses its inverse in reverse order.
func legacyUprightTextTransformCSS(shape *resolve.Shape) string {
	if shape == nil {
		return ""
	}
	parts := make([]string, 0, 3)
	if shape.RotationDeg != 0 {
		parts = append(parts, fmt.Sprintf("rotate(%.2fdeg)", -shape.RotationDeg))
	}
	if shape.FlipV {
		parts = append(parts, "scaleY(-1)")
	}
	if shape.FlipH {
		parts = append(parts, "scaleX(-1)")
	}
	return strings.Join(parts, " ")
}

// shapeTextBoxStyles returns the text-layout CSS for a textbox hosted by a
// Shape. The shape itself already owns x/y/size/rotation, so this deliberately
// does not apply tb.RotationDeg/tb.Flip* a second time.
func shapeTextPathStyles(path *resolve.TextPath) []string {
	styles := []string{
		"position:absolute",
		"inset:0",
		"box-sizing:border-box",
		"overflow:hidden",
		"display:flex",
		"align-items:center",
		"justify-content:center",
		"text-align:center",
	}
	if path == nil {
		return styles
	}
	if path.Font != "" {
		styles = append(styles, fmt.Sprintf("font-family:%q", path.Font))
	}
	if path.SizePt > 0 {
		styles = append(styles, fmt.Sprintf("font-size:%.2fpt", path.SizePt))
	}
	if path.Bold {
		styles = append(styles, "font-weight:bold")
	}
	if path.Italic {
		styles = append(styles, "font-style:italic")
	}
	if path.Vertical {
		styles = append(styles, "writing-mode:vertical-rl", "text-orientation:mixed")
	}
	return styles
}

func shapeTextBoxStyles(tb *resolve.TextBox) []string {
	if tb == nil {
		return nil
	}
	styles := []string{
		"position:absolute",
		"inset:0",
		"box-sizing:border-box",
		"overflow:hidden",
	}

	pl, pt, pr, pb := tb.PaddingLeft, tb.PaddingTop, tb.PaddingRight, tb.PaddingBottom
	if pl != 0 || pt != 0 || pr != 0 || pb != 0 {
		if pl == pr && pt == pb && pl == pt {
			styles = append(styles, fmt.Sprintf("padding:%.2fpt", pl))
		} else if pl == pr && pt == pb {
			styles = append(styles, fmt.Sprintf("padding:%.2fpt %.2fpt", pt, pl))
		} else {
			styles = append(styles, fmt.Sprintf("padding:%.2fpt %.2fpt %.2fpt %.2fpt", pt, pr, pb, pl))
		}
	}

	if tb.NoWrap {
		styles = append(styles, "white-space:nowrap")
	}
	switch tb.Vert {
	case "vert":
		styles = append(styles, "writing-mode:vertical-rl", "text-orientation:mixed")
	case "vert270":
		styles = append(styles, "writing-mode:vertical-lr", "text-orientation:mixed", "transform:rotate(180deg)", "transform-origin:50% 50%")
	}

	switch tb.Autofit {
	case "none":
		if tb.NoWrap {
			styles = replaceCSSDeclaration(styles, "overflow:", "overflow:visible")
		}
	case "shape":
		styles = replaceCSSDeclaration(styles, "overflow:", "overflow:hidden")
	case "norm":
		if tb.FontScale > 0 {
			scale := float64(tb.FontScale) / 100000.0
			styles = append(styles, fmt.Sprintf("font-size:%.2f%%", scale*100))
		}
		if tb.LnSpcReduction > 0 {
			reduction := float64(tb.LnSpcReduction) / 100000.0
			lineHeight := 1.0 - reduction
			if lineHeight < 0.1 {
				lineHeight = 0.1
			}
			styles = append(styles, fmt.Sprintf("line-height:%.2f", lineHeight))
		}
	}
	return styles
}

func replaceCSSDeclaration(styles []string, prefix, replacement string) []string {
	for i, style := range styles {
		if strings.HasPrefix(style, prefix) {
			styles[i] = replacement
			return styles
		}
	}
	return append(styles, replacement)
}

func removeCSSDeclaration(styles []string, prefix string) []string {
	out := styles[:0]
	for _, style := range styles {
		if !strings.HasPrefix(style, prefix) {
			out = append(out, style)
		}
	}
	return out
}

func (r *htmlRenderer) shapeFloatBox(shape *resolve.Shape) (layout.FloatBox, bool) {
	if shape == nil {
		return layout.FloatBox{}, false
	}
	if fb, ok := r.floatByShape[shape]; ok {
		return fb, true
	}
	// Layout can walk copied paragraph/run slices while rendering walks the
	// canonical document blocks. Pointer identity is not stable across that
	// boundary; the legacy OfficeArt FSP.spid is.
	if shape.LegacyShapeID != 0 {
		fb, ok := r.floatByLegacyShapeID[shape.LegacyShapeID]
		return fb, ok
	}
	return layout.FloatBox{}, false
}

// renderShapeTo writes a shape to buf. Floating shapes use layout-computed
// page-absolute coordinates.
func (r *htmlRenderer) renderShapeTo(buf *bytes.Buffer, shape *resolve.Shape) {
	if shape == nil {
		return
	}
	widthPt := emuToPoints(shape.Width)
	heightPt := emuToPoints(shape.Height)
	var styleAttrs []string
	if shape.BehindDoc {
		styleAttrs = append(styleAttrs, "position:absolute;z-index:0")
	} else {
		styleAttrs = append(styleAttrs, fmt.Sprintf("position:absolute;z-index:%d", r.normalizeZIndex(shape.ZIndex)))
	}
	if shape.SimplePosAttr {
		styleAttrs = append(styleAttrs, fmt.Sprintf("left:%.1fpt", emuToPoints(shape.SimplePosX)))
		styleAttrs = append(styleAttrs, fmt.Sprintf("top:%.1fpt", emuToPoints(shape.SimplePosY)))
	} else if fb, ok := r.shapeFloatBox(shape); ok {
		styleAttrs = append(styleAttrs, fmt.Sprintf("left:%.1fpt", fb.X))
		styleAttrs = append(styleAttrs, fmt.Sprintf("top:%.1fpt", fb.Y))
		widthPt = boxWidthPt(fb.Width, widthPt)
		heightPt = boxHeightPt(fb.Height, heightPt)
	} else {
		// Zero is a real offset. For example FSPA bx=margin, xaLeft=0 means
		// exactly the left margin, not "leave CSS left unset".
		styleAttrs = append(styleAttrs, fmt.Sprintf("left:%.1fpt", r.resolveFloatLeft(shape.X, shape.XAlign, shape.XRelative, shape.Width, false)))
		styleAttrs = append(styleAttrs, fmt.Sprintf("top:%.1fpt", r.resolveFloatTop(shape.Y, shape.YAlign, shape.YRelative, shape.Height, false)))
	}
	r.renderShapeWithStyleAttrs(buf, shape, styleAttrs, widthPt, heightPt)
}

func boxTransformCSS(rotationDeg float64, flipH, flipV bool) string {
	transforms := make([]string, 0, 3)
	if flipH {
		transforms = append(transforms, "scaleX(-1)")
	}
	if flipV {
		transforms = append(transforms, "scaleY(-1)")
	}
	if rotationDeg != 0 {
		transforms = append(transforms, fmt.Sprintf("rotate(%.2fdeg)", rotationDeg))
	}
	return strings.Join(transforms, " ")
}

func paragraphNeedsBlockContainerRuns(runs []resolve.Run) bool {
	for _, run := range runs {
		if run.Kind == "textbox" || run.Kind == "shape" {
			return true
		}
	}
	return false
}

// paragraphIsEmptyOrAnchorOnly returns true when the paragraph has no visible
// inline content — it is either empty or contains only floating anchor runs.
// These paragraphs need an explicit min-height so the flow contributes the same
// vertical space as Word allocates for them.
// paragraphIsFloatingAnchorOnly distinguishes a drawing host from a genuinely
// empty Word paragraph. Genuine empty paragraphs remain browser-visible because
// they own text-flow height. Drawing hosts own layout/anchor progression too, but
// their visible content lives entirely in the page overlay and must not create a
// second CSS flow box in page-body.
func paragraphIsFloatingAnchorOnly(runs []resolve.Run) bool {
	foundAnchor := false
	for _, run := range runs {
		switch run.Kind {
		case "image":
			if run.Image != nil && run.Image.Floating {
				foundAnchor = true
				continue
			}
		case "textbox":
			if run.TextBox != nil && run.TextBox.Floating {
				foundAnchor = true
				continue
			}
		case "shape":
			if run.Shape != nil && run.Shape.Floating {
				foundAnchor = true
				continue
			}
		}
		if strings.TrimSpace(run.Text) != "" || run.Kind == "horzRule" {
			return false
		}
		// Any non-empty non-anchor run is real inline content.
		if run.Kind != "" && run.Kind != "run" && run.Kind != "image" && run.Kind != "textbox" && run.Kind != "shape" {
			return false
		}
	}
	return foundAnchor
}

// paragraphHasOnlyBehindTextAnchors identifies empty anchor paragraphs that
// still own a normal blank paragraph line. The background drawing itself is
// out of flow, but replacing its host with display:contents also erases that
// line and pulls following text upward. Keep the host block so the subsequent
// text preserves the authored paragraph break.
func paragraphHasOnlyBehindTextAnchors(runs []resolve.Run) bool {
	found := false
	for _, run := range runs {
		switch run.Kind {
		case "image":
			if run.Image != nil && run.Image.Floating {
				if !run.Image.BehindDoc {
					return false
				}
				found = true
			}
		case "textbox":
			if run.TextBox != nil && run.TextBox.Floating {
				if !run.TextBox.BehindDoc {
					return false
				}
				found = true
			}
		case "shape":
			if run.Shape != nil && run.Shape.Floating {
				if !run.Shape.BehindDoc {
					return false
				}
				found = true
			}
		}
	}
	return found
}

// resolvedMarkerTemplate turns an eagerly resolved marker such as "3.", "(iv)"
// or ")" into the placeholder template that renders it, so the current level's
// counter can be substituted while the marker's punctuation survives. Mixed
// markers ("1.1.") keep their ancestor text and re-run only the last component.
func resolvedMarkerTemplate(text string, placeholderLevel int) string {
	placeholder := "%" + strconv.Itoa(placeholderLevel)
	runes := []rune(strings.TrimSpace(text))
	if len(runes) == 0 {
		return placeholder
	}
	start, end := -1, -1
	for i, r := range runes {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if start < 0 {
				start = i
			}
			end = i
		}
	}
	if start < 0 {
		// Punctuation only: the counter precedes the authored suffix.
		return placeholder + string(runes)
	}
	return string(runes[:start]) + placeholder + string(runes[end+1:])
}

func paragraphIsEmptyOrAnchorOnly(runs []resolve.Run) bool {
	if len(runs) == 0 {
		return true
	}
	for _, run := range runs {
		switch run.Kind {
		case "image":
			if run.Image != nil && run.Image.Floating {
				continue
			}
		case "textbox":
			if run.TextBox != nil && run.TextBox.Floating {
				continue
			}
		case "shape":
			if run.Shape != nil && run.Shape.Floating {
				continue
			}
		case "horzRule":
			return false
		default:
			if strings.TrimSpace(run.Text) != "" {
				return false
			}
		}
	}
	return true
}

// docGridIsActive reports whether a section's document grid actually snaps
// lines. w:docGrid/@w:type defaults to "default", which means no grid, so a
// section that only carries a line pitch must not impose one on the browser.
func docGridIsActive(grid *resolve.DocGrid) bool {
	if grid == nil {
		return false
	}
	return strings.EqualFold(grid.Type, "lines") || strings.EqualFold(grid.Type, "linesAndChars")
}

func appendInlineStyleAttr(attr, decl string) string {
	if attr == "" {
		return fmt.Sprintf(" style=%q", decl)
	}
	if strings.HasSuffix(attr, `"`) {
		return strings.TrimSuffix(attr, `"`) + ";" + decl + `"`
	}
	return attr
}

// logicalPageNumber returns the displayed page number for the given physical page
// index, accounting for section-level restarts and explicit start values.
func (r *htmlRenderer) logicalPageNumber(pageIndex int) int {
	if pageIndex < 0 || pageIndex >= len(r.pages) {
		return pageIndex + 1
	}
	logical := 1
	if r.pages[0].Section.PageNumberStart != nil {
		logical = *r.pages[0].Section.PageNumberStart
	}
	for i := 1; i <= pageIndex; i++ {
		if r.pages[i].SectionIndex != r.pages[i-1].SectionIndex {
			if r.pages[i].Section.PageNumberRestart {
				if r.pages[i].Section.PageNumberStart != nil {
					logical = *r.pages[i].Section.PageNumberStart
				} else {
					logical = 1
				}
				continue
			}
		}
		logical++
	}
	return logical
}

// parseIntegerFieldGeneralFormat extracts the common Word general-format switch used
// by integer-valued fields.  The switch is case-sensitive for ROMAN/roman and
// ALPHABETIC/alphabetic, so the returned token preserves authored case.
// MERGEFORMAT and CHARFORMAT are presentation-preservation switches, not numeric
// formats, and are intentionally ignored here.
func parseIntegerFieldGeneralFormat(instr string) string {
	fields := strings.Fields(instr)
	for i := 0; i+1 < len(fields); i++ {
		if fields[i] != `\*` {
			continue
		}
		candidate := strings.Trim(fields[i+1], `"`)
		switch strings.ToLower(candidate) {
		case "mergeformat", "charformat":
			continue
		case "arabic", "roman", "alphabetic":
			return candidate
		}
	}
	return ""
}

// wordNumberFormatForFieldGeneral maps Word field general-format names onto the
// renderer's numbering vocabulary while preserving case-sensitive variants.
func wordNumberFormatForFieldGeneral(format string) string {
	switch format {
	case "ROMAN":
		return "upperRoman"
	case "roman":
		return "lowerRoman"
	case "ALPHABETIC":
		return "upperLetter"
	case "alphabetic":
		return "lowerLetter"
	}
	switch strings.ToLower(format) {
	case "arabic":
		return "decimal"
	case "roman":
		return "lowerRoman"
	case "alphabetic":
		return "lowerLetter"
	}
	return ""
}

// formatIntegerFieldValue applies the numeric-picture (\#) and general-format
// (\*) switches accepted by PAGE-like Word fields.  An explicit field switch
// wins over fallbackFormat (normally the section page-number format).
func formatIntegerFieldValue(value int, instr, fallbackFormat string) string {
	if picture := parseRefNumberFormat(instr); picture != "" {
		return formatRefNumber(strconv.Itoa(value), picture)
	}
	if general := parseIntegerFieldGeneralFormat(instr); general != "" {
		return formatListNumber(value, wordNumberFormatForFieldGeneral(general))
	}
	return formatListNumber(value, fallbackFormat)
}

// calculateFieldValue computes the actual value for calculated fields like PAGE, DATE, etc.
// Returns empty string if the field should use the original run text.
func (r *htmlRenderer) fieldEvaluationTime() time.Time {
	if r != nil && r.options.RenderTime != nil {
		return *r.options.RenderTime
	}
	return time.Now()
}

func (r *htmlRenderer) calculateFieldValue(run resolve.Run) string {
	switch run.FieldKind {
	case "PAGE":
		// Return current page number (1-based). An explicit field-format switch
		// overrides the section page-number format, matching Word field semantics.
		pageNum := r.logicalPageNumber(r.pageIndex)
		format := ""
		if r.pageIndex >= 0 && r.pageIndex < len(r.pages) {
			format = r.pages[r.pageIndex].Section.PageNumberFormat
		}
		return formatIntegerFieldValue(pageNum, r.resolveFieldInstruction(run), format)
	case "NUMPAGES":
		// NUMPAGES is decimal by default but accepts the same general/numeric
		// formatting switches as PAGE (for example \* ROMAN or \# "000").
		pages := len(r.pages)
		if pages <= 0 {
			pages = 1
		}
		return formatIntegerFieldValue(pages, r.resolveFieldInstruction(run), "decimal")
	case "DATE":
		// DATE is volatile. Word refreshes it when the document is opened/printed; a
		// cached binary field result is not authoritative rendering content.
		return formatFieldDateTime(run.FieldInstr, run.FieldKind, r.fieldEvaluationTime())
	case "TIME":
		return formatFieldDateTime(run.FieldInstr, run.FieldKind, r.fieldEvaluationTime())
	case "CREATEDATE":
		if strings.TrimSpace(run.Text) != "" {
			return ""
		}
		return formatFieldDateTime(run.FieldInstr, run.FieldKind, parseISODate(r.doc.CreatedAt))
	case "SAVEDATE":
		if strings.TrimSpace(run.Text) != "" {
			return ""
		}
		return formatFieldDateTime(run.FieldInstr, run.FieldKind, parseISODate(r.doc.ModifiedAt))
	case "PRINTDATE":
		if strings.TrimSpace(run.Text) != "" {
			return ""
		}
		return formatFieldDateTime(run.FieldInstr, run.FieldKind, parseISODate(r.doc.LastPrintedAt))
	case "SECTION":
		return formatIntegerFieldValue(r.sectionIndex+1, r.resolveFieldInstruction(run), "decimal")
	case "SECTIONPAGES":
		pages := r.sectionPageCounts[r.sectionIndex]
		if pages <= 0 {
			pages = 1
		}
		return formatIntegerFieldValue(pages, r.resolveFieldInstruction(run), "decimal")
	case "PAGEREF":
		if run.Anchor != "" {
			if refHasSwitch(run.FieldInstr, "p") {
				return r.pageRefRelativePosition(run.Anchor)
			}
			if page, ok := r.bookmarkPages[run.Anchor]; ok && page > 0 {
				// bookmarkPages stores physical page numbers (1-based); convert to logical.
				logical := r.logicalPageNumber(page - 1)
				format := ""
				if page-1 >= 0 && page-1 < len(r.pages) {
					format = r.pages[page-1].Section.PageNumberFormat
				}
				return formatIntegerFieldValue(logical, r.resolveFieldInstruction(run), format)
			}
		}
		return ""
	case "NOTEREF":
		if strings.TrimSpace(run.Text) != "" {
			return ""
		}
		bookmark := parseNoteRefBookmark(r.resolveFieldInstruction(run))
		if bookmark == "" {
			return ""
		}
		if target, ok := r.bookmarkNoteRefs[bookmark]; ok {
			return r.noteDisplayNumber(target.kind, target.noteID)
		}
		return ""
	case "REF":
		if run.Anchor != "" {
			cached := strings.TrimSpace(run.Text)
			num := ""
			if refHasSwitch(run.FieldInstr, "p") {
				num = r.refRelativePosition(run.Anchor)
			} else if refHasSwitch(run.FieldInstr, "w") {
				num = r.refFullContextNumber(run.Anchor)
			} else if refHasSwitch(run.FieldInstr, "n") {
				num = r.refLevelNumber(run.Anchor)
			} else if refHasSwitch(run.FieldInstr, "r") {
				num = r.refRelativeContextNumber(run.Anchor)
			} else {
				num = r.bookmarkText[run.Anchor]
			}
			// If there are no content-modifying switches (or \i requests keeping
			// old formatting) and cached text exists, preserve the cached result.
			preserveCached := refHasSwitch(run.FieldInstr, "i") ||
				(!refHasSwitch(run.FieldInstr, "t") && !refHasSwitch(run.FieldInstr, "v") && !refHasSwitch(run.FieldInstr, "s") &&
					parseRefDelimiter(run.FieldInstr) == "" && parseRefNumberFormat(run.FieldInstr) == "")
			if cached != "" && preserveCached {
				return ""
			}
			// When cached text exists but formatting switches are present,
			// use the cached text as the base value.
			if cached != "" && num == "" {
				num = cached
			}
			if refHasSwitch(run.FieldInstr, "t") || refHasSwitch(run.FieldInstr, "v") {
				num = stripNonNumberText(num)
			}
			if sep := parseRefDelimiter(run.FieldInstr); sep != "" {
				num = replaceNumberDelimiters(num, sep)
			}
			if refHasSwitch(run.FieldInstr, "s") {
				num = run.Anchor + " " + num
			}
			if numFormat := parseRefNumberFormat(run.FieldInstr); numFormat != "" {
				num = formatRefNumber(num, numFormat)
			}
			if cf := parseFieldCaseFormat(run.FieldInstr); cf != "" {
				num = applyFieldCaseFormat(num, cf)
			}
			return num
		}
		return ""
	case "ASK":
		if strings.TrimSpace(run.Text) != "" {
			return ""
		}
		if run.FieldArg != "" {
			return r.bookmarkText[run.FieldArg]
		}
		return ""
	case "AUTOTEXT":
		if strings.TrimSpace(run.Text) != "" {
			return ""
		}
		if run.FieldArg != "" {
			return "[" + strings.TrimSpace(run.FieldArg) + "]"
		}
		return ""
	case "SEQ":
		if strings.TrimSpace(run.Text) != "" {
			return ""
		}
		name := normalizeSequenceName(run.FieldArg)
		if name == "" {
			return ""
		}
		if r.seqCounters == nil {
			r.seqCounters = make(map[string]int)
		}
		control := r.seqControlForRun(run)
		counterKey := name
		if control.hasRestartHeading && r.currentParagraph != nil {
			hIdx := r.nearestPrecedingHeadingIdx(r.currentParagraph.Index, control.restartHeading)
			counterKey = fmt.Sprintf("%s#h=%d", name, hIdx)
		}
		if control.restartSection {
			counterKey = fmt.Sprintf("%s#s=%d", name, r.sectionIndex)
		}
		value := 0
		if control.hasRestart {
			r.seqCounters[counterKey] = control.restart
			value = control.restart
		} else if control.repeat {
			if current := r.seqCounters[counterKey]; current > 0 {
				value = current
			}
		} else {
			r.seqCounters[counterKey]++
			value = r.seqCounters[counterKey]
		}
		if value <= 0 {
			return ""
		}
		if control.hide {
			return ""
		}
		if control.totalCount {
			return r.formatSequenceDisplayValue(r.sequenceTotalCount(name), control, r.currentParagraph)
		}
		return r.formatSequenceDisplayValue(value, control, r.currentParagraph)
	case "AUTONUM":
		if strings.TrimSpace(run.Text) != "" {
			return ""
		}
		r.autonumCounter++
		return strconv.Itoa(r.autonumCounter)
	case "AUTONUMLGL":
		if strings.TrimSpace(run.Text) != "" {
			return ""
		}
		r.autonumCounter++
		return strconv.Itoa(r.autonumCounter) + "."
	case "AUTONUMOUT":
		if strings.TrimSpace(run.Text) != "" {
			return ""
		}
		r.autonumCounter++
		return intToRoman(r.autonumCounter) + "."
	case "EDITTIME":
		if strings.TrimSpace(run.Text) != "" {
			return ""
		}
		return strconv.Itoa(r.doc.TotalEditMinutes)
	case "TITLE":
		return r.doc.Title
	case "AUTHOR":
		// Preserve a cached field result when present; legacy fixtures store the
		// resolved author text in the field result run.
		if strings.TrimSpace(run.Text) != "" {
			return ""
		}
		return r.doc.Author
	case "SUBJECT":
		return r.doc.Subject
	case "DOCPROPERTY":
		return r.resolveDocProperty(run.FieldArg)
	case "DOCVARIABLE":
		return r.resolveDocVariable(run.FieldArg)
	case "USERADDRESS":
		if strings.TrimSpace(run.Text) != "" {
			return ""
		}
		return r.resolveUserAddress()
	case "USERINITIALS":
		if strings.TrimSpace(run.Text) != "" {
			return ""
		}
		return r.resolveUserInitials()
	case "USERNAME":
		if strings.TrimSpace(run.Text) != "" {
			return ""
		}
		if r.doc.UserName != "" {
			return r.doc.UserName
		}
		return r.doc.Author
	case "REVNUM":
		if strings.TrimSpace(run.Text) != "" {
			return ""
		}
		return r.doc.RevNum
	case "TEMPLATE":
		if strings.TrimSpace(run.Text) != "" {
			return ""
		}
		return r.doc.Template
	case "FILENAME":
		// The source filename is authoritative at render time. Cached field results
		// commonly retain the fixture's original name after a document is renamed.
		if r.doc.Filename != "" {
			return r.doc.Filename
		}
		if strings.TrimSpace(run.Text) != "" {
			return ""
		}
		return r.doc.Title
	case "NUMWORDS":
		return strconv.Itoa(r.countDocWords())
	case "NUMCHARS":
		return strconv.Itoa(r.countDocChars())
	case "STYLEREF":
		return r.resolveStyleRef(run.FieldArg)
	case "TOC":
		// A block-spanning TOC commonly carries an authored cached result across
		// many paragraphs. Recomputing a TOC for each cached result run duplicates
		// the entire table of contents at every entry. Preserve authored result text;
		// synthesize only for a semantic field carrier with no cached display text.
		if strings.TrimSpace(run.Text) != "" {
			return ""
		}
		return r.generateTOC(r.resolveFieldInstruction(run))
	case "INDEX":
		// INDEX has the same block-spanning cached-result semantics as TOC.
		if strings.TrimSpace(run.Text) != "" {
			return ""
		}
		return r.generateIndex(r.resolveFieldInstruction(run))
	case "IF", "COMPARE":
		if value, ok := r.evaluateIfFieldRun(run); ok {
			return value
		}
		if strings.TrimSpace(run.Text) != "" {
			return ""
		}
		return ""
	case "QUOTE":
		return evaluateQuoteField(r.resolveFieldInstruction(run))
	case "MACROBUTTON":
		if strings.TrimSpace(run.Text) != "" {
			return ""
		}
		return parseMacroButtonText(r.resolveFieldInstruction(run))
	case "PRIVATE":
		// PRIVATE is intentionally non-executing in HTML. Preserve any cached
		// display text so nested/composite fields can still render the visible
		// fallback that Word stored in the document.
		return run.Text
	case "FORMULA":
		return evaluateWordFormulaFull(r.resolveFieldInstruction(run), run.Text)
	case "MERGEFIELD":
		value := ""
		if run.FieldArg != "" {
			if v, ok := r.mergeDataValue(run.FieldArg); ok {
				value = v
			}
		}
		if value == "" && strings.TrimSpace(run.Text) != "" {
			return ""
		}
		if value == "" {
			if run.FieldArg != "" {
				return "«" + run.FieldArg + "»"
			}
			return ""
		}
		instr := run.FieldInstr
		if len(run.FieldInstrRuns) > 0 {
			if resolved := strings.TrimSpace(r.resolveFieldInstructionRuns(run.FieldInstrRuns)); resolved != "" {
				instr = resolved
			}
		}
		options := parseMergeFieldOptions(instr)
		value = formatMergeFieldValue(value, options)
		return options.beforeText + value + options.afterText
	case "MERGEREC":
		return strconv.Itoa(r.currentMergeRecord + 1)
	case "MERGESEQ":
		return strconv.Itoa(r.mergeSeqCount + 1)
	case "NEXT":
		r.currentMergeRecord++
		r.mergeSeqCount++
		return ""
	case "NEXTIF":
		if matched, ok := r.evaluateNextIfWithStatus(r.resolveFieldInstruction(run)); ok && matched {
			r.currentMergeRecord++
			r.mergeSeqCount++
		}
		return ""
	case "SKIPIF":
		if matched, ok := r.evaluateSkipIfWithStatus(r.resolveFieldInstruction(run)); ok && matched {
			r.currentMergeRecord++
		}
		return ""
	case "DATABASE":
		// If an external executor is provided and the field contains a connection
		// string and SQL query, attempt real execution; otherwise fall back to
		// cached text so the document does not lose visible content.
		if r.options.DatabaseExecutor != nil {
			if connStr, sqlQuery := parseDatabaseQuery(run.FieldInstr); connStr != "" && sqlQuery != "" {
				if result, err := r.options.DatabaseExecutor(connStr, sqlQuery); err == nil {
					return result
				}
			}
		}
		return strings.TrimSpace(run.Text)
	case "LISTNUM":
		if strings.TrimSpace(run.Text) != "" {
			return ""
		}
		if run.FieldArg != "" {
			return "[" + run.FieldArg + "]"
		}
		return "[List Number]"
	case "INCLUDETEXT":
		if strings.TrimSpace(run.Text) != "" {
			return ""
		}
		if run.FieldArg != "" {
			return "[" + run.FieldArg + "]"
		}
		return "[Include Text]"
	case "INCLUDEPICTURE":
		if strings.TrimSpace(run.Text) != "" {
			return ""
		}
		if run.FieldArg != "" {
			return "[" + run.FieldArg + "]"
		}
		return "[Include Picture]"
	case "EQ":
		if strings.TrimSpace(run.Text) != "" {
			return ""
		}
		return evaluateWordFormulaFull(run.FieldArg, run.Text)
	case "SYMBOL":
		if strings.TrimSpace(run.Text) != "" {
			return ""
		}
		return parseSymbolField(run.FieldArg)
	case "TA":
		if strings.TrimSpace(run.Text) != "" {
			return ""
		}
		return ""
	case "TOA":
		if strings.TrimSpace(run.Text) != "" {
			return ""
		}
		if result := r.generateTOA(run.FieldArg); result != "" {
			return result
		}
		return "[Table of Authorities]"
	case "BIDIOUTLINE":
		if strings.TrimSpace(run.Text) != "" {
			return ""
		}
		return ""
	case "CITATION":
		if strings.TrimSpace(run.Text) != "" {
			return ""
		}
		if run.FieldArg != "" {
			return "[" + run.FieldArg + "]"
		}
		return "[Citation]"
	case "BIBLIOGRAPHY":
		if strings.TrimSpace(run.Text) != "" {
			return ""
		}
		return r.generateBibliography(run.FieldArg)
	case "GOTOBUTTON":
		if strings.TrimSpace(run.Text) != "" {
			return ""
		}
		bookmark, display := parseGotoButton(run.FieldArg)
		if bookmark != "" {
			return `<a href="#` + html.EscapeString(bookmark) + `" class="word-field word-field-gotobutton">` + html.EscapeString(display) + `</a>`
		}
		if run.FieldArg != "" {
			return "[" + run.FieldArg + "]"
		}
		return "[Go To]"
	case "GREETINGLINE":
		if strings.TrimSpace(run.Text) != "" {
			return ""
		}
		return r.resolveGreetingLine(run.FieldArg)
	case "ADDRESSBLOCK":
		if strings.TrimSpace(run.Text) != "" {
			return ""
		}
		return r.resolveAddressBlock(run.FieldArg)
	case "BARCODE":
		if strings.TrimSpace(run.Text) != "" {
			return ""
		}
		if run.FieldArg != "" {
			return "[Barcode: " + run.FieldArg + "]"
		}
		return "[Barcode]"
	case "RD":
		if strings.TrimSpace(run.Text) != "" {
			return ""
		}
		return ""
	case "IMPORT":
		if strings.TrimSpace(run.Text) != "" {
			return ""
		}
		if run.FieldArg != "" {
			return "[Import: " + run.FieldArg + "]"
		}
		return "[Import]"
	case "SAVEPICTURE":
		if strings.TrimSpace(run.Text) != "" {
			return ""
		}
		if run.FieldArg != "" {
			return "[Save Picture: " + run.FieldArg + "]"
		}
		return "[Save Picture]"
	case "INFO":
		if strings.TrimSpace(run.Text) != "" {
			return ""
		}
		return r.resolveDocProperty(run.FieldArg)
	case "SET":
		instr := run.FieldInstr
		if len(run.FieldInstrRuns) > 0 {
			if resolved := strings.TrimSpace(r.resolveFieldInstructionRuns(run.FieldInstrRuns)); resolved != "" {
				instr = resolved
			}
		}
		bookmark, value := parseSetField(instr)
		if bookmark != "" {
			r.bookmarkText[bookmark] = value
		}
		return ""
	case "ADVANCE":
		// ADVANCE produces no visible text; positional side-effects are handled
		// in renderRuns via applyAdvanceField.
		return ""
	case "EMBED":
		if strings.TrimSpace(run.Text) != "" {
			return ""
		}
		if run.FieldArg != "" {
			class := parseEmbedClassName(run.FieldArg)
			if class != "" {
				return "[" + class + "]"
			}
		}
		return "[Embedded Object]"
	case "LINK":
		if strings.TrimSpace(run.Text) != "" {
			return ""
		}
		if run.FieldArg != "" {
			url := parseLinkURL(run.FieldArg)
			if url != "" {
				return url
			}
		}
		return "[Linked Object]"
	default:
		// For other fields, use the original text
		return ""
	}
}

func (r *htmlRenderer) synthesizeFieldRun(field resolve.Field) resolve.Run {
	var format resolve.RunFormat
	if len(field.Children) > 0 {
		format = field.Children[0].Format
	}
	return resolve.Run{
		FieldKind:      field.Kind,
		FieldInstr:     field.Instr,
		FieldArg:       field.Arg,
		FieldInstrRuns: append([]resolve.Run(nil), field.InstrRuns...),
		Anchor:         field.Anchor,
		Target:         field.Target,
		Text:           r.aggregateFieldRunText(field.Children),
		Format:         format,
	}
}

func (r *htmlRenderer) aggregateFieldRunText(runs []resolve.Run) string {
	var sb strings.Builder
	for _, run := range runs {
		if len(run.Fields) > 0 {
			for _, field := range run.Fields {
				synthRun := r.synthesizeFieldRun(field)
				if calculated := r.calculateFieldValue(synthRun); calculated != "" {
					sb.WriteString(calculated)
				} else {
					sb.WriteString(r.aggregateFieldRunText(field.Children))
				}
			}
			continue
		}
		sb.WriteString(run.Text)
	}
	return sb.String()
}

func (r *htmlRenderer) resolveFieldInstruction(run resolve.Run) string {
	if len(run.FieldInstrRuns) > 0 {
		if instr := strings.TrimSpace(r.resolveFieldInstructionRuns(run.FieldInstrRuns)); instr != "" {
			return instr
		}
	}
	if instr := strings.TrimSpace(run.FieldArg); instr != "" {
		return instr
	}
	return strings.TrimSpace(run.FieldInstr)
}

func (r *htmlRenderer) applyAdvanceField(run resolve.Run) {
	offsets := inlineposition.ParseAdvance(r.resolveFieldInstruction(run))
	r.advanceState.InlineXPt = r.tabEstimatedX
	r.advanceState.Apply(offsets)
	r.tabEstimatedX = r.advanceState.InlineXPt

	// Emit a zero-size semantic marker for debugging/accessibility.
	fmt.Fprintf(&r.buf, `<span class="word-field word-field-advance" aria-hidden="true"></span>`)
}

func (r *htmlRenderer) openAdvanceOffsetWrapper() {
	var styles []string
	if r.advanceState.HorizontalPt != 0 {
		styles = append(styles, fmt.Sprintf("margin-left:%.2fpt", r.advanceState.HorizontalPt))
	}
	if r.advanceState.VerticalPt != 0 {
		styles = append(styles, fmt.Sprintf("transform:translateY(%.2fpt)", -r.advanceState.VerticalPt))
	}
	fmt.Fprintf(&r.buf, `<span style=%q>`, strings.Join(styles, ";"))
}

func (r *htmlRenderer) closeAdvanceOffsetWrapper() {
	r.buf.WriteString("</span>")
}

func normalizeSequenceName(name string) string {
	return strings.ToUpper(strings.TrimSpace(name))
}

func parseNoteRefBookmark(instr string) string {
	instr = strings.TrimSpace(instr)
	upper := strings.ToUpper(instr)
	if !strings.HasPrefix(upper, "NOTEREF") {
		return ""
	}
	rest := strings.TrimSpace(instr[len("NOTEREF"):])
	if rest == "" {
		return ""
	}
	if strings.HasPrefix(rest, `"`) {
		value, _ := extractFieldString(rest)
		return strings.TrimSpace(value)
	}
	fields := strings.Fields(rest)
	if len(fields) == 0 || strings.HasPrefix(fields[0], `\`) {
		return ""
	}
	return fields[0]
}

type seqControl struct {
	repeat            bool
	restart           int
	hasRestart        bool
	hide              bool
	format            string
	caseFormat        string // upper, lower, caps, firstcap
	restartHeading    int    // heading level for \s n restart (0 = disabled)
	hasRestartHeading bool
	restartSection    bool // \s without number — restart at section boundaries
	totalCount        bool // \n — display total number of items
	mergeFormat       bool // \* merge-format — adopt surrounding paragraph numbering format
	charFormat        bool // \* char-format — apply previous run's character formatting
}

func parseSeqControl(instr string) seqControl {
	var control seqControl
	if instr == "" {
		return control
	}
	upper := strings.ToUpper(strings.TrimSpace(instr))
	if !strings.HasPrefix(upper, "SEQ") {
		return control
	}
	rest := strings.TrimSpace(instr[len("SEQ"):])
	if rest == "" {
		return control
	}
	if strings.HasPrefix(rest, `"`) {
		_, rest = extractFieldString(rest)
	} else {
		fields := strings.Fields(rest)
		if len(fields) > 0 && !strings.HasPrefix(fields[0], `\`) {
			rest = strings.TrimSpace(rest[len(fields[0]):])
		}
	}
	for {
		rest = strings.TrimSpace(rest)
		if !strings.HasPrefix(rest, `\`) {
			break
		}
		rest = strings.TrimSpace(rest[1:])
		switchName, tail := extractFieldToken(rest)
		if switchName == "" {
			break
		}
		switch strings.ToUpper(switchName) {
		case "C":
			control.repeat = true
			rest = tail
		case "R":
			value, next := extractIfOperand(tail)
			if n, err := strconv.Atoi(strings.TrimSpace(value)); err == nil {
				control.restart = n
				control.hasRestart = true
			}
			rest = next
		case "H":
			control.hide = true
			rest = tail
		case "M":
			control.mergeFormat = true
			rest = tail
		case "N":
			control.totalCount = true
			rest = tail
		case "S":
			value, next := extractIfOperand(tail)
			if n, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && n >= 1 && n <= 6 {
				control.restartHeading = n
				control.hasRestartHeading = true
				rest = next
			} else {
				// \s without a number means restart at section boundaries.
				control.restartSection = true
				if value != "" && strings.HasPrefix(value, `\`) {
					// Another switch immediately follows \s; put it back for processing.
					if next != "" {
						rest = value + " " + next
					} else {
						rest = value
					}
				} else {
					rest = next
				}
			}
		case "*":
			token, next := extractFieldToken(tail)
			switch strings.ToLower(token) {
			case "upper", "lower", "caps", "firstcap":
				control.caseFormat = token
			case "mergeformat":
				control.mergeFormat = true
			case "charformat":
				control.charFormat = true
			default:
				control.format = token
			}
			rest = next
		default:
			rest = tail
			if strings.HasPrefix(rest, `"`) {
				_, rest = extractFieldString(rest)
			} else {
				parts := strings.Fields(rest)
				if len(parts) > 0 && !strings.HasPrefix(parts[0], `\`) {
					rest = strings.TrimSpace(rest[len(parts[0]):])
				}
			}
		}
	}
	return control
}

func extractFieldToken(s string) (token, rest string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", ""
	}
	if strings.HasPrefix(s, `"`) {
		return extractFieldString(s)
	}
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return "", ""
	}
	token = fields[0]
	return token, strings.TrimSpace(s[len(token):])
}

func parseFieldFloat(token string) float64 {
	token = strings.TrimSpace(token)
	token = strings.TrimSuffix(token, "pt")
	token = strings.TrimSuffix(token, "PT")
	value, _ := strconv.ParseFloat(token, 64)
	return value
}

func formatSequenceValue(n int, format, caseFormat string) string {
	var s string
	switch format {
	case "roman":
		s = strings.ToLower(intToRoman(n))
	case "ROMAN":
		s = strings.ToUpper(intToRoman(n))
	case "alphabetic":
		s = strings.ToLower(intToLetters(n))
	case "ALPHABETIC":
		s = strings.ToUpper(intToLetters(n))
	case "CardText", "CARDTEXT", "cardtext":
		s = numberToWords(n)
	case "OrdText", "ORDTEXT", "ordtext":
		s = numberToOrdinalWords(n)
	case "Ordinal", "ORDINAL", "ordinal":
		s = ordinalNumber(n)
	case "hex", "HEX", "Hex":
		s = fmt.Sprintf("%x", n)
	case "arabicDash", "ARABICDASH", "ArabicDash":
		s = fmt.Sprintf("- %d -", n)
	default:
		s = strconv.Itoa(n)
	}
	return applyFieldCaseFormat(s, caseFormat)
}

func applyFieldCaseFormat(s, caseFormat string) string {
	switch strings.ToLower(caseFormat) {
	case "upper":
		return strings.ToUpper(s)
	case "lower":
		return strings.ToLower(s)
	case "caps":
		return strings.ToTitle(s)
	case "firstcap":
		if s == "" {
			return ""
		}
		return strings.ToUpper(s[:1]) + s[1:]
	default:
		return s
	}
}

func formatNoteNumber(n int, format string) string {
	switch format {
	case "upperRoman":
		return strings.ToUpper(intToRoman(n))
	case "lowerRoman", "roman":
		return strings.ToLower(intToRoman(n))
	case "upperLetter":
		return strings.ToUpper(intToLetters(n))
	case "lowerLetter":
		return strings.ToLower(intToLetters(n))
	case "ordinal":
		return ordinalNumber(n)
	default:
		return strconv.Itoa(n)
	}
}

func intToRoman(n int) string {
	if n <= 0 {
		return ""
	}
	values := []struct {
		value int
		sym   string
	}{
		{1000, "M"}, {900, "CM"}, {500, "D"}, {400, "CD"},
		{100, "C"}, {90, "XC"}, {50, "L"}, {40, "XL"},
		{10, "X"}, {9, "IX"}, {5, "V"}, {4, "IV"}, {1, "I"},
	}
	var sb strings.Builder
	for _, entry := range values {
		for n >= entry.value {
			sb.WriteString(entry.sym)
			n -= entry.value
		}
	}
	return sb.String()
}

func intToLetters(n int) string {
	if n <= 0 {
		return ""
	}
	var out []byte
	for n > 0 {
		n--
		out = append([]byte{byte('A' + (n % 26))}, out...)
		n /= 26
	}
	return string(out)
}

func ordinalNumber(n int) string {
	if n%100 >= 11 && n%100 <= 13 {
		return strconv.Itoa(n) + "th"
	}
	switch n % 10 {
	case 1:
		return strconv.Itoa(n) + "st"
	case 2:
		return strconv.Itoa(n) + "nd"
	case 3:
		return strconv.Itoa(n) + "rd"
	default:
		return strconv.Itoa(n) + "th"
	}
}

func numberToWords(n int) string {
	if n < 0 {
		return "minus " + numberToWords(-n)
	}
	if n < 20 {
		return smallNumberWord(n)
	}
	if n < 100 {
		tens := []string{"", "", "twenty", "thirty", "forty", "fifty", "sixty", "seventy", "eighty", "ninety"}
		if n%10 == 0 {
			return tens[n/10]
		}
		return tens[n/10] + "-" + smallNumberWord(n%10)
	}
	if n < 1000 {
		if n%100 == 0 {
			return smallNumberWord(n/100) + " hundred"
		}
		return smallNumberWord(n/100) + " hundred " + numberToWords(n%100)
	}
	scales := []struct {
		value int
		word  string
	}{
		{1_000_000_000, "billion"},
		{1_000_000, "million"},
		{1000, "thousand"},
	}
	for _, scale := range scales {
		if n >= scale.value {
			if n%scale.value == 0 {
				return numberToWords(n/scale.value) + " " + scale.word
			}
			return numberToWords(n/scale.value) + " " + scale.word + " " + numberToWords(n%scale.value)
		}
	}
	return strconv.Itoa(n)
}

func numberToOrdinalWords(n int) string {
	if n < 0 {
		return "minus " + numberToOrdinalWords(-n)
	}
	ordinals := map[string]string{
		"zero":      "zeroth",
		"one":       "first",
		"two":       "second",
		"three":     "third",
		"four":      "fourth",
		"five":      "fifth",
		"six":       "sixth",
		"seven":     "seventh",
		"eight":     "eighth",
		"nine":      "ninth",
		"ten":       "tenth",
		"eleven":    "eleventh",
		"twelve":    "twelfth",
		"thirteen":  "thirteenth",
		"fourteen":  "fourteenth",
		"fifteen":   "fifteenth",
		"sixteen":   "sixteenth",
		"seventeen": "seventeenth",
		"eighteen":  "eighteenth",
		"nineteen":  "nineteenth",
		"twenty":    "twentieth",
		"thirty":    "thirtieth",
		"forty":     "fortieth",
		"fifty":     "fiftieth",
		"sixty":     "sixtieth",
		"seventy":   "seventieth",
		"eighty":    "eightieth",
		"ninety":    "ninetieth",
		"hundred":   "hundredth",
		"thousand":  "thousandth",
		"million":   "millionth",
		"billion":   "billionth",
	}
	words := numberToWords(n)
	parts := strings.Fields(words)
	if len(parts) == 0 {
		return words
	}
	last := parts[len(parts)-1]
	suffix := ""
	if strings.Contains(last, "-") {
		chunks := strings.Split(last, "-")
		last = chunks[len(chunks)-1]
		chunks[len(chunks)-1] = ordinals[last]
		parts[len(parts)-1] = strings.Join(chunks, "-")
		return strings.Join(parts, " ")
	}
	if replacement, ok := ordinals[last]; ok {
		suffix = replacement
	} else {
		suffix = last + "th"
	}
	parts[len(parts)-1] = suffix
	return strings.Join(parts, " ")
}

func smallNumberWord(n int) string {
	words := []string{
		"zero", "one", "two", "three", "four", "five", "six", "seven", "eight", "nine",
		"ten", "eleven", "twelve", "thirteen", "fourteen", "fifteen", "sixteen", "seventeen", "eighteen", "nineteen",
	}
	if n >= 0 && n < len(words) {
		return words[n]
	}
	return strconv.Itoa(n)
}

func (r *htmlRenderer) noteDisplayNumber(kind string, noteID int) string {
	switch kind {
	case "footnote":
		if value, ok := r.footnoteNumbers[noteID]; ok && value != "" {
			return value
		}
	case "endnote":
		if value, ok := r.endnoteNumbers[noteID]; ok && value != "" {
			return value
		}
	}
	return strconv.Itoa(noteID)
}

func (r *htmlRenderer) resolveFieldInstructionRuns(runs []resolve.Run) string {
	var sb strings.Builder
	for _, run := range runs {
		if len(run.Fields) > 0 {
			for _, field := range run.Fields {
				synthRun := r.synthesizeFieldRun(field)
				value := r.calculateFieldValue(synthRun)
				if value == "" {
					value = r.aggregateFieldRunText(field.Children)
				}
				// Quote values that contain whitespace so subsequent space-delimited
				// parsing (e.g. IF operands) treats them as single tokens.
				if strings.ContainsAny(value, " \t\n\r") {
					sb.WriteString(`"`)
					sb.WriteString(value)
					sb.WriteString(`"`)
				} else {
					sb.WriteString(value)
				}
			}
			continue
		}
		sb.WriteString(run.Text)
	}
	return sb.String()
}

// extractFieldString reads a double-quoted string from the start of s.
func extractFieldString(s string) (value, rest string) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, `"`) {
		return "", s
	}
	s = s[1:] // skip opening quote
	i := strings.Index(s, `"`)
	if i < 0 {
		return s, ""
	}
	return s[:i], strings.TrimSpace(s[i+1:])
}

// parseEmbedClassName extracts the OLE class name from an EMBED field instruction.
// Format: EMBED ClassName [\switches]
func parseEmbedClassName(instr string) string {
	rest := strings.TrimSpace(instr[len("EMBED"):])
	fields := strings.Fields(rest)
	if len(fields) > 0 && !strings.HasPrefix(fields[0], `\`) {
		return fields[0]
	}
	return ""
}

// parseLinkURL extracts the file path or URL from a LINK field instruction.
// Format: LINK ClassName "FilePath" "Item" [\switches]
func parseLinkURL(instr string) string {
	rest := strings.TrimSpace(instr[len("LINK"):])
	// Skip class name.
	fields := strings.Fields(rest)
	if len(fields) > 0 && !strings.HasPrefix(fields[0], `\`) {
		rest = strings.TrimSpace(rest[len(fields[0]):])
	}
	if rest == "" {
		return ""
	}
	if strings.HasPrefix(rest, `"`) {
		url, _ := extractFieldString(rest)
		return url
	}
	return ""
}

// parseSymbolField parses a SYMBOL field instruction and returns the character.
// Format: SYMBOL charNum [\f "fontName"] [\s size] [\h] [\a]
func parseSymbolField(arg string) string {
	if arg == "" {
		return ""
	}
	// The first token is the character number (decimal or hex).
	fields := strings.Fields(arg)
	if len(fields) == 0 {
		return ""
	}
	charStr := fields[0]
	var codePoint int
	if strings.HasPrefix(charStr, "0x") || strings.HasPrefix(charStr, "0X") {
		// Hex
		if n, err := strconv.ParseInt(charStr[2:], 16, 32); err == nil {
			codePoint = int(n)
		}
	} else {
		if n, err := strconv.Atoi(charStr); err == nil {
			codePoint = n
		}
	}
	if codePoint <= 0 || codePoint > 0x10FFFF {
		return ""
	}
	return string(rune(codePoint))
}

func (r *htmlRenderer) resolveDocProperty(name string) string {
	switch strings.ToUpper(strings.TrimSpace(name)) {
	case "TITLE":
		return r.doc.Title
	case "AUTHOR":
		return r.doc.Author
	case "SUBJECT":
		return r.doc.Subject
	case "DESCRIPTION", "COMMENTS":
		return r.doc.Description
	case "KEYWORDS":
		return r.doc.Keywords
	case "LASTMODIFIEDBY":
		return r.doc.LastModifiedBy
	case "COMPANY":
		return r.doc.Company
	case "MANAGER":
		return r.doc.Manager
	case "APPLICATION":
		return r.doc.Application
	case "APPVERSION":
		return r.doc.AppVersion
	case "TEMPLATE":
		return r.doc.Template
	default:
		if value, ok := r.lookupCustomProperty(name); ok {
			return value
		}
		return ""
	}
}

func (r *htmlRenderer) lookupCustomProperty(name string) (string, bool) {
	if r == nil || r.doc == nil || r.doc.CustomProperties == nil {
		return "", false
	}
	name = strings.TrimSpace(name)
	if value, ok := r.doc.CustomProperties[name]; ok {
		return value, true
	}
	for key, value := range r.doc.CustomProperties {
		if strings.EqualFold(strings.TrimSpace(key), name) {
			return value, true
		}
	}
	return "", false
}

func (r *htmlRenderer) lookupDocumentVariable(name string) (string, bool) {
	if r == nil || r.doc == nil || r.doc.DocumentVariables == nil {
		return "", false
	}
	name = strings.TrimSpace(name)
	if value, ok := r.doc.DocumentVariables[name]; ok {
		return value, true
	}
	for key, value := range r.doc.DocumentVariables {
		if strings.EqualFold(strings.TrimSpace(key), name) {
			return value, true
		}
	}
	return "", false
}

func (r *htmlRenderer) lookupDocPropertyValue(name string) (string, bool) {
	if r == nil || r.doc == nil {
		return "", false
	}
	switch strings.ToUpper(strings.TrimSpace(name)) {
	case "TITLE":
		return r.doc.Title, true
	case "AUTHOR":
		return r.doc.Author, true
	case "SUBJECT":
		return r.doc.Subject, true
	case "DESCRIPTION", "COMMENTS":
		return r.doc.Description, true
	case "KEYWORDS":
		return r.doc.Keywords, true
	case "LASTMODIFIEDBY":
		return r.doc.LastModifiedBy, true
	case "COMPANY":
		return r.doc.Company, true
	case "MANAGER":
		return r.doc.Manager, true
	case "APPLICATION":
		return r.doc.Application, true
	case "APPVERSION":
		return r.doc.AppVersion, true
	case "TEMPLATE":
		return r.doc.Template, true
	}
	return r.lookupCustomProperty(name)
}

func (r *htmlRenderer) resolveDocVariable(name string) string {
	if value, ok := r.lookupDocumentVariable(name); ok {
		return value
	}
	return ""
}

func (r *htmlRenderer) resolveUserAddress() string {
	for _, key := range []string{"UserAddress", "Address", "MailingAddress"} {
		if value, ok := r.lookupCustomProperty(key); ok {
			return value
		}
	}
	return ""
}

func (r *htmlRenderer) resolveUserInitials() string {
	if r == nil || r.doc == nil {
		return ""
	}
	for _, key := range []string{"UserInitials", "Initials"} {
		if value, ok := r.lookupCustomProperty(key); ok {
			if trimmed := strings.TrimSpace(value); trimmed != "" {
				return trimmed
			}
		}
	}
	if initials := deriveInitials(r.doc.LastModifiedBy); initials != "" {
		return initials
	}
	return deriveInitials(r.doc.Author)
}

func deriveInitials(name string) string {
	fields := strings.Fields(strings.TrimSpace(name))
	if len(fields) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, field := range fields {
		for _, ch := range field {
			if unicode.IsLetter(ch) || unicode.IsDigit(ch) {
				sb.WriteRune(unicode.ToUpper(ch))
				break
			}
		}
	}
	return sb.String()
}

func parseMacroButtonText(instr string) string {
	if instr == "" {
		return ""
	}
	upper := strings.ToUpper(strings.TrimSpace(instr))
	if !strings.HasPrefix(upper, "MACROBUTTON") {
		return ""
	}
	rest := strings.TrimSpace(instr[len("MACROBUTTON"):])
	if rest == "" {
		return ""
	}
	if strings.HasPrefix(rest, `"`) {
		_, rest = extractFieldString(rest)
	} else {
		fields := strings.Fields(rest)
		if len(fields) == 0 {
			return ""
		}
		rest = strings.TrimSpace(rest[len(fields[0]):])
	}
	return strings.TrimSpace(rest)
}

// refHasSwitch reports whether a field instruction contains the given switch
// (e.g. "p", "h", "n"). It looks for a backslash followed by the switch letter.
func refHasSwitch(instr, switchName string) bool {
	upper := strings.ToUpper(instr)
	return strings.Contains(upper, `\`+strings.ToUpper(switchName))
}

// refRelativePosition returns "above" or "below" depending on whether the
// bookmarked paragraph is before or after the current paragraph in document
// order. Falls back to the bookmark text if paragraph indices are unknown.
func (r *htmlRenderer) refRelativePosition(anchor string) string {
	if r.currentParagraph == nil {
		return r.bookmarkText[anchor]
	}
	bookmarkParaIdx, ok := r.bookmarkParaIndex[anchor]
	if !ok {
		return r.bookmarkText[anchor]
	}
	currentPos, ok := r.paragraphSeqByIdx[r.currentParagraph.Index]
	if !ok {
		return r.bookmarkText[anchor]
	}
	bookmarkPos, ok := r.paragraphSeqByIdx[bookmarkParaIdx]
	if !ok {
		return r.bookmarkText[anchor]
	}
	if bookmarkPos < currentPos {
		return "above"
	}
	if bookmarkPos > currentPos {
		return "below"
	}
	return r.bookmarkText[anchor]
}

// refFullContextNumber returns the full paragraph number (e.g. "1.2.3") for the
// bookmarked paragraph. Falls back to bookmark text if no number is available.
func (r *htmlRenderer) refFullContextNumber(anchor string) string {
	idx, ok := r.bookmarkParaIndex[anchor]
	if !ok {
		return r.bookmarkText[anchor]
	}
	if num, ok := r.paragraphNumbers[idx]; ok && num != "" {
		return num
	}
	return r.bookmarkText[anchor]
}

// refLevelNumber returns the current-level paragraph number (e.g. "3") for the
// bookmarked paragraph. Falls back to bookmark text if no number is available.
func (r *htmlRenderer) refLevelNumber(anchor string) string {
	idx, ok := r.bookmarkParaIndex[anchor]
	if !ok {
		return r.bookmarkText[anchor]
	}
	if num, ok := r.paragraphLevelNum[idx]; ok && num != "" {
		return num
	}
	return r.bookmarkText[anchor]
}

// refRelativeContextNumber returns the paragraph number of the bookmarked
// paragraph relative to the current paragraph. Only the differing suffix of
// the number is shown. Falls back to bookmark text if unavailable.
func (r *htmlRenderer) refRelativeContextNumber(anchor string) string {
	if r.currentParagraph == nil {
		return r.bookmarkText[anchor]
	}
	bookmarkIdx, ok := r.bookmarkParaIndex[anchor]
	if !ok {
		return r.bookmarkText[anchor]
	}
	bookmarkNum, ok := r.paragraphNumbers[bookmarkIdx]
	if !ok || bookmarkNum == "" {
		return r.bookmarkText[anchor]
	}
	currentNum, ok := r.paragraphNumbers[r.currentParagraph.Index]
	if !ok || currentNum == "" {
		return bookmarkNum
	}
	// Split by common separators: period, dash, parenthesis.
	sepRe := regexp.MustCompile(`[.\-()]+`)
	bookmarkParts := sepRe.Split(bookmarkNum, -1)
	currentParts := sepRe.Split(currentNum, -1)
	// Find first differing level.
	diffAt := 0
	for diffAt < len(bookmarkParts) && diffAt < len(currentParts) {
		if bookmarkParts[diffAt] != currentParts[diffAt] {
			break
		}
		diffAt++
	}
	if diffAt >= len(bookmarkParts) {
		return bookmarkNum
	}
	// Reconstruct the suffix using the original separators.
	result := bookmarkNum
	for i := 0; i < diffAt && i < len(bookmarkParts); i++ {
		result = strings.TrimPrefix(result, bookmarkParts[i])
		result = strings.TrimLeft(result, ".-()")
	}
	return strings.TrimLeft(result, ".-()")
}

// pageRefRelativePosition returns "above" or "below" depending on whether the
// bookmarked page is before or after the current page. Falls back to the page
// number if the bookmark page is unknown.
func (r *htmlRenderer) pageRefRelativePosition(anchor string) string {
	bookmarkPage, ok := r.bookmarkPages[anchor]
	if !ok || bookmarkPage <= 0 {
		return ""
	}
	currentPage := r.pageIndex + 1
	if bookmarkPage < currentPage {
		return "above"
	}
	if bookmarkPage > currentPage {
		return "below"
	}
	return ""
}

// parseSetField extracts the bookmark name and value from a SET field instruction.
// Format: SET bookmark "value" or SET bookmark value
func parseSetField(instr string) (bookmark, value string) {
	instr = strings.TrimSpace(instr)
	upper := strings.ToUpper(instr)
	if !strings.HasPrefix(upper, "SET ") {
		return "", ""
	}
	rest := strings.TrimSpace(instr[4:])
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return "", ""
	}
	bookmark = fields[0]
	rest = strings.TrimSpace(rest[len(bookmark):])
	if strings.HasPrefix(rest, `"`) {
		value, _ = extractFieldString(rest)
	} else {
		value = strings.TrimSpace(rest)
	}
	return bookmark, value
}

// parseGotoButton extracts the bookmark name and display text from a
// GOTOBUTTON field instruction.
func parseGotoButton(instr string) (bookmark, display string) {
	instr = strings.TrimSpace(instr)
	upper := strings.ToUpper(instr)
	if !strings.HasPrefix(upper, "GOTOBUTTON") {
		return "", ""
	}
	rest := strings.TrimSpace(instr[len("GOTOBUTTON"):])
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return "", ""
	}
	bookmark = fields[0]
	rest = strings.TrimSpace(rest[len(bookmark):])
	if strings.HasPrefix(rest, `"`) {
		display, _ = extractFieldString(rest)
	} else {
		display = strings.TrimSpace(rest)
	}
	if display == "" {
		display = "Go to " + bookmark
	}
	return bookmark, display
}

// greetingLineOptions holds parsed switches from a GREETINGLINE instruction.
type greetingLineOptions struct {
	errorGreeting string // \e switch
}

func parseGreetingLineOptions(instr string) greetingLineOptions {
	var opts greetingLineOptions
	upper := strings.ToUpper(instr)
	if !strings.HasPrefix(upper, "GREETINGLINE") {
		return opts
	}
	rest := strings.TrimSpace(instr[len("GREETINGLINE"):])
	for strings.HasPrefix(rest, `\`) {
		rest = strings.TrimSpace(rest[1:])
		switchName, tail := extractFieldToken(rest)
		if switchName == "" {
			break
		}
		upperSwitch := strings.ToUpper(switchName)
		if upperSwitch == "E" {
			value, next := extractIfOperand(tail)
			if v := strings.TrimSpace(value); v != "" {
				opts.errorGreeting = v
			}
			rest = next
			continue
		}
		rest = tail
	}
	return opts
}

// resolveGreetingLine composes a mail-merge greeting from merge data.
// It looks for FirstName, LastName, Title, and Company fields.
func (r *htmlRenderer) resolveGreetingLine(instr string) string {
	opts := parseGreetingLineOptions(instr)
	firstName, _ := r.mergeDataValue("FirstName")
	lastName, _ := r.mergeDataValue("LastName")
	title, _ := r.mergeDataValue("Title")
	if firstName == "" && lastName == "" {
		if title != "" {
			return "Dear " + title + ":"
		}
		if opts.errorGreeting != "" {
			return opts.errorGreeting
		}
		return "Dear Sir or Madam:"
	}
	if title != "" {
		return "Dear " + title + " " + firstName + " " + lastName + ":"
	}
	return "Dear " + firstName + " " + lastName + ":"
}

// addressBlockOptions holds parsed switches from an ADDRESSBLOCK instruction.
type addressBlockOptions struct {
	includeCountry bool // \c switch (1=include, 2=exclude)
	countryFormat  int  // \c value
}

func parseAddressBlockOptions(instr string) addressBlockOptions {
	var opts addressBlockOptions
	opts.includeCountry = true
	upper := strings.ToUpper(instr)
	if !strings.HasPrefix(upper, "ADDRESSBLOCK") {
		return opts
	}
	rest := strings.TrimSpace(instr[len("ADDRESSBLOCK"):])
	for strings.HasPrefix(rest, `\`) {
		rest = strings.TrimSpace(rest[1:])
		switchName, tail := extractFieldToken(rest)
		if switchName == "" {
			break
		}
		upperSwitch := strings.ToUpper(switchName)
		if upperSwitch == "C" {
			value, next := extractIfOperand(tail)
			if n, err := strconv.Atoi(strings.TrimSpace(value)); err == nil {
				opts.countryFormat = n
				opts.includeCountry = n != 2
			}
			rest = next
			continue
		}
		rest = tail
	}
	return opts
}

// resolveAddressBlock composes a mail-merge address from merge data.
// It looks for standard address fields and joins them with newlines.
func (r *htmlRenderer) resolveAddressBlock(instr string) string {
	opts := parseAddressBlockOptions(instr)
	lines := []string{}
	for _, key := range []string{"Company", "Address1", "Address2", "City", "State", "ZIP"} {
		if v, ok := r.mergeDataValue(key); ok && strings.TrimSpace(v) != "" {
			lines = append(lines, v)
		}
	}
	if opts.includeCountry {
		if v, ok := r.mergeDataValue("Country"); ok && strings.TrimSpace(v) != "" {
			lines = append(lines, v)
		}
	}
	if len(lines) == 0 {
		return "[Address Block]"
	}
	return strings.Join(lines, "\n")
}

type mergeFieldOptions struct {
	beforeText   string
	afterText    string
	numberFormat string
	dateFormat   string
	caseFormat   string
}

// parseMergeFieldSwitches extracts the \b (before) and \f (after) text switches
// from a MERGEFIELD instruction. Only non-empty switches are returned.
func parseMergeFieldSwitches(instr string) (beforeText, afterText string) {
	options := parseMergeFieldOptions(instr)
	return options.beforeText, options.afterText
}

func parseMergeFieldOptions(instr string) mergeFieldOptions {
	var options mergeFieldOptions
	instr = strings.TrimSpace(instr)
	upper := strings.ToUpper(instr)
	if !strings.HasPrefix(upper, "MERGEFIELD") {
		return options
	}
	rest := strings.TrimSpace(instr[len("MERGEFIELD"):])
	// Skip the field name argument.
	if strings.HasPrefix(rest, `"`) {
		_, rest = extractFieldString(rest)
	} else {
		fields := strings.Fields(rest)
		if len(fields) > 0 && !strings.HasPrefix(fields[0], `\`) {
			rest = strings.TrimSpace(rest[len(fields[0]):])
		}
	}
	for strings.HasPrefix(rest, `\`) {
		rest = strings.TrimSpace(rest[1:])
		switchName, tail := extractFieldToken(rest)
		if switchName == "" {
			break
		}
		upperSwitch := strings.ToUpper(switchName)
		if upperSwitch == "B" || upperSwitch == "F" || upperSwitch == "#" || upperSwitch == "@" {
			var text string
			if strings.HasPrefix(tail, `"`) {
				text, tail = extractFieldString(tail)
			} else {
				fields := strings.Fields(tail)
				if len(fields) > 0 {
					text = fields[0]
					tail = strings.TrimSpace(tail[len(text):])
				}
			}
			switch upperSwitch {
			case "B":
				options.beforeText = text
			case "F":
				options.afterText = text
			case "#":
				options.numberFormat = text
			case "@":
				options.dateFormat = text
			}
			rest = tail
			continue
		}
		if upperSwitch == "*" {
			token, next := extractFieldToken(tail)
			switch strings.ToLower(token) {
			case "upper", "lower", "caps", "firstcap":
				options.caseFormat = token
			}
			rest = next
			continue
		}
		// Skip unknown switches and their optional arguments.
		if strings.HasPrefix(tail, `"`) {
			_, tail = extractFieldString(tail)
		} else {
			fields := strings.Fields(tail)
			if len(fields) > 0 && !strings.HasPrefix(fields[0], `\`) {
				tail = strings.TrimSpace(tail[len(fields[0]):])
			}
		}
		rest = tail
	}
	return options
}

func formatMergeFieldValue(value string, options mergeFieldOptions) string {
	if value == "" {
		return ""
	}
	if options.dateFormat != "" {
		if t := parseISODate(value); !t.IsZero() {
			value = t.Format(convertWordDateFormat(options.dateFormat))
		}
	}
	if options.numberFormat != "" {
		value = formatRefNumber(value, options.numberFormat)
	}
	if options.caseFormat != "" {
		value = applyFieldCaseFormat(value, options.caseFormat)
	}
	return value
}

// parsePrivateField extracts the application name and payload from a PRIVATE
// field instruction. Format: PRIVATE "AppName" payload... or PRIVATE AppName payload...
func parsePrivateField(instr string) (appName, payload string) {
	instr = strings.TrimSpace(instr)
	upper := strings.ToUpper(instr)
	if !strings.HasPrefix(upper, "PRIVATE ") {
		return "", ""
	}
	rest := strings.TrimSpace(instr[8:])
	if rest == "" {
		return "", ""
	}
	if strings.HasPrefix(rest, `"`) {
		appName, rest = extractFieldString(rest)
		appName = strings.TrimSpace(appName)
	} else {
		fields := strings.Fields(rest)
		if len(fields) == 0 {
			return "", ""
		}
		appName = fields[0]
		rest = strings.TrimSpace(rest[len(appName):])
	}
	return appName, strings.TrimSpace(rest)
}

// parseDatabaseQuery extracts the connection string (\d) and SQL query (\s)
// from a DATABASE field instruction. Returns empty strings when either switch
// is absent.
func parseDatabaseQuery(instr string) (connectionString, sqlQuery string) {
	instr = strings.TrimSpace(instr)
	if !strings.HasPrefix(strings.ToUpper(instr), "DATABASE") {
		return "", ""
	}
	rest := strings.TrimSpace(instr[len("DATABASE"):])
	for rest != "" {
		rest = strings.TrimSpace(rest)
		if !strings.HasPrefix(rest, `\`) {
			break
		}
		rest = strings.TrimSpace(rest[1:])
		if len(rest) == 0 {
			break
		}
		switchName := strings.ToUpper(string(rest[0]))
		rest = strings.TrimSpace(rest[1:])
		var value string
		if strings.HasPrefix(rest, `"`) {
			value, rest = extractFieldString(rest)
		} else {
			fields := strings.Fields(rest)
			if len(fields) > 0 {
				value = fields[0]
				rest = strings.TrimSpace(rest[len(value):])
			}
		}
		switch switchName {
		case "D":
			connectionString = value
		case "S":
			sqlQuery = value
		}
	}
	return connectionString, sqlQuery
}

// mergeDataValue looks up a merge-field name in the current merge record.
// It returns the value and true when found.
func (r *htmlRenderer) mergeDataValue(name string) (string, bool) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", false
	}
	idx := r.currentMergeRecord
	if idx >= 0 && idx < len(r.options.MergeRecords) {
		if rec := r.options.MergeRecords[idx]; rec != nil {
			if v, ok := rec[name]; ok {
				return v, true
			}
			for key, value := range rec {
				if strings.EqualFold(strings.TrimSpace(key), name) {
					return value, true
				}
			}
		}
	}
	if r.options.MergeData != nil {
		if v, ok := r.options.MergeData[name]; ok {
			return v, true
		}
		for key, value := range r.options.MergeData {
			if strings.EqualFold(strings.TrimSpace(key), name) {
				return value, true
			}
		}
	}
	return "", false
}

// evaluateNextIf parses a NEXTIF instruction and evaluates its condition.
// Format: NEXTIF expression1 operator expression2
func evaluateNextIf(instr string) bool {
	return evaluateMergeConditional(instr, "NEXTIF", nil)
}

// evaluateSkipIf parses a SKIPIF instruction and evaluates its condition.
// Format: SKIPIF expression1 operator expression2
func evaluateSkipIf(instr string) bool {
	return evaluateMergeConditional(instr, "SKIPIF", nil)
}

// evaluateMergeConditional evaluates a mail-merge conditional expression.
// Supported operators: =, <>, <, >, <=, >=
// Supports the \c switch for case-sensitive comparison.
func evaluateMergeConditional(instr, keyword string, resolver func(string) (string, bool)) bool {
	instr = strings.TrimSpace(instr)
	upper := strings.ToUpper(instr)
	if !strings.HasPrefix(upper, keyword+" ") {
		return false
	}
	rest := strings.TrimSpace(instr[len(keyword)+1:])
	if rest == "" {
		return false
	}
	// Detect \c switch for case-sensitive comparison.
	caseSensitive := false
	rest = stripConditionalSwitch(rest, "c", &caseSensitive)
	if rest == "" {
		return false
	}
	// Extract two operands and an operator. The expressions may be quoted.
	expr1, rest, quoted1 := extractFieldOperand(rest)
	rest = strings.TrimSpace(rest)
	op, rest := extractFieldStringOrToken(rest)
	rest = strings.TrimSpace(rest)
	expr2, _, quoted2 := extractFieldOperand(rest)
	if op == "" {
		return false
	}
	v1 := resolveMergeConditionalOperand(expr1, quoted1, resolver)
	v2 := resolveMergeConditionalOperand(expr2, quoted2, resolver)
	switch op {
	case "=":
		if caseSensitive {
			return v1 == v2
		}
		return strings.EqualFold(v1, v2)
	case "<>":
		if caseSensitive {
			return v1 != v2
		}
		return !strings.EqualFold(v1, v2)
	}
	// Try numeric comparison.
	n1, err1 := strconv.ParseFloat(v1, 64)
	n2, err2 := strconv.ParseFloat(v2, 64)
	if err1 != nil || err2 != nil {
		return false
	}
	switch op {
	case "<":
		return n1 < n2
	case ">":
		return n1 > n2
	case "<=":
		return n1 <= n2
	case ">=":
		return n1 >= n2
	}
	return false
}

// stripConditionalSwitch removes all occurrences of a switch (e.g. \c) from
// the instruction text. If the switch is found, *found is set to true.
func stripConditionalSwitch(s, switchName string, found *bool) string {
	var result strings.Builder
	inQuotes := false
	for i := 0; i < len(s); {
		if s[i] == '"' {
			inQuotes = !inQuotes
			result.WriteByte(s[i])
			i++
			continue
		}
		if inQuotes {
			result.WriteByte(s[i])
			i++
			continue
		}
		if s[i] == '\\' && i+1 < len(s) && strings.EqualFold(string(s[i+1]), switchName) {
			// Verify this is a whole token (not part of a longer switch like \caps).
			nextIdx := i + 2
			nextOK := nextIdx >= len(s) || s[nextIdx] == ' ' || s[nextIdx] == '\t' || s[nextIdx] == '\n' || s[nextIdx] == '\r' || s[nextIdx] == '\\'
			if nextOK {
				*found = true
				i = nextIdx
				// Skip trailing whitespace.
				for i < len(s) && (s[i] == ' ' || s[i] == '\t' || s[i] == '\n' || s[i] == '\r') {
					i++
				}
				continue
			}
		}
		result.WriteByte(s[i])
		i++
	}
	return strings.TrimSpace(result.String())
}

func (r *htmlRenderer) evaluateNextIf(instr string) bool {
	return evaluateMergeConditional(instr, "NEXTIF", r.mergeDataValue)
}

func (r *htmlRenderer) evaluateSkipIf(instr string) bool {
	return evaluateMergeConditional(instr, "SKIPIF", r.mergeDataValue)
}

func (r *htmlRenderer) evaluateNextIfWithStatus(instr string) (bool, bool) {
	return evaluateMergeConditionalWithStatus(instr, "NEXTIF", r.mergeDataValue)
}

func (r *htmlRenderer) evaluateSkipIfWithStatus(instr string) (bool, bool) {
	return evaluateMergeConditionalWithStatus(instr, "SKIPIF", r.mergeDataValue)
}

// extractFieldStringOrToken returns the first quoted string or whitespace-delimited
// token from s, plus the remaining text.
func extractFieldStringOrToken(s string) (token, rest string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", ""
	}
	if strings.HasPrefix(s, `"`) {
		return extractFieldString(s)
	}
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return "", ""
	}
	return fields[0], strings.TrimSpace(s[len(fields[0]):])
}

func extractFieldOperand(s string) (token, rest string, quoted bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", "", false
	}
	if strings.HasPrefix(s, `"`) {
		token, rest = extractFieldString(s)
		return token, rest, true
	}
	token, rest = extractFieldStringOrToken(s)
	return token, rest, false
}

func resolveMergeConditionalOperand(token string, quoted bool, resolver func(string) (string, bool)) string {
	value, _ := resolveMergeConditionalOperandWithStatus(token, quoted, resolver)
	return value
}

func resolveMergeConditionalOperandWithStatus(token string, quoted bool, resolver func(string) (string, bool)) (string, bool) {
	token = strings.TrimSpace(token)
	if token == "" || quoted {
		return token, true
	}
	if resolver == nil {
		return token, !isAmbiguousFieldOperand(token)
	}
	if value, ok := resolver(token); ok {
		return strings.TrimSpace(value), true
	}
	return token, !isAmbiguousFieldOperand(token)
}

func evaluateMergeConditionalWithStatus(instr, keyword string, resolver func(string) (string, bool)) (bool, bool) {
	instr = strings.TrimSpace(instr)
	upper := strings.ToUpper(instr)
	if !strings.HasPrefix(upper, keyword+" ") {
		return false, false
	}
	rest := strings.TrimSpace(instr[len(keyword)+1:])
	if rest == "" {
		return false, false
	}
	caseSensitive := false
	rest = stripConditionalSwitch(rest, "c", &caseSensitive)
	if rest == "" {
		return false, false
	}
	expr1, rest, quoted1 := extractFieldOperand(rest)
	rest = strings.TrimSpace(rest)
	op, rest := extractFieldStringOrToken(rest)
	rest = strings.TrimSpace(rest)
	expr2, _, quoted2 := extractFieldOperand(rest)
	if op == "" {
		return false, false
	}
	v1, ok1 := resolveMergeConditionalOperandWithStatus(expr1, quoted1, resolver)
	v2, ok2 := resolveMergeConditionalOperandWithStatus(expr2, quoted2, resolver)
	if !ok1 || !ok2 {
		return false, false
	}
	switch op {
	case "=":
		if caseSensitive {
			return v1 == v2, true
		}
		return strings.EqualFold(v1, v2), true
	case "<>":
		if caseSensitive {
			return v1 != v2, true
		}
		return !strings.EqualFold(v1, v2), true
	}
	n1, err1 := strconv.ParseFloat(v1, 64)
	n2, err2 := strconv.ParseFloat(v2, 64)
	if err1 != nil || err2 != nil {
		return false, false
	}
	switch op {
	case "<":
		return n1 < n2, true
	case ">":
		return n1 > n2, true
	case "<=":
		return n1 <= n2, true
	case ">=":
		return n1 >= n2, true
	}
	return false, false
}

func isAmbiguousFieldOperand(token string) bool {
	token = strings.TrimSpace(token)
	if token == "" {
		return false
	}
	if _, err := strconv.ParseFloat(token, 64); err == nil {
		return false
	}
	switch strings.ToLower(token) {
	case "true", "false":
		return false
	}
	for _, ch := range token {
		if (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '_' || ch == '.' || ch == '-' {
			continue
		}
		return false
	}
	return true
}

// resolveStyleRef finds the first paragraph whose style name or ID matches
// styleName and returns its plain text. Used by STYLEREF field evaluation.
// When rendering inside a paragraph, prefer the nearest matching paragraph at
// or before the current paragraph; fall back to the first later match.
func (r *htmlRenderer) resolveStyleRef(styleName string) string {
	if styleName == "" {
		return ""
	}
	upper := strings.ToUpper(styleName)
	currentSeen := r.currentParagraph == nil
	nearestBefore := ""
	firstAfter := ""
	var scanBlocks func(blocks []resolve.Block)
	scanBlocks = func(blocks []resolve.Block) {
		if !currentSeen && nearestBefore != "" && firstAfter != "" {
			return
		}
		for _, b := range blocks {
			if b.Kind == "paragraph" && b.Paragraph != nil {
				p := b.Paragraph
				isCurrent := p == r.currentParagraph
				if strings.ToUpper(p.StyleName) == upper || strings.ToUpper(p.StyleID) == upper {
					text := paragraphBookmarkText(p)
					if text != "" {
						if !currentSeen || isCurrent {
							nearestBefore = text
						} else {
							if firstAfter == "" {
								firstAfter = text
							}
						}
					}
				}
				if isCurrent {
					currentSeen = true
				}
				for _, run := range p.Runs {
					if run.TextBox != nil {
						scanBlocks(run.TextBox.Blocks)
					}
				}
			} else if b.Kind == "table" && b.Table != nil {
				for _, row := range b.Table.Rows {
					for _, cell := range row.Cells {
						scanBlocks(cell.Blocks)
					}
				}
			}
		}
	}
	scanBlocks(r.doc.Blocks)
	if nearestBefore != "" {
		return nearestBefore
	}
	return firstAfter
}

// countDocWords counts whitespace-separated words in all document paragraph runs.
func (r *htmlRenderer) countDocWords() int {
	total := 0
	var countBlocks func(blocks []resolve.Block)
	countBlocks = func(blocks []resolve.Block) {
		for _, b := range blocks {
			if b.Kind == "paragraph" && b.Paragraph != nil {
				for _, run := range b.Paragraph.Runs {
					total += len(strings.Fields(run.Text))
					if run.TextBox != nil {
						countBlocks(run.TextBox.Blocks)
					}
				}
			} else if b.Kind == "table" && b.Table != nil {
				for _, row := range b.Table.Rows {
					for _, cell := range row.Cells {
						countBlocks(cell.Blocks)
					}
				}
			}
		}
	}
	countBlocks(r.doc.Blocks)
	return total
}

// countDocChars counts non-whitespace characters in all document paragraph runs.
func (r *htmlRenderer) countDocChars() int {
	total := 0
	var countBlocks func(blocks []resolve.Block)
	countBlocks = func(blocks []resolve.Block) {
		for _, b := range blocks {
			if b.Kind == "paragraph" && b.Paragraph != nil {
				for _, run := range b.Paragraph.Runs {
					for _, ch := range run.Text {
						if ch != ' ' && ch != '\t' && ch != '\n' && ch != '\r' {
							total++
						}
					}
					if run.TextBox != nil {
						countBlocks(run.TextBox.Blocks)
					}
				}
			} else if b.Kind == "table" && b.Table != nil {
				for _, row := range b.Table.Rows {
					for _, cell := range row.Cells {
						countBlocks(cell.Blocks)
					}
				}
			}
		}
	}
	countBlocks(r.doc.Blocks)
	return total
}

// evaluateIfField evaluates a simple IF or COMPARE field instruction.
// Format: IF expr1 operator expr2 "trueText" "falseText"
// Returns the appropriate branch text, or empty string on parse failure.
func (r *htmlRenderer) evaluateIfField(instr string) string {
	value, _ := r.evaluateIfFieldWithStatus(instr, false)
	return value
}

func (r *htmlRenderer) evaluateIfFieldRun(run resolve.Run) (string, bool) {
	return r.evaluateIfFieldWithStatus(r.resolveFieldInstruction(run), len(run.FieldInstrRuns) > 0)
}

func (r *htmlRenderer) evaluateIfFieldWithStatus(instr string, allowAmbiguous bool) (string, bool) {
	if instr == "" {
		return "", false
	}
	// Strip the leading keyword (IF or COMPARE).
	upper := strings.ToUpper(instr)
	var rest string
	if strings.HasPrefix(upper, "IF ") {
		rest = strings.TrimSpace(instr[3:])
	} else if strings.HasPrefix(upper, "COMPARE ") {
		rest = strings.TrimSpace(instr[8:])
	} else {
		return "", false
	}

	// Extract left operand (may be quoted).
	left, rest, leftQuoted := extractFieldOperand(rest)
	// Extract operator token.
	fields := strings.Fields(rest)
	if len(fields) < 3 {
		return "", false
	}
	op := fields[0]
	rest = strings.TrimSpace(rest[len(op):])
	right, rest, rightQuoted := extractFieldOperand(rest)
	trueText, rest := extractIfOperand(strings.TrimSpace(rest))
	falseText, _ := extractIfOperand(strings.TrimSpace(rest))

	// Resolve doc-property placeholders (e.g. left/right may reference field names).
	var ok bool
	left, ok = r.resolveIfOperandWithStatus(left, leftQuoted, allowAmbiguous)
	if !ok {
		return "", false
	}
	right, ok = r.resolveIfOperandWithStatus(right, rightQuoted, allowAmbiguous)
	if !ok {
		return "", false
	}

	result := compareIfOperands(left, right, op)
	if result {
		return trueText, true
	}
	return falseText, true
}

// evaluateQuoteField evaluates a Word QUOTE field by concatenating quoted and
// unquoted tokens after nested field expansion has been applied while
// preserving original spacing between tokens.
func evaluateQuoteField(instr string) string {
	if instr == "" {
		return ""
	}
	upper := strings.ToUpper(instr)
	if !strings.HasPrefix(upper, "QUOTE") {
		return ""
	}
	rest := strings.TrimSpace(instr[len("QUOTE"):])
	if rest == "" {
		return ""
	}
	var sb strings.Builder
	lastWasSpace := false
	for len(rest) > 0 {
		if rest[0] == '"' {
			rest = rest[1:]
			i := strings.Index(rest, `"`)
			if i < 0 {
				for _, ch := range rest {
					if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
						if !lastWasSpace {
							sb.WriteByte(' ')
							lastWasSpace = true
						}
						continue
					}
					sb.WriteRune(ch)
					lastWasSpace = false
				}
				break
			}
			for _, ch := range rest[:i] {
				if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
					if !lastWasSpace {
						sb.WriteByte(' ')
						lastWasSpace = true
					}
					continue
				}
				sb.WriteRune(ch)
				lastWasSpace = false
			}
			rest = rest[i+1:]
			continue
		}
		if rest[0] == ' ' || rest[0] == '\t' || rest[0] == '\n' || rest[0] == '\r' {
			if !lastWasSpace {
				sb.WriteByte(' ')
				lastWasSpace = true
			}
			rest = rest[1:]
			continue
		}
		sb.WriteByte(rest[0])
		lastWasSpace = false
		rest = rest[1:]
	}
	return sb.String()
}

// extractIfOperand reads the next operand from an IF field instruction.
// Operands are either quoted strings or unquoted tokens.
func extractIfOperand(s string) (value, rest string) {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, `"`) {
		s = s[1:] // skip opening quote
		i := strings.Index(s, `"`)
		if i < 0 {
			return s, ""
		}
		return s[:i], strings.TrimSpace(s[i+1:])
	}
	// Unquoted token — take until next space.
	i := strings.IndexByte(s, ' ')
	if i < 0 {
		return s, ""
	}
	return s[:i], strings.TrimSpace(s[i+1:])
}

// resolveIfOperand resolves a field reference like TITLE, AUTHOR etc.
func (r *htmlRenderer) resolveIfOperand(v string) string {
	value, _ := r.resolveIfOperandWithStatus(v, false, false)
	return value
}

func (r *htmlRenderer) resolveIfOperandWithStatus(v string, quoted, allowAmbiguous bool) (string, bool) {
	trimmed := strings.TrimSpace(v)
	if quoted || trimmed == "" {
		return trimmed, true
	}
	// Mail-merge IF / COMPARE fields frequently compare bare field names against
	// literals without nesting an explicit MERGEFIELD in the token stream.
	if value, ok := r.mergeDataValue(trimmed); ok {
		return strings.TrimSpace(value), true
	}
	// If the operand matches a known bookmark, resolve its text.
	if text, ok := r.bookmarkText[trimmed]; ok {
		return text, true
	}
	if value, ok := r.lookupDocPropertyValue(trimmed); ok {
		return strings.TrimSpace(value), true
	}
	if value, ok := r.lookupDocumentVariable(trimmed); ok {
		return strings.TrimSpace(value), true
	}
	if !allowAmbiguous && isAmbiguousFieldOperand(trimmed) {
		return trimmed, false
	}
	return trimmed, true
}

func (r *htmlRenderer) generateTOC(instr string) string {
	options := parseTOCOptions(instr)
	entries := r.collectTOCEntries(options)
	if len(entries) == 0 {
		return ""
	}
	lines := make([]string, 0, len(entries))
	for _, entry := range entries {
		lines = append(lines, formatTOCEntry(entry, !options.SuppressPageNumbers, options.Separator, options.PageSeparator))
	}
	return strings.Join(lines, "\n")
}

// generateIndex builds a back-of-book index from all XE field entries.
// Supports the \f "id" switch (filter by type) and \h letter headings switch.
func (r *htmlRenderer) generateIndex(instr string) string {
	entries := r.collectIndexEntries(instr)
	if len(entries) == 0 {
		return ""
	}
	showHeadings := parseIndexShowHeadings(instr)
	suppressPages := parseIndexSuppressPages(instr)
	pageSep := parseIndexPageSeparator(instr)
	groupSep := parseIndexGroupSeparator(instr)
	rangeSep := parseIndexRangeSeparator(instr)
	letterStart, letterEnd := parseIndexLetterRange(instr)
	sequenceSep := parseIndexSequenceSeparator(instr)

	// Filter entries by letter range if \p switch is present.
	if letterStart > 0 && letterEnd > 0 {
		filtered := entries[:0]
		for _, e := range entries {
			first := rune(0)
			if r := []rune(strings.ToUpper(e.main)); len(r) > 0 {
				first = r[0]
			}
			if first >= letterStart && first <= letterEnd {
				filtered = append(filtered, e)
			}
		}
		entries = filtered
	}

	// Group page numbers by (main, sub) key, preserving first-seen order.
	type entryKey struct{ main, sub string }
	type groupedEntry struct {
		main  string
		sub   string
		pages []indexPageRef
	}
	order := make([]entryKey, 0, len(entries))
	groups := make(map[entryKey]*groupedEntry, len(entries))
	for _, e := range entries {
		key := entryKey{strings.ToLower(e.main), strings.ToLower(e.sub)}
		if _, exists := groups[key]; !exists {
			order = append(order, key)
			groups[key] = &groupedEntry{main: e.main, sub: e.sub}
		}
		groups[key].pages = append(groups[key].pages, indexPageRef{page: e.page, sequence: e.pageSequence})
	}

	// Sort: by main (case-insensitive), then sub (case-insensitive).
	sort.Slice(order, func(i, j int) bool {
		if order[i].main != order[j].main {
			return order[i].main < order[j].main
		}
		return order[i].sub < order[j].sub
	})

	var lines []string
	var currentLetter string
	runInDelimiter := parseIndexRunInDelimiter(instr)
	if runInDelimiter != "" {
		// Run-in format: collect subentries per main entry and emit inline.
		mainSubs := make(map[string][]string)
		mainPages := make(map[string][]indexPageRef)
		mainOrder := make([]string, 0)
		seenMain := make(map[string]bool)
		for _, key := range order {
			g := groups[key]
			if g.sub == "" {
				if !seenMain[g.main] {
					mainOrder = append(mainOrder, g.main)
					seenMain[g.main] = true
				}
				mainPages[g.main] = append(mainPages[g.main], g.pages...)
			} else {
				if !seenMain[g.main] {
					mainOrder = append(mainOrder, g.main)
					seenMain[g.main] = true
				}
				pageStr := formatIndexPages(g.pages, pageSep, groupSep, rangeSep, sequenceSep)
				if suppressPages {
					pageStr = ""
				}
				mainSubs[g.main] = append(mainSubs[g.main], g.sub+" "+pageStr)
				mainPages[g.main] = append(mainPages[g.main], g.pages...)
			}
		}
		for _, main := range mainOrder {
			letter := strings.ToUpper(string([]rune(main)[:1]))
			if showHeadings && letter != currentLetter {
				lines = append(lines, letter)
				currentLetter = letter
			}
			allPages := formatIndexPages(mainPages[main], pageSep, groupSep, rangeSep, sequenceSep)
			if suppressPages {
				allPages = ""
			}
			line := formatIndexEntry(main, allPages, 0)
			if subs := mainSubs[main]; len(subs) > 0 {
				line += ": " + strings.Join(subs, runInDelimiter)
			}
			lines = append(lines, line)
		}
		return strings.Join(lines, "\n")
	}

	for _, key := range order {
		g := groups[key]
		letter := strings.ToUpper(string([]rune(g.main)[:1]))
		if showHeadings && letter != currentLetter {
			lines = append(lines, letter)
			currentLetter = letter
		}
		pageStr := formatIndexPages(g.pages, pageSep, groupSep, rangeSep, sequenceSep)
		if g.sub == "" {
			if !suppressPages {
				lines = append(lines, formatIndexEntry(g.main, pageStr, 0))
			} else {
				lines = append(lines, formatIndexEntry(g.main, "", 0))
			}
		} else {
			// Emit main heading without page if this is the first sub-entry for it.
			mainKey := entryKey{key.main, ""}
			if _, hasMain := groups[mainKey]; !hasMain {
				lines = append(lines, g.main)
			}
			if !suppressPages {
				lines = append(lines, formatIndexEntry(g.sub, pageStr, 2))
			} else {
				lines = append(lines, formatIndexEntry(g.sub, "", 2))
			}
		}
	}
	return strings.Join(lines, "\n")
}

// generateTOA builds a Table of Authorities from all TA field entries.
func (r *htmlRenderer) generateTOA(instr string) string {
	options := parseTOAOptions(instr)
	entries := r.collectTAEntries(options)
	if len(entries) == 0 {
		return ""
	}
	// Group entries by category, preserving first-seen order.
	type catKey struct {
		name string
		num  int
	}
	catOrder := make([]catKey, 0)
	cats := make(map[catKey][]taEntry)
	for _, e := range entries {
		key := catKey{name: strings.ToLower(e.category), num: e.categoryNum}
		if _, ok := cats[key]; !ok {
			catOrder = append(catOrder, key)
		}
		cats[key] = append(cats[key], e)
	}
	var lines []string
	for _, key := range catOrder {
		group := cats[key]
		// Use the first entry's category name as the heading.
		if heading := group[0].category; heading != "" {
			lines = append(lines, heading)
		}
		// Sort entries alphabetically by citation text.
		sort.Slice(group, func(i, j int) bool {
			return strings.ToLower(group[i].citation) < strings.ToLower(group[j].citation)
		})
		for _, e := range group {
			pageStr := ""
			if !options.suppressPages {
				pageStr = strconv.Itoa(e.page)
			}
			lines = append(lines, formatTOAEntry(e.citation, pageStr, options.separator))
		}
	}
	return strings.Join(lines, "\n")
}

type taEntry struct {
	citation    string
	category    string
	categoryNum int
	page        int
}

type toaOptions struct {
	category      string // \c filter
	separator     string // \p separator between entries
	suppressPages bool   // \n suppress page numbers
}

func parseTOAOptions(instr string) toaOptions {
	var opts toaOptions
	opts.separator = ", "
	upper := strings.ToUpper(instr)
	if !strings.HasPrefix(upper, "TOA") {
		return opts
	}
	rest := strings.TrimSpace(instr[len("TOA"):])
	for strings.HasPrefix(rest, `\`) {
		rest = strings.TrimSpace(rest[1:])
		switchName, tail := extractFieldToken(rest)
		if switchName == "" {
			break
		}
		upperSwitch := strings.ToUpper(switchName)
		if upperSwitch == "C" {
			value, next := extractIfOperand(tail)
			opts.category = strings.TrimSpace(value)
			rest = next
			continue
		}
		if upperSwitch == "P" {
			value, next := extractIfOperand(tail)
			if v := strings.TrimSpace(value); v != "" {
				opts.separator = v
			}
			rest = next
			continue
		}
		if upperSwitch == "N" {
			opts.suppressPages = true
			rest = tail
			continue
		}
		rest = tail
	}
	return opts
}

func (r *htmlRenderer) collectTAEntries(options toaOptions) []taEntry {
	var entries []taEntry
	for _, page := range r.pages {
		for _, box := range page.Blocks {
			if box.Block == nil || box.Block.Paragraph == nil {
				continue
			}
			for _, run := range box.Block.Paragraph.Runs {
				if run.FieldKind != "TA" {
					continue
				}
				entry, ok := parseTAEntry(run.FieldArg)
				if !ok {
					continue
				}
				if options.category != "" && !strings.EqualFold(entry.category, options.category) {
					continue
				}
				entries = append(entries, taEntry{
					citation:    entry.citation,
					category:    entry.category,
					categoryNum: entry.categoryNum,
					page:        page.Number,
				})
			}
		}
	}
	return entries
}

func parseTAEntry(instr string) (taEntry, bool) {
	var entry taEntry
	rest := strings.TrimSpace(instr)
	if len(rest) >= 2 && strings.EqualFold(rest[:2], "TA") {
		rest = strings.TrimSpace(rest[2:])
	}
	entry.categoryNum = 1
	for rest != "" {
		rest = strings.TrimSpace(rest)
		if !strings.HasPrefix(rest, `\`) || len(rest) < len(`\L`) {
			break
		}
		switchToken := strings.ToUpper(rest[:len(`\L`)])
		rest = strings.TrimSpace(rest[len(`\L`):])
		switch switchToken {
		case `\L`:
			value, next := extractIfOperand(rest)
			entry.citation = strings.TrimSpace(value)
			rest = next
		case `\S`:
			value, next := extractIfOperand(rest)
			entry.category = strings.TrimSpace(value)
			rest = next
		case `\C`:
			value, next := extractIfOperand(rest)
			if n, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && n >= 1 && n <= 16 {
				entry.categoryNum = n
			}
			rest = next
		default:
			_, rest = extractIfOperand(rest)
		}
	}
	if entry.citation == "" {
		return entry, false
	}
	return entry, true
}

func formatTOAEntry(citation, pageStr, separator string) string {
	if pageStr == "" {
		return citation
	}
	return citation + separator + pageStr
}

// generateBibliography builds a bibliography list from CITATION fields in the
// document. When no sources are available, returns a placeholder.
func (r *htmlRenderer) generateBibliography(instr string) string {
	// Collect unique citation tags from CITATION fields.
	seen := make(map[string]bool)
	var sources []string
	for _, page := range r.pages {
		for _, box := range page.Blocks {
			if box.Block == nil || box.Block.Paragraph == nil {
				continue
			}
			for _, run := range box.Block.Paragraph.Runs {
				if run.FieldKind != "CITATION" {
					continue
				}
				tag := strings.TrimSpace(run.FieldArg)
				if tag == "" {
					continue
				}
				if !seen[tag] {
					seen[tag] = true
					sources = append(sources, tag)
				}
			}
		}
	}
	if len(sources) == 0 {
		return "[Bibliography]"
	}
	sort.Strings(sources)
	return strings.Join(sources, "\n")
}

type indexEntry struct {
	main         string
	sub          string
	typeID       string
	page         int
	pageSequence int
}

type indexPageRef struct {
	page     int
	sequence int
}

func (r *htmlRenderer) collectIndexEntries(instr string) []indexEntry {
	filterID := parseIndexFilterID(instr)
	bookmarkScopeName := parseIndexBookmark(instr)
	rangeBookmark := parseIndexRangeBookmark(instr)
	sequenceName := parseIndexSequenceName(instr)
	scope, hasScope := r.bookmarkScopeFor(rangeBookmark)
	if bookmarkScopeName != "" {
		scope, hasScope = r.bookmarkScopeFor(bookmarkScopeName)
	}
	var entries []indexEntry
	for _, page := range r.pages {
		for _, box := range page.Blocks {
			if box.Block == nil || box.Block.Paragraph == nil {
				continue
			}
			if hasScope && !r.paragraphInBookmarkScope(box.Block.Paragraph, scope) {
				continue
			}
			for _, run := range box.Block.Paragraph.Runs {
				if run.FieldKind != "XE" {
					continue
				}
				main, sub, typeID, ok := parseXEEntry(run.FieldArg)
				if !ok || main == "" {
					continue
				}
				if filterID != "" && !strings.EqualFold(typeID, filterID) {
					continue
				}
				pageSequence := 0
				if sequenceName != "" {
					pageSequence, _ = r.sequenceValueAtParagraph(sequenceName, box.Block.Paragraph.Index)
				}
				entries = append(entries, indexEntry{main: main, sub: sub, typeID: typeID, page: page.Number, pageSequence: pageSequence})
			}
		}
	}
	return entries
}

// parseXEEntry extracts the main term, optional subterm, and optional type ID
// from an XE field instruction like:  XE "Main:Sub" \f "tableid"
func parseXEEntry(instr string) (main, sub, typeID string, ok bool) {
	rest := strings.TrimSpace(instr)
	if len(rest) >= 2 && strings.EqualFold(rest[:2], "XE") {
		rest = strings.TrimSpace(rest[2:])
	}
	term, rest := extractIfOperand(rest)
	if term == "" {
		return "", "", "", false
	}
	// Extract optional \f "typeid" switch.
	upper := strings.ToUpper(rest)
	if idx := strings.Index(upper, `\F`); idx >= 0 {
		typeID, _ = extractFieldSwitchValue(strings.TrimSpace(rest[idx+2:]))
	}
	// Split on first colon for sub-entry.
	if idx := strings.IndexByte(term, ':'); idx >= 0 {
		return strings.TrimSpace(term[:idx]), strings.TrimSpace(term[idx+1:]), typeID, true
	}
	return strings.TrimSpace(term), "", typeID, true
}

// parseIndexFilterID returns the \f "id" switch value from an INDEX instruction.
func parseIndexFilterID(instr string) string {
	upper := strings.ToUpper(instr)
	idx := strings.Index(upper, `\F`)
	if idx < 0 {
		return ""
	}
	v, _ := extractFieldSwitchValue(strings.TrimSpace(instr[idx+2:]))
	return v
}

// parseIndexShowHeadings returns true when the INDEX instruction contains a \h switch.
func parseIndexShowHeadings(instr string) bool {
	return strings.Contains(strings.ToUpper(instr), `\H`)
}

// parseIndexSuppressPages returns true when the INDEX instruction contains a \z switch.
// In Word this hides page numbers in Web layout view; HTML output is treated the same.
func parseIndexSuppressPages(instr string) bool {
	return strings.Contains(strings.ToUpper(instr), `\Z`)
}

// parseIndexPageSeparator returns the \e switch value from an INDEX instruction.
func parseIndexPageSeparator(instr string) string {
	upper := strings.ToUpper(instr)
	idx := strings.Index(upper, `\E`)
	if idx < 0 {
		return ""
	}
	value, ok := extractFieldSwitchValue(instr[idx+len(`\E`):])
	if !ok {
		return ""
	}
	return value
}

// parseIndexColumns returns the \c switch value (column count) from an INDEX
// instruction. Defaults to 1 when absent.
func parseIndexColumns(instr string) int {
	upper := strings.ToUpper(instr)
	idx := strings.Index(upper, `\C`)
	if idx < 0 {
		return 1
	}
	value, ok := extractFieldSwitchValue(instr[idx+len(`\C`):])
	if !ok {
		return 1
	}
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || n < 1 {
		return 1
	}
	if n > 4 {
		return 4
	}
	return n
}

// parseIndexRangeSeparator returns the \k switch value from an INDEX
// instruction. When present, consecutive page numbers are collapsed into
// ranges using this separator.
func parseIndexRangeSeparator(instr string) string {
	upper := strings.ToUpper(instr)
	idx := strings.Index(upper, `\K`)
	if idx < 0 {
		return ""
	}
	value, ok := extractFieldSwitchValue(instr[idx+len(`\K`):])
	if !ok {
		return ""
	}
	return value
}

func parseIndexSequenceName(instr string) string {
	upper := strings.ToUpper(instr)
	idx := strings.Index(upper, `\S`)
	if idx < 0 {
		return ""
	}
	value, ok := extractFieldSwitchValue(instr[idx+len(`\S`):])
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

func parseIndexSequenceSeparator(instr string) string {
	upper := strings.ToUpper(instr)
	idx := strings.Index(upper, `\D`)
	if idx < 0 {
		return ""
	}
	value, ok := extractFieldSwitchValue(instr[idx+len(`\D`):])
	if !ok {
		return ""
	}
	return value
}

// parseIndexGroupSeparator returns the \g switch value from an INDEX instruction.
// It is used as the separator between page reference groups.
func parseIndexGroupSeparator(instr string) string {
	upper := strings.ToUpper(instr)
	idx := strings.Index(upper, `\G`)
	if idx < 0 {
		return ""
	}
	value, ok := extractFieldSwitchValue(instr[idx+len(`\G`):])
	if !ok {
		return ""
	}
	return value
}

// parseIndexLetterRange returns the letter range from a \p switch in an INDEX
// instruction. The format is \p "a-m" or \p "a". Returns start and end runes;
// end is 0 when the switch is absent or invalid.
func parseIndexLetterRange(instr string) (start, end rune) {
	upper := strings.ToUpper(instr)
	idx := strings.Index(upper, `\P`)
	if idx < 0 {
		return 0, 0
	}
	value, ok := extractFieldSwitchValue(instr[idx+len(`\P`):])
	if !ok {
		return 0, 0
	}
	value = strings.TrimSpace(value)
	value = strings.Trim(value, `"`)
	if value == "" {
		return 0, 0
	}
	parts := strings.SplitN(value, "-", 2)
	startRune := []rune(strings.ToUpper(parts[0]))
	if len(startRune) == 0 {
		return 0, 0
	}
	start = startRune[0]
	end = start
	if len(parts) == 2 {
		endRune := []rune(strings.ToUpper(strings.TrimSpace(parts[1])))
		if len(endRune) > 0 {
			end = endRune[0]
		}
	}
	if end < start {
		end = start
	}
	return start, end
}

// parseIndexRunInDelimiter returns the delimiter for run-in subentries from a
// \y switch in an INDEX instruction. Returns "; " when the switch is present
// without a value, "" when absent.
func parseIndexRunInDelimiter(instr string) string {
	upper := strings.ToUpper(instr)
	idx := strings.Index(upper, `\Y`)
	if idx < 0 {
		return ""
	}
	value, ok := extractFieldSwitchValue(instr[idx+len(`\Y`):])
	if !ok || strings.TrimSpace(value) == "" {
		return "; "
	}
	return strings.TrimSpace(strings.Trim(value, `"`))
}

func parseIndexRangeBookmark(instr string) string {
	upper := strings.ToUpper(instr)
	idx := strings.Index(upper, `\R`)
	if idx < 0 {
		return ""
	}
	value, ok := extractFieldSwitchValue(instr[idx+len(`\R`):])
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

// parseIndexBookmark returns the \b "bookmark" switch value from an INDEX instruction.
func parseIndexBookmark(instr string) string {
	upper := strings.ToUpper(instr)
	idx := strings.Index(upper, `\B`)
	if idx < 0 {
		return ""
	}
	value, ok := extractFieldSwitchValue(instr[idx+len(`\B`):])
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

func formatIndexEntry(term, pages string, indent int) string {
	prefix := strings.Repeat(" ", indent)
	text := prefix + term
	dotCount := 4
	if len(text)+len(pages) < 56 {
		dotCount = 56 - len(text) - len(pages)
	}
	return text + " " + strings.Repeat(".", dotCount) + " " + pages
}

func formatIndexPageRef(ref indexPageRef, sequenceSep string) string {
	page := strconv.Itoa(ref.page)
	if ref.sequence <= 0 {
		return page
	}
	return strconv.Itoa(ref.sequence) + sequenceSep + page
}

func formatIndexPageRange(start, end indexPageRef, rangeSep, sequenceSep string) string {
	if start.sequence > 0 && start.sequence == end.sequence {
		return strconv.Itoa(start.sequence) + sequenceSep + strconv.Itoa(start.page) + rangeSep + strconv.Itoa(end.page)
	}
	return formatIndexPageRef(start, sequenceSep) + rangeSep + formatIndexPageRef(end, sequenceSep)
}

func formatIndexPages(pages []indexPageRef, separator, groupSep, rangeSep, sequenceSep string) string {
	if len(pages) == 0 {
		return ""
	}
	if separator == "" {
		separator = ", "
	}
	if sequenceSep == "" {
		sequenceSep = "-"
	}
	// Deduplicate and sort page references.
	type pageKey struct {
		page     int
		sequence int
	}
	seen := make(map[pageKey]struct{}, len(pages))
	unique := pages[:0]
	for _, p := range pages {
		key := pageKey{page: p.page, sequence: p.sequence}
		if _, ok := seen[key]; !ok {
			seen[key] = struct{}{}
			unique = append(unique, p)
		}
	}
	sort.Slice(unique, func(i, j int) bool {
		if unique[i].sequence != unique[j].sequence {
			return unique[i].sequence < unique[j].sequence
		}
		return unique[i].page < unique[j].page
	})
	collapse := rangeSep != ""
	type partInfo struct {
		text  string
		start int
		end   int
	}
	partInfos := make([]partInfo, 0, len(unique))
	for i := 0; i < len(unique); {
		start := i
		if collapse {
			for i+1 < len(unique) &&
				unique[i+1].sequence == unique[i].sequence &&
				unique[i+1].page == unique[i].page+1 {
				i++
			}
		}
		if i > start {
			partInfos = append(partInfos, partInfo{text: formatIndexPageRange(unique[start], unique[i], rangeSep, sequenceSep), start: start, end: i})
		} else {
			partInfos = append(partInfos, partInfo{text: formatIndexPageRef(unique[start], sequenceSep), start: start, end: start})
		}
		i++
	}
	// Join parts, using groupSep when sequence changes between adjacent parts.
	var sb strings.Builder
	for i, pi := range partInfos {
		if i > 0 {
			sep := separator
			if groupSep != "" && unique[partInfos[i-1].end].sequence != unique[pi.start].sequence {
				sep = groupSep
			}
			sb.WriteString(sep)
		}
		sb.WriteString(pi.text)
	}
	return sb.String()
}

type tocEntry struct {
	Level          int
	Text           string
	Page           int
	PageSequence   int
	ParagraphIndex int
}

type tocOptions struct {
	MaxHeadingLevel     int
	MinHeadingLevel     int
	UseOutlineLevel     bool // true when \l switch is present
	TCOnly              bool
	TCID                string
	CustomStyleLevels   map[string]int
	SuppressPageNumbers bool
	HeadingsOnly        bool
	Separator           string // \d switch value; empty means use default dot leader
	PageSeparator       string // \p switch value
	Bookmark            string // \b bookmark scope
	PreserveTabs        bool   // \w switch
	CaptionSequence     string // \c or \a sequence identifier
	CaptionTextOnly     bool   // \a switch
	PageSequence        string // \s switch
}

func (r *htmlRenderer) collectTOCEntries(options tocOptions) []tocEntry {
	entries := make([]tocEntry, 0)
	var scope bookmarkScope
	hasScope := false
	if options.Bookmark != "" {
		scope, hasScope = r.bookmarkScopeFor(options.Bookmark)
	}
	for _, page := range r.pages {
		for _, box := range page.Blocks {
			if box.Block == nil || box.Block.Paragraph == nil {
				continue
			}
			p := box.Block.Paragraph
			if hasScope && !r.paragraphInBookmarkScope(p, scope) {
				continue
			}
			if options.CaptionSequence != "" {
				if text, ok := r.tocCaptionText(p, options); ok && text != "" {
					pageSequence := 0
					if options.PageSequence != "" {
						pageSequence, _ = r.sequenceValueAtParagraph(options.PageSequence, p.Index)
					}
					entries = append(entries, tocEntry{Level: 1, Text: text, Page: page.Number, PageSequence: pageSequence, ParagraphIndex: p.Index})
				}
				if options.CaptionTextOnly {
					continue
				}
			}
			if !options.TCOnly {
				if level := tocParagraphLevel(p, options); level > 0 {
					if text := r.tocParagraphPlainText(p, options.PreserveTabs); text != "" {
						pageSequence := 0
						if options.PageSequence != "" {
							pageSequence, _ = r.sequenceValueAtParagraph(options.PageSequence, p.Index)
						}
						entries = append(entries, tocEntry{Level: level, Text: text, Page: page.Number, PageSequence: pageSequence, ParagraphIndex: p.Index})
					}
				}
			}
			if options.HeadingsOnly {
				continue
			}
			for _, run := range p.Runs {
				if run.FieldKind != "TC" {
					continue
				}
				entry, ok := parseTCEntry(run.FieldArg)
				if !ok {
					continue
				}
				if options.TCOnly && options.TCID != "" && !strings.EqualFold(entry.id, options.TCID) {
					continue
				}
				pageSequence := 0
				if options.PageSequence != "" {
					pageSequence, _ = r.sequenceValueAtParagraph(options.PageSequence, p.Index)
				}
				text := entry.text
				if !entry.preserveNewlines {
					text = strings.NewReplacer("\r\n", " ", "\r", " ", "\n", " ").Replace(text)
				}
				entries = append(entries, tocEntry{Level: entry.level, Text: text, Page: page.Number, PageSequence: pageSequence, ParagraphIndex: p.Index})
			}
		}
	}
	return entries
}

func parseTOCOptions(instr string) tocOptions {
	tcOnly, tcID := parseTOCTCFilter(instr)
	minLevel, maxLevel, useOutline := parseTOCLevelRange(instr)
	if !useOutline {
		maxLevel = parseTOCHeadingRange(instr)
	}
	return tocOptions{
		MaxHeadingLevel:     maxLevel,
		MinHeadingLevel:     minLevel,
		UseOutlineLevel:     useOutline || parseTOCUseOutline(instr),
		TCOnly:              tcOnly,
		TCID:                tcID,
		CustomStyleLevels:   parseTOCCustomStyles(instr),
		SuppressPageNumbers: parseTOCSuppressPageNumbers(instr),
		HeadingsOnly:        false,
		Separator:           parseTOCSeparator(instr),
		PageSeparator:       parseTOCPageSeparator(instr),
		Bookmark:            parseTOCBookmark(instr),
		PreserveTabs:        parseTOCPreserveTabs(instr),
		CaptionSequence:     parseTOCCaptionSequence(instr),
		CaptionTextOnly:     parseTOCCaptionTextOnly(instr),
		PageSequence:        parseTOCPageSequence(instr),
	}
}

func tocParagraphLevel(p *resolve.Paragraph, options tocOptions) int {
	if p == nil {
		return 0
	}
	var level int
	if options.UseOutlineLevel && p.Format.OutlineLevel != nil {
		level = *p.Format.OutlineLevel
	} else {
		level = headingLevel(p.StyleID, p.StyleName)
	}
	if level > 0 && level >= options.MinHeadingLevel && level <= options.MaxHeadingLevel {
		return level
	}
	if len(options.CustomStyleLevels) == 0 {
		return 0
	}
	for _, style := range []string{p.StyleName, p.StyleID} {
		if l, ok := options.CustomStyleLevels[strings.ToUpper(strings.TrimSpace(style))]; ok && l > 0 {
			if l >= options.MinHeadingLevel && l <= options.MaxHeadingLevel {
				return l
			}
		}
	}
	return 0
}

func (r *htmlRenderer) tocParagraphPlainText(p *resolve.Paragraph, preserveTabs bool) string {
	if p == nil {
		return ""
	}
	var sb strings.Builder
	for runIdx, run := range p.Runs {
		if preserveTabs && run.Kind == "tab" {
			sb.WriteByte('\t')
			continue
		}
		if run.FieldKind == "SEQ" {
			control := r.seqControlForRun(run)
			if !control.hide {
				if control.totalCount {
					total := r.sequenceTotalCount(run.FieldArg)
					if total > 0 {
						sb.WriteString(r.formatSequenceDisplayValue(total, control, p))
					}
				} else if value, ok := r.sequenceRunValue(p.Index, runIdx); ok {
					sb.WriteString(r.formatSequenceDisplayValue(value, control, p))
				}
			}
			continue
		}
		sb.WriteString(run.Text)
	}
	return strings.TrimSpace(sb.String())
}

func (r *htmlRenderer) tocCaptionText(p *resolve.Paragraph, options tocOptions) (string, bool) {
	if p == nil || options.CaptionSequence == "" {
		return "", false
	}
	var sb strings.Builder
	matchedCaption := false
	omitPrefix := options.CaptionTextOnly
	stripLeading := false
	for runIdx, run := range p.Runs {
		if run.FieldKind == "SEQ" && captionSequenceMatches(run.FieldArg, options.CaptionSequence) {
			matchedCaption = true
			if omitPrefix {
				omitPrefix = false
				stripLeading = true
				continue
			}
			control := r.seqControlForRun(run)
			if !control.hide {
				if control.totalCount {
					total := r.sequenceTotalCount(run.FieldArg)
					if total > 0 {
						sb.WriteString(r.formatSequenceDisplayValue(total, control, p))
					}
				} else if value, ok := r.sequenceRunValue(p.Index, runIdx); ok {
					sb.WriteString(r.formatSequenceDisplayValue(value, control, p))
				}
			}
			continue
		}
		if omitPrefix {
			continue
		}
		if options.PreserveTabs && run.Kind == "tab" {
			if stripLeading {
				continue
			}
			sb.WriteByte('\t')
			continue
		}
		text := run.Text
		if stripLeading {
			text = strings.TrimLeft(text, " \t:.-–—")
			stripLeading = false
		}
		sb.WriteString(text)
	}
	if !matchedCaption {
		return "", false
	}
	return strings.TrimSpace(sb.String()), true
}

func parseTOCHeadingRange(instr string) int {
	idx, width := findFieldSwitch(instr, "O")
	if idx < 0 {
		return 3
	}
	value, ok := extractFieldSwitchValue(instr[idx+width:])
	if !ok {
		return 3
	}
	parts := strings.SplitN(value, "-", 2)
	if len(parts) != 2 {
		return 3
	}
	maxLevel, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil || maxLevel < 1 {
		return 3
	}
	if maxLevel > 6 {
		return 6
	}
	return maxLevel
}

// parseTOCLevelRange parses the \l switch which specifies a range of outline
// levels to include. Returns minLevel, maxLevel, and true when the switch is
// present. Defaults are 1 and 3 when absent.
func parseTOCLevelRange(instr string) (minLevel, maxLevel int, ok bool) {
	idx, width := findFieldSwitch(instr, "L")
	if idx < 0 {
		return 1, 3, false
	}
	value, found := extractFieldSwitchValue(instr[idx+width:])
	if !found {
		return 1, 3, true
	}
	parts := strings.SplitN(value, "-", 2)
	minLevel = 1
	maxLevel = 3
	if n, err := strconv.Atoi(strings.TrimSpace(parts[0])); err == nil && n >= 1 {
		minLevel = n
	}
	if len(parts) == 2 {
		if n, err := strconv.Atoi(strings.TrimSpace(parts[1])); err == nil && n >= 1 {
			maxLevel = n
		}
	}
	if maxLevel < minLevel {
		maxLevel = minLevel
	}
	if maxLevel > 6 {
		maxLevel = 6
	}
	return minLevel, maxLevel, true
}

func parseTOCTCFilter(instr string) (bool, string) {
	idx, width := findFieldSwitch(instr, "F")
	if idx < 0 {
		return false, ""
	}
	value, ok := extractFieldSwitchValue(instr[idx+width:])
	if !ok {
		return true, ""
	}
	return true, strings.TrimSpace(value)
}

func parseTOCCaptionSequence(instr string) string {
	for _, switchName := range []string{"C", "A"} {
		idx, width := findFieldSwitch(instr, switchName)
		if idx < 0 {
			continue
		}
		value, ok := extractFieldSwitchValue(instr[idx+width:])
		if !ok {
			return ""
		}
		return strings.TrimSpace(value)
	}
	return ""
}

func parseTOCCaptionTextOnly(instr string) bool {
	return hasFieldSwitch(instr, "A")
}

func parseTOCCustomStyles(instr string) map[string]int {
	idx, width := findFieldSwitch(instr, "T")
	if idx < 0 {
		return nil
	}
	value, ok := extractFieldSwitchValue(instr[idx+width:])
	if !ok || strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	if len(parts) < 2 {
		return nil
	}
	styles := make(map[string]int)
	for i := 0; i+1 < len(parts); i += 2 {
		name := strings.ToUpper(strings.TrimSpace(parts[i]))
		level, err := strconv.Atoi(strings.TrimSpace(parts[i+1]))
		if name == "" || err != nil || level < 1 {
			continue
		}
		styles[name] = level
	}
	if len(styles) == 0 {
		return nil
	}
	return styles
}

func parseTOCSuppressPageNumbers(instr string) bool {
	// Word's \n switch controls page-number suppression. The \h switch makes
	// TOC entries hyperlinks; it does not alter entry selection or page numbers.
	return hasFieldSwitch(instr, "N")
}

// parseTOCUseOutline returns true when the TOC instruction contains a \u switch.
// \u tells Word to use applied paragraph outline levels in addition to heading styles.
func parseTOCUseOutline(instr string) bool {
	return hasFieldSwitch(instr, "U")
}

// parseTOCWebView returns true when the TOC instruction contains a \z switch.
// Word applies \z only in Web Layout view. Our paginated HTML models Word's
// page/print layout, so this switch must not suppress page numbers here.
func parseTOCWebView(instr string) bool {
	return hasFieldSwitch(instr, "Z")
}

func parseTOCSeparator(instr string) string {
	idx, width := findFieldSwitch(instr, "D")
	if idx < 0 {
		return ""
	}
	value, ok := extractFieldSwitchValue(instr[idx+width:])
	if !ok {
		return ""
	}
	return value
}

func parseTOCPageSeparator(instr string) string {
	idx, width := findFieldSwitch(instr, "P")
	if idx < 0 {
		return ""
	}
	value, ok := extractFieldSwitchValue(instr[idx+width:])
	if !ok {
		return ""
	}
	return value
}

func parseTOCPageSequence(instr string) string {
	idx, width := findFieldSwitch(instr, "S")
	if idx < 0 {
		return ""
	}
	value, ok := extractFieldSwitchValue(instr[idx+width:])
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

func parseTOCBookmark(instr string) string {
	idx, width := findFieldSwitch(instr, "B")
	if idx < 0 {
		return ""
	}
	value, ok := extractFieldSwitchValue(instr[idx+width:])
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

func parseTOCPreserveTabs(instr string) bool {
	return hasFieldSwitch(instr, "W")
}

func hasFieldSwitch(instr, switchName string) bool {
	idx, _ := findFieldSwitch(instr, switchName)
	return idx >= 0
}

func findFieldSwitch(instr, switchName string) (idx, width int) {
	upper := strings.ToUpper(instr)
	for _, token := range []string{`\` + strings.ToUpper(switchName), `\\` + strings.ToUpper(switchName)} {
		if i := strings.Index(upper, token); i >= 0 {
			return i, len(token)
		}
	}
	return -1, 0
}

func extractFieldSwitchValue(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", false
	}
	if strings.HasPrefix(s, `"`) {
		value, _ := extractIfOperand(s)
		return value, value != ""
	}
	value, _ := extractIfOperand(s)
	return value, value != ""
}

type tcEntry struct {
	text             string
	level            int
	id               string
	preserveNewlines bool
}

func parseTCEntry(instr string) (tcEntry, bool) {
	rest := strings.TrimSpace(instr)
	if len(rest) >= 2 && strings.EqualFold(rest[:2], "TC") {
		rest = strings.TrimSpace(rest[2:])
	}
	text, rest := extractIfOperand(rest)
	if text == "" {
		return tcEntry{}, false
	}
	entry := tcEntry{text: text, level: 1}
	for rest != "" {
		rest = strings.TrimSpace(rest)
		if !strings.HasPrefix(rest, `\\`) || len(rest) < len(`\\L`) {
			break
		}
		switchToken := strings.ToUpper(rest[:len(`\\L`)])
		rest = strings.TrimSpace(rest[len(`\\L`):])
		switch switchToken {
		case `\\L`:
			var levelText string
			levelText, rest = extractIfOperand(rest)
			if level, err := strconv.Atoi(strings.TrimSpace(levelText)); err == nil && level > 0 {
				entry.level = level
			}
		case `\\F`:
			entry.id, rest = extractIfOperand(rest)
		case `\\X`:
			entry.preserveNewlines = true
		default:
			_, rest = extractIfOperand(rest)
		}
	}
	return entry, true
}

func formatTOCEntry(entry tocEntry, includePage bool, separator, pageSeparator string) string {
	indent := strings.Repeat("  ", max(entry.Level-1, 0))
	text := indent + entry.Text
	if !includePage {
		return text
	}
	page := strconv.Itoa(entry.Page)
	if entry.PageSequence > 0 {
		if separator == "" {
			separator = "-"
		}
		page = strconv.Itoa(entry.PageSequence) + separator + page
	}
	if pageSeparator != "" {
		return text + " " + pageSeparator + " " + page
	}
	if separator != "" && entry.PageSequence == 0 {
		return text + " " + separator + " " + page
	}
	dotCount := 6
	if len(text)+len(page) < 56 {
		dotCount = 56 - len(text) - len(page)
	}
	if dotCount < 4 {
		dotCount = 4
	}
	return text + " " + strings.Repeat(".", dotCount) + " " + page
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// compareIfOperands compares left op right and returns true/false.
func compareIfOperands(left, right, op string) bool {
	// Try numeric comparison first.
	lf, lErr := strconv.ParseFloat(left, 64)
	rf, rErr := strconv.ParseFloat(right, 64)
	numeric := lErr == nil && rErr == nil
	switch strings.ToUpper(op) {
	case "=", "==":
		if numeric {
			return lf == rf
		}
		return left == right
	case "<>", "!=":
		if numeric {
			return lf != rf
		}
		return left != right
	case "<":
		if numeric {
			return lf < rf
		}
		return left < right
	case ">":
		if numeric {
			return lf > rf
		}
		return left > right
	case "<=":
		if numeric {
			return lf <= rf
		}
		return left <= right
	case ">=":
		if numeric {
			return lf >= rf
		}
		return left >= right
	}
	return false
}

func (r *htmlRenderer) withOutputBuffer(target *bytes.Buffer, fn func()) {
	if target == &r.buf {
		fn()
		return
	}
	saved := r.buf
	r.buf = *target
	fn()
	*target = r.buf
	r.buf = saved
}

// emptyParaMinHeightPt returns the minimum CSS height for an empty (or
// anchor-only) paragraph in points. This equals line-height + spacing-after so
// that the paragraph contributes the same vertical space to the flow as Word
// does — keeping paragraph-relative float anchors at their correct page
// positions.
//
// Why include spacing-after here instead of relying solely on padding-bottom:
// the global CSS reset uses box-sizing:border-box, which means padding is
// counted inside min-height rather than added to it. Without this adjustment
// every anchor paragraph would be only line-height tall (≈12.65pt) instead of
// line-height + spacing-after (≈22.65pt), collapsing the flow and pushing all
// floats toward the top of the page.
//
// Why multiply lineRatio by 1.2 for "auto" line spacing:
// In OOXML, w:line/240 is expressed relative to Word's "single" spacing, which
// equals the font's natural advance height — approximately 1.2× the point size
// (the same value a browser uses for line-height:normal). CSS line-height:1.0
// equals exactly the point size, so we scale by 1.2 to align Word's "single"
// with the CSS reference point. Without this, each empty anchor paragraph is
// about 2.5–3pt shorter than Word renders it, accumulating into significant
// vertical mis-positioning for paragraph-relative floating objects.
// emptyParaMinHeightPt returns the CSS min-height for an empty or anchor-only
// paragraph. It prefers the layout-computed height (which uses the same Word
// formula but handles all lineRule variants and contextual-spacing suppression).
// Falls back to the inline formula for paragraphs not captured by the layout
// (e.g., paragraphs inside floating text boxes).
func (r *htmlRenderer) emptyParaMinHeightPt(p *resolve.Paragraph) float64 {
	if h, ok := r.paraHeights[p.Index]; ok {
		return h
	}
	// Fallback: replicate the layout formula inline.
	fontSizePt := 12.0
	if p.DefaultRunFormat != nil && p.DefaultRunFormat.FontSize != nil {
		fontSizePt = float64(*p.DefaultRunFormat.FontSize) / 2.0
	} else if r.doc.DefaultRunFormat.FontSize != nil {
		fontSizePt = float64(*r.doc.DefaultRunFormat.FontSize) / 2.0
	}
	lineRatio := 1.2
	if p.Format.LineSpacing != nil && (p.Format.LineRule == "auto" || p.Format.LineRule == "") {
		lineRatio = math.Max(1.2, float64(*p.Format.LineSpacing)/240.0)
	}
	lineHeight := fontSizePt * lineRatio
	spacingBeforePt := 0.0
	if p.Format.SpacingBefore != nil && *p.Format.SpacingBefore > 0 {
		spacingBeforePt = twipsToPoints(*p.Format.SpacingBefore)
	}
	spacingAfterPt := 0.0
	if p.Format.SpacingAfter != nil && *p.Format.SpacingAfter > 0 {
		spacingAfterPt = twipsToPoints(*p.Format.SpacingAfter)
	}
	return spacingBeforePt + lineHeight + spacingAfterPt
}

// resolveFloatLeft is the fallback for floats not captured by the layout map.
// It computes the CSS left (pt) from the overlay origin (page top-left).
func (r *htmlRenderer) resolveFloatLeft(x int64, xAlign, relative string, widthEmu int64, _ bool) float64 {
	widthPt := emuToPoints(widthEmu)
	contentW := r.pageWidthPt - r.marginLeftPt - r.marginRightPt
	relative = strings.TrimSpace(relative)
	if xAlign != "" {
		switch xAlign {
		case "left", "inside":
			if relative == "page" {
				return 0
			}
			return r.marginLeftPt
		case "right", "outside":
			if relative == "page" {
				return r.pageWidthPt - widthPt
			}
			return r.marginLeftPt + contentW - widthPt
		case "center":
			if relative == "page" {
				return (r.pageWidthPt - widthPt) / 2
			}
			return r.marginLeftPt + (contentW-widthPt)/2
		default:
			return r.marginLeftPt
		}
	}
	offset := emuToPoints(x)
	switch relative {
	case "page":
		return offset
	case "rightMargin", "outsideMargin":
		return r.pageWidthPt - r.marginRightPt + offset
	case "column":
		// Fallback does not know the active multi-column index. Column 0 is still
		// a better semantic match than silently treating the coordinate as page.
		return r.marginLeftPt + offset
	default:
		return r.marginLeftPt + offset
	}
}

// resolveFloatTop is the fallback for floats not captured by the layout map.
// It computes the CSS top (pt) from the overlay origin (page top-left).
func (r *htmlRenderer) resolveFloatTop(y int64, yAlign, relative string, heightEmu int64, _ bool) float64 {
	heightPt := emuToPoints(heightEmu)
	contentH := r.pageHeightPt - r.marginTopPt - r.marginBottomPt
	relative = strings.TrimSpace(relative)
	if yAlign != "" {
		switch yAlign {
		case "top", "inside":
			if relative == "page" {
				return 0
			}
			return r.marginTopPt
		case "bottom", "outside":
			if relative == "page" {
				return r.pageHeightPt - heightPt
			}
			return r.marginTopPt + contentH - heightPt
		case "center":
			if relative == "page" {
				return (r.pageHeightPt - heightPt) / 2
			}
			return r.marginTopPt + (contentH-heightPt)/2
		default:
			return r.marginTopPt
		}
	}
	offset := emuToPoints(y)
	switch relative {
	case "page":
		return offset
	case "bottomMargin", "outsideMargin":
		return r.pageHeightPt - r.marginBottomPt + offset
	case "margin", "topMargin", "insideMargin", "":
		return r.marginTopPt + offset
	default:
		// paragraph/line are normally resolved by word/layout and arrive through
		// floatBy*. If that data is unavailable, top margin is the stable fallback
		// rather than leaving CSS top unset.
		return r.marginTopPt + offset
	}
}

// isBrowserImage reports whether a MIME type is natively displayable in browsers.
func isBrowserImage(mimeType string) bool {
	switch mimeType {
	case "image/png", "image/jpeg", "image/gif", "image/webp",
		"image/bmp", "image/svg+xml", "image/avif":
		return true
	}
	return false
}

// emuToPoints converts EMU (English Metric Units) to CSS pt.
// 914400 EMU = 1 inch = 72 pt → 1 EMU = 72/914400 pt = 1/12700 pt.
func emuToPoints(emu int64) float64 {
	return float64(emu) / 12700.0
}

// effectiveFontFamily returns the most applicable font name from a RunFormat,
// preferring ASCII, then hAnsi, then EastAsia, then CS.
func effectiveFontFamily(f resolve.RunFormat) string {
	if f.FontASCII != "" {
		return f.FontASCII
	}
	if f.FontHAnsi != "" {
		return f.FontHAnsi
	}
	if f.FontEastAsia != "" {
		return f.FontEastAsia
	}
	return f.FontCS
}

func (r *htmlRenderer) cssFontFamily(name string, includeFallbacks bool) string {
	if name == "" {
		return ""
	}
	families := []string{name}
	if includeFallbacks {
		families = append(families, r.fontFallbacks(name)...)
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(families))
	for _, candidate := range families {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		key := normalizeFontFamily(candidate)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		if isGenericFontFamily(candidate) {
			out = append(out, candidate)
			continue
		}
		out = append(out, quoteCSSFontFamilyName(candidate))
	}
	return strings.Join(out, ",")
}

func (r *htmlRenderer) fontFallbacks(name string) []string {
	if info, ok := r.lookupFontFamilyInfo(name); ok {
		if fallbacks := fontFallbacksFromInfo(info); len(fallbacks) > 0 {
			return fallbacks
		}
	}
	return bestEffortFontFallbacks(name)
}

func (r *htmlRenderer) lookupFontFamilyInfo(name string) (resolve.FontFamilyInfo, bool) {
	if r == nil || r.doc == nil || len(r.doc.FontFamilies) == 0 {
		return resolve.FontFamilyInfo{}, false
	}
	info, ok := r.doc.FontFamilies[normalizeFontFamily(name)]
	return info, ok
}

func fontFallbacksFromInfo(info resolve.FontFamilyInfo) []string {
	family := strings.ToLower(strings.TrimSpace(info.Family))
	pitch := strings.ToLower(strings.TrimSpace(info.Pitch))
	if pitch == "fixed" || family == "modern" {
		return []string{"Consolas", "Courier New", "monospace"}
	}
	if fallbacks := fontFallbacksFromPanose(info.Panose1); len(fallbacks) > 0 {
		return fallbacks
	}
	switch family {
	case "roman":
		return []string{"Times New Roman", "Times", "serif"}
	case "swiss":
		return []string{"Arial", "sans-serif"}
	case "script":
		return []string{"Brush Script MT", "cursive"}
	case "decorative":
		return []string{"Impact", "Arial Black", "fantasy"}
	default:
		return nil
	}
}

func fontFallbacksFromPanose(panose string) []string {
	bytes, ok := parsePanose1(panose)
	if !ok {
		return nil
	}
	switch bytes[0] {
	case 2: // Latin Text
		return latinTextPanoseFallbacks(bytes)
	case 3: // Latin Hand Written
		return []string{"Brush Script MT", "cursive"}
	case 4: // Latin Decorative
		return []string{"Impact", "Arial Black", "fantasy"}
	case 5: // Latin Symbol
		return []string{"Segoe UI Symbol", "Noto Sans Symbols", "sans-serif"}
	}
	return nil
}

func latinTextPanoseFallbacks(panose [10]byte) []string {
	if panose[3] == 9 {
		return []string{"Consolas", "Courier New", "monospace"}
	}
	switch serifStyle := panose[1]; {
	case serifStyle >= 2 && serifStyle <= 5:
		return []string{"Georgia", "Times New Roman", "serif"}
	case serifStyle >= 6 && serifStyle <= 10:
		return []string{"Times New Roman", "Times", "serif"}
	case serifStyle >= 11 && serifStyle <= 13:
		if isCondensedPanoseProportion(panose[3]) {
			return []string{"Arial Narrow", "Arial", "sans-serif"}
		}
		return []string{"Arial", "sans-serif"}
	case serifStyle == 14 || serifStyle == 15:
		return []string{"Palatino Linotype", "Book Antiqua", "serif"}
	default:
		return nil
	}
}

func isCondensedPanoseProportion(proportion byte) bool {
	switch proportion {
	case 6, 8:
		return true
	default:
		return false
	}
}

func parsePanose1(value string) ([10]byte, bool) {
	var out [10]byte
	normalized := strings.NewReplacer(" ", "", "-", "", ":", "").Replace(strings.TrimSpace(value))
	if len(normalized) != 20 {
		return out, false
	}
	for i := 0; i < len(out); i++ {
		raw, err := strconv.ParseUint(normalized[i*2:i*2+2], 16, 8)
		if err != nil {
			return out, false
		}
		out[i] = byte(raw)
	}
	return out, true
}

func cssLigatureVariant(value string) string {
	switch normalizeOpenTypeValue(value) {
	case "", "none":
		if normalizeOpenTypeValue(value) == "none" {
			return "none"
		}
		return ""
	case "standard":
		return "common-ligatures"
	case "contextual":
		return "contextual"
	case "historical":
		return "historical-ligatures"
	case "discretional", "discretionary":
		return "discretionary-ligatures"
	case "standardcontextual", "contextualstandard":
		return "common-ligatures contextual"
	case "standardhistorical":
		return "common-ligatures historical-ligatures"
	case "standarddiscretional", "standarddiscretionary":
		return "common-ligatures discretionary-ligatures"
	case "contextualhistorical":
		return "contextual historical-ligatures"
	case "contextualdiscretional", "contextualdiscretionary":
		return "contextual discretionary-ligatures"
	case "discretionalhistorical", "discretionaryhistorical":
		return "discretionary-ligatures historical-ligatures"
	case "standardcontextualhistorical":
		return "common-ligatures contextual historical-ligatures"
	case "standardcontextualdiscretional", "standardcontextualdiscretionary":
		return "common-ligatures contextual discretionary-ligatures"
	case "standarddiscretionalhistorical", "standarddiscretionaryhistorical":
		return "common-ligatures discretionary-ligatures historical-ligatures"
	case "contextualdiscretionalhistorical", "contextualdiscretionaryhistorical":
		return "contextual discretionary-ligatures historical-ligatures"
	case "all", "standardcontextualdiscretionalhistorical", "standardcontextualdiscretionaryhistorical":
		return "common-ligatures contextual discretionary-ligatures historical-ligatures"
	default:
		return ""
	}
}

func cssTextFillColor(fill dml.Fill) string {
	switch typed := fill.(type) {
	case nil:
		return ""
	case *dml.NoFill:
		return "transparent"
	case dml.NoFill:
		return "transparent"
	case *dml.SolidFill:
		return cssSolidFillColor(*typed)
	case dml.SolidFill:
		return cssSolidFillColor(typed)
	default:
		return ""
	}
}

func cssSolidFillColor(fill dml.SolidFill) string {
	if fill.Color == "" {
		return ""
	}
	if fill.Alpha > 0 && fill.Alpha < 1 {
		return cssRGBA(fill.Color, fill.Alpha)
	}
	return "#" + fill.Color
}

func cssTextOutlineStroke(line *dml.LineStyle) string {
	if line == nil {
		return ""
	}
	if line.Alpha == 0 {
		return ""
	}
	width := line.Width
	if width <= 0 {
		width = 0.75
	}
	if line.Color == "" {
		return fmt.Sprintf("%.2fpt currentColor", width)
	}
	if line.Alpha > 0 && line.Alpha < 1 {
		return fmt.Sprintf("%.2fpt %s", width, cssRGBA(line.Color, line.Alpha))
	}
	return fmt.Sprintf("%.2fpt #%s", width, line.Color)
}

func cssRunTextShadow(shadow *dml.Shadow, glow *dml.Glow) string {
	parts := make([]string, 0, 2)
	if shadow != nil && shadow.Color != "" {
		rad := shadow.Angle * math.Pi / 180
		offsetX := shadow.Distance * math.Cos(rad)
		offsetY := shadow.Distance * math.Sin(rad)
		parts = append(parts, fmt.Sprintf("%.2fpt %.2fpt %.2fpt %s", offsetX, offsetY, shadow.Blur, cssEffectColor(shadow.Color, shadow.Alpha)))
	}
	if glow != nil && glow.Color != "" {
		parts = append(parts, fmt.Sprintf("0 0 %.2fpt %s", glow.Radius, cssEffectColor(glow.Color, glow.Alpha)))
	}
	return strings.Join(parts, ", ")
}

// cssTextReflection returns a CSS approximation for Word text reflection.
// Uses -webkit-box-reflect when available; falls back to omitting the effect.
func cssTextReflection(reflection *dml.Reflection) string {
	if reflection == nil {
		return ""
	}
	// Map direction: 0=below, 180=above, 90=right, 270=left
	dir := "below"
	switch reflection.Direction {
	case 180:
		dir = "above"
	case 90:
		dir = "right"
	case 270:
		dir = "left"
	}
	offset := reflection.Distance
	if offset < 0 {
		offset = 0
	}
	// Build a gradient mask that fades the reflection.
	// Size controls how much of the reflection is visible (0.0–1.0).
	stop := 100
	if reflection.Size > 0 && reflection.Size < 1 {
		stop = int(reflection.Size * 100)
	}
	alpha := 1.0 - reflection.Alpha
	if alpha < 0 {
		alpha = 0
	}
	if alpha > 1 {
		alpha = 1
	}
	mask := fmt.Sprintf("linear-gradient(%s, rgba(0,0,0,%.2f) 0%%, transparent %d%%)", dir, alpha, stop)
	return fmt.Sprintf("-webkit-box-reflect: %s %.2fpt %s", dir, offset, mask)
}

func cssEffectColor(color string, alpha float64) string {
	if alpha > 0 && alpha < 1 {
		return cssRGBA(color, alpha)
	}
	return "#" + color
}

func cssRGBA(hex string, alpha float64) string {
	r, g, b, ok := parseCSSHexRGB(hex)
	if !ok {
		return "rgba(0, 0, 0, 0)"
	}
	return fmt.Sprintf("rgba(%d, %d, %d, %.2f)", r, g, b, alpha)
}

func parseCSSHexRGB(hex string) (uint8, uint8, uint8, bool) {
	if len(hex) != 6 {
		return 0, 0, 0, false
	}
	raw, err := strconv.ParseUint(hex, 16, 32)
	if err != nil {
		return 0, 0, 0, false
	}
	return uint8(raw >> 16), uint8(raw >> 8), uint8(raw), true
}

func cssNumericSpacingVariant(value string) string {
	switch normalizeOpenTypeValue(value) {
	case "proportional":
		return "proportional-nums"
	case "tabular":
		return "tabular-nums"
	default:
		return ""
	}
}

func cssStylisticSetFeatureSettings(ids []int) string {
	if len(ids) == 0 {
		return ""
	}
	cloned := append([]int(nil), ids...)
	sort.Ints(cloned)
	parts := make([]string, 0, len(cloned))
	seen := map[int]struct{}{}
	for _, id := range cloned {
		if id <= 0 || id > 99 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		parts = append(parts, fmt.Sprintf("'ss%02d' 1", id))
	}
	return strings.Join(parts, ", ")
}

// cssTextEmphasis returns CSS text-emphasis-style declarations for East Asian
// emphasis marks (w:em). Supported Word values: dot, comma, circle, underDot.
func cssTextEmphasis(f resolve.RunFormat) []string {
	var styles []string
	switch f.EmphasisMark {
	case "dot":
		styles = append(styles, "text-emphasis-style:dot")
	case "comma":
		styles = append(styles, "text-emphasis-style:','")
	case "circle":
		styles = append(styles, "text-emphasis-style:circle")
	case "underDot":
		styles = append(styles, "text-emphasis-style:dot", "text-emphasis-position:under")
	}
	return styles
}

func (r *htmlRenderer) paragraphNeedsFontAlignCSS(p *resolve.Paragraph) bool {
	if p == nil {
		return false
	}
	var first float64
	seen := false
	for _, run := range resolve.CanonicalVisibleRuns(p) {
		if strings.TrimSpace(run.Text) == "" || run.Kind == "tab" || run.Kind == "break" {
			continue
		}
		size := r.effectiveRunFontSizePt(run.Format, p.DefaultRunFormat)
		if !seen {
			first = size
			seen = true
			continue
		}
		if math.Abs(size-first) > 0.01 {
			return true
		}
	}
	return false
}

func (r *htmlRenderer) effectiveRunFontSizePt(format resolve.RunFormat, paragraphDefault *resolve.RunFormat) float64 {
	if format.FontSize != nil && *format.FontSize > 0 {
		return float64(*format.FontSize) / 2.0
	}
	if paragraphDefault != nil && paragraphDefault.FontSize != nil && *paragraphDefault.FontSize > 0 {
		return float64(*paragraphDefault.FontSize) / 2.0
	}
	if r.doc != nil && r.doc.DefaultRunFormat.FontSize != nil && *r.doc.DefaultRunFormat.FontSize > 0 {
		return float64(*r.doc.DefaultRunFormat.FontSize) / 2.0
	}
	return 12.0
}

// breakClearCSS maps Word break clear values to CSS clear properties.
func breakClearCSS(val string) string {
	switch val {
	case "left":
		return "clear:left"
	case "right":
		return "clear:right"
	case "all":
		return "clear:both"
	default:
		return ""
	}
}

// cssFontAlign maps Word paragraph text-alignment values (w:textAlignment /
// sprmPWAlignFont) to CSS vertical-align keywords. Returns "" for "auto",
// "baseline" (the CSS default), or unknown values.
func cssFontAlign(val string) string {
	switch val {
	case "top":
		return "top"
	case "center":
		return "middle"
	case "bottom":
		return "bottom"
	default:
		return ""
	}
}

// cssUnderlineStyle maps Word underline style values to CSS
// text-decoration-style keywords. Returns "" when no CSS equivalent exists.
func cssUnderlineStyle(val string) string {
	switch val {
	case "double":
		return "double"
	case "dotted", "dottedHeavy":
		return "dotted"
	case "dash", "dashedHeavy", "dashLong", "dashLongHeavy":
		return "dashed"
	case "wave", "wavyHeavy", "wavyDouble":
		return "wavy"
	default:
		return ""
	}
}

func normalizeOpenTypeValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return strings.NewReplacer("-", "", "_", "", " ", "").Replace(strings.ToLower(value))
}

// bestEffortFontFallbacks is the CSS stack a browser needs after an authored
// font family. It is the same table the layout measures with, so a document
// whose fonts are not installed still breaks lines against the face the
// browser will actually paint.
func bestEffortFontFallbacks(name string) []string {
	return wordfontmetrics.FamilyFallbacks(name)
}

func normalizeFontFamily(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func inferredFontFamilyWeight(name string) string {
	lower := normalizeFontFamily(name)
	switch {
	case strings.Contains(lower, "thin"), strings.Contains(lower, "hairline"):
		return "200"
	case strings.Contains(lower, "light"):
		return "300"
	case strings.Contains(lower, "medium"):
		return "500"
	case strings.Contains(lower, "semibold"), strings.Contains(lower, "semi bold"), strings.Contains(lower, "demi"), strings.Contains(lower, "demi bold"):
		return "600"
	case strings.Contains(lower, "extrabold"), strings.Contains(lower, "extra bold"), strings.Contains(lower, "ultrabold"), strings.Contains(lower, "ultra bold"):
		return "800"
	case strings.Contains(lower, "black"), strings.Contains(lower, "heavy"):
		return "900"
	default:
		return ""
	}
}

func isGenericFontFamily(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "serif", "sans-serif", "monospace", "cursive", "fantasy", "system-ui", "math", "emoji", "fangsong":
		return true
	default:
		return false
	}
}

func quoteCSSFontFamilyName(name string) string {
	if name == "" {
		return name
	}
	if strings.ContainsAny(name, " ,\"'") {
		return "'" + strings.ReplaceAll(name, "'", "\\'") + "'"
	}
	return name
}

func (r *htmlRenderer) isEmbeddedFont(name string) bool {
	if r == nil {
		return false
	}
	_, ok := r.embeddedFamilies[normalizeFontFamily(name)]
	return ok
}

func buildEmbeddedFamilyIndex(doc *resolve.Document) map[string]struct{} {
	index := map[string]struct{}{}
	if doc == nil {
		return index
	}
	for _, font := range doc.EmbeddedFonts {
		if font == nil || strings.TrimSpace(font.Family) == "" {
			continue
		}
		index[normalizeFontFamily(font.Family)] = struct{}{}
	}
	return index
}

func buildWordFontMetricsCatalog(doc *resolve.Document) *fontmetrics.Catalog {
	return wordfontmetrics.BuildCatalog(doc)
}

func (r *htmlRenderer) writeEmbeddedFontCSS() {
	if r == nil || r.doc == nil {
		return
	}
	for _, font := range r.doc.EmbeddedFonts {
		if font == nil || strings.TrimSpace(font.Family) == "" {
			continue
		}
		for _, face := range font.Faces {
			if face == nil || len(face.Data) == 0 {
				continue
			}
			sources := fontdata.EmbeddedSources(face.Data, face.ContentType, face.Family)
			if len(sources) == 0 {
				continue
			}
			fmt.Fprintf(&r.buf, "@font-face{font-family:%s;font-style:%s;font-weight:%d;font-display:swap;src:",
				quoteCSSFontFamilyName(face.Family),
				fontStyleValue(face.Style),
				fontWeightValue(face.Weight),
			)
			for i, src := range sources {
				if i > 0 {
					r.buf.WriteByte(',')
				}
				if r.options.FontMode == HTMLFontModeExternal {
					fmt.Fprintf(&r.buf, "url(%q) format(%q)", r.addFontAsset(src), src.Format)
					continue
				}
				encoded := base64.StdEncoding.EncodeToString(src.Data)
				fmt.Fprintf(&r.buf, "url(data:%s;base64,%s) format(%q)", src.MIME, encoded, src.Format)
			}
			r.buf.WriteString(";}\n")
		}
	}
}

func (r *htmlRenderer) addFontAsset(src fontdata.Source) string {
	if r == nil {
		return ""
	}
	r.nextFontIndex++
	assetPath := htmlasset.AssetPath(r.options.ExternalFontDir, "font", r.nextFontIndex, htmlasset.FontFileExtension(src.MIME, src.Format))
	r.assets = append(r.assets, HTMLAsset{
		Path:        assetPath,
		ContentType: src.MIME,
		Data:        append([]byte(nil), src.Data...),
	})
	return assetPath
}

func fontStyleValue(style string) string {
	if style == "italic" {
		return "italic"
	}
	return "normal"
}

func fontWeightValue(weight int) int {
	if weight >= 700 {
		return 700
	}
	return 400
}

// twipsToPoints converts EMU-sized twips (1/20 of a point) to CSS pt values.
func twipsToPoints(twips int) float64 {
	return float64(twips) / 20.0
}

// borderSideCSS returns CSS declarations for a single border side (e.g. "border-top").
// Returns nil if the side is nil (no border on that edge).
// listOpenElement returns the opening HTML tag string (including newline) for a
// list at the given depth. When atTarget is true (we're opening the innermost
// level for the current paragraph), the list format and start value are applied.
func listOpenElement(tag string, list *resolve.ListMarker, atTarget bool) string {
	if !atTarget || list == nil {
		return "<" + tag + ">\n"
	}
	var parts []string
	if tag == "ol" {
		if list.Start > 1 {
			parts = append(parts, fmt.Sprintf("start=\"%d\"", list.Start))
		}
	} else {
		// Bullet list: derive list-style-type from the template character.
		if css := bulletStyleTypeCSS(list.Template); css != "" {
			parts = append(parts, fmt.Sprintf("style=\"list-style-type:%s\"", css))
		}
	}
	if len(parts) == 0 {
		return "<" + tag + ">\n"
	}
	return "<" + tag + " " + strings.Join(parts, " ") + ">\n"
}

// listStyleTypeCSS maps an OOXML numbering format to a CSS list-style-type value.
// Returns "" for decimal (the browser default for <ol>) to avoid redundant attributes.
func listStyleTypeCSS(format string) string {
	switch format {
	case "upperRoman":
		return "upper-roman"
	case "lowerRoman":
		return "lower-roman"
	case "upperLetter":
		return "upper-alpha"
	case "lowerLetter":
		return "lower-alpha"
	case "ordinal":
		return "decimal" // browsers don't support ordinal; fall back to decimal
	default:
		return "" // "decimal" is the default — omit the attribute
	}
}

// bulletStyleTypeCSS maps a bullet template character to a CSS list-style-type value.
// Returns "" when the character matches the browser's default bullet (•).

func bulletMarkerText(list *resolve.ListMarker, numbering *resolve.Numbering) string {
	marker := ""
	if list != nil {
		marker = strings.TrimSpace(list.Template)
	}
	if numbering != nil && numbering.Text != "" && !strings.Contains(numbering.Text, "%") {
		marker = strings.TrimSpace(numbering.Text)
	}
	switch marker {
	case "", "binary", "%1.":
		return "•"
	case "o":
		return "○"
	default:
		return normalizeOfficeBulletGlyph(marker)
	}
}

// normalizeOfficeBulletGlyph converts the private-use characters commonly
// stored by Word for Symbol/Wingdings list bullets into equivalent Unicode
// symbols. In a DOC these characters are only meaningful together with their
// legacy symbol font; emitting the PUA code point directly into HTML produces
// visible glyphs such as "" when that exact font is unavailable or its
// formatting was not retained on the list level. Unicode markers are stable
// across browsers, PDF printers, and platforms.
func normalizeOfficeBulletGlyph(marker string) string {
	switch marker {
	case "\uF0B7": // Symbol 0xB7: bullet
		return "•"
	case "\uF0A7": // Wingdings: small square
		return "▪"
	case "\uF0A8": // Wingdings: hollow square
		return "□"
	case "\uF0D8": // Wingdings: arrow head
		return "➢"
	case "\uF0FC": // Wingdings: check mark
		return "✓"
	case "\uF076": // Wingdings: diamond
		return "◆"
	default:
		return marker
	}
}

func bulletStyleTypeCSS(template string) string {
	switch strings.TrimSpace(template) {
	case "•", "%1.":
		return "" // browser default disc
	case "o", "○", "\u25E6":
		return "circle"
	case "▪", "■":
		return "square"
	case "-", "–", "—":
		return "\"\\2013\"" // en-dash via CSS custom string
	default:
		return ""
	}
}

func hasTabRuns(runs []resolve.Run) bool {
	for _, run := range runs {
		if run.Kind == "tab" {
			return true
		}
	}
	return false
}

func (r *htmlRenderer) resetTabState(p *resolve.Paragraph) {
	r.tabStops = nil
	r.tabDefault = r.doc.DefaultTabStop
	if r.tabDefault <= 0 {
		r.tabDefault = 720 // Word default: 0.5 inch = 720 twips
	}
	r.tabIndex = 0
	r.tabEstimatedX = 0
	if p == nil {
		return
	}
	if p.Format.IndentLeft != nil && *p.Format.IndentLeft > 0 {
		r.tabEstimatedX = twipsToPoints(*p.Format.IndentLeft)
	}
	if len(p.Format.TabStops) > 0 {
		r.tabStops = make([]resolve.TabStop, len(p.Format.TabStops))
		copy(r.tabStops, p.Format.TabStops)
		sort.Slice(r.tabStops, func(i, j int) bool {
			return r.tabStops[i].Pos < r.tabStops[j].Pos
		})
	}
}

func (r *htmlRenderer) clearTabState() {
	r.tabStops = nil
	r.tabDefault = 0
	r.tabIndex = 0
	r.tabEstimatedX = 0
}

func (r *htmlRenderer) renderTab() {
	r.renderTabAligned(0, 0)
}

func (r *htmlRenderer) renderTabAligned(fieldWidth, decimalPrefix float64) {
	if r.tabStops == nil && r.currentParagraph != nil {
		r.resetTabState(r.currentParagraph)
	}
	stop := inlineposition.NextTabStop(r.tabStops, r.tabEstimatedX, r.tabDefault)
	width := inlineposition.AlignedTabAdvance(stop, r.tabEstimatedX, fieldWidth, decimalPrefix)
	if width <= 0 {
		width = inlineposition.AlignedTabAdvance(
			inlineposition.NextTabStop(nil, r.tabEstimatedX, r.tabDefault),
			r.tabEstimatedX, 0, 0,
		)
	}
	leaderStyle := tabLeaderCSS(stop.Leader)
	if r.showFormattingMarks {
		fmt.Fprintf(&r.buf, `<span style="display:inline-block;width:%.1fpt;border-bottom:1px dotted #888;color:#888;font-size:0.75em;white-space:nowrap">→</span>`, width)
	} else if leaderStyle != "" {
		fmt.Fprintf(&r.buf, `<span style="display:inline-block;width:%.1fpt;%s"></span>`, width, leaderStyle)
	} else {
		fmt.Fprintf(&r.buf, `<span style="display:inline-block;width:%.1fpt"></span>`, width)
	}
	r.tabEstimatedX += width
	r.advanceState.InlineXPt = r.tabEstimatedX
	r.tabIndex++
}

// tabLeaderCSS returns a CSS border-bottom style for the given Word tab leader.
func tabLeaderCSS(leader string) string {
	switch leader {
	case "dot":
		return "border-bottom:1px dotted #000"
	case "hyphen":
		return "border-bottom:1px dashed #000"
	case "underscore":
		return "border-bottom:1px solid #000"
	case "middleDot":
		return "border-bottom:1px dotted #000"
	default:
		return ""
	}
}

func (r *htmlRenderer) peekAheadTabFieldMetrics(runs []resolve.Run, startIdx int) (fieldWidth, decimalPrefix float64) {
	decimalFound := false
	for i := startIdx; i < len(runs); i++ {
		run := runs[i]
		if run.Kind == "tab" || run.Kind == "break" {
			break
		}
		if run.Text == "" || run.Kind == "image" || run.Kind == "chart" || run.Kind == "shape" || run.Kind == "textbox" {
			continue
		}
		text := strings.ReplaceAll(run.Text, "\n", "")
		fieldWidth += r.estimateTextWidthForRun(text, run.Format)
		if !decimalFound {
			if cut := strings.IndexAny(text, ".,"); cut >= 0 {
				decimalPrefix += r.estimateTextWidthForRun(text[:cut], run.Format)
				decimalFound = true
			} else {
				decimalPrefix += r.estimateTextWidthForRun(text, run.Format)
			}
		}
	}
	return fieldWidth, decimalPrefix
}

func (r *htmlRenderer) peekAheadTextWidth(runs []resolve.Run, startIdx int) float64 {
	w, _ := r.peekAheadTabFieldMetrics(runs, startIdx)
	return w
}

// estimateTextWidthForRun returns a better width estimate using per-character
// Unicode width factors (borrowed from the layout engine). This makes tab-stop
// alignment significantly more accurate for mixed-script text.
func (r *htmlRenderer) estimateTextWidthForRun(text string, format resolve.RunFormat) float64 {
	defaultSizePt := 12.0
	if r != nil && r.doc != nil && r.doc.DefaultRunFormat.FontSize != nil && *r.doc.DefaultRunFormat.FontSize > 0 {
		defaultSizePt = float64(*r.doc.DefaultRunFormat.FontSize) / 2.0
	}
	width, _ := wordtextmetrics.RunWidthPt(r.fontMetrics, format, strings.ReplaceAll(text, "\n", ""), defaultSizePt)
	return width
}

func (r *htmlRenderer) pageBorderStyleAttr() string {
	if r.pageBorders == nil {
		return ""
	}
	if r.doc.DoNotDisplayPageBoundaries {
		return ""
	}
	switch r.pageBorders.Display {
	case "firstPage":
		if r.pageIndex != 0 {
			return ""
		}
	case "notFirstPage":
		if r.pageIndex == 0 {
			return ""
		}
	}
	var styles []string
	if r.pageBorders.Top != nil && !r.doc.BordersDoNotSurroundHeader {
		styles = append(styles, borderSideCSS("border-top", r.pageBorders.Top)...)
	}
	leftBorder := effectivePageBorderLeft(r.pageBorders)
	if leftBorder != nil {
		styles = append(styles, borderSideCSS("border-left", leftBorder)...)
	}
	if r.pageBorders.Bottom != nil && !r.doc.BordersDoNotSurroundFooter {
		styles = append(styles, borderSideCSS("border-bottom", r.pageBorders.Bottom)...)
	}
	rightBorder := effectivePageBorderRight(r.pageBorders)
	if rightBorder != nil {
		styles = append(styles, borderSideCSS("border-right", rightBorder)...)
	}
	if len(styles) == 0 {
		return ""
	}
	return strings.Join(styles, ";")
}

func borderSideCSS(prop string, side *resolve.BorderSide) []string {
	if side == nil {
		return []string{prop + ":none"}
	}
	style := "solid"
	switch side.Val {
	case "dashed":
		style = "dashed"
	case "dotted":
		style = "dotted"
	case "double":
		style = "double"
	case "none", "nil":
		return []string{prop + ":none"}
	}
	color := "#000"
	if side.Color != "" {
		color = "#" + side.Color
	}
	return []string{fmt.Sprintf("%s:%.2fpt %s %s", prop, side.SizePt, style, color)}
}

// parseISODate parses a YYYY-MM-DDTHH:MM:SS string. It returns the zero time
// on failure so that callers can fall back to empty-string behaviour.
func parseISODate(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	for _, layout := range []string{
		"2006-01-02T15:04:05",
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04Z07:00",
		"2006-01-02T15:04Z",
		"2006-01-02 15:04:05",
		"2006-01-02",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// formatFieldDateTime extracts a \\@ switch from a Word field instruction and
// formats the given time accordingly. If no switch is present, it falls back
// to a sensible default based on the field kind.
func formatFieldDateTime(instr, fieldKind string, t time.Time) string {
	if t.IsZero() {
		return ""
	}
	// \l (last used format) — we have no application state, so fallback to system default.
	// \s (system format) — use the OS default short date/time format.
	if refHasSwitch(instr, "s") || refHasSwitch(instr, "l") {
		if fieldKind == "TIME" {
			return t.Format("3:04 PM")
		}
		return t.Format("1/2/2006")
	}
	// \b (built-in language format) — use a locale-aware default.
	if refHasSwitch(instr, "b") {
		if fieldKind == "TIME" {
			return t.Format("15:04:05")
		}
		return t.Format("2 January 2006")
	}
	wordFmt := extractFieldFormatSwitch(instr)
	if refHasSwitch(instr, "h") {
		if wordFmt == "" {
			wordFmt = "d MMMM yyyy"
		}
		return formatHebrewDate(t, wordFmt)
	}
	if wordFmt == "" {
		if fieldKind == "TIME" {
			return t.Format("3:04 PM")
		}
		return t.Format("2006-01-02")
	}
	goFmt := convertWordDateFormat(wordFmt)
	return t.Format(goFmt)
}

// extractFieldFormatSwitch pulls the quoted string after \\@ from a field
// instruction, e.g. `DATE \\@ "MMMM d, yyyy"` → `MMMM d, yyyy`.
func extractFieldFormatSwitch(instr string) string {
	idx := strings.Index(instr, "\\@")
	if idx < 0 {
		return ""
	}
	s := strings.TrimSpace(instr[idx+2:])
	if !strings.HasPrefix(s, `"`) {
		return ""
	}
	s = s[1:]
	end := strings.Index(s, `"`)
	if end < 0 {
		return s
	}
	return s[:end]
}

// convertWordDateFormat maps common Word date-format picture strings to Go
// time layouts. It is a best-effort translation; unsupported tokens are passed
// through literally, which may produce unexpected output for exotic formats.
func convertWordDateFormat(word string) string {
	// Word uses upper/lower case with specific meanings; Go uses fixed
	// reference date. We do a series of replacements from longest to
	// shortest so that e.g. "MMMM" is handled before "MMM".
	replacements := []struct{ from, to string }{
		{"yyyy", "2006"},
		{"YYYY", "2006"},
		{"yy", "06"},
		{"YY", "06"},
		{"MMMM", "January"},
		{"MMM", "Jan"},
		{"MM", "01"},
		{"M", "1"},
		{"dddd", "Monday"},
		{"ddd", "Mon"},
		{"dd", "02"},
		{"d", "2"},
		{"HH", "15"},
		{"H", "15"},
		{"hh", "03"},
		{"h", "3"},
		{"mm", "04"},
		{"m", "4"},
		{"ss", "05"},
		{"s", "5"},
		{"tt", "PM"},
		{"t", "P"},
		{"AM/PM", "PM"},
		{"am/pm", "pm"},
		{"A/P", "P"},
		{"a/p", "p"},
		{".000", ".000"},
		{".00", ".00"},
		{".0", ".0"},
	}
	goFmt := replaceQuotedLiterals(word)
	for _, r := range replacements {
		goFmt = strings.ReplaceAll(goFmt, r.from, r.to)
	}
	return goFmt
}

// replaceQuotedLiterals strips single quotes from Word date-format picture
// literals, turning 'text' into text. A doubled quote (”) becomes a single
// literal quote character.
func replaceQuotedLiterals(word string) string {
	var result strings.Builder
	for i := 0; i < len(word); i++ {
		if word[i] == '\'' {
			if i+1 < len(word) && word[i+1] == '\'' {
				result.WriteByte('\'')
				i++
				continue
			}
			j := i + 1
			for j < len(word) && word[j] != '\'' {
				j++
			}
			if j < len(word) {
				result.WriteString(word[i+1 : j])
				i = j
			} else {
				result.WriteByte(word[i])
			}
		} else {
			result.WriteByte(word[i])
		}
	}
	return result.String()
}

// runFormatToMarkerCSS extracts marker-compatible CSS properties from a run
// format (color, font-family, font-size, font-weight, font-style) for use on
// a list-item element. Returns empty string when no relevant properties are set.
func (r *htmlRenderer) resolvedListMarker(p *resolve.Paragraph) (string, bool) {
	if p == nil || p.List == nil {
		return "", false
	}
	if p.List.Kind == "ordered" {
		marker := r.paragraphNumbers[p.Index]
		if marker == "" {
			marker = r.paragraphLevelNum[p.Index]
		}
		return marker, false
	}
	return bulletMarkerText(p.List, p.Numbering), true
}

func listMarkerFallbackFontSizePt(p *resolve.Paragraph, runs []resolve.Run) float64 {
	for _, run := range runs {
		if strings.TrimSpace(run.Text) == "" {
			continue
		}
		if run.Format.FontSize != nil && *run.Format.FontSize > 0 {
			return float64(*run.Format.FontSize) / 2.0
		}
	}
	if p != nil && p.DefaultRunFormat != nil && p.DefaultRunFormat.FontSize != nil && *p.DefaultRunFormat.FontSize > 0 {
		return float64(*p.DefaultRunFormat.FontSize) / 2.0
	}
	return 0
}

func (r *htmlRenderer) listMarkerStyle(p *resolve.Paragraph, runs []resolve.Run) string {
	style := ""
	if p != nil && p.Numbering != nil && p.Numbering.RunFormat != nil {
		style = r.runFormatToMarkerCSS(*p.Numbering.RunFormat)
	}
	// Legacy LVL CHPX frequently omits the marker size. Word then uses the
	// paragraph's effective text size. Falling back to the document default
	// makes a 16pt list render with a visibly undersized 12pt bullet.
	if !strings.Contains(style, "font-size:") {
		if sizePt := listMarkerFallbackFontSizePt(p, runs); sizePt > 0 {
			if style != "" {
				style += ";"
			}
			style += fmt.Sprintf("font-size:%.1fpt", sizePt)
		}
	}
	return style
}

func (r *htmlRenderer) runFormatToMarkerCSS(f resolve.RunFormat) string {
	var parts []string
	if f.Color != "" {
		parts = append(parts, fmt.Sprintf("color:#%s", f.Color))
	}
	font := f.FontASCII
	if font == "" {
		font = f.FontHAnsi
	}
	if font != "" {
		parts = append(parts, "font-family:"+r.cssFontFamily(font, !r.isEmbeddedFont(font)))
	}
	if f.FontSize != nil && *f.FontSize > 0 {
		parts = append(parts, fmt.Sprintf("font-size:%.1fpt", float64(*f.FontSize)/2.0))
	}
	if f.Bold != nil && *f.Bold {
		parts = append(parts, "font-weight:bold")
	}
	if f.Italic != nil && *f.Italic {
		parts = append(parts, "font-style:italic")
	}
	return strings.Join(parts, ";")
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func safeOLEHref(target string) string {
	t := strings.TrimSpace(target)
	if t == "" {
		return ""
	}
	lower := strings.ToLower(t)
	if strings.HasPrefix(lower, "javascript:") || strings.HasPrefix(lower, "data:") || strings.HasPrefix(lower, "vbscript:") {
		return ""
	}
	return t
}

// oleProgIDName maps common OLE ProgIDs to human-readable labels.
func oleProgIDName(progID string) string {
	switch {
	case strings.HasPrefix(progID, "Excel.Sheet"):
		return "Microsoft Excel Worksheet"
	case strings.HasPrefix(progID, "Excel.Chart"):
		return "Microsoft Excel Chart"
	case strings.HasPrefix(progID, "Word.Document"):
		return "Microsoft Word Document"
	case strings.HasPrefix(progID, "PowerPoint.Show"), strings.HasPrefix(progID, "PowerPoint.Slide"), strings.HasPrefix(progID, "PowerPoint.Presentation"):
		return "Microsoft PowerPoint Presentation"
	case strings.HasPrefix(progID, "AcroExch"):
		return "Adobe Acrobat Document"
	case strings.HasPrefix(progID, "Visio"):
		return "Microsoft Visio Drawing"
	case strings.HasPrefix(progID, "MSProject"):
		return "Microsoft Project Document"
	case strings.HasPrefix(progID, "Equation"):
		return "Microsoft Equation"
	case progID == "":
		return "Embedded Object"
	}
	return progID
}

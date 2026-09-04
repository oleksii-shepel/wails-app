package resolve

import (
	"crypto/x509"
	"fmt"
	"html"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"office/pkg/diag"
	officeartir "office/pkg/officeart/ir"
	officeartdml "office/pkg/officeart/ooxml/drawingml"
	"office/pkg/ooxml/chart"
	dml "office/pkg/ooxml/drawing"
	"office/pkg/word/parsers/ooxml/parts"

	"office/pkg/ooxml/drawingxml"
	"office/pkg/ooxml/mathml"
	"office/pkg/ooxml/opc"
	"office/pkg/ooxml/smartart"
	smartartrender "office/pkg/ooxml/smartart/render"
	"office/pkg/ooxml/xmltree"
)

// fieldFrame tracks the state of a single complex field while parsing runs.
type fieldFrame struct {
	instr          string
	instrRuns      []Run
	result         []Run
	paraBoundaries []int   // indices into result where each paragraph's runs start
	paraCount      int     // number of paragraphs that contributed to this field
	paragraphs     []int   // paragraph indexes in the current story/document paragraph slice
	blocks         []Block // paragraph/table blocks that belong to this field
	kind           string
	arg            string
	anchor         string
	target         string
	inResult       bool
	formField      *parts.FormFieldData
}

// resolveCtx carries shared resolver state to avoid threading 8 parameters
// through every internal normalization function.
type resolveCtx struct {
	pkg                *parts.Package
	currentPart        string
	numbering          map[int]map[int]Numbering
	hyperlinks         map[string]string
	styles             map[string]Style
	charStyles         map[string]CharacterStyle
	defaultStyle       string
	defaultRun         RunFormat
	media              map[string]parts.MediaPart
	chartParts         map[string][]byte // Chart XML data keyed by relationship ID
	smartArtParts      map[string][]byte // SmartArt data model XML keyed by relationship ID
	themeColors        map[string]string // DrawingML slot name → hex RGB (e.g. "accent1" → "4472C4")
	themeFonts         map[string]string // "major" or "minor" → typeface name
	themeFmtScheme     *themeFormatIndex // theme format scheme for style-ref resolution
	footnotes          map[int]*parts.Note
	endnotes           map[int]*parts.Note
	comments           map[int]*parts.Comment
	autoTextEntries    map[string]autoTextEntry
	conditionalRunFmt  *RunFormat       // non-nil when inside a tblStylePr-matched cell
	conditionalParaFmt *ParagraphFormat // non-nil when inside a tblStylePr-matched cell
	nextParaIndex      int              // global counter for unique paragraph indices
	Diagnostics        diag.List        // accumulated parse/resolve warnings and errors
	defaultTableStyle  string           // from w:settings/w:defaultTableStyle
	fieldStack         []fieldFrame     // story-scoped complex field stack
	flushedFields      []fieldFrame     // story-spanning fields closed during this paragraph
	storyFields        []StoryField     // accumulated story-spanning fields for the current story
}

func (ctx *resolveCtx) inField() bool { return len(ctx.fieldStack) > 0 }

func (ctx *resolveCtx) inResult() bool {
	if len(ctx.fieldStack) == 0 {
		return false
	}
	return ctx.fieldStack[len(ctx.fieldStack)-1].inResult
}

func (ctx *resolveCtx) pushField(ff fieldFrame) {
	ctx.fieldStack = append(ctx.fieldStack, ff)
}

func (ctx *resolveCtx) popField() fieldFrame {
	if len(ctx.fieldStack) == 0 {
		return fieldFrame{}
	}
	top := ctx.fieldStack[len(ctx.fieldStack)-1]
	ctx.fieldStack = ctx.fieldStack[:len(ctx.fieldStack)-1]
	return top
}

func (ctx *resolveCtx) topField() *fieldFrame {
	if len(ctx.fieldStack) == 0 {
		return nil
	}
	return &ctx.fieldStack[len(ctx.fieldStack)-1]
}

func applyFieldResultLinkMetadata(ff *fieldFrame) {
	if ff == nil {
		return
	}
	for i := range ff.result {
		ff.result[i].Target = ff.target
		ff.result[i].Anchor = ff.anchor
		if ff.target != "" {
			ff.result[i].Kind = "hyperlink"
		}
	}
}

func applyFieldResultSemanticMetadata(ff *fieldFrame, instr string) {
	if ff == nil {
		return
	}
	if len(ff.result) > 0 && (ff.kind != "" || instr != "") {
		for i := range ff.result {
			ff.result[i].FieldKind = ff.kind
			ff.result[i].FieldInstr = instr
			ff.result[i].FieldArg = ff.arg
			ff.result[i].FieldInstrRuns = append([]Run(nil), ff.instrRuns...)
			ff.result[i].Anchor = ff.anchor
			if ff.target != "" {
				ff.result[i].Target = ff.target
			}
		}
	}
}

func setParagraphRuns(paragraph *Paragraph, runs []Run) {
	if paragraph == nil {
		return
	}
	paragraph.Runs = paragraph.Runs[:0]
	paragraph.Text = ""
	appendSemanticRuns(paragraph, append([]Run(nil), runs...))
}

func attachParagraphToField(ctx *resolveCtx, paragraphIndex int, block Block) {
	if ctx == nil || paragraphIndex < 0 {
		return
	}
	if ctx.inField() {
		top := ctx.topField()
		top.paragraphs = append(top.paragraphs, paragraphIndex)
		top.blocks = append(top.blocks, block)
		return
	}
	for i := range ctx.flushedFields {
		ff := &ctx.flushedFields[i]
		if ff.paraCount > len(ff.paragraphs) {
			ff.paragraphs = append(ff.paragraphs, paragraphIndex)
			ff.blocks = append(ff.blocks, block)
		}
	}
}

func attachTableToField(ctx *resolveCtx, block Block) {
	if ctx == nil {
		return
	}
	if ctx.inField() {
		top := ctx.topField()
		top.blocks = append(top.blocks, block)
		return
	}
	for i := range ctx.flushedFields {
		ff := &ctx.flushedFields[i]
		if ff.paraCount > len(ff.paragraphs) {
			ff.blocks = append(ff.blocks, block)
		}
	}
}

// flushRemainingParagraphFields flushes any unclosed fields on the stack as
// paragraph-local fields. It is used by tests and other direct callers of
// normalizeParagraph that do not go through story-scoped normalization.
func flushRemainingParagraphFields(ctx *resolveCtx, out *Paragraph, paraRunFmt RunFormat) {
	for len(ctx.fieldStack) > 0 {
		top := ctx.fieldStack[len(ctx.fieldStack)-1]
		ctx.fieldStack = ctx.fieldStack[:len(ctx.fieldStack)-1]
		if top.paraCount > 1 {
			continue // story-spanning field; leave it for the story-level flush
		}

		instr := strings.TrimSpace(top.instr)
		upper := strings.ToUpper(instr)
		kind := fieldInstrKind(upper)
		top.kind = kind
		switch kind {
		case "REF":
			top.anchor = parseFieldRef(instr)
			top.arg = top.anchor
			if top.anchor != "" {
				top.target = "#" + top.anchor
			}
		case "PAGEREF":
			top.anchor = parseFieldTarget(strings.TrimSpace(instr[len("PAGEREF"):]))
			top.arg = top.anchor
			if top.anchor != "" {
				top.target = "#" + top.anchor
			}
		case "HYPERLINK":
			url, anchor := parseFieldHyperlink(instr)
			top.arg = url
			top.target = url
			top.anchor = anchor
			if anchor != "" {
				if top.target != "" {
					top.target += "#" + anchor
				} else {
					top.target = "#" + anchor
				}
			}
		case "DOCPROPERTY", "DOCVARIABLE":
			top.arg = parseDocPropertyName(instr)
		case "STYLEREF":
			top.arg = parseFieldStyleRef(instr)
		case "FILLIN":
			top.arg = parseFieldFillinDefault(instr)
		case "SEQ", "ASK", "MERGEFIELD", "AUTOTEXT", "SET":
			top.arg = parseFieldLeadingArg(instr, kind)
		case "IF", "COMPARE", "QUOTE", "EMBED", "LINK":
			top.arg = strings.TrimSpace(instr)
		case "TOC", "INDEX", "XE", "TC":
			top.arg = strings.TrimSpace(instr)
		}

		applyFieldResultLinkMetadata(&top)
		applyFieldResultSemanticMetadata(&top, instr)

		keyword := strings.Fields(upper)
		if len(keyword) > 0 {
			switch keyword[0] {
			case "FORMTEXT", "FORMCHECKBOX", "FORMDROPDOWN":
				out.Runs = append(out.Runs, resolveFormField(keyword[0], top.formField, top.result, paraRunFmt))
				continue
			}
		}

		if kind == "TOC" || kind == "INDEX" || kind == "XE" || kind == "TC" {
			base := Run{
				Kind:           "run",
				Format:         paraRunFmt,
				FieldKind:      kind,
				FieldArg:       top.arg,
				FieldInstr:     instr,
				FieldInstrRuns: append([]Run(nil), top.instrRuns...),
			}
			if len(top.result) > 0 {
				base.Format = top.result[0].Format
				base.Text = aggregateFieldDisplayText(top.result)
			}
			appendSemanticRuns(out, []Run{base})
			continue
		}

		appendSemanticRuns(out, top.result)
	}
}

// extractStoryFields converts flushed field frames into StoryField values.
// It distributes the flat result runs across the paragraphs that contributed
// to each field, populating paragraph.Runs for each block in the StoryField.
func extractStoryFields(ctx *resolveCtx, paragraphs []Paragraph) []StoryField {
	var fields []StoryField
	for _, ff := range ctx.flushedFields {
		sf := StoryField{
			Kind:      ff.kind,
			Instr:     strings.TrimSpace(ff.instr),
			Arg:       ff.arg,
			Anchor:    ff.anchor,
			Target:    ff.target,
			InstrRuns: append([]Run(nil), ff.instrRuns...),
		}
		// Distribute result runs across paragraph blocks using paraBoundaries.
		paraIdx := 0
		for _, block := range ff.blocks {
			block.FieldKind = ff.kind
			if block.Kind == "paragraph" && block.Paragraph != nil && paraIdx < len(ff.paraBoundaries) {
				start := ff.paraBoundaries[paraIdx]
				end := len(ff.result)
				if paraIdx+1 < len(ff.paraBoundaries) {
					end = ff.paraBoundaries[paraIdx+1]
				}
				runs := append([]Run(nil), ff.result[start:end]...)
				setParagraphRuns(block.Paragraph, runs)
				if paraIdx < len(ff.paragraphs) {
					idx := ff.paragraphs[paraIdx]
					if idx >= 0 && idx < len(paragraphs) {
						setParagraphRuns(&paragraphs[idx], runs)
					}
				}
				paraIdx++
			}
			sf.Blocks = append(sf.Blocks, block)
		}
		fields = append(fields, sf)
	}
	ctx.flushedFields = ctx.flushedFields[:0]
	return fields
}

// annotateFieldBlocks sets Block.FieldKind and Block.FieldIndex on each block
// that belongs to a StoryField, using pointer identity on Paragraph/Table pointers.
func annotateFieldBlocks(blocks []Block, fields []StoryField) {
	type fieldKey struct {
		para  *Paragraph
		table *Table
	}
	fieldMap := make(map[fieldKey]struct {
		kind  string
		index int
	})
	for fi, f := range fields {
		for _, b := range f.Blocks {
			key := fieldKey{para: b.Paragraph, table: b.Table}
			fieldMap[key] = struct {
				kind  string
				index int
			}{kind: f.Kind, index: fi}
		}
	}
	for i := range blocks {
		b := &blocks[i]
		key := fieldKey{para: b.Paragraph, table: b.Table}
		if info, ok := fieldMap[key]; ok {
			b.FieldKind = info.kind
			b.FieldIndex = info.index
		} else {
			b.FieldIndex = -1
		}
	}
}

// ResolvePackage converts parsed Word parts into a semantic document model.
func ResolvePackage(pkg *parts.Package) (*Document, error) {
	if pkg == nil {
		return nil, fmt.Errorf("word resolve: package cannot be nil")
	}
	if pkg.Document == nil || pkg.Document.Body == nil {
		return nil, fmt.Errorf("word resolve: package document is missing")
	}

	document := &Document{
		Headers:         make(map[string]Story, len(pkg.Headers)),
		Footers:         make(map[string]Story, len(pkg.Footers)),
		Styles:          make(map[string]Style),
		CharacterStyles: make(map[string]CharacterStyle),
		FontFamilies:    resolveFontFamilies(pkg.FontTable),
		EmbeddedFonts:   resolveEmbeddedFonts(pkg.EmbeddedFonts),
		LatentStyles:    resolveLatentStyles(pkg.Styles),
		Footnotes:       make(map[int]Story),
		Endnotes:        make(map[int]Story),
		Comments:        make(map[int]Story),
		Diagnostics:     append(diag.List(nil), pkg.Diagnostics...),
		HasSignatures:   pkg.HasSignatures,
	}
	if pkg.CoreProperties != nil {
		document.Title = strings.TrimSpace(pkg.CoreProperties.Title)
		document.Subject = strings.TrimSpace(pkg.CoreProperties.Subject)
		document.Author = strings.TrimSpace(pkg.CoreProperties.Creator)
		document.UserName = strings.TrimSpace(pkg.CoreProperties.Creator)
		document.Description = strings.TrimSpace(pkg.CoreProperties.Description)
		document.Keywords = coreKeywordsToString(pkg.CoreProperties.Keywords)
		document.LastModifiedBy = strings.TrimSpace(pkg.CoreProperties.LastModifiedBy)
		document.CreatedAt = normalizeISODate(pkg.CoreProperties.Created)
		document.ModifiedAt = normalizeISODate(pkg.CoreProperties.Modified)
		document.LastPrintedAt = normalizeISODate(pkg.CoreProperties.LastPrinted)
	}
	if pkg.ExtendedProperties != nil {
		document.Company = strings.TrimSpace(pkg.ExtendedProperties.Company)
		document.Manager = strings.TrimSpace(pkg.ExtendedProperties.Manager)
		document.Application = strings.TrimSpace(pkg.ExtendedProperties.Application)
		document.AppVersion = strings.TrimSpace(pkg.ExtendedProperties.AppVersion)
		document.Template = strings.TrimSpace(pkg.ExtendedProperties.Template)
		document.TotalEditMinutes = pkg.ExtendedProperties.TotalTime
		document.WordCount = pkg.ExtendedProperties.Words
		document.PageCount = pkg.ExtendedProperties.Pages
		document.ParagraphCount = pkg.ExtendedProperties.Paragraphs
		document.CharCount = pkg.ExtendedProperties.Characters
	}
	if pkg.CustomProperties != nil {
		document.CustomProperties = make(map[string]string, len(pkg.CustomProperties.Properties))
		for _, p := range pkg.CustomProperties.Properties {
			document.CustomProperties[p.Name] = p.Value
		}
	}
	if len(pkg.CustomXMLParts) > 0 {
		document.CustomXMLParts = make(map[string][]byte, len(pkg.CustomXMLParts))
		for key, data := range pkg.CustomXMLParts {
			document.CustomXMLParts[key] = append([]byte(nil), data...)
		}
	}
	document.DigitalSignatures = resolveDigitalSignatures(pkg.Signatures, pkg.SignatureVerifications)

	var themeFontLang *parts.ThemeFontLang
	if pkg.Settings != nil {
		themeFontLang = pkg.Settings.ThemeFontLang
	}
	themeFonts := buildThemeFontIndex(pkg.Theme, themeFontLang)
	themeColors := buildThemeColorIndex(pkg.Theme)
	if pkg.Settings != nil && pkg.Settings.ColorSchemeMapping != nil {
		applyClrSchemeMapping(themeColors, pkg.Settings.ColorSchemeMapping)
	}
	if pkg.Document != nil && pkg.Document.Background != nil {
		bg := pkg.Document.Background
		document.BackgroundColor = bg.Color
		if bg.ThemeColor != "" {
			if resolved, ok := themeColors[bg.ThemeColor]; ok {
				document.BackgroundColor = resolved
			}
		}
		if bg.ImageRelID != "" {
			if media, ok := pkg.MediaPartFor(pkg.MainPartName, bg.ImageRelID); ok {
				document.BackgroundImage = &Image{
					MIMEType: media.ContentType,
					Data:     media.Data,
					Alt:      "Document background",
				}
			}
		}
	}
	styleIndex, defaultParagraphStyleID := buildStyleIndex(pkg.Styles, themeFonts, themeColors)
	resolveStyleInheritance(styleIndex)
	defaultParagraphFormat := buildDefaultParagraphFormat(pkg.Styles)
	defaultRunFormat := buildDefaultRunFormat(pkg.Styles, themeFonts, themeColors)
	charStyleIndex, defaultCharStyleID := buildCharacterStyleIndex(pkg.Styles, themeFonts, themeColors)
	document.DefaultCharacterStyleID = defaultCharStyleID
	resolveCharStyleInheritance(charStyleIndex)
	resolveCharStyleFormats(charStyleIndex, defaultRunFormat)
	resolveStyleFormats(styleIndex, defaultParagraphFormat, defaultRunFormat)

	document.Styles = styleIndex
	document.CharacterStyles = charStyleIndex
	document.DefaultParagraphStyleID = defaultParagraphStyleID
	document.DefaultParagraphFormat = defaultParagraphFormat
	document.DefaultRunFormat = defaultRunFormat

	themeFmtScheme := buildThemeFormatIndex(pkg.Theme)
	document.Theme = buildIRTheme(pkg.Theme, themeColors, themeFonts, themeFmtScheme)

	ctx := &resolveCtx{
		pkg:            pkg,
		currentPart:    pkg.MainPartName,
		numbering:      buildNumberingIndex(pkg.Numbering, themeFonts, themeColors),
		styles:         styleIndex,
		charStyles:     charStyleIndex,
		defaultStyle:   defaultParagraphStyleID,
		defaultRun:     defaultRunFormat,
		themeColors:    themeColors,
		themeFonts:     themeFonts,
		themeFmtScheme: themeFmtScheme,
		footnotes:      buildNoteIndex(noteSlice(pkg.Footnotes)),
		endnotes:       buildNoteIndex(noteSlice(pkg.Endnotes)),
		comments:       buildCommentIndex(pkg.Comments),
	}
	if pkg.Settings != nil && pkg.Settings.DefaultTableStyle != nil {
		ctx.defaultTableStyle = pkg.Settings.DefaultTableStyle.Val
	}
	ctx.autoTextEntries = buildAutoTextEntryIndex(ctx)
	if ctx.autoTextEntries == nil {
		ctx.autoTextEntries = make(map[string]autoTextEntry)
	}
	mergeAttachedTemplateAutoText(ctx, ctx.autoTextEntries)
	if len(ctx.autoTextEntries) == 0 {
		ctx.autoTextEntries = nil
	}

	for _, block := range pkg.Document.Body.Blocks {
		ctx.currentPart = pkg.MainPartName
		blockIndex := len(document.Blocks)
		switch block.Kind {
		case parts.BodyBlockParagraph:
			if ctx.inField() {
				top := ctx.topField()
				top.paraCount++
				top.paraBoundaries = append(top.paraBoundaries, len(top.result))
			}
			paragraph := normalizeParagraph(ctx, block.Paragraph)
			if autoTextName := standaloneAutoTextName(block.Paragraph, paragraph); autoTextName != "" && ctx.autoTextEntries != nil {
				if entry, ok := ctx.autoTextEntries[autoTextName]; ok && len(entry.Story.Blocks) > 0 {
					fieldName := entry.Name
					if strings.TrimSpace(fieldName) == "" {
						fieldName = autoTextName
					}
					expanded := cloneAutoTextBlocks(ctx, entry.Story, fieldName)
					fieldIndex := len(document.Fields)
					for i := range expanded {
						expanded[i].Index = len(document.Blocks)
						expanded[i].FieldKind = "AUTOTEXT"
						expanded[i].FieldIndex = fieldIndex
						switch expanded[i].Kind {
						case "paragraph":
							if expanded[i].Paragraph != nil {
								document.Paragraphs = append(document.Paragraphs, *expanded[i].Paragraph)
							}
						case "table":
							if expanded[i].Table != nil {
								expanded[i].Table.Index = len(document.Tables)
								document.Tables = append(document.Tables, *expanded[i].Table)
							}
						}
						document.Blocks = append(document.Blocks, expanded[i])
					}
					document.Fields = append(document.Fields, StoryField{
						Kind:   "AUTOTEXT",
						Instr:  `AUTOTEXT "` + fieldName + `"`,
						Arg:    fieldName,
						Blocks: append([]Block(nil), expanded...),
					})
					continue
				}
			}
			paragraph.Index = ctx.nextParaIndex
			ctx.nextParaIndex++
			document.Paragraphs = append(document.Paragraphs, paragraph)
			if !isSectionMarkerOnlyParagraph(block.Paragraph, &paragraph) {
				b := Block{
					Index:     blockIndex,
					Kind:      "paragraph",
					Paragraph: &paragraph,
				}
				document.Blocks = append(document.Blocks, b)
				attachParagraphToField(ctx, len(document.Paragraphs)-1, b)
			}
			if block.Paragraph != nil && block.Paragraph.Properties != nil && block.Paragraph.Properties.Section != nil {
				section := normalizeSection(pkg, block.Paragraph.Properties.Section)
				document.Sections = append(document.Sections, section)
				sectionCopy := section
				document.Blocks = append(document.Blocks, Block{
					Index:   len(document.Blocks),
					Kind:    "section",
					Section: &sectionCopy,
				})
			}
		case parts.BodyBlockTable:
			table := normalizeTable(ctx, block.Table)
			table.Index = blockIndex
			document.Tables = append(document.Tables, table)
			b := Block{
				Index: blockIndex,
				Kind:  "table",
				Table: &table,
			}
			document.Blocks = append(document.Blocks, b)
			attachTableToField(ctx, b)
		case parts.BodyBlockSection:
			section := normalizeSection(pkg, block.Section)
			document.Sections = append(document.Sections, section)
			sectionCopy := section
			document.Blocks = append(document.Blocks, Block{
				Index:   blockIndex,
				Kind:    "section",
				Section: &sectionCopy,
			})
		case parts.BodyBlockSDT:
			contentControl := normalizeContentControlWithContext(ctx, block.SdtProps)
			for _, inner := range normalizeSdtBodyBlocks(ctx, pkg, contentControl, block.SdtBlocks) {
				inner.ContentControl = contentControl
				switch inner.Kind {
				case "paragraph":
					if inner.Paragraph != nil {
						document.Paragraphs = append(document.Paragraphs, *inner.Paragraph)
					}
				case "table":
					if inner.Table != nil {
						document.Tables = append(document.Tables, *inner.Table)
					}
				case "section":
					if inner.Section != nil {
						document.Sections = append(document.Sections, *inner.Section)
					}
				}
				document.Blocks = append(document.Blocks, inner)
			}
		case parts.BodyBlockCustomXml:
			cx := normalizeCustomXml(block.CustomXml)
			for _, inner := range normalizeCustomXmlBodyBlocks(ctx, pkg, block.CustomXmlBlocks) {
				inner.CustomXml = cx
				switch inner.Kind {
				case "paragraph":
					if inner.Paragraph != nil {
						document.Paragraphs = append(document.Paragraphs, *inner.Paragraph)
					}
				case "table":
					if inner.Table != nil {
						document.Tables = append(document.Tables, *inner.Table)
					}
				case "section":
					if inner.Section != nil {
						document.Sections = append(document.Sections, *inner.Section)
					}
				}
				document.Blocks = append(document.Blocks, inner)
			}
		}
		document.Fields = append(document.Fields, extractStoryFields(ctx, document.Paragraphs)...)
		document.Fields = append(document.Fields, ctx.storyFields...)
		ctx.storyFields = ctx.storyFields[:0]
	}

	if len(pkg.Document.Body.Blocks) == 0 {
		ctx.currentPart = pkg.MainPartName
		for _, paragraph := range pkg.Document.Body.Paragraphs {
			if ctx.inField() {
				top := ctx.topField()
				top.paraCount++
				top.paraBoundaries = append(top.paraBoundaries, len(top.result))
			}
			resolved := normalizeParagraph(ctx, paragraph)
			resolved.Index = ctx.nextParaIndex
			ctx.nextParaIndex++
			document.Paragraphs = append(document.Paragraphs, resolved)
			attachParagraphToField(ctx, len(document.Paragraphs)-1, Block{Kind: "paragraph", Paragraph: &document.Paragraphs[len(document.Paragraphs)-1]})
		}
		for _, table := range pkg.Document.Body.Tables {
			resolved := normalizeTable(ctx, table)
			document.Tables = append(document.Tables, resolved)
			attachTableToField(ctx, Block{Kind: "table", Table: &resolved})
		}
	}

	// Flush any remaining story-spanning fields at body boundary.
	for len(ctx.fieldStack) > 0 {
		top := ctx.fieldStack[len(ctx.fieldStack)-1]
		ctx.fieldStack = ctx.fieldStack[:len(ctx.fieldStack)-1]
		if top.paraCount > 1 {
			ctx.flushedFields = append(ctx.flushedFields, top)
		}
	}
	document.Fields = append(document.Fields, extractStoryFields(ctx, document.Paragraphs)...)
	document.Fields = append(document.Fields, ctx.storyFields...)
	ctx.storyFields = ctx.storyFields[:0]

	if len(document.Sections) == 0 {
		sectionProperties := lastSectionProperties(pkg.Document)
		if sectionProperties != nil {
			document.Sections = append(document.Sections, normalizeSection(pkg, sectionProperties))
		}
	}
	inheritSectionProperties(document.Sections)

	for name, header := range pkg.Headers {
		ctx.currentPart = name
		document.Headers[name] = normalizeStory(ctx, header.Blocks, header.Paragraphs, header.Tables)
	}
	for name, footer := range pkg.Footers {
		ctx.currentPart = name
		document.Footers[name] = normalizeStory(ctx, footer.Blocks, footer.Paragraphs, footer.Tables)
	}

	for id, note := range ctx.footnotes {
		ctx.currentPart = pkg.FootnotesPartName
		switch note.Type {
		case "", "normal":
			document.Footnotes[id] = normalizeStory(ctx, note.Blocks, note.Paragraphs, note.Tables)
		case "separator":
			document.FootnoteSeparator = normalizeStory(ctx, note.Blocks, note.Paragraphs, note.Tables)
		case "continuationSeparator":
			document.FootnoteContinuationSep = normalizeStory(ctx, note.Blocks, note.Paragraphs, note.Tables)
		case "continuationNotice":
			document.FootnoteContinuationNotice = normalizeStory(ctx, note.Blocks, note.Paragraphs, note.Tables)
		}
	}
	for id, note := range ctx.endnotes {
		ctx.currentPart = pkg.EndnotesPartName
		switch note.Type {
		case "", "normal":
			document.Endnotes[id] = normalizeStory(ctx, note.Blocks, note.Paragraphs, note.Tables)
		case "separator":
			document.EndnoteSeparator = normalizeStory(ctx, note.Blocks, note.Paragraphs, note.Tables)
		case "continuationSeparator":
			document.EndnoteContinuationSep = normalizeStory(ctx, note.Blocks, note.Paragraphs, note.Tables)
		case "continuationNotice":
			document.EndnoteContinuationNotice = normalizeStory(ctx, note.Blocks, note.Paragraphs, note.Tables)
		}
	}
	for id, comment := range ctx.comments {
		ctx.currentPart = pkg.CommentsPartName
		document.Comments[id] = normalizeStory(ctx, comment.Blocks, comment.Paragraphs, comment.Tables)
	}
	// annotationRef runs inside comment stories reference the comment itself.
	for id, story := range document.Comments {
		for i := range story.Blocks {
			if story.Blocks[i].Paragraph == nil {
				continue
			}
			for j := range story.Blocks[i].Paragraph.Runs {
				if story.Blocks[i].Paragraph.Runs[j].Kind == "annotationRef" {
					story.Blocks[i].Paragraph.Runs[j].CommentID = id
				}
			}
		}
		for i := range story.Paragraphs {
			for j := range story.Paragraphs[i].Runs {
				if story.Paragraphs[i].Runs[j].Kind == "annotationRef" {
					story.Paragraphs[i].Runs[j].CommentID = id
				}
			}
		}
	}
	if pkg.Settings != nil && pkg.Settings.FootnoteProps != nil {
		if pkg.Settings.FootnoteProps.Format != nil {
			document.FootnoteNumberFormat = pkg.Settings.FootnoteProps.Format.Val
		}
		if pkg.Settings.FootnoteProps.Start != nil {
			document.FootnoteNumberStart = pkg.Settings.FootnoteProps.Start.Val
		}
		if pkg.Settings.FootnoteProps.Restart != nil {
			document.FootnoteRestart = pkg.Settings.FootnoteProps.Restart.Val
		}
	}
	if pkg.Settings != nil && pkg.Settings.EndnoteProps != nil && pkg.Settings.EndnoteProps.Position != nil {
		document.EndnotePosition = pkg.Settings.EndnoteProps.Position.Val
	}
	if pkg.Settings != nil && pkg.Settings.EndnoteProps != nil {
		if pkg.Settings.EndnoteProps.Format != nil {
			document.EndnoteNumberFormat = pkg.Settings.EndnoteProps.Format.Val
		}
		if pkg.Settings.EndnoteProps.Start != nil {
			document.EndnoteNumberStart = pkg.Settings.EndnoteProps.Start.Val
		}
		if pkg.Settings.EndnoteProps.Restart != nil {
			document.EndnoteRestart = pkg.Settings.EndnoteProps.Restart.Val
		}
	}
	if pkg.Settings != nil && pkg.Settings.DefaultTabStop != nil && pkg.Settings.DefaultTabStop.Val > 0 {
		document.DefaultTabStop = pkg.Settings.DefaultTabStop.Val
	}
	if pkg.Settings != nil && pkg.Settings.TrackRevisions != nil && onOffBool(pkg.Settings.TrackRevisions) {
		document.TrackChangesEnabled = true
	}
	if pkg.Settings != nil && pkg.Settings.Zoom != nil && pkg.Settings.Zoom.Percent > 0 {
		document.ZoomPercent = pkg.Settings.Zoom.Percent
	}
	if pkg.Settings != nil && pkg.Settings.Compat != nil {
		document.Compatibility = resolveCompatSettings(pkg.Settings.Compat)
	}
	if pkg.Settings != nil {
		if pkg.Settings.DocumentProtection != nil {
			document.DocumentProtection = &DocumentProtection{
				Edit:           pkg.Settings.DocumentProtection.Edit,
				Enforcement:    pkg.Settings.DocumentProtection.Enforcement,
				FormatSHA1:     pkg.Settings.DocumentProtection.FormatSHA1,
				HashValue:      pkg.Settings.DocumentProtection.HashValue,
				SaltValue:      pkg.Settings.DocumentProtection.SaltValue,
				SpinCount:      pkg.Settings.DocumentProtection.SpinCount,
				AlgorithmSID:   pkg.Settings.DocumentProtection.AlgorithmSID,
				CryptProvType:  pkg.Settings.DocumentProtection.CryptProvType,
				CryptAlgorithm: pkg.Settings.DocumentProtection.CryptAlgorithm,
			}
			if pkg.Settings.DocumentProtection.Enforcement {
				document.ProtectionEnabled = true
			}
		}
		if pkg.Settings.CharacterSpacingControl != nil {
			document.CharacterSpacingControl = pkg.Settings.CharacterSpacingControl.Val
		}
		if pkg.Settings.AttachedTemplate != nil && pkg.Settings.AttachedTemplate.RelID != "" {
			if rels := pkg.PartRels[pkg.SettingsPartName]; rels != nil {
				for _, rel := range rels.Relationships {
					if rel != nil && rel.ID == pkg.Settings.AttachedTemplate.RelID {
						document.TemplatePath = rel.Target
						break
					}
				}
			}
		}
		if pkg.Settings.View != nil {
			document.ViewMode = pkg.Settings.View.Val
		}
		if pkg.Settings.AutoHyphenation != nil && onOffBool(pkg.Settings.AutoHyphenation) {
			document.AutoHyphenation = true
		}
		if pkg.Settings.ConsecutiveHyphenLimit != nil {
			document.ConsecutiveHyphenLimit = pkg.Settings.ConsecutiveHyphenLimit.Val
		}
		if pkg.Settings.HyphenationZone != nil {
			document.HyphenationZone = pkg.Settings.HyphenationZone.Val
		}
		if pkg.Settings.DoNotHyphenateCaps != nil && onOffBool(pkg.Settings.DoNotHyphenateCaps) {
			document.DoNotHyphenateCaps = true
		}
		if pkg.Settings.EvenAndOddHeaders != nil && onOffBool(pkg.Settings.EvenAndOddHeaders) {
			document.EvenAndOddHeaders = true
		}
		if pkg.Settings.UpdateFields != nil && onOffBool(pkg.Settings.UpdateFields) {
			document.UpdateFields = true
		}
		if pkg.Settings.ThemeFontLang != nil {
			document.ThemeFontLang = &ThemeFontLang{
				Val:      pkg.Settings.ThemeFontLang.Val,
				EastAsia: pkg.Settings.ThemeFontLang.EastAsia,
				Bidi:     pkg.Settings.ThemeFontLang.Bidi,
			}
		}
		if pkg.Settings.ProofState != nil {
			document.ProofState = &ProofState{
				Spelling: pkg.Settings.ProofState.Spelling,
				Grammar:  pkg.Settings.ProofState.Grammar,
			}
		}
		if len(pkg.Settings.DocVars) > 0 {
			document.DocumentVariables = make(map[string]string, len(pkg.Settings.DocVars))
			for _, v := range pkg.Settings.DocVars {
				document.DocumentVariables[v.Name] = v.Value
			}
		}
		if pkg.Settings.MailMerge != nil {
			mm := pkg.Settings.MailMerge
			document.MailMerge = &MailMergeSettings{
				MainDocumentType:        firstString(mm.MainDocumentTypeVal, mm.MainDocumentType),
				LinkToQuery:             firstBool(mm.LinkToQueryVal, mm.LinkToQuery),
				DataType:                firstString(mm.DataTypeVal, mm.DataType),
				ConnectString:           firstString(mm.ConnectStringVal, mm.ConnectString),
				Query:                   firstString(mm.QueryVal, mm.Query),
				DataSource:              firstString(mm.DataSourceVal, mm.DataSource),
				HeaderSource:            firstString(mm.HeaderSourceVal, mm.HeaderSource),
				Destination:             firstString(mm.DestinationVal, mm.Destination),
				ActiveRecord:            firstString(mm.ActiveRecord, ""),
				AddressFieldName:        firstString(mm.AddressFieldName, ""),
				CheckErrors:             firstString(mm.CheckErrors, ""),
				DoNotSuppressBlankLines: firstBool(mm.DoNotSuppressBlankLines, false),
				DynamicAddress:          firstBool(mm.DynamicAddress, false),
				EntryMacro:              firstString(mm.EntryMacro, ""),
				ExitMacro:               firstString(mm.ExitMacro, ""),
				DataSourceHasHeaders:    firstBool(mm.FHdr, false),
				LinkedToFile:            firstBool(mm.LinkedToFile, false),
				MailAsAttachment:        firstBool(mm.MailAsAttachment, false),
				MailSubject:             firstString(mm.MailSubject, ""),
				MappedName:              firstString(mm.MappedName, ""),
				SourceFileName:          firstString(mm.SourceFileName, ""),
				StoreMappedDataAs:       firstString(mm.StoreMappedDataAs, ""),
				SubDoc:                  firstString(mm.SubDoc, ""),
				UDL:                     firstString(mm.UDL, ""),
				ViewMergedData:          firstBool(mm.ViewMergedData, false),
				Personal:                firstBool(mm.Personal, false),
				PersonalCompose:         firstBool(mm.PersonalCompose, false),
				PersonalReply:           firstBool(mm.PersonalReply, false),
			}
			if mm.Types != nil {
				for _, mmType := range mm.Types.Types {
					if val := strings.TrimSpace(mmType.Val); val != "" {
						document.MailMerge.Types = append(document.MailMerge.Types, val)
					}
				}
			}
			if mm.Recipients != nil {
				for _, rd := range mm.Recipients.RecipientData {
					document.MailMerge.Recipients = append(document.MailMerge.Recipients, RecipientData{
						Active: onOffBool(rd.Active),
					})
				}
			}
			if mm.ODSO != nil {
				odso := &ODSOSettings{
					ColDelim:       firstInt(mm.ODSO.ColDelim, 0),
					DynamicAddress: onOffBool(mm.ODSO.DynamicAddress),
					FHdr:           onOffBool(mm.ODSO.FHdr),
					SourceFileName: firstString(mm.ODSO.SourceFileName, ""),
					UDL:            firstString(mm.ODSO.UDL, ""),
				}
				for _, fm := range mm.ODSO.FieldMapData {
					odso.FieldMapData = append(odso.FieldMapData, FieldMapData{
						MappedName: fm.MappedName,
						Name:       fm.Name,
						Lang:       fm.Lang,
						Val:        fm.Val,
						Column:     fm.Column,
					})
				}
				for _, rd := range mm.ODSO.RecipientData {
					odso.RecipientData = append(odso.RecipientData, RecipientData{
						Active: onOffBool(rd.Active),
					})
				}
				document.MailMerge.ODSO = odso
			}
		}
		if pkg.Settings.DocumentType != nil {
			document.DocumentType = pkg.Settings.DocumentType.Val
		}
		if pkg.Settings.MirrorMargins != nil && onOffBool(pkg.Settings.MirrorMargins) {
			document.MirrorMargins = true
		}
		if pkg.Settings.DisplayBackgroundShape != nil {
			v := onOffBool(pkg.Settings.DisplayBackgroundShape)
			document.DisplayBackgroundShape = &v
		}
		if pkg.Settings.DefaultTableStyle != nil {
			document.DefaultTableStyle = pkg.Settings.DefaultTableStyle.Val
		}
		if pkg.Settings.DoNotDisplayPageBoundaries != nil && onOffBool(pkg.Settings.DoNotDisplayPageBoundaries) {
			document.DoNotDisplayPageBoundaries = true
		}
		if pkg.Settings.BordersDoNotSurroundFooter != nil && onOffBool(pkg.Settings.BordersDoNotSurroundFooter) {
			document.BordersDoNotSurroundFooter = true
		}
		if pkg.Settings.BordersDoNotSurroundHeader != nil && onOffBool(pkg.Settings.BordersDoNotSurroundHeader) {
			document.BordersDoNotSurroundHeader = true
		}
		if pkg.Settings.DoNotTrackFormatting != nil && onOffBool(pkg.Settings.DoNotTrackFormatting) {
			document.DoNotTrackFormatting = true
		}
		if pkg.Settings.DoNotTrackMoves != nil && onOffBool(pkg.Settings.DoNotTrackMoves) {
			document.DoNotTrackMoves = true
		}
		if len(pkg.Settings.ActiveWritingStyles) > 0 {
			document.ActiveWritingStyles = make([]ActiveWritingStyle, len(pkg.Settings.ActiveWritingStyles))
			for i, aws := range pkg.Settings.ActiveWritingStyles {
				document.ActiveWritingStyles[i] = ActiveWritingStyle{
					Lang:       aws.Lang,
					VendorID:   aws.VendorID,
					DLLVersion: aws.DLLVersion,
					NLCheck:    aws.NLCheck,
					CheckStyle: aws.CheckStyle,
					AppName:    aws.AppName,
				}
			}
		}
		if pkg.Settings.WriteProtection != nil {
			document.WriteProtection = &WriteProtection{
				Recommended:   pkg.Settings.WriteProtection.Recommended,
				HashValue:     pkg.Settings.WriteProtection.HashValue,
				SaltValue:     pkg.Settings.WriteProtection.SaltValue,
				SpinCount:     pkg.Settings.WriteProtection.SpinCount,
				AlgorithmName: "",
			}
		}
		if pkg.Settings.RSIDs != nil {
			document.RevisionSaveIDs = make([]string, 0, len(pkg.Settings.RSIDs.RSIDs))
			for _, rsid := range pkg.Settings.RSIDs.RSIDs {
				if rsid.Val != "" {
					document.RevisionSaveIDs = append(document.RevisionSaveIDs, rsid.Val)
				}
			}
		}
		document.MiscSettings = resolveMiscSettings(pkg.Settings)
	}

	document.Diagnostics = append(document.Diagnostics, ctx.Diagnostics...)

	annotateFieldBlocks(document.Blocks, document.Fields)
	for _, story := range document.Headers {
		annotateFieldBlocks(story.Blocks, story.Fields)
	}
	for _, story := range document.Footers {
		annotateFieldBlocks(story.Blocks, story.Fields)
	}
	for _, story := range document.Footnotes {
		annotateFieldBlocks(story.Blocks, story.Fields)
	}
	for _, story := range document.Endnotes {
		annotateFieldBlocks(story.Blocks, story.Fields)
	}
	for _, story := range document.Comments {
		annotateFieldBlocks(story.Blocks, story.Fields)
	}

	return document, nil
}

func resolveDigitalSignatures(signatures []opc.SignaturePart, verifications []opc.SignatureVerification) []DigitalSignature {
	if len(signatures) == 0 {
		return nil
	}

	out := make([]DigitalSignature, 0, len(signatures))
	validations := make(map[string]opc.SignatureVerification, len(verifications))
	for _, validation := range verifications {
		validations[validation.PartName] = validation
	}
	for _, signature := range signatures {
		if signature.Signature == nil {
			continue
		}
		entry := DigitalSignature{
			PartName:               signature.PartName,
			ContentType:            signature.ContentType,
			ID:                     signature.Signature.ID,
			SignatureMethod:        signature.Signature.SignatureMethodURI(),
			CanonicalizationMethod: signature.Signature.CanonicalizationMethodURI(),
			Signer:                 resolveSignatureCertificate(signature.Signer()),
		}
		for _, ref := range signature.Signature.SignedInfo.References {
			entry.References = append(entry.References, DigitalSignatureReference{
				URI:          ref.URI,
				Type:         ref.Type,
				DigestMethod: ref.DigestMethod.Algorithm,
			})
		}
		for _, cert := range signature.Certificates() {
			if resolved := resolveSignatureCertificate(cert); resolved != nil {
				entry.Certificates = append(entry.Certificates, *resolved)
			}
		}
		if validation, ok := validations[signature.PartName]; ok {
			entry.Valid = validation.Valid
			entry.SignatureValid = validation.SignatureValid
			entry.ReferencesValid = validation.SignedInfoValid && validation.PackageReferencesValid
			entry.TrustValid = validation.TrustValid
			entry.RevocationChecked = validation.RevocationChecked
			entry.Revoked = validation.Revoked
			entry.SigningTime = validation.SigningTime
			entry.SigningTimeFormat = validation.SigningTimeFormat
			entry.SigningTimeValid = validation.SigningTimeValid
			entry.TrustedTimestamp = validation.TrustedTimestamp
			for _, cert := range validation.TrustChain {
				if resolved := resolveSignatureCertificate(cert); resolved != nil {
					entry.TrustChain = append(entry.TrustChain, *resolved)
				}
			}
			entry.VerificationErrors = append([]string(nil), validation.Errors...)
		}
		out = append(out, entry)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func resolveSignatureCertificate(cert *x509.Certificate) *DigitalSignatureCertificate {
	if cert == nil {
		return nil
	}
	return &DigitalSignatureCertificate{
		Subject:      cert.Subject.String(),
		Issuer:       cert.Issuer.String(),
		SerialNumber: cert.SerialNumber.String(),
		NotBefore:    cert.NotBefore.UTC().Format(time.RFC3339),
		NotAfter:     cert.NotAfter.UTC().Format(time.RFC3339),
	}
}

func resolveCompatSettings(cs *parts.CompatSettings) *DocCompatibility {
	if cs == nil || len(cs.Settings) == 0 {
		return nil
	}
	c := &DocCompatibility{}
	for _, s := range cs.Settings {
		// Only process well-known Microsoft Office compatibility settings.
		if s.URI != "" && s.URI != "http://schemas.microsoft.com/office/word" {
			continue
		}
		v := s.Value
		switch s.Name {
		case "compatibilityMode":
			// compatibilityMode is a version number (e.g. "15" for Word 2013);
			// not stored as a boolean flag.
		case "doNotUseHTMLParagraphAutoSpacing":
			c.DoNotUseHTMLParagraphAutoSpacing = v == "1" || v == "true"
		case "alignBordersAndEdges":
			c.AlignBordersAndEdges = v == "1" || v == "true"
		case "lineWrapLikeWord6":
			c.LineWrapLikeWord6 = v == "1" || v == "true"
		case "shapeLayoutLikeWW8":
			c.ShapeLayoutLikeWW8 = v == "1" || v == "true"
		case "footnoteLayoutLikeWW8":
			c.FootnoteLayoutLikeWW8 = v == "1" || v == "true"
		case "tableWrapLikeWW8":
			c.TableWrapLikeWW8 = v == "1" || v == "true"
		case "noTabHangInd":
			c.NoTabHangInd = v == "1" || v == "true"
		case "noLeading":
			c.NoLeading = v == "1" || v == "true"
		case "spaceLikeWW8":
			c.SpaceLikeWW8 = v == "1" || v == "true"
		case "noColumnBalance":
			c.NoColumnBalance = v == "1" || v == "true"
		case "doNotBreakWrappedTables":
			c.DoNotBreakWrappedTables = v == "1" || v == "true"
		case "doNotSnapToGridInCell":
			c.DoNotSnapToGridInCell = v == "1" || v == "true"
		case "doNotAllowFieldEndSelect":
			c.DoNotAllowFieldEndSelect = v == "1" || v == "true"
		case "applyBreakingRules":
			c.ApplyBreakingRules = v == "1" || v == "true"
		case "doNotBreakConstrainedForcedTables":
			c.DoNotBreakConstrainedForcedTables = v == "1" || v == "true"
		case "doNotVertAlignCellWithSp":
			c.DoNotVertAlignCellWithSp = v == "1" || v == "true"
		case "useWord2002TableRules":
			c.UseWord2002TableRules = v == "1" || v == "true"
		case "growAutofit":
			c.GrowAutofit = v == "1" || v == "true"
		case "useNormalStyleForListNumber":
			c.UseNormalStyleForListNumber = v == "1" || v == "true"
		case "doNotUseIndentAsNumberingTabStop":
			c.DoNotUseIndentAsNumberingTabStop = v == "1" || v == "true"
		case "useAltKinsokuLineBreakRules":
			c.UseAltKinsokuLineBreakRules = v == "1" || v == "true"
		case "allowSpaceOfSameStyleInTable":
			c.AllowSpaceOfSameStyleInTable = v == "1" || v == "true"
		case "doNotSuppressIndentation":
			c.DoNotSuppressIndentation = v == "1" || v == "true"
		case "doNotAutofitConstrainedTables":
			c.DoNotAutofitConstrainedTables = v == "1" || v == "true"
		case "autofitToFirstFixedWidthCell":
			c.AutofitToFirstFixedWidthCell = v == "1" || v == "true"
		case "underlineTabInNumberingList":
			c.UnderlineTabInNumberingList = v == "1" || v == "true"
		case "displayHangulFixedWidth":
			c.DisplayHangulFixedWidth = v == "1" || v == "true"
		case "splitPgBreakAndParaMark":
			c.SplitPgBreakAndParaMark = v == "1" || v == "true"
		case "doNotVertAlignInTxbx":
			c.DoNotVertAlignInTxbx = v == "1" || v == "true"
		case "useAnsiKerningPairs":
			c.UseAnsiKerningPairs = v == "1" || v == "true"
		case "cachedFieldBalance":
			c.CachedFieldBalance = v == "1" || v == "true"
		case "uiCompat97To2003":
			c.UICompat97To2003 = v == "1" || v == "true"
		case "adjustLineHeightInTable":
			c.AdjustLineHeightInTable = v == "1" || v == "true"
		case "alignTablesRowByRow":
			c.AlignTablesRowByRow = v == "1" || v == "true"
		case "allowPNG":
			c.AllowPNG = v == "1" || v == "true"
		// Name variants that appear as direct child elements or alternate compatSetting names.
		case "doNotBreakConstrainedForcedTable":
			c.DoNotBreakConstrainedForcedTables = v == "1" || v == "true"
		case "underlineTabInNumList":
			c.UnderlineTabInNumberingList = v == "1" || v == "true"
		case "useNormalStyleForList":
			c.UseNormalStyleForListNumber = v == "1" || v == "true"
		case "useWord2002TableStyleRules":
			c.UseWord2002TableRules = v == "1" || v == "true"
		case "cachedColBalance":
			c.CachedFieldBalance = v == "1" || v == "true"
		// Remaining w:compat child elements (ECMA-376 Part 4, §14.8.3).
		case "autoSpaceLikeWord95":
			c.AutoSpaceLikeWord95 = v == "1" || v == "true"
		case "convMailMergeEsc":
			c.ConvMailMergeEsc = v == "1" || v == "true"
		case "doNotSuppressParagraphBorders":
			c.DoNotSuppressParagraphBorders = v == "1" || v == "true"
		case "doNotUseEastAsianBreakRules":
			c.DoNotUseEastAsianBreakRules = v == "1" || v == "true"
		case "doNotWrapTextWithPunct":
			c.DoNotWrapTextWithPunct = v == "1" || v == "true"
		case "forgetLastTabAlignment":
			c.ForgetLastTabAlignment = v == "1" || v == "true"
		case "layoutRawTableWidth":
			c.LayoutRawTableWidth = v == "1" || v == "true"
		case "layoutTableRowsApart":
			c.LayoutTableRowsApart = v == "1" || v == "true"
		case "mwSmallCaps":
			c.MwSmallCaps = v == "1" || v == "true"
		case "noExtraLineSpacing":
			c.NoExtraLineSpacing = v == "1" || v == "true"
		case "noSpaceRaiseLower":
			c.NoSpaceRaiseLower = v == "1" || v == "true"
		case "printBodyTextBeforeHeader":
			c.PrintBodyTextBeforeHeader = v == "1" || v == "true"
		case "printColBlack":
			c.PrintColBlack = v == "1" || v == "true"
		case "selectFldWithFirstOrLastChar":
			c.SelectFldWithFirstOrLastChar = v == "1" || v == "true"
		case "showBreaksInFrames":
			c.ShowBreaksInFrames = v == "1" || v == "true"
		case "spacingInWholePoints":
			c.SpacingInWholePoints = v == "1" || v == "true"
		case "subFontBySize":
			c.SubFontBySize = v == "1" || v == "true"
		case "suppressBottomSpacing":
			c.SuppressBottomSpacing = v == "1" || v == "true"
		case "suppressSpBfAfterPgBrk":
			c.SuppressSpBfAfterPgBrk = v == "1" || v == "true"
		case "suppressSpacingAtTopOfPage":
			c.SuppressSpacingAtTopOfPage = v == "1" || v == "true"
		case "suppressTopSpacing":
			c.SuppressTopSpacing = v == "1" || v == "true"
		case "suppressTopSpacingWP":
			c.SuppressTopSpacingWP = v == "1" || v == "true"
		case "swapBordersFacingPages":
			c.SwapBordersFacingPages = v == "1" || v == "true"
		case "truncateFontHeightsLikeWP6":
			c.TruncateFontHeightsLikeWP6 = v == "1" || v == "true"
		case "useFELayout":
			c.UseFELayout = v == "1" || v == "true"
		case "usePrinterMetrics":
			c.UsePrinterMetrics = v == "1" || v == "true"
		case "useSingleBorderforContiguousCells":
			c.UseSingleBorderforContiguousCells = v == "1" || v == "true"
		case "useWord97LineBreakRules":
			c.UseWord97LineBreakRules = v == "1" || v == "true"
		case "wpJustification":
			c.WpJustification = v == "1" || v == "true"
		case "wpSpaceWidth":
			c.WpSpaceWidth = v == "1" || v == "true"
		case "wrapTrailSpaces":
			c.WrapTrailSpaces = v == "1" || v == "true"
		}
	}
	return c
}

func resolveMiscSettings(s *parts.Settings) *MiscSettings {
	if s == nil {
		return nil
	}
	m := &MiscSettings{}
	if s.DisplayHorizontalDrawingGrid != nil {
		m.DisplayHorizontalDrawingGrid = s.DisplayHorizontalDrawingGrid.Val
	}
	if s.DisplayVerticalDrawingGrid != nil {
		m.DisplayVerticalDrawingGrid = s.DisplayVerticalDrawingGrid.Val
	}
	if s.DoNotUseMarginsForDrawingGridOrigin != nil {
		m.DoNotUseMarginsForDrawingGridOrigin = onOffBool(s.DoNotUseMarginsForDrawingGridOrigin)
	}
	if s.DrawingGridHorizontalSpacing != nil {
		m.DrawingGridHorizontalSpacing = s.DrawingGridHorizontalSpacing.Val
	}
	if s.DrawingGridVerticalSpacing != nil {
		m.DrawingGridVerticalSpacing = s.DrawingGridVerticalSpacing.Val
	}
	if s.DrawingGridHorizontalOrigin != nil {
		m.DrawingGridHorizontalOrigin = s.DrawingGridHorizontalOrigin.Val
	}
	if s.DrawingGridVerticalOrigin != nil {
		m.DrawingGridVerticalOrigin = s.DrawingGridVerticalOrigin.Val
	}
	if s.BookFoldRevPrinting != nil {
		m.BookFoldRevPrinting = onOffBool(s.BookFoldRevPrinting)
	}
	if s.BookFoldPrintingSheets != nil {
		m.BookFoldPrintingSheets = s.BookFoldPrintingSheets.Val
	}
	if s.BookFoldPrinting != nil {
		m.BookFoldPrinting = onOffBool(s.BookFoldPrinting)
	}
	if s.PrintTwoOnOne != nil {
		m.PrintTwoOnOne = onOffBool(s.PrintTwoOnOne)
	}
	if s.PrintFormsData != nil {
		m.PrintFormsData = onOffBool(s.PrintFormsData)
	}
	if s.PrintPostScriptOverText != nil {
		m.PrintPostScriptOverText = onOffBool(s.PrintPostScriptOverText)
	}
	if s.PrintFractionalCharacterWidth != nil {
		m.PrintFractionalCharacterWidth = onOffBool(s.PrintFractionalCharacterWidth)
	}
	if s.DecimalSymbol != nil {
		m.DecimalSymbol = s.DecimalSymbol.Val
	}
	if s.ListSeparator != nil {
		m.ListSeparator = s.ListSeparator.Val
	}
	if s.EmbedTrueTypeFonts != nil {
		m.EmbedTrueTypeFonts = onOffBool(s.EmbedTrueTypeFonts)
	}
	if s.EmbedSystemFonts != nil {
		m.EmbedSystemFonts = onOffBool(s.EmbedSystemFonts)
	}
	if s.SaveSubsetFonts != nil {
		m.SaveSubsetFonts = onOffBool(s.SaveSubsetFonts)
	}
	if s.SaveFormsData != nil {
		m.SaveFormsData = onOffBool(s.SaveFormsData)
	}
	if s.ShowXMLTags != nil {
		m.ShowXMLTags = onOffBool(s.ShowXMLTags)
	}
	if s.HideSpellingErrors != nil {
		m.HideSpellingErrors = onOffBool(s.HideSpellingErrors)
	}
	if s.HideGrammaticalErrors != nil {
		m.HideGrammaticalErrors = onOffBool(s.HideGrammaticalErrors)
	}
	if s.FormsDesign != nil {
		m.FormsDesign = onOffBool(s.FormsDesign)
	}
	if s.LinkStyles != nil {
		m.LinkStyles = onOffBool(s.LinkStyles)
	}
	if s.RevisionView != nil {
		m.RevisionView = onOffBool(s.RevisionView)
	}
	if s.StyleLockTheme != nil {
		m.StyleLockTheme = onOffBool(s.StyleLockTheme)
	}
	if s.StyleLockQFSet != nil {
		m.StyleLockQFSet = onOffBool(s.StyleLockQFSet)
	}
	if s.ShowEnvelope != nil {
		m.ShowEnvelope = onOffBool(s.ShowEnvelope)
	}
	if s.StrictFirstAndLastChars != nil {
		m.StrictFirstAndLastChars = onOffBool(s.StrictFirstAndLastChars)
	}
	if s.DoNotShadeFormData != nil {
		m.DoNotShadeFormData = onOffBool(s.DoNotShadeFormData)
	}
	if s.NoPunctuationKerning != nil {
		m.NoPunctuationKerning = onOffBool(s.NoPunctuationKerning)
	}
	if s.AlignBordersAndEdges != nil {
		m.AlignBordersAndEdges = onOffBool(s.AlignBordersAndEdges)
	}
	if s.GutterAtTop != nil {
		m.GutterAtTop = onOffBool(s.GutterAtTop)
	}
	if s.DoNotValidateAgainstSchema != nil {
		m.DoNotValidateAgainstSchema = onOffBool(s.DoNotValidateAgainstSchema)
	}
	if s.SaveInvalidXml != nil {
		m.SaveInvalidXml = onOffBool(s.SaveInvalidXml)
	}
	if s.IgnoreMixedContent != nil {
		m.IgnoreMixedContent = onOffBool(s.IgnoreMixedContent)
	}
	if s.AlwaysShowPlaceholderText != nil {
		m.AlwaysShowPlaceholderText = onOffBool(s.AlwaysShowPlaceholderText)
	}
	if s.DoNotDemarcateInvalidXml != nil {
		m.DoNotDemarcateInvalidXml = onOffBool(s.DoNotDemarcateInvalidXml)
	}
	if s.SaveXmlDataOnly != nil {
		m.SaveXmlDataOnly = onOffBool(s.SaveXmlDataOnly)
	}
	if s.UseXSLTWhenSaving != nil {
		m.UseXSLTWhenSaving = onOffBool(s.UseXSLTWhenSaving)
	}
	if s.AlwaysMergeEmptyNamespace != nil {
		m.AlwaysMergeEmptyNamespace = onOffBool(s.AlwaysMergeEmptyNamespace)
	}
	if s.DoNotIncludeSubdocsInStats != nil {
		m.DoNotIncludeSubdocsInStats = onOffBool(s.DoNotIncludeSubdocsInStats)
	}
	if s.DoNotAutoCompressPictures != nil {
		m.DoNotAutoCompressPictures = onOffBool(s.DoNotAutoCompressPictures)
	}
	if s.DoNotEmbedSmartTags != nil {
		m.DoNotEmbedSmartTags = onOffBool(s.DoNotEmbedSmartTags)
	}
	if s.ForceUpgrade != nil {
		m.ForceUpgrade = true
	}
	if s.Active != nil {
		m.Active = onOffBool(s.Active)
	}
	if s.BodyDiv != nil {
		m.BodyDiv = onOffBool(s.BodyDiv)
	}
	if s.CachedColBalance != nil {
		m.CachedColBalance = onOffBool(s.CachedColBalance)
	}
	if s.DoNotSnapToGridInCell != nil {
		m.DoNotSnapToGridInCell = onOffBool(s.DoNotSnapToGridInCell)
	}
	if s.DoNotSuppressIndentation != nil {
		m.DoNotSuppressIndentation = onOffBool(s.DoNotSuppressIndentation)
	}
	if s.DoNotSuppressParagraphBorders != nil {
		m.DoNotSuppressParagraphBorders = onOffBool(s.DoNotSuppressParagraphBorders)
	}
	if s.DoNotUseEastAsianBreakRules != nil {
		m.DoNotUseEastAsianBreakRules = onOffBool(s.DoNotUseEastAsianBreakRules)
	}
	if s.DoNotUseHTMLParagraphAutoSpacing != nil {
		m.DoNotUseHTMLParagraphAutoSpacing = onOffBool(s.DoNotUseHTMLParagraphAutoSpacing)
	}
	if s.DoNotWrapTextWithPunct != nil {
		m.DoNotWrapTextWithPunct = onOffBool(s.DoNotWrapTextWithPunct)
	}
	if s.GrowAutofit != nil {
		m.GrowAutofit = onOffBool(s.GrowAutofit)
	}
	if s.LvlRestart != nil {
		m.LvlRestart = s.LvlRestart.Val
	}
	if s.NumIDMacAtCleanup != nil {
		m.NumIDMacAtCleanup = s.NumIDMacAtCleanup.Val
	}
	if s.SaveSmartTagsAsXml != nil {
		m.SaveSmartTagsAsXml = onOffBool(s.SaveSmartTagsAsXml)
	}
	if s.SelectFldWithFirstOrLastChar != nil {
		m.SelectFldWithFirstOrLastChar = onOffBool(s.SelectFldWithFirstOrLastChar)
	}
	if s.SplitPgBreakAndParaMark != nil {
		m.SplitPgBreakAndParaMark = onOffBool(s.SplitPgBreakAndParaMark)
	}
	if s.StyleLink != nil {
		m.StyleLink = s.StyleLink.Val
	}
	if s.SuppressBottomSpacing != nil {
		m.SuppressBottomSpacing = onOffBool(s.SuppressBottomSpacing)
	}
	if s.SuppressOverlap != nil {
		m.SuppressOverlap = onOffBool(s.SuppressOverlap)
	}
	if s.SuppressSpacingAtTopOfPage != nil {
		m.SuppressSpacingAtTopOfPage = onOffBool(s.SuppressSpacingAtTopOfPage)
	}
	if s.SuppressTopSpacing != nil {
		m.SuppressTopSpacing = onOffBool(s.SuppressTopSpacing)
	}
	if s.SuppressTopSpacingWP != nil {
		m.SuppressTopSpacingWP = onOffBool(s.SuppressTopSpacingWP)
	}
	if s.SwapBordersFacingPages != nil {
		m.SwapBordersFacingPages = onOffBool(s.SwapBordersFacingPages)
	}
	if s.TextboxTightWrap != nil {
		m.TextboxTightWrap = onOffBool(s.TextboxTightWrap)
	}
	if s.UnderlineTabInNumList != nil {
		m.UnderlineTabInNumList = onOffBool(s.UnderlineTabInNumList)
	}
	if s.UseAnsiKerningPairs != nil {
		m.UseAnsiKerningPairs = onOffBool(s.UseAnsiKerningPairs)
	}
	if s.UsePrinterMetrics != nil {
		m.UsePrinterMetrics = onOffBool(s.UsePrinterMetrics)
	}
	if s.UseSingleBorderforContiguousCells != nil {
		m.UseSingleBorderforContiguousCells = onOffBool(s.UseSingleBorderforContiguousCells)
	}
	if s.StylePaneFormatFilter != nil {
		m.StylePaneFormatFilter = s.StylePaneFormatFilter.Val
	}
	if s.StylePaneSortMethod != nil {
		m.StylePaneSortMethod = s.StylePaneSortMethod.Val
	}
	if s.Captions != nil {
		caps := Captions{}
		for _, c := range s.Captions.Captions {
			caps.Captions = append(caps.Captions, Caption{
				Name: c.Name, Position: c.Position, ChapNum: c.ChapNum,
				Heading: c.Heading, NoLabel: c.NoLabel, NumFormat: c.NumFormat,
				Separator: c.Separator,
			})
		}
		for _, ac := range s.Captions.AutoCaptions {
			caps.AutoCaptions = append(caps.AutoCaptions, AutoCaption{
				Name: ac.Name, Caption: ac.Caption,
			})
		}
		m.Captions = &caps
	}
	if s.ReadModeInkLockDown != nil {
		m.ReadModeInkLockDown = &ReadModeInkLockDown{
			ActualPage: s.ReadModeInkLockDown.ActualPage,
			Width:      s.ReadModeInkLockDown.Width,
			Height:     s.ReadModeInkLockDown.Height,
			FontSize:   s.ReadModeInkLockDown.FontSize,
		}
	}
	for _, st := range s.SmartTagTypes {
		m.SmartTagTypes = append(m.SmartTagTypes, SmartTagType{
			NamespaceURI: st.NamespaceURI, Name: st.Name, URL: st.URL,
		})
	}
	for _, as := range s.AttachedSchemas {
		m.AttachedSchemas = append(m.AttachedSchemas, as.Val)
	}
	return m
}

func resolveFontFamilies(table *parts.FontTable) map[string]FontFamilyInfo {
	resolved := map[string]FontFamilyInfo{}
	if table == nil {
		return resolved
	}
	for _, font := range table.Fonts {
		if font == nil {
			continue
		}
		name := normalizeResolvedFontName(font.Name)
		if name == "" {
			continue
		}
		resolved[name] = FontFamilyInfo{
			Family:  resolveStringVal(font.Family),
			Pitch:   resolveStringVal(font.Pitch),
			Charset: resolveStringVal(font.Charset),
			Panose1: resolveStringVal(font.Panose1),
		}
	}
	return resolved
}

func resolveLatentStyles(styles *parts.Styles) *LatentStyles {
	if styles == nil || styles.LatentStyles == nil {
		return nil
	}
	ls := &LatentStyles{}
	for _, exc := range styles.LatentStyles.Exceptions {
		ls.Exceptions = append(ls.Exceptions, LatentStyle{
			Name:           exc.Name,
			Locked:         exc.Locked,
			UIPriority:     exc.UIPriority,
			SemiHidden:     exc.SemiHidden,
			UnhideWhenUsed: exc.UnhideWhenUsed,
			QFormat:        exc.QFormat,
		})
	}
	return ls
}

func resolveStringVal(value *parts.StringVal) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(value.Val)
}

func normalizeResolvedFontName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func isSectionMarkerOnlyParagraph(src *parts.Paragraph, resolved *Paragraph) bool {
	if src == nil || src.Properties == nil || src.Properties.Section == nil || resolved == nil {
		return false
	}
	if len(resolved.Runs) != 0 || resolved.Text != "" || len(resolved.Bookmarks) != 0 || resolved.Frame != nil {
		return false
	}
	return true
}

func normalizeParagraph(ctx *resolveCtx, paragraph *parts.Paragraph) Paragraph {
	var out Paragraph
	if paragraph == nil {
		return out
	}

	if paragraph.Properties != nil {
		if paragraph.Properties.Style != nil {
			out.StyleID = paragraph.Properties.Style.Val
		}
		if paragraph.Properties.Numbering != nil {
			numbering := Numbering{}
			if paragraph.Properties.Numbering.NumID != nil {
				numbering.NumID = paragraph.Properties.Numbering.NumID.Val
			}
			if paragraph.Properties.Numbering.Level != nil {
				numbering.Level = paragraph.Properties.Numbering.Level.Val
			}
			if resolved, ok := resolveNumbering(ctx.numbering, numbering.NumID, numbering.Level); ok {
				numbering.AbstractNumID = resolved.AbstractNumID
				numbering.Start = resolved.Start
				numbering.Format = resolved.Format
				numbering.Text = resolved.Text
				numbering.IndentLeft = resolved.IndentLeft
				numbering.IndentHanging = resolved.IndentHanging
				numbering.IndentFirstLine = resolved.IndentFirstLine
				numbering.RunFormat = resolved.RunFormat
			}
			out.Numbering = &numbering
			out.List = buildListMarker(numbering)
		}
	}
	if out.StyleID == "" {
		out.StyleID = ctx.defaultStyle
	}

	// If the paragraph has no numPr of its own, inherit it from the paragraph style.
	if out.List == nil {
		if style, ok := ctx.styles[out.StyleID]; ok && style.Numbering != nil {
			numbering := Numbering{
				NumID:     style.Numbering.NumID,
				Level:     style.Numbering.Level,
				FromStyle: true,
			}
			if resolved, ok := resolveNumbering(ctx.numbering, numbering.NumID, numbering.Level); ok {
				numbering.AbstractNumID = resolved.AbstractNumID
				numbering.Start = resolved.Start
				numbering.Format = resolved.Format
				numbering.Text = resolved.Text
				numbering.IndentLeft = resolved.IndentLeft
				numbering.IndentHanging = resolved.IndentHanging
				numbering.IndentFirstLine = resolved.IndentFirstLine
			}
			out.Numbering = &numbering
			out.List = buildListMarker(numbering)
		}
	}

	// Resolve paragraph style and compute the effective run format baseline.
	paraRunFmt := cloneRunFormat(ctx.defaultRun)
	if style, ok := ctx.styles[out.StyleID]; ok {
		out.StyleName = style.Name
		out.StylePath = append([]string(nil), style.StylePath...)
		out.Format = cloneParagraphFormat(style.Format)
		paraRunFmt = cloneRunFormat(style.RunFormat)
	}
	// Apply tblStylePr conditional run/para format between style and direct formatting.
	if ctx.conditionalRunFmt != nil {
		paraRunFmt = mergeRunFormat(paraRunFmt, *ctx.conditionalRunFmt)
	}
	if ctx.conditionalParaFmt != nil {
		out.Format = mergeParagraphFormat(out.Format, *ctx.conditionalParaFmt)
	}
	// Numbering-level paragraph properties participate in the effective paragraph
	// geometry between the paragraph style and direct paragraph formatting. In
	// particular, w:lvl/w:pPr/w:ind controls the usable line width of list items;
	// dropping it makes deeply nested lists paginate much too compactly.
	if out.Numbering != nil {
		if out.Numbering.IndentLeft != 0 {
			out.Format.IndentLeft = intPtr(out.Numbering.IndentLeft)
		}
		if out.Numbering.IndentHanging != 0 {
			out.Format.IndentHanging = intPtr(out.Numbering.IndentHanging)
		}
		if out.Numbering.IndentFirstLine != 0 {
			out.Format.IndentFirstLine = intPtr(out.Numbering.IndentFirstLine)
		}
	}
	if paragraph.Properties != nil {
		out.Format = mergeParagraphFormat(out.Format, paragraphFormatFromProperties(paragraph.Properties, ctx.themeColors))
		out.PropertyRevision = normalizePropertyRevision(paragraph.Properties.PropertyChange)
		if paragraph.Properties.Frame != nil {
			out.Frame = resolveFrameProps(paragraph.Properties.Frame)
		}
		// Apply paragraph-level w:rPr — default run properties for all runs in this paragraph.
		// ECMA-376 Part 1 §17.3.1.27.
		if paragraph.Properties.RunProps != nil {
			paraRunFmt = mergeRunFormat(paraRunFmt, runFormatFromProperties(paragraph.Properties.RunProps))
		}
	}
	// Capture the effective paragraph-mark run format for renderer use (pilcrow styling,
	// empty-paragraph height, etc.). Clone so later per-run mutations don't alias it.
	cloned := cloneRunFormat(paraRunFmt)
	out.DefaultRunFormat = &cloned

	if len(paragraph.Inlines) > 0 {
		// Complex field state machine: track fldChar begin/separate/end across runs.
		// Uses a stack so nested fields (e.g. { IF { PAGE } > 1 ... }) are captured
		// as trees instead of being overwritten/flattened.
		// The stack lives on resolveCtx so fields can span paragraph boundaries.

		flushTopField := func() {
			if !ctx.inField() {
				return
			}
			top := ctx.fieldStack[len(ctx.fieldStack)-1]
			ctx.fieldStack = ctx.fieldStack[:len(ctx.fieldStack)-1]

			instr := strings.TrimSpace(top.instr)
			upper := strings.ToUpper(instr)

			// Determine field metadata from instruction.
			top.kind = fieldInstrKind(upper)
			switch top.kind {
			case "REF":
				top.anchor = parseFieldRef(instr)
				top.arg = top.anchor
				if top.anchor != "" {
					top.target = "#" + top.anchor
				}
			case "PAGEREF":
				top.anchor = parseFieldTarget(strings.TrimSpace(instr[len("PAGEREF"):]))
				top.arg = top.anchor
				if top.anchor != "" {
					top.target = "#" + top.anchor
				}
			case "HYPERLINK":
				url, anchor := parseFieldHyperlink(instr)
				top.arg = url
				top.target = url
				top.anchor = anchor
				if anchor != "" {
					if top.target != "" {
						top.target += "#" + anchor
					} else {
						top.target = "#" + anchor
					}
				}
			case "DOCPROPERTY", "DOCVARIABLE":
				top.arg = parseDocPropertyName(instr)
			case "STYLEREF":
				top.arg = parseFieldStyleRef(instr)
			case "FILLIN":
				top.arg = parseFieldFillinDefault(instr)
			case "SEQ", "ASK", "MERGEFIELD", "AUTOTEXT", "SET":
				top.arg = parseFieldLeadingArg(instr, top.kind)
			case "IF", "COMPARE", "QUOTE", "EMBED", "LINK":
				top.arg = strings.TrimSpace(instr)
			case "TOC", "INDEX", "XE", "TC":
				top.arg = strings.TrimSpace(instr)
			}

			applyFieldResultLinkMetadata(&top)

			if ctx.inField() {
				applyFieldResultSemanticMetadata(&top, instr)
				parent := ctx.topField()
				if parent.inResult {
					// Nested field inside parent's result.
					resultRuns := top.result
					if len(resultRuns) == 0 {
						resultRuns = []Run{{}}
					}
					resultRuns[0].Fields = append(resultRuns[0].Fields, Field{
						Kind:      top.kind,
						Instr:     instr,
						Arg:       top.arg,
						Anchor:    top.anchor,
						Target:    top.target,
						Children:  top.result,
						InstrRuns: top.instrRuns,
					})
					parent.result = append(parent.result, resultRuns...)
					return
				}
				// Nested field inside parent's instruction.
				displayText := aggregateFieldDisplayText(top.result)
				parent.instr += displayText
				parent.instrRuns = append(parent.instrRuns, Run{
					Text: displayText,
					Fields: []Field{{
						Kind:      top.kind,
						Instr:     instr,
						Arg:       top.arg,
						Anchor:    top.anchor,
						Target:    top.target,
						Children:  top.result,
						InstrRuns: top.instrRuns,
					}},
				})
				return
			}

			// Story-spanning field: defer to caller (normalizeContentBlocks / body loop).
			if top.paraCount > 1 {
				ctx.flushedFields = append(ctx.flushedFields, top)
				return
			}

			// Top-level paragraph-local field.
			applyFieldResultSemanticMetadata(&top, instr)

			// Form field handling.
			keyword := strings.Fields(upper)
			if len(keyword) > 0 {
				switch keyword[0] {
				case "FORMTEXT", "FORMCHECKBOX", "FORMDROPDOWN":
					out.Runs = append(out.Runs, resolveFormField(keyword[0], top.formField, top.result, paraRunFmt))
					return
				}
			}

			// TOC/INDEX/XE/TC: aggregate into a single run.
			if top.kind == "TOC" || top.kind == "INDEX" || top.kind == "XE" || top.kind == "TC" {
				base := Run{
					Kind:           "run",
					Format:         paraRunFmt,
					FieldKind:      top.kind,
					FieldArg:       top.arg,
					FieldInstr:     instr,
					FieldInstrRuns: append([]Run(nil), top.instrRuns...),
				}
				if len(top.result) > 0 {
					base.Format = top.result[0].Format
					base.Text = aggregateFieldDisplayText(top.result)
				}
				appendSemanticRuns(&out, []Run{base})
				return
			}

			// AUTOTEXT: expand from glossary when the field has no cached result.
			if top.kind == "AUTOTEXT" && len(top.result) == 0 && ctx != nil {
				entry := ctx.autoTextEntries[normalizeAutoTextName(top.arg)]
				expanded := flattenStoryToInlineRuns(entry.Story)
				if len(expanded) > 0 {
					for i := range expanded {
						if expanded[i].FieldKind == "" {
							expanded[i].FieldKind = top.kind
							expanded[i].FieldArg = top.arg
							expanded[i].FieldInstr = instr
						}
					}
					top.result = expanded
				} else if strings.TrimSpace(entry.Text) != "" {
					top.result = []Run{{
						Kind:       "run",
						Format:     paraRunFmt,
						Text:       entry.Text,
						FieldKind:  top.kind,
						FieldArg:   top.arg,
						FieldInstr: instr,
					}}
				}
			}

			if len(top.result) == 0 {
				appendSemanticRuns(&out, []Run{{
					Kind:           "run",
					Format:         paraRunFmt,
					FieldKind:      top.kind,
					FieldArg:       top.arg,
					FieldInstr:     instr,
					FieldInstrRuns: append([]Run(nil), top.instrRuns...),
				}})
				return
			}

			appendSemanticRuns(&out, top.result)
		}

		for _, inline := range paragraph.Inlines {
			switch inline.Kind {
			case parts.ParagraphInlineRun:
				run := inline.Run
				fldType, instr := runFieldContent(run)
				switch fldType {
				case "begin":
					if ctx.inResult() {
						flushTopField()
					}
					ctx.pushField(fieldFrame{
						formField:      extractFormFieldData(run),
						paraCount:      1,
						paraBoundaries: []int{0},
					})
				case "instr":
					if ctx.inField() {
						top := ctx.topField()
						top.instr += instr
						instrRuns := normalizeRun(ctx, run, "", "", paraRunFmt)
						if len(instrRuns) == 0 {
							instrRuns = []Run{{Text: instr, Kind: "run", Format: paraRunFmt}}
						}
						top.instrRuns = append(top.instrRuns, instrRuns...)
					}
				case "separate":
					if ctx.inField() {
						top := ctx.topField()
						top.inResult = true
					}
				case "end":
					if ctx.inField() {
						flushTopField()
					}
				default:
					if ctx.inResult() {
						top := ctx.topField()
						top.result = append(top.result, normalizeRun(ctx, run, "", "", paraRunFmt)...)
					} else if ctx.inField() {
						// In instruction phase: skip regular runs.
					} else {
						appendSemanticRuns(&out, normalizeRun(ctx, run, "", "", paraRunFmt))
					}
				}
			case parts.ParagraphInlineHyperlink:
				if ctx.inResult() {
					top := ctx.topField()
					top.result = append(top.result, normalizeHyperlink(ctx, inline.Hyperlink, paraRunFmt)...)
				} else if !ctx.inField() {
					appendSemanticRuns(&out, normalizeHyperlink(ctx, inline.Hyperlink, paraRunFmt))
				}
			case parts.ParagraphInlineField:
				if ctx.inResult() && inline.Field != nil {
					var paragraph Paragraph
					emitField(ctx, &paragraph, inline.Field.Instr, inline.Field.Runs, paraRunFmt, nil)
					top := ctx.topField()
					top.result = append(top.result, paragraph.Runs...)
				} else if !ctx.inField() && inline.Field != nil {
					emitField(ctx, &out, inline.Field.Instr, inline.Field.Runs, paraRunFmt, nil)
				}
			case parts.ParagraphInlineIns:
				if ctx.inResult() && inline.Run != nil {
					runs := normalizeRun(ctx, inline.Run, "", "", paraRunFmt)
					for i := range runs {
						runs[i].Revision = "inserted"
						runs[i].RevisionAuthor = inline.RevisionAuthor
						runs[i].RevisionDate = inline.RevisionDate
					}
					top := ctx.topField()
					top.result = append(top.result, runs...)
				} else if !ctx.inField() && inline.Run != nil {
					runs := normalizeRun(ctx, inline.Run, "", "", paraRunFmt)
					for i := range runs {
						runs[i].Revision = "inserted"
						runs[i].RevisionAuthor = inline.RevisionAuthor
						runs[i].RevisionDate = inline.RevisionDate
					}
					appendSemanticRuns(&out, runs)
				}
			case parts.ParagraphInlineDel:
				if ctx.inResult() && inline.Run != nil {
					runs := normalizeRun(ctx, inline.Run, "", "", paraRunFmt)
					for i := range runs {
						runs[i].Revision = "deleted"
						runs[i].RevisionAuthor = inline.RevisionAuthor
						runs[i].RevisionDate = inline.RevisionDate
					}
					top := ctx.topField()
					top.result = append(top.result, runs...)
				} else if !ctx.inField() && inline.Run != nil {
					runs := normalizeRun(ctx, inline.Run, "", "", paraRunFmt)
					for i := range runs {
						runs[i].Revision = "deleted"
						runs[i].RevisionAuthor = inline.RevisionAuthor
						runs[i].RevisionDate = inline.RevisionDate
					}
					appendSemanticRuns(&out, runs)
				}
			case parts.ParagraphInlineMoveFrom:
				if ctx.inResult() && inline.Run != nil {
					runs := normalizeRun(ctx, inline.Run, "", "", paraRunFmt)
					for i := range runs {
						runs[i].Revision = "movedFrom"
						runs[i].RevisionAuthor = inline.RevisionAuthor
						runs[i].RevisionDate = inline.RevisionDate
						runs[i].MoveID = inline.MoveID
					}
					top := ctx.topField()
					top.result = append(top.result, runs...)
				} else if !ctx.inField() && inline.Run != nil {
					runs := normalizeRun(ctx, inline.Run, "", "", paraRunFmt)
					for i := range runs {
						runs[i].Revision = "movedFrom"
						runs[i].RevisionAuthor = inline.RevisionAuthor
						runs[i].RevisionDate = inline.RevisionDate
						runs[i].MoveID = inline.MoveID
					}
					appendSemanticRuns(&out, runs)
				}
			case parts.ParagraphInlineMoveTo:
				if ctx.inResult() && inline.Run != nil {
					runs := normalizeRun(ctx, inline.Run, "", "", paraRunFmt)
					for i := range runs {
						runs[i].Revision = "movedTo"
						runs[i].RevisionAuthor = inline.RevisionAuthor
						runs[i].RevisionDate = inline.RevisionDate
						runs[i].MoveID = inline.MoveID
					}
					top := ctx.topField()
					top.result = append(top.result, runs...)
				} else if !ctx.inField() && inline.Run != nil {
					runs := normalizeRun(ctx, inline.Run, "", "", paraRunFmt)
					for i := range runs {
						runs[i].Revision = "movedTo"
						runs[i].RevisionAuthor = inline.RevisionAuthor
						runs[i].RevisionDate = inline.RevisionDate
						runs[i].MoveID = inline.MoveID
					}
					appendSemanticRuns(&out, runs)
				}
			case parts.ParagraphInlineMath:
				if !ctx.inField() && inline.Math != nil {
					if mml := mathml.Convert(inline.Math.RawXML); mml != "" {
						out.Runs = append(out.Runs, Run{Kind: "math", MathML: mml})
					}
				}
			case parts.ParagraphInlineSDT:
				if !ctx.inField() {
					cc := normalizeContentControlWithContext(ctx, inline.SdtProps)
					if expanded, ok := expandSdtGlossaryInlines(ctx, cc, inlineText(inline.SdtInlines)); ok {
						for _, run := range expanded {
							run.ContentControl = cc
							appendSemanticRuns(&out, []Run{run})
						}
					} else {
						for _, child := range normalizeSdtInlineRuns(ctx, inline.SdtInlines, paraRunFmt) {
							child.ContentControl = cc
							appendSemanticRuns(&out, []Run{child})
						}
					}
				}
			case parts.ParagraphInlineRuby:
				if !ctx.inField() && inline.Ruby != nil {
					if r := normalizeRuby(ctx, inline.Ruby, paraRunFmt); r != nil {
						out.Runs = append(out.Runs, *r)
					}
				}
			case parts.ParagraphInlineCustomXml:
				if !ctx.inField() {
					cx := normalizeCustomXml(inline.CustomXml)
					for _, child := range normalizeCustomXmlInlineRuns(ctx, inline.CustomXmlInlines, paraRunFmt) {
						child.CustomXml = cx
						appendSemanticRuns(&out, []Run{child})
					}
				}
			}
		}
		// Unclosed fields are now story-scoped; do NOT flush them here.
	} else {
		for _, run := range paragraph.Runs {
			appendSemanticRuns(&out, normalizeRun(ctx, run, "", "", paraRunFmt))
		}
		for _, hyperlink := range paragraph.Hyperlinks {
			appendSemanticRuns(&out, normalizeHyperlink(ctx, hyperlink, paraRunFmt))
		}
	}

	if len(paragraph.Bookmarks) > 0 {
		out.Bookmarks = append([]string(nil), paragraph.Bookmarks...)
	}
	if len(paragraph.CommentRangeStarts) > 0 {
		out.CommentRangeStarts = append([]int(nil), paragraph.CommentRangeStarts...)
	}
	if len(paragraph.CommentRangeEnds) > 0 {
		out.CommentRangeEnds = append([]int(nil), paragraph.CommentRangeEnds...)
	}
	for _, mrs := range paragraph.MoveFromRangeStarts {
		out.MoveFromRangeStarts = append(out.MoveFromRangeStarts, MoveRange{ID: mrs.ID, Author: mrs.Author, Date: mrs.Date, Name: mrs.Name})
	}
	for _, mre := range paragraph.MoveFromRangeEnds {
		out.MoveFromRangeEnds = append(out.MoveFromRangeEnds, mre.ID)
	}
	for _, mrs := range paragraph.MoveToRangeStarts {
		out.MoveToRangeStarts = append(out.MoveToRangeStarts, MoveRange{ID: mrs.ID, Author: mrs.Author, Date: mrs.Date, Name: mrs.Name})
	}
	for _, mre := range paragraph.MoveToRangeEnds {
		out.MoveToRangeEnds = append(out.MoveToRangeEnds, mre.ID)
	}
	for _, ps := range paragraph.PermStarts {
		out.PermRangeStarts = append(out.PermRangeStarts, PermRange{ID: ps.ID, EdGrp: ps.EdGrp, Ed: ps.Ed, ColFirst: ps.ColFirst, ColLast: ps.ColLast})
	}
	for _, pe := range paragraph.PermEnds {
		out.PermRangeEnds = append(out.PermRangeEnds, pe.ID)
	}
	for _, mrs := range paragraph.CustomXmlInsRangeStarts {
		out.CustomXmlInsRangeStarts = append(out.CustomXmlInsRangeStarts, MoveRange{ID: mrs.ID, Author: mrs.Author, Date: mrs.Date, Name: mrs.Name})
	}
	for _, mre := range paragraph.CustomXmlInsRangeEnds {
		out.CustomXmlInsRangeEnds = append(out.CustomXmlInsRangeEnds, mre.ID)
	}
	for _, mrs := range paragraph.CustomXmlDelRangeStarts {
		out.CustomXmlDelRangeStarts = append(out.CustomXmlDelRangeStarts, MoveRange{ID: mrs.ID, Author: mrs.Author, Date: mrs.Date, Name: mrs.Name})
	}
	for _, mre := range paragraph.CustomXmlDelRangeEnds {
		out.CustomXmlDelRangeEnds = append(out.CustomXmlDelRangeEnds, mre.ID)
	}
	for _, mrs := range paragraph.CustomXmlMoveFromRangeStarts {
		out.CustomXmlMoveFromRangeStarts = append(out.CustomXmlMoveFromRangeStarts, MoveRange{ID: mrs.ID, Author: mrs.Author, Date: mrs.Date, Name: mrs.Name})
	}
	for _, mre := range paragraph.CustomXmlMoveFromRangeEnds {
		out.CustomXmlMoveFromRangeEnds = append(out.CustomXmlMoveFromRangeEnds, mre.ID)
	}
	for _, mrs := range paragraph.CustomXmlMoveToRangeStarts {
		out.CustomXmlMoveToRangeStarts = append(out.CustomXmlMoveToRangeStarts, MoveRange{ID: mrs.ID, Author: mrs.Author, Date: mrs.Date, Name: mrs.Name})
	}
	for _, mre := range paragraph.CustomXmlMoveToRangeEnds {
		out.CustomXmlMoveToRangeEnds = append(out.CustomXmlMoveToRangeEnds, mre.ID)
	}

	// Correlate inline move ranges with run positions
	out.InlineMoveRanges = correlateInlineMoveRanges(paragraph, out.Runs)

	return out
}

// correlateInlineMoveRanges correlates move-range boundaries with inline positions
// within a paragraph's runs. This enables renderers to emit inline spans for
// tracked move ranges.
func correlateInlineMoveRanges(paragraph *parts.Paragraph, runs []Run) []InlineMoveRange {
	if len(paragraph.MoveFromRangeStarts) == 0 && len(paragraph.MoveToRangeStarts) == 0 {
		return nil
	}

	var result []InlineMoveRange

	// Build a map of move IDs to their start metadata
	moveStarts := make(map[int]parts.MoveRangeStart)
	for _, mrs := range paragraph.MoveFromRangeStarts {
		moveStarts[mrs.ID] = mrs
	}
	for _, mrs := range paragraph.MoveToRangeStarts {
		moveStarts[mrs.ID] = mrs
	}

	// Find inline positions for each move range
	for id, mrs := range moveStarts {
		startRun, startChar := findMoveRangePosition(paragraph, runs, id, true)
		endRun, endChar := findMoveRangePosition(paragraph, runs, id, false)

		// Adjust end position to be within bounds
		if endRun >= 0 && endRun < len(runs) {
			result = append(result, InlineMoveRange{
				MoveRange: MoveRange{
					ID:     id,
					Author: mrs.Author,
					Date:   mrs.Date,
					Name:   mrs.Name,
				},
				StartRunIndex:   startRun,
				EndRunIndex:     endRun,
				StartCharOffset: startChar,
				EndCharOffset:   endChar,
			})
		}
	}

	return result
}

// findMoveRangePosition finds the run index and character offset for a move range
// boundary within resolved semantic runs. It locates the first (isStart) or last
// (!isStart) run that carries the matching MoveID and revision type, deriving the
// boundary from the moved content itself since the paragraph parser does not retain
// inline indices for empty range-marker elements.
func findMoveRangePosition(paragraph *parts.Paragraph, runs []Run, moveID int, isStart bool) (runIndex, charOffset int) {
	runIndex = -1
	charOffset = -1

	// Determine whether this moveID belongs to a moveFrom or moveTo range
	// within this paragraph. In well-formed documents a given ID appears in
	// only one of the two lists per paragraph.
	targetRevision := "movedFrom"
	for _, mrs := range paragraph.MoveToRangeStarts {
		if mrs.ID == moveID {
			targetRevision = "movedTo"
			break
		}
	}

	if isStart {
		for i, run := range runs {
			if run.MoveID == moveID && run.Revision == targetRevision {
				runIndex = i
				charOffset = 0
				return
			}
		}
	} else {
		for i := len(runs) - 1; i >= 0; i-- {
			if runs[i].MoveID == moveID && runs[i].Revision == targetRevision {
				runIndex = i
				charOffset = len(runs[i].Text)
				return
			}
		}
	}
	return
}

func tableFormatFromProperties(p *parts.TableProperties, themeColors map[string]string) TableFormat {
	var out TableFormat
	if p == nil {
		return out
	}
	if p.Width != nil && p.Width.Type != "nil" && p.Width.Type != "auto" {
		v := p.Width.Width
		out.Width = &v
		out.WidthType = p.Width.Type
	}
	if p.Layout != nil {
		out.Layout = p.Layout.Type
	}
	if p.Alignment != nil {
		out.Alignment = p.Alignment.Val
	}
	if p.Borders != nil {
		out.Borders = resolveTableBorders(p.Borders)
	}
	if p.Shading != nil {
		if fill := resolveShadingFill(p.Shading, themeColors); fill != "" {
			out.Shading = fill
		}
	}
	if p.Indent != nil && p.Indent.Type == "dxa" {
		out.Indent = p.Indent.Width
	}
	if p.CellSpacing != nil && p.CellSpacing.Type == "dxa" {
		out.CellSpacing = p.CellSpacing.Width
	}
	if m := p.CellMargin; m != nil {
		if m.Top != nil && m.Top.Type == "dxa" {
			out.CellPadTop = m.Top.Width
		}
		if m.Bottom != nil && m.Bottom.Type == "dxa" {
			out.CellPadBottom = m.Bottom.Width
		}
		if m.Left != nil && m.Left.Type == "dxa" {
			out.CellPadLeft = m.Left.Width
		}
		if m.Right != nil && m.Right.Type == "dxa" {
			out.CellPadRight = m.Right.Width
		}
	}
	if fp := p.FloatProps; fp != nil {
		out.FloatProps = &TableFloatProps{
			HorzAnchor:     fp.HorzAnchor,
			VertAnchor:     fp.VertAnchor,
			X:              fp.X,
			Y:              fp.Y,
			XAlign:         fp.XAlign,
			YAlign:         fp.YAlign,
			LeftFromText:   fp.LeftFromText,
			RightFromText:  fp.RightFromText,
			TopFromText:    fp.TopFromText,
			BottomFromText: fp.BottomFromText,
		}
	}
	if p.Overlap != nil {
		out.Overlap = p.Overlap.Val
	}
	out.Caption = p.Caption
	out.Description = p.Description
	if p.BidiVisual != nil && onOffBool(p.BidiVisual) {
		out.BidiVisual = true
	}
	if p.Look != nil {
		out.Look = &TableLook{
			Val:         p.Look.Val,
			FirstRow:    p.Look.FirstRow == "1" || p.Look.FirstRow == "true",
			LastRow:     p.Look.LastRow == "1" || p.Look.LastRow == "true",
			FirstColumn: p.Look.FirstCol == "1" || p.Look.FirstCol == "true",
			LastColumn:  p.Look.LastCol == "1" || p.Look.LastCol == "true",
			NoHBand:     p.Look.NoHBand == "1" || p.Look.NoHBand == "true",
			NoVBand:     p.Look.NoVBand == "1" || p.Look.NoVBand == "true",
		}
	}
	if p.StyleRowBandSize != nil {
		out.StyleRowBandSize = p.StyleRowBandSize.Val
	}
	if p.StyleColBandSize != nil {
		out.StyleColBandSize = p.StyleColBandSize.Val
	}
	return out
}

func tableRowOverrideFromTblPrEx(ex *parts.TablePropertiesException, themeColors map[string]string) *TableRowOverride {
	if ex == nil {
		return nil
	}
	var out TableRowOverride
	if ex.Shading != nil {
		if fill := resolveShadingFill(ex.Shading, themeColors); fill != "" {
			out.Shading = fill
		}
	}
	if ex.Borders != nil {
		out.Borders = resolveTableBorders(ex.Borders)
	}
	if ex.Alignment != nil {
		out.Alignment = ex.Alignment.Val
	}
	if ex.Indent != nil && ex.Indent.Type == "dxa" {
		out.Indent = ex.Indent.Width
	}
	if ex.CellMargin != nil {
		if ex.CellMargin.Top != nil && ex.CellMargin.Top.Type == "dxa" {
			out.CellPadTop = ex.CellMargin.Top.Width
		}
		if ex.CellMargin.Bottom != nil && ex.CellMargin.Bottom.Type == "dxa" {
			out.CellPadBottom = ex.CellMargin.Bottom.Width
		}
		if ex.CellMargin.Left != nil && ex.CellMargin.Left.Type == "dxa" {
			out.CellPadLeft = ex.CellMargin.Left.Width
		}
		if ex.CellMargin.Right != nil && ex.CellMargin.Right.Type == "dxa" {
			out.CellPadRight = ex.CellMargin.Right.Width
		}
	}
	if ex.BidiVisual != nil && onOffBool(ex.BidiVisual) {
		out.BidiVisual = true
	}
	// CellSpacing in tblPrEx is handled via the same path as trPr/tblCellSpacing
	// because both end up as row-level cell spacing. We intentionally do not
	// duplicate it here; normalizeTableRow reads trPr.CellSpacing directly.
	if out.Shading == "" && out.Borders == nil && out.Alignment == "" && out.Indent == 0 &&
		out.CellPadTop == 0 && out.CellPadBottom == 0 && out.CellPadLeft == 0 && out.CellPadRight == 0 && !out.BidiVisual {
		return nil
	}
	return &out
}

func normalizeTable(ctx *resolveCtx, table *parts.Table) Table {
	var out Table
	if table == nil {
		return out
	}

	// Look up base table format and conditional formatting from the table style.
	var baseFormat TableFormat
	var tblStyleProps []ConditionalTableStyle
	var tblLook *parts.TableLook
	styleName := ""
	if table.Properties != nil {
		if table.Properties.Style != nil {
			styleName = table.Properties.Style.Val
		}
		tblLook = table.Properties.Look
	}
	if styleName == "" && ctx.defaultTableStyle != "" {
		styleName = ctx.defaultTableStyle
	}
	if styleName != "" {
		if ts, ok := ctx.styles[styleName]; ok {
			if ts.TableFormat != nil {
				baseFormat = *ts.TableFormat
			}
			tblStyleProps = ts.TableStyles
		}
	}

	if table.Properties != nil {
		p := table.Properties
		out.PropertyRevision = normalizePropertyRevision(p.PropertyChange)
		out.Format = mergeTableFormat(baseFormat, tableFormatFromProperties(p, ctx.themeColors))
	} else {
		out.Format = baseFormat
	}

	if table.Grid != nil && len(table.Grid.Columns) > 0 {
		out.Format.ColumnWidths = append([]int(nil), table.Grid.Columns...)
	}
	totalRows := len(table.Rows)
	totalCols := len(out.Format.ColumnWidths)

	for rowIdx, row := range table.Rows {
		normalizedRow := normalizeTableRow(ctx, row, rowIdx, totalRows, totalCols, tblStyleProps, tblLook)
		for _, cell := range normalizedRow.Cells {
			if out.Text != "" && cell.Text != "" {
				out.Text += " | "
			}
			out.Text += cell.Text
		}
		out.Rows = append(out.Rows, normalizedRow)
	}

	return out
}

func normalizeTableRow(ctx *resolveCtx, row *parts.TableRow, rowIdx int, totalRows int, totalCols int, tblStyleProps []ConditionalTableStyle, look *parts.TableLook) TableRow {
	var out TableRow
	if row == nil {
		return out
	}

	if row.Properties != nil {
		p := row.Properties
		out.PropertyRevision = normalizePropertyRevision(p.PropertyChange)
		if p.Height != nil {
			v := p.Height.Val
			out.Format.Height = &v
			out.Format.HeightRule = p.Height.Rule
		}
		if p.IsHeader != nil {
			out.Format.IsHeader = p.IsHeader.Val == nil || bool(*p.IsHeader.Val)
		}
		if p.GridBefore != nil {
			out.Format.GridBefore = p.GridBefore.Val
		}
		if p.GridAfter != nil {
			out.Format.GridAfter = p.GridAfter.Val
		}
		if p.WidthBefore != nil && p.WidthBefore.Type != "nil" && p.WidthBefore.Type != "auto" {
			v := p.WidthBefore.Width
			out.Format.WidthBefore = &v
			out.Format.WidthBeforeType = p.WidthBefore.Type
		}
		if p.WidthAfter != nil && p.WidthAfter.Type != "nil" && p.WidthAfter.Type != "auto" {
			v := p.WidthAfter.Width
			out.Format.WidthAfter = &v
			out.Format.WidthAfterType = p.WidthAfter.Type
		}
		if p.CantSplit != nil && onOffBool(p.CantSplit) {
			out.Format.CantSplit = true
		}
		if p.Hidden != nil && onOffBool(p.Hidden) {
			out.Format.Hidden = true
		}
		if p.Justification != nil {
			out.Format.Justification = p.Justification.Val
		}
		if p.Conditional != nil {
			out.Format.Conditional = p.Conditional.Val
		}
		if p.DivID != nil {
			out.Format.DivID = p.DivID.Val
		}
		if p.CellSpacing != nil && p.CellSpacing.Type == "dxa" {
			out.Format.CellSpacing = p.CellSpacing.Width
		}
	}
	out.Override = tableRowOverrideFromTblPrEx(row.TblPrEx, ctx.themeColors)

	colIdx := 0
	for _, cell := range row.Cells {
		out.Cells = append(out.Cells, normalizeTableCell(ctx, cell, rowIdx, totalRows, colIdx, totalCols, tblStyleProps, look))
		span := 1
		if cell != nil && cell.Properties != nil && cell.Properties.GridSpan != nil {
			span = cell.Properties.GridSpan.Val
			if span < 1 {
				span = 1
			}
		}
		colIdx += span
	}
	return out
}

func normalizeTableCell(ctx *resolveCtx, cell *parts.TableCell, rowIdx int, totalRows int, colIdx int, totalCols int, tblStyleProps []ConditionalTableStyle, look *parts.TableLook) TableCell {
	var out TableCell
	if cell == nil {
		return out
	}
	if cell.Properties != nil {
		out.PropertyRevision = normalizePropertyRevision(cell.Properties.PropertyChange)
		if cell.Properties.CellIns != nil {
			out.CellIns = &RevisionMark{Author: cell.Properties.CellIns.Author, Date: cell.Properties.CellIns.Date, ID: cell.Properties.CellIns.ID}
		}
		if cell.Properties.CellDel != nil {
			out.CellDel = &RevisionMark{Author: cell.Properties.CellDel.Author, Date: cell.Properties.CellDel.Date, ID: cell.Properties.CellDel.ID}
		}
		if cell.Properties.CellMerge != nil {
			out.CellMerge = &RevisionMark{Author: cell.Properties.CellMerge.Author, Date: cell.Properties.CellMerge.Date, ID: cell.Properties.CellMerge.ID}
		}
	}

	// Apply tblStylePr conditional overlays so normalizeParagraph picks them up.
	var prevRunFmt *RunFormat
	var prevParaFmt *ParagraphFormat
	if len(tblStyleProps) > 0 {
		types := tableConditionalTypes(rowIdx, totalRows, colIdx, totalCols, look)
		cellOverlay, runOverlay, paraOverlay := buildConditionalOverlay(tblStyleProps, types)
		out.Format = mergeTableCellFormat(out.Format, cellOverlay)
		prevRunFmt = ctx.conditionalRunFmt
		prevParaFmt = ctx.conditionalParaFmt
		ctx.conditionalRunFmt = &runOverlay
		ctx.conditionalParaFmt = &paraOverlay
	}

	if cell.Properties != nil {
		out.Format = mergeTableCellFormat(out.Format, tableCellFormatFromProperties(cell.Properties, ctx.themeColors))
	}

	blocks := cell.Blocks
	paragraphs := cell.Paragraphs
	tables := cell.Tables
	if len(blocks) == 1 && blocks[0].Kind == parts.ContentBlockSDT {
		out.ContentControl = normalizeContentControlWithContext(ctx, blocks[0].SdtProps)
		blocks = blocks[0].SdtBlocks
		paragraphs = nil
		tables = nil
	}
	out.Paragraphs, out.Tables, out.Blocks, out.Text = normalizeContentBlocks(ctx, blocks, paragraphs, tables)

	if len(tblStyleProps) > 0 {
		ctx.conditionalRunFmt = prevRunFmt
		ctx.conditionalParaFmt = prevParaFmt
	}

	return out
}

func normalizeStory(ctx *resolveCtx, blocks []parts.ContentBlock, paragraphs []*parts.Paragraph, tables []*parts.Table) Story {
	var out Story
	out.Paragraphs, out.Tables, out.Blocks, out.Text = normalizeContentBlocks(ctx, blocks, paragraphs, tables)
	out.Fields = append(out.Fields, ctx.storyFields...)
	ctx.storyFields = ctx.storyFields[:0]
	return out
}

func normalizeContentBlocks(ctx *resolveCtx, blocks []parts.ContentBlock, paragraphs []*parts.Paragraph, tables []*parts.Table) ([]Paragraph, []Table, []Block, string) {
	var outParagraphs []Paragraph
	var outTables []Table
	var outBlocks []Block
	var outFields []StoryField
	text := ""

	addParagraph := func(p *parts.Paragraph) {
		if ctx.inField() {
			top := ctx.topField()
			top.paraCount++
			top.paraBoundaries = append(top.paraBoundaries, len(top.result))
		}
		resolved := normalizeParagraph(ctx, p)
		resolved.Index = ctx.nextParaIndex
		ctx.nextParaIndex++
		outParagraphs = append(outParagraphs, resolved)
		paragraphCopy := resolved
		block := Block{
			Index:     len(outBlocks),
			Kind:      "paragraph",
			Paragraph: &paragraphCopy,
		}
		outBlocks = append(outBlocks, block)
		attachParagraphToField(ctx, len(outParagraphs)-1, block)
		if text != "" && resolved.Text != "" {
			text += "\n"
		}
		text += resolved.Text
	}
	addTable := func(t *parts.Table) {
		resolved := normalizeTable(ctx, t)
		outTables = append(outTables, resolved)
		tableCopy := resolved
		block := Block{
			Index: len(outBlocks),
			Kind:  "table",
			Table: &tableCopy,
		}
		outBlocks = append(outBlocks, block)
		attachTableToField(ctx, block)
		if text != "" && resolved.Text != "" {
			text += "\n"
		}
		text += resolved.Text
	}

	if len(blocks) == 0 {
		for _, paragraph := range paragraphs {
			addParagraph(paragraph)
		}
		for _, table := range tables {
			addTable(table)
		}
		outFields = append(outFields, extractStoryFields(ctx, outParagraphs)...)
		return outParagraphs, outTables, outBlocks, text
	}

	for _, block := range blocks {
		switch block.Kind {
		case parts.ContentBlockParagraph:
			addParagraph(block.Paragraph)
			outFields = append(outFields, extractStoryFields(ctx, outParagraphs)...)
		case parts.ContentBlockTable:
			addTable(block.Table)
			outFields = append(outFields, extractStoryFields(ctx, outParagraphs)...)
		case parts.ContentBlockSDT:
			cc := normalizeContentControlWithContext(ctx, block.SdtProps)
			_, _, innerBlocks, innerText := normalizeSdtContentBlocks(ctx, cc, block.SdtBlocks)
			for i := range innerBlocks {
				innerBlocks[i].ContentControl = cc
				innerBlocks[i].Index = len(outBlocks)
				outBlocks = append(outBlocks, innerBlocks[i])
			}
			if text != "" && innerText != "" {
				text += "\n"
			}
			text += innerText
			outFields = append(outFields, extractStoryFields(ctx, outParagraphs)...)
		case parts.ContentBlockCustomXml:
			cx := normalizeCustomXml(block.CustomXml)
			for _, inner := range block.CustomXmlBlocks {
				switch inner.Kind {
				case parts.ContentBlockParagraph:
					if ctx.inField() {
						top := ctx.topField()
						top.paraCount++
						top.paraBoundaries = append(top.paraBoundaries, len(top.result))
					}
					resolved := normalizeParagraph(ctx, inner.Paragraph)
					resolved.Index = ctx.nextParaIndex
					ctx.nextParaIndex++
					outParagraphs = append(outParagraphs, resolved)
					paragraphCopy := resolved
					b := Block{
						Index:     len(outBlocks),
						Kind:      "paragraph",
						Paragraph: &paragraphCopy,
						CustomXml: cx,
					}
					outBlocks = append(outBlocks, b)
					attachParagraphToField(ctx, len(outParagraphs)-1, b)
					if text != "" && resolved.Text != "" {
						text += "\n"
					}
					text += resolved.Text
				case parts.ContentBlockTable:
					resolved := normalizeTable(ctx, inner.Table)
					outTables = append(outTables, resolved)
					tableCopy := resolved
					b := Block{
						Index:     len(outBlocks),
						Kind:      "table",
						Table:     &tableCopy,
						CustomXml: cx,
					}
					outBlocks = append(outBlocks, b)
					attachTableToField(ctx, b)
					if text != "" && resolved.Text != "" {
						text += "\n"
					}
					text += resolved.Text
				}
			}
			outFields = append(outFields, extractStoryFields(ctx, outParagraphs)...)
		}
	}

	// Flush any remaining story-spanning fields at story boundary.
	for len(ctx.fieldStack) > 0 {
		top := ctx.fieldStack[len(ctx.fieldStack)-1]
		ctx.fieldStack = ctx.fieldStack[:len(ctx.fieldStack)-1]
		if top.paraCount > 1 {
			ctx.flushedFields = append(ctx.flushedFields, top)
		}
	}
	outFields = append(outFields, extractStoryFields(ctx, outParagraphs)...)

	ctx.storyFields = append(ctx.storyFields, outFields...)
	return outParagraphs, outTables, outBlocks, text
}

func normalizeSection(pkg *parts.Package, sectionProperties *parts.SectionProperties) Section {
	section := Section{}
	if sectionProperties == nil {
		return section
	}
	if sectionProperties.Type != nil {
		section.Type = sectionProperties.Type.Val
		// w:sectPr/w:type@w:val carries the section break vocabulary that
		// the legacy parser also exposes via Section.BreakKind.
		section.BreakKind = sectionProperties.Type.Val
	}
	if sectionProperties.TitlePage != nil && onOffBool(sectionProperties.TitlePage) {
		section.TitlePage = true
	}
	section.PropertyRevision = normalizePropertyRevision(sectionProperties.PropertyChange)
	if sectionProperties.VAlign != nil && sectionProperties.VAlign.Val != "" {
		section.VerticalAlign = sectionProperties.VAlign.Val
	}
	if sectionProperties.PageNumType != nil {
		section.PageNumberRestart = true
		if sectionProperties.PageNumType.Format != "" {
			section.PageNumberFormat = sectionProperties.PageNumType.Format
		}
		if sectionProperties.PageNumType.Start > 0 {
			v := sectionProperties.PageNumType.Start
			section.PageNumberStart = &v
		}
	}
	section.PageSize = normalizePageSize(sectionProperties.PageSize)
	section.PageMargin = normalizePageMargin(sectionProperties.PageMargin, sectionProperties.RtlGutter != nil && onOffBool(sectionProperties.RtlGutter))
	section.Columns = normalizeColumns(sectionProperties.Columns)
	section.DocGrid = normalizeDocGrid(sectionProperties.DocGrid)
	if sectionProperties.PaperSrc != nil {
		section.PaperSource = &PaperSource{
			First: sectionProperties.PaperSrc.First,
			Other: sectionProperties.PaperSrc.Other,
		}
	}
	section.Headers = normalizeSectionReferences(pkg, sectionProperties.HeaderRefs)
	section.Footers = normalizeSectionReferences(pkg, sectionProperties.FooterRefs)
	if sectionProperties.TextDirection != nil {
		section.TextDirection = sectionProperties.TextDirection.Val
	}
	if sectionProperties.Bidi != nil && onOffBool(sectionProperties.Bidi) {
		section.Bidi = true
	}
	section.PageBorders = normalizePageBorders(sectionProperties.PageBorders)
	if sectionProperties.FormProt != nil && onOffBool(sectionProperties.FormProt) {
		section.FormProt = true
	}
	if ln := sectionProperties.LnNumType; ln != nil {
		countBy := ln.CountBy
		if countBy <= 0 {
			countBy = 1
		}
		start := ln.Start
		if start <= 0 {
			start = 1
		}
		restart := ln.Restart
		if restart == "" {
			restart = "newPage"
		}
		section.LineNumbering = &LineNumbering{CountBy: countBy, Start: start, Restart: restart}
	}
	if fn := sectionProperties.FootnoteProps; fn != nil {
		if fn.Position != nil {
			section.FootnotePos = fn.Position.Val
		}
		if fn.Format != nil {
			section.FootnoteNumFmt = fn.Format.Val
		}
		if fn.Start != nil {
			section.FootnoteNumStart = fn.Start.Val
		}
		if fn.Restart != nil {
			section.FootnoteRestart = fn.Restart.Val
		}
	}
	if en := sectionProperties.EndnoteProps; en != nil {
		if en.Format != nil {
			section.EndnoteNumFmt = en.Format.Val
		}
		if en.Start != nil {
			section.EndnoteNumStart = en.Start.Val
		}
		if en.Restart != nil {
			section.EndnoteRestart = en.Restart.Val
		}
	}
	if sectionProperties.NoEndnote != nil && onOffBool(sectionProperties.NoEndnote) {
		section.EndnoteAtSectionEnd = false
	} else {
		section.EndnoteAtSectionEnd = true
	}
	return section
}

// inheritSectionProperties copies missing page-layout properties from each
// section to the next, matching Word's behaviour where omitted section
// properties carry forward.
func inheritSectionProperties(sections []Section) {
	for i := 1; i < len(sections); i++ {
		prev := &sections[i-1]
		cur := &sections[i]
		if cur.PageSize == nil {
			cur.PageSize = prev.PageSize
		}
		if cur.PageMargin == nil {
			cur.PageMargin = prev.PageMargin
		}
		if cur.Columns == nil {
			cur.Columns = prev.Columns
		}
		if cur.DocGrid == nil {
			cur.DocGrid = prev.DocGrid
		}
		if cur.LineNumbering == nil {
			cur.LineNumbering = prev.LineNumbering
		}
		if cur.PageBorders == nil {
			cur.PageBorders = prev.PageBorders
		}
		if cur.TextDirection == "" {
			cur.TextDirection = prev.TextDirection
		}
		if cur.VerticalAlign == "" {
			cur.VerticalAlign = prev.VerticalAlign
		}
		if !cur.Bidi {
			cur.Bidi = prev.Bidi
		}
		if cur.FootnotePos == "" {
			cur.FootnotePos = prev.FootnotePos
		}
		if cur.FootnoteNumFmt == "" {
			cur.FootnoteNumFmt = prev.FootnoteNumFmt
		}
		if cur.FootnoteNumStart == 0 {
			cur.FootnoteNumStart = prev.FootnoteNumStart
		}
		if cur.FootnoteRestart == "" {
			cur.FootnoteRestart = prev.FootnoteRestart
		}
		if cur.EndnoteNumFmt == "" {
			cur.EndnoteNumFmt = prev.EndnoteNumFmt
		}
		if cur.EndnoteNumStart == 0 {
			cur.EndnoteNumStart = prev.EndnoteNumStart
		}
		if cur.EndnoteRestart == "" {
			cur.EndnoteRestart = prev.EndnoteRestart
		}
		if !cur.EndnoteAtSectionEnd && prev.EndnoteAtSectionEnd {
			cur.EndnoteAtSectionEnd = prev.EndnoteAtSectionEnd
		}
		if !cur.FormProt && prev.FormProt {
			cur.FormProt = prev.FormProt
		}
		if cur.PaperSource == nil {
			cur.PaperSource = prev.PaperSource
		}
	}
}

func normalizeSectionReferences(pkg *parts.Package, refs []parts.HeaderFooterReference) []Reference {
	if len(refs) == 0 {
		return nil
	}
	resolved := make([]Reference, 0, len(refs))
	for _, ref := range refs {
		if pkg == nil || ref.ID == "" {
			continue
		}
		if target, ok := pkg.RelationshipTarget(pkg.MainPartName, ref.ID); ok {
			resolved = append(resolved, Reference{
				Type:     ref.Type,
				PartName: target,
			})
		}
	}
	return resolved
}

func lastSectionProperties(document *parts.Document) *parts.SectionProperties {
	if document == nil || document.Body == nil {
		return nil
	}
	if document.Body.Section != nil {
		return document.Body.Section
	}
	for i := len(document.Body.Paragraphs) - 1; i >= 0; i-- {
		paragraph := document.Body.Paragraphs[i]
		if paragraph != nil && paragraph.Properties != nil && paragraph.Properties.Section != nil {
			return paragraph.Properties.Section
		}
	}
	return nil
}

func normalizePageSize(pageSize *parts.PageSize) *PageSize {
	if pageSize == nil {
		return nil
	}
	return &PageSize{
		Width:  pageSize.Width,
		Height: pageSize.Height,
		Orient: pageSize.Orient,
	}
}

func normalizePageMargin(pageMargin *parts.PageMargin, rtlGutter bool) *PageMargin {
	if pageMargin == nil {
		return nil
	}
	return &PageMargin{
		Top:       pageMargin.Top,
		Right:     pageMargin.Right,
		Bottom:    pageMargin.Bottom,
		Left:      pageMargin.Left,
		Header:    pageMargin.Header,
		Footer:    pageMargin.Footer,
		Gutter:    pageMargin.Gutter,
		RtlGutter: rtlGutter,
	}
}

func normalizeColumns(columns *parts.Columns) *Columns {
	if columns == nil {
		return nil
	}
	// Preserve w:cols/@w:space exactly as authored. Although the OOXML simple
	// type has a constrained range, real documents can intentionally contain
	// out-of-range values and Word lays those values out literally. Guessing
	// that a large value is EMU changes document geometry dramatically (for
	// example, 360000 twips becomes ~567 twips) and destroys pagination.
	return &Columns{
		Num:   columns.Num,
		Space: columns.Space,
		Sep:   columns.Sep,
	}
}

func normalizeDocGrid(docGrid *parts.DocGrid) *DocGrid {
	if docGrid == nil {
		return nil
	}
	return &DocGrid{
		LinePitch: docGrid.LinePitch,
		CharSpace: docGrid.CharSpace,
		Type:      docGrid.Type,
	}
}

func normalizePageBorders(b *parts.PageBorders) *PageBorders {
	if b == nil {
		return nil
	}
	return &PageBorders{
		Top:        resolveBorderSide(b.Top),
		Left:       resolveBorderSide(b.Left),
		Start:      resolveBorderSide(b.Start),
		Bottom:     resolveBorderSide(b.Bottom),
		Right:      resolveBorderSide(b.Right),
		End:        resolveBorderSide(b.End),
		Display:    b.Display,
		OffsetFrom: b.OffsetFrom,
		ZOrder:     b.ZOrder,
	}
}

func normalizeRun(ctx *resolveCtx, run *parts.Run, target, anchor string, paraRunFmt RunFormat) []Run {
	if run == nil {
		return nil
	}

	// Effective run format: paragraph-style rPr chain → character style chain → direct rPr.
	format := cloneRunFormat(paraRunFmt)
	styleID := ""
	if run.Properties != nil && run.Properties.RunStyle != nil {
		styleID = run.Properties.RunStyle.Val
		if charStyle, ok := ctx.charStyles[run.Properties.RunStyle.Val]; ok {
			format = mergeRunFormat(format, charStyle.Format)
		}
	}
	format = mergeRunFormat(format, runFormatFromPropertiesWithTheme(run.Properties, nil, ctx.themeColors))
	propRevision := (*RevisionMark)(nil)
	if run.Properties != nil {
		propRevision = normalizePropertyRevision(run.Properties.PropertyChange)
	}

	// Apply theme color when w:themeColor is set (overrides literal w:color val).
	// Also apply w:themeShade / w:themeTint to darken or lighten the base hex.
	if run.Properties != nil && run.Properties.Color != nil && run.Properties.Color.ThemeColor != "" {
		if hex, ok := ctx.themeColors[run.Properties.Color.ThemeColor]; ok {
			format.Color = applyWordColorModifier(hex, run.Properties.Color.ThemeShade, run.Properties.Color.ThemeTint)
		}
	}

	// Apply theme fonts when explicit face is absent.
	if run.Properties != nil && run.Properties.Fonts != nil {
		f := run.Properties.Fonts
		if format.FontASCII == "" && f.AsciiTheme != "" {
			format.FontASCII = resolveFontSlotWithLang(ctx, f.AsciiTheme, run.Properties.Lang)
		}
		if format.FontHAnsi == "" && f.HAnsiTheme != "" {
			format.FontHAnsi = resolveFontSlotWithLang(ctx, f.HAnsiTheme, run.Properties.Lang)
		}
		if format.FontEastAsia == "" && f.EastAsiaTheme != "" {
			format.FontEastAsia = resolveFontSlotWithLang(ctx, f.EastAsiaTheme, run.Properties.Lang)
		}
		if format.FontCS == "" && f.CSTheme != "" {
			format.FontCS = resolveFontSlotWithLang(ctx, f.CSTheme, run.Properties.Lang)
		}
	}

	// Suppress hidden (vanished) runs.
	if format.Vanish != nil && *format.Vanish {
		return nil
	}
	if format.SpecVanish != nil && *format.SpecVanish {
		return nil
	}

	if len(run.Contents) == 0 {
		text := ""
		for _, node := range run.Text {
			if node == nil {
				continue
			}
			text += node.Value
		}
		if text == "" {
			return nil
		}
		return []Run{{
			Kind:             semanticRunKind(target),
			Text:             text,
			StyleID:          styleID,
			Target:           target,
			Anchor:           anchor,
			Format:           format,
			PropertyRevision: propRevision,
		}}
	}

	out := make([]Run, 0, len(run.Contents))
	for _, content := range run.Contents {
		switch content.Kind {
		case parts.RunContentText:
			if content.Text == nil || content.Text.Value == "" {
				continue
			}
			out = append(out, Run{
				Kind:             semanticRunKind(target),
				Text:             content.Text.Value,
				Target:           target,
				Anchor:           anchor,
				Format:           format,
				PropertyRevision: propRevision,
			})
		case parts.RunContentTab:
			out = append(out, Run{
				Kind:             "tab",
				Text:             "\t",
				Target:           target,
				Anchor:           anchor,
				Format:           format,
				PropertyRevision: propRevision,
			})
		case parts.RunContentBreak:
			segment := Run{
				Kind:             "break",
				Text:             "\n",
				Target:           target,
				Anchor:           anchor,
				Format:           format,
				PropertyRevision: propRevision,
			}
			if content.Break != nil {
				segment.BreakType = content.Break.Type
				segment.BreakClear = content.Break.Clear
			}
			out = append(out, segment)
		case parts.RunContentSoftHyphen:
			out = append(out, Run{
				Kind:             semanticRunKind(target),
				Text:             "\u00AD",
				Target:           target,
				Anchor:           anchor,
				Format:           format,
				PropertyRevision: propRevision,
			})
		case parts.RunContentNoBreakHyphen:
			out = append(out, Run{
				Kind:             semanticRunKind(target),
				Text:             "\u2011",
				Target:           target,
				Anchor:           anchor,
				Format:           format,
				PropertyRevision: propRevision,
			})
		case parts.RunContentCR:
			out = append(out, Run{
				Kind:             "break",
				Text:             "\n",
				Target:           target,
				Anchor:           anchor,
				Format:           format,
				PropertyRevision: propRevision,
			})
		case parts.RunContentDrawing:
			if img := resolveDrawing(ctx, content.Drawing); img != nil {
				out = append(out, Run{
					Kind:             "image",
					Image:            img,
					Format:           format,
					PropertyRevision: propRevision,
				})
			} else if chart := resolveChart(ctx, content.Drawing); chart != nil {
				out = append(out, Run{
					Kind:             "chart",
					Image:            chart,
					Format:           format,
					PropertyRevision: propRevision,
				})
			} else if smartArtImage := resolveSmartArt(ctx, content.Drawing); smartArtImage != nil {
				out = append(out, Run{
					Kind:             "image",
					Image:            smartArtImage,
					Format:           format,
					PropertyRevision: propRevision,
				})
			} else if tb := resolveTextBox(ctx, content.Drawing); tb != nil {
				out = append(out, Run{
					Kind:             "textbox",
					TextBox:          tb,
					Format:           format,
					PropertyRevision: propRevision,
				})
			} else if shape := resolveShape(ctx, content.Drawing); shape != nil {
				out = append(out, Run{
					Kind:             "shape",
					Shape:            shape,
					Format:           format,
					PropertyRevision: propRevision,
				})
			}
		case parts.RunContentFootnoteRef:
			if content.NoteRef != nil {
				out = append(out, Run{
					Kind:             "footnoteRef",
					NoteID:           content.NoteRef.ID,
					Format:           format,
					PropertyRevision: propRevision,
				})
			}
		case parts.RunContentEndnoteRef:
			if content.NoteRef != nil {
				out = append(out, Run{
					Kind:             "endnoteRef",
					NoteID:           content.NoteRef.ID,
					Format:           format,
					PropertyRevision: propRevision,
				})
			}
		case parts.RunContentCommentRef:
			if content.CommentRef != nil {
				out = append(out, Run{
					Kind:             "commentRef",
					CommentID:        content.CommentRef.ID,
					Format:           format,
					PropertyRevision: propRevision,
				})
			}
		case parts.RunContentAnnotationRef:
			out = append(out, Run{
				Kind:             "annotationRef",
				Format:           format,
				PropertyRevision: propRevision,
			})
		case parts.RunContentDayShort, parts.RunContentDayLong,
			parts.RunContentMonthShort, parts.RunContentMonthLong,
			parts.RunContentYearShort, parts.RunContentYearLong,
			parts.RunContentPgNum, parts.RunContentLastRenderedPageBreak,
			parts.RunContentContentPart:
			// Auto-text elements (date parts, page number), lastRenderedPageBreak,
			// and contentPart references have no cached text for static HTML.
			// Skip them in static HTML output.
		case parts.RunContentSym:
			if content.Sym != nil {
				if r := symToRune(content.Sym.Char); r != 0 {
					out = append(out, Run{
						Kind:             semanticRunKind(target),
						Text:             string(r),
						Target:           target,
						Anchor:           anchor,
						Format:           format,
						PropertyRevision: propRevision,
					})
				}
			}
		case parts.RunContentVMLImage:
			if content.VMLRelID != "" && ctx.pkg != nil {
				if media, ok := ctx.pkg.MediaPartFor(ctx.currentPart, content.VMLRelID); ok {
					img := &Image{
						MIMEType: media.ContentType,
						Data:     media.Data,
					}
					if content.VMLStyle != "" {
						img.Width, img.Height = parseVMLStyleExtent(content.VMLStyle)
					}
					out = append(out, Run{
						Kind:             "image",
						Image:            img,
						Format:           format,
						PropertyRevision: propRevision,
					})
				}
			}
		case parts.RunContentVMLTextBox:
			if tb := resolveVMLTextBox(ctx, content); tb != nil {
				out = append(out, Run{
					Kind:             "textbox",
					TextBox:          tb,
					Format:           format,
					PropertyRevision: propRevision,
				})
			}
		case parts.RunContentVMLHorzRule:
			out = append(out, Run{
				Kind:             "horzRule",
				Text:             content.VMLHorzRuleColor,
				Format:           format,
				PropertyRevision: propRevision,
			})
		case parts.RunContentVMLShape:
			if shape := resolveVMLShape(content); shape != nil {
				run := Run{
					Kind:             "shape",
					Shape:            shape,
					Format:           format,
					PropertyRevision: propRevision,
				}
				if content.VMLSignatureLine != nil {
					run.SignatureLine = resolveVMLSignatureLine(content.VMLSignatureLine)
				}
				out = append(out, run)
			}
		case parts.RunContentOLE:
			if content.OLE != nil {
				label := content.OLE.ProgID
				if label == "" {
					label = "Embedded Object"
				}
				run := Run{Kind: "olePlaceholder", Text: label, OLERelID: content.OLE.RelID, OLEProgID: content.OLE.ProgID, OLEType: content.OLE.Type, OLEDrawAspect: content.OLE.DrawAspect, OLEObjectID: content.OLE.ObjectID, OLEUpdateMode: content.OLE.UpdateMode, Format: format, PropertyRevision: propRevision}
				if ctx.pkg != nil && content.OLE.RelID != "" {
					if part, ok := ctx.pkg.OLEPartFor(ctx.currentPart, content.OLE.RelID); ok {
						run.OLEData = part.Data
						run.OLEMIME = part.ContentType
					}
					if target, ok := ctx.pkg.OLELinkFor(ctx.currentPart, content.OLE.RelID); ok {
						run.OLELinkURL = target
						if run.OLEType == "" {
							run.OLEType = "Link"
						}
					}
				}
				if ctx.pkg != nil && content.OLE.PreviewRelID != "" {
					if media, ok := ctx.pkg.MediaPartFor(ctx.currentPart, content.OLE.PreviewRelID); ok {
						run.OLEPreview = &Image{MIMEType: media.ContentType, Data: append([]byte(nil), media.Data...)}
					}
				}
				out = append(out, run)
			}
		}
	}

	for i := range out {
		out[i].StyleID = styleID
	}
	return out
}

func normalizeHyperlink(ctx *resolveCtx, hyperlink *parts.Hyperlink, paraRunFmt RunFormat) []Run {
	if hyperlink == nil {
		return nil
	}

	target := ""
	if hyperlink.ID != "" && ctx.pkg != nil {
		target = ctx.pkg.HyperlinkTarget(ctx.currentPart, hyperlink.ID)
	}

	out := make([]Run, 0, len(hyperlink.Runs))
	for _, run := range hyperlink.Runs {
		runs := normalizeRun(ctx, run, target, hyperlink.Anchor, paraRunFmt)
		for i := range runs {
			runs[i].RelID = hyperlink.ID
		}
		out = append(out, runs...)
	}
	return out
}

func normalizePropertyRevision(change *parts.PropertyChangeInfo) *RevisionMark {
	if change == nil {
		return nil
	}
	return &RevisionMark{
		Author: change.Author,
		Date:   change.Date,
	}
}

// resolveDrawing extracts a resolved Image from a w:drawing content node.
// Returns nil if the drawing cannot be resolved (e.g. missing relationship).
func resolveDrawing(ctx *resolveCtx, drawing *parts.Drawing) *Image {
	if drawing == nil {
		return nil
	}

	var extent *parts.DrawingExtent
	var docPr *parts.DrawingDocPr
	var graphic *parts.Graphic
	floating := false

	if drawing.Inline != nil {
		extent = drawing.Inline.Extent
		docPr = drawing.Inline.DocPr
		graphic = drawing.Inline.Graphic
	} else if drawing.Anchor != nil {
		extent = drawing.Anchor.Extent
		docPr = drawing.Anchor.DocPr
		graphic = drawing.Anchor.Graphic
		floating = true
	}
	if graphic == nil {
		return nil
	}

	embedID := blipEmbedID(graphic)
	if embedID == "" {
		return nil
	}

	if ctx.pkg == nil {
		return nil
	}
	media, ok := ctx.pkg.MediaPartFor(ctx.currentPart, embedID)
	if !ok {
		return nil
	}

	img := &Image{
		MIMEType: media.ContentType,
		Data:     media.Data,
		Floating: floating,
	}
	if extent != nil {
		img.Width = extent.CX
		img.Height = extent.CY
	}
	if pic := pictureFromGraphic(graphic); pic != nil {
		if pic.BlipFill != nil && pic.BlipFill.SrcRect != nil {
			img.CropLeft = clampCropPct(pic.BlipFill.SrcRect.L)
			img.CropTop = clampCropPct(pic.BlipFill.SrcRect.T)
			img.CropRight = clampCropPct(pic.BlipFill.SrcRect.R)
			img.CropBottom = clampCropPct(pic.BlipFill.SrcRect.B)
		}
		if pic.ShapeProps != nil && pic.ShapeProps.Xfrm != nil {
			img.RotationDeg = drawingRotationDegrees(pic.ShapeProps.Xfrm.Rot)
			img.FlipH = pic.ShapeProps.Xfrm.FlipH
			img.FlipV = pic.ShapeProps.Xfrm.FlipV
		}
		if pic.ShapeProps != nil && pic.ShapeProps.Ln != nil {
			line := drawingxml.ResolveLineStyle(pic.ShapeProps.Ln, ctx.themeColors)
			if line != nil && line.Color != "" {
				img.BorderColor = line.Color
				img.BorderWidth = line.Width
			}
		}
		if pic.ShapeProps != nil {
			var fill dml.Fill
			switch {
			case pic.ShapeProps.SolidFill != nil:
				fill = drawingxml.ResolveSolidFill(pic.ShapeProps.SolidFill, ctx.themeColors)
			case pic.ShapeProps.NoFill != nil:
				fill = dml.NoFill{}
			case pic.ShapeProps.GradFill != nil:
				fill = drawingxml.ResolveGradientFill(pic.ShapeProps.GradFill, ctx.themeColors)
			case pic.ShapeProps.PattFill != nil:
				fill = drawingxml.ResolvePatternFill(pic.ShapeProps.PattFill, ctx.themeColors)
			}
			if fill != nil {
				img.FillColor = resolveDrawingFill(ctx, fill)
			}
			img.Effects = pic.ShapeProps.DrawingEffectList(ctx.themeColors)
		}
		if pic.BlipFill != nil {
			img.Tile = pic.BlipFill.Tile != nil
			img.Stretch = pic.BlipFill.Stretch != nil || (pic.BlipFill.Tile == nil && pic.BlipFill.Stretch == nil)
			if pic.BlipFill.Tile != nil {
				img.TileOffsetX = int64(pic.BlipFill.Tile.Tx)
				img.TileOffsetY = int64(pic.BlipFill.Tile.Ty)
				img.TileScaleX = pic.BlipFill.Tile.Sx
				img.TileScaleY = pic.BlipFill.Tile.Sy
				img.TileFlip = pic.BlipFill.Tile.Flip
				img.TileAlign = pic.BlipFill.Tile.Algn
			}
			if pic.BlipFill.Blip != nil {
				blip := pic.BlipFill.Blip
				if blip.Lum != nil {
					// DrawingML a:lum stores signed percentage adjustments where 0 is
					// unchanged. Normalize to the renderer's CSS-style multipliers.
					img.Brightness = math.Max(0, 1+float64(blip.Lum.Bright)/100000.0)
					img.Contrast = math.Max(0, 1+float64(blip.Lum.Contrast)/100000.0)
				}
				if blip.AlphaMod != nil && blip.AlphaMod.Val > 0 {
					img.Alpha = float64(blip.AlphaMod.Val) / 100000.0
				}
				if blip.Grayscale != nil {
					img.Grayscale = true
				}
				if blip.BiLevel != nil {
					img.BiLevel = true
					img.BiLevelThreshold = blip.BiLevel.Thresh
				}
			}
		}
	}
	// Fallback for DrawingML picture shape properties (SpPr) when the WPS picture
	// path has no equivalent raw fields.
	if dmlPic := graphic.GraphicData.DrawingMLPic; dmlPic != nil && dmlPic.SpPr != nil {
		if img.RotationDeg == 0 && !img.FlipH && !img.FlipV {
			img.RotationDeg = drawingRotationDegrees(dmlPic.SpPr.Xfrm.Rot)
			img.FlipH = dmlPic.SpPr.Xfrm.FlipH
			img.FlipV = dmlPic.SpPr.Xfrm.FlipV
		}
		if img.BorderColor == "" && img.BorderWidth == 0 {
			if dmlPic.SpPr.Line != nil && dmlPic.SpPr.Line.Color != "" {
				img.BorderColor = dmlPic.SpPr.Line.Color
				img.BorderWidth = dmlPic.SpPr.Line.Width
			}
		}
		if img.FillColor == "" {
			img.FillColor = resolveDrawingFill(ctx, dmlPic.SpPr.Fill)
		}
		if len(img.Effects.Effects) == 0 {
			img.Effects = dmlPic.SpPr.DrawingEffectList(ctx.themeColors)
		}
	}
	if floating && drawing.Anchor != nil {
		img.X, img.Y = drawingAnchorOffset(drawing.Anchor)
		if drawing.Anchor.PositionH != nil {
			img.XRelative = drawing.Anchor.PositionH.RelativeFrom
			img.XAlign = drawing.Anchor.PositionH.Align
			if drawing.Anchor.PositionH.Pct != 0 {
				img.X = drawing.Anchor.PositionH.Pct
			}
		}
		if drawing.Anchor.PositionV != nil {
			img.YRelative = drawing.Anchor.PositionV.RelativeFrom
			img.YAlign = drawing.Anchor.PositionV.Align
			if drawing.Anchor.PositionV.Pct != 0 {
				img.Y = drawing.Anchor.PositionV.Pct
			}
		}
		if drawing.Anchor.SimplePosAttr && drawing.Anchor.SimplePos != nil {
			img.SimplePosAttr = true
			img.SimplePosX = drawing.Anchor.SimplePos.X
			img.SimplePosY = drawing.Anchor.SimplePos.Y
		}
		if drawing.Anchor.EffectExtent != nil {
			img.EffectExtentL = drawing.Anchor.EffectExtent.L
			img.EffectExtentT = drawing.Anchor.EffectExtent.T
			img.EffectExtentR = drawing.Anchor.EffectExtent.R
			img.EffectExtentB = drawing.Anchor.EffectExtent.B
		}
		img.Locked = drawing.Anchor.Locked
		img.LayoutInCell = drawing.Anchor.LayoutInCell
		img.AllowOverlap = drawing.Anchor.AllowOverlap
		if drawing.Anchor.SizeRelH != nil {
			img.SizeRelHPct = drawing.Anchor.SizeRelH.Pct
			img.SizeRelHFrom = drawing.Anchor.SizeRelH.RelativeFrom
		}
		if drawing.Anchor.SizeRelV != nil {
			img.SizeRelVPct = drawing.Anchor.SizeRelV.Pct
			img.SizeRelVFrom = drawing.Anchor.SizeRelV.RelativeFrom
		}
		img.ZIndex = drawing.Anchor.RelativeHeight
		img.BehindDoc = drawing.Anchor.BehindDoc != 0
		img.WrapType, img.WrapSide = drawingAnchorWrap(drawing.Anchor)
		img.WrapPolygon = drawingAnchorPolygon(drawing.Anchor)
		dt, db, dl, dr := drawingAnchorWrapDistances(drawing.Anchor)
		img.DistTop = emuToPoints(dt)
		img.DistBottom = emuToPoints(db)
		img.DistLeft = emuToPoints(dl)
		img.DistRight = emuToPoints(dr)
	}
	if docPr != nil {
		img.Alt = docPr.Desc
		if img.Alt == "" {
			img.Alt = docPr.Title
		}
		if img.Alt == "" {
			img.Alt = docPr.Name
		}
		img.Title = docPr.Title
		img.Hidden = docPr.Hidden
	}
	return img
}

// blipEmbedID walks graphic → graphicData → pic → blipFill → blip to get r:embed.
func blipEmbedID(graphic *parts.Graphic) string {
	pic := pictureFromGraphic(graphic)
	if pic == nil || pic.BlipFill == nil || pic.BlipFill.Blip == nil {
		return ""
	}
	return pic.BlipFill.Blip.Embed
}

func pictureFromGraphic(graphic *parts.Graphic) *parts.Picture {
	if graphic == nil || graphic.GraphicData == nil {
		return nil
	}
	// Try WPS picture first
	if graphic.GraphicData.Picture != nil {
		return graphic.GraphicData.Picture
	}
	// Fall back to DrawingML picture
	if graphic.GraphicData.DrawingMLPic != nil {
		return convertDrawingMLPicture(graphic.GraphicData.DrawingMLPic)
	}
	return nil
}

// convertDrawingMLPicture converts a DrawingML picture to WPS picture format for compatibility.
func convertDrawingMLPicture(dmlPic *parts.DrawingMLPicture) *parts.Picture {
	if dmlPic == nil {
		return nil
	}
	pic := &parts.Picture{}
	if dmlPic.NvPicPr != nil {
		pic.NvPicPr = &parts.PictureNvProps{
			CNvPr:    dmlPic.NvPicPr.CNvPr,
			CNvPicPr: dmlPic.NvPicPr.CNvPicPr,
		}
	}
	if dmlPic.BlipFill != nil {
		pic.BlipFill = &parts.BlipFill{}
		if dmlPic.BlipFill.Blip != nil {
			pic.BlipFill.Blip = &parts.Blip{
				Embed: dmlPic.BlipFill.Blip.Embed,
			}
		}
		if dmlPic.BlipFill.SrcRect != nil {
			pic.BlipFill.SrcRect = &parts.SrcRect{
				L: dmlPic.BlipFill.SrcRect.L,
				T: dmlPic.BlipFill.SrcRect.T,
				R: dmlPic.BlipFill.SrcRect.R,
				B: dmlPic.BlipFill.SrcRect.B,
			}
		}
		pic.BlipFill.Tile = dmlPic.BlipFill.Tile
		pic.BlipFill.Stretch = dmlPic.BlipFill.Stretch
	}
	return pic
}

func clampCropPct(value int) float64 {
	if value <= 0 {
		return 0
	}
	if value >= 100000 {
		return 1
	}
	return float64(value) / 100000.0
}

func drawingRotationDegrees(raw int) float64 {
	if raw == 0 {
		return 0
	}
	return float64(raw) / 60000.0
}

// resolveBorderSide converts a parts.Border to a resolved BorderSide.
// Returns nil for "nil" and "none" values or zero-size borders.
func resolveBorderSide(b *parts.Border) *BorderSide {
	if b == nil || b.Val == "" {
		return nil
	}
	side := &BorderSide{Val: b.Val}
	if b.Val == "nil" || b.Val == "none" {
		side.SizePt = 0
	} else if b.Size > 0 {
		side.SizePt = float64(b.Size) / 8.0 // eighths-of-a-point → points
	} else {
		side.SizePt = 0.75 // Word default hairline
	}
	if b.Space > 0 {
		side.SpacePt = float64(b.Space)
	}
	if b.Color != "" && b.Color != "auto" {
		side.Color = b.Color
	}
	return side
}

// borderSideIsEmpty returns true if the side is nil or explicitly none/nil.
func borderSideIsEmpty(side *BorderSide) bool {
	return side == nil || side.Val == "none" || side.Val == "nil"
}

// resolveParagraphBorders converts parts.ParagraphBorders to the resolve IR type.
// tableConditionalTypes returns the list of tblStylePr type names applicable to a
// cell at (rowIdx, colIdx) in a (totalRows x totalCols) table, in priority order
// (lowest first: wholeTable → band → first/lastCol → first/lastRow).
func tableConditionalTypes(rowIdx, totalRows, colIdx, totalCols int, look *parts.TableLook) []string {
	var types []string
	types = append(types, "wholeTable")

	flags := effectiveTableLookFlags(look)
	noHBand := flags.NoHBand
	noVBand := flags.NoVBand
	if !noHBand {
		if rowIdx%2 == 0 {
			types = append(types, "band1H")
		} else {
			types = append(types, "band2H")
		}
	}
	if !noVBand {
		if colIdx%2 == 0 {
			types = append(types, "band1V")
		} else {
			types = append(types, "band2V")
		}
	}
	if flags.FirstCol && colIdx == 0 {
		types = append(types, "firstCol")
	}
	if flags.LastCol && totalCols > 0 && colIdx == totalCols-1 {
		types = append(types, "lastCol")
	}
	if flags.FirstRow && rowIdx == 0 {
		types = append(types, "firstRow")
	}
	if flags.LastRow && totalRows > 0 && rowIdx == totalRows-1 {
		types = append(types, "lastRow")
	}
	// Corner cells (highest priority among positional conditions)
	isFirstRow := flags.FirstRow && rowIdx == 0
	isLastRow := flags.LastRow && totalRows > 0 && rowIdx == totalRows-1
	isFirstCol := flags.FirstCol && colIdx == 0
	isLastCol := flags.LastCol && totalCols > 0 && colIdx == totalCols-1
	if isFirstRow && isFirstCol {
		types = append(types, "nwCell")
	}
	if isFirstRow && isLastCol {
		types = append(types, "neCell")
	}
	if isLastRow && isFirstCol {
		types = append(types, "swCell")
	}
	if isLastRow && isLastCol {
		types = append(types, "seCell")
	}
	return types
}

type tableLookFlags struct {
	FirstRow bool
	LastRow  bool
	FirstCol bool
	LastCol  bool
	NoHBand  bool
	NoVBand  bool
}

func effectiveTableLookFlags(look *parts.TableLook) tableLookFlags {
	flags := tableLookFlags{
		FirstRow: true,
		LastRow:  false,
		FirstCol: false,
		LastCol:  false,
		NoHBand:  false,
		NoVBand:  true,
	}
	if look == nil {
		return flags
	}
	if raw := strings.TrimSpace(look.Val); raw != "" {
		if bits, err := strconv.ParseUint(raw, 16, 16); err == nil {
			flags.FirstRow = bits&0x0020 != 0
			flags.LastRow = bits&0x0040 != 0
			flags.FirstCol = bits&0x0080 != 0
			flags.LastCol = bits&0x0100 != 0
			flags.NoHBand = bits&0x0200 != 0
			flags.NoVBand = bits&0x0400 != 0
		}
	}
	if look.FirstRow != "" {
		flags.FirstRow = look.FirstRow != "0"
	}
	if look.LastRow != "" {
		flags.LastRow = look.LastRow != "0"
	}
	if look.FirstCol != "" {
		flags.FirstCol = look.FirstCol != "0"
	}
	if look.LastCol != "" {
		flags.LastCol = look.LastCol != "0"
	}
	if look.NoHBand != "" {
		flags.NoHBand = look.NoHBand == "1"
	}
	if look.NoVBand != "" {
		flags.NoVBand = look.NoVBand == "1"
	}
	return flags
}

// resolveConditionalTableStyles converts a slice of raw OOXML tblStylePr entries
// into pre-resolved IR-level ConditionalTableStyle values. Nil entries are skipped.
func resolveConditionalTableStyles(raw []*parts.TableStyleProps, themeColors map[string]string) []ConditionalTableStyle {
	if len(raw) == 0 {
		return nil
	}
	out := make([]ConditionalTableStyle, 0, len(raw))
	for _, tsp := range raw {
		if tsp == nil {
			continue
		}
		cts := ConditionalTableStyle{Type: tsp.Type}
		if tsp.CellProps != nil {
			cts.CellFormat = tableCellFormatFromProperties(tsp.CellProps, themeColors)
		}
		if tsp.RunProps != nil {
			cts.RunFormat = runFormatFromProperties(tsp.RunProps)
		}
		if tsp.ParaProps != nil {
			cts.ParaFormat = paragraphFormatFromProperties(tsp.ParaProps, themeColors)
		}
		out = append(out, cts)
	}
	return out
}

// buildConditionalOverlay merges ConditionalTableStyle entries matching the given
// type names (in the order supplied — lowest priority first) into formatting overlays.
func buildConditionalOverlay(tblStyles []ConditionalTableStyle, types []string) (TableCellFormat, RunFormat, ParagraphFormat) {
	var cell TableCellFormat
	var run RunFormat
	var para ParagraphFormat
	for _, typeName := range types {
		for _, ts := range tblStyles {
			if ts.Type != typeName {
				continue
			}
			cell = mergeTableCellFormat(cell, ts.CellFormat)
			run = mergeRunFormat(run, ts.RunFormat)
			para = mergeParagraphFormat(para, ts.ParaFormat)
		}
	}
	return cell, run, para
}

func tableCellFormatFromProperties(p *parts.TableCellProperties, themeColors map[string]string) TableCellFormat {
	var out TableCellFormat
	if p == nil {
		return out
	}
	if p.Width != nil && p.Width.Type != "nil" {
		v := p.Width.Width
		out.Width = &v
		out.WidthType = p.Width.Type
	}
	if p.GridSpan != nil {
		out.GridSpan = p.GridSpan.Val
	}
	if p.VMerge != nil {
		if p.VMerge.Val == "restart" {
			out.VMerge = "restart"
		} else {
			out.VMerge = "continue"
		}
	}
	if p.VertAlign != nil {
		out.VertAlign = p.VertAlign.Val
	}
	if p.TextDirection != nil {
		out.TextDirection = p.TextDirection.Val
	}
	if p.Shading != nil {
		if fill := resolveShadingFill(p.Shading, themeColors); fill != "" {
			out.Shading = fill
		}
	}
	if p.Borders != nil {
		out.Borders = resolveCellBorders(p.Borders)
	}
	if p.HMerge != nil {
		out.HMerge = true
	}
	if p.NoWrap != nil && onOffBool(p.NoWrap) {
		out.NoWrap = true
	}
	if p.HideMark != nil && onOffBool(p.HideMark) {
		out.HideMark = true
	}
	if m := p.Margins; m != nil {
		if m.Top != nil && m.Top.Type == "dxa" {
			out.CellPadTop = m.Top.Width
		}
		if m.Bottom != nil && m.Bottom.Type == "dxa" {
			out.CellPadBottom = m.Bottom.Width
		}
		if m.Left != nil && m.Left.Type == "dxa" {
			out.CellPadLeft = m.Left.Width
		}
		if m.Right != nil && m.Right.Type == "dxa" {
			out.CellPadRight = m.Right.Width
		}
	}
	return out
}

func mergeTableFormat(base, overlay TableFormat) TableFormat {
	out := base
	if overlay.Width != nil {
		v := *overlay.Width
		out.Width = &v
	}
	if overlay.WidthType != "" {
		out.WidthType = overlay.WidthType
	}
	if overlay.Layout != "" {
		out.Layout = overlay.Layout
	}
	if overlay.Alignment != "" {
		out.Alignment = overlay.Alignment
	}
	if overlay.Shading != "" {
		out.Shading = overlay.Shading
	}
	if overlay.Borders != nil {
		out.Borders = overlay.Borders
	}
	if len(overlay.ColumnWidths) > 0 {
		out.ColumnWidths = append([]int(nil), overlay.ColumnWidths...)
	}
	if overlay.Indent != 0 {
		out.Indent = overlay.Indent
	}
	if overlay.CellSpacing != 0 {
		out.CellSpacing = overlay.CellSpacing
	}
	if overlay.CellPadTop != 0 {
		out.CellPadTop = overlay.CellPadTop
	}
	if overlay.CellPadBottom != 0 {
		out.CellPadBottom = overlay.CellPadBottom
	}
	if overlay.CellPadLeft != 0 {
		out.CellPadLeft = overlay.CellPadLeft
	}
	if overlay.CellPadRight != 0 {
		out.CellPadRight = overlay.CellPadRight
	}
	if overlay.FloatProps != nil {
		out.FloatProps = overlay.FloatProps
	}
	if overlay.Overlap != "" {
		out.Overlap = overlay.Overlap
	}
	if overlay.Caption != "" {
		out.Caption = overlay.Caption
	}
	if overlay.Description != "" {
		out.Description = overlay.Description
	}
	if overlay.BidiVisual {
		out.BidiVisual = true
	}
	if overlay.Look != nil {
		out.Look = overlay.Look
	}
	if overlay.StyleRowBandSize != 0 {
		out.StyleRowBandSize = overlay.StyleRowBandSize
	}
	if overlay.StyleColBandSize != 0 {
		out.StyleColBandSize = overlay.StyleColBandSize
	}
	return out
}

func mergeTableCellFormat(base, overlay TableCellFormat) TableCellFormat {
	out := base
	if overlay.Width != nil {
		v := *overlay.Width
		out.Width = &v
	}
	if overlay.WidthType != "" {
		out.WidthType = overlay.WidthType
	}
	if overlay.GridSpan > 0 {
		out.GridSpan = overlay.GridSpan
	}
	if overlay.VMerge != "" {
		out.VMerge = overlay.VMerge
	}
	if overlay.VertAlign != "" {
		out.VertAlign = overlay.VertAlign
	}
	if overlay.TextDirection != "" {
		out.TextDirection = overlay.TextDirection
	}
	if overlay.Shading != "" {
		out.Shading = overlay.Shading
	}
	if overlay.Borders != nil {
		out.Borders = overlay.Borders
	}
	if overlay.NoWrap {
		out.NoWrap = true
	}
	if overlay.HideMark {
		out.HideMark = true
	}
	if overlay.CellPadTop != 0 {
		out.CellPadTop = overlay.CellPadTop
	}
	if overlay.CellPadBottom != 0 {
		out.CellPadBottom = overlay.CellPadBottom
	}
	if overlay.CellPadLeft != 0 {
		out.CellPadLeft = overlay.CellPadLeft
	}
	if overlay.CellPadRight != 0 {
		out.CellPadRight = overlay.CellPadRight
	}
	return out
}

func resolveFrameProps(f *parts.FramePr) *FrameProps {
	if f == nil {
		return nil
	}
	fp := &FrameProps{}
	if f.W != nil {
		fp.W = *f.W
	}
	if f.H != nil {
		fp.H = *f.H
	}
	if f.X != nil {
		fp.X = *f.X
	}
	if f.Y != nil {
		fp.Y = *f.Y
	}
	fp.XAlign = f.XAlign
	fp.YAlign = f.YAlign
	fp.HAnchor = f.HAnchor
	fp.VAnchor = f.VAnchor
	fp.WrapType = f.Wrap
	if f.HSpace != nil {
		fp.HSpace = *f.HSpace
	}
	if f.VSpace != nil {
		fp.VSpace = *f.VSpace
	}
	fp.DropCap = f.DropCap
	if f.Lines != nil {
		fp.Lines = *f.Lines
	}
	return fp
}

func resolveParagraphBorders(b *parts.ParagraphBorders) *ParagraphBorders {
	if b == nil {
		return nil
	}
	out := &ParagraphBorders{
		Top:     resolveBorderSide(b.Top),
		Left:    resolveBorderSide(b.Left),
		Start:   resolveBorderSide(b.Start),
		Bottom:  resolveBorderSide(b.Bottom),
		Right:   resolveBorderSide(b.Right),
		End:     resolveBorderSide(b.End),
		Between: resolveBorderSide(b.Between),
		Bar:     resolveBorderSide(b.Bar),
	}
	if borderSideIsEmpty(out.Top) && borderSideIsEmpty(out.Left) && borderSideIsEmpty(out.Start) && borderSideIsEmpty(out.Bottom) && borderSideIsEmpty(out.Right) && borderSideIsEmpty(out.End) && borderSideIsEmpty(out.Between) && borderSideIsEmpty(out.Bar) {
		return nil
	}
	return out
}

// resolveTableBorders converts parts.TableBorders to the resolve IR type.
func resolveTableBorders(b *parts.TableBorders) *TableBorders {
	if b == nil {
		return nil
	}
	out := &TableBorders{
		Top:     resolveBorderSide(b.Top),
		Left:    resolveBorderSide(b.Left),
		Start:   resolveBorderSide(b.Start),
		Bottom:  resolveBorderSide(b.Bottom),
		Right:   resolveBorderSide(b.Right),
		End:     resolveBorderSide(b.End),
		InsideH: resolveBorderSide(b.InsideH),
		InsideV: resolveBorderSide(b.InsideV),
	}
	// Return nil if all sides are empty (no effective borders).
	if borderSideIsEmpty(out.Top) && borderSideIsEmpty(out.Left) && borderSideIsEmpty(out.Start) && borderSideIsEmpty(out.Bottom) && borderSideIsEmpty(out.Right) && borderSideIsEmpty(out.End) && borderSideIsEmpty(out.InsideH) && borderSideIsEmpty(out.InsideV) {
		return nil
	}
	return out
}

// resolveCellBorders converts parts.TableCellBorders to the resolve IR type.
func resolveCellBorders(b *parts.TableCellBorders) *CellBorders {
	if b == nil {
		return nil
	}
	out := &CellBorders{
		Top:    resolveBorderSide(b.Top),
		Left:   resolveBorderSide(b.Left),
		Start:  resolveBorderSide(b.Start),
		Bottom: resolveBorderSide(b.Bottom),
		Right:  resolveBorderSide(b.Right),
		End:    resolveBorderSide(b.End),
		TL2BR:  resolveBorderSide(b.TL2BR),
		TR2BL:  resolveBorderSide(b.TR2BL),
	}
	if borderSideIsEmpty(out.Top) && borderSideIsEmpty(out.Left) && borderSideIsEmpty(out.Start) && borderSideIsEmpty(out.Bottom) && borderSideIsEmpty(out.Right) && borderSideIsEmpty(out.End) && borderSideIsEmpty(out.TL2BR) && borderSideIsEmpty(out.TR2BL) {
		return nil
	}
	return out
}

// resolveDrawingMLTextBody converts a DrawingML text body into IR blocks,
// resolving theme fonts, colors, sizes, and formatting.
func resolveDrawingMLTextBody(ctx *resolveCtx, txBody *parts.DrawingTextBody) []Block {
	if txBody == nil {
		return nil
	}
	// Build a map of list-style level defaults.
	levelDefaults := make(map[int]*parts.DrawingRunProps)
	if txBody.LstStyle != nil {
		for _, lvl := range txBody.LstStyle.Levels {
			if lvl.DefPPr != nil && lvl.DefPPr.DefRPr != nil {
				levelDefaults[lvl.Lvl] = lvl.DefPPr.DefRPr
			}
		}
	}
	var blocks []Block
	for _, para := range txBody.Paragraphs {
		p := &Paragraph{}
		// Start with level defaults, then paragraph defRPr, then run rPr.
		levelDef := levelDefaults[0]
		var paraDef *parts.DrawingRunProps
		if para.PPr != nil {
			if para.PPr.Lvl != 0 {
				levelDef = levelDefaults[para.PPr.Lvl]
			}
			paraDef = para.PPr.DefRPr
		}
		for _, run := range para.Runs {
			r := Run{Text: run.T}
			r.Format = resolveDrawingMLRunFormat(ctx, levelDef, paraDef, run.RPr)
			p.Runs = append(p.Runs, r)
		}
		blocks = append(blocks, Block{Kind: "paragraph", Paragraph: p})
	}
	return blocks
}

// resolveDrawingMLRunFormat merges level, paragraph, and run DrawingML properties
// into a Word IR RunFormat.
func resolveDrawingMLRunFormat(ctx *resolveCtx, levelDef, paraDef, runDef *parts.DrawingRunProps) RunFormat {
	var format RunFormat
	// Merge in order: level → paragraph → run (later overrides earlier).
	for _, src := range []*parts.DrawingRunProps{levelDef, paraDef, runDef} {
		if src == nil {
			continue
		}
		if src.Lang != "" {
			format.Lang = src.Lang
		}
		if src.Latin != nil && src.Latin.Typeface != "" {
			format.FontASCII = resolveDrawingMLFont(ctx.themeFonts, src.Latin.Typeface, format.Lang)
			format.FontHAnsi = format.FontASCII
		}
		if src.EastAsian != nil && src.EastAsian.Typeface != "" {
			format.FontEastAsia = resolveDrawingMLFont(ctx.themeFonts, src.EastAsian.Typeface, format.Lang)
		}
		if src.ComplexScr != nil && src.ComplexScr.Typeface != "" {
			format.FontCS = resolveDrawingMLFont(ctx.themeFonts, src.ComplexScr.Typeface, format.Lang)
		}
		if src.Size != 0 {
			sz := src.Size / 50 // hundredths-of-pt → half-points
			format.FontSize = &sz
		}
		if src.Bold {
			format.Bold = boolPtr(true)
		}
		if src.Italic {
			format.Italic = boolPtr(true)
		}
		if src.Underline != "" {
			format.Underline = src.Underline
		}
		if src.Strike != "" {
			format.Strike = boolPtr(true)
		}
		if src.SolidFill != nil {
			if c := resolveDrawingMLSolidFill(ctx, src.SolidFill); c != "" {
				format.Color = c
			}
		}
		if src.Baseline > 0 {
			format.VertAlignVal = "superscript"
		} else if src.Baseline < 0 {
			format.VertAlignVal = "subscript"
		}
	}
	return format
}

// resolveDrawingMLFont resolves a DrawingML typeface name, expanding theme
// references like +mj-lt (major Latin) or +mn-ea (minor EastAsian).
func resolveDrawingMLFont(themeFonts map[string]string, typeface, lang string) string {
	runLang := &parts.RunLang{Val: lang, EastAsia: lang, Bidi: lang}
	switch typeface {
	case "+mj-lt":
		return themeFonts["major"]
	case "+mj-ea":
		return resolveThemeFontSlotWithLang(themeFonts, "majorEastAsia", runLang)
	case "+mj-cs":
		return resolveThemeFontSlotWithLang(themeFonts, "majorBidi", runLang)
	case "+mn-lt":
		return themeFonts["minor"]
	case "+mn-ea":
		return resolveThemeFontSlotWithLang(themeFonts, "minorEastAsia", runLang)
	case "+mn-cs":
		return resolveThemeFontSlotWithLang(themeFonts, "minorBidi", runLang)
	}
	return typeface
}

// resolveDrawingMLSolidFill resolves a DrawingML solidFill to a hex RGB color.
func resolveDrawingMLSolidFill(ctx *resolveCtx, fill *drawingxml.SolidFill) string {
	if fill == nil {
		return ""
	}
	if fill.SRGBClr != nil && fill.SRGBClr.Val != "" {
		base := fill.SRGBClr.Val
		return applyDrawingXMLSRGBTransforms(base, fill.SRGBClr)
	}
	if fill.SchemeClr != nil && fill.SchemeClr.Val != "" {
		if base, ok := ctx.themeColors[fill.SchemeClr.Val]; ok {
			return applyDrawingXMLSchemeTransforms(base, fill.SchemeClr)
		}
	}
	return ""
}

// applyDrawingXMLSRGBTransforms converts a drawingxml.SRGBClr's child transforms
// into a hex RGB string using the DrawingML HSL pipeline.
func applyDrawingXMLSRGBTransforms(base string, clr *drawingxml.SRGBClr) string {
	if clr == nil {
		return base
	}
	fill := &dml.SolidFill{}
	if clr.Alpha != nil {
		fill.Alpha = float64(clr.Alpha.Val) / 100000.0
	}
	if clr.AlphaMod != nil {
		fill.AlphaMod = float64(clr.AlphaMod.Val) / 100000.0
	}
	if clr.AlphaOff != nil {
		fill.AlphaOff = float64(clr.AlphaOff.Val) / 100000.0
	}
	if clr.Shade != nil {
		fill.Shade = float64(clr.Shade.Val) / 100000.0
		fill.HasShade = true
	}
	if clr.Tint != nil {
		fill.Tint = float64(clr.Tint.Val) / 100000.0
		fill.HasTint = true
	}
	if clr.Red != nil {
		fill.Red = float64(clr.Red.Val) / 100000.0
		fill.HasRed = true
	}
	if clr.RedMod != nil {
		fill.RedMod = float64(clr.RedMod.Val) / 100000.0
		fill.HasRedMod = true
	}
	if clr.RedOff != nil {
		fill.RedOff = float64(clr.RedOff.Val) / 100000.0
		fill.HasRedOff = true
	}
	if clr.Green != nil {
		fill.Green = float64(clr.Green.Val) / 100000.0
		fill.HasGreen = true
	}
	if clr.GreenMod != nil {
		fill.GreenMod = float64(clr.GreenMod.Val) / 100000.0
		fill.HasGreenMod = true
	}
	if clr.GreenOff != nil {
		fill.GreenOff = float64(clr.GreenOff.Val) / 100000.0
		fill.HasGreenOff = true
	}
	if clr.Blue != nil {
		fill.Blue = float64(clr.Blue.Val) / 100000.0
		fill.HasBlue = true
	}
	if clr.BlueMod != nil {
		fill.BlueMod = float64(clr.BlueMod.Val) / 100000.0
		fill.HasBlueMod = true
	}
	if clr.BlueOff != nil {
		fill.BlueOff = float64(clr.BlueOff.Val) / 100000.0
		fill.HasBlueOff = true
	}
	if clr.Hue != nil {
		fill.Hue = float64(clr.Hue.Val) / 60000.0
		fill.HasHue = true
	}
	if clr.Sat != nil {
		fill.Sat = float64(clr.Sat.Val) / 100000.0
		fill.HasSat = true
	}
	if clr.Lum != nil {
		fill.Lum = float64(clr.Lum.Val) / 100000.0
		fill.HasLum = true
	}
	if clr.LumMod != nil {
		fill.LumMod = float64(clr.LumMod.Val) / 100000.0
	}
	if clr.LumOff != nil {
		fill.LumOff = float64(clr.LumOff.Val) / 100000.0
	}
	if clr.SatMod != nil {
		fill.SatMod = float64(clr.SatMod.Val) / 100000.0
	}
	if clr.SatOff != nil {
		fill.SatOff = float64(clr.SatOff.Val) / 100000.0
	}
	if clr.HueMod != nil {
		fill.HueMod = float64(clr.HueMod.Val) / 60000.0
	}
	if clr.HueOff != nil {
		fill.HueOff = float64(clr.HueOff.Val) / 60000.0
	}
	fill.Gray = clr.Gray != nil
	fill.Comp = clr.Comp != nil
	fill.Inv = clr.Inv != nil
	fill.Gamma = clr.Gamma != nil
	fill.InvGamma = clr.InvGamma != nil
	return fill.ApplyColorTransforms(base)
}

// applyDrawingXMLSchemeTransforms converts a drawingxml.SchemeClr's child transforms
// into a hex RGB string using the DrawingML HSL pipeline.
func applyDrawingXMLSchemeTransforms(base string, clr *drawingxml.SchemeClr) string {
	if clr == nil {
		return base
	}
	fill := &dml.SolidFill{}
	if clr.Alpha != nil {
		fill.Alpha = float64(clr.Alpha.Val) / 100000.0
	}
	if clr.AlphaMod != nil {
		fill.AlphaMod = float64(clr.AlphaMod.Val) / 100000.0
	}
	if clr.AlphaOff != nil {
		fill.AlphaOff = float64(clr.AlphaOff.Val) / 100000.0
	}
	if clr.Shade != nil {
		fill.Shade = float64(clr.Shade.Val) / 100000.0
		fill.HasShade = true
	}
	if clr.Tint != nil {
		fill.Tint = float64(clr.Tint.Val) / 100000.0
		fill.HasTint = true
	}
	if clr.Red != nil {
		fill.Red = float64(clr.Red.Val) / 100000.0
		fill.HasRed = true
	}
	if clr.RedMod != nil {
		fill.RedMod = float64(clr.RedMod.Val) / 100000.0
		fill.HasRedMod = true
	}
	if clr.RedOff != nil {
		fill.RedOff = float64(clr.RedOff.Val) / 100000.0
		fill.HasRedOff = true
	}
	if clr.Green != nil {
		fill.Green = float64(clr.Green.Val) / 100000.0
		fill.HasGreen = true
	}
	if clr.GreenMod != nil {
		fill.GreenMod = float64(clr.GreenMod.Val) / 100000.0
		fill.HasGreenMod = true
	}
	if clr.GreenOff != nil {
		fill.GreenOff = float64(clr.GreenOff.Val) / 100000.0
		fill.HasGreenOff = true
	}
	if clr.Blue != nil {
		fill.Blue = float64(clr.Blue.Val) / 100000.0
		fill.HasBlue = true
	}
	if clr.BlueMod != nil {
		fill.BlueMod = float64(clr.BlueMod.Val) / 100000.0
		fill.HasBlueMod = true
	}
	if clr.BlueOff != nil {
		fill.BlueOff = float64(clr.BlueOff.Val) / 100000.0
		fill.HasBlueOff = true
	}
	if clr.Hue != nil {
		fill.Hue = float64(clr.Hue.Val) / 60000.0
		fill.HasHue = true
	}
	if clr.Sat != nil {
		fill.Sat = float64(clr.Sat.Val) / 100000.0
		fill.HasSat = true
	}
	if clr.Lum != nil {
		fill.Lum = float64(clr.Lum.Val) / 100000.0
		fill.HasLum = true
	}
	if clr.LumMod != nil {
		fill.LumMod = float64(clr.LumMod.Val) / 100000.0
	}
	if clr.LumOff != nil {
		fill.LumOff = float64(clr.LumOff.Val) / 100000.0
	}
	if clr.SatMod != nil {
		fill.SatMod = float64(clr.SatMod.Val) / 100000.0
	}
	if clr.SatOff != nil {
		fill.SatOff = float64(clr.SatOff.Val) / 100000.0
	}
	if clr.HueMod != nil {
		fill.HueMod = float64(clr.HueMod.Val) / 60000.0
	}
	if clr.HueOff != nil {
		fill.HueOff = float64(clr.HueOff.Val) / 60000.0
	}
	fill.Gray = clr.Gray != nil
	fill.Comp = clr.Comp != nil
	fill.Inv = clr.Inv != nil
	fill.Gamma = clr.Gamma != nil
	fill.InvGamma = clr.InvGamma != nil
	return fill.ApplyColorTransforms(base)
}

// resolveDrawingBodyPrInsets resolves DrawingML body property insets to points.
func resolveDrawingBodyPrInsets(bodyPr *parts.DrawingTextBodyProperties) (float64, float64, float64, float64) {
	if bodyPr == nil {
		return 0, 0, 0, 0
	}
	left := emuToPoints(bodyPr.LIns)
	top := emuToPoints(bodyPr.TIns)
	right := emuToPoints(bodyPr.RIns)
	bottom := emuToPoints(bodyPr.BIns)
	return left, top, right, bottom
}

// buildTextBoxFromDrawingMLShape builds a TextBox from a DrawingML shape's txBody.
func buildTextBoxFromDrawingMLShape(ctx *resolveCtx, dmlShape *parts.DrawingMLShape) *TextBox {
	if dmlShape == nil || dmlShape.TxBody == nil {
		return nil
	}
	blocks := resolveDrawingMLTextBody(ctx, dmlShape.TxBody)

	fillColor := ""
	var borderColor string
	var borderWidth float64
	if sp := dmlShape.SpPr; sp != nil {
		fillColor = resolveDrawingFill(ctx, sp.Fill)
		if sp.Line != nil && sp.Line.Color != "" {
			borderColor = sp.Line.Color
			borderWidth = sp.Line.Width
		}
	}
	if len(blocks) == 0 && fillColor == "" {
		return nil
	}

	styleFill, styleLine, styleEffects := resolveShapeStyle(ctx, dmlShape.Style)
	applyThemeFontFamilyToBlocks(blocks, resolveShapeStyleFontFamily(ctx.themeFonts, dmlShape.Style))
	if fillColor == "" && styleFill != nil {
		fillColor = resolveDrawingFill(ctx, styleFill)
	}
	if borderColor == "" && borderWidth == 0 && styleLine != nil {
		borderColor = styleLine.Color
		borderWidth = styleLine.Width
	}

	tb := &TextBox{Blocks: blocks, FillColor: fillColor, BorderColor: borderColor, BorderWidth: borderWidth}
	tb.PaddingLeft, tb.PaddingTop, tb.PaddingRight, tb.PaddingBottom = resolveDrawingBodyPrInsets(dmlShape.TxBody.BodyPr)
	if bodyPr := dmlShape.TxBody.BodyPr; bodyPr != nil {
		tb.Vert = bodyPr.Vert
		if strings.EqualFold(bodyPr.Wrap, "none") {
			tb.NoWrap = true
		}
		switch bodyPr.Anchor {
		case "t":
			tb.Anchor = "top"
		case "ctr":
			tb.Anchor = "center"
		case "b":
			tb.Anchor = "bottom"
		}
		tb.AnchorCenter = bodyPr.AnchorCtr
		tb.TextRotation = float64(bodyPr.Rot) / 60000.0
		if bodyPr.NoAutofit != nil {
			tb.Autofit = "none"
		} else if bodyPr.SpAutofit != nil {
			tb.Autofit = "shape"
		} else if bodyPr.NormAutofit != nil {
			tb.Autofit = "norm"
		}
		if bodyPr.NormAutofit != nil {
			tb.FontScale = bodyPr.NormAutofit.FontScale
			tb.LnSpcReduction = bodyPr.NormAutofit.LnSpcReduction
		}
		if bodyPr.PrstTxWarp != nil {
			tb.PrstTxWarp = bodyPr.PrstTxWarp.Prst
		}
		if bodyPr.FlatTx != nil {
			tb.FlatTx = true
		}
	}
	if dmlShape.SpPr != nil && dmlShape.SpPr.Xfrm != nil {
		tb.RotationDeg = drawingRotationDegrees(dmlShape.SpPr.Xfrm.Rot)
		tb.FlipH = dmlShape.SpPr.Xfrm.FlipH
		tb.FlipV = dmlShape.SpPr.Xfrm.FlipV
	}
	if dmlShape.SpPr != nil {
		tb.Effects = dmlShape.SpPr.DrawingEffectList(ctx.themeColors)
	} else if len(styleEffects.Effects) > 0 {
		tb.Effects = styleEffects
	}
	return tb
}

// resolveCell3D converts a parts.Cell3D into an IR Cell3D.
func resolveCell3D(c3d *parts.Cell3D) *Cell3D {
	if c3d == nil {
		return nil
	}
	out := &Cell3D{Material: c3d.PrstMaterial}
	if c3d.BevelT != nil {
		out.BevelTopWidth = int64(c3d.BevelT.W)
		out.BevelTopHeight = int64(c3d.BevelT.H)
	}
	if c3d.BevelB != nil {
		out.BevelBottomWidth = int64(c3d.BevelB.W)
		out.BevelBottomHeight = int64(c3d.BevelB.H)
	}
	if c := c3d.ExtrusionClr; c != nil {
		out.ExtrusionColor = resolveColorChoice(c)
	}
	if c := c3d.ContourClr; c != nil {
		out.ContourColor = resolveColorChoice(c)
	}
	return out
}

// resolveColorChoice extracts an sRGB hex color from a parts.ColorChoice.
func resolveColorChoice(c *parts.ColorChoice) string {
	if c == nil {
		return ""
	}
	if c.SrgbClr != nil && c.SrgbClr.Val != "" {
		return c.SrgbClr.Val
	}
	if c.SchemeClr != nil && c.SchemeClr.Val != "" {
		// Scheme colors require theme context; return the scheme name as a placeholder.
		return ""
	}
	return ""
}

// resolveDrawingMLTable converts a DrawingML table into an IR Table.
func resolveDrawingMLTable(ctx *resolveCtx, table *parts.DrawingTable) *Table {
	if table == nil {
		return nil
	}
	var rows []TableRow
	for _, tr := range table.Rows {
		var cells []TableCell
		for _, tc := range tr.Cells {
			cell := TableCell{}
			if tc.TcPr != nil && tc.TcPr.Cell3D != nil {
				cell.Format.Cell3D = resolveCell3D(tc.TcPr.Cell3D)
			}
			if tc.TxBody != nil {
				blocks := resolveDrawingMLTextBody(ctx, tc.TxBody)
				for _, b := range blocks {
					if b.Paragraph != nil {
						cell.Paragraphs = append(cell.Paragraphs, *b.Paragraph)
					}
				}
			}
			cells = append(cells, cell)
		}
		rows = append(rows, TableRow{Cells: cells})
	}
	return &Table{Rows: rows}
}

// resolveTextBox extracts resolved block content from a wps:txbx text box drawing.
// Returns nil if the drawing does not contain a text box shape element.
func resolveTextBox(ctx *resolveCtx, drawing *parts.Drawing) *TextBox {
	if drawing == nil {
		return nil
	}

	var graphic *parts.Graphic
	if drawing.Inline != nil {
		graphic = drawing.Inline.Graphic
	} else if drawing.Anchor != nil {
		graphic = drawing.Anchor.Graphic
	}
	if graphic == nil || graphic.GraphicData == nil {
		return nil
	}

	// Handle DrawingML shape with text body.
	if dmlShape := graphic.GraphicData.DrawingMLShape; dmlShape != nil && dmlShape.TxBody != nil {
		tb := buildTextBoxFromDrawingMLShape(ctx, dmlShape)
		if tb != nil {
			applyTextBoxPosition(tb, drawing)
			return tb
		}
	}

	// Handle DrawingML table inside graphic data.
	if graphic.GraphicData.Table != nil {
		table := resolveDrawingMLTable(ctx, graphic.GraphicData.Table)
		if table != nil {
			tb := &TextBox{
				Blocks: []Block{{Kind: "table", Table: table}},
			}
			applyTextBoxPosition(tb, drawing)
			return tb
		}
	}

	wpsShape := graphic.GraphicData.Shape
	if wpsShape == nil || wpsShape.TextBox == nil || wpsShape.TextBox.Content == nil {
		return nil
	}

	content := wpsShape.TextBox.Content
	_, _, blocks, _ := normalizeContentBlocks(ctx, content.Blocks, content.Paragraphs, content.Tables)

	// Resolve shape-level fill color; fall back to raw fill for scheme colors.
	fillColor := ""
	var borderColor string
	var borderWidth float64
	if sp := wpsShape.ShapeProps; sp != nil {
		fillColor = resolveDrawingFill(ctx, sp.Fill)
		if sp.Line != nil && sp.Line.Color != "" {
			borderColor = sp.Line.Color
			borderWidth = sp.Line.Width
		}
	}

	// Drop completely empty boxes that carry no visual information.
	if len(blocks) == 0 && fillColor == "" {
		return nil
	}

	// Apply style references when explicit properties are absent.
	styleFill, styleLine, styleEffects := resolveShapeStyle(ctx, wpsShape.Style)
	applyThemeFontFamilyToBlocks(blocks, resolveShapeStyleFontFamily(ctx.themeFonts, wpsShape.Style))
	if fillColor == "" && styleFill != nil {
		fillColor = resolveDrawingFill(ctx, styleFill)
	}
	if borderColor == "" && borderWidth == 0 && styleLine != nil {
		borderColor = styleLine.Color
		borderWidth = styleLine.Width
	}

	tb := &TextBox{Blocks: blocks, FillColor: fillColor, BorderColor: borderColor, BorderWidth: borderWidth}
	tb.PaddingLeft, tb.PaddingTop, tb.PaddingRight, tb.PaddingBottom = resolveBodyPrInsets(wpsShape.BodyPr)
	if wpsShape.BodyPr != nil {
		tb.Vert = wpsShape.BodyPr.Vert
		if strings.EqualFold(wpsShape.BodyPr.Wrap, "none") {
			tb.NoWrap = true
		}
		switch wpsShape.BodyPr.Anchor {
		case "t":
			tb.Anchor = "top"
		case "ctr":
			tb.Anchor = "center"
		case "b":
			tb.Anchor = "bottom"
		}
		tb.AnchorCenter = wpsShape.BodyPr.AnchorCtr
		tb.TextRotation = float64(wpsShape.BodyPr.Rot) / 60000.0
		tb.Upright = wpsShape.BodyPr.Upright
		tb.Autofit = wpsShape.BodyPr.Autofit
		tb.FontScale = wpsShape.BodyPr.FontScale
		tb.LnSpcReduction = wpsShape.BodyPr.LnSpcReduction
		tb.PrstTxWarp = wpsShape.BodyPr.PrstTxWarp
		tb.FlatTx = wpsShape.BodyPr.FlatTx
		tb.NumCol = wpsShape.BodyPr.NumCol
		if wpsShape.BodyPr.SpcCol > 0 {
			tb.SpcCol = emuToPoints(wpsShape.BodyPr.SpcCol)
		}
		tb.RtlCol = wpsShape.BodyPr.RtlCol
	}
	if wpsShape.ShapeProps != nil && wpsShape.ShapeProps.Xfrm != nil {
		tb.RotationDeg = drawingRotationDegrees(wpsShape.ShapeProps.Xfrm.Rot)
		tb.FlipH = wpsShape.ShapeProps.Xfrm.FlipH
		tb.FlipV = wpsShape.ShapeProps.Xfrm.FlipV
	}
	if wpsShape.ShapeProps != nil {
		tb.Effects = wpsShape.ShapeProps.DrawingEffectList(ctx.themeColors)
	} else if len(styleEffects.Effects) > 0 {
		tb.Effects = styleEffects
	}
	applyTextBoxPosition(tb, drawing)
	return tb
}

// applyTextBoxPosition applies anchor/inline positioning to a text box.
func applyTextBoxPosition(tb *TextBox, drawing *parts.Drawing) {
	if tb == nil || drawing == nil {
		return
	}
	if drawing.Anchor != nil {
		tb.Floating = true
		tb.Width, tb.Height = drawingExtentSize(drawing.Anchor.Extent)
		tb.X, tb.Y = drawingAnchorOffset(drawing.Anchor)
		if drawing.Anchor.PositionH != nil {
			tb.XRelative = drawing.Anchor.PositionH.RelativeFrom
			tb.XAlign = drawing.Anchor.PositionH.Align
			if drawing.Anchor.PositionH.Pct != 0 {
				tb.X = drawing.Anchor.PositionH.Pct
			}
		}
		if drawing.Anchor.PositionV != nil {
			tb.YRelative = drawing.Anchor.PositionV.RelativeFrom
			tb.YAlign = drawing.Anchor.PositionV.Align
			if drawing.Anchor.PositionV.Pct != 0 {
				tb.Y = drawing.Anchor.PositionV.Pct
			}
		}
		if drawing.Anchor.SimplePosAttr && drawing.Anchor.SimplePos != nil {
			tb.SimplePosAttr = true
			tb.SimplePosX = drawing.Anchor.SimplePos.X
			tb.SimplePosY = drawing.Anchor.SimplePos.Y
		}
		if drawing.Anchor.EffectExtent != nil {
			tb.EffectExtentL = drawing.Anchor.EffectExtent.L
			tb.EffectExtentT = drawing.Anchor.EffectExtent.T
			tb.EffectExtentR = drawing.Anchor.EffectExtent.R
			tb.EffectExtentB = drawing.Anchor.EffectExtent.B
		}
		tb.Locked = drawing.Anchor.Locked
		tb.LayoutInCell = drawing.Anchor.LayoutInCell
		tb.AllowOverlap = drawing.Anchor.AllowOverlap
		if drawing.Anchor.SizeRelH != nil {
			tb.SizeRelHPct = drawing.Anchor.SizeRelH.Pct
			tb.SizeRelHFrom = drawing.Anchor.SizeRelH.RelativeFrom
		}
		if drawing.Anchor.SizeRelV != nil {
			tb.SizeRelVPct = drawing.Anchor.SizeRelV.Pct
			tb.SizeRelVFrom = drawing.Anchor.SizeRelV.RelativeFrom
		}
		tb.ZIndex = drawing.Anchor.RelativeHeight
		tb.BehindDoc = drawing.Anchor.BehindDoc != 0
		tb.WrapType, tb.WrapSide = drawingAnchorWrap(drawing.Anchor)
		tb.WrapPolygon = drawingAnchorPolygon(drawing.Anchor)
		dt, db, dl, dr := drawingAnchorWrapDistances(drawing.Anchor)
		tb.DistTop = emuToPoints(dt)
		tb.DistBottom = emuToPoints(db)
		tb.DistLeft = emuToPoints(dl)
		tb.DistRight = emuToPoints(dr)
	} else if drawing.Inline != nil {
		tb.Width, tb.Height = drawingExtentSize(drawing.Inline.Extent)
	}
}

func resolveVMLTextBox(ctx *resolveCtx, content parts.RunContent) *TextBox {
	if content.VMLTextBox == nil {
		return nil
	}
	_, _, blocks, _ := normalizeContentBlocks(ctx, content.VMLTextBox.Blocks, content.VMLTextBox.Paragraphs, content.VMLTextBox.Tables)
	fillColor := normalizeVMLColor(content.VMLFillColor)
	if len(blocks) == 0 && fillColor == "" {
		return nil
	}

	tb := &TextBox{Blocks: blocks, FillColor: fillColor, Vert: parseVMLTextBoxVert(content.VMLStyle, content.VMLTextBoxStyle)}
	tb.Width, tb.Height = parseVMLStyleExtent(content.VMLStyle)
	if x, y, floating := parseVMLStylePosition(content.VMLStyle); floating {
		tb.Floating = true
		tb.WrapType = "square"
		tb.XRelative = "page"
		tb.YRelative = "page"
		tb.X = x
		tb.Y = y
	}
	return tb
}

func resolveVMLShape(content parts.RunContent) *Shape {
	if content.VMLShapeType == "" {
		return nil
	}

	kind := "rect"
	if strings.EqualFold(content.VMLShapeType, "line") {
		kind = "line"
	}

	width, height := parseVMLStyleExtent(content.VMLStyle)
	if kind == "line" && width == 0 && height == 0 {
		width, height = parseVMLLineExtent(content.VMLFrom, content.VMLTo)
	}

	hasFill := parseVMLBoolAttr(content.VMLFilledAttr, true)
	hasStroke := parseVMLBoolAttr(content.VMLStrokedAttr, true)
	fillColor := ""
	if hasFill {
		fillColor = normalizeVMLColor(content.VMLFillColor)
	}
	strokeColor := ""
	if hasStroke {
		strokeColor = normalizeVMLColor(content.VMLStrokeColor)
	}
	strokeWidth := int64(0)
	if hasStroke {
		strokeWidth = parseVMLDimensionEMU(content.VMLStrokeWeight)
	}
	if strokeWidth == 0 && kind == "line" && hasStroke {
		strokeWidth = parseVMLDimensionEMU("1pt")
	}

	if kind == "rect" && fillColor == "" && strokeColor == "" {
		return nil
	}
	if kind == "line" && !hasStroke {
		return nil
	}

	shape := &Shape{
		Kind:        kind,
		Width:       width,
		Height:      height,
		FillColor:   fillColor,
		StrokeColor: strokeColor,
		StrokeWidth: strokeWidth,
	}
	if x, y, floating := parseVMLStylePosition(content.VMLStyle); floating {
		shape.Floating = true
		shape.WrapType = "square"
		shape.XRelative = "page"
		shape.YRelative = "page"
		shape.X = x
		shape.Y = y
	}

	switch strings.ToLower(strings.TrimSpace(content.VMLShapeType)) {
	case "roundrect":
		shape.Geometry = &dml.PresetGeometry{Name: "roundRect"}
	case "oval":
		shape.Geometry = &dml.PresetGeometry{Name: "ellipse"}
	case "polyline":
		if geom := vmlPolylineGeometry(content.VMLPoints); geom != nil {
			shape.GeometryData = geom
		}
	case "curve":
		if geom := vmlCurveGeometry(content.VMLPoints); geom != nil {
			shape.GeometryData = geom
		}
	case "arc":
		if geom := vmlArcGeometry(content.VMLStartAngle, content.VMLEndAngle); geom != nil {
			shape.GeometryData = geom
		}
	case "shape":
		if geom := vmlPathGeometry(content.VMLPathData, content.VMLCoordSize, content.VMLCoordOrigin); geom != nil {
			shape.GeometryData = geom
		}
	}
	if on := content.VMLTextPathOn; on == "t" || on == "true" || on == "1" {
		if content.VMLTextPathString != "" {
			shape.TextPath = &TextPath{
				On:     on,
				String: content.VMLTextPathString,
			}
		}
	}
	if shape.Geometry != nil && strokeColor != "" {
		shape.LineStyle = &dml.LineStyle{
			Color: strokeColor,
			Width: emuToPoints(strokeWidth),
		}
	}

	return shape
}

func resolveVMLSignatureLine(vml *parts.VMLSignatureLine) *SignatureLine {
	if vml == nil {
		return nil
	}
	sl := &SignatureLine{
		SetupID:                vml.SetupID,
		SuggestedSigner:        vml.SuggestedSigner,
		SuggestedSigner2:       vml.SuggestedSigner2,
		SuggestedSignerEmail:   vml.SuggestedSignerEmail,
		SigningInstructions:    vml.SigningInstructions,
		ProviderID:             vml.ProviderID,
		AdditionalXML:          vml.AdditionalXML,
		ProviderURL:            vml.ProviderURL,
		IsSignatureLine:        vml.IsSignatureLine == "t" || vml.IsSignatureLine == "true" || vml.IsSignatureLine == "1",
		AllowComments:          vml.AllowComments == "t" || vml.AllowComments == "true" || vml.AllowComments == "1",
		ShowSignDate:           vml.ShowSignDate == "t" || vml.ShowSignDate == "true" || vml.ShowSignDate == "1",
		SigningInstructionsSet: vml.SigningInstructionsSet == "t" || vml.SigningInstructionsSet == "true" || vml.SigningInstructionsSet == "1",
	}
	return sl
}

// vmlParsePoints parses a VML points attribute (flat comma-separated numbers)
// into normalized (0-1) Point slices. Returns nil if parsing fails.
func vmlParsePoints(pointsStr string) []dml.Point {
	if pointsStr == "" {
		return nil
	}
	parts := strings.Split(pointsStr, ",")
	if len(parts) < 4 || len(parts)%2 != 0 {
		return nil
	}
	var coords []dml.Point
	var minX, minY, maxX, maxY float64
	for i := 0; i < len(parts); i += 2 {
		x, err1 := strconv.ParseFloat(strings.TrimSpace(parts[i]), 64)
		y, err2 := strconv.ParseFloat(strings.TrimSpace(parts[i+1]), 64)
		if err1 != nil || err2 != nil {
			return nil
		}
		coords = append(coords, dml.Point{X: x, Y: y})
		if i == 0 {
			minX, maxX = x, x
			minY, maxY = y, y
		} else {
			if x < minX {
				minX = x
			}
			if x > maxX {
				maxX = x
			}
			if y < minY {
				minY = y
			}
			if y > maxY {
				maxY = y
			}
		}
	}
	w := maxX - minX
	h := maxY - minY
	if w == 0 {
		w = 1
	}
	if h == 0 {
		h = 1
	}
	for i := range coords {
		coords[i].X = (coords[i].X - minX) / w
		coords[i].Y = (coords[i].Y - minY) / h
	}
	return coords
}

// vmlPolylineGeometry builds a custom Geometry from a VML polyline points string.
func vmlPolylineGeometry(pointsStr string) *dml.Geometry {
	coords := vmlParsePoints(pointsStr)
	if len(coords) < 2 {
		return nil
	}
	var cmds []dml.PathCommand
	cmd := dml.PathCommand{Type: "M", Points: []dml.Point{coords[0]}}
	cmd.Points[0].X = coords[0].X
	cmd.Points[0].Y = coords[0].Y
	cmds = append(cmds, cmd)
	for i := 1; i < len(coords); i++ {
		cmd := dml.PathCommand{Type: "L", Points: []dml.Point{coords[i]}}
		cmd.Points[0].X = coords[i].X
		cmd.Points[0].Y = coords[i].Y
		cmds = append(cmds, cmd)
	}
	return &dml.Geometry{
		Type:  dml.GeometryTypeCustom,
		Paths: []dml.Path{{Commands: cmds}},
	}
}

// vmlCurveGeometry builds a custom Geometry from a VML curve points string.
// VML curves specify 4 points: start, control1, control2, end (8 numbers).
func vmlCurveGeometry(pointsStr string) *dml.Geometry {
	coords := vmlParsePoints(pointsStr)
	if len(coords) != 4 {
		return nil
	}
	return &dml.Geometry{
		Type: dml.GeometryTypeCustom,
		Paths: []dml.Path{{Commands: []dml.PathCommand{
			{Type: "M", Points: []dml.Point{coords[0]}},
			{Type: "C", Points: []dml.Point{coords[1], coords[2], coords[3]}},
		}}},
	}
}

// vmlArcGeometry builds a custom Geometry for a VML arc shape.
// VML angles are in degrees, 0° at 12 o'clock, increasing clockwise.
// The arc is drawn as a pie wedge within the unit square [0,1]×[0,1].
func vmlArcGeometry(startAngleStr, endAngleStr string) *dml.Geometry {
	startAngle, err1 := strconv.ParseFloat(strings.TrimSpace(startAngleStr), 64)
	endAngle, err2 := strconv.ParseFloat(strings.TrimSpace(endAngleStr), 64)
	if err1 != nil || err2 != nil {
		return nil
	}
	// Normalize angles.
	startAngle = math.Mod(startAngle, 360)
	if startAngle < 0 {
		startAngle += 360
	}
	endAngle = math.Mod(endAngle, 360)
	if endAngle < 0 {
		endAngle += 360
	}
	// VML angle α: 0° at 12 o'clock, CW positive.
	// Convert to standard math θ (0° at 3 o'clock, CCW positive) for trig:
	//   θ = 90° - α   (in degrees)
	// In screen coords (y-down):
	//   x = 0.5 + 0.5*sin(α_rad)
	//   y = 0.5 - 0.5*cos(α_rad)
	rad := func(a float64) float64 { return a * math.Pi / 180 }
	pointOnEllipse := func(a float64) dml.Point {
		return dml.Point{
			X: 0.5 + 0.5*math.Sin(rad(a)),
			Y: 0.5 - 0.5*math.Cos(rad(a)),
		}
	}
	startPt := pointOnEllipse(startAngle)
	endPt := pointOnEllipse(endAngle)
	// Compute arc direction (CW span from start to end).
	delta := endAngle - startAngle
	if delta < 0 {
		delta += 360
	}
	largeArc := 0
	if delta > 180 {
		largeArc = 1
	}
	// In SVG/y-down coords, positive angles are CW, so sweep = 1 for CW.
	sweep := 1
	return &dml.Geometry{
		Type: dml.GeometryTypeCustom,
		Paths: []dml.Path{{Commands: []dml.PathCommand{
			{Type: "M", Points: []dml.Point{{X: 0.5, Y: 0.5}}},
			{Type: "L", Points: []dml.Point{startPt}},
			{Type: "A", Points: []dml.Point{endPt}, Args: []float64{0.5, 0.5, 0, float64(largeArc), float64(sweep)}},
			{Type: "Z"},
		}}},
	}
}

// vmlPathGeometry parses a VML shape path mini-language string into a custom Geometry.
// Supported commands: m (moveto), l (lineto), c (cubic bezier), x (closepath), e (end).
// Coordinates are normalized using coordSize/coordOrigin (defaults: 21600×21600, 0,0).
func vmlPathGeometry(pathData, coordSizeStr, coordOriginStr string) *dml.Geometry {
	if strings.TrimSpace(pathData) == "" {
		return nil
	}

	// Parse coordinate system.
	sizeX, sizeY := 21600.0, 21600.0
	if coordSizeStr != "" {
		parts := strings.Split(strings.TrimSpace(coordSizeStr), ",")
		if len(parts) == 2 {
			if sx, err := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64); err == nil && sx > 0 {
				sizeX = sx
			}
			if sy, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64); err == nil && sy > 0 {
				sizeY = sy
			}
		}
	}
	originX, originY := 0.0, 0.0
	if coordOriginStr != "" {
		parts := strings.Split(strings.TrimSpace(coordOriginStr), ",")
		if len(parts) == 2 {
			originX, _ = strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
			originY, _ = strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		}
	}
	normalize := func(x, y float64) dml.Point {
		return dml.Point{
			X: (x - originX) / sizeX,
			Y: (y - originY) / sizeY,
		}
	}

	// Tokenize: split on whitespace, then split comma-separated coordinates.
	tokens := strings.Fields(pathData)
	var cmds []dml.PathCommand
	var currentCmd string
	var pendingCoords []dml.Point

	flush := func() {
		if currentCmd == "" || len(pendingCoords) == 0 {
			return
		}
		switch currentCmd {
		case "m":
			cmds = append(cmds, dml.PathCommand{Type: "M", Points: []dml.Point{pendingCoords[0]}})
			if len(pendingCoords) > 1 {
				// Subsequent points after m are implicit lineto.
				cmds = append(cmds, dml.PathCommand{Type: "L", Points: pendingCoords[1:]})
			}
		case "l":
			cmds = append(cmds, dml.PathCommand{Type: "L", Points: append([]dml.Point(nil), pendingCoords...)})
		case "c":
			for len(pendingCoords) >= 3 {
				cmds = append(cmds, dml.PathCommand{Type: "C", Points: append([]dml.Point(nil), pendingCoords[:3]...)})
				pendingCoords = pendingCoords[3:]
			}
		}
		pendingCoords = nil
	}

	for _, tok := range tokens {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		lower := strings.ToLower(tok)
		switch lower {
		case "m", "l", "c":
			flush()
			currentCmd = lower
		case "x":
			flush()
			cmds = append(cmds, dml.PathCommand{Type: "Z"})
			currentCmd = ""
		case "e":
			flush()
			currentCmd = ""
		case "nf", "ns":
			// nofill / nostroke — structural hints, ignore for geometry.
		default:
			// Try to parse as coordinate pair x,y
			coords := strings.Split(tok, ",")
			if len(coords) == 2 {
				x, err1 := strconv.ParseFloat(strings.TrimSpace(coords[0]), 64)
				y, err2 := strconv.ParseFloat(strings.TrimSpace(coords[1]), 64)
				if err1 == nil && err2 == nil {
					pendingCoords = append(pendingCoords, normalize(x, y))
				}
			}
		}
	}
	flush()

	if len(cmds) == 0 {
		return nil
	}
	return &dml.Geometry{
		Type:  dml.GeometryTypeCustom,
		Paths: []dml.Path{{Commands: cmds}},
	}
}

// resolveChart extracts a chart from a drawing and returns it as an Image with SVG data.
func resolveChart(ctx *resolveCtx, drawing *parts.Drawing) *Image {
	if drawing == nil {
		return nil
	}

	var graphic *parts.Graphic
	var extent *parts.DrawingExtent
	var docPr *parts.DrawingDocPr
	floating := false

	if drawing.Inline != nil {
		graphic = drawing.Inline.Graphic
		extent = drawing.Inline.Extent
		docPr = drawing.Inline.DocPr
	} else if drawing.Anchor != nil {
		graphic = drawing.Anchor.Graphic
		extent = drawing.Anchor.Extent
		docPr = drawing.Anchor.DocPr
		floating = true
	}

	if graphic == nil || graphic.GraphicData == nil {
		return nil
	}

	// Check if this is a chart by looking at the URI
	uri := strings.ToLower(strings.TrimSpace(graphic.GraphicData.URI))
	if !strings.Contains(uri, "/chart") {
		return nil
	}

	// Find chart reference in graphic data
	// The chart relationship ID is typically stored in a nested element
	chartRef := findChartRef(graphic)
	if chartRef == "" {
		return nil
	}

	chartXML, ok := []byte(nil), false
	if ctx.pkg != nil {
		chartXML, ok = ctx.pkg.ChartPartFor(ctx.currentPart, chartRef)
	}
	if !ok || chartXML == nil {
		return nil
	}

	c, err := chart.ParseChart(chartXML)
	if err != nil || c == nil {
		return nil
	}
	c.ThemeColors = ctx.themeColors

	width, height := int64(5760000), int64(3240000) // Default 4x3 inches
	if extent != nil {
		width, height = extent.CX, extent.CY
	}

	svgStr := c.SVG(int(width/12700), int(height/12700))
	if svgStr == "" {
		return nil
	}

	img := &Image{
		MIMEType: "image/svg+xml",
		Data:     []byte(svgStr),
		Width:    width,
		Height:   height,
		Floating: floating,
		Alt:      "Chart",
	}

	if docPr != nil {
		img.Alt = docPr.Desc
		if img.Alt == "" {
			img.Alt = docPr.Title
		}
		if img.Alt == "" {
			img.Alt = docPr.Name
		}
		img.Title = docPr.Title
		img.Hidden = docPr.Hidden
	}

	if floating && drawing.Anchor != nil {
		img.X, img.Y = drawingAnchorOffset(drawing.Anchor)
		if drawing.Anchor.PositionH != nil {
			img.XRelative = drawing.Anchor.PositionH.RelativeFrom
			img.XAlign = drawing.Anchor.PositionH.Align
		}
		if drawing.Anchor.PositionV != nil {
			img.YRelative = drawing.Anchor.PositionV.RelativeFrom
			img.YAlign = drawing.Anchor.PositionV.Align
		}
		img.ZIndex = drawing.Anchor.RelativeHeight
		img.BehindDoc = drawing.Anchor.BehindDoc != 0
		img.WrapType, img.WrapSide = drawingAnchorWrap(drawing.Anchor)
		img.WrapPolygon = drawingAnchorPolygon(drawing.Anchor)
		img.DistTop = emuToPoints(drawing.Anchor.DistT)
		img.DistBottom = emuToPoints(drawing.Anchor.DistB)
		img.DistLeft = emuToPoints(drawing.Anchor.DistL)
		img.DistRight = emuToPoints(drawing.Anchor.DistR)
	}

	return img
}

// resolveSmartArt extracts a SmartArt diagram from a drawing and returns it as
// an SVG-backed image so the existing image pipeline can place it inline or as
// a float. The shared SmartArt engine is given every available companion part,
// including the optional pre-rendered diagramDrawing used by Office for exact
// geometry and styling.
func resolveSmartArt(ctx *resolveCtx, drawing *parts.Drawing) *Image {
	if drawing == nil {
		return nil
	}

	var graphic *parts.Graphic
	var extent *parts.DrawingExtent
	var docPr *parts.DrawingDocPr
	floating := false

	if drawing.Inline != nil {
		graphic = drawing.Inline.Graphic
		extent = drawing.Inline.Extent
		docPr = drawing.Inline.DocPr
	} else if drawing.Anchor != nil {
		graphic = drawing.Anchor.Graphic
		extent = drawing.Anchor.Extent
		docPr = drawing.Anchor.DocPr
		floating = true
	}
	if graphic == nil || graphic.GraphicData == nil {
		return nil
	}
	uri := strings.ToLower(strings.TrimSpace(graphic.GraphicData.URI))
	if !strings.Contains(uri, "/diagram") {
		return nil
	}

	relIDs := graphic.GraphicData.DiagramIDs
	if relIDs == nil || relIDs.DataModelID == "" || ctx.pkg == nil {
		return nil
	}
	dataModelXML, ok := ctx.pkg.SmartArtPartFor(ctx.currentPart, relIDs.DataModelID)
	if !ok || len(dataModelXML) == 0 {
		return nil
	}

	width, height := int64(5760000), int64(3240000)
	if extent != nil {
		if extent.CX > 0 {
			width = extent.CX
		}
		if extent.CY > 0 {
			height = extent.CY
		}
	}

	var layoutXML, styleXML, colorXML, drawingXML []byte
	// Some producers put lo/qs/cs directly on dgm:relIds. Prefer those exact
	// references when present, then fill any gaps from the diagram data rels.
	if relIDs.LayoutID != "" {
		layoutXML, _ = ctx.pkg.SmartArtLayoutPartFor(ctx.currentPart, relIDs.LayoutID)
	}
	if relIDs.QuickStyleID != "" {
		styleXML, _ = ctx.pkg.SmartArtStylePartFor(ctx.currentPart, relIDs.QuickStyleID)
	}
	if relIDs.ColorStyleID != "" {
		colorXML, _ = ctx.pkg.SmartArtColorPartFor(ctx.currentPart, relIDs.ColorStyleID)
	}

	dataPartName, _ := ctx.pkg.RelationshipTarget(ctx.currentPart, relIDs.DataModelID)
	drawingPartName := ""
	if dataPartName != "" {
		if rels := ctx.pkg.PartRels[dataPartName]; rels != nil {
			for _, rel := range rels.Relationships {
				if rel == nil || rel.TargetMode == "External" {
					continue
				}
				switch rel.Type {
				case "http://schemas.openxmlformats.org/officeDocument/2006/relationships/diagramLayout":
					if len(layoutXML) == 0 {
						layoutXML, _ = ctx.pkg.SmartArtLayoutPartFor(dataPartName, rel.ID)
					}
				case "http://schemas.openxmlformats.org/officeDocument/2006/relationships/diagramQuickStyle":
					if len(styleXML) == 0 {
						styleXML, _ = ctx.pkg.SmartArtStylePartFor(dataPartName, rel.ID)
					}
				case "http://schemas.openxmlformats.org/officeDocument/2006/relationships/diagramColors":
					if len(colorXML) == 0 {
						colorXML, _ = ctx.pkg.SmartArtColorPartFor(dataPartName, rel.ID)
					}
				case "http://schemas.microsoft.com/office/2007/relationships/diagramDrawing":
					drawingXML, _ = ctx.pkg.SmartArtDrawingPartFor(dataPartName, rel.ID)
					drawingPartName, _ = ctx.pkg.RelationshipTarget(dataPartName, rel.ID)
				}
			}
		}
	}

	frame := &smartartrender.FrameBounds{W: float64(width), H: float64(height)}
	shapes, err := smartartrender.ProcessDiagramWithOptionsAndFrameFormat(
		dataModelXML, layoutXML, styleXML, colorXML, drawingXML, frame, ctx.themeColors, nil,
	)
	if err != nil || len(shapes) == 0 {
		return nil
	}

	// Materialize picture fills from the diagramDrawing relationship scope.
	for _, shape := range shapes {
		materializeSmartArtPictureFill(ctx, shape, drawingPartName, dataPartName)
	}

	var connectorLine *dml.LineStyle
	if diagram, parseErr := smartart.ParseDiagramWithOptionsAndThemeFormat(dataModelXML, layoutXML, styleXML, colorXML, ctx.themeColors, nil); parseErr == nil && diagram != nil && diagram.Style != nil {
		connectorLine = diagram.Style.ConnectorLine
	}
	if len(drawingXML) > 0 {
		if line := smartartrender.ExtractConnectorStyle(drawingXML, ctx.themeColors); line != nil {
			connectorLine = line
		}
	}

	exactConnectors := resolveSmartArtDrawingConnectors(drawingXML, frame, ctx.themeColors)
	svgStr := renderSmartArtSVGFull(shapes, int(width/12700), int(height/12700), connectorLine, exactConnectors)
	if svgStr == "" {
		return nil
	}

	img := &Image{
		MIMEType: "image/svg+xml",
		Data:     []byte(svgStr),
		Width:    width,
		Height:   height,
		Floating: floating,
		Alt:      "SmartArt",
	}
	if docPr != nil {
		img.Alt = docPr.Desc
		if img.Alt == "" {
			img.Alt = docPr.Title
		}
		if img.Alt == "" {
			img.Alt = docPr.Name
		}
		img.Title = docPr.Title
		img.Hidden = docPr.Hidden
	}
	if floating && drawing.Anchor != nil {
		img.X, img.Y = drawingAnchorOffset(drawing.Anchor)
		if drawing.Anchor.PositionH != nil {
			img.XRelative = drawing.Anchor.PositionH.RelativeFrom
			img.XAlign = drawing.Anchor.PositionH.Align
		}
		if drawing.Anchor.PositionV != nil {
			img.YRelative = drawing.Anchor.PositionV.RelativeFrom
			img.YAlign = drawing.Anchor.PositionV.Align
		}
		img.ZIndex = drawing.Anchor.RelativeHeight
		img.BehindDoc = drawing.Anchor.BehindDoc != 0
		img.WrapType, img.WrapSide = drawingAnchorWrap(drawing.Anchor)
		img.WrapPolygon = drawingAnchorPolygon(drawing.Anchor)
		img.DistTop = emuToPoints(drawing.Anchor.DistT)
		img.DistBottom = emuToPoints(drawing.Anchor.DistB)
		img.DistLeft = emuToPoints(drawing.Anchor.DistL)
		img.DistRight = emuToPoints(drawing.Anchor.DistR)
	}
	return img
}

func materializeSmartArtPictureFill(ctx *resolveCtx, shape *smartartrender.SmartArtShape, scopes ...string) {
	if ctx == nil || ctx.pkg == nil || shape == nil || shape.Fill == nil {
		return
	}
	materialize := func(pf *dml.PictureFill) {
		if pf == nil || pf.EmbedID == "" || len(pf.ImageData) > 0 {
			return
		}
		for _, scope := range scopes {
			if scope == "" {
				continue
			}
			if media, ok := ctx.pkg.MediaPartFor(scope, pf.EmbedID); ok {
				pf.ImageData = append([]byte(nil), media.Data...)
				pf.ContentType = media.ContentType
				return
			}
		}
		if media, ok := ctx.pkg.MediaPartFor(ctx.currentPart, pf.EmbedID); ok {
			pf.ImageData = append([]byte(nil), media.Data...)
			pf.ContentType = media.ContentType
		}
	}
	switch fill := shape.Fill.(type) {
	case *dml.PictureFill:
		materialize(fill)
	case dml.PictureFill:
		copyFill := fill
		materialize(&copyFill)
		shape.Fill = copyFill
	}
}

// findChartRef extracts the chart relationship ID from a graphic element.
// It checks for a Chart field in the GraphicData which is populated when
// the graphic contains a c:chart element.
func findChartRef(graphic *parts.Graphic) string {
	if graphic == nil || graphic.GraphicData == nil {
		return ""
	}

	// Check if there's a chart reference in the GraphicData
	if graphic.GraphicData.Chart != nil && graphic.GraphicData.Chart.ID != "" {
		return graphic.GraphicData.Chart.ID
	}

	return ""
}

func findSmartArtRef(graphic *parts.Graphic) string {
	if graphic == nil || graphic.GraphicData == nil || graphic.GraphicData.DiagramIDs == nil {
		return ""
	}
	return graphic.GraphicData.DiagramIDs.DataModelID
}

type smartArtDrawingConnector struct {
	Type         string
	X, Y, W, H   float64 // points in the same frame coordinate space as SmartArt shapes
	Adjustments  []int
	FlipH        bool
	FlipV        bool
	Line         *dml.LineStyle
	DrawingOrder int
}

func resolveSmartArtDrawingConnectors(drawingXML []byte, frame *smartartrender.FrameBounds, themeColors map[string]string) []smartArtDrawingConnector {
	if len(drawingXML) == 0 || frame == nil {
		return nil
	}
	infos := smartartrender.ExtractConnectors(drawingXML, themeColors)
	if len(infos) == 0 {
		return nil
	}
	shapeInfos := smartartrender.ExtractShapeInfos(drawingXML, themeColors)
	minX, minY, maxX, maxY, ok := smartartrender.DrawingBounds(shapeInfos)
	if !ok || maxX == minX || maxY == minY {
		return nil
	}
	frameW, frameH := frame.W/12700.0, frame.H/12700.0
	frameX, frameY := frame.X/12700.0, frame.Y/12700.0
	scale := frameW / (maxX - minX)
	if sy := frameH / (maxY - minY); sy < scale {
		scale = sy
	}
	offsetX := frameX + (frameW-(maxX-minX)*scale)/2 - minX*scale
	offsetY := frameY + (frameH-(maxY-minY)*scale)/2 - minY*scale
	out := make([]smartArtDrawingConnector, 0, len(infos))
	for _, info := range infos {
		if info == nil {
			continue
		}
		adjs := make([]int, len(info.Adjustments))
		for i, v := range info.Adjustments {
			adjs[i] = int(math.Round(v * 100000))
		}
		out = append(out, smartArtDrawingConnector{
			Type: info.Geometry, X: info.X*scale + offsetX, Y: info.Y*scale + offsetY,
			W: info.W * scale, H: info.H * scale, Adjustments: adjs, FlipH: info.FlipH, FlipV: info.FlipV,
			Line: info.Line, DrawingOrder: info.DrawingOrder,
		})
	}
	return out
}

func renderSmartArtSVG(shapes []*smartartrender.SmartArtShape, width, height int) string {
	return renderSmartArtSVGFull(shapes, width, height, nil, nil)
}

func renderSmartArtSVGWithStyle(shapes []*smartartrender.SmartArtShape, width, height int, connectorLine *dml.LineStyle) string {
	return renderSmartArtSVGFull(shapes, width, height, connectorLine, nil)
}

func renderSmartArtSVGFull(shapes []*smartartrender.SmartArtShape, width, height int, connectorLine *dml.LineStyle, exactConnectors []smartArtDrawingConnector) string {
	if len(shapes) == 0 {
		return ""
	}
	if width <= 0 {
		width = 800
	}
	if height <= 0 {
		height = 600
	}

	ordered := append([]*smartartrender.SmartArtShape(nil), shapes...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].DrawingOrder < ordered[j].DrawingOrder })
	minX, minY := ordered[0].X, ordered[0].Y
	maxX, maxY := ordered[0].X+ordered[0].Width, ordered[0].Y+ordered[0].Height
	for _, shape := range ordered[1:] {
		minX = math.Min(minX, shape.X)
		minY = math.Min(minY, shape.Y)
		maxX = math.Max(maxX, shape.X+shape.Width)
		maxY = math.Max(maxY, shape.Y+shape.Height)
	}
	boundsW, boundsH := maxX-minX, maxY-minY
	if boundsW <= 0 {
		boundsW = 1
	}
	if boundsH <= 0 {
		boundsH = 1
	}
	padding := 12.0
	scale := math.Min((float64(width)-2*padding)/boundsW, (float64(height)-2*padding)/boundsH)
	if scale <= 0 || math.IsInf(scale, 0) || math.IsNaN(scale) {
		scale = 1
	}
	offsetX := (float64(width)-boundsW*scale)/2 - minX*scale
	offsetY := (float64(height)-boundsH*scale)/2 - minY*scale

	boxes := map[string]smartArtSVGBox{}
	for _, shape := range ordered {
		boxes[shape.ID] = smartArtSVGBox{offsetX + shape.X*scale, offsetY + shape.Y*scale, shape.Width * scale, shape.Height * scale}
	}

	var defs, body strings.Builder
	fmt.Fprintf(&body, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">`, width, height, width, height)
	body.WriteString(`<rect width="100%" height="100%" fill="white" fill-opacity="0"/>`)

	// Connectors are emitted before nodes so they remain behind shape faces.
	connColor, connWidth, connDash := "7F7F7F", 1.5, ""
	if connectorLine != nil {
		if connectorLine.Color != "" {
			connColor = strings.TrimPrefix(connectorLine.Color, "#")
		}
		if connectorLine.Width > 0 {
			connWidth = connectorLine.Width
		}
		if dash := connectorLine.DashArray(); dash != "none" {
			connDash = dash
		}
	}
	if len(exactConnectors) > 0 {
		sort.SliceStable(exactConnectors, func(i, j int) bool { return exactConnectors[i].DrawingOrder < exactConnectors[j].DrawingOrder })
		for _, conn := range exactConnectors {
			line := connectorLine
			if conn.Line != nil {
				line = conn.Line
			}
			color, width, extra := smartArtSVGStroke(line)
			spec := dml.ConnectorSpec{Type: conn.Type, BoundsX: offsetX + conn.X*scale, BoundsY: offsetY + conn.Y*scale, BoundsW: conn.W * scale, BoundsH: conn.H * scale, Adjustments: conn.Adjustments, FlipH: conn.FlipH, FlipV: conn.FlipV, LineStyle: line, LineColor: color, LineWidthPt: width}
			if line != nil {
				if line.HeadEnd != nil {
					spec.ArrowHead = string(line.HeadEnd.Type)
					spec.ArrowHeadWidth = string(line.HeadEnd.Size)
					spec.ArrowHeadLength = string(line.HeadEnd.Length)
				}
				if line.TailEnd != nil {
					spec.ArrowTail = string(line.TailEnd.Type)
					spec.ArrowTailWidth = string(line.TailEnd.Size)
					spec.ArrowTailLength = string(line.TailEnd.Length)
				}
			}
			for _, prim := range dml.BuildConnectorPrimitives(spec) {
				writeSmartArtPrimitive(&body, prim, `fill="none"`, color, width, extra)
			}
		}
	} else {
		for _, shape := range ordered {
			from, ok := boxes[shape.ID]
			if !ok {
				continue
			}
			for _, conn := range shape.Connections {
				to, ok := boxes[conn.ToID]
				if !ok {
					continue
				}
				x1, y1, x2, y2 := smartArtEdgeAnchors(from, to)
				fmt.Fprintf(&body, `<line x1="%.2f" y1="%.2f" x2="%.2f" y2="%.2f" stroke="#%s" stroke-width="%.2f"`, x1, y1, x2, y2, connColor, connWidth)
				if connDash != "" {
					fmt.Fprintf(&body, ` stroke-dasharray="%s"`, connDash)
				}
				body.WriteString(`/>`)
			}
		}
	}

	for idx, shape := range ordered {
		b := boxes[shape.ID]
		if b.w <= 0 || b.h <= 0 {
			continue
		}
		fillID := fmt.Sprintf("sa-fill-%d", idx)
		fillAttr := smartArtSVGFill(shape.Fill, fillID, &defs)
		strokeColor, strokeWidth, strokeExtra := smartArtSVGStroke(shape.Line)
		filterID := smartArtSVGEffect(shape.Effect, idx, &defs)

		transform := fmt.Sprintf("translate(%.2f %.2f)", b.x, b.y)
		cx, cy := b.w/2, b.h/2
		if shape.RotationDeg != 0 {
			transform += fmt.Sprintf(" rotate(%.4f %.2f %.2f)", shape.RotationDeg, cx, cy)
		}
		if shape.FlipH || shape.FlipV {
			sx, sy := 1.0, 1.0
			tx, ty := 0.0, 0.0
			if shape.FlipH {
				sx, tx = -1, -b.w
			}
			if shape.FlipV {
				sy, ty = -1, -b.h
			}
			transform += fmt.Sprintf(" translate(%.2f %.2f) scale(%.0f %.0f)", tx, ty, sx, sy)
		}
		fmt.Fprintf(&body, `<g transform="%s"`, transform)
		if filterID != "" {
			fmt.Fprintf(&body, ` filter="url(#%s)"`, filterID)
		}
		body.WriteString(`>`)

		preset := &dml.PresetGeometry{Name: shape.Type, Adjustments: append([]float64(nil), shape.GeometryAdjustments...)}
		geom := shape.CustomGeometry
		if geom == nil && shape.Type != "" {
			geom = &dml.Geometry{Type: dml.GeometryTypePreset, Name: shape.Type}
		}
		prims, _ := dml.BuildShapePrimitives(dml.ShapeGeometrySpec{Geometry: geom, Preset: preset, Width: b.w, Height: b.h, Fill: shape.Fill, Stroke: shape.Line, StrokeWidth: strokeWidth})
		if len(prims) == 0 {
			fmt.Fprintf(&body, `<rect x="0" y="0" width="%.2f" height="%.2f" rx="%.2f" %s stroke="#%s" stroke-width="%.2f"%s/>`, b.w, b.h, math.Min(b.w, b.h)*.08, fillAttr, strokeColor, strokeWidth, strokeExtra)
		} else {
			for _, prim := range prims {
				writeSmartArtPrimitive(&body, prim, fillAttr, strokeColor, strokeWidth, strokeExtra)
			}
		}
		renderSmartArtText(&body, shape, b.w, b.h, scale)
		body.WriteString(`</g>`)
	}
	if defs.Len() > 0 {
		body.WriteString(`<defs>` + defs.String() + `</defs>`)
	}
	body.WriteString(`</svg>`)
	return body.String()
}

type smartArtSVGBox struct{ x, y, w, h float64 }

func smartArtEdgeAnchors(a, b smartArtSVGBox) (float64, float64, float64, float64) {
	acx, acy := a.x+a.w/2, a.y+a.h/2
	bcx, bcy := b.x+b.w/2, b.y+b.h/2
	dx, dy := bcx-acx, bcy-acy
	if math.Abs(dx)*a.h > math.Abs(dy)*a.w {
		if dx >= 0 {
			acx = a.x + a.w
			bcx = b.x
		} else {
			acx = a.x
			bcx = b.x + b.w
		}
	} else {
		if dy >= 0 {
			acy = a.y + a.h
			bcy = b.y
		} else {
			acy = a.y
			bcy = b.y + b.h
		}
	}
	return acx, acy, bcx, bcy
}

func smartArtSVGFill(fill dml.Fill, id string, defs *strings.Builder) string {
	switch f := fill.(type) {
	case nil:
		return `fill="#4472C4"`
	case dml.NoFill, *dml.NoFill:
		return `fill="none"`
	case dml.SolidFill:
		return smartArtSolidFill(f)
	case *dml.SolidFill:
		if f != nil {
			return smartArtSolidFill(*f)
		}
	case dml.GradientFill:
		return smartArtGradientFill(f, id, defs)
	case *dml.GradientFill:
		if f != nil {
			return smartArtGradientFill(*f, id, defs)
		}
	case dml.PatternFill:
		return smartArtPatternFill(f, id, defs)
	case *dml.PatternFill:
		if f != nil {
			return smartArtPatternFill(*f, id, defs)
		}
	case dml.PictureFill:
		return smartArtPictureFill(f, id, defs)
	case *dml.PictureFill:
		if f != nil {
			return smartArtPictureFill(*f, id, defs)
		}
	}
	return `fill="#4472C4"`
}

func smartArtSolidFill(f dml.SolidFill) string {
	color := strings.TrimPrefix(f.ApplyColorTransforms(f.Color), "#")
	if color == "" {
		color = "4472C4"
	}
	alpha := f.ResolvedAlpha()
	if alpha < .999 {
		return fmt.Sprintf(`fill="#%s" fill-opacity="%.4f"`, color, alpha)
	}
	return fmt.Sprintf(`fill="#%s"`, color)
}

func smartArtGradientFill(f dml.GradientFill, id string, defs *strings.Builder) string {
	if f.GradientType == dml.GradientTypeRadial || f.GradientType == dml.GradientTypePath {
		fmt.Fprintf(defs, `<radialGradient id="%s">`, id)
	} else {
		x2, y2 := 100*math.Cos(f.Angle*math.Pi/180), 100*math.Sin(f.Angle*math.Pi/180)
		fmt.Fprintf(defs, `<linearGradient id="%s" x1="0%%" y1="0%%" x2="%.2f%%" y2="%.2f%%">`, id, 50+x2/2, 50+y2/2)
	}
	for _, st := range f.Stops {
		color := strings.TrimPrefix(st.Color, "#")
		alpha := st.Alpha
		if alpha <= 0 {
			alpha = 1
		}
		fmt.Fprintf(defs, `<stop offset="%.2f%%" stop-color="#%s" stop-opacity="%.4f"/>`, st.Position*100, color, alpha)
	}
	if f.GradientType == dml.GradientTypeRadial || f.GradientType == dml.GradientTypePath {
		defs.WriteString(`</radialGradient>`)
	} else {
		defs.WriteString(`</linearGradient>`)
	}
	return fmt.Sprintf(`fill="url(#%s)"`, id)
}

func smartArtPatternFill(f dml.PatternFill, id string, defs *strings.Builder) string {
	fg, bg := strings.TrimPrefix(f.Foreground, "#"), strings.TrimPrefix(f.Background, "#")
	if fg == "" {
		fg = "000000"
	}
	if bg == "" {
		bg = "FFFFFF"
	}
	fmt.Fprintf(defs, `<pattern id="%s" width="8" height="8" patternUnits="userSpaceOnUse"><rect width="8" height="8" fill="#%s"/>`, id, bg)
	switch string(f.PatternType) {
	case "darkVertical", "lightVertical", "dashedVertical":
		fmt.Fprintf(defs, `<path d="M4 0V8" stroke="#%s" stroke-width="1"/>`, fg)
	case "darkHorizontal", "lightHorizontal", "dashedHorizontal":
		fmt.Fprintf(defs, `<path d="M0 4H8" stroke="#%s" stroke-width="1"/>`, fg)
	case "smallGrid", "largeGrid", "cross":
		fmt.Fprintf(defs, `<path d="M4 0V8M0 4H8" stroke="#%s" stroke-width="1"/>`, fg)
	default:
		fmt.Fprintf(defs, `<path d="M-2 8L8-2M2 10L10 2" stroke="#%s" stroke-width="1"/>`, fg)
	}
	defs.WriteString(`</pattern>`)
	return fmt.Sprintf(`fill="url(#%s)"`, id)
}

func smartArtPictureFill(f dml.PictureFill, id string, defs *strings.Builder) string {
	src := f.SourceURL()
	if src == "" {
		return `fill="#4472C4"`
	}

	// Tile fills use the authored tile scale/offset instead of collapsing to a
	// single stretched bitmap. DrawingML scale values are thousandths of a
	// percent, so 100000 == 100% of the source bitmap.
	if f.Tile {
		sx, sy := f.TileScaleX, f.TileScaleY
		if sx <= 0 {
			sx = 100000
		}
		if sy <= 0 {
			sy = 100000
		}
		pw := math.Max(.02, float64(sx)/100000.0)
		ph := math.Max(.02, float64(sy)/100000.0)
		x := float64(f.TileOffsetX) / 914400.0
		y := float64(f.TileOffsetY) / 914400.0
		// algn selects which point of the tile grid is pinned to the shape.
		// Apply it in object-bounding-box space before the authored offset.
		ax, ay := 0.0, 0.0
		switch strings.ToLower(f.TileAlign) {
		case "t":
			ax = .5
		case "tr":
			ax = 1
		case "l":
			ay = .5
		case "ctr":
			ax, ay = .5, .5
		case "r":
			ax, ay = 1, .5
		case "bl":
			ay = 1
		case "b":
			ax, ay = .5, 1
		case "br":
			ax, ay = 1, 1
		}
		x += ax * (1 - pw)
		y += ay * (1 - ph)
		fmt.Fprintf(defs, `<pattern id="%s" x="%.5f" y="%.5f" width="%.5f" height="%.5f" patternContentUnits="objectBoundingBox">`, id, x, y, pw, ph)
		flip := ""
		switch strings.ToLower(f.TileFlip) {
		case "x":
			flip = ` transform="translate(1 0) scale(-1 1)"`
		case "y":
			flip = ` transform="translate(0 1) scale(1 -1)"`
		case "xy":
			flip = ` transform="translate(1 1) scale(-1 -1)"`
		}
		opacity := ""
		if f.Alpha > 0 && f.Alpha < 1 {
			opacity = fmt.Sprintf(` opacity="%.4f"`, f.Alpha)
		}
		fmt.Fprintf(defs, `<image href="%s" x="0" y="0" width="1" height="1" preserveAspectRatio="xMidYMid slice"%s%s/>`, html.EscapeString(src), flip, opacity)
		defs.WriteString(`</pattern>`)
		return fmt.Sprintf(`fill="url(#%s)"`, id)
	}

	// Stretch/crop fills are represented as an object-bounding-box pattern. A
	// crop rectangle is the visible source region; enlarge/offset the image so
	// that only that region lands inside the shape bounds.
	x, y, iw, ih := 0.0, 0.0, 1.0, 1.0
	if f.Crop != nil && f.Crop.Width > 0 && f.Crop.Height > 0 {
		iw = 1.0 / f.Crop.Width
		ih = 1.0 / f.Crop.Height
		x = -f.Crop.X * iw
		y = -f.Crop.Y * ih
	}
	opacity := ""
	if f.Alpha > 0 && f.Alpha < 1 {
		opacity = fmt.Sprintf(` opacity="%.4f"`, f.Alpha)
	}
	fmt.Fprintf(defs, `<pattern id="%s" width="1" height="1" patternContentUnits="objectBoundingBox"><image href="%s" x="%.6f" y="%.6f" width="%.6f" height="%.6f" preserveAspectRatio="xMidYMid slice"%s/></pattern>`, id, html.EscapeString(src), x, y, iw, ih, opacity)
	return fmt.Sprintf(`fill="url(#%s)"`, id)
}

func smartArtSVGStroke(line *dml.LineStyle) (string, float64, string) {
	if line == nil {
		return "2F5597", 1.5, ""
	}
	color := strings.TrimPrefix(line.Color, "#")
	if color == "" {
		color = "2F5597"
	}
	width := line.Width
	if width <= 0 {
		width = 1.5
	}
	extra := ""
	if dash := line.DashArray(); dash != "none" {
		extra += fmt.Sprintf(` stroke-dasharray="%s"`, dash)
	}
	if line.Alpha > 0 && line.Alpha < 1 {
		extra += fmt.Sprintf(` stroke-opacity="%.4f"`, line.Alpha)
	}
	return color, width, extra
}

func smartArtSVGEffect(effects dml.EffectList, idx int, defs *strings.Builder) string {
	if len(effects.Effects) == 0 {
		return ""
	}
	id := fmt.Sprintf("sa-effect-%d", idx)
	var nodes strings.Builder
	for _, effect := range effects.Effects {
		switch e := effect.(type) {
		case dml.Shadow:
			dx := math.Cos(e.Angle*math.Pi/180) * e.Distance
			dy := math.Sin(e.Angle*math.Pi/180) * e.Distance
			fmt.Fprintf(&nodes, `<feDropShadow dx="%.2f" dy="%.2f" stdDeviation="%.2f" flood-color="#%s" flood-opacity="%.3f"/>`, dx, dy, math.Max(.01, e.Blur/2), strings.TrimPrefix(e.Color, "#"), e.Alpha)
		case dml.InnerShadow:
			// SVG has no native inset-shadow primitive. Approximate it by blurring
			// SourceAlpha and compositing the displaced shadow inside the source.
			dx := math.Cos(e.Angle*math.Pi/180) * e.Distance
			dy := math.Sin(e.Angle*math.Pi/180) * e.Distance
			fmt.Fprintf(&nodes, `<feGaussianBlur in="SourceAlpha" stdDeviation="%.2f" result="is-blur"/><feOffset in="is-blur" dx="%.2f" dy="%.2f" result="is-off"/><feComposite in="is-off" in2="SourceAlpha" operator="in" result="is-clip"/><feFlood flood-color="#%s" flood-opacity="%.3f" result="is-color"/><feComposite in="is-color" in2="is-clip" operator="in" result="is-shadow"/><feMerge><feMergeNode in="SourceGraphic"/><feMergeNode in="is-shadow"/></feMerge>`, math.Max(.01, e.Blur/2), dx, dy, strings.TrimPrefix(e.Color, "#"), e.Alpha)
		case dml.Glow:
			fmt.Fprintf(&nodes, `<feGaussianBlur in="SourceAlpha" stdDeviation="%.2f" result="glow"/><feFlood flood-color="#%s" flood-opacity="%.3f" result="glowColor"/><feComposite in="glowColor" in2="glow" operator="in" result="glowPaint"/><feMerge><feMergeNode in="glowPaint"/><feMergeNode in="SourceGraphic"/></feMerge>`, math.Max(.01, e.Radius/2), strings.TrimPrefix(e.Color, "#"), e.Alpha)
		case dml.Blur:
			fmt.Fprintf(&nodes, `<feGaussianBlur stdDeviation="%.2f"/>`, math.Max(.01, e.Radius/2))
		case dml.SoftEdge:
			// A blurred alpha mask gives a portable feathered-edge approximation.
			fmt.Fprintf(&nodes, `<feGaussianBlur in="SourceAlpha" stdDeviation="%.2f" result="softAlpha"/><feComposite in="SourceGraphic" in2="softAlpha" operator="in"/>`, math.Max(.01, e.Radius/2))
		case dml.Reflection:
			// Keep reflection in the same filter chain so effect order remains
			// deterministic. Direction controls the reflection offset.
			dx := math.Cos(e.Direction*math.Pi/180) * e.Distance
			dy := math.Sin(e.Direction*math.Pi/180) * e.Distance
			opacity := e.Alpha
			if opacity <= 0 {
				opacity = .35
			}
			fmt.Fprintf(&nodes, `<feOffset in="SourceGraphic" dx="%.2f" dy="%.2f" result="reflOff"/><feGaussianBlur in="reflOff" stdDeviation="%.2f" result="reflBlur"/><feComponentTransfer in="reflBlur" result="reflFade"><feFuncA type="linear" slope="%.4f"/></feComponentTransfer><feMerge><feMergeNode in="reflFade"/><feMergeNode in="SourceGraphic"/></feMerge>`, dx, dy, math.Max(0, e.Blur/2), opacity)
		}
	}
	if nodes.Len() == 0 {
		return ""
	}
	fmt.Fprintf(defs, `<filter id="%s" x="-50%%" y="-50%%" width="200%%" height="200%%">%s</filter>`, id, nodes.String())
	return id
}

func writeSmartArtPrimitive(b *strings.Builder, prim dml.VectorPrimitive, fillAttr, strokeColor string, strokeWidth float64, strokeExtra string) {
	fill := fillAttr
	if prim.Style.FillColor != "" {
		fill = fmt.Sprintf(`fill="#%s"`, strings.TrimPrefix(prim.Style.FillColor, "#"))
	}
	stroke := strokeColor
	if prim.Style.StrokeColor != "" {
		stroke = strings.TrimPrefix(prim.Style.StrokeColor, "#")
	}
	sw := strokeWidth
	if prim.Style.StrokeWidth > 0 {
		sw = prim.Style.StrokeWidth
	}
	switch prim.Kind {
	case dml.PrimitiveEllipse:
		if prim.Ellipse != nil {
			fmt.Fprintf(b, `<ellipse cx="%.2f" cy="%.2f" rx="%.2f" ry="%.2f" %s stroke="#%s" stroke-width="%.2f"%s/>`, prim.Ellipse.Center.X, prim.Ellipse.Center.Y, prim.Ellipse.RX, prim.Ellipse.RY, fill, stroke, sw, strokeExtra)
		}
	case dml.PrimitivePolyline:
		if len(prim.Points) > 0 {
			var pts strings.Builder
			for i, p := range prim.Points {
				if i > 0 {
					pts.WriteByte(' ')
				}
				fmt.Fprintf(&pts, "%.2f,%.2f", p.X, p.Y)
			}
			tag := "polyline"
			if prim.Closed {
				tag = "polygon"
			}
			fmt.Fprintf(b, `<%s points="%s" %s stroke="#%s" stroke-width="%.2f"%s/>`, tag, pts.String(), fill, stroke, sw, strokeExtra)
		}
	case dml.PrimitivePath:
		if prim.Path != nil {
			fmt.Fprintf(b, `<path d="%s" %s stroke="#%s" stroke-width="%.2f"%s fill-rule="%s"/>`, smartArtPathData(prim.Path), fill, stroke, sw, strokeExtra, prim.Path.FillRule)
		}
	}
}

func smartArtPathData(p *dml.Path) string {
	if p == nil {
		return ""
	}
	var b strings.Builder
	for _, c := range p.Commands {
		b.WriteString(c.Type)
		for _, pt := range c.Points {
			fmt.Fprintf(&b, " %.2f %.2f", pt.X, pt.Y)
		}
		if c.Type == "A" {
			for _, a := range c.Args {
				fmt.Fprintf(&b, " %.2f", a)
			}
		}
		b.WriteByte(' ')
	}
	return strings.TrimSpace(b.String())
}

func renderSmartArtText(b *strings.Builder, shape *smartartrender.SmartArtShape, w, h, scale float64) {
	props := shape.TextBodyProps
	// DrawingML default text insets: L/R 91440 EMU, T/B 45720 EMU.
	insetL, insetR := props.InsetL, props.InsetR
	insetT, insetB := props.InsetT, props.InsetB
	if insetL == 0 {
		insetL = 91440
	}
	if insetR == 0 {
		insetR = 91440
	}
	if insetT == 0 {
		insetT = 45720
	}
	if insetB == 0 {
		insetB = 45720
	}
	pxL, pxR := float64(insetL)/12700.0*scale, float64(insetR)/12700.0*scale
	pxT, pxB := float64(insetT)/12700.0*scale, float64(insetB)/12700.0*scale
	contentW, contentH := math.Max(1, w-pxL-pxR), math.Max(1, h-pxT-pxB)

	paras := shape.Paragraphs
	if len(paras) == 0 {
		text := strings.TrimSpace(shape.Text)
		if text == "" {
			text = strings.TrimSpace(shape.PlaceholderText)
		}
		if text == "" {
			return
		}
		color := strings.TrimPrefix(shape.TextColor, "#")
		if color == "" {
			color = "FFFFFF"
		}
		fs := math.Max(10, math.Min(18, 14*scale))
		if props.AutofitType == "normAutofit" && props.AutofitFontScale > 0 {
			fs *= float64(props.AutofitFontScale) / 100000.0
		}
		x, y := pxL+contentW/2, pxT+contentH/2
		fmt.Fprintf(b, `<text x="%.2f" y="%.2f" text-anchor="middle" dominant-baseline="middle" font-family="Segoe UI,Arial,sans-serif" font-size="%.2f" fill="#%s"%s>%s</text>`, x, y, fs, color, smartArtTextTransform(props, x, y), html.EscapeString(text))
		return
	}

	lineMetrics := make([]float64, len(paras))
	total := 0.0
	for i, p := range paras {
		fs := smartArtParagraphFontSize(p)
		if props.AutofitType == "normAutofit" && props.AutofitFontScale > 0 {
			fs *= float64(props.AutofitFontScale) / 100000.0
		}
		lh := fs * 1.20
		if p.LineSpacingPts > 0 {
			lh = float64(p.LineSpacingPts) / 100.0
		}
		if p.LineSpacingPct > 0 {
			lh = fs * float64(p.LineSpacingPct) / 100000.0
		}
		if props.AutofitLineSpaceReduction > 0 {
			lh *= math.Max(.2, 1-float64(props.AutofitLineSpaceReduction)/100000.0)
		}
		before := float64(p.SpaceBefore) / 100.0
		after := float64(p.SpaceAfter) / 100.0
		if p.SpaceBeforePct > 0 {
			before = fs * float64(p.SpaceBeforePct) / 100000.0
		}
		if p.SpaceAfterPct > 0 {
			after = fs * float64(p.SpaceAfterPct) / 100000.0
		}
		lineMetrics[i] = before + lh + after
		total += lineMetrics[i]
	}

	startY := pxT
	switch props.Anchor {
	case "ctr":
		startY = pxT + math.Max(0, (contentH-total)/2)
	case "b":
		startY = h - pxB - total
	}
	columns := props.Columns
	if columns <= 0 {
		columns = 1
	}
	colGap := float64(props.ColumnSpacingEMU) / 12700.0 * scale
	if columns > 1 && colGap <= 0 {
		colGap = 12
	}
	colW := (contentW - float64(columns-1)*colGap) / float64(columns)
	if colW <= 0 {
		colW = contentW
	}

	col, y := 0, startY
	for i, p := range paras {
		if columns > 1 && y+lineMetrics[i] > pxT+contentH && col < columns-1 {
			col++
			y = pxT
		}
		fs := smartArtParagraphFontSize(p)
		if props.AutofitType == "normAutofit" && props.AutofitFontScale > 0 {
			fs *= float64(props.AutofitFontScale) / 100000.0
		}
		before := float64(p.SpaceBefore) / 100.0
		if p.SpaceBeforePct > 0 {
			before = fs * float64(p.SpaceBeforePct) / 100000.0
		}
		y += before
		baseX := pxL + float64(col)*(colW+colGap)
		levelIndent := float64(p.Level) * math.Min(20, colW*.08)
		anchor, x := "middle", baseX+colW/2
		switch p.Align {
		case "l", "left":
			anchor, x = "start", baseX+levelIndent
		case "r", "right":
			anchor, x = "end", baseX+colW
		}
		if props.AnchorCtr {
			anchor, x = "middle", baseX+colW/2
		}
		baseline := y + fs*.92
		transform := smartArtTextTransform(props, baseX+colW/2, pxT+contentH/2)
		fmt.Fprintf(b, `<text x="%.2f" y="%.2f" text-anchor="%s"%s>`, x, baseline, anchor, transform)
		if p.BulletChar != "" || p.AutoNum {
			bullet := p.BulletChar
			if bullet == "" {
				bullet = smartArtAutoNumber(p, i)
			}
			bc := strings.TrimPrefix(p.BulletColor, "#")
			if bc == "" {
				bc = strings.TrimPrefix(shape.TextColor, "#")
			}
			if bc == "" {
				bc = "FFFFFF"
			}
			bf := p.BulletFontFamily
			if bf == "" {
				bf = "Segoe UI"
			}
			bs := fs
			if p.BulletSize > 0 {
				bs = float64(p.BulletSize) / 100.0
			}
			if p.BulletSizePct > 0 {
				bs = fs * float64(p.BulletSizePct) / 100000.0
			}
			fmt.Fprintf(b, `<tspan font-family="%s" font-size="%.2f" fill="#%s">%s </tspan>`, html.EscapeString(bf), bs, bc, html.EscapeString(bullet))
		}
		for _, r := range p.Runs {
			if r.Text == "" {
				continue
			}
			size := float64(r.FontSize) / 100
			if size <= 0 {
				size = fs
			}
			if props.AutofitType == "normAutofit" && props.AutofitFontScale > 0 {
				size *= float64(props.AutofitFontScale) / 100000.0
			}
			family := r.FontFamily
			if family == "" {
				family = "Segoe UI"
			}
			color := strings.TrimPrefix(r.Color, "#")
			if color == "" {
				color = strings.TrimPrefix(shape.TextColor, "#")
			}
			if color == "" {
				color = "FFFFFF"
			}
			fmt.Fprintf(b, `<tspan font-family="%s" font-size="%.2f" fill="#%s"`, html.EscapeString(family), size, color)
			if r.Bold {
				b.WriteString(` font-weight="700"`)
			}
			if r.Italic {
				b.WriteString(` font-style="italic"`)
			}
			if r.Underline {
				b.WriteString(` text-decoration="underline"`)
			}
			b.WriteString(`>` + html.EscapeString(r.Text) + `</tspan>`)
		}
		b.WriteString(`</text>`)
		y += lineMetrics[i] - before
	}
}

func convertDrawingMLShapeToWps(dmlShape *parts.DrawingMLShape) *parts.WpsShape {
	if dmlShape == nil {
		return nil
	}
	wps := &parts.WpsShape{}
	// Copy non-visual properties directly if available
	if dmlShape.NvSpPr != nil {
		if dmlShape.NvSpPr.CNvPr != nil {
			wps.CNvPr = (*parts.DrawingDocPr)(dmlShape.NvSpPr.CNvPr)
		}
		if dmlShape.NvSpPr.CNvSpPr != nil {
			wps.CNvSpPr = dmlShape.NvSpPr.CNvSpPr
		}
	}
	// Shape properties and style are compatible via type conversion
	wps.ShapeProps = (*parts.WpsShapeProperties)(dmlShape.SpPr)
	wps.Style = dmlShape.Style
	return wps
}

// resolveDrawingMLPictureAsShape resolves a DrawingML picture as a shape with picture fill.
func resolveDrawingMLPictureAsShape(ctx *resolveCtx, dmlPic *parts.DrawingMLPicture, isChild bool) *Shape {
	if dmlPic == nil || dmlPic.BlipFill == nil {
		return nil
	}

	shape := &Shape{
		Kind: "rect",
	}

	blip := dmlPic.BlipFill.Blip
	if blip != nil && blip.Embed != "" && ctx.pkg != nil {
		if mp, ok := ctx.pkg.MediaPartFor(ctx.currentPart, blip.Embed); ok {
			pf := dml.PictureFill{
				ImageData:   mp.Data,
				ContentType: mp.ContentType,
			}
			if dmlPic.BlipFill.Tile != nil {
				pf.Tile = true
				pf.TileOffsetX = int64(dmlPic.BlipFill.Tile.Tx)
				pf.TileOffsetY = int64(dmlPic.BlipFill.Tile.Ty)
				pf.TileScaleX = dmlPic.BlipFill.Tile.Sx
				pf.TileScaleY = dmlPic.BlipFill.Tile.Sy
				pf.TileFlip = dmlPic.BlipFill.Tile.Flip
				pf.TileAlign = dmlPic.BlipFill.Tile.Algn
			}
			shape.Fill = pf
			shape.FillColor = "#000000"
		}
	}

	if dmlPic.SpPr != nil {
		if dmlPic.SpPr.Xfrm != nil {
			shape.RotationDeg = drawingRotationDegrees(dmlPic.SpPr.Xfrm.Rot)
			shape.FlipH = dmlPic.SpPr.Xfrm.FlipH
			shape.FlipV = dmlPic.SpPr.Xfrm.FlipV
			if isChild {
				shape.X = dmlPic.SpPr.Xfrm.OffX
				shape.Y = dmlPic.SpPr.Xfrm.OffY
				shape.Width = dmlPic.SpPr.Xfrm.ExtCX
				shape.Height = dmlPic.SpPr.Xfrm.ExtCY
			}
		}
	}

	return shape
}

// convertDrawingMLConnectorToWps converts a DrawingML connector to a WPS shape for resolution.
func convertDrawingMLConnectorToWps(cxn *parts.DrawingMLConnector) *parts.WpsShape {
	if cxn == nil {
		return nil
	}
	return &parts.WpsShape{
		ShapeProps: cxn.SpPr,
		Style:      cxn.Style,
	}
}

// buildShapeFromWps builds a Shape from WPS shape properties.
func buildShapeFromWps(ctx *resolveCtx, wpsShape *parts.WpsShape, isChild bool) *Shape {
	if wpsShape == nil {
		return nil
	}
	props := wpsShape.ShapeProps
	if props != nil && len(props.ParseDiagnostics) > 0 {
		ctx.Diagnostics = append(ctx.Diagnostics, props.ParseDiagnostics...)
	}

	kind := "rect"
	if props != nil && props.Geometry != nil && props.Geometry.Name == "line" {
		kind = "line"
	}

	fillColor := ""
	if props != nil {
		fillColor = resolveDrawingFill(ctx, props.Fill)
	}

	strokeColor := ""
	var strokeWidth int64
	if props != nil && props.Line != nil {
		strokeWidth = int64(props.Line.Width * 12700)
		if props.Line.Color != "" {
			strokeColor = props.Line.Color
		} else if props.Line.ThemeColor != "" {
			if base, ok := ctx.themeColors[props.Line.ThemeColor]; ok {
				base = applyWordColorModifier(base, props.Line.ThemeShade, props.Line.ThemeTint)
				fill := dml.SolidFill{Color: base}
				if props.Line.LumMod > 0 {
					fill.LumMod = props.Line.LumMod
				}
				fill.LumOff = props.Line.LumOff
				strokeColor = fill.ApplyColorTransforms(base)
			}
		}
	}

	hasVisibleFill := false
	if props != nil && props.Fill != nil {
		switch f := props.Fill.(type) {
		case dml.GradientFill:
			hasVisibleFill = len(f.Stops) > 1
		case *dml.GradientFill:
			hasVisibleFill = f != nil && len(f.Stops) > 1
		case dml.PatternFill, *dml.PatternFill:
			hasVisibleFill = true
		case dml.PictureFill:
			hasVisibleFill = f.SourceURL() != ""
		case *dml.PictureFill:
			hasVisibleFill = f != nil && f.SourceURL() != ""
		}
	}
	if kind == "rect" && fillColor == "" && !hasVisibleFill && strokeColor == "" {
		return nil
	}

	styleFill, styleLine, styleEffects := resolveShapeStyle(ctx, wpsShape.Style)

	shape := &Shape{
		Kind:        kind,
		StrokeColor: strokeColor,
		StrokeWidth: strokeWidth,
		FillColor:   fillColor,
	}
	if props != nil && props.Xfrm != nil {
		shape.RotationDeg = drawingRotationDegrees(props.Xfrm.Rot)
		shape.FlipH = props.Xfrm.FlipH
		shape.FlipV = props.Xfrm.FlipV
		if isChild {
			shape.X = props.Xfrm.OffX
			shape.Y = props.Xfrm.OffY
			shape.Width = props.Xfrm.ExtCX
			shape.Height = props.Xfrm.ExtCY
		}
	}
	if props != nil {
		shape.GeometryData = props.GeometryData
		shape.Geometry = props.Geometry
		shape.Fill = props.Fill
		if pf, ok := shape.Fill.(dml.PictureFill); ok && pf.EmbedID != "" && ctx.pkg != nil {
			if mp, ok := ctx.pkg.MediaPartFor(ctx.currentPart, pf.EmbedID); ok {
				pf.ImageData = mp.Data
				pf.ContentType = mp.ContentType
				shape.Fill = pf
			}
		}
		shape.LineStyle = props.Line
		shape.Effects = props.DrawingEffectList(ctx.themeColors)
	}
	if shape.Fill == nil {
		shape.Fill = styleFill
		shape.FillColor = resolveDrawingFill(ctx, styleFill)
	}
	if shape.LineStyle == nil {
		shape.LineStyle = styleLine
		if styleLine != nil {
			shape.StrokeWidth = int64(styleLine.Width * 12700)
			shape.StrokeColor = styleLine.Color
		}
	}
	if len(shape.Effects.Effects) == 0 {
		shape.Effects = styleEffects
	}
	return shape
}

// applyShapePosition applies anchor/inline positioning to a shape.
func applyShapePosition(shape *Shape, anchor *parts.DrawingAnchor, inline *parts.DrawingInline) {
	if shape == nil {
		return
	}
	if anchor != nil {
		shape.Floating = true
		shape.X, shape.Y = drawingAnchorOffset(anchor)
		shape.Width, shape.Height = drawingExtentSize(anchor.Extent)
		if anchor.PositionH != nil {
			shape.XRelative = anchor.PositionH.RelativeFrom
			shape.XAlign = anchor.PositionH.Align
			if anchor.PositionH.Pct != 0 {
				shape.X = anchor.PositionH.Pct
			}
		}
		if anchor.PositionV != nil {
			shape.YRelative = anchor.PositionV.RelativeFrom
			shape.YAlign = anchor.PositionV.Align
			if anchor.PositionV.Pct != 0 {
				shape.Y = anchor.PositionV.Pct
			}
		}
		if anchor.SimplePosAttr && anchor.SimplePos != nil {
			shape.SimplePosAttr = true
			shape.SimplePosX = anchor.SimplePos.X
			shape.SimplePosY = anchor.SimplePos.Y
		}
		if anchor.EffectExtent != nil {
			shape.EffectExtentL = anchor.EffectExtent.L
			shape.EffectExtentT = anchor.EffectExtent.T
			shape.EffectExtentR = anchor.EffectExtent.R
			shape.EffectExtentB = anchor.EffectExtent.B
		}
		shape.Locked = anchor.Locked
		shape.LayoutInCell = anchor.LayoutInCell
		shape.AllowOverlap = anchor.AllowOverlap
		if anchor.SizeRelH != nil {
			shape.SizeRelHPct = anchor.SizeRelH.Pct
			shape.SizeRelHFrom = anchor.SizeRelH.RelativeFrom
		}
		if anchor.SizeRelV != nil {
			shape.SizeRelVPct = anchor.SizeRelV.Pct
			shape.SizeRelVFrom = anchor.SizeRelV.RelativeFrom
		}
		shape.ZIndex = anchor.RelativeHeight
		shape.BehindDoc = anchor.BehindDoc != 0
		shape.WrapType, shape.WrapSide = drawingAnchorWrap(anchor)
		shape.WrapPolygon = drawingAnchorPolygon(anchor)
		dt, db, dl, dr := drawingAnchorWrapDistances(anchor)
		shape.DistTop = emuToPoints(dt)
		shape.DistBottom = emuToPoints(db)
		shape.DistLeft = emuToPoints(dl)
		shape.DistRight = emuToPoints(dr)
	} else if inline != nil {
		shape.Width, shape.Height = drawingExtentSize(inline.Extent)
	}
}

// groupTransform holds the scale and offset needed to map child coordinates to document space.
type groupTransform struct {
	ScaleX float64
	ScaleY float64
	DocX   int64
	DocY   int64
	ChOffX int64
	ChOffY int64
}

// applyGroupTransform maps a child shape's coordinates from a group's local child
// coordinate space into document space using the provided transform.
func applyGroupTransform(child *Shape, gt groupTransform) {
	if child == nil {
		return
	}
	child.X = gt.DocX + int64(float64(child.X-gt.ChOffX)*gt.ScaleX)
	child.Y = gt.DocY + int64(float64(child.Y-gt.ChOffY)*gt.ScaleY)
	child.Width = int64(float64(child.Width) * gt.ScaleX)
	child.Height = int64(float64(child.Height) * gt.ScaleY)
}

// computeGroupTransform calculates the transform from a group's xfrm to document space.
func computeGroupTransform(xfrm *parts.DrawingTransform2D, anchor *parts.DrawingAnchor, inline *parts.DrawingInline, parentScaleX, parentScaleY float64, parentDocX, parentDocY int64) (groupTransform, int64, int64, int64, int64) {
	var gt groupTransform
	var offX, offY, extCX, extCY int64
	if xfrm != nil {
		offX = xfrm.OffX
		offY = xfrm.OffY
		extCX = xfrm.ExtCX
		extCY = xfrm.ExtCY
		gt.ChOffX = xfrm.ChOffX
		gt.ChOffY = xfrm.ChOffY
		if xfrm.ChExtCX != 0 && xfrm.ChExtCY != 0 {
			gt.ScaleX = float64(xfrm.ExtCX) / float64(xfrm.ChExtCX)
			gt.ScaleY = float64(xfrm.ExtCY) / float64(xfrm.ChExtCY)
		} else {
			gt.ScaleX = 1.0
			gt.ScaleY = 1.0
		}
	} else {
		gt.ScaleX = 1.0
		gt.ScaleY = 1.0
	}
	gt.ScaleX *= parentScaleX
	gt.ScaleY *= parentScaleY
	gt.DocX = parentDocX + int64(float64(offX)*parentScaleX)
	gt.DocY = parentDocY + int64(float64(offY)*parentScaleY)
	return gt, offX, offY, extCX, extCY
}

// resolveDrawingMLGroupShape resolves a DrawingML group shape into a Shape with Children.
func resolveDrawingMLGroupShape(ctx *resolveCtx, group *parts.DrawingMLGroupShape, anchor *parts.DrawingAnchor, inline *parts.DrawingInline) *Shape {
	return resolveDrawingMLGroupShapeWithTransform(ctx, group, anchor, inline, 1.0, 1.0, 0, 0)
}

func resolveDrawingMLGroupShapeWithTransform(ctx *resolveCtx, group *parts.DrawingMLGroupShape, anchor *parts.DrawingAnchor, inline *parts.DrawingInline, parentScaleX, parentScaleY float64, parentDocX, parentDocY int64) *Shape {
	if group == nil {
		return nil
	}
	shape := &Shape{Kind: "group"}
	applyShapePosition(shape, anchor, inline)

	var xfrm *parts.DrawingTransform2D
	if group.GrpSpPr != nil {
		xfrm = group.GrpSpPr.Xfrm
	}

	gt, _, _, extCX, extCY := computeGroupTransform(xfrm, anchor, inline, parentScaleX, parentScaleY, parentDocX, parentDocY)
	if xfrm != nil {
		shape.RotationDeg = drawingRotationDegrees(xfrm.Rot)
		shape.FlipH = xfrm.FlipH
		shape.FlipV = xfrm.FlipV
	}

	// For child groups (no anchor/inline), set bounds from the computed document transform.
	if anchor == nil && inline == nil {
		shape.X = gt.DocX
		shape.Y = gt.DocY
		shape.Width = int64(float64(extCX) * parentScaleX)
		shape.Height = int64(float64(extCY) * parentScaleY)
	}

	for _, child := range group.Children {
		var childShape *Shape
		switch {
		case child.Shape != nil:
			wps := convertDrawingMLShapeToWps(child.Shape)
			childShape = buildShapeFromWps(ctx, wps, true)
			if childShape != nil && child.Shape.TxBody != nil {
				childShape.TextBox = buildTextBoxFromDrawingMLShape(ctx, child.Shape)
			}
		case child.Picture != nil:
			childShape = resolveDrawingMLPictureAsShape(ctx, child.Picture, true)
		case child.GroupShape != nil:
			// Pre-adjust the parent origin so the nested group resolver's
			//   nestedDocX = parentDocX + nested.offX * parentScaleX
			// yields the correct document coordinate after accounting for chOff.
			adjustedDocX := gt.DocX - int64(float64(gt.ChOffX)*gt.ScaleX)
			adjustedDocY := gt.DocY - int64(float64(gt.ChOffY)*gt.ScaleY)
			childShape = resolveDrawingMLGroupShapeWithTransform(ctx, child.GroupShape, nil, nil, gt.ScaleX, gt.ScaleY, adjustedDocX, adjustedDocY)
		case child.CxnSp != nil:
			wps := convertDrawingMLConnectorToWps(child.CxnSp)
			childShape = buildShapeFromWps(ctx, wps, true)
		}
		if childShape != nil {
			if child.GroupShape == nil {
				applyGroupTransform(childShape, gt)
			}
			// Nested groups are resolved recursively with the correct accumulated
			// transform, so their coordinates are already in document space.
			shape.Children = append(shape.Children, *childShape)
		}
	}
	return shape
}

// resolveWpsGroupShape resolves a WPS group shape into a Shape with Children.
func resolveWpsGroupShape(ctx *resolveCtx, group *parts.WpsGroupShape, anchor *parts.DrawingAnchor, inline *parts.DrawingInline) *Shape {
	if group == nil {
		return nil
	}
	shape := &Shape{Kind: "group"}
	applyShapePosition(shape, anchor, inline)

	var xfrm *parts.DrawingTransform2D
	if group.GrpSpPr != nil {
		xfrm = group.GrpSpPr.Xfrm
	}

	gt, _, _, extCX, extCY := computeGroupTransform(xfrm, anchor, inline, 1.0, 1.0, 0, 0)
	if xfrm != nil {
		shape.RotationDeg = drawingRotationDegrees(xfrm.Rot)
		shape.FlipH = xfrm.FlipH
		shape.FlipV = xfrm.FlipV
	}

	if anchor == nil && inline == nil {
		shape.X = gt.DocX
		shape.Y = gt.DocY
		shape.Width = extCX
		shape.Height = extCY
	}

	if group.Shape != nil && group.Shape.TextBox == nil {
		if child := buildShapeFromWps(ctx, group.Shape, true); child != nil {
			applyGroupTransform(child, gt)
			shape.Children = append(shape.Children, *child)
		}
	}
	if group.GroupShape != nil {
		adjustedDocX := gt.DocX - int64(float64(gt.ChOffX)*gt.ScaleX)
		adjustedDocY := gt.DocY - int64(float64(gt.ChOffY)*gt.ScaleY)
		if child := resolveWpsGroupShapeWithTransform(ctx, group.GroupShape, nil, nil, gt.ScaleX, gt.ScaleY, adjustedDocX, adjustedDocY); child != nil {
			shape.Children = append(shape.Children, *child)
		}
	}
	return shape
}

func resolveWpsGroupShapeWithTransform(ctx *resolveCtx, group *parts.WpsGroupShape, anchor *parts.DrawingAnchor, inline *parts.DrawingInline, parentScaleX, parentScaleY float64, parentDocX, parentDocY int64) *Shape {
	if group == nil {
		return nil
	}
	shape := &Shape{Kind: "group"}
	applyShapePosition(shape, anchor, inline)

	var xfrm *parts.DrawingTransform2D
	if group.GrpSpPr != nil {
		xfrm = group.GrpSpPr.Xfrm
	}

	gt, _, _, extCX, extCY := computeGroupTransform(xfrm, anchor, inline, parentScaleX, parentScaleY, parentDocX, parentDocY)
	if xfrm != nil {
		shape.RotationDeg = drawingRotationDegrees(xfrm.Rot)
		shape.FlipH = xfrm.FlipH
		shape.FlipV = xfrm.FlipV
	}

	if anchor == nil && inline == nil {
		shape.X = gt.DocX
		shape.Y = gt.DocY
		shape.Width = int64(float64(extCX) * parentScaleX)
		shape.Height = int64(float64(extCY) * parentScaleY)
	}

	if group.Shape != nil && group.Shape.TextBox == nil {
		if child := buildShapeFromWps(ctx, group.Shape, true); child != nil {
			applyGroupTransform(child, gt)
			shape.Children = append(shape.Children, *child)
		}
	}
	if group.GroupShape != nil {
		adjustedDocX := gt.DocX - int64(float64(gt.ChOffX)*gt.ScaleX)
		adjustedDocY := gt.DocY - int64(float64(gt.ChOffY)*gt.ScaleY)
		if child := resolveWpsGroupShapeWithTransform(ctx, group.GroupShape, nil, nil, gt.ScaleX, gt.ScaleY, adjustedDocX, adjustedDocY); child != nil {
			applyGroupTransform(child, gt)
			shape.Children = append(shape.Children, *child)
		}
	}
	return shape
}

// resolveWpsWordprocessingGroup resolves a WPS wordprocessing group into a Shape with Children.
func resolveWpsWordprocessingGroup(ctx *resolveCtx, group *parts.WpsWordprocessingGroup, anchor *parts.DrawingAnchor, inline *parts.DrawingInline) *Shape {
	if group == nil {
		return nil
	}
	shape := &Shape{Kind: "group"}
	applyShapePosition(shape, anchor, inline)

	var xfrm *parts.DrawingTransform2D
	if group.GrpSpPr != nil {
		xfrm = group.GrpSpPr.Xfrm
	}

	gt, _, _, extCX, extCY := computeGroupTransform(xfrm, anchor, inline, 1.0, 1.0, 0, 0)
	if xfrm != nil {
		shape.RotationDeg = drawingRotationDegrees(xfrm.Rot)
		shape.FlipH = xfrm.FlipH
		shape.FlipV = xfrm.FlipV
	}

	if anchor == nil && inline == nil {
		shape.X = gt.DocX
		shape.Y = gt.DocY
		shape.Width = extCX
		shape.Height = extCY
	}

	if group.Shape != nil && group.Shape.TextBox == nil {
		if child := buildShapeFromWps(ctx, group.Shape, true); child != nil {
			applyGroupTransform(child, gt)
			shape.Children = append(shape.Children, *child)
		}
	}
	if group.GroupShape != nil {
		adjustedDocX := gt.DocX - int64(float64(gt.ChOffX)*gt.ScaleX)
		adjustedDocY := gt.DocY - int64(float64(gt.ChOffY)*gt.ScaleY)
		if child := resolveWpsWordprocessingGroupWithTransform(ctx, group.GroupShape, nil, nil, gt.ScaleX, gt.ScaleY, adjustedDocX, adjustedDocY); child != nil {
			shape.Children = append(shape.Children, *child)
		}
	}
	return shape
}

func resolveWpsWordprocessingGroupWithTransform(ctx *resolveCtx, group *parts.WpsWordprocessingGroup, anchor *parts.DrawingAnchor, inline *parts.DrawingInline, parentScaleX, parentScaleY float64, parentDocX, parentDocY int64) *Shape {
	if group == nil {
		return nil
	}
	shape := &Shape{Kind: "group"}
	applyShapePosition(shape, anchor, inline)

	var xfrm *parts.DrawingTransform2D
	if group.GrpSpPr != nil {
		xfrm = group.GrpSpPr.Xfrm
	}

	gt, _, _, extCX, extCY := computeGroupTransform(xfrm, anchor, inline, parentScaleX, parentScaleY, parentDocX, parentDocY)
	if xfrm != nil {
		shape.RotationDeg = drawingRotationDegrees(xfrm.Rot)
		shape.FlipH = xfrm.FlipH
		shape.FlipV = xfrm.FlipV
	}

	if anchor == nil && inline == nil {
		shape.X = gt.DocX
		shape.Y = gt.DocY
		shape.Width = int64(float64(extCX) * parentScaleX)
		shape.Height = int64(float64(extCY) * parentScaleY)
	}

	if group.Shape != nil && group.Shape.TextBox == nil {
		if child := buildShapeFromWps(ctx, group.Shape, true); child != nil {
			applyGroupTransform(child, gt)
			shape.Children = append(shape.Children, *child)
		}
	}
	if group.GroupShape != nil {
		adjustedDocX := gt.DocX - int64(float64(gt.ChOffX)*gt.ScaleX)
		adjustedDocY := gt.DocY - int64(float64(gt.ChOffY)*gt.ScaleY)
		if child := resolveWpsWordprocessingGroupWithTransform(ctx, group.GroupShape, nil, nil, gt.ScaleX, gt.ScaleY, adjustedDocX, adjustedDocY); child != nil {
			applyGroupTransform(child, gt)
			shape.Children = append(shape.Children, *child)
		}
	}
	return shape
}

// resolveShape extracts a floating shape (line or filled rectangle) from a drawing.
// Returns nil only when the drawing has no usable shape geometry.
func resolveShape(ctx *resolveCtx, drawing *parts.Drawing) *Shape {
	if drawing == nil {
		return nil
	}

	var anchor *parts.DrawingAnchor
	var inline *parts.DrawingInline
	if drawing.Anchor != nil {
		anchor = drawing.Anchor
	} else if drawing.Inline != nil {
		inline = drawing.Inline
	}

	var graphic *parts.Graphic
	if anchor != nil {
		graphic = anchor.Graphic
	} else if inline != nil {
		graphic = inline.Graphic
	}
	if graphic == nil || graphic.GraphicData == nil {
		return nil
	}

	var shape *Shape
	if graphic.GraphicData.Shape != nil && graphic.GraphicData.Shape.TextBox == nil {
		shape = buildShapeFromWps(ctx, graphic.GraphicData.Shape, false)
	} else if graphic.GraphicData.DrawingMLShape != nil {
		wps := convertDrawingMLShapeToWps(graphic.GraphicData.DrawingMLShape)
		shape = buildShapeFromWps(ctx, wps, false)
	} else if graphic.GraphicData.DrawingMLPic != nil {
		shape = resolveDrawingMLPictureAsShape(ctx, graphic.GraphicData.DrawingMLPic, false)
	} else if graphic.GraphicData.DrawingMLGroupShape != nil {
		shape = resolveDrawingMLGroupShape(ctx, graphic.GraphicData.DrawingMLGroupShape, anchor, inline)
	} else if graphic.GraphicData.GroupShape != nil {
		shape = resolveWpsGroupShape(ctx, graphic.GraphicData.GroupShape, anchor, inline)
	} else if graphic.GraphicData.WordprocessingGroup != nil {
		shape = resolveWpsWordprocessingGroup(ctx, graphic.GraphicData.WordprocessingGroup, anchor, inline)
	}

	if shape != nil && shape.Kind != "group" {
		applyShapePosition(shape, anchor, inline)
	}
	if shape != nil {
		syncWordOfficeArtShape(shape)
	}
	return shape
}

func syncWordOfficeArtShape(shape *Shape) *officeartir.Shape {
	if shape == nil {
		return nil
	}
	spec := officeartdml.ShapeSpec{
		Kind:        wordOfficeArtKind(shape.Kind, shape),
		Bounds:      officeartir.Rect{X: shape.X, Y: shape.Y, W: shape.Width, H: shape.Height},
		RotationDeg: shape.RotationDeg,
		FlipH:       shape.FlipH,
		FlipV:       shape.FlipV,
		Geometry:    shape.GeometryData,
		Preset:      shape.Geometry,
		Fill:        shape.Fill,
		Line:        shape.LineStyle,
		Effects:     shape.Effects,
		Source:      officeartir.SourceRef{Format: "docx"},
	}
	shape.OfficeArt = officeartdml.Shape(spec)
	if len(shape.Children) > 0 {
		shape.OfficeArt.Children = shape.OfficeArt.Children[:0]
		for i := range shape.Children {
			child := &shape.Children[i]
			if composed := syncWordOfficeArtShape(child); composed != nil {
				shape.OfficeArt.Children = append(shape.OfficeArt.Children, composed)
			}
		}
		if len(shape.OfficeArt.Children) > 0 {
			shape.OfficeArt.Kind = officeartir.ShapeGroup
		}
	}
	return shape.OfficeArt
}

func wordOfficeArtKind(kind string, shape *Shape) officeartir.ShapeKind {
	switch kind {
	case "group":
		return officeartir.ShapeGroup
	case "line", "connector":
		return officeartir.ShapeConnector
	case "picture", "image":
		return officeartir.ShapePicture
	case "textbox":
		return officeartir.ShapeTextBox
	}
	if shape != nil && shape.TextBox != nil {
		return officeartir.ShapeTextBox
	}
	return officeartir.ShapeAuto
}

// resolveDrawingFill resolves a drawing.Fill to a hex RGB string.
// Returns "" if the fill is nil or cannot be resolved.
func resolveDrawingFill(ctx *resolveCtx, fill dml.Fill) string {
	if fill == nil {
		return ""
	}
	switch f := fill.(type) {
	case dml.SolidFill:
		return resolveDrawingSolidFill(ctx, f)
	case *dml.SolidFill:
		return resolveDrawingSolidFill(ctx, *f)
	case dml.NoFill:
		return ""
	case *dml.NoFill:
		return ""
	case dml.GradientFill:
		if len(f.Stops) > 0 {
			return f.Stops[0].Color
		}
		return ""
	case *dml.GradientFill:
		if len(f.Stops) > 0 {
			return f.Stops[0].Color
		}
		return ""
	default:
		return ""
	}
}

// resolveDrawingSolidFill resolves a drawing.SolidFill to a hex RGB string.
func resolveDrawingSolidFill(ctx *resolveCtx, fill dml.SolidFill) string {
	if fill.Color != "" {
		return fill.ApplyColorTransforms(fill.Color)
	}
	if fill.ThemeColor != "" {
		base := ctx.themeColors[fill.ThemeColor]
		if base == "" {
			return ""
		}
		base = applyWordColorModifier(base, fill.ThemeShade, fill.ThemeTint)
		return fill.ApplyColorTransforms(base)
	}
	return ""
}

func applyDrawingColorTransforms(fill *dml.SolidFill, alpha, alphaMod, alphaOff, shade, tint, red, redMod, redOff, green, greenMod, greenOff, blue, blueMod, blueOff, hue, sat, lum, lumMod, lumOff, satMod, satOff, hueMod, hueOff *parts.DrawingColorTransform, gray, comp, inv, gamma, invGamma bool) {
	if fill == nil {
		return
	}
	if alpha != nil {
		fill.Alpha = float64(alpha.Val) / 100000.0
	}
	if alphaMod != nil {
		fill.AlphaMod = float64(alphaMod.Val) / 100000.0
	}
	if alphaOff != nil {
		fill.AlphaOff = float64(alphaOff.Val) / 100000.0
	}
	if shade != nil {
		fill.Shade = float64(shade.Val) / 100000.0
		fill.HasShade = true
	}
	if tint != nil {
		fill.Tint = float64(tint.Val) / 100000.0
		fill.HasTint = true
	}
	if red != nil {
		fill.Red = float64(red.Val) / 100000.0
		fill.HasRed = true
	}
	if redMod != nil {
		fill.RedMod = float64(redMod.Val) / 100000.0
		fill.HasRedMod = true
	}
	if redOff != nil {
		fill.RedOff = float64(redOff.Val) / 100000.0
		fill.HasRedOff = true
	}
	if green != nil {
		fill.Green = float64(green.Val) / 100000.0
		fill.HasGreen = true
	}
	if greenMod != nil {
		fill.GreenMod = float64(greenMod.Val) / 100000.0
		fill.HasGreenMod = true
	}
	if greenOff != nil {
		fill.GreenOff = float64(greenOff.Val) / 100000.0
		fill.HasGreenOff = true
	}
	if blue != nil {
		fill.Blue = float64(blue.Val) / 100000.0
		fill.HasBlue = true
	}
	if blueMod != nil {
		fill.BlueMod = float64(blueMod.Val) / 100000.0
		fill.HasBlueMod = true
	}
	if blueOff != nil {
		fill.BlueOff = float64(blueOff.Val) / 100000.0
		fill.HasBlueOff = true
	}
	if hue != nil {
		fill.Hue = float64(hue.Val) / 60000.0
		fill.HasHue = true
	}
	if sat != nil {
		fill.Sat = float64(sat.Val) / 100000.0
		fill.HasSat = true
	}
	if lum != nil {
		fill.Lum = float64(lum.Val) / 100000.0
		fill.HasLum = true
	}
	if lumMod != nil {
		fill.LumMod = float64(lumMod.Val) / 100000.0
	}
	if lumOff != nil {
		fill.LumOff = float64(lumOff.Val) / 100000.0
	}
	if satMod != nil {
		fill.SatMod = float64(satMod.Val) / 100000.0
	}
	if satOff != nil {
		fill.SatOff = float64(satOff.Val) / 100000.0
	}
	if hueMod != nil {
		fill.HueMod = float64(hueMod.Val) / 100000.0
	}
	if hueOff != nil {
		fill.HueOff = float64(hueOff.Val) / 60000.0
	}
	fill.Gray = gray
	fill.Comp = comp
	fill.Inv = inv
	fill.Gamma = gamma
	fill.InvGamma = invGamma
}

// resolveBodyPrInsets returns the text-box internal padding in points from a
// wps:bodyPr element. Absent attributes (EmuAbsent) use the OOXML defaults:
// left/right = 91440 EMU (~7.2 pt), top/bottom = 45720 EMU (~3.6 pt).
// A nil bodyPr (element missing entirely) also uses the defaults.
func resolveBodyPrInsets(b *parts.WpsBodyProperties) (left, top, right, bottom float64) {
	const emuToPoints = 1.0 / 12700.0
	const defLR = 91440 * emuToPoints // 7.2pt
	const defTB = 45720 * emuToPoints // 3.6pt

	resolve := func(v, def float64) float64 {
		if v < 0 { // EmuAbsent sentinel
			return def
		}
		return v * emuToPoints
	}

	if b == nil {
		return defLR, defTB, defLR, defTB
	}
	return resolve(float64(b.LIns), defLR),
		resolve(float64(b.TIns), defTB),
		resolve(float64(b.RIns), defLR),
		resolve(float64(b.BIns), defTB)
}

// emuToPoints converts EMU (English Metric Units) to points. 1 pt = 12700 EMU.
func emuToPoints(emu int64) float64 {
	return float64(emu) / 12700.0
}

func drawingAnchorOffset(anchor *parts.DrawingAnchor) (int64, int64) {
	if anchor == nil {
		return 0, 0
	}
	var x, y int64
	if anchor.PositionH != nil {
		x = anchor.PositionH.PosOffset
	}
	if anchor.PositionV != nil {
		y = anchor.PositionV.PosOffset
	}
	return x, y
}

func drawingExtentSize(extent *parts.DrawingExtent) (int64, int64) {
	if extent == nil {
		return 0, 0
	}
	return extent.CX, extent.CY
}

// drawingAnchorWrap returns the WrapType and WrapSide encoded in a DrawingAnchor.
func drawingAnchorWrap(anchor *parts.DrawingAnchor) (wrapType, wrapSide string) {
	if anchor == nil {
		return "", ""
	}
	switch {
	case anchor.WrapNone != nil:
		return "none", ""
	case anchor.WrapSquare != nil:
		return "square", anchor.WrapSquare.WrapText
	case anchor.WrapTight != nil:
		return "tight", anchor.WrapTight.WrapText
	case anchor.WrapThrough != nil:
		return "through", anchor.WrapThrough.WrapText
	case anchor.WrapTopAndBottom != nil:
		return "topAndBottom", ""
	}
	return "", ""
}

// drawingAnchorWrapDistances returns the effective wrap distances for an anchor,
// preferring per-wrap-mode overrides over the anchor-level defaults.
func drawingAnchorWrapDistances(anchor *parts.DrawingAnchor) (distT, distB, distL, distR int64) {
	if anchor == nil {
		return 0, 0, 0, 0
	}
	switch {
	case anchor.WrapSquare != nil:
		return firstNonZero(anchor.WrapSquare.DistT, anchor.DistT),
			firstNonZero(anchor.WrapSquare.DistB, anchor.DistB),
			firstNonZero(anchor.WrapSquare.DistL, anchor.DistL),
			firstNonZero(anchor.WrapSquare.DistR, anchor.DistR)
	case anchor.WrapTight != nil:
		return firstNonZero(anchor.WrapTight.DistT, anchor.DistT),
			firstNonZero(anchor.WrapTight.DistB, anchor.DistB),
			firstNonZero(anchor.WrapTight.DistL, anchor.DistL),
			firstNonZero(anchor.WrapTight.DistR, anchor.DistR)
	case anchor.WrapThrough != nil:
		return firstNonZero(anchor.WrapThrough.DistT, anchor.DistT),
			firstNonZero(anchor.WrapThrough.DistB, anchor.DistB),
			firstNonZero(anchor.WrapThrough.DistL, anchor.DistL),
			firstNonZero(anchor.WrapThrough.DistR, anchor.DistR)
	case anchor.WrapTopAndBottom != nil:
		return firstNonZero(anchor.WrapTopAndBottom.DistT, anchor.DistT),
			firstNonZero(anchor.WrapTopAndBottom.DistB, anchor.DistB),
			anchor.DistL, anchor.DistR
	}
	return anchor.DistT, anchor.DistB, anchor.DistL, anchor.DistR
}

func firstNonZero(a, b int64) int64 {
	if a != 0 {
		return a
	}
	return b
}

// drawingAnchorPolygon converts a wp:wrapPolygon to a slice of object-relative
// point coordinates in points. Returns nil if no polygon is present.
func drawingAnchorPolygon(anchor *parts.DrawingAnchor) [][2]float64 {
	if anchor == nil {
		return nil
	}
	var poly *parts.WrapPolygon
	if anchor.WrapTight != nil && anchor.WrapTight.Polygon != nil {
		poly = anchor.WrapTight.Polygon
	} else if anchor.WrapThrough != nil && anchor.WrapThrough.Polygon != nil {
		poly = anchor.WrapThrough.Polygon
	}
	if poly == nil {
		return nil
	}
	pts := make([][2]float64, 0, 1+len(poly.LineTo))
	pts = append(pts, [2]float64{emuToPoints(poly.Start.X), emuToPoints(poly.Start.Y)})
	for _, pt := range poly.LineTo {
		pts = append(pts, [2]float64{emuToPoints(pt.X), emuToPoints(pt.Y)})
	}
	return pts
}

func buildNumberingIndex(numbering *parts.Numbering, themeFonts map[string]string, themeColors map[string]string) map[int]map[int]Numbering {
	if numbering == nil {
		return nil
	}

	abstractByID := make(map[int]*parts.AbstractNum, len(numbering.AbstractNums))
	for _, abstract := range numbering.AbstractNums {
		if abstract == nil {
			continue
		}
		abstractByID[abstract.ID] = abstract
	}

	index := make(map[int]map[int]Numbering, len(numbering.Nums))
	for _, num := range numbering.Nums {
		if num == nil || num.AbstractNumID == nil {
			continue
		}

		abstract := abstractByID[num.AbstractNumID.Val]
		if abstract == nil {
			continue
		}

		levels := make(map[int]Numbering, len(abstract.Levels))
		for _, level := range abstract.Levels {
			if level == nil {
				continue
			}

			entry := Numbering{
				NumID:         num.ID,
				Level:         level.Level,
				AbstractNumID: abstract.ID,
			}
			if level.Start != nil {
				entry.Start = level.Start.Val
			}
			if level.Format != nil {
				entry.Format = level.Format.Val
			}
			if level.Text != nil {
				entry.Text = level.Text.Val
			}
			if level.ParaProps != nil && level.ParaProps.Ind != nil {
				entry.IndentLeft = level.ParaProps.Ind.Left
				entry.IndentHanging = level.ParaProps.Ind.Hanging
				entry.IndentFirstLine = level.ParaProps.Ind.FirstLine
			}
			if level.RunProps != nil {
				rf := runFormatFromPropertiesWithTheme(level.RunProps, themeFonts, themeColors)
				entry.RunFormat = &rf
			}
			levels[level.Level] = entry
		}

		// Apply w:lvlOverride: per-num level start and format overrides (ECMA-376 §17.9.8).
		for _, override := range num.Overrides {
			if override == nil {
				continue
			}
			lvl, ok := levels[override.Level]
			if !ok {
				continue
			}
			if override.Start != nil {
				lvl.Start = override.Start.Val
			}
			if override.Lvl != nil {
				ol := override.Lvl
				if ol.Format != nil {
					lvl.Format = ol.Format.Val
				}
				if ol.Text != nil {
					lvl.Text = ol.Text.Val
				}
				if ol.Start != nil {
					lvl.Start = ol.Start.Val
				}
				if ol.ParaProps != nil && ol.ParaProps.Ind != nil {
					lvl.IndentLeft = ol.ParaProps.Ind.Left
					lvl.IndentHanging = ol.ParaProps.Ind.Hanging
					lvl.IndentFirstLine = ol.ParaProps.Ind.FirstLine
				}
				if ol.RunProps != nil {
					rf := runFormatFromPropertiesWithTheme(ol.RunProps, themeFonts, themeColors)
					lvl.RunFormat = &rf
				}
			}
			levels[override.Level] = lvl
		}

		index[num.ID] = levels
	}

	return index
}

func resolveNumbering(index map[int]map[int]Numbering, numID, level int) (Numbering, bool) {
	if index == nil {
		return Numbering{}, false
	}

	levels, ok := index[numID]
	if !ok {
		return Numbering{}, false
	}

	numbering, ok := levels[level]
	return numbering, ok
}

func buildListMarker(numbering Numbering) *ListMarker {
	level := numbering.Level

	// When numPr is inherited from a paragraph style (not the paragraph itself),
	// all items get ilvl=0. If the abstractNum level definition carries an indent,
	// use it to derive the display level: indent is typically 360 twips per level
	// (ListBullet→360, ListBullet2→720, ListBullet3→1080, etc.).
	if numbering.FromStyle && level == 0 && numbering.IndentLeft > 360 {
		if derived := numbering.IndentLeft/360 - 1; derived > 0 {
			level = derived
		}
	}

	marker := &ListMarker{
		Kind:     classifyListKind(numbering.Format, numbering.Text),
		Level:    level,
		Start:    numbering.Start,
		Format:   numbering.Format,
		Template: numbering.Text,
	}
	if marker.Start == 0 {
		marker.Start = 1
	}
	return marker
}

func classifyListKind(format, template string) string {
	switch format {
	case "bullet":
		return "bullet"
	case "decimal", "upperRoman", "lowerRoman", "upperLetter", "lowerLetter", "ordinal", "cardinalText", "ordinalText", "hex":
		return "ordered"
	}
	if template != "" && template[0] == '%' {
		return "ordered"
	}
	return "bullet"
}

func buildStyleIndex(styles *parts.Styles, themeFonts map[string]string, themeColors map[string]string) (map[string]Style, string) {
	index := make(map[string]Style)
	if styles == nil {
		return index, ""
	}

	defaultParagraphStyleID := ""
	for _, style := range styles.Styles {
		if style == nil || style.StyleID == "" {
			continue
		}
		// Index table styles separately for tblStylePr lookup; skip other non-paragraph types.
		if style.Type == "table" {
			entry := Style{ID: style.StyleID}
			if style.Name != nil {
				entry.Name = style.Name.Val
			}
			if style.BasedOn != nil {
				entry.BasedOn = style.BasedOn.Val
			}
			if style.Next != nil {
				entry.NextStyleID = style.Next.Val
			}
			if style.UIPriority != nil {
				entry.UIPriority = style.UIPriority.Val
			}
			entry.RSID = style.RSID
			entry.UnhideWhenUsed = onOffBool(style.Unhide)
			entry.QFormat = onOffBool(style.QuickFormat)
			entry.SemiHidden = onOffBool(style.SemiHidden)
			entry.AutoRedefine = onOffBool(style.AutoRedef)
			if style.TableProps != nil {
				tf := tableFormatFromProperties(style.TableProps, themeColors)
				entry.TableFormat = &tf
			}
			entry.TableStyles = resolveConditionalTableStyles(style.TableStyles, themeColors)
			index[entry.ID] = entry
			continue
		}
		if style.Type != "paragraph" {
			continue
		}

		entry := Style{
			ID: style.StyleID,
		}
		if style.Name != nil {
			entry.Name = style.Name.Val
		}
		if style.BasedOn != nil {
			entry.BasedOn = style.BasedOn.Val
		}
		if style.Link != nil {
			entry.LinkedCharID = style.Link.Val
		}
		if style.Next != nil {
			entry.NextStyleID = style.Next.Val
		}
		if style.UIPriority != nil {
			entry.UIPriority = style.UIPriority.Val
		}
		entry.RSID = style.RSID
		entry.UnhideWhenUsed = onOffBool(style.Unhide)
		entry.QFormat = onOffBool(style.QuickFormat)
		entry.SemiHidden = onOffBool(style.SemiHidden)
		entry.AutoRedefine = onOffBool(style.AutoRedef)
		entry.RawFormat = paragraphFormatFromProperties(style.ParaProps, themeColors)
		entry.RawRunFormat = runFormatFromPropertiesWithTheme(style.RunProps, themeFonts, themeColors)
		entry.TableStyles = resolveConditionalTableStyles(style.TableStyles, themeColors)
		if style.ParaProps != nil && style.ParaProps.Numbering != nil {
			ref := &NumberingReference{}
			if style.ParaProps.Numbering.NumID != nil {
				ref.NumID = style.ParaProps.Numbering.NumID.Val
			}
			if style.ParaProps.Numbering.Level != nil {
				ref.Level = style.ParaProps.Numbering.Level.Val
			}
			entry.Numbering = ref
		}
		if style.Default != nil && bool(*style.Default) {
			entry.Default = true
			if defaultParagraphStyleID == "" {
				defaultParagraphStyleID = style.StyleID
			}
		}
		index[entry.ID] = entry
	}

	return index, defaultParagraphStyleID
}

func buildDefaultParagraphFormat(styles *parts.Styles) ParagraphFormat {
	if styles == nil || styles.DocDefaults == nil || styles.DocDefaults.ParaDefaults == nil {
		return ParagraphFormat{}
	}
	return paragraphFormatFromProperties(styles.DocDefaults.ParaDefaults.Properties, nil)
}

func resolveStyleInheritance(index map[string]Style) {
	for id, style := range index {
		style.StylePath = buildStylePath(index, id)
		index[id] = style
	}
}

func resolveStyleFormats(index map[string]Style, defaultParagraphFormat ParagraphFormat, defaultRunFormat RunFormat) {
	for id, style := range index {
		// Resolve inherited paragraph format (most-distant ancestor first).
		paraFormat := cloneParagraphFormat(defaultParagraphFormat)
		for i := len(style.StylePath) - 1; i >= 0; i-- {
			current, ok := index[style.StylePath[i]]
			if !ok {
				continue
			}
			paraFormat = mergeParagraphFormat(paraFormat, current.RawFormat)
		}
		style.Format = paraFormat

		// Resolve inherited run format for this paragraph style's rPr chain.
		runFmt := cloneRunFormat(defaultRunFormat)
		for i := len(style.StylePath) - 1; i >= 0; i-- {
			current, ok := index[style.StylePath[i]]
			if !ok {
				continue
			}
			runFmt = mergeRunFormat(runFmt, current.RawRunFormat)
		}
		style.RunFormat = runFmt

		index[id] = style
	}
}

func buildStylePath(index map[string]Style, id string) []string {
	if id == "" {
		return nil
	}

	path := make([]string, 0, 4)
	seen := make(map[string]struct{}, 4)
	current := id

	for current != "" {
		if _, ok := seen[current]; ok {
			break
		}
		seen[current] = struct{}{}
		path = append(path, current)

		style, ok := index[current]
		if !ok || style.BasedOn == "" {
			break
		}
		current = style.BasedOn
	}

	return path
}

func buildDefaultRunFormat(styles *parts.Styles, themeFonts map[string]string, themeColors map[string]string) RunFormat {
	var format RunFormat
	if styles != nil && styles.DocDefaults != nil && styles.DocDefaults.RunDefaults != nil {
		format = runFormatFromPropertiesWithTheme(styles.DocDefaults.RunDefaults.Properties, themeFonts, themeColors)
	}
	// Fall back to theme minor fonts when doc defaults don't specify them.
	if format.FontASCII == "" && themeFonts != nil {
		if v, ok := themeFonts["minor"]; ok {
			format.FontASCII = v
		}
	}
	if format.FontHAnsi == "" && themeFonts != nil {
		if v, ok := themeFonts["minor"]; ok {
			format.FontHAnsi = v
		}
	}
	if format.FontEastAsia == "" && themeFonts != nil {
		if v, ok := themeFonts["minorEA"]; ok {
			format.FontEastAsia = v
		}
	}
	if format.FontCS == "" && themeFonts != nil {
		if v, ok := themeFonts["minorCS"]; ok {
			format.FontCS = v
		}
	}
	return format
}

func buildCharacterStyleIndex(styles *parts.Styles, themeFonts map[string]string, themeColors map[string]string) (map[string]CharacterStyle, string) {
	index := make(map[string]CharacterStyle)
	defaultID := ""
	if styles == nil {
		return index, defaultID
	}
	for _, style := range styles.Styles {
		if style == nil || style.Type != "character" || style.StyleID == "" {
			continue
		}
		entry := CharacterStyle{ID: style.StyleID}
		if style.Name != nil {
			entry.Name = style.Name.Val
		}
		if style.BasedOn != nil {
			entry.BasedOn = style.BasedOn.Val
		}
		if style.Next != nil {
			entry.NextStyleID = style.Next.Val
		}
		if style.UIPriority != nil {
			entry.UIPriority = style.UIPriority.Val
		}
		entry.RSID = style.RSID
		entry.UnhideWhenUsed = onOffBool(style.Unhide)
		entry.QFormat = onOffBool(style.QuickFormat)
		entry.SemiHidden = onOffBool(style.SemiHidden)
		entry.AutoRedefine = onOffBool(style.AutoRedef)
		entry.RawRunFormat = runFormatFromPropertiesWithTheme(style.RunProps, themeFonts, themeColors)
		index[entry.ID] = entry
		if style.Default != nil && *style.Default {
			defaultID = style.StyleID
		}
	}
	return index, defaultID
}

func resolveCharStyleInheritance(index map[string]CharacterStyle) {
	for id, style := range index {
		style.StylePath = buildCharStylePath(index, id)
		index[id] = style
	}
}

func buildCharStylePath(index map[string]CharacterStyle, id string) []string {
	if id == "" {
		return nil
	}
	path := make([]string, 0, 4)
	seen := make(map[string]struct{}, 4)
	current := id
	for current != "" {
		if _, ok := seen[current]; ok {
			break
		}
		seen[current] = struct{}{}
		path = append(path, current)
		style, ok := index[current]
		if !ok || style.BasedOn == "" {
			break
		}
		current = style.BasedOn
	}
	return path
}

func resolveCharStyleFormats(index map[string]CharacterStyle, defaultRunFormat RunFormat) {
	for id, style := range index {
		format := cloneRunFormat(defaultRunFormat)
		for i := len(style.StylePath) - 1; i >= 0; i-- {
			current, ok := index[style.StylePath[i]]
			if !ok {
				continue
			}
			format = mergeRunFormat(format, current.RawRunFormat)
		}
		style.Format = format
		index[id] = style
	}
}

func buildHyperlinkIndex(rels *parts.Relationships) map[string]string {
	index := make(map[string]string)
	if rels == nil {
		return index
	}

	for _, relationship := range rels.Relationships {
		if relationship == nil {
			continue
		}
		if relationship.Type != "http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink" {
			continue
		}
		index[relationship.ID] = relationship.Target
	}

	return index
}

// buildThemeColorIndex converts the theme's ColorScheme into a slot-name → hex-RGB map.
// ECMA-376 Part 1 §20.1.4.1.
// buildNoteIndex builds an ID → *Note map from a footnotes or endnotes parts slice.
func buildNoteIndex(notes []*parts.Note) map[int]*parts.Note {
	index := make(map[int]*parts.Note)
	for _, note := range notes {
		if note != nil {
			index[note.ID] = note
		}
	}
	return index
}

func buildCommentIndex(part *parts.CommentsPart) map[int]*parts.Comment {
	index := make(map[int]*parts.Comment)
	if part == nil {
		return index
	}
	for _, comment := range part.Comments {
		if comment != nil {
			index[comment.ID] = comment
		}
	}
	return index
}

// noteSlice extracts the []*parts.Note slice from FootnotesPart or EndnotesPart (either may be nil).
func noteSlice(fp interface{}) []*parts.Note {
	switch v := fp.(type) {
	case *parts.FootnotesPart:
		if v != nil {
			return v.Notes
		}
	case *parts.EndnotesPart:
		if v != nil {
			return v.Notes
		}
	}
	return nil
}

func buildThemeColorIndex(theme *parts.Theme) map[string]string {
	index := make(map[string]string)
	if theme == nil || theme.Elements == nil || theme.Elements.ColorScheme == nil {
		return index
	}
	cs := theme.Elements.ColorScheme
	addColor := func(slot string, entry *parts.ThemeColorEntry) {
		if entry == nil {
			return
		}
		var base string
		if entry.SRGB != nil && entry.SRGB.Val != "" {
			base = entry.SRGB.Val
			index[slot] = applyThemeColorTransforms(base, &entry.SRGB.ThemeColorTransforms)
		} else if entry.SysClr != nil && entry.SysClr.LastClr != "" {
			index[slot] = applyThemeColorTransforms(entry.SysClr.LastClr, &entry.SysClr.ThemeColorTransforms)
		}
	}
	addColor("dk1", cs.Dk1)
	addColor("lt1", cs.Lt1)
	addColor("dk2", cs.Dk2)
	addColor("lt2", cs.Lt2)
	addColor("accent1", cs.Accent1)
	addColor("accent2", cs.Accent2)
	addColor("accent3", cs.Accent3)
	addColor("accent4", cs.Accent4)
	addColor("accent5", cs.Accent5)
	addColor("accent6", cs.Accent6)
	addColor("hlink", cs.Hlink)
	addColor("folHlink", cs.FolHlink)
	if hex, ok := index["dk1"]; ok {
		index["tx1"] = hex
		index["text1"] = hex
	}
	if hex, ok := index["lt1"]; ok {
		index["bg1"] = hex
		index["background1"] = hex
	}
	if hex, ok := index["dk2"]; ok {
		index["tx2"] = hex
		index["text2"] = hex
	}
	if hex, ok := index["lt2"]; ok {
		index["bg2"] = hex
		index["background2"] = hex
	}
	return index
}

// applyThemeSRGBTransforms converts a ThemeSRGBColor's child transforms into a
// drawing.SolidFill and applies them via the same HSL-based pipeline used for
// DrawingML shapes, effects, and charts.
func applyThemeColorTransforms(base string, t *parts.ThemeColorTransforms) string {
	if t == nil {
		return base
	}
	fill := &dml.SolidFill{}
	if t.Alpha != nil {
		fill.Alpha = float64(t.Alpha.Val) / 100000.0
	}
	if t.AlphaMod != nil {
		fill.AlphaMod = float64(t.AlphaMod.Val) / 100000.0
	}
	if t.AlphaOff != nil {
		fill.AlphaOff = float64(t.AlphaOff.Val) / 100000.0
	}
	if t.Shade != nil {
		fill.Shade = float64(t.Shade.Val) / 100000.0
		fill.HasShade = true
	}
	if t.Tint != nil {
		fill.Tint = float64(t.Tint.Val) / 100000.0
		fill.HasTint = true
	}
	if t.Red != nil {
		fill.Red = float64(t.Red.Val) / 100000.0
		fill.HasRed = true
	}
	if t.RedMod != nil {
		fill.RedMod = float64(t.RedMod.Val) / 100000.0
		fill.HasRedMod = true
	}
	if t.RedOff != nil {
		fill.RedOff = float64(t.RedOff.Val) / 100000.0
		fill.HasRedOff = true
	}
	if t.Green != nil {
		fill.Green = float64(t.Green.Val) / 100000.0
		fill.HasGreen = true
	}
	if t.GreenMod != nil {
		fill.GreenMod = float64(t.GreenMod.Val) / 100000.0
		fill.HasGreenMod = true
	}
	if t.GreenOff != nil {
		fill.GreenOff = float64(t.GreenOff.Val) / 100000.0
		fill.HasGreenOff = true
	}
	if t.Blue != nil {
		fill.Blue = float64(t.Blue.Val) / 100000.0
		fill.HasBlue = true
	}
	if t.BlueMod != nil {
		fill.BlueMod = float64(t.BlueMod.Val) / 100000.0
		fill.HasBlueMod = true
	}
	if t.BlueOff != nil {
		fill.BlueOff = float64(t.BlueOff.Val) / 100000.0
		fill.HasBlueOff = true
	}
	if t.Hue != nil {
		fill.Hue = float64(t.Hue.Val) / 60000.0
		fill.HasHue = true
	}
	if t.Sat != nil {
		fill.Sat = float64(t.Sat.Val) / 100000.0
		fill.HasSat = true
	}
	if t.Lum != nil {
		fill.Lum = float64(t.Lum.Val) / 100000.0
		fill.HasLum = true
	}
	if t.LumMod != nil {
		fill.LumMod = float64(t.LumMod.Val) / 100000.0
	}
	if t.LumOff != nil {
		fill.LumOff = float64(t.LumOff.Val) / 100000.0
	}
	if t.SatMod != nil {
		fill.SatMod = float64(t.SatMod.Val) / 100000.0
	}
	if t.SatOff != nil {
		fill.SatOff = float64(t.SatOff.Val) / 100000.0
	}
	if t.HueMod != nil {
		fill.HueMod = float64(t.HueMod.Val) / 60000.0
	}
	if t.HueOff != nil {
		fill.HueOff = float64(t.HueOff.Val) / 60000.0
	}
	fill.Gray = t.Gray != nil
	fill.Comp = t.Comp != nil
	fill.Inv = t.Inv != nil
	fill.Gamma = t.Gamma != nil
	fill.InvGamma = t.InvGamma != nil
	return fill.ApplyColorTransforms(base)
}

// applyClrSchemeMapping overrides theme color aliases from w:clrSchemeMapping.
// ECMA-376 Part 1 §17.15.1.18.
func applyClrSchemeMapping(index map[string]string, mapping *parts.ColorSchemeMapping) {
	if mapping == nil {
		return
	}
	redirect := func(dst, src string) {
		if src == "" {
			return
		}
		if hex, ok := index[src]; ok {
			index[dst] = hex
		}
	}
	redirect("bg1", mapping.BG1)
	redirect("t1", mapping.T1)
	redirect("tx1", mapping.T1)
	redirect("text1", mapping.T1)
	redirect("bg2", mapping.BG2)
	redirect("t2", mapping.T2)
	redirect("tx2", mapping.T2)
	redirect("text2", mapping.T2)
	redirect("accent1", mapping.Accent1)
	redirect("accent2", mapping.Accent2)
	redirect("accent3", mapping.Accent3)
	redirect("accent4", mapping.Accent4)
	redirect("accent5", mapping.Accent5)
	redirect("accent6", mapping.Accent6)
	redirect("hlink", mapping.Hyperlink)
	redirect("folHlink", mapping.FollowedHyperlink)
}

// buildThemeFontIndex maps major/minior slots to their typeface names.
// Populates Latin, EastAsian, and ComplexScript entries.
// ECMA-376 Part 1 §20.1.4.1.
func buildThemeFontIndex(theme *parts.Theme, themeFontLang *parts.ThemeFontLang) map[string]string {
	index := make(map[string]string)
	if theme == nil || theme.Elements == nil || theme.Elements.FontScheme == nil {
		return index
	}
	fs := theme.Elements.FontScheme
	addFont := func(slot string, tf *parts.ThemeFontSet) {
		if tf == nil {
			return
		}
		if tf.Latin.Typeface != "" {
			index[slot] = tf.Latin.Typeface
		}
		for _, font := range tf.Fonts {
			if font == nil || font.Script == "" || font.Typeface == "" {
				continue
			}
			index[supplementalThemeFontKey(slot, font.Script)] = font.Typeface
		}
		if len(tf.EastAsia) > 0 && tf.EastAsia[0].Typeface != "" {
			index[slot+"EA"] = tf.EastAsia[0].Typeface
			index[slot+"EastAsia"] = tf.EastAsia[0].Typeface
		} else if script := themeFontLangScript(themeFontLang, true); script != "" {
			if typeface := supplementalThemeTypeface(tf, script); typeface != "" {
				index[slot+"EA"] = typeface
				index[slot+"EastAsia"] = typeface
			}
		}
		if len(tf.ComplexScript) > 0 && tf.ComplexScript[0].Typeface != "" {
			index[slot+"CS"] = tf.ComplexScript[0].Typeface
			index[slot+"ComplexScript"] = tf.ComplexScript[0].Typeface
		} else if script := themeFontLangScript(themeFontLang, false); script != "" {
			if typeface := supplementalThemeTypeface(tf, script); typeface != "" {
				index[slot+"CS"] = typeface
				index[slot+"ComplexScript"] = typeface
			}
		}
	}
	addFont("major", fs.MajorFont)
	addFont("minor", fs.MinorFont)
	return index
}

func supplementalThemeFontKey(slot, script string) string {
	slot = strings.TrimSpace(strings.ToLower(slot))
	script = strings.TrimSpace(strings.ToLower(script))
	if slot == "" || script == "" {
		return ""
	}
	return slot + "Script:" + script
}

func supplementalThemeTypeface(set *parts.ThemeFontSet, script string) string {
	if set == nil || script == "" {
		return ""
	}
	for _, font := range set.Fonts {
		if font == nil || font.Typeface == "" || !strings.EqualFold(font.Script, script) {
			continue
		}
		return font.Typeface
	}
	return ""
}

func themeFontLangScript(lang *parts.ThemeFontLang, eastAsia bool) string {
	if lang == nil {
		return ""
	}
	if eastAsia {
		return bcp47ToThemeScript(lang.EastAsia)
	}
	return bcp47ToThemeScript(lang.Bidi)
}

func bcp47ToThemeScript(tag string) string {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return ""
	}
	lower := strings.ToLower(tag)
	for _, part := range strings.FieldsFunc(lower, func(r rune) bool {
		return r == '-' || r == '_'
	}) {
		switch part {
		case "jpan":
			return "Jpan"
		case "hang":
			return "Hang"
		case "hans":
			return "Hans"
		case "hant":
			return "Hant"
		case "arab":
			return "Arab"
		case "hebr":
			return "Hebr"
		case "thai":
			return "Thai"
		case "ethi":
			return "Ethi"
		case "beng":
			return "Beng"
		case "gujr":
			return "Gujr"
		case "guru":
			return "Guru"
		case "knda":
			return "Knda"
		case "mlym":
			return "Mlym"
		case "orya":
			return "Orya"
		case "sinh":
			return "Sinh"
		case "taml":
			return "Taml"
		case "telu":
			return "Telu"
		case "deva":
			return "Deva"
		case "khmr":
			return "Khmr"
		case "laoo":
			return "Laoo"
		case "mymr":
			return "Mymr"
		case "geor":
			return "Geor"
		case "armn":
			return "Armn"
		case "grek":
			return "Grek"
		case "cyrl":
			return "Cyrl"
		case "tibt":
			return "Tibt"
		}
	}
	langPart := lower
	if i := strings.IndexAny(langPart, "-_"); i >= 0 {
		langPart = langPart[:i]
	}
	switch langPart {
	case "ja":
		return "Jpan"
	case "ko":
		return "Hang"
	case "zh":
		if strings.Contains(lower, "-tw") || strings.Contains(lower, "-hk") || strings.Contains(lower, "-mo") || strings.Contains(lower, "-hant") {
			return "Hant"
		}
		return "Hans"
	case "ar", "fa", "ur", "ps", "sd", "ug":
		return "Arab"
	case "he", "yi":
		return "Hebr"
	case "th":
		return "Thai"
	case "am", "ti":
		return "Ethi"
	case "bn":
		return "Beng"
	case "gu":
		return "Gujr"
	case "pa":
		return "Guru"
	case "kn":
		return "Knda"
	case "ml":
		return "Mlym"
	case "or":
		return "Orya"
	case "si":
		return "Sinh"
	case "ta":
		return "Taml"
	case "te":
		return "Telu"
	case "hi", "mr", "ne", "kok", "mai", "sa":
		return "Deva"
	case "km":
		return "Khmr"
	case "lo":
		return "Laoo"
	case "my":
		return "Mymr"
	case "ka":
		return "Geor"
	case "hy":
		return "Armn"
	case "el":
		return "Grek"
	case "ru", "uk", "bg", "sr", "mk", "be", "kk", "ky", "mn", "tg", "tt", "uz":
		return "Cyrl"
	case "bo", "dz":
		return "Tibt"
	}
	return ""
}

// themeFormatIndex holds resolved DrawingML fills, lines, and effects from the
// theme's format scheme (a:fmtScheme).  Style references (fillRef, lnRef,
// effectRef) use idx to look up entries here.
type themeFormatIndex struct {
	fills   []dml.Fill
	lines   []*dml.LineStyle
	effects []dml.EffectList
	bgFills []dml.Fill
}

func buildThemeFormatIndex(theme *parts.Theme) *themeFormatIndex {
	idx := &themeFormatIndex{}
	if theme == nil || theme.Elements == nil || theme.Elements.FmtScheme == nil {
		return idx
	}
	fs := theme.Elements.FmtScheme
	themeColors := buildThemeColorIndex(theme)
	for _, e := range fs.FillStyleList {
		idx.fills = append(idx.fills, themeFillEntryToDrawingFill(e, themeColors))
	}
	for _, e := range fs.LineStyleList {
		if e.Line != nil {
			idx.lines = append(idx.lines, drawingxml.ResolveLineStyle(e.Line, themeColors))
		}
	}
	for _, e := range fs.EffectStyleList {
		if e.EffectList != nil {
			idx.effects = append(idx.effects, dml.EffectList{Effects: parts.WpsDrawingEffectRaw(*e.EffectList, nil, nil, nil, "")})
		}
	}
	for _, e := range fs.BgFillStyleList {
		idx.bgFills = append(idx.bgFills, themeFillEntryToDrawingFill(e, themeColors))
	}
	return idx
}

func buildIRTheme(theme *parts.Theme, themeColors map[string]string, themeFonts map[string]string, fmtIdx *themeFormatIndex) *Theme {
	if theme == nil || theme.Elements == nil {
		return nil
	}
	t := &Theme{
		Name:        theme.Name,
		ColorScheme: make(map[string]string, len(themeColors)),
	}
	for k, v := range themeColors {
		t.ColorScheme[k] = v
	}
	if theme.Elements.FontScheme != nil {
		fs := theme.Elements.FontScheme
		if fs.MajorFont != nil {
			t.FontScheme.MajorFont.Latin = fs.MajorFont.Latin.Typeface
			if len(fs.MajorFont.EastAsia) > 0 {
				t.FontScheme.MajorFont.EastAsia = fs.MajorFont.EastAsia[0].Typeface
			}
			if len(fs.MajorFont.ComplexScript) > 0 {
				t.FontScheme.MajorFont.ComplexScript = fs.MajorFont.ComplexScript[0].Typeface
			}
		}
		if fs.MinorFont != nil {
			t.FontScheme.MinorFont.Latin = fs.MinorFont.Latin.Typeface
			if len(fs.MinorFont.EastAsia) > 0 {
				t.FontScheme.MinorFont.EastAsia = fs.MinorFont.EastAsia[0].Typeface
			}
			if len(fs.MinorFont.ComplexScript) > 0 {
				t.FontScheme.MinorFont.ComplexScript = fs.MinorFont.ComplexScript[0].Typeface
			}
		}
	}
	if fmtIdx != nil {
		t.FormatScheme.FillStyles = append([]dml.Fill(nil), fmtIdx.fills...)
		t.FormatScheme.LineStyles = append([]*dml.LineStyle(nil), fmtIdx.lines...)
		t.FormatScheme.EffectStyles = append([]dml.EffectList(nil), fmtIdx.effects...)
		t.FormatScheme.BgFillStyles = append([]dml.Fill(nil), fmtIdx.bgFills...)
	}
	return t
}

func themeFillEntryToDrawingFill(e parts.ThemeFillEntry, themeColors map[string]string) dml.Fill {
	switch {
	case e.SolidFill != nil:
		if sf := drawingxml.ResolveSolidFill(e.SolidFill, themeColors); sf != nil {
			return sf
		}
	case e.GradFill != nil:
		if gf := drawingxml.ResolveGradientFill(e.GradFill, themeColors); gf != nil {
			return gf
		}
	case e.PattFill != nil:
		if pf := drawingxml.ResolvePatternFill(e.PattFill, themeColors); pf != nil {
			return pf
		}
	case e.NoFill != nil:
		return &dml.NoFill{}
	case e.BlipFill != nil:
		return drawingxml.ResolveBlipFill(e.BlipFill)
	}
	return nil
}

func resolveFillRef(idx *themeFormatIndex, ref *parts.WpsStyleRef) dml.Fill {
	if idx == nil || ref == nil {
		return nil
	}
	i := ref.Idx
	if i < 0 || i >= len(idx.fills) {
		return nil
	}
	return idx.fills[i]
}

func resolveLineRef(idx *themeFormatIndex, ref *parts.WpsStyleRef) *dml.LineStyle {
	if idx == nil || ref == nil {
		return nil
	}
	i := ref.Idx
	if i < 0 || i >= len(idx.lines) {
		return nil
	}
	return idx.lines[i]
}

func resolveEffectRef(idx *themeFormatIndex, ref *parts.WpsStyleRef) dml.EffectList {
	if idx == nil || ref == nil {
		return dml.EffectList{}
	}
	i := ref.Idx
	if i < 0 || i >= len(idx.effects) {
		return dml.EffectList{}
	}
	return idx.effects[i]
}

// resolveShapeStyle resolves fillRef, lnRef, and effectRef from a shape's style
// against the theme format scheme.  It returns the resolved fill, line style, and
// effect list.  Explicit shape properties take precedence; these are only used as
// fall-backs.
func resolveShapeStyle(ctx *resolveCtx, style *parts.WpsShapeStyle) (dml.Fill, *dml.LineStyle, dml.EffectList) {
	if style == nil || ctx.themeFmtScheme == nil {
		return nil, nil, dml.EffectList{}
	}
	var fill dml.Fill
	var line *dml.LineStyle
	var effects dml.EffectList
	if style.FillRef != nil {
		fill = resolveFillRef(ctx.themeFmtScheme, style.FillRef)
	}
	if style.LnRef != nil {
		line = resolveLineRef(ctx.themeFmtScheme, style.LnRef)
	}
	if style.EffectRef != nil {
		effects = resolveEffectRef(ctx.themeFmtScheme, style.EffectRef)
	}
	return fill, line, effects
}

func resolveShapeStyleFontFamily(themeFonts map[string]string, style *parts.WpsShapeStyle) string {
	if style == nil || style.FontRef == nil {
		return ""
	}
	return resolveThemeFontSlot(themeFonts, style.FontRef.Idx)
}

func applyThemeFontFamilyToBlocks(blocks []Block, family string) {
	if family == "" {
		return
	}
	for i := range blocks {
		applyThemeFontFamilyToBlock(&blocks[i], family)
	}
}

func applyThemeFontFamilyToBlock(block *Block, family string) {
	if block == nil {
		return
	}
	if block.Paragraph != nil {
		applyThemeFontFamilyToParagraph(block.Paragraph, family)
	}
	if block.Table != nil {
		for i := range block.Table.Rows {
			for j := range block.Table.Rows[i].Cells {
				for k := range block.Table.Rows[i].Cells[j].Paragraphs {
					applyThemeFontFamilyToParagraph(&block.Table.Rows[i].Cells[j].Paragraphs[k], family)
				}
			}
		}
	}
}

func applyThemeFontFamilyToParagraph(paragraph *Paragraph, family string) {
	if paragraph == nil {
		return
	}
	for i := range paragraph.Runs {
		applyThemeFontFamilyToRun(&paragraph.Runs[i], family)
	}
}

func applyThemeFontFamilyToRun(run *Run, family string) {
	if run == nil {
		return
	}
	if run.Format.FontASCII == "" && run.Format.FontHAnsi == "" && run.Format.FontEastAsia == "" && run.Format.FontCS == "" {
		run.Format.FontASCII = family
		run.Format.FontHAnsi = family
	}
	if run.TextBox != nil {
		applyThemeFontFamilyToBlocks(run.TextBox.Blocks, family)
	}
	if run.Shape != nil && run.Shape.TextBox != nil {
		applyThemeFontFamilyToBlocks(run.Shape.TextBox.Blocks, family)
	}
}

func resolveEmbeddedFonts(fonts []*parts.EmbeddedFontData) []*EmbeddedFont {
	out := make([]*EmbeddedFont, 0, len(fonts))
	for _, font := range fonts {
		if font == nil || strings.TrimSpace(font.Typeface) == "" {
			continue
		}
		resolved := &EmbeddedFont{Family: strings.TrimSpace(font.Typeface)}
		appendFace := func(part *parts.FontPart, weight int, style string) {
			if part == nil || len(part.Data) == 0 {
				return
			}
			resolved.Faces = append(resolved.Faces, &EmbeddedFontFace{
				Family:      resolved.Family,
				Weight:      weight,
				Style:       style,
				ContentType: part.ContentType,
				Data:        part.Data,
			})
		}
		appendFace(font.Regular, 400, "normal")
		appendFace(font.Bold, 700, "normal")
		appendFace(font.Italic, 400, "italic")
		appendFace(font.BoldItalic, 700, "italic")
		if len(resolved.Faces) == 0 {
			continue
		}
		out = append(out, resolved)
	}
	return out
}

// normalizeRuby converts a parts.Ruby (w:ruby) into a resolve.Run with Kind="ruby".
func normalizeRuby(ctx *resolveCtx, ruby *parts.Ruby, paraRunFmt RunFormat) *Run {
	if ruby == nil {
		return nil
	}
	// Aggregate base text from all base runs.
	var baseSB, textSB strings.Builder
	var baseFmt RunFormat
	for _, run := range ruby.BaseRuns {
		normalized := normalizeRun(ctx, run, "", "", paraRunFmt)
		for _, r := range normalized {
			baseSB.WriteString(r.Text)
			baseFmt = r.Format
		}
	}
	for _, run := range ruby.TextRuns {
		normalized := normalizeRun(ctx, run, "", "", paraRunFmt)
		for _, r := range normalized {
			textSB.WriteString(r.Text)
		}
	}
	baseText := baseSB.String()
	rubyText := textSB.String()
	if baseText == "" && rubyText == "" {
		return nil
	}
	rc := &RubyContent{
		Base: baseText,
		Text: rubyText,
	}
	if ruby.RubyPr != nil {
		if ruby.RubyPr.RubyAlign != nil {
			rc.Align = ruby.RubyPr.RubyAlign.Val
		}
		if ruby.RubyPr.PhoneticGuideTextFontSize != nil {
			rc.FontSize = float64(ruby.RubyPr.PhoneticGuideTextFontSize.Val) / 2.0
		}
		if ruby.RubyPr.BaseTextFontSize != nil {
			rc.BaseFontSize = float64(ruby.RubyPr.BaseTextFontSize.Val) / 2.0
		}
		if ruby.RubyPr.PhoneticGuideRaise != nil {
			rc.Raise = float64(ruby.RubyPr.PhoneticGuideRaise.Val) / 2.0
		}
		if ruby.RubyPr.LanguageID != nil {
			rc.Lang = ruby.RubyPr.LanguageID.Val
		}
	}
	return &Run{
		Kind:   "ruby",
		Format: baseFmt,
		Ruby:   rc,
	}
}

// parseVMLStyleExtent extracts width and height (in EMU) from a v:shape style attribute.
// The style attribute is a CSS-like string with key:value pairs separated by semicolons.
// Dimensions may be in pt, cm, mm, or in. Returns 0,0 if parsing fails.
func parseVMLStyleExtent(style string) (widthEMU, heightEMU int64) {
	for key, val := range parseVMLStyleProperties(style) {
		switch key {
		case "width":
			widthEMU = parseVMLDimensionEMU(val)
		case "height":
			heightEMU = parseVMLDimensionEMU(val)
		}
	}
	return
}

func parseVMLStylePosition(style string) (xEMU, yEMU int64, floating bool) {
	props := parseVMLStyleProperties(style)
	if strings.ToLower(props["position"]) != "absolute" {
		return 0, 0, false
	}
	xEMU = parseVMLDimensionEMU(firstNonEmpty(props["margin-left"], props["left"]))
	yEMU = parseVMLDimensionEMU(firstNonEmpty(props["margin-top"], props["top"]))
	return xEMU, yEMU, true
}

func parseVMLTextBoxVert(shapeStyle, textBoxStyle string) string {
	for _, style := range []string{textBoxStyle, shapeStyle} {
		if style == "" {
			continue
		}
		props := parseVMLStyleProperties(style)
		for _, key := range []string{"mso-layout-flow-alt", "layout-flow"} {
			if vert := mapVMLLayoutFlowValue(props[key]); vert != "" {
				return vert
			}
		}
	}
	return ""
}

func mapVMLLayoutFlowValue(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "top-to-bottom", "vertical", "vertical-ideographic", "tb-rl":
		return "vert"
	case "bottom-to-top", "bt-lr":
		return "vert270"
	default:
		return ""
	}
}

func parseVMLStyleProperties(style string) map[string]string {
	props := map[string]string{}
	for _, part := range strings.Split(style, ";") {
		kv := strings.SplitN(strings.TrimSpace(part), ":", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(kv[0]))
		val := strings.TrimSpace(kv[1])
		props[key] = val
	}
	return props
}

func normalizeVMLColor(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "#")
	if len(value) == 6 {
		return strings.ToUpper(value)
	}
	return ""
}

func parseVMLBoolAttr(value string, def bool) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "":
		return def
	case "t", "true", "1":
		return true
	case "f", "false", "0":
		return false
	default:
		return def
	}
}

func parseVMLLineExtent(from, to string) (widthEMU, heightEMU int64) {
	x1, y1, ok1 := parseVMLPointEMU(from)
	x2, y2, ok2 := parseVMLPointEMU(to)
	if !ok1 || !ok2 {
		return 0, 0
	}
	widthEMU = x2 - x1
	if widthEMU < 0 {
		widthEMU = -widthEMU
	}
	heightEMU = y2 - y1
	if heightEMU < 0 {
		heightEMU = -heightEMU
	}
	return widthEMU, heightEMU
}

func parseVMLPointEMU(value string) (xEMU, yEMU int64, ok bool) {
	parts := strings.Split(value, ",")
	if len(parts) != 2 {
		return 0, 0, false
	}
	xEMU = parseVMLDimensionEMU(strings.TrimSpace(parts[0]))
	yEMU = parseVMLDimensionEMU(strings.TrimSpace(parts[1]))
	return xEMU, yEMU, true
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

// parseVMLDimensionEMU converts a CSS dimension string (e.g. "72pt", "2.54cm") to EMU.
// Unsupported units return 0.
func parseVMLDimensionEMU(val string) int64 {
	// Find where the numeric part ends.
	i := 0
	for i < len(val) {
		c := val[i]
		if (c >= '0' && c <= '9') || c == '.' || (i == 0 && (c == '-' || c == '+')) {
			i++
		} else {
			break
		}
	}
	if i == 0 || i == len(val) {
		return 0
	}
	num, err := strconv.ParseFloat(val[:i], 64)
	if err != nil {
		return 0
	}
	unit := strings.ToLower(strings.TrimSpace(val[i:]))
	// 1 EMU = 1/914400 in; 1pt = 914400/72 EMU; 1cm = 360000 EMU; 1mm = 36000 EMU
	switch unit {
	case "pt":
		return int64(math.Round(num * 914400 / 72))
	case "in":
		return int64(math.Round(num * 914400))
	case "cm":
		return int64(math.Round(num * 360000))
	case "mm":
		return int64(math.Round(num * 36000))
	case "px":
		// VML uses 96dpi for screen pixels.
		return int64(math.Round(num * 914400 / 96))
	}
	return 0
}

func runFormatFromProperties(properties *parts.RunProperties) RunFormat {
	return runFormatFromPropertiesWithTheme(properties, nil, nil)
}

func runFormatFromPropertiesWithTheme(properties *parts.RunProperties, themeFonts map[string]string, themeColors map[string]string) RunFormat {
	var format RunFormat
	if properties == nil {
		return format
	}

	format.Bold = onOffPtr(properties.Bold)
	format.Italic = onOffPtr(properties.Italic)
	format.Caps = onOffPtr(properties.Caps)
	format.SmallCaps = onOffPtr(properties.SmallCaps)
	format.Strike = onOffPtr(properties.Strike)
	format.DoubleStrike = onOffPtr(properties.DoubleStrike)
	format.Vanish = onOffPtr(properties.Vanish)
	format.SpecVanish = onOffPtr(properties.SpecVanish)
	format.NoProof = onOffPtr(properties.NoProof)
	format.OMath = onOffPtr(properties.OMath)
	if properties.Underline != nil {
		format.Underline = properties.Underline.Val
		format.UnderlineColor = properties.Underline.Color
	}
	if properties.Color != nil {
		switch {
		case properties.Color.Val != "" && properties.Color.Val != "auto":
			format.Color = properties.Color.Val
		case properties.Color.ThemeColor != "" && themeColors != nil:
			if hex, ok := themeColors[properties.Color.ThemeColor]; ok {
				format.Color = applyWordColorModifier(hex, properties.Color.ThemeShade, properties.Color.ThemeTint)
			}
		}
	}
	if properties.FontSize != nil {
		v := properties.FontSize.Val
		format.FontSize = &v
	}
	if properties.TextFill != nil {
		format.TextFill = runTextFillFromProperties(properties.TextFill, themeColors)
	}
	if properties.TextOutline != nil {
		format.TextOutline = runTextOutlineFromProperties(properties.TextOutline, themeColors)
	}
	if properties.Glow != nil {
		format.TextGlow = runTextGlowFromProperties(properties.Glow, themeColors)
	}
	if properties.Shadow != nil {
		format.TextShadow = runTextShadowFromProperties(properties.Shadow, themeColors)
	}
	if properties.SoftEdge != nil {
		format.TextSoftEdge = runTextSoftEdgeFromProperties(properties.SoftEdge)
	}
	if properties.Reflection != nil {
		format.TextReflection = runTextReflectionFromProperties(properties.Reflection)
	}
	if properties.Ligatures != nil {
		format.Ligatures = properties.Ligatures.Val
	}
	if properties.NumSpacing != nil {
		format.NumberSpacing = properties.NumSpacing.Val
	}
	if properties.StylisticSets != nil {
		for _, set := range properties.StylisticSets.StyleSets {
			if set.ID <= 0 || !set.Enabled() {
				continue
			}
			format.StylisticSets = append(format.StylisticSets, set.ID)
		}
	}
	if properties.Highlight != nil {
		format.Highlight = properties.Highlight.Val
	}
	if properties.VertAlign != nil {
		format.VertAlignVal = properties.VertAlign.Val
	}
	if properties.Fonts != nil {
		if properties.Fonts.ASCII != "" {
			format.FontASCII = properties.Fonts.ASCII
		}
		if properties.Fonts.HAnsi != "" {
			format.FontHAnsi = properties.Fonts.HAnsi
		}
		if properties.Fonts.EastAsia != "" {
			format.FontEastAsia = properties.Fonts.EastAsia
		}
		if properties.Fonts.CS != "" {
			format.FontCS = properties.Fonts.CS
		}
		if format.FontASCII == "" && properties.Fonts.AsciiTheme != "" {
			format.FontASCII = resolveThemeFontSlotWithLang(themeFonts, properties.Fonts.AsciiTheme, properties.Lang)
		}
		if format.FontHAnsi == "" && properties.Fonts.HAnsiTheme != "" {
			format.FontHAnsi = resolveThemeFontSlotWithLang(themeFonts, properties.Fonts.HAnsiTheme, properties.Lang)
		}
		if format.FontEastAsia == "" && properties.Fonts.EastAsiaTheme != "" {
			format.FontEastAsia = resolveThemeFontSlotWithLang(themeFonts, properties.Fonts.EastAsiaTheme, properties.Lang)
		}
		if format.FontCS == "" && properties.Fonts.CSTheme != "" {
			format.FontCS = resolveThemeFontSlotWithLang(themeFonts, properties.Fonts.CSTheme, properties.Lang)
		}
	}
	if properties.Shading != nil {
		fill := properties.Shading.Fill
		if fill == "" || fill == "auto" {
			fill = properties.Shading.Color
		}
		if fill != "" && fill != "auto" {
			if properties.Shading.ThemeFill != "" && themeColors != nil {
				if resolved, ok := themeColors[properties.Shading.ThemeFill]; ok {
					fill = applyWordColorModifier(resolved, properties.Shading.ThemeFillShade, properties.Shading.ThemeFillTint)
				}
			}
			format.Background = fill
		}
	}
	if properties.Lang != nil && properties.Lang.Val != "" {
		format.Lang = properties.Lang.Val
	}
	if properties.Spacing != nil {
		v := properties.Spacing.Val
		format.CharacterSpacing = &v
	}
	format.RTL = onOffPtr(properties.RTL)
	format.Outline = onOffPtr(properties.OutlineToggle)
	format.Shadow = onOffPtr(properties.ShadowToggle)
	format.Emboss = onOffPtr(properties.Emboss)
	format.Imprint = onOffPtr(properties.Imprint)
	if properties.Borders != nil {
		b := resolveBorderSide(properties.Borders)
		format.Borders = &RunBorders{Top: b, Left: b, Bottom: b, Right: b}
	}
	if properties.BoldCS != nil {
		format.BoldCS = onOffPtr(properties.BoldCS)
	}
	if properties.ItalicCS != nil {
		format.ItalicCS = onOffPtr(properties.ItalicCS)
	}
	if properties.FontSizeCS != nil {
		v := properties.FontSizeCS.Val
		format.FontSizeCS = &v
	}
	if properties.EastAsianLayout != nil {
		fe := &FELayout{
			CombineCharacters:  onOffBool(properties.EastAsianLayout.Combine),
			CombineBrackets:    onOffBool(properties.EastAsianLayout.CombineBrackets),
			Vertical:           onOffBool(properties.EastAsianLayout.Vert),
			VerticalCompressed: onOffBool(properties.EastAsianLayout.VertCompress),
			CombineCharCount:   properties.EastAsianLayout.CombineCharCount,
		}
		if fe.CombineCharacters || fe.CombineBrackets || fe.Vertical || fe.VerticalCompressed || fe.CombineCharCount > 0 {
			format.FELayout = fe
		}
	}
	format.ComplexScript = onOffPtr(properties.CS)
	format.WebHidden = onOffPtr(properties.WebHidden)
	if properties.Kerning != nil {
		v := properties.Kerning.Val
		format.Kerning = &v
	}
	if properties.CharScale != nil {
		v := properties.CharScale.Val
		format.CharScale = &v
	}
	if properties.FitText != nil {
		v := properties.FitText.Val
		format.FitTextWidth = &v
	}
	if properties.EmphasisMark != nil {
		format.EmphasisMark = properties.EmphasisMark.Val
	}
	if properties.Position != nil {
		v := properties.Position.Val
		format.Position = &v
	}
	if properties.Effect != nil {
		format.TextAnimation = properties.Effect.Val
	}

	return format
}

func runTextFillFromProperties(fill *parts.Word2010TextFill, themeColors map[string]string) dml.Fill {
	if fill == nil {
		return nil
	}
	if fill.NoFill != nil {
		return &dml.NoFill{}
	}
	if fill.GradFill != nil {
		return drawingxml.ResolveGradientFill(fill.GradFill, themeColors)
	}
	return drawingSolidFillFromParts(fill.SolidFill, themeColors)
}

func runTextOutlineFromProperties(outline *parts.Word2010TextOutline, themeColors map[string]string) *dml.LineStyle {
	if outline == nil {
		return nil
	}
	line := &dml.LineStyle{Width: dml.EMUToPoints(outline.Width), Alpha: 1.0}
	if outline.NoFill != nil {
		line.Alpha = 0
		return line
	}
	fill := drawingSolidFillFromParts(outline.SolidFill, themeColors)
	if fill == nil {
		return line
	}
	line.Color = fill.Color
	line.Alpha = fill.Alpha
	return line
}

func runTextGlowFromProperties(glow *parts.Word2010GlowEffect, themeColors map[string]string) *dml.Glow {
	if glow == nil {
		return nil
	}
	color, alpha := resolveWord2010EffectColor(glow.SrgbClr, glow.SchemeClr, themeColors)
	if color == "" {
		return nil
	}
	return &dml.Glow{Color: color, Alpha: alpha, Radius: dml.EMUToPoints(glow.Rad)}
}

func runTextShadowFromProperties(shadow *parts.Word2010ShadowEffect, themeColors map[string]string) *dml.Shadow {
	if shadow == nil {
		return nil
	}
	color, alpha := resolveWord2010EffectColor(shadow.SrgbClr, shadow.SchemeClr, themeColors)
	if color == "" {
		return nil
	}
	return &dml.Shadow{
		Color:    color,
		Alpha:    alpha,
		Blur:     dml.EMUToPoints(shadow.BlurRad),
		Distance: dml.EMUToPoints(shadow.Dist),
		Angle:    float64(shadow.Dir) / 60000.0,
		ScaleX:   1.0,
		ScaleY:   1.0,
	}
}

func runTextSoftEdgeFromProperties(softEdge *parts.Word2010SoftEdgeEffect) *dml.SoftEdge {
	if softEdge == nil || softEdge.Rad == 0 {
		return nil
	}
	return &dml.SoftEdge{Radius: dml.EMUToPoints(softEdge.Rad)}
}

func runTextReflectionFromProperties(reflection *parts.Word2010ReflectionEffect) *dml.Reflection {
	if reflection == nil {
		return nil
	}
	alpha := 0.3
	if reflection.StA > 0 {
		alpha = float64(reflection.StA) / 100000.0
	}
	return &dml.Reflection{
		Alpha:     alpha,
		Blur:      dml.EMUToPoints(reflection.BlurRad),
		Distance:  dml.EMUToPoints(reflection.Dist),
		Direction: float64(reflection.Dir) / 60000.0,
		Size:      1.0,
		Fade:      true,
	}
}

func drawingSolidFillFromParts(fill *parts.DrawingSolidFill, themeColors map[string]string) *dml.SolidFill {
	if fill == nil {
		return nil
	}
	solid := &dml.SolidFill{Alpha: 1.0}
	if fill.SRGB != nil && fill.SRGB.Val != "" {
		solid.Color = fill.SRGB.Val
		applyDrawingColorTransforms(solid, fill.SRGB.Alpha, fill.SRGB.AlphaMod, fill.SRGB.AlphaOff, fill.SRGB.Shade, fill.SRGB.Tint, fill.SRGB.Red, fill.SRGB.RedMod, fill.SRGB.RedOff, fill.SRGB.Green, fill.SRGB.GreenMod, fill.SRGB.GreenOff, fill.SRGB.Blue, fill.SRGB.BlueMod, fill.SRGB.BlueOff, fill.SRGB.Hue, fill.SRGB.Sat, fill.SRGB.Lum, fill.SRGB.LumMod, fill.SRGB.LumOff, fill.SRGB.SatMod, fill.SRGB.SatOff, fill.SRGB.HueMod, fill.SRGB.HueOff, fill.SRGB.Gray != nil, fill.SRGB.Comp != nil, fill.SRGB.Inv != nil, fill.SRGB.Gamma != nil, fill.SRGB.InvGamma != nil)
		solid.Color = solid.ApplyColorTransforms(solid.Color)
		return solid
	}
	if fill.SchemeClr == nil || fill.SchemeClr.Val == "" {
		return nil
	}
	solid.ThemeColor = fill.SchemeClr.Val
	applyDrawingColorTransforms(solid, fill.SchemeClr.Alpha, fill.SchemeClr.AlphaMod, fill.SchemeClr.AlphaOff, fill.SchemeClr.Shade, fill.SchemeClr.Tint, fill.SchemeClr.Red, fill.SchemeClr.RedMod, fill.SchemeClr.RedOff, fill.SchemeClr.Green, fill.SchemeClr.GreenMod, fill.SchemeClr.GreenOff, fill.SchemeClr.Blue, fill.SchemeClr.BlueMod, fill.SchemeClr.BlueOff, fill.SchemeClr.Hue, fill.SchemeClr.Sat, fill.SchemeClr.Lum, fill.SchemeClr.LumMod, fill.SchemeClr.LumOff, fill.SchemeClr.SatMod, fill.SchemeClr.SatOff, fill.SchemeClr.HueMod, fill.SchemeClr.HueOff, fill.SchemeClr.Gray != nil, fill.SchemeClr.Comp != nil, fill.SchemeClr.Inv != nil, fill.SchemeClr.Gamma != nil, fill.SchemeClr.InvGamma != nil)
	if themeColors != nil {
		if base, ok := themeColors[fill.SchemeClr.Val]; ok {
			solid.Color = solid.ApplyColorTransforms(base)
		}
	}
	return solid
}

func resolveWord2010EffectColor(srgb *parts.Word2010EffectColorRef, scheme *parts.Word2010EffectSchemeColor, themeColors map[string]string) (string, float64) {
	color := ""
	alpha := 1.0
	if srgb != nil {
		color = srgb.Val
		if srgb.Alpha != nil {
			alpha = float64(srgb.Alpha.Val) / 100000.0
		}
		return color, alpha
	}
	if scheme == nil {
		return color, alpha
	}
	if themeColors != nil {
		color = themeColors[scheme.Val]
	}
	if scheme.Alpha != nil {
		alpha = float64(scheme.Alpha.Val) / 100000.0
	}
	return color, alpha
}

func resolveThemeFontSlot(themeFonts map[string]string, slot string) string {
	return resolveThemeFontSlotWithLang(themeFonts, slot, nil)
}

func resolveThemeFontSlotWithLang(themeFonts map[string]string, slot string, lang *parts.RunLang) string {
	if themeFonts == nil {
		return ""
	}
	lower := strings.ToLower(slot)
	var base string
	switch {
	case strings.HasPrefix(lower, "minor"):
		base = "minor"
	case strings.HasPrefix(lower, "major"):
		base = "major"
	default:
		return ""
	}
	if script := themeFontScriptForSlot(lower, lang); script != "" {
		if v := themeFonts[supplementalThemeFontKey(base, script)]; v != "" {
			return v
		}
	}
	// Determine script variant: ascii/hAnsi → Latin (default), eastAsia/bidi → CS/EastAsia.
	switch {
	case strings.Contains(lower, "eastasia"):
		if v := themeFonts[base+"EA"]; v != "" {
			return v
		}
		if v := themeFonts[base+"EastAsia"]; v != "" {
			return v
		}
	case strings.Contains(lower, "bidi") || strings.Contains(lower, "cs") || strings.Contains(lower, "complex"):
		if v := themeFonts[base+"CS"]; v != "" {
			return v
		}
		if v := themeFonts[base+"ComplexScript"]; v != "" {
			return v
		}
	}
	return themeFonts[base]
}

func themeFontScriptForSlot(lowerSlot string, lang *parts.RunLang) string {
	if lang == nil {
		return ""
	}
	switch {
	case strings.Contains(lowerSlot, "eastasia"):
		if script := bcp47ToThemeScript(lang.EastAsia); script != "" {
			return script
		}
		return bcp47ToThemeScript(lang.Val)
	case strings.Contains(lowerSlot, "bidi") || strings.Contains(lowerSlot, "cs") || strings.Contains(lowerSlot, "complex"):
		if script := bcp47ToThemeScript(lang.Bidi); script != "" {
			return script
		}
		return bcp47ToThemeScript(lang.Val)
	default:
		return ""
	}
}

func onOffPtr(value *parts.OnOff) *bool {
	if value == nil {
		return nil
	}
	if value.Val == nil {
		return boolPtr(true)
	}
	v := bool(*value.Val)
	return &v
}

func boolPtr(b bool) *bool {
	return &b
}

func onOffBool(value *parts.OnOff) bool {
	if value == nil {
		return false
	}
	if value.Val == nil {
		return true
	}
	return bool(*value.Val)
}

// firstString returns the value of the first non-nil StringVal, or fallback.
func firstString(val *parts.StringVal, fallback string) string {
	if val != nil {
		return val.Val
	}
	return fallback
}

// firstBool returns true if the OnOff is present and on, otherwise fallback.
func firstBool(val *parts.OnOff, fallback bool) bool {
	if val != nil {
		return onOffBool(val)
	}
	return fallback
}

// firstInt returns the IntVal value if present, otherwise fallback.
func firstInt(val *parts.IntVal, fallback int) int {
	if val != nil {
		return val.Val
	}
	return fallback
}

// resolveFontSlot maps a DrawingML theme font slot name (e.g. "minorHAnsi", "majorAscii")
// to a concrete typeface from the theme's font scheme.
func resolveFontSlot(ctx *resolveCtx, slot string) string {
	if ctx.themeFonts == nil {
		return ""
	}
	return resolveFontSlotWithLang(ctx, slot, nil)
}

func resolveFontSlotWithLang(ctx *resolveCtx, slot string, lang *parts.RunLang) string {
	if ctx.themeFonts == nil {
		return ""
	}
	return resolveThemeFontSlotWithLang(ctx.themeFonts, slot, lang)
}

func cloneRunFormat(f RunFormat) RunFormat {
	r := RunFormat{
		Underline:      f.Underline,
		UnderlineColor: f.UnderlineColor,
		Color:          f.Color,
		Background:     f.Background,
		FontASCII:      f.FontASCII,
		FontHAnsi:      f.FontHAnsi,
		FontEastAsia:   f.FontEastAsia,
		FontCS:         f.FontCS,
		TextFill:       f.TextFill,
		Ligatures:      f.Ligatures,
		NumberSpacing:  f.NumberSpacing,
		Highlight:      f.Highlight,
		VertAlignVal:   f.VertAlignVal,
		Lang:           f.Lang,
	}
	if f.TextOutline != nil {
		outline := *f.TextOutline
		r.TextOutline = &outline
	}
	if f.TextGlow != nil {
		glow := *f.TextGlow
		r.TextGlow = &glow
	}
	if f.TextShadow != nil {
		shadow := *f.TextShadow
		r.TextShadow = &shadow
	}
	if f.TextSoftEdge != nil {
		se := *f.TextSoftEdge
		r.TextSoftEdge = &se
	}
	if f.TextReflection != nil {
		refl := *f.TextReflection
		r.TextReflection = &refl
	}
	if len(f.StylisticSets) > 0 {
		r.StylisticSets = append([]int(nil), f.StylisticSets...)
	}
	if f.Bold != nil {
		r.Bold = boolPtr(*f.Bold)
	}
	if f.Italic != nil {
		r.Italic = boolPtr(*f.Italic)
	}
	if f.Caps != nil {
		r.Caps = boolPtr(*f.Caps)
	}
	if f.SmallCaps != nil {
		r.SmallCaps = boolPtr(*f.SmallCaps)
	}
	if f.Strike != nil {
		r.Strike = boolPtr(*f.Strike)
	}
	if f.FontSize != nil {
		r.FontSize = intPtr(*f.FontSize)
	}
	if f.FontSizeCS != nil {
		r.FontSizeCS = intPtr(*f.FontSizeCS)
	}
	if f.BoldCS != nil {
		r.BoldCS = boolPtr(*f.BoldCS)
	}
	if f.ItalicCS != nil {
		r.ItalicCS = boolPtr(*f.ItalicCS)
	}
	if f.Vanish != nil {
		r.Vanish = boolPtr(*f.Vanish)
	}
	if f.SpecVanish != nil {
		r.SpecVanish = boolPtr(*f.SpecVanish)
	}
	if f.NoProof != nil {
		r.NoProof = boolPtr(*f.NoProof)
	}
	if f.RTL != nil {
		r.RTL = boolPtr(*f.RTL)
	}
	if f.Dir != "" {
		r.Dir = f.Dir
	}
	if f.Bdo != "" {
		r.Bdo = f.Bdo
	}
	if f.CharacterSpacing != nil {
		r.CharacterSpacing = intPtr(*f.CharacterSpacing)
	}
	if f.FitTextWidth != nil {
		r.FitTextWidth = intPtr(*f.FitTextWidth)
	}
	if f.Outline != nil {
		r.Outline = boolPtr(*f.Outline)
	}
	if f.Shadow != nil {
		r.Shadow = boolPtr(*f.Shadow)
	}
	if f.Emboss != nil {
		r.Emboss = boolPtr(*f.Emboss)
	}
	if f.Imprint != nil {
		r.Imprint = boolPtr(*f.Imprint)
	}
	if f.EmphasisMark != "" {
		r.EmphasisMark = f.EmphasisMark
	}
	return r
}

func mergeRunFormat(base, overlay RunFormat) RunFormat {
	r := cloneRunFormat(base)
	if overlay.Bold != nil {
		r.Bold = boolPtr(*overlay.Bold)
	}
	if overlay.Italic != nil {
		r.Italic = boolPtr(*overlay.Italic)
	}
	if overlay.Caps != nil {
		r.Caps = boolPtr(*overlay.Caps)
	}
	if overlay.SmallCaps != nil {
		r.SmallCaps = boolPtr(*overlay.SmallCaps)
	}
	if overlay.Strike != nil {
		r.Strike = boolPtr(*overlay.Strike)
	}
	if overlay.Underline != "" {
		r.Underline = overlay.Underline
	}
	if overlay.UnderlineColor != "" {
		r.UnderlineColor = overlay.UnderlineColor
	}
	if overlay.Color != "" {
		r.Color = overlay.Color
	}
	if overlay.FontSize != nil {
		r.FontSize = intPtr(*overlay.FontSize)
	}
	if overlay.FontSizeCS != nil {
		r.FontSizeCS = intPtr(*overlay.FontSizeCS)
	}
	if overlay.BoldCS != nil {
		r.BoldCS = boolPtr(*overlay.BoldCS)
	}
	if overlay.ItalicCS != nil {
		r.ItalicCS = boolPtr(*overlay.ItalicCS)
	}
	if overlay.Highlight != "" {
		r.Highlight = overlay.Highlight
	}
	if overlay.VertAlignVal != "" {
		r.VertAlignVal = overlay.VertAlignVal
	}
	if overlay.FontASCII != "" {
		r.FontASCII = overlay.FontASCII
	}
	if overlay.FontHAnsi != "" {
		r.FontHAnsi = overlay.FontHAnsi
	}
	if overlay.FontEastAsia != "" {
		r.FontEastAsia = overlay.FontEastAsia
	}
	if overlay.FontCS != "" {
		r.FontCS = overlay.FontCS
	}
	if overlay.TextFill != nil {
		r.TextFill = overlay.TextFill
	}
	if overlay.TextOutline != nil {
		outline := *overlay.TextOutline
		r.TextOutline = &outline
	}
	if overlay.TextGlow != nil {
		glow := *overlay.TextGlow
		r.TextGlow = &glow
	}
	if overlay.TextShadow != nil {
		shadow := *overlay.TextShadow
		r.TextShadow = &shadow
	}
	if overlay.TextSoftEdge != nil {
		se := *overlay.TextSoftEdge
		r.TextSoftEdge = &se
	}
	if overlay.TextReflection != nil {
		refl := *overlay.TextReflection
		r.TextReflection = &refl
	}
	if overlay.Ligatures != "" {
		r.Ligatures = overlay.Ligatures
	}
	if overlay.NumberSpacing != "" {
		r.NumberSpacing = overlay.NumberSpacing
	}
	if len(overlay.StylisticSets) > 0 {
		r.StylisticSets = append([]int(nil), overlay.StylisticSets...)
	}
	if overlay.Background != "" {
		r.Background = overlay.Background
	}
	if overlay.Vanish != nil {
		r.Vanish = boolPtr(*overlay.Vanish)
	}
	if overlay.SpecVanish != nil {
		r.SpecVanish = boolPtr(*overlay.SpecVanish)
	}
	if overlay.NoProof != nil {
		r.NoProof = boolPtr(*overlay.NoProof)
	}
	if overlay.RTL != nil {
		r.RTL = boolPtr(*overlay.RTL)
	}
	if overlay.Dir != "" {
		r.Dir = overlay.Dir
	}
	if overlay.Bdo != "" {
		r.Bdo = overlay.Bdo
	}
	if overlay.Lang != "" {
		r.Lang = overlay.Lang
	}
	if overlay.CharacterSpacing != nil {
		r.CharacterSpacing = intPtr(*overlay.CharacterSpacing)
	}
	if overlay.FitTextWidth != nil {
		r.FitTextWidth = intPtr(*overlay.FitTextWidth)
	}
	if overlay.Outline != nil {
		r.Outline = boolPtr(*overlay.Outline)
	}
	if overlay.Shadow != nil {
		r.Shadow = boolPtr(*overlay.Shadow)
	}
	if overlay.Emboss != nil {
		r.Emboss = boolPtr(*overlay.Emboss)
	}
	if overlay.Imprint != nil {
		r.Imprint = boolPtr(*overlay.Imprint)
	}
	if overlay.EmphasisMark != "" {
		r.EmphasisMark = overlay.EmphasisMark
	}
	return r
}

func semanticRunKind(target string) string {
	if target != "" {
		return "hyperlink"
	}
	return "run"
}

// symToRune converts a w:sym w:char hex string to a rune.
// Word stores private-use Wingdings/Symbol code points with the 0xF000 offset;
// strip that to get the real Unicode point. ECMA-376 Part 1 §17.3.3.30.
func symToRune(hexChar string) rune {
	hexChar = strings.TrimSpace(hexChar)
	if hexChar == "" {
		return 0
	}
	var v int64
	_, err := fmt.Sscanf(hexChar, "%X", &v)
	if err != nil {
		return 0
	}
	// Strip private-use plane offset used by legacy symbol fonts.
	if v >= 0xF000 && v <= 0xF0FF {
		v -= 0xF000
	}
	return rune(v)
}

// applyWordColorModifier applies a w:themeShade or w:themeTint modifier to a base hex RGB color
// using the same HSL-based DrawingML pipeline that shapes, effects, and charts use.
// shade darkens, tint lightens. Only one of shade/tint is applied (shade takes precedence).
// ECMA-376 Part 1 §17.3.2.6.
func applyWordColorModifier(hexRGB, shade, tint string) string {
	if hexRGB == "" || (shade == "" && tint == "") {
		return hexRGB
	}
	fill := &dml.SolidFill{}
	if shade != "" {
		var factor int64
		if _, err := fmt.Sscanf(shade, "%X", &factor); err == nil && factor != 0xFF {
			fill.Shade = float64(factor) / 255.0
			fill.HasShade = true
		}
	}
	if tint != "" {
		var factor int64
		if _, err := fmt.Sscanf(tint, "%X", &factor); err == nil && factor != 0xFF {
			fill.Tint = float64(factor) / 255.0
			fill.HasTint = true
		}
	}
	return fill.ApplyColorTransforms(hexRGB)
}

// resolveShadingFill returns the resolved hex RGB fill color for a Shading
// element, taking theme colors and tint/shade modifiers into account.
func resolveShadingFill(shading *parts.Shading, themeColors map[string]string) string {
	if shading == nil {
		return ""
	}
	fill := shading.Fill
	if fill == "" || fill == "auto" {
		fill = shading.Color
	}
	if fill == "" || fill == "auto" {
		return ""
	}
	if shading.ThemeFill != "" && themeColors != nil {
		if resolved, ok := themeColors[shading.ThemeFill]; ok {
			fill = resolved
		}
	}
	fill = applyWordColorModifier(fill, shading.ThemeFillShade, shading.ThemeFillTint)
	return fill
}

// parseHexRGB splits a 6-char hex color string into r, g, b components.
func parseHexRGB(hex string) (r, g, b uint8, ok bool) {
	hex = strings.TrimSpace(hex)
	if len(hex) != 6 {
		return 0, 0, 0, false
	}
	var v int64
	_, err := fmt.Sscanf(hex, "%06X", &v)
	if err != nil {
		return 0, 0, 0, false
	}
	return uint8(v >> 16), uint8(v >> 8), uint8(v), true
}

// runFormatsEqual reports whether two RunFormat values produce identical
// HTML output, used to decide whether adjacent runs can be coalesced.
func runFormatsEqual(a, b RunFormat) bool {
	return boolPtrsEq(a.Bold, b.Bold) &&
		boolPtrsEq(a.Italic, b.Italic) &&
		boolPtrsEq(a.Caps, b.Caps) &&
		boolPtrsEq(a.SmallCaps, b.SmallCaps) &&
		boolPtrsEq(a.Strike, b.Strike) &&
		a.Underline == b.Underline &&
		a.UnderlineColor == b.UnderlineColor &&
		a.Color == b.Color &&
		a.Background == b.Background &&
		intPtrsEq(a.FontSize, b.FontSize) &&
		intPtrsEq(a.FontSizeCS, b.FontSizeCS) &&
		boolPtrsEq(a.BoldCS, b.BoldCS) &&
		boolPtrsEq(a.ItalicCS, b.ItalicCS) &&
		a.FontASCII == b.FontASCII &&
		a.FontHAnsi == b.FontHAnsi &&
		a.FontEastAsia == b.FontEastAsia &&
		a.FontCS == b.FontCS &&
		a.Highlight == b.Highlight &&
		a.VertAlignVal == b.VertAlignVal &&
		boolPtrsEq(a.Vanish, b.Vanish) &&
		boolPtrsEq(a.SpecVanish, b.SpecVanish) &&
		boolPtrsEq(a.NoProof, b.NoProof) &&
		a.Lang == b.Lang &&
		boolPtrsEq(a.RTL, b.RTL) &&
		a.Dir == b.Dir &&
		a.Bdo == b.Bdo &&
		intPtrsEq(a.CharacterSpacing, b.CharacterSpacing)
}

func boolPtrsEq(a, b *bool) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func intPtrsEq(a, b *int) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

// fieldsEqual reports whether two []Field slices are shallow-equal.
func fieldsEqual(a, b []Field) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Kind != b[i].Kind || a[i].Instr != b[i].Instr || a[i].Arg != b[i].Arg ||
			a[i].Anchor != b[i].Anchor || a[i].Target != b[i].Target {
			return false
		}
	}
	return true
}

func fieldInstrRunsEqual(a, b []Run) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Text != b[i].Text ||
			a[i].Kind != b[i].Kind ||
			a[i].FieldKind != b[i].FieldKind ||
			a[i].FieldInstr != b[i].FieldInstr ||
			a[i].FieldArg != b[i].FieldArg ||
			a[i].Target != b[i].Target ||
			a[i].Anchor != b[i].Anchor ||
			!fieldsEqual(a[i].Fields, b[i].Fields) {
			return false
		}
	}
	return true
}

func appendSemanticRuns(paragraph *Paragraph, runs []Run) {
	for _, run := range runs {
		// Coalesce adjacent text/hyperlink runs that share identical formatting.
		// OOXML splits text across many small runs for revision-tracking reasons;
		// merging them here reduces HTML noise and span count for all renderers.
		if (run.Kind == "run" || run.Kind == "hyperlink") && len(paragraph.Runs) > 0 {
			last := &paragraph.Runs[len(paragraph.Runs)-1]
			if last.Kind == run.Kind &&
				last.Target == run.Target &&
				last.Anchor == run.Anchor &&
				last.Revision == run.Revision &&
				last.RevisionAuthor == run.RevisionAuthor &&
				last.RevisionDate == run.RevisionDate &&
				last.FieldKind == run.FieldKind &&
				last.FieldInstr == run.FieldInstr &&
				last.FieldArg == run.FieldArg &&
				fieldInstrRunsEqual(last.FieldInstrRuns, run.FieldInstrRuns) &&
				fieldsEqual(last.Fields, run.Fields) &&
				contentControlsEqual(last.ContentControl, run.ContentControl) &&
				runFormatsEqual(last.Format, run.Format) {
				last.Text += run.Text
				paragraph.Text += run.Text
				continue
			}
		}
		paragraph.Runs = append(paragraph.Runs, run)
		paragraph.Text += run.Text
	}
}

func normalizeBodyBlocks(ctx *resolveCtx, pkg *parts.Package, blocks []parts.BodyBlock) []Block {
	var out []Block
	var outParagraphs []Paragraph
	for _, block := range blocks {
		switch block.Kind {
		case parts.BodyBlockParagraph:
			if ctx.inField() {
				top := ctx.topField()
				top.paraCount++
				top.paraBoundaries = append(top.paraBoundaries, len(top.result))
			}
			paragraph := normalizeParagraph(ctx, block.Paragraph)
			paragraph.Index = ctx.nextParaIndex
			ctx.nextParaIndex++
			if !isSectionMarkerOnlyParagraph(block.Paragraph, &paragraph) {
				outParagraphs = append(outParagraphs, paragraph)
				paragraphCopy := paragraph
				blockOut := Block{Kind: "paragraph", Paragraph: &paragraphCopy}
				out = append(out, blockOut)
				attachParagraphToField(ctx, len(outParagraphs)-1, blockOut)
			}
			if block.Paragraph != nil && block.Paragraph.Properties != nil && block.Paragraph.Properties.Section != nil {
				section := normalizeSection(pkg, block.Paragraph.Properties.Section)
				sectionCopy := section
				out = append(out, Block{Kind: "section", Section: &sectionCopy})
			}
			ctx.storyFields = append(ctx.storyFields, extractStoryFields(ctx, outParagraphs)...)
		case parts.BodyBlockTable:
			table := normalizeTable(ctx, block.Table)
			tableCopy := table
			blockOut := Block{Kind: "table", Table: &tableCopy}
			out = append(out, blockOut)
			attachTableToField(ctx, blockOut)
			ctx.storyFields = append(ctx.storyFields, extractStoryFields(ctx, outParagraphs)...)
		case parts.BodyBlockSection:
			section := normalizeSection(pkg, block.Section)
			sectionCopy := section
			out = append(out, Block{Kind: "section", Section: &sectionCopy})
		case parts.BodyBlockSDT:
			cc := normalizeContentControlWithContext(ctx, block.SdtProps)
			for _, inner := range normalizeSdtBodyBlocks(ctx, pkg, cc, block.SdtBlocks) {
				inner.ContentControl = cc
				out = append(out, inner)
			}
		case parts.BodyBlockCustomXml:
			cx := normalizeCustomXml(block.CustomXml)
			for _, inner := range normalizeCustomXmlBodyBlocks(ctx, pkg, block.CustomXmlBlocks) {
				inner.CustomXml = cx
				out = append(out, inner)
			}
		case parts.BodyBlockAltChunk:
			if block.AltChunk != nil && block.AltChunk.RelID != "" && pkg != nil {
				if part, ok := pkg.ScopedAltChunkParts[parts.RelationshipRef{SourcePart: pkg.MainPartName, ID: block.AltChunk.RelID}]; ok {
					out = append(out, Block{Kind: "altChunk", AltChunk: &AltChunk{
						Data:        append([]byte(nil), part.Data...),
						ContentType: part.ContentType,
					}})
				}
			}
		}
	}
	for len(ctx.fieldStack) > 0 {
		top := ctx.fieldStack[len(ctx.fieldStack)-1]
		ctx.fieldStack = ctx.fieldStack[:len(ctx.fieldStack)-1]
		if top.paraCount > 1 {
			ctx.flushedFields = append(ctx.flushedFields, top)
		}
	}
	ctx.storyFields = append(ctx.storyFields, extractStoryFields(ctx, outParagraphs)...)
	for i := range out {
		out[i].Index = i
	}
	return out
}

func normalizeSdtContentBlocks(ctx *resolveCtx, cc *ContentControl, blocks []parts.ContentBlock) ([]Paragraph, []Table, []Block, string) {
	if expanded, ok := expandSdtGlossaryBlocks(ctx, cc, contentBlockText(blocks)); ok {
		return nil, nil, expanded.Blocks, expanded.Text
	}
	return normalizeContentBlocks(ctx, blocks, nil, nil)
}

func normalizeSdtBodyBlocks(ctx *resolveCtx, pkg *parts.Package, cc *ContentControl, blocks []parts.BodyBlock) []Block {
	if expanded, ok := expandSdtGlossaryBlocks(ctx, cc, bodyBlockText(blocks)); ok {
		return expanded.Blocks
	}
	return normalizeBodyBlocks(ctx, pkg, blocks)
}

func normalizeCustomXml(cx *parts.CustomXml) *CustomXmlInfo {
	if cx == nil {
		return nil
	}
	info := &CustomXmlInfo{
		URI:     cx.URI,
		Element: cx.Element,
	}
	if cx.Properties != nil {
		for _, attr := range cx.Properties.Attributes {
			info.Attributes = append(info.Attributes, CustomXmlAttr{
				URI:   attr.URI,
				Name:  attr.Name,
				Value: attr.Value,
			})
		}
	}
	return info
}

func normalizeCustomXmlBodyBlocks(ctx *resolveCtx, pkg *parts.Package, blocks []parts.BodyBlock) []Block {
	return normalizeBodyBlocks(ctx, pkg, blocks)
}

func normalizeCustomXmlInlineRuns(ctx *resolveCtx, inlines []parts.ParagraphInline, paraRunFmt RunFormat) []Run {
	return normalizeSdtInlineRuns(ctx, inlines, paraRunFmt)
}

type glossaryExpansion struct {
	Blocks []Block
	Text   string
}

func expandSdtGlossaryBlocks(ctx *resolveCtx, cc *ContentControl, fallbackText string) (glossaryExpansion, bool) {
	if ctx == nil || ctx.pkg == nil || ctx.pkg.Glossary == nil || cc == nil {
		return glossaryExpansion{}, false
	}
	if cc.Type != "docPartObj" && cc.Type != "docPartList" {
		return glossaryExpansion{}, false
	}
	if !cc.ShowingPlaceholder && strings.TrimSpace(fallbackText) != "" {
		return glossaryExpansion{}, false
	}
	entry := findGlossaryEntryForContentControl(ctx.pkg.Glossary.DocParts, cc, fallbackText)
	if entry == nil || entry.Body == nil {
		return glossaryExpansion{}, false
	}
	temp := *ctx
	temp.currentPart = ctx.pkg.GlossaryPartName
	_, _, blocks, text := normalizeContentBlocks(&temp, entry.Body.Blocks, entry.Body.Paragraphs, entry.Body.Tables)
	ctx.nextParaIndex = temp.nextParaIndex
	return glossaryExpansion{Blocks: blocks, Text: text}, len(blocks) > 0 || strings.TrimSpace(text) != ""
}

// expandSdtGlossaryInlines resolves an inline-level docPartObj/docPartList SDT against
// the glossary document and returns the expanded runs. If no match is found it returns
// nil, false so the caller can fall back to normalizing the SDT's inline children.
func expandSdtGlossaryInlines(ctx *resolveCtx, cc *ContentControl, fallbackText string) ([]Run, bool) {
	if ctx == nil || ctx.pkg == nil || ctx.pkg.Glossary == nil || cc == nil {
		return nil, false
	}
	if cc.Type != "docPartObj" && cc.Type != "docPartList" {
		return nil, false
	}
	if !cc.ShowingPlaceholder && strings.TrimSpace(fallbackText) != "" {
		return nil, false
	}
	entry := findGlossaryEntryForContentControl(ctx.pkg.Glossary.DocParts, cc, fallbackText)
	if entry == nil || entry.Body == nil {
		return nil, false
	}
	temp := *ctx
	temp.currentPart = ctx.pkg.GlossaryPartName
	story := normalizeStory(&temp, entry.Body.Blocks, entry.Body.Paragraphs, entry.Body.Tables)
	ctx.nextParaIndex = temp.nextParaIndex
	runs := flattenStoryToInlineRuns(story)
	return runs, len(runs) > 0
}

func findGlossaryEntryForContentControl(entries []*parts.GlossaryEntry, cc *ContentControl, fallbackText string) *parts.GlossaryEntry {
	if len(entries) == 0 || cc == nil {
		return nil
	}
	wantName := normalizeAutoTextName(cc.PlaceholderDocPart)
	if wantName != "" {
		for _, entry := range entries {
			if glossaryEntryName(entry) == wantName {
				return entry
			}
		}
	}
	fallbackName := normalizeAutoTextName(fallbackText)
	wantCategory := normalizeAutoTextName(cc.DocPartCategory)
	wantGallery := normalizeAutoTextName(cc.DocPartGallery)
	wantPlaceholderGallery := normalizeAutoTextName(cc.PlaceholderDocPartGallery)
	for _, entry := range entries {
		entryName := glossaryEntryName(entry)
		entryCategory := glossaryEntryCategory(entry)
		if fallbackName != "" && entryName == fallbackName {
			if wantCategory == "" && wantGallery == "" && wantPlaceholderGallery == "" {
				return entry
			}
			if entryCategory != "" && (entryCategory == wantCategory || entryCategory == wantGallery || entryCategory == wantPlaceholderGallery) {
				return entry
			}
		}
	}
	for _, entry := range entries {
		entryCategory := glossaryEntryCategory(entry)
		if entryCategory != "" && (entryCategory == wantCategory || entryCategory == wantGallery || entryCategory == wantPlaceholderGallery) {
			return entry
		}
	}
	return nil
}

func glossaryEntryName(entry *parts.GlossaryEntry) string {
	if entry == nil || entry.Name == nil {
		return ""
	}
	return normalizeAutoTextName(entry.Name.Val)
}

func glossaryEntryCategory(entry *parts.GlossaryEntry) string {
	if entry == nil || entry.Gallery == nil {
		return ""
	}
	return normalizeAutoTextName(entry.Gallery.Val)
}

func contentBlockText(blocks []parts.ContentBlock) string {
	var text []string
	for _, block := range blocks {
		switch block.Kind {
		case parts.ContentBlockParagraph:
			if block.Paragraph != nil {
				text = append(text, paragraphText(block.Paragraph))
			}
		case parts.ContentBlockTable:
			if block.Table != nil {
				text = append(text, tableText(block.Table))
			}
		case parts.ContentBlockSDT:
			if nested := strings.TrimSpace(contentBlockText(block.SdtBlocks)); nested != "" {
				text = append(text, nested)
			}
		case parts.ContentBlockCustomXml:
			if nested := strings.TrimSpace(contentBlockText(block.CustomXmlBlocks)); nested != "" {
				text = append(text, nested)
			}
		}
	}
	return strings.TrimSpace(strings.Join(text, "\n"))
}

func bodyBlockText(blocks []parts.BodyBlock) string {
	var text []string
	for _, block := range blocks {
		switch block.Kind {
		case parts.BodyBlockParagraph:
			if block.Paragraph != nil {
				text = append(text, paragraphText(block.Paragraph))
			}
		case parts.BodyBlockTable:
			if block.Table != nil {
				text = append(text, tableText(block.Table))
			}
		case parts.BodyBlockSDT:
			if nested := strings.TrimSpace(bodyBlockText(block.SdtBlocks)); nested != "" {
				text = append(text, nested)
			}
		case parts.BodyBlockCustomXml:
			if nested := strings.TrimSpace(bodyBlockText(block.CustomXmlBlocks)); nested != "" {
				text = append(text, nested)
			}
		}
	}
	return strings.TrimSpace(strings.Join(text, "\n"))
}

func paragraphText(p *parts.Paragraph) string {
	if p == nil {
		return ""
	}
	var sb strings.Builder
	for _, run := range p.Runs {
		if run == nil {
			continue
		}
		for _, c := range run.Contents {
			if c.Kind == parts.RunContentText && c.Text != nil {
				sb.WriteString(c.Text.Value)
			}
		}
	}
	return sb.String()
}

func inlineText(inlines []parts.ParagraphInline) string {
	var sb strings.Builder
	for _, inline := range inlines {
		if inline.Kind != parts.ParagraphInlineRun || inline.Run == nil {
			continue
		}
		for _, c := range inline.Run.Contents {
			if c.Kind == parts.RunContentText && c.Text != nil {
				sb.WriteString(c.Text.Value)
			}
		}
	}
	return sb.String()
}

func tableText(t *parts.Table) string {
	if t == nil {
		return ""
	}
	var rows []string
	for _, row := range t.Rows {
		if row == nil {
			continue
		}
		var cells []string
		for _, cell := range row.Cells {
			if cell == nil {
				continue
			}
			cellText := strings.TrimSpace(contentBlockText(cell.Blocks))
			if cellText == "" {
				for _, p := range cell.Paragraphs {
					if pText := strings.TrimSpace(paragraphText(p)); pText != "" {
						if cellText != "" {
							cellText += "\n"
						}
						cellText += pText
					}
				}
			}
			if cellText != "" {
				cells = append(cells, cellText)
			}
		}
		if len(cells) > 0 {
			rows = append(rows, strings.Join(cells, "\t"))
		}
	}
	return strings.Join(rows, "\n")
}

func normalizeContentControl(props *parts.SdtProperties) *ContentControl {
	return normalizeContentControlWithContext(nil, props)
}

func normalizeContentControlWithContext(ctx *resolveCtx, props *parts.SdtProperties) *ContentControl {
	if props == nil {
		return nil
	}
	checked := props.Checked || props.Checked14
	dataValue := resolveContentControlBinding(ctx, props)
	cc := &ContentControl{
		ID:                        props.ID,
		Tag:                       props.Tag,
		Title:                     props.Alias,
		Type:                      props.Type,
		Checked:                   checked,
		DateFormat:                props.DateFormat,
		DateFullDate:              props.DateFullDate,
		DateLanguage:              props.DateLanguage,
		DateCalendar:              props.DateCalendar,
		DocPartGallery:            props.DocPartGallery,
		DocPartCategory:           props.DocPartCategory,
		DocPartUnique:             props.DocPartUnique,
		ShowingPlaceholder:        props.ShowingPlcHdr,
		PlaceholderDocPart:        props.PlaceholderDocPart,
		PlaceholderDocPartGallery: props.PlaceholderDocPartGallery,
		DataXPath:                 props.DataXPath,
		DataStoreID:               props.DataStoreID,
		DataValue:                 dataValue,
	}
	if cc.Type == "date" && cc.DateFullDate == "" && dataValue != "" {
		cc.DateFullDate = dataValue
	}
	if len(props.ListItems) > 0 {
		cc.ListItems = make([]SdtListItem, len(props.ListItems))
		for i, item := range props.ListItems {
			cc.ListItems[i] = SdtListItem{DisplayText: item.DisplayText, Value: item.Value}
		}
	}
	return cc
}

func resolveContentControlBinding(ctx *resolveCtx, props *parts.SdtProperties) string {
	if ctx == nil || ctx.pkg == nil || props == nil {
		return ""
	}
	storeID := normalizeCustomXMLStoreID(props.DataStoreID)
	if storeID == "" || strings.TrimSpace(props.DataXPath) == "" || len(ctx.pkg.CustomXMLParts) == 0 {
		return ""
	}
	data, ok := ctx.pkg.CustomXMLParts[storeID]
	if !ok || len(data) == 0 {
		return ""
	}
	root, err := xmltree.Parse(data)
	if err != nil {
		return ""
	}
	node, attrVal := evaluateSimpleXPath(root, props.DataXPath)
	if attrVal != "" {
		return strings.TrimSpace(attrVal)
	}
	if node == nil {
		return ""
	}
	return strings.TrimSpace(node.AllText())
}

// evaluateSimpleXPath evaluates a minimal subset of XPath against an xmltree.Node.
// It returns the matched node and, if the path ends with an attribute selector,
// the attribute value. Supported features:
//   - Slash-separated element names: /root/customer
//   - Descendant axis: //customer or /root//customer
//   - Index predicates: /root/items[1] (1-based)
//   - Attribute selection: /root/customer/@id
//   - Namespace prefixes are stripped (local name only).
func evaluateSimpleXPath(root *xmltree.Node, xpath string) (*xmltree.Node, string) {
	if root == nil {
		return nil, ""
	}
	xpath = strings.TrimSpace(xpath)

	// Extract trailing attribute selector: /a/b/@attr
	attrName := ""
	if at := strings.LastIndex(xpath, "@"); at > 0 {
		prefix := strings.TrimSpace(xpath[:at])
		if len(prefix) > 0 && prefix[len(prefix)-1] == '/' {
			attrName = strings.TrimSpace(xpath[at+1:])
			if cidx := strings.Index(attrName, ":"); cidx >= 0 {
				attrName = attrName[cidx+1:]
			}
			xpath = prefix[:len(prefix)-1]
		}
	}

	parts := splitSimpleXPath(xpath)
	if len(parts) == 0 {
		if attrName != "" {
			if val, ok := root.AttrValue(attrName); ok {
				return root, val
			}
		}
		return nil, ""
	}

	// Build descendant-axis flags by scanning for // in the original xpath.
	descendantFlags := make([]bool, len(parts))
	temp := xpath
	for strings.Contains(temp, "//") {
		idx := strings.Index(temp, "//")
		before := temp[:idx]
		segCount := 0
		for _, s := range strings.Split(before, "/") {
			if strings.TrimSpace(s) != "" {
				segCount++
			}
		}
		if segCount < len(descendantFlags) {
			descendantFlags[segCount] = true
		}
		temp = temp[idx+2:]
	}

	current := root
	if descendantFlags[0] {
		matches := root.FindAll(localXPathName(parts[0]))
		if len(matches) == 0 {
			return nil, ""
		}
		current = matches[0]
	} else if localXPathName(parts[0]) != current.Name.Local {
		return nil, ""
	}

	for i := 1; i < len(parts); i++ {
		part := parts[i]
		name := localXPathName(part)
		if name == "" {
			return nil, ""
		}
		idx := xpathIndex(part)
		if descendantFlags[i] {
			matches := current.FindAll(name)
			if len(matches) == 0 {
				return nil, ""
			}
			if idx > 0 && idx <= len(matches) {
				current = matches[idx-1]
			} else {
				current = matches[0]
			}
		} else {
			children := current.ChildrenNamed(name)
			if len(children) == 0 {
				return nil, ""
			}
			if idx > 0 && idx <= len(children) {
				current = children[idx-1]
			} else {
				current = children[0]
			}
		}
	}

	if attrName != "" {
		if val, ok := current.AttrValue(attrName); ok {
			return current, val
		}
		return nil, ""
	}
	return current, ""
}

func splitSimpleXPath(xpath string) []string {
	xpath = strings.TrimSpace(xpath)
	xpath = strings.TrimPrefix(xpath, "/")
	if xpath == "" {
		return nil
	}
	raw := strings.Split(xpath, "/")
	out := make([]string, 0, len(raw))
	for _, part := range raw {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func localXPathName(segment string) string {
	segment = strings.TrimSpace(segment)
	if segment == "" || strings.HasPrefix(segment, "@") {
		return ""
	}
	if idx := strings.Index(segment, "["); idx >= 0 {
		segment = segment[:idx]
	}
	if idx := strings.Index(segment, ":"); idx >= 0 {
		segment = segment[idx+1:]
	}
	return strings.TrimSpace(segment)
}

// xpathIndex extracts a 1-based index from an XPath segment like "items[1]".
// It returns 0 when no index is present.
func xpathIndex(segment string) int {
	start := strings.Index(segment, "[")
	end := strings.Index(segment, "]")
	if start < 0 || end < 0 || end <= start {
		return 0
	}
	idx, _ := strconv.Atoi(strings.TrimSpace(segment[start+1 : end]))
	return idx
}

func normalizeCustomXMLStoreID(id string) string {
	return strings.ToUpper(strings.TrimSpace(id))
}

func normalizeSdtInlineRuns(ctx *resolveCtx, inlines []parts.ParagraphInline, paraRunFmt RunFormat) []Run {
	var out []Run
	for _, inline := range inlines {
		switch inline.Kind {
		case parts.ParagraphInlineRun:
			out = append(out, normalizeRun(ctx, inline.Run, "", "", paraRunFmt)...)
		case parts.ParagraphInlineHyperlink:
			out = append(out, normalizeHyperlink(ctx, inline.Hyperlink, paraRunFmt)...)
		case parts.ParagraphInlineIns:
			if inline.Run != nil {
				runs := normalizeRun(ctx, inline.Run, "", "", paraRunFmt)
				for i := range runs {
					runs[i].Revision = "inserted"
					runs[i].RevisionAuthor = inline.RevisionAuthor
					runs[i].RevisionDate = inline.RevisionDate
				}
				out = append(out, runs...)
			}
		case parts.ParagraphInlineDel:
			if inline.Run != nil {
				runs := normalizeRun(ctx, inline.Run, "", "", paraRunFmt)
				for i := range runs {
					runs[i].Revision = "deleted"
					runs[i].RevisionAuthor = inline.RevisionAuthor
					runs[i].RevisionDate = inline.RevisionDate
				}
				out = append(out, runs...)
			}
		case parts.ParagraphInlineMoveFrom:
			if inline.Run != nil {
				runs := normalizeRun(ctx, inline.Run, "", "", paraRunFmt)
				for i := range runs {
					runs[i].Revision = "movedFrom"
					runs[i].RevisionAuthor = inline.RevisionAuthor
					runs[i].RevisionDate = inline.RevisionDate
					runs[i].MoveID = inline.MoveID
				}
				out = append(out, runs...)
			}
		case parts.ParagraphInlineMoveTo:
			if inline.Run != nil {
				runs := normalizeRun(ctx, inline.Run, "", "", paraRunFmt)
				for i := range runs {
					runs[i].Revision = "movedTo"
					runs[i].RevisionAuthor = inline.RevisionAuthor
					runs[i].RevisionDate = inline.RevisionDate
					runs[i].MoveID = inline.MoveID
				}
				out = append(out, runs...)
			}
		case parts.ParagraphInlineDir:
			if inline.Run != nil {
				runs := normalizeRun(ctx, inline.Run, "", "", paraRunFmt)
				for i := range runs {
					runs[i].Format.Dir = inline.DirVal
				}
				out = append(out, runs...)
			}
		case parts.ParagraphInlineBdo:
			if inline.Run != nil {
				runs := normalizeRun(ctx, inline.Run, "", "", paraRunFmt)
				for i := range runs {
					runs[i].Format.Bdo = inline.DirVal
				}
				out = append(out, runs...)
			}
		case parts.ParagraphInlineSmartTag:
			if inline.Run != nil {
				runs := normalizeRun(ctx, inline.Run, "", "", paraRunFmt)
				for i := range runs {
					runs[i].SmartTag = &SmartTagInfo{
						URI:     inline.SmartTagURI,
						Element: inline.SmartTagElement,
					}
				}
				out = append(out, runs...)
			}
		case parts.ParagraphInlineMath:
			if inline.Math != nil {
				if mml := mathml.Convert(inline.Math.RawXML); mml != "" {
					out = append(out, Run{Kind: "math", MathML: mml})
				}
			}
		case parts.ParagraphInlineField:
			if inline.Field != nil {
				var paragraph Paragraph
				emitField(ctx, &paragraph, inline.Field.Instr, inline.Field.Runs, paraRunFmt, nil)
				out = append(out, paragraph.Runs...)
			}
		case parts.ParagraphInlineSDT:
			cc := normalizeContentControlWithContext(ctx, inline.SdtProps)
			for _, run := range normalizeSdtInlineRuns(ctx, inline.SdtInlines, paraRunFmt) {
				run.ContentControl = cc
				out = append(out, run)
			}
		case parts.ParagraphInlineRuby:
			if inline.Ruby != nil {
				if r := normalizeRuby(ctx, inline.Ruby, paraRunFmt); r != nil {
					out = append(out, *r)
				}
			}
		case parts.ParagraphInlineCustomXml:
			cx := normalizeCustomXml(inline.CustomXml)
			for _, run := range normalizeCustomXmlInlineRuns(ctx, inline.CustomXmlInlines, paraRunFmt) {
				run.CustomXml = cx
				out = append(out, run)
			}
		}
	}
	return out
}

func contentControlsEqual(a, b *ContentControl) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.ID != b.ID || a.Tag != b.Tag || a.Title != b.Title || a.Type != b.Type ||
		a.Checked != b.Checked || a.DateFormat != b.DateFormat ||
		a.DateFullDate != b.DateFullDate || a.DateLanguage != b.DateLanguage ||
		a.DateCalendar != b.DateCalendar || a.DocPartGallery != b.DocPartGallery ||
		a.DocPartCategory != b.DocPartCategory || a.DocPartUnique != b.DocPartUnique ||
		a.ShowingPlaceholder != b.ShowingPlaceholder ||
		a.PlaceholderDocPart != b.PlaceholderDocPart ||
		a.PlaceholderDocPartGallery != b.PlaceholderDocPartGallery ||
		a.DataXPath != b.DataXPath ||
		a.DataStoreID != b.DataStoreID || a.DataValue != b.DataValue || len(a.ListItems) != len(b.ListItems) {
		return false
	}
	for i := range a.ListItems {
		if a.ListItems[i] != b.ListItems[i] {
			return false
		}
	}
	return true
}

func paragraphFormatFromProperties(properties *parts.ParagraphProperties, themeColors map[string]string) ParagraphFormat {
	var format ParagraphFormat
	if properties == nil {
		return format
	}

	if properties.Justification != nil {
		format.Alignment = properties.Justification.Val
	}
	if properties.Shading != nil {
		if fill := resolveShadingFill(properties.Shading, themeColors); fill != "" {
			format.Shading = fill
		}
	}
	if properties.Spacing != nil {
		sp := properties.Spacing
		if sp.BeforeAutospacing != nil && bool(*sp.BeforeAutospacing) {
			format.SpacingBeforeAuto = true
		} else if sp.Before != nil {
			format.SpacingBefore = intPtr(*sp.Before)
		}
		if sp.AfterAutospacing != nil && bool(*sp.AfterAutospacing) {
			format.SpacingAfterAuto = true
		} else if sp.After != nil {
			format.SpacingAfter = intPtr(*sp.After)
		}
		if sp.BeforeLines != nil {
			format.SpacingBeforeLines = intPtr(*sp.BeforeLines)
		}
		if sp.AfterLines != nil {
			format.SpacingAfterLines = intPtr(*sp.AfterLines)
		}
		if sp.Line != nil {
			format.LineSpacing = intPtr(*sp.Line)
		}
		if sp.LineRule != "" {
			format.LineRule = sp.LineRule
		}
	}
	if properties.Indentation != nil {
		if properties.Indentation.Left != nil {
			format.IndentLeft = intPtr(*properties.Indentation.Left)
		}
		if properties.Indentation.Right != nil {
			format.IndentRight = intPtr(*properties.Indentation.Right)
		}
		if properties.Indentation.FirstLine != nil {
			format.IndentFirstLine = intPtr(*properties.Indentation.FirstLine)
		}
		if properties.Indentation.Hanging != nil {
			format.IndentHanging = intPtr(*properties.Indentation.Hanging)
		}
	}
	if onOffBool(properties.ContextualSpacing) {
		format.ContextualSpacing = true
	}
	if onOffBool(properties.SnapToGrid) {
		format.SnapToGrid = true
	}
	if onOffBool(properties.KeepNext) {
		format.KeepNext = true
	}
	if onOffBool(properties.KeepLines) {
		format.KeepLines = true
	}
	if onOffBool(properties.WidowControl) {
		format.WidowControl = true
	}
	if onOffBool(properties.PageBreakBefore) {
		format.PageBreakBefore = true
	}
	if onOffBool(properties.Bidi) {
		format.Bidi = true
	}
	if properties.TextDirection != nil {
		format.TextDirection = properties.TextDirection.Val
	}
	if onOffBool(properties.MirrorIndents) {
		format.MirrorIndents = true
	}
	if onOffBool(properties.NoAllowOverlap) {
		format.NoAllowOverlap = true
	}
	if onOffBool(properties.SuppressLineNumbers) {
		format.NoLineNumber = true
	}
	if onOffBool(properties.SuppressAutoHyphens) {
		format.NoAutoHyphenation = true
	}
	if onOffBool(properties.AdjustRightInd) {
		format.AdjustRightInd = true
	}
	if onOffBool(properties.BlockQuote) {
		format.BlockQuote = true
	}
	if properties.TextAlignment != nil {
		format.FontAlign = properties.TextAlignment.Val
	}
	if onOffBool(properties.Kinsoku) {
		format.Kinsoku = true
	}
	if properties.WordWrap != nil {
		format.WordWrap = boolPtr(onOffBool(properties.WordWrap))
	}
	if onOffBool(properties.OverflowPunct) {
		format.OverflowPunct = true
	}
	if onOffBool(properties.TopLinePunct) {
		format.TopLinePunct = true
	}
	if onOffBool(properties.AutoSpaceDE) {
		format.AutoSpaceDE = true
	}
	if onOffBool(properties.AutoSpaceDN) {
		format.AutoSpaceDN = true
	}
	if properties.OutlineLvl != nil {
		v := properties.OutlineLvl.Val
		format.OutlineLevel = &v
	}
	if properties.Borders != nil {
		format.Borders = resolveParagraphBorders(properties.Borders)
	}
	for _, tab := range properties.Tabs {
		// Skip "clear" tab stops which remove default stops but don't add a custom one.
		if tab.Val == "clear" {
			continue
		}
		format.TabStops = append(format.TabStops, TabStop{
			Pos:    tab.Pos,
			Algn:   tab.Val,
			Leader: tab.Leader,
		})
	}

	return format
}

func mergeParagraphFormat(base ParagraphFormat, overlay ParagraphFormat) ParagraphFormat {
	result := cloneParagraphFormat(base)

	if overlay.Alignment != "" {
		result.Alignment = overlay.Alignment
	}
	if overlay.Shading != "" {
		result.Shading = overlay.Shading
	}
	if overlay.SpacingBefore != nil {
		result.SpacingBefore = intPtr(*overlay.SpacingBefore)
	}
	if overlay.SpacingAfter != nil {
		result.SpacingAfter = intPtr(*overlay.SpacingAfter)
	}
	if overlay.SpacingBeforeLines != nil {
		result.SpacingBeforeLines = intPtr(*overlay.SpacingBeforeLines)
	}
	if overlay.SpacingAfterLines != nil {
		result.SpacingAfterLines = intPtr(*overlay.SpacingAfterLines)
	}
	if overlay.LineSpacing != nil {
		result.LineSpacing = intPtr(*overlay.LineSpacing)
	}
	if overlay.LineRule != "" {
		result.LineRule = overlay.LineRule
	}
	if overlay.IndentLeft != nil {
		result.IndentLeft = intPtr(*overlay.IndentLeft)
	}
	if overlay.IndentRight != nil {
		result.IndentRight = intPtr(*overlay.IndentRight)
	}
	if overlay.IndentFirstLine != nil {
		result.IndentFirstLine = intPtr(*overlay.IndentFirstLine)
	}
	if overlay.IndentHanging != nil {
		result.IndentHanging = intPtr(*overlay.IndentHanging)
	}
	if overlay.ContextualSpacing {
		result.ContextualSpacing = true
	}
	if overlay.SnapToGrid {
		result.SnapToGrid = true
	}
	if overlay.KeepNext {
		result.KeepNext = true
	}
	if overlay.KeepLines {
		result.KeepLines = true
	}
	if overlay.WidowControl {
		result.WidowControl = true
	}
	if overlay.PageBreakBefore {
		result.PageBreakBefore = true
	}
	if overlay.Bidi {
		result.Bidi = true
	}
	if overlay.MirrorIndents {
		result.MirrorIndents = true
	}
	if overlay.NoAllowOverlap {
		result.NoAllowOverlap = true
	}
	if overlay.NoLineNumber {
		result.NoLineNumber = true
	}
	if overlay.FontAlign != "" {
		result.FontAlign = overlay.FontAlign
	}
	if overlay.Kinsoku {
		result.Kinsoku = true
	}
	if overlay.WordWrap != nil {
		result.WordWrap = boolPtr(*overlay.WordWrap)
	}
	if overlay.OverflowPunct {
		result.OverflowPunct = true
	}
	if overlay.TopLinePunct {
		result.TopLinePunct = true
	}
	if overlay.AutoSpaceDE {
		result.AutoSpaceDE = true
	}
	if overlay.AutoSpaceDN {
		result.AutoSpaceDN = true
	}
	if overlay.AdjustRightInd {
		result.AdjustRightInd = true
	}
	if overlay.BlockQuote {
		result.BlockQuote = true
	}
	if overlay.SpacingBeforeAuto {
		result.SpacingBeforeAuto = true
		result.SpacingBefore = nil
		result.SpacingBeforeLines = nil
	}
	if overlay.SpacingAfterAuto {
		result.SpacingAfterAuto = true
		result.SpacingAfter = nil
		result.SpacingAfterLines = nil
	}
	if overlay.Borders != nil {
		result.Borders = overlay.Borders
	}
	if overlay.OutlineLevel != nil {
		v := *overlay.OutlineLevel
		result.OutlineLevel = &v
	}
	if len(overlay.TabStops) > 0 {
		result.TabStops = append([]TabStop(nil), overlay.TabStops...)
	}

	return result
}

func cloneParagraphFormat(format ParagraphFormat) ParagraphFormat {
	cloned := ParagraphFormat{
		Alignment:         format.Alignment,
		Shading:           format.Shading,
		LineRule:          format.LineRule,
		SnapToGrid:        format.SnapToGrid,
		ContextualSpacing: format.ContextualSpacing,
		KeepNext:          format.KeepNext,
		KeepLines:         format.KeepLines,
		WidowControl:      format.WidowControl,
		PageBreakBefore:   format.PageBreakBefore,
		SpacingBeforeAuto: format.SpacingBeforeAuto,
		SpacingAfterAuto:  format.SpacingAfterAuto,
		Borders:           format.Borders,
		Bidi:              format.Bidi,
		MirrorIndents:     format.MirrorIndents,
		NoAllowOverlap:    format.NoAllowOverlap,
		NoLineNumber:      format.NoLineNumber,
		FontAlign:         format.FontAlign,
		Kinsoku:           format.Kinsoku,
		WordWrap:          format.WordWrap,
		OverflowPunct:     format.OverflowPunct,
		TopLinePunct:      format.TopLinePunct,
		AutoSpaceDE:       format.AutoSpaceDE,
		AutoSpaceDN:       format.AutoSpaceDN,
		AdjustRightInd:    format.AdjustRightInd,
		BlockQuote:        format.BlockQuote,
	}
	if format.SpacingBefore != nil {
		cloned.SpacingBefore = intPtr(*format.SpacingBefore)
	}
	if format.SpacingAfter != nil {
		cloned.SpacingAfter = intPtr(*format.SpacingAfter)
	}
	if format.SpacingBeforeLines != nil {
		cloned.SpacingBeforeLines = intPtr(*format.SpacingBeforeLines)
	}
	if format.SpacingAfterLines != nil {
		cloned.SpacingAfterLines = intPtr(*format.SpacingAfterLines)
	}
	if format.LineSpacing != nil {
		cloned.LineSpacing = intPtr(*format.LineSpacing)
	}
	if format.IndentLeft != nil {
		cloned.IndentLeft = intPtr(*format.IndentLeft)
	}
	if format.IndentRight != nil {
		cloned.IndentRight = intPtr(*format.IndentRight)
	}
	if format.IndentFirstLine != nil {
		cloned.IndentFirstLine = intPtr(*format.IndentFirstLine)
	}
	if format.IndentHanging != nil {
		cloned.IndentHanging = intPtr(*format.IndentHanging)
	}
	if format.OutlineLevel != nil {
		cloned.OutlineLevel = intPtr(*format.OutlineLevel)
	}
	if format.WordWrap != nil {
		cloned.WordWrap = boolPtr(*format.WordWrap)
	}
	if len(format.TabStops) > 0 {
		cloned.TabStops = append([]TabStop(nil), format.TabStops...)
	}
	return cloned
}

func intPtr(value int) *int {
	v := value
	return &v
}

// coreKeywordsToString flattens a CoreKeywords value into a comma-separated
// string.  It prefers explicit <value> children; otherwise falls back to the
// raw text content.
func coreKeywordsToString(kw *parts.CoreKeywords) string {
	if kw == nil {
		return ""
	}
	if len(kw.Values) > 0 {
		vals := make([]string, 0, len(kw.Values))
		for _, v := range kw.Values {
			if s := strings.TrimSpace(v.Value); s != "" {
				vals = append(vals, s)
			}
		}
		return strings.Join(vals, ", ")
	}
	return strings.TrimSpace(kw.Text)
}

// normalizeISODate parses a W3C dateTime string (as found in OOXML core
// properties) and returns it in YYYY-MM-DDTHH:MM:SS form. If the input is
// empty or unparseable, the original string is returned as-is.
func normalizeISODate(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.999999999Z07:00",
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05",
		"2006-01-02T15:04Z07:00",
		"2006-01-02T15:04Z",
		"2006-01-02T15:04",
		"2006-01-02 15:04:05",
		"2006-01-02",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Format("2006-01-02T15:04:05")
		}
	}
	return s
}

func smartArtParagraphFontSize(p *smartart.TextParagraph) float64 {
	if p != nil {
		for _, r := range p.Runs {
			if r.FontSize > 0 {
				return float64(r.FontSize) / 100.0
			}
		}
	}
	return 14
}

func smartArtTextTransform(props dml.TextBodyProperties, cx, cy float64) string {
	rot := float64(props.Rot) / 1000.0
	switch props.Vert {
	case "vert":
		rot += 90
	case "vert270":
		rot -= 90
	}
	if math.Abs(rot) < .0001 {
		return ""
	}
	return fmt.Sprintf(` transform="rotate(%.4f %.2f %.2f)"`, rot, cx, cy)
}

func smartArtAutoNumber(p *smartart.TextParagraph, index int) string {
	n := p.AutoNumStart
	if n <= 0 {
		n = index + 1
	}
	t := strings.ToLower(p.AutoNumType)
	suffix := "."
	if strings.Contains(t, "parenr") {
		suffix = ")"
	}
	prefix := ""
	if strings.Contains(t, "parenboth") {
		prefix, suffix = "(", ")"
	}
	switch {
	case strings.Contains(t, "romanuc"):
		return prefix + strings.ToUpper(smartArtRoman(n)) + suffix
	case strings.Contains(t, "romanlc"):
		return prefix + strings.ToLower(smartArtRoman(n)) + suffix
	case strings.Contains(t, "alphauc"):
		return prefix + string(rune('A'+(n-1)%26)) + suffix
	case strings.Contains(t, "alphalc"):
		return prefix + string(rune('a'+(n-1)%26)) + suffix
	default:
		return prefix + fmt.Sprintf("%d", n) + suffix
	}
}

func smartArtRoman(n int) string {
	if n <= 0 {
		return ""
	}
	vals := []struct {
		v int
		s string
	}{{1000, "M"}, {900, "CM"}, {500, "D"}, {400, "CD"}, {100, "C"}, {90, "XC"}, {50, "L"}, {40, "XL"}, {10, "X"}, {9, "IX"}, {5, "V"}, {4, "IV"}, {1, "I"}}
	var b strings.Builder
	for _, x := range vals {
		for n >= x.v {
			b.WriteString(x.s)
			n -= x.v
		}
	}
	return b.String()
}

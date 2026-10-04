package resolve

import "office/pkg/word/parsers/ooxml/parts"

// RenderMetadata is the compact, body-independent resolver state needed by
// streaming Word layout. It intentionally excludes document body/stories so a
// DOCX reader can resolve formatting without materializing word/document.xml.
type RenderMetadata struct {
	Document  Document
	Numbering map[int]map[int]Numbering
	// themeColors is the resolved theme palette (accent1, text1, …) used when
	// table and run styles reference themeColor/themeFill. Nil without a theme.
	themeColors map[string]string
}

// BuildRenderMetadata resolves document-wide formatting metadata from the
// compact OOXML parts that are safe to preload before streaming the body.
func BuildRenderMetadata(styles *parts.Styles, numbering *parts.Numbering, theme *parts.Theme, settings *parts.Settings, fontTable *parts.FontTable) *RenderMetadata {
	meta := &RenderMetadata{}
	doc := &meta.Document

	var themeFontLang *parts.ThemeFontLang
	if settings != nil {
		themeFontLang = settings.ThemeFontLang
	}
	themeFonts := buildThemeFontIndex(theme, themeFontLang)
	themeColors := buildThemeColorIndex(theme)
	if settings != nil && settings.ColorSchemeMapping != nil {
		applyClrSchemeMapping(themeColors, settings.ColorSchemeMapping)
	}

	meta.themeColors = themeColors
	styleIndex, defaultParagraphStyleID := buildStyleIndex(styles, themeFonts, themeColors)
	resolveStyleInheritance(styleIndex)
	defaultParagraphFormat := buildDefaultParagraphFormat(styles)
	defaultRunFormat := buildDefaultRunFormat(styles, themeFonts, themeColors)
	charStyleIndex, defaultCharStyleID := buildCharacterStyleIndex(styles, themeFonts, themeColors)
	resolveCharStyleInheritance(charStyleIndex)
	resolveCharStyleFormats(charStyleIndex, defaultRunFormat)
	resolveStyleFormats(styleIndex, defaultParagraphFormat, defaultRunFormat)

	doc.Styles = styleIndex
	doc.CharacterStyles = charStyleIndex
	doc.DefaultParagraphStyleID = defaultParagraphStyleID
	doc.DefaultCharacterStyleID = defaultCharStyleID
	doc.DefaultParagraphFormat = defaultParagraphFormat
	doc.DefaultRunFormat = defaultRunFormat
	doc.FontFamilies = resolveFontFamilies(fontTable)
	doc.LatentStyles = resolveLatentStyles(styles)

	themeFmtScheme := buildThemeFormatIndex(theme)
	doc.Theme = buildIRTheme(theme, themeColors, themeFonts, themeFmtScheme)

	if settings != nil {
		if settings.DefaultTabStop != nil && settings.DefaultTabStop.Val > 0 {
			doc.DefaultTabStop = settings.DefaultTabStop.Val
		}
		if settings.DefaultTableStyle != nil {
			doc.DefaultTableStyle = settings.DefaultTableStyle.Val
		}
		if settings.TrackRevisions != nil && onOffBool(settings.TrackRevisions) {
			doc.TrackChangesEnabled = true
		}
		if settings.EvenAndOddHeaders != nil && onOffBool(settings.EvenAndOddHeaders) {
			doc.EvenAndOddHeaders = true
		}
		if settings.Zoom != nil && settings.Zoom.Percent > 0 {
			doc.ZoomPercent = settings.Zoom.Percent
		}
		if settings.FootnoteProps != nil {
			if settings.FootnoteProps.Format != nil {
				doc.FootnoteNumberFormat = settings.FootnoteProps.Format.Val
			}
			if settings.FootnoteProps.Start != nil {
				doc.FootnoteNumberStart = settings.FootnoteProps.Start.Val
			}
			if settings.FootnoteProps.Restart != nil {
				doc.FootnoteRestart = settings.FootnoteProps.Restart.Val
			}
		}
		if settings.EndnoteProps != nil {
			if settings.EndnoteProps.Position != nil {
				doc.EndnotePosition = settings.EndnoteProps.Position.Val
			}
			if settings.EndnoteProps.Format != nil {
				doc.EndnoteNumberFormat = settings.EndnoteProps.Format.Val
			}
			if settings.EndnoteProps.Start != nil {
				doc.EndnoteNumberStart = settings.EndnoteProps.Start.Val
			}
			if settings.EndnoteProps.Restart != nil {
				doc.EndnoteRestart = settings.EndnoteProps.Restart.Val
			}
		}
	}

	meta.Numbering = buildNumberingIndex(numbering, themeFonts, themeColors)
	return meta
}

// ResolveParagraphFormat combines document defaults, the effective paragraph
// style, and direct paragraph formatting. The returned RunFormat is the
// paragraph-level run base used when resolving child runs.
func (m *RenderMetadata) ResolveParagraphFormat(styleID string, direct ParagraphFormat) (ParagraphFormat, RunFormat) {
	if m == nil {
		return direct, RunFormat{}
	}
	doc := &m.Document
	base := cloneParagraphFormat(doc.DefaultParagraphFormat)
	runBase := cloneRunFormat(doc.DefaultRunFormat)
	if styleID == "" {
		styleID = doc.DefaultParagraphStyleID
	}
	if style, ok := doc.Styles[styleID]; ok {
		base = cloneParagraphFormat(style.Format)
		runBase = cloneRunFormat(style.RunFormat)
	}
	return mergeParagraphFormat(base, direct), runBase
}

// ResolveRunFormat combines the paragraph run base, an optional character
// style, and direct run formatting.
func (m *RenderMetadata) ResolveRunFormat(paragraphRunBase RunFormat, styleID string, direct RunFormat) RunFormat {
	base := cloneRunFormat(paragraphRunBase)
	if m != nil && styleID != "" {
		if style, ok := m.Document.CharacterStyles[styleID]; ok {
			// Merge only what the character style and its own chain define. The
			// index stores each character style with the document defaults baked
			// in as a base, but a character style must not override the
			// paragraph's run properties for fields it never sets: Word keeps a
			// 10pt EndnoteText body at 10pt when a run only carries the
			// reference character style, and inflating it to the document
			// default changes that paragraph's line height.
			base = mergeRunFormat(base, characterStyleDefinedFormat(style, m.Document.CharacterStyles))
		}
	}
	return mergeRunFormat(base, direct)
}

// characterStyleDefinedFormat returns what a character style contributes
// itself, without the document default run format the style index uses as a
// base for effective lookup.
func characterStyleDefinedFormat(style CharacterStyle, index map[string]CharacterStyle) RunFormat {
	var defined RunFormat
	for i := len(style.StylePath) - 1; i >= 0; i-- {
		current, ok := index[style.StylePath[i]]
		if !ok {
			continue
		}
		defined = mergeRunFormat(defined, current.RawRunFormat)
	}
	return defined
}

// ResolveParagraphNumbering resolves either a direct numPr reference or the
// numbering reference inherited from the effective paragraph style. The
// returned ListMarker is the renderer-facing list metadata used for bullets and
// numbering labels.
func (m *RenderMetadata) ResolveParagraphNumbering(styleID string, direct *NumberingReference) (*Numbering, *ListMarker) {
	if m == nil {
		return nil, nil
	}
	ref := direct
	if ref == nil {
		if styleID == "" {
			styleID = m.Document.DefaultParagraphStyleID
		}
		if style, ok := m.Document.Styles[styleID]; ok && style.Numbering != nil {
			copy := *style.Numbering
			ref = &copy
		}
	}
	if ref == nil || ref.NumID == 0 {
		return nil, nil
	}
	resolved, ok := resolveNumbering(m.Numbering, ref.NumID, ref.Level)
	if !ok {
		return nil, nil
	}
	numbering := resolved
	return &numbering, buildListMarker(resolved)
}

// TableConditionalTypes returns the w:tblStylePr condition selectors that apply
// to the cell at (rowIdx, colIdx) under the table's w:tblLook flags. Callers
// that assemble tables outside the materialized reader (the streaming bridge)
// use it so conditional table styling resolves identically.
func TableConditionalTypes(rowIdx, totalRows, colIdx, totalCols int, look *parts.TableLook) []string {
	return tableConditionalTypes(rowIdx, totalRows, colIdx, totalCols, look)
}

// ConditionalTableOverlay merges every tblStylePr entry matching types into a
// single cell/run/paragraph overlay. The result layers on top of the table
// style's base formatting and below direct cell properties.
func ConditionalTableOverlay(styles []ConditionalTableStyle, types []string) (TableCellFormat, RunFormat, ParagraphFormat) {
	return buildConditionalOverlay(styles, types)
}

// MergeTableCellFormat layers overlay on base. Exported for the streaming table
// bridge, which resolves styles after its cells are already built.
func MergeTableCellFormat(base, overlay TableCellFormat) TableCellFormat {
	return mergeTableCellFormat(base, overlay)
}

// MergeTableFormat layers overlay on base.
func MergeTableFormat(base, overlay TableFormat) TableFormat {
	return mergeTableFormat(base, overlay)
}

// MergeRunFormat layers overlay on base.
func MergeRunFormat(base, overlay RunFormat) RunFormat {
	return mergeRunFormat(base, overlay)
}

// MergeParagraphFormat layers overlay on base.
func MergeParagraphFormat(base, overlay ParagraphFormat) ParagraphFormat {
	return mergeParagraphFormat(base, overlay)
}

// TableFormatFromProperties converts direct w:tblPr properties to the IR model.
func TableFormatFromProperties(p *parts.TableProperties, themeColors map[string]string) TableFormat {
	return tableFormatFromProperties(p, themeColors)
}

// TableCellFormatFromProperties converts direct w:tcPr properties to the IR model.
func TableCellFormatFromProperties(p *parts.TableCellProperties, themeColors map[string]string) TableCellFormat {
	return tableCellFormatFromProperties(p, themeColors)
}

// ThemeColorIndex exposes the theme palette used when resolving themeColor and
// themeFill references in table styles.
func (m *RenderMetadata) ThemeColorIndex() map[string]string {
	if m == nil {
		return nil
	}
	return m.themeColors
}

package resolve

import "office/pkg/word/parsers/ooxml/parts"

// RenderMetadata is the compact, body-independent resolver state needed by
// streaming Word layout. It intentionally excludes document body/stories so a
// DOCX reader can resolve formatting without materializing word/document.xml.
type RenderMetadata struct {
	Document  Document
	Numbering map[int]map[int]Numbering
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
			base = mergeRunFormat(base, style.Format)
		}
	}
	return mergeRunFormat(base, direct)
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

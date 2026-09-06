package word97

import (
	"fmt"
	"io"

	wordir "office/pkg/word/ir"
	docbinary "office/pkg/word/parsers/doc/binary"
)

type StyleCatalog struct {
	DefaultParagraphStyleID string
	DefaultCharacterStyleID string
	ParagraphStyles         map[string]wordir.Style
	CharacterStyles         map[string]wordir.CharacterStyle
	ParagraphByIndex        map[int]wordir.Style
	CharacterByIndex        map[int]wordir.CharacterStyle
}

func LoadStyleCatalogReaderAt(table io.ReaderAt, tableSize int64, fib []byte, layout docbinary.Word97Layout) *StyleCatalog {
	blob, ok := readTableRange(table, tableSize, fib, layout, fibStyleSheet, 6)
	if !ok {
		return nil
	}
	return BuildStyleCatalog(ParseStyleSheet(blob))
}

func BuildStyleCatalog(rawStyles []RawStyle) *StyleCatalog {
	if len(rawStyles) == 0 {
		return nil
	}

	paragraphByIndex := map[int]wordir.Style{}
	characterByIndex := map[int]wordir.CharacterStyle{}
	for _, raw := range rawStyles {
		switch raw.Kind {
		case 1:
			paragraphByIndex[raw.Index] = wordir.Style{
				ID:           paragraphStyleID(raw),
				Name:         raw.Name,
				RawFormat:    cloneParagraphFormat(raw.RawParaFormat),
				RawRunFormat: cloneRunFormat(raw.RawRunFormat),
				Source: &wordir.StyleSource{
					RawSprms: append([]byte(nil), raw.ParaSprms...),
					RunSprms: append([]byte(nil), raw.RunSprms...),
				},
			}
		case 2:
			characterByIndex[raw.Index] = wordir.CharacterStyle{
				ID:           characterStyleID(raw),
				Name:         raw.Name,
				RawRunFormat: cloneRunFormat(raw.RawRunFormat),
				Source: &wordir.StyleSource{
					RunSprms: append([]byte(nil), raw.RunSprms...),
				},
			}
		}
	}

	for _, raw := range rawStyles {
		switch raw.Kind {
		case 1:
			entry, ok := paragraphByIndex[raw.Index]
			if !ok {
				continue
			}
			if base, ok := paragraphByIndex[raw.BasedOnIndex]; ok {
				entry.BasedOn = base.ID
			}
			if link, ok := characterByIndex[raw.LinkedIndex]; ok {
				entry.LinkedCharID = link.ID
			}
			paragraphByIndex[raw.Index] = entry
		case 2:
			entry, ok := characterByIndex[raw.Index]
			if !ok {
				continue
			}
			if base, ok := characterByIndex[raw.BasedOnIndex]; ok {
				entry.BasedOn = base.ID
			}
			characterByIndex[raw.Index] = entry
		}
	}

	paragraphStyles := make(map[string]wordir.Style, len(paragraphByIndex))
	for _, style := range paragraphByIndex {
		paragraphStyles[style.ID] = style
	}
	characterStyles := make(map[string]wordir.CharacterStyle, len(characterByIndex))
	for _, style := range characterByIndex {
		characterStyles[style.ID] = style
	}

	for id, style := range paragraphStyles {
		style.StylePath = stylePath(paragraphStyles, id)
		var paragraphFormat wordir.ParagraphFormat
		var runFormat wordir.RunFormat
		for i := len(style.StylePath) - 1; i >= 0; i-- {
			current, ok := paragraphStyles[style.StylePath[i]]
			if !ok {
				continue
			}
			paragraphFormat = mergeParagraphFormat(paragraphFormat, current.RawFormat)
			runFormat = mergeRunFormat(runFormat, current.RawRunFormat)
		}
		style.Format = paragraphFormat
		style.RunFormat = runFormat
		paragraphStyles[id] = style
	}

	for id, style := range characterStyles {
		style.StylePath = characterStylePath(characterStyles, id)
		var runFormat wordir.RunFormat
		for i := len(style.StylePath) - 1; i >= 0; i-- {
			current, ok := characterStyles[style.StylePath[i]]
			if !ok {
				continue
			}
			runFormat = mergeRunFormat(runFormat, current.RawRunFormat)
		}
		style.Format = runFormat
		characterStyles[id] = style
	}

	for index, style := range paragraphByIndex {
		if resolved, ok := paragraphStyles[style.ID]; ok {
			paragraphByIndex[index] = resolved
		}
	}
	for index, style := range characterByIndex {
		if resolved, ok := characterStyles[style.ID]; ok {
			characterByIndex[index] = resolved
		}
	}

	catalog := &StyleCatalog{
		ParagraphStyles:  paragraphStyles,
		CharacterStyles:  characterStyles,
		ParagraphByIndex: paragraphByIndex,
		CharacterByIndex: characterByIndex,
	}
	for _, raw := range rawStyles {
		if raw.Kind == 1 && raw.STI == 0 {
			if style, ok := paragraphByIndex[raw.Index]; ok {
				catalog.DefaultParagraphStyleID = style.ID
				entry := paragraphStyles[style.ID]
				entry.Default = true
				paragraphStyles[style.ID] = entry
			}
			break
		}
	}
	for _, raw := range rawStyles {
		if raw.Kind == 2 && raw.STI == 65 {
			if style, ok := characterByIndex[raw.Index]; ok {
				catalog.DefaultCharacterStyleID = style.ID
			}
			break
		}
	}
	return catalog
}

func (c *StyleCatalog) ResolveParagraphFormat(index int) (wordir.ParagraphFormat, bool) {
	if c == nil {
		return wordir.ParagraphFormat{}, false
	}
	style, ok := c.ParagraphByIndex[index]
	if !ok {
		return wordir.ParagraphFormat{}, false
	}
	return cloneParagraphFormat(style.Format), true
}

func (c *StyleCatalog) ApplyToDocument(doc *wordir.Document) {
	if c == nil || doc == nil {
		return
	}
	doc.DefaultParagraphStyleID = c.DefaultParagraphStyleID
	doc.DefaultCharacterStyleID = c.DefaultCharacterStyleID
	doc.Styles = cloneStyleMap(c.ParagraphStyles)
	doc.CharacterStyles = cloneCharacterStyleMap(c.CharacterStyles)
	if style, ok := c.ParagraphStyles[c.DefaultParagraphStyleID]; ok {
		doc.DefaultParagraphFormat = cloneParagraphFormat(style.Format)
		doc.DefaultRunFormat = cloneRunFormat(style.RunFormat)
	} else if style, ok := c.CharacterStyles[c.DefaultCharacterStyleID]; ok {
		doc.DefaultRunFormat = cloneRunFormat(style.Format)
	}

	for i := range doc.Paragraphs {
		paragraph := &doc.Paragraphs[i]
		if index := paragraph.LegacyStyleIndex(); index != nil {
			if style, ok := c.ParagraphByIndex[*index]; ok {
				paragraph.StyleID = style.ID
			}
		}
		for j := range paragraph.Runs {
			run := &paragraph.Runs[j]
			if run.Source == nil || run.Source.StyleIndex == nil {
				continue
			}
			if style, ok := c.CharacterByIndex[*run.Source.StyleIndex]; ok {
				run.StyleID = style.ID
			}
		}
	}
}

func ApplyParagraphStyle(run *Run, styleFormat wordir.ParagraphFormat) {
	if run == nil {
		return
	}
	run.ParagraphFormat = mergeParagraphFormat(styleFormat, run.ParagraphFormat)
	if run.ParagraphFormat.InTable {
		run.LegacyInTableHint = true
	}
}

func paragraphStyleID(raw RawStyle) string {
	switch raw.STI {
	case 0:
		return "Normal"
	case 1:
		return "Heading1"
	case 2:
		return "Heading2"
	case 3:
		return "Heading3"
	case 4:
		return "Heading4"
	case 5:
		return "Heading5"
	case 6:
		return "Heading6"
	case 7:
		return "Heading7"
	case 8:
		return "Heading8"
	case 9:
		return "Heading9"
	case 105:
		return "NoList"
	case 107:
		return "NoSpacing"
	}
	if raw.Name != "" {
		return raw.Name
	}
	return fmt.Sprintf("legacy-paragraph-style-%d", raw.Index)
}

func characterStyleID(raw RawStyle) string {
	if raw.STI == 65 {
		return "DefaultParagraphFont"
	}
	if raw.Name != "" {
		return raw.Name
	}
	return fmt.Sprintf("legacy-character-style-%d", raw.Index)
}

func stylePath(index map[string]wordir.Style, id string) []string {
	var out []string
	seen := map[string]bool{}
	for id != "" && !seen[id] {
		seen[id] = true
		out = append(out, id)
		style, ok := index[id]
		if !ok {
			break
		}
		id = style.BasedOn
	}
	return out
}

func characterStylePath(index map[string]wordir.CharacterStyle, id string) []string {
	var out []string
	seen := map[string]bool{}
	for id != "" && !seen[id] {
		seen[id] = true
		out = append(out, id)
		style, ok := index[id]
		if !ok {
			break
		}
		id = style.BasedOn
	}
	return out
}

func mergeParagraphFormat(base, overlay wordir.ParagraphFormat) wordir.ParagraphFormat {
	result := cloneParagraphFormat(base)
	if overlay.Alignment != "" {
		result.Alignment = overlay.Alignment
	}
	if overlay.Shading != "" {
		result.Shading = overlay.Shading
	}
	copyInt := func(dst **int, src *int) {
		if src != nil {
			value := *src
			*dst = &value
		}
	}
	copyInt(&result.SpacingBefore, overlay.SpacingBefore)
	copyInt(&result.SpacingAfter, overlay.SpacingAfter)
	copyInt(&result.SpacingBeforeLines, overlay.SpacingBeforeLines)
	copyInt(&result.SpacingAfterLines, overlay.SpacingAfterLines)
	copyInt(&result.LineSpacing, overlay.LineSpacing)
	if overlay.LineRule != "" {
		result.LineRule = overlay.LineRule
	}
	copyInt(&result.IndentLeft, overlay.IndentLeft)
	copyInt(&result.IndentRight, overlay.IndentRight)
	copyInt(&result.IndentFirstLine, overlay.IndentFirstLine)
	copyInt(&result.IndentHanging, overlay.IndentHanging)
	if overlay.ContextualSpacing {
		result.ContextualSpacing = true
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
	if overlay.SnapToGrid {
		result.SnapToGrid = true
	}
	if overlay.Bidi {
		result.Bidi = true
	}
	if len(overlay.TabStops) > 0 {
		result.TabStops = append([]wordir.TabStop(nil), overlay.TabStops...)
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
	if overlay.InTable {
		result.InTable = true
	}
	if overlay.IsTableTrailingParagraph {
		result.IsTableTrailingParagraph = true
	}
	if overlay.IsTableCellTrailingParagraph {
		result.IsTableCellTrailingParagraph = true
	}
	copyInt(&result.BinaryListLevel, overlay.BinaryListLevel)
	copyInt(&result.BinaryListOverride, overlay.BinaryListOverride)
	copyInt(&result.BinaryListFormat, overlay.BinaryListFormat)
	copyInt(&result.BinaryStyleIndex, overlay.BinaryStyleIndex)
	copyInt(&result.OutlineLevel, overlay.OutlineLevel)
	if overlay.BinaryFrame != nil {
		value := *overlay.BinaryFrame
		result.BinaryFrame = &value
	}
	return result
}

func cloneParagraphFormat(value wordir.ParagraphFormat) wordir.ParagraphFormat {
	result := value
	cloneInt := func(src *int) *int {
		if src == nil {
			return nil
		}
		value := *src
		return &value
	}
	result.SpacingBefore = cloneInt(value.SpacingBefore)
	result.SpacingAfter = cloneInt(value.SpacingAfter)
	result.SpacingBeforeLines = cloneInt(value.SpacingBeforeLines)
	result.SpacingAfterLines = cloneInt(value.SpacingAfterLines)
	result.LineSpacing = cloneInt(value.LineSpacing)
	result.IndentLeft = cloneInt(value.IndentLeft)
	result.IndentRight = cloneInt(value.IndentRight)
	result.IndentFirstLine = cloneInt(value.IndentFirstLine)
	result.IndentHanging = cloneInt(value.IndentHanging)
	result.BinaryListLevel = cloneInt(value.BinaryListLevel)
	result.BinaryListOverride = cloneInt(value.BinaryListOverride)
	result.BinaryListFormat = cloneInt(value.BinaryListFormat)
	result.BinaryStyleIndex = cloneInt(value.BinaryStyleIndex)
	result.OutlineLevel = cloneInt(value.OutlineLevel)
	result.TabStops = append([]wordir.TabStop(nil), value.TabStops...)
	if value.BinaryFrame != nil {
		frame := *value.BinaryFrame
		result.BinaryFrame = &frame
	}
	return result
}

func mergeRunFormat(base, overlay wordir.RunFormat) wordir.RunFormat {
	result := cloneRunFormat(base)
	copyBool := func(dst **bool, src *bool) {
		if src != nil {
			value := *src
			*dst = &value
		}
	}
	copyInt := func(dst **int, src *int) {
		if src != nil {
			value := *src
			*dst = &value
		}
	}
	copyBool(&result.Bold, overlay.Bold)
	copyBool(&result.Italic, overlay.Italic)
	copyBool(&result.Caps, overlay.Caps)
	copyBool(&result.SmallCaps, overlay.SmallCaps)
	copyBool(&result.Strike, overlay.Strike)
	if overlay.Underline != "" {
		result.Underline = overlay.Underline
	}
	if overlay.UnderlineColor != "" {
		result.UnderlineColor = overlay.UnderlineColor
	}
	if overlay.Color != "" {
		result.Color = overlay.Color
	}
	if overlay.Background != "" {
		result.Background = overlay.Background
	}
	copyInt(&result.FontSize, overlay.FontSize)
	if overlay.FontASCII != "" {
		result.FontASCII = overlay.FontASCII
	}
	if overlay.FontHAnsi != "" {
		result.FontHAnsi = overlay.FontHAnsi
	}
	if overlay.FontEastAsia != "" {
		result.FontEastAsia = overlay.FontEastAsia
	}
	if overlay.FontCS != "" {
		result.FontCS = overlay.FontCS
	}
	if overlay.Highlight != "" {
		result.Highlight = overlay.Highlight
	}
	if overlay.VertAlignVal != "" {
		result.VertAlignVal = overlay.VertAlignVal
	}
	copyBool(&result.Vanish, overlay.Vanish)
	if overlay.Lang != "" {
		result.Lang = overlay.Lang
	}
	copyInt(&result.CharacterSpacing, overlay.CharacterSpacing)
	copyBool(&result.RTL, overlay.RTL)
	copyBool(&result.Outline, overlay.Outline)
	copyBool(&result.Shadow, overlay.Shadow)
	copyBool(&result.Emboss, overlay.Emboss)
	copyBool(&result.Imprint, overlay.Imprint)
	if overlay.EmphasisMark != "" {
		result.EmphasisMark = overlay.EmphasisMark
	}
	return result
}

func cloneRunFormat(value wordir.RunFormat) wordir.RunFormat {
	result := value
	cloneBool := func(src *bool) *bool {
		if src == nil {
			return nil
		}
		value := *src
		return &value
	}
	cloneInt := func(src *int) *int {
		if src == nil {
			return nil
		}
		value := *src
		return &value
	}
	result.Bold = cloneBool(value.Bold)
	result.Italic = cloneBool(value.Italic)
	result.Caps = cloneBool(value.Caps)
	result.SmallCaps = cloneBool(value.SmallCaps)
	result.Strike = cloneBool(value.Strike)
	result.FontSize = cloneInt(value.FontSize)
	result.Vanish = cloneBool(value.Vanish)
	result.CharacterSpacing = cloneInt(value.CharacterSpacing)
	result.RTL = cloneBool(value.RTL)
	result.Outline = cloneBool(value.Outline)
	result.Shadow = cloneBool(value.Shadow)
	result.Emboss = cloneBool(value.Emboss)
	result.Imprint = cloneBool(value.Imprint)
	return result
}

func cloneStyleMap(src map[string]wordir.Style) map[string]wordir.Style {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]wordir.Style, len(src))
	for key, value := range src {
		value.RawFormat = cloneParagraphFormat(value.RawFormat)
		value.Format = cloneParagraphFormat(value.Format)
		value.RawRunFormat = cloneRunFormat(value.RawRunFormat)
		value.RunFormat = cloneRunFormat(value.RunFormat)
		value.StylePath = append([]string(nil), value.StylePath...)
		if value.Source != nil {
			source := *value.Source
			source.RawSprms = append([]byte(nil), value.Source.RawSprms...)
			source.RunSprms = append([]byte(nil), value.Source.RunSprms...)
			value.Source = &source
		}
		out[key] = value
	}
	return out
}

func cloneCharacterStyleMap(src map[string]wordir.CharacterStyle) map[string]wordir.CharacterStyle {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]wordir.CharacterStyle, len(src))
	for key, value := range src {
		value.RawRunFormat = cloneRunFormat(value.RawRunFormat)
		value.Format = cloneRunFormat(value.Format)
		value.StylePath = append([]string(nil), value.StylePath...)
		if value.Source != nil {
			source := *value.Source
			source.RunSprms = append([]byte(nil), value.Source.RunSprms...)
			value.Source = &source
		}
		out[key] = value
	}
	return out
}

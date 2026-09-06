package pre97

import (
	"encoding/binary"
	"fmt"
	"strings"

	wordir "office/pkg/word/ir"
)

// StyleCatalog is the resolved Word 6/95 style view consumed by the sequential
// pre-97 pipeline. It intentionally keeps only format-neutral IR types.
type StyleCatalog struct {
	DefaultParagraphStyleID string
	ParagraphStyles         map[string]wordir.Style
	CharacterStyles         map[string]wordir.CharacterStyle
	ParagraphByIndex        map[int]wordir.Style
	CharacterByIndex        map[int]wordir.CharacterStyle
}

type rawStyle struct {
	Index        int
	STI          int
	Kind         int
	Name         string
	BasedOnIndex int
	LinkedIndex  int
	Para         wordir.ParagraphFormat
	Run          wordir.RunFormat
}

// LoadStyleCatalog parses the Word 6/95 STSH stored in the WordDocument
// secondary span. Names are Pascal-style single-byte strings and UPX payloads
// use the old one-byte SPRM tables decoded by this package.
func LoadStyleCatalog(word []byte, span Span, fontNames []string) *StyleCatalog {
	start, size := span.SecondaryStart, span.SecondarySize
	if start <= 0 || size < 6 || start+size > len(word) { return nil }
	stsh := word[start:start+size]
	if len(stsh) < 2 { return nil }
	cbStshi := int(binary.LittleEndian.Uint16(stsh[:2]))
	if cbStshi < 4 || 2+cbStshi > len(stsh) { return nil }
	stshi := stsh[2:2+cbStshi]
	cstd := int(binary.LittleEndian.Uint16(stshi[:2]))
	baseSize := int(binary.LittleEndian.Uint16(stshi[2:4]))
	if cstd <= 0 || baseSize < 8 || baseSize > 20 { return nil }
	pos := 2 + cbStshi
	raw := make([]rawStyle, 0, cstd)
	for index := 0; index < cstd && pos+2 <= len(stsh); index++ {
		cbStd := int(binary.LittleEndian.Uint16(stsh[pos:pos+2])); pos += 2
		if cbStd == 0 { continue }
		if cbStd < 0 || pos+cbStd > len(stsh) { break }
		if style, ok := parseOldSTD(stsh[pos:pos+cbStd], index, baseSize, fontNames); ok { raw = append(raw, style) }
		pos += cbStd
	}
	if len(raw) == 0 { return nil }
	return buildStyleCatalog(raw)
}

func parseOldSTD(std []byte, index, baseSize int, fontNames []string) (rawStyle, bool) {
	if baseSize < 8 || baseSize > len(std) { return rawStyle{}, false }
	w0 := binary.LittleEndian.Uint16(std[0:2])
	w1 := binary.LittleEndian.Uint16(std[2:4])
	w2 := binary.LittleEndian.Uint16(std[4:6])
	r := rawStyle{Index:index, STI:int(w0&0x0fff), Kind:int(w1&0x000f), BasedOnIndex:int(w1>>4), LinkedIndex:-1}
	if r.BasedOnIndex == 0x0fff { r.BasedOnIndex = -1 }
	cupx := int(w2 & 0x000f)
	name, consumed, ok := parseOldStyleName(std[baseSize:])
	if !ok { return rawStyle{}, false }
	r.Name = name
	pos := baseSize + consumed
	if r.Kind == 1 && cupx >= 1 && pos < len(std) {
		if payload, used := oldUPX(std[pos:]); used > 0 { r.Para, _ = DecodePAPXGrpprl(payload); pos += used }
	}
	if (r.Kind == 1 && cupx >= 2 || r.Kind == 2 && cupx >= 1) && pos < len(std) {
		if payload, _ := oldUPX(std[pos:]); payload != nil { r.Run = DecodeCHPXGrpprl(payload, fontNames) }
	}
	if r.Kind < 1 || r.Kind > 3 { return rawStyle{}, false }
	return r, true
}

func parseOldStyleName(data []byte) (string, int, bool) {
	if len(data) < 2 { return "", 0, false }
	n := int(data[0]); need := 1+n+1
	if need > len(data) { return "", 0, false }
	name := string(data[1:1+n])
	if i := strings.IndexByte(name, ','); i >= 0 { name = name[:i] }
	name = strings.TrimSpace(name)
	consumed := need; if consumed&1 != 0 { consumed++ }
	if name == "" { return "", 0, false }
	return name, consumed, true
}

func oldUPX(data []byte) ([]byte, int) {
	if len(data) < 2 { return nil, 0 }
	n := int(binary.LittleEndian.Uint16(data[:2]))
	if n < 0 || 2+n > len(data) { return nil, 0 }
	used := 2+n; if used&1 != 0 { used++ }
	return data[2:2+n], used
}

func buildStyleCatalog(raw []rawStyle) *StyleCatalog {
	p := map[int]wordir.Style{}
	c := map[int]wordir.CharacterStyle{}
	for _, r := range raw {
		switch r.Kind {
		case 1:
			id := paragraphStyleID(r)
			p[r.Index] = wordir.Style{ID:id, Name:r.Name, RawFormat:r.Para, RawRunFormat:r.Run}
		case 2:
			id := characterStyleID(r)
			c[r.Index] = wordir.CharacterStyle{ID:id, Name:r.Name, RawRunFormat:r.Run}
		}
	}
	for _, r := range raw {
		if r.Kind == 1 {
			s := p[r.Index]
			if b, ok := p[r.BasedOnIndex]; ok { s.BasedOn = b.ID }
			if l, ok := c[r.LinkedIndex]; ok { s.LinkedCharID = l.ID }
			p[r.Index] = s
		} else if r.Kind == 2 {
			s := c[r.Index]; if b, ok := c[r.BasedOnIndex]; ok { s.BasedOn = b.ID }; c[r.Index] = s
		}
	}
	pByID := map[string]wordir.Style{}; for _, s := range p { pByID[s.ID] = s }
	for idx, s := range p { s.Format = resolveParagraphFormat(s.ID, pByID, map[string]bool{}); p[idx] = s; pByID[s.ID] = s }
	cByID := map[string]wordir.CharacterStyle{}; for _, s := range c { cByID[s.ID] = s }
	for idx, s := range c { s.Format = resolveRunFormat(s.ID, cByID, map[string]bool{}); c[idx] = s; cByID[s.ID] = s }
	def := ""
	for _, r := range raw { if r.Kind == 1 && r.STI == 0 { if s, ok := p[r.Index]; ok { def = s.ID }; break } }
	return &StyleCatalog{DefaultParagraphStyleID:def, ParagraphStyles:pByID, CharacterStyles:cByID, ParagraphByIndex:p, CharacterByIndex:c}
}

func resolveParagraphFormat(id string, styles map[string]wordir.Style, seen map[string]bool) wordir.ParagraphFormat {
	s, ok := styles[id]; if !ok || seen[id] { return wordir.ParagraphFormat{} }
	seen[id] = true
	base := wordir.ParagraphFormat{}
	if s.BasedOn != "" { base = resolveParagraphFormat(s.BasedOn, styles, seen) }
	return mergeParagraphFormat(base, s.RawFormat)
}

func resolveRunFormat(id string, styles map[string]wordir.CharacterStyle, seen map[string]bool) wordir.RunFormat {
	s, ok := styles[id]; if !ok || seen[id] { return wordir.RunFormat{} }
	seen[id] = true
	base := wordir.RunFormat{}
	if s.BasedOn != "" { base = resolveRunFormat(s.BasedOn, styles, seen) }
	return mergeRunFormat(base, s.RawRunFormat)
}

func mergeParagraphFormat(base, direct wordir.ParagraphFormat) wordir.ParagraphFormat {
	out := base
	if direct.Alignment != "" { out.Alignment = direct.Alignment }
	if direct.IndentLeft != nil { out.IndentLeft = direct.IndentLeft }
	if direct.IndentRight != nil { out.IndentRight = direct.IndentRight }
	if direct.IndentFirstLine != nil { out.IndentFirstLine = direct.IndentFirstLine }
	if direct.IndentHanging != nil { out.IndentHanging = direct.IndentHanging }
	if direct.SpacingBefore != nil { out.SpacingBefore = direct.SpacingBefore }
	if direct.SpacingAfter != nil { out.SpacingAfter = direct.SpacingAfter }
	if direct.LineSpacing != nil { out.LineSpacing = direct.LineSpacing }
	if len(direct.TabStops) > 0 { out.TabStops = append([]wordir.TabStop(nil), direct.TabStops...) }
	if direct.Shading != "" { out.Shading = direct.Shading }
	if direct.Borders != nil { out.Borders = direct.Borders }
	if direct.InTable { out.InTable = true }
	if direct.KeepNext { out.KeepNext = true }
	if direct.KeepLines { out.KeepLines = true }
	if direct.WidowControl { out.WidowControl = true }
	if direct.PageBreakBefore { out.PageBreakBefore = true }
	if direct.ContextualSpacing { out.ContextualSpacing = true }
	if direct.BinaryListOverride != nil { out.BinaryListOverride = direct.BinaryListOverride }
	if direct.BinaryListLevel != nil { out.BinaryListLevel = direct.BinaryListLevel }
	if direct.BinaryListFormat != nil { out.BinaryListFormat = direct.BinaryListFormat }
	return out
}

func mergeRunFormat(base, direct wordir.RunFormat) wordir.RunFormat {
	out := base
	if direct.Bold != nil { out.Bold = direct.Bold }; if direct.Italic != nil { out.Italic = direct.Italic }
	if direct.Strike != nil { out.Strike = direct.Strike }; if direct.SmallCaps != nil { out.SmallCaps = direct.SmallCaps }
	if direct.Caps != nil { out.Caps = direct.Caps }; if direct.Vanish != nil { out.Vanish = direct.Vanish }
	if direct.FontASCII != "" { out.FontASCII = direct.FontASCII }; if direct.FontHAnsi != "" { out.FontHAnsi = direct.FontHAnsi }
	if direct.FontSize != nil { out.FontSize = direct.FontSize }; if direct.Color != "" { out.Color = direct.Color }
	if direct.Underline != "" { out.Underline = direct.Underline }; if direct.VertAlignVal != "" { out.VertAlignVal = direct.VertAlignVal }
	return out
}

func paragraphStyleID(r rawStyle) string {
	switch r.STI { case 0:return "Normal"; case 1:return "Heading1"; case 2:return "Heading2"; case 3:return "Heading3"; case 4:return "Heading4"; case 5:return "Heading5"; case 6:return "Heading6"; case 7:return "Heading7"; case 8:return "Heading8"; case 9:return "Heading9"; case 105:return "NoList"; case 107:return "NoSpacing" }
	if r.Name != "" { return r.Name }; return fmt.Sprintf("pre97-paragraph-style-%d", r.Index)
}
func characterStyleID(r rawStyle) string { if r.STI == 65 { return "DefaultParagraphFont" }; if r.Name != "" { return r.Name }; return fmt.Sprintf("pre97-character-style-%d", r.Index) }

// NewStyleCursor overlays resolved style formatting only where direct PAPX has
// not already supplied a value.
func NewStyleCursor(inner Cursor, catalog *StyleCatalog) Cursor {
	if inner == nil || catalog == nil { return inner }
	return &styleCursor{inner:inner, catalog:catalog}
}
type styleCursor struct { inner Cursor; catalog *StyleCatalog }
func (c *styleCursor) Next() (Run, bool, error) {
	r, ok, err := c.inner.Next(); if err != nil || !ok { return r, ok, err }
	if r.ParagraphFormat.BinaryStyleIndex == nil { return r, true, nil }
	s, ok := c.catalog.ParagraphByIndex[*r.ParagraphFormat.BinaryStyleIndex]; if !ok { return r, true, nil }
	r.ParagraphFormat = mergeParagraphFormat(s.Format, r.ParagraphFormat)
	if r.ParagraphFormat.InTable { r.LegacyInTableHint = true; if strings.HasSuffix(r.Text, "\a") { r.ParagraphFormat.IsTableCellTrailingParagraph = true } }
	return r, true, nil
}

package legacy

import (
	"bytes"
	"encoding/binary"
	"sort"
	"strings"

	wordir "office/pkg/word/ir"
)

type legacyStyleWriteEntry struct {
	Index      uint16
	ID         string
	Name       string
	Kind       int // 1 paragraph, 2 character
	STI        int
	BasedOn    string
	Linked     string
	ParaFormat wordir.ParagraphFormat
	RunFormat  wordir.RunFormat
	ParaSource []byte
	RunSource  []byte
}

type legacyStyleWriteCatalog struct {
	Entries          []legacyStyleWriteEntry
	ParagraphByID    map[string]uint16
	CharacterByID    map[string]uint16
	DefaultParagraph uint16
}

func (c legacyStyleWriteCatalog) paragraphIndex(id string) uint16 {
	id = strings.TrimSpace(id)
	if id != "" {
		if index, ok := c.ParagraphByID[id]; ok {
			return index
		}
	}
	return c.DefaultParagraph
}

func (c legacyStyleWriteCatalog) characterIndex(id string) *uint16 {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}
	index, ok := c.CharacterByID[id]
	if !ok {
		return nil
	}
	return &index
}

func buildLegacyStyleWriteCatalog(doc *wordir.Document) legacyStyleWriteCatalog {
	catalog := legacyStyleWriteCatalog{
		ParagraphByID: make(map[string]uint16),
		CharacterByID: make(map[string]uint16),
	}

	paragraphs := make([]wordir.Style, 0, len(doc.Styles)+1)
	for _, style := range doc.Styles {
		paragraphs = append(paragraphs, style)
	}
	defaultID := strings.TrimSpace(doc.DefaultParagraphStyleID)
	if defaultID == "" {
		defaultID = "Normal"
	}
	if _, ok := doc.Styles[defaultID]; !ok {
		paragraphs = append(paragraphs, wordir.Style{
			ID: defaultID, Name: legacyStyleDisplayName(defaultID), Default: true,
			RawFormat:    doc.DefaultParagraphFormat,
			RawRunFormat: doc.DefaultRunFormat,
		})
	}
	sort.SliceStable(paragraphs, func(i, j int) bool {
		ai, aj := paragraphs[i].ID, paragraphs[j].ID
		if ai == defaultID && aj != defaultID {
			return true
		}
		if aj == defaultID && ai != defaultID {
			return false
		}
		si, sj := legacyParagraphSTI(ai), legacyParagraphSTI(aj)
		if si != sj {
			// Built-ins first in their canonical STI order.
			if si >= 0 && sj < 0 {
				return true
			}
			if sj >= 0 && si < 0 {
				return false
			}
			if si >= 0 && sj >= 0 {
				return si < sj
			}
		}
		return ai < aj
	})
	seen := map[string]bool{}
	for _, style := range paragraphs {
		id := strings.TrimSpace(style.ID)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		index := uint16(len(catalog.Entries))
		name := strings.TrimSpace(style.Name)
		if name == "" {
			name = legacyStyleDisplayName(id)
		}
		para := style.RawFormat
		if !legacyParagraphFormatWriteMeaningful(para) {
			para = style.Format
		}
		run := style.RawRunFormat
		if !legacyRunFormatWriteMeaningful(run) {
			run = style.RunFormat
		}
		entry := legacyStyleWriteEntry{
			Index: index, ID: id, Name: name, Kind: 1,
			STI: legacyParagraphSTI(id), BasedOn: style.BasedOn,
			Linked: style.LinkedCharID, ParaFormat: para, RunFormat: run,
		}
		if style.Source != nil {
			entry.ParaSource = append([]byte(nil), style.Source.RawSprms...)
			entry.RunSource = append([]byte(nil), style.Source.RunSprms...)
		}
		catalog.Entries = append(catalog.Entries, entry)
		catalog.ParagraphByID[id] = index
		if id == defaultID {
			catalog.DefaultParagraph = index
		}
	}

	characters := make([]wordir.CharacterStyle, 0, len(doc.CharacterStyles)+1)
	for _, style := range doc.CharacterStyles {
		characters = append(characters, style)
	}
	if _, ok := doc.CharacterStyles["DefaultParagraphFont"]; !ok {
		characters = append(characters, wordir.CharacterStyle{
			ID: "DefaultParagraphFont", Name: "Default Paragraph Font",
			RawRunFormat: doc.DefaultRunFormat,
		})
	}
	sort.SliceStable(characters, func(i, j int) bool {
		si, sj := legacyCharacterSTI(characters[i].ID), legacyCharacterSTI(characters[j].ID)
		if si >= 0 && sj < 0 {
			return true
		}
		if sj >= 0 && si < 0 {
			return false
		}
		if si >= 0 && sj >= 0 && si != sj {
			return si < sj
		}
		return characters[i].ID < characters[j].ID
	})
	for _, style := range characters {
		id := strings.TrimSpace(style.ID)
		if id == "" {
			continue
		}
		if _, exists := catalog.CharacterByID[id]; exists {
			continue
		}
		index := uint16(len(catalog.Entries))
		name := strings.TrimSpace(style.Name)
		if name == "" {
			name = legacyStyleDisplayName(id)
		}
		run := style.RawRunFormat
		if !legacyRunFormatWriteMeaningful(run) {
			run = style.Format
		}
		entry := legacyStyleWriteEntry{
			Index: index, ID: id, Name: name, Kind: 2,
			STI: legacyCharacterSTI(id), BasedOn: style.BasedOn, RunFormat: run,
		}
		if style.Source != nil {
			entry.RunSource = append([]byte(nil), style.Source.RunSprms...)
		}
		catalog.Entries = append(catalog.Entries, entry)
		catalog.CharacterByID[id] = index
	}

	return catalog
}

func buildLegacyStyleSheetData(catalog legacyStyleWriteCatalog, fonts legacyFontWriteCatalog) []byte {
	if len(catalog.Entries) == 0 {
		return nil
	}
	const cbSTDBase = 12
	var out bytes.Buffer
	// STSHI: cstd + cbSTDBaseInFile. The reader only requires these first fields.
	_ = binary.Write(&out, binary.LittleEndian, uint16(4))
	_ = binary.Write(&out, binary.LittleEndian, uint16(len(catalog.Entries)))
	_ = binary.Write(&out, binary.LittleEndian, uint16(cbSTDBase))

	for _, entry := range catalog.Entries {
		std := buildLegacySTD(entry, catalog, fonts, cbSTDBase)
		_ = binary.Write(&out, binary.LittleEndian, uint16(len(std)))
		out.Write(std)
		if out.Len()&1 != 0 {
			out.WriteByte(0)
		}
	}
	return out.Bytes()
}

func buildLegacySTD(entry legacyStyleWriteEntry, catalog legacyStyleWriteCatalog, fonts legacyFontWriteCatalog, cbSTDBase int) []byte {
	var std bytes.Buffer
	sti := entry.STI
	if sti < 0 {
		sti = 0x0FFE
	} // user-defined
	_ = binary.Write(&std, binary.LittleEndian, uint16(sti&0x0FFF))

	basedOn := uint16(0x0FFF)
	if entry.BasedOn != "" {
		if entry.Kind == 1 {
			if idx, ok := catalog.ParagraphByID[entry.BasedOn]; ok {
				basedOn = idx
			}
		} else if idx, ok := catalog.CharacterByID[entry.BasedOn]; ok {
			basedOn = idx
		}
	}
	_ = binary.Write(&std, binary.LittleEndian, uint16(entry.Kind)|(basedOn<<4))

	cupx := uint16(1)
	if entry.Kind == 1 {
		cupx = 2
	}
	_ = binary.Write(&std, binary.LittleEndian, cupx)
	_ = binary.Write(&std, binary.LittleEndian, uint16(0))
	_ = binary.Write(&std, binary.LittleEndian, uint16(0))

	linked := uint16(0)
	if entry.Linked != "" {
		if idx, ok := catalog.CharacterByID[entry.Linked]; ok {
			linked = idx
		}
	}
	_ = binary.Write(&std, binary.LittleEndian, linked)
	for std.Len() < cbSTDBase {
		std.WriteByte(0)
	}

	writeLegacyXstz(&std, entry.Name)
	if entry.Kind == 1 {
		para := append([]byte(nil), entry.ParaSource...)
		para = append(para, encodePapxGrpprl(entry.ParaFormat)...)
		writeLegacyLPUpx(&std, para)

		run := append([]byte(nil), entry.RunSource...)
		run = append(run, encodeLegacyStyleRunGrpprl(entry.RunFormat, fonts)...)
		writeLegacyLPUpx(&std, run)
	} else {
		run := append([]byte(nil), entry.RunSource...)
		run = append(run, encodeLegacyStyleRunGrpprl(entry.RunFormat, fonts)...)
		writeLegacyLPUpx(&std, run)
	}
	return std.Bytes()
}

func encodeLegacyStyleRunGrpprl(f wordir.RunFormat, fonts legacyFontWriteCatalog) []byte {
	out := encodeChpxGrpprl(f)
	out = appendLegacyFontSPRM(out, 0x4A4F, fonts.index(f.FontASCII))
	out = appendLegacyFontSPRM(out, 0x4A50, fonts.index(f.FontEastAsia))
	out = appendLegacyFontSPRM(out, 0x4A51, fonts.index(f.FontCS))
	return out
}

func writeLegacyXstz(out *bytes.Buffer, name string) {
	runes := []rune(strings.TrimSpace(name))
	if len(runes) == 0 {
		runes = []rune("Style")
	}
	if len(runes) > 255 {
		runes = runes[:255]
	}
	_ = binary.Write(out, binary.LittleEndian, uint16(len(runes)))
	for _, r := range runes {
		if r > 0xFFFF {
			r = '?'
		}
		_ = binary.Write(out, binary.LittleEndian, uint16(r))
	}
	_ = binary.Write(out, binary.LittleEndian, uint16(0))
}

func writeLegacyLPUpx(out *bytes.Buffer, grp []byte) {
	if len(grp) > 0xFFFF {
		grp = grp[:0xFFFF]
	}
	_ = binary.Write(out, binary.LittleEndian, uint16(len(grp)))
	out.Write(grp)
	if (2+len(grp))&1 != 0 {
		out.WriteByte(0)
	}
}

func legacyParagraphSTI(id string) int {
	switch id {
	case "Normal":
		return 0
	case "Heading1":
		return 1
	case "Heading2":
		return 2
	case "Heading3":
		return 3
	case "Heading4":
		return 4
	case "Heading5":
		return 5
	case "Heading6":
		return 6
	case "Heading7":
		return 7
	case "Heading8":
		return 8
	case "Heading9":
		return 9
	case "NoList":
		return 105
	case "NoSpacing":
		return 107
	default:
		return -1
	}
}

func legacyCharacterSTI(id string) int {
	if id == "DefaultParagraphFont" {
		return 65
	}
	return -1
}

func legacyStyleDisplayName(id string) string {
	switch id {
	case "Heading1":
		return "Heading 1"
	case "Heading2":
		return "Heading 2"
	case "Heading3":
		return "Heading 3"
	case "Heading4":
		return "Heading 4"
	case "Heading5":
		return "Heading 5"
	case "Heading6":
		return "Heading 6"
	case "Heading7":
		return "Heading 7"
	case "Heading8":
		return "Heading 8"
	case "Heading9":
		return "Heading 9"
	case "DefaultParagraphFont":
		return "Default Paragraph Font"
	case "NoList":
		return "No List"
	case "NoSpacing":
		return "No Spacing"
	default:
		return id
	}
}

func legacyParagraphFormatWriteMeaningful(f wordir.ParagraphFormat) bool {
	return f.Alignment != "" || f.IndentLeft != nil || f.IndentRight != nil ||
		f.IndentFirstLine != nil || f.IndentHanging != nil ||
		f.SpacingBefore != nil || f.SpacingAfter != nil || f.LineSpacing != nil ||
		f.KeepNext || f.KeepLines || f.WidowControl || f.PageBreakBefore ||
		f.Shading != "" || f.Borders != nil || len(f.TabStops) > 0
}

func legacyRunFormatWriteMeaningful(f wordir.RunFormat) bool {
	return f.Bold != nil || f.Italic != nil || f.Strike != nil || f.DoubleStrike != nil ||
		f.Caps != nil || f.SmallCaps != nil || f.Underline != "" ||
		f.Color != "" || f.Background != "" || f.FontSize != nil ||
		f.FontASCII != "" || f.FontHAnsi != "" || f.FontEastAsia != "" ||
		f.FontCS != "" || f.VertAlignVal != "" || f.Vanish != nil ||
		f.CharacterSpacing != nil || f.Kerning != nil || f.CharScale != nil
}

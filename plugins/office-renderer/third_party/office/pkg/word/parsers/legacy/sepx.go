package legacy

import (
	"bytes"
	"encoding/binary"

	"office/pkg/diag"
	"office/pkg/ooxml/cfb"
	wordir "office/pkg/word/ir"
	word97 "office/pkg/word/parsers/doc/word97"
)

// loadLegacyDocSections is now a compatibility projection only. Word97
// PlcfSed/SED/SEPX structure is owned by doc/word97; this package only reuses
// the canonical semantic context. Pre-Word97 retains its separate compatibility
// path until that older section projection is fully canonicalized.
func loadLegacyDocSections(wordStream []byte, f *cfb.File) []wordir.Section {
	if nFib, ok := legacyDocFibVersion(wordStream); ok && nFib >= legacyDocFibWord97 && f != nil {
		snapshot, err := word97.OpenSnapshotReaderAt(bytes.NewReader(wordStream), int64(len(wordStream)), f)
		if err == nil && snapshot != nil && snapshot.Context != nil && len(snapshot.Context.Sections) > 0 {
			return cloneLegacySections(snapshot.Context.Sections)
		}
		return nil
	}
	if oldCore, ok := loadPreWord97DocCore(wordStream); ok && oldCore != nil && oldCore.Canonical != nil {
		if sections := preWord97SectionsFromCanonical(oldCore.Canonical.Sections); len(sections) > 0 {
			return sections
		}
	}
	return loadPreWord97Sections(wordStream)
}

func cloneLegacySections(src []wordir.Section) []wordir.Section {
	if len(src) == 0 {
		return nil
	}
	out := make([]wordir.Section, len(src))
	copy(out, src)
	for i := range out {
		out[i].Headers = append([]wordir.Reference(nil), src[i].Headers...)
		out[i].Footers = append([]wordir.Reference(nil), src[i].Footers...)
		out[i].Diagnostics = append(diag.List(nil), src[i].Diagnostics...)
		if src[i].Source != nil {
			v := *src[i].Source
			v.SepxSprms = append([]byte(nil), src[i].Source.SepxSprms...)
			out[i].Source = &v
		}
	}
	return out
}

// loadPreWord97Sections mirrors POI's OldSectionTable path: the PlcfSed and
// Sepx grpprl bytes live in the WordDocument stream rather than 0Table/1Table.
func loadPreWord97Sections(wordStream []byte) []wordir.Section {
	span, ok := InspectPreWord97BodySpan(wordStream)
	if !ok {
		return nil
	}
	if span.SedTableOffset < 0 || span.SedTableSize < 20 || span.SedTableOffset+span.SedTableSize > len(wordStream) {
		return nil
	}
	plc := wordStream[span.SedTableOffset : span.SedTableOffset+span.SedTableSize]
	n := (len(plc) - 4) / 16
	if n <= 0 {
		return nil
	}
	sections := make([]wordir.Section, 0, n)
	for i := 0; i < n; i++ {
		sedOff := (n+1)*4 + i*12
		if sedOff+12 > len(plc) {
			break
		}
		fcSepxU := binary.LittleEndian.Uint32(plc[sedOff+2 : sedOff+6])
		if fcSepxU == 0xFFFFFFFF {
			sections = append(sections, wordir.Section{})
			continue
		}
		fcSepx := int(fcSepxU)
		if fcSepx+2 > len(wordStream) {
			sections = append(sections, wordir.Section{})
			continue
		}
		cb := int(binary.LittleEndian.Uint16(wordStream[fcSepx : fcSepx+2]))
		if cb < 0 || fcSepx+2+cb > len(wordStream) {
			sections = append(sections, wordir.Section{})
			continue
		}
		grpprl := wordStream[fcSepx+2 : fcSepx+2+cb]
		section := parsePreWord97SepxGrpprl(grpprl)
		startCP := int(binary.LittleEndian.Uint32(plc[i*4 : i*4+4]))
		endCP := int(binary.LittleEndian.Uint32(plc[(i+1)*4 : (i+1)*4+4]))
		section.Source = &wordir.SectionSource{SepxSprms: grpprl, StartCP: startCP, EndCP: endCP}
		sections = append(sections, section)
	}
	return sections
}

// parseSepxGrpprl decodes a sequence of section property SPRMs (MS-DOC §2.6.3)
// and returns a populated wordir.Section with page size, margins, and columns.
func parseSepxGrpprl(grpprl []byte) wordir.Section {
	sec, _ := parseSepxGrpprlWithMetadata(grpprl)
	return sec
}

func parseSepxGrpprlWithMetadata(grpprl []byte) (wordir.Section, *legacyPendingRevisionMark) {
	var sec wordir.Section
	var propRevision *legacyPendingRevisionMark
	var pageWidth, pageHeight int
	var marginTop, marginBottom, marginLeft, marginRight, marginHeader, marginFooter, marginGutter int
	var orientation byte
	var numCols, colSpacing int
	var colSep bool
	var colEvenlySpaced *bool
	colWidths := map[int]int{}
	colSpaces := map[int]int{}
	hasPage, hasMargin, hasCols := false, false, false
	var lnCountBy, lnStart, lnDistance int
	var lnRestart string
	var hasLineNumbering bool
	var gridLinePitch, gridCharSpace int
	var gridType string
	var hasDocGrid bool
	var pageNumFormat string
	var pgNumStart int
	var hasPgNumStart bool
	var pgNumRestart bool
	var titlePage bool
	var breakKind string
	var verticalAlign string
	var sectionBidi bool
	var rtlGutter bool
	var pageBorderTop, pageBorderLeft, pageBorderBottom, pageBorderRight *wordir.BorderSide
	var hasPageBorders bool
	var pageBorderDisplay, pageBorderOffsetFrom, pageBorderZOrder string
	var footnotePosCode byte
	var hasFtnPos bool
	var footnoteNumFmt string
	var footnoteNumStart int
	var footnoteRestart string
	var endnoteNumFmt string
	var endnoteNumStart int
	var endnoteRestart string
	var formProt bool
	var endnoteAtSectionEnd bool
	var diagnostics diag.List

	pos := 0
	for pos+2 <= len(grpprl) {
		code := binary.LittleEndian.Uint16(grpprl[pos:])
		pos += 2
		size := chpxOperandSize(code, grpprl, pos)
		if size <= 0 || pos+size > len(grpprl) {
			break
		}
		op := grpprl[pos : pos+size]
		pos += size

		switch code {
		case 0xB01F:
			pageWidth = int(binary.LittleEndian.Uint16(op)); hasPage = true
		case 0xB020:
			pageHeight = int(binary.LittleEndian.Uint16(op)); hasPage = true
		case 0xB021:
			marginLeft = int(binary.LittleEndian.Uint16(op)); hasMargin = true
		case 0x9023:
			marginTop = int(int16(binary.LittleEndian.Uint16(op))); hasMargin = true
		case 0xB022:
			marginRight = int(binary.LittleEndian.Uint16(op)); hasMargin = true
		case 0x9024:
			marginBottom = int(int16(binary.LittleEndian.Uint16(op))); hasMargin = true
		case 0xB017:
			marginHeader = int(binary.LittleEndian.Uint16(op)); hasMargin = true
		case 0xB018:
			marginFooter = int(binary.LittleEndian.Uint16(op)); hasMargin = true
		case 0xB025:
			marginGutter = int(binary.LittleEndian.Uint16(op)); hasMargin = true
		case 0xD227:
			payload := op
			if len(op) >= 8 && int(op[0]) >= 7 { payload = op[1:] }
			if len(payload) >= 7 {
				propRevision = &legacyPendingRevisionMark{Present: payload[0] != 0, Ibst: binary.LittleEndian.Uint16(payload[1:3]), HasIbst: true, Dttm: binary.LittleEndian.Uint32(payload[3:7]), HasDttm: true}
			} else if len(payload) >= 1 {
				propRevision = &legacyPendingRevisionMark{Present: payload[0] != 0}
			}
		case 0x301D:
			orientation = op[0]
		case 0x300E:
			if len(op) >= 1 { pageNumFormat = legacyNfcToListFormat(op[0]) }
		case 0x500B:
			numCols = int(int16(binary.LittleEndian.Uint16(op))) + 1; hasCols = true
		case 0x900C:
			colSpacing = int(int16(binary.LittleEndian.Uint16(op)))
		case 0x3019:
			colSep = op[0] != 0
		case 0x5033:
			sec.TextDirection = legacyTableTextDirectionFromTextFlow(binary.LittleEndian.Uint16(op))
		case 0x5015:
			lnCountBy = int(binary.LittleEndian.Uint16(op)); hasLineNumbering = true
		case 0x9016:
			if len(op) >= 2 { lnDistance = int(int16(binary.LittleEndian.Uint16(op))); hasLineNumbering = true }
		case 0x501B:
			lnStart = int(int16(binary.LittleEndian.Uint16(op))); hasLineNumbering = true
		case 0x3013:
			switch op[0] { case 0: lnRestart = "newPage"; case 1: lnRestart = "newSection"; case 2: lnRestart = "continuous" }
			hasLineNumbering = true
		case 0x9031:
			gridLinePitch = int(int16(binary.LittleEndian.Uint16(op))); hasDocGrid = true
		case 0x7030:
			if len(op) >= 4 { gridCharSpace = int(int32(binary.LittleEndian.Uint32(op))); hasDocGrid = true }
		case 0x5032:
			switch binary.LittleEndian.Uint16(op) { case 0: gridType = "default"; case 1: gridType = "lines"; case 2: gridType = "linesAndChars"; case 3: gridType = "snapToChars" }
			hasDocGrid = true
		case 0x501C:
			pgNumStart = int(binary.LittleEndian.Uint16(op)); hasPgNumStart = true
		case 0x300A:
			if len(op) >= 1 { titlePage = op[0] != 0 }
		case 0x3009:
			if len(op) >= 1 { switch op[0] { case 0: breakKind = "continuous"; case 1: breakKind = "nextColumn"; case 2: breakKind = "nextPage"; case 3: breakKind = "evenPage"; case 4: breakKind = "oddPage" } }
		case 0x301A:
			if len(op) >= 1 { switch op[0] { case 0: verticalAlign = "top"; case 1: verticalAlign = "center"; case 2: verticalAlign = "both"; case 3: verticalAlign = "bottom" } }
		case 0x3228:
			if len(op) >= 1 { sectionBidi = op[0] != 0 }
		case 0x322A:
			if len(op) >= 1 { rtlGutter = op[0] != 0 }
		case 0x3011:
			if len(op) >= 1 { pgNumRestart = op[0] != 0 }
		case 0x3012:
			if len(op) >= 1 { endnoteAtSectionEnd = op[0] != 0 }
		case 0x7044:
			if len(op) >= 4 { pgNumStart = int(binary.LittleEndian.Uint32(op)); hasPgNumStart = true }
		case 0x303B:
			if len(op) >= 1 { hasFtnPos = true; footnotePosCode = op[0] }
		case 0x303C:
			if len(op) >= 1 { switch op[0] { case 0: footnoteRestart = "continuous"; case 1: footnoteRestart = "eachSect"; case 2: footnoteRestart = "eachPage" } }
		case 0x303E:
			if len(op) >= 1 { switch op[0] { case 0: endnoteRestart = "continuous"; case 1: endnoteRestart = "eachSect" } }
		case 0x503F:
			if len(op) >= 2 { footnoteNumStart = int(binary.LittleEndian.Uint16(op)) }
		case 0x5040:
			if len(op) >= 2 { footnoteNumFmt = legacyNfcToListFormat(byte(binary.LittleEndian.Uint16(op))) }
		case 0x5041:
			if len(op) >= 2 { endnoteNumStart = int(binary.LittleEndian.Uint16(op)) }
		case 0x5042:
			if len(op) >= 2 { endnoteNumFmt = legacyNfcToListFormat(byte(binary.LittleEndian.Uint16(op))) }
		case 0x702B:
			pageBorderTop = parseBRC80(op); if pageBorderTop != nil { hasPageBorders = true }
		case 0x702C:
			pageBorderLeft = parseBRC80(op); if pageBorderLeft != nil { hasPageBorders = true }
		case 0x702D:
			pageBorderBottom = parseBRC80(op); if pageBorderBottom != nil { hasPageBorders = true }
		case 0x702E:
			pageBorderRight = parseBRC80(op); if pageBorderRight != nil { hasPageBorders = true }
		case 0xD234:
			if side := parseBRC(op); side != nil { pageBorderTop = side; hasPageBorders = true }
		case 0xD235:
			if side := parseBRC(op); side != nil { pageBorderLeft = side; hasPageBorders = true }
		case 0xD236:
			if side := parseBRC(op); side != nil { pageBorderBottom = side; hasPageBorders = true }
		case 0xD237:
			if side := parseBRC(op); side != nil { pageBorderRight = side; hasPageBorders = true }
		case 0x522F:
			if len(op) >= 2 {
				pgbProp := binary.LittleEndian.Uint16(op); hasPageBorders = true
				switch pgbProp & 0x03 { case 0: pageBorderDisplay = "allPages"; case 1: pageBorderDisplay = "firstPage"; case 2: pageBorderDisplay = "notFirstPage" }
				if pgbProp&0x04 != 0 { pageBorderOffsetFrom = "edge" } else { pageBorderOffsetFrom = "text" }
				if pgbProp&0x08 != 0 { pageBorderZOrder = "back" } else { pageBorderZOrder = "front" }
			}
		case 0xF203:
			if len(op) >= 3 { idx := int(op[0]); colWidths[idx] = int(binary.LittleEndian.Uint16(op[1:3])); if idx+1 > numCols { numCols = idx + 1 }; hasCols = true }
		case 0xF204:
			if len(op) >= 3 { idx := int(op[0]); colSpaces[idx] = int(binary.LittleEndian.Uint16(op[1:3])); if idx+2 > numCols { numCols = idx + 2 }; hasCols = true }
		case 0x3005:
			if len(op) >= 1 { v := op[0] != 0; colEvenlySpaced = &v; hasCols = true }
		case 0x3006:
			if len(op) >= 1 { formProt = op[0] != 0 }
		case 0x5007:
			legacyUnmappedSPRM(&diagnostics, code, "sprmSDmBinFirst")
		case 0x5008:
			legacyUnmappedSPRM(&diagnostics, code, "sprmSDmBinOther")
		case 0x3000:
			legacyUnmappedSPRM(&diagnostics, code, "sprmScnsPgn")
		case 0x3001:
			legacyUnmappedSPRM(&diagnostics, code, "sprmSiHeadingPgn")
		case 0x5026:
			legacyUnmappedSPRM(&diagnostics, code, "sprmSDmPaperReq")
		case 0x3239:
			legacyUnmappedSPRM(&diagnostics, code, "sprmSWall")
		case 0x703A:
			legacyUnmappedSPRM(&diagnostics, code, "sprmSRsid")
		case 0xD243:
			payload := op
			if len(op) >= 8 && int(op[0]) >= 7 { payload = op[1:] }
			if len(payload) >= 7 { propRevision = &legacyPendingRevisionMark{Present: payload[0] != 0, Ibst: binary.LittleEndian.Uint16(payload[1:3]), HasIbst: true, Dttm: binary.LittleEndian.Uint32(payload[3:7]), HasDttm: true} }
		}
	}

	if hasPage {
		orient := "portrait"
		if orientation != 0 { orient = "landscape" }
		sec.PageSize = &wordir.PageSize{Width: pageWidth, Height: pageHeight, Orient: orient}
	}
	if hasMargin {
		sec.PageMargin = &wordir.PageMargin{Top: marginTop, Bottom: marginBottom, Left: marginLeft, Right: marginRight, Header: marginHeader, Footer: marginFooter, Gutter: marginGutter, RtlGutter: rtlGutter}
	}
	if hasCols {
		if numCols < 1 { numCols = 1 }
		columns := &wordir.Columns{Num: numCols, Space: colSpacing, Sep: colSep, EvenlySpaced: colEvenlySpaced}
		if len(colWidths) > 0 {
			columns.Widths = make([]int, numCols)
			for idx, width := range colWidths { if idx >= 0 && idx < len(columns.Widths) { columns.Widths[idx] = width } }
		}
		if len(colSpaces) > 0 && numCols > 1 {
			columns.Spaces = make([]int, numCols-1)
			for idx, width := range colSpaces { if idx >= 0 && idx < len(columns.Spaces) { columns.Spaces[idx] = width } }
		}
		sec.Columns = columns
	}
	if hasLineNumbering { sec.LineNumbering = &wordir.LineNumbering{CountBy: lnCountBy, Start: lnStart, Restart: lnRestart, Distance: lnDistance} }
	if hasDocGrid { sec.DocGrid = &wordir.DocGrid{LinePitch: gridLinePitch, CharSpace: gridCharSpace, Type: gridType} }
	if hasPageBorders { sec.PageBorders = &wordir.PageBorders{Top: pageBorderTop, Left: pageBorderLeft, Bottom: pageBorderBottom, Right: pageBorderRight, Display: pageBorderDisplay, OffsetFrom: pageBorderOffsetFrom, ZOrder: pageBorderZOrder} }
	if hasPgNumStart { sec.PageNumberStart = &pgNumStart }
	sec.PageNumberFormat = pageNumFormat
	sec.PageNumberRestart = pgNumRestart
	sec.TitlePage = titlePage
	sec.BreakKind = breakKind
	sec.Type = breakKind
	sec.VerticalAlign = verticalAlign
	sec.Bidi = sectionBidi
	if hasFtnPos { switch footnotePosCode { case 0: sec.FootnotePos = "sectEnd"; case 2: sec.FootnotePos = "belowText"; default: sec.FootnotePos = "pageBottom" } }
	sec.FootnoteNumFmt = footnoteNumFmt
	sec.FootnoteNumStart = footnoteNumStart
	sec.FootnoteRestart = footnoteRestart
	sec.EndnoteNumFmt = endnoteNumFmt
	sec.EndnoteNumStart = endnoteNumStart
	sec.EndnoteRestart = endnoteRestart
	sec.EndnoteAtSectionEnd = endnoteAtSectionEnd
	sec.FormProt = formProt
	sec.Diagnostics = diagnostics
	return sec, propRevision
}

// parsePreWord97SepxGrpprl decodes a pre-Word-97 section property SPRM sequence
// using 1-byte opcode format (Word 6/95). Returns a populated wordir.Section
// with page size, margins, and basic layout from the observed SEPX bytes.
func parsePreWord97SepxGrpprl(grpprl []byte) wordir.Section {
	var sec wordir.Section
	var pageWidth, pageHeight int
	var marginTop, marginBottom, marginLeft, marginRight int
	var hasPage, hasMargin bool
	var breakKind string

	pos := 0
	for pos < len(grpprl) {
		opcode := grpprl[pos]
		pos++
		opSize := preWord97PapxOperandSize(opcode, grpprl, pos)
		if opSize <= 0 || pos+opSize > len(grpprl) { break }
		op := grpprl[pos : pos+opSize]
		pos += opSize
		switch opcode {
		case 142:
			if len(op) >= 1 { switch op[0] { case 0: breakKind = "continuous"; case 1: breakKind = "nextColumn"; case 2: breakKind = "nextPage"; case 3: breakKind = "evenPage"; case 4: breakKind = "oddPage" } }
		case 164:
			pageWidth = int(binary.LittleEndian.Uint16(op)); hasPage = true
		case 165:
			pageHeight = int(binary.LittleEndian.Uint16(op)); hasPage = true
		case 166:
			marginLeft = int(binary.LittleEndian.Uint16(op)); hasMargin = true
		case 167:
			marginRight = int(binary.LittleEndian.Uint16(op)); hasMargin = true
		case 168:
			marginTop = int(binary.LittleEndian.Uint16(op)); hasMargin = true
		case 169:
			marginBottom = int(binary.LittleEndian.Uint16(op)); hasMargin = true
		}
	}
	if hasPage { sec.PageSize = &wordir.PageSize{Width: pageWidth, Height: pageHeight, Orient: "portrait"} }
	if hasMargin { sec.PageMargin = &wordir.PageMargin{Top: marginTop, Bottom: marginBottom, Left: marginLeft, Right: marginRight} }
	sec.BreakKind = breakKind
	sec.Type = breakKind
	return sec
}

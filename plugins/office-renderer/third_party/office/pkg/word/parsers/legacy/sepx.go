package legacy

import (
	"encoding/binary"

	"office/pkg/diag"
	"office/pkg/ooxml/cfb"
	wordir "office/pkg/word/ir"
)

func loadLegacyDocSections(wordStream []byte, f *cfb.File) []wordir.Section {
	if secs := loadDocSections(wordStream, f); len(secs) > 0 {
		return secs
	}
	return loadPreWord97Sections(wordStream)
}

// loadDocSections reads PlcfSed (FibRgFcLcb97 entry 6) from the table stream
// and decodes each section's Sepx SPRM block into a wordir.Section.
// Returns nil on any parse failure so callers can skip section layout gracefully.
func loadDocSections(wordStream []byte, f *cfb.File) []wordir.Section {
	if len(wordStream) < 34 {
		return nil
	}
	if binary.LittleEndian.Uint16(wordStream[0:2]) != 0xA5EC {
		return nil
	}
	if binary.LittleEndian.Uint16(wordStream[2:4]) < 0x00C1 {
		return nil
	}
	flags := binary.LittleEndian.Uint16(wordStream[10:12])
	tableStreamName := "0Table"
	if flags&(1<<9) != 0 {
		tableStreamName = "1Table"
	}
	tableStream, ok := f.Stream(tableStreamName)
	if !ok || len(tableStream) == 0 {
		return nil
	}

	// Navigate past FibBase, FibRgW97, FibRgLw97 to reach FibRgFcLcb97.
	pos := 32
	if pos+2 > len(wordStream) {
		return nil
	}
	csw := int(binary.LittleEndian.Uint16(wordStream[pos : pos+2]))
	pos += 2 + csw*2
	if pos+2 > len(wordStream) {
		return nil
	}
	cslw := int(binary.LittleEndian.Uint16(wordStream[pos : pos+2]))
	pos += 2 + cslw*4
	if pos+2 > len(wordStream) {
		return nil
	}
	cbRgFcLcb := int(binary.LittleEndian.Uint16(wordStream[pos : pos+2]))
	pos += 2
	fibBase := pos

	const sedFibIndex = 6
	if sedFibIndex >= cbRgFcLcb || fibBase+(sedFibIndex+1)*8 > len(wordStream) {
		return nil
	}
	fc := int(binary.LittleEndian.Uint32(wordStream[fibBase+sedFibIndex*8:]))
	lcb := int(binary.LittleEndian.Uint32(wordStream[fibBase+sedFibIndex*8+4:]))
	// Minimum valid PlcfSed: 2 CPs (8 bytes) + 1 SED (12 bytes) = 20 bytes.
	if lcb < 20 || fc < 0 || fc+lcb > len(tableStream) {
		return nil
	}

	// PlcfSed: (n+1) × 4-byte CPs + n × 12-byte SEDs.
	plc := tableStream[fc : fc+lcb]
	n := (lcb - 4) / 16
	if n <= 0 {
		return nil
	}

	sttbfRMark := loadLegacySttbfRMark(wordStream, tableStream, fibBase, cbRgFcLcb)
	sections := make([]wordir.Section, 0, n)
	for i := 0; i < n; i++ {
		sedOff := (n+1)*4 + i*12
		if sedOff+12 > len(plc) {
			break
		}
		// SED layout: fnSed(2) + fcSepx(4) + fUnk(2) + fcMprNoObjs(4) = 12 bytes.
		fcSepxU := binary.LittleEndian.Uint32(plc[sedOff+2 : sedOff+6])
		if fcSepxU == 0xFFFFFFFF {
			sections = append(sections, wordir.Section{})
			continue
		}
		fcSepx := int(fcSepxU)
		// MS-DOC SED.fcSepx is a file offset into the WordDocument stream.
		// PlcfSed itself lives in 0Table/1Table, but the SEPX records it points
		// at do not. Reading fcSepx from the table stream silently discarded
		// real section geometry (page size, margins, orientation, columns) for
		// ordinary Word 97-2003 documents and made the layout fall back to the
		// renderer's US-Letter/1-inch defaults.
		//
		// Keep a table-stream fallback for older synthetic fixtures that were
		// authored before this distinction was enforced. Real files always take
		// the spec-correct WordDocument path first.
		grpprl, ok := legacySepxGrpprlAt(wordStream, fcSepx)
		if !ok {
			grpprl, ok = legacySepxGrpprlAt(tableStream, fcSepx)
		}
		if !ok {
			sections = append(sections, wordir.Section{})
			continue
		}
		section, meta := parseSepxGrpprlWithMetadata(grpprl)
		if meta != nil {
			section.PropertyRevision = resolveLegacyPropertyRevisionMark(sttbfRMark, meta.Present, meta.Ibst, meta.HasIbst, meta.Dttm, meta.HasDttm)
		}
		startCP := int(binary.LittleEndian.Uint32(plc[i*4 : i*4+4]))
		endCP := int(binary.LittleEndian.Uint32(plc[(i+1)*4 : (i+1)*4+4]))
		section.Source = &wordir.SectionSource{SepxSprms: grpprl, StartCP: startCP, EndCP: endCP}
		sections = append(sections, section)
	}
	return sections
}

func legacySepxGrpprlAt(stream []byte, fcSepx int) ([]byte, bool) {
	if fcSepx < 0 || fcSepx+2 > len(stream) {
		return nil, false
	}
	cb := int(binary.LittleEndian.Uint16(stream[fcSepx : fcSepx+2]))
	if cb <= 0 || fcSepx+2+cb > len(stream) {
		return nil, false
	}
	return stream[fcSepx+2 : fcSepx+2+cb], true
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
		case 0xB01F: // sprmSXaPage — page width in twips
			pageWidth = int(binary.LittleEndian.Uint16(op))
			hasPage = true
		case 0xB020: // sprmSYaPage — page height in twips
			pageHeight = int(binary.LittleEndian.Uint16(op))
			hasPage = true
		case 0xB021: // sprmSDxaLeft — left margin in twips
			marginLeft = int(binary.LittleEndian.Uint16(op))
			hasMargin = true
		case 0x9023: // sprmSDyaTop — top margin in twips (YAS, signed)
			marginTop = int(int16(binary.LittleEndian.Uint16(op)))
			hasMargin = true
		case 0xB022: // sprmSDxaRight — right margin in twips (XAS_nonNeg)
			marginRight = int(binary.LittleEndian.Uint16(op))
			hasMargin = true
		case 0x9024: // sprmSDyaBottom — bottom margin in twips (YAS, signed)
			marginBottom = int(int16(binary.LittleEndian.Uint16(op)))
			hasMargin = true
		case 0xB017: // sprmSDyaHdrTop — header distance from top edge in twips (YAS_nonNeg)
			marginHeader = int(binary.LittleEndian.Uint16(op))
			hasMargin = true
		case 0xB018: // sprmSDyaHdrBottom — footer distance from bottom edge in twips (YAS_nonNeg)
			marginFooter = int(binary.LittleEndian.Uint16(op))
			hasMargin = true
		case 0xB025: // sprmSDzaGutter — gutter margin in twips
			marginGutter = int(binary.LittleEndian.Uint16(op))
			hasMargin = true
		case 0xD227: // sprmSPropRMark — section property revision mark metadata
			payload := op
			if len(op) >= 8 && int(op[0]) >= 7 {
				payload = op[1:]
			}
			if len(payload) >= 7 {
				propRevision = &legacyPendingRevisionMark{
					Present: payload[0] != 0,
					Ibst:    binary.LittleEndian.Uint16(payload[1:3]),
					HasIbst: true,
					Dttm:    binary.LittleEndian.Uint32(payload[3:7]),
					HasDttm: true,
				}
			} else if len(payload) >= 1 {
				propRevision = &legacyPendingRevisionMark{Present: payload[0] != 0}
			}
		case 0x301D: // sprmSBOrientation — 0=portrait, 1=landscape
			orientation = op[0]
		case 0x300E: // sprmSNfcPgn — page number format (1 byte nfc)
			if len(op) >= 1 {
				pageNumFormat = legacyNfcToListFormat(op[0])
			}
		case 0x500B: // sprmSCcolumns — column count minus one (2 bytes unsigned); 0 = single column
			numCols = int(int16(binary.LittleEndian.Uint16(op))) + 1
			hasCols = true
			numCols = int(int16(binary.LittleEndian.Uint16(op))) + 1
			hasCols = true
		case 0x900C: // sprmSDxaColumns — space between columns in twips (XAS_nonNeg, evenly-spaced mode)
			colSpacing = int(int16(binary.LittleEndian.Uint16(op)))
		case 0x3019: // sprmSLBetween — line between columns
			colSep = op[0] != 0
		case 0x5033: // sprmSTextFlow — section text direction
			sec.TextDirection = legacyTableTextDirectionFromTextFlow(binary.LittleEndian.Uint16(op))
		case 0x5015: // sprmSNLnnMod — interval between displayed line numbers
			lnCountBy = int(binary.LittleEndian.Uint16(op))
			hasLineNumbering = true
		case 0x9016: // sprmSDxaLnn — distance from text to line numbers in twips (signed short)
			if len(op) >= 2 {
				lnDistance = int(int16(binary.LittleEndian.Uint16(op)))
				hasLineNumbering = true
			}
		case 0x501B: // sprmSLnnMin — first line number
			lnStart = int(int16(binary.LittleEndian.Uint16(op)))
			hasLineNumbering = true
		case 0x3013: // sprmSLnc — restart code (0=newPage, 1=newSection, 2=continuous)
			switch op[0] {
			case 0:
				lnRestart = "newPage"
			case 1:
				lnRestart = "newSection"
			case 2:
				lnRestart = "continuous"
			}
			hasLineNumbering = true
		case 0x9031: // sprmSDyaLinePitch — grid line pitch in twips (signed short)
			gridLinePitch = int(int16(binary.LittleEndian.Uint16(op)))
			hasDocGrid = true
		case 0x7030: // sprmSDxtCharSpace — character space (4-byte, lower 16 bits = char space twips)
			if len(op) >= 4 {
				gridCharSpace = int(int32(binary.LittleEndian.Uint32(op)))
				hasDocGrid = true
			}
		case 0x5032: // sprmSClm — doc grid type: 0=default, 1=lines, 2=lineschars, 3=snapToChars
			switch binary.LittleEndian.Uint16(op) {
			case 0:
				gridType = "default"
			case 1:
				gridType = "lines"
			case 2:
				gridType = "linesAndChars"
			case 3:
				gridType = "snapToChars"
			}
			hasDocGrid = true
		case 0x501C: // sprmSPgnStart — explicit starting page number (2 bytes)
			pgNumStart = int(binary.LittleEndian.Uint16(op))
			hasPgNumStart = true
		case 0x300A: // sprmSFTitlePage — different first-page header/footer (1-byte toggle)
			if len(op) >= 1 {
				titlePage = op[0] != 0
			}
		case 0x3009: // sprmSBkc — section break code (1 byte)
			if len(op) >= 1 {
				switch op[0] {
				case 0:
					breakKind = "continuous"
				case 1:
					breakKind = "nextColumn"
				case 2:
					breakKind = "nextPage"
				case 3:
					breakKind = "evenPage"
				case 4:
					breakKind = "oddPage"
				}
			}
		case 0x301A: // sprmSVjc — vertical justification (1 byte)
			if len(op) >= 1 {
				switch op[0] {
				case 0:
					verticalAlign = "top"
				case 1:
					verticalAlign = "center"
				case 2:
					verticalAlign = "both"
				case 3:
					verticalAlign = "bottom"
				}
			}
		case 0x3228: // sprmSFBiDi — section right-to-left column layout (1 byte Bool8)
			if len(op) >= 1 {
				sectionBidi = op[0] != 0
			}
		case 0x322A: // sprmSFRTLGutter — right-to-left gutter
			if len(op) >= 1 {
				rtlGutter = op[0] != 0
			}
		case 0x3011: // sprmSFPgnRestart — page number restart (1 byte Bool8)
			if len(op) >= 1 {
				pgNumRestart = op[0] != 0
			}
		case 0x3012: // sprmSFEndnote — endnotes shown at end of this section (1 byte Bool8)
			if len(op) >= 1 {
				endnoteAtSectionEnd = op[0] != 0
			}
		case 0x7044: // sprmSPgnStart — 4-byte unsigned page number start (supersedes 0x501C)
			if len(op) >= 4 {
				v := int(binary.LittleEndian.Uint32(op))
				pgNumStart = v
				hasPgNumStart = true
			}
		case 0x303B: // sprmSFpc — footnote position (1 byte SFpcOperand)
			// 0=sectEnd, 1=pageBottom, 2=belowText
			if len(op) >= 1 {
				hasFtnPos = true
				footnotePosCode = op[0]
			}
		case 0x303C: // sprmSRncFtn — footnote numbering restart (1 byte Rnc enum)
			// 0=continuous, 1=eachSect, 2=eachPage
			if len(op) >= 1 {
				switch op[0] {
				case 0:
					footnoteRestart = "continuous"
				case 1:
					footnoteRestart = "eachSect"
				case 2:
					footnoteRestart = "eachPage"
				}
			}
		case 0x303E: // sprmSRncEdn — endnote numbering restart (1 byte Rnc: 0=continuous,1=eachSect)
			if len(op) >= 1 {
				switch op[0] {
				case 0:
					endnoteRestart = "continuous"
				case 1:
					endnoteRestart = "eachSect"
				}
			}
		case 0x503F: // sprmSNFtn — footnote number offset (2 bytes unsigned)
			if len(op) >= 2 {
				footnoteNumStart = int(binary.LittleEndian.Uint16(op))
			}
		case 0x5040: // sprmSNfcFtnRef — footnote numbering format (2-byte MSONFC)
			if len(op) >= 2 {
				footnoteNumFmt = legacyNfcToListFormat(byte(binary.LittleEndian.Uint16(op)))
			}
		case 0x5041: // sprmSNEdn — endnote number offset (2 bytes unsigned)
			if len(op) >= 2 {
				endnoteNumStart = int(binary.LittleEndian.Uint16(op))
			}
		case 0x5042: // sprmSNfcEdnRef — endnote numbering format (2-byte MSONFC)
			if len(op) >= 2 {
				endnoteNumFmt = legacyNfcToListFormat(byte(binary.LittleEndian.Uint16(op)))
			}
		case 0x702B: // sprmSBrcTop80 — top page border (4-byte Brc80)
			pageBorderTop = parseBRC80(op)
			if pageBorderTop != nil {
				hasPageBorders = true
			}
		case 0x702C: // sprmSBrcLeft80 — left page border (4-byte Brc80)
			pageBorderLeft = parseBRC80(op)
			if pageBorderLeft != nil {
				hasPageBorders = true
			}
		case 0x702D: // sprmSBrcBottom80 — bottom page border (4-byte Brc80)
			pageBorderBottom = parseBRC80(op)
			if pageBorderBottom != nil {
				hasPageBorders = true
			}
		case 0x702E: // sprmSBrcRight80 — right page border (4-byte Brc80)
			pageBorderRight = parseBRC80(op)
			if pageBorderRight != nil {
				hasPageBorders = true
			}
		case 0xD234: // sprmSBrcTop — full BrcOperand top page border (supersedes Brc80)
			if side := parseBRC(op); side != nil {
				pageBorderTop = side
				hasPageBorders = true
			}
		case 0xD235: // sprmSBrcLeft — full BrcOperand left page border
			if side := parseBRC(op); side != nil {
				pageBorderLeft = side
				hasPageBorders = true
			}
		case 0xD236: // sprmSBrcBottom — full BrcOperand bottom page border
			if side := parseBRC(op); side != nil {
				pageBorderBottom = side
				hasPageBorders = true
			}
		case 0xD237: // sprmSBrcRight — full BrcOperand right page border
			if side := parseBRC(op); side != nil {
				pageBorderRight = side
				hasPageBorders = true
			}
		case 0x522F: // sprmSPgbProp — page border display/offset/z-order properties (2 bytes)
			if len(op) >= 2 {
				pgbProp := binary.LittleEndian.Uint16(op)
				hasPageBorders = true
				// Bits 0-1: pgbApplyTo (0=allPages, 1=firstPage, 2=notFirstPage)
				switch pgbProp & 0x03 {
				case 0:
					pageBorderDisplay = "allPages"
				case 1:
					pageBorderDisplay = "firstPage"
				case 2:
					pageBorderDisplay = "notFirstPage"
				}
				// Bit 2: pgbOffsetFrom (0=text, 1=edge)
				if pgbProp&0x04 != 0 {
					pageBorderOffsetFrom = "edge"
				} else {
					pageBorderOffsetFrom = "text"
				}
				// Bit 3: pgbInFront (0=front, 1=back)
				if pgbProp&0x08 != 0 {
					pageBorderZOrder = "back"
				} else {
					pageBorderZOrder = "front"
				}
			}
		case 0xF203: // sprmSDxaColWidth — iCol (1 byte) + column width (2-byte twips)
			if len(op) >= 3 {
				idx := int(op[0])
				colWidths[idx] = int(binary.LittleEndian.Uint16(op[1:3]))
				if idx+1 > numCols {
					numCols = idx + 1
				}
				hasCols = true
			}
		case 0xF204: // sprmSDxaColSpacing — iCol (1 byte) + following gap (2-byte twips)
			if len(op) >= 3 {
				idx := int(op[0])
				colSpaces[idx] = int(binary.LittleEndian.Uint16(op[1:3]))
				if idx+2 > numCols {
					numCols = idx + 2
				}
				hasCols = true
			}
		case 0x3005: // sprmSFEvenlySpaced — explicit equal-width mode
			if len(op) >= 1 {
				v := op[0] != 0
				colEvenlySpaced = &v
				hasCols = true
			}
		case 0x3006: // sprmSFProtected — section form-field editing protection (Bool8)
			if len(op) >= 1 {
				formProt = op[0] != 0
			}
		case 0x5007: // sprmSDmBinFirst — first-page printer paper source; not renderable
			legacyUnmappedSPRM(&diagnostics, code, "sprmSDmBinFirst")
		case 0x5008: // sprmSDmBinOther — other-pages printer paper source; not renderable
			legacyUnmappedSPRM(&diagnostics, code, "sprmSDmBinOther")
		case 0x3000: // sprmScnsPgn — chapter number separator; no IR field yet
			legacyUnmappedSPRM(&diagnostics, code, "sprmScnsPgn")
		case 0x3001: // sprmSiHeadingPgn — chapter heading level for page numbers; no IR field yet
			legacyUnmappedSPRM(&diagnostics, code, "sprmSiHeadingPgn")
		case 0x5026: // sprmSDmPaperReq — paper format tie-breaker; implementation-specific
			legacyUnmappedSPRM(&diagnostics, code, "sprmSDmPaperReq")
		case 0x3239: // sprmSWall — revision property preservation wall; structural
			legacyUnmappedSPRM(&diagnostics, code, "sprmSWall")
		case 0x703A: // sprmSRsid — revision save ID; no IR field
			legacyUnmappedSPRM(&diagnostics, code, "sprmSRsid")
		case 0xD243: // sprmSPropRMark — alias for sprmSPropRMark (0xD227); same payload
			payload := op
			if len(op) >= 8 && int(op[0]) >= 7 {
				payload = op[1:]
			}
			if len(payload) >= 7 {
				propRevision = &legacyPendingRevisionMark{
					Present: payload[0] != 0,
					Ibst:    binary.LittleEndian.Uint16(payload[1:3]),
					HasIbst: true,
					Dttm:    binary.LittleEndian.Uint32(payload[3:7]),
					HasDttm: true,
				}
			}
		}
	}

	if hasPage {
		orient := "portrait"
		if orientation != 0 {
			orient = "landscape"
		}
		sec.PageSize = &wordir.PageSize{
			Width:  pageWidth,
			Height: pageHeight,
			Orient: orient,
		}
	}
	if hasMargin {
		sec.PageMargin = &wordir.PageMargin{
			Top:       marginTop,
			Bottom:    marginBottom,
			Left:      marginLeft,
			Right:     marginRight,
			Header:    marginHeader,
			Footer:    marginFooter,
			Gutter:    marginGutter,
			RtlGutter: rtlGutter,
		}
	}
	if hasCols {
		if numCols < 1 {
			numCols = 1
		}
		columns := &wordir.Columns{
			Num:          numCols,
			Space:        colSpacing,
			Sep:          colSep,
			EvenlySpaced: colEvenlySpaced,
		}
		if len(colWidths) > 0 {
			columns.Widths = make([]int, numCols)
			for idx, width := range colWidths {
				if idx >= 0 && idx < len(columns.Widths) {
					columns.Widths[idx] = width
				}
			}
		}
		if len(colSpaces) > 0 && numCols > 1 {
			columns.Spaces = make([]int, numCols-1)
			for idx, width := range colSpaces {
				if idx >= 0 && idx < len(columns.Spaces) {
					columns.Spaces[idx] = width
				}
			}
		}
		sec.Columns = columns
	}
	if hasLineNumbering {
		sec.LineNumbering = &wordir.LineNumbering{
			CountBy:  lnCountBy,
			Start:    lnStart,
			Restart:  lnRestart,
			Distance: lnDistance,
		}
	}
	if hasDocGrid {
		sec.DocGrid = &wordir.DocGrid{
			LinePitch: gridLinePitch,
			CharSpace: gridCharSpace,
			Type:      gridType,
		}
	}
	if hasPageBorders {
		sec.PageBorders = &wordir.PageBorders{
			Top:        pageBorderTop,
			Left:       pageBorderLeft,
			Bottom:     pageBorderBottom,
			Right:      pageBorderRight,
			Display:    pageBorderDisplay,
			OffsetFrom: pageBorderOffsetFrom,
			ZOrder:     pageBorderZOrder,
		}
	}
	if hasPgNumStart {
		sec.PageNumberStart = &pgNumStart
	}
	sec.PageNumberFormat = pageNumFormat
	sec.PageNumberRestart = pgNumRestart
	sec.TitlePage = titlePage
	sec.BreakKind = breakKind
	sec.Type = breakKind
	sec.Type = breakKind
	sec.VerticalAlign = verticalAlign
	sec.Bidi = sectionBidi
	if hasFtnPos {
		switch footnotePosCode {
		case 0:
			sec.FootnotePos = "sectEnd"
		case 2:
			sec.FootnotePos = "belowText"
		default:
			sec.FootnotePos = "pageBottom"
		}
	}
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
		if opSize <= 0 || pos+opSize > len(grpprl) {
			break
		}
		op := grpprl[pos : pos+opSize]
		pos += opSize

		switch opcode {
		case 142: // legacy sprmSBkc — section break kind (1 byte)
			if len(op) >= 1 {
				switch op[0] {
				case 0:
					breakKind = "continuous"
				case 1:
					breakKind = "nextColumn"
				case 2:
					breakKind = "nextPage"
				case 3:
					breakKind = "evenPage"
				case 4:
					breakKind = "oddPage"
				}
			}
		case 164: // xaPage — page width in twips
			pageWidth = int(binary.LittleEndian.Uint16(op))
			hasPage = true
		case 165: // yaPage — page height in twips
			pageHeight = int(binary.LittleEndian.Uint16(op))
			hasPage = true
		case 166: // dxaLeft — left margin in twips
			marginLeft = int(binary.LittleEndian.Uint16(op))
			hasMargin = true
		case 167: // dxaRight — right margin in twips
			marginRight = int(binary.LittleEndian.Uint16(op))
			hasMargin = true
		case 168: // dyaTop — top margin in twips
			marginTop = int(binary.LittleEndian.Uint16(op))
			hasMargin = true
		case 169: // dyaBottom — bottom margin in twips
			marginBottom = int(binary.LittleEndian.Uint16(op))
			hasMargin = true
		}
	}

	if hasPage {
		sec.PageSize = &wordir.PageSize{
			Width:  pageWidth,
			Height: pageHeight,
			Orient: "portrait", // Pre-Word-97 doesn't track orientation in SEP
		}
	}
	if hasMargin {
		sec.PageMargin = &wordir.PageMargin{
			Top:    marginTop,
			Bottom: marginBottom,
			Left:   marginLeft,
			Right:  marginRight,
		}
	}
	sec.BreakKind = breakKind
	sec.Type = breakKind
	return sec
}

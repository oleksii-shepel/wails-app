package legacy

import (
	"encoding/binary"
	"fmt"

	"office/pkg/diag"
	wordir "office/pkg/word/ir"
)

// papxEntry holds one FC-indexed paragraph formatting record from a PapxFkp page.
// FC is the raw FcCompressed value of the first character of the paragraph.
// All characters up to (but not including) the next entry's FC share this format.
type papxEntry struct {
	FC                uint32
	Format            wordir.ParagraphFormat
	Meta              papxMetadata
	LegacyInTableHint bool
	LegacyRowMarkHint bool
	// Sprms stores the raw PAPX grpprl bytes so that lossless IR can round-trip
	// or reproject formatting without re-reading the binary file.
	Sprms []byte
}

type papxMetadata struct {
	TableDepth         int
	TableRowProps      *legacyTableRowProps
	PreWord97ListStart *int
	PropertyRevision   *legacyPendingRevisionMark
	Diagnostics        diag.List
}

type legacyPendingRevisionMark struct {
	Present bool
	Ibst    uint16
	HasIbst bool
	Dttm    uint32
	HasDttm bool
}

// loadPapxEntries reads the PlcBtePapx from the table stream (location given by
// FibRgFcLcb97[13]) and returns a sorted list of papxEntry values covering all
// paragraph positions in the document.
//
// fibBase is the byte offset of FibRgFcLcb97[0] within wordStream.
// cbRgFcLcb is the count of entries present in the FibRgFcLcb97 array.
func loadPapxEntries(wordStream, tableStream []byte, fibBase, cbRgFcLcb int) []papxEntry {
	const papxFibIndex = 13
	if papxFibIndex >= cbRgFcLcb {
		return nil
	}
	if fibBase+(papxFibIndex+1)*8 > len(wordStream) {
		return nil
	}
	fc := int(binary.LittleEndian.Uint32(wordStream[fibBase+papxFibIndex*8:]))
	lcb := int(binary.LittleEndian.Uint32(wordStream[fibBase+papxFibIndex*8+4:]))
	if lcb < 12 || fc < 0 || fc+lcb > len(tableStream) {
		return nil
	}

	plc := tableStream[fc : fc+lcb]
	// PlcBtePapx layout: (n+1) × 4-byte FC values, then n × 4-byte page numbers.
	// n = (lcb - 4) / 8
	n := (lcb - 4) / 8
	if n <= 0 || (n+1)*4+n*4 > lcb {
		return nil
	}

	var entries []papxEntry
	for i := 0; i < n; i++ {
		pn := int(binary.LittleEndian.Uint32(plc[(n+1)*4+i*4:]))
		pageOff := pn * 512
		if pageOff+512 > len(wordStream) {
			continue
		}
		entries = append(entries, parsePapxFKPPage(wordStream[pageOff:pageOff+512])...)
	}
	return entries
}

// parsePapxFKPPage decodes a single 512-byte PapxFkp page and returns its
// papxEntry records.  The structure is described in MS-DOC 2.9.130.
//
// PapxFkp layout:
//   - rgfc[0..cpara]:   (cpara+1) × 4-byte FC values
//   - rgbx[0..cpara-1]: cpara × 13-byte BxPap structs (byte 0 of each = bOffset)
//   - byte 511:         cpara
//
// At page[bOffset*2] sits a PapxInFkp (MS-DOC 2.9.132) with two forms:
//
//	cb==0: byte 0=0, byte 1=cb2, bytes 2-3=istd, grpprl at bytes 4..4+(cb2-1)*2-1
//	cb!=0: byte 0=cb, bytes 1-2=istd, grpprl at bytes 3..3+(cb-1)*2-1
func parsePapxFKPPage(page []byte) []papxEntry {
	if len(page) < 512 {
		return nil
	}
	cpara := int(page[511])
	if cpara == 0 {
		return nil
	}
	rgbxStart := (cpara + 1) * 4
	// Each BxPap entry is 13 bytes (bOffset byte + 12 bytes of tab-stop data).
	if rgbxStart+cpara*13 > 511 {
		return nil
	}

	result := make([]papxEntry, 0, cpara)
	for i := 0; i < cpara; i++ {
		fc := binary.LittleEndian.Uint32(page[i*4 : i*4+4])
		bOffset := int(page[rgbxStart+i*13]) // first byte of BxPap[i]
		var format wordir.ParagraphFormat
		var meta papxMetadata
		var sprms []byte
		if bOffset != 0 {
			off := bOffset * 2
			if off < 511 {
				cb := int(page[off])
				if cb == 0 {
					// Extended form: 0 | cb2 | istd(2) | grpprl...
					// grpprl length = (cb2-1)*2 (cb2 counts half-words including istd).
					if off+2 < 511 {
						cb2 := int(page[off+1])
						if off+4 <= 512 {
							istd := int(binary.LittleEndian.Uint16(page[off+2 : off+4]))
							format.BinaryStyleIndex = &istd
						}
						grpLen := (cb2 - 1) * 2
						grpOff := off + 4
						if grpLen > 0 && grpOff+grpLen <= 512 {
							sprms = page[grpOff : grpOff+grpLen]
							grpprlFormat, grpprlMeta := parsePapxGrpprlWithMetadata(sprms)
							grpprlFormat.BinaryStyleIndex = format.BinaryStyleIndex
							format = grpprlFormat
							meta = grpprlMeta
						}
					}
				} else {
					// Standard form: cb | istd(2) | grpprl...
					// grpprl length = (cb-1)*2 (cb counts half-words including istd).
					if off+3 <= 512 {
						istd := int(binary.LittleEndian.Uint16(page[off+1 : off+3]))
						format.BinaryStyleIndex = &istd
					}
					grpLen := (cb - 1) * 2
					grpOff := off + 3
					if grpLen > 0 && grpOff+grpLen <= 512 {
						sprms = page[grpOff : grpOff+grpLen]
						grpprlFormat, grpprlMeta := parsePapxGrpprlWithMetadata(sprms)
						grpprlFormat.BinaryStyleIndex = format.BinaryStyleIndex
						format = grpprlFormat
						meta = grpprlMeta
					}
				}
			}
		}
		result = append(result, papxEntry{FC: fc, Format: format, Meta: meta, Sprms: sprms})
	}
	return result
}

func legacyBinaryParagraphStyleID(index int) string {
	return fmt.Sprintf("legacy-istd-%d", index)
}

// parsePapxGrpprl decodes a grpprl byte sequence into a ParagraphFormat.
// Only the PAP SPRMs relevant to visual rendering are handled; unknown SPRMs
// are skipped using the same spra-based size formula used for CHPx.
// Reference: MS-DOC 2.9.256 (sprmTab).
func parsePapxGrpprl(grpprl []byte) wordir.ParagraphFormat {
	out, _ := parsePapxGrpprlWithMetadata(grpprl)
	return out
}

func parsePapxGrpprlWithMetadata(grpprl []byte) (wordir.ParagraphFormat, papxMetadata) {
	var out wordir.ParagraphFormat
	var meta papxMetadata
	pos := 0
	for pos+2 <= len(grpprl) {
		code := binary.LittleEndian.Uint16(grpprl[pos : pos+2])
		pos += 2
		opSize := chpxOperandSize(code, grpprl, pos)
		// Line-spacing sprms carry an LSPD payload (dy int16 + fMultLinespace
		// int16) even when the generic size formula would undercount them.
		// 0x8426 is the documented sprmPLspd code; 0x6412 is observed in legacy
		// DOC fixtures and uses the same 4-byte payload.
		if code == 0x8426 || code == 0x6412 {
			opSize = 4
		}
		// sprmPItap (0x6649) and sprmPDtap (0x664A) both carry 4-byte operands.
		if code == 0x6649 || code == 0x664A {
			opSize = 4
		}
		if opSize < 0 || pos+opSize > len(grpprl) {
			break
		}
		op := grpprl[pos : pos+opSize]
		pos += opSize

		switch code {
		case 0x260A: // sprmPIlvl — list level (1 byte)
			if len(op) >= 1 {
				v := int(op[0])
				out.BinaryListLevel = &v
			}
		case 0x460B: // sprmPIlfo — list format override index (2 bytes)
			if len(op) >= 2 {
				v := int(binary.LittleEndian.Uint16(op))
				out.BinaryListOverride = &v
			}
		case 0xC615: // sprmPChgTabs — delete/add paragraph tab stops
			out.TabStops = applyLegacyTabChange(out.TabStops, op, 2)
		case 0x2403: // sprmPJc — justification (1 byte)
			if len(op) >= 1 {
				switch op[0] {
				case 1:
					out.Alignment = "center"
				case 2:
					out.Alignment = "right"
				case 3:
					out.Alignment = "both"
					// 0 = left — leave Alignment empty (default)
				}
			}
		case 0x2404: // sprmPFSideBySide — side-by-side paragraph toggle (1-byte Bool8)
			// When true, paragraph is part of a side-by-side layout (two paragraphs on same line).
			// This is a legacy Word feature with no direct OOXML equivalent; decoded for coverage.
			if len(op) >= 1 {
				_ = op[0] != 0
			}
		case 0x2405: // sprmPFKeep — keep all lines of paragraph on single page (1-byte Bool8)
			if len(op) >= 1 {
				out.KeepLines = op[0] != 0
			}
		case 0x2406: // sprmPFKeepFollow — keep end of paragraph with start of next (1-byte Bool8)
			if len(op) >= 1 {
				out.KeepNext = op[0] != 0
			}
		case 0x2407: // sprmPFPageBreakBefore — force page break before paragraph (1-byte Bool8)
			if len(op) >= 1 {
				out.PageBreakBefore = op[0] != 0
			}
		case 0x2423: // sprmPWr — frame text wrap mode (1 byte)
			// 0=auto,1=notBeside,2=around,3=none,4=tight,5=through
			// WrapType in IR uses: "none"|"around"|"notBeside"
			if len(op) >= 1 {
				switch op[0] {
				case 0:
					legacyEnsureBinaryFrame(&out).WrapType = "around" // auto → treat as around
				case 1:
					legacyEnsureBinaryFrame(&out).WrapType = "notBeside"
				case 2:
					legacyEnsureBinaryFrame(&out).WrapType = "around"
				case 3:
					legacyEnsureBinaryFrame(&out).WrapType = "none"

				case 4:
					legacyEnsureBinaryFrame(&out).WrapType = "tight"
				case 5:
					legacyEnsureBinaryFrame(&out).WrapType = "around" // through → around
				}
			}
		case 0x465F: // sprmPNest — additive left indent twips, Word2000+ alias for sprmPNest80 (0x4610)
			if len(op) >= 2 {
				delta := int(int16(binary.LittleEndian.Uint16(op)))
				if out.IndentLeft != nil {
					v := *out.IndentLeft + delta
					out.IndentLeft = &v
				} else {
					out.IndentLeft = &delta
				}
			}
		case 0x242A: // sprmPFNoAutoHyph — suppress auto-hyphenation (Bool8)
			if len(op) >= 1 {
				out.NoAutoHyphenation = op[0] != 0
			}
		case 0x840E: // sprmPDxaLeft — left indent, signed twips (2 bytes)
			if len(op) >= 2 {
				v := int(int16(binary.LittleEndian.Uint16(op)))
				out.IndentLeft = &v
			}
		case 0x840F: // sprmPDxaRight — right indent, signed twips (2 bytes)
			if len(op) >= 2 {
				v := int(int16(binary.LittleEndian.Uint16(op)))
				out.IndentRight = &v
			}
		case 0x8411: // sprmPDxaLeft1 — first-line indent, signed twips (2 bytes)
			// Positive = first-line indent; negative = hanging indent.
			if len(op) >= 2 {
				v := int(int16(binary.LittleEndian.Uint16(op)))
				if v < 0 {
					hang := -v
					out.IndentHanging = &hang
				} else if v > 0 {
					out.IndentFirstLine = &v
				}
			}
		case 0x8418: // sprmPDxaAbs — frame absolute X offset (signed twips)
			if len(op) >= 2 {
				legacyEnsureBinaryFrame(&out).X = int(int16(binary.LittleEndian.Uint16(op)))
			}
		case 0x8419: // sprmPDyaAbs — frame absolute Y offset (signed twips)
			if len(op) >= 2 {
				legacyEnsureBinaryFrame(&out).Y = int(int16(binary.LittleEndian.Uint16(op)))
			}
		case 0x841A: // sprmPDxaWidth — frame width (twips)
			if len(op) >= 2 {
				v := int(int16(binary.LittleEndian.Uint16(op)))
				if v < 0 {
					v = -v
				}
				legacyEnsureBinaryFrame(&out).W = v
			}
		case 0x261B: // sprmPPc — frame alignment codes
			if len(op) >= 1 {
				frame := legacyEnsureBinaryFrame(&out)
				frame.XAlign = legacyFrameXAlignFromPC(op[0] & 0x03)
				frame.YAlign = legacyFrameYAlignFromPC((op[0] >> 2) & 0x03)
			}
		case 0xA413: // sprmPDyaBefore — space before, twips (2 bytes)
			if len(op) >= 2 {
				v := int(binary.LittleEndian.Uint16(op))
				if v > 0 {
					out.SpacingBefore = &v
				}
			}
		case 0xA414: // sprmPDyaAfter — space after, twips (2 bytes)
			if len(op) >= 2 {
				v := int(binary.LittleEndian.Uint16(op))
				if v > 0 {
					out.SpacingAfter = &v
				}
			}
		case 0x245B: // sprmPFDyaBeforeAuto — auto-spacing-before flag (1-byte toggle)
			if len(op) >= 1 {
				out.SpacingBeforeAuto = op[0] != 0
			}
		case 0x245C: // sprmPFDyaAfterAuto — auto-spacing-after flag (1-byte toggle)
			if len(op) >= 1 {
				out.SpacingAfterAuto = op[0] != 0
			}
		case 0x2640: // sprmPOutLvl — paragraph outline level (1 byte: 0..8 = heading 1..9, 9 = body)
			if len(op) >= 1 {
				v := int(op[0])
				out.OutlineLevel = &v
			}
		case 0xC63F: // sprmPPropRMark — paragraph property revision mark metadata
			payload := op
			if len(op) >= 8 && int(op[0]) >= 7 {
				payload = op[1:]
			}
			if len(payload) >= 7 {
				meta.PropertyRevision = &legacyPendingRevisionMark{
					Present: payload[0] != 0,
					Ibst:    binary.LittleEndian.Uint16(payload[1:3]),
					HasIbst: true,
					Dttm:    binary.LittleEndian.Uint32(payload[3:7]),
					HasDttm: true,
				}
			}
		case 0x8426, 0x6412: // sprmPLspd variants — line spacing (LSPD payload)
			// fMultLinespace != 0: dy is a multiple of the standard line height (in 1/240).
			// fMultLinespace == 0: dy > 0 = exact in twips; dy < 0 = at-least in twips.
			if len(op) >= 4 {
				dy := int(int16(binary.LittleEndian.Uint16(op[0:2])))
				fMult := int(int16(binary.LittleEndian.Uint16(op[2:4])))
				if fMult != 0 {
					v := dy
					out.LineSpacing = &v
					out.LineRule = "auto"
				} else if dy > 0 {
					v := dy
					out.LineSpacing = &v
					out.LineRule = "exact"
				} else if dy < 0 {
					v := -dy
					out.LineSpacing = &v
					out.LineRule = "atLeast"
				}
			}
		case 0x6424: // sprmPBrcTop80 — top paragraph border (4-byte BRC80)
			if side := parseBRC80(op); side != nil {
				if out.Borders == nil {
					out.Borders = &wordir.ParagraphBorders{}
				}
				out.Borders.Top = side
			}
		case 0x6425: // sprmPBrcLeft80 — left paragraph border
			if side := parseBRC80(op); side != nil {
				if out.Borders == nil {
					out.Borders = &wordir.ParagraphBorders{}
				}
				out.Borders.Left = side
			}
		case 0x6426: // sprmPBrcBottom80 — bottom paragraph border
			if side := parseBRC80(op); side != nil {
				if out.Borders == nil {
					out.Borders = &wordir.ParagraphBorders{}
				}
				out.Borders.Bottom = side
			}
		case 0x6427: // sprmPBrcRight80 — right paragraph border
			if side := parseBRC80(op); side != nil {
				if out.Borders == nil {
					out.Borders = &wordir.ParagraphBorders{}
				}
				out.Borders.Right = side
			}
		case 0xC60D: // sprmPShd — paragraph shading (variable; Shd = 10 bytes)
			// Shd layout: cvFore COLORREF (4), cvBack COLORREF (4), ipat uint16 (2).
			// ipat==0: no shading; ipat==1: solid (use cvFore); else use cvBack.
			if len(op) >= 11 && op[0] == 10 {
				ipat := binary.LittleEndian.Uint16(op[9:11])
				if ipat != 0 {
					var cr uint32
					if ipat == 1 {
						cr = binary.LittleEndian.Uint32(op[1:5]) // cvFore (solid fill)
					} else {
						cr = binary.LittleEndian.Uint32(op[5:9]) // cvBack (background)
					}
					if cr>>24 != 0xFF { // 0xFF high byte = auto/transparent sentinel
						// COLORREF is stored as 0x00BBGGRR in little-endian,
						// so byte order is R=cr&0xFF, G=(cr>>8)&0xFF, B=(cr>>16)&0xFF.
						out.Shading = fmt.Sprintf("%02X%02X%02X",
							cr&0xFF, (cr>>8)&0xFF, (cr>>16)&0xFF)
					}
				}
			}
		case 0x442D: // sprmPShd80 — compact paragraph shading
			if shade := parseLegacyShd80(op); shade != "" {
				out.Shading = shade
			}
		case 0x2416: // sprmPFInTable — paragraph belongs to a table cell (1-byte toggle)
			if len(op) >= 1 {
				out.InTable = op[0] != 0
			}
		case 0x2417: // sprmPFTtp — paragraph is the table-row end mark (1-byte toggle)
			if len(op) >= 1 {
				out.IsTableTrailingParagraph = op[0] != 0
			}
		case 0x240C: // sprmPFNoLineNumb — suppress line numbering for this paragraph (1-byte toggle)
			if len(op) >= 1 {
				out.NoLineNumber = op[0] != 0
			}
		case 0x4610: // sprmPNest80 — additional left indent in twips (additive, signed 2 bytes)
			if len(op) >= 2 {
				delta := int(int16(binary.LittleEndian.Uint16(op)))
				if out.IndentLeft != nil {
					v := *out.IndentLeft + delta
					out.IndentLeft = &v
				} else {
					out.IndentLeft = &delta
				}
			}
		case 0x246D: // sprmPFContextualSpacing — contextual spacing (1-byte toggle)
			if len(op) >= 1 {
				out.ContextualSpacing = op[0] != 0
			}
		case 0x2470: // sprmPFMirrorIndents — mirror left/right indents on facing pages (1-byte toggle)
			if len(op) >= 1 {
				out.MirrorIndents = op[0] != 0
			}
		case 0x2462: // sprmPFNoAllowOverlap — prevent frame overlap (1-byte toggle)
			if len(op) >= 1 {
				out.NoAllowOverlap = op[0] != 0
			}
		case 0xC66F: // sprmPPropRMark — paragraph property revision mark (same payload as 0xC63F)
			payload := op
			if len(op) >= 8 && int(op[0]) >= 7 {
				payload = op[1:]
			}
			if len(payload) >= 7 {
				meta.PropertyRevision = &legacyPendingRevisionMark{
					Present: payload[0] != 0,
					Ibst:    binary.LittleEndian.Uint16(payload[1:3]),
					HasIbst: true,
					Dttm:    binary.LittleEndian.Uint32(payload[3:7]),
					HasDttm: true,
				}
			}
		case 0xC64D: // sprmPShd — full SHDOperand paragraph shading (same layout as 0xC60D)
			if len(op) >= 11 && op[0] == 10 {
				ipat := binary.LittleEndian.Uint16(op[9:11])
				if ipat != 0 {
					var cr uint32
					if ipat == 1 {
						cr = binary.LittleEndian.Uint32(op[1:5])
					} else {
						cr = binary.LittleEndian.Uint32(op[5:9])
					}
					if cr>>24 != 0xFF {
						out.Shading = fmt.Sprintf("%02X%02X%02X",
							cr&0xFF, (cr>>8)&0xFF, (cr>>16)&0xFF)
					}
				}
			}
		case 0xC64E: // sprmPBrcTop — full BrcOperand top border (supersedes Brc80 form 0x6424)
			if side := parseBRC(op); side != nil {
				if out.Borders == nil {
					out.Borders = &wordir.ParagraphBorders{}
				}
				out.Borders.Top = side
			}
		case 0xC64F: // sprmPBrcLeft — full BrcOperand left border
			if side := parseBRC(op); side != nil {
				if out.Borders == nil {
					out.Borders = &wordir.ParagraphBorders{}
				}
				out.Borders.Left = side
			}
		case 0xC650: // sprmPBrcBottom — full BrcOperand bottom border
			if side := parseBRC(op); side != nil {
				if out.Borders == nil {
					out.Borders = &wordir.ParagraphBorders{}
				}
				out.Borders.Bottom = side
			}
		case 0xC651: // sprmPBrcRight — full BrcOperand right border
			if side := parseBRC(op); side != nil {
				if out.Borders == nil {
					out.Borders = &wordir.ParagraphBorders{}
				}
				out.Borders.Right = side
			}
		case 0xC652: // sprmPBrcBetween — inter-paragraph border
			if side := parseBRC(op); side != nil {
				if out.Borders == nil {
					out.Borders = &wordir.ParagraphBorders{}
				}
				out.Borders.Between = side
			}
		case 0xC653: // sprmPBrcBar — bar border
			if side := parseBRC(op); side != nil {
				if out.Borders == nil {
					out.Borders = &wordir.ParagraphBorders{}
				}
				out.Borders.Bar = side
			}
		case 0x6428, 0x6628, 0x6629: // sprmPBrcBetween80 / old aliases — no renderable effect per spec
		case 0x4455: // sprmPDxcRight — right indent in 1/100 char units; convert to twips via grid (approximated)
			// No IR field for char-unit indents; recognized for decode coverage.
		case 0x4456: // sprmPDxcLeft — left indent in 1/100 char units; no IR field yet
		case 0x4457: // sprmPDxcLeft1 — first-line indent in 1/100 char units; no IR field yet
		case 0x4458: // sprmPDylBefore — space before in 1/100 line units
			if len(op) >= 2 {
				v := int(int16(binary.LittleEndian.Uint16(op)))
				if v > 0 {
					out.SpacingBeforeLines = &v
				}
			}
		case 0x4459: // sprmPDylAfter — space after in 1/100 line units
			if len(op) >= 2 {
				v := int(int16(binary.LittleEndian.Uint16(op)))
				if v > 0 {
					out.SpacingAfterLines = &v
				}
			}
		case 0x6465: // sprmPIpgp — PGPInfo.ipgpSelf; structural grouping reference, no IR field
		case 0x646B: // sprmPTableProps — points to PrcData in Data Stream; resolved elsewhere
		case 0x6646: // sprmPHugePapx — points to PrcData in Data Stream; resolved elsewhere
		case 0x4600: // sprmPIstd — paragraph style application; resolved at style-eval level
		case 0xC601: // sprmPIstdPermute — style permute; applied at style-eval level
		case 0x2602: // sprmPIncLvl — outline level offset relative to style istd; applied at style-eval level
		case 0x245A: // sprmPFOpenTch — display hint for open table cell mark; structural/display-only
		case 0x2471: // sprmPTtwo — textbox tight-wrap; no IR field yet
		case 0x2664: // sprmPWall — revision property preservation wall; structural
		case 0xC666: // sprmPCnf — conditional table style formatting; applied at table-build time
		case 0xC669: // sprmPIstdListPermute — no effect per spec
		case 0xC66C: // sprmPTIstdInfo — no effect per spec
		case 0x6467: // sprmPRsid — revision save ID; no IR field yet
			// This is paragraph revision metadata, not a frame property. In
			// particular, do not create BinaryFrame here: PRsid is commonly
			// present in ordinary paragraph styles and doing so promotes every
			// paragraph to an out-of-flow frame during semantic resolution.
		case 0x2430: // sprmPFLocked — frame anchor locked (Bool8)
			if len(op) >= 1 {
				legacyEnsureBinaryFrame(&out).AnchorLock = op[0] != 0
			}
		case 0x2431: // sprmPFWidowControl
			if len(op) >= 1 {
				out.WidowControl = op[0] != 0
			}
		case 0x2433: // sprmPFKinsoku — enable Japanese/CJK line-breaking rules (1-byte toggle)
			if len(op) >= 1 {
				out.Kinsoku = op[0] != 0
			}
		case 0x2434: // sprmPFWordWrap — word-wrap at text boundary (1-byte toggle)
			if len(op) >= 1 {
				v := op[0] != 0
				out.WordWrap = &v
			}
		case 0x2435: // sprmPFOverflowPunct — allow punctuation overflow at text margin (1-byte toggle)
			if len(op) >= 1 {
				out.OverflowPunct = op[0] != 0
			}
		case 0x2436: // sprmPFTopLinePunct — compress punctuation at line start (1-byte toggle)
			if len(op) >= 1 {
				out.TopLinePunct = op[0] != 0
			}
		case 0x2437: // sprmPFAutoSpaceDE — auto-space East Asian and Latin text (1-byte toggle)
			if len(op) >= 1 {
				out.AutoSpaceDE = op[0] != 0
			}
		case 0x2438: // sprmPFAutoSpaceDN — auto-space East Asian text and numbers (1-byte toggle)
			if len(op) >= 1 {
				out.AutoSpaceDN = op[0] != 0
			}
		case 0x2441: // sprmPFBiDi
			if len(op) >= 1 {
				out.Bidi = op[0] != 0
			}
		case 0x2444: // undocumented PAP sprm — intentionally decode-only; no semantic equivalent per MS-DOC spec
		case 0x2447: // sprmPFUsePgsuSettings — use page setup settings for line grid (Bool8)
			if len(op) >= 1 {
				out.UsePageSetup = op[0] != 0
			}
		case 0x2448: // sprmPFAdjustRight — auto-adjust right indent in RTL mode
			if len(op) >= 1 {
				out.AdjustRightInd = op[0] != 0
			}
		case 0x2443: // sprmPFNumRMIns — recognized for decode coverage; no IR field yet
		case 0x244B: // sprmPFInnerTableCell — paragraph mark acts as nested-table cell end
			if len(op) >= 1 {
				out.IsTableCellTrailingParagraph = op[0] != 0
			}
		case 0x244C: // sprmPFInnerTtp — paragraph mark acts as nested-table row end
			if len(op) >= 1 {
				out.IsTableTrailingParagraph = op[0] != 0
			}
		case 0x6649: // sprmPItap — explicit table depth (4 bytes)
			if len(op) >= 4 {
				meta.TableDepth = int(binary.LittleEndian.Uint32(op))
			}
		case 0x664A: // sprmPDtap — delta to existing table depth (4 bytes)
			if len(op) >= 4 {
				meta.TableDepth += int(int32(binary.LittleEndian.Uint32(op)))
			}
		case 0x5400, 0x549A: // sprmTJc90 / sprmTJc — table justification
			// Operand is a 2-byte little-endian integer: 0=left, 1=center, 2=right.
			// 1-byte forms are also seen in older grpprls; trust op[0] as the LSB.
			if len(op) >= 1 {
				if meta.TableRowProps == nil {
					meta.TableRowProps = &legacyTableRowProps{}
				}
				switch op[0] {
				case 0:
					meta.TableRowProps.tableJc = "left"
				case 1:
					meta.TableRowProps.tableJc = "center"
				case 2:
					meta.TableRowProps.tableJc = "right"
				}
			}
		case 0x360D: // sprmTPc — floating-table anchor origins (PositionCodeOperand)
			if len(op) >= 1 {
				if meta.TableRowProps == nil {
					meta.TableRowProps = &legacyTableRowProps{}
				}
				pcVert := (op[0] >> 4) & 0x03
				pcHorz := (op[0] >> 6) & 0x03
				// A value of 3 means "not absolutely positioned" for that axis.
				// sprmTPc is absent for ordinary inline tables, so only materialize
				// FloatProps when the operand actually names an anchor.
				if pcVert != 3 || pcHorz != 3 {
					fp := legacyEnsureTableFloatProps(meta.TableRowProps)
					fp.VertAnchor = legacyTableVertAnchorFromPC(pcVert)
					fp.HorzAnchor = legacyTableHorzAnchorFromPC(pcHorz)
				}
			}
		case 0x940E: // sprmTDxaAbs — floating-table horizontal position (XAS_plusOne)
			if len(op) >= 2 {
				if meta.TableRowProps == nil {
					meta.TableRowProps = &legacyTableRowProps{}
				}
				fp := legacyEnsureTableFloatProps(meta.TableRowProps)
				v := int(int16(binary.LittleEndian.Uint16(op)))
				if align, ok := legacyTableXAlignFromAbs(v); ok {
					fp.XAlign = align
				} else {
					fp.X = v
				}
			}
		case 0x940F: // sprmTDyaAbs — floating-table vertical position (YAS_plusOne)
			if len(op) >= 2 {
				if meta.TableRowProps == nil {
					meta.TableRowProps = &legacyTableRowProps{}
				}
				fp := legacyEnsureTableFloatProps(meta.TableRowProps)
				v := int(int16(binary.LittleEndian.Uint16(op)))
				if align, ok := legacyTableYAlignFromAbs(v); ok {
					fp.YAlign = align
				} else {
					fp.Y = v
				}
			}
		case 0x9410: // sprmTDxaFromText — left wrap distance
			if len(op) >= 2 {
				if meta.TableRowProps == nil {
					meta.TableRowProps = &legacyTableRowProps{}
				}
				legacyEnsureTableFloatProps(meta.TableRowProps).LeftFromText = int(binary.LittleEndian.Uint16(op))
			}
		case 0x9411: // sprmTDyaFromText — top wrap distance
			if len(op) >= 2 {
				if meta.TableRowProps == nil {
					meta.TableRowProps = &legacyTableRowProps{}
				}
				legacyEnsureTableFloatProps(meta.TableRowProps).TopFromText = int(binary.LittleEndian.Uint16(op))
			}
		case 0x941E: // sprmTDxaFromTextRight — right wrap distance
			if len(op) >= 2 {
				if meta.TableRowProps == nil {
					meta.TableRowProps = &legacyTableRowProps{}
				}
				legacyEnsureTableFloatProps(meta.TableRowProps).RightFromText = int(binary.LittleEndian.Uint16(op))
			}
		case 0x941F: // sprmTDyaFromTextBottom — bottom wrap distance
			if len(op) >= 2 {
				if meta.TableRowProps == nil {
					meta.TableRowProps = &legacyTableRowProps{}
				}
				legacyEnsureTableFloatProps(meta.TableRowProps).BottomFromText = int(binary.LittleEndian.Uint16(op))
			}
		case 0x3465: // sprmTFNoAllowOverlap — floating tables must not overlap
			if len(op) >= 1 {
				if meta.TableRowProps == nil {
					meta.TableRowProps = &legacyTableRowProps{}
				}
				b := op[0] != 0
				meta.TableRowProps.noOverlap = &b
			}
		case 0x9601: // sprmTDxaLeft — table left margin/indent (signed twips, 2 bytes)
			if len(op) >= 2 {
				if meta.TableRowProps == nil {
					meta.TableRowProps = &legacyTableRowProps{}
				}
				v := int(int16(binary.LittleEndian.Uint16(op)))
				meta.TableRowProps.tableDxaLeft = &v
			}
		case 0x9602: // sprmTDxaGapHalf — half-gap between table cells (signed twips, 2 bytes)
			if len(op) >= 2 {
				if meta.TableRowProps == nil {
					meta.TableRowProps = &legacyTableRowProps{}
				}
				v := int(int16(binary.LittleEndian.Uint16(op)))
				meta.TableRowProps.tableDxaGap = &v
			}
		case 0x9407: // sprmTDyaRowHeight — signed row height (positive=atLeast, negative=exact)
			if len(op) >= 2 {
				if meta.TableRowProps == nil {
					meta.TableRowProps = &legacyTableRowProps{}
				}
				v := int(int16(binary.LittleEndian.Uint16(op)))
				meta.TableRowProps.rowHeight = &v
			}
		case 0x3403: // sprmTFCantSplit — keep row together across pages (1-byte toggle)
			if len(op) >= 1 {
				if meta.TableRowProps == nil {
					meta.TableRowProps = &legacyTableRowProps{}
				}
				b := op[0] != 0
				meta.TableRowProps.cantSplit = &b
			}
		case 0x3404: // sprmTFTableHeader — repeat row as header on each page (1-byte toggle)
			if len(op) >= 1 {
				if meta.TableRowProps == nil {
					meta.TableRowProps = &legacyTableRowProps{}
				}
				b := op[0] != 0
				meta.TableRowProps.tableHeader = &b
			}
		case 0xF614: // sprmTTableWidth — preferred total width of table (3-byte FtsWWidth_Table)
			if len(op) >= 3 {
				if meta.TableRowProps == nil {
					meta.TableRowProps = &legacyTableRowProps{}
				}
				fts := op[0]
				w := int(binary.LittleEndian.Uint16(op[1:3]))
				switch fts {
				case 1: // ftsNil
					meta.TableRowProps.tableWidthType = "nil"
				case 2: // ftsAuto
					meta.TableRowProps.tableWidthType = "auto"
				case 3: // ftsDxa
					meta.TableRowProps.tableWidthType = "dxa"
					meta.TableRowProps.tableWidth = &w
				case 4: // ftsPercent
					meta.TableRowProps.tableWidthType = "pct"
					meta.TableRowProps.tableWidth = &w
				}
			}
		case 0x3615: // sprmTFAutofit — auto-fit column widths (1-byte Bool8)
			if len(op) >= 1 {
				if meta.TableRowProps == nil {
					meta.TableRowProps = &legacyTableRowProps{}
				}
				b := op[0] != 0
				meta.TableRowProps.autoFit = &b
			}
		case 0x560B: // sprmTFBiDi — table is right-to-left (2-byte Bool16)
			if len(op) >= 2 {
				if meta.TableRowProps == nil {
					meta.TableRowProps = &legacyTableRowProps{}
				}
				b := binary.LittleEndian.Uint16(op) != 0
				meta.TableRowProps.bidiVisual = &b
			}
		case 0x5664: // sprmTFBiDi90 — table is right-to-left, Word 2003+ (2-byte Bool16)
			if len(op) >= 2 {
				if meta.TableRowProps == nil {
					meta.TableRowProps = &legacyTableRowProps{}
				}
				b := binary.LittleEndian.Uint16(op) != 0
				meta.TableRowProps.bidiVisual = &b
			}
		case 0x740A: // sprmTTlp — table style options (4-byte TLP: itl + grfatl)
			// grfatl semantics differ from OOXML w:tblLook; mapped as opaque for now.
		case 0xD605: // sprmTTableBorders — row/table borders (top/left/bottom/right/insideH/insideV)
			if borders, ok := parseLegacyTableBordersOperand(op); ok {
				if meta.TableRowProps == nil {
					meta.TableRowProps = &legacyTableRowProps{}
				}
				meta.TableRowProps.tableBorders = borders
			}
		case 0x442B: // sprmPWHeightAbs — frame height (signed twips)
			if len(op) >= 2 {
				v := int(int16(binary.LittleEndian.Uint16(op)))
				if v < 0 {
					v = -v
				}
				legacyEnsureBinaryFrame(&out).H = v
			}
		case 0x442C: // sprmPDcs — drop-cap specifier
			if len(op) >= 2 {
				frame := legacyEnsureBinaryFrame(&out)
				dcs := binary.LittleEndian.Uint16(op)
				switch byte(dcs & 0x07) {
				case 0:
					frame.DropCap = "none"
				case 2:
					frame.DropCap = "margin"
				default:
					frame.DropCap = "drop"
				}
				frame.Lines = int((dcs >> 3) & 0x1F)
			}
		case 0x842E: // sprmPDyaFromText — vertical distance from surrounding text
			if len(op) >= 2 {
				v := int(int16(binary.LittleEndian.Uint16(op)))
				if v < 0 {
					v = -v
				}
				legacyEnsureBinaryFrame(&out).VSpace = v
			}
		case 0x842F: // sprmPDxaFromText — horizontal distance from surrounding text
			if len(op) >= 2 {
				v := int(int16(binary.LittleEndian.Uint16(op)))
				if v < 0 {
					v = -v
				}
				legacyEnsureBinaryFrame(&out).HSpace = v
			}
		case 0x443A: // sprmPFrameTextFlow — vertical/backward/rotate flags
			if len(op) >= 2 {
				flow := binary.LittleEndian.Uint16(op)
				legacyEnsureBinaryFrame(&out).TextDirection = legacyTableTextDirectionFromTextFlow(flow)
			}
		case 0x4439: // sprmPWAlignFont — vertical font alignment within the line (2-byte enum)
			// 0=auto, 1=top, 2=center, 3=baseline, 4=bottom
			if len(op) >= 2 {
				switch binary.LittleEndian.Uint16(op) {
				case 0:
					out.FontAlign = "auto"
				case 1:
					out.FontAlign = "top"
				case 2:
					out.FontAlign = "center"
				case 3:
					out.FontAlign = "baseline"
				case 4:
					out.FontAlign = "bottom"
				}
			}
		case 0x845D: // later sprmPDxaRight alias
			if len(op) >= 2 {
				v := int(int16(binary.LittleEndian.Uint16(op)))
				out.IndentRight = &v
			}
		case 0x845E: // later sprmPDxaLeft alias
			if len(op) >= 2 {
				v := int(int16(binary.LittleEndian.Uint16(op)))
				out.IndentLeft = &v
			}
		case 0x8460: // later sprmPDxaLeft1 alias
			if len(op) >= 2 {
				v := int(int16(binary.LittleEndian.Uint16(op)))
				if v < 0 {
					hang := -v
					out.IndentHanging = &hang
				} else if v > 0 {
					out.IndentFirstLine = &v
				}
			}
		case 0x2461: // sprmPJcLogical — logical justification
			if len(op) >= 1 {
				switch op[0] {
				case 1:
					out.Alignment = "center"
				case 2:
					out.Alignment = "right"
				case 3:
					out.Alignment = "both"
				}
			}
		case 0xC645: // sprmPNumRM — recognized for decode coverage; no IR field yet
		case 0xD634: // sprmTCellPaddingDefault — default cell padding for selected row cells
			if padding := parseLegacyTableCellPaddingDefaultOperand(op); padding != nil {
				if meta.TableRowProps == nil {
					meta.TableRowProps = &legacyTableRowProps{}
				}
				if padding.cellPadTop != nil {
					meta.TableRowProps.cellPadTop = padding.cellPadTop
				}
				if padding.cellPadBottom != nil {
					meta.TableRowProps.cellPadBottom = padding.cellPadBottom
				}
				if padding.cellPadLeft != nil {
					meta.TableRowProps.cellPadLeft = padding.cellPadLeft
				}
				if padding.cellPadRight != nil {
					meta.TableRowProps.cellPadRight = padding.cellPadRight
				}
			}
		case 0x5624: // sprmTMerge — merge a cell range horizontally
			if len(op) >= 2 && op[1] > op[0] {
				if meta.TableRowProps == nil {
					meta.TableRowProps = &legacyTableRowProps{}
				}
				meta.TableRowProps.setHMerge(int(op[0]), int(op[1]))
			}
		case 0x5625: // sprmTSplit — clear horizontal merge on a cell range
			if len(op) >= 2 && op[1] >= op[0] {
				if meta.TableRowProps == nil {
					meta.TableRowProps = &legacyTableRowProps{}
				}
				meta.TableRowProps.clearHMerge(int(op[0]), int(op[1]))
			}
		case 0xD62B: // sprmTVertMerge — set vertical merge state for one cell
			if len(op) >= 3 && op[0] == 2 {
				if meta.TableRowProps == nil {
					meta.TableRowProps = &legacyTableRowProps{}
				}
				meta.TableRowProps.setVMerge(int(op[1]), op[2])
			}
		case 0xD608: // sprmTDefTable — compatibility row definition with TC80 merge flags
			if props := parseLegacyTDefTableOperand(op); props != nil {
				if meta.TableRowProps == nil {
					meta.TableRowProps = props
				} else {
					for index := 0; index < props.maxCellIndex(); index++ {
						if state := props.hMergeState(index); state != 0 {
							if meta.TableRowProps.hMerge == nil {
								meta.TableRowProps.hMerge = map[int]byte{}
							}
							meta.TableRowProps.hMerge[index] = state
						}
						if state := props.vMergeState(index); state != 0 {
							if meta.TableRowProps.vMerge == nil {
								meta.TableRowProps.vMerge = map[int]byte{}
							}
							meta.TableRowProps.vMerge[index] = state
						}
						if format := props.cellFormat(index); legacyTableCellFormatMeaningful(format) {
							meta.TableRowProps.setCellFormat(index, format)
						}
					}
				}
			}
		case 0xD609: // sprmTDefTableShd80 — default Shd80 for the row
			if shades := parseLegacyDefTableShd80Operand(op); len(shades) > 0 {
				if meta.TableRowProps == nil {
					meta.TableRowProps = &legacyTableRowProps{}
				}
				for index, shading := range shades {
					if shading == "" {
						continue
					}
					meta.TableRowProps.setCellFormat(index, wordir.TableCellFormat{Shading: shading})
				}
			}
		case 0xD60C: // sprmTDefTableShd3rd — default Shd for cells 45–63
			if shades := parseLegacyDefTableShdOperand(op); len(shades) > 0 {
				if meta.TableRowProps == nil {
					meta.TableRowProps = &legacyTableRowProps{}
				}
				for index, shading := range shades {
					if shading == "" {
						continue
					}
					meta.TableRowProps.setCellFormat(44+index, wordir.TableCellFormat{Shading: shading})
				}
			}
		case 0xD612: // sprmTDefTableShd — default Shd for cells 1–22
			if shades := parseLegacyDefTableShdOperand(op); len(shades) > 0 {
				if meta.TableRowProps == nil {
					meta.TableRowProps = &legacyTableRowProps{}
				}
				for index, shading := range shades {
					if shading == "" {
						continue
					}
					meta.TableRowProps.setCellFormat(index, wordir.TableCellFormat{Shading: shading})
				}
			}
		case 0xD616: // sprmTDefTableShd2nd — default Shd for cells 23–44
			if shades := parseLegacyDefTableShdOperand(op); len(shades) > 0 {
				if meta.TableRowProps == nil {
					meta.TableRowProps = &legacyTableRowProps{}
				}
				for index, shading := range shades {
					if shading == "" {
						continue
					}
					meta.TableRowProps.setCellFormat(22+index, wordir.TableCellFormat{Shading: shading})
				}
			}
		case 0xD620: // sprmTSetBrc80 — apply one Brc80MayBeNil side to a cell range
			first, lim, mask, side, ok := parseLegacyTableBrc80Operand(op)
			if ok {
				if meta.TableRowProps == nil {
					meta.TableRowProps = &legacyTableRowProps{}
				}
				applyLegacyCellBorderRange(meta.TableRowProps, first, lim, mask, side, false)
			}
		case 0xD62C: // sprmTVertAlign — apply vertical alignment to a cell range
			first, lim, valign, ok := parseLegacyCellRangeVertAlignOperand(op)
			if ok {
				if meta.TableRowProps == nil {
					meta.TableRowProps = &legacyTableRowProps{}
				}
				meta.TableRowProps.applyCellFormatRange(first, lim, wordir.TableCellFormat{VertAlign: valign}, false)
			}
		case 0xD62D: // sprmTSetShd — apply shading to a cell range
			first, lim, shading, ok := parseLegacyTableShadeOperand(op)
			if ok && shading != "" {
				if meta.TableRowProps == nil {
					meta.TableRowProps = &legacyTableRowProps{}
				}
				meta.TableRowProps.applyCellFormatRange(first, lim, wordir.TableCellFormat{Shading: shading}, false)
			}
		case 0xD62E: // sprmTSetShdOdd — apply shading to every other cell in a range
			first, lim, shading, ok := parseLegacyTableShadeOperand(op)
			if ok && shading != "" {
				if meta.TableRowProps == nil {
					meta.TableRowProps = &legacyTableRowProps{}
				}
				meta.TableRowProps.applyCellFormatRange(first, lim, wordir.TableCellFormat{Shading: shading}, true)
			}
		case 0xD62F: // sprmTSetBrc — apply one BrcMayBeNil side to a cell range
			first, lim, mask, side, ok := parseLegacyTableBrcOperand(op)
			if ok {
				if meta.TableRowProps == nil {
					meta.TableRowProps = &legacyTableRowProps{}
				}
				applyLegacyCellBorderRange(meta.TableRowProps, first, lim, mask, side, false)
			}
		case 0x7629: // sprmTTextFlow — apply text flow to a cell range
			first, lim, direction, ok := parseLegacyTableTextFlowOperand(op)
			if ok {
				if meta.TableRowProps == nil {
					meta.TableRowProps = &legacyTableRowProps{}
				}
				meta.TableRowProps.applyCellFormatRange(first, lim, wordir.TableCellFormat{TextDirection: direction}, false)
			}
		case 0xD635: // sprmTCellWidth — preferred width for a cell range
			first, lim, format, ok := parseLegacyTableCellWidthOperand(op)
			if ok {
				if meta.TableRowProps == nil {
					meta.TableRowProps = &legacyTableRowProps{}
				}
				meta.TableRowProps.applyCellFormatRange(first, lim, format, false)
			}
		case 0xD670: // sprmTDefTableShdRaw — default raw Shd for cells 1–22
			if shades := parseLegacyDefTableShdOperand(op); len(shades) > 0 {
				if meta.TableRowProps == nil {
					meta.TableRowProps = &legacyTableRowProps{}
				}
				for index, shading := range shades {
					if shading == "" {
						continue
					}
					meta.TableRowProps.setCellFormat(index, wordir.TableCellFormat{Shading: shading})
				}
			}
		case 0xD671: // sprmTDefTableShdRaw2nd — default raw Shd for cells 23–44
			if shades := parseLegacyDefTableShdOperand(op); len(shades) > 0 {
				if meta.TableRowProps == nil {
					meta.TableRowProps = &legacyTableRowProps{}
				}
				for index, shading := range shades {
					if shading == "" {
						continue
					}
					meta.TableRowProps.setCellFormat(22+index, wordir.TableCellFormat{Shading: shading})
				}
			}
		case 0xD672: // sprmTDefTableShdRaw3rd — default raw Shd for cells 45–63
			if shades := parseLegacyDefTableShdOperand(op); len(shades) > 0 {
				if meta.TableRowProps == nil {
					meta.TableRowProps = &legacyTableRowProps{}
				}
				for index, shading := range shades {
					if shading == "" {
						continue
					}
					meta.TableRowProps.setCellFormat(44+index, wordir.TableCellFormat{Shading: shading})
				}
			}
		case 0xD667: // sprmTPropRMark — table row property revision mark metadata
			payload := op
			if len(op) >= 8 && int(op[0]) >= 7 {
				payload = op[1:]
			}
			if len(payload) >= 7 {
				if meta.TableRowProps == nil {
					meta.TableRowProps = &legacyTableRowProps{}
				}
				meta.TableRowProps.propertyRevision = &legacyPendingRevisionMark{
					Present: payload[0] != 0,
					Ibst:    binary.LittleEndian.Uint16(payload[1:3]),
					HasIbst: true,
					Dttm:    binary.LittleEndian.Uint32(payload[3:7]),
					HasDttm: true,
				}
			}
		}
	}
	meta.Diagnostics = scanUnmappedSprms(grpprl, "PAP/TAP", unmappedPapxSprms)
	return out, meta
}

func legacyEnsureTableFloatProps(props *legacyTableRowProps) *wordir.TableFloatProps {
	if props.floatProps == nil {
		props.floatProps = &wordir.TableFloatProps{HorzAnchor: "text", VertAnchor: "margin"}
	}
	return props.floatProps
}

func legacyTableHorzAnchorFromPC(code byte) string {
	switch code {
	case 0:
		return "text" // current column
	case 1:
		return "margin"
	case 2:
		return "page"
	default:
		return ""
	}
}

func legacyTableVertAnchorFromPC(code byte) string {
	switch code {
	case 0:
		return "margin"
	case 1:
		return "page"
	case 2:
		return "text" // preceding paragraph bottom
	default:
		return ""
	}
}

func legacyTableXAlignFromAbs(value int) (string, bool) {
	switch value {
	case 0:
		return "left", true
	case -4:
		return "center", true
	case -8:
		return "right", true
	case -12:
		return "inside", true
	case -16:
		return "outside", true
	default:
		return "", false
	}
}

func legacyTableYAlignFromAbs(value int) (string, bool) {
	switch value {
	case 0:
		return "inline", true
	case -4:
		return "top", true
	case -8:
		return "center", true
	case -12:
		return "bottom", true
	case -16:
		return "inside", true
	case -20:
		return "outside", true
	default:
		return "", false
	}
}

func legacyEnsureBinaryFrame(format *wordir.ParagraphFormat) *wordir.FrameProps {
	if format.BinaryFrame == nil {
		format.BinaryFrame = &wordir.FrameProps{
			HAnchor: "margin",
			VAnchor: "margin",
		}
	}
	return format.BinaryFrame
}

func legacyFrameXAlignFromPC(code byte) string {
	switch code {
	case 0:
		return "left"
	case 1:
		return "center"
	case 2:
		return "right"
	default:
		return ""
	}
}

func legacyFrameYAlignFromPC(code byte) string {
	switch code {
	case 0:
		return "top"
	case 1:
		return "center"
	case 2:
		return "bottom"
	default:
		return ""
	}
}

func legacyFrameWrapType(code byte) string {
	switch code {
	case 1:
		return "notBeside"
	case 3:
		return "none"
	case 4:
		return "tight"
	case 5:
		return "through"
	default:
		return "around"
	}
}

func parseLegacyTDefTableOperand(op []byte) *legacyTableRowProps {
	if len(op) < 5 {
		return nil
	}
	colCount := int(op[2])
	if colCount <= 0 {
		return nil
	}
	tcOffset := 3 + (colCount+1)*2
	if tcOffset > len(op) {
		return nil
	}
	props := &legacyTableRowProps{}
	props.cellBoundaries = make([]int, colCount+1)
	for index := 0; index <= colCount; index++ {
		off := 3 + index*2
		if off+2 > len(op) {
			props.cellBoundaries = nil
			break
		}
		props.cellBoundaries[index] = int(int16(binary.LittleEndian.Uint16(op[off : off+2])))
	}
	if props.validCellBoundaries() == nil {
		props.cellBoundaries = nil
	}
	for index := 0; index < colCount; index++ {
		entryOff := tcOffset + index*20
		if entryOff+20 > len(op) {
			break
		}
		tcgrf := binary.LittleEndian.Uint16(op[entryOff : entryOff+2])
		switch byte(tcgrf & 0x3) {
		case 1:
			if props.hMerge == nil {
				props.hMerge = map[int]byte{}
			}
			props.hMerge[index] = 1
		case 2, 3:
			if props.hMerge == nil {
				props.hMerge = map[int]byte{}
			}
			props.hMerge[index] = 2
		}
		switch byte((tcgrf >> 5) & 0x3) {
		case 1:
			props.setVMerge(index, 1)
		case 3:
			props.setVMerge(index, 3)
		}
		var format wordir.TableCellFormat
		format.TextDirection = legacyTableTextDirectionFromTextFlow(uint16((tcgrf >> 2) & 0x7))
		switch byte((tcgrf >> 7) & 0x3) {
		case 1:
			format.VertAlign = "center"
		case 2:
			format.VertAlign = "bottom"
		}
		switch byte((tcgrf >> 9) & 0x7) {
		case 2:
			format.WidthType = "pct"
		case 3:
			format.WidthType = "dxa"
		}
		if format.WidthType == "dxa" || format.WidthType == "pct" {
			width := int(binary.LittleEndian.Uint16(op[entryOff+2 : entryOff+4]))
			format.Width = &width
		}
		borders := &wordir.CellBorders{
			Top:    parseBRC80MayBeNil(op[entryOff+4 : entryOff+8]),
			Left:   parseBRC80MayBeNil(op[entryOff+8 : entryOff+12]),
			Bottom: parseBRC80MayBeNil(op[entryOff+12 : entryOff+16]),
			Right:  parseBRC80MayBeNil(op[entryOff+16 : entryOff+20]),
		}
		if legacyCellBordersHasVisibleData(borders) {
			format.Borders = borders
		}
		if legacyTableCellFormatMeaningful(format) {
			props.setCellFormat(index, format)
		}
	}
	if !props.hasData() {
		return nil
	}
	return props
}

func parseLegacyTableCellPaddingDefaultOperand(op []byte) *legacyTableRowProps {
	if len(op) < 7 {
		return nil
	}
	if int(op[0]) < 6 || len(op) < 1+int(op[0]) {
		return nil
	}
	payload := op[1:]
	if len(payload) < 6 {
		return nil
	}
	if payload[3] != 3 {
		return nil
	}
	width := int(int16(binary.LittleEndian.Uint16(payload[4:6])))
	if width <= 0 {
		return nil
	}
	props := &legacyTableRowProps{}
	if payload[2]&0x01 != 0 {
		v := width
		props.cellPadTop = &v
	}
	if payload[2]&0x02 != 0 {
		v := width
		props.cellPadLeft = &v
	}
	if payload[2]&0x04 != 0 {
		v := width
		props.cellPadBottom = &v
	}
	if payload[2]&0x08 != 0 {
		v := width
		props.cellPadRight = &v
	}
	if !props.hasData() {
		return nil
	}
	return props
}

func parseLegacyDefTableShdOperand(op []byte) []string {
	if len(op) < 1 {
		return nil
	}
	cb := int(op[0])
	if cb <= 0 || cb%10 != 0 || len(op) < 1+cb {
		return nil
	}
	count := cb / 10
	shades := make([]string, 0, count)
	for index := 0; index < count; index++ {
		start := 1 + index*10
		shades = append(shades, parseLegacyShd(op[start:start+10]))
	}
	return shades
}

func parseLegacyDefTableShd80Operand(op []byte) []string {
	if len(op) < 1 {
		return nil
	}
	cb := int(op[0])
	if cb <= 0 || cb%2 != 0 || len(op) < 1+cb {
		return nil
	}
	count := cb / 2
	shades := make([]string, 0, count)
	for index := 0; index < count; index++ {
		start := 1 + index*2
		shades = append(shades, parseLegacyShd80(op[start:start+2]))
	}
	return shades
}

func parseLegacyTableShadeOperand(op []byte) (int, int, string, bool) {
	if len(op) < 13 || op[0] != 12 {
		return 0, 0, "", false
	}
	first, lim, ok := parseLegacyItcFirstLim(op[1:3])
	if !ok {
		return 0, 0, "", false
	}
	return first, lim, parseLegacyShd(op[3:13]), true
}

func parseLegacyTableBrc80Operand(op []byte) (int, int, byte, *wordir.BorderSide, bool) {
	if len(op) < 8 || op[0] != 7 {
		return 0, 0, 0, nil, false
	}
	first, lim, ok := parseLegacyItcFirstLim(op[1:3])
	if !ok {
		return 0, 0, 0, nil, false
	}
	return first, lim, op[3], parseBRC80MayBeNil(op[4:8]), true
}

func parseLegacyTableBrcOperand(op []byte) (int, int, byte, *wordir.BorderSide, bool) {
	if len(op) < 12 || op[0] != 11 {
		return 0, 0, 0, nil, false
	}
	first, lim, ok := parseLegacyItcFirstLim(op[1:3])
	if !ok {
		return 0, 0, 0, nil, false
	}
	return first, lim, op[3], parseBRCMayBeNil(op[4:12]), true
}

func parseLegacyTableBordersOperand(op []byte) (*wordir.TableBorders, bool) {
	if len(op) < 25 || op[0] < 24 {
		return nil, false
	}
	borders := &wordir.TableBorders{
		Top:     parseBRC80MayBeNil(op[1:5]),
		Left:    parseBRC80MayBeNil(op[5:9]),
		Bottom:  parseBRC80MayBeNil(op[9:13]),
		Right:   parseBRC80MayBeNil(op[13:17]),
		InsideH: parseBRC80MayBeNil(op[17:21]),
		InsideV: parseBRC80MayBeNil(op[21:25]),
	}
	for _, side := range []*wordir.BorderSide{borders.Top, borders.Left, borders.Bottom, borders.Right, borders.InsideH, borders.InsideV} {
		if side != nil {
			return borders, true
		}
	}
	return nil, false
}

func parseLegacyCellRangeVertAlignOperand(op []byte) (int, int, string, bool) {
	if len(op) < 4 || op[0] != 3 {
		return 0, 0, "", false
	}
	first, lim, ok := parseLegacyItcFirstLim(op[1:3])
	if !ok {
		return 0, 0, "", false
	}
	switch op[3] {
	case 1:
		return first, lim, "center", true
	case 2:
		return first, lim, "bottom", true
	default:
		return first, lim, "top", true
	}
}

func parseLegacyTableCellWidthOperand(op []byte) (int, int, wordir.TableCellFormat, bool) {
	if len(op) < 6 || op[0] != 5 {
		return 0, 0, wordir.TableCellFormat{}, false
	}
	first, lim, ok := parseLegacyItcFirstLim(op[1:3])
	if !ok {
		return 0, 0, wordir.TableCellFormat{}, false
	}
	format := wordir.TableCellFormat{}
	switch op[3] {
	case 2:
		format.WidthType = "pct"
	case 3, 0x13:
		format.WidthType = "dxa"
	default:
		return 0, 0, wordir.TableCellFormat{}, false
	}
	if format.WidthType == "dxa" || format.WidthType == "pct" {
		width := int(int16(binary.LittleEndian.Uint16(op[4:6])))
		if width < 0 {
			width = 0
		}
		format.Width = &width
	}
	return first, lim, format, legacyTableCellFormatMeaningful(format)
}

func parseLegacyTableTextFlowOperand(op []byte) (int, int, string, bool) {
	if len(op) < 4 {
		return 0, 0, "", false
	}
	first, lim, ok := parseLegacyItcFirstLim(op[0:2])
	if !ok {
		return 0, 0, "", false
	}
	direction := legacyTableTextDirectionFromTextFlow(binary.LittleEndian.Uint16(op[2:4]))
	if direction == "" {
		return 0, 0, "", false
	}
	return first, lim, direction, true
}

func legacyTableTextDirectionFromTextFlow(flow uint16) string {
	switch flow {
	case 1:
		return "tbRl"
	case 3:
		return "btLr"
	case 5:
		return "tbRlV"
	default:
		return ""
	}
}

func parseLegacyItcFirstLim(op []byte) (int, int, bool) {
	if len(op) < 2 {
		return 0, 0, false
	}
	first := int(op[0])
	lim := int(op[1])
	if lim <= first {
		return 0, 0, false
	}
	return first, lim, true
}

func applyLegacyCellBorderRange(props *legacyTableRowProps, first, lim int, mask byte, side *wordir.BorderSide, oddOnly bool) {
	if props == nil || lim <= first || mask == 0 {
		return
	}
	borders := &wordir.CellBorders{}
	if mask&0x01 != 0 {
		borders.Top = cloneLegacyBorderSide(side)
	}
	if mask&0x02 != 0 {
		borders.Left = cloneLegacyBorderSide(side)
	}
	if mask&0x04 != 0 {
		borders.Bottom = cloneLegacyBorderSide(side)
	}
	if mask&0x08 != 0 {
		borders.Right = cloneLegacyBorderSide(side)
	}
	if borders.Top == nil && borders.Left == nil && borders.Bottom == nil && borders.Right == nil {
		return
	}
	props.applyCellFormatRange(first, lim, wordir.TableCellFormat{Borders: borders}, oddOnly)
}

func parseLegacyShd(op []byte) string {
	if len(op) < 10 {
		return ""
	}
	ipat := binary.LittleEndian.Uint16(op[8:10])
	if ipat == 0 || ipat == 0xFFFF {
		return ""
	}
	var cr uint32
	if ipat == 1 {
		cr = binary.LittleEndian.Uint32(op[0:4])
	} else {
		cr = binary.LittleEndian.Uint32(op[4:8])
	}
	return legacyColorRefToHex(cr)
}

func parseLegacyShd80(op []byte) string {
	if len(op) < 2 {
		return ""
	}
	raw := binary.LittleEndian.Uint16(op)
	icoFore := byte(raw & 0x1F)
	icoBack := byte((raw >> 5) & 0x1F)
	ipat := uint16((raw >> 10) & 0x3F)
	if ipat == 0 || ipat == 0x3F {
		return ""
	}
	if ipat == 1 {
		return icoToHex(icoFore)
	}
	return icoToHex(icoBack)
}

func parseBRC80MayBeNil(op []byte) *wordir.BorderSide {
	if len(op) < 4 {
		return nil
	}
	if binary.LittleEndian.Uint32(op) == 0xFFFFFFFF {
		return &wordir.BorderSide{Val: "none"}
	}
	return parseBRC80(op)
}

func parseBRCMayBeNil(op []byte) *wordir.BorderSide {
	if len(op) < 8 {
		return nil
	}
	if binary.LittleEndian.Uint32(op[4:8]) == 0xFFFFFFFF {
		return &wordir.BorderSide{Val: "none"}
	}
	return parseBRC(op)
}

func parseBRC(op []byte) *wordir.BorderSide {
	if len(op) < 8 {
		return nil
	}
	// For spra=6 variable-length operands, skip the 1-byte cb prefix when
	// it equals 8 (the size of a Brc).
	if len(op) == 9 && op[0] == 8 {
		op = op[1:]
	}
	brcType := op[5]
	if brcType == 0 || brcType == 0xFF {
		return nil
	}
	width := float64(op[4]) / 8.0
	if brcType >= 0x40 {
		width = float64(op[4])
	}
	if width > 0 && width < 0.25 {
		width = 0.25
	}
	flags := binary.LittleEndian.Uint16(op[6:8])
	return &wordir.BorderSide{
		Val:     brc80TypeToVal(brcType),
		SizePt:  width,
		Color:   legacyColorRefToHex(binary.LittleEndian.Uint32(op[0:4])),
		SpacePt: float64(flags & 0x1F),
	}
}

func legacyColorRefToHex(cr uint32) string {
	if cr>>24 == 0xFF {
		return ""
	}
	return fmt.Sprintf("%02X%02X%02X", cr&0xFF, (cr>>8)&0xFF, (cr>>16)&0xFF)
}

func legacyCellBordersHasVisibleData(borders *wordir.CellBorders) bool {
	if borders == nil {
		return false
	}
	for _, side := range []*wordir.BorderSide{borders.Top, borders.Left, borders.Bottom, borders.Right} {
		if side != nil && side.Val != "none" {
			return true
		}
	}
	return false
}

// parseBRC80 decodes a 4-byte BRC80 (Border Code) record and returns a
// BorderSide, or nil when the border type indicates no border.
// MS-DOC §2.9.22 Brc80 field order:
//   - byte 0: dptLineWidth — width in eighths of a point
//   - byte 1: brcType     — 0=none, 1=single, 2=thick, 3=double, 4=hair, …
//   - byte 2: ico         — ICO colour index (0 = auto)
//   - byte 3: dptSpace    — gap between text and border in points
func parseBRC80(op []byte) *wordir.BorderSide {
	if len(op) < 4 {
		return nil
	}
	dptLineWidth := op[0]
	brcType := op[1]
	if brcType == 0 || brcType == 0xFF {
		return nil // no border or explicit "none"
	}
	return &wordir.BorderSide{
		Val:     brc80TypeToVal(brcType),
		SizePt:  float64(dptLineWidth) / 8.0,
		Color:   icoToHex(op[2]),
		SpacePt: float64(op[3]),
	}
}

// brc80TypeToVal maps a BRC80 border-type byte to a wordir.BorderSide.Val string.
func brc80TypeToVal(t byte) string {
	switch t {
	case 1:
		return "single"
	case 2:
		return "thick"
	case 3:
		return "double"
	case 4:
		return "dotted" // hair line
	case 5:
		return "dotted"
	case 6:
		return "dashed"
	default:
		return "single"
	}
}

// findPapxEntry returns the index of the last entry in sorted entries whose FC
// is ≤ fc, or -1 if all entries have FC > fc.
func findPapxEntry(entries []papxEntry, fc uint32) int {
	if len(entries) == 0 || entries[0].FC > fc {
		return -1
	}
	lo, hi, best := 0, len(entries)-1, 0
	for lo <= hi {
		mid := (lo + hi) / 2
		if entries[mid].FC <= fc {
			best = mid
			lo = mid + 1
		} else {
			hi = mid - 1
		}
	}
	return best
}

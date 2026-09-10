package legacy

import (
	"encoding/binary"
	"fmt"

	"office/pkg/diag"
	wordir "office/pkg/word/ir"
)

// chpxEntry holds one FC-indexed character formatting record from a ChpxFkp page.
// FC is the raw FcCompressed value of the first character this formatting applies to.
// All characters from FC up to (but not including) the next entry's FC use Format.
type chpxEntry struct {
	FC     uint32
	Format wordir.RunFormat
	// Revision is a best-effort mapping for legacy tracked-change flags.
	Revision string
	// Spec is true when the run is marked as a special character run (fSpec).
	Spec bool
	// Data marks a binary picture/data anchor (sprmCFData).
	Data bool
	// Obj marks an embedded OLE object anchor (sprmCFObj).
	Obj bool
	// IbstRMark is the SttbfRMark index for the run's revision author (sprmCIbstRMark/CIbstRMarkDel).
	// HasIbstRMark distinguishes "explicit 0" from "unset".
	IbstRMark    uint16
	HasIbstRMark bool
	// DttmRMark is the packed Word DTTM for the run's revision (sprmCDttmRMark/CDttmRMarkDel).
	DttmRMark    uint32
	HasDttmRMark bool
	// PropertyRevision carries tracked run-format change attribution from
	// sprmCPropRMark when present.
	PropertyRevision *legacyPendingRevisionMark
	// DisplayFieldRevision carries display-field revision metadata from
	// sprmCDispFldRMark when present.
	DisplayFieldRevision *legacyPendingRevisionMark
	// Diagnostics accumulates info-level issues from CHPX parsing.
	Diagnostics diag.List
	// Sprms stores the raw CHPX grpprl bytes so that lossless IR can round-trip
	// or reproject formatting without re-reading the binary file.
	Sprms []byte
}

type chpxMetadata struct {
	Revision             string
	Spec                 bool
	Data                 bool
	Obj                  bool
	IbstRMark            uint16
	HasIbstRMark         bool
	DttmRMark            uint32
	HasDttmRMark         bool
	PropertyRevision     *legacyPendingRevisionMark
	DisplayFieldRevision *legacyPendingRevisionMark
	Diagnostics          diag.List
}

// loadChpxEntries reads the PlcBteChpx from the table stream (location given by
// FibRgFcLcb97[12]) and returns a sorted list of chpxEntry values covering all
// character positions in the document.
//
// fibBase is the byte offset of FibRgFcLcb97[0] within wordStream.
// cbRgFcLcb is the count of entries present in the FibRgFcLcb97 array.
func loadChpxEntries(wordStream, tableStream []byte, fibBase, cbRgFcLcb int) []chpxEntry {
	const chpxFibIndex = 12
	if chpxFibIndex >= cbRgFcLcb {
		return nil
	}
	if fibBase+(chpxFibIndex+1)*8 > len(wordStream) {
		return nil
	}
	fc := int(binary.LittleEndian.Uint32(wordStream[fibBase+chpxFibIndex*8:]))
	lcb := int(binary.LittleEndian.Uint32(wordStream[fibBase+chpxFibIndex*8+4:]))
	if lcb < 12 || fc < 0 || fc+lcb > len(tableStream) {
		return nil
	}

	plc := tableStream[fc : fc+lcb]
	fontNames := loadLegacyFontNames(wordStream, tableStream, fibBase, cbRgFcLcb)
	// PlcBteChpx layout: (n+1) × 4-byte FC values, then n × 4-byte page numbers.
	// n = (lcb - 4) / 8
	n := (lcb - 4) / 8
	if n <= 0 || (n+1)*4+n*4 > lcb {
		return nil
	}

	var entries []chpxEntry
	for i := 0; i < n; i++ {
		pn := int(binary.LittleEndian.Uint32(plc[(n+1)*4+i*4:]))
		pageOff := pn * 512
		if pageOff+512 > len(wordStream) {
			continue
		}
		entries = append(entries, parseChpxFKPPageWithFonts(wordStream[pageOff:pageOff+512], fontNames)...)
	}
	return entries
}

// parseChpxFKPPage decodes a single 512-byte ChpxFkp page and returns its
// chpxEntry records.  The structure is described in MS-DOC 2.9.90.
func parseChpxFKPPage(page []byte) []chpxEntry {
	return parseChpxFKPPageWithFonts(page, nil)
}

func parseChpxFKPPageWithFonts(page []byte, fontNames []string) []chpxEntry {
	if len(page) < 512 {
		return nil
	}
	crun := int(page[511])
	if crun == 0 {
		return nil
	}
	// rgfc[0..crun]: (crun+1)*4 bytes of raw FcCompressed values.
	// rgb[0..crun-1]: crun bytes, each = 2-byte-unit offset of the Chpx within
	// the page (0 means "use default formatting").
	if (crun+1)*4+crun > 511 {
		return nil
	}
	rgbxOff := (crun + 1) * 4
	result := make([]chpxEntry, 0, crun)
	for i := 0; i < crun; i++ {
		fc := binary.LittleEndian.Uint32(page[i*4 : i*4+4])
		bx := int(page[rgbxOff+i])
		var format wordir.RunFormat
		var meta chpxMetadata
		var sprms []byte
		if bx != 0 {
			chpxOff := bx * 2
			if chpxOff+1 < 512 {
				cb := int(page[chpxOff])
				if cb > 0 && chpxOff+1+cb <= 512 {
					sprms = page[chpxOff+1 : chpxOff+1+cb]
					format, meta = parseChpxGrpprlWithMetadataAndFonts(sprms, fontNames)
				}
			}
		}
		result = append(result, chpxEntry{
			FC:                   fc,
			Format:               format,
			Revision:             meta.Revision,
			Spec:                 meta.Spec,
			Data:                 meta.Data,
			Obj:                  meta.Obj,
			IbstRMark:            meta.IbstRMark,
			HasIbstRMark:         meta.HasIbstRMark,
			DttmRMark:            meta.DttmRMark,
			HasDttmRMark:         meta.HasDttmRMark,
			PropertyRevision:     meta.PropertyRevision,
			DisplayFieldRevision: meta.DisplayFieldRevision,
			Diagnostics:          meta.Diagnostics,
			Sprms:                sprms,
		})
	}
	return result
}

// parseChpxGrpprl decodes a grpprl byte sequence (sequence of Prl records) into
// a RunFormat.  Only the CHP SPRMs relevant to visual rendering are handled;
// unknown SPRMs are skipped using the spra-based size formula.
// Reference: MS-DOC 2.9.256 (sprmTab) and 2.9.176 (FcCompressed).
func parseChpxGrpprl(grpprl []byte) wordir.RunFormat {
	out, _ := parseChpxGrpprlWithMetadataAndFonts(grpprl, nil)
	return out
}

func parseChpxGrpprlWithMetadata(grpprl []byte) (wordir.RunFormat, chpxMetadata) {
	return parseChpxGrpprlWithMetadataAndFonts(grpprl, nil)
}

func parseChpxGrpprlWithMetadataAndFonts(grpprl []byte, fontNames []string) (wordir.RunFormat, chpxMetadata) {
	var out wordir.RunFormat
	var meta chpxMetadata
	pos := 0
	for pos+2 <= len(grpprl) {
		code := binary.LittleEndian.Uint16(grpprl[pos : pos+2])
		pos += 2
		opSize := chpxOperandSize(code, grpprl, pos)
		if opSize < 0 || pos+opSize > len(grpprl) {
			break
		}
		op := grpprl[pos : pos+opSize]
		pos += opSize

		switch code {
		case 0x0801: // sprmCFRMarkIns — inserted revision run (best-effort mapping)
			if len(op) >= 1 {
				if chpxToggleBool(op[0]) {
					meta.Revision = "inserted"
				} else if meta.Revision == "inserted" {
					meta.Revision = ""
				}
			}
		case 0x0800: // sprmCFRMarkDel — deletion revision run
			if len(op) >= 1 {
				if chpxToggleBool(op[0]) {
					meta.Revision = "deleted"
				} else if meta.Revision == "deleted" {
					meta.Revision = ""
				}
			}
		case 0x0802: // sprmCFFldVanish — field instruction text vanish
			// Structural field visibility, not ordinary character vanish.
			// Field materialization owns suppression of instruction text.
		case 0x4804, 0x4863: // sprmCIbstRMark / sprmCIbstRMarkDel — author index into SttbfRMark (2 bytes)
			if len(op) >= 2 {
				meta.IbstRMark = binary.LittleEndian.Uint16(op)
				meta.HasIbstRMark = true
			}
		case 0x6805, 0x6864: // sprmCDttmRMark / sprmCDttmRMarkDel — packed DTTM (4 bytes)
			if len(op) >= 4 {
				meta.DttmRMark = binary.LittleEndian.Uint32(op)
				meta.HasDttmRMark = true
			}
		case 0x0835: // sprmCFBold — bold toggle
			if len(op) >= 1 {
				v := chpxToggleBool(op[0])
				out.Bold = &v
			}
		case 0x0836: // sprmCFItalic — italic toggle
			if len(op) >= 1 {
				v := chpxToggleBool(op[0])
				out.Italic = &v
			}
		case 0x0837: // sprmCFStrike — strikethrough toggle
			if len(op) >= 1 {
				v := chpxToggleBool(op[0])
				out.Strike = &v
			}
		case 0x083A: // sprmCFSmallCaps — small-caps toggle
			if len(op) >= 1 {
				v := chpxToggleBool(op[0])
				out.SmallCaps = &v
			}
		case 0x083B: // sprmCFCaps — all-caps toggle
			if len(op) >= 1 {
				v := chpxToggleBool(op[0])
				out.Caps = &v
			}
		case 0x083C: // sprmCFVanish — hidden text toggle
			if len(op) >= 1 {
				v := chpxToggleBool(op[0])
				out.Vanish = &v
			}
		case 0x0838: // sprmCFOutline — outline text toggle
			if len(op) >= 1 {
				v := chpxToggleBool(op[0])
				out.Outline = &v
			}
		case 0x0839: // sprmCFShadow — shadow text toggle
			if len(op) >= 1 {
				v := chpxToggleBool(op[0])
				out.Shadow = &v
			}
		case 0x0854: // sprmCFImprint — imprint/engrave text toggle
			if len(op) >= 1 {
				v := chpxToggleBool(op[0])
				out.Imprint = &v
			}
		case 0x0858: // sprmCFEmboss — emboss text toggle
			if len(op) >= 1 {
				v := chpxToggleBool(op[0])
				out.Emboss = &v
			}
		case 0x2A34: // sprmCKcd — kumimoji emphasisMark / character emphasis
			if len(op) >= 1 {
				out.EmphasisMark = chpxEmphasisMark(op[0])
			}
		case 0x2A53: // sprmCFDStrike — double-strikethrough toggle
			if len(op) >= 1 {
				v := chpxToggleBool(op[0])
				out.DoubleStrike = &v
			}
		case 0x2A3E: // sprmCKul — underline style
			if len(op) >= 1 && op[0] != 0 {
				out.Underline = chpxUnderlineStyle(op[0])
			}
		case 0x2A42: // sprmCIco — color index (ICO palette)
			if len(op) >= 1 {
				out.Color = icoToHex(op[0])
			}
		case 0x2A0C: // sprmCHighlight — text highlight color index (icoHighlight)
			if len(op) >= 1 {
				out.Highlight = icoHighlightName(op[0])
			}
		case 0x6870: // sprmCCv — 32-bit COLORREF (0x00RRGGBB little-endian)
			if len(op) >= 4 {
				// Skip "auto" sentinel (0xFF000000 or fully 0xFF bytes).
				if op[3] != 0xFF {
					out.Color = fmt.Sprintf("%02X%02X%02X", op[0], op[1], op[2])
				}
			}
		case 0x4A43: // sprmCHps — font size in half-points
			if len(op) >= 2 {
				hps := int(binary.LittleEndian.Uint16(op))
				if hps > 0 {
					out.FontSize = &hps
				}
			}
		case 0x2A48: // sprmCIss — superscript / subscript
			if len(op) >= 1 {
				switch op[0] {
				case 1:
					out.VertAlignVal = "superscript"
				case 2:
					out.VertAlignVal = "subscript"
				}
			}
		case 0x0855: // sprmCFSpec — special-character run (e.g. object/image anchors)
			if len(op) >= 1 {
				meta.Spec = chpxToggleBool(op[0])
			}
		case 0x4845: // sprmCHpsPos — vertical position in half-points
			if len(op) >= 2 {
				pos := int(int16(binary.LittleEndian.Uint16(op)))
				out.Position = &pos
				switch {
				case pos > 0:
					out.VertAlignVal = "superscript"
				case pos < 0:
					out.VertAlignVal = "subscript"
				}
			}
		case 0x4A4F: // sprmCRgFtc0 / sprmCFtcAscii — font index into the legacy font table
			if len(op) >= 2 {
				ftc := int(binary.LittleEndian.Uint16(op))
				if ftc >= 0 && ftc < len(fontNames) && fontNames[ftc] != "" {
					out.FontASCII = fontNames[ftc]
					out.FontHAnsi = fontNames[ftc]
				}
			}
		case 0x4A50: // sprmCRgFtc1 / sprmCFtcFE — East Asian font index
			if len(op) >= 2 {
				ftc := int(binary.LittleEndian.Uint16(op))
				if ftc >= 0 && ftc < len(fontNames) && fontNames[ftc] != "" {
					out.FontEastAsia = fontNames[ftc]
				}
			}
		case 0x4A51: // sprmCRgFtc2 / sprmCFtcOther — complex script font index
			if len(op) >= 2 {
				ftc := int(binary.LittleEndian.Uint16(op))
				if ftc >= 0 && ftc < len(fontNames) && fontNames[ftc] != "" {
					out.FontCS = fontNames[ftc]
				}
			}
		case 0x8840: // sprmCDxaSpace — character spacing in twips (signed)
			if len(op) >= 2 {
				v := int(int16(binary.LittleEndian.Uint16(op)))
				out.CharacterSpacing = &v
			}
		case 0x4A41: // sprmCLid — language ID (2 bytes)
			if len(op) >= 2 {
				lid := binary.LittleEndian.Uint16(op)
				out.Lang = legacyLidToBCP47(lid)
			}
		case 0x083F: // sprmCRtl — right-to-left complex script (1-byte toggle)
			if len(op) >= 1 {
				v := chpxToggleBool(op[0])
				out.RTL = &v
			}
		case 0x486D: // sprmCShd — run shading (10 bytes Shd)
			if shade := parseLegacyRunShading(op); shade != "" {
				out.Background = shade
			}
		case 0x2A44: // sprmCHpsInc — relative font-size delta in half-points
			if len(op) >= 1 && out.FontSize != nil {
				hps := legacyAdjustRelativeFontSize(*out.FontSize, int(int8(op[0]))*2)
				out.FontSize = &hps
			}
		case 0x085A: // sprmCFBiDi — right-to-left layout toggle
			if len(op) >= 1 {
				v := chpxToggleBool(op[0])
				out.RTL = &v
			}
		case 0x085C: // sprmCFBoldBi — complex-script bold
			if len(op) >= 1 && out.Bold == nil {
				v := chpxToggleBool(op[0])
				out.Bold = &v
			}
		case 0x085D: // sprmCFItalicBi — complex-script italic
			if len(op) >= 1 && out.Italic == nil {
				v := chpxToggleBool(op[0])
				out.Italic = &v
			}
		case 0x4A5E: // sprmCFtcBi — complex-script / bidi font index
			if len(op) >= 2 {
				ftc := int(binary.LittleEndian.Uint16(op))
				if ftc >= 0 && ftc < len(fontNames) && fontNames[ftc] != "" {
					out.FontCS = fontNames[ftc]
				}
			}
		case 0x485F: // sprmCLidBi — complex-script / bidi language
			if len(op) >= 2 && out.Lang == "" {
				lid := binary.LittleEndian.Uint16(op)
				out.Lang = legacyLidToBCP47(lid)
			}
		case 0x4A60: // sprmCIcoBi — complex-script / bidi color
			if len(op) >= 1 && out.Color == "" {
				out.Color = icoToHex(op[0])
			}
		case 0x4A61: // sprmCHpsBi — complex-script / bidi font size
			if len(op) >= 2 && out.FontSize == nil {
				hps := int(binary.LittleEndian.Uint16(op))
				if hps > 0 {
					out.FontSize = &hps
				}
			}
		case 0x4A4D: // sprmCHpsMul — percentage font-size growth from inherited/current size
			if len(op) >= 2 && out.FontSize != nil {
				base := *out.FontSize
				pct := int(int16(binary.LittleEndian.Uint16(op)))
				hps := legacyAdjustRelativeFontSize(base, (base*pct)/100)
				out.FontSize = &hps
			}
		case 0x4866: // sprmCShd80 — compact run shading
			if shade := parseLegacyShd80(op); shade != "" {
				out.Background = shade
			}
		case 0x4A30: // sprmCIstd — character style index (2 bytes)
			if len(op) >= 2 {
				v := int(binary.LittleEndian.Uint16(op))
				out.BinaryStyleIndex = &v
			}
		case 0x6A03: // sprmCPicLocation — picture / OLE object file offset (4 bytes)
			if len(op) >= 4 {
				out.PicLocation = int(binary.LittleEndian.Uint32(op))
			}
		case 0x0806: // sprmCFData — binary picture/data run flag
			if len(op) >= 1 {
				meta.Data = chpxToggleBool(op[0])
			}
		case 0x484B: // sprmCHpsKern — minimum font size for kerning, in half-points (2 bytes)
			if len(op) >= 2 {
				v := int(binary.LittleEndian.Uint16(op))
				out.Kerning = &v
			}
		case 0x484E: // sprmCHresi — reserved/non-renderable; no IR field
		case 0xCA47: // sprmCMajority — style-majority reset; clears direct formatting to style defaults
			resetChpxToPlain(&out)
		case 0xCA4C: // sprmCMajority50 — style-majority50 reset; clears direct formatting to style defaults
			resetChpxToPlain(&out)
		case 0xCA57: // sprmCPropRMark — tracked run property revision metadata
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
		case 0xCA62: // sprmCDispFldRMark — display-field revision metadata
			payload := op
			if len(op) >= 1 && int(op[0]) == len(op)-1 {
				payload = op[1:]
			}
			if len(payload) >= 7 {
				meta.DisplayFieldRevision = &legacyPendingRevisionMark{
					Present: payload[0] != 0,
					Ibst:    binary.LittleEndian.Uint16(payload[1:3]),
					HasIbst: true,
					Dttm:    binary.LittleEndian.Uint32(payload[3:7]),
					HasDttm: true,
				}
			}
		case 0x4865: // sprmCBrc — run border (Brc10, single border applied to all four sides)
			if len(op) >= 2 {
				if side := parseBRC10(op[0:2]); side != nil {
					out.Borders = &wordir.RunBorders{Top: side, Left: side, Bottom: side, Right: side}
				}
			}
		case 0x4852: // sprmCCharScale — horizontal character scale as a percentage (2-byte unsigned)
			// Per MS-DOC §2.6.1: value MUST be ≥1 and ≤600; 100=normal.
			if len(op) >= 2 {
				v := int(binary.LittleEndian.Uint16(op))
				if v >= 1 && v <= 600 {
					out.CharScale = &v
				}
			}
		case 0x0867: // obsolete CHP sprm — intentionally decode-only; no semantic equivalent per MS-DOC spec
		case 0x0868: // sprmCFUsePgsuSettings — use page setup settings for line grid (Bool8)
			if len(op) >= 1 {
				v := op[0] != 0
				out.UsePageSetup = &v
			}
		case 0x0869: // legacy CHP sprm — intentionally decode-only; undocumented/no semantic equivalent per MS-DOC spec
		case 0x086B: // legacy CHP sprm — intentionally decode-only; undocumented/no semantic equivalent per MS-DOC spec
		case 0x086C: // legacy CHP sprm — intentionally decode-only; undocumented/no semantic equivalent per MS-DOC spec
		case 0x4A6E: // sprmCLidFE — FE language fallback when main language is absent
			if len(op) >= 2 && out.Lang == "" {
				lid := binary.LittleEndian.Uint16(op)
				out.Lang = legacyLidToBCP47(lid)
			}
		case 0x4873: // sprmCRgLid0 — primary language (non-East-Asian); supersedes sprmCRgLid0_80
			if len(op) >= 2 {
				lid := binary.LittleEndian.Uint16(op)
				if tag := legacyLidToBCP47(lid); tag != "" {
					out.Lang = tag
				}
			}
		case 0x486E: // sprmCRgLid1_80 — East Asian language ID (Word 6.0/95 format, 2 bytes)
			if len(op) >= 2 && out.Lang == "" {
				lid := binary.LittleEndian.Uint16(op)
				if tag := legacyLidToBCP47(lid); tag != "" {
					out.Lang = tag
				}
			}
		case 0x4874: // sprmCRgLid1 — East Asian language; supersedes sprmCRgLid1_80
			if len(op) >= 2 && out.Lang == "" {
				lid := binary.LittleEndian.Uint16(op)
				if tag := legacyLidToBCP47(lid); tag != "" {
					out.Lang = tag
				}
			}
		case 0x6877: // sprmCCvUl — underline color as COLORREF (4 bytes)
			if len(op) >= 4 && op[3] != 0xFF {
				out.UnderlineColor = fmt.Sprintf("%02X%02X%02X", op[0], op[1], op[2])
			}
		case 0x0811: // sprmCFWebHidden — text hidden in Web Layout view (1-byte toggle)
			if len(op) >= 1 {
				v := chpxToggleBool(op[0])
				out.WebHidden = &v
			}
		case 0x0882: // sprmCFComplexScripts — force complex-script formatting (1-byte toggle)
			if len(op) >= 1 {
				v := chpxToggleBool(op[0])
				out.ComplexScript = &v
			}
		case 0x286F: // sprmCIdctHint — East Asian font/lang hint (0x00=default,0x01=eastAsia,0x02=cs); no IR field
		case 0x0856: // sprmCFObj — text represents embedded OLE object
			if len(op) >= 1 {
				meta.Obj = chpxToggleBool(op[0])
			}
		case 0x2859: // sprmCSfxText — animated text effects (0=none,1-6=various effects)
			if len(op) >= 1 {
				out.TextAnimation = legacyTextAnimationFromSfx(op[0])
			}
		case 0x6A09: // sprmCSymbol — symbol character (font+codepoint); handled via fSpec/pic path
		case 0xCA71: // sprmCShd — variable-length SHDOperand run shading; parsed below via 0x486D alias
			if shade := parseLegacyRunShading(op); shade != "" {
				out.Background = shade
			}
		case 0xCA72: // sprmCBrc — character borders (BrcOperand, variable length)
			// Operand contains 4 BRC structures: Top, Left, Bottom, Right.
			// Word 97+: each BRC is 8 bytes (cv 4 + dptLineWidth 1 + brcType 1 + dptSpace/fShadow/fFrame/fReserved 2)
			//             → total 32 bytes for 4 BRCs
			// Older formats: each BRC10 is 2 bytes → total 8 bytes
			// Brc.dptSpace MUST be ignored when applied to character borders per MS-DOC spec.
			if len(op) >= 32 {
				// Word 97+ format: 4 * 8 bytes
				borders := &wordir.RunBorders{}
				borders.Top = parseBRC(op[0:8])
				borders.Left = parseBRC(op[8:16])
				borders.Bottom = parseBRC(op[16:24])
				borders.Right = parseBRC(op[24:32])
				if borders.Top != nil || borders.Left != nil || borders.Bottom != nil || borders.Right != nil {
					out.Borders = borders
				}
			} else if len(op) >= 8 {
				// Older BRC10 format: 4 * 2 bytes
				borders := &wordir.RunBorders{}
				borders.Top = parseBRC10(op[0:2])
				borders.Left = parseBRC10(op[2:4])
				borders.Bottom = parseBRC10(op[4:6])
				borders.Right = parseBRC10(op[6:8])
				if borders.Top != nil || borders.Left != nil || borders.Bottom != nil || borders.Right != nil {
					out.Borders = borders
				}
			}
		case 0xCA73: // sprmCRgLid0 — primary language array (variable length); uses first entry
			if len(op) >= 3 {
				payload := op
				if int(op[0]) == len(op)-1 {
					payload = op[1:]
				}
				if len(payload) >= 2 {
					lid := binary.LittleEndian.Uint16(payload)
					if tag := legacyLidToBCP47(lid); tag != "" {
						out.Lang = tag
					}
				}
			}
		case 0xCA74: // sprmCRgLid1 — East Asian language array (variable length); uses first entry when main lang absent
			if len(op) >= 3 && out.Lang == "" {
				payload := op
				if int(op[0]) == len(op)-1 {
					payload = op[1:]
				}
				if len(payload) >= 2 {
					lid := binary.LittleEndian.Uint16(payload)
					if tag := legacyLidToBCP47(lid); tag != "" {
						out.Lang = tag
					}
				}
			}
		case 0x0875: // sprmCFNoProof
			if len(op) >= 1 {
				v := chpxToggleBool(op[0])
				out.NoProof = &v
			}
		case 0xCA89: // sprmCPropRMark — character property revision mark (newer form of 0xCA57)
			payload := op
			if len(op) >= 1 && int(op[0]) == len(op)-1 {
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
		case 0x0818: // sprmCFSpecVanish — line-break acts as style separator; structural only
		case 0x2A33: // sprmCPlain — reset char props to paragraph style default
			resetChpxToPlain(&out)
		case 0x2A83: // sprmCWall — revision property preservation wall; structural
		case 0x2A86: // sprmCNeedFontFixup — font substitution hint; no renderable IR field
		case 0x4807: // sprmCIdslRMark — revision reason code for insertion; no IR field
		case 0x4867: // sprmCIdslRMarkDel — revision reason code for deletion; no IR field
		case 0x6815: // sprmCRsidProp — revision save ID for property change; no IR field
		case 0x6816: // sprmCRsidText — revision save ID for text insertion; no IR field
		case 0x6817: // sprmCRsidRMDel — revision save ID for tracked deletion; no IR field
		case 0x6865: // sprmCBrc80 — run border (Brc80, single border applied to all four sides)
			if len(op) >= 4 {
				if side := parseBRC80(op[0:4]); side != nil {
					out.Borders = &wordir.RunBorders{Top: side, Left: side, Bottom: side, Right: side}
				}
			}
		case 0xCA31: // sprmCIstdPermute — char style permute; applied at style-eval level
		case 0xCA85: // sprmCCnf — conditional table style char formatting; applied at table-build time
		case 0xCA76: // sprmCFitText — fit text to specified width (variable-length operand)
			// Operand format: [cb:1 byte length] + width in twips (2 or 4 bytes)
			if len(op) >= 3 {
				payload := op
				if int(op[0]) == len(op)-1 {
					payload = op[1:]
				}
				if len(payload) >= 2 {
					w := int(binary.LittleEndian.Uint16(payload))
					if w > 0 {
						out.FitTextWidth = &w
					}
				}
			}
		case 0xCA78: // sprmCFELayout — East Asian layout (variable-length operand)
			// Operand: [cb:1] + flags byte + combineCharCount (1 byte)
			if len(op) >= 2 {
				payload := op
				if int(op[0]) == len(op)-1 {
					payload = op[1:]
				}
				if len(payload) >= 1 {
					flags := payload[0]
					fe := &wordir.FELayout{}
					fe.CombineCharacters = (flags & 0x01) != 0
					fe.CombineBrackets = (flags & 0x02) != 0
					fe.Vertical = (flags & 0x04) != 0
					fe.VerticalCompressed = (flags & 0x08) != 0
					if len(payload) >= 2 {
						fe.CombineCharCount = int(payload[1])
					}
					out.FELayout = fe
				}
			}
		// Missing canonical CHP SPRMs — meaningful ones implemented, remainder mirror POI's empty break;.
		case 0x680E: // sprmCFcObj — object file offset (4 bytes)
			if len(op) >= 4 {
				out.PicLocation = int(binary.LittleEndian.Uint32(op))
			}
		case 0x0832: // sprmCDefault — partial reset to default char props (1-byte toggle)
			applyChpxDefaultReset(&out)
		case 0x483D: // sprmCFtcAscii — font table index for ASCII text (2 bytes)
			if len(op) >= 2 {
				ftc := int(binary.LittleEndian.Uint16(op))
				if ftc >= 0 && ftc < len(fontNames) && fontNames[ftc] != "" {
					out.FontASCII = fontNames[ftc]
					out.FontHAnsi = fontNames[ftc]
				}
			}
		case 0x4846: // sprmCHpsPos — vertical position in half-points (2 bytes)
			if len(op) >= 2 {
				pos := int(int16(binary.LittleEndian.Uint16(op)))
				out.Position = &pos
				switch {
				case pos > 0:
					out.VertAlignVal = "superscript"
				case pos < 0:
					out.VertAlignVal = "subscript"
				}
			}
		case 0x4849: // sprmCHps — font size in half-points (2 bytes)
			if len(op) >= 2 {
				hps := int(binary.LittleEndian.Uint16(op))
				if hps > 0 {
					out.FontSize = &hps
				}
			}
		case 0x484A: // sprmCHpsInc — font size increment in half-points (2 bytes)
			if len(op) >= 2 && out.FontSize != nil {
				delta := int(int16(binary.LittleEndian.Uint16(op)))
				hps := legacyAdjustRelativeFontSize(*out.FontSize, delta)
				out.FontSize = &hps
			}
		case 0x0808, 0x080A, 0x080B, 0x080D, 0x080F, 0x0810, 0x0812, 0x0813, 0x0814, 0x0815, 0x0817, 0x0819, 0x081A, 0x081B, 0x081C, 0x081D, 0x081E, 0x081F, 0x0820, 0x0821, 0x0823, 0x0824, 0x0825, 0x0826, 0x0827, 0x0828, 0x0829, 0x082A, 0x082B, 0x082C, 0x082D, 0x082E, 0x082F, 0x085B, 0x086A:
			// CHP SPRMs with no renderable IR field; preserved for coverage completeness.
		case 0x2879: // sprmCLbcCRJ — line-break type; layout-level, not run-format
		case 0x2A90: // sprmCFSdtVanish — SDT vanish for XML delimiters; structural only
		case 0x6887: // sprmCPbiIBullet — picture bullet CP; handled via drawing/object paths
		case 0x4888: // sprmCPbiGrf — picture bullet properties; handled via drawing/object paths
		case 0xC81A: // sprmCFMathPr — equation justification; paragraph-level, not run-format
		}
	}
	meta.Diagnostics = scanUnmappedSprms(grpprl, "CHP", unmappedChpxSprms)
	return out, meta
}

// parsePreWord97ChpxFKPPage decodes a single 512-byte ChpxFkp page from a
// pre-Word-97 (Word 6 / Word 95) document.  The FKP page structure is identical
// to Word 97+, but the grpprl bytes inside use the old 1-byte sprm opcode
// format (per MS-DOC "Word 6" sprm table / antiword prop6.c).
func parsePreWord97ChpxFKPPage(page []byte, fontNames []string) []chpxEntry {
	if len(page) < 512 {
		return nil
	}
	crun := int(page[511])
	if crun == 0 {
		return nil
	}
	if (crun+1)*4+crun > 511 {
		return nil
	}
	rgbxOff := (crun + 1) * 4
	result := make([]chpxEntry, 0, crun)
	for i := 0; i < crun; i++ {
		fc := binary.LittleEndian.Uint32(page[i*4 : i*4+4])
		bx := int(page[rgbxOff+i])
		var format wordir.RunFormat
		var sprms []byte
		if bx != 0 {
			chpxOff := bx * 2
			if chpxOff+1 < 512 {
				cb := int(page[chpxOff])
				if cb > 0 && chpxOff+1+cb <= 512 {
					sprms = page[chpxOff+1 : chpxOff+1+cb]
					format = parsePreWord97ChpxGrpprl(sprms, fontNames)
				}
			}
		}
		result = append(result, chpxEntry{FC: fc, Format: format, Sprms: sprms})
	}
	return result
}

// parsePreWord97ChpxGrpprl decodes a grpprl byte sequence using the old Word 6
// / Word 95 1-byte sprm opcode table (not the 2-byte Word97+ format).
// Reference: antiword prop6.c vGet6CHPInfo, MS-DOC "Word 6 SPRM" appendix.
// Operand sizes follow antiword's iGet6InfoLength:
//   - default: 1 byte operand (total 2 bytes per sprm)
//   - codes {80, 93, 96, 97, 99}: 2 byte operand (total 3 bytes)
//   - codes {95}: 3 byte operand (total 4 bytes — cHps + cHpsPos)
func parsePreWord97ChpxGrpprl(grpprl []byte, fontNames []string) wordir.RunFormat {
	var out wordir.RunFormat
	pos := 0
	for pos < len(grpprl) {
		opcode := grpprl[pos]
		pos++
		opSize := preWord97ChpxOperandSize(opcode)
		if pos+opSize > len(grpprl) {
			break
		}
		op := grpprl[pos : pos+opSize]
		pos += opSize

		switch opcode {
		case 85: // fBold
			if len(op) >= 1 {
				v := chpxToggleBool(op[0])
				out.Bold = &v
			}
		case 86: // fItalic
			if len(op) >= 1 {
				v := chpxToggleBool(op[0])
				out.Italic = &v
			}
		case 87: // fStrike
			if len(op) >= 1 {
				v := chpxToggleBool(op[0])
				out.Strike = &v
			}
		case 90: // fSmallCaps
			if len(op) >= 1 {
				v := chpxToggleBool(op[0])
				out.SmallCaps = &v
			}
		case 91: // fCaps
			if len(op) >= 1 {
				v := chpxToggleBool(op[0])
				out.Caps = &v
			}
		case 92: // fVanish
			if len(op) >= 1 {
				v := chpxToggleBool(op[0])
				out.Vanish = &v
			}
		case 93: // cFtc — 2-byte font index
			if len(op) >= 2 {
				ftc := int(binary.LittleEndian.Uint16(op))
				if ftc >= 0 && ftc < len(fontNames) && fontNames[ftc] != "" {
					out.FontASCII = fontNames[ftc]
					out.FontHAnsi = fontNames[ftc]
				}
			}
		case 94: // cKul — underline type
			if len(op) >= 1 && op[0] != 0 {
				out.Underline = chpxUnderlineStyle(op[0])
			}
		case 95: // cHps + cHpsPos — 1-byte font size in half-points + 1-byte position
			if len(op) >= 1 && op[0] != 0 {
				hps := int(op[0])
				out.FontSize = &hps
			}
			// op[1] = cHpsPos (superscript/subscript offset) — ignored for now
		case 98: // cIco — ICO color index
			if len(op) >= 1 {
				out.Color = icoToHex(op[0])
			}
		case 99: // cHps — 2-byte font size in half-points
			if len(op) >= 2 {
				hps := int(binary.LittleEndian.Uint16(op))
				if hps > 0 {
					out.FontSize = &hps
				}
			}
		case 104: // cIss — sub/superscript
			if len(op) >= 1 {
				switch op[0] & 0x07 {
				case 1:
					out.VertAlignVal = "superscript"
				case 2:
					out.VertAlignVal = "subscript"
				}
			}
		}
	}
	return out
}

// preWord97ChpxOperandSize returns the byte length of the operand following a
// pre-Word-97 (Word 6 / Word 95) 1-byte CHP sprm opcode.
// Based on antiword iGet6InfoLength (prop6.c).
func preWord97ChpxOperandSize(opcode byte) int {
	switch opcode {
	case 80, 93, 96, 97, 99: // 2-byte operand
		return 2
	case 95: // cHps + cHpsPos — 3 bytes
		return 3
	default:
		return 1
	}
}

// chpxOperandSize returns the byte length of the operand that follows a 2-byte
// SPRM code at grpprl[pos]. Returns -1 if the size cannot be determined.
//
// MS-DOC 2.2.5.1 defines the generic spra encoding:
//
//	spra 0/1 => 1 byte, spra 2/4/5 => 2 bytes, spra 3 => 4 bytes,
//	spra 6 => variable length, spra 7 => 3 bytes.
//
// spra=6 normally stores a one-byte cb followed by cb bytes. The same table
// explicitly names sprmTDefTable and sprmPChgTabs as exceptions, so they are
// handled here instead of being patched ad-hoc in individual grpprl parsers.
func chpxOperandSize(code uint16, grpprl []byte, pos int) int {
	spra := (code >> 13) & 0x07
	switch spra {
	case 0, 1:
		return 1
	case 2, 4, 5:
		return 2
	case 3:
		return 4
	case 6:
		return legacyVariableOperandSize(code, grpprl, pos)
	case 7:
		return 3
	}
	return -1
}

func legacyVariableOperandSize(code uint16, grpprl []byte, pos int) int {
	if pos >= len(grpprl) {
		return -1
	}

	switch code {
	case 0xC615: // sprmPChgTabs: PChgTabsPapxOperand, no leading cb byte.
		delCount := int(grpprl[pos])
		addCountPos := pos + 1 + delCount*2
		if addCountPos >= len(grpprl) {
			return -1
		}
		addCount := int(grpprl[addCountPos])
		return 1 + delCount*2 + 1 + addCount*2 + addCount

	case 0xD608: // sprmTDefTable: TDefTableOperand starts with 2-byte cb.
		if pos+2 > len(grpprl) {
			return -1
		}
		cb := int(binary.LittleEndian.Uint16(grpprl[pos : pos+2]))
		// MS-DOC 2.9.321: cb is the bytes used by the remainder of the
		// structure, incremented by 1. Therefore total operand bytes are cb+1:
		//   2-byte cb field + (cb - 1) bytes remainder.
		return cb + 1
	}

	// Generic spra=6: first byte is the size of the rest of operand.
	return 1 + int(grpprl[pos])
}

// chpxToggleBool converts a toggle-byte operand to a plain bool.
// 0x00 = off, 0x01 = on, 0x80 = toggle from style (treated as "on" here since
// we have no inherited value at this parse level).
func chpxToggleBool(b byte) bool {
	return b != 0x00
}

func legacyAdjustRelativeFontSize(base, delta int) int {
	hps := base + delta
	if hps < 2 {
		return 2
	}
	return hps
}

func parseLegacyRunShading(op []byte) string {
	payload := op
	if len(payload) >= 11 && int(payload[0]) == len(payload)-1 {
		payload = payload[1:]
	}
	if len(payload) < 10 {
		return ""
	}
	ipat := binary.LittleEndian.Uint16(payload[8:10])
	if ipat == 0 {
		return ""
	}
	var cr uint32
	if ipat == 1 {
		cr = binary.LittleEndian.Uint32(payload[0:4])
	} else {
		cr = binary.LittleEndian.Uint32(payload[4:8])
	}
	if cr>>24 == 0xFF {
		return ""
	}
	return fmt.Sprintf("%02X%02X%02X", cr&0xFF, (cr>>8)&0xFF, (cr>>16)&0xFF)
}

// chpxEmphasisMark maps the kumimoji/emphasisMark byte from sprmCKcd onto the
// OOXML w:em vocabulary. MS-DOC §2.6.1 sprmCKcd encodes the East Asian
// emphasis mark style: 0=none, 1=dot, 2=comma, 3=circle, 4=underDot.
func chpxEmphasisMark(b byte) string {
	switch b {
	case 0:
		return "none"
	case 1:
		return "dot"
	case 2:
		return "comma"
	case 3:
		return "circle"
	case 4:
		return "underDot"
	}
	return ""
}

// chpxUnderlineStyle converts a kul (underline-type) byte to an OOXML-style
// underline name accepted by wordir.RunFormat.Underline.
func chpxUnderlineStyle(kul byte) string {
	switch kul {
	case 1:
		return "single"
	case 2:
		return "words"
	case 3:
		return "double"
	case 4:
		return "dotted"
	case 6:
		return "thick"
	case 7:
		return "dash"
	case 9:
		return "dotDash"
	case 10:
		return "dotDotDash"
	case 11:
		return "wave"
	default:
		return "single"
	}
}

// icoToHex converts a Word 97 ICO color-index byte to an uppercase six-digit
// hex RGB string (no leading "#").  Returns "" for ico=0 (auto) or unknown.
func icoToHex(ico byte) string {
	switch ico {
	case 1:
		return "000000"
	case 2:
		return "0000FF"
	case 3:
		return "00FFFF"
	case 4:
		return "00FF00"
	case 5:
		return "FF00FF"
	case 6:
		return "FF0000"
	case 7:
		return "FFFF00"
	case 8:
		return "FFFFFF"
	case 9:
		return "000080"
	case 10:
		return "008080"
	case 11:
		return "008000"
	case 12:
		return "800080"
	case 13:
		return "800000"
	case 14:
		return "808000"
	case 15:
		return "808080"
	case 16:
		return "C0C0C0"
	default:
		return ""
	}
}

// icoHighlightName maps a Word 97 icoHighlight byte (MS-DOC 2.9.68) to the
// WordprocessingML highlight name consumed by RunFormat.Highlight and the HTML
// renderer's highlightColor function.  Value 0 means "no highlight" and returns
// the empty string; all other unmapped values return "".
func icoHighlightName(ico byte) string {
	switch ico {
	case 1:
		return "black"
	case 2:
		return "blue"
	case 3:
		return "cyan"
	case 4:
		return "green"
	case 5:
		return "magenta"
	case 6:
		return "red"
	case 7:
		return "yellow"
	case 8:
		return "white"
	case 9:
		return "darkBlue"
	case 10:
		return "darkCyan"
	case 11:
		return "darkGreen"
	case 12:
		return "darkMagenta"
	case 13:
		return "darkRed"
	case 14:
		return "darkYellow"
	case 15:
		return "darkGray"
	case 16:
		return "lightGray"
	default:
		return ""
	}
}

// legacyLidToBCP47 maps common Word binary language IDs (LID/LCID) to BCP 47
// tags suitable for RunFormat.Lang. Unknown or special-purpose IDs return "" so
// callers preserve "unset" semantics instead of guessing.
func legacyLidToBCP47(lid uint16) string {
	switch lid {
	case 0x0404:
		return "zh-TW"
	case 0x0407:
		return "de-DE"
	case 0x0409:
		return "en-US"
	case 0x040A:
		return "es-ES"
	case 0x040C:
		return "fr-FR"
	case 0x0410:
		return "it-IT"
	case 0x0411:
		return "ja-JP"
	case 0x0412:
		return "ko-KR"
	case 0x0413:
		return "nl-NL"
	case 0x0415:
		return "pl-PL"
	case 0x0416:
		return "pt-BR"
	case 0x0419:
		return "ru-RU"
	case 0x0422:
		return "uk-UA"
	case 0x0804:
		return "zh-CN"
	case 0x0809:
		return "en-GB"
	case 0x080A:
		return "es-MX"
	case 0x0816:
		return "pt-PT"
	default:
		return ""
	}
}

// resetChpxToPlain clears all direct character formatting from out so that style
// defaults will show through. It mirrors POI's sprmCPlain (case 0x33) behavior
// which copies the parent style CHP and preserves only fSpec.
func resetChpxToPlain(out *wordir.RunFormat) {
	out.Bold = nil
	out.Italic = nil
	out.Caps = nil
	out.SmallCaps = nil
	out.Strike = nil
	out.DoubleStrike = nil
	out.Underline = ""
	out.UnderlineColor = ""
	out.Color = ""
	out.Background = ""
	out.FontSize = nil
	out.FontASCII = ""
	out.FontHAnsi = ""
	out.FontEastAsia = ""
	out.FontCS = ""
	out.Highlight = ""
	out.VertAlignVal = ""
	out.Vanish = nil
	out.NoProof = nil
	out.WebHidden = nil
	out.ComplexScript = nil
	out.Lang = ""
	out.CharacterSpacing = nil
	out.FitTextWidth = nil
	out.Kerning = nil
	out.CharScale = nil
	out.RTL = nil
	out.Outline = nil
	out.Shadow = nil
	out.Emboss = nil
	out.Imprint = nil
	out.EmphasisMark = ""
	out.FELayout = nil
	out.Borders = nil
}

// applyChpxDefaultReset performs the partial reset used by sprmCDefault (POI case 0x32).
// It clears toggle properties, underline, and colour to their defaults.
func applyChpxDefaultReset(out *wordir.RunFormat) {
	out.Bold = nil
	out.Italic = nil
	out.Outline = nil
	out.Shadow = nil
	out.Strike = nil
	out.SmallCaps = nil
	out.Caps = nil
	out.Vanish = nil
	out.Underline = ""
	out.Color = ""
}

// parseBRC10 parses a legacy BRC10 (2-byte) border structure into a wordir.BorderSide.
// Format: brcType (1 byte) + flags byte. Used only for pre-Word-97 character borders.
func parseBRC10(data []byte) *wordir.BorderSide {
	if len(data) < 2 {
		return nil
	}
	brcType := data[0]
	if brcType == 0 || brcType == 255 {
		return nil
	}
	val := "single"
	switch brcType {
	case 2:
		val = "thick"
	case 3:
		val = "double"
	case 5:
		val = "hair"
	}
	return &wordir.BorderSide{
		Val:    val,
		SizePt: 0.5, // default 1/2 pt
		Color:  "",
	}
}

// findChpxEntry returns the index of the last entry in sorted entries whose FC
// is ≤ fc, or -1 if all entries have FC > fc.
func findChpxEntry(entries []chpxEntry, fc uint32) int {
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

// legacyTextAnimationFromSfx maps a legacy sprmCSfxText value to the OOXML
// w:effect vocabulary. This advances unified IR convergence between legacy and
// OOXML parser paths.
func legacyTextAnimationFromSfx(v byte) string {
	switch v {
	case 1:
		return "lights"
	case 2:
		return "blinkBackground"
	case 3:
		return "sparkle"
	case 4:
		return "antsBlack"
	case 5:
		return "antsRed"
	case 6:
		return "shimmer"
	default:
		return "none"
	}
}

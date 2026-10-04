package docformat

import (
	"encoding/binary"
	"fmt"

	"office/pkg/diag"
	wordir "office/pkg/word/ir"
)

// DecodeCHPX decodes Word97+ character-formatting SPRMs into the canonical
// sequential DOC formatting model.
func DecodeCHPX(grpprl []byte, fontNames []string) CHPXEntry {
	var out CHPXEntry
	for pos := 0; pos+2 <= len(grpprl); {
		code := binary.LittleEndian.Uint16(grpprl[pos : pos+2])
		pos += 2
		size := OperandSize(code, grpprl, pos)
		if size < 0 || pos+size > len(grpprl) {
			break
		}
		op := grpprl[pos : pos+size]
		pos += size

		switch code {
		case 0x0801: // sprmCFRMarkIns
			if len(op) > 0 {
				if toggle(op[0]) {
					out.Revision = "inserted"
				} else if out.Revision == "inserted" {
					out.Revision = ""
				}
			}
		case 0x0800: // sprmCFRMarkDel
			if len(op) > 0 {
				if toggle(op[0]) {
					out.Revision = "deleted"
				} else if out.Revision == "deleted" {
					out.Revision = ""
				}
			}
		case 0x4804, 0x4863: // sprmCIbstRMark / sprmCIbstRMarkDel
			if len(op) >= 2 {
				out.IbstRMark = binary.LittleEndian.Uint16(op)
				out.HasIbstRMark = true
			}
		case 0x6805, 0x6864: // sprmCDttmRMark / sprmCDttmRMarkDel
			if len(op) >= 4 {
				out.DttmRMark = binary.LittleEndian.Uint32(op)
				out.HasDttmRMark = true
			}
		case 0x0802: // sprmCFFldVanish
			// Field-instruction visibility is structural metadata. It must not be
			// projected as ordinary run vanish; field materialization suppresses
			// instruction text after interpreting the field characters.
		case 0x0835:
			if len(op) > 0 {
				v := toggle(op[0])
				out.Format.Bold = &v
			}
		case 0x0836:
			if len(op) > 0 {
				v := toggle(op[0])
				out.Format.Italic = &v
			}
		case 0x0837:
			if len(op) > 0 {
				v := toggle(op[0])
				out.Format.Strike = &v
			}
		case 0x0838:
			if len(op) > 0 {
				v := toggle(op[0])
				out.Format.Outline = &v
			}
		case 0x0839:
			if len(op) > 0 {
				v := toggle(op[0])
				out.Format.Shadow = &v
			}
		case 0x083A:
			if len(op) > 0 {
				v := toggle(op[0])
				out.Format.SmallCaps = &v
			}
		case 0x083B:
			if len(op) > 0 {
				v := toggle(op[0])
				out.Format.Caps = &v
			}
		case 0x083C:
			if len(op) > 0 {
				v := toggle(op[0])
				out.Format.Vanish = &v
			}
		case 0x2A3E:
			if len(op) > 0 {
				out.Format.Underline = underlineStyle(op[0])
			}
		case 0x2A42:
			if len(op) > 0 {
				out.Format.Color = icoToHex(op[0])
			}
		case 0x2A0C:
			if len(op) > 0 {
				out.Format.Highlight = highlightName(op[0])
			}
		case 0x4A43:
			if len(op) >= 2 {
				v := int(binary.LittleEndian.Uint16(op))
				out.Format.FontSize = &v
			}
		case 0x4A61:
			if len(op) >= 2 {
				v := int(binary.LittleEndian.Uint16(op))
				out.Format.FontSizeCS = &v
			}
		case 0x2A48:
			if len(op) > 0 {
				if op[0] == 1 {
					out.Format.VertAlignVal = "superscript"
				} else if op[0] == 2 {
					out.Format.VertAlignVal = "subscript"
				}
			}
		case 0x4845:
			if len(op) >= 2 {
				v := int(int16(binary.LittleEndian.Uint16(op)))
				out.Format.Position = &v
			}
		case 0x4A4F:
			if len(op) >= 2 {
				setFont(&out.Format, fontNames, int(binary.LittleEndian.Uint16(op)), 0)
			}
		case 0x4A50:
			if len(op) >= 2 {
				setFont(&out.Format, fontNames, int(binary.LittleEndian.Uint16(op)), 1)
			}
		case 0x4A51:
			if len(op) >= 2 {
				setFont(&out.Format, fontNames, int(binary.LittleEndian.Uint16(op)), 2)
			}
		case 0x4A30: // sprmCIstd
			if len(op) >= 2 {
				v := int(binary.LittleEndian.Uint16(op))
				out.Format.BinaryStyleIndex = &v
			}
		case 0x8840:
			if len(op) >= 2 {
				v := int(int16(binary.LittleEndian.Uint16(op)))
				out.Format.CharacterSpacing = &v
			}
		case 0x4A41:
			if len(op) >= 2 {
				out.Format.Lang = lidToBCP47(binary.LittleEndian.Uint16(op))
			}
		case 0x085A:
			if len(op) > 0 {
				v := toggle(op[0])
				out.Format.RTL = &v
			}
		case 0xCA71:
			if shade := runShading(op); shade != "" {
				out.Format.Background = shade
			}
		case 0x4852:
			if len(op) >= 2 {
				v := int(binary.LittleEndian.Uint16(op))
				out.Format.CharScale = &v
			}
		case 0xCA76:
			p := variablePayload(op)
			if len(p) >= 2 {
				v := int(binary.LittleEndian.Uint16(p))
				out.Format.FitTextWidth = &v
			}
		case 0x484B:
			if len(op) >= 2 {
				v := int(binary.LittleEndian.Uint16(op))
				out.Format.Kerning = &v
			}
		case 0x6870:
			if len(op) >= 4 {
				out.Format.Color = colorRef(binary.LittleEndian.Uint32(op))
			}
		case 0x6877:
			if len(op) >= 4 {
				out.Format.UnderlineColor = colorRef(binary.LittleEndian.Uint32(op))
			}
		case 0x4866:
			if shade := shd80(op); shade != "" {
				out.Format.Background = shade
			}
		case 0x085C:
			if len(op) > 0 {
				v := toggle(op[0])
				out.Format.BoldCS = &v
			}
		case 0x085D:
			if len(op) > 0 {
				v := toggle(op[0])
				out.Format.ItalicCS = &v
			}
		case 0x0818:
			if len(op) > 0 {
				v := toggle(op[0])
				out.Format.SpecVanish = &v
			}
		case 0x2859:
			if len(op) > 0 {
				out.Format.TextAnimation = textAnimation(op[0])
			}
		case 0xCA78:
			p := variablePayload(op)
			if len(p) > 0 {
				out.Format.FELayout = &wordir.FELayout{CombineCharacters: p[0]&0x01 != 0, CombineBrackets: p[0]&0x02 != 0, Vertical: p[0]&0x04 != 0, VerticalCompressed: p[0]&0x08 != 0}
				if len(p) > 1 {
					out.Format.FELayout.CombineCharCount = int(p[1])
				}
			}
		case 0x0855:
			if len(op) > 0 {
				out.Spec = toggle(op[0])
			}
		case 0x0806:
			if len(op) > 0 {
				out.Data = toggle(op[0])
			}
		case 0x0856:
			if len(op) > 0 {
				out.Obj = toggle(op[0])
			}
		case 0xCA57:
			out.PropertyRevision = parseRevisionMark(op)
		case 0xCA62:
			out.DisplayFieldRevision = parseRevisionMark(op)
		}
	}
	out.Diagnostics = scanUnmappedCHPX(grpprl)
	return out
}

func parseRevisionMark(op []byte) *PendingRevisionMark {
	payload := variablePayload(op)
	if len(payload) < 7 {
		return nil
	}
	return &PendingRevisionMark{Present: payload[0] != 0, Ibst: binary.LittleEndian.Uint16(payload[1:3]), HasIbst: true, Dttm: binary.LittleEndian.Uint32(payload[3:7]), HasDttm: true}
}

func variablePayload(op []byte) []byte {
	// Variable-length SPRM operands include an explicit byte-count prefix. Only
	// strip it when it exactly describes the bytes that follow. The previous
	// broad `op[0] >= 7` fallback corrupted fixed/raw payloads such as SHD whose
	// first color byte can legitimately be 0xFF.
	if len(op) >= 1 && int(op[0]) == len(op)-1 {
		return op[1:]
	}
	return op
}

func setFont(out *wordir.RunFormat, names []string, index, kind int) {
	if index < 0 || index >= len(names) || names[index] == "" {
		return
	}
	switch kind {
	case 0:
		out.FontASCII, out.FontHAnsi = names[index], names[index]
	case 1:
		out.FontEastAsia = names[index]
	case 2:
		out.FontCS = names[index]
	}
}

func toggle(b byte) bool { return b != 0 }
func adjustFontSize(base, delta int) int {
	v := base + delta
	if v < 2 {
		return 2
	}
	return v
}

func emphasisMark(b byte) string {
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
func underlineStyle(b byte) string {
	switch b {
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
	}
	return "single"
}
func icoToHex(b byte) string {
	switch b {
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
	}
	return ""
}
func highlightName(b byte) string {
	switch b {
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
	}
	return ""
}
func lidToBCP47(l uint16) string {
	switch l {
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
	}
	return ""
}
func textAnimation(v byte) string {
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
	}
	return "none"
}

func runShading(op []byte) string {
	p := op
	if len(p) >= 11 && int(p[0]) == len(p)-1 {
		p = p[1:]
	}
	if len(p) < 10 {
		return ""
	}
	ipat := binary.LittleEndian.Uint16(p[8:10])
	if ipat == 0 {
		return ""
	}
	var cr uint32
	if ipat == 1 {
		cr = binary.LittleEndian.Uint32(p[:4])
	} else {
		cr = binary.LittleEndian.Uint32(p[4:8])
	}
	if cr>>24 == 0xFF {
		return ""
	}
	return colorRef(cr)
}
func shd80(op []byte) string {
	if len(op) < 2 {
		return ""
	}
	raw := binary.LittleEndian.Uint16(op)
	fore := byte(raw & 0x1F)
	back := byte((raw >> 5) & 0x1F)
	ipat := (raw >> 10) & 0x3F
	if ipat == 0 || ipat == 0x3F {
		return ""
	}
	if ipat == 1 {
		return icoToHex(fore)
	}
	return icoToHex(back)
}
func colorRef(cr uint32) string {
	if cr>>24 == 0xFF {
		return ""
	}
	return fmt.Sprintf("%02X%02X%02X", cr&0xFF, (cr>>8)&0xFF, (cr>>16)&0xFF)
}

func parseBRC10(data []byte) *wordir.BorderSide {
	if len(data) < 2 {
		return nil
	}
	t := data[0]
	if t == 0 || t == 255 {
		return nil
	}
	v := "single"
	switch t {
	case 2:
		v = "thick"
	case 3:
		v = "double"
	case 5:
		v = "hair"
	}
	return &wordir.BorderSide{Val: v, SizePt: 0.5}
}
func parseBRC80(op []byte) *wordir.BorderSide {
	if len(op) < 4 {
		return nil
	}
	if binary.LittleEndian.Uint32(op) == 0xFFFFFFFF {
		return &wordir.BorderSide{Val: "none"}
	}
	t := op[1]
	if t == 0 || t == 0xFF {
		return nil
	}
	return &wordir.BorderSide{Val: borderType(t), SizePt: float64(op[0]) / 8.0, Color: icoToHex(op[2]), SpacePt: float64(op[3])}
}
func parseBRC(op []byte) *wordir.BorderSide {
	if len(op) == 9 && op[0] == 8 {
		op = op[1:]
	}
	if len(op) < 8 {
		return nil
	}
	t := op[5]
	if t == 0 || t == 0xFF {
		return nil
	}
	width := float64(op[4]) / 8.0
	if t >= 0x40 {
		width = float64(op[4])
	}
	if width > 0 && width < 0.25 {
		width = 0.25
	}
	flags := binary.LittleEndian.Uint16(op[6:8])
	return &wordir.BorderSide{Val: borderType(t), SizePt: width, Color: colorRef(binary.LittleEndian.Uint32(op[:4])), SpacePt: float64(flags & 0x1F)}
}
func borderType(t byte) string {
	switch t {
	case 1:
		return "single"
	case 2:
		return "thick"
	case 3:
		return "double"
	case 4, 5:
		return "dotted"
	case 6:
		return "dashed"
	}
	return "single"
}

func resetPlain(out *wordir.RunFormat) {
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
func defaultReset(out *wordir.RunFormat) {
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

var unmappedCHPX = map[uint16]string{
	0x0806: "sprmCFData", 0x0808: "sprmCFBoldBi", 0x080A: "sprmCFItalicBi", 0x080B: "sprmCFUseFELayout", 0x080D: "sprmCFNoProof", 0x080F: "sprmCFitText", 0x0810: "sprmCFelidsrMark", 0x0812: "sprmCFBoldOther", 0x0813: "sprmCFItalicOther", 0x0814: "sprmCFUseHL2", 0x0815: "sprmCFBiDi", 0x0817: "sprmCFBoldOther", 0x0819: "sprmCFItalicOther", 0x081A: "sprmCFUseHL2", 0x081B: "sprmCFBiDi", 0x081C: "sprmCFDntUL", 0x081D: "sprmCFDntBdr", 0x081E: "sprmCFUser", 0x081F: "sprmCFUser", 0x0820: "sprmCFUser", 0x0821: "sprmCFUser", 0x0823: "sprmCFUser", 0x0824: "sprmCFUser", 0x0825: "sprmCFUser", 0x0826: "sprmCFUser", 0x0827: "sprmCFUser", 0x0828: "sprmCFUser", 0x0829: "sprmCFUser", 0x082A: "sprmCFUser", 0x082B: "sprmCFUser", 0x082C: "sprmCFUser", 0x082D: "sprmCFUser", 0x082E: "sprmCFUser", 0x082F: "sprmCFUser", 0x085B: "sprmCFUser", 0x086A: "sprmCFUser", 0x484E: "sprmCHresi", 0xCA62: "sprmCDispFldRMark", 0x0867: "sprmCObsolete", 0x0868: "sprmCFUsePgsuSettings", 0x0869: "sprmCObsolete", 0x086B: "sprmCObsolete", 0x086C: "sprmCObsolete", 0x286F: "sprmCIdctHint", 0x0856: "sprmCFObj", 0x2859: "sprmCSfxText", 0x6A09: "sprmCSymbol", 0x0818: "sprmCFSpecVanish", 0x2A83: "sprmCWall", 0x2A86: "sprmCNeedFontFixup", 0x4807: "sprmCIdslRMark", 0x4867: "sprmCIdslRMarkDel", 0x6815: "sprmCRsidProp", 0x6816: "sprmCRsidText", 0x6817: "sprmCRsidRMDel", 0xCA31: "sprmCIstdPermute", 0xCA85: "sprmCCnf", 0x2879: "sprmCLbcCRJ", 0x2A90: "sprmCFSdtVanish", 0x6887: "sprmCPbiIBullet", 0x4888: "sprmCPbiGrf", 0xC81A: "sprmCFMathPr",
}

func scanUnmappedCHPX(grpprl []byte) diag.List {
	var list diag.List
	pos := 0
	for pos+2 <= len(grpprl) {
		code := binary.LittleEndian.Uint16(grpprl[pos : pos+2])
		pos += 2
		size := OperandSize(code, grpprl, pos)
		if size < 0 || pos+size > len(grpprl) {
			break
		}
		if name, ok := unmappedCHPX[code]; ok {
			list.Add(diag.Diagnostic{Severity: diag.SeverityInfo, Code: "legacy.sprm.unmapped", Message: fmt.Sprintf("SPRM %s (0x%04X) is recognized but has no semantic mapping", name, code)})
		}
		pos += size
	}
	return list
}

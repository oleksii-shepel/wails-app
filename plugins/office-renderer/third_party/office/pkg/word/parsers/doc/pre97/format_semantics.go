package pre97

import (
	"encoding/binary"
	"sort"

	wordir "office/pkg/word/ir"
)

// PAPXMetadata carries old Word 6/95 paragraph-formatting semantics that are
// not represented directly on wordir.ParagraphFormat.
type PAPXMetadata struct {
	ListStart *int
	InTable   bool
	RowMark   bool
}

// DecodeCHPXGrpprl interprets the old Word 6/95 one-byte CHP sprm stream.
func DecodeCHPXGrpprl(grpprl []byte, fontNames []string) wordir.RunFormat {
	var out wordir.RunFormat
	for pos := 0; pos < len(grpprl); {
		opcode := grpprl[pos]
		pos++
		size := chpxOperandSize(opcode)
		if pos+size > len(grpprl) {
			break
		}
		op := grpprl[pos : pos+size]
		pos += size
		switch opcode {
		case 85:
			v := op[0] != 0
			out.Bold = &v
		case 86:
			v := op[0] != 0
			out.Italic = &v
		case 87:
			v := op[0] != 0
			out.Strike = &v
		case 90:
			v := op[0] != 0
			out.SmallCaps = &v
		case 91:
			v := op[0] != 0
			out.Caps = &v
		case 92:
			v := op[0] != 0
			out.Vanish = &v
		case 93:
			if len(op) >= 2 {
				idx := int(binary.LittleEndian.Uint16(op))
				if idx >= 0 && idx < len(fontNames) && fontNames[idx] != "" {
					out.FontASCII, out.FontHAnsi = fontNames[idx], fontNames[idx]
				}
			}
		case 94:
			if op[0] != 0 {
				out.Underline = underlineStyle(op[0])
			}
		case 95:
			if op[0] != 0 {
				v := int(op[0])
				out.FontSize = &v
			}
		case 98:
			out.Color = icoToHex(op[0])
		case 99:
			if len(op) >= 2 {
				v := int(binary.LittleEndian.Uint16(op))
				if v > 0 {
					out.FontSize = &v
				}
			}
		case 104:
			switch op[0] & 0x07 {
			case 1:
				out.VertAlignVal = "superscript"
			case 2:
				out.VertAlignVal = "subscript"
			}
		}
	}
	return out
}

func chpxOperandSize(opcode byte) int {
	switch opcode {
	case 80, 93, 96, 97, 99:
		return 2
	case 95:
		return 3
	default:
		return 1
	}
}

// DecodePAPXGrpprl interprets the old Word 6/95 one-byte PAP sprm stream.
func DecodePAPXGrpprl(grpprl []byte) (wordir.ParagraphFormat, PAPXMetadata) {
	var out wordir.ParagraphFormat
	var meta PAPXMetadata
	for pos := 0; pos < len(grpprl); {
		opcode := grpprl[pos]
		pos++
		size := papxOperandSize(opcode, grpprl, pos)
		if size < 0 || pos+size > len(grpprl) {
			break
		}
		op := grpprl[pos : pos+size]
		pos += size
		switch opcode {
		case 2:
			if len(op) >= 2 {
				v := int(binary.LittleEndian.Uint16(op))
				out.BinaryStyleIndex = &v
			}
		case 5:
			if len(op) > 0 {
				switch op[0] {
				case 1:
					out.Alignment = "center"
				case 2:
					out.Alignment = "right"
				case 3:
					out.Alignment = "both"
				}
			}
		case 12:
			if len(op) >= 2 {
				v := int(op[1])
				out.BinaryListFormat = &v
			}
			if len(op) >= 13 {
				v := int(binary.LittleEndian.Uint16(op[11:13]))
				if v > 0 {
					meta.ListStart = &v
				}
			}
		case 13:
			if len(op) > 0 {
				v := int(op[0])
				out.BinaryListLevel = &v
			}
		case 16:
			v := int(int16(binary.LittleEndian.Uint16(op)))
			out.IndentRight = &v
		case 17:
			v := int(int16(binary.LittleEndian.Uint16(op)))
			out.IndentLeft = &v
		case 18:
			delta := int(int16(binary.LittleEndian.Uint16(op)))
			if out.IndentLeft == nil {
				if delta > 0 {
					out.IndentLeft = &delta
				}
			} else {
				v := *out.IndentLeft + delta
				if v < 0 {
					v = 0
				}
				out.IndentLeft = &v
			}
		case 19:
			v := int(int16(binary.LittleEndian.Uint16(op)))
			if v < 0 {
				h := -v
				out.IndentHanging = &h
			} else if v > 0 {
				out.IndentFirstLine = &v
			}
		case 21:
			v := int(binary.LittleEndian.Uint16(op))
			if v > 0 {
				out.SpacingBefore = &v
			}
		case 22:
			v := int(binary.LittleEndian.Uint16(op))
			if v > 0 {
				out.SpacingAfter = &v
			}
		case 23:
			if len(op) > 1 {
				width := 2
				payload := op[1:]
				if op[0] == 255 {
					width = 4
				}
				out.TabStops = applyTabChange(out.TabStops, payload, width)
			}
		case 24:
			meta.InTable = true
			if len(op) > 0 {
				out.InTable = op[0] != 0
			}
		case 25:
			meta.RowMark = len(op) > 0 && op[0] != 0
		}
	}
	return out, meta
}

func papxOperandSize(opcode byte, grpprl []byte, pos int) int {
	switch {
	case opcode == 23:
		if pos >= len(grpprl) { return -1 }
		n := int(grpprl[pos])
		if n == 255 {
			if pos+2 > len(grpprl) { return -1 }
			del := int(grpprl[pos+1])
			if pos+2+del*4 >= len(grpprl) { return -1 }
			add := int(grpprl[pos+2+del*4])
			n = 2 + del*4 + add*3
		}
		return 1 + n
	case opcode == 3 || opcode == 12 || opcode == 15 || opcode == 81 || opcode == 103 || opcode == 108 || opcode == 188 || opcode == 190 || opcode == 191:
		if pos >= len(grpprl) { return -1 }
		return 1 + int(grpprl[pos])
	case opcode == 2 || opcode == 16 || opcode == 17 || opcode == 18 || opcode == 19 || opcode == 21 || opcode == 22 || (opcode >= 26 && opcode <= 36) || (opcode >= 38 && opcode <= 49) || opcode == 69 || opcode == 72 || opcode == 80 || opcode == 93 || opcode == 96 || opcode == 97 || opcode == 99 || opcode == 101 || opcode == 105 || opcode == 106 || opcode == 107 || opcode == 109 || opcode == 110 || (opcode >= 121 && opcode <= 124) || opcode == 140 || opcode == 141 || opcode == 144 || opcode == 145 || opcode == 148 || opcode == 149 || (opcode >= 154 && opcode <= 157) || (opcode >= 160 && opcode <= 171) || opcode == 182 || opcode == 183 || opcode == 184 || opcode == 189 || opcode == 195 || opcode == 197 || opcode == 198:
		return 2
	case opcode == 20 || opcode == 70 || opcode == 74 || opcode == 192 || opcode == 194 || opcode == 196 || opcode == 200:
		return 4
	case opcode == 68 || opcode == 193 || opcode == 199:
		return 5
	case opcode == 73 || opcode == 95 || opcode == 136 || opcode == 137:
		return 3
	case opcode == 120 || opcode == 187:
		return 12
	default:
		return 1
	}
}

func applyTabChange(base []wordir.TabStop, payload []byte, deleteEntryWidth int) []wordir.TabStop {
	if len(payload) < 2 || deleteEntryWidth < 2 {
		return append([]wordir.TabStop(nil), base...)
	}
	delCount, offset := int(payload[0]), 1
	deleteBytes := delCount * deleteEntryWidth
	if offset+deleteBytes >= len(payload) {
		return append([]wordir.TabStop(nil), base...)
	}
	m := make(map[int]wordir.TabStop, len(base))
	for _, tab := range base { m[tab.Pos] = tab }
	for i := 0; i < delCount; i++ {
		entry := payload[offset+i*deleteEntryWidth:]
		if len(entry) < 2 { return append([]wordir.TabStop(nil), base...) }
		delete(m, int(int16(binary.LittleEndian.Uint16(entry[:2]))))
	}
	offset += deleteBytes
	addCount := int(payload[offset]); offset++
	if offset+addCount*2+addCount > len(payload) { return append([]wordir.TabStop(nil), base...) }
	descOffset := offset + addCount*2
	for i := 0; i < addCount; i++ {
		p := int(int16(binary.LittleEndian.Uint16(payload[offset+i*2 : offset+i*2+2])))
		if p <= 0 { continue }
		d := payload[descOffset+i]
		m[p] = wordir.TabStop{Pos: p, Algn: tabAlignment(d), Leader: tabLeader(d)}
	}
	positions := make([]int, 0, len(m)); for p := range m { positions = append(positions, p) }; sort.Ints(positions)
	out := make([]wordir.TabStop, 0, len(positions)); for _, p := range positions { out = append(out, m[p]) }
	return out
}

func tabAlignment(d byte) string { switch d & 7 { case 1: return "center"; case 2: return "right"; case 3: return "decimal"; case 4: return "bar" }; return "left" }
func tabLeader(d byte) string { switch (d >> 3) & 7 { case 1: return "dot"; case 2: return "hyphen"; case 3: return "underscore"; case 5: return "middleDot" }; return "" }
func underlineStyle(v byte) string { switch v { case 1: return "single"; case 2: return "words"; case 3: return "double"; case 4: return "dotted"; case 6: return "thick"; case 7: return "dash"; case 9: return "dotDash"; case 10: return "dotDotDash"; case 11: return "wave" }; return "single" }
func icoToHex(v byte) string { switch v { case 1: return "000000"; case 2: return "0000FF"; case 3: return "00FFFF"; case 4: return "00FF00"; case 5: return "FF00FF"; case 6: return "FF0000"; case 7: return "FFFF00"; case 8: return "FFFFFF"; case 9: return "000080"; case 10: return "008080"; case 11: return "008000"; case 12: return "800080"; case 13: return "800000"; case 14: return "808000"; case 15: return "808080"; case 16: return "C0C0C0" }; return "" }

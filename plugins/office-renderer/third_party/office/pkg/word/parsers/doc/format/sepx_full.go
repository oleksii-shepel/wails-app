package docformat

import (
	"encoding/binary"
	"fmt"

	"office/pkg/diag"
	wordir "office/pkg/word/ir"
)

// DecodeSEPX decodes the complete Word97 section-property surface currently
// represented by the Word IR. DecodeSEPXCore remains the minimal layout path;
// this wrapper adds long-tail fidelity that previously lived in the legacy
// compatibility adapter (numbering format names, text flow, page borders and
// recognized-but-unmapped diagnostics).
func DecodeSEPX(grpprl []byte) (wordir.Section, *PendingRevisionMark) {
	sec, revision := DecodeSEPXCore(grpprl)
	var pageBorderTop, pageBorderLeft, pageBorderBottom, pageBorderRight *wordir.BorderSide
	var pageBorderDisplay, pageBorderOffsetFrom, pageBorderZOrder string
	hasPageBorders := false
	var diagnostics diag.List

	for pos := 0; pos+2 <= len(grpprl); {
		code := binary.LittleEndian.Uint16(grpprl[pos : pos+2])
		pos += 2
		size := OperandSize(code, grpprl, pos)
		if size <= 0 || pos+size > len(grpprl) {
			break
		}
		op := grpprl[pos : pos+size]
		pos += size

		switch code {
		case 0x300E: // sprmSNfcPgn
			if len(op) >= 1 {
				sec.PageNumberFormat = sectionNFCFormat(op[0])
			}
		case 0x5033: // sprmSTextFlow
			if len(op) >= 2 {
				sec.TextDirection = tableTextDirectionFromFlow(binary.LittleEndian.Uint16(op))
			}
		case 0x5040: // sprmSNfcFtnRef
			if len(op) >= 2 {
				sec.FootnoteNumFmt = sectionNFCFormat(byte(binary.LittleEndian.Uint16(op)))
			}
		case 0x5042: // sprmSNfcEdnRef
			if len(op) >= 2 {
				sec.EndnoteNumFmt = sectionNFCFormat(byte(binary.LittleEndian.Uint16(op)))
			}
		case 0x702B: // sprmSBrcTop80
			pageBorderTop = parseBRC80(op)
			hasPageBorders = hasPageBorders || pageBorderTop != nil
		case 0x702C: // sprmSBrcLeft80
			pageBorderLeft = parseBRC80(op)
			hasPageBorders = hasPageBorders || pageBorderLeft != nil
		case 0x702D: // sprmSBrcBottom80
			pageBorderBottom = parseBRC80(op)
			hasPageBorders = hasPageBorders || pageBorderBottom != nil
		case 0x702E: // sprmSBrcRight80
			pageBorderRight = parseBRC80(op)
			hasPageBorders = hasPageBorders || pageBorderRight != nil
		case 0xD234: // sprmSBrcTop
			if side := parseBRC(op); side != nil {
				pageBorderTop = side
				hasPageBorders = true
			}
		case 0xD235: // sprmSBrcLeft
			if side := parseBRC(op); side != nil {
				pageBorderLeft = side
				hasPageBorders = true
			}
		case 0xD236: // sprmSBrcBottom
			if side := parseBRC(op); side != nil {
				pageBorderBottom = side
				hasPageBorders = true
			}
		case 0xD237: // sprmSBrcRight
			if side := parseBRC(op); side != nil {
				pageBorderRight = side
				hasPageBorders = true
			}
		case 0x522F: // sprmSPgbProp
			if len(op) >= 2 {
				pgbProp := binary.LittleEndian.Uint16(op)
				hasPageBorders = true
				switch pgbProp & 0x03 {
				case 0:
					pageBorderDisplay = "allPages"
				case 1:
					pageBorderDisplay = "firstPage"
				case 2:
					pageBorderDisplay = "notFirstPage"
				}
				if pgbProp&0x04 != 0 {
					pageBorderOffsetFrom = "edge"
				} else {
					pageBorderOffsetFrom = "text"
				}
				if pgbProp&0x08 != 0 {
					pageBorderZOrder = "back"
				} else {
					pageBorderZOrder = "front"
				}
			}
		case 0x5007:
			addUnmappedSEPX(&diagnostics, code, "sprmSDmBinFirst")
		case 0x5008:
			addUnmappedSEPX(&diagnostics, code, "sprmSDmBinOther")
		case 0x3000:
			addUnmappedSEPX(&diagnostics, code, "sprmScnsPgn")
		case 0x3001:
			addUnmappedSEPX(&diagnostics, code, "sprmSiHeadingPgn")
		case 0x5026:
			addUnmappedSEPX(&diagnostics, code, "sprmSDmPaperReq")
		case 0x3239:
			addUnmappedSEPX(&diagnostics, code, "sprmSWall")
		case 0x703A:
			addUnmappedSEPX(&diagnostics, code, "sprmSRsid")
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
	sec.Diagnostics = diagnostics
	return sec, revision
}

func sectionNFCFormat(v byte) string {
	switch v {
	case 0:
		return "decimal"
	case 1:
		return "upperRoman"
	case 2:
		return "lowerRoman"
	case 3:
		return "upperLetter"
	case 4:
		return "lowerLetter"
	case 5:
		return "ordinal"
	case 6:
		return "cardinalText"
	case 7:
		return "ordinalText"
	case 22:
		return "decimalZero"
	case 23, 255:
		return "bullet"
	default:
		return ""
	}
}

func addUnmappedSEPX(list *diag.List, code uint16, name string) {
	if list == nil {
		return
	}
	list.Add(diag.Diagnostic{
		Severity: diag.SeverityInfo,
		Code:     "legacy.sprm.unmapped",
		Message:  fmt.Sprintf("SPRM %s (0x%04X) is recognized but has no semantic mapping", name, code),
	})
}

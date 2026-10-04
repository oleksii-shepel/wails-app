package legacy

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"strings"

	sheetir "office/pkg/sheet/ir"
)

// BIFF8 records written by Excel 2007+ that carry theme-based colours.
const (
	biffTheme = 0x0896 // THEME — workbook theme (default or a zipped theme part)
	biffXFExt = 0x087D // XFEXT — extended XF properties (full colours, theme tints)
)

// biffDefaultThemeVersion marks a THEME record that uses Excel's built-in theme.
const biffDefaultThemeVersion = 124226

// defaultBIFFThemeColors is the built-in Office theme referenced by a default
// THEME record, in SpreadsheetML theme-index order (lt1, dk1, lt2, dk2, accents,
// hyperlink, followed hyperlink).
var defaultBIFFThemeColors = [12]string{
	"FFFFFF", "000000", "EEECE1", "1F497D",
	"4F81BD", "C0504D", "9BBB59", "8064A2", "4BACC6", "F79646",
	"0000FF", "800080",
}

// biffContinueFrt12 continues a future record; its payload repeats a 12-byte FrtHeader.
const biffContinueFrt12 = 0x0812

// gatherThemePayload joins a THEME record with its CONTINUEFRT12 continuations.
func gatherThemePayload(data []byte, pos int) ([]byte, int) {
	_, payload, next, ok := readBiffRecord(data, pos)
	if !ok {
		return nil, pos
	}
	full := append([]byte(nil), payload...)
	for {
		id, more, nextPos, ok := readBiffRecord(data, next)
		if !ok || id != biffContinueFrt12 || len(more) < 12 {
			break
		}
		full = append(full, more[12:]...)
		next = nextPos
	}
	return full, next
}

// biffCFEx extends a CONDFMT/CF rule with Excel 2007 formatting (CFEX).
const biffCFEx = 0x087B

// condFmtID returns the nID of a CONDFMT record (bits 1-15 after ccf).
func condFmtID(payload []byte) int {
	if len(payload) < 4 {
		return 0
	}
	return int(binary.LittleEndian.Uint16(payload[2:4]) >> 1)
}

// applyCFExRecord applies a CFEX record to the classic CF rule it extends: its
// priority and the full-colour XFExtNoFXT that follows the rule's DXFN.
func applyCFExRecord(payload []byte, rulesByID map[int][]*sheetir.CFRule, palette *[56]string, theme *[12]string) {
	// FrtRefHeaderU (12) + fIsCF12 (4) + nID (2) + CFExNonCF12: icf (2), cp (1),
	// icfTemplate (1), ipriority (2), flags (1), fHasDXF (1), DXFN12...
	if len(payload) < 30 || binary.LittleEndian.Uint32(payload[12:16]) != 0 {
		return
	}
	id := int(binary.LittleEndian.Uint16(payload[16:18]))
	icf := int(binary.LittleEndian.Uint16(payload[18:20]))
	rules := rulesByID[id]
	if icf < 0 || icf >= len(rules) {
		return
	}
	rule := rules[icf]
	rule.Priority = int(binary.LittleEndian.Uint16(payload[22:24]))
	if payload[25] == 0 {
		return
	}
	cb := int(binary.LittleEndian.Uint32(payload[26:30]))
	if cb <= 0 || 30+cb > len(payload) {
		return
	}
	block := payload[30 : 30+cb]
	dxf, n, ok := parseDXFN(block, palette)
	if !ok {
		return
	}
	// XFExtNoFXT: reserved1 (2) + reserved2 (2) + reserved3 (2) + cexts...
	if n+8 <= len(block) {
		applyDXFExtProps(dxf, parseExtProps(block[n+6:]), palette, theme)
	}
	finishDXFFill(dxf)
	rule.DxfStyle = *dxf
}

// parseThemeRecord returns the workbook theme colours from a THEME record:
// the built-in scheme for a default theme, or the clrScheme of the zipped theme.
func parseThemeRecord(payload []byte) ([12]string, bool) {
	if len(payload) < 16 {
		return [12]string{}, false
	}
	version := binary.LittleEndian.Uint32(payload[12:16])
	themeZip := payload[16:]
	if len(themeZip) == 0 {
		if version == biffDefaultThemeVersion {
			return defaultBIFFThemeColors, true
		}
		return [12]string{}, false
	}
	return parseThemePackageColors(themeZip)
}

type themeColorXML struct {
	SrgbClr *struct {
		Val string `xml:"val,attr"`
	} `xml:"srgbClr"`
	SysClr *struct {
		LastClr string `xml:"lastClr,attr"`
	} `xml:"sysClr"`
}

func (c *themeColorXML) hex() string {
	switch {
	case c == nil:
		return ""
	case c.SrgbClr != nil:
		return strings.ToUpper(c.SrgbClr.Val)
	case c.SysClr != nil:
		return strings.ToUpper(c.SysClr.LastClr)
	}
	return ""
}

func parseThemePackageColors(data []byte) ([12]string, bool) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return [12]string{}, false
	}
	for _, f := range zr.File {
		if !strings.HasPrefix(strings.ToLower(f.Name), "theme/theme/theme") || !strings.HasSuffix(strings.ToLower(f.Name), ".xml") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return [12]string{}, false
		}
		raw, err := io.ReadAll(io.LimitReader(rc, 4<<20))
		rc.Close()
		if err != nil {
			return [12]string{}, false
		}
		var theme struct {
			Scheme struct {
				Dk1      themeColorXML `xml:"dk1"`
				Lt1      themeColorXML `xml:"lt1"`
				Dk2      themeColorXML `xml:"dk2"`
				Lt2      themeColorXML `xml:"lt2"`
				Accent1  themeColorXML `xml:"accent1"`
				Accent2  themeColorXML `xml:"accent2"`
				Accent3  themeColorXML `xml:"accent3"`
				Accent4  themeColorXML `xml:"accent4"`
				Accent5  themeColorXML `xml:"accent5"`
				Accent6  themeColorXML `xml:"accent6"`
				Hlink    themeColorXML `xml:"hlink"`
				FolHlink themeColorXML `xml:"folHlink"`
			} `xml:"themeElements>clrScheme"`
		}
		if err := xml.Unmarshal(raw, &theme); err != nil {
			return [12]string{}, false
		}
		s := theme.Scheme
		colors := [12]string{
			s.Lt1.hex(), s.Dk1.hex(), s.Lt2.hex(), s.Dk2.hex(),
			s.Accent1.hex(), s.Accent2.hex(), s.Accent3.hex(), s.Accent4.hex(), s.Accent5.hex(), s.Accent6.hex(),
			s.Hlink.hex(), s.FolHlink.hex(),
		}
		for i, c := range colors {
			if c == "" {
				colors[i] = defaultBIFFThemeColors[i]
			}
		}
		return colors, true
	}
	return [12]string{}, false
}

// biffExtProp is one ExtProp entry of XFEXT / XFExtNoFXT.
type biffExtProp struct {
	Type uint16
	Data []byte
}

// XFEXT / XFExtNoFXT extension types (MS-XLS ExtProp).
const (
	extPropForeColor   = 0x0004
	extPropBackColor   = 0x0005
	extPropTopColor    = 0x0007
	extPropBottomColor = 0x0008
	extPropLeftColor   = 0x0009
	extPropRightColor  = 0x000A
	extPropDiagColor   = 0x000B
	extPropTextColor   = 0x000D
)

// parseExtProps decodes cexts ExtProp entries starting at data[0:2] (cexts).
func parseExtProps(data []byte) []biffExtProp {
	if len(data) < 2 {
		return nil
	}
	count := int(binary.LittleEndian.Uint16(data[0:2]))
	pos := 2
	props := make([]biffExtProp, 0, count)
	for i := 0; i < count && pos+4 <= len(data); i++ {
		typ := binary.LittleEndian.Uint16(data[pos : pos+2])
		cb := int(binary.LittleEndian.Uint16(data[pos+2 : pos+4]))
		if cb < 4 || pos+cb > len(data) {
			break
		}
		props = append(props, biffExtProp{Type: typ, Data: data[pos+4 : pos+cb]})
		pos += cb
	}
	return props
}

// parseXFExtRecord decodes an XFEXT record into the XF index it extends and its properties.
func parseXFExtRecord(payload []byte) (int, []biffExtProp, bool) {
	// FrtHeader (12) + reserved1 (2) + ixfe (2) + reserved2 (2) + cexts...
	if len(payload) < 20 {
		return 0, nil, false
	}
	ixfe := int(binary.LittleEndian.Uint16(payload[14:16]))
	return ixfe, parseExtProps(payload[18:]), true
}

// fullColorExt resolves a FullColorExt (xclrType, nTintShade, xclrValue) to RGB hex.
func fullColorExt(data []byte, palette *[56]string, theme *[12]string) string {
	if len(data) < 8 {
		return ""
	}
	kind := binary.LittleEndian.Uint16(data[0:2])
	tint := float64(int16(binary.LittleEndian.Uint16(data[2:4]))) / 32767.0
	value := data[4:8]
	switch kind {
	case 1: // indexed
		return resolveICV(int(binary.LittleEndian.Uint32(value)), palette)
	case 2: // RGB(A)
		return applyTintToHex(fmt.Sprintf("%02X%02X%02X", value[0], value[1], value[2]), tint)
	case 3: // theme
		return resolveThemeColor(int(binary.LittleEndian.Uint32(value)), tint, theme)
	}
	return ""
}

// applyXFExtToStyle lets XFEXT colours supersede the palette approximations in an XF.
func applyXFExtToStyle(style *sheetir.CellStyle, props []biffExtProp, palette *[56]string, theme *[12]string) {
	filled := style.FillPattern != "" && style.FillPattern != "none"
	for _, prop := range props {
		color := fullColorExt(prop.Data, palette, theme)
		if color == "" {
			continue
		}
		switch prop.Type {
		case extPropForeColor:
			if filled {
				style.BgColor = color
			}
		case extPropBackColor:
			if filled && style.FillPatternBgColor != "" {
				style.FillPatternBgColor = color
			}
		case extPropTopColor:
			style.BorderTop.Color = color
		case extPropBottomColor:
			style.BorderBottom.Color = color
		case extPropLeftColor:
			style.BorderLeft.Color = color
		case extPropRightColor:
			style.BorderRight.Color = color
		case extPropDiagColor:
			style.BorderDiagonal.Color = color
		case extPropTextColor:
			style.FontColor = color
		}
	}
}

// parseDXFN decodes a DXFN differential format: the flag words followed by the
// optional number, font, alignment, border, pattern and protection blocks.
// It returns the style and the number of bytes consumed.
func parseDXFN(data []byte, palette *[56]string) (*sheetir.DxfStyle, int, bool) {
	if len(data) < 6 {
		return nil, 0, false
	}
	flags := binary.LittleEndian.Uint32(data[0:4])
	flags2 := binary.LittleEndian.Uint16(data[4:6])
	pos := 6
	dxf := &sheetir.DxfStyle{}
	if flags&0x02000000 != 0 { // ibitAtrNum
		if flags2&0x0001 != 0 { // fIfmtUser: DXFNumUsr
			if pos+2 > len(data) {
				return nil, 0, false
			}
			cb := int(binary.LittleEndian.Uint16(data[pos : pos+2]))
			if cb < 2 || pos+cb > len(data) {
				return nil, 0, false
			}
			if code, _, ok := readXLUnicodeString(data[pos+2:pos+cb], 0, 2); ok {
				dxf.NumFmtCode = code
			}
			pos += cb
		} else { // DXFNumIfmt
			if pos+2 > len(data) {
				return nil, 0, false
			}
			dxf.NumFmtCode = defaultBIFFFormatCodes()[uint16(data[pos+1])]
			pos += 2
		}
	}
	if flags&0x04000000 != 0 { // ibitAtrFnt: DXFFntD
		if pos+118 > len(data) {
			return nil, 0, false
		}
		applyClassicCFFontFormatting(dxf, data[pos:pos+118], palette)
		pos += 118
	}
	if flags&0x08000000 != 0 { // ibitAtrAlc: DXFALC
		if pos+8 > len(data) {
			return nil, 0, false
		}
		pos += 8
	}
	if flags&0x10000000 != 0 { // ibitAtrBdr: DXFBdr
		if pos+8 > len(data) {
			return nil, 0, false
		}
		applyClassicCFBorderFormatting(dxf, data[pos:pos+8], palette)
		pos += 8
	}
	if flags&0x20000000 != 0 { // ibitAtrPat: DXFPat
		if pos+4 > len(data) {
			return nil, 0, false
		}
		applyClassicCFPatternFormatting(dxf, data[pos:pos+4], palette)
		pos += 4
	}
	if flags&0x40000000 != 0 { // ibitAtrProt: DXFProt
		if pos+2 > len(data) {
			return nil, 0, false
		}
		pos += 2
	}
	return dxf, pos, true
}

// applyDXFExtProps lets XFExtNoFXT colours supersede the palette colours of a DXFN.
func applyDXFExtProps(dxf *sheetir.DxfStyle, props []biffExtProp, palette *[56]string, theme *[12]string) {
	for _, prop := range props {
		color := fullColorExt(prop.Data, palette, theme)
		if color == "" {
			continue
		}
		switch prop.Type {
		case extPropForeColor:
			dxf.FillForegroundColor = color
		case extPropBackColor:
			dxf.FillBackgroundColor = color
		case extPropTopColor:
			dxf.TopBorderColor = color
		case extPropBottomColor:
			dxf.BottomBorderColor = color
		case extPropLeftColor:
			dxf.LeftBorderColor = color
		case extPropRightColor:
			dxf.RightBorderColor = color
		case extPropDiagColor:
			dxf.DiagonalBorderColor = color
		case extPropTextColor:
			dxf.FontColor = color
		}
	}
}

// finishDXFFill derives the painted cell background of a differential fill:
// a solid (or unspecified) pattern paints its background colour, as in Excel.
func finishDXFFill(dxf *sheetir.DxfStyle) {
	if dxf == nil || (dxf.FillForegroundColor == "" && dxf.FillBackgroundColor == "") {
		return
	}
	if dxf.FillPattern == nil || *dxf.FillPattern <= 1 {
		dxf.BgColor = dxf.FillBackgroundColor
		if dxf.BgColor == "" {
			dxf.BgColor = dxf.FillForegroundColor
		}
		return
	}
	dxf.BgColor = dxf.FillForegroundColor
}

// resolveThemeColor resolves a theme colour index (SpreadsheetML order) with a
// tint against the workbook theme, or the built-in theme when theme is nil.
func resolveThemeColor(themeIdx int, tint float64, theme *[12]string) string {
	colors := &defaultBIFFThemeColors
	if theme != nil && theme[0] != "" {
		colors = theme
	}
	if themeIdx < 0 || themeIdx >= len(colors) {
		return ""
	}
	rgb := colors[themeIdx]
	if math.Abs(tint) < 1e-9 {
		return rgb
	}
	return applyTintToHex(rgb, tint)
}

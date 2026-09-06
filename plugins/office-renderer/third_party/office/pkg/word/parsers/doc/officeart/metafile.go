package officeart

import (
	"encoding/binary"
	"fmt"
	"html"
	"math"
	"strings"

	wordir "office/pkg/word/ir"
)

const (
	emrHeader                 = 1
	emrSetWindowExtEx         = 9
	emrSetWindowOrgEx         = 10
	emrEOF                    = 14
	emrSetBkMode              = 18
	emrSetTextColor           = 24
	emrSetBkColor             = 25
	emrMoveToEx               = 27
	emrIntersectClipRect      = 30
	emrSaveDC                 = 33
	emrRestoreDC              = 34
	emrSelectObject           = 37
	emrCreatePen              = 38
	emrCreateBrushIndirect    = 39
	emrDeleteObject           = 40
	emrLineTo                 = 54
	emrExtCreateFontIndirectW = 82
	emrExtTextOutW            = 84
)

type emfObject struct {
	kind       string
	color      string
	width      float64
	fontFamily string
	fontSize   float64
	fontWeight string
	italic     bool
}

type emfState struct {
	textColor   string
	bkColor     string
	transparent bool
	penColor    string
	penWidth    float64
	fontFamily  string
	fontSize    float64
	fontWeight  string
	fontItalic  bool
	currentX    float64
	currentY    float64
	windowOrgX  float64
	windowOrgY  float64
	windowExtX  float64
	windowExtY  float64
}

// MetafileImage projects a decoded OfficeArt vector BLIP into renderable Word
// IR. EMF is converted to SVG so browsers can display legacy drawing/OLE
// previews without platform GDI support. WMF remains unsupported until a
// shared WMF renderer is available.
func (b BLIP) MetafileImage() *wordir.Image {
	if b.MIMEType != "image/x-emf" || len(b.Data) == 0 {
		return nil
	}
	svg, width, height, ok := emfToSVG(b.Data)
	if !ok {
		return nil
	}
	image := &wordir.Image{MIMEType: "image/svg+xml", Data: svg}
	if width > 0 && height > 0 {
		image.Width = pixelsToEMU(int(math.Round(width)))
		image.Height = pixelsToEMU(int(math.Round(height)))
	}
	return image
}

func emfToSVG(data []byte) ([]byte, float64, float64, bool) {
	if len(data) < 108 || binary.LittleEndian.Uint32(data[0:4]) != emrHeader {
		return nil, 0, 0, false
	}
	left := int32(binary.LittleEndian.Uint32(data[8:12]))
	top := int32(binary.LittleEndian.Uint32(data[12:16]))
	right := int32(binary.LittleEndian.Uint32(data[16:20]))
	bottom := int32(binary.LittleEndian.Uint32(data[20:24]))
	width := float64(right - left)
	height := float64(bottom - top)
	if width <= 0 || height <= 0 {
		return nil, 0, 0, false
	}

	state := emfState{textColor: "#000000", bkColor: "#ffffff", transparent: true, penColor: "#000000", penWidth: 1, fontFamily: "Arial", fontSize: 12, windowExtX: width, windowExtY: height}
	objects := map[uint32]emfObject{}
	stack := []emfState{}
	var body strings.Builder
	suppressWorksheetGrid := emfLooksLikeTextGrid(data)

	for off := 0; off+8 <= len(data); {
		recordType := binary.LittleEndian.Uint32(data[off : off+4])
		recordSize := int(binary.LittleEndian.Uint32(data[off+4 : off+8]))
		if recordSize < 8 || off+recordSize > len(data) {
			break
		}
		record := data[off : off+recordSize]
		switch recordType {
		case emrSetWindowExtEx:
			if len(record) >= 16 {
				state.windowExtX = float64(int32(binary.LittleEndian.Uint32(record[8:12])))
				state.windowExtY = float64(int32(binary.LittleEndian.Uint32(record[12:16])))
			}
		case emrSetWindowOrgEx:
			if len(record) >= 16 {
				state.windowOrgX = float64(int32(binary.LittleEndian.Uint32(record[8:12])))
				state.windowOrgY = float64(int32(binary.LittleEndian.Uint32(record[12:16])))
			}
		case emrSetBkMode:
			if len(record) >= 12 {
				state.transparent = binary.LittleEndian.Uint32(record[8:12]) == 1
			}
		case emrSetTextColor:
			if len(record) >= 12 {
				state.textColor = emfColor(binary.LittleEndian.Uint32(record[8:12]))
			}
		case emrSetBkColor:
			if len(record) >= 12 {
				state.bkColor = emfColor(binary.LittleEndian.Uint32(record[8:12]))
			}
		case emrSaveDC:
			stack = append(stack, state)
		case emrRestoreDC:
			if len(stack) > 0 {
				state = stack[len(stack)-1]
				stack = stack[:len(stack)-1]
			}
		case emrCreatePen:
			if len(record) >= 28 {
				handle := binary.LittleEndian.Uint32(record[8:12])
				penWidth := math.Abs(float64(int32(binary.LittleEndian.Uint32(record[16:20]))))
				if penWidth <= 0 {
					penWidth = 1
				}
				objects[handle] = emfObject{kind: "pen", width: penWidth, color: emfColor(binary.LittleEndian.Uint32(record[24:28]))}
			}
		case emrCreateBrushIndirect:
			if len(record) >= 24 {
				handle := binary.LittleEndian.Uint32(record[8:12])
				objects[handle] = emfObject{kind: "brush", color: emfColor(binary.LittleEndian.Uint32(record[16:20]))}
			}
		case emrExtCreateFontIndirectW:
			if len(record) >= 104 {
				handle := binary.LittleEndian.Uint32(record[8:12])
				fontBase := 12
				fontHeight := math.Abs(float64(int32(binary.LittleEndian.Uint32(record[fontBase : fontBase+4]))))
				if fontHeight <= 0 {
					fontHeight = 12
				}
				weight := int32(binary.LittleEndian.Uint32(record[fontBase+16 : fontBase+20]))
				family := utf16FixedString(record[fontBase+28:], 32)
				if family == "" {
					family = "Arial"
				}
				fontWeight := "normal"
				if weight >= 600 {
					fontWeight = "bold"
				}
				objects[handle] = emfObject{kind: "font", fontFamily: family, fontSize: fontHeight, fontWeight: fontWeight, italic: record[fontBase+20] != 0}
			}
		case emrSelectObject:
			if len(record) >= 12 {
				handle := binary.LittleEndian.Uint32(record[8:12])
				if object, ok := objects[handle]; ok {
					switch object.kind {
					case "pen":
						state.penColor, state.penWidth = object.color, object.width
					case "font":
						state.fontFamily, state.fontSize = object.fontFamily, object.fontSize
						state.fontWeight, state.fontItalic = object.fontWeight, object.italic
					}
				}
			}
		case emrDeleteObject:
			if len(record) >= 12 {
				delete(objects, binary.LittleEndian.Uint32(record[8:12]))
			}
		case emrMoveToEx:
			if len(record) >= 16 {
				state.currentX, state.currentY = emfPoint(state, int32(binary.LittleEndian.Uint32(record[8:12])), int32(binary.LittleEndian.Uint32(record[12:16])), width, height)
			}
		case emrLineTo:
			if len(record) >= 16 {
				x, y := emfPoint(state, int32(binary.LittleEndian.Uint32(record[8:12])), int32(binary.LittleEndian.Uint32(record[12:16])), width, height)
				if !(suppressWorksheetGrid && strings.EqualFold(state.penColor, "#c0c0c0")) {
					fmt.Fprintf(&body, `<line x1="%.3f" y1="%.3f" x2="%.3f" y2="%.3f" stroke="%s" stroke-width="%.3f"/>`, state.currentX, state.currentY, x, y, state.penColor, math.Max(0.35, state.penWidth))
				}
				state.currentX, state.currentY = x, y
			}
		case emrExtTextOutW:
			if len(record) >= 76 {
				xRaw := int32(binary.LittleEndian.Uint32(record[36:40]))
				yRaw := int32(binary.LittleEndian.Uint32(record[40:44]))
				chars := int(binary.LittleEndian.Uint32(record[44:48]))
				stringOffset := int(binary.LittleEndian.Uint32(record[48:52]))
				if chars > 0 && stringOffset >= 0 && stringOffset+chars*2 <= len(record) {
					text := utf16String(record[stringOffset : stringOffset+chars*2])
					x, y := emfPoint(state, xRaw, yRaw, width, height)
					fontSize := state.fontSize
					if fontSize <= 0 {
						fontSize = 12
					}
					style := ""
					if state.fontItalic {
						style = ` font-style="italic"`
					}
					fmt.Fprintf(&body, `<text x="%.3f" y="%.3f" fill="%s" font-family="%s" font-size="%.3f" font-weight="%s" dominant-baseline="hanging"%s>%s</text>`, x, y, state.textColor, html.EscapeString(state.fontFamily), fontSize, state.fontWeight, style, html.EscapeString(text))
				}
			}
		case emrIntersectClipRect:
			// Browser font metrics differ from GDI; clipping here truncates text.
		case emrEOF:
			off = len(data)
			continue
		}
		off += recordSize
	}

	var svg strings.Builder
	fmt.Fprintf(&svg, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %.3f %.3f" width="%.3f" height="%.3f" preserveAspectRatio="none">`, width, height, width, height)
	svg.WriteString(body.String())
	svg.WriteString(`</svg>`)
	return []byte(svg.String()), width, height, true
}

func emfLooksLikeTextGrid(data []byte) bool {
	textCount, lineCount := 0, 0
	for off := 0; off+8 <= len(data); {
		recordType := binary.LittleEndian.Uint32(data[off : off+4])
		recordSize := int(binary.LittleEndian.Uint32(data[off+4 : off+8]))
		if recordSize < 8 || off+recordSize > len(data) {
			break
		}
		switch recordType {
		case emrExtTextOutW:
			textCount++
		case emrLineTo:
			lineCount++
		}
		off += recordSize
	}
	return textCount >= 6 && lineCount >= 4
}

func emfPoint(state emfState, x, y int32, width, height float64) (float64, float64) {
	sx, sy := 1.0, 1.0
	if state.windowExtX != 0 {
		sx = width / math.Abs(state.windowExtX)
	}
	if state.windowExtY != 0 {
		sy = height / math.Abs(state.windowExtY)
	}
	return (float64(x) - state.windowOrgX) * sx, (float64(y) - state.windowOrgY) * sy
}

func emfColor(raw uint32) string {
	return fmt.Sprintf("#%02x%02x%02x", raw&0xff, (raw>>8)&0xff, (raw>>16)&0xff)
}

func utf16String(data []byte) string {
	if len(data) < 2 {
		return ""
	}
	var b strings.Builder
	for i := 0; i+1 < len(data); i += 2 {
		u := binary.LittleEndian.Uint16(data[i : i+2])
		if u == 0 {
			break
		}
		if u >= 0xD800 && u <= 0xDFFF {
			b.WriteRune('�')
		} else {
			b.WriteRune(rune(u))
		}
	}
	return b.String()
}

func utf16FixedString(data []byte, maxUnits int) string {
	if maxUnits <= 0 {
		return ""
	}
	limit := maxUnits * 2
	if len(data) < limit {
		limit = len(data) - len(data)%2
	}
	return strings.TrimSpace(utf16String(data[:limit]))
}

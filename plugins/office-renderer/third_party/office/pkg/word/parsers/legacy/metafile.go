package legacy

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"html"
	"io"
	"math"
	"strings"

	wordir "office/pkg/word/ir"
)

const (
	emrHeader                 = 1
	emrSetWindowExtEx         = 9
	emrSetWindowOrgEx         = 10
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
	emrEOF                    = 14
)

type legacyEMFObject struct {
	kind       string
	color      string
	width      float64
	fontFamily string
	fontSize   float64
	fontWeight string
	italic     bool
}

type legacyEMFState struct {
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

func legacyExtractOfficeArtMetafile(recType uint16, payload []byte) ([]byte, bool) {
	if recType != legacyOfficeArtBlipEMF && recType != legacyOfficeArtBlipWMF || len(payload) < 50 {
		return nil, false
	}
	// OfficeArt metafile blips begin with a 16-byte UID followed by the
	// 34-byte OfficeArtMetafileHeader. cbSize is the uncompressed payload size;
	// cbSave is the compressed/stored byte count. Word normally stores EMF/WMF
	// previews with zlib compression immediately after that header.
	uncompressedSize := int(binary.LittleEndian.Uint32(payload[16:20]))
	storedSize := int(binary.LittleEndian.Uint32(payload[44:48]))
	if uncompressedSize <= 0 || storedSize <= 0 || 50+storedSize > len(payload) {
		return nil, false
	}
	stored := payload[50 : 50+storedSize]
	zr, err := zlib.NewReader(bytes.NewReader(stored))
	if err != nil {
		// A handful of producers store an uncompressed metafile. Preserve that
		// path when the byte count already matches cbSize.
		if len(stored) == uncompressedSize {
			return append([]byte(nil), stored...), true
		}
		return nil, false
	}
	decompressed, err := io.ReadAll(zr)
	closeErr := zr.Close()
	if err != nil || closeErr != nil || len(decompressed) == 0 {
		return nil, false
	}
	if uncompressedSize > 0 && len(decompressed) > uncompressedSize {
		decompressed = decompressed[:uncompressedSize]
	}
	return decompressed, true
}

func legacyOfficeArtMetafileImage(recType uint16, payload []byte) *wordir.Image {
	meta, ok := legacyExtractOfficeArtMetafile(recType, payload)
	if !ok || recType != legacyOfficeArtBlipEMF {
		return nil
	}
	svg, width, height, ok := legacyEMFToSVG(meta)
	if !ok {
		return nil
	}
	img := &wordir.Image{MIMEType: "image/svg+xml", Data: svg}
	if width > 0 && height > 0 {
		img.Width = legacyPixelsToEMU(int(math.Round(width)))
		img.Height = legacyPixelsToEMU(int(math.Round(height)))
	}
	return img
}

func legacyEMFToSVG(data []byte) ([]byte, float64, float64, bool) {
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

	state := legacyEMFState{
		textColor:   "#000000",
		bkColor:     "#ffffff",
		transparent: true,
		penColor:    "#000000",
		penWidth:    1,
		fontFamily:  "Arial",
		fontSize:    12,
		windowExtX:  width,
		windowExtY:  height,
	}
	objects := map[uint32]legacyEMFObject{}
	stack := []legacyEMFState{}
	var body strings.Builder
	// Spreadsheet/OLE previews often contain GDI's light-gray worksheet grid.
	// Word does not paint those grid guides in the embedded-object preview used
	// by this fixture. Keep authored dark borders, but suppress the regular
	// silver grid when the metafile is clearly a text-table preview.
	suppressWorksheetGrid := legacyEMFLooksLikeTextGrid(data)

	for off := 0; off+8 <= len(data); {
		recType := binary.LittleEndian.Uint32(data[off : off+4])
		recSize := int(binary.LittleEndian.Uint32(data[off+4 : off+8]))
		if recSize < 8 || off+recSize > len(data) {
			break
		}
		rec := data[off : off+recSize]
		switch recType {
		case emrSetWindowExtEx:
			if len(rec) >= 16 {
				state.windowExtX = float64(int32(binary.LittleEndian.Uint32(rec[8:12])))
				state.windowExtY = float64(int32(binary.LittleEndian.Uint32(rec[12:16])))
			}
		case emrSetWindowOrgEx:
			if len(rec) >= 16 {
				state.windowOrgX = float64(int32(binary.LittleEndian.Uint32(rec[8:12])))
				state.windowOrgY = float64(int32(binary.LittleEndian.Uint32(rec[12:16])))
			}
		case emrSetBkMode:
			if len(rec) >= 12 {
				state.transparent = binary.LittleEndian.Uint32(rec[8:12]) == 1
			}
		case emrSetTextColor:
			if len(rec) >= 12 {
				state.textColor = legacyEMFColor(binary.LittleEndian.Uint32(rec[8:12]))
			}
		case emrSetBkColor:
			if len(rec) >= 12 {
				state.bkColor = legacyEMFColor(binary.LittleEndian.Uint32(rec[8:12]))
			}
		case emrSaveDC:
			stack = append(stack, state)
		case emrRestoreDC:
			if len(stack) > 0 {
				state = stack[len(stack)-1]
				stack = stack[:len(stack)-1]
			}
		case emrCreatePen:
			if len(rec) >= 28 {
				handle := binary.LittleEndian.Uint32(rec[8:12])
				penWidth := math.Abs(float64(int32(binary.LittleEndian.Uint32(rec[16:20]))))
				if penWidth <= 0 {
					penWidth = 1
				}
				objects[handle] = legacyEMFObject{kind: "pen", width: penWidth, color: legacyEMFColor(binary.LittleEndian.Uint32(rec[24:28]))}
			}
		case emrCreateBrushIndirect:
			if len(rec) >= 24 {
				handle := binary.LittleEndian.Uint32(rec[8:12])
				objects[handle] = legacyEMFObject{kind: "brush", color: legacyEMFColor(binary.LittleEndian.Uint32(rec[16:20]))}
			}
		case emrExtCreateFontIndirectW:
			if len(rec) >= 104 {
				handle := binary.LittleEndian.Uint32(rec[8:12])
				fontBase := 12
				fontHeight := math.Abs(float64(int32(binary.LittleEndian.Uint32(rec[fontBase : fontBase+4]))))
				if fontHeight <= 0 {
					fontHeight = 12
				}
				weight := int32(binary.LittleEndian.Uint32(rec[fontBase+16 : fontBase+20]))
				family := legacyUTF16FixedString(rec[fontBase+28:], 32)
				if family == "" {
					family = "Arial"
				}
				fontWeight := "normal"
				if weight >= 600 {
					fontWeight = "bold"
				}
				objects[handle] = legacyEMFObject{kind: "font", fontFamily: family, fontSize: fontHeight, fontWeight: fontWeight, italic: rec[fontBase+20] != 0}
			}
		case emrSelectObject:
			if len(rec) >= 12 {
				handle := binary.LittleEndian.Uint32(rec[8:12])
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
			if len(rec) >= 12 {
				delete(objects, binary.LittleEndian.Uint32(rec[8:12]))
			}
		case emrMoveToEx:
			if len(rec) >= 16 {
				state.currentX, state.currentY = legacyEMFPoint(state, int32(binary.LittleEndian.Uint32(rec[8:12])), int32(binary.LittleEndian.Uint32(rec[12:16])), width, height)
			}
		case emrLineTo:
			if len(rec) >= 16 {
				x, y := legacyEMFPoint(state, int32(binary.LittleEndian.Uint32(rec[8:12])), int32(binary.LittleEndian.Uint32(rec[12:16])), width, height)
				if !(suppressWorksheetGrid && strings.EqualFold(state.penColor, "#c0c0c0")) {
					fmt.Fprintf(&body, `<line x1="%.3f" y1="%.3f" x2="%.3f" y2="%.3f" stroke="%s" stroke-width="%.3f"/>`, state.currentX, state.currentY, x, y, state.penColor, math.Max(0.35, state.penWidth))
				}
				state.currentX, state.currentY = x, y
			}
		case emrExtTextOutW:
			if len(rec) >= 76 {
				xRaw := int32(binary.LittleEndian.Uint32(rec[36:40]))
				yRaw := int32(binary.LittleEndian.Uint32(rec[40:44]))
				chars := int(binary.LittleEndian.Uint32(rec[44:48]))
				stringOffset := int(binary.LittleEndian.Uint32(rec[48:52]))
				if chars > 0 && stringOffset >= 0 && stringOffset+chars*2 <= len(rec) {
					text := legacyUTF16String(rec[stringOffset : stringOffset+chars*2])
					x, y := legacyEMFPoint(state, xRaw, yRaw, width, height)
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
			// Intentionally do not clip text to the original GDI cell rectangle.
			// Browser font metrics differ from Word/GDI and strict clipping is what
			// turns values such as "September" into "Septembe" in HTML previews.
		case emrEOF:
			off = len(data)
			continue
		}
		off += recSize
	}

	var svg strings.Builder
	fmt.Fprintf(&svg, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %.3f %.3f" width="%.3f" height="%.3f" preserveAspectRatio="none">`, width, height, width, height)
	svg.WriteString(body.String())
	svg.WriteString(`</svg>`)
	return []byte(svg.String()), width, height, true
}

func legacyEMFLooksLikeTextGrid(data []byte) bool {
	textCount := 0
	lineCount := 0
	for off := 0; off+8 <= len(data); {
		recType := binary.LittleEndian.Uint32(data[off : off+4])
		recSize := int(binary.LittleEndian.Uint32(data[off+4 : off+8]))
		if recSize < 8 || off+recSize > len(data) {
			break
		}
		switch recType {
		case emrExtTextOutW:
			textCount++
		case emrLineTo:
			lineCount++
		}
		off += recSize
	}
	return textCount >= 6 && lineCount >= 4
}

func legacyEMFPoint(state legacyEMFState, x, y int32, width, height float64) (float64, float64) {
	sx, sy := 1.0, 1.0
	if state.windowExtX != 0 {
		sx = width / math.Abs(state.windowExtX)
	}
	if state.windowExtY != 0 {
		sy = height / math.Abs(state.windowExtY)
	}
	return (float64(x) - state.windowOrgX) * sx, (float64(y) - state.windowOrgY) * sy
}

func legacyEMFColor(raw uint32) string {
	r := raw & 0xff
	g := (raw >> 8) & 0xff
	b := (raw >> 16) & 0xff
	return fmt.Sprintf("#%02x%02x%02x", r, g, b)
}

func legacyUTF16String(data []byte) string {
	if len(data) < 2 {
		return ""
	}
	units := make([]uint16, 0, len(data)/2)
	for i := 0; i+1 < len(data); i += 2 {
		u := binary.LittleEndian.Uint16(data[i : i+2])
		if u == 0 {
			break
		}
		units = append(units, u)
	}
	var b strings.Builder
	for _, u := range units {
		if u >= 0xD800 && u <= 0xDFFF {
			// The previews we encounter are overwhelmingly BMP text. Keep malformed
			// surrogate code units visible rather than failing the whole preview.
			b.WriteRune('�')
		} else {
			b.WriteRune(rune(u))
		}
	}
	return b.String()
}

func legacyUTF16FixedString(data []byte, maxUnits int) string {
	if maxUnits <= 0 {
		return ""
	}
	limit := maxUnits * 2
	if len(data) < limit {
		limit = len(data) - len(data)%2
	}
	return strings.TrimSpace(legacyUTF16String(data[:limit]))
}

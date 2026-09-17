package metafile

import (
	"encoding/binary"
	"fmt"
	"html"
	"math"
	"strings"
)

const (
	emrHeader                 = 1
	emrSetWindowExtEx         = 9
	emrSetWindowOrgEx         = 10
	emrSetMapMode             = 17
	emrSetViewportExtEx       = 11
	emrSetViewportOrgEx       = 12
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
	emrEllipse                = 42
	emrArc                    = 45
	emrPolyline16             = 87
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
	textColor    string
	bkColor      string
	transparent  bool
	penColor     string
	penWidth     float64
	brushColor   string
	brushNull    bool
	fontFamily   string
	fontSize     float64
	fontWeight   string
	fontItalic   bool
	currentX     float64
	currentY     float64
	windowOrgX   float64
	windowOrgY   float64
	windowExtX   float64
	windowExtY   float64
	viewportOrgX float64
	viewportOrgY float64
	viewportExtX float64
	viewportExtY float64
	mapMode      int32
	deviceOrgX   float64
	deviceOrgY   float64
}

// ToSVG converts browser-unsupported Office metafiles into SVG. WMF files
// that carry the standard WMFC embedded-EMF escape are reconstructed before
// using the same EMF projection path.
func ToSVG(data []byte, mime string) ([]byte, float64, float64, bool) {
	switch mime {
	case "image/emf", "image/x-emf":
		return emfToSVG(data)
	case "image/wmf", "image/x-wmf":
		if emf, ok := embeddedEMFFromWMF(data); ok {
			return emfToSVG(emf)
		}
		return wmfToSVG(data)
	default:
		return nil, 0, 0, false
	}
}

func embeddedEMFFromWMF(data []byte) ([]byte, bool) {
	off := 0
	if len(data) >= 22 && binary.LittleEndian.Uint32(data[:4]) == 0x9ac6cdd7 {
		off = 22
	}
	if len(data) < off+18 {
		return nil, false
	}
	off += 18
	var emf []byte
	for off+6 <= len(data) {
		words := int(binary.LittleEndian.Uint32(data[off : off+4]))
		fn := binary.LittleEndian.Uint16(data[off+4 : off+6])
		if words < 3 || off+words*2 > len(data) {
			break
		}
		rec := data[off+6 : off+words*2]
		if fn == 0x0626 && len(rec) >= 4 {
			escape := binary.LittleEndian.Uint16(rec[:2])
			count := int(binary.LittleEndian.Uint16(rec[2:4]))
			if escape == 0x000f && count >= 34 && len(rec) >= 4+count {
				chunk := rec[4 : 4+count]
				if len(chunk) >= 34 && string(chunk[:4]) == "WMFC" {
					emf = append(emf, chunk[34:]...)
				}
			}
		}
		off += words * 2
		if fn == 0 {
			break
		}
	}
	if len(emf) < 44 || binary.LittleEndian.Uint32(emf[:4]) != emrHeader || string(emf[40:44]) != " EMF" {
		return nil, false
	}
	return emf, true
}

type wmfObject struct {
	kind  string
	color string
	width float64
	null  bool
}

// wmfToSVG projects the browser-unsupported WMF drawing primitives used by
// legacy Office clip-art into SVG. Object handles follow the WMF object-table
// contract: create records claim the first free slot and DeleteObject releases it.
func wmfToSVG(data []byte) ([]byte, float64, float64, bool) {
	off := 0
	if len(data) >= 22 && binary.LittleEndian.Uint32(data[:4]) == 0x9ac6cdd7 {
		off = 22
	}
	if len(data) < off+18 || binary.LittleEndian.Uint16(data[off+2:off+4]) != 9 {
		return nil, 0, 0, false
	}
	objects := make([]*wmfObject, int(binary.LittleEndian.Uint16(data[off+10:off+12])))
	if len(objects) == 0 {
		objects = make([]*wmfObject, 32)
	}
	windowX, windowY := 0.0, 0.0
	windowW, windowH := 0.0, 0.0
	viewportW, viewportH := 0.0, 0.0
	viewportExtSet := false
	mapMode := int16(1) // MM_TEXT
	pen := wmfObject{kind: "pen", color: "#000000", width: 1}
	brush := wmfObject{kind: "brush", null: true}
	var body strings.Builder
	created := false
	alloc := func(obj *wmfObject) {
		for i := range objects {
			if objects[i] == nil {
				objects[i] = obj
				return
			}
		}
		objects = append(objects, obj)
	}
	off += 18
	for off+6 <= len(data) {
		words := int(binary.LittleEndian.Uint32(data[off : off+4]))
		fn := binary.LittleEndian.Uint16(data[off+4 : off+6])
		if words < 3 || off+words*2 > len(data) {
			break
		}
		params := data[off+6 : off+words*2]
		switch fn {
		case 0x0103: // META_SETMAPMODE
			if len(params) >= 2 {
				mapMode = int16(binary.LittleEndian.Uint16(params[:2]))
			}
		case 0x020b: // META_SETWINDOWORG: y, x
			if len(params) >= 4 {
				windowY = float64(int16(binary.LittleEndian.Uint16(params[0:2])))
				windowX = float64(int16(binary.LittleEndian.Uint16(params[2:4])))
			}
		case 0x020c: // META_SETWINDOWEXT: y, x
			if len(params) >= 4 {
				windowH = float64(int16(binary.LittleEndian.Uint16(params[0:2])))
				windowW = float64(int16(binary.LittleEndian.Uint16(params[2:4])))
			}
		case 0x020e: // META_SETVIEWPORTEXT: y, x
			if len(params) >= 4 {
				viewportH = float64(int16(binary.LittleEndian.Uint16(params[0:2])))
				viewportW = float64(int16(binary.LittleEndian.Uint16(params[2:4])))
				viewportExtSet = viewportW != 0 && viewportH != 0
			}
		case 0x02fa: // META_CREATEPENINDIRECT
			if len(params) >= 10 {
				style := binary.LittleEndian.Uint16(params[0:2])
				width := math.Abs(float64(int16(binary.LittleEndian.Uint16(params[2:4]))))
				if width == 0 {
					width = 1
				}
				alloc(&wmfObject{kind: "pen", color: wmfColor(params[6:10]), width: width, null: style&0x000f == 5})
			}
		case 0x02fc: // META_CREATEBRUSHINDIRECT
			if len(params) >= 8 {
				style := binary.LittleEndian.Uint16(params[0:2])
				alloc(&wmfObject{kind: "brush", color: wmfColor(params[2:6]), null: style == 1})
			}
		case 0x012d: // META_SELECTOBJECT
			if len(params) >= 2 {
				i := int(binary.LittleEndian.Uint16(params[:2]))
				if i >= 0 && i < len(objects) && objects[i] != nil {
					if objects[i].kind == "pen" {
						pen = *objects[i]
					} else if objects[i].kind == "brush" {
						brush = *objects[i]
					}
				}
			}
		case 0x01f0: // META_DELETEOBJECT
			if len(params) >= 2 {
				i := int(binary.LittleEndian.Uint16(params[:2]))
				if i >= 0 && i < len(objects) {
					objects[i] = nil
				}
			}
		case 0x0324: // META_POLYGON
			if len(params) >= 2 {
				n := int(binary.LittleEndian.Uint16(params[:2]))
				if n >= 2 && len(params) >= 2+n*4 {
					var pts strings.Builder
					for i := 0; i < n; i++ {
						x := int16(binary.LittleEndian.Uint16(params[2+i*4 : 4+i*4]))
						y := int16(binary.LittleEndian.Uint16(params[4+i*4 : 6+i*4]))
						if i > 0 {
							pts.WriteByte(' ')
						}
						fmt.Fprintf(&pts, "%d,%d", x, y)
					}
					fill := "none"
					if !brush.null {
						fill = brush.color
					}
					stroke := "none"
					if !pen.null {
						stroke = pen.color
					}
					fmt.Fprintf(&body, "<polygon points=%q fill=%q stroke=%q stroke-width=\"%.3f\"/>\n", pts.String(), fill, stroke, pen.width)
					created = true
				}
			}
		}
		off += words * 2
		if fn == 0 {
			break
		}
	}
	if !created {
		return nil, 0, 0, false
	}
	if windowW == 0 || windowH == 0 {
		return nil, 0, 0, false
	}
	// WMF has no universal y-up convention. The mapping mode plus the signs of
	// window/viewport extents determine axis direction. PowerPoint commonly stores
	// delayed WMFs in MM_ANISOTROPIC with positive extents; reflecting those files
	// unconditionally turns authored clip-art upside down.
	reflectX, reflectY := false, false
	switch mapMode {
	case 1: // MM_TEXT in embedded PowerPoint WMFs: extent signs carry playback orientation
		reflectX = windowW < 0
		reflectY = windowH < 0
	case 2, 3, 4, 5, 6: // metric/English/twips: x-right, y-up
		reflectY = true
	case 7, 8: // isotropic/anisotropic: direction follows viewport/window ratio
		if viewportExtSet {
			reflectX = (windowW < 0) != (viewportW < 0)
			reflectY = (windowH < 0) != (viewportH < 0)
		} else {
			// The default viewport extents are positive.
			reflectX = windowW < 0
			reflectY = windowH < 0
		}
	}

	if windowW < 0 {
		windowX += windowW
		windowW = -windowW
	}
	if windowH < 0 {
		windowY += windowH
		windowH = -windowH
	}

	viewX, viewY := windowX, windowY
	if reflectX {
		viewX = -windowX - windowW
	}
	if reflectY {
		viewY = -windowY - windowH
	}

	var out strings.Builder
	fmt.Fprintf(&out, "<svg xmlns=\"http://www.w3.org/2000/svg\" viewBox=\"%.3f %.3f %.3f %.3f\" width=\"100%%\" height=\"100%%\">\n", viewX, viewY, windowW, windowH)
	switch {
	case reflectX && reflectY:
		out.WriteString("<g transform=\"scale(-1,-1)\">\n")
	case reflectX:
		out.WriteString("<g transform=\"scale(-1,1)\">\n")
	case reflectY:
		out.WriteString("<g transform=\"scale(1,-1)\">\n")
	}
	out.WriteString(body.String())
	if reflectX || reflectY {
		out.WriteString("</g>\n")
	}
	out.WriteString("</svg>")
	return []byte(out.String()), windowW, windowH, true
}

func wmfColor(data []byte) string {
	if len(data) < 4 {
		return "#000000"
	}
	return fmt.Sprintf("#%02X%02X%02X", data[0], data[1], data[2])
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

	state := emfState{textColor: "#000000", bkColor: "#ffffff", transparent: true, penColor: "#000000", penWidth: 1, brushColor: "#ffffff", brushNull: true, fontFamily: "Arial", fontSize: 12, windowExtX: width, windowExtY: height, viewportExtX: width, viewportExtY: height, deviceOrgX: float64(left), deviceOrgY: float64(top)}
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
		case emrSetMapMode:
			if len(record) >= 12 {
				state.mapMode = int32(binary.LittleEndian.Uint32(record[8:12]))
			}
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
		case emrSetViewportExtEx:
			if len(record) >= 16 {
				state.viewportExtX = float64(int32(binary.LittleEndian.Uint32(record[8:12])))
				state.viewportExtY = float64(int32(binary.LittleEndian.Uint32(record[12:16])))
			}
		case emrSetViewportOrgEx:
			if len(record) >= 16 {
				state.viewportOrgX = float64(int32(binary.LittleEndian.Uint32(record[8:12])))
				state.viewportOrgY = float64(int32(binary.LittleEndian.Uint32(record[12:16])))
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
				style := binary.LittleEndian.Uint32(record[12:16])
				objects[handle] = emfObject{kind: "brush", color: emfColor(binary.LittleEndian.Uint32(record[16:20])), width: float64(style)}
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
					case "brush":
						state.brushColor, state.brushNull = object.color, int(object.width) == 1
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
		case emrPolyline16:
			if len(record) >= 28 {
				count := int(binary.LittleEndian.Uint32(record[24:28]))
				if count >= 2 && 28+count*4 <= len(record) {
					var points strings.Builder
					for i := 0; i < count; i++ {
						xRaw := int32(int16(binary.LittleEndian.Uint16(record[28+i*4 : 30+i*4])))
						yRaw := int32(int16(binary.LittleEndian.Uint16(record[30+i*4 : 32+i*4])))
						x, y := emfPoint(state, xRaw, yRaw, width, height)
						if i > 0 {
							points.WriteByte(' ')
						}
						fmt.Fprintf(&points, "%.3f,%.3f", x, y)
					}
					// EMR_POLYLINE16 is an open stroke. Never close it or apply the
					// selected brush: doing so creates phantom triangular fills.
					fmt.Fprintf(&body, `<polyline points="%s" fill="none" stroke="%s" stroke-width="%.3f"/>`, points.String(), state.penColor, math.Max(0.35, state.penWidth))
				}
			}
		case emrEllipse:
			if len(record) >= 24 {
				x1, y1 := emfPoint(state, int32(binary.LittleEndian.Uint32(record[8:12])), int32(binary.LittleEndian.Uint32(record[12:16])), width, height)
				x2, y2 := emfPoint(state, int32(binary.LittleEndian.Uint32(record[16:20])), int32(binary.LittleEndian.Uint32(record[20:24])), width, height)
				fill := "none"
				if !state.brushNull {
					fill = state.brushColor
				}
				fmt.Fprintf(&body, `<ellipse cx="%.3f" cy="%.3f" rx="%.3f" ry="%.3f" fill="%s" stroke="%s" stroke-width="%.3f"/>`, (x1+x2)/2, (y1+y2)/2, math.Abs(x2-x1)/2, math.Abs(y2-y1)/2, fill, state.penColor, math.Max(0.35, state.penWidth))
			}
		case emrArc:
			if len(record) >= 40 {
				x1, y1 := emfPoint(state, int32(binary.LittleEndian.Uint32(record[8:12])), int32(binary.LittleEndian.Uint32(record[12:16])), width, height)
				x2, y2 := emfPoint(state, int32(binary.LittleEndian.Uint32(record[16:20])), int32(binary.LittleEndian.Uint32(record[20:24])), width, height)
				sx, sy := emfPoint(state, int32(binary.LittleEndian.Uint32(record[24:28])), int32(binary.LittleEndian.Uint32(record[28:32])), width, height)
				ex, ey := emfPoint(state, int32(binary.LittleEndian.Uint32(record[32:36])), int32(binary.LittleEndian.Uint32(record[36:40])), width, height)
				rx, ry := math.Abs(x2-x1)/2, math.Abs(y2-y1)/2
				if rx > 0 && ry > 0 {
					cx, cy := (x1+x2)/2, (y1+y2)/2
					a1 := math.Atan2((sy-cy)/ry, (sx-cx)/rx)
					a2 := math.Atan2((ey-cy)/ry, (ex-cx)/rx)
					delta := a2 - a1
					for delta <= 0 {
						delta += 2 * math.Pi
					}
					large := 0
					if delta > math.Pi {
						large = 1
					}
					fmt.Fprintf(&body, `<path d="M %.3f %.3f A %.3f %.3f 0 %d 1 %.3f %.3f" fill="none" stroke="%s" stroke-width="%.3f"/>`, sx, sy, rx, ry, large, ex, ey, state.penColor, math.Max(0.35, state.penWidth))
				}
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
	// EMF drawing coordinates after window/viewport mapping live in device space.
	// rclBounds may have a non-zero origin (Office WMFC images frequently do), so
	// a 0-based viewBox clips the right/bottom and leaves blank space at left/top.
	// Preserve the authoritative device-space origin in the SVG viewBox.
	fmt.Fprintf(&svg, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="%.3f %.3f %.3f %.3f" width="%.3f" height="%.3f" preserveAspectRatio="none">`, float64(left), float64(top), width, height, width, height)
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
	// Coordinates in MM_ANISOTROPIC EMFs are logical coordinates too. They must
	// pass through the window -> viewport transform just like other scalable map
	// modes. In particular, Office WMFC payloads commonly use logical extents that
	// map almost exactly onto rclBounds; treating the records as device coordinates
	// collapses the drawing toward the header origin.
	// GDI maps logical window coordinates into viewport/device coordinates.
	// Office-generated EMFs commonly use a large logical window (for example
	// 0.01-mm units) and a much smaller viewport matching the EMF frame. Using
	// the header bounds directly as the window scale shrinks such drawings into
	// the top-left corner (notably WMFC-embedded EMFs in PPTX WMF pictures).
	vx, vy := state.viewportExtX, state.viewportExtY
	if vx == 0 {
		vx = width
	}
	if vy == 0 {
		vy = height
	}
	sx, sy := 1.0, 1.0
	if state.windowExtX != 0 {
		sx = vx / state.windowExtX
	}
	if state.windowExtY != 0 {
		sy = vy / state.windowExtY
	}
	return state.viewportOrgX + (float64(x)-state.windowOrgX)*sx,
		state.viewportOrgY + (float64(y)-state.windowOrgY)*sy
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

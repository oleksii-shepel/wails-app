package stream

import (
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"

	officeartir "office/pkg/officeart/ir"
	officeartdml "office/pkg/officeart/ooxml/drawingml"
	dml "office/pkg/ooxml/drawing"
	wordir "office/pkg/word/ir"
	wordsax "office/pkg/word/parsers/ooxml/sax"
)

func docxDrawingCarriesShape(raw wordsax.Event) bool {
	if raw.DrawingGraphicKind == "shape" {
		return true
	}
	return raw.DrawingHasTextBox || strings.Contains(raw.RawXML, "txbxContent")
}

func docxShapeFromDrawing(raw wordsax.Event) (*wordir.Shape, error) {
	if raw.RawXML == "" || !docxDrawingCarriesShape(raw) {
		return nil, nil
	}

	shape := &wordir.Shape{
		Floating: raw.DrawingKind == "anchor",
		Width:    raw.WidthEMU,
		Height:   raw.HeightEMU,
	}
	var fillColor, strokeColor string
	var strokeWidthEMU int64
	var preset string
	var vmlX, vmlY int64
	var vmlFloating bool
	var vmlShape bool
	var inShapeProps, inLine, inSolidFill bool
	var fillTarget string
	var shapePropsDepth, lineDepth, solidDepth int
	depth := 0

	dec := xml.NewDecoder(strings.NewReader(raw.RawXML))
	for {
		tok, err := dec.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			switch t.Name.Local {
			case "shape", "rect", "roundrect", "oval":
				if t.Name.Space == "urn:schemas-microsoft-com:vml" {
					vmlShape = true
					style := xmlAttr(t, "style")
					shape.Width, shape.Height = parseDOCXVMLStyleExtent(style)
					vmlX, vmlY, vmlFloating = parseDOCXVMLStylePosition(style)
					fillColor = normalizeDOCXVMLColor(xmlAttr(t, "fillcolor"))
					strokeColor = normalizeDOCXVMLColor(xmlAttr(t, "strokecolor"))
					strokeWidthEMU = parseDOCXVMLDimensionEMU(xmlAttr(t, "strokeweight"))
					switch t.Name.Local {
					case "roundrect":
						preset = "roundRect"
					case "oval":
						preset = "ellipse"
					default:
						preset = "rect"
					}
				}
			case "spPr":
				if !inShapeProps {
					inShapeProps = true
					shapePropsDepth = depth
				}
			case "xfrm":
				if inShapeProps {
					if rot := xmlAttr(t, "rot"); rot != "" {
						if v, err := strconv.ParseFloat(rot, 64); err == nil {
							shape.RotationDeg = v / 60000.0
						}
					}
					shape.FlipH = xmlOnOff(xmlAttr(t, "flipH"))
					shape.FlipV = xmlOnOff(xmlAttr(t, "flipV"))
				}
			case "prstGeom":
				if inShapeProps {
					preset = firstNonEmptyString(xmlAttr(t, "prst"), xmlAttr(t, "val"))
				}
			case "ln":
				if inShapeProps {
					inLine = true
					lineDepth = depth
					strokeWidthEMU = parseInt64Default(xmlAttr(t, "w"), 0)
				}
			case "solidFill":
				if inShapeProps {
					inSolidFill = true
					solidDepth = depth
					if inLine {
						fillTarget = "line"
					} else {
						fillTarget = "shape"
					}
				}
			case "srgbClr":
				if inSolidFill {
					color := strings.ToUpper(strings.TrimSpace(xmlAttr(t, "val")))
					if fillTarget == "line" {
						strokeColor = color
					} else if fillTarget == "shape" {
						fillColor = color
					}
				}
			case "bodyPr":
				applyDOCXTextBoxBodyProps(shape, t)
			case "txbxContent":
				tb, err := parseDOCXTextBoxContent(dec, t)
				if err != nil {
					return nil, err
				}
				if tb != nil {
					if shape.TextBox == nil {
						shape.TextBox = tb
					} else {
						shape.TextBox.Blocks = append(shape.TextBox.Blocks, tb.Blocks...)
					}
				}
				depth--
			}
		case xml.EndElement:
			if inSolidFill && depth == solidDepth && t.Name.Local == "solidFill" {
				inSolidFill = false
				fillTarget = ""
			}
			if inLine && depth == lineDepth && t.Name.Local == "ln" {
				inLine = false
			}
			if inShapeProps && depth == shapePropsDepth && t.Name.Local == "spPr" {
				inShapeProps = false
			}
			depth--
		}
	}

	if preset == "" {
		preset = "rect"
	}
	shape.Kind = preset
	shape.Geometry = &dml.PresetGeometry{Name: preset}
	shape.FillColor = fillColor
	shape.StrokeColor = strokeColor
	shape.StrokeWidth = strokeWidthEMU
	if fillColor != "" {
		shape.Fill = dml.SolidFill{Color: fillColor, Alpha: 1}
	}
	if strokeColor != "" || strokeWidthEMU != 0 {
		line := &dml.LineStyle{Color: strokeColor, Width: float64(strokeWidthEMU) / 12700.0}
		shape.LineStyle = line
	}

	if vmlShape {
		shape.Floating = vmlFloating
		shape.X, shape.Y = vmlX, vmlY
		shape.XRelative, shape.YRelative = "page", "page"
		shape.WrapType = "square"
		if shape.TextBox != nil {
			shape.TextBox.Floating = vmlFloating
			shape.TextBox.X, shape.TextBox.Y = vmlX, vmlY
			shape.TextBox.XRelative, shape.TextBox.YRelative = "page", "page"
			shape.TextBox.Width, shape.TextBox.Height = shape.Width, shape.Height
			shape.TextBox.WrapType = "square"
		}
	} else {
		applyAnchoredShapePosition(shape, raw)
	}
	if shape.TextBox != nil {
		shape.DrawingPlacement().ApplyToTextBox(shape.TextBox)
		shape.TextBox.FillColor = fillColor
		shape.TextBox.BorderColor = strokeColor
		shape.TextBox.BorderWidth = float64(strokeWidthEMU) / 12700.0
	}

	kind := officeartir.ShapeAuto
	if shape.TextBox != nil && preset == "rect" && fillColor == "" && strokeColor == "" {
		kind = officeartir.ShapeTextBox
	}
	shape.OfficeArt = officeartdml.Shape(officeartdml.ShapeSpec{
		Name:        firstNonEmptyString(raw.DrawingName, raw.DrawingDescription),
		Kind:        kind,
		Bounds:      officeartir.Rect{X: shape.X, Y: shape.Y, W: shape.Width, H: shape.Height},
		RotationDeg: shape.RotationDeg,
		FlipH:       shape.FlipH,
		FlipV:       shape.FlipV,
		Preset:      shape.Geometry,
		Fill:        shape.Fill,
		Line:        shape.LineStyle,
		Source: officeartir.SourceRef{
			Format: "docx",
			Part:   raw.PartName,
		},
	})
	return shape, nil
}

func applyAnchoredShapePosition(shape *wordir.Shape, raw wordsax.Event) {
	if shape == nil || raw.DrawingModel == nil || raw.DrawingModel.Anchor == nil {
		return
	}
	anchor := raw.DrawingModel.Anchor
	placement := drawingPlacementFromAnchor(anchor)
	placement.Width = shape.Width
	placement.Height = shape.Height
	placement.RotationDeg = shape.RotationDeg
	placement.FlipH = shape.FlipH
	placement.FlipV = shape.FlipV
	if anchor.PositionH != nil {
		placement.X = anchor.PositionH.PosOffset
		placement.XRelative = anchor.PositionH.RelativeFrom
		placement.XAlign = anchor.PositionH.Align
		if anchor.PositionH.Pct != 0 {
			placement.X = anchor.PositionH.Pct
		}
	}
	if anchor.PositionV != nil {
		placement.Y = anchor.PositionV.PosOffset
		placement.YRelative = anchor.PositionV.RelativeFrom
		placement.YAlign = anchor.PositionV.Align
		if anchor.PositionV.Pct != 0 {
			placement.Y = anchor.PositionV.Pct
		}
	}
	placement.ApplyTo(shape)
}

func applyDOCXTextBoxBodyProps(shape *wordir.Shape, start xml.StartElement) {
	if shape == nil {
		return
	}
	if shape.TextBox == nil {
		shape.TextBox = &wordir.TextBox{}
	}
	tb := shape.TextBox
	tb.Vert = xmlAttr(start, "vert")
	tb.NoWrap = strings.EqualFold(xmlAttr(start, "wrap"), "none")
	tb.Anchor = xmlAttr(start, "anchor")
	tb.AnchorCenter = xmlOnOff(xmlAttr(start, "anchorCtr"))
	if v := parseInt64Default(xmlAttr(start, "lIns"), -1); v >= 0 {
		tb.PaddingLeft = float64(v) / 12700.0
	}
	if v := parseInt64Default(xmlAttr(start, "tIns"), -1); v >= 0 {
		tb.PaddingTop = float64(v) / 12700.0
	}
	if v := parseInt64Default(xmlAttr(start, "rIns"), -1); v >= 0 {
		tb.PaddingRight = float64(v) / 12700.0
	}
	if v := parseInt64Default(xmlAttr(start, "bIns"), -1); v >= 0 {
		tb.PaddingBottom = float64(v) / 12700.0
	}
}

func parseDOCXTextBoxContent(dec *xml.Decoder, end xml.StartElement) (*wordir.TextBox, error) {
	tb := &wordir.TextBox{}
	for {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "p" {
				p, err := parseDOCXTextBoxParagraph(dec, t)
				if err != nil {
					return nil, err
				}
				tb.Blocks = append(tb.Blocks, wordir.Block{Kind: "paragraph", Paragraph: p})
			} else if err := dec.Skip(); err != nil {
				return nil, err
			}
		case xml.EndElement:
			if t.Name == end.Name {
				if len(tb.Blocks) == 0 {
					return nil, nil
				}
				return tb, nil
			}
		}
	}
}

func parseDOCXTextBoxParagraph(dec *xml.Decoder, end xml.StartElement) (*wordir.Paragraph, error) {
	p := &wordir.Paragraph{}
	for {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "pPr":
				props, _, err := scanDOCXParagraphProperties(dec, t)
				if err != nil {
					return nil, err
				}
				p.StyleID = props.StyleID
				p.Format = paragraphFormatFromSAX(props)
				if props.NumberingID != nil {
					level := 0
					if props.NumberingLevel != nil {
						level = *props.NumberingLevel
					}
					p.Numbering = &wordir.Numbering{NumID: *props.NumberingID, Level: level}
				}
			case "r":
				runs, text, err := parseDOCXTextBoxRun(dec, t)
				if err != nil {
					return nil, err
				}
				p.Runs = append(p.Runs, runs...)
				p.Text += text
			case "hyperlink", "smartTag", "sdt":
				// Text boxes frequently wrap contact data and other visible text in
				// w:hyperlink (and occasionally compatibility containers). The old
				// parser skipped the whole wrapper, leaving an empty positioned box.
				runs, text, err := parseDOCXTextBoxRunContainer(dec, t)
				if err != nil {
					return nil, err
				}
				p.Runs = append(p.Runs, runs...)
				p.Text += text
			default:
				if err := dec.Skip(); err != nil {
					return nil, err
				}
			}
		case xml.EndElement:
			if t.Name == end.Name {
				return p, nil
			}
		}
	}
}

// parseDOCXTextBoxRunContainer preserves visible runs nested in wrappers such
// as w:hyperlink. It intentionally ignores the wrapper metadata here; the
// textbox's visible content must not disappear merely because it is linked.
func parseDOCXTextBoxRunContainer(dec *xml.Decoder, end xml.StartElement) ([]wordir.Run, string, error) {
	var out []wordir.Run
	var text strings.Builder
	for {
		tok, err := dec.Token()
		if err != nil {
			return nil, "", err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "r":
				runs, value, err := parseDOCXTextBoxRun(dec, t)
				if err != nil {
					return nil, "", err
				}
				out = append(out, runs...)
				text.WriteString(value)
			case "hyperlink", "smartTag", "sdt", "sdtContent":
				runs, value, err := parseDOCXTextBoxRunContainer(dec, t)
				if err != nil {
					return nil, "", err
				}
				out = append(out, runs...)
				text.WriteString(value)
			default:
				if err := dec.Skip(); err != nil {
					return nil, "", err
				}
			}
		case xml.EndElement:
			if t.Name == end.Name {
				return out, text.String(), nil
			}
		}
	}
}

func parseDOCXTextBoxRun(dec *xml.Decoder, end xml.StartElement) ([]wordir.Run, string, error) {
	var props wordsax.RunProperties
	var out []wordir.Run
	var text strings.Builder
	for {
		tok, err := dec.Token()
		if err != nil {
			return nil, "", err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "rPr":
				p, err := scanDOCXRunProperties(dec, t)
				if err != nil {
					return nil, "", err
				}
				props = p
			case "t", "delText":
				var value string
				if err := dec.DecodeElement(&value, &t); err != nil {
					return nil, "", err
				}
				if value != "" {
					run := wordir.Run{Kind: "run", Text: value, StyleID: props.StyleID, Format: runFormatFromSAX(props)}
					out = append(out, run)
					text.WriteString(value)
				}
			case "tab":
				out = append(out, wordir.Run{Kind: "tab", StyleID: props.StyleID, Format: runFormatFromSAX(props)})
				text.WriteByte('\t')
				if err := dec.Skip(); err != nil {
					return nil, "", err
				}
			case "br", "cr":
				out = append(out, wordir.Run{Kind: "break", StyleID: props.StyleID, Format: runFormatFromSAX(props)})
				text.WriteByte('\n')
				if err := dec.Skip(); err != nil {
					return nil, "", err
				}
			default:
				if err := dec.Skip(); err != nil {
					return nil, "", err
				}
			}
		case xml.EndElement:
			if t.Name == end.Name {
				return out, text.String(), nil
			}
		}
	}
}

func parseDOCXVMLStyleExtent(style string) (int64, int64) {
	props := parseDOCXVMLStyle(style)
	return parseDOCXVMLDimensionEMU(props["width"]), parseDOCXVMLDimensionEMU(props["height"])
}

func parseDOCXVMLStylePosition(style string) (int64, int64, bool) {
	props := parseDOCXVMLStyle(style)
	x := parseDOCXVMLDimensionEMU(props["margin-left"])
	y := parseDOCXVMLDimensionEMU(props["margin-top"])
	position := strings.ToLower(strings.TrimSpace(props["position"]))
	return x, y, position == "absolute" || position == "relative"
}

func parseDOCXVMLStyle(style string) map[string]string {
	out := map[string]string{}
	for _, item := range strings.Split(style, ";") {
		key, value, ok := strings.Cut(item, ":")
		if ok {
			out[strings.ToLower(strings.TrimSpace(key))] = strings.TrimSpace(value)
		}
	}
	return out
}

func parseDOCXVMLDimensionEMU(value string) int64 {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return 0
	}
	scale := 12700.0
	switch {
	case strings.HasSuffix(value, "pt"):
		value = strings.TrimSpace(strings.TrimSuffix(value, "pt"))
	case strings.HasSuffix(value, "in"):
		value = strings.TrimSpace(strings.TrimSuffix(value, "in")); scale = 914400
	case strings.HasSuffix(value, "cm"):
		value = strings.TrimSpace(strings.TrimSuffix(value, "cm")); scale = 360000
	case strings.HasSuffix(value, "mm"):
		value = strings.TrimSpace(strings.TrimSuffix(value, "mm")); scale = 36000
	case strings.HasSuffix(value, "px"):
		value = strings.TrimSpace(strings.TrimSuffix(value, "px")); scale = 9525
	}
	v, err := strconv.ParseFloat(value, 64)
	if err != nil { return 0 }
	return int64(v * scale)
}

func normalizeDOCXVMLColor(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "#")
	if len(value) == 6 { return strings.ToUpper(value) }
	return ""
}

func appendDOCXShapeRun(session *LayoutSession, event Event) error {
	if session == nil {
		return nil
	}
	raw, ok := event.Raw.(wordsax.Event)
	if !ok {
		return StreamUnsupportedEventError{Type: event.Type}
	}
	shape, err := docxShapeFromDrawing(raw)
	if err != nil {
		return fmt.Errorf("word stream: decode shape: %w", err)
	}
	if shape == nil {
		return StreamUnsupportedEventError{Type: event.Type}
	}
	resolveDOCXShapeTextBoxSemantics(session, shape)
	session.ensureParagraph()
	run := wordir.Run{Kind: "shape", Shape: shape, StyleID: event.RunStyleID}
	if event.RunFormat != nil {
		run.Format = *event.RunFormat
	}
	session.paragraph.Runs = append(session.paragraph.Runs, run)
	return nil
}


func resolveDOCXShapeTextBoxSemantics(session *LayoutSession, shape *wordir.Shape) {
	if session == nil || shape == nil || shape.TextBox == nil || session.metadata == nil {
		return
	}
	for i := range shape.TextBox.Blocks {
		block := &shape.TextBox.Blocks[i]
		if block.Paragraph == nil {
			continue
		}
		p := block.Paragraph
		var direct *wordir.NumberingReference
		if p.Numbering != nil {
			direct = &wordir.NumberingReference{NumID: p.Numbering.NumID, Level: p.Numbering.Level}
		}
		numbering, list := session.metadata.ResolveParagraphNumbering(p.StyleID, direct)
		p.Numbering = numbering
		p.List = list
	}
}

func xmlAttr(start xml.StartElement, local string) string {
	for _, attr := range start.Attr {
		if attr.Name.Local == local {
			return attr.Value
		}
	}
	return ""
}

func xmlOnOff(value string) bool {
	value = strings.TrimSpace(strings.ToLower(value))
	return value == "1" || value == "true" || value == "on"
}

func parseInt64Default(value string, fallback int64) int64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

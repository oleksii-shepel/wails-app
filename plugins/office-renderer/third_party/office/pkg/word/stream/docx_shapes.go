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

func docxShapeFromDrawing(raw wordsax.Event) (*wordir.Shape, error) {
	if raw.DrawingGraphicKind != "shape" || raw.RawXML == "" {
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

	applyAnchoredShapePosition(shape, raw)
	if shape.TextBox != nil {
		shape.TextBox.Floating = shape.Floating
		shape.TextBox.X = shape.X
		shape.TextBox.Y = shape.Y
		shape.TextBox.Width = shape.Width
		shape.TextBox.Height = shape.Height
		shape.TextBox.XRelative = shape.XRelative
		shape.TextBox.YRelative = shape.YRelative
		shape.TextBox.XAlign = shape.XAlign
		shape.TextBox.YAlign = shape.YAlign
		shape.TextBox.ZIndex = shape.ZIndex
		shape.TextBox.BehindDoc = shape.BehindDoc
		shape.TextBox.WrapType = shape.WrapType
		shape.TextBox.WrapSide = shape.WrapSide
		shape.TextBox.DistTop = shape.DistTop
		shape.TextBox.DistBottom = shape.DistBottom
		shape.TextBox.DistLeft = shape.DistLeft
		shape.TextBox.DistRight = shape.DistRight
		shape.TextBox.FillColor = fillColor
		shape.TextBox.BorderColor = strokeColor
		shape.TextBox.BorderWidth = float64(strokeWidthEMU) / 12700.0
		shape.TextBox.RotationDeg = shape.RotationDeg
		shape.TextBox.FlipH = shape.FlipH
		shape.TextBox.FlipV = shape.FlipV
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
	shape.Floating = true
	shape.ZIndex = anchor.RelativeHeight
	shape.BehindDoc = anchor.BehindDoc != 0
	shape.Locked = anchor.Locked
	shape.LayoutInCell = anchor.LayoutInCell
	shape.AllowOverlap = anchor.AllowOverlap
	if anchor.PositionH != nil {
		shape.X = anchor.PositionH.PosOffset
		shape.XRelative = anchor.PositionH.RelativeFrom
		shape.XAlign = anchor.PositionH.Align
		if anchor.PositionH.Pct != 0 {
			shape.X = anchor.PositionH.Pct
		}
	}
	if anchor.PositionV != nil {
		shape.Y = anchor.PositionV.PosOffset
		shape.YRelative = anchor.PositionV.RelativeFrom
		shape.YAlign = anchor.PositionV.Align
		if anchor.PositionV.Pct != 0 {
			shape.Y = anchor.PositionV.Pct
		}
	}
	if anchor.WrapNone != nil {
		shape.WrapType = "none"
	} else if anchor.WrapSquare != nil {
		shape.WrapType = "square"
		shape.WrapSide = anchor.WrapSquare.WrapText
	} else if anchor.WrapTight != nil {
		shape.WrapType = "tight"
		shape.WrapSide = anchor.WrapTight.WrapText
	} else if anchor.WrapThrough != nil {
		shape.WrapType = "through"
		shape.WrapSide = anchor.WrapThrough.WrapText
	} else if anchor.WrapTopAndBottom != nil {
		shape.WrapType = "topAndBottom"
	}
	shape.DistTop = float64(anchor.DistT) / 12700.0
	shape.DistBottom = float64(anchor.DistB) / 12700.0
	shape.DistLeft = float64(anchor.DistL) / 12700.0
	shape.DistRight = float64(anchor.DistR) / 12700.0
	if anchor.SimplePosAttr && anchor.SimplePos != nil {
		shape.SimplePosAttr = true
		shape.SimplePosX = anchor.SimplePos.X
		shape.SimplePosY = anchor.SimplePos.Y
	}
	if anchor.EffectExtent != nil {
		shape.EffectExtentL = anchor.EffectExtent.L
		shape.EffectExtentT = anchor.EffectExtent.T
		shape.EffectExtentR = anchor.EffectExtent.R
		shape.EffectExtentB = anchor.EffectExtent.B
	}
	if anchor.SizeRelH != nil {
		shape.SizeRelHPct = anchor.SizeRelH.Pct
		shape.SizeRelHFrom = anchor.SizeRelH.RelativeFrom
	}
	if anchor.SizeRelV != nil {
		shape.SizeRelVPct = anchor.SizeRelV.Pct
		shape.SizeRelVFrom = anchor.SizeRelV.RelativeFrom
	}
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
			case "r":
				runs, text, err := parseDOCXTextBoxRun(dec, t)
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
				breakType := xmlAttr(t, "type")
				if breakType == "" {
					breakType = "textWrapping"
				}
				out = append(out, wordir.Run{Kind: "break", BreakType: breakType, StyleID: props.StyleID, Format: runFormatFromSAX(props)})
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

func xmlAttr(start xml.StartElement, local string) string {
	for _, attr := range start.Attr {
		if attr.Name.Local == local {
			return attr.Value
		}
	}
	return ""
}

func xmlOnOff(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "t", "on", "yes":
		return true
	default:
		return false
	}
}

func parseInt64Default(value string, fallback int64) int64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	v, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fallback
	}
	return v
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
		return fmt.Errorf("word stream: resolve DOCX shape: %w", err)
	}
	if shape == nil {
		return StreamUnsupportedEventError{Type: event.Type}
	}
	session.ensureParagraph()
	run := wordir.Run{Kind: "shape", Shape: shape, StyleID: event.RunStyleID}
	if event.RunFormat != nil {
		run.Format = *event.RunFormat
	}
	session.paragraph.Runs = append(session.paragraph.Runs, run)
	return nil
}

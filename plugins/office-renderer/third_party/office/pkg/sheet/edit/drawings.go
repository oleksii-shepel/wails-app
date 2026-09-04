package edit

import (
	"fmt"
	"strings"

	drawing "office/pkg/ooxml/drawing"
	sheetir "office/pkg/sheet/ir"
)

// Drawings returns worksheet drawing elements in storage order.
func (s *Sheet) Drawings() []*sheetir.Drawing {
	if s == nil || s.sheet == nil {
		return nil
	}
	return append([]*sheetir.Drawing(nil), s.sheet.Drawings...)
}

// AddImage adds an image drawing with an explicit drawing position.
func (s *Sheet) AddImage(name string, data []byte, contentType, altText string, position sheetir.DrawingPosition) (*sheetir.Drawing, error) {
	if s == nil || s.sheet == nil {
		return nil, fmt.Errorf("sheet edit: nil sheet")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = s.nextDrawingName("Image")
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("sheet edit: image data is empty")
	}
	if contentType == "" {
		contentType = "image/png"
	}
	if err := validateDrawingPosition(position); err != nil {
		return nil, err
	}
	drawing := &sheetir.Drawing{
		Name:     name,
		Type:     sheetir.DrawingTypeImage,
		Position: position,
		Image: &sheetir.DrawingImage{
			Data:        append([]byte(nil), data...),
			ContentType: contentType,
			AltText:     altText,
		},
	}
	s.sheet.Drawings = append(s.sheet.Drawings, drawing)
	return drawing, nil
}

// AddImageOneCell adds an image anchored to a single cell.
func (s *Sheet) AddImageOneCell(name string, data []byte, contentType, altText string, row, col int, widthEMU, heightEMU int64) (*sheetir.Drawing, error) {
	return s.AddImage(name, data, contentType, altText, sheetir.DrawingPosition{
		Anchor:   sheetir.AnchorOneCell,
		FromCell: sheetir.DrawingCellAnchor{Row: row, Col: col},
		Width:    widthEMU,
		Height:   heightEMU,
	})
}

// AddImageTwoCell adds an image anchored between two cells.
func (s *Sheet) AddImageTwoCell(name string, data []byte, contentType, altText string, fromRow, fromCol, toRow, toCol int) (*sheetir.Drawing, error) {
	return s.AddImage(name, data, contentType, altText, sheetir.DrawingPosition{
		Anchor:   sheetir.AnchorTwoCell,
		FromCell: sheetir.DrawingCellAnchor{Row: fromRow, Col: fromCol},
		ToCell:   sheetir.DrawingCellAnchor{Row: toRow, Col: toCol},
	})
}

// AddImageAbsolute adds an image using absolute EMU coordinates.
func (s *Sheet) AddImageAbsolute(name string, data []byte, contentType, altText string, xEMU, yEMU, widthEMU, heightEMU int64) (*sheetir.Drawing, error) {
	return s.AddImage(name, data, contentType, altText, sheetir.DrawingPosition{
		Anchor: sheetir.AnchorAbsolute,
		X:      xEMU,
		Y:      yEMU,
		Width:  widthEMU,
		Height: heightEMU,
	})
}

// AddShape adds a simple DrawingML shape.
func (s *Sheet) AddShape(name, geometry, text, fillColor, lineColor string, lineWidthPt float64, position sheetir.DrawingPosition) (*sheetir.Drawing, error) {
	if s == nil || s.sheet == nil {
		return nil, fmt.Errorf("sheet edit: nil sheet")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = s.nextDrawingName("Shape")
	}
	if geometry == "" {
		geometry = "rect"
	}
	if err := validateDrawingPosition(position); err != nil {
		return nil, err
	}
	drawing := &sheetir.Drawing{
		Name:     name,
		Type:     sheetir.DrawingTypeShape,
		Position: position,
		Shape: &sheetir.DrawingShape{
			Geometry:    geometry,
			Text:        text,
			FillColor:   fillColor,
			LineColor:   lineColor,
			LineWidthPt: lineWidthPt,
		},
	}
	s.sheet.Drawings = append(s.sheet.Drawings, drawing)
	return drawing, nil
}

// AddRichTextShape adds a DrawingML shape with formatted text runs.
func (s *Sheet) AddRichTextShape(name, geometry, fillColor, lineColor string, lineWidthPt float64, runs []*sheetir.RichTextRun, position sheetir.DrawingPosition) (*sheetir.Drawing, error) {
	drawing, err := s.AddShape(name, geometry, plainTextFromRuns(runs), fillColor, lineColor, lineWidthPt, position)
	if err != nil {
		return nil, err
	}
	drawing.Shape.RichTextRuns = cloneRichTextRuns(runs)
	return drawing, nil
}

// SetShapeText replaces a shape's plain text.
func (s *Sheet) SetShapeText(name, text string) error {
	drawing, ok := s.Drawing(name)
	if !ok || drawing.Shape == nil {
		return fmt.Errorf("sheet edit: shape %q not found", name)
	}
	drawing.Shape.Text = text
	drawing.Shape.RichTextRuns = nil
	return nil
}

// SetShapeRichText replaces a shape's formatted text runs.
func (s *Sheet) SetShapeRichText(name string, runs []*sheetir.RichTextRun) error {
	drawing, ok := s.Drawing(name)
	if !ok || drawing.Shape == nil {
		return fmt.Errorf("sheet edit: shape %q not found", name)
	}
	drawing.Shape.Text = plainTextFromRuns(runs)
	drawing.Shape.RichTextRuns = cloneRichTextRuns(runs)
	return nil
}

// AddConnector adds a connector/line drawing between two cell anchors.
func (s *Sheet) AddConnector(name string, fromRow, fromCol, toRow, toCol int, lineColor string, lineWidthPt float64) (*sheetir.Drawing, error) {
	drawing, err := s.AddShape(name, "line", "", "", lineColor, lineWidthPt, sheetir.DrawingPosition{
		Anchor:   sheetir.AnchorTwoCell,
		FromCell: sheetir.DrawingCellAnchor{Row: fromRow, Col: fromCol},
		ToCell:   sheetir.DrawingCellAnchor{Row: toRow, Col: toCol},
	})
	if err != nil {
		return nil, err
	}
	drawing.Type = sheetir.DrawingTypeConnector
	drawing.Shape.ConnectorType = "straightConnector1"
	return drawing, nil
}

// SetConnectorType updates the preset geometry used for a connector drawing.
func (s *Sheet) SetConnectorType(name, connectorType string) error {
	drawing, err := s.connectorDrawing(name)
	if err != nil {
		return err
	}
	connectorType = strings.TrimSpace(connectorType)
	if connectorType == "" {
		connectorType = "straightConnector1"
	}
	drawing.Shape.ConnectorType = connectorType
	return nil
}

// SetConnectorLineStyle updates the shared DrawingML line style for a connector drawing.
func (s *Sheet) SetConnectorLineStyle(name string, style *drawing.LineStyle) error {
	drawing, err := s.connectorDrawing(name)
	if err != nil {
		return err
	}
	drawing.Shape.LineStyle = cloneDrawingLineStyle(style)
	if drawing.Shape.LineStyle != nil {
		if drawing.Shape.LineStyle.Color != "" {
			drawing.Shape.LineColor = drawing.Shape.LineStyle.Color
		}
		if drawing.Shape.LineStyle.Width > 0 {
			drawing.Shape.LineWidthPt = drawing.Shape.LineStyle.Width
		}
	}
	return nil
}

// Drawing returns a worksheet drawing by name.
func (s *Sheet) Drawing(name string) (*sheetir.Drawing, bool) {
	if s == nil || s.sheet == nil {
		return nil, false
	}
	name = strings.TrimSpace(name)
	for _, drawing := range s.sheet.Drawings {
		if drawing != nil && strings.EqualFold(drawing.Name, name) {
			return drawing, true
		}
	}
	return nil, false
}

// RemoveDrawing removes a drawing by name.
func (s *Sheet) RemoveDrawing(name string) bool {
	if s == nil || s.sheet == nil {
		return false
	}
	name = strings.TrimSpace(name)
	for i, drawing := range s.sheet.Drawings {
		if strings.EqualFold(drawing.Name, name) {
			s.sheet.Drawings = append(s.sheet.Drawings[:i], s.sheet.Drawings[i+1:]...)
			return true
		}
	}
	return false
}

func (s *Sheet) connectorDrawing(name string) (*sheetir.Drawing, error) {
	drawing, ok := s.Drawing(name)
	if !ok || drawing == nil || drawing.Type != sheetir.DrawingTypeConnector || drawing.Shape == nil {
		return nil, fmt.Errorf("sheet edit: connector %q not found", name)
	}
	return drawing, nil
}

func (s *Sheet) nextDrawingName(prefix string) string {
	for i := 1; ; i++ {
		name := fmt.Sprintf("%s %d", prefix, i)
		found := false
		for _, drawing := range s.sheet.Drawings {
			if strings.EqualFold(drawing.Name, name) {
				found = true
				break
			}
		}
		if !found {
			return name
		}
	}
}

func validateDrawingPosition(position sheetir.DrawingPosition) error {
	switch position.Anchor {
	case sheetir.AnchorOneCell:
		if position.FromCell.Row < 1 || position.FromCell.Col < 1 {
			return fmt.Errorf("sheet edit: one-cell drawing anchor must have row and column >= 1")
		}
		if position.Width <= 0 || position.Height <= 0 {
			return fmt.Errorf("sheet edit: one-cell drawing dimensions must be > 0")
		}
	case sheetir.AnchorTwoCell:
		if position.FromCell.Row < 1 || position.FromCell.Col < 1 || position.ToCell.Row < 1 || position.ToCell.Col < 1 {
			return fmt.Errorf("sheet edit: two-cell drawing anchors must have row and column >= 1")
		}
	case sheetir.AnchorAbsolute:
		if position.Width <= 0 || position.Height <= 0 {
			return fmt.Errorf("sheet edit: absolute drawing dimensions must be > 0")
		}
	default:
		return fmt.Errorf("sheet edit: unsupported drawing anchor %q", position.Anchor)
	}
	return nil
}

func cloneDrawingLineStyle(style *drawing.LineStyle) *drawing.LineStyle {
	if style == nil {
		return nil
	}
	clone := *style
	if style.HeadEnd != nil {
		head := *style.HeadEnd
		clone.HeadEnd = &head
	}
	if style.TailEnd != nil {
		tail := *style.TailEnd
		clone.TailEnd = &tail
	}
	if len(style.CompoundWidths) > 0 {
		clone.CompoundWidths = append([]float64(nil), style.CompoundWidths...)
	}
	return &clone
}

// SmartArtDrawings returns all SmartArt drawing objects on the worksheet.
func (s *Sheet) SmartArtDrawings() []*sheetir.Drawing {
	if s == nil || s.sheet == nil {
		return nil
	}
	var out []*sheetir.Drawing
	for _, d := range s.sheet.Drawings {
		if d != nil && d.Type == sheetir.DrawingTypeSmartArt {
			out = append(out, d)
		}
	}
	return out
}

// OLEObjects returns all embedded or linked OLE drawing objects on the worksheet.
func (s *Sheet) OLEObjects() []*sheetir.Drawing {
	if s == nil || s.sheet == nil {
		return nil
	}
	var out []*sheetir.Drawing
	for _, d := range s.sheet.Drawings {
		if d != nil && d.Type == sheetir.DrawingTypeOLE {
			out = append(out, d)
		}
	}
	return out
}

// AddOLEObject adds an embedded OLE object to the worksheet semantic model.
func (s *Sheet) AddOLEObject(name, progID string, payload []byte, contentType string, position sheetir.DrawingPosition) (*sheetir.Drawing, error) {
	if s == nil || s.sheet == nil {
		return nil, fmt.Errorf("sheet edit: nil sheet")
	}
	if len(payload) == 0 {
		return nil, fmt.Errorf("sheet edit: OLE payload is empty")
	}
	if err := validateDrawingPosition(position); err != nil {
		return nil, err
	}
	if strings.TrimSpace(name) == "" {
		name = s.nextDrawingName("Object")
	}
	if contentType == "" {
		contentType = "application/vnd.openxmlformats-officedocument.oleObject"
	}
	d := &sheetir.Drawing{Name: name, Type: sheetir.DrawingTypeOLE, Position: position, OLE: &sheetir.DrawingOLE{ProgID: progID, Data: append([]byte(nil), payload...), ContentType: contentType}}
	s.sheet.Drawings = append(s.sheet.Drawings, d)
	return d, nil
}

// AddLinkedOLEObject adds a linked OLE object without embedding a payload.
func (s *Sheet) AddLinkedOLEObject(name, progID, target string, position sheetir.DrawingPosition) (*sheetir.Drawing, error) {
	if s == nil || s.sheet == nil {
		return nil, fmt.Errorf("sheet edit: nil sheet")
	}
	if strings.TrimSpace(target) == "" {
		return nil, fmt.Errorf("sheet edit: linked OLE target is empty")
	}
	if err := validateDrawingPosition(position); err != nil {
		return nil, err
	}
	if strings.TrimSpace(name) == "" {
		name = s.nextDrawingName("Object")
	}
	d := &sheetir.Drawing{Name: name, Type: sheetir.DrawingTypeOLE, Position: position, OLE: &sheetir.DrawingOLE{ProgID: progID, Link: target, External: true}}
	s.sheet.Drawings = append(s.sheet.Drawings, d)
	return d, nil
}

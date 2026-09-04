package edit

import (
	"fmt"
	"strings"

	drawing "office/pkg/ooxml/drawing"
	sheetir "office/pkg/sheet/ir"
)

// DrawingGroup wraps a grouped worksheet drawing for editing.
type DrawingGroup struct {
	drawing *sheetir.Drawing
	group   *sheetir.DrawingGroup
}

// AddGroup adds a grouped worksheet drawing anchored to the sheet.
func (s *Sheet) AddGroup(name string, position sheetir.DrawingPosition) (*DrawingGroup, error) {
	if s == nil || s.sheet == nil {
		return nil, fmt.Errorf("sheet edit: nil sheet")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = s.nextDrawingName("Group")
	}
	if err := validateDrawingPosition(position); err != nil {
		return nil, err
	}
	if position.Width <= 0 || position.Height <= 0 {
		return nil, fmt.Errorf("sheet edit: grouped drawings require explicit width and height")
	}
	drawing := &sheetir.Drawing{
		Name:     name,
		Type:     sheetir.DrawingTypeGroup,
		Position: position,
		Group:    &sheetir.DrawingGroup{},
	}
	s.sheet.Drawings = append(s.sheet.Drawings, drawing)
	return &DrawingGroup{drawing: drawing, group: drawing.Group}, nil
}

// Group returns a top-level grouped drawing by name.
func (s *Sheet) Group(name string) (*DrawingGroup, bool) {
	drawing, ok := s.Drawing(name)
	if !ok || drawing == nil || drawing.Type != sheetir.DrawingTypeGroup || drawing.Group == nil {
		return nil, false
	}
	return &DrawingGroup{drawing: drawing, group: drawing.Group}, true
}

// Drawing returns a direct child drawing by name.
func (g *DrawingGroup) Drawing(name string) (*sheetir.Drawing, bool) {
	if g == nil || g.group == nil {
		return nil, false
	}
	name = strings.TrimSpace(name)
	for _, drawing := range g.group.Children {
		if drawing != nil && strings.EqualFold(drawing.Name, name) {
			return drawing, true
		}
	}
	return nil, false
}

// Drawings returns the direct child drawings in storage order.
func (g *DrawingGroup) Drawings() []*sheetir.Drawing {
	if g == nil || g.group == nil {
		return nil
	}
	return append([]*sheetir.Drawing(nil), g.group.Children...)
}

// AddShape appends a simple grouped shape positioned relative to the group bounds.
func (g *DrawingGroup) AddShape(name, geometry, text, fillColor, lineColor string, lineWidthPt float64, x, y, width, height int64) (*sheetir.Drawing, error) {
	if g == nil || g.group == nil {
		return nil, fmt.Errorf("sheet edit: nil drawing group")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = g.nextChildName("Shape")
	}
	if err := validateGroupChildBounds(width, height); err != nil {
		return nil, err
	}
	if geometry == "" {
		geometry = "rect"
	}
	drawing := &sheetir.Drawing{
		Name: name,
		Type: sheetir.DrawingTypeShape,
		Position: sheetir.DrawingPosition{
			Anchor: sheetir.AnchorAbsolute,
			X:      x,
			Y:      y,
			Width:  width,
			Height: height,
		},
		Shape: &sheetir.DrawingShape{
			Geometry:    geometry,
			Text:        text,
			FillColor:   fillColor,
			LineColor:   lineColor,
			LineWidthPt: lineWidthPt,
		},
	}
	g.group.Children = append(g.group.Children, drawing)
	return drawing, nil
}

// AddRichTextShape appends a grouped shape with formatted text runs.
func (g *DrawingGroup) AddRichTextShape(name, geometry, fillColor, lineColor string, lineWidthPt float64, runs []*sheetir.RichTextRun, x, y, width, height int64) (*sheetir.Drawing, error) {
	drawing, err := g.AddShape(name, geometry, plainTextFromRuns(runs), fillColor, lineColor, lineWidthPt, x, y, width, height)
	if err != nil {
		return nil, err
	}
	drawing.Shape.RichTextRuns = cloneRichTextRuns(runs)
	return drawing, nil
}

// AddConnector appends a grouped connector positioned relative to the group bounds.
func (g *DrawingGroup) AddConnector(name string, x, y, width, height int64, lineColor string, lineWidthPt float64) (*sheetir.Drawing, error) {
	drawing, err := g.AddShape(name, "line", "", "", lineColor, lineWidthPt, x, y, width, height)
	if err != nil {
		return nil, err
	}
	drawing.Type = sheetir.DrawingTypeConnector
	drawing.Shape.ConnectorType = "straightConnector1"
	return drawing, nil
}

// SetConnectorType updates the preset geometry used for a grouped connector.
func (g *DrawingGroup) SetConnectorType(name, connectorType string) error {
	drawing, err := g.connectorDrawing(name)
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

// SetConnectorLineStyle updates the shared DrawingML line style for a grouped connector.
func (g *DrawingGroup) SetConnectorLineStyle(name string, style *drawing.LineStyle) error {
	drawing, err := g.connectorDrawing(name)
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

// AddGroup appends a nested grouped drawing positioned relative to the parent group bounds.
func (g *DrawingGroup) AddGroup(name string, x, y, width, height int64) (*DrawingGroup, error) {
	if g == nil || g.group == nil {
		return nil, fmt.Errorf("sheet edit: nil drawing group")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = g.nextChildName("Group")
	}
	if err := validateGroupChildBounds(width, height); err != nil {
		return nil, err
	}
	drawing := &sheetir.Drawing{
		Name: name,
		Type: sheetir.DrawingTypeGroup,
		Position: sheetir.DrawingPosition{
			Anchor: sheetir.AnchorAbsolute,
			X:      x,
			Y:      y,
			Width:  width,
			Height: height,
		},
		Group: &sheetir.DrawingGroup{},
	}
	g.group.Children = append(g.group.Children, drawing)
	return &DrawingGroup{drawing: drawing, group: drawing.Group}, nil
}

// RemoveDrawing removes a direct child drawing by name.
func (g *DrawingGroup) RemoveDrawing(name string) bool {
	if g == nil || g.group == nil {
		return false
	}
	name = strings.TrimSpace(name)
	for i, drawing := range g.group.Children {
		if drawing != nil && strings.EqualFold(drawing.Name, name) {
			g.group.Children = append(g.group.Children[:i], g.group.Children[i+1:]...)
			return true
		}
	}
	return false
}

func (g *DrawingGroup) connectorDrawing(name string) (*sheetir.Drawing, error) {
	drawing, ok := g.Drawing(name)
	if !ok || drawing == nil || drawing.Type != sheetir.DrawingTypeConnector || drawing.Shape == nil {
		return nil, fmt.Errorf("sheet edit: connector %q not found", name)
	}
	return drawing, nil
}

func (g *DrawingGroup) nextChildName(prefix string) string {
	for i := 1; ; i++ {
		name := fmt.Sprintf("%s %d", prefix, i)
		found := false
		for _, drawing := range g.group.Children {
			if drawing != nil && strings.EqualFold(drawing.Name, name) {
				found = true
				break
			}
		}
		if !found {
			return name
		}
	}
}

func validateGroupChildBounds(width, height int64) error {
	if width <= 0 || height <= 0 {
		return fmt.Errorf("sheet edit: grouped drawing dimensions must be > 0")
	}
	return nil
}

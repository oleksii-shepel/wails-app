package cascade

import (
	"strconv"
	"strings"

	"office/pkg/ooxml/drawing"
	"office/pkg/presentation/ir"
	"office/pkg/presentation/parsers/ooxml/parts"
)

// ApplyShapeGeometry projects DrawingML shape geometry into canonical IR.
// Layout owns the effective visual geometry; renderers should not inspect
// PresentationML preset/custom geometry directly.
func ApplyShapeGeometry(sp *parts.ShapeProperties, shape *ir.Shape) {
	if sp == nil || shape == nil {
		return
	}
	geom := sp.DrawingGeometry()
	if geom == nil {
		return
	}
	if geom.Type == drawing.GeometryTypeCustom && len(geom.Paths) > 0 {
		shape.GeometryData = geom
		shape.Geometry = ""
		return
	}
	if sp.PrstGeom != nil {
		pg := sp.PrstGeom.DrawingPresetGeometry()
		if pg != nil && pg.Name != "" && pg.Name != "rect" {
			shape.Geometry = pg.Name
		}
	}
}

// ConnectorGeometry projects connector preset geometry and authored adjustment
// guides into canonical fields.
func ConnectorGeometry(geom *parts.PresetGeometry) (string, []int) {
	if geom == nil {
		return "", nil
	}
	name := geom.Prst
	if pg := geom.DrawingPresetGeometry(); pg != nil && pg.Name != "" {
		name = pg.Name
	}
	var vals []int
	for _, guide := range geom.AvLst {
		fields := strings.Fields(guide.Fmla)
		if len(fields) != 2 || fields[0] != "val" {
			continue
		}
		n, err := strconv.Atoi(fields[1])
		if err == nil {
			vals = append(vals, n)
		}
	}
	return name, vals
}

// ConnectionPoint resolves the conventional DrawingML connection-site index
// against canonical shape bounds.
func ConnectionPoint(bounds ir.Rect, idx int) ir.Point {
	switch idx {
	case 0:
		return ir.Point{X: bounds.X + bounds.W/2, Y: bounds.Y}
	case 1:
		return ir.Point{X: bounds.X + bounds.W, Y: bounds.Y + bounds.H/2}
	case 2:
		return ir.Point{X: bounds.X + bounds.W/2, Y: bounds.Y + bounds.H}
	case 3:
		return ir.Point{X: bounds.X, Y: bounds.Y + bounds.H/2}
	default:
		return ir.Point{X: bounds.X + bounds.W/2, Y: bounds.Y + bounds.H/2}
	}
}

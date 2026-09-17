// Package ir owns the canonical, format-neutral presentation model.
package ir

import "math"

type VisualSize struct{ W, H int64 }

// VisualTransform describes a visual coordinate system independently of PPT Escher
// and DrawingML. Offset/Extent locate the node in its parent coordinate space.
// ChildOffset/ChildExtent describe the local coordinate domain used by group
// children. Rotation is clockwise degrees.
type VisualTransform struct {
	Offset      Point
	Extent      VisualSize
	ChildOffset Point
	ChildExtent VisualSize
	Rotation    float64
	FlipH       bool
	FlipV       bool
}

// MapVisualRect maps a rectangle from a group's child coordinate system into the
// parent coordinate system, including group reflection and rotation.
func MapVisualRect(child Rect, group VisualTransform) Rect {
	sx, sy := scale(group)
	w := float64(child.W) * sx
	h := float64(child.H) * sy
	cx := float64(group.Offset.X) + (float64(child.X)+float64(child.W)/2-float64(group.ChildOffset.X))*sx
	cy := float64(group.Offset.Y) + (float64(child.Y)+float64(child.H)/2-float64(group.ChildOffset.Y))*sy
	cx, cy = mapCenter(cx, cy, group)
	return Rect{X: int64(math.Round(cx - w/2)), Y: int64(math.Round(cy - h/2)), W: int64(math.Round(w)), H: int64(math.Round(h))}
}

// MapVisualPoint maps a point through the same canonical group coordinate system.
func MapVisualPoint(point *Point, group VisualTransform) *Point {
	if point == nil {
		return nil
	}
	sx, sy := scale(group)
	x := float64(group.Offset.X) + (float64(point.X)-float64(group.ChildOffset.X))*sx
	y := float64(group.Offset.Y) + (float64(point.Y)-float64(group.ChildOffset.Y))*sy
	x, y = mapCenter(x, y, group)
	return &Point{X: int64(math.Round(x)), Y: int64(math.Round(y))}
}

// ApplyVisualTransform materializes one parent group transform into a canonical shape
// tree. It deliberately knows nothing about PPT records or DrawingML xfrm.
func ApplyVisualTransform(shape *Shape, group VisualTransform) {
	if shape == nil {
		return
	}
	shape.Bounds = MapVisualRect(shape.Bounds, group)

	localRotation := shape.RotationDeg
	if group.FlipH != group.FlipV {
		localRotation = -localRotation
	}
	shape.RotationDeg = group.Rotation + localRotation
	shape.FlipH = shape.FlipH != group.FlipH
	shape.FlipV = shape.FlipV != group.FlipV
	shape.ConnectorFlipH = shape.ConnectorFlipH != group.FlipH
	shape.ConnectorFlipV = shape.ConnectorFlipV != group.FlipV
	shape.ConnectorStart = MapVisualPoint(shape.ConnectorStart, group)
	shape.ConnectorEnd = MapVisualPoint(shape.ConnectorEnd, group)
	for i := range shape.ConnectorPath {
		p := shape.ConnectorPath[i]
		if mapped := MapVisualPoint(&p, group); mapped != nil {
			shape.ConnectorPath[i] = *mapped
		}
	}
	for _, child := range shape.Children {
		ApplyVisualTransform(child, group)
	}
}

func scale(group VisualTransform) (float64, float64) {
	sx, sy := 1.0, 1.0
	if group.ChildExtent.W != 0 {
		sx = float64(group.Extent.W) / float64(group.ChildExtent.W)
	}
	if group.ChildExtent.H != 0 {
		sy = float64(group.Extent.H) / float64(group.ChildExtent.H)
	}
	return sx, sy
}

func mapCenter(x, y float64, group VisualTransform) (float64, float64) {
	cx := float64(group.Offset.X) + float64(group.Extent.W)/2
	cy := float64(group.Offset.Y) + float64(group.Extent.H)/2
	if group.FlipH {
		x = 2*cx - x
	}
	if group.FlipV {
		y = 2*cy - y
	}
	if group.Rotation == 0 {
		return x, y
	}
	angle := group.Rotation * math.Pi / 180
	dx, dy := x-cx, y-cy
	cosA, sinA := math.Cos(angle), math.Sin(angle)
	return cx + dx*cosA - dy*sinA, cy + dx*sinA + dy*cosA
}

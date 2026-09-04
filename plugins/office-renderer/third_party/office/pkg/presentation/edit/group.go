package edit

import (
	presir "office/pkg/presentation/ir"
)

// Group wraps a group shape for editing.
type Group struct {
	shape *Shape
	ir    *presir.Shape
}

// Shape returns the underlying shape wrapper for the group.
func (g *Group) Shape() *Shape {
	if g == nil {
		return nil
	}
	return g.shape
}

// Bounds returns the group bounds in EMU.
func (g *Group) Bounds() (x, y, w, h int64) {
	if g == nil || g.ir == nil {
		return 0, 0, 0, 0
	}
	return g.ir.Bounds.X, g.ir.Bounds.Y, g.ir.Bounds.W, g.ir.Bounds.H
}

// SetBounds sets the group bounds in EMU.
func (g *Group) SetBounds(x, y, w, h int64) {
	if g == nil || g.ir == nil {
		return
	}
	g.ir.Bounds = presir.Rect{X: x, Y: y, W: w, H: h}
}

// Shapes returns all direct child shapes in the group.
func (g *Group) Shapes() []*Shape {
	if g == nil || g.ir == nil {
		return nil
	}
	out := make([]*Shape, len(g.ir.Children))
	for i, child := range g.ir.Children {
		out[i] = &Shape{slide: g.shape.slide, ir: child}
	}
	return out
}

// AddTextBox adds a text box child shape and returns it.
func (g *Group) AddTextBox(x, y, w, h int64) *Shape {
	return g.addChild(presir.ShapeText, x, y, w, h)
}

// AddShape adds a shape child with the given geometry and returns it.
func (g *Group) AddShape(geometry string, x, y, w, h int64) *Shape {
	shape := g.addChild(presir.ShapeText, x, y, w, h)
	if shape != nil {
		shape.ir.Geometry = geometry
	}
	return shape
}

// AddImage adds an image child shape and returns it.
func (g *Group) AddImage(data []byte, mimeType string, x, y, w, h int64) *Shape {
	if g == nil || g.ir == nil {
		return nil
	}
	shape := &presir.Shape{
		Kind:      presir.ShapeImage,
		Name:      nextShapeName(g.ir.Children, "Picture"),
		Bounds:    presir.Rect{X: x, Y: y, W: w, H: h},
		ImageData: data,
		ImageMIME: mimeType,
	}
	g.ir.Children = append(g.ir.Children, shape)
	return &Shape{slide: g.shape.slide, ir: shape}
}

// AddOLEObject adds an embedded OLE or package object child and returns it.
func (g *Group) AddOLEObject(data []byte, mimeType, progID, objectName string, x, y, w, h int64) *Shape {
	if g == nil || g.ir == nil {
		return nil
	}
	shape := &presir.Shape{
		Kind:            presir.ShapeOleObject,
		Name:            nextShapeName(g.ir.Children, "Object"),
		Bounds:          presir.Rect{X: x, Y: y, W: w, H: h},
		OLEObjectData:   append([]byte(nil), data...),
		OLEObjectMIME:   mimeType,
		OLEObjectProgID: progID,
		OLEObjectName:   objectName,
	}
	if shape.OLEObjectName == "" {
		shape.OLEObjectName = "Object"
	}
	g.ir.Children = append(g.ir.Children, shape)
	return &Shape{slide: g.shape.slide, ir: shape}
}

// AddLinkedOLEObject adds an externally linked OLE object child and returns it.
func (g *Group) AddLinkedOLEObject(url, progID, objectName string, x, y, w, h int64) *Shape {
	if g == nil || g.ir == nil {
		return nil
	}
	shape := &presir.Shape{
		Kind:             presir.ShapeOleObject,
		Name:             nextShapeName(g.ir.Children, "Object"),
		Bounds:           presir.Rect{X: x, Y: y, W: w, H: h},
		OLEObjectProgID:  progID,
		OLEObjectName:    objectName,
		OLEObjectLinkURL: url,
	}
	if shape.OLEObjectName == "" {
		shape.OLEObjectName = "Object"
	}
	g.ir.Children = append(g.ir.Children, shape)
	return &Shape{slide: g.shape.slide, ir: shape}
}

// AddGroup adds a nested group and returns it.
func (g *Group) AddGroup(x, y, w, h int64) *Group {
	if g == nil || g.ir == nil {
		return nil
	}
	shape := &presir.Shape{
		Kind:   presir.ShapeGroup,
		Name:   nextShapeName(g.ir.Children, "Group"),
		Bounds: presir.Rect{X: x, Y: y, W: w, H: h},
	}
	g.ir.Children = append(g.ir.Children, shape)
	return &Group{shape: &Shape{slide: g.shape.slide, ir: shape}, ir: shape}
}

// RemoveShape removes the given child shape from the group. Returns true if it
// was present and removed.
func (g *Group) RemoveShape(shape *Shape) bool {
	if g == nil || g.ir == nil || shape == nil || shape.ir == nil {
		return false
	}
	return removeShapeFromList(&g.ir.Children, shape.ir)
}

// RemoveShapeByName removes the first direct child shape with the given name.
// Returns true if a shape was removed.
func (g *Group) RemoveShapeByName(name string) bool {
	if g == nil || g.ir == nil {
		return false
	}
	return removeShapeByNameFromList(&g.ir.Children, name)
}

// DuplicateShape creates a deep copy of shape and appends it to the group. The
// clone is given a fresh name to avoid duplicates.
func (g *Group) DuplicateShape(shape *Shape) *Shape {
	if g == nil || g.ir == nil || shape == nil || shape.ir == nil {
		return nil
	}
	clone := cloneShape(shape.ir)
	clone.Name = uniqueShapeName(g.ir.Children, clone.Name)
	g.ir.Children = append(g.ir.Children, clone)
	return &Shape{slide: g.shape.slide, ir: clone}
}

func (g *Group) addChild(kind presir.ShapeKind, x, y, w, h int64) *Shape {
	if g == nil || g.ir == nil {
		return nil
	}
	shape := &presir.Shape{
		Kind:       kind,
		Name:       nextShapeName(g.ir.Children, "Shape"),
		Bounds:     presir.Rect{X: x, Y: y, W: w, H: h},
		Paragraphs: []*presir.Paragraph{},
	}
	g.ir.Children = append(g.ir.Children, shape)
	return &Shape{slide: g.shape.slide, ir: shape}
}

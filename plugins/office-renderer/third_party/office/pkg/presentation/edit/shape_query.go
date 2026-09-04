package edit

import (
	presir "office/pkg/presentation/ir"
)

// FindShape returns the first direct child shape with the given name, or nil.
func (s *Slide) FindShape(name string) *Shape {
	return findShapeInList(s.ir.Shapes, name)
}

// FindShapeRecursive returns the first shape with the given name anywhere on
// the slide, including inside groups.
func (s *Slide) FindShapeRecursive(name string) *Shape {
	if s == nil || s.ir == nil {
		return nil
	}
	return findShapeRecursiveInList(s.ir.Shapes, name)
}

// FindShapeByID returns the first shape with the given non-visual shape id,
// searching recursively through groups.
func (s *Slide) FindShapeByID(id int) *Shape {
	if s == nil || s.ir == nil {
		return nil
	}
	return findShapeByIDRecursiveInList(s.ir.Shapes, id)
}

// FindShapesByKind returns all direct child shapes matching the given kind.
func (s *Slide) FindShapesByKind(kind presir.ShapeKind) []*Shape {
	if s == nil || s.ir == nil {
		return nil
	}
	return filterShapesByKind(s.ir.Shapes, kind)
}

// FindShapesByKindRecursive returns all shapes matching the given kind,
// including shapes nested inside groups.
func (s *Slide) FindShapesByKindRecursive(kind presir.ShapeKind) []*Shape {
	if s == nil || s.ir == nil {
		return nil
	}
	var out []*Shape
	collectShapesByKindRecursive(s.ir.Shapes, kind, &out)
	return out
}

// ClearShapes removes all shapes from the slide.
func (s *Slide) ClearShapes() {
	if s == nil || s.ir == nil {
		return
	}
	s.ir.Shapes = s.ir.Shapes[:0]
}

// FindShape returns the first direct child shape with the given name, or nil.
func (g *Group) FindShape(name string) *Shape {
	if g == nil || g.ir == nil {
		return nil
	}
	return findShapeInList(g.ir.Children, name)
}

// FindShapeRecursive returns the first shape with the given name anywhere in
// the group, including nested groups.
func (g *Group) FindShapeRecursive(name string) *Shape {
	if g == nil || g.ir == nil {
		return nil
	}
	return findShapeRecursiveInList(g.ir.Children, name)
}

// FindShapeByID returns the first shape with the given non-visual shape id,
// searching recursively through nested groups.
func (g *Group) FindShapeByID(id int) *Shape {
	if g == nil || g.ir == nil {
		return nil
	}
	return findShapeByIDRecursiveInList(g.ir.Children, id)
}

// FindShapesByKind returns all direct child shapes matching the given kind.
func (g *Group) FindShapesByKind(kind presir.ShapeKind) []*Shape {
	if g == nil || g.ir == nil {
		return nil
	}
	return filterShapesByKind(g.ir.Children, kind)
}

// FindShapesByKindRecursive returns all shapes matching the given kind,
// including shapes nested inside groups.
func (g *Group) FindShapesByKindRecursive(kind presir.ShapeKind) []*Shape {
	if g == nil || g.ir == nil {
		return nil
	}
	var out []*Shape
	collectShapesByKindRecursive(g.ir.Children, kind, &out)
	return out
}

// ClearShapes removes all direct child shapes from the group.
func (g *Group) ClearShapes() {
	if g == nil || g.ir == nil {
		return
	}
	g.ir.Children = g.ir.Children[:0]
}

// FindShape returns the first direct child shape with the given name, or nil.
func (m *SlideMaster) FindShape(name string) *Shape {
	if m == nil || m.ir == nil {
		return nil
	}
	return findShapeInList(m.ir.Shapes, name)
}

// FindShapeRecursive returns the first shape with the given name anywhere on
// the master, including inside groups.
func (m *SlideMaster) FindShapeRecursive(name string) *Shape {
	if m == nil || m.ir == nil {
		return nil
	}
	return findShapeRecursiveInList(m.ir.Shapes, name)
}

// FindShapesByKind returns all direct child shapes matching the given kind.
func (m *SlideMaster) FindShapesByKind(kind presir.ShapeKind) []*Shape {
	if m == nil || m.ir == nil {
		return nil
	}
	return filterShapesByKind(m.ir.Shapes, kind)
}

// ClearShapes removes all direct child shapes from the master.
func (m *SlideMaster) ClearShapes() {
	if m == nil || m.ir == nil {
		return
	}
	m.ir.Shapes = m.ir.Shapes[:0]
}

// FindShape returns the first direct child shape with the given name, or nil.
func (l *SlideLayout) FindShape(name string) *Shape {
	if l == nil || l.ir == nil {
		return nil
	}
	return findShapeInList(l.ir.Shapes, name)
}

// FindShapeRecursive returns the first shape with the given name anywhere in
// the layout, including inside groups.
func (l *SlideLayout) FindShapeRecursive(name string) *Shape {
	if l == nil || l.ir == nil {
		return nil
	}
	return findShapeRecursiveInList(l.ir.Shapes, name)
}

// FindShapesByKind returns all direct child shapes matching the given kind.
func (l *SlideLayout) FindShapesByKind(kind presir.ShapeKind) []*Shape {
	if l == nil || l.ir == nil {
		return nil
	}
	return filterShapesByKind(l.ir.Shapes, kind)
}

// ClearShapes removes all direct child shapes from the layout.
func (l *SlideLayout) ClearShapes() {
	if l == nil || l.ir == nil {
		return
	}
	l.ir.Shapes = l.ir.Shapes[:0]
}

func findShapeInList(shapes []*presir.Shape, name string) *Shape {
	for _, sh := range shapes {
		if sh != nil && sh.Name == name {
			return &Shape{ir: sh}
		}
	}
	return nil
}

func findShapeRecursiveInList(shapes []*presir.Shape, name string) *Shape {
	for _, sh := range shapes {
		if sh == nil {
			continue
		}
		if sh.Name == name {
			return &Shape{ir: sh}
		}
		if sh.Kind == presir.ShapeGroup {
			if found := findShapeRecursiveInList(sh.Children, name); found != nil {
				return found
			}
		}
	}
	return nil
}

func findShapeByIDRecursiveInList(shapes []*presir.Shape, id int) *Shape {
	for _, sh := range shapes {
		if sh == nil {
			continue
		}
		if sh.ID == id {
			return &Shape{ir: sh}
		}
		if sh.Kind == presir.ShapeGroup {
			if found := findShapeByIDRecursiveInList(sh.Children, id); found != nil {
				return found
			}
		}
	}
	return nil
}

func filterShapesByKind(shapes []*presir.Shape, kind presir.ShapeKind) []*Shape {
	var out []*Shape
	for _, sh := range shapes {
		if sh != nil && sh.Kind == kind {
			out = append(out, &Shape{ir: sh})
		}
	}
	return out
}

func collectShapesByKindRecursive(shapes []*presir.Shape, kind presir.ShapeKind, out *[]*Shape) {
	for _, sh := range shapes {
		if sh == nil {
			continue
		}
		if sh.Kind == kind {
			*out = append(*out, &Shape{ir: sh})
		}
		if sh.Kind == presir.ShapeGroup {
			collectShapesByKindRecursive(sh.Children, kind, out)
		}
	}
}
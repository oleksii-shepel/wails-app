package edit

import (
	presir "office/pkg/presentation/ir"
)

// SlideLayout wraps a presentation slide layout for editing.
type SlideLayout struct {
	master *SlideMaster
	ir     *presir.SlideLayout
}

// Type returns the layout type (e.g. "blank", "title").
func (l *SlideLayout) Type() string {
	if l == nil || l.ir == nil {
		return ""
	}
	return l.ir.Type
}

// SetType sets the layout type.
func (l *SlideLayout) SetType(layoutType string) {
	if l == nil || l.ir == nil {
		return
	}
	l.ir.Type = layoutType
}

// Name returns the layout name.
func (l *SlideLayout) Name() string {
	if l == nil || l.ir == nil {
		return ""
	}
	return l.ir.Name
}

// SetName sets the layout name.
func (l *SlideLayout) SetName(name string) {
	if l == nil || l.ir == nil {
		return
	}
	l.ir.Name = name
}

// AddTextBox adds a text box shape to the layout.
func (l *SlideLayout) AddTextBox(x, y, w, h int64) *Shape {
	return wrapShape(containerAddTextBox(&l.ir.Shapes, x, y, w, h))
}

// AddShape adds a shape with the given geometry to the layout.
func (l *SlideLayout) AddShape(geometry string, x, y, w, h int64) *Shape {
	return wrapShape(containerAddShape(&l.ir.Shapes, geometry, x, y, w, h))
}

// AddPlaceholder adds a placeholder shape to the layout.
func (l *SlideLayout) AddPlaceholder(kind presir.PlaceholderKind, x, y, w, h int64) *Shape {
	return wrapShape(containerAddPlaceholder(&l.ir.Shapes, kind, x, y, w, h))
}

// AddImage adds an image shape to the layout from raw bytes.
func (l *SlideLayout) AddImage(data []byte, mimeType string, x, y, w, h int64) *Shape {
	return wrapShape(containerAddImage(&l.ir.Shapes, data, mimeType, x, y, w, h))
}

// AddGroup adds a group shape to the layout.
func (l *SlideLayout) AddGroup(x, y, w, h int64) *Group {
	sh := containerAddGroup(&l.ir.Shapes, x, y, w, h)
	if sh == nil {
		return nil
	}
	return &Group{shape: wrapShape(sh), ir: sh}
}

// Shapes returns all shapes on the layout.
func (l *SlideLayout) Shapes() []*Shape {
	if l == nil || l.ir == nil {
		return nil
	}
	out := make([]*Shape, len(l.ir.Shapes))
	for i, sh := range l.ir.Shapes {
		out[i] = wrapShape(sh)
	}
	return out
}
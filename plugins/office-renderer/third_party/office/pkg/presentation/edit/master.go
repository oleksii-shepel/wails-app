package edit

import (
	presir "office/pkg/presentation/ir"
)

// SlideMaster wraps a presentation slide master for editing.
type SlideMaster struct {
	pres *Presentation
	ir   *presir.SlideMaster
}

// Name returns the slide master's name.
func (m *SlideMaster) Name() string {
	if m == nil || m.ir == nil {
		return ""
	}
	return m.ir.Name
}

// SetName sets the slide master's name.
func (m *SlideMaster) SetName(name string) {
	if m == nil || m.ir == nil {
		return
	}
	m.ir.Name = name
}

// AddLayout creates a new layout under this master and returns it.
func (m *SlideMaster) AddLayout(layoutType string) *SlideLayout {
	if m == nil || m.ir == nil {
		return nil
	}
	layout := &presir.SlideLayout{
		Type:   layoutType,
		Name:   layoutType,
		Master: m.ir,
	}
	m.ir.Layouts = append(m.ir.Layouts, layout)
	return &SlideLayout{master: m, ir: layout}
}

// Layouts returns the layouts belonging to this master.
func (m *SlideMaster) Layouts() []*SlideLayout {
	if m == nil || m.ir == nil {
		return nil
	}
	out := make([]*SlideLayout, len(m.ir.Layouts))
	for i, l := range m.ir.Layouts {
		out[i] = &SlideLayout{master: m, ir: l}
	}
	return out
}

// AddTextBox adds a text box shape to the master.
func (m *SlideMaster) AddTextBox(x, y, w, h int64) *Shape {
	return wrapShape(containerAddTextBox(&m.ir.Shapes, x, y, w, h))
}

// AddShape adds a shape with the given geometry to the master.
func (m *SlideMaster) AddShape(geometry string, x, y, w, h int64) *Shape {
	return wrapShape(containerAddShape(&m.ir.Shapes, geometry, x, y, w, h))
}

// AddPlaceholder adds a placeholder shape to the master.
func (m *SlideMaster) AddPlaceholder(kind presir.PlaceholderKind, x, y, w, h int64) *Shape {
	return wrapShape(containerAddPlaceholder(&m.ir.Shapes, kind, x, y, w, h))
}

// AddImage adds an image shape to the master from raw bytes.
func (m *SlideMaster) AddImage(data []byte, mimeType string, x, y, w, h int64) *Shape {
	return wrapShape(containerAddImage(&m.ir.Shapes, data, mimeType, x, y, w, h))
}

// AddGroup adds a group shape to the master.
func (m *SlideMaster) AddGroup(x, y, w, h int64) *Group {
	sh := containerAddGroup(&m.ir.Shapes, x, y, w, h)
	if sh == nil {
		return nil
	}
	return &Group{shape: wrapShape(sh), ir: sh}
}

// Shapes returns all shapes on the master.
func (m *SlideMaster) Shapes() []*Shape {
	if m == nil || m.ir == nil {
		return nil
	}
	out := make([]*Shape, len(m.ir.Shapes))
	for i, sh := range m.ir.Shapes {
		out[i] = wrapShape(sh)
	}
	return out
}
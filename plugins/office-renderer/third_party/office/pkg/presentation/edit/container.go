package edit

import (
	presir "office/pkg/presentation/ir"
)

// container helpers are shared by Slide, SlideMaster, SlideLayout, and Group
// so that shape-adding code is not duplicated for every parent container.

func wrapShape(ir *presir.Shape) *Shape {
	if ir == nil {
		return nil
	}
	return &Shape{ir: ir}
}

func containerAddTextBox(shapes *[]*presir.Shape, x, y, w, h int64) *presir.Shape {
	sh := &presir.Shape{
		Kind:       presir.ShapeText,
		Name:       nextShapeName(*shapes, "TextBox"),
		Bounds:     presir.Rect{X: x, Y: y, W: w, H: h},
		Paragraphs: []*presir.Paragraph{},
	}
	*shapes = append(*shapes, sh)
	return sh
}

func containerAddShape(shapes *[]*presir.Shape, geometry string, x, y, w, h int64) *presir.Shape {
	sh := &presir.Shape{
		Kind:       presir.ShapeText,
		Name:       nextShapeName(*shapes, "Shape"),
		Bounds:     presir.Rect{X: x, Y: y, W: w, H: h},
		Geometry:   geometry,
		Paragraphs: []*presir.Paragraph{},
	}
	*shapes = append(*shapes, sh)
	return sh
}

func containerAddPlaceholder(shapes *[]*presir.Shape, kind presir.PlaceholderKind, x, y, w, h int64) *presir.Shape {
	sh := &presir.Shape{
		Kind:        presir.ShapeText,
		Name:        placeholderDefaultName(kind),
		Bounds:      presir.Rect{X: x, Y: y, W: w, H: h},
		Placeholder: kind,
		Paragraphs:  []*presir.Paragraph{},
	}
	*shapes = append(*shapes, sh)
	return sh
}

func containerAddImage(shapes *[]*presir.Shape, data []byte, mimeType string, x, y, w, h int64) *presir.Shape {
	sh := &presir.Shape{
		Kind:      presir.ShapeImage,
		Name:      nextShapeName(*shapes, "Picture"),
		Bounds:    presir.Rect{X: x, Y: y, W: w, H: h},
		ImageData: data,
		ImageMIME: mimeType,
	}
	*shapes = append(*shapes, sh)
	return sh
}

func containerAddGroup(shapes *[]*presir.Shape, x, y, w, h int64) *presir.Shape {
	sh := &presir.Shape{
		Kind:   presir.ShapeGroup,
		Name:   nextShapeName(*shapes, "Group"),
		Bounds: presir.Rect{X: x, Y: y, W: w, H: h},
	}
	*shapes = append(*shapes, sh)
	return sh
}

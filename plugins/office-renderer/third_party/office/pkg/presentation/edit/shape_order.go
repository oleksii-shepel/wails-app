package edit

import (
	presir "office/pkg/presentation/ir"
)

// BringToFront moves the shape to the end of the slide's shape list so it
// renders on top. Returns true if the shape belonged to the slide.
func (s *Slide) BringToFront(shape *Shape) bool {
	if s == nil || s.ir == nil || shape == nil || shape.ir == nil {
		return false
	}
	return moveShapeToFront(&s.ir.Shapes, shape.ir)
}

// SendToBack moves the shape to the start of the slide's shape list so it
// renders behind other shapes. Returns true if the shape belonged to the slide.
func (s *Slide) SendToBack(shape *Shape) bool {
	if s == nil || s.ir == nil || shape == nil || shape.ir == nil {
		return false
	}
	return moveShapeToBack(&s.ir.Shapes, shape.ir)
}

// MoveShape moves shape to the given index in the slide's shape list. Returns
// true if the shape belonged to the slide and the move succeeded.
func (s *Slide) MoveShape(index int, shape *Shape) bool {
	if s == nil || s.ir == nil || shape == nil || shape.ir == nil {
		return false
	}
	return moveShapeToIndex(&s.ir.Shapes, shape.ir, index)
}

// BringToFront moves the child shape to the end of the group's shape list.
func (g *Group) BringToFront(shape *Shape) bool {
	if g == nil || g.ir == nil || shape == nil || shape.ir == nil {
		return false
	}
	return moveShapeToFront(&g.ir.Children, shape.ir)
}

// SendToBack moves the child shape to the start of the group's shape list.
func (g *Group) SendToBack(shape *Shape) bool {
	if g == nil || g.ir == nil || shape == nil || shape.ir == nil {
		return false
	}
	return moveShapeToBack(&g.ir.Children, shape.ir)
}

// MoveShape moves child shape to the given index in the group's shape list.
func (g *Group) MoveShape(index int, shape *Shape) bool {
	if g == nil || g.ir == nil || shape == nil || shape.ir == nil {
		return false
	}
	return moveShapeToIndex(&g.ir.Children, shape.ir, index)
}

func moveShapeToFront(shapes *[]*presir.Shape, target *presir.Shape) bool {
	idx := indexOfShape(*shapes, target)
	if idx < 0 {
		return false
	}
	*shapes = append((*shapes)[:idx], (*shapes)[idx+1:]...)
	*shapes = append(*shapes, target)
	return true
}

func moveShapeToBack(shapes *[]*presir.Shape, target *presir.Shape) bool {
	idx := indexOfShape(*shapes, target)
	if idx < 0 {
		return false
	}
	*shapes = append((*shapes)[:idx], (*shapes)[idx+1:]...)
	*shapes = append([]*presir.Shape{target}, (*shapes)...)
	return true
}

func moveShapeToIndex(shapes *[]*presir.Shape, target *presir.Shape, index int) bool {
	idx := indexOfShape(*shapes, target)
	if idx < 0 {
		return false
	}
	if index < 0 {
		index = 0
	}
	if index > len(*shapes)-1 {
		index = len(*shapes) - 1
	}
	if idx == index {
		return true
	}
	*shapes = append((*shapes)[:idx], (*shapes)[idx+1:]...)
	*shapes = append((*shapes)[:index], append([]*presir.Shape{target}, (*shapes)[index:]...)...)
	return true
}

func indexOfShape(shapes []*presir.Shape, target *presir.Shape) int {
	for i, sh := range shapes {
		if sh == target {
			return i
		}
	}
	return -1
}

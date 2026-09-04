package render

import (
	"math"
	"testing"

	"office/pkg/ooxml/smartart"
)

func makeDiagram(layoutType string, levels []int) *smartart.Diagram {
	diagram := &smartart.Diagram{
		Layout: &smartart.Layout{Type: layoutType},
		Style:  &smartart.Style{},
	}
	for i, lvl := range levels {
		diagram.Nodes = append(diagram.Nodes, &smartart.Node{
			ID:    string(rune('a' + i)),
			Level: lvl,
		})
	}
	return diagram
}

func TestLayoutEngineRadial(t *testing.T) {
	diagram := makeDiagram("radial", []int{0, 1, 1, 1})
	shapes := LayoutDiagram(diagram)
	if len(shapes) != 4 {
		t.Fatalf("shapes = %d, want 4", len(shapes))
	}
	// Root should be near the center, children around it.
	root := shapes[0]
	if root.X == 0 || root.Y == 0 {
		t.Errorf("root at origin: %v", root)
	}
	for i := 1; i < len(shapes); i++ {
		if shapes[i].X == root.X && shapes[i].Y == root.Y {
			t.Errorf("child %d overlaps root", i)
		}
	}
}

func TestLayoutEngineBullet(t *testing.T) {
	diagram := makeDiagram("bullet", []int{0, 1, 1, 2})
	shapes := LayoutDiagram(diagram)
	if len(shapes) != 4 {
		t.Fatalf("shapes = %d, want 4", len(shapes))
	}
	// Each successive shape should be lower.
	for i := 1; i < len(shapes); i++ {
		if shapes[i].Y <= shapes[i-1].Y {
			t.Errorf("shape %d not below previous: y=%v prev=%v", i, shapes[i].Y, shapes[i-1].Y)
		}
	}
	// Deeper levels should be indented further right.
	if shapes[3].X <= shapes[0].X {
		t.Errorf("level-2 shape not indented: x=%v root=%v", shapes[3].X, shapes[0].X)
	}
}

func TestLayoutEnginePyramid(t *testing.T) {
	diagram := makeDiagram("pyramid", []int{0, 1, 1})
	shapes := LayoutDiagram(diagram)
	if len(shapes) != 3 {
		t.Fatalf("shapes = %d, want 3", len(shapes))
	}
	// Level 0 shape should be centered above the level-1 row.
	root := shapes[0]
	rootCenter := root.X + root.Width/2
	if rootCenter < 390 || rootCenter > 410 {
		t.Errorf("root not centered: center=%v", rootCenter)
	}
	if shapes[1].Y <= root.Y {
		t.Errorf("children not below root")
	}
}

func makeHierarchyDiagram(children int) *smartart.Diagram {
	root := &smartart.Node{ID: "root", Type: "root"}
	diagram := &smartart.Diagram{
		Layout: &smartart.Layout{Type: "list", VSpacing: 80},
		Style:  &smartart.Style{},
		Nodes:  []*smartart.Node{root},
	}
	for i := 0; i < children; i++ {
		child := &smartart.Node{ID: string(rune('a' + i)), Type: "doc", Parent: root, Level: 1}
		root.Children = append(root.Children, child)
		diagram.Nodes = append(diagram.Nodes, child)
	}
	return diagram
}

func TestLayoutEngineLayoutDefinitionLinear(t *testing.T) {
	diagram := makeHierarchyDiagram(3)
	diagram.Layout.Definition = &smartart.LayoutDefinition{
		Root: &smartart.LayoutNode{
			Name: "diagram",
			Algorithm: &smartart.Algorithm{
				Type: "lin",
				Params: map[string]string{
					"linDir": "fromT",
				},
			},
		},
	}

	shapes := LayoutDiagram(diagram)
	if len(shapes) != 4 {
		t.Fatalf("shapes = %d, want 4", len(shapes))
	}

	// Find the child shapes and assert they are stacked vertically.
	var children []*SmartArtShape
	for _, s := range shapes {
		if s.ID != "root" {
			children = append(children, s)
		}
	}
	if len(children) != 3 {
		t.Fatalf("children = %d, want 3", len(children))
	}
	for i := 1; i < len(children); i++ {
		if children[i].Y <= children[i-1].Y {
			t.Errorf("child %d not below previous: y=%v prev=%v", i, children[i].Y, children[i-1].Y)
		}
		if children[i].X != children[0].X {
			t.Errorf("child %d x = %v, want %v", i, children[i].X, children[0].X)
		}
	}
}

func TestLayoutEngineLayoutDefinitionSnake(t *testing.T) {
	diagram := makeHierarchyDiagram(6)
	diagram.Layout.Definition = &smartart.LayoutDefinition{
		Root: &smartart.LayoutNode{
			Name: "diagram",
			Algorithm: &smartart.Algorithm{
				Type: "snake",
				Params: map[string]string{
					"flowDir": "row",
				},
			},
		},
	}

	shapes := LayoutDiagram(diagram)
	if len(shapes) != 7 {
		t.Fatalf("shapes = %d, want 7", len(shapes))
	}

	// With default sizes and container width, children should wrap into at least
	// two rows. Verify that not all children share the same y coordinate.
	var ys = make(map[float64]int)
	for _, s := range shapes {
		if s.ID != "root" {
			ys[s.Y]++
		}
	}
	if len(ys) < 2 {
		t.Errorf("expected snake layout to wrap into multiple rows, got %d distinct y values", len(ys))
	}
}

func TestLayoutEngineLayoutDefinitionUnsupported(t *testing.T) {
	diagram := makeHierarchyDiagram(3)
	diagram.Layout.Type = "hierarchy"
	diagram.Layout.Definition = &smartart.LayoutDefinition{
		Root: &smartart.LayoutNode{
			Name:      "diagram",
			Algorithm: &smartart.Algorithm{Type: "hierRoot"},
		},
	}

	shapes := LayoutDiagram(diagram)
	if len(shapes) != 4 {
		t.Fatalf("shapes = %d, want 4", len(shapes))
	}
	// Fallback hierarchy layout should place root above children.
	var rootShape *SmartArtShape
	var childYs []float64
	for _, s := range shapes {
		if s.ID == "root" {
			rootShape = s
		} else {
			childYs = append(childYs, s.Y)
		}
	}
	if rootShape == nil {
		t.Fatal("root shape not found")
	}
	for _, y := range childYs {
		if y <= rootShape.Y {
			t.Errorf("fallback hierarchy failed: child y=%v not below root y=%v", y, rootShape.Y)
		}
	}
}

func makeTreeNode(id string, children ...*smartart.Node) *smartart.Node {
	node := &smartart.Node{ID: id}
	for _, child := range children {
		child.Parent = node
		node.Children = append(node.Children, child)
	}
	return node
}

func buildHierarchyDiagram(direction string, root *smartart.Node) *smartart.Diagram {
	var setLevels func(n *smartart.Node, level int)
	setLevels = func(n *smartart.Node, level int) {
		n.Level = level
		for _, child := range n.Children {
			setLevels(child, level+1)
		}
	}
	setLevels(root, 0)

	var nodes []*smartart.Node
	var collect func(n *smartart.Node)
	collect = func(n *smartart.Node) {
		nodes = append(nodes, n)
		for _, child := range n.Children {
			collect(child)
		}
	}
	collect(root)

	return &smartart.Diagram{
		Layout: &smartart.Layout{
			Type:      "hierarchy",
			Direction: direction,
			HSpacing:  50,
			VSpacing:  80,
		},
		Style: &smartart.Style{},
		Nodes: nodes,
	}
}

func findShape(shapes []*SmartArtShape, id string) *SmartArtShape {
	for _, s := range shapes {
		if s.ID == id {
			return s
		}
	}
	return nil
}

func TestLayoutEngineHierarchyTree(t *testing.T) {
	root := makeTreeNode("root",
		makeTreeNode("c1",
			makeTreeNode("g1"),
			makeTreeNode("g2"),
		),
		makeTreeNode("c2",
			makeTreeNode("g3"),
		),
	)
	diagram := buildHierarchyDiagram("topDown", root)
	shapes := LayoutDiagram(diagram)

	rootShape := findShape(shapes, "root")
	c1 := findShape(shapes, "c1")
	c2 := findShape(shapes, "c2")
	g1 := findShape(shapes, "g1")
	g3 := findShape(shapes, "g3")
	if rootShape == nil || c1 == nil || c2 == nil || g1 == nil || g3 == nil {
		t.Fatal("missing shapes")
	}

	// Children should be below the root.
	if c1.Y <= rootShape.Y || c2.Y <= rootShape.Y {
		t.Errorf("children not below root: rootY=%v c1Y=%v c2Y=%v", rootShape.Y, c1.Y, c2.Y)
	}
	// Grandchildren should be below their parents.
	if g1.Y <= c1.Y || g3.Y <= c2.Y {
		t.Errorf("grandchildren not below parents")
	}

	// The root should be centered roughly over the whole tree.
	minX, maxX := shapes[0].X, shapes[0].X+shapes[0].Width
	for _, s := range shapes {
		if s.X < minX {
			minX = s.X
		}
		if rx := s.X + s.Width; rx > maxX {
			maxX = rx
		}
	}
	rootCenter := rootShape.X + rootShape.Width/2
	mid := (minX + maxX) / 2
	if math.Abs(rootCenter-mid) > 20 {
		t.Errorf("root not centered over tree: center=%v mid=%v", rootCenter, mid)
	}
}

func TestLayoutEngineHierarchyAssistant(t *testing.T) {
	assistant := makeTreeNode("asst")
	assistant.Assistant = true
	root := makeTreeNode("root",
		assistant,
		makeTreeNode("c1"),
		makeTreeNode("c2"),
	)
	diagram := buildHierarchyDiagram("topDown", root)
	shapes := LayoutDiagram(diagram)

	rootShape := findShape(shapes, "root")
	asst := findShape(shapes, "asst")
	c1 := findShape(shapes, "c1")
	if rootShape == nil || asst == nil || c1 == nil {
		t.Fatal("missing shapes")
	}

	// Assistant should be beside the parent, not below.
	if math.Abs(asst.Y-rootShape.Y) > 20 {
		t.Errorf("assistant not beside parent: rootY=%v asstY=%v", rootShape.Y, asst.Y)
	}
	if asst.X <= rootShape.X {
		t.Errorf("assistant should be to the right of the parent: rootX=%v asstX=%v", rootShape.X, asst.X)
	}
	// Regular children should be below the parent.
	if c1.Y <= rootShape.Y {
		t.Errorf("regular child not below parent: rootY=%v c1Y=%v", rootShape.Y, c1.Y)
	}
}

func TestLayoutEngineHierarchyDirections(t *testing.T) {
	for _, tc := range []struct {
		direction string
	}{
		{"topDown"},
		{"bottomUp"},
		{"leftRight"},
		{"rightLeft"},
	} {
		t.Run(tc.direction, func(t *testing.T) {
			root := makeTreeNode("root", makeTreeNode("child"))
			diagram := buildHierarchyDiagram(tc.direction, root)
			shapes := LayoutDiagram(diagram)

			rootShape := findShape(shapes, "root")
			child := findShape(shapes, "child")
			if rootShape == nil || child == nil {
				t.Fatal("missing shapes")
			}

			switch tc.direction {
			case "topDown":
				if child.Y <= rootShape.Y {
					t.Errorf("child should be below root")
				}
			case "bottomUp":
				if child.Y >= rootShape.Y {
					t.Errorf("child should be above root")
				}
			case "leftRight":
				if child.X <= rootShape.X {
					t.Errorf("child should be to the right of root")
				}
			case "rightLeft":
				if child.X >= rootShape.X {
					t.Errorf("child should be to the left of root")
				}
			}
		})
	}
}

func TestLayoutDefinitionSupportsStandardCycleAlgorithm(t *testing.T) {
	diagram := makeHierarchyDiagram(4)
	diagram.Layout.Definition = &smartart.LayoutDefinition{Root: &smartart.LayoutNode{
		Algorithm: &smartart.Algorithm{Type: "cycle"},
	}}
	shapes := LayoutDiagram(diagram)
	var positions = map[[2]int]bool{}
	for _, s := range shapes {
		if s.ID == "root" {
			continue
		}
		key := [2]int{int(math.Round(s.X)), int(math.Round(s.Y))}
		positions[key] = true
	}
	if len(positions) != 4 {
		t.Fatalf("cycle algorithm produced %d distinct positions, want 4", len(positions))
	}
}

func TestLayoutDefinitionSupportsPyramidAlgorithm(t *testing.T) {
	diagram := makeHierarchyDiagram(3)
	diagram.Layout.Definition = &smartart.LayoutDefinition{Root: &smartart.LayoutNode{
		Algorithm: &smartart.Algorithm{Type: "pyra"},
	}}
	shapes := LayoutDiagram(diagram)
	var children []*SmartArtShape
	for _, s := range shapes {
		if s.ID != "root" {
			children = append(children, s)
		}
	}
	if len(children) != 3 {
		t.Fatalf("children = %d, want 3", len(children))
	}
	if !(children[0].Width < children[1].Width && children[1].Width < children[2].Width) {
		t.Fatalf("pyramid widths should grow by row: %.1f %.1f %.1f", children[0].Width, children[1].Width, children[2].Width)
	}
	if !(children[0].Y < children[1].Y && children[1].Y < children[2].Y) {
		t.Fatalf("pyramid rows not vertically ordered")
	}
}

func TestLayoutDefinitionSupportsHierarchyAlgorithm(t *testing.T) {
	diagram := makeHierarchyDiagram(3)
	diagram.Layout.Type = "hierarchy"
	diagram.Layout.Definition = &smartart.LayoutDefinition{Root: &smartart.LayoutNode{
		Algorithm: &smartart.Algorithm{Type: "hierRoot"},
	}}
	shapes := LayoutDiagram(diagram)
	root := findShape(shapes, "root")
	if root == nil {
		t.Fatal("root not found")
	}
	for _, s := range shapes {
		if s.ID != "root" && s.Y <= root.Y {
			t.Fatalf("hierRoot algorithm did not place %s below root", s.ID)
		}
	}
}

func TestSelectAxisNodesSupportsParentAncestorsAndSiblings(t *testing.T) {
	a := makeTreeNode("a")
	b := makeTreeNode("b")
	c := makeTreeNode("c")
	root := makeTreeNode("root", a, b, c)
	grand := makeTreeNode("grand", root)
	diagram := buildHierarchyDiagram("topDown", grand)
	engine := NewLayoutEngine(diagram)
	ctx := &layoutContext{nodes: []*smartart.Node{b}}

	if got := engine.selectAxisNodes("par", "", ctx); len(got) != 1 || got[0] != root {
		t.Fatalf("par axis = %#v, want root", got)
	}
	if got := engine.selectAxisNodes("ancst", "", ctx); len(got) != 2 || got[0] != root || got[1] != grand {
		t.Fatalf("ancst axis unexpected: %#v", got)
	}
	if got := engine.selectAxisNodes("followSib", "", ctx); len(got) != 1 || got[0] != c {
		t.Fatalf("followSib axis unexpected: %#v", got)
	}
	if got := engine.selectAxisNodes("precedSib", "", ctx); len(got) != 1 || got[0] != a {
		t.Fatalf("precedSib axis unexpected: %#v", got)
	}
}

func TestLayoutEngineHierarchyMultipleAssistantsStayBesideParent(t *testing.T) {
	a1 := makeTreeNode("a1")
	a1.Assistant = true
	a2 := makeTreeNode("a2")
	a2.Assistant = true
	root := makeTreeNode("root", a1, a2, makeTreeNode("child"))
	shapes := LayoutDiagram(buildHierarchyDiagram("topDown", root))
	r := findShape(shapes, "root")
	s1 := findShape(shapes, "a1")
	s2 := findShape(shapes, "a2")
	child := findShape(shapes, "child")
	if r == nil || s1 == nil || s2 == nil || child == nil {
		t.Fatal("missing hierarchy shapes")
	}
	if s1.Y >= child.Y || s2.Y >= child.Y {
		t.Fatalf("assistants should stay above regular child row: a1=%.1f a2=%.1f child=%.1f", s1.Y, s2.Y, child.Y)
	}
	if !((s1.X > r.X && s2.X < r.X) || (s2.X > r.X && s1.X < r.X)) {
		t.Fatalf("assistants should occupy opposite sides of parent: root=%.1f a1=%.1f a2=%.1f", r.X, s1.X, s2.X)
	}
}

func TestLayoutDefinitionAppliesAbsoluteSizeConstraints(t *testing.T) {
	diagram := makeHierarchyDiagram(2)
	diagram.Layout.Definition = &smartart.LayoutDefinition{Root: &smartart.LayoutNode{
		Constraints: []smartart.Constraint{
			{Type: "w", For: "self", Val: 220, Op: "equ"},
			{Type: "h", For: "self", Val: 90, Op: "equ"},
		},
		Algorithm: &smartart.Algorithm{Type: "lin", Params: map[string]string{"linDir": "fromL"}},
	}}
	shapes := LayoutDiagram(diagram)
	for _, s := range shapes {
		if s.ID == "root" {
			continue
		}
		if math.Abs(s.Width-220) > 0.01 || math.Abs(s.Height-90) > 0.01 {
			t.Fatalf("constraint size for %s = %.2fx%.2f, want 220x90", s.ID, s.Width, s.Height)
		}
	}
}

func TestLayoutDefinitionAppliesAspectAndOffsetConstraints(t *testing.T) {
	diagram := makeHierarchyDiagram(1)
	diagram.Layout.Definition = &smartart.LayoutDefinition{Root: &smartart.LayoutNode{
		Constraints: []smartart.Constraint{
			{Type: "w", For: "self", Val: 180, Op: "equ"},
			{Type: "h", For: "self", Val: 120, Op: "equ"},
			{Type: "ar", For: "self", Val: 2, Op: "equ"},
			{Type: "lOff", For: "self", Val: 25},
			{Type: "tOff", For: "self", Val: 15},
		},
		Algorithm: &smartart.Algorithm{Type: "lin", Params: map[string]string{"linDir": "fromT"}},
	}}
	shapes := LayoutDiagram(diagram)
	child := findShape(shapes, "a")
	if child == nil {
		t.Fatal("child not found")
	}
	if math.Abs(child.Width/child.Height-2) > 0.001 {
		t.Fatalf("aspect = %.3f, want 2", child.Width/child.Height)
	}
	if child.X < 74.9 || child.Y < 64.9 {
		t.Fatalf("offset constraints not applied: x=%.1f y=%.1f", child.X, child.Y)
	}
}

func TestLayoutDefinitionConstraintCanReferenceNodeDimension(t *testing.T) {
	diagram := makeHierarchyDiagram(1)
	child := diagram.Nodes[1]
	child.Position.Width = 240
	child.Position.Height = 100
	diagram.Layout.Definition = &smartart.LayoutDefinition{Root: &smartart.LayoutNode{
		Constraints: []smartart.Constraint{{Type: "h", For: "self", RefType: "w", RefFor: "self", Fact: 0.5, Op: "equ"}},
		Algorithm:   &smartart.Algorithm{Type: "lin", Params: map[string]string{"linDir": "fromL"}},
	}}
	shapes := LayoutDiagram(diagram)
	got := findShape(shapes, "a")
	if got == nil {
		t.Fatal("child not found")
	}
	if math.Abs(got.Height-got.Width*0.5) > 0.01 {
		t.Fatalf("referenced height = %.2f, width %.2f", got.Height, got.Width)
	}
}

func TestLinearAlgorithmHonorsCrossAxisAlignment(t *testing.T) {
	diagram := makeHierarchyDiagram(2)
	diagram.Nodes[1].Position.Width = 100
	diagram.Nodes[1].Position.Height = 60
	diagram.Nodes[2].Position.Width = 200
	diagram.Nodes[2].Position.Height = 60
	diagram.Layout.Definition = &smartart.LayoutDefinition{Root: &smartart.LayoutNode{
		Algorithm: &smartart.Algorithm{Type: "lin", Params: map[string]string{"linDir": "fromT", "horzAlign": "r"}},
	}}
	shapes := LayoutDiagram(diagram)
	a, b := findShape(shapes, "a"), findShape(shapes, "b")
	if a == nil || b == nil {
		t.Fatal("children not found")
	}
	if math.Abs((a.X+a.Width)-(b.X+b.Width)) > 0.01 {
		t.Fatalf("right edges differ: a=%.1f b=%.1f", a.X+a.Width, b.X+b.Width)
	}
}

func TestSnakeAlgorithmHonorsFixedBreakpointAndReverseContinuation(t *testing.T) {
	diagram := makeHierarchyDiagram(6)
	diagram.Layout.Definition = &smartart.LayoutDefinition{Root: &smartart.LayoutNode{
		Algorithm: &smartart.Algorithm{Type: "snake", Params: map[string]string{
			"flowDir": "row", "bkpt": "fixed", "bkPtFixedVal": "3", "contDir": "revDir",
		}},
	}}
	shapes := LayoutDiagram(diagram)
	a, b, c := findShape(shapes, "a"), findShape(shapes, "b"), findShape(shapes, "c")
	d, e, f := findShape(shapes, "d"), findShape(shapes, "e"), findShape(shapes, "f")
	if a == nil || b == nil || c == nil || d == nil || e == nil || f == nil {
		t.Fatal("missing shapes")
	}
	if !(a.X < b.X && b.X < c.X) {
		t.Fatalf("first row should grow left-to-right")
	}
	if !(d.X > e.X && e.X > f.X) {
		t.Fatalf("second row should reverse direction: %.1f %.1f %.1f", d.X, e.X, f.X)
	}
	if d.Y <= a.Y {
		t.Fatalf("second row not below first")
	}
}

func TestCycleAlgorithmHonorsStartAndSpanAngles(t *testing.T) {
	diagram := makeHierarchyDiagram(3)
	diagram.Layout.Definition = &smartart.LayoutDefinition{Root: &smartart.LayoutNode{
		Algorithm: &smartart.Algorithm{Type: "cycle", Params: map[string]string{
			"stAng": "0", "spanAng": "10800000", // 180 degrees
		}},
	}}
	shapes := LayoutDiagram(diagram)
	a, b, c := findShape(shapes, "a"), findShape(shapes, "b"), findShape(shapes, "c")
	if a == nil || b == nil || c == nil {
		t.Fatal("missing shapes")
	}
	// 0, 90, 180 degrees: first and last have nearly equal Y centers and
	// opposite X sides, while the middle node is below them.
	acy, bcy, ccy := a.Y+a.Height/2, b.Y+b.Height/2, c.Y+c.Height/2
	if math.Abs(acy-ccy) > 0.01 || bcy <= acy {
		t.Fatalf("unexpected semicircle positions")
	}
	if !(a.X > c.X) {
		t.Fatalf("expected first node on right and last on left")
	}
}

func TestHierarchyAlgorithmHonorsChildDirection(t *testing.T) {
	diagram := makeHierarchyDiagram(2)
	diagram.Layout.Type = "hierarchy"
	diagram.Layout.Definition = &smartart.LayoutDefinition{Root: &smartart.LayoutNode{
		Algorithm: &smartart.Algorithm{Type: "hierRoot", Params: map[string]string{"chDir": "horz"}},
	}}
	shapes := LayoutDiagram(diagram)
	root, child := findShape(shapes, "root"), findShape(shapes, "a")
	if root == nil || child == nil {
		t.Fatal("missing shapes")
	}
	if child.X <= root.X {
		t.Fatalf("horizontal child direction should place child to the right")
	}
}

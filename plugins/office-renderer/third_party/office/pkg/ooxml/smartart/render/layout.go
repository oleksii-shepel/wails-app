package render

import (
	"math"

	"office/pkg/ooxml/drawing"
	"office/pkg/ooxml/smartart"
)

// LayoutEngine calculates positions for diagram nodes.
type LayoutEngine struct {
	Diagram     *smartart.Diagram
	Shapes      []*Shape
	FrameBounds *FrameBounds
}

// NewLayoutEngine creates a layout engine for the diagram.
func NewLayoutEngine(diagram *smartart.Diagram) *LayoutEngine {
	return &LayoutEngine{
		Diagram: diagram,
		Shapes:  []*Shape{},
	}
}

// CalculatePositions computes node positions based on layout type. When a
// dgm:layoutDef algorithm tree is available, it is executed first; if the
// executor cannot handle the tree, the existing heuristic layouts are used.
func (e *LayoutEngine) CalculatePositions() {
	if e.Diagram.Layout != nil && e.Diagram.Layout.Definition != nil {
		if e.runLayoutDefinition() {
			return
		}
	}

	switch e.Diagram.Layout.Type {
	case "hierarchy":
		e.calculateHierarchyPositions()
	case "process":
		e.calculateProcessPositions()
	case "cycle":
		e.calculateCyclePositions()
	case "radial":
		e.calculateRadialPositions()
	case "matrix":
		e.calculateMatrixPositions()
	case "pyramid":
		e.calculatePyramidPositions()
	case "bullet":
		e.calculateBulletPositions()
	case "list":
		e.calculateListPositions()
	default:
		e.calculateHierarchyPositions()
	}
}

// calculateHierarchyPositions lays out nodes in a tree structure, centering each
// parent over its own children. It supports assistant nodes and all four
// hierarchy directions, fitting the result to the engine's FrameBounds.
func (e *LayoutEngine) calculateHierarchyPositions() {
	roots := e.hierarchyRoots()
	if len(roots) == 0 {
		return
	}

	layout := e.Diagram.Layout
	if layout == nil {
		layout = &smartart.Layout{}
	}

	offsetX := 0.0
	maxH := 0.0
	for _, root := range roots {
		w, h, _ := e.layoutNodeTree(root, layout)
		// Shift the subtree so roots are laid out horizontally.
		if offsetX > 0 {
			e.shiftSubtree(root, offsetX, 0)
		}
		offsetX += w + e.hSpacing(layout)
		if h > maxH {
			maxH = h
		}
	}

	// Remove trailing spacing.
	if offsetX > 0 {
		offsetX -= e.hSpacing(layout)
	}

	e.applyDirection(layout.Direction, offsetX, maxH)
	e.fitToFrame()
}

// hierarchyRoots returns the top-level nodes of the hierarchy. If every node
// already has a parent, all nodes are treated as roots.
func (e *LayoutEngine) hierarchyRoots() []*smartart.Node {
	var roots []*smartart.Node
	for _, node := range e.Diagram.Nodes {
		if node.Parent == nil {
			roots = append(roots, node)
		}
	}
	if len(roots) == 0 && len(e.Diagram.Nodes) > 0 {
		roots = e.Diagram.Nodes
	}
	return roots
}

// layoutNodeTree recursively positions a node and its descendants in a
// normalized top-down orientation. It returns the bounding width and height of
// the subtree and the horizontal offset of the node's top-left corner from the
// left edge of the subtree. Positions are stored relative to the subtree's left
// edge (all values are non-negative), with the parent centered over its
// children's bounding box.
func (e *LayoutEngine) layoutNodeTree(node *smartart.Node, layout *smartart.Layout) (width, height, nodeOffsetX float64) {
	nw := 150.0
	nh := 60.0
	if node.Position.Width > 0 {
		nw = node.Position.Width
	}
	if node.Position.Height > 0 {
		nh = node.Position.Height
	}
	hSpacing := e.hSpacing(layout)
	vSpacing := e.vSpacing(layout)

	node.Position.Width = nw
	node.Position.Height = nh

	// Layout children first; each child subtree has its own node at (0,0).
	var regular, assistants []*smartart.Node
	for _, child := range node.Children {
		if child.Assistant {
			assistants = append(assistants, child)
		} else {
			regular = append(regular, child)
		}
	}

	type childInfo struct {
		node       *smartart.Node
		w, h, offX float64
	}
	var infos []childInfo
	regularWidth := 0.0
	regularHeight := 0.0
	for i, child := range regular {
		cw, ch, coff := e.layoutNodeTree(child, layout)
		infos = append(infos, childInfo{node: child, w: cw, h: ch, offX: coff})
		regularWidth += cw
		if i > 0 {
			regularWidth += hSpacing
		}
		if ch > regularHeight {
			regularHeight = ch
		}
	}

	var assistantInfos []childInfo
	for _, assistant := range assistants {
		aw, ah, aoff := e.layoutNodeTree(assistant, layout)
		assistantInfos = append(assistantInfos, childInfo{node: assistant, w: aw, h: ah, offX: aoff})
	}

	// Assistants belong beside the parent, not in the normal child row. Multiple
	// assistants alternate right/left and stack away from the parent so they do
	// not collapse into ordinary children. Their height also pushes the child row
	// down when needed.
	minX := 0.0
	maxX := nw
	maxY := nh
	leftY, rightY := 0.0, 0.0
	assistantBottom := nh
	for i, ai := range assistantInfos {
		aLeft := nw + hSpacing
		aY := rightY
		if i%2 == 1 {
			aLeft = -hSpacing - ai.w
			aY = leftY
			leftY += ai.h + vSpacing*0.35
		} else {
			rightY += ai.h + vSpacing*0.35
		}
		e.shiftSubtree(ai.node, aLeft, aY)
		if aLeft < minX {
			minX = aLeft
		}
		if rx := aLeft + ai.w; rx > maxX {
			maxX = rx
		}
		if by := aY + ai.h; by > assistantBottom {
			assistantBottom = by
		}
	}

	// Place regular children in a row below both the parent and any assistant
	// stack. Each child subtree keeps its own natural width, which avoids overlap
	// for unbalanced hierarchies.
	childTop := math.Max(nh, assistantBottom) + vSpacing
	rowW := regularWidth
	rowStart := (nw - rowW) / 2
	x := rowStart
	for i, ci := range infos {
		if i > 0 {
			x += hSpacing
		}
		e.shiftSubtree(ci.node, x, childTop)
		if x < minX {
			minX = x
		}
		if rx := x + ci.w; rx > maxX {
			maxX = rx
		}
		if by := childTop + ci.h; by > maxY {
			maxY = by
		}
		x += ci.w
	}
	if assistantBottom > maxY {
		maxY = assistantBottom
	}

	width = maxX - minX
	height = maxY
	nodeOffsetX = -minX

	// Center the parent over the subtree bounding box. The node is positioned
	// relative to the original origin; shiftSubtree then normalizes the left
	// edge of the subtree to zero.
	node.Position.X = minX + (width/2 - nw/2)
	node.Position.Y = 0
	e.shiftSubtree(node, nodeOffsetX, 0)

	return width, height, node.Position.X
}

// shiftSubtree adds dx/dy to every node in the subtree rooted at node.
func (e *LayoutEngine) shiftSubtree(node *smartart.Node, dx, dy float64) {
	node.Position.X += dx
	node.Position.Y += dy
	for _, child := range node.Children {
		e.shiftSubtree(child, dx, dy)
	}
}

// applyDirection transforms normalized top-down coordinates into the requested
// hierarchy direction. bboxW and bboxH describe the bounding box of the current
// positions.
func (e *LayoutEngine) applyDirection(direction string, bboxW, bboxH float64) {
	if direction == "" {
		direction = "topDown"
	}
	if direction == "topDown" {
		return
	}

	minX, minY := e.nodeBounds()

	for _, node := range e.Diagram.Nodes {
		x := node.Position.X - minX
		y := node.Position.Y - minY
		w := node.Position.Width
		h := node.Position.Height

		switch direction {
		case "bottomUp":
			node.Position.X = minX + x
			node.Position.Y = minY + (bboxH - (y + h))
		case "leftRight":
			node.Position.X = minX + y
			node.Position.Y = minY + (bboxW - (x + w))
			node.Position.Width = h
			node.Position.Height = w
		case "rightLeft":
			node.Position.X = minX + (bboxH - (y + h))
			node.Position.Y = minY + x
			node.Position.Width = h
			node.Position.Height = w
		}
	}
}

// fitToFrame scales and translates the computed layout so it fits inside the
// engine's FrameBounds (or the 800x600 pt fallback canvas). Uniform scaling is
// used to preserve aspect ratio.
func (e *LayoutEngine) fitToFrame() {
	minX, minY, maxX, maxY := e.nodeBoundsExt()
	if maxX <= minX || maxY <= minY {
		return
	}
	w := maxX - minX
	h := maxY - minY

	targetX, targetY, targetW, targetH := 0.0, 0.0, 800.0, 600.0
	margin := 10.0
	if e.FrameBounds != nil {
		targetX = e.FrameBounds.X
		targetY = e.FrameBounds.Y
		targetW = e.FrameBounds.W
		targetH = e.FrameBounds.H
	}

	scale := (targetW - 2*margin) / w
	if (targetH-2*margin)/h < scale {
		scale = (targetH - 2*margin) / h
	}
	if scale <= 0 {
		return
	}

	offsetX := targetX + (targetW-w*scale)/2
	offsetY := targetY + (targetH-h*scale)/2

	for _, node := range e.Diagram.Nodes {
		node.Position.X = (node.Position.X-minX)*scale + offsetX
		node.Position.Y = (node.Position.Y-minY)*scale + offsetY
		node.Position.Width *= scale
		node.Position.Height *= scale
	}
}

// nodeBounds returns the minimum X and Y of all nodes.
func (e *LayoutEngine) nodeBounds() (minX, minY float64) {
	if len(e.Diagram.Nodes) == 0 {
		return 0, 0
	}
	minX = e.Diagram.Nodes[0].Position.X
	minY = e.Diagram.Nodes[0].Position.Y
	for _, node := range e.Diagram.Nodes {
		if node.Position.X < minX {
			minX = node.Position.X
		}
		if node.Position.Y < minY {
			minY = node.Position.Y
		}
	}
	return
}

// nodeBoundsExt returns the full bounding box of all nodes.
func (e *LayoutEngine) nodeBoundsExt() (minX, minY, maxX, maxY float64) {
	if len(e.Diagram.Nodes) == 0 {
		return 0, 0, 0, 0
	}
	minX = e.Diagram.Nodes[0].Position.X
	minY = e.Diagram.Nodes[0].Position.Y
	maxX = minX + e.Diagram.Nodes[0].Position.Width
	maxY = minY + e.Diagram.Nodes[0].Position.Height
	for _, node := range e.Diagram.Nodes {
		if node.Position.X < minX {
			minX = node.Position.X
		}
		if node.Position.Y < minY {
			minY = node.Position.Y
		}
		if rx := node.Position.X + node.Position.Width; rx > maxX {
			maxX = rx
		}
		if by := node.Position.Y + node.Position.Height; by > maxY {
			maxY = by
		}
	}
	return
}

func (e *LayoutEngine) hSpacing(layout *smartart.Layout) float64 {
	if layout != nil && layout.HSpacing > 0 {
		return layout.HSpacing
	}
	return 50.0
}

func (e *LayoutEngine) vSpacing(layout *smartart.Layout) float64 {
	if layout != nil && layout.VSpacing > 0 {
		return layout.VSpacing
	}
	return 80.0
}

// calculateProcessPositions lays out nodes in a flowchart.
func (e *LayoutEngine) calculateProcessPositions() {
	layout := e.Diagram.Layout
	startX, startY := 50.0, 50.0

	for i, node := range e.Diagram.Nodes {
		node.Position = smartart.Position{
			X:      startX + float64(i%4)*200.0,
			Y:      startY + float64(i/4)*layout.VSpacing,
			Width:  150,
			Height: 60,
		}
	}
}

// calculateCyclePositions lays out nodes in a circle.
func (e *LayoutEngine) calculateCyclePositions() {
	centerX, centerY := 400.0, 300.0
	radius := 200.0
	nodeCount := len(e.Diagram.Nodes)

	for i, node := range e.Diagram.Nodes {
		angle := float64(i) * 2 * 3.14159 / float64(nodeCount)
		node.Position = smartart.Position{
			X:      centerX + radius*cos(angle) - 75,
			Y:      centerY + radius*sin(angle) - 30,
			Width:  150,
			Height: 60,
		}
	}
}

// calculateMatrixPositions lays out nodes in a grid.
func (e *LayoutEngine) calculateMatrixPositions() {
	cols := 3
	if len(e.Diagram.Nodes) < 3 {
		cols = len(e.Diagram.Nodes)
	}

	startX, startY := 100.0, 100.0

	for i, node := range e.Diagram.Nodes {
		row := i / cols
		col := i % cols

		node.Position = smartart.Position{
			X:      startX + float64(col)*200.0,
			Y:      startY + float64(row)*120.0,
			Width:  150,
			Height: 80,
		}
	}
}

// calculateListPositions lays out nodes vertically.
func (e *LayoutEngine) calculateListPositions() {
	startX, startY := 100.0, 50.0
	spacing := 80.0

	for i, node := range e.Diagram.Nodes {
		node.Position = smartart.Position{
			X:      startX,
			Y:      startY + float64(i)*spacing,
			Width:  600,
			Height: 50,
		}
	}
}

// ToShapes converts the diagram to positioned shapes.
func (e *LayoutEngine) ToShapes() []*Shape {
	// If any node already has a fixed position from a pre-rendered drawing part,
	// skip the heuristic layout engine so those positions are preserved.
	hasFixed := false
	for _, node := range e.Diagram.Nodes {
		if node.HasPosition {
			hasFixed = true
			break
		}
	}
	if !hasFixed {
		e.CalculatePositions()
	}

	nodeToShape := make(map[*smartart.Node]*Shape)
	for _, node := range e.Diagram.Nodes {
		shape := &Shape{
			ID:          node.ID,
			Type:        e.Diagram.Style.ShapeType,
			Text:        node.Text,
			X:           node.Position.X,
			Y:           node.Position.Y,
			Width:       node.Position.Width,
			Height:      node.Position.Height,
			RotationDeg: node.RotationDeg,
			FlipH:       node.FlipH,
			FlipV:       node.FlipV,
			Fill:        e.Diagram.Style.Fill,
			Line:        e.Diagram.Style.Line,
			Effect:      e.Diagram.Style.Effect,
			TextColor:   e.Diagram.Style.TextColor,
		}

		// Allow the pre-rendered drawing part to override style and geometry.
		if node.Geometry != "" {
			shape.Type = node.Geometry
		}
		if len(node.GeometryAdjustments) > 0 {
			shape.GeometryAdjustments = node.GeometryAdjustments
		}
		if node.CustomGeometry != nil {
			shape.CustomGeometry = node.CustomGeometry
		}
		if node.DrawingOrder != 0 {
			shape.DrawingOrder = node.DrawingOrder
		}
		if node.Fill != nil {
			shape.Fill = node.Fill
		}
		if node.Line != nil {
			shape.Line = node.Line
		}
		if len(node.Effect.Effects) > 0 {
			shape.Effect = node.Effect
		}
		if node.TextColor != "" {
			shape.TextColor = node.TextColor
		}
		shape.TextBodyProps = node.TextBodyProps
		shape.PlaceholderText = node.PropertySet.PlaceholderText
		if len(node.Paragraphs) > 0 {
			shape.Paragraphs = node.Paragraphs
		}

		// Vary color by level for hierarchy when no drawing override exists.
		if e.Diagram.Layout.Type == "hierarchy" && node.Fill == nil {
			colors := e.Diagram.Style.ColorPalette
			if len(colors) == 0 {
				colors = []string{"4472C4", "5B9BD5", "70AD47", "FFC000", "ED7D31"}
			}
			if node.Level < len(colors) {
				shape.Fill = drawing.SolidFill{Color: colors[node.Level], Alpha: 1.0}
			}
		}

		// Apply per-level line colors from the colorsDef part when available.
		if e.Diagram.Layout.Type == "hierarchy" && node.Line == nil && shape.Line != nil && len(e.Diagram.Style.LinePalette) > 0 {
			if node.Level < len(e.Diagram.Style.LinePalette) {
				shape.Line.Color = e.Diagram.Style.LinePalette[node.Level]
			}
		}

		nodeToShape[node] = shape
		e.Shapes = append(e.Shapes, shape)
	}

	// Build connections from parent-child relationships.
	for _, node := range e.Diagram.Nodes {
		shape := nodeToShape[node]
		for _, child := range node.Children {
			if childShape, ok := nodeToShape[child]; ok {
				shape.Connections = append(shape.Connections, Connection{ToID: childShape.ID})
			}
		}
	}

	return e.Shapes
}

// Helper functions
func cos(angle float64) float64 {
	return math.Cos(angle)
}

func sin(angle float64) float64 {
	return math.Sin(angle)
}

// calculateRadialPositions lays out the first node at the center and the rest
// arranged in a circle around it.
func (e *LayoutEngine) calculateRadialPositions() {
	centerX, centerY := 400.0, 300.0
	radius := 200.0
	nodeCount := len(e.Diagram.Nodes)
	if nodeCount == 0 {
		return
	}

	e.Diagram.Nodes[0].Position = smartart.Position{
		X:      centerX - 75,
		Y:      centerY - 30,
		Width:  150,
		Height: 60,
	}

	for i := 1; i < nodeCount; i++ {
		angle := float64(i-1) * 2 * 3.14159 / float64(nodeCount-1)
		e.Diagram.Nodes[i].Position = smartart.Position{
			X:      centerX + radius*cos(angle) - 75,
			Y:      centerY + radius*sin(angle) - 30,
			Width:  150,
			Height: 60,
		}
	}
}

// calculateBulletPositions lays out nodes as a bulleted list with indentation
// based on node level.
func (e *LayoutEngine) calculateBulletPositions() {
	startX, startY := 100.0, 50.0
	spacing := 80.0
	indent := 40.0

	for i, node := range e.Diagram.Nodes {
		x := startX + float64(node.Level)*indent
		width := 600.0 - float64(node.Level)*indent
		if width < 100 {
			width = 100
		}
		node.Position = smartart.Position{
			X:      x,
			Y:      startY + float64(i)*spacing,
			Width:  width,
			Height: 50,
		}
	}
}

// calculatePyramidPositions lays out nodes in stacked, centered rows.  Higher
// levels (the top of the pyramid) are narrower; deeper levels are wider.  Each
// row is centered horizontally so the pyramid is symmetrical.
func (e *LayoutEngine) calculatePyramidPositions() {
	nodesByLevel := make(map[int][]*smartart.Node)
	maxLevel := 0
	for _, node := range e.Diagram.Nodes {
		nodesByLevel[node.Level] = append(nodesByLevel[node.Level], node)
		if node.Level > maxLevel {
			maxLevel = node.Level
		}
	}

	startY := 50.0
	vSpacing := 80.0
	baseWidth := 600.0
	levelShrink := 100.0

	for level := 0; ; level++ {
		nodes, ok := nodesByLevel[level]
		if !ok {
			break
		}

		rowWidth := baseWidth - float64(maxLevel-level)*levelShrink
		if rowWidth < 120 {
			rowWidth = 120
		}
		startX := (800.0 - rowWidth) / 2
		cellW := rowWidth / float64(len(nodes))

		for i, node := range nodes {
			shapeW := cellW - 10
			node.Position = smartart.Position{
				X:      startX + float64(i)*cellW + (cellW-shapeW)/2,
				Y:      startY + float64(level)*vSpacing,
				Width:  shapeW,
				Height: 60,
			}
		}
	}
}

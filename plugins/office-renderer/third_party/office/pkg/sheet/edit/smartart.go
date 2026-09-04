package edit

import (
	"fmt"
	"strings"

	"office/pkg/ooxml/drawing"
	"office/pkg/ooxml/smartart"
	sheetir "office/pkg/sheet/ir"
)

const (
	smartArtDiagramDataRelType = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/diagramData"
	smartArtDiagramDataCT      = "application/vnd.openxmlformats-officedocument.drawingml.diagramData+xml"
)

// SmartArt is an editable SmartArt diagram hosted by a worksheet drawing.
type SmartArt struct {
	sheet   *Sheet
	drawing *sheetir.Drawing
	ir      *sheetir.DrawingSmartArt
}

// SmartArtNode is an editable data point in a worksheet SmartArt diagram.
type SmartArtNode struct {
	art *SmartArt
	ir  *sheetir.SmartArtNode
}

// SmartArts returns all editable SmartArt diagrams on the worksheet.
func (s *Sheet) SmartArts() []*SmartArt {
	if s == nil || s.sheet == nil {
		return nil
	}
	out := make([]*SmartArt, 0)
	for _, d := range s.sheet.Drawings {
		if d == nil || d.Type != sheetir.DrawingTypeSmartArt || d.SmartArt == nil {
			continue
		}
		out = append(out, &SmartArt{sheet: s, drawing: d, ir: d.SmartArt})
	}
	return out
}

// AddSmartArt creates a new semantic SmartArt diagram. New diagrams use a
// data-model-only SmartArt package; Excel and the HTML renderer can lay it out
// without requiring a producer-specific pre-rendered diagram drawing.
func (s *Sheet) AddSmartArt(name, diagramType string, position sheetir.DrawingPosition) (*SmartArt, error) {
	if s == nil || s.sheet == nil {
		return nil, fmt.Errorf("sheet edit: nil sheet")
	}
	if err := validateDrawingPosition(position); err != nil {
		return nil, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = s.nextDrawingName("Diagram")
	}
	diagramType = strings.TrimSpace(diagramType)
	if diagramType == "" {
		diagramType = "hierarchy"
	}

	relID := s.nextSmartArtRelID()
	partName := s.nextSmartArtPartName()
	diagram := &smartart.Diagram{
		Type:   diagramType,
		Layout: &smartart.Layout{Type: diagramType, Direction: "topDown", HSpacing: 50, VSpacing: 80},
		Style: &smartart.Style{
			Fill:      drawing.SolidFill{Color: "4472C4", Alpha: 1},
			Line:      &drawing.LineStyle{Color: "2F5597", Width: 1.5, Alpha: 1},
			TextColor: "FFFFFF",
			ShapeType: "rectangle",
		},
	}
	data := smartart.MarshalDataModel(diagram)
	artIR := &sheetir.DrawingSmartArt{
		DataRelID:     relID,
		DiagramType:   diagramType,
		SemanticDirty: false,
		Parts: map[string]sheetir.DrawingObjectPart{
			relID: {
				RelID:       relID,
				RelType:     smartArtDiagramDataRelType,
				PartName:    partName,
				ContentType: smartArtDiagramDataCT,
				Target:      "../diagrams/" + strings.TrimPrefix(partName, "xl/diagrams/"),
				Data:        data,
			},
		},
	}
	d := &sheetir.Drawing{Name: name, Type: sheetir.DrawingTypeSmartArt, Position: position, SmartArt: artIR}
	s.sheet.Drawings = append(s.sheet.Drawings, d)
	return &SmartArt{sheet: s, drawing: d, ir: artIR}, nil
}

func (s *Sheet) nextSmartArtPartName() string {
	used := map[string]bool{}
	for _, d := range s.sheet.Drawings {
		if d == nil || d.SmartArt == nil {
			continue
		}
		for _, part := range d.SmartArt.Parts {
			if part.PartName != "" {
				used[part.PartName] = true
			}
		}
	}
	for i := 1; ; i++ {
		name := fmt.Sprintf("xl/diagrams/data%d.xml", i)
		if !used[name] {
			return name
		}
	}
}

func (s *Sheet) nextSmartArtRelID() string {
	used := map[string]bool{}
	for _, d := range s.sheet.Drawings {
		if d != nil && d.SmartArt != nil {
			for id := range d.SmartArt.Parts {
				used[id] = true
			}
		}
	}
	for i := 1; ; i++ {
		id := fmt.Sprintf("rIdSmartArtData%d", i)
		if !used[id] {
			return id
		}
	}
}

// Name returns the hosting drawing name.
func (a *SmartArt) Name() string {
	if a == nil || a.drawing == nil {
		return ""
	}
	return a.drawing.Name
}

// Type returns the resolved SmartArt layout category.
func (a *SmartArt) Type() string {
	if a == nil || a.ir == nil {
		return ""
	}
	return a.ir.DiagramType
}

// SetType changes the fallback layout category used by semantic rendering.
func (a *SmartArt) SetType(diagramType string) {
	if a == nil || a.ir == nil {
		return
	}
	a.ir.DiagramType = strings.TrimSpace(diagramType)
	a.markDirty()
}

// Position returns the diagram's worksheet drawing position.
func (a *SmartArt) Position() sheetir.DrawingPosition {
	if a == nil || a.drawing == nil {
		return sheetir.DrawingPosition{}
	}
	return a.drawing.Position
}

// SetPosition moves/resizes the hosting SmartArt frame.
func (a *SmartArt) SetPosition(position sheetir.DrawingPosition) error {
	if a == nil || a.drawing == nil {
		return fmt.Errorf("sheet edit: nil SmartArt")
	}
	if err := validateDrawingPosition(position); err != nil {
		return err
	}
	a.drawing.Position = position
	return nil
}

// Nodes returns diagram nodes in declaration order.
func (a *SmartArt) Nodes() []*SmartArtNode {
	if a == nil || a.ir == nil {
		return nil
	}
	out := make([]*SmartArtNode, 0, len(a.ir.Nodes))
	for _, n := range a.ir.Nodes {
		if n != nil {
			out = append(out, &SmartArtNode{art: a, ir: n})
		}
	}
	return out
}

// NodeByID finds a node by its model identifier.
func (a *SmartArt) NodeByID(id string) *SmartArtNode {
	if a == nil || a.ir == nil {
		return nil
	}
	for _, n := range a.ir.Nodes {
		if n != nil && n.ID == id {
			return &SmartArtNode{art: a, ir: n}
		}
	}
	return nil
}

// AddRoot adds a top-level SmartArt node.
func (a *SmartArt) AddRoot(text string) *SmartArtNode { return a.AddNode(nil, text) }

// AddNode adds a SmartArt data point and optionally connects it to parent.
func (a *SmartArt) AddNode(parent *SmartArtNode, text string) *SmartArtNode {
	if a == nil || a.ir == nil {
		return nil
	}
	id := a.nextNodeID()
	typ := "node"
	if parent == nil {
		typ = "root"
	}
	n := &sheetir.SmartArtNode{ID: id, Type: typ, Text: text}
	a.ir.Nodes = append(a.ir.Nodes, n)
	if parent != nil && parent.ir != nil {
		a.ir.Connections = append(a.ir.Connections, sheetir.SmartArtConnection{SrcID: parent.ir.ID, DestID: id})
		parent.ir.Children = append(parent.ir.Children, id)
	}
	a.markDirty()
	return &SmartArtNode{art: a, ir: n}
}

func (a *SmartArt) nextNodeID() string {
	used := map[string]bool{}
	for _, n := range a.ir.Nodes {
		if n != nil {
			used[n.ID] = true
		}
	}
	for i := 1; ; i++ {
		id := fmt.Sprintf("pt%d", i)
		if !used[id] {
			return id
		}
	}
}

// RemoveNode removes a node and every connection referencing it.
func (a *SmartArt) RemoveNode(node *SmartArtNode) bool {
	if a == nil || a.ir == nil || node == nil || node.ir == nil {
		return false
	}
	id := node.ir.ID
	idx := -1
	for i, n := range a.ir.Nodes {
		if n == node.ir || (n != nil && n.ID == id) {
			idx = i
			break
		}
	}
	if idx < 0 {
		return false
	}
	a.ir.Nodes = append(a.ir.Nodes[:idx], a.ir.Nodes[idx+1:]...)
	connections := a.ir.Connections[:0]
	for _, c := range a.ir.Connections {
		if c.SrcID != id && c.DestID != id {
			connections = append(connections, c)
		}
	}
	a.ir.Connections = connections
	for _, n := range a.ir.Nodes {
		if n == nil {
			continue
		}
		children := n.Children[:0]
		for _, child := range n.Children {
			if child != id {
				children = append(children, child)
			}
		}
		n.Children = children
	}
	a.markDirty()
	return true
}

// MoveNode changes declaration order without changing graph edges.
func (a *SmartArt) MoveNode(from, to int) bool {
	if a == nil || a.ir == nil || from < 0 || from >= len(a.ir.Nodes) {
		return false
	}
	if to < 0 {
		to = 0
	}
	if to >= len(a.ir.Nodes) {
		to = len(a.ir.Nodes) - 1
	}
	if from == to {
		return true
	}
	node := a.ir.Nodes[from]
	a.ir.Nodes = append(a.ir.Nodes[:from], a.ir.Nodes[from+1:]...)
	a.ir.Nodes = append(a.ir.Nodes, nil)
	copy(a.ir.Nodes[to+1:], a.ir.Nodes[to:])
	a.ir.Nodes[to] = node
	a.markDirty()
	return true
}

func (a *SmartArt) markDirty() {
	if a != nil && a.ir != nil {
		a.ir.SemanticDirty = true
	}
}

// ID returns the node's SmartArt model identifier.
func (n *SmartArtNode) ID() string {
	if n == nil || n.ir == nil {
		return ""
	}
	return n.ir.ID
}

// Text returns the displayed node text.
func (n *SmartArtNode) Text() string {
	if n == nil || n.ir == nil {
		return ""
	}
	return n.ir.Text
}

// SetText changes the node text.
func (n *SmartArtNode) SetText(text string) {
	if n == nil || n.ir == nil {
		return
	}
	n.ir.Text = text
	n.art.markDirty()
}

// Type returns the SmartArt point type (root, node, assistant, etc.).
func (n *SmartArtNode) Type() string {
	if n == nil || n.ir == nil {
		return ""
	}
	return n.ir.Type
}

// SetType changes the SmartArt point type.
func (n *SmartArtNode) SetType(typ string) {
	if n == nil || n.ir == nil {
		return
	}
	n.ir.Type = strings.TrimSpace(typ)
	n.art.markDirty()
}

// SetFillColor applies a node-level solid fill override.
func (n *SmartArtNode) SetFillColor(hex string) {
	if n == nil || n.ir == nil {
		return
	}
	n.ir.FillColor = normalizeSmartArtColor(hex)
	n.art.markDirty()
}

// SetTextColor applies a node-level text color override.
func (n *SmartArtNode) SetTextColor(hex string) {
	if n == nil || n.ir == nil {
		return
	}
	n.ir.TextColor = normalizeSmartArtColor(hex)
	n.art.markDirty()
}

// AddChild adds a child node and connection.
func (n *SmartArtNode) AddChild(text string) *SmartArtNode {
	if n == nil || n.art == nil || n.ir == nil {
		return nil
	}
	return n.art.AddNode(n, text)
}

func normalizeSmartArtColor(value string) string {
	return strings.TrimPrefix(strings.TrimSpace(value), "#")
}

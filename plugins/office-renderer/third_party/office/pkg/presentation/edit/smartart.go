package edit

import (
	"fmt"
	"strings"

	presir "office/pkg/presentation/ir"
)

// SmartArt is an editable SmartArt diagram on a slide.
type SmartArt struct {
	slide *Slide
	ir    *presir.SmartArt
}

// SmartArtNode is an editable data point inside a SmartArt diagram.
type SmartArtNode struct {
	art *SmartArt
	ir  *presir.SmartArtNode
}

// NewSmartArt creates a new SmartArt diagram with the given category and frame.
// Common diagram types are "hierarchy", "process", "cycle", "radial", and "matrix".
func NewSmartArt(diagramType string, x, y, w, h int64) *SmartArt {
	return &SmartArt{
		ir: &presir.SmartArt{
			Type: diagramType,
			X:    x,
			Y:    y,
			W:    w,
			H:    h,
		},
	}
}

// Type returns the diagram category.
func (a *SmartArt) Type() string {
	if a == nil || a.ir == nil {
		return ""
	}
	return a.ir.Type
}

// SetType changes the diagram category.
func (a *SmartArt) SetType(diagramType string) {
	if a == nil || a.ir == nil {
		return
	}
	a.ir.Type = diagramType
}

// SetLayout selects the SmartArt layout identifier used for generated diagrams.
func (a *SmartArt) SetLayout(layoutID string) {
	if a == nil || a.ir == nil {
		return
	}
	a.ir.LayoutID = layoutID
}

// Layout returns the selected SmartArt layout identifier.
func (a *SmartArt) Layout() string {
	if a == nil || a.ir == nil {
		return ""
	}
	return a.ir.LayoutID
}

// SetQuickStyle selects the SmartArt quick-style identifier.
func (a *SmartArt) SetQuickStyle(styleID string) {
	if a == nil || a.ir == nil {
		return
	}
	a.ir.QuickStyleID = styleID
}

// SetColorStyle selects the SmartArt color-style identifier.
func (a *SmartArt) SetColorStyle(colorID string) {
	if a == nil || a.ir == nil {
		return
	}
	a.ir.ColorID = colorID
}

// SetColors sets diagram-wide fill, line, and text color overrides.
func (a *SmartArt) SetColors(fill, line, text string) {
	if a == nil || a.ir == nil {
		return
	}
	a.ir.FillColor = normalizeSmartArtHex(fill)
	a.ir.LineColor = normalizeSmartArtHex(line)
	a.ir.TextColor = normalizeSmartArtHex(text)
}

// Bounds returns the diagram frame in EMU.
func (a *SmartArt) Bounds() (x, y, w, h int64) {
	if a == nil || a.ir == nil {
		return
	}
	return a.ir.X, a.ir.Y, a.ir.W, a.ir.H
}

// SetBounds sets the diagram frame in EMU.
func (a *SmartArt) SetBounds(x, y, w, h int64) {
	if a == nil || a.ir == nil {
		return
	}
	a.ir.X, a.ir.Y, a.ir.W, a.ir.H = x, y, w, h
}

// Nodes returns the diagram nodes in declaration order.
func (a *SmartArt) Nodes() []*SmartArtNode {
	if a == nil || a.ir == nil {
		return nil
	}
	out := make([]*SmartArtNode, len(a.ir.Nodes))
	for i, n := range a.ir.Nodes {
		out[i] = &SmartArtNode{art: a, ir: n}
	}
	return out
}

// NodeByID returns the node with the given ID, or nil.
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

// AddRoot adds a top-level node and returns it. It is a convenience wrapper
// around AddNode with a nil parent.
func (a *SmartArt) AddRoot(text string) *SmartArtNode {
	return a.AddNode(nil, text)
}

// AddNode adds a data point to the diagram. If parent is nil, the node is the
// diagram root; otherwise an edge from parent to the new node is recorded.
func (a *SmartArt) AddNode(parent *SmartArtNode, text string) *SmartArtNode {
	if a == nil || a.ir == nil {
		return nil
	}
	id := fmt.Sprintf("pt%d", len(a.ir.Nodes)+1)
	nodeType := "node"
	if parent == nil {
		nodeType = "root"
	}
	irNode := &presir.SmartArtNode{
		ID:   id,
		Type: nodeType,
		Text: text,
	}
	a.ir.Nodes = append(a.ir.Nodes, irNode)
	if parent != nil && parent.ir != nil {
		a.ir.Connections = append(a.ir.Connections, presir.SmartArtConnection{
			SrcID:  parent.ir.ID,
			DestID: id,
		})
		parent.ir.Children = append(parent.ir.Children, id)
	}
	return &SmartArtNode{art: a, ir: irNode}
}

// RemoveNode removes a node and all edges that reference it.
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
	filtered := a.ir.Connections[:0]
	for _, c := range a.ir.Connections {
		if c.SrcID != id && c.DestID != id {
			filtered = append(filtered, c)
		}
	}
	a.ir.Connections = filtered
	for _, n := range a.ir.Nodes {
		if n == nil {
			continue
		}
		children := n.Children[:0]
		for _, childID := range n.Children {
			if childID != id {
				children = append(children, childID)
			}
		}
		n.Children = children
	}
	return true
}

// MoveNode moves a node in declaration order.
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
	a.ir.Nodes = append(a.ir.Nodes[:to], append([]*presir.SmartArtNode{node}, a.ir.Nodes[to:]...)...)
	return true
}

// ID returns the node's identifier.
func (n *SmartArtNode) ID() string {
	if n == nil || n.ir == nil {
		return ""
	}
	return n.ir.ID
}

// Text returns the node's display text.
func (n *SmartArtNode) Text() string {
	if n == nil || n.ir == nil {
		return ""
	}
	return n.ir.Text
}

// SetText changes the node's display text.
func (n *SmartArtNode) SetText(text string) {
	if n == nil || n.ir == nil {
		return
	}
	n.ir.Text = text
}

// SetFillColor sets an optional node-level fill color override.
func (n *SmartArtNode) SetFillColor(hex string) {
	if n == nil || n.ir == nil {
		return
	}
	n.ir.FillColor = normalizeSmartArtHex(hex)
}

// SetTextColor sets an optional node-level text color override.
func (n *SmartArtNode) SetTextColor(hex string) {
	if n == nil || n.ir == nil {
		return
	}
	n.ir.TextColor = normalizeSmartArtHex(hex)
}

// Type returns the node type.
func (n *SmartArtNode) Type() string {
	if n == nil || n.ir == nil {
		return ""
	}
	return n.ir.Type
}

// SetType changes the node type (e.g. "root", "node", "assistant").
func (n *SmartArtNode) SetType(t string) {
	if n == nil || n.ir == nil {
		return
	}
	n.ir.Type = t
}

// AddChild adds a child node and records the parent-child connection.
func (n *SmartArtNode) AddChild(text string) *SmartArtNode {
	if n == nil || n.art == nil || n.ir == nil {
		return nil
	}
	return n.art.AddNode(n, text)
}

func normalizeSmartArtHex(hex string) string {
	return strings.TrimPrefix(strings.TrimSpace(hex), "#")
}
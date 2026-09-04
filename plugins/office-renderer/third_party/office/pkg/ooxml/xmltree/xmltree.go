package xmltree

import (
	"bytes"
	"encoding/xml"
	"io"
	"strings"
)

// Node is a low-level XML element tree.
type Node struct {
	Name     xml.Name
	Attr     []xml.Attr
	Text     string
	Children []*Node
}

// Parse parses an XML payload into a generic node tree.
func Parse(data []byte) (*Node, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	for {
		token, err := dec.Token()
		if err != nil {
			if err == io.EOF {
				return nil, io.ErrUnexpectedEOF
			}
			return nil, err
		}

		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}

		return parseElement(dec, start)
	}
}

// AttrValue returns the first attribute value matching the requested local name.
func (n *Node) AttrValue(local string) (string, bool) {
	if n == nil {
		return "", false
	}
	for _, attr := range n.Attr {
		if attr.Name.Local == local {
			return attr.Value, true
		}
	}
	return "", false
}

// Child returns the first child whose local name matches.
func (n *Node) Child(local string) *Node {
	if n == nil {
		return nil
	}
	for _, child := range n.Children {
		if child.Name.Local == local {
			return child
		}
	}
	return nil
}

// ChildrenNamed returns all direct children whose local name matches.
func (n *Node) ChildrenNamed(local string) []*Node {
	if n == nil {
		return nil
	}
	var out []*Node
	for _, child := range n.Children {
		if child.Name.Local == local {
			out = append(out, child)
		}
	}
	return out
}

// FindAll returns all descendant nodes (at any depth) whose local name matches.
func (n *Node) FindAll(local string) []*Node {
	if n == nil {
		return nil
	}
	var out []*Node
	for _, child := range n.Children {
		if child.Name.Local == local {
			out = append(out, child)
		}
		out = append(out, child.FindAll(local)...)
	}
	return out
}

// AllText returns the concatenation of all text content at any descendant depth.
func (n *Node) AllText() string {
	if n == nil {
		return ""
	}
	if len(n.Children) == 0 {
		return n.Text
	}
	var sb strings.Builder
	if n.Text != "" {
		sb.WriteString(n.Text)
	}
	for _, child := range n.Children {
		sb.WriteString(child.AllText())
	}
	return sb.String()
}

func parseElement(dec *xml.Decoder, start xml.StartElement) (*Node, error) {
	node := &Node{
		Name: start.Name,
		Attr: append([]xml.Attr(nil), start.Attr...),
	}

	for {
		token, err := dec.Token()
		if err != nil {
			return nil, err
		}

		switch current := token.(type) {
		case xml.StartElement:
			child, err := parseElement(dec, current)
			if err != nil {
				return nil, err
			}
			node.Children = append(node.Children, child)
		case xml.EndElement:
			if current.Name == start.Name {
				node.Text = strings.TrimSpace(node.Text)
				return node, nil
			}
		case xml.CharData:
			if len(bytes.TrimSpace(current)) == 0 {
				continue
			}
			node.Text += string(current)
		}
	}
}

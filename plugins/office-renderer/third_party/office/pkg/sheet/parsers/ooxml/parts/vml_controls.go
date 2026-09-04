package parts

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"

	"office/pkg/ooxml/xmltree"
)

const (
	vmlNS             = "urn:schemas-microsoft-com:vml"
	officeNS          = "urn:schemas-microsoft-com:office:office"
	excelNS           = "urn:schemas-microsoft-com:office:excel"
	controlShapeType  = "_x0000_t201"
	commentShapeType  = "_x0000_t202"
	defaultShapeIDMin = 1025
)

// ParseLegacyDrawingVMLObjects extracts generic worksheet VML shapes while
// excluding form controls and comment-note shapes, which have dedicated models.
func ParseLegacyDrawingVMLObjects(raw []byte) []VMLObject {
	root, err := parseOrCreateVMLRoot(raw, false)
	if err != nil || root == nil {
		return nil
	}
	var out []VMLObject
	for _, child := range root.Children {
		if child == nil || child.Name.Local != "shape" || isVMLControlShape(child) || vmlNoteShapeRef(child) != "" {
			continue
		}
		obj := VMLObject{}
		obj.ID, _ = child.AttrValue("id")
		obj.Type, _ = child.AttrValue("type")
		obj.Style, _ = child.AttrValue("style")
		obj.FillColor, _ = child.AttrValue("fillcolor")
		obj.StrokeColor, _ = child.AttrValue("strokecolor")
		if textbox := child.Child("textbox"); textbox != nil {
			obj.Text = strings.TrimSpace(textbox.AllText())
		}
		if client := child.Child("ClientData"); client != nil {
			if anchor := client.Child("Anchor"); anchor != nil {
				obj.Anchor = strings.TrimSpace(anchor.AllText())
			}
		}
		if obj.ID == "" {
			obj.ID = fmt.Sprintf("vml-object-%d", len(out)+1)
		}
		out = append(out, obj)
	}
	return out
}

// MergeLegacyDrawingVMLObjects reconciles generic VML shapes by ID while
// preserving control/note shapes and unknown children/attributes of existing
// generic objects. Objects omitted from the supplied slice are removed.
func MergeLegacyDrawingVMLObjects(raw []byte, objects []VMLObject) []byte {
	create := len(objects) > 0
	root, err := parseOrCreateVMLRoot(raw, create)
	if err != nil || root == nil {
		return nil
	}
	wanted := make(map[string]VMLObject, len(objects))
	order := make([]string, 0, len(objects))
	for i, obj := range objects {
		id := strings.TrimSpace(obj.ID)
		if id == "" {
			id = fmt.Sprintf("_x0000_s%d", nextVMLShapeID(root)+i)
			obj.ID = id
		}
		wanted[id] = obj
		order = append(order, id)
	}
	seen := make(map[string]bool, len(objects))
	children := make([]*xmltree.Node, 0, len(root.Children)+len(objects))
	for _, child := range root.Children {
		if child == nil || child.Name.Local != "shape" || isVMLControlShape(child) || vmlNoteShapeRef(child) != "" {
			children = append(children, child)
			continue
		}
		id, _ := child.AttrValue("id")
		obj, keep := wanted[strings.TrimSpace(id)]
		if !keep {
			continue
		}
		applyVMLObjectNode(child, obj)
		children = append(children, child)
		seen[obj.ID] = true
	}
	for _, id := range order {
		if seen[id] {
			continue
		}
		obj := wanted[id]
		children = append(children, newVMLObjectNode(obj))
	}
	root.Children = children
	data, err := marshalXMLTree(root)
	if err != nil {
		return nil
	}
	return data
}

func applyVMLObjectNode(node *xmltree.Node, obj VMLObject) {
	if node == nil {
		return
	}
	setVMLAttr(node, "id", obj.ID)
	setVMLAttr(node, "type", obj.Type)
	setVMLAttr(node, "style", obj.Style)
	setVMLAttr(node, "fillcolor", obj.FillColor)
	setVMLAttr(node, "strokecolor", obj.StrokeColor)
	if obj.Text != "" {
		textbox := node.Child("textbox")
		if textbox == nil {
			textbox = &xmltree.Node{Name: xml.Name{Space: vmlNS, Local: "textbox"}}
			node.Children = append(node.Children, textbox)
		}
		textbox.Text = ""
		textbox.Children = []*xmltree.Node{{Name: xml.Name{Local: "div"}, Text: obj.Text}}
	}
	if obj.Anchor != "" {
		client := node.Child("ClientData")
		if client == nil {
			client = &xmltree.Node{Name: xml.Name{Space: excelNS, Local: "ClientData"}}
			node.Children = append(node.Children, client)
		}
		anchor := client.Child("Anchor")
		if anchor == nil {
			anchor = &xmltree.Node{Name: xml.Name{Space: excelNS, Local: "Anchor"}}
			client.Children = append(client.Children, anchor)
		}
		anchor.Text = obj.Anchor
		anchor.Children = nil
	}
}

func newVMLObjectNode(obj VMLObject) *xmltree.Node {
	node := &xmltree.Node{Name: xml.Name{Space: vmlNS, Local: "shape"}}
	applyVMLObjectNode(node, obj)
	return node
}

func setVMLAttr(node *xmltree.Node, local, value string) {
	if node == nil || value == "" {
		return
	}
	for i := range node.Attr {
		if node.Attr[i].Name.Local == local {
			node.Attr[i].Value = value
			return
		}
	}
	node.Attr = append(node.Attr, xml.Attr{Name: xml.Name{Local: local}, Value: value})
}

// MergeLegacyDrawingVML merges worksheet VML form controls into an existing
// legacy drawing part while preserving non-control VML content.
func MergeLegacyDrawingVML(raw []byte, controls []FormControl) []byte {
	return mergeLegacyDrawingVML(raw, controls, nil, true)
}

// MergeLegacyDrawingVMLForWorksheet reconciles worksheet VML based on the
// current form controls and comment list. Existing note shapes for live comment
// refs are preserved when present; missing ones are synthesized; stale note
// shapes are removed.
func MergeLegacyDrawingVMLForWorksheet(raw []byte, controls []FormControl, comments *Comments) []byte {
	return mergeLegacyDrawingVML(raw, controls, commentRefs(comments), false)
}

func mergeLegacyDrawingVML(raw []byte, controls []FormControl, refs []string, preserveExistingNotes bool) []byte {
	controlShapes := make([]*xmltree.Node, 0, len(controls))
	for i := range controls {
		if shape := vmlFormControlShape(&controls[i], i); shape != nil {
			controlShapes = append(controlShapes, shape)
		}
	}
	preserveExistingControls := len(controlShapes) == 0
	create := len(controlShapes) > 0 || len(refs) > 0
	root, err := parseOrCreateVMLRoot(raw, create)
	if err != nil || root == nil {
		return nil
	}
	existingNotes := make(map[string]*xmltree.Node)
	var children []*xmltree.Node
	for _, child := range root.Children {
		if child.Name.Local == "shape" {
			if isVMLControlShape(child) {
				if preserveExistingControls {
					children = append(children, child)
				}
				continue
			}
			if ref := vmlNoteShapeRef(child); ref != "" {
				if preserveExistingNotes {
					children = append(children, child)
				} else if _, exists := existingNotes[ref]; !exists {
					existingNotes[ref] = child
				}
				continue
			}
		}
		children = append(children, child)
	}
	root.Children = children
	var prefix []*xmltree.Node
	if len(refs) > 0 && !hasVMLShapeLayout(root) {
		prefix = append(prefix, defaultVMLShapeLayout())
	}
	if len(refs) > 0 && !hasVMLShapeType(root, commentShapeType) {
		prefix = append(prefix, defaultVMLCommentShapeType())
	}
	if len(controlShapes) > 0 && !hasVMLShapeType(root, controlShapeType) {
		prefix = append(prefix, defaultVMLControlShapeType())
	}
	if len(prefix) > 0 {
		root.Children = append(prefix, root.Children...)
	}
	if !preserveExistingNotes && len(refs) > 0 {
		nextShapeID := nextVMLShapeID(root)
		for _, ref := range refs {
			if shape, ok := existingNotes[ref]; ok {
				root.Children = append(root.Children, shape)
				continue
			}
			shape := vmlCommentShape(ref, nextShapeID)
			if shape == nil {
				continue
			}
			root.Children = append(root.Children, shape)
			nextShapeID++
		}
	}
	root.Children = append(root.Children, controlShapes...)
	data, err := marshalXMLTree(root)
	if err != nil {
		return nil
	}
	return data
}

func parseOrCreateVMLRoot(raw []byte, create bool) (*xmltree.Node, error) {
	if len(raw) > 0 {
		root, err := xmltree.Parse(raw)
		if err == nil && root != nil {
			ensureVMLNamespaces(root)
			return root, nil
		}
		if !create {
			return nil, err
		}
	}
	if !create {
		return nil, nil
	}
	root := &xmltree.Node{Name: xml.Name{Local: "xml"}}
	ensureVMLNamespaces(root)
	return root, nil
}

func ensureVMLNamespaces(root *xmltree.Node) {
	if root == nil {
		return
	}
	ensureXMLNSAttr(root, "v", vmlNS)
	ensureXMLNSAttr(root, "o", officeNS)
	ensureXMLNSAttr(root, "x", excelNS)
}

func ensureXMLNSAttr(node *xmltree.Node, prefix, value string) {
	for _, attr := range node.Attr {
		if attr.Name.Space == "xmlns" && attr.Name.Local == prefix {
			return
		}
	}
	node.Attr = append(node.Attr, xml.Attr{Name: xml.Name{Space: "xmlns", Local: prefix}, Value: value})
}

func hasVMLShapeType(root *xmltree.Node, id string) bool {
	if root == nil {
		return false
	}
	for _, child := range root.Children {
		if child.Name.Local != "shapetype" {
			continue
		}
		for _, attr := range child.Attr {
			if attr.Name.Local == "id" && strings.EqualFold(strings.TrimSpace(attr.Value), id) {
				return true
			}
		}
	}
	return false
}

func hasVMLShapeLayout(root *xmltree.Node) bool {
	if root == nil {
		return false
	}
	for _, child := range root.Children {
		if child.Name.Space == officeNS && child.Name.Local == "shapelayout" {
			return true
		}
	}
	return false
}

func defaultVMLShapeLayout() *xmltree.Node {
	return &xmltree.Node{
		Name: xml.Name{Space: officeNS, Local: "shapelayout"},
		Attr: []xml.Attr{{Name: xml.Name{Space: vmlNS, Local: "ext"}, Value: "edit"}},
		Children: []*xmltree.Node{{
			Name: xml.Name{Space: officeNS, Local: "idmap"},
			Attr: []xml.Attr{
				{Name: xml.Name{Space: vmlNS, Local: "ext"}, Value: "edit"},
				{Name: xml.Name{Local: "data"}, Value: "1"},
			},
		}},
	}
}

func defaultVMLControlShapeType() *xmltree.Node {
	return &xmltree.Node{
		Name: xml.Name{Space: vmlNS, Local: "shapetype"},
		Attr: []xml.Attr{
			{Name: xml.Name{Local: "id"}, Value: controlShapeType},
			{Name: xml.Name{Local: "coordsize"}, Value: "21600,21600"},
			{Name: xml.Name{Space: officeNS, Local: "spt"}, Value: "201"},
			{Name: xml.Name{Local: "path"}, Value: "m,l,21600r21600,l21600,xe"},
		},
		Children: []*xmltree.Node{{
			Name: xml.Name{Space: vmlNS, Local: "stroke"},
			Attr: []xml.Attr{{Name: xml.Name{Local: "joinstyle"}, Value: "miter"}},
		}},
	}
}

func defaultVMLCommentShapeType() *xmltree.Node {
	return &xmltree.Node{
		Name: xml.Name{Space: vmlNS, Local: "shapetype"},
		Attr: []xml.Attr{
			{Name: xml.Name{Local: "id"}, Value: commentShapeType},
			{Name: xml.Name{Local: "coordsize"}, Value: "21600,21600"},
			{Name: xml.Name{Space: officeNS, Local: "spt"}, Value: "202"},
			{Name: xml.Name{Local: "path"}, Value: "m,l,21600r21600,l21600,xe"},
		},
		Children: []*xmltree.Node{
			{
				Name: xml.Name{Space: vmlNS, Local: "stroke"},
				Attr: []xml.Attr{{Name: xml.Name{Local: "joinstyle"}, Value: "miter"}},
			},
			{
				Name: xml.Name{Space: vmlNS, Local: "path"},
				Attr: []xml.Attr{
					{Name: xml.Name{Local: "gradientshapeok"}, Value: "t"},
					{Name: xml.Name{Space: officeNS, Local: "connecttype"}, Value: "rect"},
				},
			},
		},
	}
}

func isVMLControlShape(shape *xmltree.Node) bool {
	if shape == nil || shape.Name.Local != "shape" {
		return false
	}
	for _, child := range shape.Children {
		if child.Name.Local != "ClientData" {
			continue
		}
		for _, attr := range child.Attr {
			if attr.Name.Local == "ObjectType" {
				_, ok := normalizeFormControlType(attr.Value)
				return ok
			}
		}
	}
	return false
}

func vmlNoteShapeRef(shape *xmltree.Node) string {
	if shape == nil || shape.Name.Local != "shape" {
		return ""
	}
	for _, child := range shape.Children {
		if child.Name.Local != "ClientData" {
			continue
		}
		isNote := false
		for _, attr := range child.Attr {
			if attr.Name.Local == "ObjectType" && strings.EqualFold(strings.TrimSpace(attr.Value), "Note") {
				isNote = true
				break
			}
		}
		if !isNote {
			return ""
		}
		var row, col int
		var haveRow, haveCol bool
		for _, grandChild := range child.Children {
			switch grandChild.Name.Local {
			case "Row":
				row, _ = strconv.Atoi(strings.TrimSpace(grandChild.Text))
				haveRow = true
			case "Column":
				col, _ = strconv.Atoi(strings.TrimSpace(grandChild.Text))
				haveCol = true
			}
		}
		if !haveRow || !haveCol || row < 0 || col < 0 {
			return ""
		}
		return noteRefFromRowCol(row, col)
	}
	return ""
}

func vmlFormControlShape(control *FormControl, index int) *xmltree.Node {
	if control == nil {
		return nil
	}
	controlType, ok := normalizeFormControlType(control.Type)
	if !ok {
		return nil
	}
	anchor, ok := normalizeFormControlAnchor(control.Anchor)
	if !ok {
		return nil
	}
	id := "_x0000_s" + strconv.Itoa(2049+index)
	row, col := vmlAnchorStart(anchor)
	clientData := &xmltree.Node{
		Name: xml.Name{Space: excelNS, Local: "ClientData"},
		Attr: []xml.Attr{{Name: xml.Name{Local: "ObjectType"}, Value: controlType}},
		Children: []*xmltree.Node{
			{Name: xml.Name{Space: excelNS, Local: "MoveWithCells"}},
			{Name: xml.Name{Space: excelNS, Local: "SizeWithCells"}},
			{Name: xml.Name{Space: excelNS, Local: "Anchor"}, Text: anchor},
			{Name: xml.Name{Space: excelNS, Local: "Row"}, Text: strconv.Itoa(row)},
			{Name: xml.Name{Space: excelNS, Local: "Column"}, Text: strconv.Itoa(col)},
		},
	}
	if link := strings.TrimSpace(control.FmlaLink); link != "" {
		clientData.Children = append(clientData.Children, &xmltree.Node{Name: xml.Name{Space: excelNS, Local: "FmlaLink"}, Text: link})
	}
	if groupLink := strings.TrimSpace(control.FmlaGroup); groupLink != "" && controlType == "GBox" {
		clientData.Children = append(clientData.Children, &xmltree.Node{Name: xml.Name{Space: excelNS, Local: "FmlaGroup"}, Text: groupLink})
	}
	if sourceRange := strings.TrimSpace(control.FmlaRange); sourceRange != "" {
		clientData.Children = append(clientData.Children, &xmltree.Node{Name: xml.Name{Space: excelNS, Local: "FmlaRange"}, Text: sourceRange})
	}
	if control.Mixed && controlType == "Checkbox" {
		clientData.Children = append(clientData.Children, &xmltree.Node{Name: xml.Name{Space: excelNS, Local: "Checked"}, Text: "Mixed"})
	} else if control.Checked {
		clientData.Children = append(clientData.Children, &xmltree.Node{Name: xml.Name{Space: excelNS, Local: "Checked"}, Text: "1"})
	}
	if control.FirstButton && controlType == "Radio" {
		clientData.Children = append(clientData.Children, &xmltree.Node{Name: xml.Name{Space: excelNS, Local: "FirstButton"}, Text: "True"})
	}
	switch controlType {
	case "Spin", "Scroll":
		clientData.Children = append(clientData.Children,
			&xmltree.Node{Name: xml.Name{Space: excelNS, Local: "Min"}, Text: strconv.Itoa(control.Min)},
			&xmltree.Node{Name: xml.Name{Space: excelNS, Local: "Max"}, Text: strconv.Itoa(control.Max)},
			&xmltree.Node{Name: xml.Name{Space: excelNS, Local: "Val"}, Text: strconv.Itoa(control.Val)},
		)
		if control.Increment != 0 {
			clientData.Children = append(clientData.Children, &xmltree.Node{Name: xml.Name{Space: excelNS, Local: "Inc"}, Text: strconv.Itoa(control.Increment)})
		}
		if control.PageStep != 0 {
			clientData.Children = append(clientData.Children, &xmltree.Node{Name: xml.Name{Space: excelNS, Local: "Page"}, Text: strconv.Itoa(control.PageStep)})
		}
		if control.Horizontal && control.Type == "Scroll" {
			clientData.Children = append(clientData.Children, &xmltree.Node{Name: xml.Name{Space: excelNS, Local: "Horiz"}, Text: "1"})
		}
	case "Drop", "List":
		if controlType == "List" {
			if selType, ok := formControlSelectionTypeVML(control.SelectionType); ok {
				clientData.Children = append(clientData.Children,
					&xmltree.Node{Name: xml.Name{Space: excelNS, Local: "SelType"}, Text: selType},
				)
			}
			for _, entry := range control.Selected {
				if entry <= 0 {
					continue
				}
				clientData.Children = append(clientData.Children,
					&xmltree.Node{Name: xml.Name{Space: excelNS, Local: "Sel"}, Text: strconv.Itoa(entry)},
				)
			}
		}
		if control.DropLines > 0 {
			clientData.Children = append(clientData.Children,
				&xmltree.Node{Name: xml.Name{Space: excelNS, Local: "DropLines"}, Text: strconv.Itoa(control.DropLines)},
			)
		}
		if control.Val != 0 {
			clientData.Children = append(clientData.Children,
				&xmltree.Node{Name: xml.Name{Space: excelNS, Local: "Val"}, Text: strconv.Itoa(control.Val)},
			)
		}
	}
	return &xmltree.Node{
		Name: xml.Name{Space: vmlNS, Local: "shape"},
		Attr: []xml.Attr{
			{Name: xml.Name{Local: "id"}, Value: id},
			{Name: xml.Name{Local: "type"}, Value: "#" + controlShapeType},
			{Name: xml.Name{Local: "alt"}, Value: control.AltText},
			{Name: xml.Name{Local: "style"}, Value: "position:absolute;z-index:1"},
		},
		Children: []*xmltree.Node{clientData},
	}
}

func formControlSelectionTypeVML(value string) (string, bool) {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "", "single":
		return "Single", value != ""
	case "multi":
		return "Multi", true
	case "extend", "extended":
		return "Extend", true
	default:
		return "", false
	}
}

func vmlCommentShape(ref string, shapeID int) *xmltree.Node {
	row, col, ok := parseCommentRef(ref)
	if !ok {
		return nil
	}
	anchor := defaultCommentAnchor(row, col)
	return &xmltree.Node{
		Name: xml.Name{Space: vmlNS, Local: "shape"},
		Attr: []xml.Attr{
			{Name: xml.Name{Local: "id"}, Value: fmt.Sprintf("_x0000_s%d", shapeID)},
			{Name: xml.Name{Local: "type"}, Value: "#" + commentShapeType},
			{Name: xml.Name{Local: "style"}, Value: "position:absolute;visibility:hidden"},
			{Name: xml.Name{Local: "fillcolor"}, Value: "#ffffe1"},
			{Name: xml.Name{Space: officeNS, Local: "insetmode"}, Value: "auto"},
		},
		Children: []*xmltree.Node{
			{
				Name: xml.Name{Space: vmlNS, Local: "fill"},
				Attr: []xml.Attr{{Name: xml.Name{Local: "color2"}, Value: "#ffffe1"}},
			},
			{
				Name: xml.Name{Space: vmlNS, Local: "shadow"},
				Attr: []xml.Attr{
					{Name: xml.Name{Local: "on"}, Value: "t"},
					{Name: xml.Name{Local: "color"}, Value: "black"},
					{Name: xml.Name{Local: "obscured"}, Value: "t"},
				},
			},
			{
				Name: xml.Name{Space: vmlNS, Local: "path"},
				Attr: []xml.Attr{{Name: xml.Name{Space: officeNS, Local: "connecttype"}, Value: "none"}},
			},
			{
				Name: xml.Name{Space: vmlNS, Local: "textbox"},
				Attr: []xml.Attr{{Name: xml.Name{Local: "style"}, Value: "mso-direction-alt:auto"}},
				Children: []*xmltree.Node{{
					Name: xml.Name{Local: "div"},
					Attr: []xml.Attr{{Name: xml.Name{Local: "style"}, Value: "text-align:left"}},
				}},
			},
			{
				Name: xml.Name{Space: excelNS, Local: "ClientData"},
				Attr: []xml.Attr{{Name: xml.Name{Local: "ObjectType"}, Value: "Note"}},
				Children: []*xmltree.Node{
					{Name: xml.Name{Space: excelNS, Local: "MoveWithCells"}},
					{Name: xml.Name{Space: excelNS, Local: "SizeWithCells"}},
					{Name: xml.Name{Space: excelNS, Local: "Anchor"}, Text: anchor},
					{Name: xml.Name{Space: excelNS, Local: "AutoFill"}, Text: "False"},
					{Name: xml.Name{Space: excelNS, Local: "Row"}, Text: strconv.Itoa(row)},
					{Name: xml.Name{Space: excelNS, Local: "Column"}, Text: strconv.Itoa(col)},
				},
			},
		},
	}
}

func commentRefs(comments *Comments) []string {
	if comments == nil || len(comments.CommentList) == 0 {
		return nil
	}
	out := make([]string, 0, len(comments.CommentList))
	seen := make(map[string]struct{}, len(comments.CommentList))
	for _, comment := range comments.CommentList {
		ref := strings.ToUpper(strings.TrimSpace(comment.Ref))
		if ref == "" {
			continue
		}
		if _, ok := seen[ref]; ok {
			continue
		}
		seen[ref] = struct{}{}
		out = append(out, ref)
	}
	return out
}

func normalizeFormControlType(controlType string) (string, bool) {
	switch strings.TrimSpace(controlType) {
	case "Button", "Checkbox", "Radio", "List", "Drop", "Spin", "Scroll", "GBox", "Label", "Edit":
		return strings.TrimSpace(controlType), true
	default:
		return "", false
	}
}

func normalizeFormControlAnchor(anchor string) (string, bool) {
	parts := strings.Split(anchor, ",")
	if len(parts) != 8 {
		return "", false
	}
	normalized := make([]string, 0, 8)
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if _, err := strconv.Atoi(part); err != nil {
			return "", false
		}
		normalized = append(normalized, part)
	}
	return strings.Join(normalized, ", "), true
}

func vmlAnchorStart(anchor string) (row, col int) {
	parts := strings.Split(anchor, ",")
	if len(parts) < 4 {
		return 0, 0
	}
	col, _ = strconv.Atoi(strings.TrimSpace(parts[0]))
	row, _ = strconv.Atoi(strings.TrimSpace(parts[2]))
	return row, col
}

func nextVMLShapeID(root *xmltree.Node) int {
	maxID := defaultShapeIDMin - 1
	if root == nil {
		return defaultShapeIDMin
	}
	for _, child := range root.Children {
		if child.Name.Local != "shape" {
			continue
		}
		for _, attr := range child.Attr {
			if attr.Name.Local != "id" {
				continue
			}
			value := strings.TrimSpace(attr.Value)
			if !strings.HasPrefix(value, "_x0000_s") {
				continue
			}
			id, err := strconv.Atoi(strings.TrimPrefix(value, "_x0000_s"))
			if err == nil && id > maxID {
				maxID = id
			}
		}
	}
	return maxID + 1
}

func parseCommentRef(ref string) (row, col int, ok bool) {
	ref = strings.TrimSpace(strings.ToUpper(ref))
	if ref == "" {
		return 0, 0, false
	}
	i := 0
	for i < len(ref) && ref[i] >= 'A' && ref[i] <= 'Z' {
		col = col*26 + int(ref[i]-'A'+1)
		i++
	}
	if i == 0 || i >= len(ref) {
		return 0, 0, false
	}
	rowValue, err := strconv.Atoi(ref[i:])
	if err != nil || rowValue <= 0 || col <= 0 {
		return 0, 0, false
	}
	return rowValue - 1, col - 1, true
}

func noteRefFromRowCol(row, col int) string {
	return colLettersFromIndex(col+1) + strconv.Itoa(row+1)
}

func defaultCommentAnchor(row, col int) string {
	return fmt.Sprintf("%d, 15, %d, 2, %d, 15, %d, 16", col+1, row, col+3, row+3)
}

func marshalXMLTree(root *xmltree.Node) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString(xml.Header)
	enc := xml.NewEncoder(&buf)
	if err := encodeXMLNode(enc, root); err != nil {
		return nil, err
	}
	if err := enc.Flush(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func encodeXMLNode(enc *xml.Encoder, node *xmltree.Node) error {
	if node == nil {
		return nil
	}
	start := xml.StartElement{Name: node.Name, Attr: append([]xml.Attr(nil), node.Attr...)}
	if err := enc.EncodeToken(start); err != nil {
		return err
	}
	if node.Text != "" {
		if err := enc.EncodeToken(xml.CharData([]byte(node.Text))); err != nil {
			return err
		}
	}
	for _, child := range node.Children {
		if err := encodeXMLNode(enc, child); err != nil {
			return err
		}
	}
	return enc.EncodeToken(xml.EndElement{Name: start.Name})
}

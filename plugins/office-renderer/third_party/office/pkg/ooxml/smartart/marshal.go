package smartart

import (
	"fmt"
	"strings"

	"office/pkg/ooxml/drawing"
)

// MarshalDataModel serializes a Diagram back into a dgm:dataModel XML document.
// The output does not include an XML declaration so that callers can prepend one
// as needed. Visible nodes, helper points, connections, and preserved raw
// elements (bg, whole, extLst) are written back in the order expected by the
// schema.
func MarshalDataModel(d *Diagram) []byte {
	if d == nil {
		d = &Diagram{}
	}
	var b strings.Builder

	rootAttrs := d.RootAttrs
	if rootAttrs == "" {
		rootAttrs = defaultDataModelRootAttrs
	}
	b.WriteString("<dgm:dataModel ")
	b.WriteString(rootAttrs)
	b.WriteByte('>')

	// Write point list: visible nodes first, then helper points.
	b.WriteString("<dgm:ptLst>")
	nextSyntheticID := 0
	nextID := func(prefix string) string {
		nextSyntheticID++
		return fmt.Sprintf("%s%d", prefix, nextSyntheticID)
	}
	for _, node := range d.Nodes {
		writeDataModelPoint(&b, node, nextID("pt"))
	}
	for _, hp := range d.HelperPoints {
		if len(hp.RawXML) > 0 {
			b.Write(hp.RawXML)
		} else {
			writeHelperPoint(&b, hp, nextID("helper"))
		}
	}
	b.WriteString("</dgm:ptLst>")

	// Write connection list.
	if len(d.Connections) > 0 {
		b.WriteString("<dgm:cxnLst>")
		for i, cxn := range d.Connections {
			writeDataModelConnection(&b, cxn, i+1)
		}
		b.WriteString("</dgm:cxnLst>")
	}

	if len(d.Bg) > 0 {
		b.Write(d.Bg)
	}
	if len(d.Whole) > 0 {
		b.Write(d.Whole)
	}
	if len(d.ExtLst) > 0 {
		b.Write(d.ExtLst)
	}

	b.WriteString("</dgm:dataModel>")
	return []byte(b.String())
}

const defaultDataModelRootAttrs = `xmlns:dgm="http://schemas.openxmlformats.org/drawing/2006/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"`

func writeDataModelPoint(b *strings.Builder, n *Node, fallbackID string) {
	if n == nil {
		n = &Node{}
	}
	b.WriteString("<dgm:pt")
	modelID := fallbackID
	if n.ID != "" {
		modelID = n.ID
	}
	writeAttr(b, "modelId", modelID)
	if n.Type != "" {
		writeAttr(b, "type", n.Type)
	}
	b.WriteByte('>')

	if n.PropertySet.LayoutTypeID != "" || n.PropertySet.QuickStyleTypeID != "" || n.PropertySet.ColorTypeID != "" || n.PropertySet.Placeholder || n.PropertySet.PlaceholderText != "" || n.PropertySet.CustomAngle != 0 {
		b.WriteString("<dgm:prSet")
		writeAttrIf(b, "loTypeId", n.PropertySet.LayoutTypeID)
		writeAttrIf(b, "qsTypeId", n.PropertySet.QuickStyleTypeID)
		writeAttrIf(b, "csTypeId", n.PropertySet.ColorTypeID)
		if n.PropertySet.Placeholder {
			writeAttr(b, "phldr", "1")
		}
		writeAttrIf(b, "phldrT", n.PropertySet.PlaceholderText)
		if n.PropertySet.CustomAngle != 0 {
			writeAttr(b, "custAng", fmt.Sprintf("%d", n.PropertySet.CustomAngle))
		}
		b.WriteString("/>")
	}

	writeNodeShapeProperties(b, n)

	if len(n.ExtLst) > 0 {
		b.Write(n.ExtLst)
	}

	b.WriteString("<dgm:t>")
	writePointTextBody(b, n)
	b.WriteString("</dgm:t>")

	b.WriteString("</dgm:pt>")
}

func writeHelperPoint(b *strings.Builder, hp *HelperPoint, fallbackID string) {
	if hp == nil {
		hp = &HelperPoint{}
	}
	b.WriteString("<dgm:pt")
	modelID := fallbackID
	if hp.ModelID != "" {
		modelID = hp.ModelID
	}
	writeAttr(b, "modelId", modelID)
	if hp.Type != "" {
		writeAttr(b, "type", hp.Type)
	}
	b.WriteString("><dgm:t><a:p><a:r><a:t>")
	escapeXMLText(b, hp.Text)
	b.WriteString("</a:t></a:r></a:p></dgm:t></dgm:pt>")
}

func writePointTextBody(b *strings.Builder, n *Node) {
	if len(n.Paragraphs) == 0 {
		b.WriteString("<a:p>")
		if n.Text != "" {
			b.WriteString("<a:r>")
			writeRunProperties(b, TextRun{Text: n.Text, Color: n.TextColor})
			b.WriteString("<a:t>")
			escapeXMLText(b, n.Text)
			b.WriteString("</a:t>")
			b.WriteString("</a:r>")
		} else if n.PropertySet.PlaceholderText != "" {
			b.WriteString("<a:r><a:t>")
			escapeXMLText(b, n.PropertySet.PlaceholderText)
			b.WriteString("</a:t></a:r>")
		}
		b.WriteString("</a:p>")
		return
	}
	for _, p := range n.Paragraphs {
		b.WriteString("<a:p>")
		writeParagraphProperties(b, p)
		for _, r := range p.Runs {
			b.WriteString("<a:r>")
			writeRunProperties(b, r)
			b.WriteString("<a:t>")
			escapeXMLText(b, r.Text)
			b.WriteString("</a:t></a:r>")
		}
		b.WriteString("</a:p>")
	}
}

func writeNodeShapeProperties(b *strings.Builder, n *Node) {
	if n == nil || n.Fill == nil {
		return
	}
	switch f := n.Fill.(type) {
	case drawing.SolidFill:
		if f.Color == "" {
			return
		}
		b.WriteString("<dgm:spPr><a:solidFill><a:srgbClr")
		writeAttr(b, "val", normalizeHexColor(f.Color))
		b.WriteString("/></a:solidFill></dgm:spPr>")
	case *drawing.SolidFill:
		if f == nil || f.Color == "" {
			return
		}
		b.WriteString("<dgm:spPr><a:solidFill><a:srgbClr")
		writeAttr(b, "val", normalizeHexColor(f.Color))
		b.WriteString("/></a:solidFill></dgm:spPr>")
	}
}

func writeParagraphProperties(b *strings.Builder, p *TextParagraph) {
	if p == nil || (p.DefaultTabSizeEMU == 0 && len(p.TabStops) == 0) {
		return
	}
	b.WriteString("<a:pPr")
	if p.DefaultTabSizeEMU != 0 {
		writeAttr(b, "defTabSz", fmt.Sprintf("%d", p.DefaultTabSizeEMU))
	}
	if len(p.TabStops) == 0 {
		b.WriteString("/>")
		return
	}
	b.WriteByte('>')
	b.WriteString("<a:tabLst>")
	for _, stop := range p.TabStops {
		b.WriteString("<a:tab")
		if stop.Pos != 0 {
			writeAttr(b, "pos", fmt.Sprintf("%d", stop.Pos))
		}
		writeAttrIf(b, "algn", stop.Algn)
		b.WriteString("/>")
	}
	b.WriteString("</a:tabLst></a:pPr>")
}

func writeRunProperties(b *strings.Builder, r TextRun) {
	if !r.Bold && !r.Italic && !r.Underline && r.FontSize == 0 && r.Color == "" && r.FontFamily == "" && r.FontFamilyEA == "" && r.FontFamilyCS == "" {
		return
	}
	b.WriteString("<a:rPr")
	if r.FontSize != 0 {
		writeAttr(b, "sz", fmt.Sprintf("%d", r.FontSize))
	}
	if r.Bold {
		writeAttr(b, "b", "1")
	}
	if r.Italic {
		writeAttr(b, "i", "1")
	}
	if r.Underline {
		writeAttr(b, "u", "sng")
	}
	b.WriteByte('>')
	if r.Color != "" {
		b.WriteString("<a:solidFill><a:srgbClr")
		writeAttr(b, "val", normalizeHexColor(r.Color))
		b.WriteString("/></a:solidFill>")
	}
	if r.FontFamily != "" {
		b.WriteString("<a:latin")
		writeAttr(b, "typeface", r.FontFamily)
		b.WriteString("/>")
	}
	if r.FontFamilyEA != "" {
		b.WriteString("<a:ea")
		writeAttr(b, "typeface", r.FontFamilyEA)
		b.WriteString("/>")
	}
	if r.FontFamilyCS != "" {
		b.WriteString("<a:cs")
		writeAttr(b, "typeface", r.FontFamilyCS)
		b.WriteString("/>")
	}
	b.WriteString("</a:rPr>")
}

func writeDataModelConnection(b *strings.Builder, cxn Connection, modelID int) {
	b.WriteString("<dgm:cxn")
	writeAttr(b, "modelId", fmt.Sprintf("%d", modelID))
	writeAttr(b, "srcId", cxn.SrcID)
	writeAttr(b, "destId", cxn.DestID)
	writeAttr(b, "type", "parCh")
	b.WriteString("/>")
}

func writeAttr(b *strings.Builder, name, value string) {
	b.WriteByte(' ')
	b.WriteString(name)
	b.WriteString(`="`)
	escapeXMLAttr(b, value)
	b.WriteByte('"')
}

func writeAttrIf(b *strings.Builder, name, value string) {
	if value == "" {
		return
	}
	writeAttr(b, name, value)
}

func escapeXMLAttr(b *strings.Builder, s string) {
	// Same escaping as text; double quotes are also escaped.
	for _, r := range s {
		switch r {
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		case '&':
			b.WriteString("&amp;")
		case '"':
			b.WriteString("&quot;")
		default:
			b.WriteRune(r)
		}
	}
}

func escapeXMLText(b *strings.Builder, s string) {
	for _, r := range s {
		switch r {
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		case '&':
			b.WriteString("&amp;")
		default:
			b.WriteRune(r)
		}
	}
}

func normalizeHexColor(s string) string {
	return strings.TrimPrefix(strings.TrimSpace(s), "#")
}

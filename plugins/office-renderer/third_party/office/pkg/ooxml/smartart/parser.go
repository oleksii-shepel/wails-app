package smartart

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"office/pkg/diag"
	"office/pkg/ooxml/drawing"
)

// ParseDiagram parses a SmartArt data model XML into a semantic Diagram.
// This is the primary entry point for consumers that need the diagram
// structure (nodes, relationships, layout) rather than pre-positioned shapes.
func ParseDiagram(xmlData []byte) (*Diagram, error) {
	return ParseDiagramWithOptions(xmlData, nil, nil, nil)
}

// ParseDiagramWithOptions parses a SmartArt data model XML with optional
// layout definition, quick style, and color style XML to guide layout and
// appearance instead of using hardcoded defaults.
func ParseDiagramWithOptions(xmlData, layoutXML, styleXML, colorXML []byte) (*Diagram, error) {
	return ParseDiagramWithOptionsAndTheme(xmlData, layoutXML, styleXML, colorXML, nil)
}

// ParseDiagramWithOptionsAndTheme is like ParseDiagramWithOptions but resolves
// theme reference colors (e.g. a:schemeClr) using the supplied theme color map.
func ParseDiagramWithOptionsAndTheme(xmlData, layoutXML, styleXML, colorXML []byte, themeColors map[string]string) (*Diagram, error) {
	return ParseDiagramWithOptionsAndThemeFormat(xmlData, layoutXML, styleXML, colorXML, themeColors, nil)
}

// ParseDiagramWithOptionsAndThemeFormat is like ParseDiagramWithOptionsAndTheme
// but also resolves style-matrix references (a:fillRef, a:lnRef, a:effectRef,
// a:fontRef) using the supplied theme format scheme.
func ParseDiagramWithOptionsAndThemeFormat(xmlData, layoutXML, styleXML, colorXML []byte, themeColors map[string]string, fmtScheme *ThemeFormatScheme) (*Diagram, error) {
	// SmartArt data model uses dgm namespace
	// For now, create a simple hierarchy from basic structure
	diagram := &Diagram{
		Type:  "hierarchy",
		Nodes: []*Node{},
		Layout: &Layout{
			Type:      "hierarchy",
			Direction: "topDown",
			HSpacing:  50,
			VSpacing:  80,
		},
		Style: &Style{
			Fill:      drawing.SolidFill{Color: "4472C4", Alpha: 1.0},
			Line:      &drawing.LineStyle{Color: "2F5597", Width: 1.5, Alpha: 1.0},
			TextColor: "FFFFFF",
			ShapeType: "rectangle",
		},
		RootAttrs: parseSmartArtRootAttrs(xmlData),
		ExtLst:    nil,
		Bg:        nil,
		Whole:     nil,
	}
	diagram.ExtLst, diagram.Bg, diagram.Whole = parseSmartArtTopLevelRawElements(xmlData)
	diagram.BackgroundFill, diagram.WholeEffect = parseSmartArtDiagramFormatting(xmlData, themeColors)

	// If layout XML is provided, extract layout type, direction, and the full
	// layout-definition algorithm tree.
	if len(layoutXML) > 0 {
		parseSmartArtLayout(layoutXML, diagram.Layout)
	}

	// If color XML is provided, extract color palettes for fills and lines.
	if len(colorXML) > 0 {
		if palette := parseSmartArtColorPalette(colorXML, themeColors); len(palette) > 0 {
			diagram.Style.ColorPalette = palette
		}
		if linePalette := parseSmartArtLinePalette(colorXML, themeColors); len(linePalette) > 0 {
			diagram.Style.LinePalette = linePalette
		}
		if textFillPalette := parseSmartArtTextFillPalette(colorXML, themeColors); len(textFillPalette) > 0 {
			diagram.Style.TextFillPalette = textFillPalette
		}
		if textLinePalette := parseSmartArtTextLinePalette(colorXML, themeColors); len(textLinePalette) > 0 {
			diagram.Style.TextLinePalette = textLinePalette
		}
		if effectPalette := parseSmartArtEffectPalette(colorXML, themeColors); len(effectPalette) > 0 {
			diagram.Style.EffectPalette = effectPalette
		}
	}

	// If style XML is provided, extract shape-type, fill, line, effect, and
	// text-color hints, resolving theme-matrix references when a format scheme
	// is supplied.
	if len(styleXML) > 0 {
		if shapeType := parseSmartArtStyleShapeType(styleXML); shapeType != "" {
			diagram.Style.ShapeType = shapeType
		}
		fill, line, connLine, effect, textColor, lineExplicit := parseSmartArtStyle(styleXML, themeColors, fmtScheme)
		if fill != nil {
			diagram.Style.Fill = fill
		}
		if lineExplicit {
			diagram.Style.Line = line
		}
		if connLine != nil {
			diagram.Style.ConnectorLine = connLine
		}
		if len(effect.Effects) > 0 {
			diagram.Style.Effect = effect
		}
		if textColor != "" {
			diagram.Style.TextColor = textColor
		}
	}

	// Extract points and connections. We use a token-based point parser because
	// SmartArt text lives in dgm:t>a:p>a:t runs, while legacy/placeholder data
	// may use dgm:prSet>dgm:fld. Both modelId and id attributes are accepted.
	points, helpers := parseSmartArtPoints(xmlData, themeColors)
	diagram.HelperPoints = helpers
	if len(points) == 0 {
		return parseSimpleStructure(xmlData, diagram)
	}

	connections := parseSmartArtConnections(xmlData)

	// Build a map of visible data nodes, ignoring transition and presentation
	// helper points.
	nodeMap := make(map[string]*Node)
	for _, pt := range points {
		if !isVisibleSmartArtPointType(pt.Type) {
			continue
		}
		node := &Node{
			ID:            pt.ModelID,
			Type:          pt.Type,
			Assistant:     pt.Type == "asst",
			Text:          pt.Text,
			Paragraphs:    pt.Paragraphs,
			TextBodyProps: pt.TextBodyProps,
			PropertySet:   pt.PropertySet,
			ExtLst:        pt.ExtLst,
		}
		nodeMap[pt.ModelID] = node
		diagram.Nodes = append(diagram.Nodes, node)
	}

	// If we did not find any visible nodes, fall back to the simple text
	// extractor so that diagrams with unusual markup still render something.
	if len(diagram.Nodes) == 0 {
		return parseSimpleStructure(xmlData, diagram)
	}

	// Repair duplicate node IDs so downstream maps and connections remain stable.
	seenIDs := make(map[string]struct{})
	for _, node := range diagram.Nodes {
		if _, exists := seenIDs[node.ID]; exists {
			base := node.ID
			for i := 1; ; i++ {
				candidate := fmt.Sprintf("%s-dup%d", base, i)
				if _, exists := seenIDs[candidate]; !exists {
					diagram.Diagnostics.Add(diag.Diagnostic{
						Severity: diag.SeverityWarning,
						Code:     "smartart.duplicate-node-id",
						Message:  fmt.Sprintf("Duplicate SmartArt node id %q repaired to %q", base, candidate),
					})
					node.ID = candidate
					break
				}
			}
		}
		seenIDs[node.ID] = struct{}{}
	}
	// Rebuild the map with repaired IDs.
	nodeMap = make(map[string]*Node)
	for _, node := range diagram.Nodes {
		nodeMap[node.ID] = node
	}

	// Build parent-child relationships from non-presentation connections,
	// ordering children by srcOrd when it is supplied.
	type childOrder struct {
		child *Node
		order int
	}
	childrenByParent := make(map[string][]childOrder)
	for _, cxn := range connections {
		if cxn.Type == "presOf" {
			continue
		}
		parent, hasParent := nodeMap[cxn.SrcID]
		child, hasChild := nodeMap[cxn.DestID]
		if hasParent && hasChild {
			child.Parent = parent
			childrenByParent[parent.ID] = append(childrenByParent[parent.ID], childOrder{
				child: child,
				order: cxn.SrcOrd,
			})
		}
	}
	for _, node := range diagram.Nodes {
		if list, ok := childrenByParent[node.ID]; ok {
			sort.Slice(list, func(i, j int) bool {
				return list[i].order < list[j].order
			})
			for _, co := range list {
				node.Children = append(node.Children, co.child)
			}
		}
	}

	// Preserve the data-model connections in declaration order so the resolver
	// can map them onto pre-rendered drawing connectors.
	for _, cxn := range connections {
		if cxn.Type == "presOf" {
			continue
		}
		if _, ok := nodeMap[cxn.SrcID]; !ok {
			continue
		}
		if _, ok := nodeMap[cxn.DestID]; !ok {
			continue
		}
		diagram.Connections = append(diagram.Connections, Connection{SrcID: cxn.SrcID, DestID: cxn.DestID})
	}

	// Calculate levels
	for _, node := range diagram.Nodes {
		level := 0
		parent := node.Parent
		for parent != nil {
			level++
			parent = parent.Parent
		}
		node.Level = level
	}

	// Apply per-node fill/line overrides from dgm:spPr when present.
	if styles := parseDataModelNodeStyles(xmlData, themeColors, fmtScheme); len(styles) > 0 {
		for _, node := range diagram.Nodes {
			if style, ok := styles[node.ID]; ok {
				if style.Fill != nil {
					node.Fill = style.Fill
				}
				if style.Line != nil {
					node.Line = style.Line
				}
			}
		}
	}

	return diagram, nil
}

// rawPoint is a lightweight representation of a dgm:pt element.
type rawPoint struct {
	ModelID       string
	Type          string
	Text          string
	Paragraphs    []*TextParagraph
	TextBodyProps drawing.TextBodyProperties
	PropertySet   PropertySet
	ExtLst        []byte
}

// rawConnection is a lightweight representation of a dgm:cxn element.
type rawConnection struct {
	Type    string `xml:"type,attr"`
	SrcID   string `xml:"srcId,attr"`
	DestID  string `xml:"destId,attr"`
	SrcOrd  int    `xml:"srcOrd,attr,omitempty"`
	DestOrd int    `xml:"destOrd,attr,omitempty"`
}

func parseSmartArtPropertySet(attrs []xml.Attr) PropertySet {
	var ps PropertySet
	for _, attr := range attrs {
		switch attr.Name.Local {
		case "loTypeId":
			ps.LayoutTypeID = attr.Value
		case "qsTypeId":
			ps.QuickStyleTypeID = attr.Value
		case "csTypeId":
			ps.ColorTypeID = attr.Value
		case "phldr":
			ps.Placeholder = attr.Value == "1"
		case "phldrT":
			ps.PlaceholderText = attr.Value
		case "custAng":
			if v, err := strconv.Atoi(attr.Value); err == nil {
				ps.CustomAngle = v
			}
		}
	}
	return ps
}

// isVisibleSmartArtPointType returns true for data points that should be
// rendered as diagram shapes. Transition and presentation helper points are
// excluded.
func isVisibleSmartArtPointType(t string) bool {
	switch t {
	case "parTrans", "sibTrans", "pres":
		return false
	}
	return true
}

// copyElementRaw consumes a single XML element (starting with start) from d and
// returns its serialized bytes. The returned XML preserves the element's
// children, including nested elements and character data, but normalizes
// attribute quoting and escapes text. This is sufficient for round-trip
// preservation of elements such as dgm:extLst whose content is otherwise opaque.
func copyElementRaw(d *xml.Decoder, start xml.StartElement) ([]byte, error) {
	var b bytes.Buffer
	writeStartElement(&b, start)
	depth := 1
	for {
		tok, err := d.RawToken()
		if err != nil {
			return b.Bytes(), err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			writeStartElement(&b, t)
		case xml.EndElement:
			depth--
			if depth == 0 {
				b.WriteString("</")
				writeName(&b, t.Name)
				b.WriteByte('>')
				return b.Bytes(), nil
			}
			b.WriteString("</")
			writeName(&b, t.Name)
			b.WriteByte('>')
		case xml.CharData:
			xml.EscapeText(&b, t)
		case xml.Comment:
			b.WriteString("<!--")
			b.Write(t)
			b.WriteString("-->")
		}
	}
}

func writeName(b *bytes.Buffer, name xml.Name) {
	if name.Space != "" {
		b.WriteString(name.Space)
		b.WriteByte(':')
	}
	b.WriteString(name.Local)
}

func writeStartElement(b *bytes.Buffer, start xml.StartElement) {
	b.WriteByte('<')
	writeName(b, start.Name)
	for _, attr := range start.Attr {
		b.WriteByte(' ')
		if attr.Name.Space != "" {
			b.WriteString(attr.Name.Space)
			b.WriteByte(':')
		}
		b.WriteString(attr.Name.Local)
		b.WriteByte('=')
		b.WriteByte('"')
		xml.EscapeText(b, []byte(attr.Value))
		b.WriteByte('"')
	}
	b.WriteByte('>')
}

// dataModelRootRe matches the dgm:dataModel start tag and captures its
// attributes so root namespace declarations can be preserved.
var dataModelRootRe = regexp.MustCompile(`<[\w-]*:?dataModel\b([^>]*)>`)

// parseSmartArtRootAttrs returns the raw attributes of the dgm:dataModel root
// element. If the root cannot be found, a default set of namespace declarations
// is returned.
func parseSmartArtRootAttrs(xmlData []byte) string {
	m := dataModelRootRe.FindSubmatch(xmlData)
	if len(m) > 1 {
		return string(bytes.TrimSpace(m[1]))
	}
	return `xmlns:dgm="http://schemas.openxmlformats.org/drawing/2006/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"`
}

// parseSmartArtTopLevelRawElements captures the raw dgm:extLst, dgm:bg, and
// dgm:whole elements that are direct children of the dgm:dataModel root.
func parseSmartArtTopLevelRawElements(xmlData []byte) (extLst, bg, whole []byte) {
	dec := xml.NewDecoder(strings.NewReader(string(xmlData)))
	depth := 0
	for {
		tok, err := dec.RawToken()
		if err != nil {
			break
		}
		switch se := tok.(type) {
		case xml.StartElement:
			if depth == 1 {
				switch se.Name.Local {
				case "extLst":
					extLst, _ = copyElementRaw(dec, se)
					continue
				case "bg":
					bg, _ = copyElementRaw(dec, se)
					continue
				case "whole":
					whole, _ = copyElementRaw(dec, se)
					continue
				}
			}
			depth++
		case xml.EndElement:
			depth--
		}
	}
	return
}

func parseSmartArtDiagramFormatting(xmlData []byte, themeColors map[string]string) (drawing.Fill, drawing.EffectList) {
	type colorInfo struct {
		Val   string `xml:"val,attr"`
		Alpha *struct {
			Val int64 `xml:"val,attr"`
		} `xml:"alpha"`
	}
	type solidFillDef struct {
		SrgbClr   *colorInfo `xml:"srgbClr"`
		SchemeClr *colorInfo `xml:"schemeClr"`
	}
	type gradFillDef struct {
		GsLst []struct {
			Pos       int64      `xml:"pos,attr"`
			SrgbClr   *colorInfo `xml:"srgbClr"`
			SchemeClr *colorInfo `xml:"schemeClr"`
		} `xml:"gsLst>gs"`
		Lin *struct {
			Ang int64 `xml:"ang,attr"`
		} `xml:"lin"`
	}
	type fillContainer struct {
		NoFill    *struct{}     `xml:"noFill"`
		SolidFill *solidFillDef `xml:"solidFill"`
		GradFill  *gradFillDef  `xml:"gradFill"`
		PattFill  *struct {
			Prst  string        `xml:"prst,attr"`
			FgClr *solidFillDef `xml:"fgClr"`
			BgClr *solidFillDef `xml:"bgClr"`
		} `xml:"pattFill"`
		BgClr *solidFillDef `xml:"bgClr"`
		BgPr  *struct {
			NoFill    *struct{}     `xml:"noFill"`
			SolidFill *solidFillDef `xml:"solidFill"`
			GradFill  *gradFillDef  `xml:"gradFill"`
		} `xml:"bgPr"`
	}
	var doc struct {
		Bg    *fillContainer `xml:"bg"`
		Whole *struct {
			EffectLst *struct {
				InnerXML string `xml:",innerxml"`
			} `xml:"effectLst"`
		} `xml:"whole"`
	}
	if err := xml.Unmarshal(xmlData, &doc); err != nil {
		return nil, drawing.EffectList{}
	}

	resolveColor := func(c *colorInfo) (string, float64) {
		if c == nil {
			return "", 1.0
		}
		color := c.Val
		if themeColors != nil {
			if resolved, ok := themeColors[color]; ok {
				color = resolved
			}
		}
		alpha := 1.0
		if c.Alpha != nil {
			alpha = float64(c.Alpha.Val) / 100000.0
		}
		if alpha < 0 {
			alpha = 0
		} else if alpha > 1 {
			alpha = 1
		}
		return color, alpha
	}
	solid := func(sf *solidFillDef) (string, float64) {
		if sf == nil {
			return "", 1.0
		}
		if sf.SrgbClr != nil {
			return resolveColor(sf.SrgbClr)
		}
		if sf.SchemeClr != nil {
			return resolveColor(sf.SchemeClr)
		}
		return "", 1.0
	}
	gradient := func(gf *gradFillDef) drawing.Fill {
		if gf == nil {
			return nil
		}
		var stops []drawing.GradientStop
		for _, gs := range gf.GsLst {
			color, alpha := "", 1.0
			if gs.SrgbClr != nil {
				color, alpha = resolveColor(gs.SrgbClr)
			} else if gs.SchemeClr != nil {
				color, alpha = resolveColor(gs.SchemeClr)
			}
			if color == "" {
				continue
			}
			stops = append(stops, drawing.GradientStop{Position: float64(gs.Pos) / 100000.0, Color: color, Alpha: alpha})
		}
		if len(stops) == 0 {
			return nil
		}
		angle := 0.0
		if gf.Lin != nil {
			angle = float64(gf.Lin.Ang) / 60000.0
		}
		return drawing.GradientFill{GradientType: drawing.GradientTypeLinear, Angle: angle, Stops: stops}
	}
	var resolveFill func(*fillContainer) drawing.Fill
	resolveFill = func(fc *fillContainer) drawing.Fill {
		if fc == nil {
			return nil
		}
		if fc.NoFill != nil {
			return drawing.NoFill{}
		}
		if fc.BgPr != nil {
			return resolveFill(&fillContainer{NoFill: fc.BgPr.NoFill, SolidFill: fc.BgPr.SolidFill, GradFill: fc.BgPr.GradFill})
		}
		if fc.SolidFill != nil {
			color, alpha := solid(fc.SolidFill)
			if color != "" {
				return drawing.SolidFill{Color: color, Alpha: alpha}
			}
		}
		if fc.BgClr != nil {
			color, alpha := solid(fc.BgClr)
			if color != "" {
				return drawing.SolidFill{Color: color, Alpha: alpha}
			}
		}
		if gf := gradient(fc.GradFill); gf != nil {
			return gf
		}
		if fc.PattFill != nil {
			fg, fgAlpha := solid(fc.PattFill.FgClr)
			bg, bgAlpha := solid(fc.PattFill.BgClr)
			if fg == "" {
				fg = "000000"
			}
			if bg == "" {
				bg = "FFFFFF"
			}
			return drawing.PatternFill{PatternType: drawing.PatternType(fc.PattFill.Prst), Foreground: fg, Background: bg, ForeAlpha: fgAlpha, BackAlpha: bgAlpha}
		}
		return nil
	}

	effects := drawing.EffectList{}
	if doc.Whole != nil && doc.Whole.EffectLst != nil {
		effects = ParseEffectListInnerXMLWithTheme(doc.Whole.EffectLst.InnerXML, themeColors)
	}
	return resolveFill(doc.Bg), effects
}

// ParseEffectListInnerXML parses the child XML of an a:effectLst element into a
// shared DrawingML effect list. It is used by SmartArt data-model and drawing
// part parsers that capture effectLst as inner XML.
func ParseEffectListInnerXML(inner string) drawing.EffectList {
	return ParseEffectListInnerXMLWithTheme(inner, nil)
}

// ParseEffectListInnerXMLWithTheme parses the child XML of an a:effectLst element
// into shared DrawingML effects while resolving scheme colors through themeColors.
func ParseEffectListInnerXMLWithTheme(inner string, themeColors map[string]string) drawing.EffectList {
	if strings.TrimSpace(inner) == "" {
		return drawing.EffectList{}
	}
	dec := xml.NewDecoder(strings.NewReader("<effectLst>" + inner + "</effectLst>"))
	for {
		tok, err := dec.Token()
		if err != nil {
			return drawing.EffectList{}
		}
		if se, ok := tok.(xml.StartElement); ok && se.Name.Local == "effectLst" {
			return parseSmartArtEffectListWithTheme(dec, themeColors)
		}
	}
}

func parseSmartArtPoints(xmlData []byte, themeColors map[string]string) (points []rawPoint, helpers []*HelperPoint) {
	dec := xml.NewDecoder(strings.NewReader(string(xmlData)))
	var current *rawPoint
	var inText bool

	for {
		tok, err := dec.RawToken()
		if err != nil {
			break
		}

		switch se := tok.(type) {
		case xml.StartElement:
			switch se.Name.Local {
			case "pt":
				pt := rawPoint{}
				for _, attr := range se.Attr {
					switch attr.Name.Local {
					case "modelId":
						pt.ModelID = attr.Value
					case "id":
						if pt.ModelID == "" {
							pt.ModelID = attr.Value
						}
					case "type":
						pt.Type = attr.Value
					}
				}
				if !isVisibleSmartArtPointType(pt.Type) {
					raw, _ := copyElementRaw(dec, se)
					helpers = append(helpers, &HelperPoint{
						ModelID: pt.ModelID,
						Type:    pt.Type,
						RawXML:  raw,
					})
					current = nil
					break
				}
				current = &pt
			case "t":
				if current != nil {
					inText = true
				}
			case "fld":
				if current != nil {
					inText = true
					for _, attr := range se.Attr {
						if attr.Name.Local == "val" {
							current.Text = attr.Value
							break
						}
					}
				}
			case "prSet":
				if current != nil {
					current.PropertySet = parseSmartArtPropertySet(se.Attr)
				}
			case "extLst":
				if current != nil {
					raw, _ := copyElementRaw(dec, se)
					current.ExtLst = raw
				}
			}
		case xml.EndElement:
			switch se.Name.Local {
			case "pt":
				if current != nil {
					current.Text = strings.TrimSpace(current.Text)
					points = append(points, *current)
					current = nil
				}
			case "fld", "t":
				inText = false
			}
		case xml.CharData:
			if inText && current != nil {
				current.Text += string(se)
			}
		}
	}

	// Second pass: extract styled paragraphs from dgm:t elements.
	points = enrichPointParagraphs(xmlData, points, themeColors)
	return points, helpers
}

// bodyPr captures the a:bodyPr attributes we need for text layout.
type bodyPr struct {
	Anchor      string    `xml:"anchor,attr"`
	AnchorCtr   string    `xml:"anchorCtr,attr"`
	Rot         int       `xml:"rot,attr"`
	Wrap        string    `xml:"wrap,attr"`
	Vert        string    `xml:"vert,attr"`
	LIns        *int64    `xml:"lIns,attr"`
	RIns        *int64    `xml:"rIns,attr"`
	TIns        *int64    `xml:"tIns,attr"`
	BIns        *int64    `xml:"bIns,attr"`
	NoAutofit   *struct{} `xml:"noAutofit"`
	SpAutoFit   *struct{} `xml:"spAutoFit"`
	NormAutofit *struct {
		FontScale          int `xml:"fontScale,attr"`
		LineSpaceReduction int `xml:"lnSpcReduction,attr"`
	} `xml:"normAutofit"`
}

func bodyPrToProperties(bp *bodyPr) drawing.TextBodyProperties {
	if bp == nil {
		return drawing.TextBodyProperties{}
	}
	props := drawing.TextBodyProperties{
		Anchor:    bp.Anchor,
		AnchorCtr: bp.AnchorCtr == "1",
		Rot:       bp.Rot,
		Wrap:      bp.Wrap,
		Vert:      bp.Vert,
	}
	if bp.LIns != nil {
		props.InsetL = *bp.LIns
	}
	if bp.RIns != nil {
		props.InsetR = *bp.RIns
	}
	if bp.TIns != nil {
		props.InsetT = *bp.TIns
	}
	if bp.BIns != nil {
		props.InsetB = *bp.BIns
	}
	switch {
	case bp.NoAutofit != nil:
		props.AutofitType = "noAutofit"
	case bp.SpAutoFit != nil:
		props.AutofitType = "spAutoFit"
	case bp.NormAutofit != nil:
		props.AutofitType = "normAutofit"
		props.AutofitFontScale = bp.NormAutofit.FontScale
		props.AutofitLineSpaceReduction = bp.NormAutofit.LineSpaceReduction
	}
	return props
}

// enrichPointParagraphs parses dgm:t/a:p structured text and attaches styled
// paragraphs to the matching rawPoints.  It falls back to the existing plain
// text when no structured paragraphs are found.
func enrichPointParagraphs(xmlData []byte, points []rawPoint, themeColors map[string]string) []rawPoint {
	if len(points) == 0 {
		return points
	}

	type colorInfo struct {
		Val   string `xml:"val,attr"`
		Alpha *struct {
			Val int64 `xml:"val,attr"`
		} `xml:"alpha"`
	}
	resolveColor := func(c *colorInfo) string {
		if c == nil {
			return ""
		}
		if themeColors != nil {
			if scheme, ok := themeColors[c.Val]; ok {
				return scheme
			}
		}
		return c.Val
	}

	type spacingDef struct {
		SpcPts *struct {
			Val int `xml:"val,attr"`
		} `xml:"spcPts"`
		SpcPct *struct {
			Val int `xml:"val,attr"`
		} `xml:"spcPct"`
	}

	var doc struct {
		PtLst []struct {
			ModelID string `xml:"modelId,attr"`
			ID      string `xml:"id,attr"`
			T       *struct {
				BodyPr *bodyPr `xml:"bodyPr"`
				P      []struct {
					PPr *struct {
						Algn     string      `xml:"algn,attr"`
						Lvl      int         `xml:"lvl,attr,omitempty"`
						DefTabSz int         `xml:"defTabSz,attr,omitempty"`
						SpcBef   *spacingDef `xml:"spcBef"`
						SpcAft   *spacingDef `xml:"spcAft"`
						LnSpc    *spacingDef `xml:"lnSpc"`
						TabStops []struct {
							Pos  int64  `xml:"pos,attr"`
							Algn string `xml:"algn,attr"`
						} `xml:"tabLst>tab"`
						BuChar *struct {
							Char string `xml:"char,attr"`
						} `xml:"buChar"`
						BuAutoNum *struct {
							Type    string `xml:"type,attr"`
							StartAt int    `xml:"startAt,attr,omitempty"`
						} `xml:"buAutoNum"`
						BuNone      *struct{}  `xml:"buNone"`
						BuClr       *colorInfo `xml:"buClr>srgbClr"`
						BuClrScheme *colorInfo `xml:"buClr>schemeClr"`
						BuFont      *struct {
							Typeface string `xml:"typeface,attr"`
						} `xml:"buFont"`
						BuFontTx *struct{} `xml:"buFontTx"`
						BuSzPts  *struct {
							Val int `xml:"val,attr"`
						} `xml:"buSzPts"`
						BuSzPct *struct {
							Val int `xml:"val,attr"`
						} `xml:"buSzPct"`
					} `xml:"pPr"`
					R []struct {
						RPr *struct {
							B       string `xml:"b,attr"`
							I       string `xml:"i,attr"`
							U       string `xml:"u,attr"`
							Sz      int64  `xml:"sz,attr"`
							SldFill struct {
								SrgbClr   *colorInfo `xml:"srgbClr"`
								SchemeClr *colorInfo `xml:"schemeClr"`
							} `xml:"solidFill"`
							Latin *struct {
								Typeface string `xml:"typeface,attr"`
							} `xml:"latin"`
							EA *struct {
								Typeface string `xml:"typeface,attr"`
							} `xml:"ea"`
							CS *struct {
								Typeface string `xml:"typeface,attr"`
							} `xml:"cs"`
							EffectLst *struct {
								InnerXML string `xml:",innerxml"`
							} `xml:"effectLst"`
						} `xml:"rPr"`
						T string `xml:"t"`
					} `xml:"r"`
				} `xml:"p"`
			} `xml:"t"`
		} `xml:"ptLst>pt"`
	}

	if err := xml.Unmarshal(xmlData, &doc); err != nil {
		return points
	}

	pointIdx := make(map[string]int)
	for i, p := range points {
		pointIdx[p.ModelID] = i
	}

	for _, pt := range doc.PtLst {
		id := pt.ModelID
		if id == "" {
			id = pt.ID
		}
		idx, ok := pointIdx[id]
		if !ok || pt.T == nil || len(pt.T.P) == 0 {
			continue
		}

		var paragraphs []*TextParagraph
		for _, p := range pt.T.P {
			para := &TextParagraph{}
			if p.PPr != nil {
				para.Align = p.PPr.Algn
				para.Level = p.PPr.Lvl
				para.DefaultTabSizeEMU = p.PPr.DefTabSz
				if len(p.PPr.TabStops) > 0 {
					para.TabStops = make([]TabStop, 0, len(p.PPr.TabStops))
					for _, stop := range p.PPr.TabStops {
						para.TabStops = append(para.TabStops, TabStop{Pos: stop.Pos, Algn: stop.Algn})
					}
				}
				if p.PPr.BuChar != nil {
					para.BulletChar = p.PPr.BuChar.Char
				}
				if p.PPr.BuAutoNum != nil {
					para.AutoNum = true
					para.AutoNumType = p.PPr.BuAutoNum.Type
					para.AutoNumStart = p.PPr.BuAutoNum.StartAt
				}
				if p.PPr.BuClr != nil {
					para.BulletColor = p.PPr.BuClr.Val
				}
				if p.PPr.BuClrScheme != nil {
					para.BulletColor = resolveColor(p.PPr.BuClrScheme)
				}
				if p.PPr.BuFont != nil {
					para.BulletFontFamily = p.PPr.BuFont.Typeface
				}
				if p.PPr.BuFontTx != nil {
					para.BulletFontTx = true
					para.BulletFontFamily = ""
				}
				if p.PPr.BuSzPts != nil {
					para.BulletSize = p.PPr.BuSzPts.Val
				}
				if p.PPr.BuSzPct != nil {
					para.BulletSizePct = p.PPr.BuSzPct.Val
				}
				if p.PPr.BuNone != nil {
					para.BulletChar = ""
					para.AutoNum = false
				}
				if p.PPr.SpcBef != nil {
					if p.PPr.SpcBef.SpcPts != nil {
						para.SpaceBefore = p.PPr.SpcBef.SpcPts.Val
					}
					if p.PPr.SpcBef.SpcPct != nil {
						para.SpaceBeforePct = p.PPr.SpcBef.SpcPct.Val
					}
				}
				if p.PPr.SpcAft != nil {
					if p.PPr.SpcAft.SpcPts != nil {
						para.SpaceAfter = p.PPr.SpcAft.SpcPts.Val
					}
					if p.PPr.SpcAft.SpcPct != nil {
						para.SpaceAfterPct = p.PPr.SpcAft.SpcPct.Val
					}
				}
				if p.PPr.LnSpc != nil {
					if p.PPr.LnSpc.SpcPts != nil {
						para.LineSpacingPts = p.PPr.LnSpc.SpcPts.Val
					}
					if p.PPr.LnSpc.SpcPct != nil {
						para.LineSpacingPct = p.PPr.LnSpc.SpcPct.Val
					}
				}
			}
			for _, r := range p.R {
				run := TextRun{Text: r.T}
				if r.RPr != nil {
					run.Bold = r.RPr.B == "1"
					run.Italic = r.RPr.I == "1"
					run.Underline = r.RPr.U != ""
					run.FontSize = int(r.RPr.Sz)
					if r.RPr.Latin != nil {
						run.FontFamily = r.RPr.Latin.Typeface
					}
					if r.RPr.EA != nil {
						run.FontFamilyEA = r.RPr.EA.Typeface
					}
					if r.RPr.CS != nil {
						run.FontFamilyCS = r.RPr.CS.Typeface
					}
					if r.RPr.SldFill.SrgbClr != nil {
						run.Color = r.RPr.SldFill.SrgbClr.Val
					}
					if r.RPr.SldFill.SchemeClr != nil {
						run.Color = resolveColor(r.RPr.SldFill.SchemeClr)
					}
					if r.RPr.EffectLst != nil {
						run.Effects = ParseEffectListInnerXMLWithTheme(r.RPr.EffectLst.InnerXML, themeColors)
					}
				}
				para.Runs = append(para.Runs, run)
			}
			if len(para.Runs) > 0 {
				paragraphs = append(paragraphs, para)
			}
		}
		if len(paragraphs) > 0 {
			points[idx].Paragraphs = paragraphs
		}
		points[idx].TextBodyProps = bodyPrToProperties(pt.T.BodyPr)
	}

	return points
}

// DataToPresentationMap returns a mapping from data-model node IDs to the
// presentation-node IDs that represent them in a pre-rendered drawing part.
// It is built from dgm:cxn elements whose type is "presOf".
func DataToPresentationMap(dataModelXML []byte) map[string]string {
	m := make(map[string]string)
	for _, cxn := range parseSmartArtConnections(dataModelXML) {
		if cxn.Type == "presOf" {
			m[cxn.SrcID] = cxn.DestID
		}
	}
	return m
}

// parseSmartArtConnections extracts dgm:cxn elements from the data model.
func parseSmartArtConnections(xmlData []byte) []rawConnection {
	var doc struct {
		Cxn []rawConnection `xml:"cxnLst>cxn"`
	}
	if err := xml.Unmarshal(xmlData, &doc); err != nil {
		return nil
	}
	return doc.Cxn
}

// parseSimpleStructure tries to extract any text from XML as a fallback
func parseSimpleStructure(xmlData []byte, diagram *Diagram) (*Diagram, error) {
	// Extract all text elements
	var texts []string
	var inText bool
	var currentText strings.Builder

	decoder := xml.NewDecoder(strings.NewReader(string(xmlData)))
	for {
		token, err := decoder.Token()
		if err != nil {
			break
		}

		switch se := token.(type) {
		case xml.StartElement:
			if se.Name.Local == "fld" || se.Name.Local == "t" {
				inText = true
				currentText.Reset()
			}
		case xml.EndElement:
			if se.Name.Local == "fld" || se.Name.Local == "t" {
				inText = false
				text := strings.TrimSpace(currentText.String())
				if text != "" {
					texts = append(texts, text)
				}
			}
		case xml.CharData:
			if inText {
				currentText.Write(se)
			}
		}
	}

	// Create flat hierarchy from extracted texts
	for i, text := range texts {
		node := &Node{
			ID:    fmt.Sprintf("node%d", i),
			Type:  "node",
			Text:  text,
			Level: 0,
		}
		if i == 0 {
			node.Type = "root"
		}
		diagram.Nodes = append(diagram.Nodes, node)
	}

	return diagram, nil
}

// DetectDiagramType attempts to determine the diagram type from layout XML.
func DetectDiagramType(layoutXML []byte) string {
	// Check for common layout types
	str := string(layoutXML)

	switch {
	case strings.Contains(str, "orgChart") || strings.Contains(str, "hierarchy"):
		return "hierarchy"
	case strings.Contains(str, "process") || strings.Contains(str, "flowChart"):
		return "process"
	case strings.Contains(str, "cycle"):
		return "cycle"
	case strings.Contains(str, "radial"):
		return "radial"
	case strings.Contains(str, "matrix"):
		return "matrix"
	case strings.Contains(str, "pyramid"):
		return "pyramid"
	case strings.Contains(str, "bullet"):
		return "bullet"
	case strings.Contains(str, "list"):
		return "list"
	default:
		return "hierarchy" // Default to hierarchy
	}
}

// parseSmartArtColorPalette extracts an ordered list of hex RGB colors from a
// colorsDef XML part.  It scans for <a:srgbClr>, <a:schemeClr>, and
// <a:scrgbClr> inside the first <cs:fillClrLst> (or <dgm:fillClrLst>) it
// finds.  Scheme colors are resolved through themeColors when the map is
// provided.  Best-effort only.
func parseSmartArtColorPalette(colorXML []byte, themeColors map[string]string) []string {
	return parseSmartArtColorList(colorXML, themeColors, "fillClrLst")
}

// parseSmartArtLinePalette extracts an ordered list of hex RGB colors from the
// <cs:linClrLst> (or <dgm:linClrLst>) element of a colorsDef XML part.
// Scheme colors are resolved through themeColors when the map is provided.
func parseSmartArtLinePalette(colorXML []byte, themeColors map[string]string) []string {
	return parseSmartArtColorList(colorXML, themeColors, "linClrLst")
}

// parseSmartArtTextFillPalette extracts an ordered list of hex RGB colors from
// the <cs:txFillClrLst> (or <dgm:txFillClrLst>) element of a colorsDef XML part.
func parseSmartArtTextFillPalette(colorXML []byte, themeColors map[string]string) []string {
	return parseSmartArtColorList(colorXML, themeColors, "txFillClrLst")
}

// parseSmartArtTextLinePalette extracts an ordered list of hex RGB colors from
// the <cs:txLinClrLst> (or <dgm:txLinClrLst>) element of a colorsDef XML part.
func parseSmartArtTextLinePalette(colorXML []byte, themeColors map[string]string) []string {
	return parseSmartArtColorList(colorXML, themeColors, "txLinClrLst")
}

// parseSmartArtEffectPalette extracts an ordered list of hex RGB colors from
// the <cs:effectClrLst> (or <dgm:effectClrLst>) element of a colorsDef XML part.
func parseSmartArtEffectPalette(colorXML []byte, themeColors map[string]string) []string {
	return parseSmartArtColorList(colorXML, themeColors, "effectClrLst")
}

// parseSmartArtColorList extracts colors from the first occurrence of the
// named color-list element (e.g. fillClrLst or linClrLst).  It resolves
// srgbClr, schemeClr, and scrgbClr values.
func parseSmartArtColorList(xmlData []byte, themeColors map[string]string, listName string) []string {
	if len(xmlData) == 0 {
		return nil
	}
	d := xml.NewDecoder(bytes.NewReader(xmlData))
	var palette []string
	inList := false
	for {
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == listName {
				inList = true
				palette = nil
			}
			if !inList {
				continue
			}
			if color := decodeSmartArtColor(t, themeColors); color != "" {
				palette = append(palette, color)
			}
		case xml.EndElement:
			if t.Name.Local == listName {
				if len(palette) > 0 {
					return palette
				}
				inList = false
			}
		}
	}
	return nil
}

// parseSmartArtStyleShapeType extracts a shape-type hint from a styleDef XML
// part. It prefers an explicit qs:prstGeom@prst value and falls back to
// searching for common shape names in the raw XML string.
func parseSmartArtStyleShapeType(styleXML []byte) string {
	if len(styleXML) == 0 {
		return ""
	}

	var doc struct {
		StyleLbls []struct {
			Style struct {
				PrstGeom struct {
					Prst string `xml:"prst,attr"`
				} `xml:"prstGeom"`
			} `xml:"style"`
		} `xml:"styleLbl"`
	}
	if err := xml.Unmarshal(styleXML, &doc); err == nil {
		for _, lbl := range doc.StyleLbls {
			prst := strings.ToLower(lbl.Style.PrstGeom.Prst)
			if prst != "" && prst != "rectangle" && prst != "rect" {
				return lbl.Style.PrstGeom.Prst
			}
		}
	}

	// Fallback: search raw text for known shape names.
	s := strings.ToLower(string(styleXML))
	switch {
	case strings.Contains(s, "ellipse"):
		return "ellipse"
	case strings.Contains(s, "roundrect") || strings.Contains(s, "roundrect"):
		return "roundRect"
	case strings.Contains(s, "hexagon"):
		return "hexagon"
	case strings.Contains(s, "pentagon"):
		return "pentagon"
	case strings.Contains(s, "diamond"):
		return "diamond"
	case strings.Contains(s, "triangle"):
		return "triangle"
	default:
		return ""
	}
}

// parseSmartArtStyle extracts resolved fill, line, effect, connector-line, and
// text-color hints from a dgm:styleDef XML part. It handles direct <a:ln>
// elements and resolves <a:fillRef>, <a:lnRef>, <a:effectRef>, and <a:fontRef>
// matrix references against the supplied ThemeFormatScheme. Colors may be
// srgbClr, schemeClr (resolved through themeColors), or scrgbClr. Widths from
// <a:ln> are converted from EMU to points. The returned bool is true when a
// line specification (including an explicit "no line" lnRef idx="0") was
// found. Connector styles from labels such as "lnNode1" are returned
// separately. Best-effort only.
func parseSmartArtStyle(styleXML []byte, themeColors map[string]string, fmtScheme *ThemeFormatScheme) (drawing.Fill, *drawing.LineStyle, *drawing.LineStyle, drawing.EffectList, string, bool) {
	if len(styleXML) == 0 {
		return nil, nil, nil, drawing.EffectList{}, "", false
	}

	d := xml.NewDecoder(bytes.NewReader(styleXML))
	var lblName string
	var refIdx string
	var childColor string
	var lnWidthEMU float64

	var curFill drawing.Fill
	var curLine *drawing.LineStyle
	var curLineExplicit bool
	var curTextColor string
	var curEffect drawing.EffectList
	var curDash drawing.DashPattern
	var curHeadEnd *drawing.Arrowhead
	var curTailEnd *drawing.Arrowhead

	var nodeFill drawing.Fill
	var nodeLine *drawing.LineStyle
	var nodeLineExplicit bool
	var nodeTextColor string
	var nodeEffect drawing.EffectList
	var haveNode bool

	var connLine *drawing.LineStyle

	applyColorOverride := func(fill drawing.Fill, color string) drawing.Fill {
		if color == "" {
			return fill
		}
		if sf, ok := fill.(drawing.SolidFill); ok {
			sf.Color = color
			sf.Alpha = 1.0
			return sf
		}
		return fill
	}

	for {
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, nil, drawing.EffectList{}, "", false
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "styleLbl":
				lblName = ""
				curFill = nil
				curLine = nil
				curLineExplicit = false
				curTextColor = ""
				curEffect = drawing.EffectList{}
				for _, a := range t.Attr {
					if a.Name.Local == "name" {
						lblName = a.Value
						break
					}
				}
			case "ln":
				curLineExplicit = true
				lnWidthEMU = 0
				curDash = ""
				curHeadEnd = nil
				curTailEnd = nil
				for _, a := range t.Attr {
					if a.Name.Local == "w" {
						if w, err := strconv.ParseFloat(a.Value, 64); err == nil {
							lnWidthEMU = w
						}
						break
					}
				}
			case "prstDash":
				for _, a := range t.Attr {
					if a.Name.Local == "val" {
						curDash = drawing.DashPattern(a.Value)
						break
					}
				}
			case "headEnd":
				curHeadEnd = parseSmartArtArrowhead(t.Attr)
			case "tailEnd":
				curTailEnd = parseSmartArtArrowhead(t.Attr)
			case "lnRef":
				curLineExplicit = true
				refIdx = ""
				for _, a := range t.Attr {
					if a.Name.Local == "idx" {
						refIdx = a.Value
						break
					}
				}
			case "fillRef":
				refIdx = ""
				for _, a := range t.Attr {
					if a.Name.Local == "idx" {
						refIdx = a.Value
						break
					}
				}
			case "effectRef":
				refIdx = ""
				for _, a := range t.Attr {
					if a.Name.Local == "idx" {
						refIdx = a.Value
						break
					}
				}
			case "effectLst":
				effects := parseSmartArtEffectListWithTheme(d, themeColors)
				curEffect.Effects = append(curEffect.Effects, effects.Effects...)
			case "fontRef":
				refIdx = ""
				for _, a := range t.Attr {
					if a.Name.Local == "idx" {
						refIdx = a.Value
						break
					}
				}
			case "srgbClr", "schemeClr", "scrgbClr":
				color := decodeSmartArtColor(t, themeColors)
				if color != "" {
					childColor = color
				}
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "ln":
				width := lnWidthEMU / 12700.0
				if width == 0 {
					width = 1.5
				}
				color := childColor
				if color == "" {
					color = "000000"
				}
				curLine = &drawing.LineStyle{
					Color:   color,
					Width:   width,
					Alpha:   1.0,
					Dash:    curDash,
					HeadEnd: curHeadEnd,
					TailEnd: curTailEnd,
				}
				childColor = ""
			case "lnRef":
				if refIdx == "0" {
					curLine = nil
				} else if fmtScheme != nil {
					if entry, ok := resolveLineStyleRef(refIdx, fmtScheme); ok {
						curLine = entry.Line
						if entry.UsesPlaceholder && childColor != "" && curLine != nil {
							curLine = &drawing.LineStyle{
								Color:        childColor,
								Width:        curLine.Width,
								Alpha:        1.0,
								Dash:         curLine.Dash,
								CompoundType: curLine.CompoundType,
								HeadEnd:      curLine.HeadEnd,
								TailEnd:      curLine.TailEnd,
							}
						}
					}
				}
				childColor = ""
			case "fillRef":
				if refIdx == "0" {
					curFill = drawing.NoFill{}
				} else if fmtScheme != nil {
					if entry, ok := resolveFillStyleRef(refIdx, fmtScheme); ok {
						curFill = applyColorOverride(entry.Fill, childColor)
					}
				}
				childColor = ""
			case "effectRef":
				if refIdx != "0" && fmtScheme != nil {
					if eff, ok := resolveEffectStyleRef(refIdx, fmtScheme); ok {
						curEffect.Effects = append(curEffect.Effects, eff.Effects...)
					}
				}
				childColor = ""
			case "fontRef":
				if childColor != "" {
					curTextColor = childColor
				}
				childColor = ""
			case "styleLbl":
				if isConnectorStyleLabel(lblName) {
					connLine = curLine
				} else if !haveNode {
					nodeFill = curFill
					nodeLine = curLine
					nodeLineExplicit = curLineExplicit
					nodeTextColor = curTextColor
					nodeEffect = curEffect
					haveNode = true
				}
				childColor = ""
			}
		}
	}

	return nodeFill, nodeLine, connLine, nodeEffect, nodeTextColor, nodeLineExplicit
}

// parseSmartArtEffectList token-parses an a:effectLst element and returns the
// resolved drawing.EffectList.  Scheme colors are resolved through themeColors.
func parseSmartArtEffectList(d *xml.Decoder) drawing.EffectList {
	return parseSmartArtEffectListWithTheme(d, nil)
}

func parseSmartArtEffectListWithTheme(d *xml.Decoder, themeColors map[string]string) drawing.EffectList {
	type alphaVal struct {
		Val int64 `xml:"val,attr"`
	}
	type colorInfo struct {
		Val   string    `xml:"val,attr"`
		Alpha *alphaVal `xml:"alpha"`
	}
	resolveColor := func(c *colorInfo) (string, float64) {
		if c == nil {
			return "", 1.0
		}
		color := c.Val
		if themeColors != nil {
			if resolved, ok := themeColors[c.Val]; ok {
				color = resolved
			}
		}
		alpha := 1.0
		if c.Alpha != nil {
			alpha = float64(c.Alpha.Val) / 100000.0
		}
		return color, alpha
	}
	resolveEitherColor := func(srgb, scheme *colorInfo) (string, float64) {
		if srgb != nil {
			return resolveColor(srgb)
		}
		return resolveColor(scheme)
	}
	type outerShdw struct {
		BlurRad      int64      `xml:"blurRad,attr,omitempty"`
		Dist         int64      `xml:"dist,attr,omitempty"`
		Dir          int        `xml:"dir,attr,omitempty"`
		Sx           int        `xml:"sx,attr,omitempty"`
		Sy           int        `xml:"sy,attr,omitempty"`
		Kx           int        `xml:"kx,attr,omitempty"`
		Ky           int        `xml:"ky,attr,omitempty"`
		Algn         string     `xml:"algn,attr,omitempty"`
		RotWithShape *bool      `xml:"rotWithShape,attr,omitempty"`
		SrgbClr      *colorInfo `xml:"srgbClr"`
		SchemeClr    *colorInfo `xml:"schemeClr"`
	}
	type innerShdw struct {
		BlurRad   int64      `xml:"blurRad,attr,omitempty"`
		Dist      int64      `xml:"dist,attr,omitempty"`
		Dir       int        `xml:"dir,attr,omitempty"`
		SrgbClr   *colorInfo `xml:"srgbClr"`
		SchemeClr *colorInfo `xml:"schemeClr"`
	}
	type glow struct {
		Rad       int64      `xml:"rad,attr,omitempty"`
		SrgbClr   *colorInfo `xml:"srgbClr"`
		SchemeClr *colorInfo `xml:"schemeClr"`
	}
	type softEdge struct {
		Rad int64 `xml:"rad,attr,omitempty"`
	}
	type reflection struct {
		BlurRad      int64  `xml:"blurRad,attr,omitempty"`
		Dist         int64  `xml:"dist,attr,omitempty"`
		Dir          int    `xml:"dir,attr,omitempty"`
		StA          int    `xml:"stA,attr,omitempty"`
		StPos        int    `xml:"stPos,attr,omitempty"`
		EndA         int    `xml:"endA,attr,omitempty"`
		EndPos       int    `xml:"endPos,attr,omitempty"`
		FadeDir      int    `xml:"fadeDir,attr,omitempty"`
		Sx           int    `xml:"sx,attr,omitempty"`
		Sy           int    `xml:"sy,attr,omitempty"`
		Kx           int    `xml:"kx,attr,omitempty"`
		Ky           int    `xml:"ky,attr,omitempty"`
		Algn         string `xml:"algn,attr,omitempty"`
		RotWithShape *bool  `xml:"rotWithShape,attr,omitempty"`
	}
	type blur struct {
		Rad  int64 `xml:"rad,attr,omitempty"`
		Grow *bool `xml:"grow,attr,omitempty"`
	}
	type presetShadow struct {
		Prst      string     `xml:"prst,attr"`
		Dist      int64      `xml:"dist,attr,omitempty"`
		Dir       int        `xml:"dir,attr,omitempty"`
		SrgbClr   *colorInfo `xml:"srgbClr"`
		SchemeClr *colorInfo `xml:"schemeClr"`
	}
	type gradStop struct {
		Pos       int        `xml:"pos,attr"`
		SrgbClr   *colorInfo `xml:"srgbClr"`
		SchemeClr *colorInfo `xml:"schemeClr"`
	}
	type gradFill struct {
		Gs  []gradStop `xml:"gsLst>gs"`
		Lin *struct {
			Ang int `xml:"ang,attr"`
		} `xml:"lin"`
	}
	type pattFill struct {
		Prst  string `xml:"prst,attr"`
		FgClr *struct {
			SrgbClr   *colorInfo `xml:"srgbClr"`
			SchemeClr *colorInfo `xml:"schemeClr"`
		} `xml:"fgClr"`
		BgClr *struct {
			SrgbClr   *colorInfo `xml:"srgbClr"`
			SchemeClr *colorInfo `xml:"schemeClr"`
		} `xml:"bgClr"`
	}
	type fillOverlay struct {
		Blend     string    `xml:"blend,attr,omitempty"`
		NoFill    *struct{} `xml:"noFill"`
		SolidFill *struct {
			SrgbClr   *colorInfo `xml:"srgbClr"`
			SchemeClr *colorInfo `xml:"schemeClr"`
		} `xml:"solidFill"`
		GradFill *gradFill `xml:"gradFill"`
		PattFill *pattFill `xml:"pattFill"`
	}
	resolveOverlayFill := func(e fillOverlay) drawing.Fill {
		if e.NoFill != nil {
			return drawing.NoFill{}
		}
		if e.SolidFill != nil {
			color, alpha := resolveEitherColor(e.SolidFill.SrgbClr, e.SolidFill.SchemeClr)
			return drawing.SolidFill{Color: color, Alpha: alpha}
		}
		if e.GradFill != nil {
			stops := make([]drawing.GradientStop, 0, len(e.GradFill.Gs))
			for _, gs := range e.GradFill.Gs {
				color, alpha := resolveEitherColor(gs.SrgbClr, gs.SchemeClr)
				stops = append(stops, drawing.GradientStop{Position: float64(gs.Pos) / 100000.0, Color: color, Alpha: alpha})
			}
			angle := 0.0
			if e.GradFill.Lin != nil {
				angle = float64(e.GradFill.Lin.Ang) / 60000.0
			}
			return drawing.GradientFill{GradientType: drawing.GradientTypeLinear, Angle: angle, Stops: stops}
		}
		if e.PattFill != nil {
			fg, fga, bg, bga := "000000", 1.0, "FFFFFF", 1.0
			if e.PattFill.FgClr != nil {
				if c, a := resolveEitherColor(e.PattFill.FgClr.SrgbClr, e.PattFill.FgClr.SchemeClr); c != "" {
					fg, fga = c, a
				}
			}
			if e.PattFill.BgClr != nil {
				if c, a := resolveEitherColor(e.PattFill.BgClr.SrgbClr, e.PattFill.BgClr.SchemeClr); c != "" {
					bg, bga = c, a
				}
			}
			return drawing.PatternFill{PatternType: drawing.PatternType(e.PattFill.Prst), Foreground: fg, Background: bg, ForeAlpha: fga, BackAlpha: bga}
		}
		return nil
	}

	var effects []drawing.Effect
	for {
		tok, err := d.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "outerShdw":
				var e outerShdw
				_ = d.DecodeElement(&e, &t)
				color, alpha := resolveEitherColor(e.SrgbClr, e.SchemeClr)
				sx, sy := 1.0, 1.0
				if e.Sx != 0 {
					sx = float64(e.Sx) / 100000.0
				}
				if e.Sy != 0 {
					sy = float64(e.Sy) / 100000.0
				}
				effects = append(effects, drawing.Shadow{Color: color, Alpha: alpha, Blur: drawing.EMUToPoints(e.BlurRad), Distance: drawing.EMUToPoints(e.Dist), Angle: float64(e.Dir) / 60000.0, ScaleX: sx, ScaleY: sy, SkewX: float64(e.Kx) / 60000.0, SkewY: float64(e.Ky) / 60000.0, Align: e.Algn != "", Alignment: e.Algn, RotateWithShape: e.RotWithShape != nil && *e.RotWithShape})
			case "prstShdw":
				var e presetShadow
				_ = d.DecodeElement(&e, &t)
				color, alpha := resolveEitherColor(e.SrgbClr, e.SchemeClr)
				effects = append(effects, drawing.Shadow{Color: color, Alpha: alpha, Distance: drawing.EMUToPoints(e.Dist), Angle: float64(e.Dir) / 60000.0, ScaleX: 1, ScaleY: 1})
			case "innerShdw":
				var e innerShdw
				_ = d.DecodeElement(&e, &t)
				color, alpha := resolveEitherColor(e.SrgbClr, e.SchemeClr)
				effects = append(effects, drawing.InnerShadow{Color: color, Alpha: alpha, Blur: drawing.EMUToPoints(e.BlurRad), Distance: drawing.EMUToPoints(e.Dist), Angle: float64(e.Dir) / 60000.0})
			case "glow":
				var e glow
				_ = d.DecodeElement(&e, &t)
				color, alpha := resolveEitherColor(e.SrgbClr, e.SchemeClr)
				effects = append(effects, drawing.Glow{Color: color, Alpha: alpha, Radius: drawing.EMUToPoints(e.Rad)})
			case "softEdge":
				var e softEdge
				_ = d.DecodeElement(&e, &t)
				effects = append(effects, drawing.SoftEdge{Radius: drawing.EMUToPoints(e.Rad)})
			case "reflection":
				var e reflection
				_ = d.DecodeElement(&e, &t)
				alpha := 0.3
				if e.StA > 0 {
					alpha = float64(e.StA) / 100000.0
				}
				sx, sy := 1.0, 1.0
				if e.Sx != 0 {
					sx = float64(e.Sx) / 100000.0
				}
				if e.Sy != 0 {
					sy = float64(e.Sy) / 100000.0
				}
				effects = append(effects, drawing.Reflection{Alpha: alpha, Blur: drawing.EMUToPoints(e.BlurRad), Distance: drawing.EMUToPoints(e.Dist), Direction: float64(e.Dir) / 60000.0, Size: sy, Fade: e.StA != 0 || e.EndA != 0 || e.FadeDir != 0, StartAlpha: e.StA, StartPosition: e.StPos, EndAlpha: e.EndA, EndPosition: e.EndPos, FadeDirection: float64(e.FadeDir) / 60000.0, ScaleX: sx, ScaleY: sy, SkewX: float64(e.Kx) / 60000.0, SkewY: float64(e.Ky) / 60000.0, Alignment: e.Algn, RotateWithShape: e.RotWithShape != nil && *e.RotWithShape})
			case "blur":
				var e blur
				_ = d.DecodeElement(&e, &t)
				effects = append(effects, drawing.Blur{Radius: drawing.EMUToPoints(e.Rad), Grow: e.Grow != nil && *e.Grow})
			case "fillOverlay":
				var e fillOverlay
				_ = d.DecodeElement(&e, &t)
				effects = append(effects, drawing.FillOverlay{Fill: resolveOverlayFill(e), BlendMode: e.Blend})
			case "alphaMod":
				var e struct {
					Amt int `xml:"amt,attr"`
				}
				_ = d.DecodeElement(&e, &t)
				effects = append(effects, drawing.AlphaMod{Amount: e.Amt})
			case "alphaModFix":
				var e struct {
					Amt int `xml:"amt,attr"`
				}
				_ = d.DecodeElement(&e, &t)
				effects = append(effects, drawing.AlphaModFix{Amount: e.Amt})
			case "alphaOff":
				var e struct {
					Amt int `xml:"amt,attr"`
				}
				_ = d.DecodeElement(&e, &t)
				effects = append(effects, drawing.AlphaOff{Amount: e.Amt})
			case "alphaRepl":
				var e struct {
					A int `xml:"a,attr"`
				}
				_ = d.DecodeElement(&e, &t)
				effects = append(effects, drawing.AlphaRepl{Alpha: e.A})
			case "alphaBiLevel":
				var e struct {
					Thresh int `xml:"thresh,attr"`
				}
				_ = d.DecodeElement(&e, &t)
				effects = append(effects, drawing.AlphaBiLevel{Threshold: e.Thresh})
			case "alphaOutset":
				var e struct {
					Rad int64 `xml:"rad,attr"`
				}
				_ = d.DecodeElement(&e, &t)
				effects = append(effects, drawing.AlphaOutset{Radius: drawing.EMUToPoints(e.Rad)})
			case "alphaInv":
				_ = d.Skip()
				effects = append(effects, drawing.AlphaInv{})
			case "alphaCeiling":
				_ = d.Skip()
				effects = append(effects, drawing.AlphaCeiling{})
			case "alphaFloor":
				_ = d.Skip()
				effects = append(effects, drawing.AlphaFloor{})
			case "grayscl":
				_ = d.Skip()
				effects = append(effects, drawing.Grayscale{})
			case "deSat":
				_ = d.Skip()
				effects = append(effects, drawing.DeSat{})
			case "deSatLow":
				_ = d.Skip()
				effects = append(effects, drawing.DeSatLow{})
			case "deSatMod":
				var e struct {
					Amt int `xml:"amt,attr"`
				}
				_ = d.DecodeElement(&e, &t)
				effects = append(effects, drawing.DeSatMod{Amount: e.Amt})
			case "hsleff":
				var e struct{ Hue, Sat, Lum int }
				for _, a := range t.Attr {
					switch a.Name.Local {
					case "hue":
						fmt.Sscan(a.Value, &e.Hue)
					case "sat":
						fmt.Sscan(a.Value, &e.Sat)
					case "lum":
						fmt.Sscan(a.Value, &e.Lum)
					}
				}
				_ = d.Skip()
				effects = append(effects, drawing.HSLEffect{Hue: e.Hue, Sat: e.Sat, Lum: e.Lum})
			case "lum":
				var e struct {
					Bright   int `xml:"bright,attr"`
					Contrast int `xml:"contrast,attr"`
				}
				_ = d.DecodeElement(&e, &t)
				effects = append(effects, drawing.Lum{Brightness: e.Bright, Contrast: e.Contrast})
			case "clrRepl":
				var e struct {
					SrgbClr   *colorInfo `xml:"srgbClr"`
					SchemeClr *colorInfo `xml:"schemeClr"`
				}
				_ = d.DecodeElement(&e, &t)
				color, _ := resolveEitherColor(e.SrgbClr, e.SchemeClr)
				effects = append(effects, drawing.ClrRepl{Color: color})
			case "relOff":
				var e struct {
					Tx int `xml:"tx,attr"`
					Ty int `xml:"ty,attr"`
				}
				_ = d.DecodeElement(&e, &t)
				effects = append(effects, drawing.RelativeOffset{X: float64(e.Tx) / 100000.0, Y: float64(e.Ty) / 100000.0})
			case "tint":
				var e struct {
					Hue int `xml:"hue,attr"`
					Amt int `xml:"amt,attr"`
				}
				_ = d.DecodeElement(&e, &t)
				effects = append(effects, drawing.Tint{Hue: e.Hue, Amount: e.Amt})
			case "xfrm":
				var e struct {
					Sx int `xml:"sx,attr"`
					Sy int `xml:"sy,attr"`
					Kx int `xml:"kx,attr"`
					Ky int `xml:"ky,attr"`
					Tx int `xml:"tx,attr"`
					Ty int `xml:"ty,attr"`
				}
				_ = d.DecodeElement(&e, &t)
				sx, sy := 1.0, 1.0
				if e.Sx != 0 {
					sx = float64(e.Sx) / 100000.0
				}
				if e.Sy != 0 {
					sy = float64(e.Sy) / 100000.0
				}
				effects = append(effects, drawing.TransformEffect{ScaleX: sx, ScaleY: sy, SkewX: float64(e.Kx) / 60000.0, SkewY: float64(e.Ky) / 60000.0, TranslateX: float64(e.Tx) / 100000.0, TranslateY: float64(e.Ty) / 100000.0})
			default:
				_ = d.Skip()
			}
		case xml.EndElement:
			if t.Name.Local == "effectLst" {
				return drawing.EffectList{Effects: effects}
			}
		}
	}
	return drawing.EffectList{Effects: effects}
}

func resolveFillStyleRef(idxStr string, fmtScheme *ThemeFormatScheme) (ThemeFillStyle, bool) {
	var empty ThemeFillStyle
	if fmtScheme == nil {
		return empty, false
	}
	idx, bg, ok := parseStyleMatrixIndex(idxStr)
	if !ok {
		return empty, false
	}
	if bg {
		if idx < 0 || idx >= len(fmtScheme.BgFillStyles) {
			return empty, false
		}
		return fmtScheme.BgFillStyles[idx], true
	}
	if idx < 0 || idx >= len(fmtScheme.FillStyles) {
		return empty, false
	}
	return fmtScheme.FillStyles[idx], true
}

// resolveLineStyleRef returns the theme line entry referenced by a style ref
// index. Indices are 1-based; idx "0" means no line.
func resolveLineStyleRef(idxStr string, fmtScheme *ThemeFormatScheme) (ThemeLineStyle, bool) {
	var empty ThemeLineStyle
	if fmtScheme == nil {
		return empty, false
	}
	idx, _, ok := parseStyleMatrixIndex(idxStr)
	if !ok {
		return empty, false
	}
	if idx < 0 || idx >= len(fmtScheme.LineStyles) {
		return empty, false
	}
	return fmtScheme.LineStyles[idx], true
}

// resolveEffectStyleRef returns the theme effect list referenced by a style
// ref index. Indices are 1-based; idx "0" means no effect.
func resolveEffectStyleRef(idxStr string, fmtScheme *ThemeFormatScheme) (drawing.EffectList, bool) {
	if fmtScheme == nil {
		return drawing.EffectList{}, false
	}
	idx, _, ok := parseStyleMatrixIndex(idxStr)
	if !ok {
		return drawing.EffectList{}, false
	}
	if idx < 0 || idx >= len(fmtScheme.EffectStyles) {
		return drawing.EffectList{}, false
	}
	return fmtScheme.EffectStyles[idx], true
}

// parseStyleMatrixIndex converts a style ref index string to a 0-based slice
// index and a flag indicating the background-fill list (idx >= 1001).
func parseStyleMatrixIndex(idxStr string) (idx int, bg bool, ok bool) {
	idxStr = strings.TrimSpace(idxStr)
	if idxStr == "" {
		return -1, false, false
	}
	v, err := strconv.Atoi(idxStr)
	if err != nil {
		return -1, false, false
	}
	if v == 0 {
		return -1, false, false
	}
	if v >= 1001 {
		return v - 1001, true, true
	}
	return v - 1, false, true
}

// parseSmartArtArrowhead builds an Arrowhead from <a:headEnd> or <a:tailEnd>
// attributes.
func parseSmartArtArrowhead(attrs []xml.Attr) *drawing.Arrowhead {
	a := &drawing.Arrowhead{Type: drawing.ArrowheadNone}
	for _, attr := range attrs {
		switch attr.Name.Local {
		case "type":
			a.Type = drawing.ArrowheadType(attr.Value)
		case "w":
			a.Size = drawing.ArrowheadSize(attr.Value)
		case "len":
			a.Length = drawing.ArrowheadSize(attr.Value)
		}
	}
	if a.Type == drawing.ArrowheadNone {
		return nil
	}
	return a
}

// isConnectorStyleLabel returns true for style labels that describe connector
// lines (e.g. "lnNode1").
func isConnectorStyleLabel(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasPrefix(lower, "ln") || strings.Contains(lower, "connector") || strings.Contains(lower, "cxn")
}

// decodeSmartArtColor converts an a:srgbClr, a:schemeClr, or a:scrgbClr start
// element into a hex RGB string.  Scheme colors are resolved through
// themeColors when the map is supplied.
func decodeSmartArtColor(t xml.StartElement, themeColors map[string]string) string {
	switch t.Name.Local {
	case "srgbClr":
		for _, a := range t.Attr {
			if a.Name.Local == "val" {
				return a.Value
			}
		}
	case "schemeClr":
		for _, a := range t.Attr {
			if a.Name.Local == "val" && themeColors != nil {
				return themeColors[a.Value]
			}
		}
	case "scrgbClr":
		var r, g, b int
		for _, a := range t.Attr {
			switch a.Name.Local {
			case "r":
				r, _ = strconv.Atoi(a.Value)
			case "g":
				g, _ = strconv.Atoi(a.Value)
			case "b":
				b, _ = strconv.Atoi(a.Value)
			}
		}
		return fmt.Sprintf("%02X%02X%02X", scrgbToByte(r), scrgbToByte(g), scrgbToByte(b))
	}
	return ""
}

// scrgbToByte converts an OOXML scrgbClr percentage (0..100000) to a byte
// value (0..255).
func scrgbToByte(v int) int {
	b := v * 255 / 100000
	if b < 0 {
		return 0
	}
	if b > 255 {
		return 255
	}
	return b
}

// spPrStyle holds per-node appearance overrides extracted from dgm:spPr.
type spPrStyle struct {
	Fill drawing.Fill
	Line *drawing.LineStyle
}

// parseDataModelNodeStyles extracts per-node fill and line overrides from the
// dgm:spPr elements inside the data model.  These are used as fallbacks when
// no pre-rendered drawing part is available.
func clampPictureFraction(value int) float64 {
	if value <= 0 {
		return 0
	}
	if value >= 100000 {
		return 1
	}
	return float64(value) / 100000.0
}

func maxPictureFraction(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func parseDataModelNodeStyles(xmlData []byte, themeColors map[string]string, fmtScheme *ThemeFormatScheme) map[string]*spPrStyle {
	if len(xmlData) == 0 {
		return nil
	}

	type colorInfo struct {
		Val   string `xml:"val,attr"`
		Alpha *struct {
			Val int64 `xml:"val,attr"`
		} `xml:"alpha"`
		AlphaMod *struct {
			Val int64 `xml:"val,attr"`
		} `xml:"alphaMod"`
		AlphaOff *struct {
			Val int64 `xml:"val,attr"`
		} `xml:"alphaOff"`
	}
	resolveInfo := func(c *colorInfo) (string, float64) {
		if c == nil {
			return "", 1.0
		}
		alpha := 1.0
		if c.Alpha != nil {
			alpha = float64(c.Alpha.Val) / 100000.0
		}
		if c.AlphaMod != nil {
			alpha *= float64(c.AlphaMod.Val) / 100000.0
		}
		if c.AlphaOff != nil {
			alpha += float64(c.AlphaOff.Val) / 100000.0
		}
		if alpha < 0 {
			alpha = 0
		} else if alpha > 1 {
			alpha = 1
		}
		return c.Val, alpha
	}
	resolveColor := func(c *colorInfo) string {
		color, _ := resolveInfo(c)
		if c != nil && themeColors != nil {
			if scheme, ok := themeColors[color]; ok {
				return scheme
			}
		}
		return color
	}
	resolveAlpha := func(c *colorInfo) float64 {
		_, alpha := resolveInfo(c)
		return alpha
	}
	type solidFillDef struct {
		SrgbClr   *colorInfo `xml:"srgbClr"`
		SchemeClr *colorInfo `xml:"schemeClr"`
		ScrgbClr  *struct {
			R int `xml:"r,attr"`
			G int `xml:"g,attr"`
			B int `xml:"b,attr"`
		} `xml:"scrgbClr"`
	}
	resolveSpPrSolidFill := func(sf *solidFillDef) (string, float64) {
		if sf == nil {
			return "", 1.0
		}
		switch {
		case sf.SrgbClr != nil:
			return resolveInfo(sf.SrgbClr)
		case sf.SchemeClr != nil:
			return resolveColor(sf.SchemeClr), resolveAlpha(sf.SchemeClr)
		case sf.ScrgbClr != nil:
			c := sf.ScrgbClr
			return fmt.Sprintf("%02X%02X%02X", scrgbToByte(c.R), scrgbToByte(c.G), scrgbToByte(c.B)), 1.0
		}
		return "", 1.0
	}

	type gradFillDef struct {
		GsLst []struct {
			Pos       int64      `xml:"pos,attr"`
			SrgbClr   *colorInfo `xml:"srgbClr"`
			SchemeClr *colorInfo `xml:"schemeClr"`
			ScrgbClr  *struct {
				R int `xml:"r,attr"`
				G int `xml:"g,attr"`
				B int `xml:"b,attr"`
			} `xml:"scrgbClr"`
		} `xml:"gsLst>gs"`
		Lin *struct {
			Ang int64 `xml:"ang,attr"`
		} `xml:"lin"`
	}

	parseGrad := func(gf *gradFillDef) drawing.Fill {
		if gf == nil || len(gf.GsLst) == 0 {
			return nil
		}
		var stops []drawing.GradientStop
		for _, gs := range gf.GsLst {
			var color string
			var alpha float64 = 1.0
			switch {
			case gs.SrgbClr != nil:
				color, alpha = resolveInfo(gs.SrgbClr)
			case gs.SchemeClr != nil:
				color = resolveColor(gs.SchemeClr)
				alpha = resolveAlpha(gs.SchemeClr)
			case gs.ScrgbClr != nil:
				c := gs.ScrgbClr
				color = fmt.Sprintf("%02X%02X%02X", scrgbToByte(c.R), scrgbToByte(c.G), scrgbToByte(c.B))
			}
			if color == "" {
				continue
			}
			stops = append(stops, drawing.GradientStop{
				Position: float64(gs.Pos) / 100000.0,
				Color:    color,
				Alpha:    alpha,
			})
		}
		if len(stops) == 0 {
			return nil
		}
		angle := 0.0
		if gf.Lin != nil {
			angle = float64(gf.Lin.Ang) / 60000.0
		}
		return drawing.GradientFill{
			GradientType: drawing.GradientTypeLinear,
			Angle:        angle,
			Stops:        stops,
		}
	}

	type styleRefDef struct {
		Idx       string     `xml:"idx,attr"`
		SrgbClr   *colorInfo `xml:"srgbClr"`
		SchemeClr *colorInfo `xml:"schemeClr"`
		ScrgbClr  *struct {
			R int `xml:"r,attr"`
			G int `xml:"g,attr"`
			B int `xml:"b,attr"`
		} `xml:"scrgbClr"`
	}
	applyColorOverride := func(fill drawing.Fill, color string) drawing.Fill {
		if color == "" {
			return fill
		}
		if sf, ok := fill.(drawing.SolidFill); ok {
			sf.Color = color
			sf.Alpha = 1.0
			return sf
		}
		return fill
	}
	resolveStyleRefColor := func(ref *styleRefDef) (string, float64) {
		if ref == nil {
			return "", 1.0
		}
		switch {
		case ref.SrgbClr != nil:
			return resolveInfo(ref.SrgbClr)
		case ref.SchemeClr != nil:
			return resolveColor(ref.SchemeClr), resolveAlpha(ref.SchemeClr)
		case ref.ScrgbClr != nil:
			c := ref.ScrgbClr
			return fmt.Sprintf("%02X%02X%02X", scrgbToByte(c.R), scrgbToByte(c.G), scrgbToByte(c.B)), 1.0
		}
		return "", 1.0
	}

	var doc struct {
		PtLst []struct {
			ModelID string `xml:"modelId,attr"`
			ID      string `xml:"id,attr"`
			SpPr    *struct {
				NoFill    *struct{}     `xml:"noFill"`
				SolidFill *solidFillDef `xml:"solidFill"`
				GradFill  *gradFillDef  `xml:"gradFill"`
				PattFill  *struct {
					Prst  string        `xml:"prst,attr"`
					FgClr *solidFillDef `xml:"fgClr"`
					BgClr *solidFillDef `xml:"bgClr"`
				} `xml:"pattFill"`
				BlipFill *struct {
					DPI          int    `xml:"dpi,attr"`
					RotWithShape string `xml:"rotWithShape,attr"`
					Blip         struct {
						Embed string `xml:"embed,attr"`
					} `xml:"blip"`
					SrcRect *struct {
						L int `xml:"l,attr"`
						T int `xml:"t,attr"`
						R int `xml:"r,attr"`
						B int `xml:"b,attr"`
					} `xml:"srcRect"`
					Stretch *struct {
						FillRect *struct {
							L int `xml:"l,attr"`
							T int `xml:"t,attr"`
							R int `xml:"r,attr"`
							B int `xml:"b,attr"`
						} `xml:"fillRect"`
					} `xml:"stretch"`
				} `xml:"blipFill"`
				Ln *struct {
					W         int64         `xml:"w,attr"`
					SolidFill *solidFillDef `xml:"solidFill"`
					GradFill  *gradFillDef  `xml:"gradFill"`
				} `xml:"ln"`
				Style *struct {
					LnRef     *styleRefDef `xml:"lnRef"`
					FillRef   *styleRefDef `xml:"fillRef"`
					EffectRef *styleRefDef `xml:"effectRef"`
					FontRef   *styleRefDef `xml:"fontRef"`
				} `xml:"style"`
			} `xml:"spPr"`
		} `xml:"ptLst>pt"`
	}

	if err := xml.Unmarshal(xmlData, &doc); err != nil {
		return nil
	}

	styles := make(map[string]*spPrStyle)
	for _, pt := range doc.PtLst {
		id := pt.ModelID
		if id == "" {
			id = pt.ID
		}
		if id == "" || pt.SpPr == nil {
			continue
		}
		style := &spPrStyle{}

		// Resolve a:style matrix references first; direct properties override them.
		if pt.SpPr.Style != nil && fmtScheme != nil {
			if ref := pt.SpPr.Style.FillRef; ref != nil {
				if ref.Idx == "0" {
					style.Fill = drawing.NoFill{}
				} else if entry, ok := resolveFillStyleRef(ref.Idx, fmtScheme); ok {
					color, _ := resolveStyleRefColor(ref)
					style.Fill = applyColorOverride(entry.Fill, color)
				}
			}
			if ref := pt.SpPr.Style.LnRef; ref != nil {
				if ref.Idx == "0" {
					style.Line = nil
				} else if entry, ok := resolveLineStyleRef(ref.Idx, fmtScheme); ok {
					color, _ := resolveStyleRefColor(ref)
					line := entry.Line
					if entry.UsesPlaceholder && color != "" && line != nil {
						line = &drawing.LineStyle{
							Color:        color,
							Width:        line.Width,
							Alpha:        1.0,
							Dash:         line.Dash,
							CompoundType: line.CompoundType,
							HeadEnd:      line.HeadEnd,
							TailEnd:      line.TailEnd,
						}
					}
					style.Line = line
				}
			}
		}

		if pt.SpPr.NoFill != nil {
			style.Fill = drawing.NoFill{}
		}
		if pt.SpPr.SolidFill != nil {
			color, alpha := resolveSpPrSolidFill(pt.SpPr.SolidFill)
			if color != "" {
				style.Fill = drawing.SolidFill{Color: color, Alpha: alpha}
			}
		}
		if gf := parseGrad(pt.SpPr.GradFill); gf != nil {
			style.Fill = gf
		}
		if pt.SpPr.PattFill != nil {
			fg, fgAlpha := resolveSpPrSolidFill(pt.SpPr.PattFill.FgClr)
			bg, bgAlpha := resolveSpPrSolidFill(pt.SpPr.PattFill.BgClr)
			if fg == "" {
				fg = "000000"
			}
			if bg == "" {
				bg = "FFFFFF"
			}
			style.Fill = drawing.PatternFill{
				PatternType: drawing.PatternType(pt.SpPr.PattFill.Prst),
				Foreground:  fg,
				Background:  bg,
				ForeAlpha:   fgAlpha,
				BackAlpha:   bgAlpha,
			}
		}
		if pt.SpPr.BlipFill != nil && pt.SpPr.BlipFill.Blip.Embed != "" {
			bf := pt.SpPr.BlipFill
			pf := &drawing.PictureFill{EmbedID: bf.Blip.Embed, Stretch: true, Alpha: 1.0, DPI: bf.DPI}
			if bf.RotWithShape != "" {
				pf.RotWithShapeSet = true
				pf.RotWithShape = bf.RotWithShape != "0" && bf.RotWithShape != "false"
			}
			if bf.SrcRect != nil {
				l, t, r, b := clampPictureFraction(bf.SrcRect.L), clampPictureFraction(bf.SrcRect.T), clampPictureFraction(bf.SrcRect.R), clampPictureFraction(bf.SrcRect.B)
				pf.Crop = &drawing.Rect{X: l, Y: t, Width: maxPictureFraction(0, 1-l-r), Height: maxPictureFraction(0, 1-t-b)}
			}
			if bf.Stretch != nil && bf.Stretch.FillRect != nil {
				fr := bf.Stretch.FillRect
				l, t, r, b := clampPictureFraction(fr.L), clampPictureFraction(fr.T), clampPictureFraction(fr.R), clampPictureFraction(fr.B)
				pf.FillRect = &drawing.Rect{X: l, Y: t, Width: maxPictureFraction(0, 1-l-r), Height: maxPictureFraction(0, 1-t-b)}
			}
			style.Fill = pf
		}
		if pt.SpPr.Ln != nil {
			line := &drawing.LineStyle{Width: 1.0}
			if pt.SpPr.Ln.W > 0 {
				line.Width = float64(pt.SpPr.Ln.W) / 12700.0
			}
			if pt.SpPr.Ln.SolidFill != nil {
				color, alpha := resolveSpPrSolidFill(pt.SpPr.Ln.SolidFill)
				line.Color = color
				line.Alpha = alpha
			}
			if gf := parseGrad(pt.SpPr.Ln.GradFill); gf != nil {
				line.Fill = gf
				if grad, ok := gf.(drawing.GradientFill); ok && len(grad.Stops) > 0 {
					line.Color = grad.Stops[0].Color
					line.Alpha = grad.Stops[0].Alpha
				}
			}
			if line.Color != "" || line.Fill != nil || pt.SpPr.Ln.W > 0 {
				style.Line = line
			}
		}
		if style.Fill != nil || style.Line != nil {
			styles[id] = style
		}
	}
	return styles
}

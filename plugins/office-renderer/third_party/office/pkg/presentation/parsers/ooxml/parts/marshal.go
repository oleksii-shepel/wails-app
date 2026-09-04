package parts

import (
	"bytes"
	"encoding/xml"
	"io"
	"sort"
	"strconv"
)

// MarshalXML emits ShapeTree children in the order preserved in Items.
func (st ShapeTree) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	for _, item := range st.Items {
		switch item.Kind {
		case ShapeTreeItemShape:
			if item.Index >= 0 && item.Index < len(st.Shapes) {
				if err := e.EncodeElement(st.Shapes[item.Index], xml.StartElement{Name: xml.Name{Local: "sp"}}); err != nil {
					return err
				}
			}
		case ShapeTreeItemPicture:
			if item.Index >= 0 && item.Index < len(st.Pictures) {
				if err := e.EncodeElement(st.Pictures[item.Index], xml.StartElement{Name: xml.Name{Local: "pic"}}); err != nil {
					return err
				}
			}
		case ShapeTreeItemGroup:
			if item.Index >= 0 && item.Index < len(st.GroupShapes) {
				if err := e.EncodeElement(st.GroupShapes[item.Index], xml.StartElement{Name: xml.Name{Local: "grpSp"}}); err != nil {
					return err
				}
			}
		case ShapeTreeItemConnector:
			if item.Index >= 0 && item.Index < len(st.Connectors) {
				if err := e.EncodeElement(st.Connectors[item.Index], xml.StartElement{Name: xml.Name{Local: "cxnSp"}}); err != nil {
					return err
				}
			}
		case ShapeTreeItemTable:
			if item.Index >= 0 && item.Index < len(st.Tables) {
				if err := e.EncodeElement(st.Tables[item.Index], xml.StartElement{Name: xml.Name{Local: "graphicFrame"}}); err != nil {
					return err
				}
			}
		case ShapeTreeItemGraphicFrame:
			if item.Index >= 0 && item.Index < len(st.GraphicFrames) {
				if err := e.EncodeElement(st.GraphicFrames[item.Index], xml.StartElement{Name: xml.Name{Local: "graphicFrame"}}); err != nil {
					return err
				}
			}
		case ShapeTreeItemSmartArt:
			if item.Index >= 0 && item.Index < len(st.SmartArtFrames) {
				if err := marshalRawXMLElement(e, st.SmartArtFrames[item.Index].RawXML); err != nil {
					return err
				}
			}
		case ShapeTreeItemOleObject:
			if item.Index >= 0 && item.Index < len(st.OleObjectFrames) {
				frame := st.OleObjectFrames[item.Index]
				if len(frame.RawXML) > 0 {
					if err := marshalRawXMLElement(e, frame.RawXML); err != nil {
						return err
					}
				} else if err := e.EncodeElement(frame, xml.StartElement{Name: xml.Name{Local: "graphicFrame"}}); err != nil {
					return err
				}
			}
		}
	}
	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

// MarshalXML emits GroupShape as p:grpSp in child order preserved by Items.
func (g GroupShape) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	pNS := "http://schemas.openxmlformats.org/presentationml/2006/main"
	start.Name = xml.Name{Space: pNS, Local: "grpSp"}
	if err := e.EncodeToken(start); err != nil {
		return err
	}

	// nvGrpSpPr
	if g.NvGrpSpPr != nil {
		if err := e.EncodeElement(g.NvGrpSpPr, xml.StartElement{Name: xml.Name{Space: pNS, Local: "nvGrpSpPr"}}); err != nil {
			return err
		}
	}

	// grpSpPr
	if g.GrpSpPr != nil {
		if err := e.EncodeElement(g.GrpSpPr, xml.StartElement{Name: xml.Name{Space: pNS, Local: "grpSpPr"}}); err != nil {
			return err
		}
	}

	for _, item := range g.Items {
		switch item.Kind {
		case ShapeTreeItemShape:
			if item.Index >= 0 && item.Index < len(g.Shapes) {
				if err := e.EncodeElement(g.Shapes[item.Index], xml.StartElement{Name: xml.Name{Space: pNS, Local: "sp"}}); err != nil {
					return err
				}
			}
		case ShapeTreeItemPicture:
			if item.Index >= 0 && item.Index < len(g.Pictures) {
				if err := e.EncodeElement(g.Pictures[item.Index], xml.StartElement{Name: xml.Name{Space: pNS, Local: "pic"}}); err != nil {
					return err
				}
			}
		case ShapeTreeItemGroup:
			if item.Index >= 0 && item.Index < len(g.GroupShapes) {
				if err := e.EncodeElement(g.GroupShapes[item.Index], xml.StartElement{Name: xml.Name{Space: pNS, Local: "grpSp"}}); err != nil {
					return err
				}
			}
		case ShapeTreeItemConnector:
			if item.Index >= 0 && item.Index < len(g.Connectors) {
				if err := e.EncodeElement(g.Connectors[item.Index], xml.StartElement{Name: xml.Name{Space: pNS, Local: "cxnSp"}}); err != nil {
					return err
				}
			}
		case ShapeTreeItemTable:
			if item.Index >= 0 && item.Index < len(g.Tables) {
				if err := e.EncodeElement(g.Tables[item.Index], xml.StartElement{Name: xml.Name{Space: pNS, Local: "graphicFrame"}}); err != nil {
					return err
				}
			}
		case ShapeTreeItemSmartArt:
			if item.Index >= 0 && item.Index < len(g.SmartArtFrames) {
				if err := marshalRawXMLElement(e, g.SmartArtFrames[item.Index].RawXML); err != nil {
					return err
				}
			}
		case ShapeTreeItemOleObject:
			if item.Index >= 0 && item.Index < len(g.OleObjectFrames) {
				frame := g.OleObjectFrames[item.Index]
				if len(frame.RawXML) > 0 {
					if err := marshalRawXMLElement(e, frame.RawXML); err != nil {
						return err
					}
				} else if err := e.EncodeElement(frame, xml.StartElement{Name: xml.Name{Space: pNS, Local: "graphicFrame"}}); err != nil {
					return err
				}
			}
		}
	}
	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

// MarshalXML emits GraphicFrame as p:graphicFrame.
func (gf GraphicFrame) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	pNS := "http://schemas.openxmlformats.org/presentationml/2006/main"
	start.Name = xml.Name{Space: pNS, Local: "graphicFrame"}
	if err := e.EncodeToken(start); err != nil {
		return err
	}

	// nvGraphicFramePr
	nvStart := xml.StartElement{Name: xml.Name{Space: pNS, Local: "nvGraphicFramePr"}}
	if err := e.EncodeToken(nvStart); err != nil {
		return err
	}
	if err := e.EncodeElement(gf.CNvPr, xml.StartElement{Name: xml.Name{Space: pNS, Local: "cNvPr"}}); err != nil {
		return err
	}
	if err := e.EncodeElement(gf.CNvGraphicFramePr, xml.StartElement{Name: xml.Name{Space: pNS, Local: "cNvGraphicFramePr"}}); err != nil {
		return err
	}
	if err := e.EncodeElement(gf.NvPr, xml.StartElement{Name: xml.Name{Space: pNS, Local: "nvPr"}}); err != nil {
		return err
	}
	if err := e.EncodeToken(xml.EndElement{Name: nvStart.Name}); err != nil {
		return err
	}

	// xfrm
	if gf.Xfrm != nil {
		if err := e.EncodeElement(gf.Xfrm, xml.StartElement{Name: xml.Name{Space: pNS, Local: "xfrm"}}); err != nil {
			return err
		}
	}

	// graphic
	if err := e.EncodeElement(gf.Graphic, xml.StartElement{Name: xml.Name{Space: "http://schemas.openxmlformats.org/drawingml/2006/main", Local: "graphic"}}); err != nil {
		return err
	}

	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

// MarshalXML emits OleObjectFrame as p:graphicFrame.
func (of OleObjectFrame) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	pNS := "http://schemas.openxmlformats.org/presentationml/2006/main"
	aNS := "http://schemas.openxmlformats.org/drawingml/2006/main"
	rNS := "http://schemas.openxmlformats.org/officeDocument/2006/relationships"

	start.Name = xml.Name{Space: pNS, Local: "graphicFrame"}
	if err := e.EncodeToken(start); err != nil {
		return err
	}

	nvStart := xml.StartElement{Name: xml.Name{Space: pNS, Local: "nvGraphicFramePr"}}
	if err := e.EncodeToken(nvStart); err != nil {
		return err
	}
	cnvPr := CnvPr{ID: of.ID, Name: of.Name, Desc: of.Desc}
	if err := e.EncodeElement(cnvPr, xml.StartElement{Name: xml.Name{Space: pNS, Local: "cNvPr"}}); err != nil {
		return err
	}
	locks := CnvGraphicFramePr{GraphicFrameLocks: &GraphicFrameLocks{NoGrp: true}}
	if err := e.EncodeElement(locks, xml.StartElement{Name: xml.Name{Space: pNS, Local: "cNvGraphicFramePr"}}); err != nil {
		return err
	}
	if err := e.EncodeElement(ApplicationNonVisualDrawingProps{}, xml.StartElement{Name: xml.Name{Space: pNS, Local: "nvPr"}}); err != nil {
		return err
	}
	if err := e.EncodeToken(xml.EndElement{Name: nvStart.Name}); err != nil {
		return err
	}

	if of.Xfrm != nil {
		if err := e.EncodeElement(of.Xfrm, xml.StartElement{Name: xml.Name{Space: pNS, Local: "xfrm"}}); err != nil {
			return err
		}
	}

	graphicStart := xml.StartElement{Name: xml.Name{Space: aNS, Local: "graphic"}}
	if err := e.EncodeToken(graphicStart); err != nil {
		return err
	}
	gdStart := xml.StartElement{
		Name: xml.Name{Space: aNS, Local: "graphicData"},
		Attr: []xml.Attr{{Name: xml.Name{Local: "uri"}, Value: drawingGraphicDataURIOLETransitional}},
	}
	if err := e.EncodeToken(gdStart); err != nil {
		return err
	}

	objectName := of.ObjectName
	if objectName == "" {
		objectName = of.Name
	}
	oleStart := xml.StartElement{
		Name: xml.Name{Space: pNS, Local: "oleObj"},
		Attr: []xml.Attr{
			{Name: xml.Name{Local: "name"}, Value: objectName},
			{Name: xml.Name{Space: rNS, Local: "id"}, Value: of.RelID},
		},
	}
	if of.ShowAsIcon || of.IconRelID != "" {
		oleStart.Attr = append(oleStart.Attr, xml.Attr{Name: xml.Name{Local: "showAsIcon"}, Value: "1"})
	}
	if of.ProgID != "" {
		oleStart.Attr = append(oleStart.Attr, xml.Attr{Name: xml.Name{Local: "progId"}, Value: of.ProgID})
	}
	if of.ImgW > 0 {
		oleStart.Attr = append(oleStart.Attr, xml.Attr{Name: xml.Name{Local: "imgW"}, Value: strconv.FormatInt(of.ImgW, 10)})
	}
	if of.ImgH > 0 {
		oleStart.Attr = append(oleStart.Attr, xml.Attr{Name: xml.Name{Local: "imgH"}, Value: strconv.FormatInt(of.ImgH, 10)})
	}
	if err := e.EncodeToken(oleStart); err != nil {
		return err
	}

	linkLocal := "embed"
	if !of.Embed {
		linkLocal = "link"
	}
	linkStart := xml.StartElement{Name: xml.Name{Space: pNS, Local: linkLocal}}
	if err := e.EncodeToken(linkStart); err != nil {
		return err
	}
	if err := e.EncodeToken(xml.EndElement{Name: linkStart.Name}); err != nil {
		return err
	}

	if of.IconRelID != "" {
		picStart := xml.StartElement{Name: xml.Name{Space: pNS, Local: "pic"}}
		if err := e.EncodeToken(picStart); err != nil {
			return err
		}
		blipFillStart := xml.StartElement{Name: xml.Name{Space: pNS, Local: "blipFill"}}
		if err := e.EncodeToken(blipFillStart); err != nil {
			return err
		}
		blipStart := xml.StartElement{
			Name: xml.Name{Space: aNS, Local: "blip"},
			Attr: []xml.Attr{{Name: xml.Name{Space: rNS, Local: "embed"}, Value: of.IconRelID}},
		}
		if err := e.EncodeToken(blipStart); err != nil {
			return err
		}
		if err := e.EncodeToken(xml.EndElement{Name: blipStart.Name}); err != nil {
			return err
		}
		if err := e.EncodeToken(xml.EndElement{Name: blipFillStart.Name}); err != nil {
			return err
		}

		spPrStart := xml.StartElement{Name: xml.Name{Space: pNS, Local: "spPr"}}
		if err := e.EncodeToken(spPrStart); err != nil {
			return err
		}
		iconXfrm := &Transform2D{Ext: Extents{CX: of.ImgW, CY: of.ImgH}}
		if of.ImgW <= 0 || of.ImgH <= 0 {
			iconXfrm.Ext = Extents{CX: 914400, CY: 914400}
		}
		if err := e.EncodeElement(iconXfrm, xml.StartElement{Name: xml.Name{Space: aNS, Local: "xfrm"}}); err != nil {
			return err
		}
		if err := e.EncodeToken(xml.EndElement{Name: spPrStart.Name}); err != nil {
			return err
		}
		if err := e.EncodeToken(xml.EndElement{Name: picStart.Name}); err != nil {
			return err
		}
	}

	if err := e.EncodeToken(xml.EndElement{Name: oleStart.Name}); err != nil {
		return err
	}
	if err := e.EncodeToken(xml.EndElement{Name: gdStart.Name}); err != nil {
		return err
	}
	if err := e.EncodeToken(xml.EndElement{Name: graphicStart.Name}); err != nil {
		return err
	}
	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

// MarshalXML emits DMLParagraph children in the order preserved in Inlines.
func (p DMLParagraph) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	if p.Properties != nil {
		if err := e.EncodeElement(p.Properties, xml.StartElement{Name: xml.Name{Local: "pPr"}}); err != nil {
			return err
		}
	}
	for _, inline := range p.Inlines {
		switch inline.Kind {
		case DMLParaInlineRun:
			if inline.Run != nil {
				if err := e.EncodeElement(inline.Run, xml.StartElement{Name: xml.Name{Local: "r"}}); err != nil {
					return err
				}
			}
		case DMLParaInlineField:
			if inline.Field != nil {
				if err := e.EncodeElement(inline.Field, xml.StartElement{Name: xml.Name{Local: "fld"}}); err != nil {
					return err
				}
			}
		case DMLParaInlineBreak:
			if inline.Break != nil {
				if err := e.EncodeElement(inline.Break, xml.StartElement{Name: xml.Name{Local: "br"}}); err != nil {
					return err
				}
			}
		}
	}
	if p.EndParaRPr != nil {
		if err := e.EncodeElement(p.EndParaRPr, xml.StartElement{Name: xml.Name{Local: "endParaRPr"}}); err != nil {
			return err
		}
	}
	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

// emitRawXMLElement re-parses raw XML bytes and emits them token-by-token
// through the encoder, preserving the exact element structure.
func emitRawXMLElement(e *xml.Encoder, raw []byte) error {
	dec := xml.NewDecoder(bytes.NewReader(raw))
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	start, ok := tok.(xml.StartElement)
	if !ok {
		return nil
	}
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	depth := 1
	for depth > 0 {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		if err := e.EncodeToken(tok); err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
		case xml.EndElement:
			depth--
			if depth == 0 && t.Name == start.Name {
				return nil
			}
		}
	}
	return nil
}

// MarshalXML emits Slide with the correct element name and children.
func (s Slide) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	pNS := "http://schemas.openxmlformats.org/presentationml/2006/main"
	start.Name = xml.Name{Space: pNS, Local: "sld"}
	if s.Show != nil {
		start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "show"}, Value: boolXMLValue(*s.Show)})
	}
	if s.ShowMasterSp != nil {
		start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "showMasterSp"}, Value: boolXMLValue(*s.ShowMasterSp)})
	}
	if s.ShowMasterPhAnim != nil {
		start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "showMasterPhAnim"}, Value: boolXMLValue(*s.ShowMasterPhAnim)})
	}
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	if err := e.EncodeElement(s.CSld, xml.StartElement{Name: xml.Name{Space: pNS, Local: "cSld"}}); err != nil {
		return err
	}
	if s.ClrMapOvr != nil {
		if err := e.EncodeElement(s.ClrMapOvr, xml.StartElement{Name: xml.Name{Space: pNS, Local: "clrMapOvr"}}); err != nil {
			return err
		}
	}
	if s.Transition != nil {
		if err := e.EncodeElement(s.Transition, xml.StartElement{Name: xml.Name{Space: pNS, Local: "transition"}}); err != nil {
			return err
		}
	}
	if len(s.TimingRawXML) > 0 {
		if err := emitRawXMLElement(e, s.TimingRawXML); err != nil {
			return err
		}
	} else if slideHasAnimation(s) {
		if err := emitRawXMLElement(e, buildTimingFromSlide(s)); err != nil {
			return err
		}
	}
	if len(s.ExtLstRawXML) > 0 {
		if err := emitRawXMLElement(e, s.ExtLstRawXML); err != nil {
			return err
		}
	}
	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

func boolXMLValue(v bool) string {
	if v {
		return "1"
	}
	return "0"
}

func slideHasAnimation(s Slide) bool {
	return len(s.AnimSteps) > 0 || len(s.EmphasisEffects) > 0 ||
		len(s.MotionPaths) > 0 || len(s.AnimationTriggers) > 0
}

func buildTimingFromSlide(s Slide) []byte {
	ids := map[int]struct{}{}
	for id := range s.AnimSteps {
		ids[id] = struct{}{}
	}
	for id := range s.EmphasisEffects {
		ids[id] = struct{}{}
	}
	for id := range s.MotionPaths {
		ids[id] = struct{}{}
	}
	for id := range s.AnimationTriggers {
		ids[id] = struct{}{}
	}
	var shapeIDs []int
	for id := range ids {
		shapeIDs = append(shapeIDs, id)
	}
	sort.Ints(shapeIDs)

	var buf bytes.Buffer
	buf.WriteString(`<p:timing xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:tnLst><p:par>`)
	buf.WriteString(`<p:cTn id="1" dur="indefinite" restart="never" nodeType="tmRoot"><p:childTnLst>`)
	buf.WriteString(`<p:seq concurrent="1" nextAc="seek"><p:cTn id="2" dur="indefinite" nodeType="mainSeq"><p:childTnLst>`)

	tnID := 3

	// Entrance click-step animations, grouped by step number.
	stepGroups := make(map[int][]int)
	for _, id := range shapeIDs {
		if step, ok := s.AnimSteps[id]; ok {
			stepGroups[step] = append(stepGroups[step], id)
		}
	}
	for _, step := range sortedInts(stepGroups) {
		ids := stepGroups[step]
		buf.WriteString(`<p:par><p:cTn id="`)
		buf.WriteString(strconv.Itoa(tnID))
		tnID++
		buf.WriteString(`" fill="hold"><p:stCondLst><p:cond delay="indefinite"/></p:stCondLst><p:childTnLst>`)
		grpID := step - 1
		for _, id := range ids {
			tnID = writeClickEffect(&buf, tnID, id, grpID, false, "")
		}
		buf.WriteString(`</p:childTnLst></p:cTn></p:par>`)
	}

	// Emphasis effects.
	for _, id := range shapeIDs {
		for _, effect := range s.EmphasisEffects[id] {
			tnID = writeEmphasisEffect(&buf, tnID, id, effect)
		}
	}

	// Motion paths.
	for _, id := range shapeIDs {
		if mp, ok := s.MotionPaths[id]; ok {
			tnID = writeMotionPath(&buf, tnID, id, mp)
		}
	}

	// Triggered animations.
	for _, id := range shapeIDs {
		for _, event := range s.AnimationTriggers[id] {
			tnID = writeClickEffect(&buf, tnID, id, 0, true, event)
		}
	}

	buf.WriteString(`</p:childTnLst></p:cTn></p:seq></p:childTnLst></p:cTn></p:par></p:tnLst></p:timing>`)
	return buf.Bytes()
}

func sortedInts(m map[int][]int) []int {
	var keys []int
	for k := range m {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	return keys
}

// writeClickEffect emits a p:par animation node. When isTrigger is true it
// carries a p:cond@evt wrapped around the target shape instead of the normal
// click-step start condition.
func writeClickEffect(buf *bytes.Buffer, tnID, shapeID, grpID int, isTrigger bool, event string) int {
	buf.WriteString(`<p:par><p:cTn id="`)
	buf.WriteString(strconv.Itoa(tnID))
	tnID++
	buf.WriteString(`" fill="hold" nodeType="clickEffect"`)
	if !isTrigger {
		buf.WriteString(` grpId="`)
		buf.WriteString(strconv.Itoa(grpID))
		buf.WriteString(`"`)
	}
	buf.WriteString(`><p:stCondLst>`)
	if isTrigger {
		buf.WriteString(`<p:cond evt="`)
		xml.EscapeText(buf, []byte(event))
		buf.WriteString(`" delay="0"><p:tgtEl><p:spTgt spid="`)
		buf.WriteString(strconv.Itoa(shapeID))
		buf.WriteString(`"/></p:tgtEl></p:cond>`)
	} else {
		buf.WriteString(`<p:cond delay="0"/>`)
	}
	buf.WriteString(`</p:stCondLst><p:childTnLst><p:set><p:cBhvr><p:cTn id="`)
	buf.WriteString(strconv.Itoa(tnID))
	tnID++
	buf.WriteString(`" dur="1" fill="hold"/><p:tgtEl><p:spTgt spid="`)
	buf.WriteString(strconv.Itoa(shapeID))
	buf.WriteString(`"/></p:tgtEl><p:attrNameLst><p:attrName>style.visibility</p:attrName></p:attrNameLst></p:cBhvr><p:to><p:strVal val="visible"/></p:to></p:set></p:childTnLst></p:cTn></p:par>`)
	return tnID
}

func writeEmphasisEffect(buf *bytes.Buffer, tnID, shapeID int, effect string) int {
	var elem, attrName, extra string
	switch effect {
	case "spin":
		elem = "animRot"
		attrName = "style.rotation"
		extra = ` by="360000" dur="1000"`
	case "growShrink":
		elem = "animScale"
		attrName = "style.fontSize"
		extra = ` by="125000" dur="500"`
	default:
		elem = "anim"
		attrName = "style.visibility"
		extra = ` dur="500" calcmode="lin" valueType="num"`
	}
	buf.WriteString(`<p:par><p:cTn id="`)
	buf.WriteString(strconv.Itoa(tnID))
	tnID++
	buf.WriteString(`" fill="hold" nodeType="emphEffect"><p:stCondLst><p:cond delay="0"/></p:stCondLst><p:childTnLst><p:`)
	buf.WriteString(elem)
	buf.WriteString(extra)
	buf.WriteString(`><p:cBhvr><p:cTn id="`)
	buf.WriteString(strconv.Itoa(tnID))
	tnID++
	buf.WriteString(`" dur="1" fill="hold"/><p:tgtEl><p:spTgt spid="`)
	buf.WriteString(strconv.Itoa(shapeID))
	buf.WriteString(`"/></p:tgtEl><p:attrNameLst><p:attrName>`)
	buf.WriteString(attrName)
	buf.WriteString(`</p:attrName></p:attrNameLst></p:cBhvr></p:`)
	buf.WriteString(elem)
	buf.WriteString(`></p:childTnLst></p:cTn></p:par>`)
	return tnID
}

func writeMotionPath(buf *bytes.Buffer, tnID, shapeID int, mp AnimMotionPath) int {
	dur := mp.Dur
	if dur <= 0 {
		dur = 500
	}
	origin := mp.Origin
	if origin == "" {
		origin = "layout"
	}
	buf.WriteString(`<p:par><p:cTn id="`)
	buf.WriteString(strconv.Itoa(tnID))
	tnID++
	buf.WriteString(`" fill="hold" nodeType="emphEffect"><p:stCondLst><p:cond delay="0"/></p:stCondLst><p:childTnLst><p:animMotion path="`)
	xml.EscapeText(buf, []byte(mp.Path))
	buf.WriteString(`" dur="`)
	buf.WriteString(strconv.Itoa(dur))
	buf.WriteString(`" origin="`)
	xml.EscapeText(buf, []byte(origin))
	buf.WriteString(`"><p:cBhvr><p:cTn id="`)
	buf.WriteString(strconv.Itoa(tnID))
	tnID++
	buf.WriteString(`" dur="`)
	buf.WriteString(strconv.Itoa(dur))
	buf.WriteString(`" fill="hold"/><p:tgtEl><p:spTgt spid="`)
	buf.WriteString(strconv.Itoa(shapeID))
	buf.WriteString(`"/></p:tgtEl><p:attrNameLst><p:attrName>ppt_x</p:attrName></p:attrNameLst></p:cBhvr></p:animMotion></p:childTnLst></p:cTn></p:par>`)
	return tnID
}

// MarshalXML emits TableFrame as p:graphicFrame with a nested table.
func (tf TableFrame) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	pNS := "http://schemas.openxmlformats.org/presentationml/2006/main"
	aNS := "http://schemas.openxmlformats.org/drawingml/2006/main"
	tblURI := "http://schemas.openxmlformats.org/drawingml/2006/table"

	start.Name = xml.Name{Space: pNS, Local: "graphicFrame"}
	if err := e.EncodeToken(start); err != nil {
		return err
	}

	// nvGraphicFramePr
	nvStart := xml.StartElement{Name: xml.Name{Space: pNS, Local: "nvGraphicFramePr"}}
	if err := e.EncodeToken(nvStart); err != nil {
		return err
	}
	cnvPr := CnvPr{ID: tf.ID, Name: tf.Name, Desc: tf.Desc}
	if err := e.EncodeElement(cnvPr, xml.StartElement{Name: xml.Name{Space: pNS, Local: "cNvPr"}}); err != nil {
		return err
	}
	if err := e.EncodeElement(CnvGraphicFramePr{}, xml.StartElement{Name: xml.Name{Space: pNS, Local: "cNvGraphicFramePr"}}); err != nil {
		return err
	}
	if err := e.EncodeElement(ApplicationNonVisualDrawingProps{}, xml.StartElement{Name: xml.Name{Space: pNS, Local: "nvPr"}}); err != nil {
		return err
	}
	if err := e.EncodeToken(xml.EndElement{Name: nvStart.Name}); err != nil {
		return err
	}

	// xfrm
	if tf.Xfrm != nil {
		if err := e.EncodeElement(tf.Xfrm, xml.StartElement{Name: xml.Name{Space: pNS, Local: "xfrm"}}); err != nil {
			return err
		}
	}

	// graphic
	graphicStart := xml.StartElement{Name: xml.Name{Space: aNS, Local: "graphic"}}
	if err := e.EncodeToken(graphicStart); err != nil {
		return err
	}

	// graphicData
	gdStart := xml.StartElement{Name: xml.Name{Space: aNS, Local: "graphicData"}}
	gdStart.Attr = []xml.Attr{{Name: xml.Name{Local: "uri"}, Value: tblURI}}
	if err := e.EncodeToken(gdStart); err != nil {
		return err
	}

	// table
	if tf.Table != nil {
		if err := e.EncodeElement(*tf.Table, xml.StartElement{Name: xml.Name{Space: aNS, Local: "tbl"}}); err != nil {
			return err
		}
	}

	if err := e.EncodeToken(xml.EndElement{Name: gdStart.Name}); err != nil {
		return err
	}
	if err := e.EncodeToken(xml.EndElement{Name: graphicStart.Name}); err != nil {
		return err
	}

	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

// marshalRawXMLElement decodes a standalone XML element and re-emits its tokens
// so that it can be inserted into an already open encoder stream. This is used to
// preserve exact SmartArt graphicFrame markup on write-back.
func marshalRawXMLElement(e *xml.Encoder, raw []byte) error {
	dec := xml.NewDecoder(bytes.NewReader(raw))
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if err := e.EncodeToken(tok); err != nil {
			return err
		}
	}
	return e.Flush()
}

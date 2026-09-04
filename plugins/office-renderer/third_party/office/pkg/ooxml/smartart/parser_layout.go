package smartart

import (
	"encoding/xml"
	"math"
	"strconv"
	"strings"
)

// parseSmartArtLayout parses a dgm:layoutDef XML part into the shared Layout
// metadata and, when possible, a full LayoutDefinition AST. Missing or
// malformed XML is silently ignored so hardcoded defaults remain intact.
func parseSmartArtLayout(layoutXML []byte, layout *Layout) {
	if layout == nil || len(layoutXML) == 0 {
		return
	}

	layout.Type = DetectDiagramType(layoutXML)

	var doc rawLayoutDef
	if err := xml.Unmarshal(layoutXML, &doc); err != nil {
		return
	}

	if doc.Dir != "" {
		switch strings.ToLower(doc.Dir) {
		case "lrtb", "lr", "l":
			layout.Direction = "leftRight"
		case "tblr", "tb", "t":
			layout.Direction = "topDown"
		case "rltb", "rl", "r":
			layout.Direction = "rightLeft"
		case "btlr", "bt", "b":
			layout.Direction = "bottomUp"
		}
	}

	if doc.Root.Name != "" {
		layout.Definition = &LayoutDefinition{
			UniqueID: doc.UniqueID,
			Root:     convertRawLayoutNode(&doc.Root),
		}
	}
}

type rawLayoutDef struct {
	XMLName  xml.Name      `xml:"layoutDef"`
	UniqueID string        `xml:"uniqueId,attr"`
	Dir      string        `xml:"dir,attr,omitempty"`
	Root     rawLayoutNode `xml:"layoutNode"`
	VarLst   []rawVariable `xml:"varLst>..."`
}

type rawLayoutNode struct {
	Name        string          `xml:"name,attr"`
	StyleLbl    string          `xml:"styleLbl,attr"`
	Algorithm   *rawAlgorithm   `xml:"alg"`
	Shape       *rawShape       `xml:"shape"`
	PresOf      *rawPresOf      `xml:"presOf"`
	Constraints []rawConstraint `xml:"constrLst>constr"`
	Rules       []rawRule       `xml:"ruleLst>rule"`
	VarLst      rawVarList      `xml:"varLst"`
	Children    []rawLayoutNode `xml:"layoutNode"`
	ForEach     []rawForEach    `xml:"forEach"`
	Choose      []rawChoose     `xml:"choose"`
}

type rawAlgorithm struct {
	Type   string     `xml:"type,attr"`
	Params []rawParam `xml:"param"`
}

type rawParam struct {
	Type string `xml:"type,attr"`
	Val  string `xml:"val,attr"`
}

type rawShape struct {
	Type string `xml:"type,attr"`
	Rot  string `xml:"rot,attr"`
	Blip string `xml:"r:blip,attr"`
}

type rawPresOf struct {
	Axis   string `xml:"axis,attr"`
	PtType string `xml:"ptType,attr"`
	Cnt    string `xml:"cnt,attr"`
	Start  string `xml:"st,attr"`
	Step   string `xml:"step,attr"`
}

type rawConstraint struct {
	Type       string `xml:"type,attr"`
	For        string `xml:"for,attr"`
	ForName    string `xml:"forName,attr"`
	RefType    string `xml:"refType,attr"`
	RefFor     string `xml:"refFor,attr"`
	RefForName string `xml:"refForName,attr"`
	Op         string `xml:"op,attr"`
	Val        string `xml:"val,attr"`
	Fact       string `xml:"fact,attr"`
}

type rawRule struct {
	Type    string `xml:"type,attr"`
	For     string `xml:"for,attr"`
	ForName string `xml:"forName,attr"`
	PtType  string `xml:"ptType,attr"`
	Val     string `xml:"val,attr"`
	Fact    string `xml:"fact,attr"`
	Max     string `xml:"max,attr"`
}

type rawForEach struct {
	Name          string          `xml:"name,attr"`
	Axis          string          `xml:"axis,attr"`
	PtType        string          `xml:"ptType,attr"`
	Cnt           string          `xml:"cnt,attr"`
	Start         string          `xml:"st,attr"`
	Step          string          `xml:"step,attr"`
	HideLastTrans string          `xml:"hideLastTrans,attr"`
	Ref           string          `xml:"ref,attr"`
	Children      []rawLayoutNode `xml:"layoutNode"`
}

type rawChoose struct {
	Name    string          `xml:"name,attr"`
	IfCases []rawChooseCase `xml:"if"`
	Else    *rawChooseCase  `xml:"else"`
}

type rawChooseCase struct {
	Name        string          `xml:"name,attr"`
	Func        string          `xml:"func,attr"`
	Arg         string          `xml:"arg,attr"`
	Op          string          `xml:"op,attr"`
	Val         string          `xml:"val,attr"`
	Axis        string          `xml:"axis,attr"`
	PtType      string          `xml:"ptType,attr"`
	Algorithm   *rawAlgorithm   `xml:"alg"`
	Constraints []rawConstraint `xml:"constrLst>constr"`
	Children    []rawLayoutNode `xml:"layoutNode"`
}

type rawVarList struct {
	Items []rawVariable `xml:",any"`
}

type rawVariable struct {
	XMLName xml.Name
	Val     string `xml:"val,attr"`
}

func convertRawLayoutNode(raw *rawLayoutNode) *LayoutNode {
	if raw == nil {
		return nil
	}
	ln := &LayoutNode{
		Name:     raw.Name,
		StyleLbl: raw.StyleLbl,
	}
	if raw.Algorithm != nil {
		ln.Algorithm = &Algorithm{
			Type:   raw.Algorithm.Type,
			Params: make(map[string]string),
		}
		for _, p := range raw.Algorithm.Params {
			if p.Type != "" {
				ln.Algorithm.Params[p.Type] = p.Val
			}
		}
	}
	if raw.Shape != nil {
		ln.Shape = &LayoutShape{
			Type: raw.Shape.Type,
			Blip: raw.Shape.Blip,
		}
		if rot, err := strconv.ParseFloat(raw.Shape.Rot, 64); err == nil {
			ln.Shape.Rot = rot
		}
	}
	if raw.PresOf != nil {
		ln.PresOf = &PresentationOf{
			Axis:   raw.PresOf.Axis,
			PtType: raw.PresOf.PtType,
		}
		if v, err := strconv.Atoi(strings.TrimSpace(raw.PresOf.Cnt)); err == nil {
			ln.PresOf.Cnt = v
		}
		if v, err := strconv.Atoi(strings.TrimSpace(raw.PresOf.Start)); err == nil {
			ln.PresOf.Start = v
		}
		if v, err := strconv.Atoi(strings.TrimSpace(raw.PresOf.Step)); err == nil {
			ln.PresOf.Step = v
		}
	}
	for _, c := range raw.Constraints {
		ln.Constraints = append(ln.Constraints, convertRawConstraint(c))
	}
	for _, r := range raw.Rules {
		ln.Rules = append(ln.Rules, convertRawRule(r))
	}
	ln.Variables = make(map[string]string)
	for _, v := range raw.VarLst.Items {
		name := strings.ToLower(v.XMLName.Local)
		if name == "" {
			continue
		}
		if v.Val != "" {
			ln.Variables[name] = v.Val
		} else {
			ln.Variables[name] = "1"
		}
	}
	for _, child := range raw.Children {
		ln.Children = append(ln.Children, convertRawLayoutNode(&child))
	}
	if len(raw.ForEach) > 0 {
		ln.ForEach = convertRawForEach(&raw.ForEach[0])
	}
	if len(raw.Choose) > 0 {
		ln.Choose = convertRawChoose(&raw.Choose[0])
	}
	return ln
}

func convertRawForEach(raw *rawForEach) *ForEachSpec {
	if raw == nil {
		return nil
	}
	fe := &ForEachSpec{
		Name:   raw.Name,
		Axis:   raw.Axis,
		PtType: raw.PtType,
		Ref:    raw.Ref,
	}
	if st, err := strconv.Atoi(raw.Start); err == nil {
		fe.Start = st
	}
	if step, err := strconv.Atoi(raw.Step); err == nil {
		fe.Step = step
	}
	switch strings.ToLower(strings.TrimSpace(raw.HideLastTrans)) {
	case "1", "true", "t", "yes":
		fe.HideLastTrans = true
	}
	if cnt, err := strconv.Atoi(raw.Cnt); err == nil {
		fe.Cnt = cnt
	}
	for _, child := range raw.Children {
		fe.Children = append(fe.Children, convertRawLayoutNode(&child))
	}
	return fe
}

func convertRawChoose(raw *rawChoose) *ChooseSpec {
	if raw == nil {
		return nil
	}
	ch := &ChooseSpec{Name: raw.Name}
	for _, c := range raw.IfCases {
		ch.Cases = append(ch.Cases, convertRawChooseCase(&c))
	}
	if raw.Else != nil {
		ch.Default = convertRawChooseCase(raw.Else)
	}
	return ch
}

func convertRawChooseCase(raw *rawChooseCase) *ChooseCase {
	cc := &ChooseCase{
		Name:   raw.Name,
		Func:   raw.Func,
		Arg:    raw.Arg,
		Op:     raw.Op,
		Val:    raw.Val,
		Axis:   raw.Axis,
		PtType: raw.PtType,
	}
	if raw.Algorithm != nil {
		cc.Algorithm = &Algorithm{
			Type:   raw.Algorithm.Type,
			Params: make(map[string]string),
		}
		for _, p := range raw.Algorithm.Params {
			if p.Type != "" {
				cc.Algorithm.Params[p.Type] = p.Val
			}
		}
	}
	for _, c := range raw.Constraints {
		cc.Constraints = append(cc.Constraints, convertRawConstraint(c))
	}
	for _, child := range raw.Children {
		cc.Children = append(cc.Children, convertRawLayoutNode(&child))
	}
	return cc
}

func convertRawConstraint(raw rawConstraint) Constraint {
	c := Constraint{
		Type:       raw.Type,
		For:        raw.For,
		ForName:    raw.ForName,
		RefType:    raw.RefType,
		RefFor:     raw.RefFor,
		RefForName: raw.RefForName,
		Op:         raw.Op,
	}
	if v, err := strconv.ParseFloat(raw.Val, 64); err == nil {
		c.Val = v
	} else {
		c.Val = math.NaN()
	}
	if f, err := strconv.ParseFloat(raw.Fact, 64); err == nil {
		c.Fact = f
	} else {
		c.Fact = math.NaN()
	}
	return c
}

func convertRawRule(raw rawRule) Rule {
	r := Rule{Type: raw.Type, For: raw.For, ForName: raw.ForName, PtType: raw.PtType}
	if v, err := strconv.ParseFloat(raw.Val, 64); err == nil {
		r.Val = v
	} else {
		r.Val = math.NaN()
	}
	if f, err := strconv.ParseFloat(raw.Fact, 64); err == nil {
		r.Fact = f
	} else {
		r.Fact = math.NaN()
	}
	if m, err := strconv.ParseFloat(raw.Max, 64); err == nil {
		r.Max = m
	} else {
		r.Max = math.NaN()
	}
	return r
}

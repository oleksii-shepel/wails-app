// Package mathml converts OMML (Office Math Markup Language) to MathML.
// OMML is the XML math format used inside OOXML documents (DOCX).
// MathML is the W3C standard supported natively by modern browsers.
package mathml

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"html"
	"strconv"
	"strings"
	"unicode"
)

// Convert converts the bytes of a single m:oMath or m:oMathPara element to a
// MathML string wrapped in <math xmlns="http://www.w3.org/1998/Math/MathML">.
// Returns an empty string on parse error or empty input.
func Convert(xmlBytes []byte) string {
	if len(xmlBytes) == 0 {
		return ""
	}
	root, err := parseRoot(xmlBytes)
	if err != nil || root == nil {
		return ""
	}
	display := "inline"
	justify := ""
	if root.local == "oMathPara" {
		display = "block"
		if paraPr := firstChild(*root, "oMathParaPr"); paraPr.local != "" {
			if jcEl := firstChild(paraPr, "jc"); jcEl.local != "" {
				justify = jcEl.attrs["val"]
			}
		}
	}
	var sb strings.Builder
	if justify != "" && justify != "center" {
		switch justify {
		case "left", "right":
			sb.WriteString(fmt.Sprintf(`<div style="text-align:%s">`, justify))
		default:
			sb.WriteString(`<div style="text-align:center">`)
		}
	}
	sb.WriteString(`<math display="`)
	sb.WriteString(display)
	sb.WriteString(`" xmlns="http://www.w3.org/1998/Math/MathML">`)
	if root.local == "oMathPara" && hasAlignmentPoint(*root) {
		renderAlignedPara(&sb, *root)
	} else {
		for _, child := range root.children {
			renderNode(&sb, child)
		}
	}
	sb.WriteString("</math>")
	if justify != "" && justify != "center" {
		sb.WriteString("</div>")
	}
	return sb.String()
}

// node is an internal OMML element tree node.
type node struct {
	local    string
	attrs    map[string]string
	children []node
	text     string // accumulated CharData
}

func parseRoot(xmlBytes []byte) (*node, error) {
	dec := xml.NewDecoder(bytes.NewReader(xmlBytes))
	for {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		if se, ok := tok.(xml.StartElement); ok {
			n, err := parseElement(dec, se)
			if err != nil {
				return nil, err
			}
			return &n, nil
		}
	}
}

func parseElement(dec *xml.Decoder, start xml.StartElement) (node, error) {
	n := node{
		local: start.Name.Local,
		attrs: make(map[string]string, len(start.Attr)),
	}
	for _, attr := range start.Attr {
		n.attrs[attr.Name.Local] = attr.Value
	}
	for {
		tok, err := dec.Token()
		if err != nil {
			return n, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			child, err := parseElement(dec, t)
			if err != nil {
				return n, err
			}
			n.children = append(n.children, child)
		case xml.CharData:
			n.text += string(t)
		case xml.EndElement:
			return n, nil
		}
	}
}

// hasAlignmentPoint reports whether n or any descendant contains an m:r with
// m:rPr/m:aln, indicating an alignment-marked equation group.
func hasAlignmentPoint(n node) bool {
	if n.local == "r" || n.local == "run" {
		rPr := firstChild(n, "rPr")
		if rPr.local != "" && firstChild(rPr, "aln").local != "" {
			return true
		}
	}
	for _, child := range n.children {
		if hasAlignmentPoint(child) {
			return true
		}
	}
	return false
}

// renderAlignedPara renders an m:oMathPara that contains alignment points as a
// MathML <mtable>. Each m:oMath child becomes a row, split into cells at runs
// that carry m:rPr/m:aln.
func renderAlignedPara(sb *strings.Builder, para node) {
	sb.WriteString(`<mtable>`)
	for _, child := range para.children {
		if child.local != "oMath" {
			continue
		}
		sb.WriteString("<mtr>")
		// Group children into cells split at alignment-marked runs.
		var cellNodes []node
		for _, c := range child.children {
			if isAlignmentRun(c) {
				// Flush current cell before starting new one.
				if len(cellNodes) > 0 {
					sb.WriteString("<mtd>")
					for _, cn := range cellNodes {
						renderNode(sb, cn)
					}
					sb.WriteString("</mtd>")
					cellNodes = cellNodes[:0]
				}
			}
			cellNodes = append(cellNodes, c)
		}
		// Flush final cell.
		if len(cellNodes) > 0 {
			sb.WriteString("<mtd>")
			for _, cn := range cellNodes {
				renderNode(sb, cn)
			}
			sb.WriteString("</mtd>")
		}
		sb.WriteString("</mtr>")
	}
	sb.WriteString("</mtable>")
}

// isAlignmentRun reports whether n is an m:r whose m:rPr contains m:aln.
func isAlignmentRun(n node) bool {
	if n.local != "r" && n.local != "run" {
		return false
	}
	rPr := firstChild(n, "rPr")
	return rPr.local != "" && firstChild(rPr, "aln").local != ""
}

// renderNode converts a single OMML node to MathML text.
func renderNode(sb *strings.Builder, n node) {
	switch n.local {
	case "oMath":
		// Nested oMath (child of oMathPara) — just recurse.
		renderChildren(sb, n)

	case "r", "run":
		// m:r — math run containing m:rPr and m:t.
		renderMathRun(sb, n)

	case "f":
		// m:f — fraction: m:num / m:den.
		num := firstChild(n, "num")
		den := firstChild(n, "den")
		fPr := firstChild(n, "fPr")
		fracType := ""
		if fPr.local != "" {
			if typeEl := firstChild(fPr, "type"); typeEl.local != "" {
				fracType = typeEl.attrs["val"]
			}
		}
		switch fracType {
		case "noBar":
			sb.WriteString(`<mfrac linethickness="0">`)
		case "lin", "skw":
			sb.WriteString(`<mfrac bevelled="true">`)
		default:
			sb.WriteString("<mfrac>")
		}
		sb.WriteString("<mrow>")
		renderChildren(sb, num)
		sb.WriteString("</mrow>")
		sb.WriteString("<mrow>")
		renderChildren(sb, den)
		sb.WriteString("</mrow>")
		sb.WriteString("</mfrac>")

	case "rad":
		// m:rad — radical (sqrt or nth root).
		radPr := firstChild(n, "radPr")
		degHide := onOffValue(firstChild(radPr, "degHide"), false)
		deg := firstChild(n, "deg")
		e := firstChild(n, "e")
		if degHide || !hasContent(deg) {
			sb.WriteString("<msqrt>")
			renderChildren(sb, e)
			sb.WriteString("</msqrt>")
		} else {
			sb.WriteString("<mroot>")
			sb.WriteString("<mrow>")
			renderChildren(sb, e)
			sb.WriteString("</mrow>")
			sb.WriteString("<mrow>")
			renderChildren(sb, deg)
			sb.WriteString("</mrow>")
			sb.WriteString("</mroot>")
		}

	case "sSup":
		// m:sSup — superscript.
		e := firstChild(n, "e")
		sup := firstChild(n, "sup")
		sb.WriteString("<msup>")
		sb.WriteString("<mrow>")
		renderChildren(sb, e)
		sb.WriteString("</mrow>")
		sb.WriteString("<mrow>")
		renderChildren(sb, sup)
		sb.WriteString("</mrow>")
		sb.WriteString("</msup>")

	case "sSub":
		// m:sSub — subscript.
		e := firstChild(n, "e")
		sub := firstChild(n, "sub")
		sb.WriteString("<msub>")
		sb.WriteString("<mrow>")
		renderChildren(sb, e)
		sb.WriteString("</mrow>")
		sb.WriteString("<mrow>")
		renderChildren(sb, sub)
		sb.WriteString("</mrow>")
		sb.WriteString("</msub>")

	case "sSubSup":
		// m:sSubSup — subscript + superscript.
		e := firstChild(n, "e")
		sub := firstChild(n, "sub")
		sup := firstChild(n, "sup")
		sb.WriteString("<msubsup>")
		sb.WriteString("<mrow>")
		renderChildren(sb, e)
		sb.WriteString("</mrow>")
		sb.WriteString("<mrow>")
		renderChildren(sb, sub)
		sb.WriteString("</mrow>")
		sb.WriteString("<mrow>")
		renderChildren(sb, sup)
		sb.WriteString("</mrow>")
		sb.WriteString("</msubsup>")

	case "sPre":
		// m:sPre — pre-sub-superscript (e.g. tensor notation, combinations).
		e := firstChild(n, "e")
		sub := firstChild(n, "sub")
		sup := firstChild(n, "sup")
		sb.WriteString("<mmultiscripts>")
		sb.WriteString("<mrow>")
		renderChildren(sb, e)
		sb.WriteString("</mrow>")
		sb.WriteString("<mprescripts/>")
		sb.WriteString("<mrow>")
		renderChildren(sb, sub)
		sb.WriteString("</mrow>")
		sb.WriteString("<mrow>")
		renderChildren(sb, sup)
		sb.WriteString("</mrow>")
		sb.WriteString("</mmultiscripts>")

	case "d":
		// m:d — delimiter (parentheses, brackets, braces, etc.).
		dPr := firstChild(n, "dPr")
		open := "("
		close_ := ")"
		sep := "|"
		if dPr.local != "" {
			if bc := firstChild(dPr, "begChr"); bc.local != "" {
				if v, ok := bc.attrs["val"]; ok {
					open = v
				}
			}
			if ec := firstChild(dPr, "endChr"); ec.local != "" {
				if v, ok := ec.attrs["val"]; ok {
					close_ = v
				}
			}
			if sc := firstChild(dPr, "sepChr"); sc.local != "" {
				if v, ok := sc.attrs["val"]; ok {
					sep = v
				}
			}
		}
		sb.WriteString("<mrow>")
		if open != "" {
			sb.WriteString(`<mo stretchy="true">`)
			sb.WriteString(html.EscapeString(open))
			sb.WriteString("</mo>")
		}
		first := true
		for _, child := range n.children {
			if child.local == "dPr" {
				continue
			}
			if child.local == "e" {
				if !first && sep != "" {
					sb.WriteString("<mo>")
					sb.WriteString(html.EscapeString(sep))
					sb.WriteString("</mo>")
				}
				first = false
				renderChildren(sb, child)
			}
		}
		if close_ != "" {
			sb.WriteString(`<mo stretchy="true">`)
			sb.WriteString(html.EscapeString(close_))
			sb.WriteString("</mo>")
		}
		sb.WriteString("</mrow>")

	case "nary":
		// m:nary — n-ary operator (∑, ∏, ∫, etc.).
		naryPr := firstChild(n, "naryPr")
		chr := "∑"
		limLoc := "undOvr"
		subHide, supHide := false, false
		if naryPr.local != "" {
			if chrEl := firstChild(naryPr, "chr"); chrEl.local != "" {
				if v, ok := chrEl.attrs["val"]; ok && v != "" {
					chr = v
				}
			}
			if llEl := firstChild(naryPr, "limLoc"); llEl.local != "" {
				if v, ok := llEl.attrs["val"]; ok {
					limLoc = v
				}
			}
			subHide = onOffValue(firstChild(naryPr, "subHide"), false)
			supHide = onOffValue(firstChild(naryPr, "supHide"), false)
		}
		sub := firstChild(n, "sub")
		sup := firstChild(n, "sup")
		e := firstChild(n, "e")

		// Choose munder/mover/munderover vs msub/msup/msubsup based on limLoc.
		var tag string
		if limLoc == "subSup" {
			switch {
			case subHide && supHide:
				tag = ""
			case subHide:
				tag = "msup"
			case supHide:
				tag = "msub"
			default:
				tag = "msubsup"
			}
		} else {
			switch {
			case subHide && supHide:
				tag = ""
			case subHide:
				tag = "mover"
			case supHide:
				tag = "munder"
			default:
				tag = "munderover"
			}
		}

		if tag == "" {
			sb.WriteString("<mo>")
			sb.WriteString(html.EscapeString(chr))
			sb.WriteString("</mo>")
		} else {
			sb.WriteString("<")
			sb.WriteString(tag)
			sb.WriteString(">")
			sb.WriteString("<mo>")
			sb.WriteString(html.EscapeString(chr))
			sb.WriteString("</mo>")
			if !subHide && sub.local != "" {
				sb.WriteString("<mrow>")
				renderChildren(sb, sub)
				sb.WriteString("</mrow>")
			}
			if !supHide && sup.local != "" {
				sb.WriteString("<mrow>")
				renderChildren(sb, sup)
				sb.WriteString("</mrow>")
			}
			sb.WriteString("</")
			sb.WriteString(tag)
			sb.WriteString(">")
		}
		if e.local != "" {
			renderChildren(sb, e)
		}

	case "func":
		// m:func — function application (sin, cos, lim, etc.).
		fName := firstChild(n, "fName")
		e := firstChild(n, "e")
		sb.WriteString("<mrow>")
		renderChildren(sb, fName)
		sb.WriteString("<mo>&#x2061;</mo>") // invisible function application U+2061
		renderChildren(sb, e)
		sb.WriteString("</mrow>")

	case "acc":
		// m:acc — accent (hat, tilde, dot, etc.).
		accPr := firstChild(n, "accPr")
		chr := "̂" // default: combining circumflex U+0302
		if accPr.local != "" {
			if chrEl := firstChild(accPr, "chr"); chrEl.local != "" {
				if v, ok := chrEl.attrs["val"]; ok && v != "" {
					chr = v
				}
			}
		}
		e := firstChild(n, "e")
		sb.WriteString("<mover>")
		sb.WriteString("<mrow>")
		renderChildren(sb, e)
		sb.WriteString("</mrow>")
		sb.WriteString("<mo>")
		sb.WriteString(html.EscapeString(chr))
		sb.WriteString("</mo>")
		sb.WriteString("</mover>")

	case "bar":
		// m:bar — overbar or underbar.
		barPr := firstChild(n, "barPr")
		pos := "top"
		if barPr.local != "" {
			if posEl := firstChild(barPr, "pos"); posEl.local != "" {
				if v, ok := posEl.attrs["val"]; ok {
					pos = v
				}
			}
		}
		e := firstChild(n, "e")
		if pos == "bot" {
			sb.WriteString("<munder>")
			sb.WriteString("<mrow>")
			renderChildren(sb, e)
			sb.WriteString("</mrow>")
			sb.WriteString(`<mo stretchy="true">_</mo>`)
			sb.WriteString("</munder>")
		} else {
			sb.WriteString("<mover>")
			sb.WriteString("<mrow>")
			renderChildren(sb, e)
			sb.WriteString("</mrow>")
			sb.WriteString(`<mo stretchy="true">&#x203E;</mo>`) // overline
			sb.WriteString("</mover>")
		}

	case "limLow":
		// m:limLow — expression with a lower limit.
		e := firstChild(n, "e")
		lim := firstChild(n, "lim")
		sb.WriteString("<munder>")
		sb.WriteString("<mrow>")
		renderChildren(sb, e)
		sb.WriteString("</mrow>")
		sb.WriteString("<mrow>")
		renderChildren(sb, lim)
		sb.WriteString("</mrow>")
		sb.WriteString("</munder>")

	case "limUpp":
		// m:limUpp — expression with an upper limit.
		e := firstChild(n, "e")
		lim := firstChild(n, "lim")
		sb.WriteString("<mover>")
		sb.WriteString("<mrow>")
		renderChildren(sb, e)
		sb.WriteString("</mrow>")
		sb.WriteString("<mrow>")
		renderChildren(sb, lim)
		sb.WriteString("</mrow>")
		sb.WriteString("</mover>")

	case "groupChr":
		// m:groupChr — grouping character (brace, bracket) over or under.
		gcPr := firstChild(n, "groupChrPr")
		chr := "⏞" // default: top curly bracket U+23DE
		pos := "top"
		if gcPr.local != "" {
			if chrEl := firstChild(gcPr, "chr"); chrEl.local != "" {
				if v, ok := chrEl.attrs["val"]; ok && v != "" {
					chr = v
				}
			}
			if posEl := firstChild(gcPr, "pos"); posEl.local != "" {
				if v, ok := posEl.attrs["val"]; ok {
					pos = v
				}
			}
		}
		e := firstChild(n, "e")
		if pos == "bot" {
			sb.WriteString("<munder>")
			sb.WriteString("<mrow>")
			renderChildren(sb, e)
			sb.WriteString("</mrow>")
			sb.WriteString("<mo>")
			sb.WriteString(html.EscapeString(chr))
			sb.WriteString("</mo>")
			sb.WriteString("</munder>")
		} else {
			sb.WriteString("<mover>")
			sb.WriteString("<mrow>")
			renderChildren(sb, e)
			sb.WriteString("</mrow>")
			sb.WriteString("<mo>")
			sb.WriteString(html.EscapeString(chr))
			sb.WriteString("</mo>")
			sb.WriteString("</mover>")
		}

	case "eqArr":
		// m:eqArr — array of aligned equations.
		sb.WriteString("<mtable>")
		for _, child := range n.children {
			if child.local == "e" {
				sb.WriteString("<mtr><mtd>")
				renderChildren(sb, child)
				sb.WriteString("</mtd></mtr>")
			}
		}
		sb.WriteString("</mtable>")

	case "m":
		// m:m — matrix. Parse mPr/mcs/mc for column alignment and column gap.
		mPr := firstChild(n, "mPr")
		var colAligns []string
		var colSpacing string
		if mPr.local != "" {
			if mcs := firstChild(mPr, "mcs"); mcs.local != "" {
				for _, mc := range mcs.children {
					if mc.local != "mc" {
						continue
					}
					mcPr := firstChild(mc, "mcPr")
					if mcPr.local == "" {
						continue
					}
					align := ""
					if mcJc := firstChild(mcPr, "mcJc"); mcJc.local != "" {
						align = mcJc.attrs["val"]
					}
					count := 1
					if countEl := firstChild(mcPr, "count"); countEl.local != "" {
						if v, err := strconv.Atoi(countEl.attrs["val"]); err == nil && v > 0 {
							count = v
						}
					}
					for i := 0; i < count; i++ {
						colAligns = append(colAligns, align)
					}
				}
			}
			cGpRule := 0
			if cGpRuleEl := firstChild(mPr, "cGpRule"); cGpRuleEl.local != "" {
				if v, err := strconv.Atoi(cGpRuleEl.attrs["val"]); err == nil {
					cGpRule = v
				}
			}
			if cGpRule == 3 || cGpRule == 4 {
				if cGpEl := firstChild(mPr, "cGp"); cGpEl.local != "" {
					if v, err := strconv.Atoi(cGpEl.attrs["val"]); err == nil {
						colSpacing = fmt.Sprintf("%dem", v)
					}
				}
			}
		}
		sb.WriteString("<mtable")
		if len(colAligns) > 0 {
			sb.WriteString(` columnalign="`)
			sb.WriteString(strings.Join(colAligns, " "))
			sb.WriteString(`"`)
		}
		if colSpacing != "" {
			sb.WriteString(fmt.Sprintf(` columnspacing="%s"`, colSpacing))
		}
		sb.WriteString(">")
		for _, child := range n.children {
			if child.local == "mr" {
				sb.WriteString("<mtr>")
				for _, cell := range child.children {
					if cell.local == "e" {
						sb.WriteString("<mtd>")
						renderChildren(sb, cell)
						sb.WriteString("</mtd>")
					}
				}
				sb.WriteString("</mtr>")
			}
		}
		sb.WriteString("</mtable>")

	case "phant":
		// m:phant — phantom (takes up space but not visible).
		e := firstChild(n, "e")
		sb.WriteString("<mphantom>")
		renderChildren(sb, e)
		sb.WriteString("</mphantom>")

	case "borderBox":
		// m:borderBox — boxed expression.
		e := firstChild(n, "e")
		sb.WriteString(`<menclose notation="box">`)
		renderChildren(sb, e)
		sb.WriteString("</menclose>")

	case "box":
		// m:box — layout grouping without visible border.
		renderChildren(sb, n)

	// Content-container elements: just recurse.
	case "e", "num", "den", "sub", "sup", "lim", "fName", "deg", "oMathPara":
		renderChildren(sb, n)

	// Property elements: no output.
	case "fPr", "radPr", "dPr", "naryPr", "sSupPr", "sSubPr", "sSubSupPr",
		"accPr", "barPr", "groupChrPr", "eqArrPr", "mPr", "mcs", "mc",
		"mcPr", "rPr", "pPr", "limLowPr", "limUppPr", "boxPr",
		"borderBoxPr", "phantPr", "funcPr", "ctrlPr", "sPr",
		"oMathParaPr":
		// skip

	default:
		// Unknown element — recurse into children as a best effort.
		renderChildren(sb, n)
	}
}

// renderMathRun converts a single m:r math run to MathML token elements.
func renderMathRun(sb *strings.Builder, n node) {
	rPr := firstChild(n, "rPr")
	lit := false
	sty := ""
	sz := ""
	color := ""
	hasBrk := false
	if rPr.local != "" {
		lit = onOffValue(firstChild(rPr, "lit"), false)
		if styEl := firstChild(rPr, "sty"); styEl.local != "" {
			sty = styEl.attrs["val"]
		}
		if szEl := firstChild(rPr, "sz"); szEl.local != "" {
			if v := szEl.attrs["val"]; v != "" {
				if hp, err := strconv.Atoi(v); err == nil && hp > 0 {
					sz = fmt.Sprintf("%.1fpt", float64(hp)/2.0)
				}
			}
		}
		if colEl := firstChild(rPr, "color"); colEl.local != "" {
			if v := colEl.attrs["val"]; v != "" {
				color = normalizeMathColor(v)
			}
		}
		hasBrk = firstChild(rPr, "brk").local != ""
	}

	tEl := firstChild(n, "t")
	text := tEl.text
	if text == "" {
		return
	}

	escaped := html.EscapeString(text)

	if lit {
		attrs := mathRunAttrs("mtext", text, sty, sz, color)
		sb.WriteString("<mtext")
		if attrs != "" {
			sb.WriteString(" ")
			sb.WriteString(attrs)
		}
		sb.WriteString(">")
		sb.WriteString(escaped)
		sb.WriteString("</mtext>")
		if hasBrk {
			sb.WriteString(`<mspace linebreak="newline"/>`)
		}
		return
	}

	tag := classifyMathText(text)
	attrs := mathRunAttrs(tag, text, sty, sz, color)
	sb.WriteString("<")
	sb.WriteString(tag)
	if attrs != "" {
		sb.WriteString(" ")
		sb.WriteString(attrs)
	}
	sb.WriteString(">")
	sb.WriteString(escaped)
	sb.WriteString("</")
	sb.WriteString(tag)
	sb.WriteString(">")
	if hasBrk {
		sb.WriteString(`<mspace linebreak="newline"/>`)
	}
}

// classifyMathText returns the MathML token element name for a text string:
// "mi" for identifiers, "mn" for numbers, "mo" for operators.
func classifyMathText(text string) string {
	runes := []rune(text)
	if len(runes) == 0 {
		return "mi"
	}
	if len(runes) == 1 {
		r := runes[0]
		if unicode.IsDigit(r) {
			return "mn"
		}
		if isMathOperator(r) {
			return "mo"
		}
		return "mi"
	}
	// Multi-character text.
	allNumeric := true
	for _, r := range runes {
		if !unicode.IsDigit(r) && r != '.' && r != ',' {
			allNumeric = false
			break
		}
	}
	if allNumeric {
		return "mn"
	}
	allOps := true
	for _, r := range runes {
		if !isMathOperator(r) {
			allOps = false
			break
		}
	}
	if allOps {
		return "mo"
	}
	return "mi"
}

// mathRunAttrs returns the MathML style attribute string for a math run,
// combining mathvariant, mathsize, and mathcolor.
func mathRunAttrs(tag, text, sty, sz, color string) string {
	var parts []string
	switch sty {
	case "b":
		parts = append(parts, `mathvariant="bold"`)
	case "bi":
		parts = append(parts, `mathvariant="bold-italic"`)
	case "n", "p":
		parts = append(parts, `mathvariant="normal"`)
	}
	// Default: single identifier letter uses italic (browser default for <mi>),
	// multi-char identifiers use normal (upright) to match Word behavior.
	if sty == "" && tag == "mi" && len([]rune(text)) > 1 {
		parts = append(parts, `mathvariant="normal"`)
	}
	if sz != "" {
		parts = append(parts, fmt.Sprintf(`mathsize="%s"`, sz))
	}
	if color != "" {
		parts = append(parts, fmt.Sprintf(`mathcolor="%s"`, color))
	}
	return strings.Join(parts, " ")
}

// normalizeMathColor converts an OMML color value to a MathML-compatible color.
// It accepts 6-digit hex (with or without leading #) and returns a #RRGGBB string.
func normalizeMathColor(v string) string {
	v = strings.TrimPrefix(v, "#")
	v = strings.TrimSpace(v)
	if len(v) == 6 {
		valid := true
		for _, r := range v {
			if !unicode.IsDigit(r) && (r < 'a' || r > 'f') && (r < 'A' || r > 'F') {
				valid = false
				break
			}
		}
		if valid {
			return "#" + strings.ToUpper(v)
		}
	}
	return ""
}

// isMathOperator reports whether r is a common mathematical operator character.
func isMathOperator(r rune) bool {
	switch r {
	case '+', '-', '−', '·', '×', '÷', '/', '=', '<', '>',
		'≤', '≥', '≠', '≈', '≡', '≅', '∝', '∈', '∉', '∋',
		'⊂', '⊃', '⊆', '⊇', '∪', '∩', '∧', '∨', '¬',
		'→', '←', '↔', '⇒', '⇐', '⇔',
		'±', '∓', '∞', '∂', '∇', '∀', '∃', '∄',
		'∑', '∏', '∫', '∮', '∯', '∰',
		'~', '^', '|', '\\', '!', '%', ',', ';', ':',
		'∘', '∙', '⊕', '⊗', '⊖', '⊙',
		'⌈', '⌉', '⌊', '⌋':
		return true
	}
	return false
}

// renderChildren renders all children of n to sb.
func renderChildren(sb *strings.Builder, n node) {
	for _, child := range n.children {
		renderNode(sb, child)
	}
}

// firstChild returns the first child with the given local name, or a zero node.
func firstChild(n node, localName string) node {
	for _, child := range n.children {
		if child.local == localName {
			return child
		}
	}
	return node{}
}

// hasContent reports whether n has any m:r or m:t descendants with non-whitespace text.
func hasContent(n node) bool {
	if strings.TrimSpace(n.text) != "" {
		return true
	}
	for _, child := range n.children {
		if hasContent(child) {
			return true
		}
	}
	return false
}

// onOffValue reads a CT_OnOff element's effective boolean value.
// OOXML CT_OnOff: element absent → dflt; present without val → true;
// present with val="1"/"true" → true; val="0"/"false" → false.
func onOffValue(n node, dflt bool) bool {
	if n.local == "" {
		return dflt
	}
	v, ok := n.attrs["val"]
	if !ok {
		return true
	}
	return v == "1" || v == "true" || v == "on"
}

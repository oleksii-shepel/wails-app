package mathtype

import (
	"encoding/binary"
	"office/pkg/ooxml/cfb"
	"strings"
	"testing"
)

func ch(r byte, embell bool) []byte {
	tag := byte(2)
	if embell {
		tag |= 0x20
	}
	return []byte{tag, 131, r, 0}
}
func line(content ...[]byte) []byte {
	b := []byte{1}
	for _, x := range content {
		b = append(b, x...)
	}
	return append(b, 0)
}
func stream(content ...[]byte) []byte {
	b := []byte{3, 1, 1, 3, 0, 10}
	for _, x := range content {
		b = append(b, x...)
	}
	return append(b, 0)
}
func TestMTEF3SubscriptToMathML(t *testing.T) {
	tmpl := []byte{3, 15, 1, 0}
	tmpl = append(tmpl, line(ch('1', false))...)
	tmpl = append(tmpl, 0)
	data := stream(line(ch('t', false), tmpl))
	eq, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	m, err := ToMathML(eq)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(m, "<msub>") || !strings.Contains(m, ">t<") || !strings.Contains(m, ">1<") {
		t.Fatalf("mathml=%s", m)
	}
}
func TestMTEF3Hat(t *testing.T) {
	c := ch('L', true)
	c = append(c, 6, 9, 0)
	data := stream(line(c))
	eq, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	m, _ := ToMathML(eq)
	if !strings.Contains(m, `<mover accent="true">`) {
		t.Fatalf("mathml=%s", m)
	}
}
func TestMTEF3Matrix(t *testing.T) {
	mx := []byte{5, 1, 2, 2, 2, 2, 0, 0}
	for _, r := range []byte{'1', '2', '3', '4'} {
		mx = append(mx, line(ch(r, false))...)
	}
	mx = append(mx, 0)
	data := stream(line(mx))
	eq, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	m, _ := ToMathML(eq)
	if strings.Count(m, "<mtd>") != 4 {
		t.Fatalf("mathml=%s", m)
	}
}

func TestMTEF3SuperscriptVariationUsesSupSlot(t *testing.T) {
	tmpl := []byte{3, 15, 0, 0}
	tmpl = append(tmpl, line(ch('2', false))...)
	tmpl = append(tmpl, 0)
	eq, err := Parse(stream(line(ch('x', false), tmpl)))
	if err != nil {
		t.Fatal(err)
	}
	m, err := ToMathML(eq)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(m, "<msup>") || strings.Contains(m, "<msub>") {
		t.Fatalf("mathml=%s", m)
	}
}

func TestMTEF3ExplicitPointSizeStaysPositive(t *testing.T) {
	// SIZE marker 101 stores the positive magnitude of -lsize in 32nds of a point.
	data := []byte{3, 1, 1, 3, 0, 9, 101, 0x80, 0x01}
	data = append(data, line(ch('x', false))...)
	data = append(data, 0)
	eq, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	m, err := ToMathML(eq)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(m, `mathsize="12.000pt"`) {
		t.Fatalf("mathml=%s", m)
	}
}

func TestMTEF3UnderArrowRendersAsAccentUnder(t *testing.T) {
	tmpl := []byte{3, 46, 1, 0}
	tmpl = append(tmpl, line(ch('v', false))...)
	tmpl = append(tmpl, 0)
	eq, err := Parse(stream(line(tmpl)))
	if err != nil {
		t.Fatal(err)
	}
	m, err := ToMathML(eq)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(m, `<munder accentunder="true">`) || !strings.Contains(m, "→") {
		t.Fatalf("mathml=%s", m)
	}
}

func TestMTEF3TemplateAlwaysConsumesOptionsByte(t *testing.T) {
	// Real Equation Editor 3.x writes the TMPL options byte even for SCRIPT.
	// If it is mistaken for the first object-list END, the script is detached
	// from its base and the following slot leaks into the outer row.
	tmpl := []byte{3, 15, 1, 0}
	tmpl = append(tmpl, 11) // SUB typesize
	tmpl = append(tmpl, line(ch('1', false))...)
	tmpl = append(tmpl, 0)
	eq, err := Parse(stream(line(ch('t', false), tmpl)))
	if err != nil {
		t.Fatal(err)
	}
	m, err := ToMathML(eq)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(m, `<msub><mi>t</mi>`) || strings.Contains(m, `><`) {
		t.Fatalf("mathml=%s", m)
	}
}

func TestMTEF3PrivateSpacingBecomesMathSpace(t *testing.T) {
	space := []byte{2, 152, 0x02, 0xEB}
	eq, err := Parse(stream(line(ch('a', false), space, ch('b', false))))
	if err != nil {
		t.Fatal(err)
	}
	m, err := ToMathML(eq)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(m, `<mspace width="0.167em"/>`) || strings.ContainsRune(m, '\uEB02') {
		t.Fatalf("mathml=%s", m)
	}
}

func TestMTEF3TopLevelEndIgnoresContainerTail(t *testing.T) {
	data := stream(line(ch('x', false)))
	data = append(data, 3, 0xAD, 0) // non-MTEF bytes after equation END
	eq, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	m, err := ToMathML(eq)
	if err != nil || !strings.Contains(m, `>x<`) {
		t.Fatalf("mathml=%s err=%v", m, err)
	}
}

func TestMathMLLogicalTypeSizesUseMathSemantics(t *testing.T) {
	eq := &Equation{Root: &Row{Items: []Expr{
		&Glyph{Rune: 'x', Style: Style{Size: Size{Kind: SizeFull}}},
		&Glyph{Rune: 'i', Style: Style{Size: Size{Kind: SizeSub}}},
		&Glyph{Rune: 'j', Style: Style{Size: Size{Kind: SizeSub2}}},
		&Glyph{Rune: '∑', Style: Style{Size: Size{Kind: SizeSymbol}}},
	}}}
	m, err := ToMathML(eq)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`scriptlevel="1"`, `scriptlevel="2"`, `mathsize="150%"`} {
		if !strings.Contains(m, want) {
			t.Fatalf("MathML missing %s: %s", want, m)
		}
	}
}

func TestMathMLMatrixCarriesAuthoredAlignment(t *testing.T) {
	eq := &Equation{Root: &Matrix{Rows: 1, Cols: 1, HAlign: 2, VAlign: 0, Cells: []Expr{&Glyph{Rune: 'x'}}}}
	m, err := ToMathML(eq)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(m, `align="top" columnalign="center" rowalign="top"`) {
		t.Fatalf("matrix alignment lost: %s", m)
	}
}

func TestMathMLLargeOperatorRespectsLimitPlacement(t *testing.T) {
	lower := &Glyph{Rune: '0'}
	upper := &Glyph{Rune: '1'}
	body := &Glyph{Rune: 'x'}
	integral := &Equation{Root: &LargeOperator{Operator: '∫', Body: body, Lower: lower, Upper: upper}}
	m, err := ToMathML(integral)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(m, `<msubsup><mo largeop="true" movablelimits="false" stretchy="false">∫</mo>`) || strings.Contains(m, `<munderover>`) {
		t.Fatalf("integral-style limits moved above/below: %s", m)
	}

	sum := &Equation{Root: &LargeOperator{Operator: '∑', Body: body, Lower: lower, Upper: upper, LimitsAboveBelow: true}}
	m, err = ToMathML(sum)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(m, `<munderover><mo largeop="true" movablelimits="false" stretchy="false">∑</mo>`) {
		t.Fatalf("summation-style limits not above/below: %s", m)
	}
}

func TestMathMLIntegralExpandControlsStretchyOperator(t *testing.T) {
	eq := &Equation{Root: &LargeOperator{Operator: '∫', Body: &Glyph{Rune: 'x'}, Expand: true}}
	m, err := ToMathML(eq)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(m, `largeop="true" movablelimits="false" stretchy="true"`) {
		t.Fatalf("expanding integral lost MTEF option: %s", m)
	}
}

func TestMathMLFenceAlignmentControlsSymmetry(t *testing.T) {
	axis := &Equation{Root: &Fence{Left: '(', Right: ')', Body: &Glyph{Rune: 'x'}, Alignment: 0}}
	m, err := ToMathML(axis)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(m, `symmetric="true"`) {
		t.Fatalf("axis-centered fence lost symmetry: %s", m)
	}

	content := &Equation{Root: &Fence{Left: '(', Right: ')', Body: &Glyph{Rune: 'x'}, Alignment: 1}}
	m, err = ToMathML(content)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(m, `symmetric="false"`) {
		t.Fatalf("content-centered fence incorrectly symmetric: %s", m)
	}
}

func TestMathMLNudgeIsLocalAndUsesMTEFPointUnits(t *testing.T) {
	n := &Offset{X: 32, Y: -16}
	eq := &Equation{Root: &Row{Items: []Expr{
		&Glyph{Rune: 'a'},
		&Glyph{Rune: 'b', Style: Style{Nudge: n}},
		&Glyph{Rune: 'c'},
	}}}
	m, err := ToMathML(eq)
	if err != nil {
		t.Fatal(err)
	}
	want := `<mpadded lspace="1.000pt" voffset="-0.500pt"><mi>b</mi></mpadded>`
	if !strings.Contains(m, want) {
		t.Fatalf("nudge not localized/converted: %s", m)
	}
	if strings.Count(m, `<mpadded`) != 1 {
		t.Fatalf("nudge leaked beyond target node: %s", m)
	}
}

func TestMathMLResolvesExplicitFontDefinition(t *testing.T) {
	eq := &Equation{Fonts: map[int]FontDef{3: {Name: "Times New Roman", Italic: true}}, Root: &Glyph{Rune: 'x', Style: Style{Typeface: 3}}}
	resolveEquationStyles(eq)
	m, err := ToMathML(eq)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(m, `mathfamily="Times New Roman"`) || !strings.Contains(m, `mathvariant="italic"`) {
		t.Fatalf("font definition not preserved: %s", m)
	}
}

func TestMathMLMatrixCarriesPartitionRules(t *testing.T) {
	eq := &Equation{Root: &Matrix{
		Rows: 2, Cols: 2,
		Cells:    []Expr{&Glyph{Rune: 'a'}, &Glyph{Rune: 'b'}, &Glyph{Rune: 'c'}, &Glyph{Rune: 'd'}},
		RowRules: []byte{1, 2, 1}, ColRules: []byte{1, 1, 1},
	}}
	m, err := ToMathML(eq)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`notation="top bottom left right"`, `rowlines="dashed"`, `columnlines="solid"`} {
		if !strings.Contains(m, want) {
			t.Fatalf("matrix decoration missing %s: %s", want, m)
		}
	}
}

func TestMathMLMatrixPreservesDottedPartitionSemantics(t *testing.T) {
	eq := &Equation{Root: &Matrix{
		Rows: 2, Cols: 2,
		Cells:    []Expr{&Glyph{Rune: 'a'}, &Glyph{Rune: 'b'}, &Glyph{Rune: 'c'}, &Glyph{Rune: 'd'}},
		RowRules: []byte{3, 3, 2}, ColRules: []byte{2, 3, 3},
	}}
	m, err := ToMathML(eq)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`data-mtef-row-parts="3 3 2"`,
		`data-mtef-col-parts="2 3 3"`,
		`rowlines="dashed"`,
		`columnlines="dashed"`,
	} {
		if !strings.Contains(m, want) {
			t.Fatalf("matrix partition semantics missing %s: %s", want, m)
		}
	}
	if strings.Contains(m, `notation="top`) || strings.Contains(m, ` top`) {
		t.Fatalf("dotted outer partition must not be silently promoted to solid: %s", m)
	}
}

func TestMathMLEmbellishmentUsesMathAccentGlyphs(t *testing.T) {
	eq := &Equation{Root: &Row{Items: []Expr{
		&Accent{Body: &Glyph{Rune: 'x'}, Kind: EmbellHat},
		&Accent{Body: &Glyph{Rune: 'y'}, Kind: EmbellTilde},
		&Accent{Body: &Glyph{Rune: 'z'}, Kind: EmbellBackPrime},
	}}}
	m, err := ToMathML(eq)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"ˆ", "˜", "‵"} {
		if !strings.Contains(m, want) {
			t.Fatalf("accent glyph missing %q: %s", want, m)
		}
	}
}

func TestMathMLRejectsEmptySemanticEquation(t *testing.T) {
	_, err := ToMathML(&Equation{Root: &Row{}})
	if err == nil {
		t.Fatal("ToMathML(empty row) succeeded; empty semantic math must retain OLE preview fallback")
	}
}

func TestMathMLAcceptsVisibleSemanticEquation(t *testing.T) {
	m, err := ToMathML(&Equation{Root: &Row{Items: []Expr{&Glyph{Rune: 'x'}}}})
	if err != nil {
		t.Fatalf("ToMathML: %v", err)
	}
	if !strings.Contains(m, "<mi>x</mi>") {
		t.Fatalf("MathML = %q", m)
	}
}

func TestMTEF3StyleOnlyEquationIsValidButNotRenderable(t *testing.T) {
	// Real Equation Editor 3.x objects can contain only FULL-size state and END.
	// This is valid MTEF framing, not a corrupt equation and not a reason to
	// suppress the authored OLE preview.
	data := []byte{3, 1, 1, 3, 0, 10, 0, 0}
	eq, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if HasVisibleContent(eq.Root) {
		t.Fatal("style-only equation reported visible content")
	}
	if _, err = ToMathML(eq); err != ErrNoVisibleContent {
		t.Fatalf("ToMathML error = %v, want ErrNoVisibleContent", err)
	}
}

func TestExtractMTEFFromOle10Native(t *testing.T) {
	mtef := []byte{4, 1, 0, 3, 5, 10, 0}
	native := make([]byte, 4+len(mtef))
	binary.LittleEndian.PutUint32(native[:4], uint32(len(mtef)))
	copy(native[4:], mtef)
	w := cfb.NewWriter()
	if err := w.AddStream("\x01Ole10Native", native); err != nil {
		t.Fatal(err)
	}
	ole, err := w.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	got, err := ExtractMTEF(ole)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(mtef) {
		t.Fatalf("MTEF = %v, want %v", got, mtef)
	}
}

func TestExtractMTEFRejectsMalformedOle10NativeLength(t *testing.T) {
	native := []byte{0xff, 0xff, 0xff, 0x7f, 4, 1, 0, 3, 5, 10, 0}
	w := cfb.NewWriter()
	if err := w.AddStream("\x01Ole10Native", native); err != nil {
		t.Fatal(err)
	}
	ole, err := w.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ExtractMTEF(ole); err == nil {
		t.Fatal("malformed Ole10Native length accepted")
	}
}

func TestMTEF4UsesSingleByteCharacterCodes(t *testing.T) {
	// MTEF4 CHAR stores an 8-bit encoded character. Treating it as the MTEF3
	// 16-bit MTCode consumes the next record tag and desynchronizes the stream.
	data := []byte{4, 1, 0, 3, 5, 10, 1,
		2, 0x81, 'P', 2, 0x81, 'D', 2, 0x81, 'O', 2, 0x81, 'P', 0}
	m, err := MathML(data)
	if err != nil {
		t.Fatalf("MathML: %v", err)
	}
	for _, want := range []string{"P", "D", "O"} {
		if !strings.Contains(m, want) {
			t.Fatalf("missing %q in %s", want, m)
		}
	}
}

func v4ch(typeface byte, r byte) []byte {
	return []byte{2, 128 + typeface, r}
}

func v4stream(content ...[]byte) []byte {
	b := []byte{4, 1, 0, 3, 5, 10}
	for _, x := range content {
		b = append(b, x...)
	}
	return append(b, 0)
}

func TestMTEF4SuperscriptUsesSecondPositionalSlotAfterNullSubscript(t *testing.T) {
	// MTEF4 ScrBoxClass order is subscript, superscript. A deleted subscript
	// remains a NULL LINE, so x² is [NULL, LINE(2)], not just [LINE(2)].
	tmpl := []byte{3, 15, 0, 0, 0x11}
	tmpl = append(tmpl, line(v4ch(8, '2'))...)
	tmpl = append(tmpl, 0)
	eq, err := Parse(v4stream(line(v4ch(3, 'x'), tmpl)))
	if err != nil {
		t.Fatal(err)
	}
	m, err := ToMathML(eq)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(m, `<msup><mi>x</mi>`) || !strings.Contains(m, `>2</mn>`) || strings.Contains(m, `<msub>`) {
		t.Fatalf("mathml=%s", m)
	}
}

func TestMTEF4GreekStyleCharactersDecodeToUnicode(t *testing.T) {
	// fnLCGREEK stores keyboard-position bytes; d/r are delta/rho, not Latin.
	eq, err := Parse(v4stream(line(v4ch(4, 'd'), v4ch(4, 'r'))))
	if err != nil {
		t.Fatal(err)
	}
	m, err := ToMathML(eq)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(m, `>δ</mi>`) || !strings.Contains(m, `>ρ</mi>`) || strings.Contains(m, `>d</mi>`) || strings.Contains(m, `>r</mi>`) {
		t.Fatalf("mathml=%s", m)
	}
}

func TestMTEF4PDOPSquaresSurviveCanonicalScriptAttachment(t *testing.T) {
	square := func(base byte) []byte {
		tmpl := []byte{3, 15, 0, 0, 0x11}
		tmpl = append(tmpl, line(v4ch(8, '2'))...)
		tmpl = append(tmpl, 0)
		return append(v4ch(3, base), tmpl...)
	}
	body := line(square('E'), v4ch(6, '+'), square('N'), v4ch(6, '+'), square('V'))
	root := []byte{3, 13, 0, 0}
	root = append(root, body...)
	root = append(root, 0)
	eq, err := Parse(v4stream(line(root)))
	if err != nil {
		t.Fatal(err)
	}
	m, err := ToMathML(eq)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(m, `<msup>`) != 3 || !strings.Contains(m, `<msqrt>`) {
		t.Fatalf("mathml=%s", m)
	}
}

func TestMTEF4BigOperatorVariationControlsLimitPresence(t *testing.T) {
	lineExpr := func(ch byte) Expr { return &Row{Items: []Expr{&Glyph{Rune: rune(ch)}}} }
	xs := []Expr{lineExpr('f'), lineExpr('u'), lineExpr('l'), &Glyph{Rune: '∑'}}

	lowerOnly := v3Template(29, 0, 0, xs, Style{}).(*LargeOperator)
	if lowerOnly.Lower == nil || lowerOnly.Upper != nil {
		t.Fatalf("sum lower-only limits = upper:%T lower:%T", lowerOnly.Upper, lowerOnly.Lower)
	}
	both := v3Template(29, 1, 0, xs, Style{}).(*LargeOperator)
	if both.Lower == nil || both.Upper == nil {
		t.Fatalf("sum both limits = upper:%T lower:%T", both.Upper, both.Lower)
	}
	none := v3Template(29, 2, 0, xs, Style{}).(*LargeOperator)
	if none.Lower != nil || none.Upper != nil {
		t.Fatalf("sum no-limits retained slots = upper:%T lower:%T", none.Upper, none.Lower)
	}
}

func TestMTEF4IntegralVariantsPreserveContourAreaAndVolumeOperators(t *testing.T) {
	cases := []struct {
		selector, variation byte
		want                rune
	}{
		{21, 3, '∮'}, {22, 2, '∯'}, {23, 2, '∰'},
		{24, 2, '∮'}, {25, 0, '∯'}, {26, 0, '∰'},
	}
	for _, tc := range cases {
		op := v3Template(tc.selector, tc.variation, 0, []Expr{&Row{}}, Style{}).(*LargeOperator)
		if op.Operator != tc.want {
			t.Fatalf("selector %d variation %d operator=%q want %q", tc.selector, tc.variation, op.Operator, tc.want)
		}
	}
}

func TestMTEF4BigOperatorUsesAuthoredOperatorCharacter(t *testing.T) {
	xs := []Expr{&Row{}, &Row{}, &Row{}, &Glyph{Rune: '⊕'}}
	op := v3Template(29, 2, 0, xs, Style{}).(*LargeOperator)
	if op.Operator != '⊕' {
		t.Fatalf("operator=%q want authored ⊕", op.Operator)
	}
}

func TestMTEF34CustomBigOperatorPreservesStructuredOperatorAndSourceOrderLimits(t *testing.T) {
	lower := &Row{Items: []Expr{&Glyph{Rune: 'm'}, &Glyph{Rune: '='}, &Glyph{Rune: '1'}}}
	upper := &Row{Items: []Expr{&Glyph{Rune: 'M'}}}
	operator := &Row{Items: []Expr{&Glyph{Rune: 'Y'}, &Glyph{Rune: '='}, &Glyph{Rune: 'X'}}}
	op := v3Template(42, 2, 0, []Expr{&Row{}, lower, upper, operator}, Style{}).(*LargeOperator)
	if op.Operator != 0 || op.OperatorExpr != operator || op.Lower != lower || op.Upper != upper {
		t.Fatalf("custom BigOp = %+v, want structured operator with source-order limits", op)
	}
	math, err := ToMathML(&Equation{Root: op})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(math, `<msubsup><mrow><mi>Y</mi><mo>=</mo><mi>X</mi></mrow><mrow><mi>m</mi><mo>=</mo><mn>1</mn></mrow><mrow><mi>M</mi></mrow></msubsup>`) {
		t.Fatalf("structured custom BigOp lost from MathML: %s", math)
	}
}

func TestMTEF3FenceTemplateIgnoresLayoutFontPositions(t *testing.T) {
	fence := v3Template(0, 0, 0, []Expr{
		&Row{Items: []Expr{&Glyph{Rune: 'x'}}},
		&Glyph{Rune: 'e'},
		&Glyph{Rune: 'f'},
	}, Style{}).(*Fence)
	if fence.Left != '⟨' || fence.Right != '⟩' {
		t.Fatalf("fence = %q %q, want angle delimiters", fence.Left, fence.Right)
	}
	math, err := ToMathML(&Equation{Root: fence})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(math, `>e</mo>`) || strings.Contains(math, `>f</mo>`) || !strings.Contains(math, `>⟨</mo>`) || !strings.Contains(math, `>⟩</mo>`) {
		t.Fatalf("layout-font fence positions leaked into MathML: %s", math)
	}
}

func TestMTEF4ParBoxPreservesAuthoredFenceCharacters(t *testing.T) {
	xs := []Expr{&Row{Items: []Expr{&Glyph{Rune: 'x'}}}, &Glyph{Rune: '〈'}, &Glyph{Rune: '〉'}}
	f := v3Template(1, 0, 2, xs, Style{}).(*Fence)
	if f.Left != '〈' || f.Right != '〉' || f.Alignment != 2 {
		t.Fatalf("fence=%q %q alignment=%d", f.Left, f.Right, f.Alignment)
	}

	rightOnly := v3Template(3, 2, 0, []Expr{xs[0], &Glyph{Rune: '⟧'}}, Style{}).(*Fence)
	if rightOnly.Left != 0 || rightOnly.Right != '⟧' {
		t.Fatalf("right-only fence=%q %q", rightOnly.Left, rightOnly.Right)
	}
}

func TestMTEF4HorizontalBraceUsesAuthoredBraceCharacter(t *testing.T) {
	xs := []Expr{&Row{}, &Row{}, &Glyph{Rune: '⏟'}}
	u := v3Template(27, 0, 0, xs, Style{}).(*UnderOver)
	if u.Operator != '⏟' {
		t.Fatalf("operator=%q want authored brace", u.Operator)
	}
}

func TestMTEF4DiracPreservesAuthoredDelimiterCharacters(t *testing.T) {
	xs := []Expr{&Row{Items: []Expr{&Glyph{Rune: 'a'}}}, &Row{Items: []Expr{&Glyph{Rune: 'b'}}}, &Glyph{Rune: '〈'}, &Glyph{Rune: '‖'}, &Glyph{Rune: '〉'}}
	d := v3Template(45, 0, 0, xs, Style{}).(*Dirac)
	if d.LeftFence != '〈' || d.Middle != '‖' || d.RightFence != '〉' {
		t.Fatalf("dirac delimiters=%q %q %q", d.LeftFence, d.Middle, d.RightFence)
	}
	eq := &Equation{Root: d}
	m, err := ToMathML(eq)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"〈", "‖", "〉"} {
		if !strings.Contains(m, want) {
			t.Fatalf("missing %q in %s", want, m)
		}
	}
}

func TestMTEF4ExpandingArrowUsesAuthoredArrowCharacter(t *testing.T) {
	xs := []Expr{&Row{Items: []Expr{&Glyph{Rune: 'x'}}}, &Glyph{Rune: '⇀'}}
	a := v3Template(19, 0, 0, xs, Style{}).(*Accent)
	if a.Operator != '⇀' || a.Under {
		t.Fatalf("accent operator=%q under=%t", a.Operator, a.Under)
	}
	m, err := ToMathML(&Equation{Root: a})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(m, `>⇀</mo>`) || !strings.Contains(m, `<mover accent="true">`) {
		t.Fatalf("mathml=%s", m)
	}
}

func TestMTEF4LimitVariationSuppressesDeletedSlots(t *testing.T) {
	xs := []Expr{&Row{Items: []Expr{&Glyph{Rune: 'x'}}}, &Row{Items: []Expr{&Glyph{Rune: 'l'}}}, &Row{Items: []Expr{&Glyph{Rune: 'u'}}}}
	upper := v3Template(39, 0, 0, xs, Style{}).(*Limit)
	if upper.Lower != nil || upper.Upper == nil {
		t.Fatalf("upper-only limit lower=%T upper=%T", upper.Lower, upper.Upper)
	}
	lower := v3Template(39, 1, 0, xs, Style{}).(*Limit)
	if lower.Lower == nil || lower.Upper != nil {
		t.Fatalf("lower-only limit lower=%T upper=%T", lower.Lower, lower.Upper)
	}
	both := v3Template(39, 2, 0, xs, Style{}).(*Limit)
	if both.Lower == nil || both.Upper == nil {
		t.Fatalf("both limit lower=%T upper=%T", both.Lower, both.Upper)
	}
}

func TestMTEF4LongDivisionVariationSuppressesDeletedQuotient(t *testing.T) {
	xs := []Expr{&Row{Items: []Expr{&Glyph{Rune: 'd'}}}, &Row{Items: []Expr{&Glyph{Rune: 'q'}}}}
	with := v3Template(40, 0, 0, xs, Style{}).(*LongDivision)
	without := v3Template(40, 1, 0, xs, Style{}).(*LongDivision)
	if with.Quotient == nil || without.Quotient != nil {
		t.Fatalf("quotients with=%T without=%T", with.Quotient, without.Quotient)
	}
}

func TestMTEF4FractionVariationsPreserveAuthoredSizing(t *testing.T) {
	xs := []Expr{&Row{Items: []Expr{&Glyph{Rune: 'a'}}}, &Row{Items: []Expr{&Glyph{Rune: 'b'}}}}
	piece := v3Template(14, 1, 0, xs, Style{}).(*Fraction)
	if !piece.Compact {
		t.Fatal("piece fraction lost compact sizing")
	}
	m, err := ToMathML(&Equation{Root: piece})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(m, `<mstyle scriptlevel="+1"><mfrac>`) {
		t.Fatalf("piece mathml=%s", m)
	}

	slash := v3Template(41, 2, 0, xs, Style{}).(*Fraction)
	if !slash.Slash || slash.SlashVariant != 2 {
		t.Fatalf("slash=%t variation=%d", slash.Slash, slash.SlashVariant)
	}
	m, err = ToMathML(&Equation{Root: slash})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(m, `<mstyle scriptlevel="+1"><mrow>`) {
		t.Fatalf("slash mathml=%s", m)
	}
}

func TestMathMLUsesPublishedMTEFPileAlignmentValues(t *testing.T) {
	cases := []struct {
		h, v byte
		want string
	}{
		{1, 0, `columnalign="left" rowalign="top"`},
		{2, 1, `columnalign="center" rowalign="baseline"`},
		{3, 2, `columnalign="right" rowalign="bottom"`},
		{2, 3, `columnalign="center" rowalign="center"`},
	}
	for _, tc := range cases {
		eq := &Equation{Root: &Pile{Lines: []Expr{&Glyph{Rune: 'x'}}, HAlign: tc.h, VAlign: tc.v}}
		m, err := ToMathML(eq)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(m, tc.want) {
			t.Fatalf("pile alignment %d/%d: want %q in %s", tc.h, tc.v, tc.want, m)
		}
	}
}

func TestMathMLSeparatesMatrixContainerAndCellVerticalAlignment(t *testing.T) {
	eq := &Equation{Root: &Matrix{
		Rows: 1, Cols: 1, Cells: []Expr{&Glyph{Rune: 'x'}},
		VAlign: 3, HAlign: 3, CellVAlign: 0,
	}}
	m, err := ToMathML(eq)
	if err != nil {
		t.Fatal(err)
	}
	want := `<mtable align="center" columnalign="right" rowalign="top">`
	if !strings.Contains(m, want) {
		t.Fatalf("matrix alignments conflated: want %q in %s", want, m)
	}
}

func TestMTEF4SymbolStyleUsesUnicodeInsteadOfLegacyFontPositions(t *testing.T) {
	eq, err := Parse(v4stream(line(v4ch(6, 0xBB), v4ch(6, 0xC1))))
	if err != nil {
		t.Fatal(err)
	}
	m, err := ToMathML(eq)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(m, `>≈</mo>`) || !strings.Contains(m, `>ℑ</mi>`) || strings.Contains(m, `»`) || strings.Contains(m, `Á`) {
		t.Fatalf("mathml=%s", m)
	}
}

func TestMTEF4SymbolIntegralFontPositionDecodesBeforeTemplateOwnership(t *testing.T) {
	// 0xF2 is the integral glyph in MathType's fnSYMBOL/Symbol encoding.
	// Decoding it to Unicode prevents browsers without the legacy Symbol font
	// from rendering the authored operator as a question mark/tofu glyph.
	eq, err := Parse(v4stream(line(v4ch(6, 0xF2))))
	if err != nil {
		t.Fatal(err)
	}
	m, err := ToMathML(eq)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(m, `>∫</mo>`) || strings.Contains(m, `ò`) {
		t.Fatalf("mathml=%s", m)
	}
}

func TestNormalizeHoistsClosingDelimiterScriptOntoWholeFence(t *testing.T) {
	eq := &Equation{Root: normalize(&Row{Items: []Expr{
		&Glyph{Rune: '('},
		&Script{Base: &Glyph{Rune: 'G'}, Sup: &Glyph{Rune: 'T'}},
		&Glyph{Rune: 'G'},
		&Script{Base: &Glyph{Rune: ')'}, Sup: &Row{Items: []Expr{&Glyph{Rune: '−'}, &Glyph{Rune: '1'}}}},
	}})}
	m, err := ToMathML(eq)
	if err != nil {
		t.Fatal(err)
	}
	want := `<msup><mrow><mo stretchy="true" fence="true" symmetric="true">(</mo><mrow><msup><mi>G</mi><mi>T</mi></msup><mi>G</mi></mrow><mo stretchy="true" fence="true" symmetric="true">)</mo></mrow><mrow><mo>−</mo><mn>1</mn></mrow></msup>`
	if !strings.Contains(m, want) {
		t.Fatalf("closing-delimiter script was not hoisted onto paired fence: %s", m)
	}
	if strings.Contains(m, `<msup><mo>)</mo>`) {
		t.Fatalf("script remained attached to closing delimiter: %s", m)
	}
}

func TestMathMLPreservesOrdinaryMTEFSpaceGlyphs(t *testing.T) {
	eq := &Equation{Root: &Row{Items: []Expr{
		&Glyph{Rune: 'D'}, &Glyph{Rune: 'e'}, &Glyph{Rune: 'l'}, &Glyph{Rune: 'a'}, &Glyph{Rune: 'y'},
		&Glyph{Rune: ' '}, &Glyph{Rune: ' '}, &Glyph{Rune: ' '}, &Glyph{Rune: ' '}, &Glyph{Rune: 'e'},
	}}}
	m, err := ToMathML(eq)
	if err != nil {
		t.Fatalf("ToMathML: %v", err)
	}
	if got := strings.Count(m, `<mspace width="0.25em"/>`); got != 4 {
		t.Fatalf("ordinary MTEF spaces = %d, want 4: %s", got, m)
	}
}

func TestMTEF4SpaceTypefaceDoesNotEmitMissingGlyph(t *testing.T) {
	// fnSPACE is layout metadata in MTEF4, not a printable character.
	eq, err := Parse(v4stream(line(v4ch(24, 0x08), v4ch(3, 'x'))))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	m, err := ToMathML(eq)
	if err != nil {
		t.Fatalf("ToMathML: %v", err)
	}
	if !strings.Contains(m, `<mspace width="0.278em"/>`) {
		t.Fatalf("MathML = %s, want normal mspace", m)
	}
	if strings.ContainsRune(m, '\u0008') || strings.ContainsRune(m, '\uFFFD') {
		t.Fatalf("MathML contains printable control/replacement glyph: %q", m)
	}
}

func TestMTEF4UnknownSpaceCodeStillStaysNonPrinting(t *testing.T) {
	eq, err := Parse(v4stream(line(v4ch(24, 0x7f), v4ch(3, 'x'))))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	m, err := ToMathML(eq)
	if err != nil {
		t.Fatalf("ToMathML: %v", err)
	}
	if strings.ContainsRune(m, '\u007f') || strings.ContainsRune(m, '\uFFFD') {
		t.Fatalf("MathML contains fnSPACE byte as glyph: %q", m)
	}
}

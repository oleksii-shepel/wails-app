package mathml

import (
	"fmt"
	"strings"
	"testing"
)

func TestConvertEmpty(t *testing.T) {
	if got := Convert(nil); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
	if got := Convert([]byte{}); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestConvertSimpleIdentifier(t *testing.T) {
	// m:oMath containing a single run with text "x"
	input := `<m:oMath xmlns:m="http://schemas.openxmlformats.org/officeDocument/2006/math"><m:r><m:t>x</m:t></m:r></m:oMath>`
	got := Convert([]byte(input))
	if !strings.Contains(got, `<math`) {
		t.Fatalf("expected <math> element, got: %s", got)
	}
	if !strings.Contains(got, `display="inline"`) {
		t.Errorf("expected inline display, got: %s", got)
	}
	if !strings.Contains(got, "<mi>x</mi>") {
		t.Errorf("expected <mi>x</mi>, got: %s", got)
	}
}

func TestConvertNumber(t *testing.T) {
	input := `<m:oMath xmlns:m="http://schemas.openxmlformats.org/officeDocument/2006/math"><m:r><m:t>42</m:t></m:r></m:oMath>`
	got := Convert([]byte(input))
	if !strings.Contains(got, "<mn>42</mn>") {
		t.Errorf("expected <mn>42</mn>, got: %s", got)
	}
}

func TestConvertOperator(t *testing.T) {
	input := `<m:oMath xmlns:m="http://schemas.openxmlformats.org/officeDocument/2006/math"><m:r><m:t>+</m:t></m:r></m:oMath>`
	got := Convert([]byte(input))
	if !strings.Contains(got, "<mo>+</mo>") {
		t.Errorf("expected <mo>+</mo>, got: %s", got)
	}
}

func TestConvertFraction(t *testing.T) {
	// x/y fraction
	input := `<m:oMath xmlns:m="http://schemas.openxmlformats.org/officeDocument/2006/math">
		<m:f>
			<m:num><m:r><m:t>a</m:t></m:r></m:num>
			<m:den><m:r><m:t>b</m:t></m:r></m:den>
		</m:f>
	</m:oMath>`
	got := Convert([]byte(input))
	if !strings.Contains(got, "<mfrac>") {
		t.Fatalf("expected <mfrac>, got: %s", got)
	}
	if !strings.Contains(got, "<mi>a</mi>") {
		t.Errorf("expected <mi>a</mi> in numerator, got: %s", got)
	}
	if !strings.Contains(got, "<mi>b</mi>") {
		t.Errorf("expected <mi>b</mi> in denominator, got: %s", got)
	}
}

func TestConvertSqrt(t *testing.T) {
	// Square root with hidden degree
	input := `<m:oMath xmlns:m="http://schemas.openxmlformats.org/officeDocument/2006/math">
		<m:rad>
			<m:radPr><m:degHide m:val="1"/></m:radPr>
			<m:deg></m:deg>
			<m:e><m:r><m:t>x</m:t></m:r></m:e>
		</m:rad>
	</m:oMath>`
	got := Convert([]byte(input))
	if !strings.Contains(got, "<msqrt>") {
		t.Fatalf("expected <msqrt>, got: %s", got)
	}
}

func TestConvertNthRoot(t *testing.T) {
	// Cube root
	input := `<m:oMath xmlns:m="http://schemas.openxmlformats.org/officeDocument/2006/math">
		<m:rad>
			<m:deg><m:r><m:t>3</m:t></m:r></m:deg>
			<m:e><m:r><m:t>x</m:t></m:r></m:e>
		</m:rad>
	</m:oMath>`
	got := Convert([]byte(input))
	if !strings.Contains(got, "<mroot>") {
		t.Fatalf("expected <mroot>, got: %s", got)
	}
	if !strings.Contains(got, "<mn>3</mn>") {
		t.Errorf("expected degree <mn>3</mn>, got: %s", got)
	}
}

func TestConvertSuperscript(t *testing.T) {
	// x^2
	input := `<m:oMath xmlns:m="http://schemas.openxmlformats.org/officeDocument/2006/math">
		<m:sSup>
			<m:e><m:r><m:t>x</m:t></m:r></m:e>
			<m:sup><m:r><m:t>2</m:t></m:r></m:sup>
		</m:sSup>
	</m:oMath>`
	got := Convert([]byte(input))
	if !strings.Contains(got, "<msup>") {
		t.Fatalf("expected <msup>, got: %s", got)
	}
	if !strings.Contains(got, "<mi>x</mi>") {
		t.Errorf("expected <mi>x</mi> as base, got: %s", got)
	}
	if !strings.Contains(got, "<mn>2</mn>") {
		t.Errorf("expected <mn>2</mn> as exponent, got: %s", got)
	}
}

func TestConvertSubscript(t *testing.T) {
	// x_i
	input := `<m:oMath xmlns:m="http://schemas.openxmlformats.org/officeDocument/2006/math">
		<m:sSub>
			<m:e><m:r><m:t>x</m:t></m:r></m:e>
			<m:sub><m:r><m:t>i</m:t></m:r></m:sub>
		</m:sSub>
	</m:oMath>`
	got := Convert([]byte(input))
	if !strings.Contains(got, "<msub>") {
		t.Fatalf("expected <msub>, got: %s", got)
	}
}

func TestConvertSubSup(t *testing.T) {
	// x_i^2
	input := `<m:oMath xmlns:m="http://schemas.openxmlformats.org/officeDocument/2006/math">
		<m:sSubSup>
			<m:e><m:r><m:t>x</m:t></m:r></m:e>
			<m:sub><m:r><m:t>i</m:t></m:r></m:sub>
			<m:sup><m:r><m:t>2</m:t></m:r></m:sup>
		</m:sSubSup>
	</m:oMath>`
	got := Convert([]byte(input))
	if !strings.Contains(got, "<msubsup>") {
		t.Fatalf("expected <msubsup>, got: %s", got)
	}
}

func TestConvertDelimiter(t *testing.T) {
	// (x + y)
	input := `<m:oMath xmlns:m="http://schemas.openxmlformats.org/officeDocument/2006/math">
		<m:d>
			<m:dPr><m:begChr m:val="("/><m:endChr m:val=")"/></m:dPr>
			<m:e><m:r><m:t>x</m:t></m:r></m:e>
		</m:d>
	</m:oMath>`
	got := Convert([]byte(input))
	if !strings.Contains(got, `<mo stretchy="true">(</mo>`) {
		t.Fatalf("expected opening paren, got: %s", got)
	}
	if !strings.Contains(got, `<mo stretchy="true">)</mo>`) {
		t.Fatalf("expected closing paren, got: %s", got)
	}
}

func TestConvertNary(t *testing.T) {
	// Summation ∑_{i=0}^{n}
	input := `<m:oMath xmlns:m="http://schemas.openxmlformats.org/officeDocument/2006/math">
		<m:nary>
			<m:naryPr><m:chr m:val="∑"/><m:limLoc m:val="undOvr"/></m:naryPr>
			<m:sub><m:r><m:t>i</m:t></m:r></m:sub>
			<m:sup><m:r><m:t>n</m:t></m:r></m:sup>
			<m:e><m:r><m:t>x</m:t></m:r></m:e>
		</m:nary>
	</m:oMath>`
	got := Convert([]byte(input))
	if !strings.Contains(got, "<munderover>") {
		t.Fatalf("expected <munderover>, got: %s", got)
	}
	if !strings.Contains(got, "<mo>∑</mo>") {
		t.Errorf("expected summation symbol, got: %s", got)
	}
}

func TestConvertDisplayMode(t *testing.T) {
	// m:oMathPara → display="block"
	input := `<m:oMathPara xmlns:m="http://schemas.openxmlformats.org/officeDocument/2006/math">
		<m:oMath><m:r><m:t>x</m:t></m:r></m:oMath>
	</m:oMathPara>`
	got := Convert([]byte(input))
	if !strings.Contains(got, `display="block"`) {
		t.Errorf("expected block display for oMathPara, got: %s", got)
	}
}

func TestConvertLiteralText(t *testing.T) {
	// m:rPr/m:lit — output as <mtext>
	input := `<m:oMath xmlns:m="http://schemas.openxmlformats.org/officeDocument/2006/math">
		<m:r><m:rPr><m:lit/></m:rPr><m:t>hello</m:t></m:r>
	</m:oMath>`
	got := Convert([]byte(input))
	if !strings.Contains(got, "<mtext>hello</mtext>") {
		t.Errorf("expected <mtext>hello</mtext>, got: %s", got)
	}
}

func TestConvertMultiLetterIdentifier(t *testing.T) {
	// Multi-letter identifiers get mathvariant="normal"
	input := `<m:oMath xmlns:m="http://schemas.openxmlformats.org/officeDocument/2006/math">
		<m:r><m:t>sin</m:t></m:r>
	</m:oMath>`
	got := Convert([]byte(input))
	if !strings.Contains(got, `mathvariant="normal"`) {
		t.Errorf("expected mathvariant=normal for multi-letter id, got: %s", got)
	}
}

func TestConvertMatrix(t *testing.T) {
	// 2x2 matrix
	input := `<m:oMath xmlns:m="http://schemas.openxmlformats.org/officeDocument/2006/math">
		<m:m>
			<m:mr>
				<m:e><m:r><m:t>a</m:t></m:r></m:e>
				<m:e><m:r><m:t>b</m:t></m:r></m:e>
			</m:mr>
			<m:mr>
				<m:e><m:r><m:t>c</m:t></m:r></m:e>
				<m:e><m:r><m:t>d</m:t></m:r></m:e>
			</m:mr>
		</m:m>
	</m:oMath>`
	got := Convert([]byte(input))
	if !strings.Contains(got, "<mtable>") {
		t.Fatalf("expected <mtable>, got: %s", got)
	}
	if strings.Count(got, "<mtr>") != 2 {
		t.Errorf("expected 2 rows, got: %s", got)
	}
}

func TestConvertHTMLEscaping(t *testing.T) {
	// Ensure < > & are escaped in output
	input := `<m:oMath xmlns:m="http://schemas.openxmlformats.org/officeDocument/2006/math">
		<m:r><m:t>&lt;</m:t></m:r>
	</m:oMath>`
	got := Convert([]byte(input))
	if !strings.Contains(got, "<mo>&lt;</mo>") {
		t.Errorf("expected HTML-escaped operator, got: %s", got)
	}
}


func TestConvertPreSubSup(t *testing.T) {
	// m:sPre — pre-sub-superscript (tensor-like notation)
	input := `<m:oMath xmlns:m="http://schemas.openxmlformats.org/officeDocument/2006/math">
		<m:sPre>
			<m:e><m:r><m:t>A</m:t></m:r></m:e>
			<m:sub><m:r><m:t>i</m:t></m:r></m:sub>
			<m:sup><m:r><m:t>j</m:t></m:r></m:sup>
		</m:sPre>
	</m:oMath>`
	got := Convert([]byte(input))
	if !strings.Contains(got, "<mmultiscripts>") {
		t.Fatalf("expected <mmultiscripts>, got: %s", got)
	}
	if !strings.Contains(got, "<mprescripts/>") {
		t.Errorf("expected <mprescripts/>, got: %s", got)
	}
	if !strings.Contains(got, "<mi>A</mi>") {
		t.Errorf("expected base A, got: %s", got)
	}
	if !strings.Contains(got, "<mi>i</mi>") {
		t.Errorf("expected sub i, got: %s", got)
	}
	if !strings.Contains(got, "<mi>j</mi>") {
		t.Errorf("expected sup j, got: %s", got)
	}
}


func TestConvertLimLow(t *testing.T) {
	// m:limLow — expression with a lower limit (e.g. lim_{x→0}).
	input := `<m:oMath xmlns:m="http://schemas.openxmlformats.org/officeDocument/2006/math">
		<m:limLow>
			<m:e><m:r><m:t>lim</m:t></m:r></m:e>
			<m:lim><m:r><m:t>x→0</m:t></m:r></m:lim>
		</m:limLow>
	</m:oMath>`
	got := Convert([]byte(input))
	if !strings.Contains(got, "<munder>") {
		t.Fatalf("expected <munder>, got: %s", got)
	}
	if !strings.Contains(got, "<mi mathvariant=\"normal\">lim</mi>") {
		t.Errorf("expected base lim, got: %s", got)
	}
	if !strings.Contains(got, "x→0") {
		t.Errorf("expected limit text, got: %s", got)
	}
}

func TestConvertLimUpp(t *testing.T) {
	// m:limUpp — expression with an upper limit.
	input := `<m:oMath xmlns:m="http://schemas.openxmlformats.org/officeDocument/2006/math">
		<m:limUpp>
			<m:e><m:r><m:t>A</m:t></m:r></m:e>
			<m:lim><m:r><m:t>B</m:t></m:r></m:lim>
		</m:limUpp>
	</m:oMath>`
	got := Convert([]byte(input))
	if !strings.Contains(got, "<mover>") {
		t.Fatalf("expected <mover>, got: %s", got)
	}
	if !strings.Contains(got, "<mi>A</mi>") {
		t.Errorf("expected base A, got: %s", got)
	}
	if !strings.Contains(got, "<mi>B</mi>") {
		t.Errorf("expected upper limit B, got: %s", got)
	}
}


func TestConvertBorderBox(t *testing.T) {
	input := `<m:oMath xmlns:m="http://schemas.openxmlformats.org/officeDocument/2006/math">
		<m:borderBox>
			<m:e><m:r><m:t>x</m:t></m:r></m:e>
		</m:borderBox>
	</m:oMath>`
	got := Convert([]byte(input))
	if !strings.Contains(got, "<menclose notation=\"box\">") {
		t.Fatalf("expected <menclose notation=\"box\">, got: %s", got)
	}
	if !strings.Contains(got, "<mi>x</mi>") {
		t.Errorf("expected base x, got: %s", got)
	}
}

func TestConvertBox(t *testing.T) {
	input := `<m:oMath xmlns:m="http://schemas.openxmlformats.org/officeDocument/2006/math">
		<m:box>
			<m:e><m:r><m:t>a</m:t></m:r></m:e>
			<m:e><m:r><m:t>b</m:t></m:r></m:e>
		</m:box>
	</m:oMath>`
	got := Convert([]byte(input))
	if !strings.Contains(got, "<mi>a</mi>") {
		t.Errorf("expected a, got: %s", got)
	}
	if !strings.Contains(got, "<mi>b</mi>") {
		t.Errorf("expected b, got: %s", got)
	}
}

func TestConvertEqArr(t *testing.T) {
	input := `<m:oMath xmlns:m="http://schemas.openxmlformats.org/officeDocument/2006/math">
		<m:eqArr>
			<m:e><m:r><m:t>x=1</m:t></m:r></m:e>
			<m:e><m:r><m:t>y=2</m:t></m:r></m:e>
		</m:eqArr>
	</m:oMath>`
	got := Convert([]byte(input))
	if !strings.Contains(got, "<mtable>") {
		t.Fatalf("expected <mtable>, got: %s", got)
	}
	if strings.Count(got, "<mtr>") != 2 {
		t.Errorf("expected 2 rows, got: %s", got)
	}
}

func TestConvertMathRunSize(t *testing.T) {
	input := `<m:oMath xmlns:m="http://schemas.openxmlformats.org/officeDocument/2006/math">
		<m:r>
			<m:rPr><m:sz m:val="24"/></m:rPr>
			<m:t>x</m:t>
		</m:r>
	</m:oMath>`
	got := Convert([]byte(input))
	if !strings.Contains(got, `mathsize="12.0pt"`) {
		t.Errorf("expected mathsize=12.0pt, got: %s", got)
	}
}

func TestConvertMathRunColor(t *testing.T) {
	input := `<m:oMath xmlns:m="http://schemas.openxmlformats.org/officeDocument/2006/math">
		<m:r>
			<m:rPr><m:color m:val="FF0000"/></m:rPr>
			<m:t>x</m:t>
		</m:r>
	</m:oMath>`
	got := Convert([]byte(input))
	if !strings.Contains(got, `mathcolor="#FF0000"`) {
		t.Errorf("expected mathcolor=#FF0000, got: %s", got)
	}
}

func TestConvertMathRunSizeAndColor(t *testing.T) {
	input := `<m:oMath xmlns:m="http://schemas.openxmlformats.org/officeDocument/2006/math">
		<m:r>
			<m:rPr>
				<m:sz m:val="36"/>
				<m:color m:val="00FF00"/>
			</m:rPr>
			<m:t>sin</m:t>
		</m:r>
	</m:oMath>`
	got := Convert([]byte(input))
	if !strings.Contains(got, `mathsize="18.0pt"`) {
		t.Errorf("expected mathsize=18.0pt, got: %s", got)
	}
	if !strings.Contains(got, `mathcolor="#00FF00"`) {
		t.Errorf("expected mathcolor=#00FF00, got: %s", got)
	}
	if !strings.Contains(got, `mathvariant="normal"`) {
		t.Errorf("expected mathvariant=normal for multi-char id, got: %s", got)
	}
}

func TestConvertMathRunLiteralWithStyle(t *testing.T) {
	input := `<m:oMath xmlns:m="http://schemas.openxmlformats.org/officeDocument/2006/math">
		<m:r>
			<m:rPr>
				<m:lit/>
				<m:sz m:val="20"/>
				<m:color m:val="0000FF"/>
			</m:rPr>
			<m:t>hello</m:t>
		</m:r>
	</m:oMath>`
	got := Convert([]byte(input))
	if !strings.Contains(got, `<mtext mathsize="10.0pt" mathcolor="#0000FF">hello</mtext>`) {
		t.Errorf("expected styled mtext, got: %s", got)
	}
}

func TestConvertMathRunBoldWithSize(t *testing.T) {
	input := `<m:oMath xmlns:m="http://schemas.openxmlformats.org/officeDocument/2006/math">
		<m:r>
			<m:rPr>
				<m:sty m:val="b"/>
				<m:sz m:val="28"/>
			</m:rPr>
			<m:t>A</m:t>
		</m:r>
	</m:oMath>`
	got := Convert([]byte(input))
	if !strings.Contains(got, `mathvariant="bold"`) {
		t.Errorf("expected bold, got: %s", got)
	}
	if !strings.Contains(got, `mathsize="14.0pt"`) {
		t.Errorf("expected mathsize=14.0pt, got: %s", got)
	}
}

func TestConvertAlignedEquations(t *testing.T) {
	input := `<m:oMathPara xmlns:m="http://schemas.openxmlformats.org/officeDocument/2006/math">
		<m:oMath>
			<m:r><m:t>a+b</m:t></m:r>
			<m:r>
				<m:rPr><m:aln/></m:rPr>
				<m:t>-c=2</m:t>
			</m:r>
		</m:oMath>
		<m:oMath>
			<m:r><m:t>x</m:t></m:r>
			<m:r>
				<m:rPr><m:aln/></m:rPr>
				<m:t>-y+z=3</m:t>
			</m:r>
		</m:oMath>
	</m:oMathPara>`
	got := Convert([]byte(input))
	if !strings.Contains(got, "<mtable>") {
		t.Fatalf("expected <mtable>, got: %s", got)
	}
	if strings.Count(got, "<mtr>") != 2 {
		t.Errorf("expected 2 rows, got: %s", got)
	}
	if strings.Count(got, "<mtd>") != 4 {
		t.Errorf("expected 4 cells (2 per row), got: %s", got)
	}
	if !strings.Contains(got, "<mi mathvariant=\"normal\">a+b</mi>") {
		t.Errorf("expected a+b in output, got: %s", got)
	}
	if !strings.Contains(got, "<mi mathvariant=\"normal\">-c=2</mi>") {
		t.Errorf("expected -c=2 in output, got: %s", got)
	}
	if !strings.Contains(got, "<mi>x</mi>") {
		t.Errorf("expected x in output, got: %s", got)
	}
	if !strings.Contains(got, "<mi mathvariant=\"normal\">-y+z=3</mi>") {
		t.Errorf("expected -y+z=3 in output, got: %s", got)
	}
}

func TestConvertAlignedEquationsMultipleAlignments(t *testing.T) {
	input := `<m:oMathPara xmlns:m="http://schemas.openxmlformats.org/officeDocument/2006/math">
		<m:oMath>
			<m:r><m:t>a</m:t></m:r>
			<m:r>
				<m:rPr><m:aln/></m:rPr>
				<m:t>=b</m:t>
			</m:r>
			<m:r>
				<m:rPr><m:aln/></m:rPr>
				<m:t>=c</m:t>
			</m:r>
		</m:oMath>
		<m:oMath>
			<m:r><m:t>x</m:t></m:r>
			<m:r>
				<m:rPr><m:aln/></m:rPr>
				<m:t>=y</m:t>
			</m:r>
			<m:r>
				<m:rPr><m:aln/></m:rPr>
				<m:t>=z</m:t>
			</m:r>
		</m:oMath>
	</m:oMathPara>`
	got := Convert([]byte(input))
	if strings.Count(got, "<mtr>") != 2 {
		t.Errorf("expected 2 rows, got: %s", got)
	}
	if strings.Count(got, "<mtd>") != 6 {
		t.Errorf("expected 6 cells (3 per row), got: %s", got)
	}
}

func TestConvertAlignedEquationsNoAlignment(t *testing.T) {
	// Single equation without alignment should NOT become a table.
	input := `<m:oMathPara xmlns:m="http://schemas.openxmlformats.org/officeDocument/2006/math">
		<m:oMath>
			<m:r><m:t>x=1</m:t></m:r>
		</m:oMath>
	</m:oMathPara>`
	got := Convert([]byte(input))
	if strings.Contains(got, "<mtable>") {
		t.Errorf("expected no <mtable> for non-aligned para, got: %s", got)
	}
	if !strings.Contains(got, `<math display="block"`) {
		t.Errorf("expected block display, got: %s", got)
	}
}

func TestConvertAlignedEquationsNested(t *testing.T) {
	// Alignment point after a fraction.
	input := `<m:oMathPara xmlns:m="http://schemas.openxmlformats.org/officeDocument/2006/math">
		<m:oMath>
			<m:f>
				<m:num><m:r><m:t>a</m:t></m:r></m:num>
				<m:den><m:r><m:t>b</m:t></m:r></m:den>
			</m:f>
			<m:r>
				<m:rPr><m:aln/></m:rPr>
				<m:t>=c</m:t>
			</m:r>
		</m:oMath>
		<m:oMath>
			<m:r><m:t>x</m:t></m:r>
			<m:r>
				<m:rPr><m:aln/></m:rPr>
				<m:t>=y</m:t>
			</m:r>
		</m:oMath>
	</m:oMathPara>`
	got := Convert([]byte(input))
	if strings.Count(got, "<mtr>") != 2 {
		t.Errorf("expected 2 rows, got: %s", got)
	}
	if strings.Count(got, "<mtd>") != 4 {
		t.Errorf("expected 4 cells, got: %s", got)
	}
	if !strings.Contains(got, "<mfrac>") {
		t.Errorf("expected fraction in output, got: %s", got)
	}
}

func TestConvertFractionNoBar(t *testing.T) {
	input := `<m:oMath xmlns:m="http://schemas.openxmlformats.org/officeDocument/2006/math">
		<m:f>
			<m:fPr><m:type m:val="noBar"/></m:fPr>
			<m:num><m:r><m:t>a</m:t></m:r></m:num>
			<m:den><m:r><m:t>b</m:t></m:r></m:den>
		</m:f>
	</m:oMath>`
	got := Convert([]byte(input))
	if !strings.Contains(got, `<mfrac linethickness="0">`) {
		t.Errorf("expected no-bar fraction, got: %s", got)
	}
}

func TestConvertFractionBevelled(t *testing.T) {
	cases := []struct {
		fracType string
	}{
		{"lin"},
		{"skw"},
	}
	for _, tc := range cases {
		input := fmt.Sprintf(`<m:oMath xmlns:m="http://schemas.openxmlformats.org/officeDocument/2006/math">
			<m:f>
				<m:fPr><m:type m:val="%s"/></m:fPr>
				<m:num><m:r><m:t>a</m:t></m:r></m:num>
				<m:den><m:r><m:t>b</m:t></m:r></m:den>
			</m:f>
		</m:oMath>`, tc.fracType)
		got := Convert([]byte(input))
		if !strings.Contains(got, `<mfrac bevelled="true">`) {
			t.Errorf("expected bevelled fraction for type=%s, got: %s", tc.fracType, got)
		}
	}
}

func TestConvertEquationJustification(t *testing.T) {
	cases := []struct {
		jc       string
		wantWrap string
	}{
		{"left", `text-align:left`},
		{"right", `text-align:right`},
		{"center", ""},
	}
	for _, tc := range cases {
		input := fmt.Sprintf(`<m:oMathPara xmlns:m="http://schemas.openxmlformats.org/officeDocument/2006/math">
			<m:oMathParaPr><m:jc m:val="%s"/></m:oMathParaPr>
			<m:oMath><m:r><m:t>x=1</m:t></m:r></m:oMath>
		</m:oMathPara>`, tc.jc)
		got := Convert([]byte(input))
		if tc.wantWrap != "" {
			if !strings.Contains(got, tc.wantWrap) {
				t.Errorf("expected %s wrapper for jc=%s, got: %s", tc.wantWrap, tc.jc, got)
			}
			if !strings.HasPrefix(got, "<div") {
				t.Errorf("expected <div> wrapper for jc=%s, got: %s", tc.jc, got)
			}
		} else {
			if strings.HasPrefix(got, "<div") {
				t.Errorf("expected no wrapper for jc=center, got: %s", got)
			}
		}
		if !strings.Contains(got, `<math display="block"`) {
			t.Errorf("expected block display, got: %s", got)
		}
	}
}

func TestConvertMatrixColumnGap(t *testing.T) {
	input := `<m:oMath xmlns:m="http://schemas.openxmlformats.org/officeDocument/2006/math">
		<m:m>
			<m:mPr>
				<m:cGpRule m:val="4"/>
				<m:cGp m:val="3"/>
			</m:mPr>
			<m:mr>
				<m:e><m:r><m:t>a</m:t></m:r></m:e>
				<m:e><m:r><m:t>b</m:t></m:r></m:e>
			</m:mr>
			<m:mr>
				<m:e><m:r><m:t>c</m:t></m:r></m:e>
				<m:e><m:r><m:t>d</m:t></m:r></m:e>
			</m:mr>
		</m:m>
	</m:oMath>`
	got := Convert([]byte(input))
	if !strings.Contains(got, `columnspacing="3em"`) {
		t.Errorf("expected columnspacing=3em, got: %s", got)
	}
}

func TestConvertMatrixColumnGapRuleIgnored(t *testing.T) {
	// cGpRule values other than 3 or 4 should ignore cGp.
	input := `<m:oMath xmlns:m="http://schemas.openxmlformats.org/officeDocument/2006/math">
		<m:m>
			<m:mPr>
				<m:cGpRule m:val="1"/>
				<m:cGp m:val="5"/>
			</m:mPr>
			<m:mr>
				<m:e><m:r><m:t>a</m:t></m:r></m:e>
				<m:e><m:r><m:t>b</m:t></m:r></m:e>
			</m:mr>
		</m:m>
	</m:oMath>`
	got := Convert([]byte(input))
	if strings.Contains(got, `columnspacing`) {
		t.Errorf("expected no columnspacing for cGpRule=1, got: %s", got)
	}
}

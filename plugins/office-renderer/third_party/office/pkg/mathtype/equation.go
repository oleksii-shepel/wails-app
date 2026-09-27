// Package mathtype decodes MathType MTEF equations into a format-neutral
// semantic equation tree. Presentation code consumes MathML produced from this
// tree; it never interprets MTEF records itself.
package mathtype

import "fmt"

type Version uint8

const (
	MTEF3 Version = 3
	MTEF4 Version = 4
	MTEF5 Version = 5
)

type Header struct {
	Version                                              Version
	Platform, Product, ProductVersion, ProductSubversion byte
	Application                                          string
	Options                                              byte
}
type Offset struct{ X, Y int16 }
type SizeKind uint8

const (
	SizeFull SizeKind = iota
	SizeSub
	SizeSub2
	SizeSymbol
	SizeSubSymbol
	SizeUser1
	SizeUser2
)

type Size struct {
	Kind     SizeKind
	Delta    int16
	Points32 *int16
}
type Style struct {
	Typeface     int
	Font         string
	Bold, Italic bool
	Size         Size
	Nudge        *Offset
}

type Expr interface{ expr() }
type Row struct {
	Items []Expr
	Style Style
}

func (*Row) expr() {}

type Glyph struct {
	Rune           rune
	Style          Style
	Embellishments []Embellishment
}

func (*Glyph) expr() {}

type Fraction struct {
	Numerator, Denominator Expr
	Slash                  bool
	// Compact preserves the authored piece-fraction sizing. SlashVariant
	// preserves tmSLFRACT's normal/baseline/subscript-sized variation.
	Compact      bool
	SlashVariant byte
	Style        Style
}

func (*Fraction) expr() {}

type Script struct {
	Base, Sub, Sup Expr
	Leading        bool
	Style          Style
}

func (*Script) expr() {}

type Radical struct {
	Body, Degree Expr
	Style        Style
}

func (*Radical) expr() {}

type Fence struct {
	Body        Expr
	Left, Right rune
	Alignment   byte
	Style       Style
}

func (*Fence) expr() {}

type Accent struct {
	Body Expr
	Kind Embellishment
	// Operator is an authored template-owned accent/arrow character. When set,
	// it takes precedence over the selector-derived Kind fallback.
	Operator rune
	Under    bool
	Style    Style
}

func (*Accent) expr() {}

type LargeOperator struct {
	Operator rune
	// OperatorExpr retains the authored final BigOpBox slot when it is a
	// structured expression rather than a single operator glyph.
	OperatorExpr       Expr
	Body, Lower, Upper Expr
	Expand             bool
	LimitsAboveBelow   bool
	Style              Style
}

func (*LargeOperator) expr() {}

type UnderOver struct {
	Base, Annotation Expr
	Operator         rune
	Under            bool
	Style            Style
}

func (*UnderOver) expr() {}

type Limit struct {
	Body, Lower, Upper Expr
	Style              Style
}

func (*Limit) expr() {}

type LongDivision struct {
	Dividend, Quotient Expr
	Style              Style
}

func (*LongDivision) expr() {}

type Dirac struct {
	Left, Right                   Expr
	LeftFence, Middle, RightFence rune
	HasLeft, HasRight             bool
	Style                         Style
}

func (*Dirac) expr() {}

type Matrix struct {
	Rows, Cols int
	Cells      []Expr
	// VAlign positions the matrix in its containing line. HAlign and
	// CellVAlign are the horizontal/vertical justification of cell contents.
	HAlign, VAlign, CellVAlign byte
	RowRules, ColRules         []byte
	Style                      Style
}

func (*Matrix) expr() {}

type Pile struct {
	Lines          []Expr
	HAlign, VAlign byte
	Style          Style
}

func (*Pile) expr() {}

type Space struct{ Width string }

func (*Space) expr() {}

type Unsupported struct{ Selector, Variation byte }

func (*Unsupported) expr() {}

type Embellishment uint8

const (
	EmbellDot         Embellishment = 2
	EmbellDDot                      = 3
	EmbellTDot                      = 4
	EmbellPrime                     = 5
	EmbellDPrime                    = 6
	EmbellBackPrime                 = 7
	EmbellTilde                     = 8
	EmbellHat                       = 9
	EmbellNot                       = 10
	EmbellRightArrow                = 11
	EmbellLeftArrow                 = 12
	EmbellBothArrow                 = 13
	EmbellMidBar                    = 16
	EmbellOverBar                   = 17
	EmbellTriplePrime               = 18
	EmbellFrown                     = 19
	EmbellSmile                     = 20
)

type Equation struct {
	Header      Header
	Root        Expr
	InitialSize Size
	Fonts       map[int]FontDef
}
type FontDef struct {
	Name         string
	Bold, Italic bool
}

var ErrInvalidMTEF = fmt.Errorf("mathtype: invalid MTEF")
var ErrUnsupportedVersion = fmt.Errorf("mathtype: unsupported MTEF version")

// ErrNoVisibleContent means the MTEF stream is structurally valid but contains
// only framing/style state and no mathematical object worth rendering.
// Callers should preserve the authored OLE preview for this case.
var ErrNoVisibleContent = fmt.Errorf("mathtype: no visible equation content")

// HasVisibleContent reports whether e contains semantic content worth replacing
// the authored OLE preview with. Empty LINE/ROW records are valid MTEF framing,
// but must not activate an empty native-MathML object in presentation output.
func HasVisibleContent(e Expr) bool {
	if e == nil {
		return false
	}
	switch x := e.(type) {
	case *Row:
		for _, item := range x.Items {
			if HasVisibleContent(item) {
				return true
			}
		}
		return false
	case *Space:
		return false
	case *Unsupported:
		return false
	case *Glyph:
		return x.Rune != 0
	case *Fraction:
		return HasVisibleContent(x.Numerator) || HasVisibleContent(x.Denominator)
	case *Script:
		return HasVisibleContent(x.Base) || HasVisibleContent(x.Sub) || HasVisibleContent(x.Sup)
	case *Radical:
		return HasVisibleContent(x.Body) || HasVisibleContent(x.Degree)
	case *Fence:
		return HasVisibleContent(x.Body)
	case *Accent:
		return HasVisibleContent(x.Body)
	case *LargeOperator:
		return x.Operator != 0 || HasVisibleContent(x.OperatorExpr) || HasVisibleContent(x.Body) || HasVisibleContent(x.Lower) || HasVisibleContent(x.Upper)
	case *UnderOver:
		return x.Operator != 0 || HasVisibleContent(x.Base) || HasVisibleContent(x.Annotation)
	case *Limit:
		return HasVisibleContent(x.Body) || HasVisibleContent(x.Lower) || HasVisibleContent(x.Upper)
	case *LongDivision:
		return HasVisibleContent(x.Dividend) || HasVisibleContent(x.Quotient)
	case *Dirac:
		return HasVisibleContent(x.Left) || HasVisibleContent(x.Right)
	case *Matrix:
		for _, cell := range x.Cells {
			if HasVisibleContent(cell) {
				return true
			}
		}
		return false
	case *Pile:
		for _, line := range x.Lines {
			if HasVisibleContent(line) {
				return true
			}
		}
		return false
	default:
		return false
	}
}

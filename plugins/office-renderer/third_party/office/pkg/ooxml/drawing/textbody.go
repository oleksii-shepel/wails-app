package drawing

// TextBody represents a DrawingML text body (a:txBody).
// It holds the body layout properties and the ordered list of paragraphs.
type TextBody struct {
	BodyProps  TextBodyProperties
	Paragraphs []TextParagraph
}

// TextBodyProperties corresponds to a:bodyPr — controls layout of text within a shape.
type TextBodyProperties struct {
	// Wrap: "none" suppresses wrapping; "square" (default) wraps at shape edge.
	Wrap string
	// Anchor is vertical alignment: "t" (top), "ctr" (centre), "b" (bottom).
	Anchor string
	// AnchorCtr centers the body rotation anchor. HasAnchorCtr preserves an
	// explicitly authored false value across placeholder inheritance.
	AnchorCtr    bool
	HasAnchorCtr bool
	// Rot is the text rotation in thousandths of a degree (clockwise). HasRot
	// distinguishes an authored zero rotation from an omitted/inherited value.
	Rot    int
	HasRot bool
	// AutofitType: "noAutofit", "spAutoFit" (shape grows), "normAutofit" (font shrinks).
	AutofitType string
	// AutofitFontScale is the normAutofit font scale in thousandths of a percent.
	AutofitFontScale int
	// AutofitLineSpaceReduction is the normAutofit line spacing reduction in thousandths of a percent.
	AutofitLineSpaceReduction int
	// Vert controls vertical text direction: "horz" (default), "vert", "vert270".
	Vert string
	// Columns is the number of text columns (0 = unset = 1).
	Columns int
	// ColumnSpacingEMU is the gap between columns in EMU.
	ColumnSpacingEMU int64
	// Insets in EMU. OOXML defaults: T/B = 45720, L/R = 91440.
	InsetT, InsetB, InsetL, InsetR int64
}

// TextParagraph corresponds to a:p — one paragraph inside a text body.
type TextParagraph struct {
	Props ParagraphProperties
	Runs  []TextRun
}

// ParagraphProperties corresponds to a:pPr.
type ParagraphProperties struct {
	// Align: "l", "r", "ctr", "just", "dist", or "" (inherit).
	Align string
	// IndentEMU and MarginLeftEMU control indentation.
	IndentEMU     int64
	MarginLeftEMU int64
	// LineSpacing in hundredths of a point (positive) or percentage × 1000 (negative sentinel).
	LineSpacing int64
	// SpaceBefore and SpaceAfter in hundredths of a point.
	SpaceBefore int64
	SpaceAfter  int64
}

// TextRun corresponds to a:r — a contiguous run of identically-formatted text.
type TextRun struct {
	Text  string
	Props RunProperties
}

// RunProperties corresponds to a:rPr — character-level formatting for a run.
type RunProperties struct {
	Bold      bool
	Italic    bool
	Underline bool
	Strike    bool
	// FontSizePt is the font size in points (0 = inherit).
	FontSizePt float64
	// FontFamily is the typeface name, e.g. "Calibri".
	FontFamily string
	// Color is the foreground colour in sRGB hex, e.g. "FF0000".
	Color string
	// Lang is the BCP-47 language tag, e.g. "en-US".
	Lang string
}

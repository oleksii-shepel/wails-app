package edit

import (
	"strings"

	"office/pkg/ooxml/drawing"
	wordir "office/pkg/word/ir"
)

// Run wraps an IR run for editing.
type Run struct {
	para  *Paragraph
	index int
	irPtr *wordir.Run // non-nil when detached (cloned)
}

func (r *Run) ir() *wordir.Run {
	if r == nil {
		return nil
	}
	if r.irPtr != nil {
		return r.irPtr
	}
	if r.para == nil || r.para.ir == nil || r.index < 0 || r.index >= len(r.para.ir.Runs) {
		return nil
	}
	return &r.para.ir.Runs[r.index]
}

// SetText sets the run text.
func (r *Run) SetText(v string) {
	if ir := r.ir(); ir != nil {
		ir.Text = v
	}
}

// Text returns the run text.
func (r *Run) Text() string {
	if ir := r.ir(); ir != nil {
		return ir.Text
	}
	return ""
}

// SetStyleID assigns a character style to the run.
func (r *Run) SetStyleID(styleID string) {
	if ir := r.ir(); ir != nil {
		ir.StyleID = strings.TrimSpace(styleID)
	}
}

// StyleID returns the run's character style ID.
func (r *Run) StyleID() string {
	if ir := r.ir(); ir != nil {
		return ir.StyleID
	}
	return ""
}

// TextAt returns the text segment at index.
// The edit IR stores normalized runs with a single text segment, so index 0
// returns Text() and any other index returns an empty string.
func (r *Run) TextAt(index int) string {
	if index != 0 {
		return ""
	}
	return r.Text()
}

// SetBold sets the bold flag.
func (r *Run) SetBold(v bool) {
	if ir := r.ir(); ir != nil {
		ir.Format.Bold = &v
	}
}

// SetItalic sets the italic flag.
func (r *Run) SetItalic(v bool) {
	if ir := r.ir(); ir != nil {
		ir.Format.Italic = &v
	}
}

// SetUnderline sets the underline style.
func (r *Run) SetUnderline(v string) {
	if ir := r.ir(); ir != nil {
		ir.Format.Underline = v
	}
}

// SetStrike sets the strikethrough flag.
func (r *Run) SetStrike(v bool) {
	if ir := r.ir(); ir != nil {
		ir.Format.Strike = &v
	}
}

// SetFontSize sets the font size in half-points.
func (r *Run) SetFontSize(halfPoints int) {
	if ir := r.ir(); ir != nil {
		ir.Format.FontSize = &halfPoints
	}
}

// SetColor sets the font color as a hex string (e.g., "FF0000").
func (r *Run) SetColor(hex string) {
	if ir := r.ir(); ir != nil {
		ir.Format.Color = hex
	}
}

// SetHighlight sets the highlight color.
func (r *Run) SetHighlight(v string) {
	if ir := r.ir(); ir != nil {
		ir.Format.Highlight = v
	}
}

// SetFontFamily sets the ASCII font family.
func (r *Run) SetFontFamily(v string) {
	if ir := r.ir(); ir != nil {
		ir.Format.FontASCII = v
	}
}

// SetFontFamilyHAnsi sets the High-ANSI font family.
func (r *Run) SetFontFamilyHAnsi(v string) {
	if ir := r.ir(); ir != nil {
		ir.Format.FontHAnsi = v
	}
}

// SetCaps sets the all-caps flag.
func (r *Run) SetCaps(v bool) {
	if ir := r.ir(); ir != nil {
		ir.Format.Caps = &v
	}
}

// SetSmallCaps sets the small-caps flag.
func (r *Run) SetSmallCaps(v bool) {
	if ir := r.ir(); ir != nil {
		ir.Format.SmallCaps = &v
	}
}

// SetSubscript sets the subscript flag.
func (r *Run) SetSubscript(v bool) {
	if ir := r.ir(); ir != nil {
		if v {
			ir.Format.VertAlignVal = "subscript"
		} else if ir.Format.VertAlignVal == "subscript" {
			ir.Format.VertAlignVal = ""
		}
	}
}

// SetSuperscript sets the superscript flag.
func (r *Run) SetSuperscript(v bool) {
	if ir := r.ir(); ir != nil {
		if v {
			ir.Format.VertAlignVal = "superscript"
		} else if ir.Format.VertAlignVal == "superscript" {
			ir.Format.VertAlignVal = ""
		}
	}
}

// Bold returns whether the run is bold.
func (r *Run) Bold() bool {
	ir := r.ir()
	if ir == nil || ir.Format.Bold == nil {
		return false
	}
	return *ir.Format.Bold
}

// Italic returns whether the run is italic.
func (r *Run) Italic() bool {
	ir := r.ir()
	if ir == nil || ir.Format.Italic == nil {
		return false
	}
	return *ir.Format.Italic
}

// Underline returns the underline style.
func (r *Run) Underline() string {
	if ir := r.ir(); ir != nil {
		return ir.Format.Underline
	}
	return ""
}

// FontSize returns the font size in half-points, or 0 if unset.
func (r *Run) FontSize() int {
	ir := r.ir()
	if ir == nil || ir.Format.FontSize == nil {
		return 0
	}
	return *ir.Format.FontSize
}

// Color returns the font color hex string.
func (r *Run) Color() string {
	if ir := r.ir(); ir != nil {
		return ir.Format.Color
	}
	return ""
}

// FontFamily returns the ASCII font family.
func (r *Run) FontFamily() string {
	if ir := r.ir(); ir != nil {
		return ir.Format.FontASCII
	}
	return ""
}

// Strike returns whether the run has strikethrough.
func (r *Run) Strike() bool {
	ir := r.ir()
	if ir == nil || ir.Format.Strike == nil {
		return false
	}
	return *ir.Format.Strike
}

// DoubleStrike returns whether the run has double strikethrough.
func (r *Run) DoubleStrike() bool {
	ir := r.ir()
	if ir == nil || ir.Format.DoubleStrike == nil {
		return false
	}
	return *ir.Format.DoubleStrike
}

// Caps returns whether the run is in all caps.
func (r *Run) Caps() bool {
	ir := r.ir()
	if ir == nil || ir.Format.Caps == nil {
		return false
	}
	return *ir.Format.Caps
}

// SmallCaps returns whether the run is in small caps.
func (r *Run) SmallCaps() bool {
	ir := r.ir()
	if ir == nil || ir.Format.SmallCaps == nil {
		return false
	}
	return *ir.Format.SmallCaps
}

// Highlight returns the highlight color.
func (r *Run) Highlight() string {
	if ir := r.ir(); ir != nil {
		return ir.Format.Highlight
	}
	return ""
}

// Background returns the run background (shading) color.
func (r *Run) Background() string {
	if ir := r.ir(); ir != nil {
		return ir.Format.Background
	}
	return ""
}

// Subscript reports whether the run is subscript.
func (r *Run) Subscript() bool {
	if ir := r.ir(); ir != nil {
		return ir.Format.VertAlignVal == "subscript"
	}
	return false
}

// Superscript reports whether the run is superscript.
func (r *Run) Superscript() bool {
	if ir := r.ir(); ir != nil {
		return ir.Format.VertAlignVal == "superscript"
	}
	return false
}

// FontSizeCS returns the complex-script font size in half-points, or 0 if unset.
func (r *Run) FontSizeCS() int {
	ir := r.ir()
	if ir == nil || ir.Format.FontSizeCS == nil {
		return 0
	}
	return *ir.Format.FontSizeCS
}

// BoldCS returns the complex-script bold flag.
func (r *Run) BoldCS() bool {
	ir := r.ir()
	if ir == nil || ir.Format.BoldCS == nil {
		return false
	}
	return *ir.Format.BoldCS
}

// ItalicCS returns the complex-script italic flag.
func (r *Run) ItalicCS() bool {
	ir := r.ir()
	if ir == nil || ir.Format.ItalicCS == nil {
		return false
	}
	return *ir.Format.ItalicCS
}

// FontFamilyHAnsi returns the high-ANSI font family.
func (r *Run) FontFamilyHAnsi() string {
	if ir := r.ir(); ir != nil {
		return ir.Format.FontHAnsi
	}
	return ""
}

// FontFamilyEastAsia returns the East Asian font family.
func (r *Run) FontFamilyEastAsia() string {
	if ir := r.ir(); ir != nil {
		return ir.Format.FontEastAsia
	}
	return ""
}

// FontFamilyCS returns the complex-script font family.
func (r *Run) FontFamilyCS() string {
	if ir := r.ir(); ir != nil {
		return ir.Format.FontCS
	}
	return ""
}

// Vanish returns whether the run is hidden (vanished).
func (r *Run) Vanish() bool {
	ir := r.ir()
	if ir == nil || ir.Format.Vanish == nil {
		return false
	}
	return *ir.Format.Vanish
}

// NoProof returns whether the run is excluded from proofing.
func (r *Run) NoProof() bool {
	ir := r.ir()
	if ir == nil || ir.Format.NoProof == nil {
		return false
	}
	return *ir.Format.NoProof
}

// WebHidden returns whether the run is hidden in web layout.
func (r *Run) WebHidden() bool {
	ir := r.ir()
	if ir == nil || ir.Format.WebHidden == nil {
		return false
	}
	return *ir.Format.WebHidden
}

// Lang returns the run language tag.
func (r *Run) Lang() string {
	if ir := r.ir(); ir != nil {
		return ir.Format.Lang
	}
	return ""
}

// CharacterSpacing returns the character spacing in twentieths of a point, or 0 if unset.
func (r *Run) CharacterSpacing() int {
	ir := r.ir()
	if ir == nil || ir.Format.CharacterSpacing == nil {
		return 0
	}
	return *ir.Format.CharacterSpacing
}

// Kerning returns the kerning in half-points, or 0 if unset.
func (r *Run) Kerning() int {
	ir := r.ir()
	if ir == nil || ir.Format.Kerning == nil {
		return 0
	}
	return *ir.Format.Kerning
}

// CharScale returns the character scale percentage, or 0 if unset.
func (r *Run) CharScale() int {
	ir := r.ir()
	if ir == nil || ir.Format.CharScale == nil {
		return 0
	}
	return *ir.Format.CharScale
}

// Shadow returns whether the run has shadow.
func (r *Run) Shadow() bool {
	ir := r.ir()
	if ir == nil || ir.Format.Shadow == nil {
		return false
	}
	return *ir.Format.Shadow
}

// Outline returns whether the run has outline.
func (r *Run) Outline() bool {
	ir := r.ir()
	if ir == nil || ir.Format.Outline == nil {
		return false
	}
	return *ir.Format.Outline
}

// Emboss returns whether the run is embossed.
func (r *Run) Emboss() bool {
	ir := r.ir()
	if ir == nil || ir.Format.Emboss == nil {
		return false
	}
	return *ir.Format.Emboss
}

// Imprint returns whether the run is imprinted.
func (r *Run) Imprint() bool {
	ir := r.ir()
	if ir == nil || ir.Format.Imprint == nil {
		return false
	}
	return *ir.Format.Imprint
}

// RTL returns whether the run is right-to-left.
func (r *Run) RTL() bool {
	ir := r.ir()
	if ir == nil || ir.Format.RTL == nil {
		return false
	}
	return *ir.Format.RTL
}

// ComplexScript returns whether complex script formatting is forced.
func (r *Run) ComplexScript() bool {
	ir := r.ir()
	if ir == nil || ir.Format.ComplexScript == nil {
		return false
	}
	return *ir.Format.ComplexScript
}

// UnderlineColor returns the underline color.
func (r *Run) UnderlineColor() string {
	if ir := r.ir(); ir != nil {
		return ir.Format.UnderlineColor
	}
	return ""
}

// Ligatures returns the ligatures setting.
func (r *Run) Ligatures() string {
	if ir := r.ir(); ir != nil {
		return ir.Format.Ligatures
	}
	return ""
}

// NumberSpacing returns the number spacing setting.
func (r *Run) NumberSpacing() string {
	if ir := r.ir(); ir != nil {
		return ir.Format.NumberSpacing
	}
	return ""
}

// StylisticSets returns the enabled stylistic set IDs.
func (r *Run) StylisticSets() []int {
	if ir := r.ir(); ir != nil && len(ir.Format.StylisticSets) > 0 {
		out := make([]int, len(ir.Format.StylisticSets))
		copy(out, ir.Format.StylisticSets)
		return out
	}
	return nil
}

// SetHyperlink marks the run as a hyperlink to the given URL.
func (r *Run) SetHyperlink(url string) {
	if ir := r.ir(); ir != nil {
		ir.Kind = "hyperlink"
		ir.Target = url
	}
}

// Hyperlink returns the hyperlink URL, or empty if none.
func (r *Run) Hyperlink() string {
	if ir := r.ir(); ir != nil {
		return ir.Target
	}
	return ""
}

// SetAnchorBookmark marks the run as linking to an internal bookmark.
func (r *Run) SetAnchorBookmark(name string) {
	if ir := r.ir(); ir != nil {
		ir.Kind = "hyperlink"
		ir.Anchor = name
	}
}

// AnchorBookmark returns the internal bookmark name, or empty if none.
func (r *Run) AnchorBookmark() string {
	if ir := r.ir(); ir != nil {
		return ir.Anchor
	}
	return ""
}

// SetContentControl attaches a content control (SDT) to this run.
// Pass nil to remove.
func (r *Run) SetContentControl(cc *wordir.ContentControl) {
	if ir := r.ir(); ir != nil {
		ir.ContentControl = cc
	}
}

// ContentControl returns the content control attached to this run, or nil.
func (r *Run) ContentControl() *wordir.ContentControl {
	if ir := r.ir(); ir != nil {
		return ir.ContentControl
	}
	return nil
}

// FormField returns the legacy form field attached to this run, or nil.
func (r *Run) FormField() *FormField {
	if ir := r.ir(); ir != nil && ir.FormField != nil {
		return &FormField{ir: ir.FormField}
	}
	return nil
}

// SetField sets a generic field instruction on the run (e.g. "PAGE", "DATE",
// "DOCPROPERTY Title"). It also extracts the field kind from the instruction.
func (r *Run) SetField(instr string) {
	if ir := r.ir(); ir != nil {
		ir.FieldInstr = instr
		ir.FieldKind = extractFieldKind(instr)
	}
}

// FieldInstr returns the raw field instruction text.
func (r *Run) FieldInstr() string {
	if ir := r.ir(); ir != nil {
		return ir.FieldInstr
	}
	return ""
}

// SetPageNumber marks the run as a page-number field.
func (r *Run) SetPageNumber() {
	r.SetField("PAGE")
}

// SetNumPages marks the run as a total-pages field.
func (r *Run) SetNumPages() {
	r.SetField("NUMPAGES")
}

// SetDate marks the run as a date field.
func (r *Run) SetDate() {
	r.SetField("DATE")
}

// SetDocProperty marks the run as a document-property field (e.g. "Title",
// "Author", "Subject").
func (r *Run) SetDocProperty(name string) {
	if ir := r.ir(); ir != nil {
		ir.FieldInstr = "DOCPROPERTY " + name
		ir.FieldArg = name
	}
}

// SetDoubleStrike toggles double strikethrough.
func (r *Run) SetDoubleStrike(v bool) {
	if ir := r.ir(); ir != nil {
		ir.Format.DoubleStrike = &v
	}
}

// SetUnderlineColor sets the underline color as a 6-digit hex string.
func (r *Run) SetUnderlineColor(hex string) {
	if ir := r.ir(); ir != nil {
		ir.Format.UnderlineColor = hex
	}
}

// SetBackground sets the run background shading as a 6-digit hex string.
func (r *Run) SetBackground(hex string) {
	if ir := r.ir(); ir != nil {
		ir.Format.Background = hex
	}
}

// SetFontSizeCS sets the complex-script font size in half-points.
func (r *Run) SetFontSizeCS(halfPoints int) {
	if ir := r.ir(); ir != nil {
		ir.Format.FontSizeCS = &halfPoints
	}
}

// SetBoldCS toggles complex-script bold.
func (r *Run) SetBoldCS(v bool) {
	if ir := r.ir(); ir != nil {
		ir.Format.BoldCS = &v
	}
}

// SetItalicCS toggles complex-script italic.
func (r *Run) SetItalicCS(v bool) {
	if ir := r.ir(); ir != nil {
		ir.Format.ItalicCS = &v
	}
}

// SetFontEastAsia sets the East Asian font name.
func (r *Run) SetFontEastAsia(v string) {
	if ir := r.ir(); ir != nil {
		ir.Format.FontEastAsia = v
	}
}

// SetFontCS sets the complex-script font name.
func (r *Run) SetFontCS(v string) {
	if ir := r.ir(); ir != nil {
		ir.Format.FontCS = v
	}
}

// SetVanish hides the run (hidden text).
func (r *Run) SetVanish(v bool) {
	if ir := r.ir(); ir != nil {
		ir.Format.Vanish = &v
	}
}

// SetNoProof excludes this run from proofing.
func (r *Run) SetNoProof(v bool) {
	if ir := r.ir(); ir != nil {
		ir.Format.NoProof = &v
	}
}

// SetWebHidden hides this run in Web Layout view.
func (r *Run) SetWebHidden(v bool) {
	if ir := r.ir(); ir != nil {
		ir.Format.WebHidden = &v
	}
}

// SetLang sets the language hint for this run (e.g., "en-US", "fr-FR").
func (r *Run) SetLang(v string) {
	if ir := r.ir(); ir != nil {
		ir.Format.Lang = v
	}
}

// SetCharacterSpacing sets character spacing in twentieths of a point
// (positive = expanded, negative = condensed).
func (r *Run) SetCharacterSpacing(twentieths int) {
	if ir := r.ir(); ir != nil {
		ir.Format.CharacterSpacing = &twentieths
	}
}

// SetFitTextWidth sets the width (in twips) that the run text should be scaled to fit into.
// 0 disables fit-text scaling.
func (r *Run) SetFitTextWidth(twips int) {
	if ir := r.ir(); ir != nil {
		if twips > 0 {
			ir.Format.FitTextWidth = &twips
		} else {
			ir.Format.FitTextWidth = nil
		}
	}
}

// SetKerning sets the minimum font size (in half-points) above which kerning is applied.
func (r *Run) SetKerning(halfPoints int) {
	if ir := r.ir(); ir != nil {
		ir.Format.Kerning = &halfPoints
	}
}

// SetCharScale sets horizontal character scaling as a percentage of normal width.
// 100 is normal.
func (r *Run) SetCharScale(percent int) {
	if ir := r.ir(); ir != nil {
		ir.Format.CharScale = &percent
	}
}

// SetShadow toggles the simple text shadow effect (w:shadow).
func (r *Run) SetShadow(v bool) {
	if ir := r.ir(); ir != nil {
		ir.Format.Shadow = &v
	}
}

// SetOutline toggles the simple text outline effect (w:outline).
func (r *Run) SetOutline(v bool) {
	if ir := r.ir(); ir != nil {
		ir.Format.Outline = &v
	}
}

// SetEmboss toggles the embossed text effect (w:emboss).
func (r *Run) SetEmboss(v bool) {
	if ir := r.ir(); ir != nil {
		ir.Format.Emboss = &v
	}
}

// SetImprint toggles the imprint / engraved text effect (w:imprint).
func (r *Run) SetImprint(v bool) {
	if ir := r.ir(); ir != nil {
		ir.Format.Imprint = &v
	}
}

// SetTextShadow sets a Word 2010 text effect shadow.
func (r *Run) SetTextShadow(color string, blur, dist float64) {
	if ir := r.ir(); ir != nil {
		ir.Format.TextShadow = &drawing.Shadow{
			Color:    color,
			Blur:     blur,
			Distance: dist,
		}
	}
}

// SetTextOutline sets a Word 2010 text effect outline.
func (r *Run) SetTextOutline(color string, width float64) {
	if ir := r.ir(); ir != nil {
		ir.Format.TextOutline = &drawing.LineStyle{
			Color: color,
			Width: width,
		}
	}
}

// SetRTL sets the right-to-left flag for this run.
func (r *Run) SetRTL(v bool) {
	if ir := r.ir(); ir != nil {
		ir.Format.RTL = &v
	}
}

// SetBorder sets a run border.
// val is the border style (e.g., "single", "double").
// color is a 6-digit hex string or "auto".
// sizePt is the border width in points.
// spacePt is the padding between border and text in points.
func (r *Run) SetBorder(val, color string, sizePt, spacePt float64) {
	if ir := r.ir(); ir != nil {
		if ir.Format.Borders == nil {
			ir.Format.Borders = &wordir.RunBorders{}
		}
		b := &wordir.BorderSide{Val: val, SizePt: sizePt, Color: color, SpacePt: spacePt}
		ir.Format.Borders.Top = b
		ir.Format.Borders.Left = b
		ir.Format.Borders.Bottom = b
		ir.Format.Borders.Right = b
	}
}

// ClearBorder removes the run border.
func (r *Run) ClearBorder() {
	if ir := r.ir(); ir != nil {
		ir.Format.Borders = nil
	}
}

// SetEastAsianLayout sets East Asian text layout options.
func (r *Run) SetEastAsianLayout(combine, vert, vertCompress bool) {
	if ir := r.ir(); ir != nil {
		ir.Format.FELayout = &wordir.FELayout{
			CombineCharacters:  combine,
			Vertical:           vert,
			VerticalCompressed: vertCompress,
		}
	}
}

// SetTextFill sets a solid text fill color.
func (r *Run) SetTextFill(color string) {
	if ir := r.ir(); ir != nil {
		ir.Format.TextFill = drawing.SolidFill{Color: color, Alpha: 1.0}
	}
}

// SetTextGlow sets a Word 2010 text glow effect.
func (r *Run) SetTextGlow(color string, radius float64) {
	if ir := r.ir(); ir != nil {
		ir.Format.TextGlow = &drawing.Glow{
			Color:  color,
			Radius: radius,
		}
	}
}

// SetTextSoftEdge sets a Word 2010 text soft edge effect.
func (r *Run) SetTextSoftEdge(radius float64) {
	if ir := r.ir(); ir != nil {
		ir.Format.TextSoftEdge = &drawing.SoftEdge{Radius: radius}
	}
}

// SetTextReflection sets a Word 2010 text reflection effect.
func (r *Run) SetTextReflection(alpha, blur, distance, size float64) {
	if ir := r.ir(); ir != nil {
		ir.Format.TextReflection = &drawing.Reflection{
			Alpha:    alpha,
			Blur:     blur,
			Distance: distance,
			Size:     size,
		}
	}
}

// SetLigatures sets the ligature setting (e.g., "none", "standard", "contextual").
func (r *Run) SetLigatures(v string) {
	if ir := r.ir(); ir != nil {
		ir.Format.Ligatures = v
	}
}

// SetNumberSpacing sets the number spacing setting (e.g., "default", "proportional", "tabular").
func (r *Run) SetNumberSpacing(v string) {
	if ir := r.ir(); ir != nil {
		ir.Format.NumberSpacing = v
	}
}

// SetStylisticSets enables the given stylistic set IDs.
func (r *Run) SetStylisticSets(ids []int) {
	if ir := r.ir(); ir != nil {
		ir.Format.StylisticSets = append([]int(nil), ids...)
	}
}

// SetComplexScript marks this run as complex script.
func (r *Run) SetComplexScript(v bool) {
	if ir := r.ir(); ir != nil {
		ir.Format.ComplexScript = &v
	}
}

// extractFieldKind extracts the field kind from a field instruction string.
// For example, "PAGE" → "PAGE", "DOCPROPERTY Title" → "DOCPROPERTY".
func extractFieldKind(instr string) string {
	instr = strings.TrimSpace(instr)
	if i := strings.IndexAny(instr, " \\\""); i > 0 {
		return strings.ToUpper(instr[:i])
	}
	if instr != "" {
		return strings.ToUpper(instr)
	}
	return ""
}

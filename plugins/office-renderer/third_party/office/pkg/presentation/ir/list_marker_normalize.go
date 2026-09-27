package ir

import "strings"

// NormalizeDuplicateBulletGlyphShapes removes drawing primitives that duplicate
// a paragraph's semantic list marker. Older PowerPoint producers can persist a
// bullet twice: once as the paragraph marker and once as a tiny filled ellipse.
// The paragraph marker is the semantic owner. This normalization is format-
// neutral because both binary PPT and OOXML PPTX can carry the duplicate paint.
func NormalizeDuplicateBulletGlyphShapes(p *Presentation) {
	if p == nil {
		return
	}
	for _, slide := range p.Slides {
		if slide != nil {
			slide.Shapes = NormalizeDuplicateBulletGlyphShapesInList(slide.Shapes)
		}
	}
}

// NormalizeDuplicateBulletGlyphShapesInSlides is the slide-list counterpart
// used by streaming OOXML entry points that materialize slides without a full
// Presentation wrapper.
func NormalizeDuplicateBulletGlyphShapesInSlides(slides []*Slide) {
	for _, slide := range slides {
		if slide != nil {
			slide.Shapes = NormalizeDuplicateBulletGlyphShapesInList(slide.Shapes)
		}
	}
}

// NormalizeDuplicateBulletGlyphShapesInList normalizes one visual container.
// Text can live inside a sibling group while the duplicate ellipse is a direct
// child of the enclosing group, so matching considers text descendants of the
// whole container rather than direct siblings only.
func NormalizeDuplicateBulletGlyphShapesInList(shapes []*Shape) []*Shape {
	for _, shape := range shapes {
		if shape != nil && len(shape.Children) > 0 {
			shape.Children = NormalizeDuplicateBulletGlyphShapesInList(shape.Children)
		}
	}

	texts := collectBulletTextDescendants(shapes)
	out := shapes[:0]
	for _, shape := range shapes {
		if shape == nil || !duplicateBulletEllipse(shape, texts) {
			out = append(out, shape)
		}
	}
	return out
}

func collectBulletTextDescendants(shapes []*Shape) []*Shape {
	var out []*Shape
	var walk func(*Shape)
	walk = func(shape *Shape) {
		if shape == nil {
			return
		}
		if len(shape.Paragraphs) > 0 {
			out = append(out, shape)
		}
		for _, child := range shape.Children {
			walk(child)
		}
	}
	for _, shape := range shapes {
		walk(shape)
	}
	return out
}

func duplicateBulletEllipse(candidate *Shape, texts []*Shape) bool {
	if candidate == nil || candidate.Geometry != "ellipse" || candidate.FillColor == "" ||
		shapeHasVisibleParagraphText(candidate) || len(candidate.Children) != 0 ||
		candidate.Bounds.W <= 0 || candidate.Bounds.H <= 0 {
		return false
	}
	w, h := float64(candidate.Bounds.W), float64(candidate.Bounds.H)
	if w/h < 0.5 || w/h > 2.0 {
		return false
	}

	for _, text := range texts {
		if text == nil || text == candidate || text.Bounds.W <= 0 || text.Bounds.H <= 0 {
			continue
		}
		if candidate.Bounds.W*10 > text.Bounds.W || candidate.Bounds.H*10 > text.Bounds.H {
			continue
		}
		centerY := candidate.Bounds.Y + candidate.Bounds.H/2
		if centerY < text.Bounds.Y || centerY > text.Bounds.Y+text.Bounds.H {
			continue
		}
		xTolerance := text.Bounds.W / 50
		if candidate.Bounds.W*2 > xTolerance {
			xTolerance = candidate.Bounds.W * 2
		}
		dx := candidate.Bounds.X - text.Bounds.X
		if dx < 0 {
			dx = -dx
		}
		if dx > xTolerance {
			continue
		}

		for _, paragraph := range text.Paragraphs {
			if paragraph == nil || paragraph.BulletNone || (paragraph.BulletChar == "" && !paragraph.AutoNum) || !paragraphHasVisibleText(paragraph) {
				continue
			}
			markerColor := effectiveParagraphMarkerColor(paragraph)
			if markerColor == "" || !strings.EqualFold(markerColor, candidate.FillColor) {
				continue
			}
			// Some PPTX producers leave buClr implicit while also materializing
			// the marker as a shape. In that case the ellipse is reliable evidence
			// of the effective marker paint; keep canonical marker paint aligned
			// with the text/duplicate shape before discarding the redundant glyph.
			if !paragraph.BulletColorExplicit {
				paragraph.BulletColor = candidate.FillColor
			}
			return true
		}
	}
	return false
}

func shapeHasVisibleParagraphText(shape *Shape) bool {
	if shape == nil {
		return false
	}
	for _, paragraph := range shape.Paragraphs {
		if paragraphHasVisibleText(paragraph) {
			return true
		}
	}
	return false
}

func effectiveParagraphMarkerColor(p *Paragraph) string {
	if p == nil {
		return ""
	}
	if p.BulletColorExplicit && p.BulletColor != "" {
		return p.BulletColor
	}
	for _, run := range p.Runs {
		if run != nil && strings.TrimSpace(run.Text) != "" && run.Color != "" {
			return run.Color
		}
	}
	return p.BulletColor
}

func paragraphHasVisibleText(p *Paragraph) bool {
	for _, run := range p.Runs {
		if run != nil && strings.TrimSpace(run.Text) != "" {
			return true
		}
	}
	return false
}

// NormalizeLegacyListMarkerGlyphs converts legacy symbol-font marker codes into
// stable Unicode list-marker semantics at the canonical IR boundary. Office
// formats can persist Wingdings/Symbol bullets as ASCII or private-use code
// points whose visual meaning depends on the original font. Keeping those raw
// codes in IR leaks storage encoding into layout/HTML and produces visible
// letters when the legacy font is unavailable. Once mapped, the Unicode marker
// owns its visual identity and follows the paragraph text font.
func NormalizeLegacyListMarkerGlyphs(p *Presentation) {
	if p == nil {
		return
	}
	NormalizeLegacyListMarkerGlyphsInSlides(p.Slides)
}

// NormalizeLegacyListMarkerGlyphsInSlides is the slide-list counterpart used by
// streaming OOXML entry points.
func NormalizeLegacyListMarkerGlyphsInSlides(slides []*Slide) {
	var walk func([]*Shape)
	walk = func(shapes []*Shape) {
		for _, shape := range shapes {
			if shape == nil {
				continue
			}
			for _, paragraph := range shape.Paragraphs {
				normalizeLegacyListMarkerGlyph(paragraph)
			}
			walk(shape.Children)
		}
	}
	for _, slide := range slides {
		if slide != nil {
			walk(slide.Shapes)
		}
	}
}

func normalizeLegacyListMarkerGlyph(p *Paragraph) {
	if p == nil || p.BulletChar == "" || p.BulletNone || p.AutoNum {
		return
	}
	font := strings.ToLower(strings.TrimSpace(p.BulletFontFamily))
	marker := p.BulletChar
	mapped := ""
	if strings.Contains(font, "wingdings") || strings.Contains(font, "webdings") {
		switch marker {
		case "n", "\uf06e", "\uf0a7":
			mapped = "▪"
		case "w", "\uf077", "\uf076":
			mapped = "◆"
		case "l", "\uf06c":
			mapped = "●"
		case "\uf0a8":
			mapped = "□"
		case "\uf0d8":
			mapped = "➢"
		case "\uf0fc":
			mapped = "✓"
		}
	} else if strings.Contains(font, "symbol") {
		switch marker {
		case "\uf0b7", "·":
			mapped = "•"
		}
	}
	if mapped == "" {
		return
	}
	p.BulletChar = mapped
	// The marker is now Unicode semantic content rather than a glyph index into
	// a legacy symbol font. Let it follow the effective paragraph text font.
	p.BulletFontFamily = ""
	p.BulletFontTx = true
}

package render

import (
	"encoding/base64"
	"fmt"
	"html"
	"strings"

	"office/pkg/officeart/ir"
	"office/pkg/render/textsafe"
)

// HTMLShape renders one shared OfficeArt node. It is deliberately small in
// Phase 1: vector path fidelity will move here in the next integration pass.
func HTMLShape(shape *ir.Shape) (string, error) {
	if err := shape.Validate(); err != nil {
		return "", err
	}
	var b strings.Builder
	renderShape(&b, shape)
	return b.String(), nil
}

func renderShape(b *strings.Builder, shape *ir.Shape) {
	fmt.Fprintf(b, `<div class="officeart-shape" data-officeart-id="%d" data-officeart-kind="%s" style="%s">`,
		shape.ID, html.EscapeString(string(shape.Kind)), html.EscapeString(StyleString(ShapeStyle(shape))))

	renderVectorShape(b, shape)

	if shape.Image != nil {
		src := shape.Image.URL
		if src == "" && len(shape.Image.Data) > 0 && shape.Image.MIMEType != "" {
			src = "data:" + shape.Image.MIMEType + ";base64," + base64.StdEncoding.EncodeToString(shape.Image.Data)
		}
		if src != "" {
			style := imageStyle(shape.Image.Crop)
			if hasImageCrop(shape.Image.Crop) {
				b.WriteString(`<div class="officeart-image-crop" style="position:absolute;inset:0;overflow:hidden">`)
				fmt.Fprintf(b, `<img class="officeart-image" src="%s" alt="" style="%s">`, html.EscapeString(src), html.EscapeString(style))
				b.WriteString(`</div>`)
			} else {
				fmt.Fprintf(b, `<img class="officeart-image" src="%s" alt="" style="%s">`, html.EscapeString(src), html.EscapeString(style))
			}
		}
	}

	if shape.Text != nil {
		fmt.Fprintf(b, `<div class="officeart-text" style="%s">`, html.EscapeString(StyleString(TextStyle(shape.Text))))
		for _, p := range shape.Text.Paragraphs {
			paragraphStyle := officeArtParagraphStyle(p)
			if paragraphStyle == "" {
				b.WriteString(`<div class="officeart-p">`)
			} else {
				fmt.Fprintf(b, `<div class="officeart-p" style="%s">`, html.EscapeString(paragraphStyle))
			}
			for _, run := range p.Runs {
				text := html.EscapeString(textsafe.Visible(run.Text))
				if style := officeArtTextRunStyle(run); style != "" {
					fmt.Fprintf(b, `<span class="officeart-run" style="%s">%s</span>`,
						html.EscapeString(style), text)
				} else {
					b.WriteString(text)
				}
			}
			b.WriteString(`</div>`)
		}
		b.WriteString(`</div>`)
	}

	for _, child := range shape.Children {
		if child != nil {
			renderShape(b, child)
		}
	}
	b.WriteString(`</div>`)
}

func officeArtParagraphStyle(p ir.Paragraph) string {
	if p.Align == "" {
		return ""
	}
	align := p.Align
	switch align {
	case "ctr":
		align = "center"
	case "r":
		align = "right"
	case "just":
		align = "justify"
	}
	return "text-align:" + align
}

func officeArtTextRunStyle(run ir.TextRun) string {
	var styles []string
	if run.FontFamily != "" {
		styles = append(styles, fmt.Sprintf("font-family:%q", run.FontFamily))
	}
	if run.FontSizePt > 0 {
		styles = append(styles, fmt.Sprintf("font-size:%.2fpt", run.FontSizePt))
	}
	if run.Bold {
		styles = append(styles, "font-weight:bold")
	}
	if run.Italic {
		styles = append(styles, "font-style:italic")
	}
	if run.Underline {
		styles = append(styles, "text-decoration:underline")
	}
	if run.Color != "" {
		color := strings.TrimPrefix(run.Color, "#")
		styles = append(styles, "color:#"+color)
	}
	return strings.Join(styles, ";")
}

func hasImageCrop(crop ir.Crop) bool {
	return crop.Left != 0 || crop.Top != 0 || crop.Right != 0 || crop.Bottom != 0
}

func imageStyle(crop ir.Crop) string {
	visibleW := 1 - crop.Left - crop.Right
	visibleH := 1 - crop.Top - crop.Bottom
	if visibleW <= 0 || visibleH <= 0 {
		return "width:100%;height:100%;display:block"
	}
	if !hasImageCrop(crop) {
		return "width:100%;height:100%;display:block"
	}
	return fmt.Sprintf(
		"position:absolute;width:%.6f%%;height:%.6f%%;left:%.6f%%;top:%.6f%%;max-width:none;display:block",
		100/visibleW, 100/visibleH, -100*crop.Left/visibleW, -100*crop.Top/visibleH,
	)
}

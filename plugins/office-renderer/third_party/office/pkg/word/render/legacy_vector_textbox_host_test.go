package render

import (
	"bytes"
	"strings"
	"testing"

	dml "office/pkg/ooxml/drawing"
	"office/pkg/word/parsers/ooxml/resolve"
)

func TestVectorShapeTextIsInsideSamePositionedHost(t *testing.T) {
	shape := &resolve.Shape{
		Kind:   "rect",
		X:      914400,
		Y:      914400,
		Width:  1828800,
		Height: 914400,
		Geometry: &dml.PresetGeometry{
			Name: "rect",
		},
		TextBox: &resolve.TextBox{
			Blocks: []resolve.Block{{
				Kind: "paragraph",
				Paragraph: &resolve.Paragraph{
					Runs: []resolve.Run{{Text: "inside"}},
				},
			}},
		},
	}

	var out bytes.Buffer
	r := &htmlRenderer{doc: &resolve.Document{}}
	r.renderShapeWithStyleAttrs(&out, shape, []string{"position:absolute", "left:10pt", "top:20pt"}, 144, 72)
	html := out.String()

	host := strings.Index(html, `class="shape-vector-host"`)
	text := strings.Index(html, `class="shape-text"`)
	closeHost := strings.LastIndex(html, `</div>`)
	if host < 0 || text < 0 || closeHost < 0 || !(host < text && text < closeHost) {
		t.Fatalf("shape text is not nested in vector host: %s", html)
	}
	if strings.Count(html, `class="shape-text"`) != 1 {
		t.Fatalf("shape text rendered more than once: %s", html)
	}
}

func TestVectorShapeHostOwnsRotation(t *testing.T) {
	shape := &resolve.Shape{
		Kind:        "rect",
		Width:       1828800,
		Height:      914400,
		RotationDeg: 90,
		Geometry:    &dml.PresetGeometry{Name: "rect"},
		TextBox:     &resolve.TextBox{},
	}
	var out bytes.Buffer
	r := &htmlRenderer{doc: &resolve.Document{}}
	r.renderShapeWithStyleAttrs(&out, shape, []string{"position:absolute"}, 144, 72)
	html := out.String()

	hostStart := strings.Index(html, `class="shape-vector-host"`)
	svgStart := strings.Index(html, `class="shape__vector"`)
	if hostStart < 0 || svgStart < 0 {
		t.Fatalf("missing host/svg: %s", html)
	}
	hostFragment := html[hostStart:svgStart]
	if !strings.Contains(hostFragment, "rotate(90.00deg)") {
		t.Fatalf("rotation is not owned by host: %s", html)
	}
}

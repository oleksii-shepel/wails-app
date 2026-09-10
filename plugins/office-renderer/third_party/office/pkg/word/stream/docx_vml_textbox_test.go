package stream

import (
	"testing"
	wordsax "office/pkg/word/parsers/ooxml/sax"
)

func TestDOCXVMLPictRecoversTextBoxContent(t *testing.T) {
	raw := wordsax.Event{Type: wordsax.EventDrawing, DrawingKind: "anchor", DrawingGraphicKind: "shape",
		RawXML: `<w:pict xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:v="urn:schemas-microsoft-com:vml"><v:shape style="position:absolute;margin-left:12pt;margin-top:24pt;width:100pt;height:40pt"><v:textbox><w:txbxContent><w:p><w:r><w:t>VISIBLE</w:t></w:r></w:p></w:txbxContent></v:textbox></v:shape></w:pict>`}
	shape, err := docxShapeFromDrawing(raw)
	if err != nil { t.Fatal(err) }
	if shape == nil || shape.TextBox == nil || len(shape.TextBox.Blocks) != 1 { t.Fatalf("shape/textbox = %#v", shape) }
	p := shape.TextBox.Blocks[0].Paragraph
	if p == nil || p.Text != "VISIBLE" { t.Fatalf("textbox paragraph = %#v", p) }
	if shape.Width != 1270000 || shape.Height != 508000 { t.Fatalf("extent = %dx%d", shape.Width, shape.Height) }
	if !shape.Floating || shape.X != 152400 || shape.Y != 304800 { t.Fatalf("position = floating:%v x:%d y:%d", shape.Floating, shape.X, shape.Y) }
}

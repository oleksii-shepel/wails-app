package stream

import (
	"testing"

	wordsax "office/pkg/word/parsers/ooxml/sax"
)

func TestDOCXDrawingCarriesShapeFromRawTextBoxWhenTypedKindIsUnavailable(t *testing.T) {
	raw := wordsax.Event{
		DrawingGraphicKind: "http://schemas.microsoft.com/office/word/2010/wordprocessingshape",
		RawXML: `<w:drawing xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:txbxContent><w:p><w:r><w:t>VISIBLE</w:t></w:r></w:p></w:txbxContent></w:drawing>`,
	}
	if !docxDrawingCarriesShape(raw) {
		t.Fatal("textbox-bearing DrawingML must be routed to the shape decoder")
	}
}

func TestDOCXShapeDecoderAcceptsWordprocessingTextBoxWithoutTypedShapeKind(t *testing.T) {
	raw := wordsax.Event{
		DrawingKind: "anchor",
		DrawingGraphicKind: "http://schemas.microsoft.com/office/word/2010/wordprocessingshape",
		RawXML: `<w:drawing xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:txbxContent><w:p><w:r><w:t>VISIBLE</w:t></w:r></w:p></w:txbxContent></w:drawing>`,
	}
	shape, err := docxShapeFromDrawing(raw)
	if err != nil {
		t.Fatal(err)
	}
	if shape == nil || shape.TextBox == nil || len(shape.TextBox.Blocks) == 0 {
		t.Fatalf("shape/textbox not recovered: %#v", shape)
	}
}

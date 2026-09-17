package parts

import (
	"encoding/xml"
	"testing"
)

func TestSpreadsheetDrawingPreservesInterleavedAnchorDocumentOrder(t *testing.T) {
	const src = `<xdr:wsDr xmlns:xdr="http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
      <xdr:twoCellAnchor><xdr:from/><xdr:to/><xdr:clientData/></xdr:twoCellAnchor>
      <xdr:absoluteAnchor><xdr:pos x="1" y="2"/><xdr:ext cx="3" cy="4"/><xdr:clientData/></xdr:absoluteAnchor>
      <xdr:oneCellAnchor><xdr:from/><xdr:ext cx="5" cy="6"/><xdr:clientData/></xdr:oneCellAnchor>
    </xdr:wsDr>`
	var got SpreadsheetDrawing
	if err := xml.Unmarshal([]byte(src), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.TwoCellAnchors) != 1 || got.TwoCellAnchors[0].DocumentOrder != 0 {
		t.Fatalf("two-cell order = %#v", got.TwoCellAnchors)
	}
	if len(got.AbsoluteAnchors) != 1 || got.AbsoluteAnchors[0].DocumentOrder != 1 {
		t.Fatalf("absolute order = %#v", got.AbsoluteAnchors)
	}
	if len(got.OneCellAnchors) != 1 || got.OneCellAnchors[0].DocumentOrder != 2 {
		t.Fatalf("one-cell order = %#v", got.OneCellAnchors)
	}
}

package render

import (
	"office/pkg/ooxml/drawing"
	"testing"
)

func TestDrawingShapePreservesSchemeTint(t *testing.T) {
	xml := []byte(`<dsp:drawing xmlns:dsp="http://schemas.microsoft.com/office/drawing/2008/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><dsp:spTree><dsp:sp modelId="circle"><dsp:nvSpPr><dsp:cNvPr id="1"/></dsp:nvSpPr><dsp:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="100" cy="100"/></a:xfrm><a:prstGeom prst="ellipse"><a:avLst/></a:prstGeom><a:solidFill><a:schemeClr val="accent1"><a:tint val="50000"/></a:schemeClr></a:solidFill></dsp:spPr></dsp:sp></dsp:spTree></dsp:drawing>`)
	infos := ExtractShapeInfos(xml, map[string]string{"accent1": "00A0C6"})
	sf, ok := infos["circle"].Fill.(drawing.SolidFill)
	if !ok {
		t.Fatalf("fill = %#v", infos["circle"].Fill)
	}
	if !sf.HasTint || sf.Tint != .5 {
		t.Fatalf("tint = %v, has=%v", sf.Tint, sf.HasTint)
	}
}

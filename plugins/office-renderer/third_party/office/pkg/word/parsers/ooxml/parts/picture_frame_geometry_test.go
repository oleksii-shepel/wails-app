package parts

import (
	"encoding/xml"
	"testing"
)

func TestPictureShapePropsPreservesPresetFrameGeometry(t *testing.T) {
	const src = `<pic:spPr xmlns:pic="http://schemas.openxmlformats.org/drawingml/2006/picture" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><a:prstGeom prst="flowChartConnector"><a:avLst/></a:prstGeom></pic:spPr>`
	var props PictureShapeProps
	if err := xml.Unmarshal([]byte(src), &props); err != nil {
		t.Fatal(err)
	}
	if props.PrstGeom == nil || props.PrstGeom.Prst != "flowChartConnector" {
		t.Fatalf("preset geometry = %#v, want flowChartConnector", props.PrstGeom)
	}
}

package drawingxml

import (
	"encoding/xml"
	"testing"
)

func TestPass32CustomGeometryPreservesPathLocalFillAndStrokeFlags(t *testing.T) {
	const src = `<a:custGeom xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><a:pathLst>` +
		`<a:path w="100" h="100" fill="none"><a:moveTo><a:pt x="0" y="0"/></a:moveTo><a:lnTo><a:pt x="100" y="100"/></a:lnTo></a:path>` +
		`<a:path w="100" h="100" stroke="0"><a:moveTo><a:pt x="0" y="100"/></a:moveTo><a:lnTo><a:pt x="100" y="0"/></a:lnTo></a:path>` +
		`</a:pathLst></a:custGeom>`
	var cg CustomGeometry
	if err := xml.Unmarshal([]byte(src), &cg); err != nil {
		t.Fatal(err)
	}
	geom := cg.DrawingGeometry()
	if geom == nil || len(geom.Paths) != 2 {
		t.Fatalf("geometry = %#v", geom)
	}
	if p := geom.Paths[0]; !p.PaintFlagsSet || p.FillEnabled || !p.StrokeEnabled {
		t.Fatalf("stroke path flags = set:%v fill:%v stroke:%v", p.PaintFlagsSet, p.FillEnabled, p.StrokeEnabled)
	}
	if p := geom.Paths[1]; !p.PaintFlagsSet || !p.FillEnabled || p.StrokeEnabled {
		t.Fatalf("fill path flags = set:%v fill:%v stroke:%v", p.PaintFlagsSet, p.FillEnabled, p.StrokeEnabled)
	}
}

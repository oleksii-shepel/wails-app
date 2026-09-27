package sax

import (
	"strings"
	"testing"
)

func TestPass9OLEPreviewPictureBelongsToGraphicFrame(t *testing.T) {
	const xml = `<?xml version="1.0"?>
<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"
       xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
       xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <p:cSld><p:spTree>
    <p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr/>
    <p:graphicFrame>
      <p:nvGraphicFramePr><p:cNvPr id="411" name="Chart object"/><p:cNvGraphicFramePr/><p:nvPr/></p:nvGraphicFramePr>
      <p:xfrm><a:off x="100" y="200"/><a:ext cx="300" cy="400"/></p:xfrm>
      <a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/presentationml/2006/ole">
        <p:oleObj r:id="rId1">
          <p:embed/>
          <p:pic>
            <p:nvPicPr><p:cNvPr id="412" name="Preview"/><p:cNvPicPr/><p:nvPr/></p:nvPicPr>
            <p:blipFill><a:blip r:embed="rId2"/><a:stretch/></p:blipFill>
            <p:spPr><a:xfrm><a:off x="100" y="200"/><a:ext cx="300" cy="400"/></a:xfrm><a:prstGeom prst="rect"/></p:spPr>
          </p:pic>
        </p:oleObj>
      </a:graphicData></a:graphic>
    </p:graphicFrame>
  </p:spTree></p:cSld>
</p:sld>`

	var starts []Event
	var blips []Event
	if err := walkShapeTreeReader(strings.NewReader(xml), "ppt/slides/slide4.xml", 3, func(ev Event) error {
		switch ev.Type {
		case EventStartShape:
			starts = append(starts, ev)
		case EventBlip:
			if ev.RelID != "" {
				blips = append(blips, ev)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(starts) != 1 {
		t.Fatalf("start shapes = %d, want only the OLE graphicFrame", len(starts))
	}
	if starts[0].ShapeID != 411 {
		t.Fatalf("start shape id = %d, want 411", starts[0].ShapeID)
	}
	if len(blips) != 1 {
		t.Fatalf("preview blips = %d, want 1", len(blips))
	}
	if blips[0].ShapeID != 411 || blips[0].RelID != "rId2" {
		t.Fatalf("preview blip owner = shape %d rel %q, want shape 411 rel rId2", blips[0].ShapeID, blips[0].RelID)
	}
}

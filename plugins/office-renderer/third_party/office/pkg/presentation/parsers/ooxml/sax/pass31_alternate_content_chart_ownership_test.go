package sax

import (
	"strings"
	"testing"
)

func TestPass31AlternateContentChoosesSemanticChartOverFallbackPicture(t *testing.T) {
	const slide = `<?xml version="1.0"?>
<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"
       xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
       xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart"
       xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"
       xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006">
  <p:cSld><p:spTree>
    <mc:AlternateContent>
      <mc:Choice Requires="c">
        <p:graphicFrame>
          <p:nvGraphicFramePr><p:cNvPr id="7" name="Semantic Chart"/></p:nvGraphicFramePr>
          <a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/chart"><c:chart r:id="rIdChart"/></a:graphicData></a:graphic>
        </p:graphicFrame>
      </mc:Choice>
      <mc:Fallback>
        <p:pic>
          <p:nvPicPr><p:cNvPr id="8" name="Chart Preview"/></p:nvPicPr>
          <p:blipFill><a:blip r:embed="rIdPreview"/></p:blipFill><p:spPr/>
        </p:pic>
      </mc:Fallback>
    </mc:AlternateContent>
    <p:pic>
      <p:nvPicPr><p:cNvPr id="9" name="Ordinary Picture"/></p:nvPicPr>
      <p:blipFill><a:blip r:embed="rIdOrdinary"/></p:blipFill><p:spPr/>
    </p:pic>
  </p:spTree></p:cSld>
</p:sld>`

	var starts []Event
	var graphics []Event
	var blips []Event
	if err := walkShapeTreeReader(strings.NewReader(slide), "ppt/slides/slide1.xml", 0, func(ev Event) error {
		switch ev.Type {
		case EventStartShape:
			starts = append(starts, ev)
		case EventGraphicObject:
			graphics = append(graphics, ev)
		case EventBlip:
			blips = append(blips, ev)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if len(starts) != 2 || starts[0].ShapeID != 7 || starts[1].ShapeID != 9 {
		t.Fatalf("shape starts = %#v, want semantic chart 7 plus ordinary picture 9", starts)
	}
	if len(graphics) != 1 || graphics[0].GraphicObjectKind != "chart" || graphics[0].RelID != "rIdChart" {
		t.Fatalf("graphic objects = %#v", graphics)
	}
	if len(blips) != 1 || blips[0].ShapeID != 9 || blips[0].RelID != "rIdOrdinary" {
		t.Fatalf("blips = %#v, fallback preview must be suppressed while ordinary picture survives", blips)
	}
}

func TestPass31AlternateContentUsesFallbackForUnsupportedExtensionChoice(t *testing.T) {
	const slide = `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"
 xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
 xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006"
 xmlns:p14="http://schemas.microsoft.com/office/powerpoint/2010/main">
 <p:cSld><p:spTree><mc:AlternateContent>
  <mc:Choice Requires="p14"><p:pic><p:nvPicPr><p:cNvPr id="20" name="Unsupported"/></p:nvPicPr><p:spPr/></p:pic></mc:Choice>
  <mc:Fallback><p:pic><p:nvPicPr><p:cNvPr id="21" name="Fallback"/></p:nvPicPr><p:spPr/></p:pic></mc:Fallback>
 </mc:AlternateContent></p:spTree></p:cSld></p:sld>`

	var starts []Event
	if err := walkShapeTreeReader(strings.NewReader(slide), "ppt/slides/slide1.xml", 0, func(ev Event) error {
		if ev.Type == EventStartShape {
			starts = append(starts, ev)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(starts) != 1 || starts[0].ShapeID != 21 {
		t.Fatalf("shape starts = %#v, want fallback shape 21", starts)
	}
}

package stream

import (
	"encoding/xml"
	"math"
	"testing"

	wordir "office/pkg/word/ir"
	wordparts "office/pkg/word/parsers/ooxml/parts"
)

func TestPopulateDOCXPictureProperties(t *testing.T) {
	const drawingXML = `<w:drawing xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:pic="http://schemas.openxmlformats.org/drawingml/2006/picture">
  <wp:anchor><wp:extent cx="914400" cy="457200"/><a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/picture"><pic:pic><pic:blipFill><a:blip><a:lum bright="50000" contrast="-20000"/><a:alphaMod val="60000"/><a:grayscl/><a:biLevel thresh="40000"/></a:blip><a:srcRect l="10000" t="5000" r="20000" b="15000"/><a:tile tx="12700" ty="25400" sx="80000" sy="120000" flip="xy" algn="ctr"/></pic:blipFill><pic:spPr><a:xfrm rot="5400000" flipH="1" flipV="1"/><a:prstGeom prst="ellipse"/><a:solidFill><a:srgbClr val="112233"/></a:solidFill><a:ln w="25400"><a:solidFill><a:srgbClr val="445566"/></a:solidFill></a:ln><a:effectLst><a:glow rad="25400"><a:srgbClr val="778899"/></a:glow></a:effectLst></pic:spPr></pic:pic></a:graphicData></a:graphic></wp:anchor>
</w:drawing>`

	var drawing wordparts.Drawing
	if err := xml.Unmarshal([]byte(drawingXML), &drawing); err != nil {
		t.Fatalf("unmarshal drawing: %v", err)
	}
	image := &wordir.Image{}
	populateDOCXPictureProperties(image, &drawing, nil)

	if image.CropLeft != 0.1 || image.CropTop != 0.05 || image.CropRight != 0.2 || image.CropBottom != 0.15 {
		t.Fatalf("crop = %#v", image)
	}
	if image.RotationDeg != 90 || !image.FlipH || !image.FlipV || image.FrameGeometry != "ellipse" {
		t.Fatalf("transform/frame = %#v", image)
	}
	if image.BorderColor != "445566" || image.BorderWidth != 2 || image.FillColor != "112233" {
		t.Fatalf("frame paint = %#v", image)
	}
	if !image.Tile || image.Stretch || image.TileOffsetX != 12700 || image.TileOffsetY != 25400 || image.TileScaleX != 80000 || image.TileScaleY != 120000 || image.TileFlip != "xy" || image.TileAlign != "ctr" {
		t.Fatalf("tile = %#v", image)
	}
	if math.Abs(image.Brightness-1.5) > 1e-9 || math.Abs(image.Contrast-0.8) > 1e-9 || !image.HasAlpha || math.Abs(image.Alpha-0.6) > 1e-9 || !image.Grayscale || !image.BiLevel || image.BiLevelThreshold != 40000 {
		t.Fatalf("blip adjustments = %#v", image)
	}
	if len(image.Effects.Effects) != 1 {
		t.Fatalf("effects = %#v, want one glow", image.Effects)
	}
}

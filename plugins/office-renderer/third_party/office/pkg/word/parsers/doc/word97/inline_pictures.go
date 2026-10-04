package word97

import (
	"encoding/binary"
	"math"

	officeartir "office/pkg/officeart/ir"
	officeartescher "office/pkg/officeart/legacy/escher"
	"office/pkg/ooxml/units"
	wordir "office/pkg/word/ir"
	docofficeart "office/pkg/word/parsers/doc/officeart"
)

// InlinePictureDescriptor preserves the PICF display geometry and shared
// OfficeArt visual properties for one Word97 inline picture.
type InlinePictureDescriptor struct {
	GoalWidthTwips  int
	GoalHeightTwips int
	ScaleXPermille  int
	ScaleYPermille  int
	Visual          *officeartir.Shape
}

// ScanInlinePictureDescriptors finds Word97 PICF + picture-frame SpContainer
// pairs in Data/DataStream. Structural OfficeArt decoding is shared; only the
// Word-specific PICF fields are interpreted here.
func ScanInlinePictureDescriptors(data []byte) []InlinePictureDescriptor {
	var out []InlinePictureDescriptor
	records := officeartescher.Scan(data, func(h officeartescher.Header) bool {
		return h.Type == officeartescher.RecordSpContainer && h.IsContainer()
	})
	for _, record := range records {
		node, err := officeartescher.ShapeNodeFromRecord(record)
		if err != nil || node == nil || node.ShapeType != 75 {
			continue
		}
		desc := InlinePictureDescriptor{ScaleXPermille: 1000, ScaleYPermille: 1000}
		if record.Offset >= 0x44 {
			start := int(record.Offset) - 0x44
			if start >= 0 && start+0x44 <= len(data) {
				h := data[start : start+0x44]
				if binary.LittleEndian.Uint16(h[4:6]) == 0x44 {
					desc.GoalWidthTwips = int(binary.LittleEndian.Uint16(h[28:30]))
					desc.GoalHeightTwips = int(binary.LittleEndian.Uint16(h[30:32]))
					if mx := int(binary.LittleEndian.Uint16(h[32:34])); mx > 0 {
						desc.ScaleXPermille = mx
					}
					if my := int(binary.LittleEndian.Uint16(h[34:36])); my > 0 {
						desc.ScaleYPermille = my
					}
				}
			}
		}
		desc.Visual = pictureFrameVisual(data[int(record.Offset):int(record.Offset)+officeartescher.HeaderSize+int(record.Header.Length)])
		out = append(out, desc)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// pictureFrameVisual decodes the shared OfficeArt visual of the picture-frame
// SpContainer stored inside a Word97 picture block. A PICF carries only display
// geometry and the media location; Word keeps picture crop and the brightness,
// contrast, grayscale and bi-level corrections on the frame shape, so any
// resolver that reads media straight from a PICF block must read the frame too
// or the authored corrections are lost.
func pictureFrameVisual(block []byte) *officeartir.Shape {
	if len(block) < officeartescher.HeaderSize {
		return nil
	}
	records := officeartescher.Scan(block, func(h officeartescher.Header) bool {
		return h.Type == officeartescher.RecordSpContainer && h.IsContainer()
	})
	for _, record := range records {
		node, err := officeartescher.ShapeNodeFromRecord(record)
		if err != nil || node == nil || node.ShapeType != 75 {
			continue
		}
		end := int(record.Offset) + officeartescher.HeaderSize + int(record.Header.Length)
		if end > len(block) {
			continue
		}
		catalog := docofficeart.DecodeStreams(block[record.Offset:end])
		if shape, ok := catalog.Find(node.ShapeID); ok && shape != nil {
			return shape
		}
	}
	return nil
}

// ApplyInlinePictureDescriptor applies authored PICF scaling and shared
// OfficeArt visual semantics to a resolved inline picture.
func ApplyInlinePictureDescriptor(image *wordir.Image, desc InlinePictureDescriptor) {
	if image == nil {
		return
	}
	if desc.GoalWidthTwips > 0 {
		width := float64(desc.GoalWidthTwips) * float64(desc.ScaleXPermille) / 1000.0
		image.Width = units.TwipsToEMU(int64(math.Round(width)))
	}
	if desc.GoalHeightTwips > 0 {
		height := float64(desc.GoalHeightTwips) * float64(desc.ScaleYPermille) / 1000.0
		image.Height = units.TwipsToEMU(int64(math.Round(height)))
	}
	if desc.ScaleXPermille > 0 && desc.ScaleYPermille > 0 && desc.ScaleXPermille != desc.ScaleYPermille {
		image.Stretch = true
	}
	visual := desc.Visual
	if visual == nil {
		return
	}
	image.RotationDeg = visual.Transform.RotationDeg
	image.FlipH = visual.Transform.FlipH
	image.FlipV = visual.Transform.FlipV
	if visual.Image != nil {
		image.CropTop = visual.Image.Crop.Top
		image.CropBottom = visual.Image.Crop.Bottom
		image.CropLeft = visual.Image.Crop.Left
		image.CropRight = visual.Image.Crop.Right
		image.Brightness = visual.Image.Brightness
		image.Contrast = visual.Image.Contrast
		image.Grayscale = visual.Image.Grayscale
		image.BiLevel = visual.Image.BiLevel
	}
}

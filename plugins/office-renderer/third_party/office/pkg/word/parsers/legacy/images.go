package legacy

import (
	"bytes"
	"encoding/binary"
	"math"

	"office/pkg/ooxml/cfb"
	"office/pkg/ooxml/units"
	wordir "office/pkg/word/ir"
)

const legacyImageDefaultDPI = 96.0

func extractLegacyInlineImages(f *cfb.File) []*wordir.Image {
	if f == nil {
		return nil
	}
	dataStream, ok := f.Stream("Data")
	if !ok || len(dataStream) == 0 {
		dataStream, ok = f.Stream("DataStream")
		if !ok || len(dataStream) == 0 {
			return nil
		}
	}
	blobs := scanLegacyImageBlobs(dataStream)
	if len(blobs) == 0 {
		return nil
	}
	descriptors := scanLegacyInlinePictureDescriptors(dataStream)
	images := make([]*wordir.Image, 0, len(blobs))
	for blobIndex, blob := range blobs {
		mime := detectLegacyImageMIME(blob)
		if mime == "" {
			continue
		}
		widthPx, heightPx := detectLegacyImageDimensions(blob, mime)
		copied := make([]byte, len(blob))
		copy(copied, blob)
		img := &wordir.Image{MIMEType: mime, Data: copied}
		if widthPx > 0 && heightPx > 0 {
			img.Width = units.InchesToEMU(float64(widthPx) / legacyImageDefaultDPI)
			img.Height = units.InchesToEMU(float64(heightPx) / legacyImageDefaultDPI)
		}
		if blobIndex < len(descriptors) {
			applyLegacyInlinePictureDescriptor(img, descriptors[blobIndex])
		}
		images = append(images, img)
	}
	if len(images) == 0 {
		return nil
	}
	return images
}

// legacyInlinePictureDescriptor preserves the PICF display geometry and the
// OfficeArt properties that accompany one inline picture in the Data stream.
// Word 97-2003 commonly stores a 68-byte PICF immediately before an OfficeArt
// picture SpContainer; the bitmap payload follows the container.
type legacyInlinePictureDescriptor struct {
	GoalWidthTwips  int
	GoalHeightTwips int
	ScaleXPermille  int
	ScaleYPermille  int
	Props           legacyOfficeArtShapeProps
}

func scanLegacyInlinePictureDescriptors(data []byte) []legacyInlinePictureDescriptor {
	var out []legacyInlinePictureDescriptor
	for off := 0; off+8 <= len(data); off++ {
		recVer, recInstance, recType, recLen, ok := legacyOfficeArtRecordHeader(data[off:])
		if !ok || recType != legacyOfficeArtSpContainer || recVer != 0xF || recLen <= 0 || off+8+recLen > len(data) {
			continue
		}
		// Picture shape type 75 is the OfficeArt picture frame used by these
		// inline PICF records. Ignore non-picture shape containers.
		_ = recInstance
		record := data[off : off+8+recLen]
		propsByID := map[uint32]legacyOfficeArtShapeProps{}
		legacyCollectOfficeArtShapeProps(record, propsByID)
		var props legacyOfficeArtShapeProps
		for _, candidate := range propsByID {
			if candidate.ShapeType == 75 {
				props = candidate
				break
			}
		}
		if props.ShapeType != 75 {
			off += 7 + recLen
			continue
		}

		desc := legacyInlinePictureDescriptor{Props: props, ScaleXPermille: 1000, ScaleYPermille: 1000}
		// PICF is exactly 0x44 bytes in Word 97+ for these pictures. Validate
		// cbHeader before reading display goal/scaling fields.
		if off >= 0x44 {
			h := data[off-0x44 : off]
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
		out = append(out, desc)
		off += 7 + recLen
	}
	return out
}

func applyLegacyInlinePictureDescriptor(img *wordir.Image, desc legacyInlinePictureDescriptor) {
	if img == nil {
		return
	}
	if desc.GoalWidthTwips > 0 {
		width := float64(desc.GoalWidthTwips) * float64(desc.ScaleXPermille) / 1000.0
		img.Width = units.TwipsToEMU(int64(math.Round(width)))
	}
	if desc.GoalHeightTwips > 0 {
		height := float64(desc.GoalHeightTwips) * float64(desc.ScaleYPermille) / 1000.0
		img.Height = units.TwipsToEMU(int64(math.Round(height)))
	}
	// PICF mx/my are independent display-axis scales. When they differ, Word
	// intentionally distorts the bitmap to the authored rectangle. Mark the
	// image as stretched so HTML does not add object-fit:contain, which would
	// preserve the intrinsic aspect ratio and silently cancel the X/Y scaling.
	if desc.ScaleXPermille > 0 && desc.ScaleYPermille > 0 && desc.ScaleXPermille != desc.ScaleYPermille {
		img.Stretch = true
	}
	p := desc.Props
	if p.HasRotation {
		img.RotationDeg = p.Rotation
	}
	img.FlipH = p.FlipH
	img.FlipV = p.FlipV
	img.CropTop = p.CropTop
	img.CropBottom = p.CropBottom
	img.CropLeft = p.CropLeft
	img.CropRight = p.CropRight
	img.Brightness = p.Brightness
	img.Contrast = p.Contrast
	img.Grayscale = p.Grayscale
	img.BiLevel = p.BiLevel
}

func scanLegacyImageBlobs(data []byte) [][]byte {
	var out [][]byte
	for i := 0; i < len(data); i++ {
		switch {
		case i+8 <= len(data) && bytes.Equal(data[i:i+8], []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1A, '\n'}):
			if end, ok := scanPNGEnd(data, i); ok {
				out = append(out, data[i:end])
				i = end - 1
			}
		case i+2 <= len(data) && data[i] == 0xFF && data[i+1] == 0xD8:
			if end, ok := scanJPEGEnd(data, i); ok {
				out = append(out, data[i:end])
				i = end - 1
			}
		case i+6 <= len(data) && (bytes.Equal(data[i:i+6], []byte("GIF87a")) || bytes.Equal(data[i:i+6], []byte("GIF89a"))):
			if end, ok := scanGIFEnd(data, i); ok {
				out = append(out, data[i:end])
				i = end - 1
			}
		case i+6 <= len(data) && data[i] == 'B' && data[i+1] == 'M':
			sz := int(binary.LittleEndian.Uint32(data[i+2 : i+6]))
			if sz > 14 && i+sz <= len(data) {
				out = append(out, data[i:i+sz])
				i += sz - 1
			}
		}
	}
	return out
}

func scanPNGEnd(data []byte, start int) (int, bool) {
	off := start + 8
	for off+12 <= len(data) {
		chunkLen := int(binary.BigEndian.Uint32(data[off : off+4]))
		if chunkLen < 0 || off+12+chunkLen > len(data) {
			return 0, false
		}
		chunkType := data[off+4 : off+8]
		off += 8 + chunkLen + 4
		if bytes.Equal(chunkType, []byte("IEND")) {
			return off, true
		}
	}
	return 0, false
}

func scanJPEGEnd(data []byte, start int) (int, bool) {
	for i := start + 2; i+1 < len(data); i++ {
		if data[i] == 0xFF && data[i+1] == 0xD9 {
			return i + 2, true
		}
	}
	return 0, false
}

func scanGIFEnd(data []byte, start int) (int, bool) {
	for i := start + 6; i < len(data); i++ {
		if data[i] == 0x3B {
			return i + 1, true
		}
	}
	return 0, false
}

func detectLegacyImageMIME(data []byte) string {
	switch {
	case len(data) >= 8 && bytes.Equal(data[:8], []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1A, '\n'}):
		return "image/png"
	case len(data) >= 2 && data[0] == 0xFF && data[1] == 0xD8:
		return "image/jpeg"
	case len(data) >= 6 && (bytes.Equal(data[:6], []byte("GIF87a")) || bytes.Equal(data[:6], []byte("GIF89a"))):
		return "image/gif"
	case len(data) >= 2 && data[0] == 'B' && data[1] == 'M':
		return "image/bmp"
	default:
		return ""
	}
}

func detectLegacyImageDimensions(data []byte, mime string) (int, int) {
	switch mime {
	case "image/png":
		return parsePNGDims(data)
	case "image/jpeg":
		return parseJPEGDims(data)
	case "image/gif":
		return parseGIFDims(data)
	case "image/bmp":
		return parseBMPDims(data)
	default:
		return 0, 0
	}
}

func parsePNGDims(data []byte) (int, int) {
	if len(data) < 24 {
		return 0, 0
	}
	if !bytes.Equal(data[:8], []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1A, '\n'}) {
		return 0, 0
	}
	if !bytes.Equal(data[12:16], []byte("IHDR")) {
		return 0, 0
	}
	w := int(binary.BigEndian.Uint32(data[16:20]))
	h := int(binary.BigEndian.Uint32(data[20:24]))
	if w <= 0 || h <= 0 {
		return 0, 0
	}
	return w, h
}

func parseGIFDims(data []byte) (int, int) {
	if len(data) < 10 {
		return 0, 0
	}
	if !(bytes.Equal(data[:6], []byte("GIF87a")) || bytes.Equal(data[:6], []byte("GIF89a"))) {
		return 0, 0
	}
	w := int(binary.LittleEndian.Uint16(data[6:8]))
	h := int(binary.LittleEndian.Uint16(data[8:10]))
	if w <= 0 || h <= 0 {
		return 0, 0
	}
	return w, h
}

func parseBMPDims(data []byte) (int, int) {
	if len(data) < 26 || data[0] != 'B' || data[1] != 'M' {
		return 0, 0
	}
	if len(data) < 26 {
		return 0, 0
	}
	w := int(int32(binary.LittleEndian.Uint32(data[18:22])))
	h := int(int32(binary.LittleEndian.Uint32(data[22:26])))
	if h < 0 {
		h = -h
	}
	if w <= 0 || h <= 0 {
		return 0, 0
	}
	return w, h
}

func parseJPEGDims(data []byte) (int, int) {
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		return 0, 0
	}
	i := 2
	for i+3 < len(data) {
		if data[i] != 0xFF {
			i++
			continue
		}
		for i < len(data) && data[i] == 0xFF {
			i++
		}
		if i >= len(data) {
			break
		}
		marker := data[i]
		i++
		if marker == 0xD8 || marker == 0xD9 {
			continue
		}
		if i+1 >= len(data) {
			break
		}
		segLen := int(binary.BigEndian.Uint16(data[i : i+2]))
		if segLen < 2 || i+segLen > len(data) {
			break
		}
		if isJPEGSOFMarker(marker) {
			if segLen < 7 {
				return 0, 0
			}
			h := int(binary.BigEndian.Uint16(data[i+3 : i+5]))
			w := int(binary.BigEndian.Uint16(data[i+5 : i+7]))
			if w <= 0 || h <= 0 {
				return 0, 0
			}
			return w, h
		}
		i += segLen
	}
	return 0, 0
}

func isJPEGSOFMarker(marker byte) bool {
	switch marker {
	case 0xC0, 0xC1, 0xC2, 0xC3, 0xC5, 0xC6, 0xC7, 0xC9, 0xCA, 0xCB, 0xCD, 0xCE, 0xCF:
		return true
	default:
		return false
	}
}

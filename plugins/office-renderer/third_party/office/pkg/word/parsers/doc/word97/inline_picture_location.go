package word97

import (
	"encoding/binary"

	"office/pkg/ooxml/units"
	wordir "office/pkg/word/ir"
	docformat "office/pkg/word/parsers/doc/format"
	docofficeart "office/pkg/word/parsers/doc/officeart"
)

const inlinePICFHeaderSize = 0x44

type inlinePictureState uint8

const (
	inlinePictureInvalid inlinePictureState = iota
	inlinePicturePlaceholder
	inlinePictureRenderable
)

// resolveInlinePicturesByLocation resolves fixed Word97 pictures from the
// sprmCPicLocation offsets carried by special character runs. The offset is a
// byte position in Data/DataStream where the PICF begins; resolving from that
// exact block preserves duplicate/reused picture references and avoids relying
// on a global sequential BLIP scan.
func resolveInlinePicturesByLocation(data []byte, entries []docformat.CHPXEntry) map[int]*wordir.Image {
	if len(data) < inlinePICFHeaderSize || len(entries) == 0 {
		return nil
	}
	out := make(map[int]*wordir.Image)
	for _, entry := range entries {
		if !entry.Spec || entry.Obj || entry.Data {
			continue
		}
		location, ok := picLocationFromCHPX(entry.Sprms)
		if !ok || location < 0 {
			continue
		}
		if _, exists := out[location]; exists {
			continue
		}
		if image, state := resolveInlinePicture(data, location); state == inlinePictureRenderable && image != nil {
			out[location] = image
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func resolveInlinePictureAt(data []byte, location int) *wordir.Image {
	image, state := resolveInlinePicture(data, location)
	if state != inlinePictureRenderable {
		return nil
	}
	return image
}

// resolveInlinePicture owns PICF structural classification. A recognized,
// structurally valid picture block with no decodable media is a placeholder,
// not malformed input: some Word97 producers use such anchors only to preserve
// layout. Stream cursors consume that semantic result without decoding PICF.
func resolveInlinePicture(data []byte, location int) (*wordir.Image, inlinePictureState) {
	if location < 0 || location+inlinePICFHeaderSize > len(data) {
		return nil, inlinePictureInvalid
	}

	blockType := binary.LittleEndian.Uint16(data[location+0x0E : location+0x10])
	mappingMode := binary.LittleEndian.Uint16(data[location+0x06 : location+0x08])
	if !recognizedInlinePictureBlock(blockType, mappingMode) {
		return nil, inlinePictureInvalid
	}

	length := int(binary.LittleEndian.Uint32(data[location : location+4]))
	headerSize := int(binary.LittleEndian.Uint16(data[location+4 : location+6]))
	if headerSize < inlinePICFHeaderSize || headerSize > length || length > len(data)-location {
		return nil, inlinePictureInvalid
	}
	block := data[location : location+length]
	payload := block[headerSize:]

	// Word97 PICF payloads occur in two forms in real documents. Newer ones
	// contain OfficeArt/Escher records, while older mm=0x64 blocks can contain
	// bitmap bytes directly after the PICF header.
	var image *wordir.Image
	if images := docofficeart.ProjectImages(payload); len(images) > 0 && images[0] != nil {
		copyImage := *images[0]
		copyImage.Data = append([]byte(nil), images[0].Data...)
		image = &copyImage
	} else if blobs := scanImageBlobs(payload); len(blobs) > 0 {
		blob := blobs[0]
		mime := detectImageMIME(blob)
		if mime != "" {
			image = &wordir.Image{MIMEType: mime, Data: append([]byte(nil), blob...)}
			if w, h := detectImageDimensions(blob, mime); w > 0 && h > 0 {
				image.Width = units.InchesToEMU(float64(w) / inlineImageDefaultDPI)
				image.Height = units.InchesToEMU(float64(h) / inlineImageDefaultDPI)
			}
		}
	}
	if image == nil {
		return nil, inlinePicturePlaceholder
	}

	goalWidth := int(binary.LittleEndian.Uint16(block[28:30]))
	goalHeight := int(binary.LittleEndian.Uint16(block[30:32]))
	scaleX := int(binary.LittleEndian.Uint16(block[32:34]))
	scaleY := int(binary.LittleEndian.Uint16(block[34:36]))
	if scaleX <= 0 {
		scaleX = 1000
	}
	if scaleY <= 0 {
		scaleY = 1000
	}
	ApplyInlinePictureDescriptor(image, InlinePictureDescriptor{
		GoalWidthTwips:  goalWidth,
		GoalHeightTwips: goalHeight,
		ScaleXPermille:  scaleX,
		ScaleYPermille:  scaleY,
		// Crop and picture corrections live on the picture-frame shape in the
		// OfficeArt records after the PICF header, not in the PICF itself.
		Visual: pictureFrameVisual(payload),
	})
	return image, inlinePictureRenderable
}

func recognizedInlinePictureBlock(blockType, mappingMode uint16) bool {
	switch blockType {
	case 0x0008, 0x000A:
		return true
	case 0x0000, 0x0002:
		return mappingMode == 0x0064
	default:
		return false
	}
}

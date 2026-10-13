package imageutil

import "encoding/binary"

// NaturalDPI returns the resolution stored in raster image metadata, or 0 when
// the image states none. DrawingML tiles a picture fill at the image's natural
// size, which is the pixel size resolved at that resolution: 41246-2 slide 16's
// crumpled-paper texture is a 128 px JPEG tagged 75 dpi, and PowerPoint paints
// its tiles 122.9 pt wide (128 px at 75 dpi), not the 96 pt a 96 dpi default
// would give.
func NaturalDPI(data []byte) int {
	if dpi := jpegDPI(data); dpi > 0 {
		return dpi
	}
	return pngDPI(data)
}

func jpegDPI(data []byte) int {
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		return 0
	}
	for i := 2; i+4 <= len(data); {
		if data[i] != 0xFF {
			i++
			continue
		}
		marker := data[i+1]
		// Standalone markers carry no length; SOI/EOI and RSTn end the scan.
		if marker == 0xD8 || marker == 0x01 || (marker >= 0xD0 && marker <= 0xD9) {
			i += 2
			continue
		}
		if marker == 0xDA { // SOS: entropy-coded data follows.
			return 0
		}
		if i+4 > len(data) {
			return 0
		}
		length := int(binary.BigEndian.Uint16(data[i+2 : i+4]))
		if length < 2 || i+2+length > len(data) {
			return 0
		}
		payload := data[i+4 : i+2+length]
		if marker == 0xE0 && len(payload) >= 12 && string(payload[0:5]) == "JFIF\x00" {
			units := payload[7]
			density := int(binary.BigEndian.Uint16(payload[8:10]))
			switch units {
			case 1:
				return density
			case 2:
				return int(float64(density)*2.54 + 0.5)
			default:
				return 0
			}
		}
		i += 2 + length
	}
	return 0
}

func pngDPI(data []byte) int {
	const signature = "\x89PNG\r\n\x1a\n"
	if len(data) < 8 || string(data[:8]) != signature {
		return 0
	}
	for i := 8; i+8 <= len(data); {
		length := int(binary.BigEndian.Uint32(data[i : i+4]))
		chunkType := string(data[i+4 : i+8])
		if length < 0 || i+12+length > len(data) {
			return 0
		}
		if chunkType == "pHYs" && length >= 9 {
			ppuX := binary.BigEndian.Uint32(data[i+8 : i+12])
			unit := data[i+16]
			if unit == 1 { // pixels per meter
				return int(float64(ppuX)*2.54/100.0 + 0.5)
			}
			return 0
		}
		i += 12 + length
	}
	return 0
}

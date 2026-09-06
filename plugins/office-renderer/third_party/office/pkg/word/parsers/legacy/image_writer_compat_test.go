package legacy

import (
	"bytes"
	"encoding/binary"
)

// Raw image scanning is retained only for writer compatibility assertions.
// Production DOC parsing uses word97.LoadInlineImages and doc/officeart BLIP discovery.
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
	if len(data) < 24 || !bytes.Equal(data[:8], []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1A, '\n'}) || !bytes.Equal(data[12:16], []byte("IHDR")) {
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
	if len(data) < 10 || !(bytes.Equal(data[:6], []byte("GIF87a")) || bytes.Equal(data[:6], []byte("GIF89a"))) {
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

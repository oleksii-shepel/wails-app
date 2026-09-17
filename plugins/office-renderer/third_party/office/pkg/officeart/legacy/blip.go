package legacy

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"io"

	"office/pkg/officeart/legacy/escher"
)

type Media struct {
	MIME string
	Data []byte
}

func IsBlipRecordType(recordType uint16) bool {
	switch recordType {
	case escher.RecordBlipEMF, escher.RecordBlipWMF, escher.RecordBlipPICT,
		escher.RecordBlipJPEG, escher.RecordBlipPNG, escher.RecordBlipDIB:
		return true
	default:
		return false
	}
}

// ExtractBLIP converts the OfficeArt BLIP record payload into the embedded
// media bytes. It handles the common UID/metafile headers and compressed
// EMF/WMF/PICT payloads shared by DOC/XLS/PPT.
func ExtractBLIP(recordType uint16, payload []byte) Media {
	switch recordType {
	case escher.RecordBlipEMF:
		if data := extractVectorBLIP(payload, "image/x-emf"); len(data) > 0 {
			return Media{MIME: "image/x-emf", Data: data}
		}
	case escher.RecordBlipWMF:
		if data := extractVectorBLIP(payload, "image/x-wmf"); len(data) > 0 {
			return Media{MIME: "image/x-wmf", Data: data}
		}
	case escher.RecordBlipPICT:
		if data := extractPICTBLIP(payload); len(data) > 0 {
			return Media{MIME: "image/x-pict", Data: data}
		}
	case escher.RecordBlipJPEG, escher.RecordBlipPNG, escher.RecordBlipDIB:
		if data, mime := ExtractRaster(payload); len(data) > 0 {
			return Media{MIME: mime, Data: data}
		}
	}
	return Media{}
}

// ExtractRaster finds the first complete raster image embedded in data.
func ExtractRaster(data []byte) ([]byte, string) {
	for start := 0; start < len(data); start++ {
		mime := SniffImageMIME(data[start:])
		if mime == "" || mime == "image/x-emf" || mime == "image/x-wmf" {
			continue
		}
		if end, ok := ImageEnd(data[start:], mime); ok {
			return append([]byte(nil), data[start:start+end]...), mime
		}
	}

	// Some producers nest a BLIP record under extra OfficeArt containers.
	records := escher.Scan(data, func(h escher.Header) bool {
		return IsBlipRecordType(h.Type)
	})
	for _, record := range records {
		if record.Header.IsContainer() {
			continue
		}
		media := ExtractBLIP(record.Header.Type, record.Data)
		if len(media.Data) > 0 && media.MIME != "" {
			return media.Data, media.MIME
		}
	}
	return nil, ""
}

func SniffImageMIME(data []byte) string {
	if len(data) >= 44 && binary.LittleEndian.Uint32(data[0:4]) == 1 && string(data[40:44]) == " EMF" {
		return "image/x-emf"
	}
	if looksLikeWMF(data) || (len(data) >= 4 && bytes.Equal(data[:4], []byte{0xD7, 0xCD, 0xC6, 0x9A})) {
		return "image/x-wmf"
	}
	if len(data) >= 8 && bytes.Equal(data[:8], []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}) {
		return "image/png"
	}
	if len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		return "image/jpeg"
	}
	if len(data) >= 6 && (string(data[:6]) == "GIF87a" || string(data[:6]) == "GIF89a") {
		return "image/gif"
	}
	if len(data) >= 2 && data[0] == 'B' && data[1] == 'M' {
		return "image/bmp"
	}
	return ""
}

func ImageEnd(data []byte, mime string) (int, bool) {
	switch mime {
	case "image/x-emf", "image/x-wmf", "image/x-pict":
		return len(data), len(data) > 0
	case "image/png":
		return scanPNGEnd(data)
	case "image/jpeg":
		return scanJPEGEnd(data)
	case "image/gif":
		return scanGIFEnd(data)
	case "image/bmp":
		if len(data) < 6 {
			return 0, false
		}
		size := int(binary.LittleEndian.Uint32(data[2:6]))
		if size <= 14 || size > len(data) {
			return 0, false
		}
		return size, true
	default:
		return 0, false
	}
}

func extractVectorBLIP(payload []byte, mime string) []byte {
	if data := extractVectorBlob(payload, mime); len(data) > 0 {
		return data
	}
	maxOffset := len(payload)
	if maxOffset > 64 {
		maxOffset = 64
	}
	for offset := 0; offset < maxOffset; offset++ {
		inflated := inflate(payload[offset:])
		if len(inflated) == 0 {
			continue
		}
		if data := extractVectorBlob(inflated, mime); len(data) > 0 {
			return data
		}
	}
	return nil
}

func extractVectorBlob(data []byte, mime string) []byte {
	start, ok := vectorImageStart(data, mime)
	if !ok || start < 0 || start >= len(data) {
		return nil
	}
	return append([]byte(nil), data[start:]...)
}

func vectorImageStart(data []byte, mime string) (int, bool) {
	switch mime {
	case "image/x-emf":
		if len(data) >= 44 && binary.LittleEndian.Uint32(data[0:4]) == 1 && string(data[40:44]) == " EMF" {
			return 0, true
		}
		for index := 40; index+4 <= len(data); index++ {
			if string(data[index:index+4]) != " EMF" {
				continue
			}
			start := index - 40
			if start >= 0 && start+44 <= len(data) && binary.LittleEndian.Uint32(data[start:start+4]) == 1 {
				return start, true
			}
		}
	case "image/x-wmf":
		if looksLikeWMF(data) {
			return 0, true
		}
		if index := bytes.Index(data, []byte{0xD7, 0xCD, 0xC6, 0x9A}); index >= 0 {
			return index, true
		}
		for index := 0; index+18 <= len(data); index++ {
			if looksLikeWMF(data[index:]) {
				return index, true
			}
		}
	}
	return 0, false
}

func looksLikeWMF(data []byte) bool {
	if len(data) < 18 {
		return false
	}
	typeVal := binary.LittleEndian.Uint16(data[0:2])
	headerSize := binary.LittleEndian.Uint16(data[2:4])
	version := binary.LittleEndian.Uint16(data[4:6])
	return (typeVal == 1 || typeVal == 2) && headerSize == 9 && (version == 0x0100 || version == 0x0300)
}

func extractPICTBLIP(payload []byte) []byte {
	const metafileHeaderSize = 34
	const macHeaderSize = 512
	for _, uidSize := range []int{16, 32} {
		offset := uidSize + metafileHeaderSize
		if offset >= len(payload) {
			continue
		}
		inflated := inflate(payload[offset:])
		if len(inflated) == 0 {
			continue
		}
		out := make([]byte, macHeaderSize+len(inflated))
		copy(out[macHeaderSize:], inflated)
		return out
	}
	return nil
}

func inflate(data []byte) []byte {
	if len(data) < 2 {
		return nil
	}
	reader, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil
	}
	defer reader.Close()
	out, err := io.ReadAll(reader)
	if err != nil || len(out) == 0 {
		return nil
	}
	return out
}

func scanPNGEnd(data []byte) (int, bool) {
	if len(data) < 8 {
		return 0, false
	}
	off := 8
	for off+8 <= len(data) {
		chunkLen := int(binary.BigEndian.Uint32(data[off : off+4]))
		if chunkLen < 0 || off+12+chunkLen > len(data) {
			return 0, false
		}
		chunkType := string(data[off+4 : off+8])
		off += 12 + chunkLen
		if chunkType == "IEND" {
			return off, true
		}
	}
	return 0, false
}

func scanJPEGEnd(data []byte) (int, bool) {
	// JPEG payloads may contain arbitrary application metadata, including
	// embedded JPEG previews. A byte scan for FF D9 therefore truncates the
	// outer image at an EOI marker that belongs to metadata. Walk the JPEG
	// marker stream instead and skip length-delimited segments atomically.
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		return 0, false
	}

	pos := 2
	inScan := false
	for pos < len(data) {
		if inScan {
			// Entropy-coded scan data uses FF 00 byte stuffing and may contain
			// restart markers. Any other unstuffed marker ends the scan and is
			// processed by the normal marker parser below.
			for pos < len(data) {
				if data[pos] != 0xFF {
					pos++
					continue
				}
				markerPos := pos
				for pos < len(data) && data[pos] == 0xFF {
					pos++
				}
				if pos >= len(data) {
					return 0, false
				}
				marker := data[pos]
				if marker == 0x00 { // stuffed literal FF
					pos++
					continue
				}
				if marker >= 0xD0 && marker <= 0xD7 { // restart marker
					pos++
					continue
				}
				pos = markerPos
				inScan = false
				break
			}
			if inScan {
				return 0, false
			}
		}

		if pos >= len(data) || data[pos] != 0xFF {
			return 0, false
		}
		for pos < len(data) && data[pos] == 0xFF {
			pos++
		}
		if pos >= len(data) {
			return 0, false
		}
		marker := data[pos]
		pos++

		switch {
		case marker == 0xD9: // EOI
			return pos, true
		case marker == 0xD8, marker == 0x01, marker >= 0xD0 && marker <= 0xD7:
			// SOI, TEM and restart markers have no length field.
			continue
		}

		if pos+2 > len(data) {
			return 0, false
		}
		segmentLen := int(binary.BigEndian.Uint16(data[pos : pos+2]))
		if segmentLen < 2 || pos+segmentLen > len(data) {
			return 0, false
		}
		pos += segmentLen
		if marker == 0xDA { // SOS: entropy-coded bytes follow the scan header.
			inScan = true
		}
	}
	return 0, false
}

func scanGIFEnd(data []byte) (int, bool) {
	if len(data) < 7 {
		return 0, false
	}
	for i := 6; i < len(data); i++ {
		if data[i] == 0x3B {
			return i + 1, true
		}
	}
	return 0, false
}

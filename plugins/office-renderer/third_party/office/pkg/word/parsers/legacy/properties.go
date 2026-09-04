package legacy

import (
	"encoding/binary"
	"strings"
	"unicode/utf16"

	"office/pkg/ooxml/cfb"

	"golang.org/x/text/encoding/charmap"
)

const (
	legacySummaryInformationStream = "\x05SummaryInformation"
	legacyDocSummaryInformationStream = "\x05DocumentSummaryInformation"

	legacyPropertyCodePage   = 0x00000001
	legacyPropertyTitle      = 0x00000002
	legacyPropertySubject    = 0x00000003
	legacyPropertyAuthor     = 0x00000004
	legacyPropertyKeywords   = 0x00000005
	legacyPropertyComments   = 0x00000006
	legacyPropertyTemplate   = 0x00000007
	legacyPropertyLastAuthor = 0x00000008
	legacyPropertyEditTime   = 0x0000000A
	legacyPropertyAppName    = 0x00000012

	legacyDocSummaryPropertyManager = 0x0000000E
	legacyDocSummaryPropertyCompany = 0x0000000F
	legacyDocSummaryPropertyDict    = 0x00000000

	legacyVTI2       = 0x0002
	legacyVTUI2      = 0x0012
	legacyVTI4       = 0x0003
	legacyVTFILETIME = 0x0040
	legacyVTLPSTR    = 0x001E
	legacyVTLPWSTR   = 0x001F
	legacyVTBLOB     = 0x0041
	legacyVTVECTOR   = 0x1000
	legacyVTVARIANT  = 0x000C
)

type legacyDocProperties struct {
	Title            string
	Subject          string
	Author           string
	Description      string
	Keywords         string
	LastModifiedBy   string
	Template         string
	Application      string
	TotalEditMinutes int
	Company          string
	Manager          string
	CustomProperties map[string]string
}

func loadLegacyDocProperties(f *cfb.File) legacyDocProperties {
	if f == nil {
		return legacyDocProperties{}
	}
	out := legacyDocProperties{}
	if stream, ok := f.Stream(legacySummaryInformationStream); ok && len(stream) > 0 {
		props := parseLegacyPropertySetStrings(stream)
		out.Title = props[legacyPropertyTitle]
		out.Subject = props[legacyPropertySubject]
		out.Author = props[legacyPropertyAuthor]
		out.Description = props[legacyPropertyComments]
		out.Keywords = props[legacyPropertyKeywords]
		out.LastModifiedBy = props[legacyPropertyLastAuthor]
		out.Template = props[legacyPropertyTemplate]
		out.Application = props[legacyPropertyAppName]
		out.TotalEditMinutes = parseLegacyEditTimeMinutes(stream)
	}
	if stream, ok := f.Stream(legacyDocSummaryInformationStream); ok && len(stream) > 0 {
		sections := parseLegacyPropertySetSections(stream)
		if len(sections) > 0 {
			first := sections[0]
			out.Company = first[legacyDocSummaryPropertyCompany]
			out.Manager = first[legacyDocSummaryPropertyManager]
		}
		if len(sections) > 1 {
			// We need raw section bytes to parse the dictionary BLOB at PID 0.
			out.CustomProperties = parseLegacyCustomPropertiesFromStream(stream, 1)
		}
	}
	return out
}

// parseLegacyCustomPropertiesFromStream extracts custom properties from the
// given section index of a DocumentSummaryInformation stream.
func parseLegacyCustomPropertiesFromStream(data []byte, sectionIndex int) map[string]string {
	if len(data) < 48 || binary.LittleEndian.Uint16(data[0:2]) != 0xFFFE {
		return nil
	}
	sectionCount := int(binary.LittleEndian.Uint32(data[24:28]))
	if sectionCount <= 0 || sectionIndex >= sectionCount || 28+sectionCount*20 > len(data) {
		return nil
	}
	offset := int(binary.LittleEndian.Uint32(data[28+sectionIndex*20+16 : 28+sectionIndex*20+20]))
	if offset < 0 || offset+8 > len(data) {
		return nil
	}
	section := data[offset:]
	sectionSize := int(binary.LittleEndian.Uint32(section[0:4]))
	if sectionSize > 0 && sectionSize < len(section) {
		section = section[:sectionSize]
	}
	if len(section) < 8 {
		return nil
	}
	propCount := int(binary.LittleEndian.Uint32(section[4:8]))
	if propCount <= 0 || 8+propCount*8 > len(section) {
		return nil
	}
	codepage := 1252
	for index := 0; index < propCount; index++ {
		propID := binary.LittleEndian.Uint32(section[8+index*8 : 12+index*8])
		if propID != legacyPropertyCodePage {
			continue
		}
		propOffset := int(binary.LittleEndian.Uint32(section[12+index*8 : 16+index*8]))
		if propOffset < 0 || propOffset+6 > len(section) {
			continue
		}
		propType := binary.LittleEndian.Uint32(section[propOffset : propOffset+4])
		if propType != legacyVTI2 && propType != legacyVTUI2 {
			continue
		}
		if cp := int(binary.LittleEndian.Uint16(section[propOffset+4 : propOffset+6])); cp != 0 {
			codepage = cp
		}
		break
	}
	dict := parseLegacyPropertySetDict(section)
	if dict == nil {
		return nil
	}
	out := make(map[string]string)
	for index := 0; index < propCount; index++ {
		propID := binary.LittleEndian.Uint32(section[8+index*8 : 12+index*8])
		if propID == legacyDocSummaryPropertyDict {
			continue
		}
		propOffset := int(binary.LittleEndian.Uint32(section[12+index*8 : 16+index*8]))
		if propOffset < 0 || propOffset+8 > len(section) {
			continue
		}
		propType := binary.LittleEndian.Uint32(section[propOffset : propOffset+4])
		var value string
		switch propType {
		case legacyVTLPSTR:
			value = parseLegacyLPSTR(section[propOffset:], codepage)
		case legacyVTLPWSTR:
			value = parseLegacyLPWSTR(section[propOffset:])
		}
		if name, ok := dict[propID]; ok && name != "" {
			out[name] = value
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// parseLegacyPropertySetStrings parses the first section of a property set and
// returns string-valued properties keyed by PID. For multi-section sets (e.g.
// DocumentSummaryInformation) use parseLegacyPropertySetSections.
func parseLegacyPropertySetStrings(data []byte) map[uint32]string {
	sections := parseLegacyPropertySetSections(data)
	if len(sections) == 0 {
		return nil
	}
	return sections[0]
}

// parseLegacyPropertySetSections parses all sections of an OLE property set.
// Each section is returned as a map of PID → string value.
func parseLegacyPropertySetSections(data []byte) []map[uint32]string {
	if len(data) < 48 || binary.LittleEndian.Uint16(data[0:2]) != 0xFFFE {
		return nil
	}
	sectionCount := int(binary.LittleEndian.Uint32(data[24:28]))
	if sectionCount <= 0 || 28+sectionCount*20 > len(data) {
		return nil
	}
	out := make([]map[uint32]string, 0, sectionCount)
	for s := 0; s < sectionCount; s++ {
		offset := int(binary.LittleEndian.Uint32(data[28+s*20+16 : 28+s*20+20]))
		if offset < 0 || offset+8 > len(data) {
			continue
		}
		section := data[offset:]
		sectionSize := int(binary.LittleEndian.Uint32(section[0:4]))
		if sectionSize > 0 && sectionSize < len(section) {
			section = section[:sectionSize]
		}
		if len(section) < 8 {
			continue
		}
		propCount := int(binary.LittleEndian.Uint32(section[4:8]))
		if propCount <= 0 || 8+propCount*8 > len(section) {
			continue
		}
		codepage := 1252
		for index := 0; index < propCount; index++ {
			propID := binary.LittleEndian.Uint32(section[8+index*8 : 12+index*8])
			if propID != legacyPropertyCodePage {
				continue
			}
			propOffset := int(binary.LittleEndian.Uint32(section[12+index*8 : 16+index*8]))
			if propOffset < 0 || propOffset+6 > len(section) {
				continue
			}
			propType := binary.LittleEndian.Uint32(section[propOffset : propOffset+4])
			if propType != legacyVTI2 && propType != legacyVTUI2 {
				continue
			}
			if cp := int(binary.LittleEndian.Uint16(section[propOffset+4 : propOffset+6])); cp != 0 {
				codepage = cp
			}
			break
		}
		sectionMap := map[uint32]string{}
		for index := 0; index < propCount; index++ {
			propID := binary.LittleEndian.Uint32(section[8+index*8 : 12+index*8])
			propOffset := int(binary.LittleEndian.Uint32(section[12+index*8 : 16+index*8]))
			if propOffset < 0 || propOffset+8 > len(section) {
				continue
			}
			propType := binary.LittleEndian.Uint32(section[propOffset : propOffset+4])
			var value string
			switch propType {
			case legacyVTLPSTR:
				value = parseLegacyLPSTR(section[propOffset:], codepage)
			case legacyVTLPWSTR:
				value = parseLegacyLPWSTR(section[propOffset:])
			}
			if value != "" {
				sectionMap[propID] = value
			}
		}
		if len(sectionMap) > 0 {
			out = append(out, sectionMap)
		}
	}
	return out
}

// parseLegacyPropertySetDict extracts the PID→name dictionary from a
// DocumentSummaryInformation section. The dictionary is stored as a BLOB at
// PID 0 (or as a vector|variant). Returns nil if no dictionary is found.
func parseLegacyPropertySetDict(section []byte) map[uint32]string {
	if len(section) < 8 {
		return nil
	}
	propCount := int(binary.LittleEndian.Uint32(section[4:8]))
	if propCount <= 0 || 8+propCount*8 > len(section) {
		return nil
	}
	for index := 0; index < propCount; index++ {
		propID := binary.LittleEndian.Uint32(section[8+index*8 : 12+index*8])
		if propID != legacyDocSummaryPropertyDict {
			continue
		}
		propOffset := int(binary.LittleEndian.Uint32(section[12+index*8 : 16+index*8]))
		if propOffset < 0 || propOffset+8 > len(section) {
			continue
		}
		propType := binary.LittleEndian.Uint32(section[propOffset : propOffset+4])
		// Dictionary may be VT_BLOB (0x41) or VT_VECTOR|VT_VARIANT (0x101C).
		if propType != legacyVTBLOB && propType != (legacyVTVECTOR|legacyVTVARIANT) {
			continue
		}
		// Read BLOB size and data.
		blobSize := int(binary.LittleEndian.Uint32(section[propOffset+4 : propOffset+8]))
		if blobSize <= 0 || propOffset+8+blobSize > len(section) {
			continue
		}
		blob := section[propOffset+8 : propOffset+8+blobSize]
		if len(blob) < 4 {
			continue
		}
		cEntries := int(binary.LittleEndian.Uint32(blob[0:4]))
		pos := 4
		out := make(map[uint32]string)
		for i := 0; i < cEntries && pos+8 <= len(blob); i++ {
			pid := binary.LittleEndian.Uint32(blob[pos : pos+4])
			cbName := int(binary.LittleEndian.Uint32(blob[pos+4 : pos+8]))
			pos += 8
			if cbName <= 0 || pos+cbName > len(blob) {
				break
			}
			nameBytes := blob[pos : pos+cbName]
			pos += cbName
			// nameBytes is UTF-16LE, null-terminated.
			words := make([]uint16, 0, len(nameBytes)/2)
			for j := 0; j+1 < len(nameBytes); j += 2 {
				v := binary.LittleEndian.Uint16(nameBytes[j : j+2])
				if v == 0 {
					break
				}
				words = append(words, v)
			}
			if len(words) > 0 {
				out[pid] = string(utf16.Decode(words))
			}
		}
		return out
	}
	return nil
}

// parseLegacyCustomProperties extracts custom property names and values from
// the second section of DocumentSummaryInformation. The section bytes must
// contain a PID dictionary (PID 0) that maps numeric PIDs to property names.
func parseLegacyCustomProperties(sectionBytes []byte, sectionStrings map[uint32]string) map[string]string {
	if sectionStrings == nil {
		return nil
	}
	dict := parseLegacyPropertySetDict(sectionBytes)
	if dict == nil {
		return nil
	}
	out := make(map[string]string)
	for pid, value := range sectionStrings {
		if pid == legacyDocSummaryPropertyDict {
			continue
		}
		if name, ok := dict[pid]; ok && name != "" {
			out[name] = value
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func parseLegacyLPSTR(data []byte, codepage int) string {
	if len(data) < 8 {
		return ""
	}
	size := int(binary.LittleEndian.Uint32(data[4:8]))
	if size <= 0 || 8+size > len(data) {
		return ""
	}
	raw := append([]byte(nil), data[8:8+size]...)
	if idx := bytesIndexByte(raw, 0); idx >= 0 {
		raw = raw[:idx]
	}
	return strings.TrimSpace(decodeLegacyPropertyBytes(raw, codepage))
}

func parseLegacyLPWSTR(data []byte) string {
	if len(data) < 8 {
		return ""
	}
	charCount := int(binary.LittleEndian.Uint32(data[4:8]))
	if charCount <= 0 {
		return ""
	}
	byteCount := charCount * 2
	if 8+byteCount > len(data) {
		return ""
	}
	raw := data[8 : 8+byteCount]
	words := make([]uint16, 0, len(raw)/2)
	for i := 0; i+1 < len(raw); i += 2 {
		v := binary.LittleEndian.Uint16(raw[i : i+2])
		if v == 0 {
			break
		}
		words = append(words, v)
	}
	if len(words) == 0 {
		return ""
	}
	return strings.TrimSpace(string(utf16.Decode(words)))
}

func decodeLegacyPropertyBytes(data []byte, codepage int) string {
	if len(data) == 0 {
		return ""
	}
	switch codepage {
	case 65001:
		return string(data)
	case 1200:
		return parseLegacyLPWSTR(append([]byte{0, 0, 0, 0}, append(le32Bytes(uint32(len(data)/2)), data...)...))
	case 1250:
		decoded, _ := charmap.Windows1250.NewDecoder().Bytes(data)
		return string(decoded)
	case 1251:
		decoded, _ := charmap.Windows1251.NewDecoder().Bytes(data)
		return string(decoded)
	case 1253:
		decoded, _ := charmap.Windows1253.NewDecoder().Bytes(data)
		return string(decoded)
	case 1254:
		decoded, _ := charmap.Windows1254.NewDecoder().Bytes(data)
		return string(decoded)
	case 1255:
		decoded, _ := charmap.Windows1255.NewDecoder().Bytes(data)
		return string(decoded)
	case 1256:
		decoded, _ := charmap.Windows1256.NewDecoder().Bytes(data)
		return string(decoded)
	case 1257:
		decoded, _ := charmap.Windows1257.NewDecoder().Bytes(data)
		return string(decoded)
	case 1258:
		decoded, _ := charmap.Windows1258.NewDecoder().Bytes(data)
		return string(decoded)
	case 437:
		decoded, _ := charmap.CodePage437.NewDecoder().Bytes(data)
		return string(decoded)
	case 850:
		decoded, _ := charmap.CodePage850.NewDecoder().Bytes(data)
		return string(decoded)
	case 852:
		decoded, _ := charmap.CodePage852.NewDecoder().Bytes(data)
		return string(decoded)
	case 855:
		decoded, _ := charmap.CodePage855.NewDecoder().Bytes(data)
		return string(decoded)
	case 858:
		decoded, _ := charmap.CodePage858.NewDecoder().Bytes(data)
		return string(decoded)
	case 860:
		decoded, _ := charmap.CodePage860.NewDecoder().Bytes(data)
		return string(decoded)
	case 862:
		decoded, _ := charmap.CodePage862.NewDecoder().Bytes(data)
		return string(decoded)
	case 863:
		decoded, _ := charmap.CodePage863.NewDecoder().Bytes(data)
		return string(decoded)
	case 865:
		decoded, _ := charmap.CodePage865.NewDecoder().Bytes(data)
		return string(decoded)
	case 866:
		decoded, _ := charmap.CodePage866.NewDecoder().Bytes(data)
		return string(decoded)
	case 874:
		decoded, _ := charmap.Windows874.NewDecoder().Bytes(data)
		return string(decoded)
	case 10000:
		decoded, _ := charmap.Macintosh.NewDecoder().Bytes(data)
		return string(decoded)
	default:
		decoded, _ := charmap.Windows1252.NewDecoder().Bytes(data)
		return string(decoded)
	}
}

// parseLegacyEditTimeMinutes scans a SummaryInformation property-set stream for
// PID 10 (EditTime) and converts it to minutes. The property may be stored as
// VT_FILETIME (8-byte Windows FILETIME) or VT_I4 (4-byte signed integer).
func parseLegacyEditTimeMinutes(data []byte) int {
	if len(data) < 48 || binary.LittleEndian.Uint16(data[0:2]) != 0xFFFE {
		return 0
	}
	sectionCount := int(binary.LittleEndian.Uint32(data[24:28]))
	if sectionCount <= 0 || 28+sectionCount*20 > len(data) {
		return 0
	}
	// Only the first section matters for SummaryInformation.
	offset := int(binary.LittleEndian.Uint32(data[28+16 : 28+20]))
	if offset < 0 || offset+8 > len(data) {
		return 0
	}
	section := data[offset:]
	sectionSize := int(binary.LittleEndian.Uint32(section[0:4]))
	if sectionSize > 0 && sectionSize < len(section) {
		section = section[:sectionSize]
	}
	if len(section) < 8 {
		return 0
	}
	propCount := int(binary.LittleEndian.Uint32(section[4:8]))
	if propCount <= 0 || 8+propCount*8 > len(section) {
		return 0
	}
	for index := 0; index < propCount; index++ {
		propID := binary.LittleEndian.Uint32(section[8+index*8 : 12+index*8])
		if propID != legacyPropertyEditTime {
			continue
		}
		propOffset := int(binary.LittleEndian.Uint32(section[12+index*8 : 16+index*8]))
		if propOffset < 0 || propOffset+8 > len(section) {
			return 0
		}
		propType := binary.LittleEndian.Uint32(section[propOffset : propOffset+4])
		switch propType {
		case legacyVTFILETIME:
			if propOffset+12 > len(section) {
				return 0
			}
			ft := binary.LittleEndian.Uint64(section[propOffset+4 : propOffset+12])
			// FILETIME is 100-ns intervals; convert to minutes.
			return int((ft / 10_000_000) / 60)
		case legacyVTI4:
			if propOffset+8 > len(section) {
				return 0
			}
			return int(int32(binary.LittleEndian.Uint32(section[propOffset+4 : propOffset+8])))
		}
	}
	return 0
}

func le32Bytes(v uint32) []byte {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, v)
	return buf
}
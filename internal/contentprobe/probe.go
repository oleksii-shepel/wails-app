package contentprobe

import (
	"bytes"
	"encoding/base64"
	"io"
	"math"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/wailsapp/mimetype"
	"golang.org/x/text/encoding/charmap"
	xunicode "golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

const SampleLimit = 64 * 1024

type Result struct {
	MimeType  string `json:"mimeType"`
	Encoding  string `json:"encoding,omitempty"`
	IsText    bool   `json:"isText"`
	IsArchive bool   `json:"isArchive"`
}

func ProbeReader(r io.Reader) ([]byte, Result, error) {
	sample, err := io.ReadAll(io.LimitReader(r, SampleLimit))
	if err != nil {
		return nil, Result{}, err
	}
	return sample, ProbeBytes(sample), nil
}

func ProbeBytes(sample []byte) Result {
	mimeType := detectMime(sample)
	res := Result{
		MimeType: mimeType,
		IsText:   isTextMime(mimeType) || looksLikeText(sample),
	}
	res.IsArchive = isArchiveMime(mimeType) || looksLikeArchive(sample)
	if res.IsText {
		res.Encoding = detectEncoding(sample)
	}
	return res
}

func DetectEncoding(sample []byte) string {
	return detectEncoding(sample)
}

func Decode(sample []byte, encoding string) (string, error) {
	normalized := normalizeEncoding(encoding)
	if normalized == "" || normalized == "utf8" {
		return string(stripBOM(sample)), nil
	}
	if normalized == "base64" {
		return base64.StdEncoding.EncodeToString(sample), nil
	}

	reader := bytes.NewReader(stripBOM(sample))
	switch normalized {
	case "utf16", "utf16le":
		return decodeReader(reader, xunicode.UTF16(xunicode.LittleEndian, xunicode.IgnoreBOM).NewDecoder())
	case "utf16be":
		return decodeReader(reader, xunicode.UTF16(xunicode.BigEndian, xunicode.IgnoreBOM).NewDecoder())
	case "windows-1252", "cp1252":
		return decodeReader(reader, charmap.Windows1252.NewDecoder())
	case "iso-8859-1", "latin1", "latin-1":
		return decodeReader(reader, charmap.ISO8859_1.NewDecoder())
	default:
		return string(sample), nil
	}
}

func normalizeEncoding(raw string) string {
	enc := strings.ToLower(strings.TrimSpace(raw))
	enc = strings.ReplaceAll(enc, "_", "-")
	switch enc {
	case "utf-8":
		return "utf8"
	case "utf16", "utf-16":
		return "utf16"
	case "utf-16le":
		return "utf16le"
	case "utf-16be":
		return "utf16be"
	case "windows1252":
		return "windows-1252"
	case "iso8859-1":
		return "iso-8859-1"
	case "latin-1":
		return "latin1"
	}
	return enc
}

func detectMime(sample []byte) string {
	if mimeType := detectArchiveMime(sample); mimeType != "" {
		return mimeType
	}
	if looksLikeMPEGTS(sample) {
		return "video/mp2t"
	}

	detected := mimetype.Detect(sample)
	if detected != nil {
		mimeType := normalizeMime(detected.String())
		if mimeType != "" && mimeType != "application/octet-stream" {
			return mimeType
		}
	}

	if looksLikeText(sample) {
		return "text/plain"
	}
	return "application/octet-stream"
}

func detectArchiveMime(sample []byte) string {
	switch {
	case hasPrefix(sample, []byte("PK\x03\x04")), hasPrefix(sample, []byte("PK\x05\x06")), hasPrefix(sample, []byte("PK\x07\x08")):
		return "application/zip"
	case hasPrefix(sample, []byte("Rar!\x1A\x07\x00")), hasPrefix(sample, []byte("Rar!\x1A\x07\x01\x00")):
		return "application/vnd.rar"
	case hasPrefix(sample, []byte("7z\xBC\xAF'\x1C")):
		return "application/x-7z-compressed"
	case hasPrefix(sample, []byte{0x1F, 0x8B}):
		return "application/gzip"
	case hasPrefix(sample, []byte{0xFD, 0x37, 0x7A, 0x58, 0x5A, 0x00}):
		return "application/x-xz"
	case hasPrefix(sample, []byte("BZh")):
		return "application/x-bzip2"
	case looksLikeTar(sample):
		return "application/x-tar"
	default:
		return ""
	}
}

func isArchiveMime(mimeType string) bool {
	switch normalizeMime(mimeType) {
	case "application/zip",
		"application/vnd.rar",
		"application/x-7z-compressed",
		"application/gzip",
		"application/x-xz",
		"application/x-bzip2",
		"application/x-tar":
		return true
	default:
		return false
	}
}

func looksLikeArchive(sample []byte) bool {
	return detectArchiveMime(sample) != ""
}

func looksLikeTar(sample []byte) bool {
	if len(sample) < 512 {
		return false
	}
	return bytes.Contains(sample[:512], []byte("ustar"))
}

func hasPrefix(sample []byte, prefix []byte) bool {
	return len(sample) >= len(prefix) && bytes.Equal(sample[:len(prefix)], prefix)
}

func normalizeMime(raw string) string {
	if raw == "" {
		return ""
	}
	if i := strings.Index(raw, ";"); i >= 0 {
		raw = raw[:i]
	}
	return strings.ToLower(strings.TrimSpace(raw))
}

func isTextMime(mimeType string) bool {
	mimeType = normalizeMime(mimeType)
	switch {
	case strings.HasPrefix(mimeType, "text/"):
		return true
	case strings.HasSuffix(mimeType, "+json"), strings.HasSuffix(mimeType, "+xml"):
		return true
	case mimeType == "application/json",
		mimeType == "application/xml",
		mimeType == "application/javascript",
		mimeType == "application/ecmascript",
		mimeType == "application/x-javascript",
		mimeType == "application/x-www-form-urlencoded",
		mimeType == "application/yaml",
		mimeType == "application/x-yaml":
		return true
	default:
		return false
	}
}

func detectEncoding(sample []byte) string {
	if len(sample) == 0 {
		return "utf8"
	}

	switch {
	case bytes.HasPrefix(sample, []byte{0xEF, 0xBB, 0xBF}):
		return "utf8"
	case bytes.HasPrefix(sample, []byte{0xFF, 0xFE}):
		return "utf16le"
	case bytes.HasPrefix(sample, []byte{0xFE, 0xFF}):
		return "utf16be"
	}

	if looksLikeUTF16LE(sample) && decodedLooksReasonable(sample, xunicode.UTF16(xunicode.LittleEndian, xunicode.IgnoreBOM).NewDecoder()) {
		return "utf16le"
	}
	if looksLikeUTF16BE(sample) && decodedLooksReasonable(sample, xunicode.UTF16(xunicode.BigEndian, xunicode.IgnoreBOM).NewDecoder()) {
		return "utf16be"
	}
	if utf8.Valid(sample) {
		return "utf8"
	}
	if looksLikeText(sample) {
		return detectSingleByteEncoding(sample)
	}
	return "utf8"
}

func looksLikeText(sample []byte) bool {
	if len(sample) == 0 {
		return true
	}

	ctrl := 0
	printable := 0
	for _, b := range sample {
		switch {
		case b == 0:
			return false
		case b == '\n', b == '\r', b == '\t', b >= 0x20:
			printable++
		default:
			ctrl++
		}
	}

	if printable == 0 {
		return false
	}
	return float64(ctrl)/float64(len(sample)) < 0.15
}

func looksLikeMPEGTS(sample []byte) bool {
	if len(sample) < 188*3 {
		return false
	}

	limit := len(sample) / 188
	if limit > 12 {
		limit = 12
	}

	matches := 0
	for i := 0; i < limit; i++ {
		if sample[i*188] == 0x47 {
			matches++
		}
	}

	return matches >= 3 && float64(matches)/float64(limit) >= 0.6
}

func looksLikeUTF16LE(sample []byte) bool {
	if len(sample) < 4 {
		return false
	}

	zerosEven := 0
	zerosOdd := 0
	for i, b := range sample {
		if b != 0 {
			continue
		}
		if i%2 == 0 {
			zerosEven++
		} else {
			zerosOdd++
		}
	}

	return zerosOdd > zerosEven*2 && zerosOdd >= len(sample)/8
}

func looksLikeUTF16BE(sample []byte) bool {
	if len(sample) < 4 {
		return false
	}

	zerosEven := 0
	zerosOdd := 0
	for i, b := range sample {
		if b != 0 {
			continue
		}
		if i%2 == 0 {
			zerosEven++
		} else {
			zerosOdd++
		}
	}

	return zerosEven > zerosOdd*2 && zerosEven >= len(sample)/8
}

func decodeReader(r io.Reader, decoder transform.Transformer) (string, error) {
	decoded, err := io.ReadAll(transform.NewReader(r, decoder))
	if err != nil {
		return "", err
	}
	if utf8.Valid(decoded) {
		return string(decoded), nil
	}
	return string(decoded), nil
}

func detectSingleByteEncoding(sample []byte) string {
	cp1252, cp1252OK := tryDecode(sample, charmap.Windows1252.NewDecoder())
	latin1, latin1OK := tryDecode(sample, charmap.ISO8859_1.NewDecoder())

	if !hasCP1252OnlyBytes(sample) {
		if latin1OK {
			return "iso-8859-1"
		}
		return "windows-1252"
	}

	if cp1252OK && decodedTextScore(cp1252) >= decodedTextScore(latin1)-0.05 {
		return "windows-1252"
	}

	if latin1OK && decodedTextScore(latin1) > decodedTextScore(cp1252)+0.2 {
		return "iso-8859-1"
	}

	return "windows-1252"
}

func decodedLooksReasonable(sample []byte, decoder transform.Transformer) bool {
	decoded, ok := tryDecode(sample, decoder)
	return ok && decodedTextScore(decoded) > 0.6
}

func tryDecode(sample []byte, decoder transform.Transformer) (string, bool) {
	decoded, err := io.ReadAll(transform.NewReader(bytes.NewReader(stripBOM(sample)), decoder))
	if err != nil {
		return "", false
	}
	if !utf8.Valid(decoded) {
		return "", false
	}
	return string(decoded), true
}

func hasCP1252OnlyBytes(sample []byte) bool {
	for _, b := range sample {
		if b >= 0x80 && b <= 0x9F {
			return true
		}
	}
	return false
}

func decodedTextScore(s string) float64 {
	if s == "" {
		return 1
	}

	printable := 0
	letters := 0
	controls := 0
	replacement := 0
	total := 0

	for _, r := range s {
		total++
		switch {
		case r == utf8.RuneError:
			replacement++
		case r == '\n' || r == '\r' || r == '\t':
			printable++
		case unicode.IsPrint(r):
			printable++
			if unicode.IsLetter(r) || unicode.IsNumber(r) {
				letters++
			}
		case unicode.IsControl(r):
			controls++
		}
	}

	if total == 0 {
		return 1
	}

	printableRatio := float64(printable) / float64(total)
	letterRatio := float64(letters) / float64(total)
	controlRatio := float64(controls) / float64(total)
	replacementRatio := float64(replacement) / float64(total)

	score := printableRatio + math.Min(letterRatio, 0.35) - (controlRatio * 2.0) - (replacementRatio * 3.0)
	if score < 0 {
		return 0
	}
	if score > 1 {
		return 1
	}
	return score
}

func stripBOM(sample []byte) []byte {
	switch {
	case bytes.HasPrefix(sample, []byte{0xEF, 0xBB, 0xBF}):
		return sample[3:]
	case bytes.HasPrefix(sample, []byte{0xFF, 0xFE}), bytes.HasPrefix(sample, []byte{0xFE, 0xFF}):
		return sample[2:]
	default:
		return sample
	}
}

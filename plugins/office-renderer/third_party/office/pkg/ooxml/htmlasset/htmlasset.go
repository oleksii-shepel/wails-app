package htmlasset

import (
	"fmt"
	"path"
	"strings"
)

// Asset is a sidecar file emitted alongside rendered HTML.
type Asset struct {
	Path        string
	ContentType string
	Data        []byte
}

// NormalizeDir trims whitespace and normalizes path separators for an HTML
// asset directory.
func NormalizeDir(dir string) string {
	dir = strings.ReplaceAll(strings.TrimSpace(dir), "\\", "/")
	dir = strings.TrimPrefix(dir, "./")
	return strings.Trim(dir, "/")
}

// AssetPath builds a deterministic sidecar asset path under dir.
func AssetPath(dir, prefix string, index int, ext string) string {
	name := fmt.Sprintf("%s-%03d", prefix, index)
	if ext != "" {
		name += "." + ext
	}
	dir = NormalizeDir(dir)
	if dir == "" {
		return name
	}
	return path.Join(dir, name)
}

// ImageFileExtension maps an image MIME type to a file extension.
func ImageFileExtension(contentType string) string {
	baseType := strings.ToLower(strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0]))
	switch baseType {
	case "image/png":
		return "png"
	case "image/jpeg", "image/jpg":
		return "jpg"
	case "image/gif":
		return "gif"
	case "image/webp":
		return "webp"
	case "image/svg+xml":
		return "svg"
	case "image/bmp":
		return "bmp"
	case "image/tiff":
		return "tiff"
	case "image/avif":
		return "avif"
	default:
		return "bin"
	}
}

// MediaFileExtension maps an audio/video MIME type to a file extension.
func MediaFileExtension(contentType string) string {
	baseType := strings.ToLower(strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0]))
	switch baseType {
	case "video/mp4":
		return "mp4"
	case "video/webm":
		return "webm"
	case "video/ogg":
		return "ogv"
	case "audio/mpeg", "audio/mp3":
		return "mp3"
	case "audio/mp4":
		return "m4a"
	case "audio/ogg":
		return "ogg"
	case "audio/wav", "audio/x-wav":
		return "wav"
	case "audio/webm":
		return "weba"
	default:
		return "bin"
	}
}

// FontFileExtension maps a font MIME type or OpenType/TrueType format label to
// a browser-friendly file extension.
func FontFileExtension(contentType, format string) string {
	baseType := strings.ToLower(strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0]))
	switch baseType {
	case "font/woff2":
		return "woff2"
	case "font/woff":
		return "woff"
	case "font/opentype":
		return "otf"
	case "font/truetype":
		return "ttf"
	case "application/vnd.ms-fontobject":
		return "eot"
	}
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "woff2":
		return "woff2"
	case "woff":
		return "woff"
	case "opentype":
		return "otf"
	case "truetype":
		return "ttf"
	case "embedded-opentype":
		return "eot"
	}
	return "bin"
}

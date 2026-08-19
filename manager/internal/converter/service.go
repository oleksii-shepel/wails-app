package converter

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	"image/png"
	_ "image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"wails-app/internal/utility"
)

// Service handles image format conversion for preview.
type Service struct {
	vipsPath          string
	vipsThumbnailPath string
	hasVips           bool
	hasVipsThumbnail  bool
	detectOnce        sync.Once
}

// NewService creates a new image converter service.
func NewService() *Service {
	return &Service{}
}

func (s *Service) detectVips() {
	vipsPaths := []string{
		"vips",
		"vips.exe",
		`C:\Program Files\libvips\bin\vips.exe`,
		`C:\Program Files\vips-dev-8.17\bin\vips.exe`,
		`C:\vips-dev-8.17\bin\vips.exe`,
		"/usr/bin/vips",
		"/usr/local/bin/vips",
	}
	if path, ok := utility.ResolveExecutable(vipsPaths...); ok {
		s.vipsPath = path
		s.hasVips = true
	}

	thumbnailPaths := []string{
		"vipsthumbnail",
		"vipsthumbnail.exe",
		`C:\Program Files\libvips\bin\vipsthumbnail.exe`,
		`C:\Program Files\vips-dev-8.17\bin\vipsthumbnail.exe`,
		`C:\vips-dev-8.17\bin\vipsthumbnail.exe`,
		"/usr/bin/vipsthumbnail",
		"/usr/local/bin/vipsthumbnail",
	}
	if path, ok := utility.ResolveExecutable(thumbnailPaths...); ok {
		s.vipsThumbnailPath = path
		s.hasVipsThumbnail = true
	}
}

func (s *Service) ensureDetected() {
	s.detectOnce.Do(s.detectVips)
}

// ConvertFormat converts image from one format to another.
// Supported output formats: webp, png, jpg, jpeg, bmp, gif, avif (webp recommended for previews)
func (s *Service) ConvertFormat(inputPath string, outputFormat string) ([]byte, error) {
	outputFormat = strings.ToLower(outputFormat)

	// Validate output format
	if !isSupportedOutputFormat(outputFormat) {
		return nil, fmt.Errorf("unsupported output format: %s", outputFormat)
	}

	// Try libvips first for broader format coverage.
	s.ensureDetected()
	if s.hasVips {
		return s.convertWithVips(inputPath, outputFormat)
	}

	// Fallback to Go native decoders for basic formats
	return s.convertWithGoImage(inputPath, outputFormat)
}

// CreateThumbnail converts and resizes an image to fit within the provided bounds.
func (s *Service) CreateThumbnail(inputPath string, outputFormat string, maxWidth int, maxHeight int) ([]byte, error) {
	outputFormat = strings.ToLower(outputFormat)
	maxWidth, maxHeight = normalizeThumbnailBounds(maxWidth, maxHeight)

	if !isSupportedOutputFormat(outputFormat) {
		return nil, fmt.Errorf("unsupported output format: %s", outputFormat)
	}

	s.ensureDetected()
	if s.hasVipsThumbnail {
		return s.thumbnailWithVips(inputPath, outputFormat, maxWidth, maxHeight)
	}

	return s.thumbnailWithGoImage(inputPath, outputFormat, maxWidth, maxHeight)
}

func (s *Service) convertWithVips(inputPath string, outputFormat string) ([]byte, error) {
	outputPath, cleanup, err := createConvertedOutputPath(outputFormat)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	cmd := exec.Command(s.vipsPath, "copy", inputPath, outputPath)
	hideWindow(cmd)

	var errOut bytes.Buffer
	cmd.Stderr = &errOut

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("libvips conversion failed: %v - %s", err, strings.TrimSpace(errOut.String()))
	}

	converted, err := os.ReadFile(outputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read converted image: %w", err)
	}
	return converted, nil
}

func (s *Service) thumbnailWithVips(inputPath string, outputFormat string, maxWidth int, maxHeight int) ([]byte, error) {
	outputPath, cleanup, err := createConvertedOutputPath(outputFormat)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	size := fmt.Sprintf("%dx%d", maxWidth, maxHeight)
	cmd := exec.Command(s.vipsThumbnailPath, inputPath, "--size", size, "-o", outputPath)
	hideWindow(cmd)

	var errOut bytes.Buffer
	cmd.Stderr = &errOut

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("libvips thumbnail failed: %v - %s", err, strings.TrimSpace(errOut.String()))
	}

	thumbnail, err := os.ReadFile(outputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read converted thumbnail: %w", err)
	}
	return thumbnail, nil
}

// convertWithGoImage uses Go's native image decoders (PNG, JPEG, GIF).
func (s *Service) convertWithGoImage(inputPath string, outputFormat string) ([]byte, error) {
	// Read the source image
	file, err := os.Open(inputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open source image: %w", err)
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	// Encode to target format
	var out bytes.Buffer
	switch outputFormat {
	case "png":
		if err := png.Encode(&out, img); err != nil {
			return nil, fmt.Errorf("failed to encode PNG: %w", err)
		}
	case "jpg", "jpeg":
		if err := jpeg.Encode(&out, img, &jpeg.Options{Quality: 90}); err != nil {
			return nil, fmt.Errorf("failed to encode JPEG: %w", err)
		}
	default:
		return nil, fmt.Errorf("go native conversion: unsupported format %s (install libvips for broader format support)", outputFormat)
	}

	return out.Bytes(), nil
}

func (s *Service) thumbnailWithGoImage(inputPath string, outputFormat string, maxWidth int, maxHeight int) ([]byte, error) {
	file, err := os.Open(inputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open source image: %w", err)
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	thumb := resizeToFit(img, maxWidth, maxHeight)

	var out bytes.Buffer
	switch outputFormat {
	case "png":
		if err := png.Encode(&out, thumb); err != nil {
			return nil, fmt.Errorf("failed to encode PNG: %w", err)
		}
	case "jpg", "jpeg":
		if err := jpeg.Encode(&out, thumb, &jpeg.Options{Quality: 90}); err != nil {
			return nil, fmt.Errorf("failed to encode JPEG: %w", err)
		}
	default:
		return nil, fmt.Errorf("go native thumbnail: unsupported format %s (install libvips for broader format support)", outputFormat)
	}

	return out.Bytes(), nil
}

func normalizeThumbnailBounds(maxWidth int, maxHeight int) (int, int) {
	if maxWidth <= 0 {
		maxWidth = 256
	}
	if maxHeight <= 0 {
		maxHeight = 256
	}
	if maxWidth > 2048 {
		maxWidth = 2048
	}
	if maxHeight > 2048 {
		maxHeight = 2048
	}
	return maxWidth, maxHeight
}

func resizeToFit(src image.Image, maxWidth int, maxHeight int) image.Image {
	bounds := src.Bounds()
	srcWidth := bounds.Dx()
	srcHeight := bounds.Dy()
	if srcWidth <= 0 || srcHeight <= 0 {
		return src
	}

	scale := math.Min(float64(maxWidth)/float64(srcWidth), float64(maxHeight)/float64(srcHeight))
	if scale >= 1 {
		return src
	}

	dstWidth := int(math.Max(1, math.Round(float64(srcWidth)*scale)))
	dstHeight := int(math.Max(1, math.Round(float64(srcHeight)*scale)))
	dst := image.NewRGBA(image.Rect(0, 0, dstWidth, dstHeight))

	for y := 0; y < dstHeight; y++ {
		srcY := bounds.Min.Y + (y*srcHeight)/dstHeight
		for x := 0; x < dstWidth; x++ {
			srcX := bounds.Min.X + (x*srcWidth)/dstWidth
			dst.Set(x, y, src.At(srcX, srcY))
		}
	}

	return dst
}

func createConvertedOutputPath(outputFormat string) (string, func(), error) {
	dir, err := os.MkdirTemp("", "wails-image-converter-*")
	if err != nil {
		return "", func() {}, fmt.Errorf("failed to create temp output directory: %w", err)
	}
	outputPath := filepath.Join(dir, "output."+normalizeOutputExtension(outputFormat))
	cleanup := func() {
		_ = os.RemoveAll(dir)
	}
	return outputPath, cleanup, nil
}

func normalizeOutputExtension(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "jpeg":
		return "jpg"
	case "tiff":
		return "tif"
	default:
		return strings.ToLower(strings.TrimSpace(format))
	}
}

// isSupportedOutputFormat checks if the format can be converted to.
func isSupportedOutputFormat(format string) bool {
	supported := map[string]bool{
		"png":  true,
		"jpg":  true,
		"jpeg": true,
		"gif":  true,
		"webp": true,
		"bmp":  true,
		"avif": true,
		"tiff": true,
		"tif":  true,
	}
	return supported[format]
}

// IsSupportedSourceFormat checks if the source format can be converted.
func (s *Service) IsSupportedSourceFormat(extension string) bool {
	extension = strings.ToLower(extension)

	// Basic Go support
	goSupported := map[string]bool{
		"png":  true,
		"jpg":  true,
		"jpeg": true,
		"gif":  true,
		"bmp":  true,
	}

	if goSupported[extension] {
		return true
	}

	// Extended support with libvips.
	s.ensureDetected()
	if s.hasVips {
		extended := map[string]bool{
			"webp": true,
			"tiff": true,
			"tif":  true,
			"exr":  true, // OpenEXR
			"heif": true,
			"heic": true,
			"jp2":  true, // JPEG 2000
			"j2k":  true, // JPEG 2000
			"avif": true,
			"jxl":  true, // JPEG XL
			"fits": true,
			"hdr":  true,
			"pfm":  true,
		}
		return extended[extension]
	}

	return false
}

// GetAvailableConversionFormats returns the list of formats this source can be converted to.
// Depends on libvips availability.
func (s *Service) GetAvailableConversionFormats(sourceExt string) []string {
	// Always available if we can convert at all
	if s.IsSupportedSourceFormat(sourceExt) {
		return []string{"png", "jpg", "jpeg", "gif", "webp", "bmp"}
	}
	return []string{}
}
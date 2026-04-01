package converter

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	_ "image/png"
	"math"
	"os"
	"os/exec"
	"strings"
	"sync"

	"wails-app/internal/utility"
)

// Service handles image format conversion for preview.
type Service struct {
	imagemagickPath string
	hasImagemagick  bool
	detectOnce      sync.Once
}

// NewService creates a new image converter service.
func NewService() *Service {
	return &Service{}
}

func (s *Service) detectImagemagick() {
	// Try to locate ImageMagick's convert command
	paths := []string{"magick", "convert"}
	if path, ok := utility.ResolveExecutable(paths...); ok {
		s.imagemagickPath = path
		s.hasImagemagick = true
		return
	}
}

func (s *Service) ensureDetected() {
	s.detectOnce.Do(s.detectImagemagick)
}

// ConvertFormat converts image from one format to another.
// Supported output formats: png, jpg, jpeg, webp, bmp, gif, avif
func (s *Service) ConvertFormat(inputPath string, outputFormat string) ([]byte, error) {
	outputFormat = strings.ToLower(outputFormat)

	// Validate output format
	if !isSupportedOutputFormat(outputFormat) {
		return nil, fmt.Errorf("unsupported output format: %s", outputFormat)
	}

	// Try ImageMagick first (best compatibility)
	s.ensureDetected()
	if s.hasImagemagick {
		return s.convertWithImagemagick(inputPath, outputFormat)
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
	if s.hasImagemagick {
		return s.thumbnailWithImagemagick(inputPath, outputFormat, maxWidth, maxHeight)
	}

	return s.thumbnailWithGoImage(inputPath, outputFormat, maxWidth, maxHeight)
}

// convertWithImagemagick uses ImageMagick to convert the image.
func (s *Service) convertWithImagemagick(inputPath string, outputFormat string) ([]byte, error) {
	cmd := exec.Command(s.imagemagickPath, inputPath, fmt.Sprintf("%s:-", outputFormat))
	hideWindow(cmd)

	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("imagemagick conversion failed: %v - %s", err, errOut.String())
	}

	return out.Bytes(), nil
}

func (s *Service) thumbnailWithImagemagick(inputPath string, outputFormat string, maxWidth int, maxHeight int) ([]byte, error) {
	geometry := fmt.Sprintf("%dx%d>", maxWidth, maxHeight)
	cmd := exec.Command(s.imagemagickPath, inputPath, "-auto-orient", "-thumbnail", geometry, fmt.Sprintf("%s:-", outputFormat))
	hideWindow(cmd)

	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("imagemagick thumbnail failed: %v - %s", err, errOut.String())
	}

	return out.Bytes(), nil
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
	default:
		return nil, fmt.Errorf("go native conversion: unsupported format %s (install ImageMagick for broader format support)", outputFormat)
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
	default:
		return nil, fmt.Errorf("go native thumbnail: unsupported format %s (install ImageMagick for broader format support)", outputFormat)
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

	// Extended support with ImageMagick
	s.ensureDetected()
	if s.hasImagemagick {
		extended := map[string]bool{
			"psd":  true, // Photoshop
			"ai":   true, // Illustrator
			"eps":  true, // EPS
			"xcf":  true, // GIMP
			"webp": true,
			"tiff": true,
			"tif":  true,
			"dng":  true, // RAW
			"tga":  true, // TARGA
			"icns": true, // Apple Icon
			"pnm":  true, // Portable Bitmap
			"dds":  true, // DirectDraw Surface
			"xwd":  true, // X Window Dump
			"scr":  true, // ZX-Spectrum
			"xpm":  true, // X Pixmap
			"exr":  true, // OpenEXR
			"heif": true,
			"heic": true,
			"jp2":  true, // JPEG 2000
			"j2k":  true, // JPEG 2000
			"avif": true,
			"age":  true,
		}
		return extended[extension]
	}

	return false
}

// GetAvailableConversionFormats returns the list of formats this source can be converted to.
// Depends on ImageMagick availability.
func (s *Service) GetAvailableConversionFormats(sourceExt string) []string {
	// Always available if we can convert at all
	if s.IsSupportedSourceFormat(sourceExt) {
		return []string{"png", "jpg", "jpeg", "gif", "webp", "bmp"}
	}
	return []string{}
}

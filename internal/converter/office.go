package converter

import (
	"fmt"
	oslib "os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// OfficeService handles conversion of supported documents to PDF.
type OfficeService struct {
	sofficeExePath     string
	calibreExePath     string
	imagemagickExePath string
	hasLibreOffice     bool
	hasCalibre         bool
	hasImagemagick     bool
}

// NewOfficeService creates a new document converter service.
func NewOfficeService() *OfficeService {
	svc := &OfficeService{}
	svc.detectLibreOffice()
	svc.detectCalibre()
	svc.detectImagemagick()
	return svc
}

func (s *OfficeService) detectLibreOffice() {
	possiblePaths := []string{
		"soffice",
		"soffice.exe",
		`C:\Program Files\LibreOffice\program\soffice.exe`,
		`C:\Program Files (x86)\LibreOffice\program\soffice.exe`,
		"/usr/bin/soffice",
		"/usr/local/bin/soffice",
		"/Applications/LibreOffice.app/Contents/MacOS/soffice",
	}
	if path, ok := findExecutable(possiblePaths); ok {
		s.sofficeExePath = path
		s.hasLibreOffice = true
	}
}

func (s *OfficeService) detectCalibre() {
	possiblePaths := []string{
		"ebook-convert",
		"ebook-convert.exe",
		`C:\Program Files\Calibre2\ebook-convert.exe`,
		`C:\Program Files (x86)\Calibre2\ebook-convert.exe`,
		"/usr/bin/ebook-convert",
		"/usr/local/bin/ebook-convert",
		"/Applications/calibre.app/Contents/MacOS/ebook-convert",
	}
	if path, ok := findExecutable(possiblePaths); ok {
		s.calibreExePath = path
		s.hasCalibre = true
	}
}

func (s *OfficeService) detectImagemagick() {
	possiblePaths := []string{"magick", "magick.exe"}
	if runtime.GOOS != "windows" {
		possiblePaths = append(possiblePaths, "convert")
	}
	possiblePaths = append(possiblePaths,
		`C:\Program Files\ImageMagick-7.1.1-Q16-HDRI\magick.exe`,
		`C:\Program Files\ImageMagick-7.1.1-Q16\magick.exe`,
		"/usr/bin/magick",
		"/usr/local/bin/magick",
	)
	if path, ok := findExecutable(possiblePaths); ok {
		s.imagemagickExePath = path
		s.hasImagemagick = true
	}
}

func findExecutable(paths []string) (string, bool) {
	for _, path := range paths {
		if _, err := exec.LookPath(path); err == nil {
			return path, true
		}
		if filepath.IsAbs(path) {
			if _, err := oslib.Stat(path); err == nil {
				return path, true
			}
		}
	}
	return "", false
}

func (s *OfficeService) IsAvailable() bool {
	return s.hasLibreOffice || s.hasCalibre || s.hasImagemagick
}

func (s *OfficeService) IsAvailableForFormat(ext string) bool {
	switch classifyDocumentFormat(ext) {
	case documentFormatOffice:
		return s.hasLibreOffice
	case documentFormatEpub:
		return s.hasCalibre
	case documentFormatDjvu:
		return s.hasImagemagick
	default:
		return false
	}
}

func (s *OfficeService) SupportedInputFormats() []string {
	return []string{
		"doc", "docx", "docm",
		"xls", "xlsx", "xlsm",
		"ppt", "pptx", "pptm",
		"odt", "ods", "odp",
		"rtf",
		"epub",
		"djvu", "djv",
	}
}

func (s *OfficeService) IsSupportedFormat(ext string) bool {
	ext = strings.ToLower(strings.TrimPrefix(ext, "."))
	for _, supported := range s.SupportedInputFormats() {
		if ext == supported {
			return true
		}
	}
	return false
}

func (s *OfficeService) AvailabilityInstructions(ext string) string {
	switch classifyDocumentFormat(ext) {
	case documentFormatOffice:
		return `LibreOffice is required for Word, Excel, PowerPoint, OpenDocument, and RTF previews.

Install LibreOffice:
• Windows: https://www.libreoffice.org/download/download/
• macOS: brew install libreoffice
• Linux: sudo apt install libreoffice`
	case documentFormatEpub:
		return `Calibre is required for EPUB previews.

Install Calibre:
• Windows: https://calibre-ebook.com/download_windows
• macOS: brew install --cask calibre
• Linux: sudo -v ; wget -nv -O- https://download.calibre-ebook.com/linux-installer.sh | sh /dev/stdin`
	case documentFormatDjvu:
		return `ImageMagick with DjVu support is required for DjVu previews.

Install ImageMagick:
• Windows: https://imagemagick.org/script/download.php#windows
• macOS: brew install imagemagick
• Linux: sudo apt install imagemagick`
	default:
		return "No converter is available for this format."
	}
}

func (s *OfficeService) ConvertToPDF(inputPath string) (string, error) {
	if _, err := oslib.Stat(inputPath); err != nil {
		return "", fmt.Errorf("input file not found: %w", err)
	}

	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(inputPath), "."))
	if !s.IsSupportedFormat(ext) {
		return "", fmt.Errorf("unsupported document format: .%s", ext)
	}

	jobDir, err := createDocumentOutputDir()
	if err != nil {
		return "", err
	}

	outputPath := filepath.Join(jobDir, outputBaseName(inputPath)+".pdf")
	switch classifyDocumentFormat(ext) {
	case documentFormatOffice:
		if !s.hasLibreOffice {
			return "", fmt.Errorf("LibreOffice not found")
		}
		return s.convertWithLibreOffice(inputPath, jobDir)
	case documentFormatEpub:
		if !s.hasCalibre {
			return "", fmt.Errorf("Calibre not found")
		}
		return s.convertWithCalibre(inputPath, outputPath)
	case documentFormatDjvu:
		if !s.hasImagemagick {
			return "", fmt.Errorf("ImageMagick not found")
		}
		return s.convertWithImagemagick(inputPath, outputPath)
	default:
		return "", fmt.Errorf("unsupported document format: .%s", ext)
	}
}

func createDocumentOutputDir() (string, error) {
	root := filepath.Join(oslib.TempDir(), "wails-document-converter")
	if err := oslib.MkdirAll(root, 0755); err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}
	jobDir := filepath.Join(root, fmt.Sprintf("job-%d", time.Now().UnixNano()))
	if err := oslib.MkdirAll(jobDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create conversion directory: %w", err)
	}
	return jobDir, nil
}

func outputBaseName(inputPath string) string {
	baseName := filepath.Base(inputPath)
	return strings.TrimSuffix(baseName, filepath.Ext(baseName))
}

func (s *OfficeService) convertWithLibreOffice(inputPath, outputDir string) (string, error) {
	outputPath := filepath.Join(outputDir, outputBaseName(inputPath)+".pdf")
	cmd := exec.Command(
		s.sofficeExePath,
		"--headless",
		"--convert-to", "pdf",
		"--outdir", outputDir,
		inputPath,
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("LibreOffice conversion failed: %v%s", err, formatCommandOutput(output))
	}
	if _, err := oslib.Stat(outputPath); err != nil {
		return "", fmt.Errorf("PDF output file not created: %w", err)
	}
	return outputPath, nil
}

func (s *OfficeService) convertWithCalibre(inputPath, outputPath string) (string, error) {
	cmd := exec.Command(s.calibreExePath, inputPath, outputPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("Calibre conversion failed: %v%s", err, formatCommandOutput(output))
	}
	if _, err := oslib.Stat(outputPath); err != nil {
		return "", fmt.Errorf("PDF output file not created: %w", err)
	}
	return outputPath, nil
}

func (s *OfficeService) convertWithImagemagick(inputPath, outputPath string) (string, error) {
	cmd := exec.Command(s.imagemagickExePath, inputPath, outputPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("ImageMagick conversion failed: %v%s", err, formatCommandOutput(output))
	}
	if _, err := oslib.Stat(outputPath); err != nil {
		return "", fmt.Errorf("PDF output file not created: %w", err)
	}
	return outputPath, nil
}

func formatCommandOutput(output []byte) string {
	text := strings.TrimSpace(string(output))
	if text == "" {
		return ""
	}
	return ": " + text
}

type documentFormatKind string

const (
	documentFormatUnknown documentFormatKind = "unknown"
	documentFormatOffice  documentFormatKind = "office"
	documentFormatEpub    documentFormatKind = "epub"
	documentFormatDjvu    documentFormatKind = "djvu"
)

func classifyDocumentFormat(ext string) documentFormatKind {
	ext = strings.ToLower(strings.TrimPrefix(ext, "."))
	switch ext {
	case "doc", "docx", "docm", "xls", "xlsx", "xlsm", "ppt", "pptx", "pptm", "odt", "ods", "odp", "rtf":
		return documentFormatOffice
	case "epub":
		return documentFormatEpub
	case "djvu", "djv":
		return documentFormatDjvu
	default:
		return documentFormatUnknown
	}
}

func (s *OfficeService) GetConversionError() string {
	return "Failed to convert document to PDF"
}

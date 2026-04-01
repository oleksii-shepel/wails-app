package converter

import (
	"fmt"
	oslib "os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// OfficeService handles conversion of supported documents to PDF.
type OfficeService struct {
	sofficeExePath     string
	calibreExePath     string
	djvuLibreExePath   string
	imagemagickExePath string
	hasLibreOffice     bool
	hasCalibre         bool
	hasDjVuLibre       bool
	hasImagemagick     bool
	hasDjvuDecode      bool
	detectOnce         sync.Once
}

// NewOfficeService creates a new document converter service.
func NewOfficeService() *OfficeService {
	return &OfficeService{}
}

func (s *OfficeService) ensureDetected() {
	s.detectOnce.Do(func() {
		s.detectLibreOffice()
		s.detectCalibre()
		s.detectDjVuLibre()
		s.detectImagemagick()
	})
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

func (s *OfficeService) detectDjVuLibre() {
	possiblePaths := []string{"ddjvu", "ddjvu.exe"}
	possiblePaths = append(possiblePaths, localDjVuLibrePaths("ddjvu.exe")...)
	if path, ok := findExecutable(possiblePaths); ok {
		s.djvuLibreExePath = path
		s.hasDjVuLibre = true
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
		s.hasDjvuDecode = s.detectImagemagickReadSupport("DJVU")
	}
}

func (s *OfficeService) detectImagemagickReadSupport(format string) bool {
	if !s.hasImagemagick || s.imagemagickExePath == "" {
		return false
	}

	cmd := exec.Command(s.imagemagickExePath, "-list", "format")
	hideWindow(cmd)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}

	return imagemagickFormatCanRead(string(output), format)
}

func imagemagickFormatCanRead(output string, format string) bool {
	target := strings.ToUpper(strings.TrimSpace(format))
	if target == "" {
		return false
	}

	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		name := strings.TrimSuffix(strings.ToUpper(fields[0]), "*")
		if name != target {
			continue
		}

		return strings.Contains(fields[2], "r")
	}

	return false
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

func localDjVuLibrePaths(executableName string) []string {
	paths := []string{}
	appendCandidate := func(base string) {
		if strings.TrimSpace(base) == "" {
			return
		}
		for _, relative := range []string{
			filepath.Join("DjVuLibre", executableName),
			filepath.Join("..", "DjVuLibre", executableName),
			filepath.Join("..", "..", "DjVuLibre", executableName),
			filepath.Join("..", "..", "..", "DjVuLibre", executableName),
		} {
			paths = append(paths, filepath.Join(base, relative))
		}
	}

	if cwd, err := oslib.Getwd(); err == nil {
		appendCandidate(cwd)
	}
	if exePath, err := oslib.Executable(); err == nil {
		appendCandidate(filepath.Dir(exePath))
	}

	return paths
}

func (s *OfficeService) IsAvailable() bool {
	s.ensureDetected()
	return s.hasLibreOffice || s.hasCalibre || s.hasDjVuLibre || s.hasImagemagick
}

func (s *OfficeService) IsAvailableForFormat(ext string) bool {
	s.ensureDetected()
	switch classifyDocumentFormat(ext) {
	case documentFormatOffice:
		return s.hasLibreOffice
	case documentFormatEpub:
		return s.hasCalibre
	case documentFormatDjvu:
		return s.hasDjVuLibre || (s.hasImagemagick && s.hasDjvuDecode)
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
		return `DjVuLibre or ImageMagick with readable DjVu support is required for DjVu previews.

Recommended local install path: C:\Users\oleks\Desktop\file-manager\DjVuLibre

Install DjVuLibre:
• Windows: https://sourceforge.net/projects/djvu/files/latest/download
• The converter looks for ddjvu.exe on PATH and in nearby DjVuLibre folders.

Alternative: install ImageMagick with DjVu support:
• Windows: https://imagemagick.org/script/download.php#windows
• macOS: brew install imagemagick
• Linux: sudo apt install imagemagick`
	default:
		return "No converter is available for this format."
	}
}

func (s *OfficeService) ConvertToPDF(inputPath string) (string, error) {
	s.ensureDetected()

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
		if s.hasDjVuLibre {
			return s.convertWithDjVuLibre(inputPath, outputPath)
		}
		if !s.hasImagemagick {
			return "", fmt.Errorf("DjVuLibre and ImageMagick are both unavailable")
		}
		if !s.hasDjvuDecode {
			return "", fmt.Errorf("DjVuLibre is unavailable and ImageMagick does not include readable DjVu support")
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
	hideWindow(cmd)
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
	hideWindow(cmd)
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("Calibre conversion failed: %v%s", err, formatCommandOutput(output))
	}
	if _, err := oslib.Stat(outputPath); err != nil {
		return "", fmt.Errorf("PDF output file not created: %w", err)
	}
	return outputPath, nil
}

func (s *OfficeService) convertWithDjVuLibre(inputPath, outputPath string) (string, error) {
	cmd := exec.Command(s.djvuLibreExePath, "-format=pdf", inputPath, outputPath)
	hideWindow(cmd)
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("DjVuLibre conversion failed: %v%s", err, formatCommandOutput(output))
	}
	if _, err := oslib.Stat(outputPath); err != nil {
		return "", fmt.Errorf("PDF output file not created: %w", err)
	}
	return outputPath, nil
}

func (s *OfficeService) convertWithImagemagick(inputPath, outputPath string) (string, error) {
	cmd := exec.Command(s.imagemagickExePath, inputPath, outputPath)
	hideWindow(cmd)
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

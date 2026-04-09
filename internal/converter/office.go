package converter

import (
	"fmt"
	oslib "os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"wails-app/internal/utility"
)

// OfficeService handles conversion of supported documents to PDF.
type OfficeService struct {
	sofficeExePath   string
	mutoolExePath    string
	djvuLibreExePath string
	hasLibreOffice   bool
	hasMuPDF         bool
	hasDjVuLibre     bool
	detectOnce       sync.Once
}

// NewOfficeService creates a new document converter service.
func NewOfficeService() *OfficeService {
	return &OfficeService{}
}

func (s *OfficeService) ensureDetected() {
	s.detectOnce.Do(func() {
		s.detectLibreOffice()
		s.detectMuPDF()
		s.detectDjVuLibre()
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

func (s *OfficeService) detectMuPDF() {
	possiblePaths := []string{
		"mutool",
		"mutool.exe",
		`C:\Program Files\MuPDF\mutool.exe`,
		`C:\Program Files (x86)\MuPDF\mutool.exe`,
		"/usr/bin/mutool",
		"/usr/local/bin/mutool",
		"/Applications/MuPDF.app/Contents/MacOS/mutool",
	}
	if path, ok := findExecutable(possiblePaths); ok {
		s.mutoolExePath = path
		s.hasMuPDF = true
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

func findExecutable(paths []string) (string, bool) {
	return utility.ResolveExecutable(paths...)
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
	return s.hasLibreOffice || s.hasMuPDF || s.hasDjVuLibre
}

func (s *OfficeService) IsAvailableForFormat(ext string) bool {
	s.ensureDetected()
	switch classifyDocumentFormat(ext) {
	case documentFormatOffice:
		return s.hasLibreOffice
	case documentFormatEpub:
		return s.hasMuPDF
	case documentFormatDjvu:
		return s.hasDjVuLibre
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
		return `MuPDF is required for EPUB previews.

Install MuPDF:
• Windows: https://mupdf.com/downloads/
• macOS: brew install mupdf
• Linux: sudo apt install mupdf-tools`
	case documentFormatDjvu:
		return `DjVuLibre is required for DjVu previews.

Recommended local install path: C:\Users\oleks\Desktop\file-manager\DjVuLibre

Install DjVuLibre:
• Windows: https://sourceforge.net/projects/djvu/files/latest/download
• The converter looks for ddjvu.exe on PATH and in nearby DjVuLibre folders.`
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
		if !s.hasMuPDF {
			return "", fmt.Errorf("MuPDF not found")
		}
		return s.convertWithMuPDF(inputPath, outputPath)
	case documentFormatDjvu:
		if !s.hasDjVuLibre {
			return "", fmt.Errorf("DjVuLibre not found")
		}
		return s.convertWithDjVuLibre(inputPath, outputPath)
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

func (s *OfficeService) convertWithMuPDF(inputPath, outputPath string) (string, error) {
	cmd := exec.Command(s.mutoolExePath, "convert", "-F", "pdf", "-o", outputPath, inputPath)
	hideWindow(cmd)
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("MuPDF conversion failed: %v%s", err, formatCommandOutput(output))
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

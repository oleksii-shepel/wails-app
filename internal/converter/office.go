package converter

import (
	"fmt"
	oslib "os"
	"os/exec"
	"path/filepath"
	"strings"
)

// OfficeService handles conversion of Office documents to PDF.
type OfficeService struct {
	sofficeExePath string
	available      bool
}

// NewOfficeService creates a new office converter service.
func NewOfficeService() *OfficeService {
	svc := &OfficeService{}
	svc.detectLibreOffice()
	return svc
}

// detectLibreOffice tries to find LibreOffice soffice executable.
func (s *OfficeService) detectLibreOffice() {
	// Try standard locations
	possiblePaths := []string{
		"soffice",     // On PATH
		"soffice.exe", // Windows
		"C:\\Program Files\\LibreOffice\\program\\soffice.exe",       // Windows default
		"C:\\Program Files (x86)\\LibreOffice\\program\\soffice.exe", // Windows alternative
		"/usr/bin/soffice",       // Linux
		"/usr/local/bin/soffice", // macOS/Linux
		"/Applications/LibreOffice.app/Contents/MacOS/soffice", // macOS
	}

	for _, path := range possiblePaths {
		if _, err := exec.LookPath(path); err == nil {
			s.sofficeExePath = path
			s.available = true
			return
		}

		// If absolute path, just check if it exists
		if filepath.IsAbs(path) {
			if _, err := oslib.Stat(path); err == nil {
				s.sofficeExePath = path
				s.available = true
				return
			}
		}
	}
}

// IsAvailable returns whether LibreOffice is available on the system.
func (s *OfficeService) IsAvailable() bool {
	return s.available
}

// SupportedInputFormats returns the list of supported office document formats.
func (s *OfficeService) SupportedInputFormats() []string {
	return []string{
		"doc", "docx", "docm", // Word
		"xls", "xlsx", "xlsm", // Excel
		"ppt", "pptx", "pptm", // PowerPoint
		"odt", "ods", "odp", // OpenDocument
		"rtf", // Rich Text Format
	}
}

// IsSupportedFormat checks if a file extension is supported for conversion.
func (s *OfficeService) IsSupportedFormat(ext string) bool {
	ext = strings.ToLower(strings.TrimPrefix(ext, "."))
	for _, supported := range s.SupportedInputFormats() {
		if ext == supported {
			return true
		}
	}
	return false
}

// ConvertToPDF converts an office document to PDF and returns the PDF file path.
// If LibreOffice is not available, returns an error.
// The returned PDF path is in the system temp directory and should be cleaned up by the caller.
func (s *OfficeService) ConvertToPDF(inputPath string) (string, error) {
	if !s.available {
		return "", fmt.Errorf("LibreOffice not found: please install LibreOffice to view office documents")
	}

	// Verify the input file exists
	if _, err := oslib.Stat(inputPath); err != nil {
		return "", fmt.Errorf("input file not found: %w", err)
	}

	// Create temp directory for output
	tempDir := oslib.TempDir()
	outputDir := filepath.Join(tempDir, "wails-office-converter")
	if err := oslib.MkdirAll(outputDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Get the base filename without extension for the output PDF
	baseName := filepath.Base(inputPath)
	baseName = strings.TrimSuffix(baseName, filepath.Ext(baseName))
	outputPath := filepath.Join(outputDir, baseName+".pdf")

	// Run LibreOffice in headless mode to convert to PDF
	// soffice --headless --convert-to pdf --outdir <output> <input>
	cmd := exec.Command(
		s.sofficeExePath,
		"--headless",
		"--convert-to", "pdf",
		"--outdir", outputDir,
		inputPath,
	)

	// Suppress output
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("LibreOffice conversion failed: %w", err)
	}

	// Verify the output file was created
	if _, err := oslib.Stat(outputPath); err != nil {
		return "", fmt.Errorf("PDF output file not created: %w", err)
	}

	return outputPath, nil
}

// GetConversionError returns a user-friendly error message if conversion failed.
func (s *OfficeService) GetConversionError() string {
	if !s.available {
		return `LibreOffice not installed.

To view Office documents (Word, Excel, PowerPoint), please install LibreOffice:
• Windows: https://www.libreoffice.org/download/download/
• macOS: brew install libreoffice
• Linux: sudo apt install libreoffice (Ubuntu/Debian) or equivalent

Or try online office viewers for better compatibility.`
	}
	return "Failed to convert document to PDF"
}

package app

import (
	"fmt"
	"path/filepath"
	"strings"

	"wails-app/internal/contracts"
)

// GetConvertedImageURL returns a stream URL that serves a converted preview image.
func (a *App) GetConvertedImageURL(path string, targetFormat string) (string, error) {
	info, err := a.State.Filesystem.GetFileInfo(path)
	if err != nil {
		return "", fmt.Errorf("cannot access file: %w", err)
	}

	if info.Type == contracts.FileTypeDirectory {
		return "", fmt.Errorf("cannot convert directory")
	}

	ext := strings.TrimPrefix(filepath.Ext(path), ".")
	ext = strings.ToLower(ext)
	if !a.State.Converter.IsSupportedSourceFormat(ext) {
		return "", fmt.Errorf("unsupported source format: .%s", ext)
	}

	targetFormat = strings.ToLower(strings.TrimPrefix(targetFormat, "."))
	return ConvertedStreamURLForFile(a.StreamBaseURL(), path, targetFormat), nil
}

// GetConversionCapabilities returns what formats are supported for a given source format.
func (a *App) GetConversionCapabilities(sourceFileExt string) map[string]interface{} {
	ext := strings.ToLower(sourceFileExt)
	ext = strings.TrimPrefix(ext, ".")

	available := a.State.Converter.GetAvailableConversionFormats(ext)

	return map[string]interface{}{
		"sourceFormat":      ext,
		"supported":         a.State.Converter.IsSupportedSourceFormat(ext),
		"availableFormats":  available,
		"recommendedFormat": "png", // PNG is lossless and widely compatible
	}
}

// CanConvertImage checks if an image format can be converted for preview.
func (a *App) CanConvertImage(fileExt string) bool {
	ext := strings.ToLower(fileExt)
	ext = strings.TrimPrefix(ext, ".")
	return a.State.Converter.IsSupportedSourceFormat(ext)
}

// ConvertOfficeDocument converts an office document to PDF and returns the PDF file path.
// This is used by the office viewer component to convert documents before display.
func (a *App) ConvertOfficeDocument(path string) (map[string]interface{}, error) {
	if a.State.OfficeConverter == nil {
		return nil, fmt.Errorf("office converter not available")
	}

	info, err := a.State.Filesystem.GetFileInfo(path)
	if err != nil {
		return nil, fmt.Errorf("cannot access file: %w", err)
	}

	if info.Type == contracts.FileTypeDirectory {
		return nil, fmt.Errorf("cannot convert directory")
	}

	ext := strings.TrimPrefix(filepath.Ext(path), ".")
	ext = strings.ToLower(ext)

	if !a.State.OfficeConverter.IsSupportedFormat(ext) {
		return nil, fmt.Errorf("unsupported office document format: .%s", ext)
	}

	if !a.State.OfficeConverter.IsAvailable() {
		return map[string]interface{}{
			"success":      false,
			"error":        "LibreOffice not available",
			"availability": false,
			"instructions": "Please install LibreOffice to view Office documents",
		}, nil
	}

	// Resolve VFS path to real filesystem path for LibreOffice
	realPath, err := a.State.Filesystem.RealPath(path)
	if err != nil {
		return map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("cannot resolve file path: %v", err),
		}, nil
	}

	pdfPath, err := a.State.OfficeConverter.ConvertToPDF(realPath)
	if err != nil {
		return map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		}, nil
	}

	return map[string]interface{}{
		"success": true,
		"pdfPath": pdfPath,
	}, nil
}

// IsOfficeDocumentAvailable checks if office documents can be converted
func (a *App) IsOfficeDocumentAvailable() bool {
	if a.State.OfficeConverter == nil {
		return false
	}
	return a.State.OfficeConverter.IsAvailable()
}

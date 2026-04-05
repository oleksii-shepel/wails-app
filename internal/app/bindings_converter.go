package app

import (
	"fmt"
	"path/filepath"
	"strings"

	"wails-app/internal/contracts"
)

// GetConvertedImageURL returns a stream URL that serves a converted preview image.
func (a *App) GetConvertedImageURL(path string, targetFormat string) (string, error) {
	info, err := a.State.GetFileInfo(path)
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

// GetImageThumbnailURL returns a stream URL that serves a bounded thumbnail image.
func (a *App) GetImageThumbnailURL(path string, maxWidth int, maxHeight int) (string, error) {
	info, err := a.State.GetFileInfo(path)
	if err != nil {
		return "", fmt.Errorf("cannot access file: %w", err)
	}

	if info.Type == contracts.FileTypeDirectory {
		return "", fmt.Errorf("cannot thumbnail directory")
	}

	ext := strings.TrimPrefix(filepath.Ext(path), ".")
	ext = strings.ToLower(ext)
	if !a.State.Converter.IsSupportedSourceFormat(ext) {
		return "", fmt.Errorf("unsupported source format: .%s", ext)
	}

	return ThumbnailStreamURLForFile(a.StreamBaseURL(), path, "png", maxWidth, maxHeight), nil
}

// GetArchiveImageThumbnailURL returns a thumbnail stream URL for an archive entry.
func (a *App) GetArchiveImageThumbnailURL(fsPath string, nestedChain []string, entryPath string, maxWidth int, maxHeight int) (string, error) {
	ext := strings.TrimPrefix(filepath.Ext(entryPath), ".")
	ext = strings.ToLower(ext)
	if !a.State.Converter.IsSupportedSourceFormat(ext) {
		return "", fmt.Errorf("unsupported source format: .%s", ext)
	}

	return ThumbnailStreamURLForEntry(a.StreamBaseURL(), fsPath, nestedChain, entryPath, "png", maxWidth, maxHeight), nil
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

	info, err := a.State.GetFileInfo(path)
	if err != nil {
		return nil, fmt.Errorf("cannot access file: %w", err)
	}

	if info.Type == contracts.FileTypeDirectory {
		return nil, fmt.Errorf("cannot convert directory")
	}

	ext := strings.TrimPrefix(filepath.Ext(path), ".")
	ext = strings.ToLower(ext)

	if !a.State.OfficeConverter.IsSupportedFormat(ext) {
		return nil, fmt.Errorf("unsupported document format: .%s", ext)
	}

	if !a.State.OfficeConverter.IsAvailableForFormat(ext) {
		return map[string]interface{}{
			"success":      false,
			"error":        "Document converter not available",
			"availability": false,
			"instructions": a.State.OfficeConverter.AvailabilityInstructions(ext),
		}, nil
	}

	// Resolve VFS path to a real filesystem path for the selected converter.
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

// IsOfficeDocumentAvailable checks if any document conversion backend is available.
func (a *App) IsOfficeDocumentAvailable() bool {
	if a.State.OfficeConverter == nil {
		return false
	}
	return a.State.OfficeConverter.IsAvailable()
}

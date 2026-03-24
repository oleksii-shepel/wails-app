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

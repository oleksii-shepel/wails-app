package plugin

import (
	"context"
)

// PreviewPlugin allows plugins to provide custom file preview and thumbnail
// generation for file types not handled by the built-in converter.
type PreviewPlugin interface {
	Plugin
	// CanPreview returns true if this plugin can generate a preview for the
	// given MIME type and file extension.
	CanPreview(mimeType, ext string) bool
	// Priority returns the plugin priority. Higher values win over lower ones.
	Priority() int
	// GenerateThumbnail creates a thumbnail image for the given file path.
	// The returned bytes should be a valid image (PNG, WebP, JPEG).
	GenerateThumbnail(ctx context.Context, path string) ([]byte, error)
	// Convert converts the file at path to the target format.
	// Supported targets depend on the plugin (e.g., "pdf", "html", "txt").
	Convert(ctx context.Context, path string, targetFormat string) ([]byte, error)
}

// PreviewPicker selects the best preview plugin for a given file.
type PreviewPicker struct {
	plugins []PreviewPlugin
}

// NewPreviewPicker creates a picker from the registered plugins.
func NewPreviewPicker(plugins []PreviewPlugin) *PreviewPicker {
	return &PreviewPicker{plugins: plugins}
}

// Pick returns the highest-priority plugin that can handle the file, or nil.
func (p *PreviewPicker) Pick(mimeType, ext string) PreviewPlugin {
	var best PreviewPlugin
	var bestPriority int
	for _, plugin := range p.plugins {
		if !plugin.CanPreview(mimeType, ext) {
			continue
		}
		if prio := plugin.Priority(); prio > bestPriority {
			bestPriority = prio
			best = plugin
		}
	}
	return best
}

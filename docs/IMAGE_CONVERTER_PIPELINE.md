# Image Converter Backend Pipeline

## Overview

A backend image conversion service has been implemented to support preview of extended image formats beyond what browsers natively support. The system uses **libvips** command-line tools (`vips` and `vipsthumbnail`) as the primary converter when available and falls back to Go's native image decoders for basic formats.

## Current State

The current implementation is intentionally split into two paths:

1. **Native browser preview** for formats that can already be rendered by the browser.
2. **Backend conversion preview** for formats that need server-side decoding before display.

The converted preview path now uses the existing local stream server. Earlier iterations used base64 transport over the Wails bridge, but that was replaced because it inflated payload size and duplicated memory usage in both Go and the browser.

## Architecture

### Backend Components

#### 1. Converter Service (`internal/converter/service.go`)
- **ConvertFormat()** - Converts image from source format to target format (PNG, JPEG, WebP, etc.)
- **IsSupportedSourceFormat()** - Checks if a source format can be converted
- **GetAvailableConversionFormats()** - Returns list of supported output formats for a source

**Conversion Strategy:**
- Primary: Uses libvips CLI tools (`vips` for conversions, `vipsthumbnail` for resized previews)
- Fallback: Go's native image decoders for PNG, JPEG, GIF, BMP

**Supported Source Formats:**
- With libvips: TIFF, WebP, AVIF, HEIF/HEIC, JPEG 2000, OpenEXR, JPEG XL, FITS, HDR, PFM, and other formats supported by the bundled libvips build
- Without libvips (Go fallback): PNG, JPEG, GIF, BMP

**Output Formats:**
- PNG (recommended - lossless)
- JPEG
- WebP
- GIF
- BMP
- AVif
- TIFF

#### 2. App Bindings (`internal/app/bindings_converter.go`)

Three public Wails methods expose the converter to the frontend:

```go
GetConvertedImageURL(path string, targetFormat string) (string, error)
// Returns a /wails-stream URL that converts on demand
// Usage: window.go.app.App.GetConvertedImageURL("/path/to/image.psd", "png")

GetConversionCapabilities(sourceFileExt string) map[string]interface{}
// Returns conversion capabilities for a file extension
// Returns: { sourceFormat, supported, availableFormats, recommendedFormat }

CanConvertImage(fileExt string) bool
// Quick check if an extension can be converted
// Usage: window.go.app.App.CanConvertImage("psd")
```

#### 3. Service Integration
- Added `Converter` field to `Core`, `Services`, and `State` structs
- Initialized in `NewCore()` → `NewServices()` → `BuildState()`
- Follows existing service pattern with dependency injection

#### 4. Unified Stream Handler Integration (`internal/app/stream_handler.go`)
- Added on-demand converted preview support through `/wails-stream`
- New request shape: `/wails-stream?path=<path>&convert=png`
- Converted images are served with normal HTTP semantics instead of being marshalled through Wails IPC

### Frontend Updates

#### 1. Image Viewer Component (`image-viewer.component.ts`)
- Enhanced `loadFromDisk()` to attempt backend conversion for unsupported formats
- New method `convertImageAndDisplay()` requests a converted preview URL from the backend stream server
- Archives: Conversion not yet supported (would need temp extraction)

#### 2. Viewer Registry (`viewer-registry.service.ts`)
- Extended image file scoring to include convertible formats
- Score: 50 for native formats, 40 for convertible formats
- Routes convertible raster formats to the image viewer

#### 3. API Service (`api.service.ts`)
- Added `getConvertedImageURL()` helper
- Keeps converted preview loading consistent with other streamed preview URLs

## How It Works

### User Flow

1. User opens a file preview for an image in unsupported format (e.g., `.jp2`)
2. Viewer registry recognizes it as convertible → routes to Image Viewer
3. Image Viewer attempts to load → no direct MIME type available
4. Component calls `window.go.app.App.CanConvertImage("psd")`
5. If supported, calls `window.go.app.App.GetConvertedImageURL(path, "png")`
6. Backend either:
   - Uses libvips to convert the image
   - Falls back to Go decoder if libvips is unavailable and the source is natively decodable
7. Streams the converted PNG bytes from `/wails-stream`
8. Component displays the returned preview URL in `<img src="...">`

### Why The Base64 Path Was Removed

The original conversion transport returned a base64-encoded image string to the frontend. That approach was removed because it had several downsides:

- Base64 adds measurable payload overhead.
- The full image had to cross the Wails bridge as one large string.
- The browser then had to decode the same payload again into a data URL.
- Large images caused unnecessary memory duplication on both sides.

The current streamed-preview approach avoids those issues while preserving the existing viewer model.

### Data Flow

```
Frontend                Backend
┌──────────────────┐   ┌──────────────────────┐
│ Image Viewer     │   │ App Bindings         │
│ Component        │───→ GetConvertedImageURL() │
└──────────────────┘   └──────┬───────────────┘
                               │
             returns preview URL
                │
Frontend requests ◄────────────┘
GET /wails-stream?path=...&convert=png
                │
                         ┌─────▼──────────────┐
                         │ Converter Service  │
                         ├────────────────────┤
                         │ libvips CLI        │
                         │ (or Go decoder)    │
                         └──────┬─────────────┘
                                │
                         ┌──────▼──────────────┐
                         │ [Image bytes]      │
                         └─────────────────────┘
                               │
               Browser receives streamed preview
```

## Prerequisites

### For Full Format Support
Install **libvips**:

**Windows (Chocolatey):**
```powershell
choco install vips
```

**Windows (Direct):**
Download from https://www.libvips.org/

**macOS (Homebrew):**
```bash
brew install vips
```

**Linux (Ubuntu/Debian):**
```bash
sudo apt-get install libvips-tools
```

**Note:** Without libvips, only PNG, JPEG, GIF, and BMP conversions work (via Go).

## Testing

### Backend

```bash
# Test Go compilation
go build ./...

# Test conversion capability
go test ./internal/converter
```

### Frontend

```bash
# Build Angular
cd frontend
npm run build

# Run full dev environment
wails dev
```

### Manual Testing

1. Place test images in different formats:
   - `.jp2` (JPEG 2000)
   - `.heic`
   - `.avif`
   - `.exr`

2. Open image in file manager preview tab

3. Observe:
   - Loading animation should appear
   - Image should convert and display
   - Or error message if libvips not installed

### Browser DevTools

Monitor in Console for conversion calls:
```javascript
// Check if conversion is available
await window.go.app.App.CanConvertImage("jp2")
// Returns: true or false

// Get converted preview URL
await window.go.app.App.GetConvertedImageURL("C:/images/test.jp2", "png")

// Get capabilities
await window.go.app.App.GetConversionCapabilities("jp2")
// Returns: {sourceFormat: "jp2", supported: true, availableFormats: [...], ...}
```

## Performance Notes

- **First conversion:** May take 1-2 seconds (libvips process startup)
- **Subsequent conversions:** Faster as process is reused
- **Transport:** Converted previews are streamed over HTTP instead of sent through the Wails bridge
- **Memory:** This avoids duplicating image payloads as base64 strings in JS memory
- **Still in-memory per request:** The converter currently produces the final preview bytes in memory before the stream handler serves them

## Limitations

### Archive Support
- Conversions only work for files on disk
- Archive entries cannot be converted (would need temp extraction)
- Future: Could add temp extraction support

### Size Limits
- No file size limit enforced currently
- Large files (>500MB) may cause memory issues
- Recommendation: Add size check before conversion (see TODO below)

### Output Quality
- PNG: Lossless, preserves quality
- JPEG: Lossy
- libvips settings are not currently configurable from the UI

## Configuration Options (Future)

```go
// Could add to Converter service:
type ConversionOptions struct {
    OutputFormat string
    MaxWidth     int
    MaxHeight    int
    Quality      int // 1-100 for JPEG
    Timeout      time.Duration
}

ConvertFormatWithOptions(path string, opts ConversionOptions) ([]byte, error)
```

## Error Handling

Common errors and resolutions:

| Error | Cause | Solution |
|-------|-------|----------|
| `unsupported source format: .xyz` | Extension not recognized | Check if extension is in supported list |
| `libvips conversion failed` | libvips not installed or file corrupted | Install libvips or try another build |
| `failed to decode image` | Go decoder couldn't read format | Requires libvips |
| `conversion timeout` | Large file or slow system | Could implement timeout handling |

## Future Enhancements

1. **Thumbnail Generation:** Create small previews for thumbnails
2. **Caching:** Cache converted images to avoid re-converting
3. **True conversion streaming:** Stream conversion output directly instead of buffering the full converted file first
4. **EXIF Extraction:** Extract metadata from image headers
5. **Archive Support:** Temp-extract and convert image entries
6. **Web Format Optimization:** Auto-convert to WebP for faster loading
7. **Batch Conversion:** API for converting multiple files

## Implementation Files

Backend:
- `internal/converter/service.go` - Core conversion logic
- `internal/app/core.go` - Core initialization
- `internal/app/services.go` - Service registration
- `internal/app/bindings_converter.go` - Frontend bindings
- `internal/app/state.go` - State struct and interfaces

Frontend:
- `frontend/src/app/viewers/builtins/image-viewer.component.ts` - Conversion integration
- `frontend/src/app/viewers/viewer-registry.service.ts` - Format routing
- `frontend/src/app/services/api.service.ts` - Converted preview URL helper

## References

- **libvips Documentation:** https://www.libvips.org/
- **Go Image Package:** https://golang.org/pkg/image/
- **Wails Go ↔ Frontend Binding:** https://wails.io/docs/guides/calling-go-from-frontend

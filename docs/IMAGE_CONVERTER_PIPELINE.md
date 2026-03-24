# Image Converter Backend Pipeline

## Overview

A backend image conversion service has been implemented to support preview of extended image formats beyond what browsers natively support. The system uses **ImageMagick** as the primary converter (when available) and falls back to Go's native image decoders for basic formats.

## Architecture

### Backend Components

#### 1. Converter Service (`internal/converter/service.go`)
- **ConvertFormat()** - Converts image from source format to target format (PNG, JPEG, WebP, etc.)
- **IsSupportedSourceFormat()** - Checks if a source format can be converted
- **GetAvailableConversionFormats()** - Returns list of supported output formats for a source

**Conversion Strategy:**
- Primary: Uses ImageMagick's `convert` or `magick` command (detected via PATH)
- Fallback: Go's native image decoders for PNG, JPEG, GIF, BMP

**Supported Source Formats:**
- With ImageMagick: PSD/PSB, AI (Illustrator), EPS, XCF (GIMP), TIFF, DNG, TARGA, ICNS, PNM, DDS, XWD, SCR, XPM, OpenEXR, HEIF/HEIC, JPEG 2000, AGE, and more
- Without ImageMagick (Go fallback): PNG, JPEG, GIF, BMP

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

### Frontend Updates

#### 1. Image Viewer Component (`image-viewer.component.ts`)
- Enhanced `loadFromDisk()` to attempt backend conversion for unsupported formats
- New method `convertImageAndDisplay()` requests a converted preview URL from the backend stream server
- Archives: Conversion not yet supported (would need temp extraction)

#### 2. Viewer Registry (`viewer-registry.service.ts`)
- Extended image file scoring to include convertible formats
- Score: 50 for native formats, 40 for convertible formats
- Routes files like `.psd`, `.ai`, `.eps`, `.xcf` to image viewer

## How It Works

### User Flow

1. User opens a file preview for an image in unsupported format (e.g., `.psd`)
2. Viewer registry recognizes it as convertible → routes to Image Viewer
3. Image Viewer attempts to load → no direct MIME type available
4. Component calls `window.go.app.App.CanConvertImage("psd")`
5. If supported, calls `window.go.app.App.GetConvertedImageURL(path, "png")`
6. Backend either:
   - Uses ImageMagick to convert the image
   - Falls back to Go decoder if ImageMagick unavailable
7. Streams the converted PNG bytes from `/wails-stream`
8. Component displays the returned preview URL in `<img src="...">`

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
                         │ ImageMagick        │
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
Install **ImageMagick**:

**Windows (Chocolatey):**
```powershell
choco install imagemagick
```

**Windows (Direct):**
Download from https://imagemagick.org/script/download.php#windows

**macOS (Homebrew):**
```bash
brew install imagemagick
```

**Linux (Ubuntu/Debian):**
```bash
sudo apt-get install imagemagick
```

**Note:** Without ImageMagick, only PNG, JPEG, GIF, and BMP conversions work (via Go).

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
   - `.psd` (Photoshop)
   - `.ai` (Illustrator)
   - `.eps` (PostScript)
   - `.xcf` (GIMP)
   - `.jp2` (JPEG 2000)

2. Open image in file manager preview tab

3. Observe:
   - Loading animation should appear
   - Image should convert and display
   - Or error message if ImageMagick not installed

### Browser DevTools

Monitor in Console for conversion calls:
```javascript
// Check if conversion is available
await window.go.app.App.CanConvertImage("psd")
// Returns: true or false

// Get converted preview URL
await window.go.app.App.GetConvertedImageURL("C:/images/test.psd", "png")

// Get capabilities
await window.go.app.App.GetConversionCapabilities("psd")
// Returns: {sourceFormat: "psd", supported: true, availableFormats: [...], ...}
```

## Performance Notes

- **First conversion:** May take 1-2 seconds (ImageMagick startup)
- **Subsequent conversions:** Faster as process is reused
- **Transport:** Converted previews are streamed over HTTP instead of sent through the Wails bridge
- **Memory:** This avoids duplicating image payloads as base64 strings in JS memory

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
- JPEG: Lossy, configurable quality
- ImageMagick settings not currently configurable

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
| `imagemagick conversion failed` | ImageMagick not installed or file corrupted | Install ImageMagick or try repair tool |
| `failed to decode image` | Go decoder couldn't read format | Requires ImageMagick |
| `conversion timeout` | Large file or slow system | Could implement timeout handling |

## Future Enhancements

1. **Thumbnail Generation:** Create small previews for thumbnails
2. **Caching:** Cache converted images to avoid re-converting
3. **Streaming:** Support streaming for very large images
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

## References

- **ImageMagick Documentation:** https://imagemagick.org/
- **Go Image Package:** https://golang.org/pkg/image/
- **Wails Go ↔ Frontend Binding:** https://wails.io/docs/guides/calling-go-from-frontend

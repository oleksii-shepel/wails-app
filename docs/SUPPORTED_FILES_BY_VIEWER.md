# Viewer Support - Selected Formats

## Font Viewer

Font files are previewed directly in-app with a dedicated font surface. The viewer renders sample text, multilingual glyph rows, and basic file metadata.

Supported detection:
- By extension: TTF, TTC, OTF, OTC, WOFF, WOFF2, EOT
- By MIME: font/* plus common legacy font MIME types

This also covers extensionless web-font cache files when the backend probe resolves them as font MIME types.

## Image Viewer

## Browser-Native Formats
These formats are rendered directly by the browser without conversion:
- PNG (including Apple CgBI)
- JPEG
- GIF
- SVG
- WebP
- BMP
- ICO
- AVIF
- TIFF

## Convertible Formats (with Backend Processing)
The following formats are converted to PNG for preview via libvips. **libvips must be installed** for extended format support; without it, only basic formats can be used.

### With libvips Installed:
- JPEG 2000 (JP2, J2K)
- OpenEXR (EXR)
- HEIF/HEIC (Apple High Efficiency)
- AVIF
- TIFF
- WebP
- JPEG XL (JXL)
- FITS / HDR / PFM
- Exact coverage still depends on how the bundled libvips build was compiled

### Without libvips (Go Fallback):
- PNG
- JPEG
- GIF
- BMP

## Architecture

The image viewer uses a two-tier approach:

1. **Tier 1 (Native):** Browser directly renders supported formats
2. **Tier 2 (Conversion):** For unsupported formats, backend converts to PNG via libvips

Converted previews are not sent through the Wails bridge as base64 payloads. They are served from the local preview stream as regular image URLs, which keeps transport overhead lower and lets the browser load them normally.

See [IMAGE_CONVERTER_PIPELINE.md](IMAGE_CONVERTER_PIPELINE.md) for detailed architecture and setup instructions.

## Installation

To enable full format support, install libvips:

**Windows:**
```powershell
choco install vips
# or download a Windows build from https://www.libvips.org/
```

**macOS:**
```bash
brew install vips
```

**Linux:**
```bash
sudo apt-get install libvips-tools
```

## Format Categories

### Original Supported
PNG (including Apple CgBI), JPEG, GIF, SVG, TIFF, ICO, BMP, WebP, AVIF, HEIF/HEIC, JPEG 2000, OpenEXR, JPEG XL, FITS, HDR, PFM, and browser-native formats.

### Notes
- Archive images (inside ZIP, RAR, etc.) cannot be converted yet
- Very large images may take time to convert on first load
- Converted images are served from the local preview stream as regular image URLs

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
The following formats are converted to PNG for preview via ImageMagick. **ImageMagick must be installed** for full support; without it, only basic formats can be used.

### With ImageMagick Installed:
- JPEG 2000 (JP2, J2K)
- PSD/PSB (Adobe Photoshop)
- AI (Adobe Illustrator)
- EPS (PostScript)
- XCF (GIMP)
- DNG (Digital Negative - RAW)
- TGA/TARGA (Targa Image)
- ICNS (Apple Icon Image)
- PNM (Portable Bitmap)
- DDS (DirectDraw Surface)
- XWD (X Window Dump)
- SCR (ZX-Spectrum Screen)
- XPM (X Pixmap)
- OpenEXR (EXR)
- HEIF/HEIC (Apple High Efficiency)
- AGE (various)
- And many more format supported by ImageMagick

### Without ImageMagick (Go Fallback):
- PNG
- JPEG
- GIF
- BMP

## Architecture

The image viewer uses a two-tier approach:

1. **Tier 1 (Native):** Browser directly renders supported formats
2. **Tier 2 (Conversion):** For unsupported formats, backend converts to PNG via ImageMagick

Converted previews are not sent through the Wails bridge as base64 payloads. They are served from the local preview stream as regular image URLs, which keeps transport overhead lower and lets the browser load them normally.

See [IMAGE_CONVERTER_PIPELINE.md](IMAGE_CONVERTER_PIPELINE.md) for detailed architecture and setup instructions.

## Installation

To enable full format support, install ImageMagick:

**Windows:**
```powershell
choco install imagemagick
# or download from https://imagemagick.org/script/download.php#windows
```

**macOS:**
```bash
brew install imagemagick
```

**Linux:**
```bash
sudo apt-get install imagemagick
```

## Format Categories

### Original Supported
PNG (including Apple CgBI), JPEG, JPEG 2000, PSD/PSB (Adobe Photoshop), AI (Adobe Illustrator), EPS, XCF (GIMP), GIF, SVG, TIFF, DNG, TARGA, ICO, ICNS (Apple Icon Image), BMP, PNM, DDS, XWD, SCR (ZX-Spectrum screen), XPM, WebP, OpenEXR, HEIF/HEIC, AVIF, AGE, and many more.

### Notes
- Archive images (inside ZIP, RAR, etc.) cannot be converted yet
- Very large images may take time to convert on first load
- Converted images are served from the local preview stream as regular image URLs

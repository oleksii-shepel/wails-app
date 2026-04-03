$ErrorActionPreference = "Stop"

$workspaceRoot = $PSScriptRoot | Split-Path -Parent
$ffmpegToolDir = "$workspaceRoot\tools\ffmpeg-8.1-full_build-shared"
$pkgConfigPath = "$ffmpegToolDir\pkgconfig"
$binPath = "$ffmpegToolDir\bin"

# Export pkg-config path for CGO
$env:PKG_CONFIG_PATH = $pkgConfigPath

# Update PATH so the compiler and linker find dynamic libraries during build if needed
$env:PATH = "$binPath;" + $env:PATH

Write-Host "Building Wails App with FFmpeg (go-astiav) bindings enabled..." -ForegroundColor Green
wails build

$ErrorActionPreference = "Stop"

$workspaceRoot = $PSScriptRoot | Split-Path -Parent
$ffmpegToolDir = "$workspaceRoot\tools\ffmpeg-8.1-full_build-shared"
$pkgConfigPath = "$ffmpegToolDir\pkgconfig"
$binPath = "$ffmpegToolDir\bin"

# Export pkg-config path for CGO
$env:PKG_CONFIG_PATH = $pkgConfigPath

# Ensure ffmpeg shared libraries (DLLs) are in the path so the built exe can find them at runtime 
$env:PATH = "$binPath;" + $env:PATH

Write-Host "Running Wails Dev with FFmpeg (go-astiav) bindings enabled..." -ForegroundColor Green
wails dev

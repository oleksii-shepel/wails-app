param(
  [string]$GStreamerSdkRoot = 'C:\Users\oleks\AppData\Local\Programs\gstreamer\1.0\msvc_x86_64',
  [string[]]$GoArgs = @('./...')
)

$ErrorActionPreference = 'Stop'

$projectRoot = Split-Path -Parent $PSScriptRoot
$gstreamerBin = Join-Path $GStreamerSdkRoot 'bin'
$pkgConfigPath = Join-Path $GStreamerSdkRoot 'lib\pkgconfig'

if (-not (Test-Path $gstreamerBin)) {
  throw "GStreamer SDK bin directory not found: $gstreamerBin"
}

if (-not (Test-Path (Join-Path $gstreamerBin 'pkg-config.exe'))) {
  throw "pkg-config.exe not found under GStreamer SDK bin: $gstreamerBin"
}

if (-not (Test-Path $pkgConfigPath)) {
  throw "GStreamer SDK pkg-config directory not found: $pkgConfigPath"
}

$env:PATH = "$gstreamerBin;$env:PATH"
$env:PKG_CONFIG_PATH = $pkgConfigPath

if (-not $env:GOCACHE) {
  $env:GOCACHE = Join-Path $projectRoot '.gocache'
}

Push-Location $projectRoot
try {
  Write-Host "Using GStreamer SDK root: $GStreamerSdkRoot"
  Write-Host "Running: go build $($GoArgs -join ' ')"
  & go build @GoArgs
} finally {
  Pop-Location
}

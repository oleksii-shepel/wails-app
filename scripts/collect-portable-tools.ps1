$ErrorActionPreference = 'Stop'

$projectRoot = Split-Path -Parent $PSScriptRoot
$toolsRoot = Join-Path $projectRoot 'tools'

$toolSpecs = @(
  @{
    Name = 'ffmpeg'
    Sources = @(
      (Join-Path $HOME 'scoop\apps\ffmpeg\current\bin'),
      'C:\ffmpeg\bin'
    )
  },
  @{
    Name = 'imagemagick'
    Sources = @(
      'C:\Program Files\ImageMagick-7.1.2-Q16-HDRI',
      'C:\Program Files\ImageMagick-7.1.1-Q16-HDRI',
      'C:\Program Files\ImageMagick-7.1.1-Q16'
    )
  },
  @{
    Name = 'calibre'
    Sources = @(
      'C:\Program Files\Calibre2'
    )
  },
  @{
    Name = 'libreoffice'
    Sources = @(
      'C:\Program Files\LibreOffice',
      'C:\Program Files (x86)\LibreOffice'
    )
  },
  @{
    Name = 'djvulibre'
    Sources = @(
      (Join-Path $projectRoot 'DjVuLibre'),
      (Join-Path (Split-Path -Parent $projectRoot) 'DjVuLibre')
    )
  }
)

New-Item -ItemType Directory -Force -Path $toolsRoot | Out-Null

foreach ($tool in $toolSpecs) {
  $destination = Join-Path $toolsRoot $tool.Name
  New-Item -ItemType Directory -Force -Path $destination | Out-Null

  $source = $tool.Sources | Where-Object { Test-Path $_ } | Select-Object -First 1
  if (-not $source) {
    Write-Warning "$($tool.Name): source not found"
    continue
  }

  Write-Host "Copying $($tool.Name) from $source"
  Copy-Item (Join-Path $source '*') $destination -Recurse -Force
}

Write-Host "Portable tool collection complete: $toolsRoot"
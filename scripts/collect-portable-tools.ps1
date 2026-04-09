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
    Name = 'libvips'
    Sources = @(
      'C:\Program Files\libvips\bin',
      'C:\Program Files\vips-dev-8.17\bin',
      'C:\vips-dev-8.17\bin'
    )
  },
  @{
    Name = 'mupdf'
    Sources = @(
      'C:\Program Files\MuPDF'
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

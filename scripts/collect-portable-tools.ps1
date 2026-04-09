$ErrorActionPreference = 'Stop'

$projectRoot = Split-Path -Parent $PSScriptRoot
$toolsRoot = Join-Path $projectRoot 'tools'

function Resolve-FirstSourcePath {
  param(
    [string[]]$Candidates
  )

  foreach ($candidate in $Candidates) {
    if ([string]::IsNullOrWhiteSpace($candidate)) {
      continue
    }

    $matches = @(Get-Item -Path $candidate -ErrorAction SilentlyContinue)
    $match = $matches | Where-Object { $_.PSIsContainer } | Select-Object -First 1
    if ($match) {
      return $match.FullName
    }
  }

  return $null
}

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
      'C:\vips-dev-8.17\bin',
      (Join-Path $env:LOCALAPPDATA 'Microsoft\WinGet\Packages\libvips.libvips_Microsoft.Winget.Source_8wekyb3d8bbwe\vips-dev-*\bin')
    )
  },
  @{
    Name = 'mupdf'
    Sources = @(
      'C:\Program Files\MuPDF',
      (Join-Path $env:LOCALAPPDATA 'Microsoft\WinGet\Packages\ArtifexSoftware.mutool_Microsoft.Winget.Source_8wekyb3d8bbwe\mupdf-*-windows')
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

  $source = Resolve-FirstSourcePath -Candidates $tool.Sources
  if (-not $source) {
    Write-Warning "$($tool.Name): source not found"
    continue
  }

  Write-Host "Copying $($tool.Name) from $source"
  Copy-Item (Join-Path $source '*') $destination -Recurse -Force
}

Write-Host "Portable tool collection complete: $toolsRoot"

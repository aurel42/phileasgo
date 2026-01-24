# Download Natural Earth GeoJSON data for country boundaries
# Source: Natural Earth 110m Admin 0 Countries (pre-converted GeoJSON from GitHub)
# Downloads to ../../data/ folder, then strips it for embedding in pkg/geo/

$ErrorActionPreference = "Stop"

$ScriptDir = $PSScriptRoot
$RootDir = Join-Path $ScriptDir "..\..\" | Resolve-Path
$DataDir = Join-Path $RootDir "data"
$RawFile = Join-Path $DataDir "ne_110m_admin_0_countries.geojson"
$SlimFile = Join-Path $RootDir "pkg\geo\countries.geojson"

# Pre-converted GeoJSON from Natural Earth's official GitHub repo
$DownloadUrl = "https://raw.githubusercontent.com/nvkelso/natural-earth-vector/master/geojson/ne_110m_admin_0_countries.geojson"

# Check if slimmed file already exists
if (Test-Path $SlimFile) {
    Write-Host "Slimmed GeoJSON already exists at: $SlimFile"
    Write-Host "Delete the file manually if you want to re-download and regenerate."
    exit 0
}

# Create data directory if needed
if (-not (Test-Path $DataDir)) {
    New-Item -ItemType Directory -Path $DataDir -Force | Out-Null
    Write-Host "Created data directory: $DataDir"
}

# Step 1: Download raw GeoJSON (if not already present)
if (-not (Test-Path $RawFile)) {
    Write-Host "Downloading Natural Earth 110m Countries GeoJSON..."
    Write-Host "URL: $DownloadUrl"
    
    try {
        Invoke-WebRequest -Uri $DownloadUrl -OutFile $RawFile -UseBasicParsing
        $FileSize = [math]::Round((Get-Item $RawFile).Length / 1KB, 1)
        Write-Host "Downloaded: $RawFile ($FileSize KB)"
    } catch {
        Write-Host "ERROR: Failed to download"
        Write-Host $_.Exception.Message
        exit 1
    }
} else {
    Write-Host "Raw GeoJSON already exists: $RawFile"
}

# Step 2: Run slim_geojson to strip unnecessary properties
Write-Host ""
Write-Host "Running slim_geojson to strip unused properties..."

try {
    Push-Location $RootDir
    & go run ./cmd/slim_geojson $RawFile $SlimFile
    Pop-Location
    
    if ($LASTEXITCODE -ne 0) {
        Write-Host "ERROR: slim_geojson failed"
        exit 1
    }
    
    Write-Host ""
    Write-Host "SUCCESS: Slimmed GeoJSON ready for embedding at:"
    Write-Host "  $SlimFile"

} catch {
    Pop-Location
    Write-Host "ERROR: Failed to run slim_geojson"
    Write-Host $_.Exception.Message
    exit 1
}

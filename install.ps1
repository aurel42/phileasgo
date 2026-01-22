# install.ps1 - PhileasGo Installation Helper
# This script is idempotent - safe to run multiple times

Write-Host "=== PhileasGo Installation ===" -ForegroundColor Cyan
Write-Host ""

# Create directories (only if needed)
$dirs = @("data", "logs", "configs")
foreach ($dir in $dirs) {
    if (-not (Test-Path $dir)) {
        New-Item -ItemType Directory -Path $dir | Out-Null
        Write-Host "Created directory: $dir" -ForegroundColor Green
    }
}

# Download GeoNames cities1000 (only if not present)
$geonamesUrl = "https://download.geonames.org/export/dump/cities1000.zip"
$geonamesZip = "data/cities1000.zip"
$geonamesTxt = "data/cities1000.txt"

if (-not (Test-Path $geonamesTxt)) {
    Write-Host "Downloading GeoNames cities1000..." -ForegroundColor Yellow
    try {
        Invoke-WebRequest -Uri $geonamesUrl -OutFile $geonamesZip
        Write-Host "Extracting..." -ForegroundColor Yellow
        Expand-Archive -Path $geonamesZip -DestinationPath "data" -Force
        Remove-Item $geonamesZip
        Write-Host "GeoNames data installed!" -ForegroundColor Green
    } catch {
        Write-Host "Failed to download GeoNames: $_" -ForegroundColor Red
        Write-Host "Please download manually from: $geonamesUrl" -ForegroundColor Yellow
    }
} else {
    Write-Host "GeoNames data already exists - skipping." -ForegroundColor Gray
}

# Download GeoNames Admin1 Codes (for region names)
$admin1Url = "https://download.geonames.org/export/dump/admin1CodesASCII.txt"
$admin1File = "data/admin1CodesASCII.txt"

if (-not (Test-Path $admin1File)) {
    Write-Host "Downloading GeoNames Admin1 Codes..." -ForegroundColor Yellow
    try {
        Invoke-WebRequest -Uri $admin1Url -OutFile $admin1File
        Write-Host "Admin1 Codes downloaded!" -ForegroundColor Green
    } catch {
        Write-Host "Failed to download Admin1 Codes: $_" -ForegroundColor Red
    }
} else {
    Write-Host "Admin1 Codes already exists - skipping." -ForegroundColor Gray
}

# Download ETOPO1 Elevation Data (for Line-of-Sight)
$etopoUrl = "https://www.ngdc.noaa.gov/mgg/global/relief/ETOPO1/data/ice_surface/grid_registered/binary/etopo1_ice_g_i2.zip"
$etopoZip = "data/etopo1.zip"
$etopoDir = "data"     # content is inside a folder or we flat extract? The zip usually contains the file.
# ETOPO zip usually contains the .bin file directly or in a folder. We'll check.
# Actually, let's extract to data/etopo1 to be clean.
$etopoDest = "data/etopo1"
$etopoFile = "data/etopo1/etopo1_ice_g_i2.bin"

if (-not (Test-Path $etopoFile)) {
    Write-Host "Downloading ETOPO1 Elevation Data (~360MB Compressed)..." -ForegroundColor Yellow
    try {
        Invoke-WebRequest -Uri $etopoUrl -OutFile $etopoZip
        Write-Host "Extracting ETOPO1..." -ForegroundColor Yellow
        
        if (-not (Test-Path $etopoDest)) {
            New-Item -ItemType Directory -Path $etopoDest | Out-Null
        }
        
        Expand-Archive -Path $etopoZip -DestinationPath $etopoDest -Force
        Remove-Item $etopoZip
        
        # Verify file exists (sometimes names vary, we assume standard NOAA naming)
        if (Test-Path $etopoFile) {
             Write-Host "ETOPO1 data installed!" -ForegroundColor Green
        } else {
             Write-Host "ETOPO1 extracted but file name might differ. Check $etopoDest" -ForegroundColor Yellow
        }
    } catch {
        Write-Host "Failed to download ETOPO1: $_" -ForegroundColor Red
        Write-Host "Manually download from: $etopoUrl" -ForegroundColor Yellow
    }
} else {
    Write-Host "ETOPO1 data already exists - skipping." -ForegroundColor Gray
}

# LittleNavMap POIs instructions (only if not present)
$masterCsv = "data/Master.csv"
if (-not (Test-Path $masterCsv)) {
    Write-Host ""
    Write-Host "=== Manual Step Required ===" -ForegroundColor Yellow
    Write-Host "Please download LittleNavMap MSFS POIs from:"
    Write-Host "  https://flightsim.to/file/81114/littlenavmap-msfs-poi-s" -ForegroundColor Cyan
    Write-Host ""
    Write-Host "Extract the downloaded file and copy 'Master.csv' to the 'data/' folder."
    Write-Host ""
    Read-Host "Press Enter after you have copied Master.csv (or press Enter to skip for now)..."
} else {
    Write-Host "Master.csv already exists - skipping." -ForegroundColor Gray
}

# Generate config file (only if not present)
$configFile = "configs/phileas.yaml"
$exeFile = "phileasgo.exe"

if (-not (Test-Path $configFile)) {
    if (Test-Path $exeFile) {
        Write-Host "Generating config file..." -ForegroundColor Yellow
        & ".\$exeFile" --init-config
        if (Test-Path $configFile) {
            Write-Host "Config file created: $configFile" -ForegroundColor Green
        } else {
            Write-Host "Config generation may have failed. Check for errors above." -ForegroundColor Red
        }
    } else {
        Write-Host "phileasgo.exe not found - cannot generate config." -ForegroundColor Yellow
        Write-Host "Please build the application first with 'make build'" -ForegroundColor Yellow
    }
} else {
    Write-Host "Config file already exists - skipping." -ForegroundColor Gray
}

# .env Setup
if (-not (Test-Path ".env") -and -not (Test-Path ".env.local")) {
    if (Test-Path ".env.template") {
        Write-Host ""
        $choice = Read-Host "No environment file (.env) detected. Would you like to create .env.local and configure your API keys? (y/n)"
        if ($choice -eq "y") {
            Copy-Item ".env.template" ".env.local"
            Write-Host ".env.local created. Opening in Notepad..." -ForegroundColor Green
            Start-Process notepad.exe ".env.local"
        }
    }
}

# API Key configuration reminder
Write-Host ""
Write-Host "=== Configuration ===" -ForegroundColor Yellow
Write-Host "Edit .env.local or configs/phileas.yaml to add your API keys:"
Write-Host "  - GEMINI_API_KEY (REQUIRED for narration)" -ForegroundColor White
Write-Host "  - Azure/Fish Audio keys (optional)" -ForegroundColor Gray
Write-Host ""

Write-Host "Installation complete!" -ForegroundColor Green
Write-Host "Run phileasgo.exe to start the application." -ForegroundColor Cyan

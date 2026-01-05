# Copy SimConnect.dll from SDK to lib folder
# Tries MSFS 2024 SDK first, then MSFS 2020 SDK

$dest = "pkg\sim\simconnect\lib"
New-Item -ItemType Directory -Path $dest -Force | Out-Null

$sources = @(
    "C:\MSFS 2024 SDK\SimConnect SDK\lib\SimConnect.dll",
    "C:\MSFS SDK\SimConnect SDK\lib\SimConnect.dll"
)

foreach ($src in $sources) {
    if (Test-Path $src) {
        Copy-Item -Path $src -Destination $dest -Force
        Write-Host "Copied SimConnect.dll from: $src"
        exit 0
    }
}

Write-Error "SimConnect.dll not found in SDK paths. Install MSFS SDK first."
exit 1

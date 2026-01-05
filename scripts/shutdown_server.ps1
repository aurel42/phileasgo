$ErrorActionPreference = "Stop"

$Url = "http://localhost:1920/api/shutdown"

try {
    Write-Host "Sending shutdown request to $Url..."
    $response = Invoke-RestMethod -Uri $Url -Method Post
    Write-Host "Success: $response"
} catch {
    Write-Error "Failed to shut down server: $_"
}

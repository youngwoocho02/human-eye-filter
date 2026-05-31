$ErrorActionPreference = "Stop"

$repo = "youngwoocho02/human-eye-filter"
$installDir = "$env:LOCALAPPDATA\human-eye-filter"
$exe = "$installDir\hef.exe"

New-Item -ItemType Directory -Force -Path $installDir | Out-Null

$url = "https://github.com/$repo/releases/latest/download/hef-windows-amd64.exe"
Write-Host "Downloading hef for windows/amd64..."
Invoke-WebRequest -Uri $url -OutFile $exe -UseBasicParsing

$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($userPath -notlike "*$installDir*") {
    [Environment]::SetEnvironmentVariable("Path", "$installDir;$userPath", "User")
    $env:Path = "$installDir;$env:Path"
    Write-Host "Added $installDir to PATH (restart shell to apply)"
}

Write-Host "Installed hef to $exe"
& $exe version

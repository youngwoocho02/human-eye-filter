$ErrorActionPreference = "Stop"

$repo = "youngwoocho02/human-eye-filter"
$installDir = "$env:LOCALAPPDATA\human-eye-filter"
$exe = "$installDir\humaneye.exe"

New-Item -ItemType Directory -Force -Path $installDir | Out-Null

$url = "https://github.com/$repo/releases/latest/download/humaneye-windows-amd64.exe"
Write-Host "Downloading humaneye for windows/amd64..."
Invoke-WebRequest -Uri $url -OutFile $exe -UseBasicParsing

$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($userPath -notlike "*$installDir*") {
    [Environment]::SetEnvironmentVariable("Path", "$installDir;$userPath", "User")
    $env:Path = "$installDir;$env:Path"
    Write-Host "Added $installDir to PATH (restart shell to apply)"
}

Write-Host "Installed humaneye to $exe"
& $exe version

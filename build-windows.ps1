$ErrorActionPreference = "Stop"
$Mini = $PSScriptRoot
$Root = Split-Path -Parent $Mini
Set-Location $Root
$Out = Join-Path $Mini "dist\windows"
New-Item -ItemType Directory -Force -Path $Out | Out-Null
$env:CGO_ENABLED = "0"
$env:GOOS = "windows"
$env:GOARCH = "amd64"
go build -o (Join-Path $Out "miniapp.exe") .\MINI_APP\cmd\miniapp
if (Test-Path (Join-Path $Out "web")) { Remove-Item -Recurse -Force (Join-Path $Out "web") }
Copy-Item -Recurse -Force (Join-Path $Mini "web") (Join-Path $Out "web")
Write-Host "OK  $Out\miniapp.exe"
Write-Host "Перед запуском задайте MINIAPP_BOT_TOKEN в окружении."

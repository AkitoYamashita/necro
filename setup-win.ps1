## 実行: powershell -ExecutionPolicy Bypass .\setup-win.ps1
$ErrorActionPreference = "Stop"

$Version = "0.37.24"
$Url = "https://github.com/sagiegurari/cargo-make/releases/download/$Version/cargo-make-v$Version-x86_64-pc-windows-msvc.zip"

$Zip = "cargo-make.zip"
$Dir = "cargo-make"

# zip なければ DL
if (-not (Test-Path $Zip)) {
	Write-Host "download..."
	Invoke-WebRequest -Uri $Url -OutFile $Zip
}

# 解凍ディレクトリなければ 解凍
if (-not (Test-Path $Dir)) {
	Write-Host "extract..."
	Expand-Archive -Path $Zip -DestinationPath $Dir
}

Write-Host "done."
Write-Host "binary: .\$Dir\cargo-make-v$Version-x86_64-pc-windows-msvc\cargo-make.exe"

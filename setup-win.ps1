$ErrorActionPreference = "Stop"

$Url = "https://github.com/sagiegurari/cargo-make/releases/download/0.37.24/cargo-make-v0.37.24-x86_64-pc-windows-msvc.zip"

$Zip = "cargo-make.zip"
$Dir = "cargo-make"
$Out = "makers.exe"

# zip なければ DL
if (-not (Test-Path $Zip)) {
	Write-Host "download..."
	Invoke-WebRequest -Uri $Url -OutFile $Zip
}

# 解凍ディレクトリなければ 解凍
if (-not (Test-Path $Dir)) {
	Write-Host "extract..."
	New-Item -ItemType Directory -Force -Path $Dir | Out-Null
	Expand-Archive -Path $Zip -DestinationPath $Dir -Force
}

# exe を探して直下に makers.exe としてコピー
$Bin = Get-ChildItem -Path $Dir -Recurse -File |
	Where-Object { $_.Name -in @("cargo-make.exe", "makers.exe") } |
	Sort-Object { if ($_.Name -eq "cargo-make.exe") { 0 } else { 1 } } |
	Select-Object -First 1

if (-not $Bin) {
	throw "cargo-make.exe / makers.exe not found under .\$Dir"
}

Copy-Item -Force $Bin.FullName $Out

Write-Host "installed: .\$Out"
Write-Host "try: .\makers.exe --version"

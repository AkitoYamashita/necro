## 実行コマンド: powershell -ExecutionPolicy Bypass .\setup-win.ps1
$ErrorActionPreference = "Stop"

$Out = Join-Path (Get-Location) "makers.exe"
$Url = "https://github.com/sagiegurari/cargo-make/releases/download/0.37.24/cargo-make-v0.37.24-x86_64-pc-windows-msvc.zip"

$tmp = Join-Path $env:TEMP ("cargo-make-" + [Guid]::NewGuid().ToString())
New-Item -ItemType Directory -Force -Path $tmp | Out-Null

try {
	$zip = Join-Path $tmp "cargo-make.zip"
	Invoke-WebRequest -Uri $Url -OutFile $zip

	Expand-Archive -Path $zip -DestinationPath $tmp -Force

	# zip内のバイナリを探す（cargo-make.exe優先、なければmakers.exe）
	$bin = Get-ChildItem -Path $tmp -Recurse -File |
		Where-Object { $_.Name -in @("cargo-make.exe", "makers.exe") } |
		Sort-Object { if ($_.Name -eq "cargo-make.exe") { 0 } else { 1 } } |
		Select-Object -First 1

	if (-not $bin) {
		Write-Host "debug: files in temp dir:" -ForegroundColor Yellow
		Get-ChildItem -Path $tmp -Recurse -File | Select-Object FullName | Format-Table -AutoSize
		throw "cargo-make.exe / makers.exe not found in zip (maybe removed by Defender/EDR?)"
	}

	Copy-Item -Force $bin.FullName $Out

	New-Item -ItemType Directory -Force -Path dist, log, out | Out-Null

	Write-Host "installed: $Out (from: $($bin.FullName))"
	Write-Host "try: .\makers.exe --version"
	Write-Host "try: .\makers.exe --list-all-steps"
}
finally {
	Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}

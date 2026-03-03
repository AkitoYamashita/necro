# execute: `powershell -ExecutionPolicy Bypass .\script.ps1`
# Usage:
#   pwsh -File .\gen-aws-config.ps1 -InJson .\org-accounts.json -OutJsonl .\org-account-list.jsonl -Template .\aws_config_template.txt -OutConfig .\aws_config.txt -Region ap-northeast-1 -SsoSession AWS_SESSION

param(
	[Parameter(Mandatory=$true)]
	[string]$InJson,

	[Parameter(Mandatory=$true)]
	[string]$OutJsonl,

	[Parameter(Mandatory=$true)]
	[string]$Template,

	[Parameter(Mandatory=$true)]
	[string]$OutConfig,

	[string]$Region = "ap-northeast-1",
	[string]$SsoSession = "AWS_SESSION"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Ensure-ParentDir([string]$path) {
	$dir = Split-Path -Parent $path
	if ($dir -and !(Test-Path -LiteralPath $dir)) {
		New-Item -ItemType Directory -Path $dir -Force | Out-Null
	}
}

function To-LowerSafe([string]$s) {
	if ($null -eq $s) { return "" }
	return $s.ToLowerInvariant()
}

function Derive-SystemEnv([string]$profileName) {
	# "COM_PRD" -> system=com, env=prd
	$parts = $profileName -split "_", 2
	$system = if ($parts.Length -ge 1) { To-LowerSafe $parts[0] } else { "" }
	$env    = if ($parts.Length -ge 2) { To-LowerSafe $parts[1] } else { "" }
	return @{ system = $system; env = $env }
}

# --- Read input JSON ---
if (!(Test-Path -LiteralPath $InJson)) {
	throw "Input file not found: $InJson"
}
$raw = Get-Content -LiteralPath $InJson -Raw
$data = $raw | ConvertFrom-Json

if ($null -eq $data.Accounts) {
	throw "Invalid JSON: missing .Accounts"
}

# --- Build records ---
$records = @()
foreach ($acc in $data.Accounts) {
	$name = [string]$acc.Name
	$id   = [string]$acc.Id
	if ([string]::IsNullOrWhiteSpace($name) -or [string]::IsNullOrWhiteSpace($id)) {
		continue
	}
	$se = Derive-SystemEnv $name
	$records += [pscustomobject]@{
		profile    = $name
		account_id = $id
		system     = $se.system
		env        = $se.env
	}
}

# --- Write NDJSON (1 account per line) ---
Ensure-ParentDir $OutJsonl
$ndjson = $records | ForEach-Object { $_ | ConvertTo-Json -Compress }
Set-Content -LiteralPath $OutJsonl -Value $ndjson -Encoding utf8

# --- Generate aws_config.txt (template + generated block) ---
if (!(Test-Path -LiteralPath $Template)) {
	throw "Template file not found: $Template"
}
Ensure-ParentDir $OutConfig

$templateText = Get-Content -LiteralPath $Template -Raw

$lines = New-Object System.Collections.Generic.List[string]
$lines.Add($templateText.TrimEnd("`r","`n"))
$lines.Add("")
$lines.Add(";; GENERATED CODE DO NOT DELETE")
$lines.Add("")

foreach ($r in $records) {
	$lines.Add("[profile $($r.profile)]")
	$lines.Add("region = $Region")
	$lines.Add("sso_account_id = $($r.account_id)")
	$lines.Add("sso_session = $SsoSession")
	$lines.Add("sso_role_name = ps-org-admin-for-$($r.system)-$($r.env)")
	$lines.Add("")
}

Set-Content -LiteralPath $OutConfig -Value ($lines -join "`r`n") -Encoding utf8

Write-Host "OK"
Write-Host "  NDJSON : $OutJsonl"
Write-Host "  CONFIG : $OutConfig"
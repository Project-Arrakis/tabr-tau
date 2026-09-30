<#
.SYNOPSIS
  Copy a Dune: Awakening single-player save into a labelled, LOCAL snapshot folder (and optional zip).
.DESCRIPTION
  Read-only against the game folders. Copies game.db, game_prepatch.db, autosave/, SOLO/ to
  <Dest>\<Label>\ and writes manifest.json LAST (its presence means the snapshot is complete).
  By default it does NOT copy Game.ini (it contains account IDs, platform IDs and display names) and
  the default destination is a local, non-cloud-synced folder. Nothing in the game folders is modified.
  Take snapshots with the game CLOSED; the manifest records if it was running.
  Prerequisites: Windows PowerShell 5.1+ or PowerShell 7. If scripts are blocked, run once in this window:
      Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass
.EXAMPLE
  .\snapshot.ps1 -Label A0-baseline
  .\snapshot.ps1 -Label A0-baseline -SteamId 12345678901234567   # when several accounts exist
  .\snapshot.ps1 -Label B1-ingame-literjon -Zip
#>
param(
  [Parameter(Mandatory)][ValidatePattern('^[A-Za-z0-9._-]{1,64}$')][string]$Label,
  [string]$Dest = (Join-Path $env:USERPROFILE 'tabr-tau-snapshots'),
  [switch]$Zip,
  [switch]$IncludeConfig,  # copies ServerCustomSettings.ini only (never Game.ini)
  [ValidatePattern('^\d{17}$')][string]$SteamId   # required when several Steam accounts have saves on this PC
)
$ErrorActionPreference = 'Stop'
if (-not $env:LOCALAPPDATA) { throw 'LOCALAPPDATA is not set: run this on the Windows PC that has the game.' }
$saved = Join-Path $env:LOCALAPPDATA 'DuneSandbox\Saved'
$fls = Join-Path $saved 'Cloud\PlayerClientStorage\FLS_retail'
if (-not (Test-Path $fls)) { throw "Not found: $fls" }
$cands = @(Get-ChildItem $fls -Directory | Where-Object Name -Match '^\d{17}$')
if ($SteamId) {
  $acct = @($cands | Where-Object Name -eq $SteamId)
  if ($acct.Count -ne 1) { throw "No folder named $SteamId under $fls" }
} elseif ($cands.Count -eq 1) {
  $acct = $cands
} else {
  Write-Host "Found $($cands.Count) Steam-id folders under $fls :"
  $cands | ForEach-Object {
    $db = Join-Path $_.FullName 'game.db'
    [pscustomobject]@{ SteamId = $_.Name; GameDb = (Test-Path $db); GameDbModified = $(if (Test-Path $db) { (Get-Item $db).LastWriteTime } else { $null }); GameDbBytes = $(if (Test-Path $db) { (Get-Item $db).Length } else { $null }) }
  } | Format-Table -AutoSize | Out-String | Write-Host
  throw 'Several accounts found. Re-run with -SteamId <id> (usually the one whose game.db was modified most recently).'
}
if (-not (Test-Path (Join-Path $acct[0].FullName 'game.db'))) { throw 'game.db not found in the account folder.' }
New-Item -ItemType Directory -Path $Dest -Force | Out-Null
$out = Join-Path $Dest $Label
if (Test-Path $out) { throw "Snapshot '$Label' already exists at $out. Pick a new label; never reuse one." }

$running = [bool](Get-Process -Name 'DuneSandbox-Win64-Shipping' -ErrorAction SilentlyContinue)
if ($running) { Write-Warning 'The game is RUNNING. game.db may lag in-memory state or be mid-write. Quit the game and take the snapshot again.' }

New-Item -ItemType Directory -Path $out | Out-Null
foreach ($f in 'game.db','game_prepatch.db') {
  $p = Join-Path $acct[0].FullName $f; if (Test-Path $p) { Copy-Item $p $out }
}
foreach ($d in 'autosave','SOLO') {
  $p = Join-Path $acct[0].FullName $d; if (Test-Path $p) { Copy-Item $p (Join-Path $out $d) -Recurse }
}
if ($IncludeConfig) {
  $p = Join-Path $saved 'Config\Windows\ServerCustomSettings.ini'
  if (Test-Path $p) { $cfg = Join-Path $out 'config'; New-Item -ItemType Directory $cfg | Out-Null; Copy-Item $p $cfg }
}
$files = Get-ChildItem $out -Recurse -File | ForEach-Object {
  [pscustomobject]@{
    path = $_.FullName.Substring($out.Length + 1); bytes = $_.Length
    mtimeUtc = $_.LastWriteTimeUtc.ToString('o'); sha256 = (Get-FileHash $_.FullName -Algorithm SHA256).Hash
  }
}
# Manifest is written LAST. Consumers must ignore any snapshot folder that has no manifest.json.
[pscustomobject]@{ label = $Label; takenUtc = (Get-Date).ToUniversalTime().ToString('o'); gameRunning = $running; files = $files } |
  ConvertTo-Json -Depth 4 | Set-Content (Join-Path $out 'manifest.json')
Write-Host "Snapshot '$Label' -> $out ($($files.Count) files). Game running: $running"
if ($Zip) {
  $zip = "$out.zip"; Compress-Archive -Path $out -DestinationPath $zip
  Write-Host "Zip: $zip  (sha256 $((Get-FileHash $zip -Algorithm SHA256).Hash))"
}
Write-Host 'This snapshot contains your account/platform IDs. Keep it private; do not commit, post, or share it.'

<#
.SYNOPSIS
  Copy a Dune: Awakening single-player save + configs into a labelled snapshot folder.
.DESCRIPTION
  Read-only against the game folders. Copies game.db, game_prepatch.db, autosave/, SOLO/ and the
  Windows config INIs to <Dest>\<Label>\, and writes manifest.json (sizes, mtimes, sha256, and
  whether the game process is running). Never deletes or overwrites anything in the game folders.
  Snapshot with the game CLOSED for a consistent game.db; the manifest records if it was running.
.EXAMPLE
  .\snapshot.ps1 -Label 01-baseline
  .\snapshot.ps1 -Label 02-after-give -Dest "$env:OneDrive\tabr-tau-samples\snapshots"
#>
param(
  [Parameter(Mandatory)][string]$Label,
  [string]$Dest = "$env:OneDrive\tabr-tau-samples\snapshots"
)
$ErrorActionPreference = 'Stop'
$saved = Join-Path $env:LOCALAPPDATA 'DuneSandbox\Saved'
$fls = Join-Path $saved 'Cloud\PlayerClientStorage\FLS_retail'
$acct = Get-ChildItem $fls -Directory | Where-Object Name -Match '^\d{17}$'
if ($acct.Count -ne 1) { throw "Expected exactly one Steam-id folder under $fls, found $($acct.Count)" }
$out = Join-Path $Dest $Label
if (Test-Path $out) { throw "Snapshot '$Label' already exists at $out. Pick a new label." }
New-Item -ItemType Directory -Path $out | Out-Null

$running = [bool](Get-Process -Name 'DuneSandbox-Win64-Shipping' -ErrorAction SilentlyContinue)
if ($running) { Write-Warning 'Game is RUNNING: game.db may not reflect in-memory state, and may be mid-write.' }

foreach ($f in 'game.db','game_prepatch.db') { Copy-Item (Join-Path $acct.FullName $f) $out }
Copy-Item (Join-Path $acct.FullName 'autosave') (Join-Path $out 'autosave') -Recurse
Copy-Item (Join-Path $acct.FullName 'SOLO')     (Join-Path $out 'SOLO')     -Recurse
$cfg = Join-Path $out 'config'; New-Item -ItemType Directory -Path $cfg | Out-Null
foreach ($f in 'Game.ini','GameUserSettings.ini','ServerCustomSettings.ini','Engine.ini','Input.ini') {
  $p = Join-Path $saved "Config\Windows\$f"; if (Test-Path $p) { Copy-Item $p $cfg }
}
$files = Get-ChildItem $out -Recurse -File | ForEach-Object {
  [pscustomobject]@{
    path = $_.FullName.Substring($out.Length + 1); bytes = $_.Length
    mtimeUtc = $_.LastWriteTimeUtc.ToString('o'); sha256 = (Get-FileHash $_.FullName -Algorithm SHA256).Hash
  }
}
[pscustomobject]@{ label = $Label; takenUtc = (Get-Date).ToUniversalTime().ToString('o'); gameRunning = $running; files = $files } |
  ConvertTo-Json -Depth 4 | Set-Content (Join-Path $out 'manifest.json')
Write-Host "Snapshot '$Label' -> $out ($($files.Count) files). Game running: $running"

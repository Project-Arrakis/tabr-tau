<#
.SYNOPSIS
  Send a completed tabr-tau snapshot to an analysis host over scp/ssh, with hash verification. No cloud token.
.DESCRIPTION
  1. Requires <SnapshotDir>\manifest.json (proof the snapshot is complete; written last by snapshot.ps1).
  2. Zips the snapshot, computes SHA-256.
  3. scp's the zip to -Target, then asks the remote to compute the SHA-256 and compares.
  Uses the Windows built-in OpenSSH client (scp.exe / ssh.exe). Authenticate with an SSH KEY (recommended);
  the script never asks for or stores a password and never prints key material.
  The snapshot contains account/platform IDs: send only to a host you control, to an UNPRIVILEGED account
  (do not use root on a shared or production host), and delete both copies when the experiment ends.
.PARAMETER Target
  user@host:/absolute/remote/dir   e.g. tabrdrop@192.0.2.10:/home/tabrdrop/incoming
.EXAMPLE
  .\send-snapshot.ps1 -SnapshotDir "$env:USERPROFILE\tabr-tau-snapshots\A0-baseline" -Target tabrdrop@192.0.2.10:/home/tabrdrop/incoming
#>
param(
  [Parameter(Mandatory)][string]$SnapshotDir,
  [Parameter(Mandatory)][ValidatePattern('^[A-Za-z0-9._-]+@[A-Za-z0-9._:-]+:/[A-Za-z0-9._/~-]+$')][string]$Target,
  [string]$IdentityFile,             # optional path to a private key; passed as ssh -i
  [switch]$KeepZip
)
$ErrorActionPreference = 'Stop'
foreach ($exe in 'scp.exe', 'ssh.exe') {
  if (-not (Get-Command $exe -ErrorAction SilentlyContinue)) {
    throw "$exe not found. Windows: Settings > Apps > Optional features > add 'OpenSSH Client'."
  }
}
$dir = (Resolve-Path $SnapshotDir).Path
if (-not (Test-Path (Join-Path $dir 'manifest.json'))) { throw "No manifest.json in $dir: snapshot incomplete or not a snapshot folder." }
$m = Get-Content (Join-Path $dir 'manifest.json') -Raw | ConvertFrom-Json
if ($m.gameRunning) { Write-Warning "This snapshot was taken while the game was RUNNING (manifest says so). Prefer a fresh snapshot with the game closed." }

# Split user@host:path (validated above).
$remoteUserHost, $remoteDir = $Target -split ':/', 2
$remoteDir = '/' + $remoteDir
$label = Split-Path $dir -Leaf
if ($label -notmatch '^[A-Za-z0-9._-]{1,64}$') { throw "Snapshot folder name '$label' has characters outside A-Z a-z 0-9 . _ - (it is used in a remote command)." }
$zip = Join-Path ([IO.Path]::GetTempPath()) "$label.zip"
if (Test-Path $zip) { Remove-Item $zip -Force }
Compress-Archive -Path $dir -DestinationPath $zip
$localHash = (Get-FileHash $zip -Algorithm SHA256).Hash.ToLower()
Write-Host "Zip $zip  sha256 $localHash"

$sshArgs = @('-o', 'BatchMode=yes', '-o', 'StrictHostKeyChecking=accept-new')   # key auth only; never prompts for a password
if ($IdentityFile) { $sshArgs += @('-i', (Resolve-Path $IdentityFile).Path) }
try {
  & scp.exe @sshArgs $zip "${remoteUserHost}:${remoteDir}/"
  if ($LASTEXITCODE -ne 0) { throw "scp failed (exit $LASTEXITCODE). Check the host, the key, and that ${remoteDir} exists and is writable." }
  $remoteFile = "$remoteDir/$label.zip"
  $remoteHash = (& ssh.exe @sshArgs $remoteUserHost "sha256sum '$remoteFile'") -split '\s+' | Select-Object -First 1
  if ($remoteHash -ne $localHash) { throw "HASH MISMATCH: local $localHash, remote $remoteHash. Do not use the remote copy." }
  Write-Host "OK: '$label' delivered and verified ($localHash)."
  Write-Host 'Reminder: the archive contains your account IDs. Delete the local and remote copies when finished.'
}
finally {
  if (-not $KeepZip -and (Test-Path $zip)) { Remove-Item $zip -Force }
}

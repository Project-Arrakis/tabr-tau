<#
.SYNOPSIS
  Prints the linker flags that stamp the version into a build of the editor.
.DESCRIPTION
  The one place that works out the version of a build (design note docs/design/releases-and-updates.md, section 1). The local build,
  the CI cross-compile and the release workflow all call it, so that they cannot drift apart. It reads git only and changes nothing.

    Version  the nearest tag vMAJOR.MINOR.PATCH as MAJOR.MINOR.PATCH; with +N added when the build is N commits after the tag, and
             .dirty added (or +dirty after a plain tag) when tracked files have local changes. With no tag yet it is 0.0.0+<build>.
             A tag has no pre-release part unless it was written with one (v1.0.0-rc.1): that is kept.
    Build    the number of commits that lead to HEAD (git rev-list --count HEAD)
    Commit   the first 7 characters of the commit id
    Date     the commit date, not the clock, so a rebuild of one commit gives the same bytes

  The output is one line to pass to go build, for example:
    $flags = & scripts/buildinfo.ps1
    go build -trimpath -ldflags $flags ./cmd/tabr-tau
.PARAMETER Repo
  The repository folder. Default: the one this script is in.
.PARAMETER Module
  The Go module path. Default: read from go.mod in the repository.
.PARAMETER BaseFlags
  Linker flags put in front of the stamp. Default: the flags of the Windows build.
.PARAMETER AllowShallow
  Accept a shallow clone. The build number and the commits since the tag are wrong there, so this is for tests only.
.EXAMPLE
  .\scripts\buildinfo.ps1
  -H=windowsgui -s -w -X github.com/example/module/internal/buildinfo.Version=0.1.0+4 -X ...Build=412 -X ...Commit=a1b2c3d -X ...Date=2026-11-16
#>
param(
  [string]$Repo = (Split-Path -Parent $PSScriptRoot),
  [string]$Module = '',
  [ValidatePattern('^[-A-Za-z0-9=_. ]*$')][string]$BaseFlags = '-H=windowsgui -s -w',
  [switch]$AllowShallow
)
$ErrorActionPreference = 'Stop'
$Repo = (Resolve-Path -LiteralPath $Repo).Path

# Runs git in the repository and returns its output; a failure stops the script (the caller can ask for it to be tolerated).
function Invoke-Git {
  param([string[]]$GitArgs, [switch]$Tolerate)
  # Windows PowerShell 5.1 turns a line on stderr into a terminating error under Stop; git's exit code is what is judged here.
  $saved = $ErrorActionPreference
  $ErrorActionPreference = 'Continue'
  $out = & git -C $Repo @GitArgs 2>$null
  $code = $LASTEXITCODE
  $ErrorActionPreference = $saved
  if ($code -ne 0) {
    if ($Tolerate) { return $null }
    throw "git $($GitArgs -join ' ') failed in $Repo"
  }
  return ($out -join "`n").Trim()
}

if ($Module -eq '') {
  $line = Select-String -LiteralPath (Join-Path $Repo 'go.mod') -Pattern '^module\s+(\S+)\s*$' | Select-Object -First 1
  if (-not $line) { throw "no module line in $(Join-Path $Repo 'go.mod')" }
  $Module = $line.Matches[0].Groups[1].Value
}
if ($Module -notmatch '^[A-Za-z0-9._/~-]+$') { throw "unexpected module path: $Module" }

if (-not $AllowShallow -and (Invoke-Git @('rev-parse', '--is-shallow-repository')) -eq 'true') {
  throw 'this is a shallow clone: the build number would be wrong. Fetch the full history (git fetch --unshallow).'
}

$commit = Invoke-Git @('rev-parse', '--short=7', 'HEAD')
$build = Invoke-Git @('rev-list', '--count', 'HEAD')
$date = Invoke-Git @('show', '-s', '--format=%cs', 'HEAD')
if ($commit -notmatch '^[0-9a-f]{7}$' -or $build -notmatch '^[0-9]+$' -or $date -notmatch '^[0-9]{4}-[0-9]{2}-[0-9]{2}$') {
  throw "unexpected git output: commit '$commit', build '$build', date '$date'"
}

# The nearest release tag. design/v... and kit/v... are tags of other modules and do not match.
$desc = Invoke-Git @('describe', '--tags', '--long', '--match', 'v[0-9]*') -Tolerate
if ($desc -and $desc -match '^v([0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?)-([0-9]+)-g[0-9a-f]+$') {
  $version = $Matches[1]
  if ([int]$Matches[2] -gt 0) { $version += '+' + $Matches[2] }
} else {
  $version = '0.0.0+' + $build
}
$dirty = Invoke-Git @('status', '--porcelain', '--untracked-files=no')
if ($dirty) {
  if ($version.Contains('+')) { $version += '.dirty' } else { $version += '+dirty' }
}

$pkg = "$Module/internal/buildinfo"
$parts = @()
if ($BaseFlags.Trim() -ne '') { $parts += $BaseFlags.Trim() }
$parts += "-X $pkg.Version=$version", "-X $pkg.Build=$build", "-X $pkg.Commit=$commit", "-X $pkg.Date=$date"
Write-Output ($parts -join ' ')

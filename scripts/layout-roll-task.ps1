<#
.SYNOPSIS
  Registers (or removes) the Windows scheduled task that runs "tabr-tau layout-roll" every Tuesday at 05:00 local time (#1141).

.DESCRIPTION
  Nothing is registered unless you run this script yourself. See docs/help/deep-desert-layout-roll.md, which has every command with full paths and, under "Going back", where the
  Game.ini backup and the checkpoint of the save are written and how to restore them.

  The examples below are written for PowerShell opened in the "scripts" folder of the repository. From any other folder, use the full path of this script instead, for example:
    & "$env:USERPROFILE\tabr-tau\scripts\layout-roll-task.ps1" -ExePath "$env:USERPROFILE\Downloads\tabr-tau.exe" -WhatIf
  $env:USERPROFILE is your own user folder. Replace "tabr-tau\scripts" with the folder the repository was downloaded to, and "Downloads\tabr-tau.exe" with where tabr-tau.exe is.
  "C:\path\to\tabr-tau.exe" in the examples below stands for that same full path of tabr-tau.exe.

  Install:   .\layout-roll-task.ps1 -ExePath "C:\path\to\tabr-tau.exe"
  Preview:   .\layout-roll-task.ps1 -ExePath "C:\path\to\tabr-tau.exe" -WhatIf
  Name the save (needed when more than one save is found on the computer):
             .\layout-roll-task.ps1 -ExePath "C:\path\to\tabr-tau.exe" -SavePath "C:\path\to\the\folder\with\game.db"
  Recycle your Deep Desert bases before the roll (OFF unless you give it; read "Recycling the Deep Desert bases" in docs/help/deep-desert-layout-roll.md first, and run
             "tabr-tau layout-roll --dry-run --recycle-bases" by hand first):
             .\layout-roll-task.ps1 -ExePath "C:\path\to\tabr-tau.exe" -RecycleBases
  Keep your Deep Desert markers (a real run removes them from the save otherwise, so the map shows no places of earlier layouts; see "The Deep Desert markers" in docs/help/deep-desert-layout-roll.md):
             .\layout-roll-task.ps1 -ExePath "C:\path\to\tabr-tau.exe" -KeepMarkers
  Remove:    .\layout-roll-task.ps1 -Remove

  The task belongs to the current user only, runs only while that user is logged on, has no password stored and asks for no elevation. tabr-tau.exe is a windowless program, so
  the run shows no window. If the computer was off at 05:00 the task runs when it is next available. The roll itself refuses while the game is running and rolls at most once
  per week, so an extra run does no harm. It also takes a verified checkpoint copy of the save, reads the save and refuses while a base or the character of yours is in the Deep Desert, after exporting each
  such base to a base backup file in a dated folder beside layout-roll.json. Vehicles of yours in the Deep Desert are stored (a Scout, a Sandbike) or moved in front of the character by this run itself, in one edit of the save that is read back. The save it checks is the one named with -SavePath
  (written into the task action as --save <path>, before the word layout-roll), else the only one found on the computer; with none or several found the run refuses, so give -SavePath when you
  have more than one save. A run that changed nothing says why in %APPDATA%\tabr-tau\layout-roll.json ("lastRun"), with ONE exception: the editor holds the save for as long as its window is open,
  so close the editor before 05:00. A run that meets the open editor (or another layout roll) is refused before it reads layout-roll.json and changes nothing; it exits with an error (the task's
  Last Run Result is not 0), is not retried, and is recorded only as one dated line in %APPDATA%\tabr-tau\layout-roll.log. A run that is given no save to check is refused the same way.

.PARAMETER ExePath
  The full path of tabr-tau.exe. Keep the file where it is: the task points at it.

.PARAMETER SavePath
  Optional. The save the weekly run checks: the game.db file or the folder that holds it. It must exist. It is written into the task action as --save "<path>" before the word
  layout-roll. Without it the run checks the only save found on the computer and refuses when there are none or several. A relative path is made into a full path first, because the
  task does not run from your folder. Ignored with -Remove.

.PARAMETER RecycleBases
  Optional, off by default. Adds --recycle-bases to the task action (after the word layout-roll): the weekly run then recycles your own bases in the Deep Desert, one at a time, each with a
  Solido, before it rolls, instead of refusing while a base stands. The Reserve must be empty before each base, so the run moves the whole Reserve into your storage boxes named
  DD-1, DD-2 ... up to DD-6 (Developer storage containers, named exactly so in the game, as many as run in a row from DD-1) between the bases and after the last one; a run that could not recycle every base (a box is missing, or the
  boxes are full) stops before the roll and says so in %APPDATA%\tabr-tau\layout-roll.json, and the next run continues. The task is given 30 minutes instead of 5. Without -RecycleBases
  the task is exactly what it was.

.PARAMETER KeepMarkers
  Optional, off by default. Adds --keep-markers to the task action (after the word layout-roll): the weekly run then leaves the Deep Desert markers in the save. Without it the run removes
  the Deep Desert's markers, what your character discovered of them and its resource fields from the save before it changes Game.ini (single-player never clears them by itself, so the
  game's map would still show places of earlier layouts); the game adds markers again as you discover places. Nothing else is touched; the checkpoint of the save holds them if you want them back.

.PARAMETER Remove
  Remove the task instead of registering it.
#>
[CmdletBinding(SupportsShouldProcess = $true)]
param(
    [string]$ExePath,
    [string]$SavePath,
    [switch]$RecycleBases,
    [switch]$KeepMarkers,
    [switch]$Remove
)

$ErrorActionPreference = 'Stop'
$TaskName = 'Tabr Tau - Deep Desert layout roll'

if ($Remove) {
    $existing = Get-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue
    if (-not $existing) {
        Write-Host "No task named '$TaskName' is registered; nothing to remove."
        return
    }
    if ($PSCmdlet.ShouldProcess($TaskName, 'Remove the scheduled task')) {
        Unregister-ScheduledTask -TaskName $TaskName -Confirm:$false
        Write-Host "Removed the task '$TaskName'. Game.ini and its backups were not touched."
    }
    return
}

if (-not $ExePath) { throw 'Give the full path of tabr-tau.exe with -ExePath.' }
if (-not [System.IO.Path]::IsPathRooted($ExePath)) { throw "-ExePath must be a full path, not '$ExePath'." }
if (-not (Test-Path -LiteralPath $ExePath -PathType Leaf)) { throw "There is no file at '$ExePath'." }

$layoutRollWords = 'layout-roll'
if ($RecycleBases) { $layoutRollWords = 'layout-roll --recycle-bases' }
if ($KeepMarkers) { $layoutRollWords = $layoutRollWords + ' --keep-markers' }
$taskArgument = $layoutRollWords
if ($SavePath) {
    # The save must exist; the task runs from another folder, so the path is made a full one now.
    if (-not (Test-Path -LiteralPath $SavePath)) { throw "There is no file or folder at '$SavePath' (-SavePath)." }
    $fullSave = (Resolve-Path -LiteralPath $SavePath).ProviderPath
    if ($fullSave.Contains('"')) { throw "-SavePath must not contain a double quote: '$SavePath'." }
    # The path goes in double quotes inside the task's argument line. A run of backslashes just before the closing quote would escape it, so it is doubled.
    $quotedSave = '"' + ($fullSave -replace '(\\+)$', '$1$1') + '"'
    $taskArgument = "--save $quotedSave $layoutRollWords"
}

$user = [System.Security.Principal.WindowsIdentity]::GetCurrent().Name
$action = New-ScheduledTaskAction -Execute $ExePath -Argument $taskArgument
$trigger = New-ScheduledTaskTrigger -Weekly -DaysOfWeek Tuesday -At '5:00AM'
# A run that recycles bases copies the save, exports and recycles every base on a working copy, and writes the save once: it is given longer than the plain run.
$minutes = 5
if ($RecycleBases) { $minutes = 30 }
$settings = New-ScheduledTaskSettingsSet -StartWhenAvailable -Hidden -MultipleInstances IgnoreNew `
    -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -ExecutionTimeLimit (New-TimeSpan -Minutes $minutes)
$principal = New-ScheduledTaskPrincipal -UserId $user -LogonType Interactive -RunLevel Limited

if ($PSCmdlet.ShouldProcess($TaskName, "Register: every Tuesday 05:00, as $user, running '$ExePath $taskArgument'")) {
    Register-ScheduledTask -TaskName $TaskName -Action $action -Trigger $trigger -Settings $settings -Principal $principal `
        -Description 'Changes the single-player Deep Desert layout once a week (tabr-tau layout-roll). Remove it with layout-roll-task.ps1 -Remove.' | Out-Null
    Write-Host "Registered the task '$TaskName' (Tuesday 05:00, user $user). Remove it with: layout-roll-task.ps1 -Remove"
}

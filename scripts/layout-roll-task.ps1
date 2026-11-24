<#
.SYNOPSIS
  Registers (or removes) the Windows scheduled task that runs "tabr-tau layout-roll" every Tuesday at 05:00 local time (#1141).

.DESCRIPTION
  Nothing is registered unless you run this script yourself. See docs/help/deep-desert-layout-roll.md.

  Install:   .\layout-roll-task.ps1 -ExePath "C:\path\to\tabr-tau.exe"
  Preview:   .\layout-roll-task.ps1 -ExePath "C:\path\to\tabr-tau.exe" -WhatIf
  Name the save (needed when more than one save is found on the computer):
             .\layout-roll-task.ps1 -ExePath "C:\path\to\tabr-tau.exe" -SavePath "C:\path\to\the\folder\with\game.db"
  Remove:    .\layout-roll-task.ps1 -Remove

  The task belongs to the current user only, runs only while that user is logged on, has no password stored and asks for no elevation. tabr-tau.exe is a windowless program, so
  the run shows no window. If the computer was off at 05:00 the task runs when it is next available. The roll itself refuses while the game is running and rolls at most once
  per week, so an extra run does no harm. It also takes a verified checkpoint copy of the save, reads the save and refuses while a base or the character of yours is in the Deep Desert, after exporting each
  such base to a base backup file in a dated folder beside layout-roll.json. Vehicles of yours in the Deep Desert are stored (a Scout, a Sandbike) or moved in front of the character by this run itself, in one edit of the save that is read back. The save it checks is the one named with -SavePath
  (written into the task action as --save <path>, before the word layout-roll), else the only one found on the computer; with none or several found the run refuses, so give -SavePath when you
  have more than one save. A run that changed nothing says why in %APPDATA%\tabr-tau\layout-roll.json ("lastRun").

.PARAMETER ExePath
  The full path of tabr-tau.exe. Keep the file where it is: the task points at it.

.PARAMETER SavePath
  Optional. The save the weekly run checks: the game.db file or the folder that holds it. It must exist. It is written into the task action as --save "<path>" before the word
  layout-roll. Without it the run checks the only save found on the computer and refuses when there are none or several. A relative path is made into a full path first, because the
  task does not run from your folder. Ignored with -Remove.

.PARAMETER Remove
  Remove the task instead of registering it.
#>
[CmdletBinding(SupportsShouldProcess = $true)]
param(
    [string]$ExePath,
    [string]$SavePath,
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

$taskArgument = 'layout-roll'
if ($SavePath) {
    # The save must exist; the task runs from another folder, so the path is made a full one now.
    if (-not (Test-Path -LiteralPath $SavePath)) { throw "There is no file or folder at '$SavePath' (-SavePath)." }
    $fullSave = (Resolve-Path -LiteralPath $SavePath).ProviderPath
    if ($fullSave.Contains('"')) { throw "-SavePath must not contain a double quote: '$SavePath'." }
    # The path goes in double quotes inside the task's argument line. A run of backslashes just before the closing quote would escape it, so it is doubled.
    $quotedSave = '"' + ($fullSave -replace '(\\+)$', '$1$1') + '"'
    $taskArgument = "--save $quotedSave layout-roll"
}

$user = [System.Security.Principal.WindowsIdentity]::GetCurrent().Name
$action = New-ScheduledTaskAction -Execute $ExePath -Argument $taskArgument
$trigger = New-ScheduledTaskTrigger -Weekly -DaysOfWeek Tuesday -At '5:00AM'
$settings = New-ScheduledTaskSettingsSet -StartWhenAvailable -Hidden -MultipleInstances IgnoreNew `
    -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -ExecutionTimeLimit (New-TimeSpan -Minutes 5)
$principal = New-ScheduledTaskPrincipal -UserId $user -LogonType Interactive -RunLevel Limited

if ($PSCmdlet.ShouldProcess($TaskName, "Register: every Tuesday 05:00, as $user, running '$ExePath $taskArgument'")) {
    Register-ScheduledTask -TaskName $TaskName -Action $action -Trigger $trigger -Settings $settings -Principal $principal `
        -Description 'Changes the single-player Deep Desert layout once a week (tabr-tau layout-roll). Remove it with layout-roll-task.ps1 -Remove.' | Out-Null
    Write-Host "Registered the task '$TaskName' (Tuesday 05:00, user $user). Remove it with: layout-roll-task.ps1 -Remove"
}

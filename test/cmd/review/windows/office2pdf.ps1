<#
.SYNOPSIS
Exports each file of Microsoft Office in a folder to PDF, for the visual
review of ivy.

.DESCRIPTION
office2pdf.ps1 opens each file of the folder -Source with Word, Excel,
PowerPoint, Visio or Publisher, by its extension, through COM. It opens the
file read only, without a window, without prompts and with the macros off,
and exports it to <Out>\<relative path>.pdf. It writes one line for each
file to <Out>\export.tsv and a log to <Out>\export.log.

A file that fails, hangs or asks a question does not stop the run. Each
file has a time of -TimeoutSeconds. At the end of that time, a watchdog
kills the application, and the next file starts it again. After each
failure, the script closes the application and starts it again.

The script skips a file whose PDF file exists, unless -Force is given. It
exits with 0 when it finished, even if some files failed, and with 1 when
its parameters are wrong.

It runs in Windows PowerShell 5.1 and later, with Office installed. It
needs no console and asks nothing.

.PARAMETER Source
The folder of the files, such as a copy of test/review/cache/files or the
folder that "review bundle" writes.

.PARAMETER Out
The folder of the PDF files, export.tsv and export.log.

.PARAMETER Passwords
A TSV file of the passwords: on each line, the relative path of a file
with slashes, a tab and its password. "review bundle" writes it as
passwords.tsv.

.PARAMETER TimeoutSeconds
The time that one file can take. The default is 180.

.PARAMETER Force
Export each file again, even if its PDF file exists.

.EXAMPLE
powershell -NoProfile -ExecutionPolicy Bypass -File office2pdf.ps1 -Source C:\ivy\files -Out C:\ivy\out\msoffice -Passwords C:\ivy\files\passwords.tsv
#>
[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)] [string] $Source,
    [Parameter(Mandatory = $true)] [string] $Out,
    [string] $Passwords = '',
    [int] $TimeoutSeconds = 180,
    [switch] $Force
)

Set-StrictMode -Version 2
$ErrorActionPreference = 'Stop'

# The numeric constants of the object models of Office. Each one names its
# enumeration in the VBA reference of Microsoft.
$msoAutomationSecurityForceDisable = 3  # MsoAutomationSecurity: no macro runs, and no alert shows
$wdAlertsNone = 0                       # WdAlertLevel: no alert or message box
$wdFormatPDF = 17                       # WdSaveFormat
$wdRevisionsViewFinal = 0               # WdRevisionsView: the text with the revisions applied
$wdDoNotSaveChanges = 0                 # WdSaveOptions
$xlUpdateLinksNever = 0                 # UpdateLinks of Workbooks.Open: do not update the links
$xlTypePDF = 0                          # XlFixedFormatType
$ppAlertsNone = 1                       # PpAlertLevel
$msoTrue = -1                           # MsoTriState
$msoFalse = 0                           # MsoTriState
$ppSaveAsPDF = 32                       # PpSaveAsFileType
$visOpenRO = 0x2                        # VisOpenSaveArgs: read only
$visOpenDontList = 0x8                  # VisOpenSaveArgs: not in the recent files
$visOpenMacrosDisabled = 0x80           # VisOpenSaveArgs: macros off
$visOpenNoWorkspace = 0x100             # VisOpenSaveArgs: no workspace
$visFixedFormatPDF = 1                  # VisFixedFormatTypes
$visDocExIntentPrint = 1                # VisDocExIntent
$visPrintAll = 0                        # VisPrintOutRange
$IDNO = 7                               # AlertResponse of Visio: answer No to each alert
$pbDoNotSaveChanges = 3                 # PbSaveOptions
$pbFixedFormatTypePDF = 2               # PbFixedFormatType
$pbIntentPrinting = 3                   # PbFixedFormatIntent

# A password for a file without one: an application then fails on a file
# that has a password, and does not ask for it.
$noPassword = 'ivy-review-no-password'

# The applications, with the extensions of their files. Publisher has a
# single document interface, so it starts again for each file.
$Apps = @(
    @{ Name = 'Word'; ProgId = 'Word.Application'; Process = 'WINWORD'; PerFile = $false
       Exts = @('docx', 'dotx', 'docm', 'dotm', 'doc', 'dot', 'rtf', 'odt', 'ott') },
    @{ Name = 'Excel'; ProgId = 'Excel.Application'; Process = 'EXCEL'; PerFile = $false
       Exts = @('xlsx', 'xltx', 'xlsm', 'xltm', 'xls', 'xlt', 'ods', 'ots', 'csv') },
    @{ Name = 'PowerPoint'; ProgId = 'PowerPoint.Application'; Process = 'POWERPNT'; PerFile = $false
       Exts = @('pptx', 'potx', 'ppsx', 'pptm', 'ppsm', 'ppt', 'pps', 'pot', 'odp', 'otp') },
    @{ Name = 'Visio'; ProgId = 'Visio.InvisibleApp'; Process = 'VISIO'; PerFile = $false
       Exts = @('vsdx', 'vsdm', 'vstx', 'vstm', 'vsd', 'vst') },
    @{ Name = 'Publisher'; ProgId = 'Publisher.Application'; Process = 'MSPUB'; PerFile = $true
       Exts = @('pub') }
)

# The watchdog kills a process at the end of a time, from a thread of its
# own. A call of COM that waits for the killed application then fails.
Add-Type -TypeDefinition @'
using System;
using System.Diagnostics;
using System.Threading;

public sealed class IvyWatchdog : IDisposable {
    private readonly Timer timer;
    private int fired;

    public IvyWatchdog(int pid, int milliseconds) {
        timer = new Timer(Kill, pid, milliseconds, Timeout.Infinite);
    }

    public bool Fired { get { return Interlocked.CompareExchange(ref fired, 0, 0) != 0; } }

    private void Kill(object state) {
        Interlocked.Exchange(ref fired, 1);
        try { Process.GetProcessById((int)state).Kill(); } catch (Exception) { }
    }

    public void Dispose() { timer.Dispose(); }
}
'@

$utf8 = New-Object System.Text.UTF8Encoding($false)
$missing = [System.Reflection.Missing]::Value

function Write-Log([string] $text) {
    $line = '{0} {1}' -f (Get-Date -Format 'yyyy-MM-dd HH:mm:ss'), $text
    [System.IO.File]::AppendAllText($script:LogFile, $line + "`r`n", $utf8)
}

# Clean returns text for one column of a TSV file.
function Clean([string] $text) {
    if ($null -eq $text) { return '' }
    return ($text -replace "[`t`r`n]+", ' ').Trim()
}

function Write-Result([string] $file, [string] $app, [string] $version, [string] $result, [double] $seconds, [string] $err) {
    $cols = @((Clean $file), (Clean $app), (Clean $version), $result, ('{0:0.0}' -f $seconds), (Clean $err))
    [System.IO.File]::AppendAllText($script:ResultFile, ($cols -join "`t") + "`r`n", $utf8)
}

# Release releases a COM object, and ignores an object that is gone.
function Release($obj) {
    if ($null -ne $obj -and [System.Runtime.InteropServices.Marshal]::IsComObject($obj)) {
        try { [void][System.Runtime.InteropServices.Marshal]::FinalReleaseComObject($obj) } catch { }
    }
}

# Start-App starts the application app through COM, and returns its state:
# the COM object, the process and the version.
function Start-App($app) {
    $before = @(Get-Process -Name $app.Process -ErrorAction SilentlyContinue | ForEach-Object { $_.Id })
    $com = New-Object -ComObject $app.ProgId
    $proc = $null
    for ($i = 0; $i -lt 50 -and $null -eq $proc; $i++) {
        $proc = Get-Process -Name $app.Process -ErrorAction SilentlyContinue |
            Where-Object { $before -notcontains $_.Id } | Sort-Object StartTime -Descending | Select-Object -First 1
        if ($null -eq $proc) { Start-Sleep -Milliseconds 100 }
    }
    if ($null -eq $proc) {
        Release $com
        throw "the process $($app.Process) of $($app.Name) did not start"
    }
    $state = @{ App = $app; Com = $com; Pid = $proc.Id; Version = '' }
    $dog = [IvyWatchdog]::new([int]$proc.Id, [int](60000))
    try {
        try { $state.Version = '{0} build {1}' -f $com.Version, $com.Build } catch { $state.Version = '' + $com.Version }
        switch ($app.Name) {
            'Word' {
                $com.Visible = $false
                $com.DisplayAlerts = $wdAlertsNone
                $com.AutomationSecurity = $msoAutomationSecurityForceDisable
                $com.ScreenUpdating = $false
            }
            'Excel' {
                $com.Visible = $false
                $com.DisplayAlerts = $false
                $com.AutomationSecurity = $msoAutomationSecurityForceDisable
                $com.AskToUpdateLinks = $false
                $com.EnableEvents = $false
                $com.ScreenUpdating = $false
            }
            'PowerPoint' {
                # PowerPoint refuses Visible = false, so each presentation
                # opens without a window
                $com.DisplayAlerts = $ppAlertsNone
                $com.AutomationSecurity = $msoAutomationSecurityForceDisable
            }
            'Visio' {
                $com.AlertResponse = $IDNO
                $com.EventsEnabled = 0
                try { $com.AutomationSecurity = $msoAutomationSecurityForceDisable } catch { }
            }
            'Publisher' {
                try { $com.AutomationSecurity = $msoAutomationSecurityForceDisable } catch { }
            }
        }
    } finally {
        $dog.Dispose()
    }
    Write-Log "started $($app.Name) $($state.Version), process $($state.Pid)"
    return $state
}

# Stop-App closes the application of state, releases its COM object, and
# kills its process if it still runs.
function Stop-App($state) {
    if ($null -eq $state) { return }
    $dog = [IvyWatchdog]::new([int]$state.Pid, [int](30000))
    try { $state.Com.Quit() } catch { } finally { $dog.Dispose() }
    Release $state.Com
    $state.Com = $null
    [System.GC]::Collect()
    [System.GC]::WaitForPendingFinalizers()
    $proc = Get-Process -Id $state.Pid -ErrorAction SilentlyContinue
    if ($null -ne $proc -and -not $proc.WaitForExit(10000)) {
        try { $proc.Kill() } catch { }
    }
    Write-Log "stopped $($state.App.Name), process $($state.Pid)"
}

# Export-File opens the file src with the application of state, and
# exports it to the PDF file dst. It throws an error when it fails.
function Export-File($state, [string] $src, [string] $dst, [string] $password) {
    $com = $state.Com
    $doc = $null
    $pw = $password
    if ($pw -eq '') { $pw = $noPassword }
    try {
        switch ($state.App.Name) {
            'Word' {
                # FileName, ConfirmConversions, ReadOnly, AddToRecentFiles,
                # PasswordDocument, PasswordTemplate. Word takes each
                # parameter by reference, and the binder of PowerShell
                # fails on some of the later ones, such as Revert and
                # Visible, so the call stops after the passwords.
                $doc = $com.Documents.Open($src, $false, $true, $false, $pw, $pw)
                # The PDF shows the document without the markup of the
                # revisions and the comments, as ExportAsFixedFormat does
                # with wdExportDocumentContent.
                try {
                    $doc.ActiveWindow.View.RevisionsView = $wdRevisionsViewFinal
                    $doc.ActiveWindow.View.ShowRevisionsAndComments = $false
                } catch { }
                # ExportAsFixedFormat fails or hangs in the desktop of the
                # scheduled task, and SaveAs2 does not. FileName,
                # FileFormat.
                $doc.SaveAs2([ref]$dst, [ref]$wdFormatPDF)
                $doc.Close([ref]$wdDoNotSaveChanges)
            }
            'Excel' {
                # FileName, UpdateLinks, ReadOnly, Format, Password. Excel
                # refuses to open a file when WriteResPassword or a later
                # parameter is given.
                $doc = $com.Workbooks.Open($src, $xlUpdateLinksNever, $true, $missing, $pw)
                # Type, FileName. The method of the workbook exports each
                # sheet.
                $doc.ExportAsFixedFormat($xlTypePDF, $dst)
                $doc.Close($false)
            }
            'PowerPoint' {
                # PowerPoint takes the password in the name of the file, as
                # "name::password::"
                $name = $src
                if ($password -ne '') { $name = '{0}::{1}::' -f $src, $password }
                # FileName, ReadOnly, Untitled, WithWindow
                $doc = $com.Presentations.Open($name, $msoTrue, $msoFalse, $msoFalse)
                $doc.SaveAs($dst, $ppSaveAsPDF)
                $doc.Close()
            }
            'Visio' {
                $flags = $visOpenRO + $visOpenDontList + $visOpenMacrosDisabled + $visOpenNoWorkspace
                $doc = $com.Documents.OpenEx($src, $flags)
                # FixedFormat, OutputFileName, Intent, PrintRange, FromPage,
                # ToPage, ColorAsBlack, IncludeBackground,
                # IncludeDocumentProperties, IncludeStructureTags,
                # UseISO19005_1
                $doc.ExportAsFixedFormat($visFixedFormatPDF, $dst, $visDocExIntentPrint, $visPrintAll,
                    1, -1, $false, $true, $true, $true, $false)
                $doc.Saved = $true
                $doc.Close()
            }
            'Publisher' {
                # FileName, ReadOnly, AddToRecentFiles, SaveChanges
                $doc = $com.Open($src, $true, $false, $pbDoNotSaveChanges)
                # Format, FileName, Intent, IncludeDocumentProperties
                $doc.ExportAsFixedFormat($pbFixedFormatTypePDF, $dst, $pbIntentPrinting, $true)
                $doc.Close()
            }
        }
    } finally {
        Release $doc
    }
}

# Check the parameters.
if (-not (Test-Path -LiteralPath $Source -PathType Container)) {
    [Console]::Error.WriteLine("the folder -Source $Source does not exist")
    exit 1
}
$Source = (Resolve-Path -LiteralPath $Source).ProviderPath.TrimEnd('\')
New-Item -ItemType Directory -Force -Path $Out | Out-Null
$Out = (Resolve-Path -LiteralPath $Out).ProviderPath.TrimEnd('\')
$script:LogFile = Join-Path $Out 'export.log'
$script:ResultFile = Join-Path $Out 'export.tsv'
if (-not (Test-Path -LiteralPath $script:ResultFile)) {
    [System.IO.File]::WriteAllText($script:ResultFile, "file`tapplication`tversion`tresult`tseconds`terror`r`n", $utf8)
}
Write-Log "office2pdf: $Source -> $Out, timeout $TimeoutSeconds s, force $Force"

# Read the passwords.
$pwMap = @{}
if ($Passwords -ne '') {
    foreach ($line in [System.IO.File]::ReadAllLines($Passwords, $utf8)) {
        $parts = $line.Split("`t")
        if ($parts.Count -ge 2 -and $parts[0] -ne '') { $pwMap[$parts[0]] = $parts[1] }
    }
    Write-Log "read $($pwMap.Count) passwords"
}

# The extension of each application.
$appOf = @{}
foreach ($app in $Apps) {
    foreach ($ext in $app.Exts) { $appOf[$ext] = $app }
}

$files = @(Get-ChildItem -LiteralPath $Source -Recurse -File | Where-Object {
    $appOf.ContainsKey($_.Extension.TrimStart('.').ToLowerInvariant())
} | Sort-Object FullName)
Write-Log "$($files.Count) files"

# The running applications, by name, and the applications that cannot start.
$running = @{}
$broken = @{}

foreach ($file in $files) {
    $rel = $file.FullName.Substring($Source.Length).TrimStart('\')
    $name = $rel.Replace('\', '/')
    $app = $appOf[$file.Extension.TrimStart('.').ToLowerInvariant()]
    $dst = Join-Path $Out ($rel + '.pdf')
    if ((Test-Path -LiteralPath $dst) -and -not $Force) {
        Write-Result $name $app.Name '' 'skipped' 0 'the PDF file exists'
        continue
    }
    if ($broken.ContainsKey($app.Name)) {
        Write-Result $name $app.Name '' 'error' 0 $broken[$app.Name]
        continue
    }
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $dst) | Out-Null
    $tmp = $dst + '.tmp.pdf'
    Remove-Item -LiteralPath $tmp -Force -ErrorAction SilentlyContinue
    $password = ''
    if ($pwMap.ContainsKey($name)) { $password = $pwMap[$name] }
    $start = Get-Date
    $state = $running[$app.Name]
    if ($null -eq $state) {
        try {
            $state = Start-App $app
            $running[$app.Name] = $state
        } catch {
            $msg = "$($app.Name) did not start: $($_.Exception.Message)"
            Write-Log $msg
            # an application that is not installed fails each time
            if ($_.Exception.Message -match '80040154|Class not registered') { $broken[$app.Name] = $msg }
            Write-Result $name $app.Name '' 'error' 0 $msg
            continue
        }
    }
    $dog = [IvyWatchdog]::new([int]$state.Pid, [int]($TimeoutSeconds * 1000))
    $result = 'ok'
    $err = ''
    try {
        Export-File $state $file.FullName $tmp $password
        if (-not (Test-Path -LiteralPath $tmp)) { throw "$($app.Name) wrote no PDF file" }
        Move-Item -LiteralPath $tmp -Destination $dst -Force
    } catch {
        $result = 'error'
        $err = $_.Exception.Message
        if ($dog.Fired) {
            $result = 'timeout'
            $err = "the file took more than $TimeoutSeconds s: $err"
        }
    } finally {
        $dog.Dispose()
    }
    $seconds = ((Get-Date) - $start).TotalSeconds
    Write-Result $name $app.Name $state.Version $result $seconds $err
    Write-Log ('{0} {1} {2:0.0} s {3}' -f $result, $name, $seconds, $err)
    if ($result -ne 'ok') {
        Remove-Item -LiteralPath $tmp -Force -ErrorAction SilentlyContinue
    }
    if ($result -ne 'ok' -or $app.PerFile) {
        # the application can be in a bad state after a failure
        Stop-App $state
        $running.Remove($app.Name)
    }
}

foreach ($state in @($running.Values)) { Stop-App $state }
Write-Log 'office2pdf: done'
exit 0

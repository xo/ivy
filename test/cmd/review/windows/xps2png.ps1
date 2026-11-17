<#
.SYNOPSIS
Draws each page of each XPS file of a folder to a PNG image with WPF, for
the visual review of ivy.

.DESCRIPTION
xps2png.ps1 opens each .xps and .oxps file of the folder -Source with the
XPS classes of WPF, the engine of the XPS viewer of Windows, and draws each
page to <Out>\<relative path>.p<N>.png, with N from 1. It writes one line
for each file to <Out>\xps2png.tsv and a log to <Out>\xps2png.log.

For each page, the script takes the page of the DocumentPaginator of the
FixedDocumentSequence of the XpsDocument. It fills the page with white,
draws a VisualBrush of the visual of the page over the size of the page,
and renders that with RenderTargetBitmap at -Dpi dots per inch. A side of
the image is the side of the page in pixels of 1/96 inch, times -Dpi / 96,
rounded up. This is the size of the page of ivy.

The XpsDocument of WPF reads XPS, and not OpenXPS (ECMA-388), the content
of most .oxps files. The line of such a file says why WPF did not open it.

Each file is drawn by a child process of PowerShell in a single-threaded
apartment, so a file that hangs WPF stops only its child. The child has a
time of -TimeoutSeconds. The script needs no desktop, and it can run as
SYSTEM. It skips a file whose first image exists, unless -Force is given.
It exits with 0 when it finished, even if some files failed, and with 1
when its parameters are wrong. It needs no console and asks nothing.

.PARAMETER Source
The folder of the files, such as a copy of test/review/cache/files.

.PARAMETER Out
The folder of the PNG files, xps2png.tsv and xps2png.log.

.PARAMETER Dpi
The resolution in dots per inch. The default is 96, the default of the
review.

.PARAMETER TimeoutSeconds
The time that one file can take. The default is 180.

.PARAMETER Force
Draw each file again, even if its images exist.

.PARAMETER One
Used by the script itself: draw only this file to the images of the base
name -OneOut, and write the result to -OneResult.

.EXAMPLE
powershell -Sta -NoProfile -ExecutionPolicy Bypass -File xps2png.ps1 -Source C:\ivy\files -Out C:\ivy\out\xps
#>
[CmdletBinding()]
param(
    [string] $Source = '',
    [string] $Out = '',
    [int] $Dpi = 96,
    [int] $TimeoutSeconds = 180,
    [switch] $Force,
    [string] $One = '',
    [string] $OneOut = '',
    [string] $OneResult = ''
)

Set-StrictMode -Version 2
$ErrorActionPreference = 'Stop'
$utf8 = New-Object System.Text.UTF8Encoding($false)

function Clean([string] $text) {
    if ($null -eq $text) { return '' }
    return ($text -replace "[`t`r`n]+", ' ').Trim()
}

# Draw-Xps draws each page of the XPS file src to the images base.p<N>.png.tmp,
# and returns the number of the pages. It throws an error when it fails,
# with the number of the pages that it drew in $script:Drawn.
function Draw-Xps([string] $src, [string] $base) {
    Add-Type -AssemblyName PresentationCore, PresentationFramework, ReachFramework, WindowsBase
    $script:Drawn = 0
    $xps = New-Object System.Windows.Xps.Packaging.XpsDocument($src, [System.IO.FileAccess]::Read)
    try {
        $seq = $xps.GetFixedDocumentSequence()
        if ($null -eq $seq) { throw 'WPF found no fixed document sequence in the file, as in an OpenXPS file' }
        $pag = $seq.DocumentPaginator
        $pag.ComputePageCount()
        $count = $pag.PageCount
        for ($i = 0; $i -lt $count; $i++) {
            $page = $pag.GetPage($i)
            try {
                $size = $page.Size
                $w = [int][Math]::Ceiling($size.Width * $Dpi / 96.0 - 1e-6)
                $h = [int][Math]::Ceiling($size.Height * $Dpi / 96.0 - 1e-6)
                if ($w -lt 1 -or $h -lt 1 -or $w * [long]$h -gt 268435456) { throw "page $($i + 1) of $w by $h pixels" }
                $rect = New-Object System.Windows.Rect(0, 0, $size.Width, $size.Height)
                # the brush maps the page, in its own units, onto the page
                $brush = New-Object System.Windows.Media.VisualBrush($page.Visual)
                $brush.ViewboxUnits = [System.Windows.Media.BrushMappingMode]::Absolute
                $brush.Viewbox = $rect
                $brush.Stretch = [System.Windows.Media.Stretch]::Fill
                $visual = New-Object System.Windows.Media.DrawingVisual
                $dc = $visual.RenderOpen()
                try {
                    $dc.DrawRectangle([System.Windows.Media.Brushes]::White, $null, $rect)
                    $dc.DrawRectangle($brush, $null, $rect)
                } finally {
                    $dc.Close()
                }
                $bmp = New-Object System.Windows.Media.Imaging.RenderTargetBitmap($w, $h, $Dpi, $Dpi,
                    [System.Windows.Media.PixelFormats]::Pbgra32)
                $bmp.Render($visual)
                $enc = New-Object System.Windows.Media.Imaging.PngBitmapEncoder
                $enc.Frames.Add([System.Windows.Media.Imaging.BitmapFrame]::Create($bmp))
                $fs = [System.IO.File]::Create(('{0}.p{1}.png.tmp' -f $base, ($i + 1)))
                try { $enc.Save($fs) } finally { $fs.Dispose() }
                $script:Drawn = $i + 1
            } finally {
                $page.Dispose()
            }
        }
        return $count
    } finally {
        $xps.Close()
    }
}

# The child: draw one file, and write its result: ok or error, the number
# of the pages, and the error.
if ($One -ne '') {
    try {
        $n = Draw-Xps $One $OneOut
        $line = "ok`t$n`t"
    } catch {
        $line = "error`t$($script:Drawn)`t" + (Clean $_.Exception.Message)
    }
    [System.IO.File]::WriteAllText($OneResult, $line, $utf8)
    exit 0
}

if ($Source -eq '' -or $Out -eq '' -or -not (Test-Path -LiteralPath $Source -PathType Container)) {
    [Console]::Error.WriteLine('expected an existing folder -Source and a folder -Out')
    exit 1
}
$Source = (Resolve-Path -LiteralPath $Source).ProviderPath.TrimEnd('\')
New-Item -ItemType Directory -Force -Path $Out | Out-Null
$Out = (Resolve-Path -LiteralPath $Out).ProviderPath.TrimEnd('\')
$logFile = Join-Path $Out 'xps2png.log'
$resultFile = Join-Path $Out 'xps2png.tsv'
if (-not (Test-Path -LiteralPath $resultFile)) {
    [System.IO.File]::WriteAllText($resultFile, "file`tpages`tresult`tseconds`terror`r`n", $utf8)
}

function Write-Log([string] $text) {
    $line = '{0} {1}' -f (Get-Date -Format 'yyyy-MM-dd HH:mm:ss'), $text
    [System.IO.File]::AppendAllText($logFile, $line + "`r`n", $utf8)
}

# Remove-Pages removes the images of the base name base whose names end
# with suffix.
function Remove-Pages([string] $base, [string] $suffix) {
    $dir = Split-Path -Parent $base
    $leaf = Split-Path -Leaf $base
    Get-ChildItem -LiteralPath $dir -File -ErrorAction SilentlyContinue | Where-Object {
        $_.Name.StartsWith($leaf + '.p') -and $_.Name.EndsWith($suffix) -and
            $_.Name.Length -gt ($leaf.Length + 2 + $suffix.Length) -and
            $_.Name.Substring($leaf.Length + 2, $_.Name.Length - $leaf.Length - 2 - $suffix.Length) -match '^[0-9]+$'
    } | Remove-Item -Force -ErrorAction SilentlyContinue
}

Write-Log "xps2png: $Source -> $Out, $Dpi dpi, $([System.Environment]::OSVersion.VersionString)"
$files = @(Get-ChildItem -LiteralPath $Source -Recurse -File | Where-Object {
    @('.xps', '.oxps') -contains $_.Extension.ToLowerInvariant()
} | Sort-Object FullName)
Write-Log "$($files.Count) files"
$self = $MyInvocation.MyCommand.Path
$powershell = Join-Path $PSHOME 'powershell.exe'

foreach ($file in $files) {
    $rel = $file.FullName.Substring($Source.Length).TrimStart('\')
    $name = $rel.Replace('\', '/')
    $base = Join-Path $Out $rel
    $start = Get-Date
    $pages = ''
    $err = ''
    if ((Test-Path -LiteralPath ($base + '.p1.png')) -and -not $Force) {
        $result = 'skipped'
        $err = 'the images exist'
    } else {
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $base) | Out-Null
        Remove-Pages $base '.png.tmp'
        $res = $base + '.result'
        Remove-Item -LiteralPath $res -Force -ErrorAction SilentlyContinue
        $childArgs = @('-Sta', '-NoProfile', '-NonInteractive', '-ExecutionPolicy', 'Bypass', '-File', ('"{0}"' -f $self),
            '-Dpi', $Dpi, '-One', ('"{0}"' -f $file.FullName), '-OneOut', ('"{0}"' -f $base), '-OneResult', ('"{0}"' -f $res))
        $child = Start-Process -FilePath $powershell -ArgumentList $childArgs -WindowStyle Hidden -PassThru
        if (-not $child.WaitForExit($TimeoutSeconds * 1000)) {
            try { $child.Kill() } catch { }
            $result = 'timeout'
            $err = "the file took more than $TimeoutSeconds s"
        } elseif (Test-Path -LiteralPath $res) {
            $f = [System.IO.File]::ReadAllText($res, $utf8).Split("`t")
            $result = $f[0]
            if ($f.Count -ge 2) { $pages = $f[1] }
            if ($f.Count -ge 3) { $err = $f[2] }
        } else {
            $result = 'error'
            $err = 'the child process wrote no result'
        }
        Remove-Item -LiteralPath $res -Force -ErrorAction SilentlyContinue
        if ($result -eq 'ok') {
            # the images of an earlier run go, and the new ones take their names
            Remove-Pages $base '.png'
            for ($i = 1; $i -le [int]$pages; $i++) {
                Move-Item -LiteralPath ('{0}.p{1}.png.tmp' -f $base, $i) -Destination ('{0}.p{1}.png' -f $base, $i) -Force
            }
        } else {
            Remove-Pages $base '.png.tmp'
        }
    }
    $seconds = '{0:0.0}' -f ((Get-Date) - $start).TotalSeconds
    $cols = @((Clean $name), $pages, $result, $seconds, (Clean $err))
    [System.IO.File]::AppendAllText($resultFile, ($cols -join "`t") + "`r`n", $utf8)
    Write-Log "$result $name $pages pages $seconds s $err"
}

Write-Log 'xps2png: done'
exit 0

<#
.SYNOPSIS
Draws each metafile of a folder to a PNG image with GDI+, for the visual
review of ivy.

.DESCRIPTION
metafile2png.ps1 draws each .emf, .wmf, .emz and .wmz file of the folder
-Source with GDI+ (System.Drawing.Imaging.Metafile and Graphics.DrawImage)
onto a white bitmap, and writes <Out>\<relative path>.png. It writes one
line for each file to <Out>\metafile2png.tsv, with the size of the image
and the frame that gave it, and a log to <Out>\metafile2png.log. An .emz or
a .wmz file is a metafile in gzip, and the script unpacks it first.

The size of the image is the size of the page that the package emf of ivy
gives the file, at -Dpi dots per inch, rounded up:

- An EMF file takes its frame, rclFrame of its header, in hundredths of a
  millimeter.
- A WMF file with a placeable header takes its bounds and its units for
  each inch.
- Another file takes the bounds and the resolution that GDI+ reads from its
  header, and its line says "gdiplus".

The review uses the images only at the same resolution, 96 dots per inch
by default.

Each file is drawn by a child process of PowerShell, so a file that hangs
GDI+ stops only its child. The child has a time of -TimeoutSeconds. The
script skips a file whose PNG file exists, unless -Force is given. It exits
with 0 when it finished, even if some files failed, and with 1 when its
parameters are wrong. It needs no console and asks nothing.

.PARAMETER Source
The folder of the files, such as a copy of test/review/cache/files.

.PARAMETER Out
The folder of the PNG files, metafile2png.tsv and metafile2png.log.

.PARAMETER Dpi
The resolution in dots per inch. The default is 96, the default of the
review.

.PARAMETER TimeoutSeconds
The time that one file can take. The default is 60.

.PARAMETER Force
Draw each file again, even if its PNG file exists.

.PARAMETER One
Used by the script itself: draw only this file to -OneOut, and write the
result to -OneResult.

.EXAMPLE
powershell -NoProfile -ExecutionPolicy Bypass -File metafile2png.ps1 -Source C:\ivy\files -Out C:\ivy\out\gdiplus
#>
[CmdletBinding()]
param(
    [string] $Source = '',
    [string] $Out = '',
    [int] $Dpi = 96,
    [int] $TimeoutSeconds = 60,
    [switch] $Force,
    [string] $One = '',
    [string] $OneOut = '',
    [string] $OneResult = ''
)

Set-StrictMode -Version 2
$ErrorActionPreference = 'Stop'
$utf8 = New-Object System.Text.UTF8Encoding($false)

# Get-Frame returns the size of the page of the metafile in the bytes data,
# in inches, and the frame that gave it, or $null.
function Get-Frame([byte[]] $data) {
    if ($data.Length -ge 44 -and [BitConverter]::ToUInt32($data, 0) -eq 1 -and [BitConverter]::ToUInt32($data, 40) -eq 0x464D4520) {
        # EMR_HEADER: rclFrame at 24, in hundredths of a millimeter, and the
        # signature " EMF" at 40
        $w = [BitConverter]::ToInt32($data, 32) - [BitConverter]::ToInt32($data, 24)
        $h = [BitConverter]::ToInt32($data, 36) - [BitConverter]::ToInt32($data, 28)
        if ($w -ne 0 -and $h -ne 0) {
            return @{ Width = [Math]::Abs($w) / 2540.0; Height = [Math]::Abs($h) / 2540.0; Frame = 'emf-frame' }
        }
        return $null
    }
    if ($data.Length -ge 22 -and [BitConverter]::ToUInt32($data, 0) -eq 0x9AC6CDD7) {
        # the placeable header of WMF: the bounds at 6, as left, top, right
        # and bottom, and the units for each inch at 14
        $w = [BitConverter]::ToInt16($data, 10) - [BitConverter]::ToInt16($data, 6)
        $h = [BitConverter]::ToInt16($data, 12) - [BitConverter]::ToInt16($data, 8)
        $inch = [BitConverter]::ToUInt16($data, 14)
        if ($w -ne 0 -and $h -ne 0 -and $inch -ne 0) {
            return @{ Width = [Math]::Abs($w) / [double]$inch; Height = [Math]::Abs($h) / [double]$inch; Frame = 'wmf-placeable' }
        }
    }
    return $null
}

# Draw-One draws the metafile src to the PNG file dst, and returns the
# result line: width, height, dpi and frame.
function Draw-One([string] $src, [string] $dst) {
    Add-Type -AssemblyName System.Drawing
    $data = [System.IO.File]::ReadAllBytes($src)
    $ext = [System.IO.Path]::GetExtension($src).ToLowerInvariant()
    $file = $src
    $tmp = $null
    if ($ext -eq '.emz' -or $ext -eq '.wmz') {
        $in = New-Object System.IO.MemoryStream(, $data)
        $gz = New-Object System.IO.Compression.GZipStream($in, [System.IO.Compression.CompressionMode]::Decompress)
        $plain = New-Object System.IO.MemoryStream
        try { $gz.CopyTo($plain) } finally { $gz.Dispose() }
        $data = $plain.ToArray()
        $tmp = [System.IO.Path]::Combine([System.IO.Path]::GetTempPath(), [System.Guid]::NewGuid().ToString() + ($ext -replace 'z$', 'f'))
        [System.IO.File]::WriteAllBytes($tmp, $data)
        $file = $tmp
    }
    $mf = $null
    $bmp = $null
    $g = $null
    try {
        $mf = New-Object System.Drawing.Imaging.Metafile($file)
        $frame = Get-Frame $data
        if ($null -eq $frame) {
            $head = $mf.GetMetafileHeader()
            $frame = @{ Width = $head.Bounds.Width / [double]$head.DpiX; Height = $head.Bounds.Height / [double]$head.DpiY; Frame = 'gdiplus' }
        }
        $w = [int][Math]::Ceiling($frame.Width * $Dpi - 1e-6)
        $h = [int][Math]::Ceiling($frame.Height * $Dpi - 1e-6)
        if ($w -lt 1 -or $h -lt 1 -or $w * [long]$h -gt 268435456) { throw "a page of $w by $h pixels" }
        $bmp = New-Object System.Drawing.Bitmap($w, $h, [System.Drawing.Imaging.PixelFormat]::Format24bppRgb)
        $bmp.SetResolution($Dpi, $Dpi)
        $g = [System.Drawing.Graphics]::FromImage($bmp)
        $g.Clear([System.Drawing.Color]::White)
        $g.SmoothingMode = [System.Drawing.Drawing2D.SmoothingMode]::AntiAlias
        $g.TextRenderingHint = [System.Drawing.Text.TextRenderingHint]::AntiAlias
        $g.InterpolationMode = [System.Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic
        # DrawImage maps the frame of the metafile onto the rectangle
        $g.DrawImage($mf, (New-Object System.Drawing.Rectangle(0, 0, $w, $h)))
        $bmp.Save($dst, [System.Drawing.Imaging.ImageFormat]::Png)
        return ("{0}`t{1}`t{2}`t{3}" -f $w, $h, $Dpi, $frame.Frame)
    } finally {
        if ($null -ne $g) { $g.Dispose() }
        if ($null -ne $bmp) { $bmp.Dispose() }
        if ($null -ne $mf) { $mf.Dispose() }
        if ($null -ne $tmp) { Remove-Item -LiteralPath $tmp -Force -ErrorAction SilentlyContinue }
    }
}

# The child: draw one file, and write its result.
if ($One -ne '') {
    try {
        $line = "ok`t" + (Draw-One $One $OneOut)
    } catch {
        $line = "error`t`t`t`t`t" + (($_.Exception.Message -replace "[`t`r`n]+", ' ').Trim())
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
$logFile = Join-Path $Out 'metafile2png.log'
$resultFile = Join-Path $Out 'metafile2png.tsv'
if (-not (Test-Path -LiteralPath $resultFile)) {
    [System.IO.File]::WriteAllText($resultFile, "file`tresult`twidth`theight`tdpi`tframe`tseconds`terror`r`n", $utf8)
}

function Write-Log([string] $text) {
    $line = '{0} {1}' -f (Get-Date -Format 'yyyy-MM-dd HH:mm:ss'), $text
    [System.IO.File]::AppendAllText($logFile, $line + "`r`n", $utf8)
}

Write-Log "metafile2png: $Source -> $Out, $Dpi dpi, $([System.Environment]::OSVersion.VersionString)"
$files = @(Get-ChildItem -LiteralPath $Source -Recurse -File | Where-Object {
    @('.emf', '.wmf', '.emz', '.wmz') -contains $_.Extension.ToLowerInvariant()
} | Sort-Object FullName)
Write-Log "$($files.Count) files"
$self = $MyInvocation.MyCommand.Path
$powershell = Join-Path $PSHOME 'powershell.exe'

foreach ($file in $files) {
    $rel = $file.FullName.Substring($Source.Length).TrimStart('\')
    $name = $rel.Replace('\', '/')
    $dst = Join-Path $Out ($rel + '.png')
    $start = Get-Date
    if ((Test-Path -LiteralPath $dst) -and -not $Force) {
        $line = "skipped`t`t`t`t`t0`tthe PNG file exists"
    } else {
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $dst) | Out-Null
        $tmp = $dst + '.tmp.png'
        $res = $dst + '.result'
        Remove-Item -LiteralPath $tmp, $res -Force -ErrorAction SilentlyContinue
        $childArgs = @('-NoProfile', '-NonInteractive', '-ExecutionPolicy', 'Bypass', '-File', ('"{0}"' -f $self),
            '-Dpi', $Dpi, '-One', ('"{0}"' -f $file.FullName), '-OneOut', ('"{0}"' -f $tmp), '-OneResult', ('"{0}"' -f $res))
        $child = Start-Process -FilePath $powershell -ArgumentList $childArgs -WindowStyle Hidden -PassThru
        if (-not $child.WaitForExit($TimeoutSeconds * 1000)) {
            try { $child.Kill() } catch { }
            $result = "timeout`t`t`t`t`tthe file took more than $TimeoutSeconds s"
        } elseif (Test-Path -LiteralPath $res) {
            $result = [System.IO.File]::ReadAllText($res, $utf8)
        } else {
            $result = "error`t`t`t`t`tthe child process wrote no result"
        }
        $f = $result.Split("`t")
        if ($f[0] -eq 'ok' -and (Test-Path -LiteralPath $tmp)) {
            Move-Item -LiteralPath $tmp -Destination $dst -Force
        } else {
            Remove-Item -LiteralPath $tmp -Force -ErrorAction SilentlyContinue
        }
        Remove-Item -LiteralPath $res -Force -ErrorAction SilentlyContinue
        $seconds = '{0:0.0}' -f ((Get-Date) - $start).TotalSeconds
        # result, width, height, dpi, frame, seconds, error
        $err = ''
        if ($f.Count -ge 6) { $err = $f[5] }
        $line = @($f[0], $f[1], $f[2], $f[3], $f[4], $seconds, $err) -join "`t"
    }
    [System.IO.File]::AppendAllText($resultFile, $name + "`t" + $line + "`r`n", $utf8)
    Write-Log "$name $($line.Replace("`t", ' '))"
}

Write-Log 'metafile2png: done'
exit 0

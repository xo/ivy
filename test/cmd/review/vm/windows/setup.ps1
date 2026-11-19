# setup.ps1 prepares the Windows VM of the review. Run it as an
# administrator, for example through `incus exec`, which runs as SYSTEM:
#
#   powershell -NoProfile -ExecutionPolicy Bypass -File C:\ivy\setup.ps1
#
# It sets Office for the user `user` so that the export runs without a
# question: the license agreements, the first run, Protected View and the
# blocks of old file formats. It also makes Excel keep the paper of a sheet,
# and makes "Microsoft Print to PDF" the default printer of the user. The maintainer installs and activates Office
# by hand, in the desktop of the VM, before the script runs. If Office is
# not installed, the script stops.
#
# With -Install, the script first installs Office with the Office Deployment
# Tool and office.xml, which must be next to this script. It does not
# activate Office.

param([switch]$Install)

$ErrorActionPreference = 'Stop'
$here = Split-Path -Parent $MyInvocation.MyCommand.Path

$word = 'C:\Program Files\Microsoft Office\root\Office16\WINWORD.EXE'
if (-not (Test-Path $word) -and -not $Install) {
	throw "Office is not installed: there is no $word. Install Office in the desktop of the VM, or run the script with -Install"
}
if (-not (Test-Path $word)) {
	$work = 'C:\ivy\odt'
	New-Item -ItemType Directory -Force -Path $work | Out-Null
	$odt = Join-Path $work 'setup.exe'
	if (-not (Test-Path $odt)) {
		Write-Output 'downloading the Office Deployment Tool'
		Invoke-WebRequest -UseBasicParsing -Uri 'https://officecdn.microsoft.com/pr/wsus/setup.exe' -OutFile $odt
	}
	Write-Output 'installing Office, which takes a while'
	$p = Start-Process -FilePath $odt -ArgumentList @('/configure', (Join-Path $here 'office.xml')) -WorkingDirectory $work -Wait -PassThru
	if ($p.ExitCode -ne 0) {
		throw "the Office Deployment Tool exited with $($p.ExitCode)"
	}
}
Write-Output ('Office: ' + (Get-Item $word).VersionInfo.ProductVersion)

# The settings of all users, in HKLM.
$common = 'HKLM:\SOFTWARE\Microsoft\Office\16.0\Registration'
if (-not (Test-Path $common)) {
	New-Item -Force -Path $common | Out-Null
}
Set-ItemProperty -Path $common -Name AcceptAllEulas -Value 1 -Type DWord

# The settings of the user `user`, in the hive of the user. The hive is
# loaded while the user is logged off, and written through HKEY_USERS of
# the logged on user otherwise.
$sid = (New-Object System.Security.Principal.NTAccount('user')).Translate([System.Security.Principal.SecurityIdentifier]).Value
$root = "Registry::HKEY_USERS\$sid"
$loaded = $false
if (-not (Test-Path $root)) {
	& reg.exe load "HKU\$sid" 'C:\Users\user\NTUSER.DAT' | Out-Null
	$loaded = $true
}
function Set-UserValue($path, $name, $value, $type = 'DWord') {
	$key = "$root\$path"
	# New-Item -Force replaces a key that exists, with its values, so it
	# runs only for a key that is missing.
	if (-not (Test-Path $key)) {
		New-Item -Force -Path $key | Out-Null
	}
	Set-ItemProperty -Path $key -Name $name -Value $value -Type $type
}
$o = 'Software\Microsoft\Office\16.0'
Set-UserValue "$o\Common\General" ShownFirstRunOptin 1
Set-UserValue "$o\Common\General" DisableBootToOfficeStart 1
Set-UserValue "$o\Common\Privacy" DisconnectedState 2
Set-UserValue "$o\Common\Privacy" UserContentDisabled 2
Set-UserValue "$o\Common\Privacy" DownloadContentDisabled 2
Set-UserValue "$o\Common\Privacy" ControllerConnectedServicesEnabled 2
Set-UserValue "$o\Registration" AcceptAllEulas 1
foreach ($app in 'Word', 'Excel', 'PowerPoint', 'Visio') {
	# Protected View opens a file read only, in a view without automation.
	Set-UserValue "$o\$app\Security\ProtectedView" DisableInternetFilesInPV 1
	Set-UserValue "$o\$app\Security\ProtectedView" DisableAttachementsInPV 1
	Set-UserValue "$o\$app\Security\ProtectedView" DisableUnsafeLocationsInPV 1
	# Macros stay off. The export also sets AutomationSecurity.
	Set-UserValue "$o\$app\Security" VBAWarnings 4
}
# File Block: 0 lets Office open the old formats, such as Word 95 and
# PowerPoint 95, and the files of the corpus in them. The values go in the
# key of the policies, because Word replaces the values of its own key
# Security\FileBlock when it starts.
$blocks = @{
	'Word'       = 'Word2Files', 'Word60Files', 'Word95Files', 'Word97Files', 'Word2000Files', 'Word2003Files', 'Word2007Files', 'RtfFiles', 'OpenDocumentTextFiles'
	'Excel'      = 'XL2Worksheets', 'XL3Worksheets', 'XL4Worksheets', 'XL4Workbooks', 'XL95Workbooks', 'XL9597WorkbooksandTemplates', 'XL97WorkbooksandTemplates', 'XL2000Files', 'OpenDocumentSpreadsheetFiles', 'DBaseFiles', 'SYLKFiles', 'DifandSylkFiles'
	'PowerPoint' = 'PowerPoint95Files', 'PowerPoint97Files', 'PowerPoint2000Files', 'OpenDocumentPresentationFiles'
}
foreach ($app in $blocks.Keys) {
	foreach ($name in $blocks[$app]) {
		Set-UserValue "Software\Policies\Microsoft\Office\16.0\$app\Security\FileBlock" $name 0
	}
}
# A4Letter 0 turns off the option of Excel "Scale content for A4 or 8.5 x 11"
# paper sizes", Application.MapPaperSize. With the option on, Excel printed
# and exported a sheet with an A4 page setup on US Letter, the paper of the
# region of the user, and shrank the sheet to fit it.
Set-UserValue "$o\Excel\Options" A4Letter 0
# Excel lays out a page with the driver of the default printer of the user,
# also when it exports a PDF file. The first default printer of the VM is
# "OneNote (Desktop)", a printer that sends the pages to OneNote. "Microsoft
# Print to PDF" is the default printer instead. On 2026-11-19 the
# two printers gave the same pages of Excel, within 0.03 pt. The export
# cannot set Excel.ActivePrinter: it failed with 0x800A03EC from the
# scheduled task. Device holds the name of the printer, a comma and its value
# in the key Devices of the user, such as "winspool,Ne00:".
# LegacyDefaultPrinterMode 1 stops Windows from making the last printer used
# the default.
$pdf = 'Microsoft Print to PDF'
$port = (Get-ItemProperty -ErrorAction SilentlyContinue -Path "$root\Software\Microsoft\Windows NT\CurrentVersion\Devices").$pdf
if (-not $port) {
	$port = 'winspool,Ne00:'
}
Set-UserValue 'Software\Microsoft\Windows NT\CurrentVersion\Windows' LegacyDefaultPrinterMode 1
Set-UserValue 'Software\Microsoft\Windows NT\CurrentVersion\Windows' Device "$pdf,$port" String
if ($loaded) {
	[gc]::Collect()
	& reg.exe unload "HKU\$sid" | Out-Null
}
Write-Output "the settings of Office are set, and the default printer is $pdf"

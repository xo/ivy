# run-as-user.ps1 runs a script in the desktop of the logged-on user `user`,
# and waits for it. COM automation of Office needs that desktop, and
# `incus exec` runs as SYSTEM in session 0, which has none. Run it as an
# administrator:
#
#   powershell -NoProfile -ExecutionPolicy Bypass -File C:\ivy\run-as-user.ps1 `
#     -Script C:\ivy\scripts\export.ps1 -Done C:\ivy\out\msoffice\.done `
#     -Arguments '-Source C:\ivy\files -Out C:\ivy\out\msoffice'
#
# It writes a runner next to Done, starts it through a scheduled task that
# runs only while the user is logged on, and waits until the runner writes
# Done with the exit code of the script, or until Timeout seconds pass.

param(
	[Parameter(Mandatory = $true)][string]$Script,
	[Parameter(Mandatory = $true)][string]$Done,
	[string]$Arguments = '',
	[int]$Timeout = 14400
)

$ErrorActionPreference = 'Stop'
$dir = Split-Path -Parent $Done
New-Item -ItemType Directory -Force -Path $dir | Out-Null
Remove-Item -Force -ErrorAction SilentlyContinue $Done

# The user must be able to write the folder of the output.
& icacls.exe $dir /grant 'user:(OI)(CI)M' | Out-Null

$runner = Join-Path $dir 'runner.ps1'
@"
`$code = 1
try {
	& powershell.exe -NoProfile -ExecutionPolicy Bypass -File '$Script' $Arguments *> '$dir\run.log'
	`$code = `$LASTEXITCODE
} finally {
	Set-Content -Path '$Done' -Value `$code
}
"@ | Set-Content -Path $runner -Encoding UTF8

$name = 'ivy-run-as-user'
$action = "powershell.exe -NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -File `"$runner`""
& schtasks.exe /Create /F /TN $name /TR $action /SC ONCE /ST 00:00 /RU user /IT | Out-Null
& schtasks.exe /Run /TN $name | Out-Null

$start = Get-Date
while (-not (Test-Path $Done)) {
	if (((Get-Date) - $start).TotalSeconds -gt $Timeout) {
		& schtasks.exe /End /TN $name | Out-Null
		throw "the script did not finish within $Timeout seconds"
	}
	Start-Sleep -Seconds 5
}
& schtasks.exe /Delete /F /TN $name | Out-Null
$code = [int](Get-Content $Done)
Write-Output "the script exited with $code"
exit $code

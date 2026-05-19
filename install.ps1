$ErrorActionPreference = "Stop"

$Repo = "AgusRdz/bonsai"
$InstallDir = if ($env:BONSAI_INSTALL_DIR) { $env:BONSAI_INSTALL_DIR } else { "$env:LOCALAPPDATA\Programs\bonsai" }

# Detect architecture
$Arch = if ([System.Runtime.InteropServices.RuntimeInformation]::ProcessArchitecture -eq [System.Runtime.InteropServices.Architecture]::Arm64) {
    "arm64"
} else {
    "amd64"
}

$Binary = "bonsai-windows-$Arch.exe"

# Get latest version
if (-not $env:BONSAI_VERSION) {
    $Release = Invoke-RestMethod "https://api.github.com/repos/$Repo/releases/latest"
    $env:BONSAI_VERSION = $Release.tag_name
}

if (-not $env:BONSAI_VERSION) {
    Write-Error "failed to determine latest version"
    exit 1
}

$Url = "https://github.com/$Repo/releases/download/$($env:BONSAI_VERSION)/$Binary"

Write-Host "installing bonsai $($env:BONSAI_VERSION) (windows/$Arch)..."

New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null

$Destination = Join-Path $InstallDir "bonsai.exe"
$TmpDestination = "$Destination.tmp"
$OldDestination = "$Destination.old"

Invoke-WebRequest -Uri $Url -OutFile $TmpDestination

# Stop any running bonsai processes so we can replace the binary.
Get-Process -Name bonsai -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue

# Two-step replace: move current aside, then move new into place.
# Handles the case where AV briefly locks the new file after download.
if (Test-Path $Destination) {
    Remove-Item $OldDestination -Force -ErrorAction SilentlyContinue
    Rename-Item $Destination $OldDestination -Force
}
$retryDelays = @(0, 500, 1000, 2000)
$replaced = $false
foreach ($delay in $retryDelays) {
    if ($delay -gt 0) { Start-Sleep -Milliseconds $delay }
    try {
        Rename-Item $TmpDestination $Destination -Force
        $replaced = $true
        break
    } catch {}
}
if (-not $replaced) {
    if (Test-Path $OldDestination) { Rename-Item $OldDestination $Destination -Force }
    Remove-Item $TmpDestination -Force -ErrorAction SilentlyContinue
    Write-Error "failed to install bonsai — antivirus may be blocking the file"
    exit 1
}
Remove-Item $OldDestination -Force -ErrorAction SilentlyContinue

Write-Host "installed bonsai to $Destination"
Write-Host ""

# Add to user PATH if not already present
$UserPath = [Environment]::GetEnvironmentVariable("PATH", "User")
$CleanInstallDir = $InstallDir.TrimEnd("\")
$PathParts = $UserPath -split ";" | ForEach-Object { $_.TrimEnd("\") }

if ($PathParts -notcontains $CleanInstallDir) {
    $NewUserPath = "$InstallDir;$UserPath"
    [Environment]::SetEnvironmentVariable("PATH", $NewUserPath, "User")
    Write-Host "added $InstallDir to PATH"
}

# Update current session PATH
$CurrentPathParts = $env:PATH -split ";" | ForEach-Object { $_.TrimEnd("\") }
if ($CurrentPathParts -notcontains $CleanInstallDir) {
    $env:PATH = "$InstallDir;$env:PATH"
}

# Notify Windows of PATH change
$HWND_BROADCAST = [IntPtr]0xffff
$WM_SETTINGCHANGE = 0x001a
$MethodDefinition = @'
[DllImport("user32.dll", SetLastError = true, CharSet = CharSet.Auto)]
public static extern IntPtr SendMessageTimeout(IntPtr hWnd, uint Msg, IntPtr wParam, string lParam, uint fuFlags, uint uTimeout, out IntPtr lpdwResult);
'@
$User32 = Add-Type -MemberDefinition $MethodDefinition -Name "User32" -Namespace "Win32" -PassThru
$result = [IntPtr]::Zero
$User32::SendMessageTimeout($HWND_BROADCAST, $WM_SETTINGCHANGE, [IntPtr]::Zero, "Environment", 2, 100, [ref]$result) | Out-Null

Write-Host ""
Write-Host "bonsai is ready. Run 'bonsai help' to get started."

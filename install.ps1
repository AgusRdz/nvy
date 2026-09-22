$ErrorActionPreference = "Stop"

$Repo = "AgusRdz/nvy"
$InstallDir = if ($env:NVY_INSTALL_DIR) { $env:NVY_INSTALL_DIR } else { "$env:LOCALAPPDATA\Programs\nvy" }

# Detect architecture
$Arch = if ([System.Runtime.InteropServices.RuntimeInformation]::ProcessArchitecture -eq [System.Runtime.InteropServices.Architecture]::Arm64) {
    "arm64"
} else {
    "amd64"
}

$Binary = "nvy-windows-$Arch.exe"

# Get latest version — use the newest release from the list (newest first)
# rather than /releases/latest, which lags while GitHub flips the "latest" flag.
if (-not $env:NVY_VERSION) {
    $Releases = Invoke-RestMethod "https://api.github.com/repos/$Repo/releases"
    $env:NVY_VERSION = ($Releases | Select-Object -First 1).tag_name
}

if (-not $env:NVY_VERSION) {
    Write-Error "failed to determine latest version"
    exit 1
}

$Url = "https://github.com/$Repo/releases/download/$($env:NVY_VERSION)/$Binary"
$ChecksumsUrl = "https://github.com/$Repo/releases/download/$($env:NVY_VERSION)/checksums.txt"

Write-Host "installing nvy $($env:NVY_VERSION) (windows/$Arch)..."

# Create install dir
New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null

$Destination = Join-Path $InstallDir "nvy.exe"
$TmpDestination = "$Destination.tmp"

# Download binary to temp file
Invoke-WebRequest -Uri $Url -OutFile $TmpDestination

# Verify SHA256 checksum before installing
try {
    $ChecksumsContent = Invoke-RestMethod $ChecksumsUrl
} catch {
    Write-Error "failed to download checksums.txt: $_"
    Remove-Item -Force $TmpDestination -ErrorAction SilentlyContinue
    exit 1
}

$ExpectedLine = $ChecksumsContent -split "`n" | Where-Object { $_ -match "\s$([regex]::Escape($Binary))$" } | Select-Object -First 1
if (-not $ExpectedLine) {
    Write-Error "checksum not found for $Binary"
    Remove-Item -Force $TmpDestination -ErrorAction SilentlyContinue
    exit 1
}
$Expected = ($ExpectedLine -split '\s+')[0].Trim()

$Actual = (Get-FileHash -Algorithm SHA256 $TmpDestination).Hash.ToLower()
if ($Actual -ne $Expected) {
    Write-Error "checksum mismatch for ${Binary}: expected $Expected, got $Actual"
    Remove-Item -Force $TmpDestination -ErrorAction SilentlyContinue
    exit 1
}

Move-Item -Force $TmpDestination $Destination

Write-Host "installed nvy to $Destination"
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

# Update current session PATH so it can be used immediately
$CurrentPathParts = $env:PATH -split ";" | ForEach-Object { $_.TrimEnd("\") }
if ($CurrentPathParts -notcontains $CleanInstallDir) {
    $env:PATH = "$InstallDir;$env:PATH"
}

# Notify system of PATH change
$HWND_BROADCAST = [IntPtr]0xffff
$WM_SETTINGCHANGE = 0x001a
$MethodDefinition = @'
[DllImport("user32.dll", SetLastError = true, CharSet = CharSet.Auto)]
public static extern IntPtr SendMessageTimeout(IntPtr hWnd, uint Msg, IntPtr wParam, string lParam, uint fuFlags, uint uTimeout, out IntPtr lpdwResult);
'@
$User32 = Add-Type -MemberDefinition $MethodDefinition -Name "User32" -Namespace "Win32" -PassThru
$result = [IntPtr]::Zero
$User32::SendMessageTimeout($HWND_BROADCAST, $WM_SETTINGCHANGE, [IntPtr]::Zero, "Environment", 2, 100, [ref]$result) | Out-Null

# Install the $PROFILE prompt hook and register the daily expiration check task
& "$Destination" init

Write-Host ""
Write-Host "Installation complete."
Write-Host ""
Write-Host "  The current session's PATH is already updated, so 'nvy' works right now."
Write-Host "  The prompt hook was added to your `$PROFILE but only takes effect in a NEW"
Write-Host "  shell (or after running: . `$PROFILE) — this session's prompt is not"
Write-Host "  retroactively hooked."
Write-Host ""
Write-Host "  After that, global vars and project .env files sync automatically."

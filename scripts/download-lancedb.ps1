<#
.SYNOPSIS
    Downloads LanceDB native libraries for Windows.
.DESCRIPTION
    PowerShell equivalent of the upstream download-artifacts.sh script.
    Downloads static library, dynamic library, and header file from GitHub releases.
.PARAMETER Version
    Version to download (e.g., v0.1.2). If not specified, downloads the latest release.
.EXAMPLE
    .\download-lancedb.ps1 v0.1.2
#>

param(
    [string]$Version
)

$ErrorActionPreference = "Stop"

# Configuration
$GitHubRepo = "lancedb/lancedb-go"
$GitHubApiUrl = "https://api.github.com/repos/$GitHubRepo"
$GitHubReleasesUrl = "https://github.com/$GitHubRepo/releases/download"

# --- Helper Functions ---

function Write-Info($Message)    { Write-Host "[INFO]    $Message" -ForegroundColor Blue }
function Write-Success($Message) { Write-Host "[OK]      $Message" -ForegroundColor Green }
function Write-Warn($Message)    { Write-Host "[WARN]    $Message" -ForegroundColor Yellow }
function Write-Err($Message)     { Write-Host "[ERROR]   $Message" -ForegroundColor Red }

function Get-LatestVersion {
    Write-Info "Fetching latest release version..."
    try {
        $response = Invoke-RestMethod -Uri "$GitHubApiUrl/releases/latest" -Headers @{ "User-Agent" = "PowerShell" }
        return $response.tag_name
    } catch {
        Write-Err "Failed to fetch latest version from GitHub API: $_"
        exit 1
    }
}

function Test-ReleaseExists($Ver) {
    try {
        $null = Invoke-RestMethod -Uri "$GitHubApiUrl/releases/tags/$Ver" -Headers @{ "User-Agent" = "PowerShell" }
    } catch {
        Write-Err "Release $Ver not found."
        exit 1
    }
}

function Get-FileFromGitHub($Url, $OutPath) {
    $filename = Split-Path $OutPath -Leaf
    Write-Info "Downloading $filename..."

    $dir = Split-Path $OutPath -Parent
    if (-not (Test-Path $dir)) { New-Item -ItemType Directory -Path $dir -Force | Out-Null }

    try {
        Invoke-WebRequest -Uri $Url -OutFile $OutPath -UseBasicParsing
        Write-Success "Downloaded $filename"
        return $true
    } catch {
        Write-Warn "Failed to download $filename from $Url"
        return $false
    }
}

function Get-CompleteArchive($Ver, $TargetPlatform) {
    Write-Info "Downloading complete archive as fallback..."

    $archiveUrl  = "$GitHubReleasesUrl/$Ver/lancedb-go-native-binaries.tar.gz"
    $archivePath = "lancedb-go-native-binaries.tar.gz"

    if (Get-FileFromGitHub $archiveUrl $archivePath) {
        Write-Info "Extracting archive..."
        try {
            tar -xzf $archivePath
            Remove-Item $archivePath -Force -ErrorAction SilentlyContinue
            if (Test-Path "lib/$TargetPlatform") {
                Write-Success "Platform-specific files found in archive"
                return $true
            } else {
                Write-Err "Platform $TargetPlatform not found in archive"
                return $false
            }
        } catch {
            Write-Err "Failed to extract archive: $_"
            Remove-Item $archivePath -Force -ErrorAction SilentlyContinue
            return $false
        }
    } else {
        Write-Err "Failed to download complete archive"
        return $false
    }
}

# --- Main ---

Write-Info "LanceDB Go Native Artifacts Downloader (Windows PowerShell)"
Write-Info "============================================================"

$platform     = "windows"
$arch         = if ([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture -eq "Arm64") { "arm64" } else { "amd64" }
$platformArch = "${platform}_${arch}"

Write-Info "Target platform: $platformArch"

# Resolve version
if (-not $Version) {
    $Version = Get-LatestVersion
    Write-Info "Using latest version: $Version"
} else {
    if ($Version -notmatch '^v\d+\.\d+\.\d+') {
        Write-Err "Invalid version format: $Version (expected vX.Y.Z)"
        exit 1
    }
    Write-Info "Using specified version: $Version"
}

Test-ReleaseExists $Version

# Directory setup
$currentDir = Get-Location
$libDir     = Join-Path $currentDir "lib/$platformArch"
$includeDir = Join-Path $currentDir "include"

Write-Info "Creating directory structure..."
New-Item -ItemType Directory -Path $libDir     -Force | Out-Null
New-Item -ItemType Directory -Path $includeDir -Force | Out-Null

$baseUrl        = "$GitHubReleasesUrl/$Version"
$staticLibName  = "liblancedb_go.a"
$dynamicLibName = "lancedb_go.dll"

# Download static library
$staticUrl  = "$baseUrl/$staticLibName"
$staticPath = Join-Path $libDir $staticLibName

if (-not (Get-FileFromGitHub $staticUrl $staticPath)) {
    Write-Warn "Individual file download failed. Falling back to complete archive..."
    if (-not (Get-CompleteArchive $Version $platformArch)) {
        Write-Err "All download methods failed."
        exit 1
    }
}

# Download dynamic library (optional)
$dynamicUrl  = "$baseUrl/$dynamicLibName"
$dynamicPath = Join-Path $libDir $dynamicLibName
$null = Get-FileFromGitHub $dynamicUrl $dynamicPath

# Download header
$headerUrl  = "$baseUrl/lancedb.h"
$headerPath = Join-Path $includeDir "lancedb.h"
$null = Get-FileFromGitHub $headerUrl $headerPath

# Verify
Write-Info "Verifying downloaded files..."
foreach ($f in @($staticPath, $dynamicPath, $headerPath)) {
    if (Test-Path $f) {
        $size = (Get-Item $f).Length
        $sizeKB = [math]::Round($size / 1KB, 1)
        Write-Success "$f (${sizeKB} KB)"
    }
}

Write-Success "Download completed for $platformArch!"

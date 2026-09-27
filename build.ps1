<#
.SYNOPSIS
    Build script for Downloader (PowerShell).

.DESCRIPTION
    Builds the Downloader application for Linux and Windows targets, with cross-compilation support.

.PARAMETER Target
    Build target: 'all', 'linux', 'windows', 'current' (default).

.PARAMETER Clean
    Cleans the output directory before building.

.PARAMETER OutputDir
    Directory where built binaries will be stored (default: 'bin').

.EXAMPLE
    .\build.ps1
    .\build.ps1 -Target linux
    .\build.ps1 -Target windows
    .\build.ps1 -Target all
    .\build.ps1 -Clean
#>

[CmdletBinding()]
param (
    [ValidateSet("all", "linux", "lunix", "windows", "win", "current", "clean")]
    [string]$Target = "current",

    [switch]$Clean,

    [string]$OutputDir = "bin"
)

$ErrorActionPreference = "Stop"

$AppName = "downloader"
$Package = "./cmd/downloader"
$LdFlags = "-s -w"

function Assert-GoInstalled {
    if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
        Write-Error "[ERROR] 'go' command not found. Please install Go (https://golang.org/dl/) or add it to PATH."
        exit 1
    }
}

function Invoke-Clean {
    if (Test-Path $OutputDir) {
        Write-Host "Cleaning output directory '$OutputDir'..." -ForegroundColor Yellow
        Remove-Item -Recurse -Force $OutputDir
        Write-Host "Clean complete." -ForegroundColor Green
    }
}

function Build-Binary {
    param (
        [string]$TargetOS,
        [string]$TargetArch
    )

    $ext = if ($TargetOS -eq "windows") { ".exe" } else { "" }
    $outFile = Join-Path $OutputDir "$AppName-$TargetOS-$TargetArch$ext"

    Write-Host "Building $AppName for $TargetOS/$TargetArch -> $outFile..." -ForegroundColor Cyan

    $env:CGO_ENABLED = "0"
    $env:GOOS = $TargetOS
    $env:GOARCH = $TargetArch

    & go build -trimpath "-ldflags=$LdFlags" -o $outFile $Package
    if ($LASTEXITCODE -ne 0) {
        Write-Error "Build failed for $TargetOS/$TargetArch!"
        exit $LASTEXITCODE
    }
}

function Build-Linux {
    if (-not (Test-Path $OutputDir)) { New-Item -ItemType Directory -Path $OutputDir | Out-Null }
    Build-Binary -TargetOS "linux" -TargetArch "amd64"
    Build-Binary -TargetOS "linux" -TargetArch "arm64"

    # Default linux binary
    Copy-Item -Path (Join-Path $OutputDir "$AppName-linux-amd64") -Destination (Join-Path $OutputDir $AppName) -Force -ErrorAction SilentlyContinue
}

function Build-Windows {
    if (-not (Test-Path $OutputDir)) { New-Item -ItemType Directory -Path $OutputDir | Out-Null }
    Build-Binary -TargetOS "windows" -TargetArch "amd64"
    Build-Binary -TargetOS "windows" -TargetArch "arm64"

    # Default windows binary
    Copy-Item -Path (Join-Path $OutputDir "$AppName-windows-amd64.exe") -Destination (Join-Path $OutputDir "$AppName.exe") -Force -ErrorAction SilentlyContinue
}

function Build-Current {
    if (-not (Test-Path $OutputDir)) { New-Item -ItemType Directory -Path $OutputDir | Out-Null }

    $hostOS = (& go env GOOS).Trim()
    $hostArch = (& go env GOARCH).Trim()
    $ext = if ($hostOS -eq "windows") { ".exe" } else { "" }
    $outFile = Join-Path $OutputDir "$AppName$ext"

    Write-Host "Building $AppName for host platform ($hostOS/$hostArch) -> $outFile..." -ForegroundColor Cyan

    $env:CGO_ENABLED = "0"
    $env:GOOS = $hostOS
    $env:GOARCH = $hostArch

    & go build -trimpath "-ldflags=$LdFlags" -o $outFile $Package
    if ($LASTEXITCODE -ne 0) {
        Write-Error "Build failed!"
        exit $LASTEXITCODE
    }
}

# Main execution
Assert-GoInstalled

if ($Clean -or $Target -eq "clean") {
    Invoke-Clean
    if ($Target -eq "clean") {
        exit 0
    }
}

switch ($Target.ToLower()) {
    "all" {
        Write-Host "=== Building all targets (Linux & Windows) ===" -ForegroundColor Green
        Build-Linux
        Build-Windows
        Write-Host "=== Build finished successfully! Outputs in '$OutputDir/' ===" -ForegroundColor Green
    }
    { $_ -in "linux", "lunix" } {
        Write-Host "=== Building Linux targets ===" -ForegroundColor Green
        Build-Linux
        Write-Host "=== Build finished successfully! Outputs in '$OutputDir/' ===" -ForegroundColor Green
    }
    { $_ -in "windows", "win" } {
        Write-Host "=== Building Windows targets ===" -ForegroundColor Green
        Build-Windows
        Write-Host "=== Build finished successfully! Outputs in '$OutputDir/' ===" -ForegroundColor Green
    }
    "current" {
        Write-Host "=== Building current host target ===" -ForegroundColor Green
        Build-Current
        Write-Host "=== Build finished successfully! Output in '$OutputDir/' ===" -ForegroundColor Green
    }
}

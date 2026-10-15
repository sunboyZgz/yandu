param(
    [switch]$Media,
    [switch]$Check,
    [switch]$AllowSystemInstall,
    [string]$PythonExe,
    [string]$StateDir = '.yandu-storage-dev',
    [string]$BinDir = '.local/bin'
)
$ErrorActionPreference = 'Stop'
& (Join-Path $PSScriptRoot 'scripts/setup-windows-env.ps1') @PSBoundParameters
exit $LASTEXITCODE

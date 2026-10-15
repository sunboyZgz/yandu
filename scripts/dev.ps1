param(
    [ValidateSet('run', 'build', 'ui', 'status', 'doctor', 'plan')]
    [string]$Mode = 'run',
    [string]$BinDir = '.local/bin',
    [string]$StateDir = '.yandu',
    [string]$PythonExe,
    [switch]$NoOpen
)
$ErrorActionPreference = 'Stop'

# Windows PowerShell 5.1 and PowerShell 7; no administrator requirement or service registration.
$YanduPython = & {
    param($SelectedPython)
    # Keep the setup script's parameters in a child scope so source directories
    # supplied above are preserved. Dot-sourcing only defines the probe functions.
    . (Join-Path $PSScriptRoot 'setup-windows-env.ps1')
    Assert-YanduWindowsHost
    Find-YanduPython ([version]'3.10.0') -PythonExe $SelectedPython
} $PythonExe
if ($null -eq $YanduPython) { throw 'Python 3.10+ x64 is required. Check the detection output or select an existing interpreter with -PythonExe; no installer was called.' }

$YanduArguments = @(
    (Join-Path $PSScriptRoot 'dev.py'), $Mode.ToLowerInvariant(),
    '--target', 'windows-amd64',
    '--bin-dir', $BinDir,
    '--state-dir', $StateDir
)
if ($NoOpen) { $YanduArguments += '--no-open' }
& $YanduPython.Path @YanduArguments
exit $LASTEXITCODE

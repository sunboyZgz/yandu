param(
    [ValidateSet('run', 'build', 'ui', 'status', 'doctor', 'plan')]
    [string]$Mode = 'run',
    [string]$BinDir = '.local/bin',
    [string]$StateDir = '.yandu',
    [switch]$NoOpen
)
$ErrorActionPreference = 'Stop'

# Windows PowerShell 5.1 and PowerShell 7; no administrator requirement or service registration.
$YanduPython = $null
$YanduPythonPrefix = @()
foreach ($YanduCandidate in @(
    @{ Name = 'py'; Prefix = @('-3') },
    @{ Name = 'python'; Prefix = @() },
    @{ Name = 'python3'; Prefix = @() }
)) {
    $YanduCommand = Get-Command $YanduCandidate.Name -CommandType Application -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($null -eq $YanduCommand) { continue }
    $YanduProbePrefix = $YanduCandidate.Prefix
    & $YanduCommand.Source @YanduProbePrefix -c 'import sys; sys.exit(0 if sys.version_info >= (3, 10) else 1)' | Out-Null
    if ($LASTEXITCODE -eq 0) {
        $YanduPython = $YanduCommand.Source
        $YanduPythonPrefix = $YanduCandidate.Prefix
        break
    }
}
if ($null -eq $YanduPython) { throw 'Python 3.10+ is required. Install Python and reopen PowerShell.' }

$YanduArguments = @(
    (Join-Path $PSScriptRoot 'dev.py'), $Mode.ToLowerInvariant(),
    '--target', 'windows-amd64',
    '--bin-dir', $BinDir,
    '--state-dir', $StateDir
)
if ($NoOpen) { $YanduArguments += '--no-open' }
& $YanduPython @YanduPythonPrefix @YanduArguments
exit $LASTEXITCODE

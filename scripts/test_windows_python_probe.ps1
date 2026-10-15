param([Parameter(Mandatory = $true)][string]$PythonExe)
$ErrorActionPreference = 'Stop'
$probePythonExe = $PythonExe
. (Join-Path $PSScriptRoot 'setup-windows-env.ps1')
$previousPrefix = $env:CONDA_PREFIX
$previousPython = $env:CONDA_PYTHON_EXE
$previousPassing = $PSNativeCommandArgumentPassing
$temporary = $null
try {
    # Exercise a native interpreter, not the PowerShell fixture used by policy
    # tests. Legacy reproduces Windows PowerShell 5.1 quote marshalling on PS7.
    $PSNativeCommandArgumentPassing = 'Legacy'
    $selected = Find-YanduPython ([version]'3.12.0') -PythonExe $probePythonExe
    if (!$selected -or [version]$selected.Version -lt [version]'3.12.0') { throw 'Native Python probe rejected a compatible interpreter in Legacy mode.' }
    $expected = $selected.Version
    $native = $selected.Path
    if ([IO.Path]::GetFileName($probePythonExe) -eq 'python.exe') {
        $env:CONDA_PREFIX = Split-Path $probePythonExe -Parent
    } else {
        # On a Unix validation host, expose the native interpreter under the
        # Windows Conda filename without installing or copying a Python runtime.
        $temporary = Join-Path ([IO.Path]::GetTempPath()) ('yandu conda probe ' + [Guid]::NewGuid().ToString('N'))
        New-Item -ItemType Directory -Path $temporary | Out-Null
        New-Item -ItemType SymbolicLink -Path (Join-Path $temporary 'python.exe') -Target $native | Out-Null
        $env:CONDA_PREFIX = $temporary
    }
    $env:CONDA_PYTHON_EXE = $native
    $selected = Find-YanduPython ([version]'3.12.0')
    if (!$selected -or $selected.Source -ne 'active Conda environment' -or $selected.Version -ne $expected) { throw 'Active Conda Python was not preferred or its version was misread.' }
    Write-Host 'Native Python and active Conda discovery passed in Legacy argument mode; no system package manager was called.'
} finally {
    $env:CONDA_PREFIX = $previousPrefix
    $env:CONDA_PYTHON_EXE = $previousPython
    $PSNativeCommandArgumentPassing = $previousPassing
    if ($temporary) { Remove-Item -LiteralPath $temporary -Recurse -Force }
}

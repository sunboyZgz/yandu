param(
    [switch]$Media,
    [switch]$Check,
    [string]$StateDir = '.yandu-storage-dev',
    [string]$BinDir = '.local/bin'
)
$ErrorActionPreference = 'Stop'
$YanduRoot = Split-Path $PSScriptRoot -Parent

function Update-YanduProcessPath {
    # Refresh only this process after installers update their normal locations.
    $paths = @($env:PATH -split ';')
    $paths += @([Environment]::GetEnvironmentVariable('PATH', 'Machine') -split ';')
    $paths += @([Environment]::GetEnvironmentVariable('PATH', 'User') -split ';')
    $paths += @(
        (Join-Path $env:ProgramFiles 'Go/bin'),
        (Join-Path $env:ProgramFiles 'Git/cmd'),
        (Join-Path $env:ProgramFiles 'CMake/bin'),
        (Join-Path $env:ProgramFiles '7-Zip')
    )
    foreach ($base in @((Join-Path $env:LOCALAPPDATA 'Programs/Python'), $env:ProgramFiles)) {
        if (Test-Path -LiteralPath $base) {
            $paths += @(Get-ChildItem -LiteralPath $base -Directory -Filter 'Python*' | Select-Object -ExpandProperty FullName)
        }
    }
    $env:PATH = (@($paths | Where-Object { $_ } | Select-Object -Unique) -join ';')
}

function Find-YanduPython([version]$Minimum) {
    foreach ($name in @('python.exe', 'python3.exe', 'py.exe')) {
        foreach ($command in @(Get-Command $name -CommandType Application -All -ErrorAction SilentlyContinue)) {
            if ($command.Source -like '*\Microsoft\WindowsApps\*') { continue }
            $prefix = @()
            if ($name -eq 'py.exe') { $prefix = @('-3') }
            try {
                $value = & $command.Source @prefix -c 'import sys; print(".".join(map(str, sys.version_info[:3]))); sys.exit(0 if sys.maxsize > 2**32 else 1)' 2>$null
                if ($LASTEXITCODE -eq 0 -and "$value" -match '^\d+\.\d+\.\d+$' -and [version]"$value" -ge $Minimum) {
                    return @{ Path = $command.Source; Prefix = $prefix; Version = "$value" }
                }
            } catch { continue }
        }
    }
    return $null
}

function Find-YanduGo([version]$Minimum) {
    foreach ($command in @(Get-Command 'go.exe' -CommandType Application -All -ErrorAction SilentlyContinue)) {
        $previous = $env:GOTOOLCHAIN
        try {
            $env:GOTOOLCHAIN = 'local'
            $value = & $command.Source version 2>$null
            if ($LASTEXITCODE -eq 0 -and "$value" -match '^go version go(\d+\.\d+(?:\.\d+)?) windows/amd64$') {
                $version = $Matches[1]
                if ([version]$version -ge $Minimum) { return @{ Path = $command.Source; Version = $version } }
            }
        } catch { continue } finally { $env:GOTOOLCHAIN = $previous }
    }
    return $null
}

function Install-YanduPackage([string]$Id, [string[]]$Extra = @()) {
    $winget = Get-Command 'winget.exe' -CommandType Application -ErrorAction SilentlyContinue
    if ($null -eq $winget) {
        throw 'WinGet is required to install missing tools. Install/update Microsoft App Installer, reopen PowerShell, then retry.'
    }
    Write-Host "Installing missing prerequisite: $Id"
    & $winget.Source install --id $Id --exact --source winget --architecture x64 --silent --accept-package-agreements --accept-source-agreements --disable-interactivity @Extra
    if ($LASTEXITCODE -notin @(0, 3010)) { throw "WinGet failed for $Id (exit $LASTEXITCODE). Check its output and retry." }
    Update-YanduProcessPath
}

function Find-YanduVisualStudio {
    $vswhere = Join-Path ${env:ProgramFiles(x86)} 'Microsoft Visual Studio/Installer/vswhere.exe'
    if (!(Test-Path -LiteralPath $vswhere)) { return $null }
    $location = & $vswhere -latest -products '*' -version '[17.0,)' -requires Microsoft.VisualStudio.Component.VC.Tools.x86.x64 -property installationPath
    if ($LASTEXITCODE -eq 0 -and $location) { return "$location" }
    return $null
}

function Initialize-YanduMediaTools([switch]$CheckOnly) {
    foreach ($item in @(
        @{ Command = 'git.exe'; Id = 'Git.Git' },
        @{ Command = 'cmake.exe'; Id = 'Kitware.CMake' },
        @{ Command = '7z.exe'; Id = '7zip.7zip' }
    )) {
        if (!(Get-Command $item.Command -CommandType Application -ErrorAction SilentlyContinue)) {
            if ($CheckOnly) { throw "Missing $($item.Command); run setup-windows-env.ps1 -Media." }
            Install-YanduPackage $item.Id
            if (!(Get-Command $item.Command -CommandType Application -ErrorAction SilentlyContinue)) { throw "$($item.Command) was not found after installation. Reopen PowerShell and retry." }
        }
    }
    $studio = Find-YanduVisualStudio
    if (!$studio) {
        if ($CheckOnly) { throw 'Missing Visual C++ x64 build tools; run setup-windows-env.ps1 -Media.' }
        $vswhere = Join-Path ${env:ProgramFiles(x86)} 'Microsoft Visual Studio/Installer/vswhere.exe'
        $existing = $null
        if (Test-Path -LiteralPath $vswhere) {
            $instances = & $vswhere -latest -products Microsoft.VisualStudio.Product.BuildTools -version '[17.0,)' -format json | ConvertFrom-Json
            if ($instances) { $existing = @($instances)[0] }
        }
        if ($existing) {
            $installer = Join-Path ${env:ProgramFiles(x86)} 'Microsoft Visual Studio/Installer/setup.exe'
            $arguments = 'modify --installPath "' + $existing.installationPath + '" --channelId "' + $existing.channelId + '" --add Microsoft.VisualStudio.Workload.VCTools --includeRecommended --passive --norestart'
            $process = Start-Process -FilePath $installer -ArgumentList $arguments -Verb RunAs -Wait -PassThru
            if ($process.ExitCode -notin @(0, 3010)) { throw "Visual C++ setup failed (exit $($process.ExitCode))." }
        } else {
            Install-YanduPackage 'Microsoft.VisualStudio.2022.BuildTools' @('--override', '--wait --passive --add Microsoft.VisualStudio.Workload.VCTools --includeRecommended --norestart')
        }
        $studio = Find-YanduVisualStudio
        if (!$studio) { throw 'Visual C++ x64 tools are not ready. Finish the installer or restart Windows if requested, then retry -Media.' }
    }
    if (!$CheckOnly) {
        $module = Join-Path $studio 'Common7/Tools/Microsoft.VisualStudio.DevShell.dll'
        Import-Module $module
        Enter-VsDevShell -VsInstallPath $studio -SkipAutomaticLocation -DevCmdArguments '-arch=x64 -host_arch=x64' | Out-Null
    }
}

function Test-YanduPreparedComponents([string]$Config, [switch]$Media) {
    if (!(Test-Path -LiteralPath $Config)) { throw "Components have not been prepared: $Config. Run setup-windows-env.ps1 first." }
    $record = Get-Content -LiteralPath $Config -Raw -Encoding UTF8 | ConvertFrom-Json
    $required = @('caddy', 'frpc', 'sftpgo')
    if ($Media) { $required += @('node', 'ffmpeg', 'ffprobe', 'heif-thumbnailer') }
    foreach ($name in $required) {
        $tool = $record.tools.$name
        if (!$tool -or !(Test-Path -LiteralPath $tool.path -PathType Leaf)) { throw "Missing prepared component: $name. Run setup-windows-env.ps1 again." }
        if ((Get-FileHash -LiteralPath $tool.path -Algorithm SHA256).Hash -ne $tool.sha256) { throw "Component changed: $name. Run setup-windows-env.ps1 again." }
    }
    if (!$record.sftpgoTemplates -or !(Test-Path -LiteralPath (Join-Path $record.sftpgoTemplates 'email') -PathType Container)) { throw 'Missing SFTPGo email templates; prepare components again.' }
}

function Invoke-YanduWindowsSetup {
    param([switch]$Media, [switch]$Check, [string]$StateDir = '.yandu-storage-dev', [string]$BinDir = '.local/bin')
    if ($env:OS -ne 'Windows_NT' -or $env:PROCESSOR_ARCHITECTURE -ne 'AMD64') { throw 'Use 64-bit PowerShell on Windows x64.' }
    Update-YanduProcessPath
    $goLine = Select-String -LiteralPath (Join-Path $YanduRoot 'go.mod') -Pattern '^go\s+(\d+\.\d+(?:\.\d+)?)\s*$'
    if (!$goLine) { throw 'go.mod is missing the minimum Go version.' }
    $goMinimum = [version]$goLine.Matches[0].Groups[1].Value
    $pythonMinimum = [version]'3.10.0'
    if ($Media) { $pythonMinimum = [version]'3.12.0' }
    $python = Find-YanduPython $pythonMinimum
    if (!$python -and !$Check) { Install-YanduPackage 'Python.Python.3.14'; $python = Find-YanduPython $pythonMinimum }
    if (!$python) { throw "Python $pythonMinimum+ is required. Run setup-windows-env.ps1, or -Media for media tools." }
    $go = Find-YanduGo $goMinimum
    if (!$go -and !$Check) { Install-YanduPackage 'GoLang.Go'; $go = Find-YanduGo $goMinimum }
    if (!$go) { throw "Go $goMinimum+ is required. Finish installation, reopen PowerShell, and retry." }
    Write-Host "Python $($python.Version): $($python.Path)"
    Write-Host "Go $($go.Version): $($go.Path)"
    if ($Media) { Initialize-YanduMediaTools -CheckOnly:$Check }
    $statePath = $StateDir
    if (![IO.Path]::IsPathRooted($statePath)) { $statePath = Join-Path $YanduRoot $statePath }
    $state = [IO.Path]::GetFullPath($statePath)
    if ($Check) { Test-YanduPreparedComponents (Join-Path $state 'tools.json') -Media:$Media; Write-Host 'Windows runtime and prepared component checks passed.'; return }
    # dev.py must use the qualified Go rather than an older PATH entry.
    $env:PATH = (Split-Path $go.Path -Parent) + ';' + $env:PATH
    $env:GOTOOLCHAIN = 'local'
    $prefix = $python.Prefix
    $arguments = @((Join-Path $YanduRoot 'scripts/storage-dev.py'), '--prepare-only', '--state-dir', $StateDir, '--bin-dir', $BinDir)
    if (!$Media) { $arguments += '--no-media' }
    & $python.Path @prefix @arguments
    if ($LASTEXITCODE -ne 0) { throw "Dependency preparation failed (exit $LASTEXITCODE). Fix the reported component and rerun this script; verified caches are kept." }
    Write-Host 'Windows source environment is ready. No Agent or SFTPGo service was started.'
    Write-Host 'If system tools were installed, reopen PowerShell before starting development.'
    $next = @($python.Path) + @($prefix) + @((Join-Path $YanduRoot 'scripts/storage-dev.py'), '--state-dir', $StateDir, '--bin-dir', $BinDir)
    if (!$Media) { $next += '--no-media' }
    Write-Host ('Next: & ' + (($next | ForEach-Object { "'" + $_.Replace("'", "''") + "'" }) -join ' '))
}

if ($MyInvocation.InvocationName -ne '.') {
    try { Invoke-YanduWindowsSetup @PSBoundParameters; exit 0 }
    catch { Write-Error -ErrorAction Continue ("Windows environment preparation failed: " + $_.Exception.Message); exit 1 }
}

param(
    [switch]$Media,
    [switch]$Check,
    [switch]$AllowSystemInstall,
    [string]$PythonExe,
    [string]$StateDir = '.yandu-storage-dev',
    [string]$BinDir = '.local/bin'
)
$ErrorActionPreference = 'Stop'
$YanduRoot = Split-Path $PSScriptRoot -Parent

function Assert-YanduWindowsHost([string]$System = $env:OS, [string]$Architecture = $env:PROCESSOR_ARCHITECTURE, [int]$ProcessBits = ([IntPtr]::Size * 8), [int]$Build = [Environment]::OSVersion.Version.Build) {
    if ($System -ne 'Windows_NT' -or $Architecture -ne 'AMD64' -or $ProcessBits -ne 64 -or $Build -lt 22000) {
        throw "Requires Windows 11+ x64 and 64-bit PowerShell; detected $System/$Architecture, $ProcessBits-bit process, build $Build. No tools were installed."
    }
}

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

function Find-YanduPython([version]$Minimum, [string]$PythonExe) {
    $script:YanduPythonProbeErrors = @()
    $candidates = @()
    if ($PythonExe) {
        if (![IO.Path]::IsPathRooted($PythonExe)) { throw '-PythonExe must be an absolute interpreter path.' }
        $candidates += @{ Path = $PythonExe; Prefix = @(); Source = 'explicit -PythonExe' }
    } else {
        if ($env:CONDA_PREFIX) {
            $candidates += @{ Path = (Join-Path $env:CONDA_PREFIX 'python.exe'); Prefix = @(); Source = 'active Conda environment' }
        }
        if ($env:CONDA_PYTHON_EXE) {
            $candidates += @{ Path = $env:CONDA_PYTHON_EXE; Prefix = @(); Source = 'Conda interpreter' }
        }
        foreach ($name in @('python.exe', 'python3.exe', 'py.exe')) {
            foreach ($command in @(Get-Command $name -CommandType Application -All -ErrorAction SilentlyContinue)) {
                if ($command.Source -like '*\Microsoft\WindowsApps\*') {
                    Write-Host "Skipping Windows App Execution Alias: $($command.Source). Use -PythonExe to select a known interpreter."
                    continue
                }
                $prefix = @()
                if ($name -eq 'py.exe') { $prefix = @('-3') }
                $candidates += @{ Path = $command.Source; Prefix = $prefix; Source = "PATH $name" }
            }
        }
    }
    $seen = @{}
    foreach ($candidate in $candidates) {
        $path = $candidate.Path
        $prefix = $candidate.Prefix
        $key = $path + '|' + ($prefix -join ' ')
        if ($seen.ContainsKey($key)) { continue }
        $seen[$key] = $true
        if (!(Test-Path -LiteralPath $path -PathType Leaf)) {
            Write-Host "Python candidate not found: $path ($($candidate.Source))"
            if ($PythonExe) { throw "Selected -PythonExe does not exist: $path" }
            continue
        }
        Write-Host "Checking Python: $path ($($candidate.Source)); required $Minimum+ x64"
        $previousErrorAction = $ErrorActionPreference
        try {
            # Quote-free Python code also survives Windows PowerShell 5.1's
            # native argument marshalling. -I avoids project/user import hooks.
            $ErrorActionPreference = 'Continue'
            $raw = @(& $path @prefix -I -c 'import sys,json; print(json.dumps(dict(version=list(sys.version_info[:3]),bits=64 if sys.maxsize>2**32 else 32,executable=sys.executable)))' 2>&1)
            $code = $LASTEXITCODE
            if ($code -ne 0) { throw "probe exited $code`: $(($raw | ForEach-Object { "$_" }) -join ' ')" }
            $info = $null
            foreach ($line in $raw) {
                try {
                    $record = "$line" | ConvertFrom-Json -ErrorAction Stop
                    if ($record.version.Count -eq 3 -and $record.executable -and $record.bits) { $info = $record }
                } catch { continue }
            }
            if (!$info) { throw "probe returned invalid metadata: $(($raw | ForEach-Object { "$_" }) -join ' ')" }
            $version = $info.version -join '.'
            Write-Host "Detected Python $version ($($info.bits)-bit): $($info.executable)"
            if ($info.bits -ne 64 -or [version]$version -lt $Minimum) {
                $reason = "requires Python $Minimum+ x64; found $version ($($info.bits)-bit)"
                Write-Host "Python candidate not selected: $reason"
                if ($PythonExe) { throw "Selected -PythonExe is incompatible: $reason" }
                continue
            }
            return @{ Path = [string]$info.executable; Prefix = @(); Version = $version; Source = $candidate.Source }
        } catch {
            $message = "Python detection failed for $path`: $($_.Exception.Message)"
            $script:YanduPythonProbeErrors += $message
            Write-Host $message
            if ($PythonExe) { throw $message }
        } finally { $ErrorActionPreference = $previousErrorAction }
    }
    return $null
}

function Find-YanduGo([version]$Minimum) {
    $script:YanduGoProbeErrors = @()
    foreach ($command in @(Get-Command 'go.exe' -CommandType Application -All -ErrorAction SilentlyContinue)) {
        Write-Host "Checking Go: $($command.Source); required $Minimum+ windows/amd64"
        $previous = $env:GOTOOLCHAIN
        $previousErrorAction = $ErrorActionPreference
        try {
            $env:GOTOOLCHAIN = 'local'
            $ErrorActionPreference = 'Continue'
            $raw = @(& $command.Source version 2>&1)
            $code = $LASTEXITCODE
            $value = ($raw | ForEach-Object { "$_" }) -join ' '
            if ($code -ne 0) { throw "probe exited $code`: $value" }
            if ($value -notmatch '^go version go(\d+\.\d+(?:\.\d+)?) (\S+)$') { throw "unrecognized version output: $value" }
            $version = $Matches[1]
            $target = $Matches[2]
            Write-Host "Detected Go $version ($target): $($command.Source)"
            if ($target -eq 'windows/amd64' -and [version]$version -ge $Minimum) {
                return @{ Path = $command.Source; Version = $version }
            }
            Write-Host "Go candidate not selected: requires Go $Minimum+ windows/amd64; found $version ($target)"
        } catch {
            $message = "Go detection failed for $($command.Source)`: $($_.Exception.Message)"
            $script:YanduGoProbeErrors += $message
            Write-Host $message
        } finally { $env:GOTOOLCHAIN = $previous; $ErrorActionPreference = $previousErrorAction }
    }
    return $null
}

function Install-YanduPackage([string]$Id, [string[]]$Extra = @(), [switch]$AllowSystemInstall) {
    if (!$AllowSystemInstall) {
        throw "Missing system tool: $Id. System installation is disabled by default. Reuse an existing tool, or explicitly pass -AllowSystemInstall."
    }
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
    if ($LASTEXITCODE -ne 0) { throw "Existing Visual Studio detection failed (exit $LASTEXITCODE); it is not treated as missing." }
    if ($location) { return "$location" }
    return $null
}

function Initialize-YanduMediaTools([switch]$AllowSystemInstall) {
    $missing = @()
    foreach ($item in @(
        @{ Command = 'git.exe'; Id = 'Git.Git' },
        @{ Command = 'cmake.exe'; Id = 'Kitware.CMake' },
        @{ Command = '7z.exe'; Id = '7zip.7zip' }
    )) {
        $command = Get-Command $item.Command -CommandType Application -ErrorAction SilentlyContinue
        if (!$command) { $missing += $item; continue }
        $arguments = @('--version')
        if ($item.Command -eq '7z.exe') { $arguments = @('i') }
        $previousErrorAction = $ErrorActionPreference
        try {
            $ErrorActionPreference = 'Continue'
            $raw = @(& $command.Source @arguments 2>&1)
            if ($LASTEXITCODE -ne 0) { throw "Existing build tool detection failed for $($command.Source) (exit $LASTEXITCODE): $($raw -join ' '). No replacement was installed." }
            Write-Host "Build tool checked: $($command.Source); $($raw | Select-Object -First 1)"
            if ($item.Command -eq 'cmake.exe') {
                if (($raw -join ' ') -notmatch 'cmake version (\d+\.\d+\.\d+)') { throw "Existing CMake returned unrecognized version metadata: $($raw -join ' '); no replacement was installed." }
                if ([version]$Matches[1] -lt [version]'3.16.3') {
                    Write-Host 'CMake 3.16.3+ is required; installed CMake is preserved unless system installation is explicitly allowed.'
                    $missing += $item
                }
            }
        } finally { $ErrorActionPreference = $previousErrorAction }
    }
    $studio = Find-YanduVisualStudio
    if (!$studio) {
        if (!$AllowSystemInstall) { throw 'Missing Visual C++ x64 build tools. Use existing tools/components, or explicitly pass -AllowSystemInstall.' }
        $vswhere = Join-Path ${env:ProgramFiles(x86)} 'Microsoft Visual Studio/Installer/vswhere.exe'
        $existing = $null
        if (Test-Path -LiteralPath $vswhere) {
            $output = & $vswhere -latest -products Microsoft.VisualStudio.Product.BuildTools -version '[17.0,)' -format json
            if ($LASTEXITCODE -ne 0) { throw "Existing Visual Studio inventory failed (exit $LASTEXITCODE); no replacement was installed." }
            $instances = $output | ConvertFrom-Json
            if ($instances) { $existing = @($instances)[0] }
        }
    }
    foreach ($item in $missing) {
        Install-YanduPackage $item.Id -AllowSystemInstall:$AllowSystemInstall
        if (!(Get-Command $item.Command -CommandType Application -ErrorAction SilentlyContinue)) { throw "$($item.Command) was not found after installation. Reopen PowerShell and retry." }
    }
    if (!$studio) {
        if ($existing) {
            $installer = Join-Path ${env:ProgramFiles(x86)} 'Microsoft Visual Studio/Installer/setup.exe'
            $arguments = 'modify --installPath "' + $existing.installationPath + '" --channelId "' + $existing.channelId + '" --add Microsoft.VisualStudio.Workload.VCTools --includeRecommended --passive --norestart'
            $process = Start-Process -FilePath $installer -ArgumentList $arguments -Verb RunAs -Wait -PassThru
            if ($process.ExitCode -notin @(0, 3010)) { throw "Visual C++ setup failed (exit $($process.ExitCode))." }
        } else {
            Install-YanduPackage 'Microsoft.VisualStudio.2022.BuildTools' @('--override', '--wait --passive --add Microsoft.VisualStudio.Workload.VCTools --includeRecommended --norestart') -AllowSystemInstall
        }
        $studio = Find-YanduVisualStudio
        if (!$studio) { throw 'Visual C++ x64 tools are not ready. Finish the installer or restart Windows if requested, then retry -Media.' }
    }
    $module = Join-Path $studio 'Common7/Tools/Microsoft.VisualStudio.DevShell.dll'
    Import-Module $module
    Enter-VsDevShell -VsInstallPath $studio -SkipAutomaticLocation -DevCmdArguments '-arch=x64 -host_arch=x64' | Out-Null
}

function Invoke-YanduPreparedToolCheck([string]$Binary, [string[]]$Arguments) {
    & $Binary @Arguments
    if ($LASTEXITCODE -ne 0) { throw 'Existing component compatibility check failed; check the executable, version, architecture and media module diagnostics above.' }
}

function Test-YanduPreparedComponents([string]$Config, [switch]$Media, [string]$BinDir = '.local/bin') {
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
    $binary = Join-Path $BinDir 'yandu.exe'
    if (![IO.Path]::IsPathRooted($binary)) { $binary = Join-Path $YanduRoot $binary }
    if (!(Test-Path -LiteralPath $binary -PathType Leaf)) { throw "Prepared Agent missing: $binary. Run setup first; -Check does not compile or download tools." }
    $arguments = @('tools', 'prepare', '--check', '--role', 'local', '--config', $Config, '--component-root', $YanduRoot)
    if (!$Media) { $arguments += '--no-media' }
    foreach ($name in $required) { $arguments += @('--tool', ($name + '=' + $record.tools.$name.path)) }
    Invoke-YanduPreparedToolCheck $binary $arguments
}

function Invoke-YanduWindowsSetup {
    param([switch]$Media, [switch]$Check, [switch]$AllowSystemInstall, [string]$PythonExe, [string]$StateDir = '.yandu-storage-dev', [string]$BinDir = '.local/bin')
    Assert-YanduWindowsHost
    if ($Check -and $AllowSystemInstall) { throw '-Check cannot be combined with -AllowSystemInstall; no system tools were installed.' }
    if ($AllowSystemInstall) { Write-Host 'System installation explicitly enabled: installers may update system packages, dependencies and PATH; compatible existing runtimes are reused.' } else { Write-Host 'System installation disabled; reusing existing runtimes and preparing project-local components only.' }
    Update-YanduProcessPath
    $goLine = Select-String -LiteralPath (Join-Path $YanduRoot 'go.mod') -Pattern '^go\s+(\d+\.\d+(?:\.\d+)?)\s*$'
    if (!$goLine) { throw 'go.mod is missing the minimum Go version.' }
    $goMinimum = [version]$goLine.Matches[0].Groups[1].Value
    $pythonMinimum = [version]'3.10.0'
    if ($Media) { $pythonMinimum = [version]'3.12.0' }
    $python = Find-YanduPython $pythonMinimum -PythonExe $PythonExe
    $go = Find-YanduGo $goMinimum
    if (!$python -and $script:YanduPythonProbeErrors.Count -gt 0) { throw 'Existing Python detection failed; it is not treated as missing. Check the diagnostics above or select a known interpreter with -PythonExe. No Python installation was attempted.' }
    if (!$go -and $script:YanduGoProbeErrors.Count -gt 0) { throw 'Existing Go detection failed; it is not treated as missing. Check the diagnostics above. No Go installation was attempted.' }
    if (!$python -and !$Check -and $AllowSystemInstall) {
        Install-YanduPackage 'Python.Python.3.14' -AllowSystemInstall
        $python = Find-YanduPython $pythonMinimum
    }
    if (!$python) { throw "Python $pythonMinimum+ x64 is required. Use -PythonExe for an existing interpreter. System installation requires explicit -AllowSystemInstall." }
    if (!$go -and !$Check -and $AllowSystemInstall) { Install-YanduPackage 'GoLang.Go' -AllowSystemInstall; $go = Find-YanduGo $goMinimum }
    if (!$go) { throw "Go $goMinimum+ is required. Reuse an installed Go, or explicitly pass -AllowSystemInstall." }
    Write-Host "Python $($python.Version): $($python.Path)"
    Write-Host "Go $($go.Version): $($go.Path)"
    $statePath = $StateDir
    if (![IO.Path]::IsPathRooted($statePath)) { $statePath = Join-Path $YanduRoot $statePath }
    $state = [IO.Path]::GetFullPath($statePath)
    if ($Check) { Test-YanduPreparedComponents (Join-Path $state 'tools.json') -Media:$Media -BinDir $BinDir; Write-Host 'Windows runtime and prepared component checks passed.'; return }
    if ($Media -and $AllowSystemInstall) {
        $preparedMedia = $false
        try { Test-YanduPreparedComponents (Join-Path $state 'tools.json') -Media -BinDir $BinDir; $preparedMedia = $true } catch { }
        if (!$preparedMedia) { Initialize-YanduMediaTools -AllowSystemInstall }
    }
    # dev.py must use the qualified Go rather than an older PATH entry.
    $env:PATH = (Split-Path $go.Path -Parent) + ';' + $env:PATH
    $env:GOTOOLCHAIN = 'local'
    $prefix = $python.Prefix
    $arguments = @((Join-Path $YanduRoot 'scripts/storage-dev.py'), '--prepare-only', '--state-dir', $StateDir, '--bin-dir', $BinDir)
    if (!$Media) { $arguments += '--no-media' }
    & $python.Path @prefix @arguments
    if ($LASTEXITCODE -ne 0) { throw "Dependency preparation failed (exit $LASTEXITCODE). Check the component error above. Existing tools and verified caches are reused; missing native build tools can be installed explicitly with -AllowSystemInstall." }
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

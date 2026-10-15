$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'setup-windows-env.ps1')
$temporary = Join-Path ([IO.Path]::GetTempPath()) ('yandu env spaces ' + [Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $temporary | Out-Null
$previousOS = $env:OS
$previousArchitecture = $env:PROCESSOR_ARCHITECTURE
$previousCondaPrefix = $env:CONDA_PREFIX
$previousCondaPython = $env:CONDA_PYTHON_EXE
$previousPath = $env:PATH
$previousGoToolchain = $env:GOTOOLCHAIN
$env:OS = 'Windows_NT'
$env:PROCESSOR_ARCHITECTURE = 'AMD64'
$env:CONDA_PREFIX = $null
$env:CONDA_PYTHON_EXE = $null

function Assert-Yandu([bool]$Condition, [string]$Message) {
    if (!$Condition) { throw "Assertion failed: $Message" }
}

try {
    $mediaToolsImplementation = ${function:Initialize-YanduMediaTools}
    Assert-YanduWindowsHost -System Windows_NT -Architecture AMD64 -ProcessBits 64 -Build 26100
    foreach ($facts in @(
        @{System='Windows_NT';Architecture='ARM64';ProcessBits=64;Build=26100},
        @{System='Windows_NT';Architecture='AMD64';ProcessBits=32;Build=26100},
        @{System='Windows_NT';Architecture='AMD64';ProcessBits=64;Build=19045}
    )) {
        $failed = $false
        try { Assert-YanduWindowsHost @facts } catch { $failed = $true }
        Assert-Yandu $failed 'unsupported Windows architecture, process or OS must be rejected'
    }
    function Assert-YanduWindowsHost { }
    function Invoke-YanduPreparedToolCheck {
        param([string]$Binary, [string[]]$Arguments)
        $global:YanduTestCompatibilityArguments = $Arguments
    }
    $pythonFixture = Join-Path $temporary 'python fixture.ps1'
    $goFixture = Join-Path $temporary 'go fixture.ps1'
    $wingetFixture = Join-Path $temporary 'winget fixture.ps1'
    $brokenBuildFixture = Join-Path $temporary 'broken compiler tool.ps1'
    @'
param([Parameter(ValueFromRemainingArguments=$true)][string[]]$Arguments)
$global:LASTEXITCODE = 1
Write-Output 'existing build tool cannot load its runtime library'
'@ | Set-Content -LiteralPath $brokenBuildFixture -Encoding utf8
    @'
param([switch]$I, [Parameter(ValueFromRemainingArguments=$true)][string[]]$Arguments)
$global:LASTEXITCODE = 0
if ($Arguments -contains '-c') {
    if ($global:YanduTestPythonProbeFailure) { Write-Output 'probe execution failed'; $global:LASTEXITCODE = 1; return }
    @{version = @($global:YanduTestPythonVersion.Split('.') | ForEach-Object { [int]$_ }); bits = $global:YanduTestPythonBits; executable = $PSCommandPath} | ConvertTo-Json -Compress
    return
}
$global:YanduTestPrepareArguments = @($Arguments)
'@ | Set-Content -LiteralPath $pythonFixture -Encoding utf8
    @'
param([Parameter(ValueFromRemainingArguments=$true)][string[]]$Arguments)
$global:LASTEXITCODE = 0
if ($global:YanduTestGoProbeFailure) { Write-Output 'go probe execution failed'; $global:LASTEXITCODE = 1; return }
Write-Output ('go version go' + $global:YanduTestGoVersion + ' windows/amd64')
'@ | Set-Content -LiteralPath $goFixture -Encoding utf8
    @'
param([Parameter(ValueFromRemainingArguments=$true)][string[]]$Arguments)
$global:YanduTestPackages += $Arguments[2]
$global:YanduTestWingetArguments = @($Arguments)
$global:LASTEXITCODE = $global:YanduTestWingetExit
if ($global:LASTEXITCODE -eq 0) {
    if ($Arguments[2] -eq 'GoLang.Go') { $global:YanduTestGoAvailable = $true; $global:YanduTestGoVersion = '1.99.0' }
    if ($Arguments[2] -eq 'Python.Python.3.14') { $global:YanduTestPythonAvailable = $true; $global:YanduTestPythonVersion = '3.14.0'; $global:YanduTestPythonBits = 64 }
}
'@ | Set-Content -LiteralPath $wingetFixture -Encoding utf8
    function Get-Command {
        param([string]$Name, [string]$CommandType, [switch]$All, [string]$ErrorAction)
        switch ($Name) {
            'python.exe' { if ($global:YanduTestPythonAvailable) { return [pscustomobject]@{Source = $pythonFixture} } }
            'go.exe' { if ($global:YanduTestGoAvailable) { return [pscustomobject]@{Source = $goFixture} } }
            'winget.exe' { return [pscustomobject]@{Source = $wingetFixture} }
            'git.exe' { if ($global:YanduTestBrokenBuildTool) { return [pscustomobject]@{Source = $brokenBuildFixture} } }
        }
    }
    function Update-YanduProcessPath { }
    function Initialize-YanduMediaTools {
        param([switch]$AllowSystemInstall)
        $global:YanduTestMedia = $true
    }
    function Reset-YanduFixture {
        $global:YanduTestPythonVersion = '3.13.12'
        $global:YanduTestPythonBits = 64
        $global:YanduTestPythonProbeFailure = $false
        $global:YanduTestGoVersion = '1.99.0'
        $global:YanduTestGoProbeFailure = $false
        $global:YanduTestPythonAvailable = $true
        $global:YanduTestGoAvailable = $true
        $global:YanduTestPrepareArguments = @()
        $global:YanduTestPackages = @()
        $global:YanduTestWingetArguments = @()
        $global:YanduTestWingetExit = 0
        $global:YanduTestMedia = $false
        $global:YanduTestBrokenBuildTool = $false
    }

    Reset-YanduFixture
    Invoke-YanduWindowsSetup -StateDir 'state spaces & literal' -BinDir 'bin spaces'
    Assert-Yandu ($global:YanduTestPackages.Count -eq 0) 'newer runtimes should be reused'
    Assert-Yandu ($global:YanduTestPrepareArguments -contains '--prepare-only') 'setup must prepare without launching services'
    Assert-Yandu ($global:YanduTestPrepareArguments -contains '--no-media') 'default setup must only prepare file tools'
    Assert-Yandu ($global:YanduTestPrepareArguments -contains 'state spaces & literal') 'argument boundaries were lost'

    Reset-YanduFixture
    $global:YanduTestGoVersion = '1.25.0'
    $failed = $false
    try { Invoke-YanduWindowsSetup } catch { $failed = $true }
    Assert-Yandu ($failed -and $global:YanduTestPackages.Count -eq 0 -and $global:YanduTestPrepareArguments.Count -eq 0) 'default setup must not upgrade an old Go'
    Invoke-YanduWindowsSetup -AllowSystemInstall
    Assert-Yandu ($global:YanduTestPackages.Count -eq 1 -and $global:YanduTestPackages[0] -eq 'GoLang.Go') 'only obsolete Go should be upgraded'

    Reset-YanduFixture
    $global:YanduTestPythonAvailable = $false
    $failed = $false
    try { Invoke-YanduWindowsSetup } catch { $failed = $true }
    Assert-Yandu ($failed -and $global:YanduTestPackages.Count -eq 0) 'default setup must not install a global Python'
    Invoke-YanduWindowsSetup -AllowSystemInstall
    Assert-Yandu ($global:YanduTestPackages.Count -eq 1 -and $global:YanduTestPackages[0] -eq 'Python.Python.3.14') 'only missing Python should be installed'

    Reset-YanduFixture
    $global:YanduTestGoAvailable = $false
    $failed = $false
    try { Invoke-YanduWindowsSetup -Check } catch { $failed = $true }
    Assert-Yandu ($failed -and $global:YanduTestPackages.Count -eq 0 -and $global:YanduTestPrepareArguments.Count -eq 0) 'check must not install or prepare missing tools'

    Reset-YanduFixture
    $global:YanduTestGoAvailable = $false
    $global:YanduTestWingetExit = 23
    $failed = $false
    try { Invoke-YanduWindowsSetup -AllowSystemInstall } catch { $failed = $true }
    Assert-Yandu ($failed -and $global:YanduTestPrepareArguments.Count -eq 0) 'failed WinGet must stop before component preparation'

    Reset-YanduFixture
    $failed = $false
    try { Install-YanduPackage 'GoLang.Go' } catch { $failed = $true }
    Assert-Yandu ($failed -and $global:YanduTestPackages.Count -eq 0) 'package installer requires explicit opt-in at its own boundary'
    Install-YanduPackage 'Microsoft.VisualStudio.2022.BuildTools' @('--override', '--wait --passive --add Microsoft.VisualStudio.Workload.VCTools --includeRecommended --norestart') -AllowSystemInstall
    Assert-Yandu ($global:YanduTestWingetArguments -contains '--override') 'Visual Studio override flag lost'
    Assert-Yandu ($global:YanduTestWingetArguments -contains '--wait --passive --add Microsoft.VisualStudio.Workload.VCTools --includeRecommended --norestart') 'Visual Studio override was split into multiple arguments'

    Reset-YanduFixture
    $global:YanduTestBrokenBuildTool = $true
    $failed = $false
    try { & $mediaToolsImplementation -AllowSystemInstall } catch { $failed = $true }
    Assert-Yandu ($failed -and $global:YanduTestPackages.Count -eq 0) 'an existing build tool probe error must stop before installing any missing media prerequisites'

    Reset-YanduFixture
    $global:YanduTestPythonVersion = '3.11.9'
    $failed = $false
    try { Invoke-YanduWindowsSetup -Media } catch { $failed = $true }
    Assert-Yandu ($failed -and $global:YanduTestPackages.Count -eq 0) 'media must report old Python without installing by default'
    Invoke-YanduWindowsSetup -Media -AllowSystemInstall -StateDir (Join-Path $temporary 'fresh media')
    Assert-Yandu ($global:YanduTestPackages -contains 'Python.Python.3.14') 'media requires Python 3.12 or newer'
    Assert-Yandu ($global:YanduTestMedia -and !($global:YanduTestPrepareArguments -contains '--no-media')) 'media setup must prepare native media tools'

    Reset-YanduFixture
    Invoke-YanduWindowsSetup -Media -PythonExe $pythonFixture
    Assert-Yandu ($global:YanduTestPackages.Count -eq 0 -and !$global:YanduTestMedia) 'qualified Python 3.13 x64 and media must not install or modify system build tools by default'

    Reset-YanduFixture
    $global:YanduTestPythonVersion = '3.11.9'
    $failed = $false
    try { Invoke-YanduWindowsSetup -Media -PythonExe $pythonFixture -AllowSystemInstall } catch { $failed = $true }
    Assert-Yandu ($failed -and $global:YanduTestPackages.Count -eq 0) 'an incompatible explicitly selected Python must stop without installing a replacement'

    Reset-YanduFixture
    $failed = $false
    try { Invoke-YanduWindowsSetup -PythonExe (Join-Path $temporary 'missing python.exe') -AllowSystemInstall } catch { $failed = $true }
    Assert-Yandu ($failed -and $global:YanduTestPackages.Count -eq 0) 'a missing explicit Python must not fall back to another interpreter or installation'

    Reset-YanduFixture
    $global:YanduTestPythonProbeFailure = $true
    $failed = $false
    try { Invoke-YanduWindowsSetup -Media -AllowSystemInstall } catch { $failed = $true }
    Assert-Yandu ($failed -and $global:YanduTestPackages.Count -eq 0) 'Python probe errors must never be interpreted as absence'

    Reset-YanduFixture
    $global:YanduTestGoProbeFailure = $true
    $failed = $false
    try { Invoke-YanduWindowsSetup -AllowSystemInstall } catch { $failed = $true }
    Assert-Yandu ($failed -and $global:YanduTestPackages.Count -eq 0) 'Go probe errors must never be interpreted as absence'

    Reset-YanduFixture
    $global:YanduTestPythonAvailable = $false
    $global:YanduTestGoProbeFailure = $true
    $failed = $false
    try { Invoke-YanduWindowsSetup -AllowSystemInstall } catch { $failed = $true }
    Assert-Yandu ($failed -and $global:YanduTestPackages.Count -eq 0) 'all runtime probes must finish before even an explicitly allowed Python installation'

    Reset-YanduFixture
    $failed = $false
    try { Invoke-YanduWindowsSetup -Check -AllowSystemInstall } catch { $failed = $true }
    Assert-Yandu ($failed -and $global:YanduTestPackages.Count -eq 0) 'check cannot enable system installation'

    Reset-YanduFixture
    $global:YanduTestPythonBits = 32
    $failed = $false
    try { Invoke-YanduWindowsSetup -Media } catch { $failed = $true }
    Assert-Yandu ($failed -and $global:YanduTestPackages.Count -eq 0) '32-bit Python must be reported without silent installation'

    $componentRoot = Join-Path $temporary 'components'
    New-Item -ItemType Directory -Path (Join-Path $componentRoot 'templates/email') -Force | Out-Null
    $preparedBin = Join-Path $temporary 'prepared bin'
    New-Item -ItemType Directory -Path $preparedBin | Out-Null
    Set-Content -LiteralPath (Join-Path $preparedBin 'yandu.exe') -Value 'Agent fixture'
    $tools = @{}
    foreach ($name in @('caddy', 'frpc', 'sftpgo')) {
        $file = Join-Path $componentRoot ($name + '.exe')
        Set-Content -LiteralPath $file -Value $name
        $tools[$name] = @{ path = $file; sha256 = (Get-FileHash -LiteralPath $file -Algorithm SHA256).Hash }
    }
    $config = Join-Path $componentRoot 'tools.json'
    @{schemaVersion = 1; role = 'local'; tools = $tools; sftpgoTemplates = (Join-Path $componentRoot 'templates')} | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath $config
    Test-YanduPreparedComponents $config -BinDir $preparedBin
    Assert-Yandu ($global:YanduTestCompatibilityArguments -contains '--check' -and $global:YanduTestCompatibilityArguments -contains ('sftpgo=' + $tools.sftpgo.path)) 'check must execute registered components with explicit paths, not just hash their files'
    Reset-YanduFixture
    Invoke-YanduWindowsSetup -Check -StateDir $componentRoot -BinDir $preparedBin
    Assert-Yandu ($global:YanduTestPackages.Count -eq 0 -and $global:YanduTestPrepareArguments.Count -eq 0) 'successful check must not install or prepare'
    foreach ($name in @('node', 'ffmpeg', 'ffprobe', 'heif-thumbnailer')) {
        $file = Join-Path $componentRoot ($name + '.exe')
        Set-Content -LiteralPath $file -Value $name
        $tools[$name] = @{ path = $file; sha256 = (Get-FileHash -LiteralPath $file -Algorithm SHA256).Hash }
    }
    @{schemaVersion = 1; role = 'local'; tools = $tools; sftpgoTemplates = (Join-Path $componentRoot 'templates')} | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath $config
    Reset-YanduFixture
    Invoke-YanduWindowsSetup -Media -AllowSystemInstall -StateDir $componentRoot -BinDir $preparedBin
    Assert-Yandu ($global:YanduTestPackages.Count -eq 0 -and !$global:YanduTestMedia) 'verified media components must skip system build tool installation even when allowed'
    Set-Content -LiteralPath $tools.sftpgo.path -Value 'changed'
    $failed = $false
    try { Test-YanduPreparedComponents $config } catch { $failed = $true }
    Assert-Yandu $failed 'changed component must not pass hash checks'
    Write-Host 'Windows environment orchestration tests passed (package manager and Windows installers mocked).'
} finally {
    $env:OS = $previousOS
    $env:PROCESSOR_ARCHITECTURE = $previousArchitecture
    $env:CONDA_PREFIX = $previousCondaPrefix
    $env:CONDA_PYTHON_EXE = $previousCondaPython
    $env:PATH = $previousPath
    $env:GOTOOLCHAIN = $previousGoToolchain
    Remove-Item -LiteralPath $temporary -Recurse -Force
}

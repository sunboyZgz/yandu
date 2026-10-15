$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'setup-windows-env.ps1')
$temporary = Join-Path ([IO.Path]::GetTempPath()) ('yandu env spaces ' + [Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $temporary | Out-Null
$previousOS = $env:OS
$previousArchitecture = $env:PROCESSOR_ARCHITECTURE
$env:OS = 'Windows_NT'
$env:PROCESSOR_ARCHITECTURE = 'AMD64'

function Assert-Yandu([bool]$Condition, [string]$Message) {
    if (!$Condition) { throw "Assertion failed: $Message" }
}

try {
    $pythonFixture = Join-Path $temporary 'python fixture.ps1'
    $goFixture = Join-Path $temporary 'go fixture.ps1'
    $wingetFixture = Join-Path $temporary 'winget fixture.ps1'
    @'
param([Parameter(ValueFromRemainingArguments=$true)][string[]]$Arguments)
$global:LASTEXITCODE = 0
if ($Arguments -contains '-c') { Write-Output $global:YanduTestPythonVersion; return }
$global:YanduTestPrepareArguments = @($Arguments)
'@ | Set-Content -LiteralPath $pythonFixture -Encoding utf8
    @'
param([Parameter(ValueFromRemainingArguments=$true)][string[]]$Arguments)
$global:LASTEXITCODE = 0
Write-Output ('go version go' + $global:YanduTestGoVersion + ' windows/amd64')
'@ | Set-Content -LiteralPath $goFixture -Encoding utf8
    @'
param([Parameter(ValueFromRemainingArguments=$true)][string[]]$Arguments)
$global:YanduTestPackages += $Arguments[2]
$global:YanduTestWingetArguments = @($Arguments)
$global:LASTEXITCODE = $global:YanduTestWingetExit
if ($global:LASTEXITCODE -eq 0) {
    if ($Arguments[2] -eq 'GoLang.Go') { $global:YanduTestGoAvailable = $true; $global:YanduTestGoVersion = '1.99.0' }
    if ($Arguments[2] -eq 'Python.Python.3.14') { $global:YanduTestPythonAvailable = $true; $global:YanduTestPythonVersion = '3.14.0' }
}
'@ | Set-Content -LiteralPath $wingetFixture -Encoding utf8
    function Get-Command {
        param([string]$Name, [string]$CommandType, [switch]$All, [string]$ErrorAction)
        switch ($Name) {
            'python.exe' { if ($global:YanduTestPythonAvailable) { return [pscustomobject]@{Source = $pythonFixture} } }
            'go.exe' { if ($global:YanduTestGoAvailable) { return [pscustomobject]@{Source = $goFixture} } }
            'winget.exe' { return [pscustomobject]@{Source = $wingetFixture} }
        }
    }
    function Update-YanduProcessPath { }
    function Initialize-YanduMediaTools {
        param([switch]$CheckOnly)
        $global:YanduTestMedia = $true
    }
    function Reset-YanduFixture {
        $global:YanduTestPythonVersion = '3.14.1'
        $global:YanduTestGoVersion = '1.99.0'
        $global:YanduTestPythonAvailable = $true
        $global:YanduTestGoAvailable = $true
        $global:YanduTestPrepareArguments = @()
        $global:YanduTestPackages = @()
        $global:YanduTestWingetArguments = @()
        $global:YanduTestWingetExit = 0
        $global:YanduTestMedia = $false
    }

    Reset-YanduFixture
    Invoke-YanduWindowsSetup -StateDir 'state spaces & literal' -BinDir 'bin spaces'
    Assert-Yandu ($global:YanduTestPackages.Count -eq 0) 'newer runtimes should be reused'
    Assert-Yandu ($global:YanduTestPrepareArguments -contains '--prepare-only') 'setup must prepare without launching services'
    Assert-Yandu ($global:YanduTestPrepareArguments -contains '--no-media') 'default setup must only prepare file tools'
    Assert-Yandu ($global:YanduTestPrepareArguments -contains 'state spaces & literal') 'argument boundaries were lost'

    Reset-YanduFixture
    $global:YanduTestGoVersion = '1.25.0'
    Invoke-YanduWindowsSetup
    Assert-Yandu ($global:YanduTestPackages.Count -eq 1 -and $global:YanduTestPackages[0] -eq 'GoLang.Go') 'only obsolete Go should be upgraded'

    Reset-YanduFixture
    $global:YanduTestPythonAvailable = $false
    Invoke-YanduWindowsSetup
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
    try { Invoke-YanduWindowsSetup } catch { $failed = $true }
    Assert-Yandu ($failed -and $global:YanduTestPrepareArguments.Count -eq 0) 'failed WinGet must stop before component preparation'

    Reset-YanduFixture
    Install-YanduPackage 'Microsoft.VisualStudio.2022.BuildTools' @('--override', '--wait --passive --add Microsoft.VisualStudio.Workload.VCTools --includeRecommended --norestart')
    Assert-Yandu ($global:YanduTestWingetArguments -contains '--override') 'Visual Studio override flag lost'
    Assert-Yandu ($global:YanduTestWingetArguments -contains '--wait --passive --add Microsoft.VisualStudio.Workload.VCTools --includeRecommended --norestart') 'Visual Studio override was split into multiple arguments'

    Reset-YanduFixture
    $global:YanduTestPythonVersion = '3.11.9'
    Invoke-YanduWindowsSetup -Media
    Assert-Yandu ($global:YanduTestPackages -contains 'Python.Python.3.14') 'media requires Python 3.12 or newer'
    Assert-Yandu ($global:YanduTestMedia -and !($global:YanduTestPrepareArguments -contains '--no-media')) 'media setup must prepare native media tools'

    $componentRoot = Join-Path $temporary 'components'
    New-Item -ItemType Directory -Path (Join-Path $componentRoot 'templates/email') -Force | Out-Null
    $tools = @{}
    foreach ($name in @('caddy', 'frpc', 'sftpgo')) {
        $file = Join-Path $componentRoot ($name + '.exe')
        Set-Content -LiteralPath $file -Value $name
        $tools[$name] = @{ path = $file; sha256 = (Get-FileHash -LiteralPath $file -Algorithm SHA256).Hash }
    }
    $config = Join-Path $componentRoot 'tools.json'
    @{schemaVersion = 1; role = 'local'; tools = $tools; sftpgoTemplates = (Join-Path $componentRoot 'templates')} | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath $config
    Test-YanduPreparedComponents $config
    Reset-YanduFixture
    Invoke-YanduWindowsSetup -Check -StateDir $componentRoot
    Assert-Yandu ($global:YanduTestPackages.Count -eq 0 -and $global:YanduTestPrepareArguments.Count -eq 0) 'successful check must not install or prepare'
    Set-Content -LiteralPath $tools.sftpgo.path -Value 'changed'
    $failed = $false
    try { Test-YanduPreparedComponents $config } catch { $failed = $true }
    Assert-Yandu $failed 'changed component must not pass hash checks'
    Write-Host 'Windows environment orchestration tests passed (package manager and Windows installers mocked).'
} finally {
    $env:OS = $previousOS
    $env:PROCESSOR_ARCHITECTURE = $previousArchitecture
    Remove-Item -LiteralPath $temporary -Recurse -Force
}

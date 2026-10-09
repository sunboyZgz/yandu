#Requires -RunAsAdministrator
param([string]$StatePath="$env:ProgramData\Yandu")
$ErrorActionPreference='Stop'
$Record=Get-Content "$StatePath\installation.json" -Raw | ConvertFrom-Json
& "$PSScriptRoot\install-windows.ps1" -InstallPath $Record.installPath -StatePath $StatePath -SkipOpen

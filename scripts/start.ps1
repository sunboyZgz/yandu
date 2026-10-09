param([string]$StatePath="$env:ProgramData\Yandu")
$ErrorActionPreference='Stop'
& "$PSScriptRoot\yandu.exe" --state-dir $StatePath ui
if($LASTEXITCODE -ne 0){throw '启动失败，请检查服务和本地日志'}

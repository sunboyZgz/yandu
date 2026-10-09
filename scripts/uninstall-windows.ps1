#Requires -RunAsAdministrator
param([string]$StatePath="$env:ProgramData\Yandu")
$ErrorActionPreference='Stop'
$Record=Join-Path $StatePath 'installation.json'
if(-not(Test-Path $Record)){throw '没有安装归属记录，拒绝删除未知目录或服务'}
$Install=Get-Content $Record -Raw | ConvertFrom-Json
if($Install.service -ne 'Yandu' -or [IO.Path]::GetFullPath($Install.installPath) -ne [IO.Path]::GetFullPath("$env:ProgramFiles\Yandu")){throw '安装归属记录无效，拒绝删除目录'}
Stop-Service Yandu -ErrorAction SilentlyContinue
& sc.exe delete Yandu | Out-Null
if($Install.pathEntryAdded){$MachinePath=[Environment]::GetEnvironmentVariable('Path','Machine');$NewPath=($MachinePath.Split(';') | Where-Object {$_ -ne $Install.installPath}) -join ';';[Environment]::SetEnvironmentVariable('Path',$NewPath,'Machine')}
Remove-Item $Install.installPath -Recurse -Force
$Shortcut=[Environment]::GetFolderPath('Desktop')+'\檐渡.lnk'
if(Test-Path $Shortcut){Remove-Item $Shortcut}
Write-Host "已卸载程序和服务。保留 $StatePath 及所有业务素材。请先停用项目；离线卸载后云端拒绝规则需由管理员核对。"

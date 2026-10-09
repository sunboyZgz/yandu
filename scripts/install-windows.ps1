#Requires -RunAsAdministrator
param([string]$ProfilePath,[string]$InstallPath="$env:ProgramFiles\Yandu",[string]$StatePath="$env:ProgramData\Yandu",[switch]$SkipOpen)
$ErrorActionPreference='Stop'
if (-not [Environment]::Is64BitOperatingSystem) { throw '需要 Windows x64' }
$Source=$PSScriptRoot
$PreexistingService=Get-Service -Name Yandu -ErrorAction SilentlyContinue
if(($PreexistingService -or (Test-Path "$InstallPath\yandu.exe")) -and -not (Test-Path "$StatePath\installation.json")){throw '检测到同名服务或程序但缺少安装归属记录，拒绝覆盖'}
if([IO.Path]::GetFullPath($InstallPath) -ne [IO.Path]::GetFullPath("$env:ProgramFiles\Yandu")){throw '首版仅安装到 Program Files\Yandu，避免覆盖素材或其他应用目录'}
foreach($file in @('yandu.exe','bin\caddy.exe','bin\frpc.exe')) { if(-not (Test-Path (Join-Path $Source $file))) { throw "发布包缺少 $file" } }
New-Item -ItemType Directory -Path $InstallPath,$StatePath -Force | Out-Null
$User=[System.Security.Principal.WindowsIdentity]::GetCurrent().User.Value
$OwnedPathAdded=$false
if(Test-Path "$StatePath\installation.json"){$Previous=Get-Content "$StatePath\installation.json" -Raw | ConvertFrom-Json;$OwnedPathAdded=[bool]$Previous.pathEntryAdded}
# Only the installing desktop user, LocalService, Administrators and SYSTEM can access state.
& icacls.exe $StatePath /inheritance:r /grant:r '*S-1-5-18:(OI)(CI)F' '*S-1-5-32-544:(OI)(CI)F' '*S-1-5-19:(OI)(CI)M' "*$($User):(OI)(CI)M" | Out-Null
if($LASTEXITCODE -ne 0){throw '状态目录 ACL 设置失败'}
$Existing=Get-Service -Name Yandu -ErrorAction SilentlyContinue
if($Existing){Stop-Service Yandu -Force; $Existing.WaitForStatus('Stopped',[TimeSpan]::FromSeconds(20))}
$Stamp=Get-Date -Format yyyyMMdd-HHmmss
$Backup=Join-Path $StatePath "backups\$Stamp"
New-Item -ItemType Directory -Path $Backup -Force | Out-Null
if(Test-Path "$StatePath\yandu.db") { Copy-Item "$StatePath\yandu.db*" $Backup }
if(Test-Path "$InstallPath\yandu.exe") { Copy-Item "$InstallPath\yandu.exe" "$Backup\yandu.exe"; if(Test-Path "$InstallPath\bin"){Copy-Item "$InstallPath\bin" "$Backup\bin" -Recurse} }
try {
 Copy-Item "$Source\yandu.exe" "$InstallPath\yandu.exe" -Force
 New-Item -ItemType Directory -Path "$InstallPath\bin" -Force | Out-Null
 Copy-Item "$Source\bin\*" "$InstallPath\bin" -Force
 Copy-Item "$Source\start.ps1" "$InstallPath\start.ps1" -Force
 $Bin='"'+$InstallPath+'\yandu.exe" --state-dir "'+$StatePath+'" serve'
 if(-not $Existing){& sc.exe create Yandu binPath= $Bin start= auto obj= 'NT AUTHORITY\LocalService' DisplayName= '檐渡 · 本地资源服务' | Out-Null; if($LASTEXITCODE -ne 0){throw '服务创建失败'}}
 else { & sc.exe config Yandu binPath= $Bin start= auto | Out-Null; if($LASTEXITCODE -ne 0){throw '服务配置更新失败'} }
 & sc.exe failure Yandu reset= 86400 actions= restart/5000/restart/15000/restart/60000 | Out-Null
 Start-Service Yandu
 $Ready=$false
 for($i=0;$i -lt 60;$i++){try{$Health=Invoke-RestMethod 'http://127.0.0.1:18765/api/v1/health';if($Health.version -eq '0.1.0'){$Ready=$true;break}}catch{};Start-Sleep -Milliseconds 500}
 if(-not $Ready){throw 'Agent 健康检查失败'}
 $Runtime=& "$InstallPath\yandu.exe" --state-dir $StatePath status | ConvertFrom-Json
 if(-not $Runtime.components.fileService){throw 'Caddy 文件服务未启动，请检查组件或端口'}
 if($ProfilePath){& "$InstallPath\yandu.exe" --state-dir $StatePath connection import $ProfilePath;if($LASTEXITCODE -ne 0){throw '连接包导入失败'}}
 $Shell=New-Object -ComObject WScript.Shell
 $Shortcut=$Shell.CreateShortcut([Environment]::GetFolderPath('Desktop')+'\檐渡.lnk')
 $Shortcut.TargetPath="$InstallPath\yandu.exe";$Shortcut.Arguments='--state-dir "'+$StatePath+'" ui';$Shortcut.WorkingDirectory=$InstallPath;$Shortcut.Save()
 $MachinePath=[Environment]::GetEnvironmentVariable('Path','Machine')
 if(-not ($MachinePath.Split(';') -contains $InstallPath)){[Environment]::SetEnvironmentVariable('Path',($MachinePath.TrimEnd(';')+';'+$InstallPath),'Machine');$OwnedPathAdded=$true}
 @{version='0.1.0';installPath=$InstallPath;statePath=$StatePath;userSid=$User;service='Yandu';pathEntryAdded=$OwnedPathAdded} | ConvertTo-Json | Set-Content "$StatePath\installation.json" -Encoding UTF8
 if(-not $SkipOpen){& "$InstallPath\yandu.exe" --state-dir $StatePath ui}
 Write-Host '檐渡已安装。关闭浏览器不会停止服务。资源目录需单独授权并授予 LocalService 读取权限。'
} catch {
 Stop-Service Yandu -ErrorAction SilentlyContinue
 if(Test-Path "$Backup\yandu.exe") {Copy-Item "$Backup\yandu.exe" "$InstallPath\yandu.exe" -Force;Copy-Item "$Backup\bin\*" "$InstallPath\bin" -Force;Remove-Item "$StatePath\yandu.db*" -Force -ErrorAction SilentlyContinue;Copy-Item "$Backup\yandu.db*" $StatePath -ErrorAction SilentlyContinue;Start-Service Yandu}
 throw
}

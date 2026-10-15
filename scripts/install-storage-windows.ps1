#Requires -Version 5.1
#Requires -RunAsAdministrator
param([string]$InstallPath="$env:ProgramFiles\Yandu",[string]$StatePath="$env:ProgramData\Yandu",[switch]$SkipOpen)
$ErrorActionPreference='Stop'
if($env:OS -ne 'Windows_NT' -or $env:PROCESSOR_ARCHITECTURE -ne 'AMD64' -or -not [Environment]::Is64BitProcess -or [Environment]::OSVersion.Version.Build -lt 22000){throw '需要 Windows 11+ x64 和 64 位 PowerShell；未修改系统'}
$Source=$PSScriptRoot
if([IO.Path]::GetFullPath($InstallPath) -ne [IO.Path]::GetFullPath("$env:ProgramFiles\Yandu")){throw '使用受管 Program Files\Yandu 目录'}
foreach($file in @('yandu.exe','bin\sftpgo.exe','bin\node.exe','bin\ffmpeg.exe','bin\ffprobe.exe','bin\heif-thumbnailer.exe','bin\caddy.exe','bin\frpc.exe','media\worker.mjs','component-receipt.json')){if(-not (Test-Path "$Source\$file")){throw "完整原生组件包缺少 $file"}}
& "$Source\yandu.exe" storage-components-verify --component-root $Source
if($LASTEXITCODE -ne 0){throw '原生组件 SHA256 校验失败'}
$ComponentArguments=@('tools','prepare','--check','--role','local','--component-root',$Source,'--config',"$Source\.yandu-preflight.json")
foreach($name in @('caddy','frpc','sftpgo','node','ffmpeg','ffprobe','heif-thumbnailer')){$ComponentArguments+=@('--tool',"$name=$Source\bin\$name.exe")}
& "$Source\yandu.exe" @ComponentArguments
if($LASTEXITCODE -ne 0){throw '组件无法在当前系统运行；未修改服务或安装目录'}
foreach($directory in @($InstallPath,$StatePath)){
 $root=[IO.Path]::GetPathRoot([IO.Path]::GetFullPath($directory))
 if($root -notmatch '^[A-Za-z]:\\$'){throw '安装与状态目录需要本机固定 NTFS 卷'}
 $volume=Get-Volume -DriveLetter $root.Substring(0,1) -ErrorAction Stop
 if($volume.FileSystem -ne 'NTFS' -or $volume.DriveType -ne 'Fixed'){throw "安装与状态目录需要固定 NTFS 卷：$directory"}
}
Write-Host 'Installation changes: Program Files/ProgramData Yandu directories, Yandu/YanduSFTPGo services and their ACLs. No Python/Go/Node system installation or persistent PATH change.'
foreach($name in @('Yandu','YanduSFTPGo')){if((Get-Service $name -ErrorAction SilentlyContinue) -and -not (Test-Path "$StatePath\installation.json")){throw '缺少安装归属记录，拒绝接管'}}
$Operator=[System.Security.Principal.WindowsIdentity]::GetCurrent().User.Value
foreach($name in @('Yandu','YanduSFTPGo')){Stop-Service $name -ErrorAction SilentlyContinue}
if(Test-Path "$InstallPath\yandu.exe"){
 $Backup="$env:ProgramData\YanduBackups\$(Get-Date -Format yyyyMMdd-HHmmss)"
 New-Item -Type Directory $Backup -Force | Out-Null
 & "$InstallPath\yandu.exe" --state-dir $StatePath storage-maintenance --action backup --destination "$Backup\State"
 if($LASTEXITCODE -ne 0){throw '升级前一致性备份失败'}
 Copy-Item $InstallPath "$Backup\App" -Recurse
}
New-Item -ItemType Directory $InstallPath,$StatePath -Force | Out-Null
Copy-Item "$Source\*" $InstallPath -Recurse -Force
& icacls.exe $InstallPath /inheritance:r /grant:r '*S-1-5-18:(OI)(CI)F' '*S-1-5-32-544:(OI)(CI)F' '*S-1-5-32-545:(OI)(CI)RX' | Out-Null
if($LASTEXITCODE -ne 0){throw '程序 ACL 设置失败'}
foreach($name in @('Yandu','YanduSFTPGo')){
 $Args=if($name -eq 'Yandu'){'serve'}else{'component-sftpgo'}
 $Command='"'+$InstallPath+'\yandu.exe" --state-dir "'+$StatePath+'" '+$Args
 if(-not (Get-Service $name -ErrorAction SilentlyContinue)){& sc.exe create $name binPath= $Command start= auto obj= "NT SERVICE\$name" | Out-Null}else{& sc.exe config $name binPath= $Command start= auto obj= "NT SERVICE\$name" | Out-Null}
 if($LASTEXITCODE -ne 0){throw "服务配置失败 $name"}
 & sc.exe sidtype $name unrestricted | Out-Null
 & sc.exe failure $name reset= 86400 actions= restart/3000/restart/10000/restart/30000 | Out-Null
}
& "$InstallPath\yandu.exe" --state-dir $StatePath storage-bootstrap --component-root $InstallPath
if($LASTEXITCODE -ne 0){throw 'SFTPGo 初始化失败'}
$Core=(New-Object Security.Principal.NTAccount('NT SERVICE','Yandu')).Translate([Security.Principal.SecurityIdentifier]).Value
$Backend=(New-Object Security.Principal.NTAccount('NT SERVICE','YanduSFTPGo')).Translate([Security.Principal.SecurityIdentifier]).Value
& icacls.exe $StatePath /inheritance:r /grant:r '*S-1-5-18:(OI)(CI)F' '*S-1-5-32-544:(OI)(CI)F' "*$($Core):(OI)(CI)M" "*$($Backend):RX" "*$($Operator):RX" | Out-Null
foreach($dir in @('storage','sftpgo','Management','Storage')){New-Item -ItemType Directory "$StatePath\$dir" -Force | Out-Null}
& icacls.exe "$StatePath\Storage" /inheritance:r /grant:r '*S-1-5-18:(OI)(CI)F' '*S-1-5-32-544:(OI)(CI)F' "*$($Core):(OI)(CI)M" "*$($Backend):RX" | Out-Null
if($LASTEXITCODE -ne 0){throw '存储根目录遍历 ACL 设置失败'}
& icacls.exe "$StatePath\tools.json" /inheritance:r /grant:r '*S-1-5-18:F' '*S-1-5-32-544:F' "*$($Core):M" "*$($Backend):R" "*$($Operator):R" | Out-Null
if($LASTEXITCODE -ne 0){throw '工具配置读取 ACL 设置失败'}
@{schemaVersion=2;kind='yandu-storage-base'} | ConvertTo-Json | Set-Content "$StatePath\Storage\storage-base.json" -Encoding UTF8
foreach($dir in @('storage','sftpgo')){
 $SID=if($dir -eq 'storage'){$Core}else{$Backend}
 & icacls.exe "$StatePath\$dir" /reset /T | Out-Null
 if($LASTEXITCODE -ne 0){throw "凭据目录 ACL 重设失败 $dir"}
 & icacls.exe "$StatePath\$dir" /inheritance:r /grant:r '*S-1-5-18:(OI)(CI)F' '*S-1-5-32-544:(OI)(CI)F' "*$($SID):(OI)(CI)M" | Out-Null
 if($LASTEXITCODE -ne 0){throw "凭据目录 ACL 设置失败 $dir"}
}
# Existing v1 state may have desktop write ACEs: remove inherited and explicit operator grants from private files.
foreach($file in @('yandu.db','yandu.db-wal','yandu.db-shm','local.token')){if(Test-Path "$StatePath\$file"){
 & icacls.exe "$StatePath\$file" /reset | Out-Null
 & icacls.exe "$StatePath\$file" /inheritance:r /grant:r '*S-1-5-18:F' '*S-1-5-32-544:F' "*$($Core):M" | Out-Null
 if($LASTEXITCODE -ne 0){throw "私有文件 ACL 设置失败 $file"}
}}
& icacls.exe "$StatePath\Management" /reset /T | Out-Null
& icacls.exe "$StatePath\Management" /inheritance:r /grant:r '*S-1-5-18:(OI)(CI)F' '*S-1-5-32-544:(OI)(CI)F' "*$($Core):(OI)(CI)M" "*$($Operator):(OI)(CI)RX" | Out-Null
Start-Service YanduSFTPGo
Start-Service Yandu
$Ready=$false
for($i=0;$i -lt 60;$i++){try{Invoke-RestMethod 'http://127.0.0.1:18765/api/v1/health' | Out-Null;$Ready=$true;break}catch{};Start-Sleep -Milliseconds 500}
if(-not $Ready){throw '服务健康检查失败'}
@{schemaVersion=2;service='Yandu';installPath=$InstallPath;statePath=$StatePath;userSid=$Operator;services=@('Yandu','YanduSFTPGo')} | ConvertTo-Json | Set-Content "$StatePath\installation.json" -Encoding UTF8
& icacls.exe "$StatePath\installation.json" /grant:r "*$($Operator):R" | Out-Null
if($LASTEXITCODE -ne 0){throw '安装记录读取 ACL 设置失败'}
if(-not $SkipOpen){& "$InstallPath\yandu.exe" --state-dir $StatePath ui}
Write-Host '通用存储服务已安装。使用 storage-root-windows.ps1 为项目配置固定 NTFS 卷目录。'

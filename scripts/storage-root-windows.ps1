#Requires -RunAsAdministrator
param([Parameter(Mandatory=$true)][string]$ProjectId,[string]$BasePath="$env:ProgramData\Yandu\Storage")
$ErrorActionPreference='Stop'
if($ProjectId -notmatch '^[a-z][a-z0-9-]{0,47}$'){throw '项目 ID 无效'}
$Root=Join-Path $BasePath $ProjectId
if(Test-Path $Root){throw '目录已存在，拒绝覆盖'}
$Volume=Get-Volume -FilePath ([IO.Path]::GetPathRoot([IO.Path]::GetFullPath($Root)))
if($Volume.FileSystem -ne 'NTFS' -or $Volume.DriveType -ne 'Fixed'){throw '要求固定 NTFS 卷'}
$Core=(New-Object Security.Principal.NTAccount('NT SERVICE','Yandu')).Translate([Security.Principal.SecurityIdentifier]).Value
$Backend=(New-Object Security.Principal.NTAccount('NT SERVICE','YanduSFTPGo')).Translate([Security.Principal.SecurityIdentifier]).Value
if((Test-Path $BasePath) -and (Get-ChildItem $BasePath -Force) -and -not (Test-Path "$BasePath\storage-base.json")){throw '使用空目录或已有 Yandu 受管存储目录，不接管已有业务文件'}
New-Item -ItemType Directory $BasePath -Force | Out-Null
& icacls.exe $BasePath /inheritance:r /grant:r '*S-1-5-18:(OI)(CI)F' '*S-1-5-32-544:(OI)(CI)F' "*$($Core):(OI)(CI)M" "*$($Backend):RX" | Out-Null
if($LASTEXITCODE -ne 0){throw '存储基目录 ACL 配置失败'}
@{schemaVersion=2;kind='yandu-storage-base'} | ConvertTo-Json | Set-Content "$BasePath\storage-base.json" -Encoding UTF8
New-Item -ItemType Directory $Root,"$Root\files","$Root\temporary\uploads","$Root\temporary\jobs" -Force | Out-Null
& icacls.exe $Root /inheritance:r /grant:r '*S-1-5-18:(OI)(CI)F' '*S-1-5-32-544:(OI)(CI)F' "*$($Core):(OI)(CI)M" "*$($Backend):RX" | Out-Null
& icacls.exe "$Root\files" /inheritance:r /grant:r '*S-1-5-18:(OI)(CI)F' '*S-1-5-32-544:(OI)(CI)F' "*$($Core):(OI)(CI)RX" "*$($Backend):(OI)(CI)M" | Out-Null
if($LASTEXITCODE -ne 0){throw '正式文件 ACL 配置失败'}
& icacls.exe "$Root\temporary" /inheritance:r /grant:r '*S-1-5-18:(OI)(CI)F' '*S-1-5-32-544:(OI)(CI)F' "*$($Core):(OI)(CI)M" | Out-Null
if($LASTEXITCODE -ne 0){throw '暂存 ACL 配置失败'}
Write-Output $Root

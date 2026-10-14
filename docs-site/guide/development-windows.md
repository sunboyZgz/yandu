---
title: Windows 开发模式
description: Windows 11 x64 的源码启动、端口、NTFS 测试目录和重启方法。
---

# Windows 开发模式

命令在仓库根目录执行，使用当前用户运行 SFTPGo 和 Agent，不注册 SCM 服务。状态默认 `.yandu-storage-dev`，程序在 `.local\bin\yandu.exe`。

## 默认端口与配置

管理页 `127.0.0.1:18765`、存储 API `127.0.0.1:18770`、SFTPGo `127.0.0.1:18771`；兼容资源与组件管理使用回环 18766–18769。当前本机端口固定，配置来源及占用检查见[端口配置](./ports.md#本机开发与安装)。

## 1. 准备工具

使用 Go 1.26.2+、Python 3.12+。只有 `py` 启动器时用 `py -3` 替代 `python`。Node/npm 和原生工具按[依赖检测](./dependency-tools.md)准备；运行时只要求最低版本，无需降级已有的更高版本。

补齐 HEIC 需要 Visual Studio C++ Build Tools、CMake、Git，并在 Developer PowerShell 中运行；FFmpeg 的 `.7z` 解包需要 7-Zip。平台验证范围见[记录](/reference/storage-verification)。

## 2. 启动

只调试文件：

```powershell
python .\scripts\storage-dev.py --no-media
```

该方式不需要媒体构建工具，`mediaReady` 为 `false`。需要 HEIC/视频时，Ctrl+C 停止后去掉 `--no-media` 重新运行。终端显示 `Source Agent ready` 后进入管理页，保持终端运行。

## 3. 创建 NTFS 测试目录

```powershell
New-Item -ItemType Directory -Path "$PWD\.local\storage-projects\demo" -Force | Out-Null
```

使用本机固定 NTFS 卷的空目录，当前用户可读写，不使用 UNC、网络盘或链接目录。在管理页创建 `demo`、填入绝对路径和配额并启用，后续按[快速开始](/guide/getting-started)操作。

## 4. 状态和管理会话

在第二个 PowerShell 中进入同一仓库：

```powershell
$YanduStorageState = Join-Path $PWD '.yandu-storage-dev'
& .\.local\bin\yandu.exe --state-dir $YanduStorageState status
& .\.local\bin\yandu.exe --state-dir $YanduStorageState doctor
& .\.local\bin\yandu.exe --state-dir $YanduStorageState ui --require-running
```

自定义目录可在启动命令追加 `--state-dir .local\test-state --bin-dir .local\test-bin --no-open`，后续 CLI 使用相同状态目录。

## 5. 停止与重启

Ctrl+C 停止两个进程。Go/React 修改后重新运行，项目与文件保留；源码启动和原生打包顺序执行。

切换已安装的服务前，在管理员 PowerShell 中核对并停止：

```powershell
Get-Service Yandu,YanduSFTPGo
Stop-Service Yandu,YanduSFTPGo
```

开发结束后用 `Start-Service YanduSFTPGo,Yandu` 恢复安装版。

完整包构建与 SCM/ACL 安装见[本机安装](/guide/storage-deployment)。文档站默认回环 5174，启动与端口设置见[文档部署](/reference/development)。旧 `dev.ps1` 流程见[只读开发](./readonly-development-windows.md)。

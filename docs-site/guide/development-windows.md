---
title: Windows 开发模式
description: Windows 11 x64 的源码启动、端口、NTFS 测试目录和重启方法。
---

# Windows 开发模式

命令在仓库根目录执行，使用当前用户运行 SFTPGo 和 Agent，不注册 SCM 服务。状态默认 `.yandu-storage-dev`，程序在 `.local\bin\yandu.exe`。

## 默认端口与配置

管理页 `127.0.0.1:18765`、存储 API `127.0.0.1:18770`、SFTPGo `127.0.0.1:18771`；兼容资源与组件管理使用回环 18766–18769。当前本机端口固定，配置来源及占用检查见[端口配置](./ports.md#本机开发与安装)。

## 1. 准备工具

新克隆后，在仓库根目录的 64 位 PowerShell 执行：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\setup-windows-env.ps1
```

默认检查 Go 1.26.2+、Python 3.10+，优先复用已激活的 Conda 和满足最低版本的现有运行时；随后准备项目私有 Node/npm、Caddy、frpc、SFTPGo 及模板，并构建客户端。默认不调用系统安装器；缺少运行时会显示原因并停止，可用 `-PythonExe` 指定已有解释器。

需要 HEIC/视频时加 `-Media`，会检查 Python 3.12+ 并准备媒体组件。只有显式加 `-AllowSystemInstall`，才允许安装缺少的 Python/Go 和媒体构建工具。准备完成后按末尾 `Next:` 命令启动；如安装了系统工具，先重新打开 PowerShell。Conda 指定方法、只检查与自定义目录见[环境准备](./environment.md#windows-本机-一键准备)，工具复用规则见[依赖检测](./dependency-tools.md)。平台验证范围见[记录](/reference/storage-verification)。

## 2. 启动

只调试文件：

```powershell
python .\scripts\storage-dev.py --no-media
```

该方式不需要媒体构建工具，`mediaReady` 为 `false`。需要 HEIC/视频时，先 Ctrl+C 停止旧进程，再执行 `-Media` 环境准备，完成后去掉 `--no-media` 启动。终端显示 `Source Agent ready` 后进入管理页，保持终端运行。只有 `py` 启动器时，可用 `py -3` 替代下列 `python`，或直接使用准备脚本输出的实际解释器命令。

## 3. 创建 NTFS 测试目录

```powershell
New-Item -ItemType Directory -Path "$PWD\.local\storage-projects\demo" -Force | Out-Null
```

使用本机固定 NTFS 卷的空目录，当前用户可读写，不使用 UNC、网络盘或链接目录。在管理页创建 `demo`、填入绝对路径和配额并启用，后续按[快速开始](/guide/getting-started)操作。

## 4. 状态和管理会话

保持启动终端运行，在第二个 PowerShell 中进入同一仓库，重新打开管理页：

```powershell
& .\.local\bin\yandu.exe --state-dir "$PWD\.yandu-storage-dev" ui --require-running
```

`--require-running` 是 `yandu.exe ui` 的参数。`make ui` 仅构建前端，`make ui --require-running` 会被 make 拒绝；本页开发流程直接使用 Python 和 PowerShell 命令。若提示 Agent 未运行，先按上面的启动步骤运行 `storage-dev.py`。

需要查看状态或诊断时，再按需执行：

```powershell
$YanduStorageState = Join-Path $PWD '.yandu-storage-dev'
& .\.local\bin\yandu.exe --state-dir $YanduStorageState status
& .\.local\bin\yandu.exe --state-dir $YanduStorageState doctor
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

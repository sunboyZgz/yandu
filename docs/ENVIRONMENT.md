# 环境准备

先按运行位置选择准备方式。Ubuntu 云端与 Windows 本机源码均提供独立环境准备入口；Ubuntu 云端安装命令本身也会自动准备环境。

| 你要做什么 | 准备方式 |
| --- | --- |
| Ubuntu 云端，从源码安装 | 一键补齐环境，再自动编译和安装；见下方步骤 |
| Ubuntu 云端，使用 Linux 发布包 | 解压后直接安装；包内已有程序和 frps，无需 Go、Python 开发环境 |
| Windows 本机，从源码运行 | 使用下方 `setup-windows-env.ps1`，自动补齐运行时和本机组件 |
| macOS 本机源码 / 本机完整组件包 | 按对应平台开发或安装指南准备 |

## Windows 本机：一键准备

在新克隆的源码仓库根目录，用 **Windows x64 的 64 位 PowerShell** 执行：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\setup-windows-env.ps1
```

默认准备文件功能：检查 Go **1.26.2+**、Python **3.10+**，缺少时通过 [WinGet](https://learn.microsoft.com/en-us/windows/package-manager/winget/install) 安装；复用满足最低版本的现有运行时。随后自动准备 Node/npm、Caddy、frpc、SFTPGo 及原始模板，安装项目 npm 依赖并构建客户端。组件按锁文件校验并保存到私有目录，重复执行会复用正确工具与缓存。

完成后按脚本末尾的 `Next:` 命令启动，默认等价于 `python .\scripts\storage-dev.py --no-media`。如果本次安装了 Go/Python 等系统工具，先重新打开 PowerShell。准备脚本只准备依赖和构建程序，启动器负责启动 Agent 与 SFTPGo；已有源码实例运行时，先 Ctrl+C 停止再准备，避免 EXE 被占用。

需要 HEIC、视频处理时，加 `-Media`：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\setup-windows-env.ps1 -Media
```

该模式要求 Python **3.12+**，自动补齐 Git、CMake、7-Zip 和 Visual Studio C++ x64 构建工具，再准备 FFmpeg/FFprobe、HEIC 解码器和媒体 Worker 依赖。原生工具缺失时会从锁定源码构建；首次 C++ 工具安装及媒体编译可能耗时较长，安装器可能申请管理员权限。已有合适的 Visual Studio 工具会复用。Visual Studio 安装调用方式见[微软说明](https://learn.microsoft.com/en-us/visualstudio/install/use-command-line-parameters-to-install-visual-studio?view=vs-2022)。

只检查运行时及已记录组件的文件摘要，用 `-Check`；检查媒体环境时再加 `-Media`。自定义目录用 `-StateDir`、`-BinDir`，启动时使用同一组目录。WinGet 缺失时，先安装或更新 Microsoft App Installer，再重新打开 PowerShell；网络需能访问软件源、官方组件下载和 npm/Go 模块源。

若曾出现 `expected one sftpgo.exe executable in private payload`，更新后的准备器会按 PE 文件架构选择 x64 程序，已有完整下载缓存可直接复用。原始便携包包含多个架构，并不表示需要自行收集缺失文件。

## Ubuntu 云端：一键准备

适用 **Ubuntu 24.04 及更新版本的 x64 服务器，包括 26.04**，需带 apt、systemd 和用户管理工具。ARM 云服务器和容器内安装暂不在此入口的覆盖范围。允许安装的范围与实际验收结果见[验证记录](STORAGE_VERIFICATION.md)。

在服务器的 **Yandu 源码仓库根目录**执行：

```bash
sudo bash ./setup-cloud-env.sh
```

脚本自动完成：

1. 检查系统，只安装缺失的 `python3`、`ca-certificates`、`git`、`acl` 软件包。
2. 检查 Go：优先复用已安装且满足 `go.mod` 最低版本的稳定版（当前 **1.26.2+**），更高版本无需降级。
3. 缺少合适的 Go 时，从 [Go 官方下载](https://go.dev/dl/)获取归档和 SHA256，校验后安装到 `/usr/local/lib/yandu-env/`。云端安装器会自动找到该私有路径。

Python 要求 **3.10+**，通过 Ubuntu 软件包准备；云端源码安装不需要 Node.js/npm 或前端构建工具。重复执行会复用已满足要求的环境。已有 Go 安装和系统 PATH 保留。

准备过程需要访问 Ubuntu 软件源和 Go 官方下载。后续源码安装还需要访问 Go 模块源和 frp 的 GitHub 发布资产；如果现有工具或组件可复用，则跳过对应下载。下载失败会显示原因，可恢复网络后重试。

frps 下载超时的处理和离线准备方式见[依赖检测](DEPENDENCY_TOOLS.md#frps-下载超时)。

成功时会打印实际的 Python、Go 版本和路径，以及 `Ubuntu 云端源码安装环境已就绪`。

## 接着安装云端服务

仍在仓库根目录执行，把示例 IP 换成服务器公网 IP：

```bash
sudo bash ./install-cloud.sh --mode storage-only --server 203.0.113.10 --device owner
```

这条命令本身也会自动准备环境，因此**首次部署可以只执行这一条**。安装器接着编译云端程序、准备 frps、启动服务，最后提示设置连接包口令；此时本机客户端无需启动。

公网 **7000/TCP** 的放行、云端回环地址、连接包导入和启动检查，继续看[云端服务器启动](CLOUD_STARTUP.md)。

## 只检查或预览

```bash
sudo bash ./setup-cloud-env.sh --check     # 只检查；缺少依赖时返回失败
sudo bash ./setup-cloud-env.sh --dry-run   # 预览缺少的软件包与 Go 准备策略
```

这两个选项均不安装软件或下载文件。环境准备入口仅准备依赖；创建 Yandu 配置、服务与连接包由 `install-cloud.sh` 完成。

各平台的版本要求和工具复用规则见[依赖检测](DEPENDENCY_TOOLS.md)。

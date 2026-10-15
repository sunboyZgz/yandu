# 环境准备

先按运行位置选择准备方式。Ubuntu 云端与 Windows 本机源码均提供独立环境准备入口；Ubuntu 云端安装命令本身也会自动准备环境。

| 你要做什么 | 准备方式 |
| --- | --- |
| Ubuntu 云端，从源码安装 | 先检查运行时；显式允许后补齐系统依赖，再编译和安装 |
| Ubuntu 云端，使用 Linux 发布包 | 解压后直接安装；包内已有程序和 frps，无需 Go、Python 开发环境 |
| Windows 本机，从源码运行 | 使用下方 `setup-windows-env.ps1`，检查现有运行时并准备项目组件 |
| macOS 本机源码 / 本机完整组件包 | 按对应平台开发或安装指南准备 |

## Windows 本机：一键准备

在新克隆的源码仓库根目录，用 **Windows 11+ x64 的 64 位 PowerShell** 执行：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\setup-windows-env.ps1
```

**默认不安装或升级系统工具。** 脚本检查 Go **1.26.2+**、Python **3.10+**，显示候选路径、实际版本和不满足要求的原因；版本足够就复用，更高版本无需降级。检测执行失败会明确报错，不会按“未安装”处理。

`-AllowSystemInstall` 控制系统级安装。**不加该参数，准备脚本仍会检查并补齐项目私有组件**，具体如下：

| 依赖 | 已有工具合格时 | 需要补齐时 | 需要 `-AllowSystemInstall` |
| --- | --- | --- | --- |
| Python、Go | 复用现有解释器或编译器 | 通过 WinGet 安装系统工具 | **需要** |
| Node.js 与配套 npm | 复用合格 Node 及其 npm 入口 | 下载带 npm 的私有 Node，记录绝对路径 | 不需要 |
| Caddy、frpc、SFTPGo 及原始模板 | 验证架构、版本、实际执行与模板后复用 | 下载并校验到 Yandu 私有目录 | 不需要 |
| 项目 npm 依赖 | 锁文件匹配且可加载时复用 | 在 `web/node_modules`、媒体模式的 `media/node_modules` 中安装 | 不需要 |
| Git、CMake、7-Zip、VS C++ | 媒体构建需要时检查并复用 | 通过系统安装器补齐构建工具 | **需要** |

源码组件主要放在 `.yandu-storage-dev/tools/`，构建产物和部分缓存位于 `.local/`；自定义状态目录时随配置保存。私有组件不会覆盖系统安装、添加持久 PATH 项、注册系统服务或向 Conda 安装 Python 包。运行时合格后，脚本准备这些组件并构建客户端；Go/npm 还可能写入用户缓存。**只检测、连私有下载和构建也不执行时，加 `-Check`。**

需要 HEIC、视频时加 `-Media`，Python 最低要求提高到 **3.12+**。已激活 Conda 时优先检查该环境，也可用 `-PythonExe` 固定解释器，例如：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\setup-windows-env.ps1 -Media -PythonExe "$env:CONDA_PREFIX\python.exe"
```

`-PythonExe` 接受绝对路径；显式选择的解释器不可用或版本不足时直接停止。未激活 Conda 时，可指定实际安装路径。Python **3.13.12 x64** 满足文件与媒体两种模式的要求。

| 选项 | 行为 |
| --- | --- |
| `-Check` | 检查运行时、已记录组件摘要及实际可执行性、架构、版本和媒体模块，不下载、不构建；首次尚无组件或 Agent 时会报告未准备 |
| `-Media` | 增加 FFmpeg/FFprobe、HEIC 解码器和媒体 Worker；优先复用，缺少原生组件时用已有构建工具准备 |
| `-AllowSystemInstall` | 允许系统级安装 Python/Go；配合 `-Media` 允许安装媒体构建工具。上表中的 Node/npm、Caddy、frpc、SFTPGo 私有补齐默认可执行，无需此开关 |
| `-StateDir` / `-BinDir` | 自定义项目状态与程序目录，启动时使用相同目录 |

只有你决定补齐宿主机工具时，才运行下面的命令；已有合格 Python/Go 仍会复用。`-Check` 与 `-AllowSystemInstall` 不能同时使用。

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\setup-windows-env.ps1 -Media -AllowSystemInstall
```

媒体源码构建需要 Git、CMake 3.16.3+、7-Zip、Visual Studio 2022+ C++ x64 工具。先执行已有工具探针，检测错误不按缺失处理。缺少时默认停止并显示组件错误；允许系统安装后才会调用 [WinGet](https://learn.microsoft.com/en-us/windows/package-manager/winget/install) 或 [Visual Studio 安装器](https://learn.microsoft.com/en-us/visualstudio/install/use-command-line-parameters-to-install-visual-studio?view=vs-2022)，可能申请管理员权限，并影响系统软件及 PATH。已有完整媒体组件记录时跳过构建工具安装。

完成后按末尾 `Next:` 命令启动；如安装了系统工具，先重新打开 PowerShell。准备脚本负责依赖和构建，启动器负责启动 Agent 与 SFTPGo。已有源码实例运行时，先 Ctrl+C 停止再准备，避免 EXE 被占用。网络需能访问实际使用的官方组件源与 npm/Go 模块源。

Docker 适合隔离 Linux 构建或接口联调；[Docker Desktop 的 WSL 2 后端](https://docs.docker.com/desktop/features/wsl/)运行 Linux 容器，不能替代本项目 Windows 原生 NTFS 权限、路径和 SCM 服务的验证。目前这个入口使用现有运行时与项目私有组件，没有提供容器启动配置。

若曾出现 `expected one sftpgo.exe executable in private payload`，更新后的准备器会按 PE 文件架构选择 x64 程序，已有完整下载缓存可直接复用。原始便携包包含多个架构，并不表示需要自行收集缺失文件。

## Ubuntu 云端：一键准备

适用 **Ubuntu 24.04 及更新版本的 x64 服务器，包括 26.04**，需带 apt、systemd 和用户管理工具。ARM 云服务器和容器内安装暂不在此入口的覆盖范围。允许安装的范围与实际验收结果见[验证记录](STORAGE_VERIFICATION.md)。

在服务器的 **Yandu 源码仓库根目录**先检查：

```bash
bash ./setup-cloud-env.sh --check
```

默认只检测并复用，不调用 apt、不下载到系统目录。若确实需要补齐环境，显式允许系统安装：

```bash
sudo bash ./setup-cloud-env.sh --allow-system-install
```

允许后，脚本按缺失项完成：

1. 检查 Ubuntu 版本、x64、现有 Python/Go 的版本和可执行性。检测执行失败先停止，避免将其误判为缺失。
2. 通过 apt 补齐缺少的 `ca-certificates`、`git`、`acl`，仅在没有合格 Python 时补齐 `python3`；apt 可能同时改变系统依赖。
3. 复用满足 `go.mod` 最低版本的稳定版 Go（当前 **1.26.2+**）；缺少时从 [Go 官方下载](https://go.dev/dl/)获取归档和 SHA256，校验后写入 Yandu 专用 `/usr/local/lib/yandu-env/`。不替换 `/usr/local/go`，不改持久 PATH。

Python 要求 **3.10+**，通过 Ubuntu 软件包准备；云端源码安装不需要 Node.js/npm 或前端构建工具。重复执行会复用已满足要求的环境。已有 Go 安装和系统 PATH 保留。

准备过程需要访问 Ubuntu 软件源和 Go 官方下载。后续源码安装还需要访问 Go 模块源和 frp 的 GitHub 发布资产；如果现有工具或组件可复用，则跳过对应下载。下载失败会显示原因，可恢复网络后重试。

frps 下载超时的处理和离线准备方式见[依赖检测](DEPENDENCY_TOOLS.md#frps-下载超时)。

成功时会打印实际的 Python、Go 版本和路径，以及 `Ubuntu 云端源码安装环境已就绪`。

## 接着安装云端服务

仍在仓库根目录执行，把示例 IP 换成服务器公网 IP：

```bash
sudo bash ./install-cloud.sh --mode storage-only --server 203.0.113.10 --device owner
```

已有合格环境时直接执行上述命令；缺少系统依赖时会停止并说明原因。若希望一次补齐依赖并安装服务，在同一条安装命令末尾加 `--allow-system-install`。安装器接着编译云端程序、准备 frps、启动服务，最后提示设置连接包口令；此时本机客户端无需启动。

公网 **7000/TCP** 的放行、云端回环地址、连接包导入和启动检查，继续看[云端服务器启动](CLOUD_STARTUP.md)。

## 只检查或预览

```bash
sudo bash ./setup-cloud-env.sh --check     # 只检查；缺少依赖时返回失败
sudo bash ./setup-cloud-env.sh --dry-run   # 预览缺少的软件包与 Go 准备策略
```

这两个选项均不安装软件或下载文件。环境准备入口仅准备依赖；创建 Yandu 配置、服务与连接包由 `install-cloud.sh` 完成。

## 检查范围与系统影响

| 对象 | 检查内容 | 写入范围 |
| --- | --- | --- |
| 源码运行时 | 系统版本、CPU 与解释器位数；Python/Go 最低版本、Go 本机架构；检测失败显示原始原因 | 默认复用；系统安装需显式开关 |
| Node/npm | Node 22.12.0+ 稳定版、本机架构、npm 入口；项目模块实际加载 | 缺少时使用私有 Node；项目 `node_modules`，不全局安装 npm 包 |
| Caddy、frpc/frps、SFTPGo | 原生可执行格式、锁定组件版本、实际执行；SFTPGo 原始模板 | 缺少合格候选时准备私有副本，保留系统工具 |
| FFmpeg、FFprobe、HEIC | 原生架构、版本、动态库可加载；FFmpeg 编码器/封装器/滤镜，Sharp/ExifTool 模块 | 私有组件和媒体目录；无组件时先检查所需构建工具 |
| C++ 与构建工具 | CMake 最低版本；Git、7-Zip、pkg-config、make、Apple 工具或 VS C++ 可用性 | 默认不安装；Windows 系统安装需显式允许，macOS 手动准备 |
| 正式服务安装 | 操作系统、组件摘要与实际运行能力；Windows 完整存储包还检查固定 NTFS 卷 | 会写入系统安装/状态目录、创建身份与服务、设置 ACL；云端网站模式还涉及 Nginx/SSH |

源码准备使用项目 `.local`、状态目录和 `node_modules`；Go/npm 也可能写入当前用户的构建与下载缓存。指定系统安装开关后，apt/WinGet/VS 安装器可改变系统依赖和 PATH，不能称为零系统改动。旧只读 Windows 安装器仅在显式加 `-AddToPath` 时新增持久 PATH 项。

已有 Yandu 程序时，可对任何角色执行不落盘的组件检查：

```bash
.local/bin/yandu tools prepare --check --role local --component-root "$PWD" --config "$PWD/.yandu-storage-dev/tools.json"
```

只查文件组件加 `--no-media`；云端用 `yandu-cloud` 与 `--role cloud`；Windows 用 `.exe` 和对应 PowerShell 路径。此命令不下载、不构建、不保存配置或创建锁文件；未准备齐全会报告原因。正式安装的写入范围由安装入口在执行前输出，平台验收限制见[验证记录](STORAGE_VERIFICATION.md)。

各平台的版本要求和工具复用规则见[依赖检测](DEPENDENCY_TOOLS.md)。

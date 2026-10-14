# 依赖检测与私有工具路径

云端需要 frps；本机需要 SFTPGo、Caddy、frpc，启用媒体时增加 Node、FFmpeg、FFprobe 和 HEIC 解码器。

首次部署先看[环境准备与一键安装](ENVIRONMENT.md)。Ubuntu 云端源码安装自动准备所需环境，也可在仓库根目录单独执行 `sudo bash ./setup-cloud-env.sh`。

## 运行时最低版本

| 运行时 | 最低要求 | 使用场景 |
| --- | --- | --- |
| Go | **1.26.2+** | 源码编译；`go.mod` 的 `go` 行表示最低版本，允许更高版本 |
| Node.js | **22.12.0+ 稳定版**，附带 npm | 前端、文档构建与媒体 Worker；接受 Node 24、26 等更高主版本 |
| Python | **3.10+** | 源码启动器、云端源码安装；缺失原生媒体组件、需要从源码构建时使用 **3.12+** |

这些运行时不设版本上限，也不因已有版本更新而要求降级。Go 的最低版本语义见[官方说明](https://go.dev/doc/toolchain)；Node 下限满足当前 [Vite 7 的构建要求](https://v7.vite.dev/guide/migration#node-js-support)。完整安装包运行时不需要另装 Go 或 Python。

云端 storage-only 可直接在源码仓库根目录运行 `sudo bash ./install-cloud.sh ...`，仅使用 Go/Python 准备云端程序和 frps，不构建前端或安装 Node/npm。

云端源码准备优先复用 PATH 或 `/usr/local/go/bin/go` 中满足最低版本的 Go；缺少时在 `/usr/local/lib/yandu-env/` 安装经官方 SHA256 校验的副本，后续安装自动使用该路径。

`storage-components.lock.json` 中 Node 的 `minimumVersion` 控制已有运行时的准入，`version` 与 SHA256 控制缺失时下载的固定副本。发布包和 CI 使用具体版本以复现构建，不限制使用者安装更高版本的运行时。服务组件如 frp、SFTPGo 和媒体原生工具仍按组件契约验证。

## 检查顺序

1. 检查 PATH 中的工具，验证原生架构、可执行性，以及运行时最低版本或服务组件版本。
2. 检查 `tools.json` 中的路径。
3. 复用私有缓存或包内离线组件。
4. 仅下载或构建缺失项，放入 Yandu 私有目录。
5. 保存绝对路径、实际版本、来源和 SHA256，供启动器使用。

不符合要求的已有工具会保留。FFmpeg 需具备 libx264、AAC、MJPEG 编码器、MP4/HLS 封装和 scale/setsar 滤镜。组件版本与校验值由 `release/dependencies.lock.json`、`release/storage-components.lock.json` 锁定。

## 配置与私有目录

| 场景 | 配置位置 | 包内候选 / 补齐目录 |
| --- | --- | --- |
| macOS 安装 | `/Library/Application Support/Yandu/tools.json` | `/Library/Yandu/bin` / 状态目录 `tools/` |
| Windows 安装 | `C:\ProgramData\Yandu\tools.json` | `C:\Program Files\Yandu\bin` / 状态目录 `tools\` |
| 云端 | `/etc/yandu/tools.json` | `/usr/local/lib/yandu/frps` / `/etc/yandu/tools/` |
| 存储源码 | `.yandu-storage-dev/tools.json` | 仓库 `.local` / 状态目录 `tools/` |

Yandu 不修改系统 PATH 或安装全局 npm 包。源码构建 Node/npm 另存 `build-tools.json`，模块安装在项目 `node_modules`；锁文件匹配且可加载时复用，否则运行 `npm ci`。

`tools.json` 不存项目密钥，也不设置端口；监听地址见[端口配置](PORTS.md)。

## 查看和重新准备

云端：

```bash
sudo yandu tools list
sudo yandu tools prepare --role cloud
```

源码：

```bash
.local/bin/yandu --state-dir "$PWD/.yandu-storage-dev" tools list
.local/bin/yandu --state-dir "$PWD/.yandu-storage-dev" tools prepare --component-root "$PWD" --role local
```

Windows 使用 `.exe`。`list` 只展示记录，可加 `--json`；`prepare` 重新验证。只调试文件时加 `--no-media`。更换路径前停实例，准备后重启。

## 自定义路径

普通配置路径位于 PATH 之后，例如：

```json
{
  "schemaVersion": 1,
  "role": "local",
  "tools": {
    "ffmpeg": { "path": "/absolute/tools/ffmpeg" },
    "ffprobe": { "path": "/absolute/tools/ffprobe" }
  }
}
```

要固定路径，在 `tools prepare` 后添加 `--tool 'ffmpeg=/absolute/path with spaces/ffmpeg'`，可重复指定其他工具。记录中的 `pinned: true` 使它优先使用；验证失败则报错。取消固定时去掉 `pinned` 字段，再重新准备。

源码 `storage-dev.py` 的 `YANDU_FFMPEG`、`YANDU_FFPROBE`、`YANDU_HEIF_THUMBNAILER` 也会作为固定路径记录。

## 故障与构建前提

工具及动态库必须能被实际服务身份读取、执行。普通路径不可用时会验证包内候选，固定路径失败则停止。文件 SHA256 变化需重新准备；配置损坏或 schema 不支持时不会覆盖。

完整包提供离线组件。源码缺失组件时，macOS 构建 HEIC/FFmpeg 需要 Xcode Command Line Tools、CMake、pkg-config；Windows HEIC 需要 Visual Studio C++、CMake/Git，FFmpeg 解包需要 7-Zip。构建器的 `--only heif` / `--only ffmpeg` 可单独准备对应组件。

## frps 下载超时

`frps unavailable: context deadline exceeded` 表示组件准备超时。首次源码安装通常发生在从 GitHub 下载 frp 期间，此时尚未创建云端 Yandu 服务；不是本机客户端未启动，也与入站 7000 端口无关。检查安装输出中的下载地址、出站 HTTPS、DNS 和代理；使用 sudo 时需确认安装进程继承了所需代理环境。

下载最多尝试 3 次，每次最长 5 分钟，终端会显示地址、尝试次数和失败原因。网络恢复后可直接重跑原安装命令。源码准备使用仓库 `.local/cloud-install/` 持久缓存，已校验的组件会保留并复用。

如果云服务器无法访问 GitHub，可使用完整 Linux 发布包，或在能联网的机器下载 [frp 0.71.0 Linux x64 官方归档](https://github.com/fatedier/frp/releases/download/v0.71.0/frp_0.71.0_linux_amd64.tar.gz)，上传到云端源码仓库根目录，然后执行：

```bash
(
  set -e
  printf '%s  %s\n' \
    '84f27e39f11169f7adcef8e8b70c9329de17747b1f14dad9fb95eef5682ea716' \
    'frp_0.71.0_linux_amd64.tar.gz' | sha256sum -c -
  mkdir -p bin
  tar -xzf frp_0.71.0_linux_amd64.tar.gz -C bin --strip-components=1 frp_0.71.0_linux_amd64/frps
  chmod 0755 bin/frps
)
```

校验、解压成功后，重新执行原来的 `sudo bash ./install-cloud.sh --mode storage-only ...`。安装器会验证并复用 `bin/frps` 的原生架构和组件版本。此处版本与 SHA256 对应 `release/dependencies.lock.json`；使用其他版本源码时，以该锁文件为准。

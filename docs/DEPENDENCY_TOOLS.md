# 依赖检测与私有工具路径

云端需要 frps；本机需要 SFTPGo、Caddy、frpc，启用媒体时增加 Node、FFmpeg、FFprobe 和 HEIC 解码器。

## 检查顺序

1. 检查 PATH 中的工具，验证原生架构、可执行性和锁定版本。
2. 检查 `tools.json` 中的路径。
3. 复用私有缓存或包内离线组件。
4. 仅下载或构建缺失项，放入 Yandu 私有目录。
5. 保存绝对路径、版本、来源和 SHA256，供启动器使用。

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

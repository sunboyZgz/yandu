---
title: 只读映射开发模式
description: macOS 和 Linux 只读映射源码启动、素材目录、重建与停止。
---

# 只读映射开发模式

本页适用于 `readonly_mapping`。应用存储使用[存储开发入口](./development.md)，Windows 见[只读开发指南](./readonly-development-windows.md)。命令在仓库根目录执行。

## 默认端口与配置

管理页回环 18765，资源、鉴权和组件管理使用回环 18766–18769。共享启动器预检查 18765–18770，此入口不启动 SFTPGo。当前本机端口固定，配置来源见[端口配置](./ports.md#本机开发与安装)。

## 选择启动方式

| 命令 | 行为 |
| --- | --- |
| `make dev` | 构建并后台运行，打开管理页 |
| `make dev-serve` | 构建并前台运行，Ctrl+C 停止 |
| `make dev-build` | 仅构建 |

macOS Apple Silicon 使用 Go 1.26.2+、Python 3.10+；Node/npm 和 Caddy/frpc 按[依赖检测](./dependency-tools.md)准备。验证范围见[记录](/reference/storage-verification)。

## 第一次启动并打开工具

```bash
make dev
```

管理页在 `http://127.0.0.1:18765/`。需要重新打开或查看状态：

```bash
.local/bin/yandu --state-dir "$PWD/.yandu" ui
.local/bin/yandu --state-dir "$PWD/.yandu" status
.local/bin/yandu --state-dir "$PWD/.yandu" doctor
```

CLI 使用本项目 `.yandu` 状态目录；没有连接包时可查看界面，但无法创建完整网站映射。

## 开发模式下使用项目

先按[云端网站指南](./cloud.md)取得自己的连接包，在管理页导入，或执行：

```bash
.local/bin/yandu --state-dir "$PWD/.yandu" connection import "$HOME/Downloads/main.yandu-profile"
mkdir -p "$HOME/yandu-assets/blog/xxx"
```

将真实图片放入 `xxx/photo.png`。在管理页填写素材目录的完整绝对路径、实际站点、页面基础路径 `/blog` 和资源前缀 `/blog/xxx/`。站点授权须包含该前缀；目录需允许当前用户读取。

保存草稿、检查文件后，点击「应用」并确认。具体表单和验证步骤见[项目管理](./projects.md)，URL 到本机路径的规则见[路径映射](./routes.md)。

## 修改代码后如何生效

前台模式 Ctrl+C 停止后重新执行 `make dev-serve`。后台模式先停止原实例，再执行 `make dev`；仅构建不能使已运行的进程加载新代码。项目状态与素材保留。

独立 React Vite 页面不能完成管理会话，完整界面通过内嵌管理页验证。文档站默认回环 5174，启动方法见[文档部署](/reference/development)。

## 停止后台实例

```bash
lsof -nP -iTCP:18765 -sTCP:LISTEN
ps -p 12345 -o pid,command
```

将 `12345` 换成查到的实际 PID，确认它对应本项目的程序、`.yandu` 状态及 `serve` 后，再执行：

```bash
kill -TERM 12345
```

停止期间已发布资源不可访问。

## 状态、日志和常见问题

后台日志在 `.yandu/agent.log`，前台日志在终端；状态、组件配置与日志均在 `.yandu/`。会话问题用相同状态目录的 `ui` 重新打开。

端口、目录、连接与隧道问题见[故障排查](./troubleshooting.md)。

## 其他平台从源码启动

### Linux x64

准备 Go、Python，在仓库根目录执行：

```bash
python3 scripts/dev.py run --no-open
```

保持前台终端运行。CLI 使用相同 `.yandu` 状态；管理页需在具有桌面会话的本机打开。

### Windows x64

运行 `scripts/dev.ps1`，完整步骤见[Windows 指南](./readonly-development-windows.md)。

## 验证代码修改

停止开发实例后执行 `make check`、`make test-race`。真实 Nginx 链路验证与端口要求见[历史验证报告](/reference/verification)。

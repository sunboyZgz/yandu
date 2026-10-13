---
title: macOS 开发模式
description: macOS Apple Silicon 的源码启动、端口、测试目录、状态和重启方法。
---

# macOS 开发模式

本页适用于 macOS 14+ Apple Silicon，命令在仓库根目录执行。Windows 见[Windows 开发模式](./development-windows.md)，旧网站映射见[只读开发](./readonly-development.md)。

## 环境与目录

| 项目 | 要求 / 位置 |
| --- | --- |
| 开发工具 | Go 1.26.2、Python 3.12+、Xcode Command Line Tools |
| 补齐媒体组件时 | CMake、pkg-config |
| 程序 | `.local/bin/yandu` |
| 状态 | `.yandu-storage-dev` |

Node/npm 和原生工具按[依赖检测](./dependency-tools.md)准备。开发使用当前用户，正式服务身份的验证范围见[记录](/reference/storage-verification)。

## 默认端口与配置

管理页 `127.0.0.1:18765`、存储 API `127.0.0.1:18770`、SFTPGo `127.0.0.1:18771`；兼容资源与组件管理使用回环 18766–18769。当前本机端口固定，配置来源及占用检查见[端口配置](./ports.md#本机开发与安装)。

## 先运行文件功能

```bash
python3 scripts/storage-dev.py --no-media
```

启动器构建客户端并运行 SFTPGo、Agent。终端显示 `Source Agent ready` 后进入管理页；保持终端运行。`--no-media` 支持文件、续传、版本和回收站，`mediaReady` 为 `false`。

## 启用 HEIC 和视频处理

先 Ctrl+C 停止，再运行：

```bash
python3 scripts/storage-dev.py
```

缺失媒体组件会按锁定版本准备；自定义工具路径和构建前提见[依赖配置](./dependency-tools.md)。

## 创建测试项目

```bash
mkdir -p "$PWD/.local/storage-projects/demo"
```

使用空 APFS 目录。在管理页创建 `demo`、填入绝对路径和配额并启用，后续上传与恢复按[快速开始](/guide/getting-started)操作。

## 状态、诊断与重新打开

在第二个终端进入同一仓库：

```bash
.local/bin/yandu --state-dir "$PWD/.yandu-storage-dev" status
.local/bin/yandu --state-dir "$PWD/.yandu-storage-dev" doctor
.local/bin/yandu --state-dir "$PWD/.yandu-storage-dev" ui --require-running
```

自定义开发目录：

```bash
python3 scripts/storage-dev.py --state-dir .local/test-state --bin-dir .local/test-bin --no-open
```

后续 CLI 使用相同状态目录。状态目录不改变端口。

## 重建与停止

Ctrl+C 停止两个受管进程；Go/React 修改后重新运行启动命令，项目与文件保留。源码启动和原生打包共用前端目录，顺序执行。安装服务或其他实例占用端口时，先核对归属再停止。

源码测试见[验证记录](/reference/storage-verification#复现)，完整包构建见[本机安装](/guide/storage-deployment#构建完整客户端包)，文档站默认回环 5174，启动和 `DOCS_PORT` 配置见[文档部署](/reference/development)。

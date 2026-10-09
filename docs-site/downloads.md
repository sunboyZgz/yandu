---
title: 下载檐渡
description: 下载经过 SHA-256 校验的 Windows、Linux 和 macOS 验证版组件包。
outline: [2, 3]
---

# 下载檐渡

当前软件版本为 **v0.1.0 验证版**。选择与你实际安装位置对应的包：Windows 管理本地资源，Linux 接入云端网站，macOS 用于本机开发与验证。

<ReleaseDownloads />

## 下载后检查

所有可下载文件均来自当前工作区的实际构建，并在准备文档站时与发布清单核对。下载后再次校验可以检查传输完整性。

Windows：

```powershell
Get-FileHash .\yandu-0.1.0-windows-amd64.zip -Algorithm SHA256
```

Linux：

```bash
sha256sum yandu-0.1.0-linux-amd64.tar.gz
```

macOS：

```bash
shasum -a 256 yandu-0.1.0-darwin-arm64.tar.gz
```

将结果与对应文件的 SHA-256 比较。包内还有各文件的 SHA256SUMS 与第三方许可。

## 版本与运行条件

- Windows 目标：Windows 11 x64。客户端含 Caddy、frpc 和安装脚本。
- 云端目标：Ubuntu 24.04 x64、宿主机 Nginx。独立服务器仍需系统包安装、DNS、HTTPS 与网络条件。
- macOS ARM64 包用于开发与本机验证，不替代 Windows 正式支持声明。
- 当前没有 Authenticode、发行签名或 macOS 公证；校验和用于完整性检查，不代表发行者签名。

本机实测与尚待验收的内容见 [验证报告](./reference/verification.md)。取得包后，按 [快速开始](./guide/getting-started.md) 安装。

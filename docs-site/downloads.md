---
title: 下载檐渡
description: 下载经过 SHA-256 校验的 v0.2 通用存储组件包并查看平台验收状态。
outline: [2, 3]
---

# 下载檐渡

当前源码版本为 **v0.2.0**。以下列出实际构建且校验通过的文件。未构建的平台显示缺失；发布状态见[验证记录](/reference/storage-verification)。

<ReleaseDownloads />

## 下载后检查

所有可下载文件均来自当前工作区的实际构建，并在准备文档站时与发布清单核对。下载后再次校验可以检查传输完整性。

Windows：

```powershell
Get-FileHash .\yandu-0.2.0-windows-amd64.zip -Algorithm SHA256
```

Linux：

```bash
sha256sum yandu-0.2.0-linux-amd64.tar.gz
```

macOS：

```bash
shasum -a 256 yandu-0.2.0-darwin-arm64.tar.gz
```

将结果与对应文件的 SHA-256 比较。完整存储包内的 `component-receipt.json` 包含各文件校验值，安装器会再次验证；云端包内包含 `SHA256SUMS`。两个类型都保留第三方许可。

## 版本与运行条件

- Windows 目标：Windows 11 x64，使用 SCM 和独立服务身份；需在 Windows 原生构建媒体组件。
- 通用存储云端：安装器接受 Ubuntu 24.04 及更新版本的 x64 系统，包括 26.04；无需业务域名或 Nginx。旧只读映射仍以 Ubuntu 24.04 为安装基线，沿用网站配置。实际部署验收状态见下方验证记录。
- macOS 目标：macOS 14+ Apple Silicon，使用 launchd 和独立服务身份。
- 当前没有 Authenticode、发行签名或 macOS 公证；校验和用于完整性检查，不代表发行者签名。

本机实测与尚待验收的内容见 [v0.2 验证记录](./reference/storage-verification.md)。取得包后，Linux 云端按[云端服务器启动](./guide/cloud-startup.md)部署，macOS 与 Windows 客户端按[本机安装](./guide/storage-deployment.md)部署；完整顺序见[快速开始](./guide/getting-started.md)。旧版映射操作见[只读映射快速开始](./guide/readonly-getting-started.md)。

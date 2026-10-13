---
title: 快速开始
description: 启动云端和本机、导入连接包、创建项目并验证业务读写的完整流程。
---

# 快速开始

完整链路：**云端服务 → 本机服务 → 连接与配对 → 存储项目 → 业务读写验证**。云端 frps/网关和本机 Agent/SFTPGo 都需保持运行；本机无需公网 IP。平台发布状态见[验证记录](/reference/storage-verification)。

## 1. 启动云端服务器

在 Ubuntu 24.04 x64 上解压 Linux 包，首次安装：

```bash
sudo bash ./install-cloud.sh --mode storage-only --server 203.0.113.10 --device owner
```

替换成真实服务器 IP。安装器启动 `yandu-frps`、`yandu-gateway`，并生成 `/etc/yandu/owner.yandu-profile`；连接包口令由终端输入。安全交付该包，口令单独传递。

已安装时无需重复安装，用 `sudo systemctl start yandu-frps yandu-gateway` 启动。默认 TLS 隧道 `0.0.0.0:7000`、转发 `127.0.0.1:19090`、网关 `127.0.0.1:19091`；19090 在本机注册代理后监听。安装、启动检查与配置方法见[云端服务器启动](./cloud-startup.md)。

## 2. 启动本机客户端

| 方式 | 入口 | 状态目录与权限 |
| --- | --- | --- |
| macOS ARM64 源码 | [macOS 开发模式](./development.md) | `.yandu-storage-dev`，当前用户 |
| Windows x64 源码 | [Windows 开发模式](./development-windows.md) | `.yandu-storage-dev`，当前用户 |
| 完整存储组件包 | [本机安装](./storage-deployment.md)、[下载](/downloads) | 系统受管目录，独立服务身份 |

本机默认回环管理页 18765、存储 API 18770、SFTPGo 18771。一键入口端口固定，配置与占用检查见[端口说明](./ports.md#本机开发与安装)。启动完成后，用启动器或 `ui --require-running` 打开管理页。

## 3. 导入连接包并配对

在本机「服务器连接」导入云端生成的 schema v2 包，输入口令并检查隧道状态。本机主动连接云端 7000；云端 19090 和 19091 保持回环。

导入会停用已有项目，已有项目需核对后重新启用；新项目在下一步创建。详细步骤见[连接与配对](./connection.md)。

## 4. 创建存储项目

源码模式先创建空测试目录。macOS：

```bash
mkdir -p "$PWD/.local/storage-projects/demo"
```

Windows：

```powershell
New-Item -ItemType Directory -Path "$PWD\.local\storage-projects\demo" -Force
```

macOS 使用 APFS，Windows 使用固定 NTFS 卷。安装模式先按[本机安装](./storage-deployment.md)使用目录脚本授予服务权限。非空来源目录通过[显式导入](./business-integration.md#导入已有文件)接入。

在管理页创建项目 `demo`，填写目录的绝对路径与配额，例如 10 GiB，点击「授权目录并启用项目」，等待状态为 `active`。上传一个小文件，媒体预设留空；以校验入库完成、文件可下载为准。版本、回收站操作见[文件管理](./storage-files.md)。

## 5. 创建业务凭证

在项目「凭证」页申请 `read`、`upload`；业务需要目录修改或媒体任务时增加对应权限。密钥只留在业务服务端，浏览器使用业务自己的会话。

云端业务使用 `YANDU_GATEWAY=http://127.0.0.1:19091` 或同一 SDK `basePath`。Docker 与受信内网地址见[云端接入](./cloud-startup.md#docker-业务接入)，SDK 和示例启动见[业务接入](./business-integration.md)。

## 6. 从云端验证业务链路

在云服务器执行 `sudo yandu list`，确认本机在线及 `demo` 项目状态。然后在业务服务端安全设置 `YANDU_PROJECT_KEY`，验证项目容量：

```bash
curl --fail --show-error --max-time 10 \
  -H "Authorization: Bearer $YANDU_PROJECT_KEY" \
  http://127.0.0.1:19091/storage/v1/projects/demo/capacity
```

还需通过业务示例上传、读取一个原件，确认版本为 `READY`，再验证下载及 Range。容量查询成功只证明该接口可达，不替代文件读写。设备离线时返回 503 `DEVICE_OFFLINE`，排查见[故障指南](./troubleshooting.md)。

## 只调试本机功能

单机调试可跳过云端与配对，用源码入口运行，业务直接访问 `http://127.0.0.1:18770`。这适用于本机测试；远端业务的完整链路按上面步骤启动两端。

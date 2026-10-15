---
title: 连接包与私有隧道
description: 导入 schema v2 存储连接包、核对设备与 TLS 隧道，以及兼容 v1 网站映射的不同连接行为。
---

# 连接包与私有隧道

本机创建、上传和读取通用存储项目不需要连接包。远端业务接入时，在自己的云端部署 [storage-only](./cloud-startup.md)，再导入该服务器生成的加密材料。

## 默认端口与配置

本机 `127.0.0.1:18770` → frpc → 云端 TLS `7000` → 云端回环 `19090` → 业务网关回环 `19091`。无需给本机开放入站端口。

业务网关的端口通过 `gateway.json.listen` 配置，业务与监控地址需同步。当前配对器固定生成 7000，存储连接包只接受转发端口 19090，不能只改 frps 配置。具体方法见[云端端口配置](./ports.md#云端-storage-only)。

## 两种连接材料

| 连接包 | 信息与作用 |
| --- | --- |
| schema v2 storage-only | 设备身份、服务器、TLS CA/名称、存储转发端口及网关信息；无需网站或 SSH 配置通道 |
| schema v1 只读映射 | 网站和页面授权、SSH 主机指纹与配置身份、资源隧道；继续用于旧 Nginx 网站映射 |

首版支持单设备、多项目。不要把另一台服务器或别人的连接包作为自己的授权材料。已有只读云端不能由新的 storage-only 安装器覆盖，按[云端部署说明](./cloud-startup.md)处理。

## 在管理页导入

先把云端的 `/etc/yandu/owner.yandu-profile` 下载到**运行 Yandu 的本机电脑**，例如 Windows 的 `C:\Users\你的用户名\Downloads\owner.yandu-profile` 或 macOS 的 `~/Downloads/owner.yandu-profile`。本机接收目录可自由选择，无需放进源码仓库、安装目录或状态目录。下载方法见[云端启动：将连接包导入本机](./cloud-startup.md#_2-将连接包导入本机-yandu)。

1. 用启动器或 `yandu ui` 打开本机管理页。
2. 进入「服务器连接」，点击「选择加密连接包」，选中下载的 `.yandu-profile`。
3. 输入云端安装或配对时设置的连接包口令，点击「导入并配对」。

两端都由自己操作时，直接输入自己设置的口令；由云端管理员提供文件时，让管理员通过另一渠道告知口令。导入成功后连接材料由 Yandu 保存，原始下载文件可移到私有备份位置。

连接包上限为 256 KB。v2 存储包显示设备、存储转发端口与网关信息；v1 显示 SSH 指纹、站点和页面范围。

导入会替换旧材料和隧道配置，现有项目保持停用，需逐项核对并重新启用。连接包不是项目读写凭证；业务仍需单独申请项目凭证。

## 验证存储隧道

点击连接页的测试按钮。storage-only 检查 frpc 中存储代理是否运行；没有站点的存储包不会要求 SSH 或 Nginx。

再从**实际业务运行位置**调用网关项目的能力或容量接口，确认端到端访问。默认网关 `http://127.0.0.1:19091` 指云端服务器的回环地址，不是本机浏览器的同一地址。

“材料已导入”“进程正在运行”与真实文件可读是不同状态。进一步验证一个已 `READY` 原件的 HEAD、Range 和完整下载。

## 从源码命令行导入

macOS：

```bash
.local/bin/yandu --state-dir "$PWD/.yandu-storage-dev" connection import "$HOME/Downloads/owner.yandu-profile"
```

Windows：

```powershell
& .\.local\bin\yandu.exe --state-dir "$PWD\.yandu-storage-dev" connection import "$env:USERPROFILE\Downloads\owner.yandu-profile"
```

口令由终端读取，不写在参数或日志中。安装版使用包内启动器/实际程序路径和系统状态目录，见[CLI](/reference/cli)。

## TLS、离线与材料更新

本机主动连接云端 7000/TCP，验证包内 CA 和服务器名称；证书支持正确的 IP 或 DNS SAN。无公网 IP 的设备无需开放入站端口。

设备离线时网关返回 503 `DEVICE_OFFLINE`、`Retry-After: 5`。排查服务器地址、网络、frps 状态、证书有效期及代理注册；不要关闭证书校验来绕过错误。

口令错误、损坏或无效材料会使导入失败。轮换材料前核对服务器和设备身份，再通过可信方式生成、交付和导入；首版已有设备时不会自动追加第二个存储身份。

## 旧只读映射的配置通道

v1 的测试会验证 SSH 主机指纹与设备站点授权，后续路径同步使用该通道。`SSH_HOST_KEY_MISMATCH` 需要管理员确认主机变化原因，再重新配对，不自动信任新指纹。

旧映射还需检查 Nginx 入口和实际资源版本。操作见[只读云端指南](./cloud.md)，轮换流程见[旧映射运维参考](/reference/operations)。

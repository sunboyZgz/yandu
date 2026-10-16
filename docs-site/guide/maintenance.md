---
title: 升级、恢复与卸载
description: 在保留配置和素材的前提下维护两端程序，并理解失败恢复和凭据轮换。
---

# 升级、恢复与卸载

云端升级同时适用于网站资源映射和通用应用存储。本机应用存储的升级步骤见[通用存储部署](./storage-deployment.md)。

业务素材与工具程序使用独立目录。维护前先核对当前项目、连接状态和需要保留的数据。

## 升级 Windows 客户端

解压完整新包，在管理员 PowerShell 中执行：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\upgrade-windows.ps1
```

升级停止服务，备份对应数据库与旧程序组件，再更新并做健康检查。失败时恢复旧程序和相应数据库备份。

当前没有不可逆数据库迁移；未来版本的回退应遵循该版本说明。Windows 升级与 SCM 行为仍需目标机器验收。

## 升级云端组件

可以直接在云服务器的**源码仓库根目录**执行，无需先制作或解压发布包。更新工作区源码后运行：

```bash
sudo bash ./upgrade-cloud.sh --component cli --dry-run
sudo bash ./upgrade-cloud.sh
```

源码入口自动检查已安装环境、Go 和 Python，只编译云端 Go 程序，无需 Node/npm 或构建前端。CLI、网关更新不准备 frps；选择隧道或全部时，才复用或下载并校验 frps。编译及组件准备失败时不会进入部署，临时构建目录自动清理，已校验的组件缓存保留以便重试。缺少工具时会提示先执行 `sudo bash ./setup-cloud-env.sh --allow-system-install`，升级本身不会自动安装系统依赖。也支持从仓库执行 `sudo bash ./scripts/upgrade-cloud.sh ...`。

构建程序不会断开连接。脚本默认只更新 CLI，网关和隧道分别更新；下列命令也适用于解压后的 Linux 发布包：

| 更新范围 | 命令 | 连接影响 |
| --- | --- | --- |
| CLI（默认） | `sudo bash ./upgrade-cloud.sh --component cli` | 原子替换命令行程序，不重启服务 |
| 网关 | `sudo bash ./upgrade-cloud.sh --component gateway --drain-timeout 300` | 等待正在传输的请求完成，最长 300 秒；切换期间的新请求需重试 |
| 隧道 | `sudo bash ./upgrade-cloud.sh --component tunnel --allow-disconnect` | 仅在 frps 或配置发生变化时重启；已有连接会断开，客户端需要重连或续传 |
| 全部 | `sudo bash ./upgrade-cloud.sh --component all --allow-disconnect` | 分别应用上述规则，不因为 CLI 有变化就重启隧道 |

`--dry-run` 可以与任一范围组合。源码模式只检查环境和更新范围，不编译、不下载、不修改部署；尚未生成候选程序，因此是否需要重启要等正式构建后按程序和配置判断。发布包模式会校验已有候选程序/配置并展示具体计划，不替换文件、不操作服务；CLI 更新只需同目录的新版 `yandu-cloud`，隧道更新还需包中的 `bin/frps`。活动隧道确需重启时，未提供 `--allow-disconnect` 会在替换程序前退出。

网关和隧道启动器分别固定在 `/usr/local/lib/yandu/yandu-gateway`、`yandu-tunnel`，与 CLI 的 `yandu-cloud` 路径分离。旧部署首次执行升级时，会先固定旧运行文件并重载 systemd 定义，不为这一步重启服务。以后 CLI 更新不会改变后台服务下次启动使用的程序。

新网关收到停止信号后停止接收新请求，等待当前 HTTP 传输结束。多分块上传的后续请求仍需要重试或续传。`--drain-timeout` 为等待秒数，范围 1–86400；systemd 的停止期限自动设置为等待时间加 15 秒。超过期限的传输会断开并报告失败，需客户端重试/续传。这不是零停机切换。

**旧部署首次更新网关仍需维护窗口**：旧进程没有正确等待传输完成的能力，新代码不能改变已运行的旧进程。旧隧道没有运行指纹时，首次显式更新隧道会保守要求允许重连；后续才可根据已记录的程序和配置判断是否跳过重启。

更新前会校验对应配置，备份本次涉及的程序和启动配置；替换、配置准备或服务启动检查失败时自动回退，只恢复本次已重启的服务。备份位于 `/etc/yandu/backups/upgrade-*`。证书、连接包、项目及业务凭据保留，无需重新配对。原本停止的服务不会自动启动，封禁状态也不会因升级解除；业务 Nginx 服务不重启。

## 进程或磁盘故障恢复

后台子进程异常退出时会退避重启；Agent 重启时会核对数据库、有效配置与资源探针。未完成操作会记录为被中断，不直接假定成功。

盘符尚未就绪、目录不可读或证书失效时，修复具体条件后再应用。工具不会回退到素材根目录的父目录。

## 卸载客户端

先停用项目并等待云端同步成功，再执行：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\uninstall-windows.ps1
```

卸载自己的服务、程序和快捷方式，保留 `ProgramData` 状态和素材文件。离线卸载未完成云端同步时，由管理员核对入口拒绝规则。

## 卸载云端组件

```bash
sudo bash ./uninstall-cloud.sh
```

云端停隧道并保留资源拒绝路径，移除自己的 SSH/sudo 入口和程序。原网站、业务文件与恢复记录保留，不让旧资源掉回 SPA。

## 证书与凭据轮换

隧道叶证书的安装有效期为 365 天，CA 为 10 年。轮换 token、CA 或 SSH 设备密钥可能需要组件重连，不属于普通项目热更新。

本版提供管理员操作流程，详见 [运维与凭据轮换](../reference/operations.md)。不要用不明来历的连接包替换当前信任材料。

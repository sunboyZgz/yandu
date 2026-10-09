---
title: 升级、恢复与卸载
description: 在保留配置和素材的前提下维护两端程序，并理解失败恢复和凭据轮换。
---

# 升级、恢复与卸载

业务素材与工具程序使用独立目录。维护前先核对当前项目、连接状态和需要保留的数据。

## 升级 Windows 客户端

解压完整新包，在管理员 PowerShell 中执行：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\upgrade-windows.ps1
```

升级停止服务，备份对应数据库与旧程序组件，再更新并做健康检查。失败时恢复旧程序和相应数据库备份。

当前没有不可逆数据库迁移；未来版本的回退应遵循该版本说明。Windows 升级与 SCM 行为仍需目标机器验收。

## 升级云端组件

在云端新包目录执行：

```bash
sudo bash ./upgrade-cloud.sh
```

安装器备份自己的 helper/frps 并校验配置。frps 重连可能暂时影响资源连接，业务 Nginx 服务不重启。封禁状态会保留，不因升级自动解封。

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

---
title: 存储与连接故障排查
description: 排查 v0.2 原生组件、固定端口、受管目录、凭证、容量、Tus、媒体与设备离线，兼顾旧网站映射。
---

# 存储与连接故障排查

先确认项目是通用存储还是旧只读映射，再查看管理页项目状态、媒体任务和「操作」记录。管理页在 18765，文档在 5174；本机测试地址和云端回环网关不是同一台机器。

## 启动与固定端口

通用存储需要 Agent 18765、存储 API 18770 和独立 SFTPGo 18771。兼容资源与组件管理还使用回环 18766–18769。当前本机端口固定；占用时先核对进程/服务归属。可配置项与修改方法见[端口与监听配置](./ports.md)。

macOS 查看占用：

```bash
lsof -nP -iTCP:18765 -iTCP:18770 -iTCP:18771 -sTCP:LISTEN
```

Windows PowerShell 查看：

```powershell
Get-NetTCPConnection -State Listen | Where-Object LocalPort -in 18765,18770,18771
Get-Service Yandu,YanduSFTPGo -ErrorAction SilentlyContinue
```

从源码打开管理页时，使用[开发指南](./development.md)中的实际程序和 `.yandu-storage-dev`。`SESSION_REQUIRED` 或会话过期时用 `ui --require-running` 获取新会话，不手工复制内部令牌。

SFTPGo 缺失或启动失败时核对 2.7.6 原生程序与原始 `templates/email`。当前完整包用 `component-receipt.json` 校验；传统只读组件包没有完整存储/媒体组件。

## 项目与受管目录

| 现象 / 错误 | 检查 |
| --- | --- |
| 项目 `applying` | 初始化是后台操作，查看记录，等到 `active` 后操作文件 |
| `IMPORT_REQUIRED` | 根目录非空；新建空根后显式导入已有文件 |
| `PATH_NOT_FOUND` / 卷不可用 | 路径、卷挂载、解锁与文件系统 |
| `ACCESS_DENIED` | 实际服务身份是否能访问正式文件/暂存 |
| `REPARSE_POINT_DENIED` | 根路径或祖先是否含链接、junction、reparse |
| `VOLUME_CHANGED` | 卷/容器身份变化，需维护迁移或核验 |
| `INTEGRITY_QUARANTINE` | 外部修改或链接异常，先恢复/核验，不手改数据库解除 |
| `PROJECT_DISABLED` | 项目停用或隔离；核对管理页再决定启用 |

开发以当前用户运行；安装版用独立身份。macOS 正式根使用 `/Library/Application Support/Yandu/Storage`，Windows 要求固定 NTFS 卷，安装前由目录脚本设置权限。开发成功不能代替服务身份隔离测试。

## 凭证和项目边界

401 时核对 Bearer 格式、凭证归属及撤销状态。403 `SCOPE_DENIED` 时按[权限表](./storage-files.md#项目凭证)检查操作权限：例如上传状态需要 `upload`，回收站列表需要 `restore`，媒体请求需要 `process`。

凭证不能跨项目，也不能在公共网关调用管理 `/credentials`。浏览器使用业务会话，项目密钥留在服务器。遗失原文后新建并撤销旧凭证。

## 上传、续传和空间

进度 100% 后还在校验入库，查询 `/uploads/{id}`，以 `READY` 为准。`failed` / `cancelled` 必须处理原因，不能继续 PATCH 关闭的上传。

- `IDEMPOTENCY_CONFLICT`：同一键用于不同请求；恢复原逻辑上传参数或为新的逻辑上传创建新键。
- `UPLOAD_CLOSED`：先查询意图，可能已入库或失败，避免重复创建 Tus。
- `VERSION_CONFLICT`：并发替换或条目修改，刷新当前版本再决定操作。
- `NAME_CONFLICT`：同目录已有同名对象，检查是否是响应丢失后重试。
- 507 `INSUFFICIENT_STORAGE`：同时检查项目配额、活动预留和卷/容器空闲空间；回收站和历史版本也计入已用。

不得直接修改 Tus `.info`、删除正式对象或手改容量统计。恢复和空间维护见[存储维护](./storage-maintenance.md)。

## 媒体任务

`MEDIA_UNAVAILABLE` 时检查 `/capabilities` 的 `mediaReady`，以及是否以 `--no-media` 启动。检查 `tools.json` 的已解析工具路径，重新验证用 `tools prepare`；完整包提供原生候选，详见[依赖检测](./dependency-tools.md)。

任务排队不会阻塞上传入库。查看任务状态、尝试次数和管理端错误；核对输入格式、编码、文件是否损坏，以及工作区预算。预设重复提交不会重置同一版本已失败的任务。

HEIC 必须验证主图、方向及颜色，不能只用 PNG 的成功证明 HEIC 可用。处理与产物行为见[媒体指南](./media.md)。

## 云端网关与设备离线

首次安装出现 `frps unavailable: context deadline exceeded` 时，按[frps 下载超时](./dependency-tools.md#frps-下载超时)检查出站下载或预置离线组件，再重跑安装命令。

云端使用 `sudo yandu list`、`sudo yandu disks` 查询项目和磁盘；`--json` 用于监控。缺少配置、旧设备版本或旧连接包需按[监控指南](./cloud-monitoring.md)升级和交付。离线历史快照会标注时间并返回退出码 1，不能用于当前写入判断。

503 `DEVICE_OFFLINE` 说明网关连接不到本机存储，不表示服务器上有可替代的云副本。检查本机在线和 18770、frpc 代理、服务器 7000/TCP、frps、TLS CA/服务器名称与证书有效期。

19090 和 19091 默认保持云端回环。另一台主机或容器要按[云端接入说明](./cloud-startup.md)接入；把客户端的 `127.0.0.1` 填成远端服务地址不会连到云端。

## 删除和恢复

删除后立刻拒绝读取，后台移动尚未完成时恢复可能返回 `OPERATION_PENDING`。查看项目「操作」页，等待后刷新。父目录已删除、同名冲突需要先解决。

回收站默认不自动清空。文件恢复或任务完成不会释放所有历史版本的配额，详见[文件管理](./storage-files.md)。

## 日志与诊断

源码状态在 `.yandu-storage-dev`，SFTPGo 日志在其 `sftpgo/sftpgo.log`；Agent 输出在启动终端。安装版日志及受限状态在系统状态目录，需合适身份访问。

```bash
.local/bin/yandu --state-dir "$PWD/.yandu-storage-dev" status
.local/bin/yandu --state-dir "$PWD/.yandu-storage-dev" doctor
```

Windows 用对应 `.exe` 与 PowerShell 路径。管理页也有「导出诊断」。分享前核对输出及日志，不上传连接包、数据库或私钥作为普通问题附件。

## 旧只读映射

`ROUTE_CONFLICT`、`PAGE_ROUTE_PROTECTED`、SSH、Nginx 或外网版本探针错误属于旧网站路径流程，见[路径映射](./routes.md)、[云端网站](./cloud.md)及[旧映射运维](/reference/operations)。这些条件不是创建 `application_storage` 项目的前提。

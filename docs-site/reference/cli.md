---
title: 命令行 CLI
description: v0.2 本机程序路径、存储开发状态、配对、维护命令与旧只读项目 CLI。
---

# 命令行 CLI

日常管理命令通过已经运行的 Agent，与管理页共用服务逻辑。存储备份、恢复、核验和迁移是维护命令，有各自停止条件。

## 使用正确的程序与状态目录

| 场景 | 程序 | 状态 |
| --- | --- | --- |
| 通用存储源码 | `.local/bin/yandu` 或 `yandu.exe` | `.yandu-storage-dev` |
| 旧只读源码 | 同一构建目录的程序 | `.yandu` |
| macOS 完整安装 | `/Library/Yandu/yandu` | `/Library/Application Support/Yandu` |
| Windows 完整安装 | `C:\Program Files\Yandu\yandu.exe` | `C:\ProgramData\Yandu` |

全局参数 `--state-dir` 放在命令之前。下文 `yandu` 代表上表中实际程序，不保证安装目录已加入 PATH。

macOS 源码：

```bash
.local/bin/yandu --state-dir "$PWD/.yandu-storage-dev" ui --require-running
.local/bin/yandu --state-dir "$PWD/.yandu-storage-dev" status
.local/bin/yandu --state-dir "$PWD/.yandu-storage-dev" doctor
.local/bin/yandu version
```

Windows 源码：

```powershell
$YanduStorageState = Join-Path $PWD '.yandu-storage-dev'
& .\.local\bin\yandu.exe --state-dir $YanduStorageState ui --require-running
& .\.local\bin\yandu.exe --state-dir $YanduStorageState status
& .\.local\bin\yandu.exe --state-dir $YanduStorageState doctor
```

`ui` 打开本机会话；`--require-running` 不自动启动安装服务。`--no-open` 准备会话但不打开浏览器。`status` 查看实际状态，`doctor` 导出诊断，`version` 不要求 Agent 已运行。

上面的 `ui --require-running` 必须跟在 Yandu 可执行程序之后。`make ui` 是安装 npm 依赖并构建前端的 Makefile 目标；`make ui --require-running` 会把参数交给 make 而报错。Windows 源码开发使用上述 PowerShell 命令，首次启动见[Windows 开发模式](/guide/development-windows)。

## 源码启动与停止

工具路径由 `tools.json` 记录，启动时按 PATH、配置、私有候选和按需补齐解析。可用 `yandu --state-dir /absolute/state tools list` 查看，`tools prepare --role local --component-root /absolute/bundle` 重新验证；源码只启用文件能力时加 `--no-media`。云端用 `sudo yandu tools prepare --role cloud`。详情见[依赖检测与私有工具路径](/guide/dependency-tools)。

macOS `python3 scripts/storage-dev.py`，Windows `python .\scripts\storage-dev.py`，前置组件准备见[macOS 开发模式](/guide/development)或[Windows 开发模式](/guide/development-windows)。只调试原件加 `--no-media`，不要以单独 `serve` 代替后端准备。

启动器支持 `--state-dir`、`--bin-dir`、`--sftpgo`、`--no-open`。管理、存储、后端默认回环 18765 / 18770 / 18771；端口固定，多个状态目录不能同时使用同一组监听地址。配置项与源码位置见[端口配置](/guide/ports)。Ctrl+C 停止启动器拥有的进程。

`yandu serve` 用于运行已准备的 Agent；安装模式由 launchd/SCM 管理。

## 导入连接包

```bash
yandu --state-dir /absolute/state connection import ./owner.yandu-profile
```

口令由终端读取，不作为命令参数。支持 schema v2 存储及 v1 只读材料；导入后逐项启用项目。参见[私有隧道](/guide/connection)。

## 通用存储操作入口

项目创建、配额和凭证从「通用应用存储」管理页操作，或使用本机管理 API。目录、上传、下载、版本、删除、恢复和媒体使用[存储 API](/reference/storage-api)及 SDK。

现有 `project add` CLI 只创建传统只读映射，**没有通用存储 `--mode` / `--quota` 参数**，不要用其网站参数创建存储项目。

## 组件校验与初始化

完整原生包安装器调用以下命令：

```bash
yandu storage-components-verify --component-root /absolute/native-bundle
yandu --state-dir /absolute/state storage-bootstrap --component-root /absolute/native-bundle
```

`storage-components-verify` 校验目标平台和每个收据文件。`storage-bootstrap` 准备私有后端配置并初始化 SFTPGo，要求 18771 空闲；日常启动不重复执行。Windows 的 `component-sftpgo` 包装独立 SCM 后端，普通使用由安装器管理。

## 存储维护

```bash
yandu --state-dir /absolute/state storage-maintenance --action backup --destination /absolute/empty/backup
yandu --state-dir /absolute/empty/state storage-maintenance --action restore --destination /absolute/complete/backup
yandu --state-dir /absolute/state storage-maintenance --action verify --project demo
yandu --state-dir /absolute/state storage-maintenance --action migrate --project demo --destination /absolute/new-empty-root
```

| 动作 | 进程条件 |
| --- | --- |
| `backup` | Agent、SFTPGo 都停止 |
| `restore` | 服务停止，新状态与备份原根路径为空 |
| `verify` | Agent 停止，SFTPGo 运行 |
| `migrate` | Agent 停止，SFTPGo 运行；更新官方用户 home |

安装目录需要管理员权限。备份保留凭证、原件及 ID，恢复后项目保持停用。步骤和权限恢复见[维护指南](/guide/storage-maintenance)。

## 旧只读映射命令

下列示例要求自己的站点授权包含 `/blog/`，使用旧 `.yandu` 或安装版的实际状态：

```powershell
yandu root grant D:/oss/blog
yandu project add --id blog --name 个人博客 --site main --root D:/oss/blog --base /blog --prefix /blog/xxx/
yandu project list
yandu project apply blog --confirm-publish
yandu project disable blog
yandu project remove blog
```

`add` 保存草稿，`apply` 需确认并返回后台 operationId。多个前缀逗号分隔，不添加 `*`。通用存储的条目版本条件与旧映射配置 revision 是不同层次。

```powershell
yandu route release main blog-412212693349
yandu config export projects.json
yandu config import projects.json
```

释放使用真实 routeId，针对已停用的网站前缀；它不是回收站清空操作。草稿导出不含配对秘密，导入不会自动启用。完整只读流程见[兼容指南](/guide/readonly-getting-started)。

## 云端命令

云端的 `yandu` 和 `yandu-cloud` 支持相同命令；安装/升级将 `/usr/local/bin/yandu` 链接到 `/usr/local/lib/yandu/yandu-cloud`。简短入口用于日常运维，实际程序名称保留给安装脚本、systemd 和旧映射 SSH 固定命令。Linux 包内两个可执行文件的内容相同。

| 位置 / 服务 | 程序与职责 |
| --- | --- |
| 云端命令 `yandu` | `yandu-cloud` 的入口，查询项目/容量及配对 |
| 云端 `yandu-gateway` 服务 | 执行 `yandu-cloud storage-gateway`，持续运行流式网关 |
| 云端 `yandu-frps` 服务 | 执行官方 `frps`，接收本机主动建立的 TLS 隧道 |
| macOS / Windows 命令 `yandu` | 本机 Agent，管理本机存储和隧道，使用 `ui`、`serve`、`status` 等命令 |

云端查询命令运行后退出，不需要另启网关。名称、安装路径及旧映射助手的详细关系见[云端程序与服务说明](/guide/cloud-monitoring#yandu-与-yandu-cloud-的关系)。

```bash
sudo yandu list
sudo yandu disks
sudo yandu list --json
```

查询本地设备全部受管项目、配额和去重后的磁盘/APFS 容器。默认读取受限的 `/etc/yandu/monitor.json`。新 storage-only 配对自动交付专用只读监控凭证；旧部署需更新程序和连接包，见[云端命令行与容量监控](/guide/cloud-monitoring)。

```bash
sudo yandu pair --scope storage-only --dir /etc/yandu --device owner --server 203.0.113.10 --out /etc/yandu/owner.yandu-profile
yandu-cloud storage-gateway --config /etc/yandu/gateway.json
```

示例 IP 换成真实服务器地址。配对需先完成自己的 TLS/frps 安装，口令交互读取；网关默认由 `yandu-gateway` systemd 服务管理，不重复监听同一端口。完整步骤见[云端部署](/guide/cloud-startup)。

安装已完成但口令设置失败时，可直接重试上述 `pair` 命令；交互输入过短时会重新提示。已有设备配对成功后的监控凭证更新才使用 `--refresh-monitor`，见[配对恢复](/guide/cloud-startup#连接包口令失败后的恢复)。

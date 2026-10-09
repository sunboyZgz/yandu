---
title: Windows 开发模式
description: 在 Windows x64 的普通 PowerShell 中，从源码一键构建并启动檐渡，使用本地素材目录和项目，不需要安装客户端服务。
---

# Windows 开发模式

Windows x64 可以原生从源码使用檐渡。在项目根目录运行 `scripts/dev.ps1`，它自动构建管理页、Windows 客户端和配套组件，再以前台进程启动 Agent 并打开管理页面。

这个入口使用当前登录用户运行，默认不需要管理员权限、GNU Make、WSL 或 Windows 安装包。它不注册系统服务；状态保存在本项目 `.yandu/`，程序位于 `.local/bin/`。

::: info 验证范围
Windows 开发入口已实现，并完成 Windows x64 程序交叉构建、组件校验、启动逻辑测试及 PowerShell 入口验证。当前执行环境是 macOS，尚未在 Windows 真机执行完整的启动、NTFS ACL 和浏览器联调；请结合 [验证报告](../reference/verification.md) 验收。
:::

## 1. 准备环境

目标为 Windows 11 x64。安装并确认这些开发工具可以在普通 PowerShell 中找到：

```powershell
go version
node --version
npm --version
py -3 --version
```

工程使用 Go 1.26.2、Node.js 22、npm 与 Python 3.10+。如果没有 `py` 启动器，脚本也会查找 `python`、`python3`；安装后重新打开 PowerShell，让 PATH 生效。

进入你的源码目录，例如：

```powershell
cd C:\code\yandu
```

将示例路径替换为实际位置。路径中有空格时用引号：

```powershell
cd 'C:\Users\你的用户名\My Projects\yandu'
```

首次构建需要获取 npm/Go 依赖及 GitHub 上的组件。组件版本和 SHA-256 从 `release/dependencies.lock.json` 读取，不依赖全局 Caddy 或 frp。

## 2. 一键启动并打开管理页

在源码根目录执行：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\dev.ps1
```

这个执行策略只作用于本次 PowerShell 进程。已经允许执行项目脚本时，也可以直接运行：

```powershell
.\scripts\dev.ps1
```

默认 `-Mode run` 依次执行：

1. 检查本机 18765–18769 端口，冲突时停止并给出提示。
2. 安装 React 的锁定依赖并构建管理页。
3. 下载并校验 Windows x64 的 Caddy、frp 资产，准备 `caddy.exe` 和 `frpc.exe`。
4. 编译 `.local/bin/yandu.exe`，管理页嵌入该程序。
5. 为开发状态目录设置当前用户、SYSTEM 和 Administrators 的访问权限。
6. 以前台进程运行 Agent，用本项目状态完成认证，并等待文件服务就绪。
7. 用一次性票据打开 `http://127.0.0.1:18765/`。

**完成标志：** 终端显示 `Source Agent ready`，浏览器显示檐渡总览。保持终端运行；结束时在这个终端按 **Ctrl+C**，脚本向自己启动的 Agent 发送停止信号，由 Agent 关闭配套进程。

文档网站在 `127.0.0.1:5174`，与工具管理页不同。

### 已安装客户端服务时

正式安装的 Yandu 服务与源码实例使用相同的固定端口，不能同时运行。开发入口会提示冲突，不会自动停止或切换到那个服务。

如果确实要切换到源码开发，在另一个**管理员 PowerShell** 中核对并停止安装服务：

```powershell
Get-Service Yandu
Stop-Service Yandu
```

这会暂时停止原客户端提供的资源。之后回到普通 PowerShell 运行开发入口；开发结束后可由管理员执行 `Start-Service Yandu` 恢复安装版。

## 3. 日常启动和诊断命令

下面的命令在源码根目录执行。简写形式要求当前 PowerShell 已允许运行本项目脚本。

| 命令 | 用途 |
| --- | --- |
| `.\scripts\dev.ps1` | 构建、前台启动并打开管理页 |
| `.\scripts\dev.ps1 -NoOpen` | 构建并启动，在另一个终端自行打开页面 |
| `.\scripts\dev.ps1 -Mode build` | 仅构建，不启动 Agent |
| `.\scripts\dev.ps1 -Mode ui` | 打开已运行且通过本项目状态认证的管理页 |
| `.\scripts\dev.ps1 -Mode status` | 查看实际状态 |
| `.\scripts\dev.ps1 -Mode doctor` | 导出诊断信息 |
| `.\scripts\dev.ps1 -Mode plan` | 查看目标平台、组件和路径，不构建也不启动 |

例如，在第二个 PowerShell 窗口进入同一源码目录，重新打开页面：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\dev.ps1 -Mode ui
```

`ui`、`status`、`doctor` 使用本项目的开发程序和状态目录，不会在未认证时替你启动已安装的系统服务。

## 4. 导入连接包并创建项目

源码实例同样需要你自己的云端连接包，才能取得授权站点。没有连接包时可以启动并查看界面，但不能创建完整的资源项目。

按照 [准备云端网站](./cloud.md) 配好实际网站。在管理页“服务器连接”导入 `.yandu-profile` 文件，输入口令并测试配置通道；也可在第二个终端执行：

```powershell
$YanduDevState = Join-Path $PWD '.yandu'
& .\.local\bin\yandu.exe --state-dir $YanduDevState connection import .\main.yandu-profile
```

将连接包路径替换为实际文件位置。口令由终端隐藏读取。

### Windows 素材目录

示例使用 `D:\oss\blog`：

```powershell
New-Item -ItemType Directory -Force D:\oss\blog\xxx | Out-Null
```

放入一张真实图片 `D:\oss\blog\xxx\photo.png`。在“项目管理”新建项目：

| 字段 | 示例 |
| --- | --- |
| 项目名称 | Windows 开发博客 |
| 项目标识 | `blog-dev` |
| 本地素材目录 | `D:/oss/blog` |
| 网站站点 | 从自己的连接包授权站点中选择 |
| 页面基础路径 | `/blog` |
| 资源路径范围 | `/blog/xxx/` |
| 访问权限 | 公开只读 |
| 缓存策略 | 每次重验证 |

目录由当前登录用户读取；需要创建子目录时还需要写入权限。开发模式使用当前用户权限，安装版的 LocalService 权限配置不适用于这个前台实例。

授权目录后保存，先形成未发布草稿。点击“应用”并确认后，才会把符合策略的素材接入真实网站。示例要求实际授权包含 `/blog/`；若已有项目占用该前缀，请调整为授权范围内未被占用的路径。

CLI 的对应操作如下，前提是连接包里的站点 ID 为 `main`：

```powershell
$YanduDevState = Join-Path $PWD '.yandu'
& .\.local\bin\yandu.exe --state-dir $YanduDevState root grant D:/oss/blog
& .\.local\bin\yandu.exe --state-dir $YanduDevState project add --id blog-dev --name 开发博客 --site main --root D:/oss/blog --base /blog --prefix /blog/xxx/
& .\.local\bin\yandu.exe --state-dir $YanduDevState project apply blog-dev --confirm-publish
```

页面与 CLI 是两种操作方式，创建同名项目时选择一种即可。应用是异步操作，在“操作记录”查看进度，并确认实际图片响应。

停用、删除映射、独立密码和释放旧路径的流程与 [日常项目指南](./projects.md) 相同；源码开发不会跳过这些规则。

## 5. 修改代码并重新加载

Go 和 React 管理页当前需要重新编译，没有自动热重启：

1. 在开发终端按 Ctrl+C，等待 Agent 停止。
2. 修改 Go 或 `web/src/` 中的代码。
3. 再运行 `.\scripts\dev.ps1`，重新构建并启动。
4. 使用重新打开的管理页会话验证变更。

Windows 运行中的 EXE 可能被锁定。构建器会在更新程序前检查，发现锁定时提示先停止；不会靠强杀任意进程完成更新。

状态目录与素材保留，已有项目不会因重新编译变成一个全新配置。单独运行 `npm --prefix web run dev` 的 5173 页面，仍不能完成当前管理 API 的精确 Host/Origin 校验；完整功能请使用 Go 内嵌管理页。

## 自定义开发路径

```powershell
.\scripts\dev.ps1 -BinDir '.local\win-dev' -StateDir '.local\win-state'
.\scripts\dev.ps1 -Mode ui -BinDir '.local\win-dev' -StateDir '.local\win-state'
```

相对路径均从源码根目录解析。后续 `ui`、诊断和直接 CLI 操作必须使用同一个状态目录。路径中有空格时继续使用引号。

监听端口仍固定；不同状态目录不能让两个实例并行占用这些端口。

## 常见问题

| 现象 | 处理 |
| --- | --- |
| 脚本执行策略阻止运行 | 使用上面带 `-ExecutionPolicy Bypass -File` 的单次命令 |
| Python / Go / npm 未找到 | 确认开发工具安装并重新打开终端 |
| `Cannot locate npm-cli.js` | 使用包含 npm 的标准 Node.js 安装，检查 Node/npm PATH |
| 端口冲突 | 检查安装服务或另一开发实例，明确停止后再运行 |
| EXE 被锁定 | 在原开发终端 Ctrl+C，等待退出后重建 |
| 状态目录 ACL 设置失败 | 使用当前用户可管理的专用目录，检查原目录归属和权限 |
| 管理页无法重新打开 | 确认前台 Agent 仍运行，且 `-StateDir` 与启动一致 |
| 无站点 / 不能保存项目 | 先导入自己的连接包 |
| 文件不存在或不可读 | 核对真实盘符、目录、当前用户权限和媒体类型 |

开发启动逻辑可在源码根目录验证：

```powershell
py -3 scripts/test_dev.py -v
```

完整 Go/真实组件测试前先停止开发 Agent，避免资源端口冲突。跨平台启动入口使用 Python 标准库；[Python 进程接口](https://docs.python.org/3/library/subprocess.html)与 [Windows ACL 工具](https://learn.microsoft.com/en-us/windows-server/administration/windows-commands/icacls)的官方说明可用于核对维护行为。

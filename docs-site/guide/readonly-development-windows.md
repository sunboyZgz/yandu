---
title: 只读映射 Windows 开发
description: Windows 11 x64 的只读映射源码启动、开发路径、素材目录与重启。
---

# 只读映射 Windows 开发

本页适用于 `readonly_mapping`，通用存储见[Windows 存储开发](./development-windows.md)。命令在仓库根目录执行；当前用户运行，不注册系统服务。程序在 `.local\bin`，状态在 `.yandu`。

## 默认端口与配置

管理页回环 18765，资源、鉴权和组件管理使用回环 18766–18769。共享启动器预检查 18765–18770，此入口不启动 SFTPGo。当前本机端口固定，配置来源见[端口配置](./ports.md#本机开发与安装)。

## 1. 准备环境

Windows 11 x64 使用 Go 1.26.2+、Python 3.10+。脚本查找 `py`、`python` 或 `python3`；Node/npm 和组件按[依赖检测](./dependency-tools.md)准备。平台实测见[验证记录](/reference/storage-verification)。

## 2. 一键启动并打开管理页

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\dev.ps1
```

执行策略只作用于本次进程。终端显示 `Source Agent ready` 后进入回环 18765 的管理页，保持终端运行；Ctrl+C 停止 Agent 及其组件。

切换已安装的服务前，在管理员 PowerShell 中核对并停止 `Yandu`；如果同时安装存储后端，还需停止 `YanduSFTPGo`。开发结束后恢复安装服务。

## 3. 日常启动和诊断命令

以下简写要求允许运行项目脚本，否则沿用上面的 `powershell.exe ... -File` 调用方式。

| 参数 | 用途 |
| --- | --- |
| `-NoOpen` | 启动但不打开页面 |
| `-Mode build` | 仅构建 |
| `-Mode ui` | 打开已运行的管理页 |
| `-Mode status` | 查看状态 |
| `-Mode doctor` | 导出诊断 |
| `-Mode plan` | 查看平台和路径，不构建 |

例如 `.\scripts\dev.ps1 -Mode ui`。后续操作使用同一 `.yandu` 状态目录。

## 4. 导入连接包并创建项目

按[云端网站指南](./cloud.md)取得连接包，在管理页导入，或执行：

```powershell
$YanduDevState = Join-Path $PWD '.yandu'
& .\.local\bin\yandu.exe --state-dir $YanduDevState connection import .\main.yandu-profile
New-Item -ItemType Directory -Force D:\oss\blog\xxx | Out-Null
```

将真实图片放入 `D:\oss\blog\xxx\photo.png`。在管理页使用目录 `D:/oss/blog`、实际站点、页面基础路径 `/blog` 和资源前缀 `/blog/xxx/`。目录由当前用户读取，不使用安装服务的 LocalService ACL。

检查草稿与文件后点击「应用」并确认。表单、密码及发布验证见[项目管理](./projects.md)。

## 5. 修改代码并重新加载

Ctrl+C 等待进程退出，再运行启动脚本。Go/React 需重新编译，项目与素材保留。运行中的 EXE 可能被锁定，先停止再构建；完整界面使用内嵌管理页。

## 自定义开发路径

```powershell
.\scripts\dev.ps1 -BinDir '.local\win-dev' -StateDir '.local\win-state'
.\scripts\dev.ps1 -Mode ui -BinDir '.local\win-dev' -StateDir '.local\win-state'
```

路径相对仓库根目录解析。状态目录不改变监听端口。

## 常见问题

| 现象 | 处理 |
| --- | --- |
| 执行策略阻止脚本 | 使用单次 `-ExecutionPolicy Bypass -File` 调用 |
| EXE 被锁定 | 在原终端 Ctrl+C，等待退出 |
| 状态目录 ACL 失败 | 核对目录归属和当前用户权限 |
| 会话认证失败 | 核对启动与 CLI 的状态目录一致 |

连接、端口与文件问题见[故障排查](./troubleshooting.md)，源码测试见[验证记录](/reference/storage-verification#复现)。文档站默认回环 5174，启动和端口配置见[文档部署](/reference/development)。

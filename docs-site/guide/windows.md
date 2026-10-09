---
title: 安装 Windows 客户端
description: 安装后台服务、导入连接包，并为素材目录授予适当的读取权限。
---

# 安装 Windows 客户端

下载并解压 [Windows x64 发布包](../downloads.md)。最终客户端自带程序、Caddy 和 frpc，无需安装 Node.js、Go 或 Docker Desktop。

## 安装程序

在发布包目录打开管理员 PowerShell：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\install-windows.ps1 -ProfilePath .\main.yandu-profile
```

连接包口令由终端隐藏输入，不放到命令参数。也可以先安装，再在“服务器连接”中导入：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\install-windows.ps1
```

安装器配置服务和受限状态目录，建立桌面快捷方式。检测到同名服务或程序却没有归属记录时会停止，避免覆盖未知安装。

## 程序和文件的位置

| 目录 | 保存内容 |
| --- | --- |
| `C:\Program Files\Yandu\` | 客户端与锁定版本组件 |
| `C:\ProgramData\Yandu\` | 数据库、凭据、生成配置、日志与备份 |
| `D:\oss\blog\` | 你的素材，独立于程序安装和卸载 |

## 授予素材读取权限

后台服务以 LocalService 身份运行。素材目录需要允许该身份读取：

```powershell
icacls D:\oss\blog /grant '*S-1-5-19:(OI)(CI)RX'
```

这里的 `RX` 用于只读素材。目录创建功能还需要该目录的写入权限；若保留只读 ACL，创建子目录时会显示权限错误。管理页的目录授权不会自动修改 Windows ACL。

只授予明确素材目录的权限。网络映射盘、UNC、符号链接和 junction 不在本版可发布目录的支持范围。

## 日常打开管理页

双击“檐渡”桌面快捷方式，或在终端执行：

```powershell
yandu ui
```

管理页面只监听本机回环地址。启动器取得短期票据并兑换会话；直接输入地址没有会话时会提示从本机启动。

关闭浏览器不会停止服务。正常安装的服务随系统启动，打开管理页不要求云端此刻在线。

::: info 目标环境验收
安装脚本和 Windows 程序已完成构建；SCM、ACL、开机恢复等行为仍需 Windows 真机验收。请结合 [验证报告](../reference/verification.md) 使用当前验证版。
:::

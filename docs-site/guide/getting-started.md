---
title: 快速开始
description: 按顺序安装云端与 Windows 客户端，导入连接包，发布并验证第一张图片。
---

# 快速开始

目标是让 `https://example.com/blog/xxx/photo.png` 读取到 `D:/oss/blog/xxx/photo.png`，同时保留云端 `/blog` 的页面行为。

## 开始前

准备以下条件：

- 一台 Windows 11 x64 设备及管理员安装权限。
- 一台 Ubuntu 24.04 x64 服务器，运行宿主机 Nginx。
- 已能通过 HTTPS 访问的网站，例如 `example.com`。
- 一个只存放允许发布素材的本地目录。
- [Windows 与 Linux 发布包](../downloads.md)。安装最终客户端不需要 Go、Node.js 或 Python。

::: tip 只想在当前 macOS 上体验界面
在源码目录执行 `make dev`。它使用仓库内 `.yandu/` 保存开发状态。没有连接包时仍可打开管理页，但不会产生公网资源入口。详见 [开发运行](../reference/development.md)。
:::

## 1. 准备云端网站

解压 Linux 包，在服务器上先预览接入修改：

```bash
sudo bash ./install-cloud.sh \
  --mode existing-nginx \
  --site-id main \
  --domain example.com \
  --server-config /etc/nginx/sites-available/main.conf \
  --allow /blog/ \
  --dry-run
```

把示例域名、站点 ID 和配置路径换成你的实际值。确认选中了自己的 HTTPS server 块后，去掉 `--dry-run` 执行安装。

安装器会通过终端让你输入连接包口令，并生成 `main.yandu-profile`。安全传送这个加密包，单独交付口令。

**完成标志：** 网站保持正常，取得加密连接包，服务器 SSH 和隧道 TCP 7000 可达。更详细的步骤见 [准备云端网站](./cloud.md)。

## 2. 安装 Windows 客户端

将连接包放入解压后的 Windows 发布包目录。在管理员 PowerShell 中执行：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\install-windows.ps1 -ProfilePath .\main.yandu-profile
```

按提示输入连接包口令。安装器注册后台服务并创建“檐渡”桌面快捷方式；临时执行策略只作用于本次 PowerShell 进程。

给本地素材目录授予服务账号读取权限：

```powershell
icacls D:\oss\blog /grant '*S-1-5-19:(OI)(CI)RX'
```

**完成标志：** 可以用快捷方式打开本机管理页，文件服务已启动。完整说明见 [安装 Windows 客户端](./windows.md)。

## 3. 确认连接

在“服务器连接”中确认设备身份、授权站点和 SSH 主机指纹，再点击“测试配置通道”。若安装时没有导入连接包，在这里选择 `.yandu-profile` 并输入口令。

**完成标志：** 配置通道测试通过。导入连接包不会自动公开已有项目。

## 4. 创建第一个项目

先准备一个真实图片：

```text
D:/oss/blog/xxx/photo.png
```

在“项目管理”点击“新建项目”，填写：

| 字段 | 示例 |
| --- | --- |
| 项目名称 | 个人博客 |
| 项目标识 | `blog` |
| 本地素材目录 | `D:/oss/blog` |
| 网站站点 | `example.com · main` |
| 页面基础路径 | `/blog` |
| 资源路径范围 | `/blog/xxx/` |
| 访问权限 | 公开只读 |
| 缓存策略 | 每次重验证 |

明确授权该素材目录，点击“保存项目”。此时状态应为“未发布”。

## 5. 确认并应用

点击“应用”，检查目录与资源范围，勾选发布确认后点击“确认并应用”。

后台会依次校验配置、准备文件服务、注册隧道、同步云端入口并验证实际资源响应。可以在“操作记录”查看各阶段结果。

**完成标志：** 项目状态显示“已生效”，下面两个地址分别返回预期内容：

```text
https://example.com/blog                 → 云端博客页面
https://example.com/blog/xxx/photo.png   → 本地图片
```

如果未生效，按照 [状态与故障排查](./troubleshooting.md) 处理对应阶段。选择密码保护的项目还需要输入资源密码完成 [外网验证](./access.md)。

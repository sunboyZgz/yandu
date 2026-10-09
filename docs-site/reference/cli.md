---
title: 命令行 CLI
description: 查看檐渡启动、诊断、连接、项目与配置操作的实际命令。
---

# 命令行 CLI

CLI 与管理页使用同一套服务逻辑。本机 Agent 需要先运行；Windows 日常可通过安装后的快捷方式打开。

## 打开与诊断

```powershell
yandu ui
yandu status
yandu doctor
yandu version
```

`ui` 检查或启动 Agent 并打开浏览器；`status` 返回脱敏实际状态；`doctor` 导出诊断信息。

## 指定状态目录

全局参数放在命令前：

```bash
yandu --state-dir /absolute/state ui
yandu --state-dir /absolute/state status
```

当前 macOS 源码开发状态使用仓库 `.yandu/`：

```bash
.local/bin/yandu --state-dir "$PWD/.yandu" ui
```

## 配对与目录授权

```powershell
yandu connection import .\main.yandu-profile
yandu root grant D:/oss/blog
```

导入口令由终端隐藏读取。目录授权不自动更改 Windows ACL。

## 项目操作

以下创建示例适用于已授权的 `/blog/` 页面范围：

```powershell
yandu project add --id blog --name 个人博客 --site main --root D:/oss/blog --base /blog --prefix /blog/xxx/
yandu project list
yandu project apply blog --confirm-publish
yandu project disable blog
yandu project remove blog
```

`add` 保存草稿。`apply` 明确要求 `--confirm-publish`；命令返回 operationId，状态可在管理页或本地 API 查看。

多个前缀由逗号分隔，不加入通配符：

```powershell
yandu root grant D:/oss/gallery
yandu project add --id gallery --name 相册 --site main --root D:/oss/gallery --base /blog --prefix /blog/photos/,/blog/albums/
```

## 释放路由

```powershell
yandu route release main blog-412212693349
```

最后一项是实际 routeId，不能直接把资源 URL 填进去。通过管理页“已保留的旧路径”操作更便于核对范围；释放只针对已经停用的路径。

## 导入与导出草稿

```powershell
yandu config export projects.json
yandu config import projects.json
```

导出不含密码和配对材料；导入不会自动公开。存在同名项目时应在管理页编辑，密码项目需要重新设置密码。

请求结构、错误码和异步操作接口见 [本地 API 与云端协议](./api.md)。

---
title: 开发与文档部署
description: 构建源码、运行 VitePress 文档站，以及把静态文档部署到自己的目录。
---

# 开发与文档部署

工具管理页是 Go 内嵌的 React 应用；本网站是独立的 VitePress 文档站。文档站不提供目录管理或云端配置 API。

## 工具开发模式入口

通用存储的源码入口、依赖和重启方法见[macOS 开发模式](/guide/development)与[Windows 开发模式](/guide/development-windows)。管理、存储和后端默认监听本机回环 18765 / 18770 / 18771，当前端口固定，配置来源见[端口配置](/guide/ports#本机开发与安装)。旧入口 `make dev` / `scripts/dev.ps1` 见[兼容开发指南](/guide/readonly-development)。

## 运行本网站

在项目根目录执行：

```bash
npm --prefix docs-site ci
npm --prefix docs-site run dev
```

默认监听 `127.0.0.1:5174`。改用 5175：

```bash
DOCS_PORT=5175 npm --prefix docs-site run dev
```

Windows PowerShell 先执行 `$env:DOCS_PORT='5175'`，再运行同一 npm 命令。开发和静态预览共用 `DOCS_PORT`，端口冲突时不自动换号。完整示例见[文档站端口配置](/guide/ports#文档站开发与静态预览)。

VitePress 与 Vue 版本由 `docs-site/package-lock.json` 锁定。

## 构建静态网站

```bash
npm --prefix docs-site run build
npm --prefix docs-site run check
```

生成目录为 `docs-site/.vitepress/dist/`。其中 HTML、搜索索引、主题资源和准备好的下载包，可由静态 HTTP 服务提供。VitePress 构建会检查文档中的内部链接；额外检查核对生成页面的本地资源与下载校验值。

预览生产产物：

```bash
DOCS_PORT=5176 npm --prefix docs-site run preview
```

## 下载区的来源

构建前，`prepare-downloads.mjs` 从 release 产物中选择与 SHA256SUMS 一致的包，缺失平台显示尚未构建；`prepare-reference.mjs` 同步公开 OpenAPI。软件发行包先构建完成，再运行文档构建；准备器不复制连接包、数据库或私钥。

## 部署到子路径

默认网站部署于域名根目录。若部署到 `/yandu/`：

```bash
DOCS_BASE=/yandu/ npm --prefix docs-site run build
DOCS_BASE=/yandu/ npm --prefix docs-site run check
# 按相同基础路径预览：
DOCS_BASE=/yandu/ DOCS_PORT=5176 npm --prefix docs-site run preview
```

`DOCS_BASE` 必须以 `/` 开始和结束；自定义主题的导航、下载和静态资源也使用该基础路径。

将 dist 内容放入对应目录。下面的 Nginx 示例监听 HTTP 80，产物位于 `/var/www/yandu/`：

```nginx
server {
  listen 80;
  server_name docs.example.com;
  root /var/www;
  location /yandu/ {
    try_files $uri $uri.html $uri/ =404;
  }
}
```

将域名换成实际文档域名。需要其他 HTTP 端口时改 `listen`，例如 `listen 8080;`；HTTPS 的 443 与证书由已有站点配置提供。检查 `sudo nginx -t` 后执行 `sudo systemctl reload nginx`。静态网站端口由 Nginx 决定，`DOCS_PORT` 只影响本地开发/预览。

## 维护内容

用户指南位于 `docs-site/guide/`。技术参考通过 Markdown include 复用仓库 `docs/` 的已实现契约，保持来源一致；入口配置和中文搜索在 `.vitepress/config.mts`，主题在 `.vitepress/theme/`。

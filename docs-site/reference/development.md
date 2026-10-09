---
title: 开发与文档部署
description: 构建源码、运行 VitePress 文档站，以及把静态文档部署到自己的目录。
---

# 开发与文档部署

工具管理页是 Go 内嵌的 React 应用；本网站是独立的 VitePress 文档站。文档站不提供目录管理或云端配置 API。

## 运行工具源码

当前 macOS ARM64 开发环境需要 Go 1.26.2、Node.js 22 和 Python 3：

```bash
make dev
```

构建客户端，下载并校验锁定组件，使用仓库 `.yandu/` 启动本机界面。

```bash
make check
make test-race
make release
```

真实组件联调需提供实际 Nginx 二进制路径：

```bash
YANDU_TEST_NGINX=/absolute/path/nginx make integration
```

缺少真实组件时测试会明确 skip，不能当作目标环境验收通过。

## 运行本网站

在项目根目录执行：

```bash
npm --prefix docs-site ci
npm --prefix docs-site run dev
```

默认文档地址为 `http://127.0.0.1:5174/`。也可以使用 `make docs-dev`。

VitePress 依赖与 Vue 版本由 `docs-site/package-lock.json` 锁定。当前使用稳定版 VitePress 1.6.4，文档构建与预览命令遵循 [VitePress 官方部署说明](https://vuejs.github.io/vitepress/v1/guide/deploy)。

## 构建静态网站

```bash
npm --prefix docs-site run build
npm --prefix docs-site run check
```

生成目录为 `docs-site/.vitepress/dist/`。其中 HTML、搜索索引、主题资源和准备好的下载包，可由静态 HTTP 服务提供。VitePress 构建会检查文档中的内部链接；额外检查核对生成页面的本地资源与下载校验值。

预览生产产物：

```bash
npm --prefix docs-site run preview
```

## 下载区的来源

开发或构建前会运行 `scripts/prepare-downloads.mjs`：

1. 从实际代码读取 Yandu 版本。
2. 在 `release/artifacts/` 中寻找三个明确命名的公开发布包。
3. 与现有 SHA256SUMS 核对后复制到文档下载区。
4. 缺少的发布包显示为尚未构建，文档站仍可构建。

准备脚本不会复制 `.yandu/`、`.local/`、连接包、数据库或凭据。更新软件发行后，重新构建文档站即可同步下载文件。

## 部署到子路径

默认网站部署于域名根目录。若部署到 `/yandu/`：

```bash
DOCS_BASE=/yandu/ npm --prefix docs-site run build
DOCS_BASE=/yandu/ npm --prefix docs-site run check
```

`DOCS_BASE` 必须以 `/` 开始和结束；自定义主题的导航、下载和静态资源也使用该基础路径。

将 dist 内容放入你自己的对应目录，静态服务应支持干净 URL，例如 Nginx：

```nginx
location /yandu/ {
    root /var/www;
    try_files $uri $uri.html $uri/ =404;
}
```

这个示例要求产物位于 `/var/www/yandu/`；域名、HTTPS 和服务端口由你已有的站点决定。这里只说明文档网站部署，不能把文档服务器当作公网管理入口。

## 维护内容

用户指南位于 `docs-site/guide/`。技术参考通过 Markdown include 复用仓库 `docs/` 的已实现契约，保持来源一致；入口配置和中文搜索在 `.vitepress/config.mts`，主题在 `.vitepress/theme/`。

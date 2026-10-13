# 檐渡 VitePress 文档网站

独立的中文使用手册，采用 VitePress 1.6.4。导航按完整链路组织：开始使用、启动与部署、开发指南、通用应用存储、维护与排查。云端与本机分别说明，快速开始串联配对和业务验证。旧只读网站映射保留独立侧栏及原路径，部分原开发/入门长文归档到 `readonly-*`。

“开发指南”下的 macOS 和 Windows 页面分别位于 `guide/development.md`、`guide/development-windows.md`；它们说明工具源码入口。本文件说明文档网站服务器。

依赖版本由 `package-lock.json` 锁定。

## 本地运行

在项目根目录使用 Node.js 22 和 npm：

```bash
npm --prefix docs-site ci
npm --prefix docs-site run dev
```

访问 http://127.0.0.1:5174/ 。`make docs-dev` 也可安装锁定依赖并启动站点。端口冲突会明确报错，不会占用工具管理页的 18765 端口。

开发和静态预览均通过 `DOCS_PORT` 指定端口，默认回环监听，示例：

```bash
DOCS_PORT=5175 npm --prefix docs-site run dev
DOCS_PORT=5176 npm --prefix docs-site run preview
```

Windows PowerShell 使用 `$env:DOCS_PORT='5175'` 后运行 npm 命令。预览通过已锁定的 Vite 静态服务器提供构建产物。所有服务的默认端口与配置边界见 `guide/ports.md`；静态生产部署使用 Nginx 的实际 `listen`。

## 构建与检查

```bash
make docs-build
npm --prefix docs-site run preview
```

静态产物位于 `.vitepress/dist/`。构建检查 Markdown 链接，`npm run check` 额外检查生成页面的本地链接、章节锚点、静态资源和发布包 SHA-256。

若部署到子路径，构建与检查使用同一个基础路径：

```bash
DOCS_BASE=/yandu/ npm --prefix docs-site run build
DOCS_BASE=/yandu/ npm --prefix docs-site run check
```

将 dist 内容放到静态 HTTP 服务的对应目录，并支持将干净 URL 回退到 `.html` 文件。完整 Nginx 示例见网站的“开发与文档部署”。

## 内容来源与下载区

- `guide/`：面向使用者的操作指南。
- `reference/`：存储 API 与技术参考；架构、部署、维护和验证记录通过 Markdown include 复用仓库 `docs/`。
- `.vitepress/config.mts`：中文导航、侧栏、搜索与基础路径。
- `.vitepress/theme/`：响应式主题、交互路径演示与下载卡片。
- `scripts/prepare-downloads.mjs`：从 `release/artifacts/native/` 选择完整 macOS/Windows 存储包，从 `release/artifacts/` 选择 Linux 云端包；仅复制与 SHA256SUMS 一致的明确文件，缺少的平台显示尚未构建。
- `scripts/prepare-reference.mjs`：验证并公开 `schemas/storage.openapi.json`，供 API 页面下载；不遍历其他工程文件。

`public/downloads/`、`public/api/`、`.vitepress/generated/` 和 dist 为生成产物。无需提交，也无需手工维护；不要在下载生成目录中放置个人文件。文档构建不读取或复制连接包、数据库、私钥以及客户端状态目录。

软件组件包先完成构建，再准备文档下载；源码启动和原生打包共用前端目录，不能并行构建。仅更新文档内容无需重建客户端包。

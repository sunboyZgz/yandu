# 檐渡 VitePress 文档网站

独立的中文使用手册，采用 VitePress 1.6.4 稳定版。安装、连接、项目发布、目录映射、密码保护、维护和诊断均根据实际代码与仓库技术文档编写。

依赖锁定 Vue 3.5.43，并通过 npm overrides 使用 Vite 6.4.3，修复旧 Vite 开发服务器的已知问题。该版本在 Vue 插件的兼容范围内；构建、开发模式与浏览器交互均需在更新依赖后复核。[Vite 官方修复公告](https://github.com/vitejs/vite/security/advisories/GHSA-fx2h-pf6j-xcff)。

## 本地运行

在项目根目录使用 Node.js 22 和 npm：

```bash
npm --prefix docs-site ci
npm --prefix docs-site run dev
```

访问 http://127.0.0.1:5174/ 。`make docs-dev` 也可安装锁定依赖并启动站点。端口冲突会明确报错，不会占用工具管理页的 18765 端口。

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
- `reference/`：技术参考，其中五篇通过 Markdown include 复用仓库 `docs/`，修改原文件后重建即可同步。
- `.vitepress/config.mts`：中文导航、侧栏、搜索与基础路径。
- `.vitepress/theme/`：响应式主题、交互路径演示与下载卡片。
- `scripts/prepare-downloads.mjs`：读取当前程序版本，只复制 `release/artifacts/` 下明确命名且通过 SHA256SUMS 校验的三个公开发布包。缺少的包显示为尚未构建。

`public/downloads/`、`.vitepress/generated/` 和 dist 为生成产物。无需提交，也无需手工维护；不要在下载生成目录中放置个人文件。文档构建不读取或复制连接包、数据库、私钥以及客户端状态目录。

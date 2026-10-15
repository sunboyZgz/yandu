import { defineConfig } from 'vitepress'
import { docsListenOptions, docsBasePath } from '../scripts/listen-options.mjs'

const base = docsBasePath()

export default defineConfig({
  lang: 'zh-CN',
  title: '檐渡 · 使用文档',
  titleTemplate: ':title | 檐渡 Yandu',
  description: '檐渡 Yandu v0.2：通用存储、macOS 与 Windows 开发、Tus 续传、媒体、业务 SDK 与私有云端接入。',
  base,
  cleanUrls: true,
  srcExclude: ['README.md'],
  head: [
    ['link', { rel: 'icon', type: 'image/svg+xml', href: `${base}logo.svg` }],
    ['link', { rel: 'alternate icon', href: `${base}favicon.ico` }],
    ['meta', { name: 'theme-color', content: '#187c70' }],
  ],
  markdown: { codeCopyButtonTitle: '复制代码', config(md) {
    md.core.ruler.after('inline','storage-document-links', state => {
      for (const token of state.tokens) for (const child of token.children ?? []) {
        if (child.type === 'link_open' && child.attrGet('href') === 'STORAGE_VERIFICATION.md') child.attrSet('href','/reference/storage-verification')
        if (child.type === 'link_open' && child.attrGet('href')?.startsWith('DEPENDENCY_TOOLS.md')) child.attrSet('href', child.attrGet('href')!.replace('DEPENDENCY_TOOLS.md','/guide/dependency-tools'))
        if (child.type === 'link_open' && child.attrGet('href')?.startsWith('ENVIRONMENT.md')) child.attrSet('href', child.attrGet('href')!.replace('ENVIRONMENT.md','/guide/environment'))
        if (child.type === 'link_open' && child.attrGet('href')?.startsWith('PORTS.md')) child.attrSet('href', child.attrGet('href')!.replace('PORTS.md','/guide/ports'))
        if (child.type === 'link_open' && child.attrGet('href')?.startsWith('MEDIA_ACCEPTANCE.md')) child.attrSet('href', child.attrGet('href')!.replace('MEDIA_ACCEPTANCE.md','/reference/media-acceptance'))
        if (child.type === 'link_open' && child.attrGet('href')?.startsWith('CLOUD_STARTUP.md')) child.attrSet('href', child.attrGet('href')!.replace('CLOUD_STARTUP.md','/guide/cloud-startup'))
      }
    })
  } },
  vite: {
    server: docsListenOptions(),
    preview: docsListenOptions(),
  },
  themeConfig: {
    logo: { src: '/logo.svg', alt: '檐渡' },
    siteTitle: '檐渡',
    skipToContentLabel: '跳至正文',
    nav: [
      { text: '使用指南', link: '/guide/getting-started', activeMatch: '^/guide/(?!(?:development(?:-windows)?|environment|cloud-startup|storage-deployment|connection|business-integration)$)' },
      { text: '启动与部署', items: [
        { text: '环境准备与一键安装', link: '/guide/environment' },
        { text: '云端服务器启动', link: '/guide/cloud-startup' },
        { text: '本机安装（macOS / Windows）', link: '/guide/storage-deployment' },
        { text: '连接与配对', link: '/guide/connection' },
        { text: '业务接入与验证', link: '/guide/business-integration' },
      ] },
      { text: '开发指南', items: [
        { text: 'macOS 开发模式', link: '/guide/development' },
        { text: 'Windows 开发模式', link: '/guide/development-windows' },
      ] },
      { text: 'API 与 SDK', link: '/reference/storage-api', activeMatch: '^/reference/' },
      { text: '下载', link: '/downloads' },
      { text: 'v0.2 开发版', items: [
        { text: '版本说明', link: '/changelog' },
        { text: '验证与支持范围', link: '/reference/storage-verification' },
      ] },
    ],
    sidebar: [
      { text: '开始使用', items: [
        { text: '认识檐渡', link: '/guide/introduction' },
        { text: '环境准备与一键安装', link: '/guide/environment' },
        { text: '快速开始', link: '/guide/getting-started' },
        { text: '端口与监听配置', link: '/guide/ports' },
      ] },
      { text: '启动与部署', items: [
        { text: '云端服务器启动', link: '/guide/cloud-startup' },
        { text: '本机安装（macOS / Windows）', link: '/guide/storage-deployment' },
        { text: '连接与配对', link: '/guide/connection' },
        { text: '业务接入与验证', link: '/guide/business-integration' },
      ] },
      { text: '开发指南', items: [
        { text: 'macOS 开发模式', link: '/guide/development' },
        { text: 'Windows 开发模式', link: '/guide/development-windows' },
      ] },
      { text: '通用应用存储', items: [
        { text: '存储架构与工作流程', link: '/guide/storage' },
        { text: '文件、版本、回收站与凭证', link: '/guide/storage-files' },
        { text: '图片、HEIC、视频与任务', link: '/guide/media' },
        { text: '依赖检测与私有工具路径', link: '/guide/dependency-tools' },
        { text: '备份、恢复与迁移', link: '/guide/storage-maintenance' },
      ] },
      { text: '维护与排查', items: [
        { text: '云端命令行与容量监控', link: '/guide/cloud-monitoring' },
        { text: '存储与连接故障排查', link: '/guide/troubleshooting' },
        { text: '常见问题', link: '/guide/faq' },
        { text: 'v0.2 验证与发布门槛', link: '/reference/storage-verification' },
        { text: 'MP4 压缩验收记录', link: '/reference/media-acceptance' },
      ] },
      { text: '兼容的只读网站映射', collapsed: true, items: [
        { text: '只读映射快速开始', link: '/guide/readonly-getting-started' },
        { text: '只读映射源码开发', link: '/guide/readonly-development' },
        { text: '只读映射 Windows 开发', link: '/guide/readonly-development-windows' },
        { text: '准备 Nginx 云端网站', link: '/guide/cloud' },
        { text: '旧版 Windows 客户端', link: '/guide/windows' },
        { text: '创建与发布项目', link: '/guide/projects' },
        { text: 'URL 与目录映射', link: '/guide/routes' },
        { text: '浏览目录与复制地址', link: '/guide/files' },
        { text: '独立密码保护', link: '/guide/access' },
        { text: '停用、删除与释放', link: '/guide/lifecycle' },
        { text: '旧映射升级与卸载', link: '/guide/maintenance' },
        { text: 'v0.1 历史验证', link: '/reference/verification' },
      ] },
      { text: '技术参考', collapsed: true, items: [
        { text: '存储 API、鉴权与错误码', link: '/reference/storage-api' },
        { text: '命令行 CLI', link: '/reference/cli' },
        { text: '管理 API 与旧云端协议', link: '/reference/api' },
        { text: '旧映射部署契约', link: '/reference/deployment' },
        { text: '旧映射运维与凭据轮换', link: '/reference/operations' },
        { text: '旧映射安全边界', link: '/reference/security' },
        { text: '开发与文档部署', link: '/reference/development' },
      ] },
    ],
    outline: { level: [2, 3], label: '本页内容' },
    search: {
      provider: 'local',
      options: {
        miniSearch: {
          options: {
            // Intl segments Chinese words instead of treating a whole sentence as one token.
            tokenize: (text: string) => Array.from(
              new Intl.Segmenter('zh-CN', { granularity: 'word' }).segment(text),
            ).filter(item => item.isWordLike).map(item => item.segment.toLowerCase()),
          },
        },
        locales: { root: { translations: {
          button: { buttonText: '搜索文档', buttonAriaLabel: '搜索文档' },
          modal: {
            displayDetails: '显示详细内容', resetButtonTitle: '清空搜索',
            backButtonTitle: '关闭搜索', noResultsText: '没有找到相关内容',
            footer: {
              selectText: '选择', selectKeyAriaLabel: '回车', navigateText: '切换结果',
              navigateUpKeyAriaLabel: '向上', navigateDownKeyAriaLabel: '向下',
              closeText: '关闭', closeKeyAriaLabel: 'Esc',
            },
          },
        } } },
      },
    },
    docFooter: { prev: '上一篇', next: '下一篇' },
    sidebarMenuLabel: '文档目录', returnToTopLabel: '返回顶部',
    darkModeSwitchLabel: '外观', lightModeSwitchTitle: '切换为浅色',
    darkModeSwitchTitle: '切换为深色',
    footer: { copyright: '檐渡 Yandu · MIT License' },
    notFound: { title: '这条路径没有文档', quote: '可以从快速开始继续，或搜索你要解决的问题。', linkLabel: '返回文档首页', linkText: '返回文档首页' },
  },
})

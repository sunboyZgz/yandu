import { defineConfig } from 'vitepress'

const base = process.env.DOCS_BASE || '/'
if (!/^\/(?:[A-Za-z0-9_-]+\/)*$/.test(base)) {
  throw new Error('DOCS_BASE 必须是以 / 开始和结束的路径，例如 / 或 /yandu/。')
}

export default defineConfig({
  lang: 'zh-CN',
  title: '檐渡 · 使用文档',
  titleTemplate: ':title | 檐渡 Yandu',
  description: '檐渡 Yandu 使用手册：安装、连接、本地目录、资源路径、发布与诊断。',
  base,
  cleanUrls: true,
  srcExclude: ['README.md'],
  head: [
    ['link', { rel: 'icon', type: 'image/svg+xml', href: `${base}logo.svg` }],
    ['link', { rel: 'alternate icon', href: `${base}favicon.ico` }],
    ['meta', { name: 'theme-color', content: '#187c70' }],
  ],
  markdown: { codeCopyButtonTitle: '复制代码' },
  vite: {
    server: { host: '127.0.0.1', port: 5174, strictPort: true },
    preview: { host: '127.0.0.1', port: 5174, strictPort: true },
  },
  themeConfig: {
    logo: { src: '/logo.svg', alt: '檐渡' },
    siteTitle: '檐渡',
    skipToContentLabel: '跳至正文',
    nav: [
      { text: '使用指南', link: '/guide/getting-started', activeMatch: '^/guide/(?!development)' },
      { text: '开发模式', link: '/guide/development' },
      { text: '参考资料', link: '/reference/cli', activeMatch: '^/reference/' },
      { text: '下载', link: '/downloads' },
      { text: 'v0.1.0', items: [
        { text: '版本说明', link: '/changelog' },
        { text: '验证与支持范围', link: '/reference/verification' },
      ] },
    ],
    sidebar: [
      { text: '开始使用', items: [
        { text: '认识檐渡', link: '/guide/introduction' },
        { text: '快速开始', link: '/guide/getting-started' },
        { text: '开发模式使用', link: '/guide/development' },
        { text: '准备云端网站', link: '/guide/cloud' },
        { text: '安装 Windows 客户端', link: '/guide/windows' },
        { text: '导入连接包', link: '/guide/connection' },
      ] },
      { text: '日常使用', items: [
        { text: '创建与发布项目', link: '/guide/projects' },
        { text: 'URL 与目录映射', link: '/guide/routes' },
        { text: '浏览目录与复制地址', link: '/guide/files' },
        { text: '独立密码保护', link: '/guide/access' },
        { text: '停用、删除与释放', link: '/guide/lifecycle' },
      ] },
      { text: '维护与排查', items: [
        { text: '状态与故障排查', link: '/guide/troubleshooting' },
        { text: '升级、恢复与卸载', link: '/guide/maintenance' },
        { text: '常见问题', link: '/guide/faq' },
      ] },
      { text: '技术参考', collapsed: true, items: [
        { text: '命令行 CLI', link: '/reference/cli' },
        { text: '本地 API 与云端协议', link: '/reference/api' },
        { text: '两端部署契约', link: '/reference/deployment' },
        { text: '运维与凭据轮换', link: '/reference/operations' },
        { text: '安全边界', link: '/reference/security' },
        { text: '验证报告', link: '/reference/verification' },
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
    footer: { message: '文件在本地，管理也在本地。', copyright: '檐渡 Yandu · MIT License · 使用文档' },
    notFound: { title: '这条路径没有文档', quote: '可以从快速开始继续，或搜索你要解决的问题。', linkLabel: '返回文档首页', linkText: '返回文档首页' },
  },
})

import DefaultTheme from 'vitepress/theme'
import type { Theme } from 'vitepress'
import DocHome from './components/DocHome.vue'
import ReleaseDownloads from './components/ReleaseDownloads.vue'
import './custom.css'

export default {
  extends: DefaultTheme,
  enhanceApp({ app }) {
    app.component('DocHome', DocHome)
    app.component('ReleaseDownloads', ReleaseDownloads)
  },
} satisfies Theme

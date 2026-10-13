import { fileURLToPath } from 'node:url'
import { preview } from 'vite'
import { docsListenOptions, docsBasePath } from './listen-options.mjs'

const server = await preview({
  root: fileURLToPath(new URL('..', import.meta.url)),
  configFile: false,
  appType: 'mpa',
  base: docsBasePath(),
  build: { outDir: '.vitepress/dist' },
  preview: docsListenOptions(),
})
server.printUrls()

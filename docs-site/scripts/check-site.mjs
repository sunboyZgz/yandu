import { createReadStream } from 'node:fs'
import { createHash } from 'node:crypto'
import { readFile, readdir, stat } from 'node:fs/promises'
import { dirname, resolve, relative, sep } from 'node:path'
import { fileURLToPath } from 'node:url'

const site = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const dist = resolve(site, '.vitepress/dist')
const base = process.env.DOCS_BASE || '/'
const errors = new Set()
const htmlFiles = []
async function walk(directory) {
  for (const entry of await readdir(directory, { withFileTypes: true })) {
    const path = resolve(directory, entry.name)
    if (entry.isDirectory()) await walk(path)
    else if (entry.name.endsWith('.html')) htmlFiles.push(path)
  }
}
try { await walk(dist) } catch (error) {
  if (error.code === 'ENOENT') throw new Error('尚无构建产物，请先执行 npm run build。')
  throw error
}
const pages = new Map()
for (const file of htmlFiles) {
  const html = await readFile(file, 'utf8')
  // Ignore inline JavaScript and CSS; they are not HTML elements.
  const markup = html.replace(/(<(script|style)\b[^>]*>)[\s\S]*?<\/\2>/gi, '$1</$2>')
  const ids = new Set(Array.from(markup.matchAll(/\bid="([^"]+)"/g), match => match[1]))
  pages.set(file, { markup, ids })
}
const decode = value => value.replace(/&amp;/g, '&').replace(/&#39;/g, "'").replace(/&quot;/g, '"')
let links = 0
for (const [file, { markup }] of pages) {
  const path = relative(dist, file).split(sep).join('/').replace(/index\.html$/, '').replace(/\.html$/, '')
  const current = new URL(base + path, 'https://docs.local')
  for (const tag of markup.matchAll(/<[a-z][^>]*>/gi)) {
    for (const attribute of tag[0].matchAll(/\b(?:href|src)="([^"]+)"/g)) {
      const value = decode(attribute[1])
      if (/^(?:https?:|mailto:|tel:|data:|javascript:|\/\/)/i.test(value)) continue
      const url = new URL(value, current)
      if (!url.pathname.startsWith(base)) {
        errors.add(`${path || '/'}: 链接缺少基础路径 ${value}`)
        continue
      }
      const local = decodeURIComponent(url.pathname.slice(base.length))
      const target = resolve(dist, local)
      if (target !== dist && !target.startsWith(dist + sep)) {
        errors.add(`${path}: 链接超出网站目录 ${value}`)
        continue
      }
      const candidates = [target, target + '.html', resolve(target, 'index.html')]
      let found
      for (const candidate of candidates) {
        try { if ((await stat(candidate)).isFile()) { found = candidate; break } }
        catch (error) { if (error.code !== 'ENOENT' && error.code !== 'ENOTDIR') throw error }
      }
      if (!found) errors.add(`${path || '/'}: 链接或资源不存在 ${value}`)
      else if (url.hash && pages.has(found)) {
        const id = decodeURIComponent(url.hash.slice(1))
        if (!pages.get(found).ids.has(id)) errors.add(`${path || '/'}: 章节锚点不存在 ${value}`)
      }
      links++
    }
  }
}
const releases = JSON.parse(await readFile(resolve(site, '.vitepress/generated/releases.json'), 'utf8'))
const manifest = await readFile(resolve(dist, 'downloads/SHA256SUMS'), 'utf8')
let downloads = 0
for (const item of releases.filter(item => item.available)) {
  const file = resolve(dist, 'downloads', item.filename)
  const info = await stat(file)
  const digest = createHash('sha256')
  for await (const chunk of createReadStream(file)) digest.update(chunk)
  if (info.size !== item.size || digest.digest('hex') !== item.sha256) errors.add(`发布包不一致: ${item.filename}`)
  if (!manifest.includes(`${item.sha256}  ${item.filename}\n`)) errors.add(`校验清单缺少: ${item.filename}`)
  downloads++
}
if (errors.size) throw new Error(`网站检查失败:\n${Array.from(errors).join('\n')}`)
console.log(`检查通过：${htmlFiles.length} 个页面，${links} 个本地链接与资源，${downloads} 个 SHA-256 一致的下载包。基础路径 ${base}`)

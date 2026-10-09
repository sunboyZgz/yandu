import { createReadStream } from 'node:fs'
import { createHash } from 'node:crypto'
import { mkdir, readFile, stat, copyFile, writeFile, rm } from 'node:fs/promises'
import { fileURLToPath } from 'node:url'
import { dirname, resolve } from 'node:path'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '../..')
const site = resolve(root, 'docs-site')
const source = resolve(root, 'release/artifacts')
const destination = resolve(site, 'public/downloads')
const version = (await readFile(resolve(root, 'internal/model/model.go'), 'utf8')).match(/const Version = "([0-9.]+)"/)?.[1]
if (!version) throw new Error('无法读取 Yandu 版本')
const variants = [
  { platform: 'Windows x64', role: '本地客户端', suffix: 'windows-amd64.zip', format: 'ZIP', detail: '客户端、Caddy、frpc 与 PowerShell 安装脚本' },
  { platform: 'Linux x64', role: '云端安装包', suffix: 'linux-amd64.tar.gz', format: 'TAR.GZ', detail: 'Ubuntu 24.04 目标环境，含云端助手、frps 与安装脚本' },
  { platform: 'macOS ARM64', role: '本机开发与验证', suffix: 'darwin-arm64.tar.gz', format: 'TAR.GZ', detail: 'Apple Silicon 开发环境，含客户端、Caddy 与 frpc' },
]
const checksums = new Map()
try {
  for (const line of (await readFile(resolve(source, 'SHA256SUMS'), 'utf8')).trim().split('\n')) {
    const match = line.match(/^([a-f0-9]{64})\s+(.+)$/)
    if (match) checksums.set(match[2], match[1])
  }
} catch (error) { if (error.code !== 'ENOENT') throw error }

// This is a managed output directory: copy only public, explicitly named release assets.
await rm(destination, { recursive: true, force: true })
await mkdir(destination, { recursive: true })
const releases = []
for (const variant of variants) {
  const filename = `yandu-${version}-${variant.suffix}`
  const file = resolve(source, filename)
  let info
  try { info = await stat(file) } catch (error) { if (error.code !== 'ENOENT') throw error }
  if (!info) { releases.push({ ...variant, filename, version, available: false }); continue }
  const digest = createHash('sha256')
  for await (const chunk of createReadStream(file)) digest.update(chunk)
  const sha256 = digest.digest('hex')
  if (checksums.get(filename) !== sha256) throw new Error(`发布包校验失败：${filename}。请重新生成 release/artifacts。`)
  await copyFile(file, resolve(destination, filename))
  releases.push({ ...variant, filename, version, available: true, size: info.size, sha256 })
}
const available = releases.filter(item => item.available)
await writeFile(resolve(destination, 'SHA256SUMS'), available.map(item => `${item.sha256}  ${item.filename}\n`).join(''))
await mkdir(resolve(site, '.vitepress/generated'), { recursive: true })
await writeFile(resolve(site, '.vitepress/generated/releases.json'), JSON.stringify(releases, null, 2) + '\n')
console.log(`文档下载区：${available.length} 个校验通过的发布包；缺失的包会明确显示为尚未构建。`)

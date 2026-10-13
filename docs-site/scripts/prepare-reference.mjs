import { copyFile, mkdir, readFile } from 'node:fs/promises'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '../..')
const source = resolve(root, 'schemas/storage.openapi.json')
const schema = JSON.parse(await readFile(source, 'utf8'))
if (schema.openapi !== '3.0.3' || !schema.paths || !schema.info?.version) {
  throw new Error('存储 OpenAPI 契约无效，停止发布参考文件。')
}
const destination = resolve(root, 'docs-site/public/api')
await mkdir(destination, { recursive: true })
// Publish this named contract only; private state and credentials are not inputs.
await copyFile(source, resolve(destination, 'storage.openapi.json'))
console.log(`存储 API 参考：OpenAPI ${schema.openapi}，v${schema.info.version}。`)

export function docsListenOptions(env = process.env) {
  const value = env.DOCS_PORT ?? '5174'
  if (!/^[0-9]+$/.test(value) || Number(value) < 1 || Number(value) > 65535) {
    throw new Error('DOCS_PORT 必须是 1–65535 的整数。')
  }
  return { host: '127.0.0.1', port: Number(value), strictPort: true }
}

export function docsBasePath(env = process.env) {
  const base = env.DOCS_BASE || '/'
  if (!/^\/(?:[A-Za-z0-9_-]+\/)*$/.test(base)) {
    throw new Error('DOCS_BASE 必须是以 / 开始和结束的路径，例如 / 或 /yandu/。')
  }
  return base
}

<script setup lang="ts">
import { ref } from 'vue'
import { withBase } from 'vitepress'
import releases from '../../generated/releases.json'
const copied = ref('')
const copyError = ref('')
async function copyHash(name: string, hash: string) {
  try { await navigator.clipboard.writeText(hash); copied.value = name; copyError.value = ''; window.setTimeout(() => { copied.value = '' }, 2500) }
  catch { copyError.value = '剪贴板不可用，请选择下方校验值并手动复制。' }
}
</script>
<template>
  <div class="release-cards">
    <article v-for="item in releases" :key="item.filename" class="release-card">
      <div class="release-role">{{ item.role }}</div><h2>{{ item.platform }}</h2><p>{{ item.detail }}</p>
      <div class="release-meta">v{{ item.version }}<template v-if="item.available"> · {{ (item.size! / 1024 / 1024).toFixed(1) }} MB</template></div>
      <a v-if="item.available" class="docs-button primary" :href="withBase('/downloads/' + item.filename)" download>下载 {{ item.format }} <span aria-hidden="true">↓</span></a>
      <span v-else class="download-unavailable">当前工作区尚未构建此包</span>
      <div v-if="item.available" class="release-checksum"><div><span>SHA-256</span><button :aria-label="'复制 ' + item.platform + ' 的校验值'" @click="copyHash(item.filename, item.sha256!)">{{ copied === item.filename ? '已复制' : '复制' }}</button></div><code>{{ item.sha256 }}</code></div>
    </article>
  </div>
  <p v-if="copyError" role="status" class="copy-feedback">{{ copyError }}</p>
  <p class="release-manifest"><a :href="withBase('/downloads/SHA256SUMS')" download>下载全部校验值 SHA256SUMS</a><span aria-live="polite">{{ copied ? '校验值已复制到剪贴板' : '' }}</span></p>
</template>

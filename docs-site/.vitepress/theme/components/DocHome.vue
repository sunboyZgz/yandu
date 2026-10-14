<script setup lang="ts">
import { computed, ref } from 'vue'
import { withBase } from 'vitepress'
const request = ref<'storage' | 'resource' | 'page'>('storage')
const path = computed(() => request.value === 'storage' ? '/storage/v1/projects/demo/objects/…/content' : request.value === 'resource' ? '/blog/xxx/photo.png' : '/blog')
const links = [
  { name: '跑通完整链路', text: '启动云端和本机，配对后创建项目并验证读写。', link: '/guide/getting-started', icon: 'start', meta: '快速开始' },
  { name: '管理文件与媒体', text: '续传、版本、回收站、凭证，以及 HEIC、MP4 和 HLS。', link: '/guide/storage-files', icon: 'folder', meta: '日常操作' },
  { name: '接入你的业务服务', text: '运行独立业务示例，通过会话中转和 TypeScript SDK 访问文件。', link: '/guide/business-integration', icon: 'diagnose', meta: 'API 与 SDK' },
]
</script>

<template>
  <div class="docs-home">
    <section class="docs-hero" aria-labelledby="home-title">
      <div class="hero-copy">
        <div class="docs-eyebrow"><span></span>檐渡 YANDU <i>/</i> 使用手册</div>
        <h1 id="home-title">Yandu 通用存储<br>开发、部署与使用</h1>
        <p class="hero-intro">通过私有隧道，为业务提供本地文件存储。</p>
        <div class="hero-actions">
          <a class="docs-button primary" :href="withBase('/guide/getting-started')">完整链路入门 <span aria-hidden="true">↗</span></a>
          <a class="docs-button secondary" :href="withBase('/guide/environment')">环境准备</a>
        </div>
        <div class="home-development-links">
          <a class="home-development-link" :href="withBase('/guide/development')">macOS 开发模式 <span aria-hidden="true">↗</span></a>
          <a class="home-development-link" :href="withBase('/guide/development-windows')">Windows 开发模式 <span aria-hidden="true">↗</span></a>
        </div>
        <div class="hero-note"><span class="version-label">v0.2</span><span>开发版</span></div>
      </div>

      <div class="path-card">
        <div class="path-card-header"><span class="card-icon" aria-hidden="true">↗</span><strong>文件怎样抵达应用</strong><span class="path-live">{{ request === 'storage' ? '私有 API' : '只读映射' }}</span></div>
        <div class="request-switch" role="group" aria-label="查看请求去向">
          <button :class="{ selected: request === 'storage' }" :aria-pressed="request === 'storage'" @click="request = 'storage'">存储读写</button>
          <button :class="{ selected: request === 'resource' }" :aria-pressed="request === 'resource'" @click="request = 'resource'">图片资源</button>
          <button :class="{ selected: request === 'page' }" :aria-pressed="request === 'page'" @click="request = 'page'">网站页面</button>
        </div>
        <div class="request-address"><span>{{ request === 'storage' ? '业务服务请求' : '浏览器请求' }}</span><code>{{ request === 'storage' ? '私有网关' : 'example.com' }}<b>{{ path }}</b></code></div>
        <div class="path-trace" aria-live="polite">
          <template v-if="request === 'storage'">
            <div class="trace-row"><span class="trace-dot cloud"></span><div><strong>你的业务服务</strong><p>浏览器用业务会话，项目密钥留在服务端</p></div><span class="trace-tag">按项目授权</span></div>
            <div class="trace-row"><span class="trace-dot tunnel"></span><div><strong>私有网关与隧道</strong><p>流式转发，本机只需主动连出</p></div><span class="trace-tag">TLS</span></div>
            <div class="trace-row"><span class="trace-dot local"></span><div><strong>本机 SFTPGo 存储</strong><p>虚拟目录、对象与版本，原件校验后就绪</p></div><span class="trace-tag local">READY</span></div>
            <div class="file-destination"><span class="destination-icon" aria-hidden="true">▱</span><code>项目 <b>demo</b> · 原件 / 版本 / 派生物</code></div>
          </template>
          <template v-else-if="request === 'resource'">
            <div class="trace-row"><span class="trace-dot cloud"></span><div><strong>云端网站入口</strong><p>只分流已登记的资源前缀</p></div><code>/blog/xxx/</code></div>
            <div class="trace-row"><span class="trace-dot tunnel"></span><div><strong>资源隧道</strong><p>验证服务器身份，保留原始路径</p></div><span class="trace-tag">TLS</span></div>
            <div class="trace-row"><span class="trace-dot local"></span><div><strong>本地素材文件</strong><p>去掉一次页面基础路径 <code>/blog</code></p></div><span class="trace-tag local">只读</span></div>
            <div class="file-destination"><span class="destination-icon" aria-hidden="true">▱</span><code>D:/oss/blog/<b>xxx/photo.png</b></code></div>
          </template>
          <template v-else>
            <div class="trace-row"><span class="trace-dot cloud"></span><div><strong>原云端网站</strong><p>页面和业务接口保留原有行为</p></div><span class="trace-tag">页面</span></div>
            <div class="page-destination"><div class="page-window" aria-hidden="true"><i></i><i></i><i></i><span></span><span></span></div><div><strong>/blog</strong><p>由你的业务网站提供</p></div></div>
            <div class="page-behavior"><span aria-hidden="true">✓</span>文章、API 和前端构建资源继续走云端</div>
          </template>
        </div>
      </div>
    </section>

    <section class="guide-paths" aria-labelledby="paths-title">
      <div class="home-section-title"><h2 id="paths-title">使用指南</h2></div>
      <div class="guide-cards">
        <a v-for="item in links" :key="item.link" :href="withBase(item.link)" class="guide-card">
          <div class="guide-card-top"><svg v-if="item.icon === 'start'" viewBox="0 0 24 24" aria-hidden="true"><path d="m7 17 10-10M7 7h10v10M4 4v16h16"/></svg><svg v-else-if="item.icon === 'folder'" viewBox="0 0 24 24" aria-hidden="true"><path d="M3 7h7l2 2h9v11H3zM3 7V4h7l2 3"/></svg><svg v-else viewBox="0 0 24 24" aria-hidden="true"><path d="M4 12h4l3-7 3 14 3-7h3M3 3h18v18H3z"/></svg><span>{{ item.meta }}</span><b aria-hidden="true">↗</b></div>
          <h3>{{ item.name }}</h3><p>{{ item.text }}</p>
        </a>
      </div>
    </section>

    <div class="docs-home-bottom"><a :href="withBase('/guide/ports')">端口与监听配置 <b aria-hidden="true">↗</b></a></div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { withBase } from 'vitepress'
const request = ref<'resource' | 'page'>('resource')
const path = computed(() => request.value === 'resource' ? '/blog/xxx/photo.png' : '/blog')
const links = [
  { name: '第一次使用', text: '选择源码或安装包，配对两端并发布第一张图片。', link: '/guide/getting-started', icon: 'start', meta: '从这里开始' },
  { name: '管理资源项目', text: '选定目录和资源范围，保存草稿后确认应用。', link: '/guide/projects', icon: 'folder', meta: '项目与发布' },
  { name: '排查连接问题', text: '根据实际状态检查目录、隧道和网站入口。', link: '/guide/troubleshooting', icon: 'diagnose', meta: '状态与诊断' },
]
</script>

<template>
  <div class="docs-home">
    <section class="docs-hero" aria-labelledby="home-title">
      <div class="hero-copy">
        <div class="docs-eyebrow"><span></span>檐渡 YANDU <i>/</i> 使用手册</div>
        <h1 id="home-title">本地的文件，<br>网站里的资源。</h1>
        <p class="hero-intro">从安装到第一次发布，把素材目录接入现有网站。<br class="desktop-break">管理留在本机，文件留在你的目录里。</p>
        <div class="hero-actions">
          <a class="docs-button primary" :href="withBase('/guide/getting-started')">开始使用 <span aria-hidden="true">↗</span></a>
          <a class="docs-button secondary" :href="withBase('/downloads')">下载验证版</a>
        </div>
        <a class="home-development-link" :href="withBase('/guide/development')">从源码运行？查看开发模式使用 <span aria-hidden="true">↗</span></a>
        <div class="hero-note"><span class="version-label">v0.1.0</span><span>验证版本 · 支持范围与实测结果见文档</span></div>
      </div>

      <div class="path-card">
        <div class="path-card-header"><span class="card-icon" aria-hidden="true">↗</span><strong>一个 URL 的去向</strong><span class="path-live">路径分流</span></div>
        <div class="request-switch" role="group" aria-label="查看请求去向">
          <button :class="{ selected: request === 'resource' }" :aria-pressed="request === 'resource'" @click="request = 'resource'">图片资源</button>
          <button :class="{ selected: request === 'page' }" :aria-pressed="request === 'page'" @click="request = 'page'">网站页面</button>
        </div>
        <div class="request-address"><span>浏览器请求</span><code>example.com<b>{{ path }}</b></code></div>
        <div class="path-trace" aria-live="polite">
          <template v-if="request === 'resource'">
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
        <div class="path-card-footer"><svg viewBox="0 0 16 16" aria-hidden="true"><path d="m8 1 5 2v5c0 3-5 6-5 6S3 11 3 8V3zM5.5 7.5 7 9l3.5-3.5"/></svg>文件不会复制到云端</div>
      </div>
    </section>

    <section class="guide-paths" aria-labelledby="paths-title">
      <div class="home-section-title"><h2 id="paths-title">找到你现在要做的事</h2><span>按使用流程，逐步完成</span></div>
      <div class="guide-cards">
        <a v-for="item in links" :key="item.link" :href="withBase(item.link)" class="guide-card">
          <div class="guide-card-top"><svg v-if="item.icon === 'start'" viewBox="0 0 24 24" aria-hidden="true"><path d="m7 17 10-10M7 7h10v10M4 4v16h16"/></svg><svg v-else-if="item.icon === 'folder'" viewBox="0 0 24 24" aria-hidden="true"><path d="M3 7h7l2 2h9v11H3zM3 7V4h7l2 3"/></svg><svg v-else viewBox="0 0 24 24" aria-hidden="true"><path d="M4 12h4l3-7 3 14 3-7h3M3 3h18v18H3z"/></svg><span>{{ item.meta }}</span><b aria-hidden="true">↗</b></div>
          <h3>{{ item.name }}</h3><p>{{ item.text }}</p>
        </a>
      </div>
    </section>

    <section class="docs-first-step" aria-labelledby="first-title">
      <div><span class="docs-eyebrow">第一次接入</span><h2 id="first-title">连接两端，再发布第一个项目。</h2><p>每一步都有检查结果，保存草稿不会公开资源。</p></div>
      <ol class="first-step-list">
        <li><span>1</span><a :href="withBase('/guide/cloud')"><strong>准备云端</strong><small>接入现有 Nginx 网站</small></a></li>
        <li><span>2</span><a :href="withBase('/guide/getting-started#选择运行方式')"><strong>准备本机</strong><small>安装包或源码启动</small></a></li>
        <li><span>3</span><a :href="withBase('/guide/connection')"><strong>导入连接包</strong><small>确认站点和服务器指纹</small></a></li>
        <li><span>4</span><a :href="withBase('/guide/projects')"><strong>确认并应用</strong><small>验证实际资源响应</small></a></li>
      </ol>
    </section>
    <div class="docs-home-bottom"><span><svg viewBox="0 0 20 20" aria-hidden="true"><path d="m3 8 7-5 7 5M5 9h10M6 9v7m8-7v7m-7-1c2-3 4-3 6 0"/></svg>屋檐之下，也能抵达。</span><a :href="withBase('/reference/verification')">查看版本验证范围 <b aria-hidden="true">↗</b></a></div>
  </div>
</template>

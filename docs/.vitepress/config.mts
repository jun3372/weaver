import { defineConfig } from 'vitepress'

const guide = (prefix: string, t: Record<string, string>) => [
  { text: t.gettingStarted, link: `${prefix}/guide/getting-started` },
  { text: t.concepts, link: `${prefix}/guide/concepts` },
  { text: t.config, link: `${prefix}/guide/config` },
  { text: t.servers, link: `${prefix}/guide/servers` },
  { text: t.logger, link: `${prefix}/guide/logger` },
  { text: t.lifecycle, link: `${prefix}/guide/lifecycle` },
  { text: t.generate, link: `${prefix}/guide/generate` },
  { text: t.opentelemetry, link: `${prefix}/guide/opentelemetry` },
]

const sidebarLabels = (prefix: string, zh: boolean) => ({
  gettingStarted: zh ? '快速开始' : 'Getting Started',
  concepts: zh ? '核心概念:组件与依赖注入' : 'Core Concepts: Components & DI',
  config: zh ? '配置管理' : 'Configuration',
  servers: zh ? '服务组件:HTTP / TCP / UDP' : 'Server Components: HTTP / TCP / UDP',
  logger: zh ? '日志系统' : 'Logging',
  lifecycle: zh ? '生命周期管理' : 'Lifecycle Management',
  generate: zh ? 'CLI 工具' : 'CLI Tools',
  opentelemetry: zh ? 'OpenTelemetry 集成' : 'OpenTelemetry Integration',
})

export default defineConfig({
  base: '/weaver/',
  lang: 'zh-CN',
  title: 'Weaver',
  description: '轻量级 Go 组件化应用框架',
  head: [
    ['link', { rel: 'icon', type: 'image/svg+xml', href: '/weaver/logo.svg' }],
    ['link', { rel: 'preconnect', href: 'https://fonts.googleapis.com' }],
    ['link', { rel: 'preconnect', href: 'https://fonts.gstatic.com', crossorigin: '' }],
    [
      'link',
      {
        rel: 'stylesheet',
        href: 'https://fonts.googleapis.com/css2?family=IBM+Plex+Sans:wght@400;500;600;700&family=JetBrains+Mono:wght@400;500;600&display=swap',
      },
    ],
  ],

  locales: {
    root: {
      label: '简体中文',
      themeConfig: {
        nav: [
          { text: '指南', link: '/guide/getting-started' },
          { text: '示例', link: '/examples' },
          { text: 'GitHub', link: 'https://github.com/jun3372/weaver' },
        ],
        sidebar: {
          '/guide/': [{ text: '指南', items: guide('', sidebarLabels('', true)) }],
          '/': [],
        },
        outline: { label: '本页目录' },
        docFooter: { prev: '上一篇', next: '下一篇' },
        lastUpdated: { text: '最后更新于' },
        returnToTopLabel: '回到顶部',
        sidebarMenuLabel: '菜单',
        darkModeSwitchLabel: '主题',
        lightModeSwitchTitle: '切换到浅色模式',
        darkModeSwitchTitle: '切换到深色模式',
      },
    },
    en: {
      label: 'English',
      lang: 'en-US',
      link: '/en/',
      themeConfig: {
        nav: [
          { text: 'Guide', link: '/en/guide/getting-started' },
          { text: 'Examples', link: '/en/examples' },
          { text: 'GitHub', link: 'https://github.com/jun3372/weaver' },
        ],
        sidebar: {
          '/en/guide/': [{ text: 'Guide', items: guide('/en', sidebarLabels('/en', false)) }],
          '/en/': [],
        },
        outline: { label: 'On This Page' },
        docFooter: { prev: 'Previous', next: 'Next' },
        lastUpdated: { text: 'Last updated' },
        appearanceSwitchLabel: 'Appearance',
      },
    },
  },

  themeConfig: {
    socialLinks: [{ icon: 'github', link: 'https://github.com/jun3372/weaver' }],
    search: { provider: 'local' },
    footer: {
      message: 'Released under the MIT License.',
      copyright: 'Copyright © 2026 jun3372',
    },
  },

  markdown: {
    // 代码块两种模式都是深色底，统一用深色高亮主题，避免浅色模式深底配深字
    theme: 'github-dark',
  },

  lastUpdated: true,
})

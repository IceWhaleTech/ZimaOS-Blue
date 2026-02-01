import { defineConfig } from 'vitepress'

export default defineConfig({
  title: 'ZimaOS Echo',
  description: 'NAS-Native Agent Runtime',

  head: [
    ['link', { rel: 'icon', href: '/favicon.ico' }],
  ],

  locales: {
    root: {
      label: 'English',
      lang: 'en',
    },
    zh_CN: {
      label: '简体中文',
      lang: 'zh-CN',
      themeConfig: {
        nav: [
          { text: '首页', link: '/zh_CN/' },
          { text: '指南', link: '/zh_CN/guide/getting-started' },
          { text: 'API', link: '/zh_CN/api/' },
        ],
        sidebar: {
          '/zh_CN/guide/': [
            {
              text: '入门',
              items: [
                { text: '快速开始', link: '/zh_CN/guide/getting-started' },
                { text: '安装', link: '/zh_CN/guide/installation' },
                { text: '配置', link: '/zh_CN/guide/configuration' },
              ],
            },
            {
              text: '进阶',
              items: [
                { text: '架构', link: '/zh_CN/guide/architecture' },
                { text: '部署', link: '/zh_CN/guide/deployment' },
                { text: '聊天交互流程', link: '/zh_CN/guide/chat-interaction-flow' },
                { text: '开发者指南', link: '/zh_CN/guide/developer' },
                { text: '常见问题', link: '/zh_CN/guide/faq' },
                { text: 'NAS 集成', link: '/zh_CN/guide/nas-integration' },
                { text: '性能剖析', link: '/zh_CN/guide/profiling' },
              ],
            },
          ],
          '/zh_CN/api/': [
            {
              text: 'API 参考',
              items: [
                { text: '概览', link: '/zh_CN/api/' },
                { text: '健康检查', link: '/zh_CN/api/health' },
              ],
            },
          ],
        },
      },
    },
  },

  themeConfig: {
    logo: '/logo.svg',

    nav: [
      { text: 'Home', link: '/' },
      { text: 'Guide', link: '/guide/getting-started' },
      { text: 'API', link: '/api/' },
    ],

    sidebar: {
      '/guide/': [
        {
          text: 'Introduction',
          items: [
            { text: 'Getting Started', link: '/guide/getting-started' },
            { text: 'Installation', link: '/guide/installation' },
            { text: 'Configuration', link: '/guide/configuration' },
          ],
        },
        {
          text: 'Advanced',
          items: [
            { text: 'Architecture', link: '/guide/architecture' },
            { text: 'Deployment', link: '/guide/deployment' },
            { text: 'Chat Interaction Flow', link: '/guide/chat-interaction-flow' },
            { text: 'Developer Guide', link: '/guide/developer' },
            { text: 'FAQ', link: '/guide/faq' },
            { text: 'NAS Integration', link: '/guide/nas-integration' },
            { text: 'Performance Profiling', link: '/guide/profiling' },
          ],
        },
      ],
      '/api/': [
        {
          text: 'API Reference',
          items: [
            { text: 'Overview', link: '/api/' },
            { text: 'Health', link: '/api/health' },
          ],
        },
      ],
    },

    socialLinks: [
      { icon: 'github', link: 'https://github.com/IceWhaleTech/ZimaOS-Echo/server' },
    ],

    footer: {
      message: 'Released under the MIT License.',
      copyright: 'Copyright © 2024 ZimaOS Team',
    },

    search: {
      provider: 'local',
    },
  },
})

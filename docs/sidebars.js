const currentLocale = process.env.DOCUSAURUS_CURRENT_LOCALE === 'zh-CN' ? 'zh-CN' : 'en'

const englishSidebar = [
  {
    type: 'category',
    label: 'Get Started',
    items: [
      'index',
      'start/getting-started',
      'start/installation',
      'start/first-run',
      'start/quick-start',
      'start/docs-directory',
    ],
  },
  {
    type: 'category',
    label: 'Product',
    items: ['product/overview', 'product/features', 'product/architecture'],
  },
  {
    type: 'category',
    label: 'Guides',
    items: [
      'guides/settings-provider-pool',
      'guides/identity-access',
      'guides/providers-models',
      'guides/research-workflows',
      'guides/channels',
      'guides/automation',
      'guides/harness-evolution',
      'guides/extensions',
      'guides/voice-form-filling',
      'guides/admin-operations',
    ],
  },
  {
    type: 'category',
    label: 'Help',
    items: ['help/faq', 'help/troubleshooting', 'help/deployment'],
  },
]

const chineseSidebar = [
  {
    type: 'category',
    label: '开始使用',
    items: [
      'index',
      'start/getting-started',
      'start/installation',
      'start/first-run',
      'start/quick-start',
      'start/docs-directory',
    ],
  },
  {
    type: 'category',
    label: '产品',
    items: ['product/overview', 'product/features', 'product/architecture'],
  },
  {
    type: 'category',
    label: '指南',
    items: [
      'guides/settings-provider-pool',
      'guides/identity-access',
      'guides/providers-models',
      'guides/research-workflows',
      'guides/channels',
      'guides/automation',
      'guides/harness-evolution',
      'guides/extensions',
      'guides/voice-form-filling',
      'guides/admin-operations',
    ],
  },
  {
    type: 'category',
    label: '帮助',
    items: ['help/faq', 'help/troubleshooting', 'help/deployment'],
  },
]

/** @type {import('@docusaurus/plugin-content-docs').SidebarsConfig} */
const sidebars = {
  docsSidebar: currentLocale === 'zh-CN' ? chineseSidebar : englishSidebar,
}

module.exports = sidebars

/** @type {import('@docusaurus/plugin-content-docs').SidebarsConfig} */
const sidebars = {
  docsSidebar: [
    {
      type: 'category',
      label: 'Get Started',
      items: [
        'index',
        'start/getting-started',
        'start/installation',
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
    {
      type: 'category',
      label: '简体中文 入门',
      items: [
        'zh-CN/index',
        'zh-CN/start/getting-started',
        'zh-CN/start/installation',
        'zh-CN/start/quick-start',
        'zh-CN/start/docs-directory',
      ],
    },
    {
      type: 'category',
      label: '简体中文 产品',
      items: ['zh-CN/product/features'],
    },
    {
      type: 'category',
      label: '简体中文 指南',
      items: [
        'zh-CN/guides/settings-provider-pool',
        'zh-CN/guides/identity-access',
        'zh-CN/guides/providers-models',
        'zh-CN/guides/research-workflows',
        'zh-CN/guides/channels',
        'zh-CN/guides/automation',
        'zh-CN/guides/harness-evolution',
        'zh-CN/guides/extensions',
        'zh-CN/guides/voice-form-filling',
        'zh-CN/guides/admin-operations',
      ],
    },
    {
      type: 'category',
      label: '简体中文 帮助',
      items: [
        'zh-CN/help/faq',
        'zh-CN/help/troubleshooting',
        'zh-CN/help/deployment',
      ],
    },
  ],
}

module.exports = sidebars

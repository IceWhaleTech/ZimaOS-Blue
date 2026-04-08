import {createRequire} from 'node:module'
import type {Config} from '@docusaurus/types'
import type * as Preset from '@docusaurus/preset-classic'
import {themes as prismThemes} from 'prism-react-renderer'

const require = createRequire(import.meta.url)

const repository = process.env.GITHUB_REPOSITORY ?? 'IceWhaleTech/ZimaOS-Blue'
const [organizationName, projectName] = repository.split('/')
const isGitHubActions = process.env.GITHUB_ACTIONS === 'true'
const url = process.env.DOCUSAURUS_URL ?? `https://${organizationName}.github.io`
const baseUrl =
  process.env.DOCUSAURUS_BASE_URL ?? (isGitHubActions ? `/${projectName}/` : '/')

const rootLinkCompatPlugin = require('./plugins/remark-root-links.cjs')

const config: Config = {
  title: 'ZimaOS Blue',
  tagline:
    'Open-source, auditable, local-first agent runtime and toolkit for self-hosted personal AI agents.',
  favicon: 'assets/bannerX.png',

  url,
  baseUrl,
  organizationName,
  projectName,
  trailingSlash: false,

  onBrokenLinks: 'throw',
  markdown: {
    hooks: {
      onBrokenMarkdownLinks: 'throw',
    },
  },

  i18n: {
    defaultLocale: 'en',
    locales: ['en'],
  },

  presets: [
    [
      'classic',
      {
        docs: {
          path: '../docs',
          routeBasePath: '/',
          include: [
            'index.mdx',
            'start/**/*.{md,mdx}',
            'product/**/*.{md,mdx}',
            'guides/**/*.{md,mdx}',
            'help/**/*.{md,mdx}',
            'zh-CN/**/*.{md,mdx}',
          ],
          sidebarPath: require.resolve('./sidebars.js'),
          editUrl: 'https://github.com/IceWhaleTech/ZimaOS-Blue/tree/main/docs/',
          showLastUpdateAuthor: false,
          showLastUpdateTime: true,
          remarkPlugins: [rootLinkCompatPlugin],
        },
        blog: false,
        theme: {
          customCss: require.resolve('./src/css/custom.css'),
        },
      } satisfies Preset.Options,
    ],
  ],

  themeConfig: {
    image: 'assets/bannerX.png',
    navbar: {
      title: 'ZimaOS Blue',
      logo: {
        alt: 'ZimaOS Blue',
        src: 'assets/bannerX.png',
      },
      items: [
        {
          type: 'doc',
          docId: 'index',
          label: 'English',
          position: 'left',
        },
        {
          type: 'doc',
          docId: 'zh-CN/index',
          label: '中文',
          position: 'left',
        },
        {
          href: 'https://github.com/IceWhaleTech/ZimaOS-Blue',
          label: 'GitHub',
          position: 'right',
        },
        {
          href: 'https://github.com/IceWhaleTech/ZimaOS-Blue/releases',
          label: 'Releases',
          position: 'right',
        },
        {
          href: 'https://deepwiki.com/IceWhaleTech/ZimaOS-Blue',
          label: 'DeepWiki',
          position: 'right',
        },
      ],
    },
    footer: {
      style: 'dark',
      links: [
        {
          title: 'Docs',
          items: [
            {
              label: 'English',
              to: '/',
            },
            {
              label: '简体中文',
              to: '/zh-CN/',
            },
          ],
        },
        {
          title: 'Project',
          items: [
            {
              label: 'GitHub',
              href: 'https://github.com/IceWhaleTech/ZimaOS-Blue',
            },
            {
              label: 'Releases',
              href: 'https://github.com/IceWhaleTech/ZimaOS-Blue/releases',
            },
            {
              label: 'DeepWiki',
              href: 'https://deepwiki.com/IceWhaleTech/ZimaOS-Blue',
            },
          ],
        },
      ],
      copyright: `Copyright © ${new Date().getFullYear()} IceWhaleTech.`,
    },
    prism: {
      theme: prismThemes.github,
      darkTheme: prismThemes.dracula,
    },
  } satisfies Preset.ThemeConfig,
}

export default config

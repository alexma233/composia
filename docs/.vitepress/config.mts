import { defineConfig } from 'vitepress'
import { existsSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

// 说明：文档站的线上根地址（同时用于 sitemap 与 canonical/hreflang）。
// 注意：不要以 / 结尾，避免出现双斜杠。
const SITE_URL = 'https://docs.composia.xyz'

// 说明：从当前配置文件位置推导 docs 根目录，用于检查多语言页面是否存在。
const DOCS_ROOT_DIR = fileURLToPath(new URL('..', import.meta.url))
const CONTENT_DIR = path.join(DOCS_ROOT_DIR, 'content')

// 说明：把 markdown 的"文档相对路径"转换为站点 URL 路径。
// - index.md -> / 或 /foo/
// - other.md -> /other.html 或 /foo/bar.html
function docRelPathToUrlPath(docRelPath: string, routePrefix = '') {
  const normalized = docRelPath.replace(/\\/g, '/').replace(/^\/+/, '')
  const prefix = routePrefix ? `/${routePrefix.replace(/^\/+|\/+$/g, '')}` : ''

  if (normalized === 'index.md') return `${prefix}/`
  if (normalized.endsWith('/index.md')) {
    const dir = normalized.slice(0, -'/index.md'.length)
    return `${prefix}/${dir}/`
  }

  return `${prefix}/${normalized.replace(/\.md$/, '.html')}`
}

// 说明：从 pageData.relativePath 推导"当前语言"和"去语言前缀后的文档路径"。
function parseI18nRelativePath(relativePath: string) {
  const p = relativePath.replace(/\\/g, '/').replace(/^\/+/, '')

  if (p.startsWith('zh-hans/')) {
    return { localeKey: 'zh-hans', docRelPath: p.slice('zh-hans/'.length) }
  }

  // 说明：English 目录（en-us）通过 rewrites 映射到根路由；某些钩子里可能已被剥离前缀。
  if (p.startsWith('en-us/')) {
    return { localeKey: 'root', docRelPath: p.slice('en-us/'.length) }
  }

  return { localeKey: 'root', docRelPath: p }
}

// 说明：判断某语言版本的页面源文件是否存在，避免生成指向 404 的 hreflang。
function hasLocalePage(sourceDir: string, docRelPath: string) {
  return existsSync(path.join(CONTENT_DIR, sourceDir, docRelPath))
}

// https://vitepress.dev/reference/site-config
export default defineConfig({
  srcDir: 'content',

  // 说明：为文档站生成 sitemap.xml（用于搜索引擎收录）。
  // 注意：hostname 必须是线上可访问的 docs 域名，否则生成的 URL 不正确。
  sitemap: {
    hostname: SITE_URL,
  },

  // 说明：站点级 head 配置，对所有 locales 生效。
  head: [
    ['link', { rel: 'icon', type: 'image/svg+xml', href: '/favicon.svg' }],
    ['meta', { name: 'theme-color', content: '#5f67ee' }],
    [
      'script',
      {
        defer: '',
        src: 'https://umi.alexma.top/script.js',
        'data-website-id': '9c3c0e7d-fff8-4749-acd7-694abb3f7e5e',
      },
    ],
  ],

  // 说明：页面标题和描述模板
  titleTemplate: ':title | Composia',

  // 说明：为每个页面注入 canonical 与多语言 hreflang。
  // 注意：通过 frontmatter.head 注入，确保 dev 与 build 均生效。
  transformPageData(pageData, { siteConfig }) {
    const base = (siteConfig.site.base || '/').replace(/\/+$/, '/')
    const { localeKey, docRelPath } = parseI18nRelativePath(pageData.relativePath)

    // 说明：仅移除本配置生成的 link，避免误删手写的 head。
    pageData.frontmatter.head ??= []
    pageData.frontmatter.head = (pageData.frontmatter.head as any[]).filter((entry) => {
      return !(
        Array.isArray(entry) &&
        entry[0] === 'link' &&
        entry[1] &&
        typeof entry[1] === 'object' &&
        entry[1]['data-composia-seo'] === '1'
      )
    })

    const urlPath = docRelPathToUrlPath(docRelPath, localeKey === 'zh-hans' ? 'zh-hans' : '')
    const canonicalUrl = `${SITE_URL}${base === '/' ? '' : base.replace(/\/+$/g, '')}${urlPath}`

    ;(pageData.frontmatter.head as any[]).push([
      'link',
      {
        rel: 'canonical',
        href: canonicalUrl,
        'data-composia-seo': '1',
      },
    ])

    // 说明：仅在对应源文件存在时生成 hreflang。
    const enExists = hasLocalePage('en-us', docRelPath)
    const zhExists = hasLocalePage('zh-hans', docRelPath)
    const enUrl = enExists
      ? `${SITE_URL}${base === '/' ? '' : base.replace(/\/+$/g, '')}${docRelPathToUrlPath(docRelPath)}`
      : ''
    const zhUrl = zhExists
      ? `${SITE_URL}${base === '/' ? '' : base.replace(/\/+$/g, '')}${docRelPathToUrlPath(docRelPath, 'zh-hans')}`
      : ''

    if (enExists) {
      ;(pageData.frontmatter.head as any[]).push([
        'link',
        {
          rel: 'alternate',
          hreflang: 'en-US',
          href: enUrl,
          'data-composia-seo': '1',
        },
      ])
      ;(pageData.frontmatter.head as any[]).push([
        'link',
        {
          rel: 'alternate',
          hreflang: 'x-default',
          href: enUrl,
          'data-composia-seo': '1',
        },
      ])
    }

    if (zhExists) {
      ;(pageData.frontmatter.head as any[]).push([
        'link',
        {
          rel: 'alternate',
          hreflang: 'zh-Hans',
          href: zhUrl,
          'data-composia-seo': '1',
        },
      ])
    }
  },

  // 说明：为文档站启用 i18n。
  // 约定：默认语言（English）使用根路由 /；额外语言使用小写路由前缀（/zh-hans/）。
  locales: {
    root: {
      label: 'English',
      lang: 'en-US',
      description: 'A self-hosted service manager built around Docker Compose',
      themeConfig: {
        siteTitle: 'Composia',
        nav: [
          { text: 'Home', link: '/' },
          { text: 'Guide', link: '/guide/' }
        ],
        sidebar: {
          '/guide/': [
            {
              text: 'Getting Started',
              items: [
                { text: 'Introduction', link: '/guide/' },
                { text: 'Quick Start', link: '/guide/quick-start' },
                { text: 'Architecture', link: '/guide/architecture' }
              ]
            },
            {
              text: 'Core Concepts',
              items: [
                { text: 'Core Concepts', link: '/guide/core-concepts' },
                { text: 'Configuration', link: '/guide/configuration' },
                { text: 'Controller Configuration', link: '/guide/configuration/controller' },
                { text: 'Agent Configuration', link: '/guide/configuration/agent' },
                { text: 'Git Remote Sync', link: '/guide/configuration/git-sync' },
                { text: 'DNS Configuration', link: '/guide/configuration/dns' },
                { text: 'Backup Configuration', link: '/guide/configuration/backup' },
                { text: 'Secrets Configuration', link: '/guide/configuration/secrets' },
                { text: 'Service Definition', link: '/guide/service-definition' }
              ]
            },
            {
              text: 'User Guide',
              items: [
                { text: 'Deployment', link: '/guide/deployment' },
                { text: 'Networking', link: '/guide/networking' },
                { text: 'Backup & Migration', link: '/guide/backup-migrate' },
                { text: 'Operations', link: '/guide/operations' }
              ]
            },
            {
              text: 'Development',
              items: [
                { text: 'Development Guide', link: '/guide/development' },
                { text: 'API Reference', link: '/guide/api/' },
                { text: 'Controller API Reference', link: '/guide/api/controller-reference' },
                { text: 'Agent Internal API Reference', link: '/guide/api/agent-internal-reference' }
              ]
            }
          ]
        },
        footer: {
          message: 'Released under the AGPL-3.0 License.',
          copyright: 'Copyright © 2026-present Composia contributors'
        },
        editLink: {
          pattern: 'https://forgejo.alexma.top/alexma233/composia/_edit/main/docs/content/:path',
          text: 'Edit this page on Forgejo'
        },
        docFooter: {
          prev: 'Previous page',
          next: 'Next page'
        },
        outline: {
          label: 'On this page'
        },
        lastUpdated: {
          text: 'Last updated'
        },
        langMenuLabel: 'Change language',
        returnToTopLabel: 'Return to top',
        sidebarMenuLabel: 'Menu',
        darkModeSwitchLabel: 'Appearance'
      }
    },
    'zh-hans': {
      label: '简体中文',
      lang: 'zh-Hans',
      description: '基于 Docker Compose 的自托管服务管理器',
      themeConfig: {
        siteTitle: 'Composia',
        nav: [
          { text: '首页', link: '/zh-hans/' },
          { text: '指南', link: '/zh-hans/guide/' }
        ],
        sidebar: {
          '/zh-hans/guide/': [
            {
              text: '入门',
              items: [
                { text: '简介', link: '/zh-hans/guide/' },
                { text: '快速开始', link: '/zh-hans/guide/quick-start' },
                { text: '架构概览', link: '/zh-hans/guide/architecture' }
              ]
            },
            {
              text: '核心概念',
              items: [
                { text: '核心概念', link: '/zh-hans/guide/core-concepts' },
                { text: '配置指南', link: '/zh-hans/guide/configuration' },
                { text: 'Controller 配置', link: '/zh-hans/guide/configuration/controller' },
                { text: 'Agent 配置', link: '/zh-hans/guide/configuration/agent' },
                { text: 'Git 远端同步', link: '/zh-hans/guide/configuration/git-sync' },
                { text: 'DNS 配置', link: '/zh-hans/guide/configuration/dns' },
                { text: '备份配置', link: '/zh-hans/guide/configuration/backup' },
                { text: 'Secrets 配置', link: '/zh-hans/guide/configuration/secrets' },
                { text: '服务定义', link: '/zh-hans/guide/service-definition' }
              ]
            },
            {
              text: '功能指南',
              items: [
                { text: '部署管理', link: '/zh-hans/guide/deployment' },
                { text: '网络配置', link: '/zh-hans/guide/networking' },
                { text: '备份与迁移', link: '/zh-hans/guide/backup-migrate' },
                { text: '日常运维', link: '/zh-hans/guide/operations' }
              ]
            },
            {
              text: '开发',
              items: [
                { text: '开发指南', link: '/zh-hans/guide/development' },
                { text: 'API 参考', link: '/zh-hans/guide/api/' },
                { text: 'Controller API Reference', link: '/guide/api/controller-reference' },
                { text: 'Agent Internal API Reference', link: '/guide/api/agent-internal-reference' }
              ]
            }
          ]
        },
        footer: {
          message: '基于 AGPL-3.0 许可发布。',
          copyright: 'Copyright © 2026-present Composia 贡献者'
        },
        editLink: {
          pattern: 'https://forgejo.alexma.top/alexma233/composia/_edit/main/docs/content/:path',
          text: '在 Forgejo 上编辑此页'
        },
        docFooter: {
          prev: '上一页',
          next: '下一页'
        },
        outline: {
          label: '本页内容'
        },
        lastUpdated: {
          text: '最后更新'
        },
        langMenuLabel: '切换语言',
        returnToTopLabel: '回到顶部',
        sidebarMenuLabel: '菜单',
        darkModeSwitchLabel: '外观'
      }
    }
  },

  // 说明：按目录写作并保持路由一致：
  // - docs/content/en-us/** -> /
  // - docs/content/zh-hans/** -> /zh-hans/
  rewrites: {
    'en-us/:rest*': ':rest*',
    'zh-hans/:rest*': 'zh-hans/:rest*',
  },

  themeConfig: {
    // https://vitepress.dev/reference/default-theme-config
    logo: '/logo.svg',
    socialLinks: [
      { icon: 'forgejo', link: 'https://forgejo.alexma.top/alexma233/composia' }
    ],
    search: {
      provider: 'local',
      options: {
        translations: {
          button: {
            buttonText: 'Search',
            buttonAriaLabel: 'Search'
          },
          modal: {
            displayDetails: 'Display detailed list',
            resetButtonTitle: 'Reset search',
            backButtonTitle: 'Close search',
            noResultsText: 'No results for',
            footer: {
              selectText: 'to select',
              selectKeyAriaLabel: 'enter',
              navigateText: 'to navigate',
              navigateUpKeyAriaLabel: 'up arrow',
              navigateDownKeyAriaLabel: 'down arrow',
              closeText: 'to close',
              closeKeyAriaLabel: 'escape'
            }
          }
        }
      }
    }
  }
})

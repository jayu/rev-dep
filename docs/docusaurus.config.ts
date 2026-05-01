import {themes as prismThemes} from 'prism-react-renderer';
import type {Config} from '@docusaurus/types';
import type * as Preset from '@docusaurus/preset-classic';

// This runs in Node.js - Don't use client-side code here (browser APIs, JSX...)

const siteTitle = 'Rev-dep | High-Speed Dependency Graph Analysis for JS/TS Monorepos';
const siteDescription =
  'Enforce module boundaries, find circular imports, dead files, unused exports, and dependency issues in one fast CLI. Audit 500k+ LoC in around 500ms.';

const config: Config = {
  title: siteTitle,
  tagline: siteDescription,
  favicon: 'img/favicon.ico',

  // Future flags, see https://docusaurus.io/docs/api/docusaurus-config#future
  future: {
    v4: true, // Improve compatibility with the upcoming Docusaurus v4
  },

  // GitHub Pages can publish this either on the custom domain or on the
  // default project-pages URL.
  url: process.env.DOCS_URL ?? 'https://rev-dep.com',
  baseUrl: process.env.DOCS_BASE_URL ?? '/',

  // GitHub pages deployment config.
  organizationName: 'jayu',
  projectName: 'rev-dep',
  deploymentBranch: 'gh-pages',
  trailingSlash: false,

  onBrokenLinks: 'throw',

  // Even if you don't use internationalization, you can use this field to set
  // useful metadata like html lang. For example, if your site is Chinese, you
  // may want to replace "en" with "zh-Hans".
  i18n: {
    defaultLocale: 'en',
    locales: ['en'],
  },

  presets: [
    [
      'classic',
      {
        docs: {
          sidebarPath: './sidebars.ts',
          // Please change this to your repo.
          // Remove this to remove the "edit this page" links.
          editUrl: 'https://github.com/jayu/rev-dep/blob/master/docs',
        },
        blog: {
          showReadingTime: true,
          feedOptions: {
            type: ['rss', 'atom'],
            xslt: true,
          },
          onInlineTags: 'warn',
          onInlineAuthors: 'warn',
          onUntruncatedBlogPosts: 'warn',
        },
        theme: {
          customCss: './src/css/custom.css',
        },
        gtag: {
          trackingID: 'G-7ZM35PJ1K4',
          anonymizeIP: true,
        },
      } satisfies Preset.Options,
    ],
  ],

  plugins: [
    function gtagFallbackPlugin() {
      return {
        name: 'gtag-fallback-plugin',
        getClientModules() {
          // Keep local navigation safe when the gtag script is blocked or not initialized yet.
          return ['./src/gtagFallback.ts'];
        },
      };
    },
    [
      '@cmfcmf/docusaurus-search-local',
      {
        indexBlog: false,
        indexPages: false,
      },
    ],
  ],

  themeConfig: {
    // Replace with your project's social card
    image: 'img/og-logo.jpg',
    metadata: [
      {name: 'description', content: siteDescription},
      {property: 'og:type', content: 'website'},
      {property: 'og:title', content: siteTitle},
      {property: 'og:description', content: siteDescription},
      {name: 'twitter:card', content: 'summary_large_image'},
      {name: 'twitter:title', content: siteTitle},
      {name: 'twitter:description', content: siteDescription},
    ],
    colorMode: {
      respectPrefersColorScheme: true,
    },
    navbar: {
      title: 'Rev-dep',
      items: [
        {
          type: 'docSidebar',
          sidebarId: 'tutorialSidebar',
          position: 'left',
          label: 'Docs',
        },
        {type: 'search', position: 'right'},
        // {to: '/blog', label: 'Blog', position: 'left'},
        {type:'html', position:'right', value:'<iframe src="https://ghbtns.com/github-btn.html?user=jayu&repo=rev-dep&type=star&count=true" frameborder="0" scrolling="0" width="120" height="20" title="GitHub" style="margin-top: 8px;"></iframe>'},
      ],
    },
    footer: {
      style: 'dark',
      links: [
        {
          title: 'Docs',
          items: [
            {
              label: 'Docs',
              to: '/docs/intro',
            },
          ],
        },
        
        {
          title: 'More',
          items: [
            
            {
              label: 'Rev-Dep GitHub',
              href: 'https://github.com/jayu/rev-dep',
            },
          ],
        },
        {
          title: 'Jayu\'s Other Projects',
          items: [
            {
              label: 'CodeQue',
              href: 'https://codeque.co',
            },
            {
              label: 'Jayu\'s GitHub',
              href: 'https://github.com/jayu',
            },
          ],
        },
      ],
      copyright: `Copyright © ${new Date().getFullYear()} Jakub Mazurek, Docs Built with Docusaurus.`,
    },
    prism: {
      theme: prismThemes.github,
      darkTheme: prismThemes.dracula,
    },
  } satisfies Preset.ThemeConfig,
};

export default config;

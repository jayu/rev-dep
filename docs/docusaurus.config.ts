import {themes as prismThemes} from 'prism-react-renderer';
import type {Config} from '@docusaurus/types';
import type * as Preset from '@docusaurus/preset-classic';

// This runs in Node.js - Don't use client-side code here (browser APIs, JSX...)

const siteTitle = 'Rev-dep - High-Speed Dependency Graph Analysis for JS/TS Monorepos';
const siteDescription =
  'Enforce module boundaries, find circular imports, dead files, unused exports, and dependency issues in one fast CLI. Audit 500k+ LoC in around 150ms.';

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

  // Preload both faces. Without this the browser only discovers the fonts
  // after parsing CSS, so the first paint uses the fallback and the headline
  // re-breaks when Geist swaps in. Metric-matched fallbacks get the widths
  // close, but a line ending near a wrap boundary can still flip.
  headTags: [
    {
      tagName: 'link',
      attributes: {
        rel: 'preload',
        href: '/fonts/Geist-Variable.woff2',
        as: 'font',
        type: 'font/woff2',
        crossorigin: 'anonymous',
      },
    },
    {
      tagName: 'link',
      attributes: {
        rel: 'preload',
        href: '/fonts/GeistMono-Variable.woff2',
        as: 'font',
        type: 'font/woff2',
        crossorigin: 'anonymous',
      },
    },
    // Hide the document until the webfonts resolve, then fade it in.
    // Must be inline in <head> so it applies before the first paint - React
    // cannot do this, because hydration runs after the HTML has painted.
    //
    // A data attribute, NOT a class: Docusaurus manages <html class> through
    // Helmet and replaces it wholesale on render, which silently wiped an
    // earlier class-based version of this.
    // src/fontsReady.ts removes the class when document.fonts is ready; the
    // timeout here is the safety net if that never happens.
    {
      tagName: 'script',
      attributes: {},
      innerHTML: `(function(){
  var el = document.documentElement;
  el.setAttribute('data-rd-boot', '');
  setTimeout(function () { el.removeAttribute('data-rd-boot'); }, 2500);
})();`,
    },
    // Without JavaScript the class is never added, so the page is simply
    // visible from the start.
    {
      tagName: 'noscript',
      attributes: {},
      innerHTML: '<style>html[data-rd-boot] .rd-hold,html[data-rd-boot] .rd-enter{opacity:1 !important;transform:none !important;}</style>',
    },
  ],

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
          // v3 is the `current` version and is edited in place in `docs/`, so it
          // keeps the unprefixed /docs/* URLs that are already indexed by search
          // engines. v2 is a frozen snapshot served under /docs/2/*.
          lastVersion: 'current',
          versions: {
            current: {label: '3.x', badge: true},
            '2': {
              label: '2.x',
              path: '2',
              banner: 'unmaintained',
              // v2 pages are near-duplicates of the canonical /docs/* v3 pages.
              // Keeping them out of the index avoids splitting ranking authority.
              noIndex: true,
            },
          },
          // Must be the function form: a static string would point the "edit this
          // page" link on a v2 page at the v3 source file.
          editUrl: ({versionDocsDirPath, docPath}) =>
            `https://github.com/jayu/rev-dep/blob/master/docs/${versionDocsDirPath}/${docPath}`,
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
          return ['./src/gtagFallback.ts', './src/fontsReady.ts'];
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
    [
      '@docusaurus/plugin-client-redirects',
      {
        // Short links used by `rev-dep config init` output.
        redirects: [
          {from: '/init/monorepo', to: '/docs/monorepo-integration-guide'},
          {from: '/init/single-workspace', to: '/docs/single-workspace-integration-guide'},
          {from: '/troubleshooting', to: '/docs/troubleshooting'},
        ],
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
      // The mark, not the full wordmark: the wordmark's strokes are hairlines
      // at navbar size and read as washed out. The title stays as real text so
      // it renders crisply in Geist at any zoom.
      logo: {
        alt: 'Rev-dep logo',
        src: 'img/logo-mark-card.png',
        width: 30,
        height: 30,
      },
      items: [
        {
          type: 'docSidebar',
          sidebarId: 'tutorialSidebar',
          position: 'left',
          label: 'Docs',
        },
        {type: 'docsVersionDropdown', position: 'right'},
        {type: 'search', position: 'right'},
        // {to: '/blog', label: 'Blog', position: 'left'},
        // All three glyphs are 24px wide to match the colour-mode toggle: with
        // equal 32px buttons, a different glyph width is what makes the
        // whitespace between them look uneven.
        // Plain links, not third-party widgets. The GitHub star button was an
        // <iframe> from ghbtns.com: unstylable, an extra network request, and
        // it shifted the navbar as it loaded. Both marks are inline SVG using
        // currentColor so they match the other navbar items.
        {
          type: 'html',
          position: 'right',
          value:
            '<a class="navbar__icon-link" href="https://github.com/jayu/rev-dep" target="_blank" rel="noopener noreferrer" aria-label="Rev-dep on GitHub">' +
            '<svg viewBox="0 0 16 16" width="24" height="24" fill="currentColor" aria-hidden="true">' +
            '<path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27s1.36.09 2 .27c1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.01 8.01 0 0 0 16 8c0-4.42-3.58-8-8-8Z"/>' +
            '</svg></a>',
        },
        {
          type: 'html',
          position: 'right',
          value:
            '<a class="navbar__icon-link" href="https://www.npmjs.com/package/rev-dep" target="_blank" rel="noopener noreferrer" aria-label="rev-dep on npm">' +
            // The square tile, not the wordmark. The wordmark is 24x9 - it can
            // never read as the same size as the two circles it sits between -
            // and its "m" ends flush against the slab edge while the "n" has a
            // clear margin, which reads as the glyph being clipped. This is the
            // standard npm mark: 24x24, ink to all four edges, symmetric.
            '<svg viewBox="0 0 24 24" width="24" height="24" fill="currentColor" aria-hidden="true">' +
            '<path d="M1.763 0C.786 0 0 .786 0 1.763v20.474C0 23.214.786 24 1.763 24h20.474c.977 0 1.763-.786 1.763-1.763V1.763C24 .786 23.214 0 22.237 0zM5.13 5.323l13.837.019-.009 13.836h-3.464l.01-10.382h-3.456L12.04 19.17H5.113z"/>' +
            '</svg></a>',
        },
      ],
    },
    footer: {
      style: 'dark',
      links: [
        {
          title: 'Get started',
          items: [
            {label: 'Overview', to: '/docs/intro'},
            {label: 'Installation', to: '/docs/installation'},
            {label: 'Monorepo integration guide', to: '/docs/monorepo-integration-guide'},
            {label: 'Single package guide', to: '/docs/single-workspace-integration-guide'},
          ],
        },
        {
          title: 'Features',
          items: [
            {label: 'Config-based checks', to: '/docs/config-based-checks/overview'},
            {label: 'Module boundaries', to: '/docs/config-based-checks/checks/module-boundaries'},
            {label: 'Exploratory toolkit', to: '/docs/exploratory-toolkit/overview'},
            {label: 'Aliases & resolution', to: '/docs/other-concepts-and-features/module-resolution-and-path-aliases'},
          ],
        },
        {
          title: 'Compare & migrate',
          items: [
            {label: 'Comparison overview', to: '/docs/comparison-with-other-tools/overview'},
            {label: 'knip vs rev-dep', to: '/docs/comparison-with-other-tools/knip-vs-rev-dep'},
            {label: 'madge vs rev-dep', to: '/docs/comparison-with-other-tools/madge-vs-rev-dep'},
            {label: 'Migration guides', to: '/docs/migrating-from-other-tools/overview'},
          ],
        },
        {
          title: 'Reference',
          items: [
            {label: 'CLI reference', to: '/docs/cli-reference/overview'},
            {label: 'Glossary', to: '/docs/glossary'},
            {label: 'Telemetry', to: '/docs/telemetry'},
            {label: 'v3 breaking changes', to: '/docs/upgrade-guides/v3-breaking-changes'},
          ],
        },
        {
          title: 'More',
          items: [
            {label: 'GitHub', href: 'https://github.com/jayu/rev-dep'},
            {label: 'npm package', href: 'https://www.npmjs.com/package/rev-dep'},
            {label: 'CodeQue - code search', href: 'https://codeque.co'},
            {label: "Jayu's GitHub", href: 'https://github.com/jayu'},
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

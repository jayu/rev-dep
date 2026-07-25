/**
 * The "will it work on MY repo?" section.
 *
 * Each entry names the real-world situation first and the config key second -
 * a reader recognises "we use pnpm" faster than "nodeModulesResolution".
 */

export type CompatibilityItem = {
  title: string;
  body: string;
  /** Config key or flag, shown in monospace. */
  key?: string;
  docs: string;
};

export const compatibilityItems: CompatibilityItem[] = [
  {
    title: 'Source packages and compiled packages',
    body: 'Most monorepos mix both. rev-dep traces into packages consumed as source, and treats built packages as external boundaries - exactly as your bundler does. Choose per package, not all or nothing.',
    key: 'followMonorepoPackages',
    docs: '/docs/other-concepts-and-features/following-monorepo-packages',
  },
  {
    title: 'pnpm strictness, or npm and yarn hoisting',
    body: 'Dependencies can be validated against the consuming package, or against the package that owns each file. The second matches how pnpm actually resolves, so a followed package declaring its own deps stops looking like a missing dependency.',
    key: 'nodeModulesResolution',
    docs: '/docs/other-concepts-and-features/node-modules-resolution',
  },
  {
    title: 'Path aliases and exports maps',
    body: 'tsconfig paths and baseUrl, package.json imports and exports maps, and conditional targets - resolved with the same rules your bundler and Node use.',
    key: 'conditionNames',
    docs: '/docs/other-concepts-and-features/module-resolution-and-path-aliases',
  },
  {
    title: 'Globs that behave like .gitignore',
    body: 'Every path pattern - entry points, ignores, boundaries - follows gitignore matching, negation with ! included. Patterns you already wrote work the same here.',
    key: 'ignoreFiles',
    docs: '/docs/other-concepts-and-features/glob-patterns',
  },
  {
    title: 'More than .ts and .js',
    body: 'TypeScript and JavaScript in every extension, plus the script blocks of .vue and .svelte components. Asset imports resolve instead of showing up as errors, and you can add your own extensions.',
    key: 'customAssetExtensions',
    docs: '/docs/other-concepts-and-features/supported-file-types',
  },
];

/**
 * Stated on the page for the same reason the "what it does not do" card exists:
 * anyone who hits these finds out in five minutes anyway, and naming them is
 * what makes the rest of the list believable.
 */
export const resolutionLimits: string[] = [
  'An alias with several fallback targets resolves against the first one only.',
  'Alias targets must be relative paths; non-relative targets are skipped.',
  'Vue and Svelte files are parsed for their script blocks, not their templates.',
];

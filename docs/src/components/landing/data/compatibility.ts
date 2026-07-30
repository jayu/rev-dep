/**
 * The "will it work on MY repo?" section.
 *
 * Each entry names the real-world situation first and the config key second -
 * a reader recognises "we use pnpm" faster than "nodeModulesResolution".
 *
 * The framing is deliberately "there is a setting for this", not "it handles
 * this". rev-dep does not infer a project's resolution strategy, and copy that
 * implies otherwise sets up a bad first run: the reader hits missing-dependency
 * noise on day one and concludes the tool is wrong, when the config had simply
 * not been pointed at their setup yet. Naming the work is what makes the payoff
 * believable.
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
    body: 'Most monorepos mix both. You declare which packages rev-dep follows into their source and which it treats as an external boundary - the same distinction your bundler makes, set per package rather than guessed.',
    key: 'followMonorepoPackages',
    docs: '/docs/other-concepts-and-features/following-monorepo-packages',
  },
  {
    title: 'pnpm strictness, or npm and yarn hoisting',
    body: 'Choose whether dependencies are validated against the consuming package or against the package that owns each file. The second matches how pnpm actually resolves; the first suits a hoisted npm or yarn tree. Picking the wrong one is the usual source of false missing-dependency reports.',
    key: 'nodeModulesResolution',
    docs: '/docs/other-concepts-and-features/node-modules-resolution',
  },
  {
    title: 'Path aliases and exports maps',
    body: 'tsconfig paths and baseUrl, package.json imports and exports maps, and conditional targets are read from your project rather than reimplemented. Which condition names apply is yours to set - a browser build and a test run do not resolve the same way.',
    key: 'conditionNames',
    docs: '/docs/other-concepts-and-features/module-resolution-and-path-aliases',
  },
  {
    title: 'Globs that behave like .gitignore',
    body: 'Every path pattern - entry points, ignores, boundaries - uses gitignore matching, negation with ! included. One syntax across the whole config, and it is one you already know.',
    key: 'ignoreFiles',
    docs: '/docs/other-concepts-and-features/glob-patterns',
  },
  {
    title: 'Different rules per package',
    body: 'Checks are configured per workspace, so a package that is not ready keeps them off while the rest of the monorepo is enforced. Tuning does not have to happen everywhere at once - get one workspace right, then move to the next.',
    key: 'workspaces',
    docs: '/docs/config-based-checks/config-file-structure',
  },
  {
    title: 'More than .ts and .js',
    body: 'TypeScript and JavaScript in every extension, plus the script blocks of .vue and .svelte components. Asset imports resolve once their extensions are declared - the common ones ship as defaults, anything else your project imports is one line of config.',
    key: 'customAssetExtensions',
    docs: '/docs/other-concepts-and-features/supported-file-types',
  },
];

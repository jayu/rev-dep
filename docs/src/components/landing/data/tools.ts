/**
 * The tools rev-dep consolidates. Every row links to the comparison page that
 * already exists in the docs, so the landing page feeds them traffic.
 */

export type ToolRow = {
  name: string;
  covers: string;
  href: string;
};

export const replacedTools: ToolRow[] = [
  { name: 'knip', covers: 'unused files, exports, dependencies', href: '/docs/comparison-with-other-tools/knip-vs-rev-dep' },
  { name: 'madge', covers: 'circular dependencies', href: '/docs/comparison-with-other-tools/madge-vs-rev-dep' },
  { name: 'dependency-cruiser', covers: 'dependency rules, circular, orphans', href: '/docs/comparison-with-other-tools/dependency-cruiser-vs-rev-dep' },
  { name: 'depcheck', covers: 'unused & missing dependencies', href: '/docs/comparison-with-other-tools/depcheck-vs-rev-dep' },
  { name: 'dpdm', covers: 'circular dependencies, unused files', href: '/docs/comparison-with-other-tools/dpdm-vs-rev-dep' },
  { name: 'ts-prune', covers: 'unused exports', href: '/docs/comparison-with-other-tools/ts-prune-vs-rev-dep' },
  { name: 'ts-unused-exports', covers: 'unused exports', href: '/docs/comparison-with-other-tools/ts-unused-exports-vs-rev-dep' },
  { name: 'unimported', covers: 'unused files, unresolved imports', href: '/docs/comparison-with-other-tools/unimported-vs-rev-dep' },
  { name: 'skott', covers: 'circular deps, unused files & deps', href: '/docs/comparison-with-other-tools/skott-vs-rev-dep' },
  { name: 'good-fences', covers: 'directory import boundaries', href: '/docs/comparison-with-other-tools/good-fences-vs-rev-dep' },
  { name: 'sheriff', covers: 'module boundaries & dependency rules', href: '/docs/comparison-with-other-tools/sheriff-vs-rev-dep' },
  { name: 'eslint-plugin-import', covers: 'import-graph ESLint rules', href: '/docs/comparison-with-other-tools/eslint-plugin-import-vs-rev-dep' },
  { name: 'npm-check', covers: 'unused dependencies', href: '/docs/comparison-with-other-tools/npm-check-vs-rev-dep' },
];

/**
 * Stated plainly on the page. A tool that admits its limits gets believed
 * about everything else.
 */
export const limitations: string[] = [
  'No dependency-graph visualisation - rev-dep reports, it does not draw.',
  'File, export and dependency granularity - not unused class members, enum members or namespace-level exports.',
  'No plugin ecosystem - configuration covers the framework and resolution setups listed in the docs.',
];

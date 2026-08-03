/**
 * The tools rev-dep consolidates. Every row links to the comparison page that
 * already exists in the docs, so the landing page feeds them traffic.
 */

export type ToolRow = {
  name: string;
  covers: string;
  href: string;
};

/**
 * One vocabulary, one order, for every row.
 *
 * The point of this list is that a reader can scan down it and see which tools
 * overlap. That only works if the same capability is always called the same
 * thing and always appears in the same position - otherwise "orphans" in one
 * row and "unused files" in the next look like two different features, and a
 * reader has to re-parse every row instead of comparing them.
 *
 * The order runs dead code -> dependency correctness -> structure, matching how
 * the checks section above groups rev-dep's own checks:
 *
 *   1. unused files
 *   2. unused exports
 *   3. duplicated code
 *   4. unused dependencies
 *   5. missing dependencies
 *   6. unresolved imports
 *   7. circular imports
 *   8. module boundaries
 *
 * Each row lists only the entries that tool actually covers, in that order.
 * Capabilities are taken from each tool's own comparison page, so this list and
 * the "At a glance" tables there cannot drift apart.
 */
export const replacedTools: ToolRow[] = [
  { name: 'knip', covers: 'unused files, unused exports, unused dependencies, circular imports', href: '/docs/comparison-with-other-tools/knip-vs-rev-dep' },
  { name: 'madge', covers: 'unused files, circular imports', href: '/docs/comparison-with-other-tools/madge-vs-rev-dep' },
  { name: 'jscpd', covers: 'duplicated code', href: '/docs/comparison-with-other-tools/jscpd-vs-rev-dep' },
  { name: 'dependency-cruiser', covers: 'unused files, circular imports, module boundaries', href: '/docs/comparison-with-other-tools/dependency-cruiser-vs-rev-dep' },
  { name: 'depcheck', covers: 'unused dependencies, missing dependencies', href: '/docs/comparison-with-other-tools/depcheck-vs-rev-dep' },
  { name: 'dpdm', covers: 'unused files, circular imports', href: '/docs/comparison-with-other-tools/dpdm-vs-rev-dep' },
  { name: 'ts-prune', covers: 'unused exports', href: '/docs/comparison-with-other-tools/ts-prune-vs-rev-dep' },
  { name: 'ts-unused-exports', covers: 'unused files, unused exports', href: '/docs/comparison-with-other-tools/ts-unused-exports-vs-rev-dep' },
  { name: 'unimported', covers: 'unused files, unused dependencies, unresolved imports', href: '/docs/comparison-with-other-tools/unimported-vs-rev-dep' },
  { name: 'skott', covers: 'unused files, unused dependencies, circular imports', href: '/docs/comparison-with-other-tools/skott-vs-rev-dep' },
  { name: 'good-fences', covers: 'module boundaries', href: '/docs/comparison-with-other-tools/good-fences-vs-rev-dep' },
  { name: 'sheriff', covers: 'module boundaries', href: '/docs/comparison-with-other-tools/sheriff-vs-rev-dep' },
  { name: 'eslint-plugin-import', covers: 'unused exports, unresolved imports, circular imports, module boundaries', href: '/docs/comparison-with-other-tools/eslint-plugin-import-vs-rev-dep' },
  { name: 'npm-check', covers: 'unused dependencies', href: '/docs/comparison-with-other-tools/npm-check-vs-rev-dep' },
];

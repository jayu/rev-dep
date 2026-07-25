/**
 * The FAQ answers the objections that actually block adoption - not whether
 * the tool is free or supports TypeScript. Each entry is something a skeptical
 * engineer asks before they will run the install command.
 */

export type FaqEntry = {
  question: string;
  /** Paragraphs. Kept as an array so long answers stay readable in source. */
  answer: string[];
};

export const faqEntries: FaqEntry[] = [
  {
    question: 'Why is it so much faster? What is the catch?',
    answer: [
      'It is a native binary, so there is no Node startup and no JIT warm-up. It uses a purpose-built parser that extracts only imports and exports - it never builds a full AST, never walks function bodies and never runs a type-checker, so most of every file is skipped because most of it is irrelevant to a dependency graph.',
      'Everything runs in parallel: file discovery, parsing, module resolution and check evaluation. And the graph is built once and shared by every enabled check, so twelve checks cost roughly what one costs.',
      'The catch, stated plainly: this is resolution and graph analysis, not type-aware analysis. That is exactly why it is fast.',
    ],
  },
  {
    question: 'What about false positives?',
    answer: [
      'Once the config is right, there are none. An import either resolves or it does not - there is no heuristic doing guesswork behind the scenes.',
      'What gets experienced as a false positive is almost always the config describing the wrong graph: an entry point that was never declared, so live code looks orphaned; an asset extension the resolver does not recognise; a file reached only through tooling the config was not told about. If a finding looks wrong, the graph is telling you something true about a configuration that is incomplete.',
    ],
  },
  {
    question: 'Will it understand my setup?',
    answer: [
      'tsconfig path aliases, package.json exports and imports maps, condition names, cross-package resolution in pnpm/yarn/npm workspaces, .vue and .svelte script blocks, and custom asset extensions.',
      'The supported surface is documented in full rather than implied - if something is not covered, the docs say so.',
    ],
  },
  {
    question: 'Do I have to rip out ESLint and knip?',
    answer: [
      'No, but there is something worth deleting. If you use eslint-plugin-import, drop the import/no-cycle and import/no-unused-modules rules: they are typically the slowest rules in a config, because ESLint re-resolves the import graph for every file. Removing them makes ESLint faster and improves detection, since a per-file linter structurally cannot see that a file is unreachable or that a cycle spans eight files across three packages. Keep ESLint for what only it can do - inline feedback as you type.',
      'knip overlaps with rev-dep on unused files, exports and dependencies, so running both means building the graph twice for the same findings. Where knip is genuinely different is finer granularity: unused class and enum members, namespace-level exports, duplicate exports. If you need that, run rev-dep on every commit and knip occasionally.',
    ],
  },
  {
    question: 'Will it slow down my CI?',
    answer: [
      'It should do the opposite, and not only because each check is faster. A typical stack runs several tools in sequence, each paying its own Node startup, its own file discovery and its own graph build - the same expensive work repeated. rev-dep builds the graph once and runs every enabled check across it in parallel.',
      'It also installs no dependency tree of its own: the npm package is a launcher plus one prebuilt binary, so the install step shrinks too.',
    ],
  },
  {
    question: 'What does it not do?',
    answer: [
      'It does not draw dependency graphs - it reports, it does not visualise. It analyses at file, export and dependency granularity, so it will not find an unused class member or enum member. And it has no plugin ecosystem; the framework and resolution setups it understands are the ones listed in the docs.',
    ],
  },
];

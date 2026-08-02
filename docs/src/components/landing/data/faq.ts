/**
 * The FAQ answers the objections that actually block adoption - not whether
 * the tool is free or supports TypeScript. Each entry is something a skeptical
 * engineer asks before they will run the install command.
 */

export type FaqEntry = {
  /**
   * Stable slug for analytics. Deliberately not derived from the question:
   * questions get reworded, and a reworded question should stay the same row
   * in a report rather than appearing as a new one.
   */
  id: string;
  question: string;
  /** Paragraphs. Kept as an array so long answers stay readable in source. */
  answer: string[];
  /** Optional call to action under the answer, when the honest reply is "tell us". */
  link?: { label: string; to: string };
};

export const faqEntries: FaqEntry[] = [
  {
    id: 'why_faster',
    question: 'Why is it so much faster? What is the catch?',
    answer: [
      'It is a native binary, so there is no Node startup and no JIT warm-up. It uses a purpose-built parser that extracts only imports and exports - it never builds a full AST, never walks function bodies and never runs a type-checker, so most of every file is skipped because most of it is irrelevant to a dependency graph.',
      'Everything runs in parallel: file discovery, parsing, module resolution and check evaluation. And the graph is built once and shared by every enabled check, so several checks within multiple workspaces cost roughly what one costs.',
    ],
  },
  {
    id: 'performance_limits',
    question: 'What are the performance limitations?',
    answer: [
      'In practice there is no size at which it stops being fast. A large monorepo evaluates in subsecond time on modest hardware.',
      'The one thing that might slow it down is being pointed at files that were never meant to be read: committed build output. A single minified bundle can be longer than the rest of the source put together, and the parser has no way to know it is looking at generated code.',
      'Gitignored files are skipped automatically, so this only bites when dist or build folders are committed to the repository. Add them to ignoreFiles and the analysis goes back to full speed.',
    ],
    link: {
      label: 'How ignoring files works →',
      to: '/docs/other-concepts-and-features/ignoring-files',
    },
  },
  {
    id: 'false_positives',
    question: 'What about false positives?',
    answer: [
      'Once the config is right, there are none. An import either resolves or it does not - there is no heuristic doing guesswork behind the scenes.',
      'What gets experienced as a false positive is almost always the config describing the wrong graph: an entry point that was never declared, so live code looks orphaned; an asset extension the resolver does not recognise; a file reached only through tooling the config was not told about. If a finding looks wrong, the graph is telling you something true about a configuration that is incomplete.',
    ],
  },
  {
    id: 'setup_support',
    question: 'Will it understand my setup?',
    answer: [
      'Almost certainly. Resolution follows the ESM module resolution algorithm and industry standard aliasing mechanisms: tsconfig path aliases, package.json exports and imports maps including condition names, and cross-package resolution across pnpm, yarn and npm workspaces - all without extra configuration.',
      'It parses every JavaScript and TypeScript extension in common use - .ts, .tsx, .mts, .d.ts, .js, .jsx, .cjs, .mjs - plus the script blocks of .vue and .svelte components. Imports of images, fonts, styles, JSON and YAML resolve out of the box, and any other extension your project uses takes one line of config.',
      'If something behaves differently than your project expects, or you need something that is not listed here, open an issue. That is the fastest way to get it covered, and it is the kind of report that makes the resolver better for everyone.',
    ],
    link: { label: 'Report it on GitHub →', to: 'https://github.com/jayu/rev-dep/issues/new' },
  },
  {
    id: 'replace_eslint_knip',
    question: 'Do I have to rip out ESLint and knip?',
    answer: [
      'No, but there is something worth deleting. If you use eslint-plugin-import, drop the import/no-cycle and import/no-unused-modules rules: they are typically the slowest rules in a config, because ESLint re-resolves the import graph for every file. Removing them makes ESLint faster and improves detection, since a per-file linter structurally cannot see that a file is unreachable or that a cycle spans eight files across three packages. Keep ESLint for what only it can do - inline feedback as you type.',
      'knip overlaps with rev-dep on unused files, exports and dependencies, so running both means building the graph twice for the same findings. Where knip is genuinely different is finer granularity: unused class and enum members, namespace-level exports, duplicate exports. If you need that, run rev-dep on every commit and knip occasionally.',
    ],
  },
  {
    id: 'ci_impact',
    question: 'Will it slow down my CI?',
    answer: [
      'It should do the opposite, and not only because each check is faster. A typical stack runs several tools in sequence, each paying its own Node startup, its own file discovery and its own graph build - the same expensive work repeated. rev-dep builds the graph once and runs every enabled check across it in parallel.',
      'It also installs no dependency tree of its own: the npm package is a launcher plus one prebuilt binary, so the install step shrinks too.',
    ],
  },
  {
    id: 'not_supported',
    question: 'What does it not do?',
    answer: [
      'It does not draw dependency graphs - it reports, it does not visualise. It analyses at file, export and dependency granularity, so it will not find an unused class member or enum member. And it has no plugin ecosystem - instead it supports different resolution strategies so it\'s a matter of proper configuration to make it work in your project. Each project is different and plugins usually only works partially. If something is genuinely missing, please open a github issue.',
    ],
  },
];

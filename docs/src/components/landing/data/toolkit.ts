import type { TerminalLine } from '../primitives/Terminal';

/**
 * The exploratory toolkit, presented as a tabbed terminal.
 *
 * Output shapes here mirror the CLI's real output, verified against the golden
 * files in /testdata (files.golden, resolve.golden, circular.golden,
 * imported-by-fileA.golden, lines-of-code.golden, node-modules-unused.golden,
 * entry-points-deps-count.golden). Only the paths and counts are illustrative -
 * the formatting, headers and arrow chains are the real thing.
 */

export type ToolkitTab = {
  id: string;
  /** Tab label - the subcommand name. */
  label: string;
  /** The question this command answers, in the user's words. */
  question: string;
  /** When you reach for it. Condensed from the command's own docs page. */
  explanation: string;
  command: string;
  lines: TerminalLine[];
  docs: string;
};

export const toolkitTabs: ToolkitTab[] = [
  {
    id: 'resolve',
    label: 'resolve',
    question: 'Why is this file still reachable?',
    explanation:
      'Prints the actual import chains that lead to a file or package. This is the command for "I deleted the import, so why is this still in the build?" - it shows every route from every entry point.',
    command: 'rev-dep resolve --file src/utils/legacyFormat.ts',
    docs: '/docs/exploratory-toolkit/resolve',
    lines: [
      { text: '$ rev-dep resolve --file src/utils/legacyFormat.ts', tone: 'prompt' },
      { text: '' },
      { text: "Dependency paths from entry points to 'src/utils/legacyFormat.ts':" },
      { text: '' },
      { text: '/src/index.ts (1):', tone: 'accent' },
      { text: '' },
      { text: 'Path 1:' },
      { text: ' ➞ /src/index.ts' },
      { text: '  ➞ /src/pages/Dashboard.tsx' },
      { text: '   ➞ /src/widgets/RevenueCard.tsx' },
      { text: '    ➞ /src/utils/legacyFormat.ts' },
      { text: '' },
      { text: 'Total: 1', tone: 'dim' },
    ],
  },
  {
    id: 'entry-points',
    label: 'entry-points',
    question: 'What are the real roots of this project?',
    explanation:
      'An entry point is any file nothing else imports - usually a real application root, occasionally dead code. This is how you find candidates before writing prodEntryPoints and devEntryPoints.',
    command: 'rev-dep entry-points --print-deps-count',
    docs: '/docs/exploratory-toolkit/entry-points',
    lines: [
      { text: '$ rev-dep entry-points --print-deps-count', tone: 'prompt' },
      { text: '' },
      { text: '/src/index.ts                 4812' },
      { text: '/src/worker.ts                 318' },
      { text: '/scripts/generate-sitemap.ts    42' },
      { text: '/src/legacy/oldDashboard.tsx   127' },
      { text: '' },
      { text: '// Last one looks like a component, not an entry point - a likely orphan file.', tone: 'dim' },
    ],
  },
  {
    id: 'files',
    label: 'files',
    question: 'What does this entry point actually pull in?',
    explanation:
      'Lists every file reachable from the chosen entry point, including the entry point itself. Use it to understand bundle scope, or to work out why a file is being pulled in at all.',
    command: 'rev-dep files --entry-point src/index.ts',
    docs: '/docs/exploratory-toolkit/files',
    lines: [
      { text: '$ rev-dep files --entry-point src/index.ts', tone: 'prompt' },
      { text: '' },
      { text: 'src/index.ts' },
      { text: 'src/app/router.tsx' },
      { text: 'src/pages/Dashboard.tsx' },
      { text: 'src/widgets/RevenueCard.tsx' },
      { text: 'src/utils/format.ts' },
      { text: 'src/utils/legacyFormat.ts' },
      { text: 'src/types.ts' },
      { text: '' },
      { text: '# add --count for just the number', tone: 'dim' },
      { text: '$ rev-dep files --entry-point src/index.ts --count', tone: 'prompt' },
      { text: '4812' },
    ],
  },
  {
    id: 'imported-by',
    label: 'imported-by',
    question: 'Is anything still importing this directly?',
    explanation:
      'Shows the direct importers only - one hop, not the whole chain. Fast local impact analysis before you move, rename or delete a module.',
    command: 'rev-dep imported-by --file src/utils/legacyFormat.ts',
    docs: '/docs/exploratory-toolkit/imported-by',
    lines: [
      { text: '$ rev-dep imported-by --file src/utils/legacyFormat.ts', tone: 'prompt' },
      { text: '' },
      { text: 'src/widgets/RevenueCard.tsx' },
      { text: 'src/widgets/ForecastCard.tsx' },
    ],
  },
  {
    id: 'circular',
    label: 'circular',
    question: 'Where are the import cycles?',
    explanation:
      'Cycle-focused output with no config file needed, so it works during a refactor or before you adopt the config runner. Exits non-zero, so it also stands alone as a CI check.',
    command: 'rev-dep circular',
    docs: '/docs/exploratory-toolkit/circular',
    lines: [
      { text: '$ rev-dep circular', tone: 'prompt' },
      { text: '' },
      { text: 'Found 2 circular dependencies:' },
      { text: '' },
      { text: 'Circular Dependency 1:', tone: 'accent' },
      { text: ' ➞ /src/store/session.ts (cycle start)' },
      { text: "  ➞ /src/api/client.ts ('../api/client')" },
      { text: "   ➞ /src/store/session.ts ('../store/session')" },
      { text: '' },
      { text: 'Circular Dependency 2:', tone: 'accent' },
      { text: ' ➞ /src/types/user.ts (cycle start)' },
      { text: "  ➞ /src/types/account.ts ('./account')" },
      { text: "   ➞ /src/types/user.ts ('./user')" },
    ],
  },
  {
    id: 'node-modules',
    label: 'node-modules',
    question: 'Which packages am I actually using?',
    explanation:
      'Answers package-hygiene questions ad-hoc: used, unused, missing, installed, and duplicate packages, plus how much disk each one costs.',
    command: 'rev-dep node-modules unused',
    docs: '/docs/exploratory-toolkit/node-modules',
    lines: [
      { text: '$ rev-dep node-modules unused', tone: 'prompt' },
      { text: '' },
      { text: 'lodash.debounce' },
      { text: 'moment' },
      { text: '@types/uuid' },
      { text: '' },
      { text: '# also: used, missing, installed, dirs-size', tone: 'dim' },
    ],
  },
  {
    id: 'lines-of-code',
    label: 'lines-of-code',
    question: 'How big is this codebase, really?',
    explanation:
      'Counts effective lines across the project, excluding blank lines, comments and tagged template strings. Useful as a baseline before a refactor, or when comparing tool performance.',
    command: 'rev-dep lines-of-code',
    docs: '/docs/exploratory-toolkit/lines-of-code',
    lines: [
      { text: '$ rev-dep lines-of-code', tone: 'prompt' },
      { text: '' },
      { text: 'Metric                                 Lines   Percentage' },
      { text: '------                                 -----   ----------' },
      { text: 'Total lines                            581402  100.00%' },
      { text: 'Without comments                       498117  85.68%' },
      { text: 'Without comments and template strings  486903  83.75%' },
    ],
  },
];

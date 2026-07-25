/**
 * Benchmark data for the speed section.
 *
 * Source: PERFORMANCE.md at the repo root. Keep the two in sync - re-run
 * hyperfine at release time and update both.
 */

export type BenchmarkRow = {
  tool: string;
  /** Milliseconds, mean. */
  ms: number;
  /** Command or note shown under the tool name. */
  note?: string;
  /** Marks our own bar so it can be styled and announced differently. */
  ours?: boolean;
};

/**
 * Circular-dependency detection.
 *
 * skott is measured too (61 612.7 ms) but is deliberately NOT in this array:
 * at 400x the fastest result it flattens every other bar to a few pixels and
 * the chart stops communicating. It is reported as a note instead - see
 * `outlier` below - so nothing is hidden.
 */
export const circularBenchmark: BenchmarkRow[] = [
  { tool: 'rev-dep', ms: 153.6, note: 'rev-dep circular', ours: true },
  { tool: 'knip', ms: 3039.9, note: 'knip --cycles' },
  { tool: 'circular-dependency-scanner', ms: 3354.5, note: 'ds .' },
  { tool: 'dpdm-fast', ms: 6069.7, note: 'dpdm --no-tree (fast fork)' },
  { tool: 'dpdm', ms: 6667.4, note: 'dpdm --no-tree' },
  { tool: 'dependency-cruiser', ms: 8257.7, note: 'depcruise --output-type err' },
  { tool: 'madge', ms: 13568.8, note: 'madge --circular' },
];

/** Measured, but off the scale of the chart above. */
export const outlier = {
  tool: 'skott',
  label: '61.6 s',
  note: 'Also measured: skott takes 61.6 s on the same project - 400x rev-dep, and too far off the scale to plot.',
};

export type TaskRow = {
  task: string;
  /** Milliseconds, so the section can draw bars rather than print a table. */
  oursMs: number;
  rivalMs: number;
  rivalName: string;
  factor: string;
};

/** The remaining measured tasks, each drawn as a rev-dep vs alternative pair. */
export const taskComparison: TaskRow[] = [
  { task: 'Find unused exports', oursMs: 186.4, rivalMs: 3176.1, rivalName: 'knip', factor: '17×' },
  { task: 'Find unused files', oursMs: 168.2, rivalMs: 3005.9, rivalName: 'knip', factor: '18×' },
  { task: 'Find unused dependencies', oursMs: 170.0, rivalMs: 3068.9, rivalName: 'knip', factor: '18×' },
  { task: 'Find missing dependencies', oursMs: 159.5, rivalMs: 3076.1, rivalName: 'knip', factor: '19×' },
  { task: 'List files from an entry point', oursMs: 81.2, rivalMs: 6591.4, rivalName: 'madge', factor: '81×' },
  { task: 'Discover entry points', oursMs: 148.9, rivalMs: 13632.0, rivalName: 'madge', factor: '92×' },
];

/**
 * Methodology shown beneath the chart. A benchmark without project size and
 * hardware is marketing; with them it is evidence.
 */
export const benchmarkContext = {
  project: '580,000 lines of code · 6,024 source files · Next.js app',
  hardware: 'Intel i9-14900KF · WSL Debian',
  method: 'hyperfine -w 4 -r 8 (4 warm-up + 8 measured runs)',
};

/**
 * knip skips type-only import edges and offers no flag to include them, so on
 * this codebase it reports 0 cycles. Its 3 040 ms is a real full analysis, just
 * of a smaller graph - and `rev-dep circular -t` applies the same rule and
 * agrees exactly, in 143 ms. Disclosing this up front is the point.
 */
export const knipFootnote =
  'knip always ignores type-only import edges, so on this codebase it analyses a slightly smaller graph. ' +
  'Running rev-dep with the same rule (rev-dep circular -t) agrees with it exactly, in 143 ms.';

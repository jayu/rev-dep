/**
 * One formatter for every bar on the page.
 *
 * Built once at module scope: constructing an Intl.NumberFormat is expensive
 * relative to using one, and playback formats every visible row on every
 * animation frame.
 */
const decimal = new Intl.NumberFormat('en-US');

/** `13 569 ms` - thin space as the thousands separator, matching the CLI. */
export default function formatMs(ms: number): string {
  return `${decimal.format(Math.round(ms)).replace(/,/g, ' ')} ms`;
}

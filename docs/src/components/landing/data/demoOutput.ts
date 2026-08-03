/**
 * The `rev-dep config run` output shown in the hero terminal.
 *
 * Kept here rather than inline in Hero.tsx because scripts/build-demo.mjs
 * imports it too: the same lines are rendered headlessly to demo.png for the
 * README and the config-based-checks overview page. One source, so the image
 * and the landing page can never disagree.
 *
 * Real output from a 7 015-file monorepo. The terminal is the proof - it shows
 * scale, breadth of checks and the runtime at once.
 */
import type { TerminalLine } from '../primitives/Terminal';

/**
 * Every one of the thirteen detectors appears at least once, spread across four
 * workspaces the way a real config does - a workspace enables the checks that
 * make sense for it, not all of them. Repeating the same list under every
 * workspace would pad the window without showing anything new.
 *
 * Check order inside a workspace is not arbitrary: it follows the order
 * internal/cli/config_run.go prints them in, so this reads as a transcript
 * rather than an arrangement. Spacing matches too - one space before
 * `(N files)`, two after the ✨ - see testdata/*.golden.
 *
 * The one deliberate departure: separators carry `gap`, which draws them at
 * half height. The CLI emits a real blank line there, but on screen at this
 * font size a full one leaves a block-sized hole.
 */
export const demoOutput: TerminalLine[] = [
  { text: '$ rev-dep config run', tone: 'prompt' },
  { text: '', gap: true },
  // Only two things are highlighted: how much code was analysed, and how long
  // it took. Everything else stays plain so those two actually stand out.
  // build-demo.mjs anchors its arrows to exactly these `strong` parts.
  {
    parts: [
      { text: '📁 Workspace: . (root) (' },
      { text: '7015 files', tone: 'accent', strong: true },
      { text: ')' },
    ],
  },
  // The ✅ carries the "passed" signal on its own - colouring the check name
  // green too makes the block read as one solid wall of green.
  { text: '✅ Orphan Files', indent: 1 },
  { text: '✅ Duplicated Code', indent: 1 },
  { text: '✅ Module Boundaries', indent: 1 },
  { text: '✅ Unused Exports', indent: 1 },
  { text: '', gap: true },
  { text: '📁 Workspace: apps/web (6096 files)' },
  { text: '✅ Circular Dependencies', indent: 1 },
  { text: '✅ Orphan Files', indent: 1 },
  { text: '✅ Unused Node Modules', indent: 1 },
  { text: '✅ Missing Node Modules', indent: 1 },
  { text: '✅ Import Conventions', indent: 1 },
  { text: '✅ Unresolved Imports', indent: 1 },
  { text: '✅ Dev Deps Usage On Prod', indent: 1 },
  { text: '✅ Restricted Imports', indent: 1 },
  { text: '✅ Restricted Importers', indent: 1 },
  { text: '✅ Restricted Direct Importers', indent: 1 },
  { text: '', gap: true },
  { text: '📁 Workspace: apps/mobile (742 files)' },
  { text: '✅ Circular Dependencies', indent: 1 },
  { text: '✅ Orphan Files', indent: 1 },
  { text: '✅ Unused Node Modules', indent: 1 },
  { text: '✅ Missing Node Modules', indent: 1 },
  { text: '✅ Unresolved Imports', indent: 1 },
  { text: '✅ Dev Deps Usage On Prod', indent: 1 },
  { text: '', gap: true },
  { text: '📁 Workspace: packages/shared (88 files)' },
  { text: '✅ Circular Dependencies', indent: 1 },
  { text: '', gap: true },
  { text: '✅ All checks passed!' },
  { text: '', gap: true },
  {
    parts: [
      { text: '✨  Done in ' },
      { text: '175ms', tone: 'accent', strong: true },
      { text: '.' },
    ],
  },
];

/** Window chrome caption, shared with the generated image. */
export const demoTitle = 'rev-dep config run';

import type { TerminalLine } from '../primitives/Terminal';

/**
 * Copy and transcript for the getting-started section.
 *
 * Here rather than in Adoption.tsx so every section keeps its wording in the
 * same place: sections own markup, data modules own content.
 */

/**
 * Real `rev-dep config init` output, abridged only by trimming the box-drawing
 * frame around the prompt. Wording and ordering are verbatim - the wizard asks
 * how strict you want to start, and that answer matters.
 */
export const initOutput: TerminalLine[] = [
  { text: '$ rev-dep config init', tone: 'prompt' },
  { text: '' },
  { text: 'Which detectors should the config enable?' },
  { text: '' },
  { text: '1) No detectors', indent: 1, tone: 'dim' },
  {
    parts: [{ text: '2) Unresolved imports only (a good start to confirm', tone: 'dim' }],
    indent: 1,
  },
  { text: '   your setup and that resolution works)', indent: 1, tone: 'dim' },
  { text: '*  3) Unresolved + circular imports', indent: 1 },
  { text: '4) Circular + unresolved enabled, other detectors', indent: 1, tone: 'dim' },
  { text: '   listed but disabled', indent: 1, tone: 'dim' },
  { text: '5) All detectors enabled', indent: 1, tone: 'dim' },
  { text: '' },
  { text: 'Type 1-5 [default: 3]:', tone: 'prompt' },
  { text: '' },
  { text: '✅ Created .rev-dep.config.jsonc' },
  { text: '' },
  { text: '📦 Monorepo detected: discovered 3 workspace packages' },
  { text: '   and created a rule for each:' },
  { text: '- packages/app', indent: 1, tone: 'dim' },
  { text: '- packages/shared', indent: 1, tone: 'dim' },
  { text: '- packages/ui', indent: 1, tone: 'dim' },
  { text: '' },
  { text: 'Adjust rules to make them relevant to your project setup.', tone: 'dim' },
];

export type AdoptionStep = {
  title: string;
  body: string;
};

/** How to take the day-one backlog in pieces instead of all at once. */
export const adoptionSteps: AdoptionStep[] = [
  {
    title: 'Start with two checks',
    body: 'The default is unresolved + circular imports. Both are near-zero noise and they prove your path aliases and monorepo resolution are set up correctly.',
  },
  {
    title: 'Add one detector at a time',
    body: 'Each check is a separate key in the config. Turn on unused exports, fix that backlog, commit, then move to the next one.',
  },
  {
    title: 'Let --fix do the boring part',
    body: 'Orphan files, unused exports and import conventions repair themselves. Deleting one dead file often reveals another, so run it until it comes back clean.',
  },
  {
    title: 'Scope rules per workspace',
    body: 'A monorepo package that is not ready yet keeps its checks off. Nothing is all-or-nothing.',
  },
];

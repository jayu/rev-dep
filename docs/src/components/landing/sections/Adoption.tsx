import Heading from '@theme/Heading';

import Section from '../primitives/Section';
import Card from '../primitives/Card';
import Grid from '../primitives/Grid';
import Terminal, { type TerminalLine } from '../primitives/Terminal';
import styles from './Adoption.module.css';

/** The init wizard asks how strict you want to start. That answer matters. */
/**
 * Real `rev-dep config init` output, abridged only by trimming the box-drawing
 * frame around the prompt. Wording and ordering are verbatim.
 */
const initOutput: TerminalLine[] = [
  { text: '$ rev-dep config init', tone: 'prompt' },
  { text: '' },
  { text: 'Which detectors should the config enable?' },
  { text: '' },
  { text: '1) No detectors', indent: 1, tone: 'dim' },
  {
    parts: [
      { text: '2) Unresolved imports only (a good start to confirm', tone: 'dim' },
    ],
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
  { text: '📦 Monorepo detected: discovered 5 workspace packages' },
  { text: '   and created a rule for each:' },
  { text: '- packages/app', indent: 1, tone: 'dim' },
  { text: '- packages/shared', indent: 1, tone: 'dim' },
  { text: '- packages/ui', indent: 1, tone: 'dim' },
  { text: '' },
  { text: 'Adjust rules to make them relevant to your project setup.', tone: 'dim' },
];

const steps = [
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

export default function Adoption() {
  return (
    <Section
      id="adoption"
      eyebrow="Getting started"
      title="You will get a lot of findings on day one. That is the point."
      intro="Every tool like this shows a backlog on the first run - it is the work that piled up while nothing was checking. What matters is that it is a one-time cleanup, and that you can take it in pieces instead of all at once. After that, a normal pull request surfaces a handful of issues at most."
    >
      <div className={styles.layout}>
        <div className={styles.visual}>
          <Terminal lines={initOutput} title="rev-dep config init" />
        </div>

        <Grid cols={2} gap="tight" className={styles.steps}>
          {steps.map((step) => (
            <Card key={step.title} hover>
              <Heading as="h3" className={styles.stepTitle}>
                {step.title}
              </Heading>
              <p className={styles.stepBody}>{step.body}</p>
            </Card>
          ))}
        </Grid>
      </div>
    </Section>
  );
}

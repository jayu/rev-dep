import Heading from '@theme/Heading';

import Terminal, { type TerminalLine } from '../primitives/Terminal';
import CopyCommand from '../primitives/CopyCommand';
import Button from '../primitives/Button';
import styles from './Hero.module.css';

/**
 * Real output from `rev-dep config run` on a 7 015-file monorepo. The terminal
 * is the proof - it shows scale, breadth of checks and the runtime at once.
 */
const heroOutput: TerminalLine[] = [
  { text: '$ rev-dep config run', tone: 'prompt' },
  { text: '' },
  // Only two things are highlighted: how much code was analysed, and how long
  // it took. Everything else stays plain so those two actually stand out.
  {
    parts: [
      { text: '📁 Rule: .  (' },
      { text: '7015 files', tone: 'accent', strong: true },
      { text: ')' },
    ],
  },
  // The ✅ carries the "passed" signal on its own - colouring the check name
  // green too makes the block read as one solid wall of green.
  { text: '✅ Orphan Files', indent: 1 },
  { text: '✅ Module Boundaries', indent: 1 },
  { text: '✅ Unused Exports', indent: 1 },
  { text: '' },
  { text: '📁 Rule: apps/web  (6096 files)' },
  { text: '✅ Circular Dependencies', indent: 1 },
  { text: '✅ Unused Node Modules', indent: 1 },
  { text: '✅ Missing Node Modules', indent: 1 },
  { text: '✅ Dev Deps Usage On Prod', indent: 1 },
  { text: '✅ Restricted Imports', indent: 1 },
  { text: '✅ Import Conventions', indent: 1 },
  { text: '' },
  { text: '📁 Rule: apps/mobile  (742 files)' },
  { text: '✅ Circular Dependencies', indent: 1 },
  { text: '✅ Orphan Files', indent: 1 },
  { text: '' },
  { text: '✅ All checks passed!' },
  {
    parts: [
      { text: '✨ Done in ' },
      { text: '175ms', tone: 'accent', strong: true },
      { text: '.' },
    ],
  },
];

export default function Hero() {
  return (
    <header className={styles.hero}>
      <div className="container">
        <div className={styles.layout}>
          <div className={styles.copy}>
            <Heading as="h1" className={styles.title}>
              Dead code, cycles and architecture violations.
              <span className={styles.titleAccent}> Found before you finish reading this.</span>
            </Heading>

            <p className={styles.subtitle}>
              Rev-dep consolidates fragmented, sequential checks from multiple slow tools into a
              single high-performance engine. Twelve checks, one config, one binary - across your
              whole JS/TS monorepo.
            </p>

            <div className={styles.actions}>
              <Button to="/docs/installation">Get Started 🚀</Button>
              <Button to="https://github.com/jayu/rev-dep" variant="outline">
                View on GitHub
              </Button>
            </div>

            <CopyCommand command="npm install -D rev-dep" className={styles.install} />
          </div>

          <div className={styles.visual}>
            <Terminal lines={heroOutput} title="rev-dep config run" />
          </div>
        </div>
      </div>
    </header>
  );
}

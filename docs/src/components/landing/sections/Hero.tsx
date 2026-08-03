import clsx from 'clsx';
import Heading from '@theme/Heading';

import Terminal from '../primitives/Terminal';
import CopyCommand from '../primitives/CopyCommand';
import Button from '../primitives/Button';
import { demoOutput, demoTitle } from '../data/demoOutput';
import styles from './Hero.module.css';

/**
 * The single call to action. Rendered in two places and toggled by CSS: inside
 * the copy column, and as a centred row under both columns.
 *
 * CSS cannot move an element between parents, and the two positions are not
 * siblings - one is nested inside the left column, the other follows the grid.
 * Rendering it twice and hiding one with `display: none` keeps it to CSS, and
 * `display: none` also drops the hidden copy out of the accessibility tree, so
 * only ever one button is exposed.
 *
 * Each position passes its own analytics id: only one is ever clickable, but
 * which one it is depends on the viewport, and that is worth being able to see
 * in the numbers rather than having to infer.
 */
const cta = (id: string) => (
  <Button to="/docs/installation" event={{ id, section: 'hero' }}>
    Get Started
  </Button>
);

export default function Hero() {
  return (
    <header className={styles.hero}>
      <div className="container">
        <div className={styles.layout}>
          <div className={styles.copy}>
            <Heading as="h1" className={clsx(styles.title, 'rd-hold')}>
              Dead code, duplicate code, dependency cycles, and architecture violations.
              <span className={styles.titleAccent}> Found before you finish reading this.</span>
            </Heading>

            <p className={clsx(styles.subtitle, 'rd-hold')}>
              Rev-dep consolidates fragmented, sequential checks<br/> from multiple slow tools into a
              single high-performance engine. <br/><span style={{marginTop:10, display:'block'}}>Evaluate your entire JS/TS monorepo without breaking a sweat.</span>
            </p>

            <div className={clsx(styles.actions, 'rd-hold')}>{cta('hero_get_started')}</div>

            {/* Wrapper, because CopyCommand is inline-flex and sizes to its
                content - it has nothing to centre itself against. */}
            {/* <div className={clsx(styles.install, 'rd-hold')}>
              <CopyCommand
                command="npm install -D rev-dep"
                event={{ id: 'hero_copy_install', section: 'hero' }}
              />
            </div> */}
          </div>

          <div className={clsx(styles.visual, 'rd-enter')}>
            <Terminal lines={demoOutput} title={demoTitle} />
          </div>
        </div>

        <div className={clsx(styles.actionsBelow, 'rd-hold')}>
          {cta('hero_get_started_below')}
        </div>
      </div>
    </header>
  );
}

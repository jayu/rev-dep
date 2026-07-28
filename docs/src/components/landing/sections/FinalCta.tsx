import Heading from '@theme/Heading';

import CopyCommand from '../primitives/CopyCommand';
import Button from '../primitives/Button';
import Reveal from '../primitives/Reveal';
import styles from './FinalCta.module.css';

export default function FinalCta() {
  return (
    <section className={styles.cta}>
      <div className="container">
        {/* This section does not use the Section primitive - it has its own
            band and centred layout - so it has to opt into the scroll reveal
            explicitly. Revealed as one block: it is a single centred column,
            and staggering its five lines would draw more attention to the
            animation than to the call to action. */}
        <Reveal className={styles.inner}>
          <p className={styles.eyebrow}>Get started</p>
          <Heading as="h2" className={styles.title}>
            Point it at your repo and see what it finds
          </Heading>
          <p className={styles.body}>
            Install rev-dep, generate initial config <br/>and follow integration guides to set it up.
          </p>

          <CopyCommand command="npm install -D rev-dep" className={styles.install} />

          <div className={styles.actions}>
            <Button to="/docs/monorepo-integration-guide">Monorepo guide</Button>
            <Button to="/docs/single-workspace-integration-guide" variant="outline">
              Single workspace guide
            </Button>
          </div>

          <p className={styles.meta}>
            MIT licensed · no account · no 3rd party dependencies
          </p>
        </Reveal>
      </div>
    </section>
  );
}

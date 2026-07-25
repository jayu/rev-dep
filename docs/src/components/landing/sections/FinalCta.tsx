import Heading from '@theme/Heading';

import CopyCommand from '../primitives/CopyCommand';
import Button from '../primitives/Button';
import styles from './FinalCta.module.css';

export default function FinalCta() {
  return (
    <section className={styles.cta}>
      <div className="container">
        <div className={styles.inner}>
          <p className={styles.eyebrow}>Get started</p>
          <Heading as="h2" className={styles.title}>
            Point it at your repo and see what it finds
          </Heading>
          <p className={styles.body}>
            The first run takes about a second, and tells you exactly what has been accumulating
            while nothing was checking.
          </p>

          <CopyCommand command="npm install -D rev-dep" className={styles.install} />

          <div className={styles.actions}>
            <Button to="/docs/monorepo-integration-guide">Monorepo guide</Button>
            <Button to="/docs/single-workspace-integration-guide" variant="outline">
              Single package guide
            </Button>
          </div>

          <p className={styles.meta}>
            MIT licensed · no account · no 3rd party dependencies
          </p>
        </div>
      </div>
    </section>
  );
}

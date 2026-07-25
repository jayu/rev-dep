import Heading from '@theme/Heading';

import CopyCommand from '../primitives/CopyCommand';
import Button from '../primitives/Button';
import styles from './FinalCta.module.css';

export default function FinalCta() {
  return (
    <section className={styles.cta}>
      <div className="container">
        <div className={styles.inner}>
          <Heading as="h2" className={styles.title}>
            Point it at your repo and see what it finds
          </Heading>
          <p className={styles.body}>
            Install it, run <code className={styles.inlineCode}>rev-dep config init</code>, and the
            first check runs in under a second. MIT licensed, no account, no dependency tree.
          </p>

          <CopyCommand command="npm install -D rev-dep" className={styles.install} />

          <div className={styles.actions}>
            <Button to="/docs/monorepo-integration-guide">Monorepo guide</Button>
            <Button to="/docs/single-workspace-integration-guide" variant="outline">
              Single package guide
            </Button>
          </div>
        </div>
      </div>
    </section>
  );
}

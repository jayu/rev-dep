import Link from '@docusaurus/Link';
import Heading from '@theme/Heading';

import Section from '../primitives/Section';
import Card from '../primitives/Card';
import Grid from '../primitives/Grid';
import { checkGroups } from '../data/checks';
import styles from './Checks.module.css';

export default function Checks() {
  return (
    <Section
      id="checks"
      eyebrow="What it catches"
      title="Twelve checks, one config, one pass"
      intro="Grouped by the problem they solve. Turn on what you need - each check is opt-in per workspace."
    >
      {checkGroups.map((group) => (
        <div className={styles.group} key={group.id}>
          <div className={styles.groupHead}>
            <Heading as="h3" className={styles.groupTitle}>
              {group.title}
            </Heading>
            <p className={styles.groupBlurb}>{group.blurb}</p>
          </div>

          <Grid cols={3} gap="tight">
            {group.checks.map((check) => (
              <Card key={check.key} hover>
                <div className={styles.checkHead}>
                  <Heading as="h4" className={styles.checkTitle}>
                    {check.title}
                  </Heading>
                  {check.autofix && <span className={styles.fixBadge}>--fix</span>}
                </div>
                <code className={styles.checkKey}>{check.key}</code>
                <p className={styles.checkBody}>{check.description}</p>
                <Link className={styles.checkLink} to={check.docs}>
                  Read the docs
                </Link>
              </Card>
            ))}
          </Grid>
        </div>
      ))}
    </Section>
  );
}

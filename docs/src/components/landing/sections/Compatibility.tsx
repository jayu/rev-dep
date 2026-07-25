import Link from '@docusaurus/Link';
import Heading from '@theme/Heading';

import Section from '../primitives/Section';
import Card from '../primitives/Card';
import Grid from '../primitives/Grid';
import { compatibilityItems, resolutionLimits } from '../data/compatibility';
import styles from './Compatibility.module.css';

export default function Compatibility() {
  return (
    <Section
      id="compatibility"
      eyebrow="Resolution"
      title="Built for the setup you actually have"
      intro="A dependency checker is only useful if it resolves imports the same way your build does. rev-dep follows the same rules your bundler, TypeScript and package manager already follow - so the graph it reports is the graph you ship."
    >
      <Grid cols={3} gap="tight">
        {compatibilityItems.map((item) => (
          <Card key={item.title} hover>
            <Heading as="h3" className={styles.title}>
              {item.title}
            </Heading>
            <p className={styles.body}>{item.body}</p>
            <div className={styles.footer}>
              {item.key && <code className={styles.key}>{item.key}</code>}
              <Link className={styles.link} to={item.docs}>
                Docs
              </Link>
            </div>
          </Card>
        ))}

        <Card tone="panel">
          <Heading as="h3" className={styles.limitsTitle}>
            Known limits
          </Heading>
          <ul className={styles.limits}>
            {resolutionLimits.map((limit) => (
              <li className={styles.limit} key={limit}>
                {limit}
              </li>
            ))}
          </ul>
        </Card>
      </Grid>
    </Section>
  );
}

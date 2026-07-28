import Link from '@docusaurus/Link';
import Heading from '@theme/Heading';

import Section from '../primitives/Section';
import Card from '../primitives/Card';
import { replacedTools, limitations } from '../data/tools';
import styles from './ReplacesStack.module.css';

export default function ReplacesStack() {
  return (
    <Section
      id="replaces"
      eyebrow="One tool instead of several"
      title="What rev-dep replaces"
      intro="rev-dep covers every one of these. Each one starts its own Node process, discovers your files again and builds its own graph - run four and you pay for that four times. Follow any row for an honest side-by-side."
    >
      <div className={styles.layout}>
        <ul className={styles.toolList}>
          {replacedTools.map((tool) => (
            <li className={styles.toolItem} key={tool.name}>
              <Link className={styles.toolLink} to={tool.href}>
                <span className={styles.toolName}>{tool.name}</span>
                <span className={styles.toolCovers}>{tool.covers}</span>
                <span className={styles.toolChevron} aria-hidden="true" />
              </Link>
            </li>
          ))}
        </ul>

        <Card size="large" fit className={styles.honestCard}>
          <Heading as="h3" className={styles.honestTitle}>
            And what it does not do
          </Heading>
          <ul className={styles.honestList}>
            {limitations.map((item) => (
              <li className={styles.honestItem} key={item}>
                {item}
              </li>
            ))}
          </ul>
          <p className={styles.honestFooter}>
            If you need those, keep the tool that provides them. The comparison pages say exactly
            where each one still wins.
          </p>
        </Card>
      </div>
    </Section>
  );
}

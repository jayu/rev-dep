import Link from '@docusaurus/Link';

import Section from '../primitives/Section';
import Split from '../primitives/Split';
import Card from '../primitives/Card';
import MicroLabel from '../primitives/MicroLabel';
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
      <Split left="1.35fr" right="1fr" gap="2rem">
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

        {/* Plain card, not a highlighted one: this is an honesty note, and it
            should not out-shout the list of things the tool actually replaces. */}
        <Card size="large" fit className={styles.honestCard}>
          <MicroLabel as="h3">And what it does not do</MicroLabel>
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
      </Split>
    </Section>
  );
}

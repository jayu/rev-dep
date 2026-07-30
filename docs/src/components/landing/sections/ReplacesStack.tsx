import Link from '@docusaurus/Link';

import Section from '../primitives/Section';
import { replacedTools } from '../data/tools';
import styles from './ReplacesStack.module.css';

export default function ReplacesStack() {
  return (
    <Section
      id="replaces"
      eyebrow="One tool instead of several"
      title="What rev-dep replaces"
      intro="Rev-dep covers every one of these. Each one starts its own Node process, discovers your files again and builds its own graph - run four and you pay for that four times. Follow any row for an honest side-by-side."
    >
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
    </Section>
  );
}

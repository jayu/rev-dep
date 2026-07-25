import { useState } from 'react';
import clsx from 'clsx';
import Link from '@docusaurus/Link';
import Heading from '@theme/Heading';

import Section from '../primitives/Section';
import { faqEntries } from '../data/faq';
import styles from './Faq.module.css';

export default function Faq() {
  // `details` stays uncontrolled - React only observes the open state, so
  // keyboard support and no-JS rendering keep working. The state exists to put
  // a class on the chevron and the open item: doing that with
  // `details[open] .chevron` would be a two-part selector.
  const [open, setOpen] = useState<Record<string, boolean>>({});

  return (
    <Section id="faq">
      <div className={styles.layout}>
        <div className={styles.intro}>
          <p className={styles.eyebrow}>Questions</p>
          <Heading as="h2" className={styles.title}>
            The things people ask before installing
          </Heading>
          <p className={styles.introBody}>
            The objections that actually come up - answered with the mechanism, not a slogan.
          </p>
          <Link className={styles.introLink} to="https://github.com/jayu/rev-dep/issues">
            Ask something else on GitHub →
          </Link>
        </div>

        <div className={styles.list}>
          {faqEntries.map((entry) => (
            <details
              className={clsx(styles.item, open[entry.question] && styles.itemOpen)}
              key={entry.question}
              onToggle={(event) => {
                // Read synchronously: React nulls `currentTarget` once the
                // handler returns, and a functional state updater runs later,
                // during render - by then `event.currentTarget` is null.
                const isOpen = event.currentTarget.open;
                setOpen((state) => ({ ...state, [entry.question]: isOpen }));
              }}
            >
              <summary className={styles.question}>
                <span className={styles.questionText}>{entry.question}</span>
                <span
                  className={clsx(styles.chevron, open[entry.question] && styles.chevronOpen)}
                  aria-hidden="true"
                />
              </summary>
              <div className={styles.answer}>
                {entry.answer.map((paragraph) => (
                  <p className={styles.paragraph} key={paragraph.slice(0, 40)}>
                    {paragraph}
                  </p>
                ))}
              </div>
            </details>
          ))}
        </div>
      </div>
    </Section>
  );
}

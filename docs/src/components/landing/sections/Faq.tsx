import { useState } from 'react';
import clsx from 'clsx';
import Link from '@docusaurus/Link';

import Section from '../primitives/Section';
import SectionHeader from '../primitives/SectionHeader';
import Split from '../primitives/Split';
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
      <Split left="1fr" right="1.7fr" gap="3.5rem">
        {/* The header sits beside the answers rather than above them, so it
            uses SectionHeader directly instead of Section's own header slot. */}
        <SectionHeader
          className={styles.intro}
          eyebrow="Questions"
          title="The things people ask before installing"
          intro="The objections that actually come up - answered with the mechanism, not a slogan."
        >
          <Link className={styles.introLink} to="https://github.com/jayu/rev-dep/issues">
            Ask something else on GitHub →
          </Link>
        </SectionHeader>

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
                {entry.answer.map((paragraph, i) => (
                  <p className={styles.paragraph} key={i}>
                    {paragraph}
                  </p>
                ))}
                {entry.link && (
                  <Link className={styles.answerLink} to={entry.link.to}>
                    {entry.link.label}
                  </Link>
                )}
              </div>
            </details>
          ))}
        </div>
      </Split>
    </Section>
  );
}

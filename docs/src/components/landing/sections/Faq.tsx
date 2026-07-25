import { useState } from 'react';
import clsx from 'clsx';

import Section from '../primitives/Section';
import { faqEntries } from '../data/faq';
import styles from './Faq.module.css';

export default function Faq() {
  // `details` stays uncontrolled - React only observes the open state, so
  // keyboard support and no-JS rendering keep working. The state exists purely
  // to put a class on the chevron: rotating it via `details[open] .chevron`
  // would be a two-part selector, which the CSS rules here do not allow.
  const [open, setOpen] = useState<Record<string, boolean>>({});

  return (
    <Section id="faq" eyebrow="Questions" title="The things people ask before installing">
      <div className={styles.list}>
        {faqEntries.map((entry) => (
          <details
            className={styles.item}
            key={entry.question}
            onToggle={(event) =>
              setOpen((state) => ({ ...state, [entry.question]: event.currentTarget.open }))
            }
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
    </Section>
  );
}

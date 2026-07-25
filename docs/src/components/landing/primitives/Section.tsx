import type { ReactNode } from 'react';
import clsx from 'clsx';
import Heading from '@theme/Heading';

import styles from './Section.module.css';

type SectionProps = {
  /** Anchor id, so nav links and the FAQ can deep-link to a section. */
  id?: string;
  /** Small uppercase kicker above the title. */
  eyebrow?: string;
  title?: ReactNode;
  /** Lead paragraph under the title. */
  intro?: ReactNode;
  /** `tinted` paints the section background; use it to alternate bands. */
  tone?: 'plain' | 'tinted';
  align?: 'left' | 'center';
  /** Adds a top hairline; used when two plain sections sit next to each other. */
  divided?: boolean;
  /** Optional panel rendered beside the header - e.g. methodology, key facts. */
  aside?: ReactNode;
  children: ReactNode;
};

/**
 * The single section wrapper for the landing page. Every section uses it, so
 * vertical rhythm, container width and header typography are defined once.
 */
export default function Section({
  id,
  eyebrow,
  title,
  intro,
  tone = 'plain',
  align = 'left',
  divided = false,
  aside,
  children,
}: SectionProps) {
  const hasHeader = Boolean(eyebrow || title || intro);

  return (
    <section
      id={id}
      className={clsx(
        styles.section,
        tone === 'tinted' && styles.sectionTinted,
        divided && styles.sectionDivided,
      )}
    >
      <div className="container">
        {(hasHeader || aside) && (
          <div className={clsx(styles.headerRow, aside && styles.headerRowSplit)}>
            {hasHeader && (
              <header className={clsx(styles.header, align === 'center' && styles.headerCenter)}>
                {eyebrow && <p className={styles.eyebrow}>{eyebrow}</p>}
                {title && (
                  <Heading as="h2" className={styles.title}>
                    {title}
                  </Heading>
                )}
                {intro && <p className={styles.intro}>{intro}</p>}
              </header>
            )}
            {aside && <div className={styles.aside}>{aside}</div>}
          </div>
        )}
        {children}
      </div>
    </section>
  );
}

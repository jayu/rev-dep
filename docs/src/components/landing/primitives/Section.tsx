import { Children, type ReactNode } from 'react';
import clsx from 'clsx';
import Heading from '@theme/Heading';

import Reveal from './Reveal';

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
  /**
   * Ambient corner light on the section background. Use the corner a terminal
   * sits in, so the window overlaps its edge.
   */
  corner?: 'top-right' | 'bottom-left';
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
  corner,
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
        corner === 'top-right' && styles.cornerTopRight,
        corner === 'bottom-left' && styles.cornerBottomLeft,
      )}
    >
      <div className="container">
        {(hasHeader || aside) && (
          <Reveal className={clsx(styles.headerRow, aside && styles.headerRowSplit)}>
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
          </Reveal>
        )}
        {/* Each top-level child animates on its own, so a long section reveals
            in pieces as you scroll rather than all at once. */}
        {Children.map(children, (child) =>
          child == null || typeof child === 'boolean' ? child : <Reveal>{child}</Reveal>,
        )}
      </div>
    </section>
  );
}

import type { ReactNode } from 'react';
import clsx from 'clsx';
import Heading from '@theme/Heading';

import styles from './SectionHeader.module.css';

type SectionHeaderProps = {
  /** Small uppercase kicker above the title. */
  eyebrow?: string;
  title?: ReactNode;
  /** Lead paragraph under the title. */
  intro?: ReactNode;
  className?: string;
  /** Rendered after the intro - the FAQ hangs a link there. */
  children?: ReactNode;
};

/**
 * Eyebrow, title and lead paragraph.
 *
 * Section renders one above its content; the FAQ renders the same thing inside
 * a sticky column beside its list, which is why this is a component rather than
 * markup inside Section. It used to be both, and the two copies had already
 * drifted on the intro's font size.
 */
export default function SectionHeader({
  eyebrow,
  title,
  intro,
  className,
  children,
}: SectionHeaderProps) {
  return (
    <header className={clsx(styles.header, className)}>
      {eyebrow && <p className={styles.eyebrow}>{eyebrow}</p>}
      {title && (
        <Heading as="h2" className={styles.title}>
          {title}
        </Heading>
      )}
      {intro && <p className={styles.intro}>{intro}</p>}
      {children}
    </header>
  );
}

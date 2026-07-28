import type { ReactNode } from 'react';
import clsx from 'clsx';

import useReveal from './useReveal';
import styles from './Reveal.module.css';

type RevealProps = {
  className?: string;
  children: ReactNode;
};

/**
 * Fades and lifts one block as it scrolls into view.
 *
 * Deliberately per-block rather than per-section: a section two viewports tall
 * would otherwise play its whole animation while most of it is still below the
 * fold, so by the time you reach the lower half it has already finished.
 */
export default function Reveal({ className, children }: RevealProps) {
  const { ref, state } = useReveal<HTMLDivElement>();

  return (
    <div
      ref={ref}
      className={clsx(
        styles.reveal,
        state === 'pending' && styles.pending,
        state === 'revealed' && styles.in,
        className,
      )}
    >
      {children}
    </div>
  );
}

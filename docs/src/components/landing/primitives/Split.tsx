import type { CSSProperties, ReactNode } from 'react';
import clsx from 'clsx';

import styles from './Split.module.css';

type SplitProps = {
  /** Width of the first column, as a grid track. */
  left?: string;
  /** Width of the second column, as a grid track. */
  right?: string;
  gap?: string;
  /**
   * `center` when the two columns are different heights and neither is sticky.
   * Sticky columns must stay `start` or they never stick.
   */
  align?: 'start' | 'center';
  className?: string;
  children: ReactNode;
};

/**
 * Two columns on desktop, stacked below 996px.
 *
 * The ratio travels as custom properties rather than a prop-per-variant class:
 * every use is a different pair of tracks, so a class list would have as many
 * entries as call sites.
 */
export default function Split({
  left = '1fr',
  right = '1fr',
  gap = '2.5rem',
  align = 'start',
  className,
  children,
}: SplitProps) {
  return (
    <div
      className={clsx(styles.split, align === 'center' && styles.splitCenter, className)}
      style={
        {
          '--rd-split-a': left,
          '--rd-split-b': right,
          '--rd-split-gap': gap,
        } as CSSProperties
      }
    >
      {children}
    </div>
  );
}

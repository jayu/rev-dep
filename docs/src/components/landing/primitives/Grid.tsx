import type { ReactNode } from 'react';
import clsx from 'clsx';

import styles from './Grid.module.css';

type GridProps = {
  /** Desktop column count. All variants collapse to 1 column on small screens. */
  cols: 2 | 3 | 4 | 5;
  /** `tight` for dense lists of small cards. */
  gap?: 'default' | 'tight';
  className?: string;
  children: ReactNode;
};

/** Responsive grid. Collapse behaviour is defined once, per column count. */
export default function Grid({ cols, gap = 'default', className, children }: GridProps) {
  return (
    <div
      className={clsx(
        styles.grid,
        cols === 2 && styles.cols2,
        cols === 3 && styles.cols3,
        cols === 4 && styles.cols4,
        cols === 5 && styles.cols5,
        gap === 'tight' && styles.gapTight,
        className,
      )}
    >
      {children}
    </div>
  );
}

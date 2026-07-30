import type { ReactNode } from 'react';
import clsx from 'clsx';
import Heading from '@theme/Heading';

import styles from './MicroLabel.module.css';

type MicroLabelProps = {
  /** `p` when the label only titles a block, a heading when it opens a section. */
  as?: 'p' | 'h3' | 'h4';
  className?: string;
  children: ReactNode;
};

/** Small uppercase caption above a subordinate block. */
export default function MicroLabel({ as = 'p', className, children }: MicroLabelProps) {
  const classes = clsx(styles.microLabel, className);

  if (as === 'p') {
    return <p className={classes}>{children}</p>;
  }

  return (
    <Heading as={as} className={classes}>
      {children}
    </Heading>
  );
}

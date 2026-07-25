import type { ReactNode } from 'react';
import Link from '@docusaurus/Link';
import clsx from 'clsx';

import styles from './Button.module.css';

type ButtonProps = {
  to: string;
  /** `primary` is the filled brand button, `outline` the bordered secondary. */
  variant?: 'primary' | 'outline';
  size?: 'default' | 'large';
  className?: string;
  children: ReactNode;
};

/**
 * Landing page button. Both places it is used (hero, final CTA) sit on the
 * dark background, so the outline variant is styled for that - it borrows the
 * surrounding text colour rather than a fixed grey.
 *
 * `Link` handles internal routing and external URLs, so `to` works for both.
 */
export default function Button({
  to,
  variant = 'primary',
  size = 'large',
  className,
  children,
}: ButtonProps) {
  return (
    <Link
      to={to}
      className={clsx(
        styles.button,
        variant === 'primary' ? styles.buttonPrimary : styles.buttonOutline,
        size === 'large' && styles.buttonLarge,
        className,
      )}
    >
      {children}
    </Link>
  );
}

import type { ReactNode } from 'react';
import clsx from 'clsx';

import TrackedLink from './TrackedLink';
import type { ClickEvent } from '../analytics';
import styles from './Button.module.css';

type ButtonProps = {
  to: string;
  /** `primary` is the filled brand button, `outline` the bordered secondary. */
  variant?: 'primary' | 'outline';
  size?: 'default' | 'large';
  className?: string;
  /**
   * Required, not optional: these are the page's calls to action, and a button
   * that quietly reports nothing is the one measurement worth never losing.
   */
  event: Omit<ClickEvent, 'target' | 'type'>;
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
  event,
  children,
}: ButtonProps) {
  return (
    <TrackedLink
      to={to}
      event={{ ...event, type: 'cta' }}
      className={clsx(
        styles.button,
        variant === 'primary' ? styles.buttonPrimary : styles.buttonOutline,
        size === 'large' && styles.buttonLarge,
        className,
      )}
    >
      {children}
    </TrackedLink>
  );
}

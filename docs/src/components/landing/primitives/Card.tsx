import type { ReactNode } from 'react';
import clsx from 'clsx';

import styles from './Card.module.css';

type CardProps = {
  /** `panel` is the inverted dark surface used for highlights. */
  tone?: 'default' | 'panel';
  /** Lift on hover. Only for cards that are themselves interesting to scan. */
  hover?: boolean;
  /** Roomier padding for cards that carry a lot of content. */
  size?: 'default' | 'large';
  /**
   * Hug the content instead of filling the grid row. Cards stretch by default
   * so a row of them lines up; a lone card beside a long list must not.
   */
  fit?: boolean;
  className?: string;
  children: ReactNode;
};

/**
 * The one raised surface on the page. Four sections used to define this
 * separately; now the background, border, radius and shadow live in one place.
 */
export default function Card({
  tone = 'default',
  hover = false,
  size = 'default',
  fit = false,
  className,
  children,
}: CardProps) {
  return (
    <article
      className={clsx(
        styles.card,
        tone === 'panel' && styles.cardPanel,
        hover && styles.cardHover,
        size === 'large' && styles.cardLarge,
        fit && styles.cardFit,
        className,
      )}
    >
      {children}
    </article>
  );
}

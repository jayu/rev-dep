import type { ReactNode } from 'react';
import clsx from 'clsx';
import Heading from '@theme/Heading';

import Card from './Card';
import styles from './FeatureCard.module.css';

type FeatureCardProps = {
  title: ReactNode;
  body: ReactNode;
  /** Heading level. `h4` where the cards sit under a group heading. */
  as?: 'h3' | 'h4';
  /** Badges or tags rendered opposite the title. */
  badges?: ReactNode;
  /** Usually a CardFooter - config key plus a docs link. */
  footer?: ReactNode;
  /** Draws the card's border in the accent colour. */
  featured?: boolean;
  className?: string;
};

/**
 * Title, blurb, and optionally a footer - the card four sections were each
 * building by hand out of Card, a Heading and a paragraph. They had already
 * drifted to three different title sizes and two body sizes.
 */
export default function FeatureCard({
  title,
  body,
  as = 'h3',
  badges,
  footer,
  featured = false,
  className,
}: FeatureCardProps) {
  return (
    <Card hover className={clsx(featured && styles.cardFeatured, className)}>
      <div className={styles.head}>
        <Heading as={as} className={styles.title}>
          {title}
        </Heading>
        {badges}
      </div>
      <p className={styles.body}>{body}</p>
      {footer}
    </Card>
  );
}

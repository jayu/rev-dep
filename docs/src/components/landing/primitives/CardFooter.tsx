import clsx from 'clsx';

import TrackedLink from './TrackedLink';
import type { ClickEvent } from '../analytics';
import styles from './CardFooter.module.css';

type CardFooterProps = {
  /** Config key, shown in monospace on the left. */
  code?: string;
  link: { to: string; label: string };
  /** The link's destination is the target, so only the rest is passed in. */
  event: Omit<ClickEvent, 'target'>;
  className?: string;
};

/** Hairline footer for a FeatureCard: config key on the left, link on the right. */
export default function CardFooter({ code, link, event, className }: CardFooterProps) {
  return (
    <div className={clsx(styles.footer, className)}>
      {code && <code className={styles.code}>{code}</code>}
      <TrackedLink className={styles.link} to={link.to} event={event}>
        {link.label}
      </TrackedLink>
    </div>
  );
}

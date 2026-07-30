import Link from '@docusaurus/Link';
import clsx from 'clsx';

import styles from './CardFooter.module.css';

type CardFooterProps = {
  /** Config key, shown in monospace on the left. */
  code?: string;
  link: { to: string; label: string };
  className?: string;
};

/** Hairline footer for a FeatureCard: config key on the left, link on the right. */
export default function CardFooter({ code, link, className }: CardFooterProps) {
  return (
    <div className={clsx(styles.footer, className)}>
      {code && <code className={styles.code}>{code}</code>}
      <Link className={styles.link} to={link.to}>
        {link.label}
      </Link>
    </div>
  );
}

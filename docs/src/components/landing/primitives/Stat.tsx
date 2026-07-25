import clsx from 'clsx';

import styles from './Stat.module.css';

type StatProps = {
  value: string;
  label: string;
  /** `panel` for use inside a dark Card, `plain` on a normal background. */
  tone?: 'panel' | 'plain';
  className?: string;
};

/** A single number + caption tile. */
export default function Stat({ value, label, tone = 'panel', className }: StatProps) {
  return (
    <div className={clsx(styles.stat, tone === 'plain' && styles.statPlain, className)}>
      <span className={clsx(styles.value, tone === 'plain' && styles.valuePlain)}>{value}</span>
      <span className={clsx(styles.label, tone === 'plain' && styles.labelPlain)}>{label}</span>
    </div>
  );
}

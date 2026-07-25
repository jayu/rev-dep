import { useCallback, useEffect, useRef, useState } from 'react';
import clsx from 'clsx';

import styles from './CopyCommand.module.css';

type CopyCommandProps = {
  command: string;
  className?: string;
};

/** Install command with click-to-copy. */
export default function CopyCommand({ command, className }: CopyCommandProps) {
  const [copied, setCopied] = useState(false);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => () => {
    if (timer.current) clearTimeout(timer.current);
  }, []);

  const copy = useCallback(() => {
    // Older Safari and any non-secure origin have no clipboard API; the command
    // is selectable text either way, so failing silently is fine.
    navigator.clipboard?.writeText(command).then(
      () => {
        setCopied(true);
        if (timer.current) clearTimeout(timer.current);
        timer.current = setTimeout(() => setCopied(false), 1800);
      },
      () => undefined,
    );
  }, [command]);

  return (
    <button
      type="button"
      onClick={copy}
      className={clsx(styles.copyCommand, className)}
      aria-label={`Copy "${command}" to clipboard`}
    >
      <span className={styles.prompt}>$</span>
      <code className={styles.command}>{command}</code>
      <span className={styles.status} aria-live="polite">
        {copied ? 'Copied' : 'Copy'}
      </span>
    </button>
  );
}

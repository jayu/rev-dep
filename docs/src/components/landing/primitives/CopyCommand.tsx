import { useCallback, useEffect, useRef, useState } from 'react';
import clsx from 'clsx';

import styles from './CopyCommand.module.css';

type CopyCommandProps = {
  command: string;
  className?: string;
};

type State = 'idle' | 'copied' | 'failed';

const LABEL: Record<State, string> = {
  idle: 'Copy',
  copied: 'Copied',
  failed: 'Select it',
};

/**
 * Clipboard fallback for origins where `navigator.clipboard` does not exist.
 *
 * That is not an edge case: the API is gated on a secure context, so it is
 * missing whenever the site is opened over plain http - a LAN address like
 * http://192.168.1.5:3000 while testing on a phone, or any http deployment.
 * execCommand is deprecated but works there, and is still supported everywhere.
 */
function legacyCopy(text: string): boolean {
  const area = document.createElement('textarea');
  area.value = text;
  area.setAttribute('readonly', '');
  // Off-screen but still selectable - display:none or visibility:hidden would
  // make select() a no-op.
  area.style.position = 'fixed';
  area.style.top = '-9999px';
  document.body.appendChild(area);

  // Copying steals the selection, so put the user's back afterwards.
  const selection = document.getSelection();
  const previous = selection && selection.rangeCount > 0 ? selection.getRangeAt(0) : null;

  area.select();

  let ok = false;
  try {
    ok = document.execCommand('copy');
  } catch {
    ok = false;
  }

  document.body.removeChild(area);
  if (selection && previous) {
    selection.removeAllRanges();
    selection.addRange(previous);
  }
  return ok;
}

/** Install command with click-to-copy. */
export default function CopyCommand({ command, className }: CopyCommandProps) {
  const [state, setState] = useState<State>('idle');
  const commandRef = useRef<HTMLElement>(null);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(
    () => () => {
      if (timer.current) clearTimeout(timer.current);
    },
    [],
  );

  const settle = useCallback((next: State) => {
    setState(next);
    if (timer.current) clearTimeout(timer.current);
    timer.current = setTimeout(() => setState('idle'), 2000);
  }, []);

  const copy = useCallback(async () => {
    let ok = false;

    // writeText can also reject on a permission denial, so the fallback has to
    // sit behind both the feature check and the catch.
    if (navigator.clipboard && window.isSecureContext) {
      try {
        await navigator.clipboard.writeText(command);
        ok = true;
      } catch {
        ok = false;
      }
    }
    if (!ok) ok = legacyCopy(command);

    // Last resort: select the command so the keyboard shortcut works. Better
    // than a button that reports success it did not achieve, or one that does
    // nothing at all and leaves you guessing.
    if (!ok) {
      const node = commandRef.current;
      const selection = document.getSelection();
      if (node && selection) {
        const range = document.createRange();
        range.selectNodeContents(node);
        selection.removeAllRanges();
        selection.addRange(range);
      }
    }

    settle(ok ? 'copied' : 'failed');
  }, [command, settle]);

  return (
    <button
      type="button"
      onClick={copy}
      className={clsx(styles.copyCommand, className)}
      aria-label={`Copy "${command}" to clipboard`}
    >
      <span className={styles.prompt}>$</span>
      <code className={styles.command} ref={commandRef}>
        {command}
      </code>
      <span
        className={clsx(
          styles.status,
          state === 'copied' && styles.statusCopied,
          state === 'failed' && styles.statusFailed,
        )}
        aria-live="polite"
      >
        {LABEL[state]}
      </span>
    </button>
  );
}

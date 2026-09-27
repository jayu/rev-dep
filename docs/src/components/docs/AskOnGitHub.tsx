import {useEffect, useRef, useState} from 'react';
import clsx from 'clsx';
import Link from '@docusaurus/Link';

import styles from './AskOnGitHub.module.css';

const DISCUSSIONS_URL = 'https://github.com/jayu/rev-dep/discussions';
const ISSUES_URL = 'https://github.com/jayu/rev-dep/issues/new';

export default function AskOnGitHub() {
  const [pinned, setPinned] = useState(false);
  const cardRef = useRef<HTMLElement>(null);
  const rowRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const card = cardRef.current;
    const row = rowRef.current;
    if (!card || !row) {
      return undefined;
    }

    const line =
      parseFloat(getComputedStyle(row).top) -
      parseFloat(getComputedStyle(card).marginBottom);

    const observer = new IntersectionObserver(
      ([entry]) => setPinned(!entry.isIntersecting),
      {rootMargin: `-${Math.round(line)}px 0px 0px 0px`},
    );
    observer.observe(card);
    return () => observer.disconnect();
  }, []);

  return (
    <>
      <aside
        ref={cardRef}
        className={clsx(styles.card, pinned && styles.cardFaded)}
        inert={pinned}>
        <p className={styles.greeting}>Hey 👋</p>
        <p className={styles.copy}>Something not clear?</p>
        <p className={clsx(styles.copy, styles.copyLast)}>
          Have a question or doubt?
        </p>

        <Link className={styles.button} href={DISCUSSIONS_URL}>
          Ask on GitHub
        </Link>

        <p className={styles.footnote}>
          Found a bug?{' '}
          <Link className={styles.footnoteLink} href={ISSUES_URL}>
            Report an issue
          </Link>
        </p>
      </aside>

      <div ref={rowRef} className={styles.pillRow}>
        <Link
          className={clsx(styles.button, styles.pill, pinned && styles.pillShown)}
          href={DISCUSSIONS_URL}
          inert={!pinned}>
          Ask on GitHub
        </Link>
      </div>
    </>
  );
}

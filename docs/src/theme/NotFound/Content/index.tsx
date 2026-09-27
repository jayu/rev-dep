import {useEffect, useState, type ReactNode} from 'react';
import clsx from 'clsx';
import Link from '@docusaurus/Link';
import Heading from '@theme/Heading';
import type {Props} from '@theme/NotFound/Content';
import Terminal, {type TerminalLine} from '@site/src/components/landing/primitives/Terminal';
import {DISCUSSIONS_URL, NEW_ISSUE_URL} from '@site/src/links';

import styles from './styles.module.css';

const QUICK_LINKS = [
  {to: '/docs/installation', label: 'Installation'},
  {to: '/docs/config-based-checks/overview', label: 'Config-based checks'},
  {to: '/docs/exploratory-toolkit/overview', label: 'Exploratory toolkit'},
];

const PLACEHOLDER_PATH = '/the/page/you/wanted';

function useRequestedPath(): string {
  const [path, setPath] = useState(PLACEHOLDER_PATH);
  useEffect(() => setPath(window.location.pathname), []);
  return path;
}

export default function NotFoundContent({className}: Props): ReactNode {
  const path = useRequestedPath();

  const lines: TerminalLine[] = [
    {text: `$ rev-dep resolve -p rev-dep.com -f ${path}`, tone: 'prompt'},
    {text: 'Error: Target file not found in dependency tree:', tone: 'bad'},
    {text: path, tone: 'bad', indent: 1},
    {gap: true},
    {text: 'Nothing links here. It moved, or it never existed.', tone: 'dim'},
  ];

  return (
    <main className={clsx('container margin-vert--xl', className, styles.page)}>
      <p className={styles.eyebrow}>Error 404</p>

      <Heading as="h1" className={styles.title}>
        This path could not be resolved
      </Heading>

      <Terminal className={styles.terminal} title="rev-dep resolve" lines={lines} />

      <p className={styles.copy}>
        Did not find what you were looking for? Ask in Discussions, or open an issue.
      </p>

      <div className={styles.actions}>
        <Link className={clsx(styles.button, styles.buttonPrimary)} href={DISCUSSIONS_URL}>
          Ask in Discussions
        </Link>
        <Link className={clsx(styles.button, styles.buttonQuiet)} href={NEW_ISSUE_URL}>
          Open an issue
        </Link>
      </div>

      <nav className={styles.quickLinks} aria-label="Popular pages">
        <span className={styles.quickLinksLabel}>Or start here:</span>
        {QUICK_LINKS.map(({to, label}) => (
          <Link key={to} className={styles.quickLink} to={to}>
            {label}
          </Link>
        ))}
      </nav>
    </main>
  );
}

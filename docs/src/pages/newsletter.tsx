import { useEffect, useRef, useState, type ReactNode } from 'react';
import clsx from 'clsx';
import Layout from '@theme/Layout';
import Link from '@docusaurus/Link';
import useBaseUrl from '@docusaurus/useBaseUrl';

import { DISCUSSIONS_URL } from '@site/src/links';
import styles from './newsletter.module.css';

const FORM_URL = 'https://06i2v.mjt.lu/wgt/06i2v/09qm/form?c=5cd9d424';
const WIDGET_SCRIPT = 'https://app.mailjet.com/pas-nc-embedded-v2.js';

const SIZE_MESSAGE_PREFIX = '[iFrameSizer]';

const BADGE_CROP_PX = 150;

const MAILJET_URL =
  'https://www.mailjet.com/?utm_source=footer&utm_medium=email&utm_campaign=logo1';

const BADGE_WIDTH = 240;
const BADGE_HEIGHT = 49;

type IFrameResize = (options: {log: boolean}, target: HTMLIFrameElement) => void;
type ResizerWindow = Window & {iFrameResize?: IFrameResize};
type ResizedFrame = HTMLIFrameElement & {iFrameResizer?: unknown};

const MIN_UNCROPPED_PX = 160;

export default function Newsletter(): ReactNode {
  const frameRef = useRef<HTMLIFrameElement>(null);
  const badgeUrl = useBaseUrl('/img/mailjet-logo.png');
  const [height, setHeight] = useState<number | null>(null);

  useEffect(() => {
    const frame = frameRef.current;
    if (!frame) return undefined;

    const onMessage = (event: MessageEvent) => {
      if (event.source !== frame.contentWindow) return;
      if (typeof event.data !== 'string' || !event.data.startsWith(SIZE_MESSAGE_PREFIX)) return;

      const height = Number(event.data.slice(SIZE_MESSAGE_PREFIX.length).split(':')[1]);
      if (!Number.isFinite(height) || height <= 0) return;

      frame.style.height = `${height}px`;
      setHeight(height);
    };

    window.addEventListener('message', onMessage);
    return () => window.removeEventListener('message', onMessage);
  }, []);

  useEffect(() => {
    const frame = frameRef.current as ResizedFrame | null;
    if (!frame) return undefined;

    const init = () => {
      const resize = (window as ResizerWindow).iFrameResize;
      if (resize && !frame.iFrameResizer) resize({log: false}, frame);
    };

    const existing = document.querySelector<HTMLScriptElement>(`script[src="${WIDGET_SCRIPT}"]`);
    if (existing) {
      init();
      existing.addEventListener('load', init);
      return () => existing.removeEventListener('load', init);
    }

    const script = document.createElement('script');
    script.src = WIDGET_SCRIPT;
    script.async = true;
    script.addEventListener('load', init);
    document.body.appendChild(script);
    return () => script.removeEventListener('load', init);
  }, []);

  const cropped = height !== null && height > BADGE_CROP_PX + MIN_UNCROPPED_PX;

  return (
    <Layout
      title="Newsletter"
      description="Email updates about rev-dep: new releases, new features and bug fixes."
    >
      <main className={styles.page}>
        <p className={styles.eyebrow}>Newsletter</p>

        <h1 className={styles.title}>Stay up to date</h1>

        <p className={styles.copy}>
          Get an email about new rev-dep releases, features and bug fixes. You can unsubscribe at
          any time.
        </p>

        <div className={styles.formCard}>
          <div
            className={styles.formCrop}
            style={cropped ? {height: `${height - BADGE_CROP_PX}px`} : undefined}
          >
            <iframe
              ref={frameRef}
              className={clsx(styles.form, height !== null && styles.formSized)}
              data-w-type="embedded"
              title="Newsletter sign-up form"
              src={FORM_URL}
              width="100%"
              frameBorder={0}
              scrolling="no"
              marginHeight={0}
              marginWidth={0}
            />
          </div>

          {cropped && (
            <a className={styles.badge} href={MAILJET_URL} target="_blank" rel="noopener noreferrer">
              <img
                className={styles.badgeImage}
                src={badgeUrl}
                alt="Powered by Mailjet"
                width={BADGE_WIDTH}
                height={BADGE_HEIGHT}
              />
            </a>
          )}
        </div>

        <p className={styles.footnote}>
          The form not loading? <Link href={FORM_URL}>Open it in a new tab</Link>, or say hello in{' '}
          <Link href={DISCUSSIONS_URL}>Discussions</Link>.
        </p>
      </main>
    </Layout>
  );
}

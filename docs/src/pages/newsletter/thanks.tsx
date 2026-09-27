import { type ReactNode } from 'react';
import Head from '@docusaurus/Head';
import Layout from '@theme/Layout';
import Link from '@docusaurus/Link';

import styles from './thanks.module.css';

export default function NewsletterThanks(): ReactNode {
  return (
    <Layout
      title="Thank you for subscribing"
      description="Your rev-dep newsletter subscription is confirmed."
    >
      <Head>
        <meta name="robots" content="noindex" />
      </Head>

      <main className={styles.page}>
        <p className={styles.eyebrow}>Newsletter</p>

        <h1 className={styles.title}>Thank you for subscribing</h1>

        <p className={styles.copy}>
          Your email is confirmed.
        </p>

        <Link className={styles.button} to="/docs/intro">
          Back to the docs
        </Link>
      </main>
    </Layout>
  );
}

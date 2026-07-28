import type { ReactNode } from 'react';
import Link from '@docusaurus/Link';
import useBaseUrl from '@docusaurus/useBaseUrl';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';

import styles from './styles.module.css';

/**
 * Swizzled so the mark and the wordmark are two separate links rather than one.
 *
 * The stock component wraps both in a single anchor, which makes the gap
 * between them part of the click target and impossible to tune independently.
 * Both still point at the homepage - they are just no longer one element.
 */
export default function NavbarLogo(): ReactNode {
  const { siteConfig } = useDocusaurusContext();
  // themeConfig is typed as `{}`, so the navbar shape has to be asserted.
  const navbar = (
    siteConfig.themeConfig as {
      navbar?: { title?: string; logo?: { src?: string; alt?: string } };
    }
  ).navbar;
  const logo = navbar?.logo;
  // navbar.title, not siteConfig.title - the latter is the long SEO title.
  const title = navbar?.title ?? siteConfig.title;
  const src = useBaseUrl(logo?.src ?? 'img/logo-mark-card.png');
  const home = useBaseUrl('/');

  return (
    <div className={styles.brand}>
      <Link to={home} className={styles.markLink} aria-label={title}>
        <img src={src} alt={logo?.alt ?? ''} className={styles.mark} width={30} height={30} />
      </Link>
      <Link to={home} className={styles.titleLink}>
        {title}
      </Link>
    </div>
  );
}

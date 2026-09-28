import {type ReactNode} from 'react';
import clsx from 'clsx';
import Link from '@docusaurus/Link';
import {translate} from '@docusaurus/Translate';
import {usePluralForm} from '@docusaurus/theme-common';
import {useDateTimeFormat} from '@docusaurus/theme-common/internal';
import {useBlogPost} from '@docusaurus/plugin-content-blog/client';
import type {Props} from '@theme/BlogPostItem/Header/Info';

import styles from './styles.module.css';

// Same pluralization as the stock component, so existing translations still apply.
function useReadingTimePlural() {
  const {selectMessage} = usePluralForm();
  return (readingTimeFloat: number) => {
    const readingTime = Math.ceil(readingTimeFloat);
    return selectMessage(
      readingTime,
      translate(
        {
          id: 'theme.blog.post.readingTime.plurals',
          description:
            'Pluralized label for "{readingTime} min read". Use as much plural forms (separated by "|") as your language support (see https://www.unicode.org/cldr/cldr-aux/charts/34/supplemental/language_plural_rules.html)',
          message: 'One min read|{readingTime} min read',
        },
        {readingTime},
      ),
    );
  };
}

function Separator(): ReactNode {
  return (
    <span className={styles.separator} aria-hidden="true">
      ·
    </span>
  );
}

function Authors(): ReactNode {
  const {
    metadata: {authors},
    assets,
  } = useBlogPost();

  const named = authors
    .map((author, idx) => ({
      ...author,
      imageURL: assets.authorsImageUrls[idx] ?? author.imageURL,
    }))
    .filter((author) => author.name);

  if (named.length === 0) {
    return null;
  }

  return (
    <span className={styles.authors}>
      {named.map((author, idx) => {
        const link =
          author.page?.permalink ||
          author.url ||
          (author.email && `mailto:${author.email}`) ||
          undefined;
        const content = (
          <>
            {author.imageURL && (
              <img className={styles.avatar} src={author.imageURL} alt="" />
            )}
            <span translate="no">{author.name}</span>
          </>
        );
        return (
          <span key={idx} className={styles.author}>
            {link ? (
              <Link className={styles.authorLink} href={link}>
                {content}
              </Link>
            ) : (
              content
            )}
          </span>
        );
      })}
    </span>
  );
}

export default function BlogPostItemHeaderInfo({className}: Props): ReactNode {
  const {metadata} = useBlogPost();
  const {date, readingTime} = metadata;
  const readingTimePlural = useReadingTimePlural();
  const dateTimeFormat = useDateTimeFormat({
    day: 'numeric',
    month: 'long',
    year: 'numeric',
    timeZone: 'UTC',
  });

  const hasAuthors = metadata.authors.some((author) => author.name);

  return (
    <div className={clsx(styles.container, className)}>
      {hasAuthors && (
        <>
          <Authors />
          <Separator />
        </>
      )}
      <time dateTime={date}>{dateTimeFormat.format(new Date(date))}</time>
      {typeof readingTime !== 'undefined' && (
        <>
          <Separator />
          <span>{readingTimePlural(readingTime)}</span>
        </>
      )}
    </div>
  );
}

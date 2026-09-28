import {type ReactNode} from 'react';
import Footer from '@theme-original/BlogPostItem/Footer';
import type FooterType from '@theme/BlogPostItem/Footer';
import type {WrapperProps} from '@docusaurus/types';
import {useBlogPost} from '@docusaurus/plugin-content-blog/client';
import BlogPostItemHeaderAuthors from '@theme/BlogPostItem/Header/Authors';

import styles from './styles.module.css';

type Props = WrapperProps<typeof FooterType>;

// The author card moved here from the header; the header shows a one-line byline.
// Only on the post page: in the list view the byline is enough.
export default function FooterWrapper(props: Props): ReactNode {
  const {isBlogPostPage} = useBlogPost();
  return (
    <>
      <Footer {...props} />
      {isBlogPostPage && (
        <div className={styles.authors}>
          <BlogPostItemHeaderAuthors />
        </div>
      )}
    </>
  );
}

import {type ReactNode} from 'react';
import BlogPostItemHeaderTitle from '@theme/BlogPostItem/Header/Title';
import BlogPostItemHeaderInfo from '@theme/BlogPostItem/Header/Info';

// The byline (author · date · reading time) sits above the title. The full
// author card is rendered by BlogPostItem/Footer instead of here.
export default function BlogPostItemHeader(): ReactNode {
  return (
    <header>
      <BlogPostItemHeaderInfo />
      <BlogPostItemHeaderTitle />
    </header>
  );
}

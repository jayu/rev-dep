import {type ReactNode} from 'react';
import clsx from 'clsx';
import {ThemeClassNames} from '@docusaurus/theme-common';
import {useDoc} from '@docusaurus/plugin-content-docs/client';
import TOC from '@theme/TOC';
import AskOnGitHub from '@site/src/components/docs/AskOnGitHub';

import styles from './styles.module.css';

export default function DocItemTOCDesktop(): ReactNode {
  const {toc, frontMatter} = useDoc();
  return (
    <>
      <AskOnGitHub />
      <TOC
        toc={toc}
        minHeadingLevel={frontMatter.toc_min_heading_level}
        maxHeadingLevel={frontMatter.toc_max_heading_level}
        className={clsx(ThemeClassNames.docs.docTocDesktop, styles.railToc)}
      />
    </>
  );
}

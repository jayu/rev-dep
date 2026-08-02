import type { ComponentProps, ReactNode } from 'react';
import Link from '@docusaurus/Link';

import { trackClick, type ClickEvent } from '../analytics';

type TrackedLinkProps = Omit<ComponentProps<typeof Link>, 'children'> & {
  /** Everything but `target`, which is filled in from `to`/`href`. */
  event: Omit<ClickEvent, 'target'>;
  children: ReactNode;
};

/**
 * Docusaurus `Link` that reports its own clicks.
 *
 * Every landing page link goes through here so the event shape stays in one
 * place. `to` and `href` are both accepted because `Link` accepts both - the
 * internal links use `to`, the ones pointing at GitHub use `href`.
 *
 * The click is reported on `onClick`, which covers plain clicks and keyboard
 * activation. Middle-click and cmd-click open a background tab without firing
 * it, so these counts are of visits intended, not tabs opened.
 */
export default function TrackedLink({ event, children, onClick, ...props }: TrackedLinkProps) {
  return (
    <Link
      {...props}
      onClick={(e) => {
        trackClick({ ...event, target: props.to ?? props.href ?? '' });
        onClick?.(e);
      }}
    >
      {children}
    </Link>
  );
}

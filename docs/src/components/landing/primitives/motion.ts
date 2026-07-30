import { useEffect, useLayoutEffect } from 'react';

/**
 * Motion helpers shared by every animated part of the landing page.
 *
 * Three components used to answer "may I animate?" and "am I on screen?" for
 * themselves, in three different ways. They live here so the answer is given
 * once - see useInView for the viewport half.
 */

/** True when the visitor has asked the OS to keep motion to a minimum. */
export function prefersReducedMotion(): boolean {
  return Boolean(window.matchMedia?.('(prefers-reduced-motion: reduce)').matches);
}

/**
 * useLayoutEffect on the client, useEffect on the server.
 *
 * Needed by the components that render their *finished* state on the server -
 * so the page works without JavaScript - and then rewind to frame zero once
 * they know an animation is going to run. Doing that in a plain effect lets the
 * finished state paint first and produces a visible flash of the whole
 * animation before it starts; a layout effect commits before paint. React warns
 * about useLayoutEffect during SSR, hence the swap.
 */
export const useIsomorphicLayoutEffect = typeof window !== 'undefined' ? useLayoutEffect : useEffect;

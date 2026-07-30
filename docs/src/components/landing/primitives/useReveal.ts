import { useState } from 'react';

import useInView from './useInView';
import { prefersReducedMotion, useIsomorphicLayoutEffect } from './motion';

type RevealState = 'shown' | 'pending' | 'revealed';

/** Fraction of the viewport height an element must reach before it reveals. */
const TRIGGER = 0.82;

/**
 * Fades and lifts an element the first time it scrolls into view.
 *
 * Starts in `shown`, which is what the server renders: if JavaScript never
 * runs, the content is simply visible rather than stuck at opacity 0. Only
 * after mount, and only for elements still below the fold, does it arm itself -
 * so nothing already on screen flashes.
 */
export default function useReveal<T extends HTMLElement = HTMLDivElement>() {
  const [armed, setArmed] = useState(false);
  const { ref, inView } = useInView<T>({ enabled: armed, trigger: TRIGGER });

  useIsomorphicLayoutEffect(() => {
    const node = ref.current;
    if (!node) return;

    if (prefersReducedMotion()) return;

    // Anything already touching the viewport stays visible: no animation, no
    // flash, and no risk of a tall block being stuck hidden at load.
    if (node.getBoundingClientRect().top < window.innerHeight) return;

    setArmed(true);
  }, []);

  const state: RevealState = !armed ? 'shown' : inView ? 'revealed' : 'pending';

  return { ref, state };
}

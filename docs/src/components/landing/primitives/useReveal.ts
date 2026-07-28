import { useEffect, useRef, useState } from 'react';

type RevealState = 'shown' | 'pending' | 'revealed';

type Entry = { node: HTMLElement; reveal: () => void };

/** Fraction of the viewport height an element must reach before it reveals. */
const TRIGGER = 0.82;

/**
 * One shared listener for every pending block.
 *
 * IntersectionObserver is the obvious tool here and it is the wrong one: it
 * only reports *changes* in intersection, so an anchor jump or a fast momentum
 * scroll that takes an element from far below the viewport to far above it in
 * a single frame produces no callback at all - and the block stays invisible
 * for good. A position sweep cannot miss that case.
 */
const pending = new Set<Entry>();
let frame = 0;

function sweep() {
  frame = 0;
  const limit = window.innerHeight * TRIGGER;

  for (const entry of pending) {
    const rect = entry.node.getBoundingClientRect();
    // Entering from below, or already scrolled past above.
    if (rect.top < limit || rect.bottom < 0) {
      entry.reveal();
      pending.delete(entry);
    }
  }

  if (pending.size === 0) {
    window.removeEventListener('scroll', schedule);
    window.removeEventListener('resize', schedule);
  }
}

function schedule() {
  if (frame) return;
  frame = requestAnimationFrame(sweep);
}

function watch(entry: Entry) {
  const first = pending.size === 0;
  pending.add(entry);
  if (first) {
    window.addEventListener('scroll', schedule, { passive: true });
    window.addEventListener('resize', schedule, { passive: true });
  }
  schedule();
}

/**
 * Fades and lifts an element the first time it scrolls into view.
 *
 * Starts in `shown`, which is what the server renders: if JavaScript never
 * runs, the content is simply visible rather than stuck at opacity 0. Only
 * after mount, and only for elements still below the fold, does it switch to
 * `pending` - so nothing already on screen flashes.
 */
export default function useReveal<T extends HTMLElement = HTMLDivElement>() {
  const ref = useRef<T>(null);
  const [state, setState] = useState<RevealState>('shown');

  useEffect(() => {
    const node = ref.current;
    if (!node) return;

    if (window.matchMedia?.('(prefers-reduced-motion: reduce)').matches) return;

    // Anything already touching the viewport stays visible: no animation, no
    // flash, and no risk of a tall block being stuck hidden at load.
    if (node.getBoundingClientRect().top < window.innerHeight) return;

    setState('pending');

    const entry: Entry = { node, reveal: () => setState('revealed') };
    watch(entry);

    return () => {
      pending.delete(entry);
    };
  }, []);

  return { ref, state };
}

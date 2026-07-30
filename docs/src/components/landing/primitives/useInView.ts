import { useEffect, useRef, useState } from 'react';

/**
 * One shared position sweep for every element waiting to come into view.
 *
 * IntersectionObserver is the obvious tool here and it is the wrong one, twice:
 *
 *   - It only reports *changes* in intersection, so an anchor jump or a fast
 *     momentum scroll that takes an element from far below the viewport to far
 *     above it in a single frame produces no callback at all.
 *   - `intersectionRatio` is a fraction of the *target*, so a threshold like
 *     0.6 is unreachable for anything taller than ~1.67 viewports. A group that
 *     tall would wait forever for a callback that cannot arrive.
 *
 * A position sweep has neither problem: it reads the current geometry, so it
 * cannot miss a transition, and it measures coverage against what could
 * possibly be on screen rather than against the element's full height.
 *
 * One listener and one rAF for the whole page, regardless of how many elements
 * are waiting.
 */

type Entry = {
  node: HTMLElement;
  test: (rect: DOMRect, viewportHeight: number) => boolean;
  fire: () => void;
};

const pending = new Set<Entry>();
let frame = 0;

function sweep() {
  frame = 0;
  const viewportHeight = window.innerHeight;

  for (const entry of pending) {
    if (entry.test(entry.node.getBoundingClientRect(), viewportHeight)) {
      pending.delete(entry);
      entry.fire();
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

export type InViewOptions = {
  /**
   * Set false to stop watching. For callers that decide after mount whether an
   * animation is going to run at all - there is no point measuring an element
   * that is going to stay in its finished state.
   */
  enabled?: boolean;
  /**
   * Fire once the element's top edge rises above this fraction of the viewport
   * height. 1 means "the moment any part of it is on screen", 0.82 waits until
   * it is a little way in. Ignored when `coverage` is set.
   */
  trigger?: number;
  /**
   * Fire once this much of the element is on screen, as a fraction of *how much
   * of it could be on screen at all*. An element twice the height of the
   * viewport reaches 1 when it fills the viewport; a short one reaches 1 when
   * it is fully visible. Use it when an animation should not start until the
   * whole group is being looked at.
   */
  coverage?: number;
};

function makeTest({ trigger = 1, coverage }: InViewOptions) {
  if (coverage === undefined) {
    // `bottom < 0` catches an element that was jumped straight past: it will
    // never come down into the trigger band, and it must not stay hidden.
    return (rect: DOMRect, viewportHeight: number) =>
      rect.top < viewportHeight * trigger || rect.bottom < 0;
  }

  return (rect: DOMRect, viewportHeight: number) => {
    if (rect.bottom < 0) return true;
    const visible = Math.min(rect.bottom, viewportHeight) - Math.max(rect.top, 0);
    const reachable = Math.min(rect.height, viewportHeight);
    return reachable > 0 && visible / reachable >= coverage;
  };
}

/**
 * Reports - once - when the referenced element has come into view.
 *
 * `inView` only ever goes false to true. Everything on this page is a one-shot
 * entrance, so there is nothing to reverse when it scrolls back out.
 */
export default function useInView<T extends HTMLElement = HTMLDivElement>(
  options: InViewOptions = {},
) {
  const ref = useRef<T>(null);
  const [inView, setInView] = useState(false);
  const { enabled = true, trigger, coverage } = options;

  useEffect(() => {
    const node = ref.current;
    if (!node || !enabled) return;

    const entry: Entry = {
      node,
      test: makeTest({ trigger, coverage }),
      fire: () => setInView(true),
    };
    watch(entry);

    return () => {
      pending.delete(entry);
    };
  }, [enabled, trigger, coverage]);

  return { ref, inView };
}

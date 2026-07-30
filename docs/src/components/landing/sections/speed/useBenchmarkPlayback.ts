import { useCallback, useEffect, useRef, useState } from 'react';

import useInView from '../../primitives/useInView';
import { prefersReducedMotion, useIsomorphicLayoutEffect } from '../../primitives/motion';
import formatMs from './formatMs';

/**
 * How much of the group has to be on screen before the clock starts. Measured
 * against how much of it *can* be on screen - see useInView - so a group taller
 * than the viewport still qualifies once it fills it.
 */
const COVERAGE = 0.9;

export type BenchmarkTrack = {
  bar: HTMLElement;
  value: HTMLElement;
  /** How long this row runs, in benchmark milliseconds. */
  ms: number;
  /** What a full-width track represents, in benchmark milliseconds. */
  scaleMs: number;
  /** Toggled while the bar is empty, so min-width does not draw a stub. */
  emptyClass: string;
  /** Toggled once this row has finished, if the chart distinguishes that. */
  doneClass?: string;
};

export type RegisterTrack = (track: BenchmarkTrack) => () => void;

function paint(track: BenchmarkTrack, shown: number) {
  track.bar.style.width = `${(shown / track.scaleMs) * 100}%`;
  track.bar.classList.toggle(track.emptyClass, shown === 0);
  track.value.textContent = formatMs(shown);
  if (track.doneClass) track.value.classList.toggle(track.doneClass, shown >= track.ms);
}

/**
 * Drives a group of bars from one clock at 1:1 real time - no compression.
 * A tool that takes 13.6s on the real benchmark fills for 13.6s here.
 *
 * Every bar starts together and clamps the shared elapsed time to its own
 * runtime, so the fast ones visibly finish while the slow ones grind on - that
 * contrast is the whole argument.
 *
 * Playback writes to the DOM directly rather than through state. At 60fps for
 * the length of a 13.6-second benchmark, a `setElapsed` per frame re-rendered
 * the entire section - both charts, every card, the methodology panel - around
 * eight hundred times, and reformatted every visible number on each pass. The
 * bars are the only thing that changes, so they are the only thing written.
 *
 * Rows render *finished* on the server, so the chart is a real chart without
 * JavaScript. The rewind to zero happens in a layout effect, before paint, and
 * only when there is going to be an animation to watch.
 */
export default function useBenchmarkPlayback(durationMs: number) {
  const [armed, setArmed] = useState(false);
  const { ref, inView } = useInView<HTMLDivElement>({ enabled: armed, coverage: COVERAGE });
  const tracks = useRef(new Set<BenchmarkTrack>());

  const register = useCallback<RegisterTrack>((track) => {
    tracks.current.add(track);
    return () => {
      tracks.current.delete(track);
    };
  }, []);

  useIsomorphicLayoutEffect(() => {
    if (prefersReducedMotion()) return;

    // Children register in their own layout effects, which React runs before
    // this one, so every bar is already known here.
    for (const track of tracks.current) paint(track, 0);
    setArmed(true);
  }, []);

  useEffect(() => {
    if (!armed || !inView) return;

    let frame = 0;
    let startTime = 0;
    const rows = tracks.current;

    const tick = (now: number) => {
      if (!startTime) startTime = now;
      const elapsed = Math.min(now - startTime, durationMs);
      for (const track of rows) paint(track, Math.min(elapsed, track.ms));
      if (elapsed < durationMs) frame = requestAnimationFrame(tick);
    };

    frame = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(frame);
  }, [armed, inView, durationMs]);

  return { ref, register };
}

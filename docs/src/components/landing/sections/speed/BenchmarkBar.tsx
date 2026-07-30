import { useRef } from 'react';
import clsx from 'clsx';

import { useIsomorphicLayoutEffect } from '../../primitives/motion';
import formatMs from './formatMs';
import type { RegisterTrack } from './useBenchmarkPlayback';

type BenchmarkBarProps = {
  /** How long this row runs, in benchmark milliseconds. */
  ms: number;
  /** What a full-width track represents, in benchmark milliseconds. */
  scaleMs: number;
  register: RegisterTrack;
  /** Class names from the calling chart's own stylesheet. */
  classes: { track: string; bar: string; empty: string; value: string; done?: string };
};

/**
 * One filling track plus its running total, as a fragment: both charts lay
 * these out as cells of their own grid, so the pair must not be wrapped.
 *
 * Rendered at its finished width, which is what the server emits and what a
 * visitor without JavaScript keeps. Playback rewinds it to zero on mount and
 * writes width and text imperatively from then on - React never re-renders it,
 * so the props it was given stay put and nothing overwrites the live values.
 */
export default function BenchmarkBar({ ms, scaleMs, register, classes }: BenchmarkBarProps) {
  const barRef = useRef<HTMLDivElement>(null);
  const valueRef = useRef<HTMLSpanElement>(null);

  useIsomorphicLayoutEffect(() => {
    const bar = barRef.current;
    const value = valueRef.current;
    if (!bar || !value) return;

    return register({
      bar,
      value,
      ms,
      scaleMs,
      emptyClass: classes.empty,
      doneClass: classes.done,
    });
  }, [register, ms, scaleMs, classes.empty, classes.done]);

  return (
    <>
      <div className={classes.track}>
        <div className={classes.bar} ref={barRef} style={{ width: `${(ms / scaleMs) * 100}%` }} />
      </div>
      <span className={clsx(classes.value, classes.done)} ref={valueRef}>
        {formatMs(ms)}
      </span>
    </>
  );
}

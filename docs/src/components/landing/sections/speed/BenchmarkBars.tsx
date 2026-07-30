import type { RefObject } from 'react';
import clsx from 'clsx';

import type { BenchmarkRow } from '../../data/benchmarks';
import BenchmarkBar from './BenchmarkBar';
import type { RegisterTrack } from './useBenchmarkPlayback';
import styles from './BenchmarkBars.module.css';

type BenchmarkBarsProps = {
  rows: BenchmarkRow[];
  /** What a full-width track represents: the slowest tool in the group. */
  scaleMs: number;
  containerRef: RefObject<HTMLDivElement | null>;
  register: RegisterTrack;
};

/**
 * The headline race: every tool on one shared axis, so the bars can be compared
 * directly against each other.
 */
export default function BenchmarkBars({
  rows,
  scaleMs,
  containerRef,
  register,
}: BenchmarkBarsProps) {
  return (
    <div className={styles.chart} ref={containerRef}>
      {rows.map((row) => (
        <div className={styles.row} key={row.tool}>
          <div className={styles.rowHead}>
            <span className={clsx(styles.toolName, row.ours && styles.toolNameOurs)}>
              {row.tool}
            </span>
            {row.note && <span className={styles.toolNote}>{row.note}</span>}
          </div>

          <BenchmarkBar
            ms={row.ms}
            scaleMs={scaleMs}
            register={register}
            classes={{
              track: styles.track,
              bar: clsx(styles.bar, row.ours ? styles.barOurs : styles.barRival),
              empty: styles.barEmpty,
              value: clsx(styles.value, row.ours && styles.valueOurs),
              done: styles.valueDone,
            }}
          />
        </div>
      ))}
    </div>
  );
}

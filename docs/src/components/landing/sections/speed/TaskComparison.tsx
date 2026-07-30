import type { RefObject } from 'react';
import clsx from 'clsx';
import Heading from '@theme/Heading';

import Grid from '../../primitives/Grid';
import type { TaskRow } from '../../data/benchmarks';
import BenchmarkBar from './BenchmarkBar';
import type { RegisterTrack } from './useBenchmarkPlayback';
import styles from './TaskComparison.module.css';

type TaskComparisonProps = {
  rows: TaskRow[];
  containerRef: RefObject<HTMLDivElement | null>;
  register: RegisterTrack;
};

/**
 * The remaining eight tasks, each as its own two-bar chart scaled to its own
 * alternative - so the shape of each gap reads without a shared axis.
 */
export default function TaskComparison({ rows, containerRef, register }: TaskComparisonProps) {
  return (
    <div className={styles.tasks} ref={containerRef}>
      <Heading as="h3" className={styles.tasksTitle}>
        The same gap on every other check
      </Heading>
      <p className={styles.tasksIntro}>
        Eight more tasks, each against the fastest tool that does the same job. Same project, same
        machine, same method.
      </p>

      <Grid cols={2} className={styles.taskGrid}>
        {rows.map((row, i) => {
          // The vertical run is drawn once on the grid container. Each
          // horizontal run is drawn by the left-column tile of a row and spans
          // both columns, so the two cross rather than stopping short.
          const isLeftColumn = i % 2 === 0;
          const isLast = i === rows.length - 1;
          const isDesktopLastRow = i >= rows.length - 2;

          return (
            <div
              className={clsx(
                styles.task,
                isLeftColumn && !isDesktopLastRow && styles.taskLineBottom,
                !isLeftColumn && !isLast && styles.taskLineBottomMobile,
              )}
              key={row.task}
            >
              <div className={styles.taskHead}>
                <span className={styles.taskName}>{row.task}</span>
                <span className={styles.taskFactor}>{row.factor} faster</span>
              </div>

              <div className={styles.taskBars}>
                <span className={styles.taskLabelOurs}>rev-dep</span>
                <BenchmarkBar
                  ms={row.oursMs}
                  scaleMs={row.rivalMs}
                  register={register}
                  classes={{
                    track: styles.taskTrack,
                    bar: styles.taskBarOurs,
                    empty: styles.barEmpty,
                    value: styles.taskValueOurs,
                  }}
                />

                <span className={styles.taskLabel}>{row.rivalName}</span>
                <BenchmarkBar
                  ms={row.rivalMs}
                  scaleMs={row.rivalMs}
                  register={register}
                  classes={{
                    track: styles.taskTrack,
                    bar: styles.taskBarRival,
                    empty: styles.barEmpty,
                    value: styles.taskValue,
                  }}
                />
              </div>
            </div>
          );
        })}
      </Grid>
    </div>
  );
}

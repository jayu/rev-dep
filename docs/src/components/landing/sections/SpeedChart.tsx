import { useEffect, useRef, useState } from 'react';
import clsx from 'clsx';

import Section from '../primitives/Section';
import Card from '../primitives/Card';
import Grid from '../primitives/Grid';
import {
  circularBenchmark,
  taskComparison,
  benchmarkContext,
  knipFootnote,
  outlier,
} from '../data/benchmarks';
import styles from './SpeedChart.module.css';

const maxMs = Math.max(...circularBenchmark.map((row) => row.ms));
const taskMaxMs = Math.max(...taskComparison.map((row) => row.rivalMs));

function formatMs(ms: number): string {
  return `${Math.round(ms).toLocaleString('en-US').replace(/,/g, ' ')} ms`;
}

function prefersReducedMotion(): boolean {
  return Boolean(window.matchMedia?.('(prefers-reduced-motion: reduce)').matches);
}

/**
 * Drives a group of bars from one clock at 1:1 real time - no compression.
 * A tool that takes 13.6s on the real benchmark fills for 13.6s here.
 *
 * `elapsed` is milliseconds of benchmark time, so each bar just clamps it to
 * its own runtime. Every bar starts together, so the fast ones visibly finish
 * while the slow ones grind on - that contrast is the whole argument.
 *
 * Playback only begins once the entire group is on screen; otherwise the
 * quickest rows have already finished by the time they are scrolled into view.
 */
function useBenchmarkPlayback(durationMs: number) {
  const ref = useRef<HTMLDivElement>(null);
  const [elapsed, setElapsed] = useState(0);
  const [started, setStarted] = useState(false);

  useEffect(() => {
    const node = ref.current;
    if (!node) return;

    if (prefersReducedMotion() || typeof IntersectionObserver === 'undefined') {
      setStarted(true);
      setElapsed(durationMs);
      return;
    }

    let frame = 0;
    let startTime = 0;

    const tick = (now: number) => {
      if (!startTime) startTime = now;
      const real = now - startTime;
      setElapsed(Math.min(real, durationMs));
      if (real < durationMs) frame = requestAnimationFrame(tick);
    };

    // A group taller than the viewport can never be 100% visible, so relax the
    // requirement rather than never starting at all.
    const fitsOnScreen = node.getBoundingClientRect().height <= window.innerHeight;
    const threshold = fitsOnScreen ? 0.99 : 0.6;

    const observer = new IntersectionObserver(
      (entries) => {
        if (!entries.some((e) => e.intersectionRatio >= threshold)) return;
        observer.disconnect();
        setStarted(true);
        frame = requestAnimationFrame(tick);
      },
      { threshold: [threshold, 1] },
    );

    observer.observe(node);
    return () => {
      observer.disconnect();
      cancelAnimationFrame(frame);
    };
  }, [durationMs]);

  return { ref, elapsed, started };
}

export default function SpeedChart() {
  const main = useBenchmarkPlayback(maxMs);
  const tasks = useBenchmarkPlayback(taskMaxMs);

  return (
    <Section
      id="speed"
      tone="tinted"
      eyebrow="Performance"
      title="Other tools take seconds. Rev-dep takes milliseconds."
      intro="Circular import detection is the one check every tool below implements, so it is the only place all seven can be lined up at once. The bars run at real speed: each one takes exactly as long to fill as that tool takes. rev-dep is done in 154 milliseconds. madge keeps going for another thirteen seconds."
      aside={
        <Card className={styles.setupCard}>
          <p className={styles.setupTitle}>Benchmark setup</p>
          <dl className={styles.setup}>
            <dt className={styles.setupTerm}>Project</dt>
            <dd className={styles.setupValue}>{benchmarkContext.project}</dd>
            <dt className={styles.setupTerm}>Hardware</dt>
            <dd className={styles.setupValue}>{benchmarkContext.hardware}</dd>
            <dt className={styles.setupTerm}>Method</dt>
            <dd className={styles.setupValue}>{benchmarkContext.method}</dd>
          </dl>
        </Card>
      }
    >
      <div className={styles.chart} ref={main.ref}>
        {circularBenchmark.map((row) => {
          const shown = Math.min(main.elapsed, row.ms);
          const done = main.started && main.elapsed >= row.ms;

          return (
            <div className={styles.row} key={row.tool}>
              <div className={styles.rowHead}>
                <span className={clsx(styles.toolName, row.ours && styles.toolNameOurs)}>
                  {row.tool}
                </span>
                {row.note && <span className={styles.toolNote}>{row.note}</span>}
              </div>

              <div className={styles.track}>
                <div
                  className={clsx(
                    styles.bar,
                    row.ours ? styles.barOurs : styles.barRival,
                    // min-width gives the shortest bar a clean pill shape, but
                    // it must not apply before the run starts or every bar
                    // shows a stub instead of being empty.
                    shown === 0 && styles.barEmpty,
                  )}
                  style={{ width: main.started ? `${(shown / maxMs) * 100}%` : '0%' }}
                />
              </div>

              <span
                className={clsx(
                  styles.value,
                  row.ours && styles.valueOurs,
                  done && styles.valueDone,
                )}
              >
                {formatMs(shown)}
              </span>
            </div>
          );
        })}
      </div>

      <p className={styles.outlier}>{outlier.note}</p>

      <p className={styles.footnote}>{knipFootnote}</p>

      <div className={styles.tasks} ref={tasks.ref}>
        <h3 className={styles.tasksTitle}>The same gap on every other check</h3>
        <p className={styles.tasksIntro}>
          Eight more tasks, each against the fastest tool that does the same job. Same project, same
          machine, same method.
        </p>

        <Grid cols={2} className={styles.taskGrid}>
          {taskComparison.map((row, i) => {
            // The vertical run is drawn once on the grid container. Each
            // horizontal run is drawn by the left-column tile of a row and
            // spans both columns, so the two cross rather than stopping short.
            const isLeftColumn = i % 2 === 0;
            const isLast = i === taskComparison.length - 1;
            const isDesktopLastRow = i >= taskComparison.length - 2;
            // Each task is scaled to its own alternative, so the shape of the
            // gap reads without a shared axis across tasks.
            const ours = Math.min(tasks.elapsed, row.oursMs);
            const rival = Math.min(tasks.elapsed, row.rivalMs);

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
                  <div className={styles.taskTrack}>
                    <div
                      className={clsx(styles.taskBarOurs, ours === 0 && styles.barEmpty)}
                      style={{
                        width: tasks.started ? `${(ours / row.rivalMs) * 100}%` : '0%',
                      }}
                    />
                  </div>
                  <span className={styles.taskValueOurs}>{formatMs(ours)}</span>

                  <span className={styles.taskLabel}>{row.rivalName}</span>
                  <div className={styles.taskTrack}>
                    <div
                      className={clsx(styles.taskBarRival, rival === 0 && styles.barEmpty)}
                      style={{
                        width: tasks.started ? `${(rival / row.rivalMs) * 100}%` : '0%',
                      }}
                    />
                  </div>
                  <span className={styles.taskValue}>{formatMs(rival)}</span>
                </div>
              </div>
            );
          })}
        </Grid>
      </div>

      <p className={styles.closing}>
        Every number above is a <strong>single check</strong>, measured on its own. In practice the
        gap gets wider: running all twelve checks costs rev-dep almost nothing extra, because the
        graph is built once and shared between them. Three separate tools build it three times.
      </p>
    </Section>
  );
}

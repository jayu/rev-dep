import Section from '../primitives/Section';
import Card from '../primitives/Card';
import MicroLabel from '../primitives/MicroLabel';
import {
  circularBenchmark,
  taskComparison,
  benchmarkContext,
  outlier,
} from '../data/benchmarks';
import BenchmarkBars from './speed/BenchmarkBars';
import TaskComparison from './speed/TaskComparison';
import useBenchmarkPlayback from './speed/useBenchmarkPlayback';
import styles from './SpeedChart.module.css';

/** Both groups are drawn to the slowest bar in them. */
const maxMs = Math.max(...circularBenchmark.map((row) => row.ms));
const taskMaxMs = Math.max(...taskComparison.map((row) => row.rivalMs));

export default function SpeedChart() {
  // One clock per group: each starts when its own group is on screen, so the
  // fast rows are not already finished by the time you scroll to them.
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
          <MicroLabel>Benchmark setup</MicroLabel>
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
      <BenchmarkBars
        rows={circularBenchmark}
        scaleMs={maxMs}
        containerRef={main.ref}
        register={main.register}
      />

      <p className={styles.outlier}>{outlier.note}</p>

      <TaskComparison rows={taskComparison} containerRef={tasks.ref} register={tasks.register} />

      <p className={styles.closing}>
        Every number above is a <strong>single check</strong>, measured on its own. In practice the
        gap gets wider: running all twelve checks costs rev-dep almost nothing extra, because the
        graph is built once and shared between them. Three separate tools build it three times.
      </p>
    </Section>
  );
}

import Section from '../primitives/Section';
import Card from '../primitives/Card';
import Grid from '../primitives/Grid';
import Stat from '../primitives/Stat';
import { projectProfiles, scaleCeilings } from '../data/projects';
import styles from './RealProjects.module.css';

export default function RealProjects() {
  return (
    <Section
      eyebrow="In production"
      title="From 400-file packages to 197-workspace monorepos"
      intro="Anonymous usage data from projects running rev-dep. Each card is one real codebase, except the last, which is the median. The interesting part is the range."
    >
      <Grid cols={5} gap="tight">
        {projectProfiles.map((project) => (
          <Card key={project.kind} hover>
            <p className={styles.kind}>
              {project.kind}
              {project.region && <span className={styles.region}> · {project.region}</span>}
            </p>
            <p className={styles.headline}>{project.headline}</p>
            <p className={styles.headlineLabel}>{project.headlineLabel}</p>
            <ul className={styles.facts}>
              {project.facts.map((fact) => (
                <li className={styles.fact} key={fact}>
                  {fact}
                </li>
              ))}
            </ul>
          </Card>
        ))}
      </Grid>

      <div className={styles.ceilings}>
        <p className={styles.ceilingsTitle}>Largest recorded so far</p>
        <Grid cols={4} gap="tight">
          {scaleCeilings.map((ceiling) => (
            <Stat key={ceiling.label} tone="plain" value={ceiling.value} label={ceiling.label} />
          ))}
        </Grid>
      </div>
    </Section>
  );
}

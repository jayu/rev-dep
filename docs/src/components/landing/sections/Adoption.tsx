import Section from '../primitives/Section';
import Split from '../primitives/Split';
import Grid from '../primitives/Grid';
import FeatureCard from '../primitives/FeatureCard';
import Terminal from '../primitives/Terminal';
import { initOutput, adoptionSteps } from '../data/adoption';
import styles from './Adoption.module.css';

export default function Adoption() {
  return (
    <Section
      id="adoption"
      corner="bottom-left"
      eyebrow="Getting started"
      title="You will get a lot of findings on day one. That is the point."
      intro="Every tool like this shows a backlog on the first run - it is the work that piled up while nothing was checking. What matters is that it is a one-time cleanup, and that you can take it in pieces instead of all at once. After that, a normal pull request surfaces a handful of issues at most."
    >
      <Split left="1fr" right="1.1fr" align="center">
        <div className={styles.visual}>
          <Terminal lines={initOutput} title="rev-dep config init" />
        </div>

        <Grid cols={2} gap="tight" className={styles.steps}>
          {adoptionSteps.map((step) => (
            <FeatureCard key={step.title} title={step.title} body={step.body} />
          ))}
        </Grid>
      </Split>
    </Section>
  );
}

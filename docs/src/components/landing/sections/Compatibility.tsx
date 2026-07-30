import Section from '../primitives/Section';
import Grid from '../primitives/Grid';
import FeatureCard from '../primitives/FeatureCard';
import CardFooter from '../primitives/CardFooter';
import MicroLabel from '../primitives/MicroLabel';
import { compatibilityItems, resolutionLimits } from '../data/compatibility';
import styles from './Compatibility.module.css';

export default function Compatibility() {
  return (
    <Section
      id="compatibility"
      eyebrow="Resolution"
      title="Built for the setup you actually have"
      intro="A dependency checker is only useful if it resolves imports the same way your build does. rev-dep follows the same rules your bundler, TypeScript and package manager already follow - so the graph it reports is the graph you ship."
    >
      <Grid cols={3} gap="tight">
        {compatibilityItems.map((item) => (
          <FeatureCard
            key={item.title}
            title={item.title}
            body={item.body}
            footer={<CardFooter code={item.key} link={{ to: item.docs, label: 'Docs' }} />}
          />
        ))}
      </Grid>

      <div className={styles.limitsRow}>
        <MicroLabel>Known limits</MicroLabel>
        <ul className={styles.limits}>
          {resolutionLimits.map((limit) => (
            <li className={styles.limit} key={limit}>
              {limit}
            </li>
          ))}
        </ul>
      </div>
    </Section>
  );
}

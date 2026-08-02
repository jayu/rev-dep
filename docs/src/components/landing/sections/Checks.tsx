import Heading from '@theme/Heading';

import Section from '../primitives/Section';
import Grid from '../primitives/Grid';
import FeatureCard from '../primitives/FeatureCard';
import CardFooter from '../primitives/CardFooter';
import { checkGroups } from '../data/checks';
import styles from './Checks.module.css';

export default function Checks() {
  return (
    <Section
      id="checks"
      eyebrow="What it catches"
      title="Multiple checks, unlimited workspaces, one extremely fast pass"
      intro="Grouped by the problem they solve. Turn on what you need - each check is opt-in per workspace."
    >
      {checkGroups.map((group) => (
        <div className={styles.group} key={group.id}>
          <div className={styles.groupHead}>
            <Heading as="h3" className={styles.groupTitle}>
              {group.title}
            </Heading>
            <p className={styles.groupBlurb}>{group.blurb}</p>
          </div>

          <Grid cols="auto" gap="tight">
            {group.checks.map((check) => (
              <FeatureCard
                key={check.key}
                as="h4"
                title={check.title}
                body={check.description}
                featured={check.featured}
                badges={
                  <>
                    {check.autofix && <span className={styles.fixBadge}>--fix</span>}
                    {check.featured && <span className={styles.featuredTag}>Start here</span>}
                  </>
                }
                footer={
                  <CardFooter
                    code={check.key}
                    link={{ to: check.docs, label: 'Docs' }}
                    // The config key is the stable identity here - the card
                    // titles are copy and get reworded.
                    event={{ id: `check_docs_${check.key}`, section: 'checks', type: 'docs_link' }}
                  />
                }
              />
            ))}
          </Grid>
        </div>
      ))}
    </Section>
  );
}

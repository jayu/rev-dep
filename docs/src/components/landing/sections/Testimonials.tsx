import Link from '@docusaurus/Link';

import Section from '../primitives/Section';
import Card from '../primitives/Card';
import Grid from '../primitives/Grid';
import testimonialCandidates from '../../../data/testimonialCandidates.json';
import styles from './Testimonials.module.css';

const testimonials = [
  testimonialCandidates.candidates[1],
  testimonialCandidates.candidates[2],
  testimonialCandidates.candidates[0],
];

export default function Testimonials() {
  return (
    <Section
      eyebrow="Feedback"
      title="What early users say"
      intro="Unprompted comments from public GitHub threads, posted while teams were evaluating rev-dep on their own codebases. Every one links back to the issue it came from."
    >
      <Grid cols={3}>
        {testimonials.map((testimonial) => (
          <Card key={`${testimonial.author.login}-${testimonial.source.issueNumber}`} hover>
            <p className={styles.quote}>“{testimonial.quote}”</p>
            <div className={styles.footer}>
              <img
                src={testimonial.author.avatarUrl}
                alt=""
                className={styles.avatar}
                loading="lazy"
                width={40}
                height={40}
              />
              <div className={styles.meta}>
                <Link className={styles.author} href={testimonial.author.profileUrl}>
                  {testimonial.author.name}
                </Link>
                <Link
                  className={styles.source}
                  href={testimonial.source.commentUrl ?? testimonial.source.issueUrl}
                >
                  Issue #{testimonial.source.issueNumber}
                </Link>
              </div>
            </div>
          </Card>
        ))}
      </Grid>
    </Section>
  );
}

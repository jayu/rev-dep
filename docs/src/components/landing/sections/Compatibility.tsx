import Section from '../primitives/Section';
import Grid from '../primitives/Grid';
import FeatureCard from '../primitives/FeatureCard';
import CardFooter from '../primitives/CardFooter';
import { compatibilityItems } from '../data/compatibility';

export default function Compatibility() {
  return (
    <Section
      id="compatibility"
      eyebrow="Resolution"
      title="A setting for the way your project actually resolves"
      intro="A dependency checker is only as accurate as its resolution, and no tool can infer every setup. rev-dep does not try to: each way real projects resolve imports is a setting you point at yours. Expect to spend time on the first config - that is the part that makes every finding afterwards trustworthy."
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
    </Section>
  );
}

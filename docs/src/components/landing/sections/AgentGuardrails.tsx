import Section from '../primitives/Section';
import Split from '../primitives/Split';
import FeatureCard from '../primitives/FeatureCard';
import Terminal from '../primitives/Terminal';
import { agentJsonOutput, agentPoints } from '../data/agents';
import styles from './AgentGuardrails.module.css';

export default function AgentGuardrails() {
  return (
    <Section
      id="agents"
      tone="tinted"
      corner="top-right"
      eyebrow="AI-generated code"
      title="Agents write fast. Something has to check the structure."
      intro="Coding agents produce a lot of code quickly. They also leave abandoned files, duplicate helpers, import cycles they cannot see, and boundary violations - because they never read your architecture decisions. And that debris makes the next agent run worse: reasoning budget gets spent on code nothing runs."
    >
      <Split align="center">
        <div className={styles.points}>
          {agentPoints.map((point) => (
            <FeatureCard key={point.title} title={point.title} body={point.body} />
          ))}
        </div>

        <div className={styles.visual}>
          <Terminal lines={agentJsonOutput} title="machine-readable output" />
          <p className={styles.caption}>
            Add <code className={styles.inlineCode}>rev-dep config run --format json</code> to your{' '}
            <code className={styles.inlineCode}>AGENTS.md</code> or{' '}
            <code className={styles.inlineCode}>CLAUDE.md</code>. The agent runs it after editing,
            reads the exact locations, and fixes its own mistakes before you see the pull request.
          </p>
        </div>
      </Split>
    </Section>
  );
}

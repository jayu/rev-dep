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
      intro="Coding agents can produce code at remarkable speed. They can also leave behind abandoned files, hidden import cycles, and architectural boundary violations-because they don't understand the reasoning behind your system design. And because each task is answered in isolation, they write the same helper a third time instead of finding the two that already exist. Left unchecked, task after task, they gradually turn a well-structured codebase into a dumping ground."
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
            <code className={styles.inlineCode}>AGENTS.md</code>. The agent runs it after editing,
            reads the exact locations, and fixes its own mistakes before you see the pull request.
          </p>
        </div>
      </Split>
    </Section>
  );
}

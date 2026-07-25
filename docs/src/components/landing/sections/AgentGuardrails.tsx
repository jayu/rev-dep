import Heading from '@theme/Heading';

import Section from '../primitives/Section';
import Card from '../primitives/Card';
import Terminal, { type TerminalLine } from '../primitives/Terminal';
import styles from './AgentGuardrails.module.css';

/** Real shape of `--format json`: typed issues with exact line/column ranges. */
const jsonOutput: TerminalLine[] = [
  { text: '$ rev-dep config run --format json', tone: 'prompt' },
  { text: '' },
  { text: '{' },
  { text: '  "version": "2.0",' },
  { text: '  "hasFailures": true,', tone: 'bad' },
  { text: '  "workspaces": [' },
  { text: '    {' },
  { text: '      "path": "apps/web",' },
  { text: '      "checks": {' },
  { text: '        "moduleBoundaries": {', tone: 'accent' },
  { text: '          "status": "fail",', tone: 'bad' },
  { text: '          "issues": [' },
  { text: '            {' },
  { text: '              "ruleName": "ui-cannot-import-api",' },
  { text: '              "filePath": "src/ui/Chart.tsx",' },
  { text: '              "importPath": "src/api/client.ts",' },
  { text: '              "startLine": 3,', tone: 'ok' },
  { text: '              "startCol": 1', tone: 'ok' },
  { text: '            }' },
  { text: '          ]' },
  { text: '        }' },
  { text: '      }' },
  { text: '    }' },
  { text: '  ]' },
  { text: '}' },
];

const points = [
  {
    title: 'Deterministic, not another model',
    body: 'No LLM inside. The same code gives the same verdict every time, so it can gate a loop that is already probabilistic.',
  },
  {
    title: 'Exact edit locations',
    body: 'Every issue carries file, line and column. The agent jumps straight to the fix instead of re-reading the repo to find it.',
  },
  {
    title: 'Fast enough to run every time',
    body: 'A sub-second check can run after each edit. A ten-second one gets run at the end, if at all.',
  },
  {
    title: 'It can clean up after itself',
    body: 'rev-dep config run --fix --recheck removes dead files and unused exports, then verifies the result in one command.',
  },
];

export default function AgentGuardrails() {
  return (
    <Section
      id="agents"
      tone="tinted"
      eyebrow="AI-generated code"
      title="Agents write fast. Something has to check the structure."
      intro="Coding agents produce a lot of code quickly. They also leave abandoned files, duplicate helpers, import cycles they cannot see, and boundary violations - because they never read your architecture decisions. And that debris makes the next agent run worse: reasoning budget gets spent on code nothing runs."
    >
      <div className={styles.layout}>
        <div className={styles.points}>
          {points.map((point) => (
            <Card key={point.title}>
              <Heading as="h3" className={styles.pointTitle}>
                {point.title}
              </Heading>
              <p className={styles.pointBody}>{point.body}</p>
            </Card>
          ))}
        </div>

        <div className={styles.visual}>
          <Terminal lines={jsonOutput} title="machine-readable output" />
          <p className={styles.caption}>
            Add <code className={styles.inlineCode}>rev-dep config run --format json</code> to your{' '}
            <code className={styles.inlineCode}>AGENTS.md</code> or{' '}
            <code className={styles.inlineCode}>CLAUDE.md</code>. The agent runs it after editing,
            reads the exact locations, and fixes its own mistakes before you see the pull request.
          </p>
        </div>
      </div>
    </Section>
  );
}

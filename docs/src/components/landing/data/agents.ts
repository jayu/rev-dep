import type { TerminalLine } from '../primitives/Terminal';

/**
 * Copy and transcript for the AI-generated-code section.
 *
 * Here rather than in AgentGuardrails.tsx so every section keeps its wording in
 * the same place: sections own markup, data modules own content.
 */

/** Real shape of `--format json`: typed issues with exact line/column ranges. */
export const agentJsonOutput: TerminalLine[] = [
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
  { text: '        },' },
  { text: '        "duplicatedCode": [{', tone: 'accent' },
  { text: '          "status": "fail",', tone: 'bad' },
  { text: '          "blinding": "exact",' },
  { text: '          "snippets": 3,', tone: 'ok' },
  { text: '          "occurrences": 9,', tone: 'ok' },
  { text: '          "files": 6,' },
  { text: '          "configIndex": 0' },
  { text: '        }]' },
  { text: '      }' },
  { text: '    }' },
  { text: '  ]' },
  { text: '}' },
];

export type AgentPoint = {
  title: string;
  body: string;
};

/** Why a deterministic checker is the right thing to put next to an agent. */
export const agentPoints: AgentPoint[] = [
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

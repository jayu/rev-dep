export type ProjectProfile = {
  kind: string;
  headline: string;
  headlineLabel: string;
  facts: string[];
  region?: string;
};

export const projectProfiles: ProjectProfile[] = [
  {
    kind: 'Large monorepo',
    region: 'Italy',
    headline: '197',
    headlineLabel: 'workspaces in one repo',
    facts: ['3,473 source files', '5 checks configured', 'Mostly run locally'],
  },
  {
    kind: 'Large single codebase',
    region: 'Japan',
    headline: '17,604',
    headlineLabel: 'source files, one workspace',
    facts: ['No monorepo required', 'The largest tracked codebase'],
  },
  {
    kind: 'Architecture-first',
    region: 'Germany',
    headline: '104',
    headlineLabel: 'module boundary rules',
    facts: ['9,288 source files', 'One workspace, heavily governed'],
  },
  {
    kind: 'Whole engineering team',
    region: 'United States',
    headline: '25',
    headlineLabel: 'developers on one codebase',
    facts: ['8,339 source files', '174 workspaces', 'Run locally, not only in CI'],
  },
  {
    kind: 'Typical single package',
    headline: '374',
    headlineLabel: 'source files',
    facts: ['The median tracked project', '2 workspaces', 'Same binary, same config'],
  },
];

export const scaleCeilings = [
  { value: '17,604', label: 'files in the largest codebase' },
  { value: '197', label: 'workspaces in the largest monorepo' },
  { value: '104', label: 'boundary rules on one project' },
  { value: '5,467', label: 'runs on one project in 90 days' },
];

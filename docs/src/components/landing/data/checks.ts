/**
 * The 12 config checks, grouped by the problem they solve rather than by
 * check name. All three groups carry equal weight - they are different jobs,
 * and different visitors arrive needing different ones.
 */

export type Check = {
  /** Config key, shown in monospace. */
  key: string;
  title: string;
  description: string;
  /** True when `rev-dep config run --fix` can repair it. */
  autofix?: boolean;
  docs: string;
};

export type CheckGroup = {
  id: string;
  title: string;
  blurb: string;
  checks: Check[];
};

export const checkGroups: CheckGroup[] = [
  {
    id: 'dead-weight',
    title: 'Dead weight',
    blurb: 'Code and dependencies nothing reaches any more. Three of these can delete themselves.',
    checks: [
      {
        key: 'orphanFilesDetection',
        title: 'Orphan files',
        description: 'Files no entry point can reach. Removing one often orphans the next, so --fix peels them layer by layer.',
        autofix: true,
        docs: '/docs/config-based-checks/checks/orphan-files',
      },
      {
        key: 'unusedExportsDetection',
        title: 'Unused exports',
        description: 'Exported symbols nothing imports - the public surface a module never actually needed.',
        autofix: true,
        docs: '/docs/config-based-checks/checks/unused-exports',
      },
      {
        key: 'unusedNodeModulesDetection',
        title: 'Unused dependencies',
        description: 'Packages declared in package.json that no code imports. Smaller installs, smaller attack surface.',
        docs: '/docs/config-based-checks/checks/unused-node-modules',
      },
    ],
  },
  {
    id: 'structural-bugs',
    title: 'Structural bugs',
    blurb: 'Graph-level defects that a per-file linter or type-checker cannot see.',
    checks: [
      {
        key: 'circularImportsDetection',
        title: 'Circular imports',
        description: 'Import cycles that cause initialisation-order bugs and make modules impossible to test in isolation.',
        docs: '/docs/config-based-checks/checks/circular-imports',
      },
      {
        key: 'unresolvedImportsDetection',
        title: 'Unresolved imports',
        description: 'Import requests that resolve to nothing - broken paths, stale aliases, renamed files.',
        docs: '/docs/config-based-checks/checks/unresolved-imports',
      },
      {
        key: 'missingNodeModulesDetection',
        title: 'Missing dependencies',
        description: 'Packages the code imports but never declares. They work locally and break in CI.',
        docs: '/docs/config-based-checks/checks/missing-node-modules',
      },
      {
        key: 'devDepsUsageOnProdDetection',
        title: 'Dev deps in production',
        description: 'devDependencies reachable from a production entry point - shipped by accident.',
        docs: '/docs/config-based-checks/checks/dev-deps-on-prod',
      },
    ],
  },
  {
    id: 'architecture',
    title: 'Architecture rules',
    blurb: 'The layering you agreed on, written down and enforced. Most tools in this category have nothing here.',
    checks: [
      {
        key: 'moduleBoundaries',
        title: 'Module boundaries',
        description: 'Declare which parts of the codebase may import which. UI cannot reach the database layer; features stay independent.',
        docs: '/docs/config-based-checks/checks/module-boundaries',
      },
      {
        key: 'restrictedImportsDetection',
        title: 'Restricted imports',
        description: 'Forbid specific files or packages from being reachable at all from a given entry point.',
        docs: '/docs/config-based-checks/checks/restricted-imports',
      },
      {
        key: 'restrictedImportersDetection',
        title: 'Restricted importers',
        description: 'The allow-list form: only these entry points may transitively reach this code.',
        docs: '/docs/config-based-checks/checks/restricted-importers',
      },
      {
        key: 'restrictedDirectImportersDetection',
        title: 'Restricted direct importers',
        description: 'Constrain which files may import a module directly, without following the graph.',
        docs: '/docs/config-based-checks/checks/restricted-direct-importers',
      },
      {
        key: 'importConventions',
        title: 'Import conventions',
        description: 'Enforce relative-vs-alias import style per domain, and rewrite the offenders automatically.',
        autofix: true,
        docs: '/docs/config-based-checks/checks/import-conventions',
      },
    ],
  },
];

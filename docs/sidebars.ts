import type {SidebarsConfig} from '@docusaurus/plugin-content-docs';

const sidebars: SidebarsConfig = {
  tutorialSidebar: [
    'intro',
    {
      type: 'category',
      label: 'Quick Start',
      items: [
        'quick-start/overview',
        'quick-start/install',
        'quick-start/init-config',
        'quick-start/first-checks',
        'quick-start/monorepo-first-setup',
      ],
    },
    {
      type: 'category',
      label: 'Config-Based Checks',
      items: [
        'config-based-checks/overview',
        'config-based-checks/config-file-structure',
        'config-based-checks/entry-points',
        'config-based-checks/running-checks',
        'config-based-checks/fix-mode-and-autofix',
        'config-based-checks/output-formats',
        {
          type: 'category',
          label: 'Checks',
          items: [
            'config-based-checks/checks/module-boundaries',
            'config-based-checks/checks/import-conventions',
            'config-based-checks/checks/unused-exports',
            'config-based-checks/checks/orphan-files',
            'config-based-checks/checks/unused-node-modules',
            'config-based-checks/checks/missing-node-modules',
            'config-based-checks/checks/unresolved-imports',
            'config-based-checks/checks/circular-imports',
            'config-based-checks/checks/dev-deps-on-prod',
            'config-based-checks/checks/restricted-imports',
          ],
        },
        {
          type: 'category',
          label: 'Recipes',
          items: [
            'config-based-checks/recipes/single-package-app',
            'config-based-checks/recipes/monorepo-root',
            'config-based-checks/recipes/per-workspace-rules',
            'config-based-checks/recipes/ci-setup',
          ],
        },
      ],
    },
    {
      type: 'category',
      label: 'Exploratory Toolkit',
      items: [
        'exploratory-toolkit/overview',
        'exploratory-toolkit/entry-points',
        'exploratory-toolkit/files',
        'exploratory-toolkit/imported-by',
        'exploratory-toolkit/resolve',
        'exploratory-toolkit/circular',
        'exploratory-toolkit/node-modules',
        'exploratory-toolkit/lines-of-code',
        'exploratory-toolkit/workflows',
      ],
    },
    {
      type: 'category',
      label: 'Guides',
      items: [
        'guides/integration-guide',
        'guides/following-monorepo-packages',
        'guides/pnpm-workspaces',
        'guides/import-export-maps',
        'guides/tsconfig-paths-and-aliases',
        'guides/orphan-files-in-workspaces',
        'guides/missing-dependency-false-positives',
        'guides/unresolved-imports-troubleshooting',
        'guides/glob-patterns-and-gitignore',
        'guides/dynamic-imports-and-webpack-comments',
        'guides/supported-file-types',
        'guides/faq',
      ],
    },
    {
      type: 'category',
      label: 'CLI Reference',
      items: [
        // cli-reference-generated-start
        'cli-reference/overview',
        'cli-reference/generated/rev-dep_circular',
        {
          type: 'category',
          label: 'rev-dep config',
          items: [
            'cli-reference/generated/rev-dep_config',
            'cli-reference/generated/rev-dep_config_run',
            'cli-reference/generated/rev-dep_config_init',
          ],
        },
        'cli-reference/generated/rev-dep_entry-points',
        'cli-reference/generated/rev-dep_files',
        'cli-reference/generated/rev-dep_imported-by',
        'cli-reference/generated/rev-dep_lines-of-code',
        'cli-reference/generated/rev-dep_list-cwd-files',
        'cli-reference/generated/rev-dep_unresolved',
        {
          type: 'category',
          label: 'rev-dep node-modules',
          items: [
            'cli-reference/generated/rev-dep_node-modules',
            'cli-reference/generated/rev-dep_node-modules_dirs-size',
            'cli-reference/generated/rev-dep_node-modules_installed-duplicates',
            'cli-reference/generated/rev-dep_node-modules_installed',
            'cli-reference/generated/rev-dep_node-modules_missing',
            'cli-reference/generated/rev-dep_node-modules_unused',
            'cli-reference/generated/rev-dep_node-modules_used',
          ],
        },
        'cli-reference/generated/rev-dep_resolve',
        // cli-reference-generated-end
      ],
    },
    'glossary',
  ],
};

export default sidebars;

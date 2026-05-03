import type {SidebarsConfig} from '@docusaurus/plugin-content-docs';

const sidebars: SidebarsConfig = {
  tutorialSidebar: [
    'intro',
    'installation',
    // TODO full-docs
    // 'monorepo-integration-guide',
    // 'single-workspace-integration-guide',
    {
      type: 'category',
      label: 'Config-Based Checks',
      items: [
        'config-based-checks/overview',
        'config-based-checks/config-file-structure',
        'config-based-checks/entry-points-definition',
        {
          type: 'category',
          label: 'Checks',
          items: [
            'config-based-checks/checks/module-boundaries',
            'config-based-checks/checks/restricted-imports',
            'config-based-checks/checks/import-conventions',
            'config-based-checks/checks/circular-imports',
            'config-based-checks/checks/orphan-files',
            'config-based-checks/checks/unused-exports',
            'config-based-checks/checks/unused-node-modules',
            'config-based-checks/checks/missing-node-modules',
            'config-based-checks/checks/dev-deps-on-prod',
            'config-based-checks/checks/unresolved-imports',
          ],
        },
        'config-based-checks/running-checks-and-autofix',
        'config-based-checks/output-formats',
      ],
    },
    // TODO full-docs
    // {
    //   type: 'category',
    //   label: 'Exploratory Toolkit',
    //   items: [
    //     'exploratory-toolkit/overview',
    //     'exploratory-toolkit/entry-points',
    //     'exploratory-toolkit/files',
    //     'exploratory-toolkit/imported-by',
    //     'exploratory-toolkit/resolve',
    //     'exploratory-toolkit/circular',
    //     'exploratory-toolkit/node-modules',
    //     'exploratory-toolkit/lines-of-code',
    //     'exploratory-toolkit/workflows',
    //   ],
    // },
    // {
    //   type: 'category',
    //   label: 'Other Concepts and Features',
    //   items: [
    //     'other-concepts-and-features/following-monorepo-packages',
    //     'other-concepts-and-features/ignoring-files',
    //     'other-concepts-and-features/module-resolution-and-path-aliases',
    //     'other-concepts-and-features/supported-file-types',
    //     'other-concepts-and-features/svelte-support',
    //     'other-concepts-and-features/vue-support'
    //   ],
    // },
    // {
    //   type: 'category',
    //   label: 'Troubleshooting',
    //   items: [
    //     'troubleshooting/unresolved-imports-troubleshooting',
    //     'troubleshooting/missing-or-unused-dependency-false-positives',
    //     'troubleshooting/orphan-files-in-shared-workspace-packages',
    //     'troubleshooting/unused-exports-in-shared-workspace-packages'
    //   ],
    // },
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

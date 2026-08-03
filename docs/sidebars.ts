import type {SidebarsConfig} from '@docusaurus/plugin-content-docs';

const sidebars: SidebarsConfig = {
  tutorialSidebar: [
    // ── Get Started ──────────────────────────────────────────────
    {
      type: 'category',
      label: 'Get Started',
      collapsible: false,
      className: 'sidebar-group',
      customProps: {icon: 'overview'},
      items: [
        {type: 'doc', id: 'intro', label: 'Overview', customProps: {icon: 'overview'}},
        {type: 'doc', id: 'installation', customProps: {icon: 'install'}},
        {
          type: 'doc',
          id: 'monorepo-integration-guide',
          label: 'Monorepo integration guide',
          className: 'sidebar-item--highlight',
          customProps: {icon: 'monorepo'},
        },
        {
          type: 'doc',
          id: 'single-workspace-integration-guide',
          customProps: {icon: 'workspace'},
        },
      ],
    },

    // ── Core Features ────────────────────────────────────────────
    {
      type: 'category',
      label: 'Core Features',
      collapsible: false,
      className: 'sidebar-group',
      customProps: {icon: 'shield'},
      items: [
        {
          type: 'category',
          label: 'Config-Based Checks',
          collapsed: false,
          customProps: {icon: 'checks'},
          items: [
            'config-based-checks/overview',
            'config-based-checks/config-file-structure',
            'config-based-checks/entry-points-definition',
            {
              type: 'category',
              label: 'Checks',
              customProps: {icon: 'sliders'},
              items: [
                'config-based-checks/checks/module-boundaries',
                'config-based-checks/checks/restricted-imports',
                'config-based-checks/checks/restricted-importers',
                'config-based-checks/checks/restricted-direct-importers',
                'config-based-checks/checks/import-conventions',
                'config-based-checks/checks/circular-imports',
                'config-based-checks/checks/duplicated-code',
                'config-based-checks/checks/orphan-files',
                'config-based-checks/checks/unused-exports',
                'config-based-checks/checks/unused-node-modules',
                'config-based-checks/checks/missing-node-modules',
                'config-based-checks/checks/dev-deps-on-prod',
                'config-based-checks/checks/unresolved-imports',
              ],
            },
            'config-based-checks/running-checks-and-autofix',
            'config-based-checks/linting-the-config',
            'config-based-checks/output-formats',
            {
              type: 'category',
              label: 'Troubleshooting',
              link: {type: 'doc', id: 'troubleshooting/index'},
              customProps: {icon: 'troubleshooting'},
              items: [
                'troubleshooting/unresolved-imports-troubleshooting',
                'troubleshooting/missing-or-unused-dependency-false-positives',
                'troubleshooting/orphan-files-and-unused-exports-in-shared-packages',
              ],
            },
          ],
        },
        {
          type: 'category',
          label: 'Exploratory Toolkit',
          collapsed: false,
          customProps: {icon: 'explore'},
          items: [
            'exploratory-toolkit/overview',
            'exploratory-toolkit/entry-points',
            'exploratory-toolkit/files',
            'exploratory-toolkit/imported-by',
            'exploratory-toolkit/resolve',
            'exploratory-toolkit/circular',
            'exploratory-toolkit/duplicated-code',
            'exploratory-toolkit/node-modules',
            'exploratory-toolkit/lines-of-code',
            'exploratory-toolkit/debug',
            'exploratory-toolkit/workflows',
          ],
        },
        {
          type: 'category',
          label: 'Other Concepts & Features',
          collapsed: false,
          customProps: {icon: 'concepts'},
          items: [
            'other-concepts-and-features/duplicated-code-detection',
            'other-concepts-and-features/following-monorepo-packages',
            'other-concepts-and-features/node-modules-resolution',
            'other-concepts-and-features/ignoring-files',
            'other-concepts-and-features/glob-patterns',
            'other-concepts-and-features/module-resolution-and-path-aliases',
            'other-concepts-and-features/supported-file-types',
            'other-concepts-and-features/svelte-support',
            'other-concepts-and-features/vue-support',
          ],
        },
      ],
    },

    // ── Migration & Comparison ───────────────────────────────────
    {
      type: 'category',
      label: 'Migration & Comparison',
      collapsible: false,
      className: 'sidebar-group',
      customProps: {icon: 'migrate'},
      items: [
        {
          type: 'category',
          label: 'Migrating from other tools',
          customProps: {icon: 'migrate'},
          items: [
            'migrating-from-other-tools/overview',
            'migrating-from-other-tools/migrating-from-knip',
            'migrating-from-other-tools/migrating-from-dependency-cruiser',
            'migrating-from-other-tools/migrating-from-depcheck',
            'migrating-from-other-tools/migrating-from-madge',
            'migrating-from-other-tools/migrating-from-dpdm',
            'migrating-from-other-tools/migrating-from-ts-prune',
            'migrating-from-other-tools/migrating-from-ts-unused-exports',
            'migrating-from-other-tools/migrating-from-unimported',
            'migrating-from-other-tools/migrating-from-skott',
            'migrating-from-other-tools/migrating-from-eslint-plugin-import',
            'migrating-from-other-tools/migrating-from-good-fences',
            'migrating-from-other-tools/migrating-from-sheriff',
            'migrating-from-other-tools/migrating-from-npm-check',
          ],
        },
        {
          type: 'category',
          label: 'Comparison with other tools',
          customProps: {icon: 'compare'},
          items: [
            'comparison-with-other-tools/overview',
            'comparison-with-other-tools/knip-vs-rev-dep',
            'comparison-with-other-tools/dependency-cruiser-vs-rev-dep',
            'comparison-with-other-tools/depcheck-vs-rev-dep',
            'comparison-with-other-tools/madge-vs-rev-dep',
            'comparison-with-other-tools/jscpd-vs-rev-dep',
            'comparison-with-other-tools/dpdm-vs-rev-dep',
            'comparison-with-other-tools/ts-prune-vs-rev-dep',
            'comparison-with-other-tools/ts-unused-exports-vs-rev-dep',
            'comparison-with-other-tools/unimported-vs-rev-dep',
            'comparison-with-other-tools/skott-vs-rev-dep',
            'comparison-with-other-tools/eslint-plugin-import-vs-rev-dep',
            'comparison-with-other-tools/good-fences-vs-rev-dep',
            'comparison-with-other-tools/sheriff-vs-rev-dep',
            'comparison-with-other-tools/npm-check-vs-rev-dep',
          ],
        },
      ],
    },

    // ── Reference ────────────────────────────────────────────────
    {
      type: 'category',
      label: 'Reference',
      collapsible: false,
      className: 'sidebar-group',
      customProps: {icon: 'gear'},
      items: [
        {
          type: 'category',
          label: 'CLI Reference',
          customProps: {icon: 'cli'},
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
            'cli-reference/generated/rev-dep_config_lint',
            'cli-reference/generated/rev-dep_config_migrate',
          ],
        },
        {
          type: 'category',
          label: 'rev-dep debug',
          items: [
            'cli-reference/generated/rev-dep_debug',
            'cli-reference/generated/rev-dep_debug_get-tree-for-cwd',
            'cli-reference/generated/rev-dep_debug_list-cwd-files',
            'cli-reference/generated/rev-dep_debug_parse-file',
            'cli-reference/generated/rev-dep_debug_parse-tsconfig',
          ],
        },
        'cli-reference/generated/rev-dep_duplicated-code',
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
            'cli-reference/generated/rev-dep_node-modules_analyze-size',
            'cli-reference/generated/rev-dep_node-modules_dirs-size',
            'cli-reference/generated/rev-dep_node-modules_installed-duplicates',
            'cli-reference/generated/rev-dep_node-modules_installed',
            'cli-reference/generated/rev-dep_node-modules_missing',
            'cli-reference/generated/rev-dep_node-modules_prune-docs',
            'cli-reference/generated/rev-dep_node-modules_unused',
            'cli-reference/generated/rev-dep_node-modules_used',
          ],
        },
        'cli-reference/generated/rev-dep_resolve',
        // cli-reference-generated-end
          ],
        },
        {
          type: 'category',
          label: 'Upgrade Guides',
          customProps: {icon: 'upgrade'},
          items: ['upgrade-guides/v3-breaking-changes'],
        },
        {type: 'doc', id: 'glossary', customProps: {icon: 'glossary'}},
        {type: 'doc', id: 'telemetry', customProps: {icon: 'telemetry'}},
      ],
    },

  ],
};

export default sidebars;

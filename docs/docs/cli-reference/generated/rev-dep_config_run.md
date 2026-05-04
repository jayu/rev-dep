---
title: "rev-dep config run"
description: "Execute all checks defined in (.)rev-dep.config.json(c)"
---

Execute all checks defined in (.)rev-dep.config.json(c)

### Synopsis

Process (.)rev-dep.config.json(c) and execute all enabled checks (circular imports, orphan files, module boundaries, import conventions, node modules, unused exports, unresolved imports, restricted imports and restricted dev deps usage) per rule.

```
rev-dep config run [flags]
```

### Options

```
      --condition-names strings                                     List of conditions for package.json imports resolution (e.g. node, imports, default)
  -c, --cwd string                                                  Working directory (default "$PWD")
      --fix                                                         Automatically fix fixable issues
      --follow-monorepo-packages strings                            Enable resolution of imports from monorepo workspace packages. Pass without value to follow all, or pass package names
      --format string                                               Output format (json, issues-list)
  -h, --help                                                        help for run
      --list-all-issues                                             List all issues instead of limiting output
      --package-json string                                         Path to package.json (default: ./package.json)
      --recheck                                                     Run all checks again after '--fix' to validate the final state
      --rules strings                                               Subset of rules to run (comma-separated list of rule paths)
      --tsconfig-json string                                        Path to tsconfig.json (default: ./tsconfig.json)
  -v, --verbose                                                     Show warnings and verbose output
```

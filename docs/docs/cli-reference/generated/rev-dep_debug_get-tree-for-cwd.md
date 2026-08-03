---
title: "rev-dep debug get-tree-for-cwd"
description: "Debug: Show complete dependency tree for analysis"
---

Debug: Show complete dependency tree for analysis

### Synopsis

Debugging tool to inspect the complete dependency tree. Output does not follow semver.

```
rev-dep debug get-tree-for-cwd [flags]
```

### Options

```
      --condition-names strings                                     List of conditions for package.json imports resolution (e.g. node, imports, default)
      --cwd string                                                  Working directory for the command (default "$PWD")
      --follow-monorepo-packages strings                            Enable resolution of imports from monorepo workspace packages. Pass without value to follow all, or pass package names
  -h, --help                                                        help for get-tree-for-cwd
  -t, --ignore-type-imports                                         Exclude type imports from the analysis
      --include-dev-deps-from-root                                  Treat the monorepo root package.json devDependencies as available to package code, so they are not reported as missing or unresolved. Mirrors config nodeModulesResolution.includeDevDepsFromRoot
      --node-modules-resolution string                              Which package.json each import is validated against: 'entry-package' (the cwd package.json, default) or 'nearest-package' (each file's own nearest package.json) (default "entry-package")
      --tsconfig-json string                                        Path to tsconfig.json (default: ./tsconfig.json)
  -v, --verbose                                                     Show warnings and verbose output
```

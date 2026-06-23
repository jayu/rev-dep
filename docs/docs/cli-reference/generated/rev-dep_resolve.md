---
title: "rev-dep resolve"
description: "Trace and display the dependency path between files in your project"
---

Trace and display the dependency path between files in your project

### Synopsis

Analyze and display the dependency chain between specified files.
Helps understand how different parts of your codebase are connected.

```
rev-dep resolve [flags]
```

### Examples

```
rev-dep resolve -p src/index.ts -f src/utils/helpers.ts
```

### Options

```
  -a, --all                                                         Show all possible resolution paths, not just the first one
      --compact-summary                                             Display a compact summary of found paths
      --condition-names strings                                     List of conditions for package.json imports resolution (e.g. node, imports, default)
  -c, --cwd string                                                  Working directory for the command (default "$PWD")
  -p, --entry-points strings                                        Entry point file(s) or glob pattern(s) to start analysis from (default: auto-detected)
  -f, --file string                                                 Target file to check for dependencies
      --follow-monorepo-packages strings                            Enable resolution of imports from monorepo workspace packages. Pass without value to follow all, or pass package names
      --graph-exclude strings                                       Glob patterns to exclude files from dependency analysis
  -h, --help                                                        help for resolve
  -t, --ignore-type-imports                                         Exclude type imports from the analysis
      --include-dev-deps-from-root                                  Treat the monorepo root package.json devDependencies as available to package code, so they are not reported as missing or unresolved. Mirrors config nodeModulesResolution.includeDevDepsFromRoot
      --module string                                               Target node module name to check for dependencies
      --node-modules-resolution string                              Which package.json each import is validated against: 'entry-package' (the cwd package.json, default) or 'nearest-package' (each file's own nearest package.json) (default "entry-package")
      --package-json string                                         Path to package.json (default: ./package.json)
      --process-ignored-files strings                               Glob patterns to process even if they are ignored by gitignore or exclude patterns
      --tsconfig-json string                                        Path to tsconfig.json (default: ./tsconfig.json)
  -v, --verbose                                                     Show warnings and verbose output
```

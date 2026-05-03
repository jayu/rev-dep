---
title: rev-dep resolve
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
      --module string                                               Target node module name to check for dependencies
      --package-json string                                         Path to package.json (default: ./package.json)
      --process-ignored-files strings                               Glob patterns to process even if they are ignored by gitignore or exclude patterns
      --tsconfig-json string                                        Path to tsconfig.json (default: ./tsconfig.json)
  -v, --verbose                                                     Show warnings and verbose output
```

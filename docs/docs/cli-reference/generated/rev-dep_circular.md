---
title: rev-dep circular
---

Detect circular dependencies in your project

### Synopsis

Analyzes the project to find circular dependencies between modules.
Circular dependencies can cause hard-to-debug issues and should generally be avoided.

```
rev-dep circular [flags]
```

### Examples

```
rev-dep circular --ignore-types-imports
```

### Options

```
      --algorithm string                                            Cycle detection algorithm: DFS (default) or SCC (default "DFS")
      --condition-names strings                                     List of conditions for package.json imports resolution (e.g. node, imports, default)
  -c, --cwd string                                                  Working directory for the command (default "$PWD")
      --follow-monorepo-packages strings                            Enable resolution of imports from monorepo workspace packages. Pass without value to follow all, or pass package names
  -h, --help                                                        help for circular
  -t, --ignore-type-imports                                         Exclude type imports from the analysis
      --package-json string                                         Path to package.json (default: ./package.json)
      --process-ignored-files strings                               Glob patterns to process even if they are ignored by gitignore or exclude patterns
      --tsconfig-json string                                        Path to tsconfig.json (default: ./tsconfig.json)
  -v, --verbose                                                     Show warnings and verbose output
```

---
title: rev-dep entry-points
---

Discover and list all entry points in the project

### Synopsis

Analyzes the project structure to identify all potential entry points.
Useful for understanding your application's architecture and dependencies.

```
rev-dep entry-points [flags]
```

### Examples

```
rev-dep entry-points --print-deps-count
```

### Options

```
      --condition-names strings                                     List of conditions for package.json imports resolution (e.g. node, imports, default)
  -n, --count                                                       Only display the number of entry points found
  -c, --cwd string                                                  Working directory for the command (default "$PWD")
      --follow-monorepo-packages strings                            Enable resolution of imports from monorepo workspace packages. Pass without value to follow all, or pass package names
      --graph-exclude strings                                       Exclude files matching these glob patterns from analysis
  -h, --help                                                        help for entry-points
  -t, --ignore-type-imports                                         Exclude type imports from the analysis
      --package-json string                                         Path to package.json (default: ./package.json)
      --print-deps-count                                            Show the number of dependencies for each entry point
      --process-ignored-files strings                               Glob patterns to process even if they are ignored by gitignore or exclude patterns
      --result-exclude strings                                      Exclude files matching these glob patterns from results
      --result-include strings                                      Only include files matching these glob patterns in results
      --tsconfig-json string                                        Path to tsconfig.json (default: ./tsconfig.json)
  -v, --verbose                                                     Show warnings and verbose output
```

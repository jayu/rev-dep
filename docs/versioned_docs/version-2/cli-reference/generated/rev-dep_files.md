---
title: "rev-dep files"
description: "List all files in the dependency tree of an entry point"
---

List all files in the dependency tree of an entry point

### Synopsis

Recursively finds and lists all files that are required
by the specified entry point.

```
rev-dep files [flags]
```

### Examples

```
rev-dep files --entry-point src/index.ts
```

### Options

```
      --condition-names strings                                     List of conditions for package.json imports resolution (e.g. node, imports, default)
  -n, --count                                                       Only display the count of files in the dependency tree
  -c, --cwd string                                                  Working directory for the command (default "$PWD")
  -p, --entry-point string                                          Entry point file to analyze (required)
      --follow-monorepo-packages strings                            Enable resolution of imports from monorepo workspace packages. Pass without value to follow all, or pass package names
  -h, --help                                                        help for files
  -t, --ignore-type-imports                                         Exclude type imports from the analysis
      --package-json string                                         Path to package.json (default: ./package.json)
      --process-ignored-files strings                               Glob patterns to process even if they are ignored by gitignore or exclude patterns
      --tsconfig-json string                                        Path to tsconfig.json (default: ./tsconfig.json)
  -v, --verbose                                                     Show warnings and verbose output
```

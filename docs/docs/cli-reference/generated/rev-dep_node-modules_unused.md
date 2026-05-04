---
title: "rev-dep node-modules unused"
description: "Find installed packages that aren't imported in your code"
---

Find installed packages that aren't imported in your code

### Synopsis

Compares package.json dependencies with actual imports in your codebase
to identify potentially unused packages.

```
rev-dep node-modules unused [flags]
```

### Examples

```
rev-dep node-modules unused --exclude-modules=@types/*
```

### Options

```
      --condition-names strings                                     List of conditions for package.json imports resolution (e.g. node, imports, default)
  -n, --count                                                       Only display the count of modules
  -c, --cwd string                                                  Working directory for the command (default "$PWD")
  -p, --entry-points strings                                        Entry point file(s) to start analysis from (default: auto-detected)
  -e, --exclude-modules strings                                     list of modules to exclude from the output
  -b, --files-with-binaries strings                                 Additional files to search for binary usages. Use paths relative to cwd
  -m, --files-with-node-modules strings                             Additional files to search for module imports. Use paths relative to cwd
      --follow-monorepo-packages strings                            Enable resolution of imports from monorepo workspace packages. Pass without value to follow all, or pass package names
  -h, --help                                                        help for unused
  -t, --ignore-type-imports                                         Exclude type imports from the analysis
  -i, --include-modules strings                                     list of modules to include in the output
      --package-json string                                         Path to package.json (default: ./package.json)
      --pkg-fields-with-binaries strings                            Additional package.json fields to check for binary usages
      --tsconfig-json string                                        Path to tsconfig.json (default: ./tsconfig.json)
  -v, --verbose                                                     Show warnings and verbose output
      --zero-exit-code                                              Use this flag to always return zero exit code
```

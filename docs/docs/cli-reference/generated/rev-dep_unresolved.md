---
title: "rev-dep unresolved"
description: "List unresolved imports in the project"
---

List unresolved imports in the project

### Synopsis

Detect and list imports that could not be resolved during imports resolution. Groups imports by file.

```
rev-dep unresolved [flags]
```

### Options

```
      --condition-names strings                                     List of conditions for package.json imports resolution (e.g. node, imports, default)
      --custom-asset-extensions strings                             Additional asset extensions treated as resolvable (e.g. glb,mp3)
  -c, --cwd string                                                  Working directory for the command (default "$PWD")
      --follow-monorepo-packages strings                            Enable resolution of imports from monorepo workspace packages. Pass without value to follow all, or pass package names
  -h, --help                                                        help for unresolved
      --ignore stringToString                                       Map of file path (relative to cwd) to exact import request to ignore (e.g. --ignore src/index.ts=some-module) (default [])
      --ignore-files strings                                        File path glob patterns to ignore in unresolved output
      --ignore-imports strings                                      Import requests to ignore globally in unresolved output
      --include-dev-deps-from-root                                  Treat the monorepo root package.json devDependencies as available to package code, so they are not reported as missing or unresolved. Mirrors config nodeModulesResolution.includeDevDepsFromRoot
      --node-modules-resolution string                              Which package.json each import is validated against: 'entry-package' (the cwd package.json, default) or 'nearest-package' (each file's own nearest package.json) (default "entry-package")
      --package-json string                                         Path to package.json (default: ./package.json)
      --process-ignored-files strings                               Glob patterns to process even if they are ignored by gitignore or exclude patterns
      --tsconfig-json string                                        Path to tsconfig.json (default: ./tsconfig.json)
  -v, --verbose                                                     Show warnings and verbose output
```

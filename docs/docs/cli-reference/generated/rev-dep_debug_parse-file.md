---
title: "rev-dep debug parse-file"
description: "Debug: Show parsed imports for a single file"
---

Debug: Show parsed imports for a single file

### Synopsis

Debugging tool to inspect how the parser processes a specific file. Output does not follow semver.

```
rev-dep debug parse-file [flags]
```

### Options

```
      --condition-names strings                                     List of conditions for package.json imports resolution (e.g. node, imports, default)
      --cwd string                                                  Working directory for the command (default "$PWD")
      --file string                                                 file to parse
      --follow-monorepo-packages strings                            Enable resolution of imports from monorepo workspace packages. Pass without value to follow all, or pass package names
  -h, --help                                                        help for parse-file
      --include-dev-deps-from-root                                  Treat the monorepo root package.json devDependencies as available to package code, so they are not reported as missing or unresolved. Mirrors config nodeModulesResolution.includeDevDepsFromRoot
      --node-modules-resolution string                              Which package.json each import is validated against: 'entry-package' (the cwd package.json, default) or 'nearest-package' (each file's own nearest package.json) (default "entry-package")
      --tsconfig-json string                                        Path to tsconfig.json (default: ./tsconfig.json)
  -v, --verbose                                                     Show warnings and verbose output
```

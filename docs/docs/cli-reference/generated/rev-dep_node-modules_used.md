---
title: "rev-dep node-modules used"
description: "List all npm packages imported in your code"
---

List all npm packages imported in your code

### Synopsis

Analyzes your code to identify which npm packages are actually being used.
Helps keep track of your project's runtime dependencies.

```
rev-dep node-modules used [flags]
```

### Examples

```
rev-dep node-modules used -p src/index.ts --group-by-module
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
      --group-by-entry-point                                        Organize output by entry point file path
      --group-by-entry-point-modules-count                          Organize output by entry point and show count of unique modules
      --group-by-file                                               Organize output by project file path
      --group-by-module                                             Organize output by npm package name
      --group-by-module-entry-points-count                          Organize output by npm package name and show count of entry points using it
      --group-by-module-files-count                                 Organize output by npm package name and show count of files using it
      --group-by-module-show-entry-points                           Organize output by npm package name and list entry points using it
  -h, --help                                                        help for used
  -t, --ignore-type-imports                                         Exclude type imports from the analysis
      --include-dev-deps-from-root                                  Treat the monorepo root package.json devDependencies as available to package code, so they are not reported as missing or unresolved. Mirrors config nodeModulesResolution.includeDevDepsFromRoot
  -i, --include-modules strings                                     list of modules to include in the output
      --node-modules-resolution string                              Which package.json each import is validated against: 'entry-package' (the cwd package.json, default) or 'nearest-package' (each file's own nearest package.json) (default "entry-package")
      --pkg-fields-with-binaries strings                            Additional package.json fields to check for binary usages
      --tsconfig-json string                                        Path to tsconfig.json (default: ./tsconfig.json)
  -v, --verbose                                                     Show warnings and verbose output
```

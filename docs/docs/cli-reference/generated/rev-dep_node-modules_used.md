---
title: rev-dep node-modules used
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
  -i, --include-modules strings                                     list of modules to include in the output
      --package-json string                                         Path to package.json (default: ./package.json)
      --pkg-fields-with-binaries strings                            Additional package.json fields to check for binary usages
      --tsconfig-json string                                        Path to tsconfig.json (default: ./tsconfig.json)
  -v, --verbose                                                     Show warnings and verbose output
```

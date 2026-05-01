---
title: rev-dep imported-by
---

List all files that directly import the specified file

### Synopsis

Finds and lists all files in the project that directly import the specified file.
This is useful for understanding the impact of changes to a particular file.

```
rev-dep imported-by [flags]
```

### Examples

```
rev-dep imported-by --file src/utils/helpers.ts
```

### Options

```
      --condition-names strings                                     List of conditions for package.json imports resolution (e.g. node, imports, default)
  -n, --count                                                       Only display the count of importing files
  -c, --cwd string                                                  Working directory for the command (default "$PWD")
  -f, --file string                                                 Target file to find importers for (required)
      --follow-monorepo-packages strings[=__REV_DEP_FOLLOW_ALL__]   Enable resolution of imports from monorepo workspace packages. Pass without value to follow all, or pass package names
  -h, --help                                                        help for imported-by
      --list-imports                                                List the import identifiers used by each file
      --package-json string                                         Path to package.json (default: ./package.json)
      --process-ignored-files strings                               Glob patterns to process even if they are ignored by gitignore or exclude patterns
      --tsconfig-json string                                        Path to tsconfig.json (default: ./tsconfig.json)
  -v, --verbose                                                     Show warnings and verbose output
```

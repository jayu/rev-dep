<p align="center">
<img src="./logo.png" width="400">
</p>

<p align="center">
  Dependency analysis and optimization toolkit for modern TypeScript projects.
  <br>
  <a href="#reimplemented-to-achieve-7x-37x-speedup">Completely rewritten in Go for maximum speed and efficiency ⚡</a>
</p>

---

<img alt="rev-dep version" src="https://img.shields.io/npm/v/rev-dep"> <img alt="rev-dep license" src="https://img.shields.io/npm/l/rev-dep"> <img alt="rev-dep PRs welcome" src="https://img.shields.io/badge/PRs-welcome-brightgreen.svg?style=flat-square">


## About 📣

The tool was created to help with daily development struggles by answering these questions:

👉 What entry points does my codebase have?

👉 Which entry points use a given file?

👉 Which dependencies does a given file have?

👉 Do any circular dependencies exist in the project?

👉 Which node modules are unused or not listed in package.json?

👉 Which node modules take up the most space on disk?


This helps to debug project dependencies, plan refactoring, optimize bundles or plan code splitting.

It's especially useful in JS world without TypeScript or tests coverage.

It also helps to identify and eliminate dead files, understand the complexity of the file dependencies

[🦘 Jump to CLI reference](#cli-reference-)

### Use cases 🧑‍💻

- [You plan to refactor some file and you wonder which entry points are affected](#how-to-identify-where-a-file-is-used-in-the-project)
- [You are wondering wether a given source file is used](#how-to-check-if-a-file-is-used-in-the-project)
- [You wonder if there are any dead files in your project](#how-to-identify-dead-files-in-the-project)
- [You want to verify if a given entry point imports only the required files](#how-to-check-which-files-are-imported-by-a-given-file)
- [You want to optimize the amount of files imported by an entry point](#how-to-reduce-amount-of-files-imported-by-entry-point)
- [You want to detect circular dependencies in your project](#how-to-detect-circular-dependencies-in-the-project)
- [You want to find unused node modules to clean up dependencies](#how-to-find-unused-node-modules)
- [You want to identify which node modules are consuming the most disk space](#how-to-identify-space-consuming-node-modules)

### How about dependency or bundle graphs?

There are tool that can output nice, visual representation of project dependencies like [webpack-bundle-analyzer](https://www.npmjs.com/package/webpack-bundle-analyzer) or [dependency-cruiser](https://www.npmjs.com/package/dependency-cruiser) (_which btw rev-dep uses for non-TS codebases_)

While graphs can be useful to identify major problems like too big bundle size or to visualize mess in your deps, it's hard to take any action based on them (_at least it was hard for me_ 🤷‍♂️)

`rev-dep` visualize dependencies as lists, so it's really easy to see where to cut the line to solve the problem.

## Getting Started 🎉

### Install globally to use as CLI tool

`yarn global add rev-dep`

`npm -g install rev-dep`

`pnpm global add rev-dep`

## Recipes 🌶️

### How to identify where a file is used in the project?

Just use `rev-dep resolve --file path/to/file.ts`

You will see all the entry points that implicitly require given file together with resolution path.

[`resolve` Command CLI reference](#rev-dep-resolve)

#### Getting more details about file resolution in given entry point

To find out all paths combination use `rev-dep resolve` with `-a` flag

> You might be surprised how complex dependency tree can be!

</details>

### How to check if a file is used in the project?

Use `rev-dep resolve --file path/to/file.ts --compact-summary`

As a result you will see total amount of entry points requiring a given file.

> Note that among the entry points list there might be some dead files importing the searched file

[`resolve` Command CLI reference](#rev-dep-resolve)

### How to identify dead files in the project?

Use `rev-dep entry-points` to get list of all files that are not required by any other files in the project.

You might want to exclude some file paths that are meant to be actual entry point like `index.js` or `**/pages/**` in `next.js` projects using `--result-exclude` flag. The same for configuration files like `babel.config.js`

Review the list and look for suspicious files like `src/ui/components/SomeComponent/index.js`

[`entry-points` command CLI reference](#rev-dep-entry-points)

### How to check which files are imported by a given file?

To get a full list of files imported by given entry point use `rev-dep files --entry-point path/to/file.ts`.

You can use `--count` flag if you are interested in the amount.

This is a good indicator of how heavy a given entry point or component is

[`files` command CLI reference](#rev-dep-files)

### How to reduce amount of files imported by entry point?

There is no easy how to for this process, but you can do it iteratively using `rev-dep` commands `files` and `resolve`

1. Get the list of files imported by entry-point

   `rev-dep files --entry-point path/to/entry-point`

2. Identify some suspicious files on the list, components that should not be used on the given page or not related utility files
3. Get all resolution paths for a suspicious file

   `rev-dep resolve --file path/to/suspicious-file --entry-points path/to/entry-point --all`

4. You would usually find out that there is some file, like directory `index` file that given entry point is using, which is mandatory, but as a side effect it imports a few files that are redundant for your entry point. In most cases you should be able to decouple the imports or reverse the dependency to cut off the resolution path for the unwanted file

### How to detect circular dependencies in the project?

Use `rev-dep circular` to find all circular dependencies between modules in your project.

Circular dependencies can cause runtime errors, memory leaks, and make code difficult to understand and maintain. It's important to identify and resolve them early.

You can use `--ignore-type-imports` flag to exclude type-only imports from the analysis if they're not relevant to your use case.

[`circular` command CLI reference](#rev-dep-circular)

### How to find unused node modules?

Use `rev-dep node-modules unused` to identify packages that are installed but not actually imported in your code.

This helps clean up your `package.json` and reduce the size of your `node_modules` directory. You might want to exclude type definitions using `--exclude-modules=@types/*` since they're often used indirectly.

[`node-modules unused` command CLI reference](#rev-dep-node-modules-unused)

### How to identify space-consuming node modules?

Use `rev-dep node-modules dirs-size` to calculate and display the size of `node_modules` directories.

This helps identify which packages are taking up the most disk space, allowing you to make informed decisions about dependency management or look for lighter alternatives.

For detailed analysis of specific modules, use `rev-dep node-modules analyze-size` with the package names you want to investigate.

[`node-modules dirs-size` command CLI reference](#rev-dep-node-modules-dirs-size)

## Reimplemented to achieve 7x-37x speedup

Rev-dep@2.0.0 was reimplemented in Go from scratch to leverage it's concurrency features and better memory management of compiled languages.

As a result v2 is up to 37x faster than v1 and consumes up to 13x less memory.

### Performance comparison

To compare performance rev-dep was benchmarked with hyperfine using 8 runs per test, taking mean time values as a result.
Benchmark was run on TypeScript codebase with 507658 lines of code and 5977 source code files.

Memory usage on Mac was measure using `/usr/bin/time` utility. Memory usage on Linux was not measured because I could't find reliable way to measure RAM usage on Linux. Subsequent runs had too much fluctuation.

### Mac book Pro M1 256GB, power save off; 

| Command                                                      | V1 Time | V2 Time | Time Change | V1 RAM     | V2 RAM    | RAM Change |
| ------------------------------------------------------------ | ------- | ------- | ----------- | ---------- | --------- | ---------- |
| List entry-points `rev-dep entry-points`                     | 6500ms  | 347ms   | 19x         | ~680MB RAM | ~51MB RAM | 13x        |
| List entry-points with dependent files count `-pdc`          | 8333ms  | 782ms   | 11x         | ~885MB RAM | ~110MB RAM| 8x         |
| List entry-point files `rev-dep files`                       | 2729ms  | 400ms   | 7x          | ~330MB RAM | ~36MB RAM | 9x         |
| Resolve dependency path `rev-dep resolve`                    | 2984ms  | 359ms   | 8x          | ~330MB RAM | ~35MB RAM | 9x         |

### WSL Linux Debian Intel(R) Core(TM) i9-14900KF CPU @ 2.80GHz

| Command                                                                 | V1 Time | V2 Time | Time Change |
| ----------------------------------------------------------------------- | ------- | ------- | ----------- |
| List entry-points `rev-dep entry-points`                                | 9904ms  | 270ms   | 37x         |
| List entry-points with dependent files count `--print-deps-count`       | 10562ms | 458ms   | 23x         |
| List entry-point files `rev-dep files`                                  | 3097ms  | 230ms   | 13x         |
| Resolve dependency path `rev-dep resolve`                               | 3146ms  | 230ms   | 14x         |

### New features

V2 comes with bunch of new commands

- `circular` - detects circular dependencies in the project
- `lines-of-code` - counts actual lines of code in the project excluding comments and blank lines
- `list-cwd-files` - lists all files in the current working directory
- `node-modules used` - lists all used node modules
- `node-modules unused` - lists all unused node modules
- `node-modules missing` - lists all missing node modules
- `node-modules installed` - lists all installed node modules
- `node-modules installed-duplicates` - lists all installed node modules that exist in file system with the same version multiple times
- `node-modules analyze-size` - analyzes size of specific node modules and helps to identify space-hogging dependencies
- `node-modules dirs-size` - calculates cumulative files size in node_modules directories

### ⚠️ What's not supported

Comparing to previous versions, these tsconfig features are not supported

#### Config extends

If you tsconfig uses extends, and you have some paths defined in config being extended, paths from extended config won't be used during resolution

eg. 

```json
// tsconfig.json
{
  "extends": "./tsconfig.base.json",
  "paths": {
    "@/components": ["src/components/*"]
  }
}
```

```json
// tsconfig.base.json
{
  "paths": {
    "@/utils": ["src/utils/*"]
  }
}
```

In that scenario imports with `@/utils` won't be resolved.

Why it's not supported ? I consider this typescript capability as an usage edge-case. I forgot to to implement it at the begging and I don't feel like investing more time now, before this package get some reasonable adoption and people will be actually requesting this feature.

#### Multiple path aliases

Only first path will be used in resolution.

```json
// tsconfig.json
{
  "paths": {
    "@/components": ["src/components/*", "src/components2/*"]
  }
}
```

Imports that should resolve to `src/components2/*` will be considered unresolved.

Why it's not supported? I consider this typescript capability as an anti-pattern. It introduces unnecessary ambiguity in module resolution.
Implementing this would make code more complex, less maintainable and slower.

#### Using rev-dep as node module

Importing rev-dep in JS/TS is no longer supported. Preferred way is to run rev-dep using child process. 

#### Other discrepancies

Any other discrepancies between TypeScript module resolution and rev-dep should be considered as a bug.

### Supported Platforms

- Linux x64
- MacOS Apple Silicon

There are the platforms I use and know. For these I build and tested binaries. 
Go allows for cross-compiling, so I'm happy to build and distribute binaries for other platforms as well, but I haven't tested it. 
Feel free to open an issue if you need support for another platform.

## CLI reference 📖

<!-- cli-docs-start -->

### rev-dep circular

Detect circular dependencies in your project

#### Synopsis

Analyzes the project to find circular dependencies between modules.
Circular dependencies can cause hard-to-debug issues and should generally be avoided.

```
rev-dep circular [flags]
```

#### Examples

```
rev-dep circular --ignore-types-imports
```

#### Options

```
  -c, --cwd string             Working directory for the command (default "$PWD")
  -h, --help                   help for circular
  -t, --ignore-type-imports    Exclude type imports from the analysis
      --package-json string    Path to package.json (default: ./package.json) (default "package.json")
      --tsconfig-json string   Path to tsconfig.json (default: ./tsconfig.json) (default "tsconfig.json")
```


### rev-dep entry-points

Discover and list all entry points in the project

#### Synopsis

Analyzes the project structure to identify all potential entry points.
Useful for understanding your application's architecture and dependencies.

```
rev-dep entry-points [flags]
```

#### Examples

```
rev-dep entry-points --print-deps-count
```

#### Options

```
  -n, --count                    Only display the number of entry points found
  -c, --cwd string               Working directory for the command (default "$PWD")
      --graph-exclude strings    Exclude files matching these glob patterns from analysis
  -h, --help                     help for entry-points
  -t, --ignore-type-imports      Exclude type imports from the analysis
      --package-json string      Path to package.json (default: ./package.json) (default "package.json")
      --print-deps-count         Show the number of dependencies for each entry point
      --result-exclude strings   Exclude files matching these glob patterns from results
      --result-include strings   Only include files matching these glob patterns in results
      --tsconfig-json string     Path to tsconfig.json (default: ./tsconfig.json) (default "tsconfig.json")
```


### rev-dep files

List all files in the dependency tree of an entry point

#### Synopsis

Recursively finds and lists all files that are required
by the specified entry point.

```
rev-dep files [flags]
```

#### Examples

```
rev-dep files --entry-point src/index.ts
```

#### Options

```
  -n, --count                  Only display the count of files in the dependency tree
  -c, --cwd string             Working directory for the command (default "$PWD")
  -p, --entry-point string     Entry point file to analyze (required)
  -h, --help                   help for files
  -t, --ignore-type-imports    Exclude type imports from the analysis
      --package-json string    Path to package.json (default: ./package.json) (default "package.json")
      --tsconfig-json string   Path to tsconfig.json (default: ./tsconfig.json) (default "tsconfig.json")
```


### rev-dep lines-of-code

Count actual lines of code in the project excluding comments and blank lines

```
rev-dep lines-of-code [flags]
```

#### Examples

```
rev-dep lines-of-code
```

#### Options

```
  -c, --cwd string   Directory to analyze (default "$PWD")
  -h, --help         help for lines-of-code
```


### rev-dep list-cwd-files

List all files in the current working directory

#### Synopsis

Recursively lists all files in the specified directory,
with options to filter results.

```
rev-dep list-cwd-files [flags]
```

#### Examples

```
rev-dep list-cwd-files --include='*.ts' --exclude='*.test.ts'
```

#### Options

```
      --count             Only display the count of matching files
      --cwd string        Directory to list files from (default "$PWD")
      --exclude strings   Exclude files matching these glob patterns
  -h, --help              help for list-cwd-files
      --include strings   Only include files matching these glob patterns
```


### rev-dep node-modules

Analyze and manage Node.js dependencies

#### Synopsis

Tools for analyzing and managing Node.js module dependencies.
Helps identify unused, missing, or duplicate dependencies in your project.

#### Examples

```
  rev-dep node-modules used -p src/index.ts
  rev-dep node-modules unused --exclude-modules=@types/*
  rev-dep node-modules missing --entry-points=src/main.ts
```

#### Options

```
  -h, --help   help for node-modules
```


### rev-dep node-modules dirs-size

Calculates cumulative files size in node_modules directories

#### Synopsis

Calculates and displays the size of node_modules folders
in the current directory and subdirectories. Sizes will be smaller than actual file size taken on disk. Tool is calculating actual file size rather than file size on disk (related to disk blocks usage)

```
rev-dep node-modules dirs-size [flags]
```

#### Examples

```
rev-dep node-modules dirs-size
```

#### Options

```
  -c, --cwd string   Working directory for the command (default "$PWD")
  -h, --help         help for dirs-size
```


### rev-dep node-modules installed-duplicates

Find and optimize duplicate package installations

#### Synopsis

Identifies packages that are installed multiple times in node_modules.
Can optimize storage by creating symlinks between duplicate packages.

```
rev-dep node-modules installed-duplicates [flags]
```

#### Examples

```
rev-dep node-modules installed-duplicates --optimize --size-stats
```

#### Options

```
  -c, --cwd string   Working directory for the command (default "$PWD")
  -h, --help         help for installed-duplicates
      --isolate      Create symlinks only within the same top-level node_module directories. By default optimize creates symlinks between top-level node_module directories (eg. when workspaces are used). Needs --optimize flag to take effect
      --optimize     Automatically create symlinks to deduplicate packages
      --size-stats   Print node modules dirs size before and after optimization. Might take longer than optimization itself
      --verbose      Show detailed information about each optimization
```


### rev-dep node-modules installed

List all installed npm packages in the project

#### Synopsis

Recursively scans node_modules directories to list all installed packages.
Helpful for auditing dependencies across monorepos.

```
rev-dep node-modules installed [flags]
```

#### Examples

```
rev-dep node-modules installed --include-modules=@myorg/*
```

#### Options

```
  -c, --cwd string                Working directory for the command (default "$PWD")
  -e, --exclude-modules strings   list of modules to exclude from the output
  -h, --help                      help for installed
  -i, --include-modules strings   list of modules to include in the output
```


### rev-dep node-modules missing

Find imported packages not listed in package.json

#### Synopsis

Identifies packages that are imported in your code but not declared
in your package.json dependencies.

```
rev-dep node-modules missing [flags]
```

#### Examples

```
rev-dep node-modules missing --entry-points=src/main.ts
```

#### Options

```
  -n, --count                              Only display the count of modules
  -c, --cwd string                         Working directory for the command (default "$PWD")
  -p, --entry-points strings               Entry point file(s) to start analysis from (default: auto-detected)
  -e, --exclude-modules strings            list of modules to exclude from the output
  -b, --files-with-binaries strings        Additional files to search for binary usages. Use paths relative to cwd
  -m, --files-with-node-modules strings    Additional files to search for module imports. Use paths relative to cwd
      --group-by-file                      Organize output by project file path
      --group-by-module                    Organize output by npm package name
  -h, --help                               help for missing
  -t, --ignore-type-imports                Exclude type imports from the analysis
  -i, --include-modules strings            list of modules to include in the output
      --package-json string                Path to package.json (default: ./package.json) (default "package.json")
      --pkg-fields-with-binaries strings   Additional package.json fields to check for binary usages
      --tsconfig-json string               Path to tsconfig.json (default: ./tsconfig.json) (default "tsconfig.json")
      --zero-exit-code                     Use this flag to always return zero exit code
```


### rev-dep node-modules unused

Find installed packages that aren't imported in your code

#### Synopsis

Compares package.json dependencies with actual imports in your codebase
to identify potentially unused packages.

```
rev-dep node-modules unused [flags]
```

#### Examples

```
rev-dep node-modules unused --exclude-modules=@types/*
```

#### Options

```
  -n, --count                              Only display the count of modules
  -c, --cwd string                         Working directory for the command (default "$PWD")
  -p, --entry-points strings               Entry point file(s) to start analysis from (default: auto-detected)
  -e, --exclude-modules strings            list of modules to exclude from the output
  -b, --files-with-binaries strings        Additional files to search for binary usages. Use paths relative to cwd
  -m, --files-with-node-modules strings    Additional files to search for module imports. Use paths relative to cwd
  -h, --help                               help for unused
  -t, --ignore-type-imports                Exclude type imports from the analysis
  -i, --include-modules strings            list of modules to include in the output
      --package-json string                Path to package.json (default: ./package.json) (default "package.json")
      --pkg-fields-with-binaries strings   Additional package.json fields to check for binary usages
      --tsconfig-json string               Path to tsconfig.json (default: ./tsconfig.json) (default "tsconfig.json")
      --zero-exit-code                     Use this flag to always return zero exit code
```


### rev-dep node-modules used

List all npm packages imported in your code

#### Synopsis

Analyzes your code to identify which npm packages are actually being used.
Helps keep track of your project's runtime dependencies.

```
rev-dep node-modules used [flags]
```

#### Examples

```
rev-dep node-modules used -p src/index.ts --group-by-module
```

#### Options

```
  -n, --count                              Only display the count of modules
  -c, --cwd string                         Working directory for the command (default "$PWD")
  -p, --entry-points strings               Entry point file(s) to start analysis from (default: auto-detected)
  -e, --exclude-modules strings            list of modules to exclude from the output
  -b, --files-with-binaries strings        Additional files to search for binary usages. Use paths relative to cwd
  -m, --files-with-node-modules strings    Additional files to search for module imports. Use paths relative to cwd
      --group-by-file                      Organize output by project file path
      --group-by-module                    Organize output by npm package name
  -h, --help                               help for used
  -t, --ignore-type-imports                Exclude type imports from the analysis
  -i, --include-modules strings            list of modules to include in the output
      --package-json string                Path to package.json (default: ./package.json) (default "package.json")
      --pkg-fields-with-binaries strings   Additional package.json fields to check for binary usages
      --tsconfig-json string               Path to tsconfig.json (default: ./tsconfig.json) (default "tsconfig.json")
```


### rev-dep resolve

Trace and display the dependency path between files in your project

#### Synopsis

Analyze and display the dependency chain between specified files.
Helps understand how different parts of your codebase are connected.

```
rev-dep resolve [flags]
```

#### Examples

```
rev-dep resolve -p src/index.ts -f src/utils/helpers.ts
```

#### Options

```
  -a, --all                     Show all possible resolution paths, not just the first one
      --compact-summary         Display a compact summary of found paths
  -c, --cwd string              Working directory for the command (default "$PWD")
  -p, --entry-points strings    Entry point file(s) to start analysis from (default: auto-detected)
  -f, --file string             Target file to check for dependencies
      --graph-exclude strings   Glob patterns to exclude files from dependency analysis
  -h, --help                    help for resolve
  -t, --ignore-type-imports     Exclude type imports from the analysis
      --package-json string     Path to package.json (default: ./package.json) (default "package.json")
      --tsconfig-json string    Path to tsconfig.json (default: ./tsconfig.json) (default "tsconfig.json")
```



<!-- cli-docs-end -->

## Made in 🇵🇱 and 🇯🇵 with 🧠 by [@jayu](https://github.com/jayu)

I hope that this small piece of software will help you discover and understood complexity of your project hence make you more confident while refactoring. If this tool was useful, don't hesitate to give it a ⭐!
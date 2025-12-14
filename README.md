<p align="center">
<img src="https://github.com/jayu/rev-dep/raw/master/logo.png" width="400" alt="Rev-dep logo">
</p>

<p align="center">
  <a href="#key-features-">Key Features</a>&nbsp;&nbsp;•&nbsp;&nbsp;  
  <a href="#installation-">Installation</a>&nbsp;&nbsp;•&nbsp;&nbsp; 
  <a href="#practical-examples-">Practical Examples</a>&nbsp;&nbsp;•&nbsp;&nbsp; 
  <a href="#cli-reference-">CLI Reference</a>
</p>

<p align="center">
  Dependency analysis and optimization toolkit for modern JavaScript and TypeScript projects.  
  <br>
  Trace imports, find unused code, clean dependencies — all from a blazing-fast CLI.
</p>

---

<img alt="rev-dep version" src="https://img.shields.io/npm/v/rev-dep"> <img alt="rev-dep license" src="https://img.shields.io/npm/l/rev-dep"> <img alt="rev-dep PRs welcome" src="https://img.shields.io/badge/PRs-welcome-brightgreen.svg?style=flat-square">



# **About 📣**

Working in large JS/TS projects makes it difficult to answer simple but crucial questions:

* Which files depend on this file?
* Is this file even used?
* Which files does this entry point import?
* Do I have circular dependencies?
* Which packages in node_modules are unused?
* Which modules take the most disk space?

Rev-dep helps you understand the real structure of your codebase so you can debug issues faster, refactor safely, and keep your dependencies clean.

It's particularly useful for JavaScript projects without TypeScript or test coverage — places where answering question "What will break if I change this" is not straightforward  


## **Why Rev-dep? 🤔**

Rev-dep is designed for **fast iteration** and **minimal, actionable results** — no noise, just answers.

### ✅ **Results in milliseconds**

Built in Go for speed. Even on large codebases, rev-dep responds almost instantly.

### ✅ **Actionable, list-based output**

You get **exact file paths**, **import chains**, and **clear dependency relationships** — the kind of information you can fix or clean up right away.

### ✅ **Designed for real-world JS/TS**

Works with mixed JS/TS projects, path aliases and thousands of files without configuration hassles.

### ✅ **Deep analysis, one CLI**

Unused files, unused or missing dependencies, reverse-imports, entry point detection, node_modules insights, dependency paths — everything in one tool.


### ✅ **Much faster than alternatives**

Rev-dep outperforms Madge, dpdm, dependency-cruiser, skott, knip, depcheck and other similar tools. 

For large project with 500k+ lines of code and 6k+ source code files get checks as fast as:

| Task | Execution Time [ms] | Alternative | Alternative Time [ms] | Slower Than Rev-dep | 
|------|-------|--------------|------|----|
| Find circular dependencies | 289 | dpdm-fast | 7061|  24x|
| Find unused files | 588 | knip | 6346 | 11x |
| Find unused node modules | 594 | knip | 6230 | 10x |
| Find missing node modules | 553 | knip| 6226 | 11x |
| List all files imported by an entry point | 229 | madge | 4467 | 20x | 
| Discover entry points | 323 | madge | 67000 | 207x
| Resolve dependency path between files | 228 | please suggest | 
| Count lines of code | 342 | please suggest | 
| Check node_modules disk usage | 1619 | please suggest | 
| Analyze node_modules directory sizes | 521 | please suggest | 

>Benchmark run on WSL Linux Debian Intel(R) Core(TM) i9-14900KF CPU @ 2.80GHz

# **Key Features 🚀**

* 🔍 **Reverse dependency lookup** — see all entry points that require a given file
* 🗂️ **Entry point discovery**
* 🧹 **Dead file detection**
* 📦 **Unused / missing / used node modules / dependencies analysis**
* 🔄 **Circular imports/dependencies detection**
* 🧭 **Trace all import paths between files**
* 📁 **List all files imported by any entry point**
* 📏 **Count actual lines of code (excluding comments and blanks)**
* 💽 **Node modules disk usage & size analysis**
* 💡 **Works with both JavaScript and TypeScript**
* ⚡ **Built for large codebases**

# **Installation 📦**

Install globally to use as a CLI tool:

```
yarn global add rev-dep
```

```
npm install -g rev-dep
```

```
pnpm global add rev-dep
```


# **Quick Examples ⚡**

A few instant-use examples to get a feel for the tool:

```bash
# Detect unused node modules
rev-dep node-modules unused

# Detect circular imports/dependencies
rev-dep circular

# List all entry points in the project
rev-dep entry-points

# Check which files an entry point imports
rev-dep files --entry-point src/index.ts

# Find every entry point that depends on a file
rev-dep resolve --file src/utils/math.ts

# Resolve dependency path between files
rev-dep resolve --file src/utils/math.ts --entry-point src/index.ts

```

# **Practical Examples 🔧**


Practical examples show how to use rev-dep commands to build code quality checks for your project.

### **How to identify where a file is used in the project**

```
rev-dep resolve --file path/to/file.ts
```

You’ll see all entry points that implicitly require that file, along with resolution paths.

### **How to check if a file is used**

```
rev-dep resolve --file path/to/file.ts --compact-summary
```

Shows how many entry points indirectly depend on the file.

### **How to identify dead files**

```
rev-dep entry-points
```

Exclude framework entry points if needed using `--result-exclude`.

### **How to list all files imported by an entry point**

```
rev-dep files --entry-point path/to/file.ts
```

Useful for identifying heavy components or unintended dependencies.

### **How to reduce unnecessary imports for an entry point**

1. List all files imported:

   ```
   rev-dep files --entry-point path/to/entry.ts
   ```
2. Identify suspicious files.
3. Trace why they are included:

   ```
   rev-dep resolve --file path/to/suspect --entry-points path/to/entry.ts --all
   ```

### **How to detect circular dependencies**

```
rev-dep circular
```

### **How to find unused node modules**

```
rev-dep node-modules unused
```

### **How to find missing node modules**

```
rev-dep node-modules missing
```

### **How to check node_modules space usage**

```
rev-dep node-modules dirs-size
```


## Reimplemented to achieve 7x-37x speedup

Rev-dep@2.0.0 was reimplemented in Go from scratch to leverage it's concurrency features and better memory management of compiled languages.

As a result v2 is up to 37x faster than v1 and consumes up to 13x less memory.

### Performance comparison

To compare performance rev-dep was benchmarked with hyperfine using 8 runs per test, taking mean time values as a result.
Benchmark was run on TypeScript codebase with 507658 lines of code and 5977 source code files.

Memory usage on Mac was measure using `/usr/bin/time` utility. Memory usage on Linux was not measured because I could't find reliable way to measure RAM usage on Linux. Subsequent runs had too much fluctuation.

### MacBook Pro with Apple M1 chip, 16GB of RAM and 256GB of storage. Power save mode off

| Command                                                      | V1 Time | V2 Time | Time Change | V1 RAM | V2 RAM | RAM Change |
| ------------------------------------------------------------ | ------- | ------- | ----------- | ------ | ------ | ---------- |
| List entry-points `rev-dep entry-points`                     | 6500ms  | 347ms   | 19x         | ~680MB | ~51MB  | 13x        |
| List entry-points with dependent files count `-pdc`          | 8333ms  | 782ms   | 11x         | ~885MB | ~110MB | 8x         |
| List entry-point files `rev-dep files`                       | 2729ms  | 400ms   | 7x          | ~330MB | ~36MB  | 9x         |
| Resolve dependency path `rev-dep resolve`                    | 2984ms  | 359ms   | 8x          | ~330MB | ~35MB  | 9x         |

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
- Windows x64

Go allows for cross-compiling, so I'm happy to build and distribute binaries for other platforms as well.
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
      --package-json string    Path to package.json (default: ./package.json)
      --tsconfig-json string   Path to tsconfig.json (default: ./tsconfig.json)
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
      --package-json string      Path to package.json (default: ./package.json)
      --print-deps-count         Show the number of dependencies for each entry point
      --result-exclude strings   Exclude files matching these glob patterns from results
      --result-include strings   Only include files matching these glob patterns in results
      --tsconfig-json string     Path to tsconfig.json (default: ./tsconfig.json)
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
      --package-json string    Path to package.json (default: ./package.json)
      --tsconfig-json string   Path to tsconfig.json (default: ./tsconfig.json)
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
      --package-json string                Path to package.json (default: ./package.json)
      --pkg-fields-with-binaries strings   Additional package.json fields to check for binary usages
      --tsconfig-json string               Path to tsconfig.json (default: ./tsconfig.json)
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
      --package-json string                Path to package.json (default: ./package.json)
      --pkg-fields-with-binaries strings   Additional package.json fields to check for binary usages
      --tsconfig-json string               Path to tsconfig.json (default: ./tsconfig.json)
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
      --package-json string                Path to package.json (default: ./package.json)
      --pkg-fields-with-binaries strings   Additional package.json fields to check for binary usages
      --tsconfig-json string               Path to tsconfig.json (default: ./tsconfig.json)
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
      --package-json string     Path to package.json (default: ./package.json)
      --tsconfig-json string    Path to tsconfig.json (default: ./tsconfig.json)
```



<!-- cli-docs-end -->

## Circular check performance comparison

Benchmark performed on TypeScript codebase with `6034` source code files and `518862` lines of code.

Benchmark performed on MacBook Pro with Apple M1 chip, 16GB of RAM and 256GB of Storage. Power save mode off.

Benchmark performed with `hyperfine` using 8 runs per test and 4 warm up runs, taking mean time values as a result. If single run was taking more than 10s, only 1 run was performed.

`rev-dep` circular check is **12 times** faster than the fastest alternative❗

| Tool | Version | Command to Run Circular Check | Time |
|------|---------|-------------------------------|------|
| 🥇 [rev-dep](https://github.com/jayu/rev-dep) | 2.0.0 | `rev-dep circular` | 397 ms |
| 🥈 [dpdm-fast](https://github.com/SunSince90/dpdm-fast) | 1.0.14 | `dpdm --no-tree --no-progress  --no-warning` + list of directories with source code  | 4960 ms |
| 🥉 [dpdm](https://github.com/acrazing/dpdm) | 3.14.0 | `dpdm  --no-warning` + list of directories with source code | 5030 ms |
| [skott](https://github.com/antoine-coulon/skott) | 0.35.6 | node skoscript using `findCircularDependencies` function  | 29575 ms |
| [madge](https://github.com/pahen/madge) | 8.0.0 | `madge --circular --extensions js,ts,jsx,tsx .` | 69328 ms |
| [circular-dependency-scanner](https://github.com/emosheeep/circular-dependency-scanner) | 2.3.0 | `ds` - out of memory error | n/a |

## Glossary

Some of the terms used in the problem space that **rev-dep** covers can be confusing.
Here is a small glossary to help you navigate the concepts.

### Dependency

A *dependency* can be understood literally. In the context of a project’s dependency graph, it may refer to:

* a **node module / package** (a package is a dependency of a project or file), or
* a **source code file** (a file is a dependency of another file if it imports it).

### Entry point

An *entry point* is a source file that is **not imported by any other file**.
It can represent:

* the main entry of the application
* an individual page or feature
* configuration or test bootstrap files

— depending on the project structure.

### Unused / Dead file

A file is considered *unused* or *dead* when:

* it is an **entry point** (nothing imports it), **and**
* running it does **not produce any meaningful output** or side effect.

In practice, such files can often be removed safely.

### Circular dependency

A *circular dependency* occurs when a file **directly or indirectly imports itself** through a chain of imports.

This can lead to unpredictable runtime behavior, uninitialized values, or subtle bugs.
However, circular dependencies between **TypeScript type-only imports** are usually harmless.

### Reverse dependency (or "dependents")

Files that *import* a given file.
Useful for answering: "What breaks if I change or delete this file?"

### Import graph / Dependency graph

A visual representation of how files or modules import each other.

### Missing dependency / unused node module

A module that your code imports but is **not listed in package.json**.

### Unused dependency / unused node module

A dependency listed in **package.json** that is **never imported** in the source code.

### Root directory / Project root

The top-level directory used as the starting point for dependency analysis.

## Made in 🇵🇱 and 🇯🇵 with 🧠 by [@jayu](https://github.com/jayu)

I hope that this small piece of software will help you discover and understood complexity of your project hence make you more confident while refactoring. If this tool was useful, don't hesitate to give it a ⭐!
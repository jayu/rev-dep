<h3 align="center">
  <code>rev⭠dep</code>
</h3>

<p align="center">
  Dependency debugging tool for JavaScript and TypeScript projects
</p>

---

<img alt="rev-dep version" src="https://img.shields.io/npm/v/rev-dep"> <img alt="rev-dep license" src="https://img.shields.io/npm/l/rev-dep"> <img alt="rev-dep PRs welcome" src="https://img.shields.io/badge/PRs-welcome-brightgreen.svg?style=flat-square">

## About

The tool was created help with daily dev struggles by answering these questions:

- What entry points my codebase have
- Which entry points uses a given file
- Which dependencies a given file has

This helps to debug project dependencies, plan refactoring, optimize bundles or plan code splitting.

It's especially useful in JS world without TypeScript or tests coverage.

It also helps to identify and eliminate dead files, understand the complexity of the file dependencies

[Jump to CLI reference](#CLI-reference)

[`export * from` problem](#Export-from-problem)

### Use cases

- You plan to refactor some file and you wonder which entry points are affected
- You are wondering wether a given source file is used
- You wonder if there are any dead files in your project
- You want to identify all dead files at once
- You want to verify if a given entry point imports only the required files
- You want to optimize the amount of files imported by an entry point

### How about dependency or bundle graphs?

There are tool that can output nice, visual representation of project dependencies like [webpack-bundle-analyzer](https://www.npmjs.com/package/webpack-bundle-analyzer) or [dependency-cruiser](https://www.npmjs.com/package/dependency-cruiser) (_which btw rev-dep uses for non-TS codebases_)

While graphs can be useful to identify major problems like too big bundle size or to visualize mess in your deps, it's hard to take any action based on them (_at least it was hard for me_ 🤷‍♂️)

`rev-dep` visualize dependencies as lists, so it's really easy to see where to cut the line to solve the problem.

## Getting Started

### Install globally to use as CLI tool

`yarn global add rev-dep`

or

`npm -g install rev-dep`

### Install in project to use as a module

`yarn add rev-dep`

or

`npm install rev-dep`

## Recipes

### How to identify where a file is used in the project?

### How to check if a file is used in the project?

### How to identify dead files in the project?

### How to check which files are imported by a given file?

### How to reduce amount of files imported by entry point?

## Usage

Project can be used as a CLI tool or as a module

### CLI Tool

For CLI usage see [CLI reference](#CLI-reference)

### Module

#### `find` Function

```ts
import { find } from "rev-dep";

const path = find({
  entryPoints: ["index.js"],
  filePath: "utils.js",
});

console.log(path);
```

#### `find` Function

```ts
import { find } from "rev-dep";

const path = find({
  entryPoints: ["index.js"],
  filePath: "utils.js",
});

console.log(path);
```

## CLI reference

<!-- cli-docs-start -->

### Command `resolve`

Checks if a filePath is required from entryPoint(s) and prints the resolution path

#### Usage

```sh
rev-dep resolve <filePath> [entryPoints...] [options]
```

#### Arguments

- `filePath` - Path to a file that should be resolved in entry points (**required**),\* `entryPoints...` - List of entry points to look for file (_optional_)

#### Options

- `-wc, --webpackConfig <path>` - path to webpack config to enable webpack aliases support (_optional_)
- `--cwd <path>` - path to a directory that should be used as a resolution root (_optional_)
- `-i --include <globs...>` - A list of globs to determine files included in entry points search (_optional_)
- `-e --exclude <globs...>` - A list of globs to determine files excluded in entry points search (_optional_)
- `-cs, --compactSummary` - print a compact summary of reverse resolution with a count of found paths (_optional_)
- `-a, --all` - finds all paths combination of a given dependency. Might work very slow or crash for some projects due to heavy usage of RAM (_optional_)

### Command `entry-points`

Print list of entry points in current directory

#### Usage

```sh
rev-dep entry-points [options]
```

#### Options

- `-wc, --webpackConfig <path>` - path to webpack config to enable webpack aliases support (_optional_)
- `--cwd <path>` - path to a directory that should be used as a resolution root (_optional_)
- `-i --include <globs...>` - A list of globs to determine files included in entry points search (_optional_)
- `-e --exclude <globs...>` - A list of globs to determine files excluded in entry points search (_optional_)
- `-pdc, --printDependenciesCount` - print count of entry point dependencies (_optional_)
- `-c, --count` - print just count of found entry points (_optional_)

### Command `files`

Get list of files required by entry point

#### Usage

```sh
rev-dep files <entryPoint> [options]
```

#### Arguments

- `entryPoint` - Path to entry point (**required**)

#### Options

- `-wc, --webpackConfig <path>` - path to webpack config to enable webpack aliases support (_optional_)
- `--cwd <path>` - path to a directory that should be used as a resolution root (_optional_)
- `-c, --count` - print only count of entry point dependencies (_optional_)

### Command `docs`

Generate documentation of available commands into md file.

#### Usage

```sh
rev-dep docs <outputPath> [options]
```

#### Arguments

- `outputPath` - path to output \*.md file (**required**)

#### Options

- `-hl, --headerLevel <value>` - Initial header level (_optional_)
<!-- cli-docs-end -->

## Export from problem

`rev-dep` attempts to also solve `export * from` by a babel plugin that can be used as follows

```js
// babel.config.js
module.exports = {
  plugins: [
    'rev-dep/babel'
  ]
};
```

The plugins is currently **experimental** and might not work for all codebases!

It helps by rewiring paths to re-exported modules

```ts
// file.ts
import { add } from "./utils";

// utils/index.ts

export * from "./math";
export * from "./otherModule";
export * from "./anotherModule";

// utils/math.ts

export const add = () => {};
```

And for `file.ts` it would rewire the import like this

```ts
// file.ts
import { add } from "./utils/math";
```

So as a result, we don't implicitly require `./otherModule` and `./anotherModule` which we will not use anyway

### Benefits

I don't have solid evidence for this, but I think it reduced RAM usage of the dev server I worked with (_blitz.js_). It crashed less often due to reaching heap size limit.

But for sure it reduced bundle size, _slightly_, but still 😀

It all depends on the the project dependencies structure.

By using the babel plugin you will reduce a risk of problems like implicitly importing `front-end` modules on the `server` or similar while still being able to benefit from short import paths.

Once I got an incident that, after a rebase with main branch, my project stopped compiling due to the problem caused by `export * from`. I spend a few hours debugging that, very frustrating.

## Contributing

Project is open to contributions, just rise an issue if you have some ideas about features or you noticed a bug. After discussion we can approach implementation :)

## Development

1. Clone repo
2. Install deps using `yarn`
3. Run `yarn build:watch`
4. Code!

For testing purpose use

`yarn dev [command] --cwd path/to/some/codebase`

or you can install CLI tool from the file system using

`yarn global add $PWD`

and then just run

`rev-dep`

## Made with 🧠 by [@jayu](https://github.com/jayu)

I hope that this small piece of software will help you discover and understood complexity of your project hence make you more confident while refactoring. If this tool was useful, don't hesitate to give it a ⭐!

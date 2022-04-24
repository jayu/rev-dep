<h3 align="center">
  <code>rev-dep</code>
</h3>

<p align="center">
  File dependency debugging tool for TypeScript projects
</p>

---

<img alt="rev-dep version" src="https://img.shields.io/npm/v/rev-dep"> <img alt="rev-dep license" src="https://img.shields.io/npm/l/rev-dep"> <img alt="rev-dep PRs welcome" src="https://img.shields.io/badge/PRs-welcome-brightgreen.svg?style=flat-square">

## Pardon 🤫

Since you landed here, you might be also interested in my other project - supercharged multiline code search and replace tool - [codeque.co](https://codeque.co) 🥳

## About 📣

The tool was created help with daily dev struggles by answering these questions:

👉 What entry points my codebase have

👉 Which entry points uses a given file

👉 Which dependencies a given file has

This helps to debug project dependencies, plan refactoring, optimize bundles or plan code splitting.

It's especially useful in JS world without TypeScript or tests coverage.

It also helps to identify and eliminate dead files, understand the complexity of the file dependencies

[🦘 Jump to CLI reference](#cli-reference-)

[🕸️ `export * from` problem](#export-from-problem)

### Use cases 🧑‍💻

- [You plan to refactor some file and you wonder which entry points are affected](#how-to-identify-where-a-file-is-used-in-the-project)
- [You are wondering wether a given source file is used](#how-to-check-if-a-file-is-used-in-the-project)
- [You wonder if there are any dead files in your project](#how-to-identify-dead-files-in-the-project)
- [You want to verify if a given entry point imports only the required files](#how-to-check-which-files-are-imported-by-a-given-file)
- [You want to optimize the amount of files imported by an entry point](#how-to-reduce-amount-of-files-imported-by-entry-point)

### How about dependency or bundle graphs?

There are tool that can output nice, visual representation of project dependencies like [webpack-bundle-analyzer](https://www.npmjs.com/package/webpack-bundle-analyzer) or [dependency-cruiser](https://www.npmjs.com/package/dependency-cruiser) (_which btw rev-dep uses for non-TS codebases_)

While graphs can be useful to identify major problems like too big bundle size or to visualize mess in your deps, it's hard to take any action based on them (_at least it was hard for me_ 🤷‍♂️)

`rev-dep` visualize dependencies as lists, so it's really easy to see where to cut the line to solve the problem.

## Getting Started 🎉

### Install globally to use as CLI tool

`yarn global add rev-dep`

or

`npm -g install rev-dep`

### Install in project to use as a module

`yarn add rev-dep`

or

`npm install rev-dep`

## Recipes 🌶️

### How to identify where a file is used in the project?

Just use `rev-dep resolve path/to/file.ts`

You will see all the entry points that implicitly require given file together with resolution path.

[`resolve` Command CLI reference](#command-resolve)

<details>
<summary>Example for the rev-dep repository</summary>

command:

`rev-dep resolve src/lib/utils.ts`

output:

```s
src/babel/index.js :

 ➞ src/babel/index.js
  ➞ src/lib/utils.ts
_____________________

src/cli/index.ts :

 ➞ src/cli/index.ts
  ➞ src/cli/createCommands.ts
   ➞ src/cli/resolve/index.ts
    ➞ src/lib/find.ts
     ➞ src/lib/getDepsTree.ts
      ➞ src/lib/getDepsSetWebpack.ts
       ➞ src/lib/utils.ts
_____________________

```

</details>

#### Getting more details about file resolution in given entry point

To find out all paths combination use `rev-dep resolve` with `-a` flag

> You might be surprised how complex dependency tree can be!

<details>
<summary>Example for the rev-dep repository</summary>

command:

`rev-dep resolve src/lib/utils.ts src/cli/index.ts --all`

output:

```s
src/cli/index.ts :

 ➞ src/cli/index.ts
  ➞ src/cli/createCommands.ts
   ➞ src/cli/resolve/index.ts
    ➞ src/lib/find.ts
     ➞ src/lib/getDepsTree.ts
      ➞ src/lib/getDepsSetWebpack.ts
       ➞ src/lib/utils.ts

 ➞ src/cli/index.ts
  ➞ src/cli/createCommands.ts
   ➞ src/cli/resolve/index.ts
    ➞ src/lib/find.ts
     ➞ src/lib/getEntryPoints.ts
      ➞ src/lib/getDepsTree.ts
       ➞ src/lib/getDepsSetWebpack.ts
        ➞ src/lib/utils.ts

 ➞ src/cli/index.ts
  ➞ src/cli/createCommands.ts
   ➞ src/cli/entryPoints/index.ts
    ➞ src/lib/getEntryPoints.ts
     ➞ src/lib/getDepsTree.ts
      ➞ src/lib/getDepsSetWebpack.ts
       ➞ src/lib/utils.ts

 ➞ src/cli/index.ts
  ➞ src/cli/createCommands.ts
   ➞ src/cli/files/index.ts
    ➞ src/lib/getDepsTree.ts
     ➞ src/lib/getDepsSetWebpack.ts
      ➞ src/lib/utils.ts

 ➞ src/cli/index.ts
  ➞ src/cli/createCommands.ts
   ➞ src/cli/resolve/index.ts
    ➞ src/lib/find.ts
     ➞ src/lib/getEntryPoints.ts
      ➞ src/lib/utils.ts

 ➞ src/cli/index.ts
  ➞ src/cli/createCommands.ts
   ➞ src/cli/entryPoints/index.ts
    ➞ src/lib/getEntryPoints.ts
     ➞ src/lib/utils.ts

 ➞ src/cli/index.ts
  ➞ src/cli/createCommands.ts
   ➞ src/cli/resolve/index.ts
    ➞ src/lib/find.ts
     ➞ src/lib/utils.ts

 ➞ src/cli/index.ts
  ➞ src/cli/createCommands.ts
   ➞ src/cli/resolve/index.ts
    ➞ src/lib/utils.ts

 ➞ src/cli/index.ts
  ➞ src/cli/createCommands.ts
   ➞ src/cli/entryPoints/index.ts
    ➞ src/lib/utils.ts

 ➞ src/cli/index.ts
  ➞ src/cli/createCommands.ts
   ➞ src/cli/files/index.ts
    ➞ src/lib/utils.ts

```

</details>

### How to check if a file is used in the project?

Use `rev-dep resolve path/to/file.ts --compactSummary`

As a result you will see total amount of entry points requiring a given file.

> Note that among the entry points list there might be some dead files importing the searched file

[`resolve` Command CLI reference](#command-resolve)

<details>
<summary>Example for the rev-dep repository</summary>

command:

`rev-dep resolve src/lib/utils.ts --compactSummary`

output:

```s
Results:

__tests__/find.test.js        : 0
babel.js                      : 0
bin.js                        : 0
scripts/addDocsToReadme.js    : 0
src/babel/index.js            : 1
src/cli/index.ts              : 1
src/lib/getMaxDepthInGraph.ts : 0
types.d.ts                    : 0

Total: 2
```

</details>

### How to identify dead files in the project?

Use `rev-dep entry-points` to get list of all files that are not required by any other files in the project.

You might want to exclude some file paths that are meant to be actual entry point like `index.js` or `**/pages/**` in `next.js` projects using `--exclude` flag. The same for configuration files like `babel.config.js`

Review the list and look for suspicious files like `src/ui/components/SomeComponent/index.js`

[`entry-points` command CLI reference](#command-entry-points)

<details>
<summary>Example for the rev-dep repository</summary>

command:

`rev-dep entry-points --exclude '__tests__/**' 'types.d.ts'`

output:

```s
babel.js
bin.js
scripts/addDocsToReadme.js
src/babel/index.js
src/cli/index.ts
src/lib/getMaxDepthInGraph.ts

```

The last one `src/lib/getMaxDepthInGraph.ts` is the source file that is not used at the moment.

The rest of them looks legit!

</details>

### How to check which files are imported by a given file?

To get a full list of files imported by given entry point use `rev-dep files path/to/file.ts`.

You can use `--count` flag if you are interested in the amount.

This is a good indicator of how heavy a given entry point or component is

[`files` command CLI reference](#command-files)

<details>
<summary>Example for the rev-dep repository</summary>

command:

`rev-dep files files src/cli/index.ts`

output:

```s
src/cli/index.ts
src/cli/createCommands.ts
package.json
src/cli/resolve/index.ts
src/cli/docs/index.ts
src/cli/entryPoints/index.ts
src/cli/files/index.ts
src/lib/find.ts
src/cli/resolve/types.ts
src/cli/resolve/formatResults.ts
src/lib/utils.ts
src/cli/commonOptions.ts
src/cli/docs/generate.ts
src/cli/entryPoints/types.ts
src/lib/getEntryPoints.ts
src/lib/buildDepsGraph.ts
src/cli/files/types.ts
src/lib/getDepsTree.ts
src/lib/types.ts
src/cli/docs/template.ts
src/lib/getDepsSetWebpack.ts
src/lib/cleanupDpdmDeps.ts

```

As you can see cli even import `package.json`. This is to print version of the cli

</details>

### How to reduce amount of files imported by entry point?

There is no easy how to for this process, but you can do it iteratively using `rev-dep` commands `files` and `resolve`

1. Get the list of files imported by entry-point

   `rev-dep files path/to/entry-point`

2. Identify some suspicious files on the list, components that should not be used on the given page or not related utility files
3. Get all resolution paths for a suspicious file

   `rev-dep resolve path/to/suspicious-file path/to/entry-point --all`

4. You would usually find out that there is some file, like directory `index` file that given entry point is using, which is mandatory, but as a side effect it imports a few files that are redundant for your entry point. In most cases you should be able to decouple the imports or reverse the dependency to cut off the resolution path for the unwanted file

## Usage 🎨

Project can be used as a CLI tool or as a module

### CLI Tool

For CLI usage see [CLI reference](#cli-reference-)

### Module

#### `resolve` Function

```ts
import { resolve } from "rev-dep";

const [paths] = await resolve({
  entryPoints: ["index.js"],
  filePath: "utils.js",
});

console.log(paths);
```

#### `getEntryPoints` Function

```ts
import { getEntryPoints } from "rev-dep";

const [entryPoints] = await getEntryPoints({
  cwd: process.cwd(),
});

console.log(entryPoints);
```

## CLI reference 📖

<!-- cli-docs-start -->

### Command `resolve`

Checks if a filePath is required from entryPoint(s) and prints the resolution path

#### Usage

```sh
rev-dep resolve <filePath> [entryPoints...] [options]
```

#### Arguments

- `filePath` - Path to a file that should be resolved in entry points (**required**)
- `entryPoints...` - List of entry points to look for file (_optional_)

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
  plugins: ["rev-dep/babel"],
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

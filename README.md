<h3 align="center">
  <code>rev🠄dep</code>
</h3>

<p align="center">
  A small tool for JavaScript project discovery
</p>

---

<img alt="rev-dep version" src="https://img.shields.io/npm/v/rev-dep"> <img alt="rev-dep license" src="https://img.shields.io/npm/l/rev-dep"> <img alt="rev-dep PRs welcome" src="https://img.shields.io/badge/PRs-welcome-brightgreen.svg?style=flat-square">

## Installation

### Install globally to use as CLI tool

`yarn global add rev-dep`

or

`npm -g install rev-dep`

### Install in project to use as a module

`yarn add rev-dep`

or

`npm install rev-dep`

## Example

For this repo

```sh
rev-dep resolve getDepsSet.js cli.js
```

will output

```
Results:

 ➞ cli.js
  ➞ find.js
   ➞ getDepsSet.js
```

Which says that `getDepsSet.js` file is used in `cli.js` entry point and is required through `find.js`

## About

The tool was created to determine places in the project where a particular file is used, to test wether the refactoring do not break functionalities.

It's especially useful in JS world without TypeScript or tests coverage.

Except the reverse dependency resolution path, it can print statistics about how many times a particular module is required in the project, which might be helpful for code-splitting planning.

## Usage

Project can be used as a CLI tool or as a regular JS module

### CLI Tool

Avaliable commands:

#### `resolve`

```sh
rev-dep resolve <filePath> <entryPoints...>
```

Available options are

- `-cs or --compactSummary` - instead of file paths print a compact summary of reverse resolution with a count of found paths
- `--verbose` - log currently performed operation
- `-wc, --webpackConfig <path>` - path to webpack config to enable webpack aliases support

### Module

#### `find` Function

```js
import { find } from 'rev-dep'

const path = find({
  entryPoints: ['index.js'],
  filePath: 'utils.js'
})

console.log(path)
```

#### `find` Options

- `entryPoints (Array)` - Array of entry points to build a tree for search. Usually it will be one entry point, but project can have many of them, eg. next.js application. **Required**
- `filePath (String)` - A file that we want to find path for. **Required**
- `skipRegex (String | RegExp)` - If a file path matches the pattern, we stop to traverse it's dependencies and do not include that file in the search tree. _Optional_, default: `'(node_modules|/__tests__|/__test__|/__mockContent__|.scss)'`
- `verbose (Boolean)` - when set to true, will print current operation performed by find function. _Optional_, default: `false`
- `cwd` - root for resolved files, must be an absolute path. _Optional_, default: `process.cwd()`
- `webpackConfig (String)` - path to webpack config to enable webpack aliases support

### Additional setup may be required

#### Resolving implicit file extensions

A vast amount of JS/TS projects are configured in a way that allows (or even forces) to skip file extensions in import statements. Rev-dep is strongly based on [dependency-cruiser](https://github.com/sverweij/dependency-cruiser) which by default support implicit file extensions for `*.js, *.cjs, *.mjs, *.jsx` files (check [source](https://github.com/sverweij/dependency-cruiser/blob/96e34d0cf158034f2b7c8cafe9cec72dd74d8c45/src/extract/transpile/meta.js)).
In order to resolve implicit extensions for other JS based languages it look for available corresponding compiler in `package.json`. If compiler is available, then extension is supported.

If you installed `rev-dep` **globally**, you will have appropriate compiler installed **globally** as well. If you use it as a module, your project has to have compiler in it's package.json.

For example, to support `*.ts` and `*.tsx` implicit extensions in globally installed `rev-dep`, you have to also install globally `typescript` package (see [source](https://github.com/sverweij/dependency-cruiser/blob/96e34d0cf158034f2b7c8cafe9cec72dd74d8c45/src/extract/transpile/typescript-wrap.js))

## Contributing

Project is open to contributions, just rise an issue if you have some ideas about features or you noticed a bug. After discussion we can approach implementation :)

## Development

1. Clone repo
2. Install deps using `yarn`
3. Run tests using `yarn test --watch`
4. Code!

For testing purpose you can install CLI tool from the file system using

`yarn global add file:ABSOLUTE_PATH_TO_REPO`

or just

`yarn global add file:$(echo $PWD)`

## Made with 🧠 by [@jayu](https://github.com/jayu)

I hope that this small piece of software will help you discover and understood complexity of your project hence make you more confident while refactoring. If this tool was useful, don't hesitate to give it a ⭐!

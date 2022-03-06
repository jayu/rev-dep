const path = require('path')
const getDepsSet = require('./getDepsSet')
const { parseDependencyTree } = require('dpdm');
const escapeGlob = require('glob-escape');

const buildTree = (deps) => (entryPoint) => {
  const inner = (path) => {
    const dep = deps.find((d) => d.source === path)
    if (dep === undefined) {
      throw new Error(`Dependency '${path}' not found!`)
    }

    return {
      path,
      children: dep.dependencies.map((d) => {
        if (d.circular) {
          return { path: 'CIRCULAR', children: [] }
        }
        return inner(d.resolved)
      })
    }
  }
  return inner(entryPoint)
}

const buildTreeDpdm = (_deps) => (entryPoint) => {

  const deps = Object.entries(_deps).reduce((deps, [id, data]) => {
    if (!id.includes('node_modules')) {
      return Object.assign({}, deps, { [id]: data ? data.filter(({ id }) => id && !id.includes('node_modules')) : data })
    }
    return deps
  }, {})

  const inner = (path, visited = new Set()) => {
    if (visited.has(path)) {
      return {
        path,
        children: []
      }
    }
    visited.add(path);
    const dep = deps[path]
    if (dep === undefined) {
      throw new Error(`Dependency '${path}' not found!`)
    }

    return {
      path,
      children: (dep || [])
        .map(d => d.id)
        .filter(path => path && !path.includes('node_modules'))
        .map((path) => inner(path, visited))
    }
  }
  return inner(entryPoint)
}

const traverse = (file) => (tree) => {
  if (tree.path === file) {
    return [[file]]
  } else {
    return tree.children
      .map(traverse(file)) // [ [[]],[[]],[[]] ]
      .filter((p) => p.length > 0)
      .map((pathsArr) => pathsArr.filter((p) => p.length > 0))
      .reduce((flat, subPath) => {
        return [...flat, ...subPath]
      }, [])
      .map((p) => [tree.path, ...p])
  }
}

const removeInitialDot = (path) => path.replace(/^\.\//, '')

const _resolveAbsolutePath = (cwd) => (p) => typeof p === 'string' ? path.resolve(cwd, p) : p

const find = async ({
  entryPoints,
  filePath,
  skipRegex,
  verbose,
  webpackConfig,
  typescriptConfig,
  cwd = process.cwd()
}) => {
  const resolveAbsolutePath = _resolveAbsolutePath(cwd)
  const absoluteEntryPoints = entryPoints.map(resolveAbsolutePath)
  const globEscapedEntryPoints = entryPoints.map(escapeGlob);

  if (verbose) {
    console.log('Entry points:')
    console.log(absoluteEntryPoints)
    console.log('Getting dependency set for entry points...')
  }
  const deps = typescriptConfig ? await parseDependencyTree(globEscapedEntryPoints, { context: process.cwd() }) : getDepsSet(
    absoluteEntryPoints,
    skipRegex,
    resolveAbsolutePath(webpackConfig),
    resolveAbsolutePath(typescriptConfig)
  )

  const cleanedEntryPoints = entryPoints.map(removeInitialDot)
  const cleanedFilePath = removeInitialDot(filePath)
  if (verbose) {
    console.log('Building dependency trees for entry points...')
  }
  const forest = cleanedEntryPoints.map(typescriptConfig ? buildTreeDpdm(deps) : buildTree(deps))
  if (verbose) {
    console.log('Finding paths in dependency trees...')
  }
  const resolvedPaths = forest.reduce((allPaths, tree) => {
    const paths = traverse(cleanedFilePath)(tree)
    return [...allPaths, paths]
  }, [])
  return resolvedPaths
}

module.exports = { find }

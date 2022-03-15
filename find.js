const path = require('path')
const fs = require('fs/promises');
const getDepsSet = require('./getDepsSet')
const {
  parseDependencyTree
} = require('dpdm');
const escapeGlob = require('glob-escape');
const minimatch = require("minimatch")

const getEntryPoints = (deps, exclude = []) => {
  const referencedIds = new Set();

  Object.values(deps).forEach((entry) => {
    if (entry !== null) {
      entry.forEach(({
        id
      }) => referencedIds.add(id))
    }
  })

  return Object.keys(deps)
    .filter((id) => /\.(ts|tsx|mjs|js|jsx)$/.test(id) && !/node_modules/.test(id) && !referencedIds.has(id))
    .filter((id) => exclude.reduce((result, pattern) => result && !minimatch(id, pattern), true))
}

const getMaxDepth = (depth = 1, path = [], cache = new Map()) => {

  return (tree) => {
    const depthFromCache = cache.get(tree.path)

    if (depthFromCache) {
      return depthFromCache
    }

    const newPath = [...path, tree.path];

    if (tree.children.length === 0) {
      return [depth, newPath]
    }

    const results = tree.children.map(getMaxDepth(depth + 1, newPath, cache))

    const maxChildDepth = Math.max(...results.map(([depth]) => depth))

    const itemWithMaxDepth = results.find(([depth]) => depth === maxChildDepth);

    cache.set(tree.path, itemWithMaxDepth)

    return itemWithMaxDepth
  }
}

const cleanupDpdmDeps = (deps) => {
  const newDeps = {}

  Object.entries(deps).forEach(([id, dependencies]) => {
    if (!id.includes('node_modules')) {
      newDeps[id] = dependencies.filter(({
        id
      }) => id && !id.includes('node_modules')).map(({
        id,
        request
      }) => ({
        id,
        request
      }))
    }
  })

  return newDeps
}

const buildTreeDpdm = (deps, maxDepth, filePath) => (entryPoint) => {
  console.log('build tree for', entryPoint)

  const cache = new Map();
  let fileNode = null;

  const inner = (path, visited = new Set(), depth = 1, parent = null) => {
    const nodeFromCache = cache.get(path)

    if (nodeFromCache) {
      nodeFromCache.parents.push(parent)

      return nodeFromCache
    }

    const localVisited = new Set(visited)

    if (localVisited.has(path)) {
      console.error('CIRCULAR DEP', ...localVisited.values(), path)
      return {
        path: 'CIRCULAR',
        parent,
        children: []
      }
    }

    localVisited.add(path);

    const dep = deps[path]
    if (dep === undefined) {
      throw new Error(`Dependency '${path}' not found!`)
    }

    const node = {
      parents: parent ? [parent] : [],
      path,
    }

    const children = (dep || [])
      .map(d => d.id)
      .filter(path => path && !path.includes('node_modules'))
      .map((path) => inner(path, localVisited, depth + 1, node))

    node.children = children
    cache.set(path, node)

    // console.log('cache set', node)

    if (path === filePath) {
      fileNode = node
    }

    return node
  }
  return [inner(entryPoint), fileNode]
}

const resolvePathsToRoot = (node, onlyFirst = false, resolvedPaths = [
  []
]) => {
  const newPaths = resolvedPaths.map((resolvedPath) => [node.path, ...resolvedPath])
  if (node.parents.length === 0) {
    return newPaths
  }

  if (onlyFirst) {
    return resolvePathsToRoot(node.parents[0], onlyFirst, newPaths)
  }
  return node.parents.map((parentPath) => resolvePathsToRoot(parentPath, false, newPaths)).flat(1)
}
const removeInitialDot = (path) => path.replace(/^\.\//, '')

const _resolveAbsolutePath = (cwd) => (p) => typeof p === 'string' ? path.resolve(cwd, p) : p

const asyncFilter = async (arr, predicate) => {
  const results = await Promise.all(arr.map(predicate));

  return arr.filter((_v, index) => results[index]);
}

const getDirectoriesForEntryPoints = async (dir) => {
  const entries = await fs.readdir(dir)
  const directories = await asyncFilter(entries, async (pathName) => {
    if (pathName === 'node_modules' || pathName.startsWith('.')) {
      return false
    }
    const stat = await fs.lstat(path.resolve(dir, pathName))
    return stat.isDirectory()
  })

  const joinedWithDir = directories.map((pathName) => path.join(dir, pathName));

  return [...joinedWithDir, ...(await Promise.all(joinedWithDir.map(getDirectoriesForEntryPoints))).flat(1)]

}

/**
 * TODO
 * - support cruiser conditionally
 * - reuse already scanned deps
 */
const getPossibleEntryPoints = async (cwd) => {

  const dirs = await getDirectoriesForEntryPoints(cwd);
  console.log('dirs', dirs);
  const globs = dirs.map((dirName) => path.relative(cwd, dirName)).map((dirName) => `${dirName}/*`)
  console.log('globs', globs)
  const possibleEntryPoints = getEntryPoints(await parseDependencyTree(
    ['*', ...globs], {
    context: cwd,
  }
  ), ['**/*stories*', '**stories**', '**/*test*', '**/pages/**', '**/api/**', 'cypress/**', '**/*config.*']);
  console.log('possibleEntryPoints', possibleEntryPoints);
  console.log('possibleEntryPoints.length', possibleEntryPoints.length);
  return possibleEntryPoints
}

const find = async ({
  entryPoints: _entryPoints,
  filePath,
  skipRegex,
  verbose,
  webpackConfig,
  typescriptConfig,
  cwd = process.cwd(),
  maxDepth,
  printMaxDepth,
  printDependentCount,
  checkOnly
}) => {
  const resolveAbsolutePath = _resolveAbsolutePath(cwd)
  const entryPoints = _entryPoints.length > 0 ? _entryPoints : await getPossibleEntryPoints(cwd)
  const absoluteEntryPoints = entryPoints.map(resolveAbsolutePath)
  const globEscapedEntryPoints = entryPoints.map(escapeGlob);

  if (verbose) {
    console.log('Entry points:')
    console.log(absoluteEntryPoints)
    console.log('Getting dependency set for entry points...')
  }

  const deps = typescriptConfig ? cleanupDpdmDeps(await parseDependencyTree(globEscapedEntryPoints, {
    context: cwd
  })) : getDepsSet(
    absoluteEntryPoints,
    skipRegex,
    resolveAbsolutePath(webpackConfig),
    resolveAbsolutePath(typescriptConfig)
  )

  console.log('deps', deps);

  const cleanedEntryPoints = entryPoints.map(removeInitialDot)
  const cleanedFilePath = removeInitialDot(filePath)
  if (verbose) {
    console.log('Building dependency trees for entry points...')
  }
  const forest = cleanedEntryPoints.map(buildTreeDpdm(deps, maxDepth, cleanedFilePath))

  if (printMaxDepth) {
    forest.forEach((maybeTree) => {
      const tree = typescriptConfig ? maybeTree[0] : maybeTree
      console.log('Max depth', ...getMaxDepth()(tree))
    })
  }

  //todo it does not work properly for multiple entry points
  // Need to count vertices from graph
  if (printDependentCount) {
    console.log('Deps count ', deps.length || Object.keys(deps).length)
  }

  if (verbose) {
    console.log('Finding paths in dependency trees...')
  }

  const resolvedPaths = forest.reduce((allPaths, [tree, fileNode], idx) => {
    // console.log('FileNode', fileNode)
    console.log('resolve for', entryPoints[idx])
    if (!fileNode) {
      console.log('0')
      return [...allPaths, []]
    }
    const pathsForTree = resolvePathsToRoot(fileNode, checkOnly)
    console.log(pathsForTree.length)

    return [...allPaths, pathsForTree]
  }, [])
  return resolvedPaths
}

module.exports = {
  find
}
const path = require('path')
const getDepsSet = require('./getDepsSet')
const { parseDependencyTree } = require('dpdm');
const escapeGlob = require('glob-escape');

const getEntryPoints = (deps) => {
  const referencedIds = new Set();

  Object.values(deps).forEach((entry) => {
    if (entry !== null) {
      entry.forEach(({ id }) => referencedIds.add(id))
    }
  })

  return Object.keys(deps).filter((id) => !/(api|pages)/.test(id) && /\.(ts|tsx|mjs|js|jsx|json)$/.test(id) && !/node_modules|\.test|\.sql|\.stories/.test(id) && !referencedIds.has(id))
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

const buildTreeDpdm = (_deps, maxDepth, filePath) => (entryPoint) => {

  const deps = Object.entries(_deps).reduce((deps, [id, data]) => {
    if (!id.includes('node_modules')) {
      return Object.assign({}, deps, { [id]: data ? data.filter(({ id }) => id && !id.includes('node_modules')) : data })
    }
    return deps
  }, {})

  const cache = new Map();
  const fileNodes = [];

  const inner = (path, visited = new Set(), depth = 1, parent = null) => {
    // console.log(visited)
    const nodeFromCache = cache.get(path)

    if (nodeFromCache) {
      nodeFromCache.parents.push(parent)

      return nodeFromCache
    }

    const localVisited = new Set(visited)

    // if (depth > maxDepth) {
    //   return {
    //     path,
    //     parent,
    //     children: []
    //   }
    // }

    if (localVisited.has(path)) {
      // throw new Error('circular' + ([...localVisited.values(), path].join(', ')))
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
      fileNodes.push(node)
    }

    return node
  }
  return [inner(entryPoint), fileNodes]
}

const traverse = (file) => (tree) => {
  if (tree.path === file) {
    // console.log('found leaf')
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

const resolvePathsToRoot = (node, resolvedPaths = [[]]) => {
  const newPaths = resolvedPaths.map((resolvedPath) => [node.path, ...resolvedPath])
  if (node.parents.length === 0) {
    console.log('fount path end', resolvedPaths.length)
    return newPaths
  }

  return node.parents.map((parentPath) => resolvePathsToRoot(parentPath, newPaths)).flat(1)
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
  cwd = process.cwd(),
  maxDepth,
  printMaxDepth,
  printDependentCount
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

  console.log('deps', deps)

  const possibleEntryPoints = getEntryPoints(await parseDependencyTree(
    ['./db/**/*', './app/**/*', './jobs/**/*', './utils/**/*'],
    { context: process.cwd() }
  ));
  console.log('possibleEntryPoints', possibleEntryPoints);
  console.log('possibleEntryPoints', possibleEntryPoints.slice(100));
  console.log('possibleEntryPoints', possibleEntryPoints.slice(200));

  const cleanedEntryPoints = entryPoints.map(removeInitialDot)
  const cleanedFilePath = removeInitialDot(filePath)
  if (verbose) {
    console.log('Building dependency trees for entry points...')
  }
  const forest = cleanedEntryPoints.map(typescriptConfig ? buildTreeDpdm(deps, maxDepth, cleanedFilePath) : buildTree(deps))

  if (printMaxDepth) {
    forest.forEach((maybeTree) => {
      const tree = typescriptConfig ? maybeTree[0] : maybeTree
      console.log('Max depth', ...getMaxDepth()(tree))
    })
  }

  if (printDependentCount) {
    console.log('Deps count ', deps.length || Object.keys(deps).length)
  }

  if (verbose) {
    console.log('Finding paths in dependency trees...')
  }
  if (!typescriptConfig) {

    const resolvedPaths = forest.reduce((allPaths, tree) => {
      const paths = traverse(cleanedFilePath)(tree)
      return [...allPaths, paths]
    }, [])
    return resolvedPaths
  }
  else {
    const resolvedPaths = forest.reduce((allPaths, [tree, fileNodes]) => {
      console.log(fileNodes[0])
      const pathsForTree = fileNodes.map((fileNode) => resolvePathsToRoot(fileNode)).flat(1)
      console.log(pathsForTree)
      return [...allPaths, pathsForTree]
    }, [])
    return resolvedPaths
  }
}

module.exports = { find }

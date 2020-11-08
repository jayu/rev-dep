const path = require('path')
const getDepsSet = require('./getDepsSet')

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

const resolveAbsolutePath = (cwd) => (p) => path.resolve(cwd, p)

const find = ({
  entryPoints,
  file,
  skipRegex,
  verbose,
  cwd = process.cwd()
}) => {
  const absoluteEntryPoints = entryPoints.map((e) =>
    resolveAbsolutePath(cwd)(e)
  )

  if (verbose) {
    console.log('Entry points:')
    console.log(absoluteEntryPoints)
    console.log('Getting dependency set for entry points...')
  }
  const deps = getDepsSet(absoluteEntryPoints, skipRegex)
  const cleanedEntryPoints = entryPoints.map(removeInitialDot)
  if (verbose) {
    console.log('Building dependency trees for entry points...')
  }
  const forest = cleanedEntryPoints.map(buildTree(deps))
  if (verbose) {
    console.log('Finding paths in dependency trees...')
  }
  const resolvedPaths = forest.reduce((allPaths, tree) => {
    const paths = traverse(file)(tree)
    return [...allPaths, paths]
  }, [])
  return resolvedPaths
}

module.exports = { find }
